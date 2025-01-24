package pkg

import (
	"context"
	"fmt"
	"strings"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	"github.com/cubrid/cubrid-operator/pkg/settings"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GetStatefulSetFromPod retrieves the StatefulSet object that owns the given Pod.
func GetStatefulSetFromPod(k8sClient client.Client, pod *corev1.Pod) (*appsv1.StatefulSet, error) {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "StatefulSet" {
			statefulSet := &appsv1.StatefulSet{}
			err := k8sClient.Get(context.Background(), client.ObjectKey{
				Namespace: pod.Namespace,
				Name:      ownerRef.Name,
			}, statefulSet)

			if err != nil {
				if errors.IsNotFound(err) {
					return nil, fmt.Errorf("statefulset not found: %s", ownerRef.Name)
				}
				return nil, fmt.Errorf("failed to get StatefulSet: %w", err)
			}
			log.Log.Info("GetStatefulSetFromPod", "statefulSet", statefulSet.Name)
			return statefulSet, nil
		}
	}
	return nil, fmt.Errorf("pod %s is not owned by any StatefulSet", pod.Name)
}

// GetCubridDBFromStatefulSet retrieves the CubridDB object associated with the given StatefulSet.
func GetCubridDBFromStatefulSet(k8sClient client.Client, statefulSet *appsv1.StatefulSet) (*cubridv1.CubridDB, error) {
	name, ok := statefulSet.Labels["app"] // 적절한 레이블 키를 사용하세요.
	if !ok {
		return nil, fmt.Errorf("CubridDB name label not found in StatefulSet %s", statefulSet.Name)
	}

	cubridDB := &cubridv1.CubridDB{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{
		Namespace: statefulSet.Namespace,
		Name:      name,
	}, cubridDB)

	if err != nil {
		return nil, fmt.Errorf("failed to get CubridDB: %w", err)
	}

	return cubridDB, nil
}

// GetCubridDBFromPod는 Pod 객체를 사용하여 CubridDB 객체를 반환하는 함수입니다.
func GetCubridDBFromPod(c client.Client, pod *corev1.Pod) (*cubridv1.CubridDB, error) {
	statefulSet, err := GetStatefulSetFromPod(c, pod)
	if err != nil {
		return nil, err
	}

	cubridDB, err := GetCubridDBFromStatefulSet(c, statefulSet)
	if err != nil {
		return nil, err
	}

	return cubridDB, nil
}

func GetStatefulSetFromCubridDB(k8sClient client.Client, cubridDB *cubridv1.CubridDB) (*appsv1.StatefulSet, error) {
	// StatefulSet을 가져옴
	statefulSet := &appsv1.StatefulSet{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{
		Namespace: cubridDB.Namespace,
		Name:      cubridDB.Name,
	}, statefulSet); err != nil {
		return nil, err
	}

	return statefulSet, nil
}

// GetCubridDBByName은 주어진 네임스페이스와 이름으로 CubridDB 객체를 반환하는 함수입니다.
func GetCubridDBByName(c client.Client, namespace, name string) (*cubridv1.CubridDB, error) {
	cubridDB := &cubridv1.CubridDB{}
	// 네임스페이스와 이름을 사용하여 CubridDB 객체를 조회
	if err := c.Get(context.Background(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, cubridDB); err != nil {
		return nil, err
	}
	return cubridDB, nil
}

// UpdateReplicaLink는 주어진 네임스페이스와 이름의 CubridDB 객체의 Spec를 수정한 후 적용하는 함수입니다.
func UpdateReplicaLink(c client.Client, cubridDB *cubridv1.CubridDB, name string, updatType int) error {

	// Spec의 일부를 수정하는 함수 호출
	if updatType == settings.ADD_REPLICALINK {
		AddReplicaLink(cubridDB, name)
	} else if updatType == settings.DELETE_REPLICALINK {
		RemoveReplicaLink(cubridDB, name)
	} else {
		return nil
	}

	// 변경된 객체를 클러스터에 적용
	if err := c.Update(context.Background(), cubridDB); err != nil {
		return err
	}

	return nil
}

// AddReplicaLink adds a replica to ReplicaLink if it does not exist
func AddReplicaLink(c *cubridv1.CubridDB, name string) {
	log.Log.Info("AddReplicaLink", "name", name)

	// HAmodeType이 nil인지 확인
	if c.Spec.Replication == nil {
		c.Spec.Replication = &cubridv1.Replication{} // Replication을 초기화
	}

	if c.Spec.Replication.HAmodeType == nil {
		c.Spec.Replication.HAmodeType = &cubridv1.HAmodeType{} // HAmodeType을 초기화
	}

	if c.Spec.Replication.HAmodeType.CubridRef == nil {
		c.Spec.Replication.HAmodeType.CubridRef = &cubridv1.CubridRef{} // CubridRef를 초기화
	}

	// 원본 ReplicaLink 필드를 참조
	replicaLink := &c.Spec.Replication.HAmodeType.CubridRef.ReplicaLink
	links := strings.Split(*replicaLink, ":")

	// 이미 존재하는 경우 추가하지 않음
	for _, link := range links {
		if link == name {
			return
		}
	}

	// 값 추가
	if *replicaLink == "" {
		*replicaLink = name
	} else {
		*replicaLink += ":" + name
	}
}

// RemoveReplicaLink removes a replica from ReplicaLink if it exists
func RemoveReplicaLink(c *cubridv1.CubridDB, name string) {
	log.Log.Info("RemoveReplicaLink", "name", name)

	if c.Spec.Replication.HAmodeType == nil || c.Spec.Replication.HAmodeType.CubridRef == nil {
		return
	}

	// 원본 ReplicaLink 필드를 참조
	replicaLink := &c.Spec.Replication.HAmodeType.CubridRef.ReplicaLink
	links := strings.Split(*replicaLink, ":")
	var newLinks []string

	// 해당 replica를 제외한 나머지 값만 저장
	for _, link := range links {
		if link != name {
			newLinks = append(newLinks, link)
		}
	}

	// 수정된 값으로 원본 필드 업데이트
	*replicaLink = strings.Join(newLinks, ":")
}

// SplitReplicaLink splits ReplicaLink into individual components
func SplitReplicaLink(c *cubridv1.CubridDB) []string {

	replicaLink := &c.Spec.Replication.HAmodeType.CubridRef.ReplicaLink

	if c.Spec.Replication.HAmodeType.CubridRef.ReplicaLink == "" {
		return []string{}
	}
	return strings.Split(*replicaLink, ":")
}

func GetCubridDBPodList(
	ctx context.Context,
	c client.Client,
	name, namespace string,
) (*corev1.PodList, string, error) {
	log := log.FromContext(ctx)
	var statefulSet appsv1.StatefulSet
	var serviceName string

	log.Info("getCubridDBPodList", "Statefulset Name", name, "Namespace", namespace)

	if err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &statefulSet); err != nil {
		return nil, "", fmt.Errorf("unable to fetch StatefulSet: %w", err)
	}

	log.Info("getCubridDBPodList", "Statefulset MatchLabels", statefulSet.Spec.Selector.MatchLabels)

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(statefulSet.Spec.Selector.MatchLabels),
	}
	if err := c.List(ctx, podList, listOpts...); err != nil {
		return nil, "", fmt.Errorf("unable to list pods: %w", err)
	}

	serviceName = statefulSet.Spec.ServiceName

	return podList, serviceName, nil
}

func UpdateCubridDB(ctx context.Context, c client.Client, pod *corev1.Pod) error {
	// 1. Pod의 OwnerReferences에서 StatefulSet을 찾기
	var statefulSetName string
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "StatefulSet" {
			statefulSetName = ownerRef.Name
			log.Log.V(1).Info("UpdateCubridDB", "statefulset name", statefulSetName)
			break
		}
	}

	if statefulSetName == "" {
		return fmt.Errorf("no StatefulSet owner found for Pod %s", pod.Name)
	}

	// 2. StatefulSet 가져오기
	var statefulSet appsv1.StatefulSet
	if err := c.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: statefulSetName}, &statefulSet); err != nil {
		return fmt.Errorf("failed to get StatefulSet %s: %v", statefulSetName, err)
	}

	// 3. StatefulSet의 OwnerReferences에서 CubridDB 찾기
	var cubridDBName string
	for _, ownerRef := range statefulSet.OwnerReferences {
		if ownerRef.Kind == "CubridDB" {
			cubridDBName = ownerRef.Name
			break
		}
	}

	if cubridDBName == "" {
		return fmt.Errorf("no CubridDB owner found for StatefulSet %s", statefulSetName)
	}

	// 4. CubridDB 가져오기
	var cubridDB cubridv1.CubridDB
	if err := c.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: cubridDBName}, &cubridDB); err != nil {
		return fmt.Errorf("failed to get CubridDB %s: %v", cubridDBName, err)
	}

	// StatefulSet의 replicas 값을 CR에 반영합니다.
	if *statefulSet.Spec.Replicas != cubridDB.Spec.Replication.Replicas {
		cubridDB.Spec.Replication.Replicas = *statefulSet.Spec.Replicas

		// CR 업데이트
		if err := c.Update(ctx, &cubridDB); err != nil {
			return fmt.Errorf("unable to update CubridDB: %v", err)
		}
	}

	return nil
}

func InitCubridRef(cubridDB *cubridv1.CubridDB) *cubridv1.CubridRef {
	ref := cubridDB.Spec.Replication.HAmodeType.CubridRef
	if ref == nil {
		ref = &cubridv1.CubridRef{}
		cubridDB.Spec.Replication.HAmodeType.CubridRef = ref
	}
	return ref
}

func NewCubridRef(cubridDB *cubridv1.CubridDB) *cubridv1.CubridRef {
	ref := cubridDB.Spec.Replication.HAmodeType.CubridRef
	if ref == nil {
		return &cubridv1.CubridRef{
			Name:        "",
			Namespace:   "",
			ReplicaLink: "",
		}
	}
	return ref
}

func AllContainersRunning(pod *corev1.Pod) (bool, error) {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running == nil {
			if containerStatus.State.Waiting != nil {
				return false,
					fmt.Errorf("container %s is waiting: %v",
						containerStatus.Name,
						containerStatus.State.Waiting.Reason,
					)
			} else if containerStatus.State.Terminated != nil {
				return false,
					fmt.Errorf("container %s is terminated: %v",
						containerStatus.Name,
						containerStatus.State.Terminated.Reason,
					)
			}
			return false, nil
		}
	}
	return true, nil
}

func CheckIfPodIsRunning(c client.Client, ctx context.Context, podName string, namespace string) (bool, error) {
	var pod corev1.Pod
	err := c.Get(ctx, types.NamespacedName{Name: podName, Namespace: namespace}, &pod)
	if err != nil {
		return false, fmt.Errorf("failed to get Pod %s/%s: %v", namespace, podName, err)
	}

	return pod.Status.Phase == corev1.PodRunning, nil
}

func FetchStatefulSet(ctx context.Context, c client.Client, name, namespace string) (*appsv1.StatefulSet, error) {
	var statefulSet appsv1.StatefulSet
	if err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &statefulSet); err != nil {
		return nil, err
	}
	return &statefulSet, nil
}

func FetchPodList(
	ctx context.Context,
	c client.Client,
	statefulSet *appsv1.StatefulSet,
	namespace string,
) ([]corev1.Pod, error) {
	labelSelector := labels.SelectorFromSet(statefulSet.Spec.Selector.MatchLabels)
	var podList corev1.PodList
	if err := c.List(ctx, &podList, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
	}); err != nil {
		return nil, err
	}
	return podList.Items, nil
}

func FetchServiceName(ctx context.Context, c client.Client, name, namespace string) (string, error) {
	var statefulSet appsv1.StatefulSet
	if err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &statefulSet); err != nil {
		return "", err
	}
	return statefulSet.Spec.ServiceName, nil
}

func CreateDNSShortName(podName, serviceName string) string {
	return fmt.Sprintf(settings.POD_DNS_SHORT_NAME, podName, serviceName)
}

func CreateDNSFullName(podName, serviceName, namespace string) string {
	return fmt.Sprintf(settings.POD_DNS_FULL_NAME, podName, serviceName, namespace)
}

func GenerateFullURL(podDNSName string, port int) string {
	return fmt.Sprintf(settings.CMS_HTTPS_URL, podDNSName, port)
}

func CreatePodFullURLs(
	ctx context.Context,
	c client.Client,
	statefulSetName,
	namespace string,
	port int,
) ([]string, error) {
	statefulSet, err := FetchStatefulSet(ctx, c, statefulSetName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch StatefulSet: %w", err)
	}

	pods, err := FetchPodList(ctx, c, statefulSet, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Pods: %w", err)
	}

	serviceName, err := FetchServiceName(ctx, c, statefulSetName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch service: %w", err)
	}

	urls := make([]string, 0, len(pods))
	for _, pod := range pods {
		podDNSName := CreateDNSFullName(pod.Name, serviceName, namespace)
		fullURL := GenerateFullURL(podDNSName, settings.CMS_PORT)
		urls = append(urls, fullURL)
	}

	return urls, nil
}
