package manage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	"github.com/cubrid/cubrid-operator/pkg"
	"github.com/cubrid/cubrid-operator/pkg/settings"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type HAHandler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

var hahandlelog = log.Log.WithName("StatufulSet")

func NewHAHandler(client client.Client, scheme *runtime.Scheme, config *rest.Config) *HAHandler {
	return &HAHandler{
		Client: client,
		Scheme: scheme,
		Config: config,
	}
}

func (r *HAHandler) HandleHAMode(
	ctx context.Context,
	cubridDB *cubridv1.CubridDB,
	req ctrl.Request,
) (ctrl.Result, error) {
	var result ctrl.Result
	var err error
	var msCubridDBName, replicaCubridDBName string

	switch cubridDB.HAmodeType() {
	case settings.HA_MASTER_SLAVE_TYPE:
		msCubridDBName = cubridDB.Name

		// Ensure that CubridRef is initialized
		replicaRef := pkg.InitCubridRef(cubridDB)
		replicaCubridDBName = replicaRef.ReplicaLink

		hahandlelog.Info("Master-Slave info", "Master-Slave", msCubridDBName, "Replica", replicaCubridDBName)

		// Update the Master-Slave configuration
		if result, err = r.SetMasterSlaveConfig(
			ctx,
			msCubridDBName,
			replicaCubridDBName,
			cubridDB.Namespace,
			req,
		); err != nil {
			return result, err
		}

	case settings.HA_REPLICA_TYPE:
		hahandlelog.Info("Replica info", "Replica Name", cubridDB.Name)

		replicaCubridDBName = cubridDB.Name
		cubridRef := cubridDB.Spec.Replication.HAmodeType.CubridRef

		if cubridRef == nil || cubridRef.Name == "" {
			return ctrl.Result{}, nil
		}

		if result, err = r.SetReplicaConfig(ctx, cubridRef.Name, replicaCubridDBName, cubridDB.Namespace, req); err != nil {
			return result, err
		}

	default:
		return ctrl.Result{}, fmt.Errorf("It is an unknown HA Mode type. : %s", cubridDB.HAmodeType())
	}

	return result, nil
}

func (r *HAHandler) SetMasterSlaveConfig(
	ctx context.Context,
	msCubridDBName,
	rCubridDBName,
	namespace string,
	req ctrl.Request,
) (ctrl.Result, error) {
	if err := r.settingHAMasterSlave(ctx, msCubridDBName, namespace, req); err != nil {
		return ctrl.Result{}, err
	}

	if rCubridDBName != "" {
		if err := r.updateHAReplicaList(ctx, msCubridDBName, rCubridDBName, namespace, req); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *HAHandler) SetReplicaConfig(
	ctx context.Context,
	masterName,
	replicaName,
	namespace string,
	req ctrl.Request,
) (ctrl.Result, error) {
	if err := r.settingHAReplica(ctx, replicaName, namespace, req); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.updateHAReplicaList(ctx, masterName, replicaName, namespace, req); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.updateHANodeListAndSyncMode(ctx, masterName, replicaName, namespace, req); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *HAHandler) settingHAMasterSlave(
	ctx context.Context,
	cubridDBName string,
	namespace string,
	req ctrl.Request,
) error {
	msPodLists, msServiceName, err := r.getCubridDBPodList(ctx, cubridDBName, namespace)
	if err != nil {
		return err
	}

	if len(msPodLists.Items) > 0 {
		nodeDNSList := CreateDNSList(msPodLists.Items, msServiceName, req.Namespace)
		haNodeList := fmt.Sprintf("cubrid@%s", strings.Join(nodeDNSList, ":"))
		haCopySyncMode := strings.TrimSuffix(strings.Repeat("sync:", len(nodeDNSList)), ":")

		command := updateMasterSlaveCommand(haNodeList, haCopySyncMode)
		if err := r.execCommandsInPods(ctx, msPodLists.Items, r.Config, namespace, command); err != nil {
			return err
		}
	} else {
		// PodList가 0이면 haNodeList를 빈 문자열로 설정
		hahandlelog.V(1).Info("Could not find a host to configure HA.")
	}

	return nil
}

func (r *HAHandler) updateHANodeListAndSyncMode(
	ctx context.Context,
	msName string,
	rName string,
	namespace string,
	req ctrl.Request,
) error {
	var msNodeList string = ""
	var msCopySyncMode string = ""

	if msName != "" {
		msPodLists, msServiceName, err := r.getCubridDBPodList(ctx, msName, namespace)
		if err != nil {
			hahandlelog.Error(err, "unable to fetch pods for StatefulSet")
			return err
		}

		if len(msPodLists.Items) > 0 {
			msDNSList := CreateDNSList(msPodLists.Items, msServiceName, req.Namespace)
			msNodeList = fmt.Sprintf("cubrid@%s", strings.Join(msDNSList, ":"))
			msCopySyncMode = strings.TrimSuffix(strings.Repeat("sync:", len(msDNSList)), ":")

			command := getMSNodeListCommand(msNodeList, msCopySyncMode)
			if err := r.execCommandsInPods(ctx, msPodLists.Items, r.Config, namespace, command); err != nil {
				return err
			}
		} else {
			hahandlelog.V(1).Info("No pods found")
		}
	}

	if rName != "" {
		rList, _, err := r.getCubridDBPodList(ctx, rName, namespace)
		if err != nil {
			hahandlelog.Error(err, "unable to fetch pods for StatefulSet")
			return err
		}

		command := getMSNodeListCommand(msNodeList, msCopySyncMode)
		if err := r.execCommandsInPods(ctx, rList.Items, r.Config, namespace, command); err != nil {
			return err
		}
	}
	return nil
}

func (r *HAHandler) updateHAReplicaList(
	ctx context.Context,
	msCubridDBName string,
	replicaCubridDBName string,
	namespace string,
	req ctrl.Request,
) error {
	var command []string = []string{}
	var repDNSListStr []string
	var repNodeListStr string = ""
	var rList *v1.PodList = &v1.PodList{}

	if replicaCubridDBName != "" {
		var rServiceName string

		rList, rServiceName, _ = r.getCubridDBPodList(ctx, replicaCubridDBName, namespace)

		if rList != nil && len(rList.Items) > 0 {
			repDNSListStr = CreateDNSList(rList.Items, rServiceName, req.Namespace)
			repNodeListStr = fmt.Sprintf("cubrid@%s", strings.Join(repDNSListStr, ":"))
			command = updateReplicaCommand(repNodeListStr)
			if err := r.execCommandsInPods(ctx, rList.Items, r.Config, namespace, command); err != nil {
				return err
			}
		} else {
			hahandlelog.Info("No pods found", "repNodeListStr", repNodeListStr)
		}
	}

	if msCubridDBName != "" {
		msPodLists, _, err := r.getCubridDBPodList(ctx, msCubridDBName, namespace)
		if err != nil {
			hahandlelog.Error(err, "unable to fetch pods for StatefulSet")
			return err
		}

		if rList != nil && len(rList.Items) > 0 {
			command = updateReplicaCommand(repNodeListStr)
		} else {
			command = deleteReplicaCommand()
		}

		if err := r.execCommandsInPods(ctx, msPodLists.Items, r.Config, namespace, command); err != nil {
			return err
		}
	}
	return nil
}

func (r *HAHandler) settingHAReplica(
	ctx context.Context,
	statefulSetName string,
	namespace string,
	req ctrl.Request,
) error {
	podList, serviceName, err := r.getCubridDBPodList(ctx, statefulSetName, namespace)
	if err != nil {
		hahandlelog.Error(err, "unable to fetch pods for StatefulSet")
		return err
	}

	if len(podList.Items) > 0 {
		nodeDNSList := CreateDNSList(podList.Items, serviceName, req.Namespace)
		haNodeList := fmt.Sprintf("cubrid@%s", strings.Join(nodeDNSList, ":"))

		command := updateHAReplicaCommand(haNodeList)
		if err := r.execCommandsInPods(ctx, podList.Items, r.Config, namespace, command); err != nil {
			return err
		}
	} else {
		hahandlelog.V(1).Info("Could not find a host to configure the replica.")
	}

	return nil
}

func CreateDNSList(pods []v1.Pod, serviceName, namespace string) []string {
	dnsList := make([]string, 0, len(pods))

	for _, pod := range pods {
		hostname := pod.Spec.Hostname
		if hostname == "" {
			hostname = pod.Name
		}
		dnsName := pkg.CreateDNSShortName(hostname, serviceName)
		dnsList = append(dnsList, dnsName)
	}
	return dnsList
}

func updateMasterSlaveCommand(haNodeList, haCopySyncMode string) []string {
	cubridPath := getCubridPath()

	// cubrid.conf
	haModeStr := fmt.Sprintf(
		settings.HaModeTemplate,
		cubridPath,
		cubridPath,
		cubridPath,
	)

	// cubrid_ha.conf
	haCommonStr := fmt.Sprintf(
		settings.HaCommonConfigTemplate,
		cubridPath,
		cubridPath,
		cubridPath,
		cubridPath,
		cubridPath,
	)

	haNodeListsStr := fmt.Sprintf(
		settings.HaNodeListTemplate,
		cubridPath,
		haNodeList,
		cubridPath,
		haNodeList,
		cubridPath,
	)

	haSyncModeStr := fmt.Sprintf(
		settings.HaSyncModeTemplate,
		cubridPath,
		haCopySyncMode,
		cubridPath,
		haCopySyncMode,
		cubridPath,
	)

	logMaxArchivesStr := fmt.Sprintf(
		settings.HaLogMaxArchivesTemplate,
		cubridPath,
		cubridPath,
		cubridPath,
		cubridPath,
	)

	command := haModeStr + haCommonStr + haNodeListsStr + haSyncModeStr + logMaxArchivesStr
	return []string{"sh", "-c", command}
}

func updateHAReplicaCommand(haReplicaList string) []string {
	cubridPath := getCubridPath()

	// cubrid.conf
	haModeReplicaStr := fmt.Sprintf(
		settings.HaReplicaModeTemplate,
		cubridPath,
		cubridPath,
		cubridPath,
	)

	// cubrid_ha.conf
	haCommonStr := fmt.Sprintf(
		settings.HaCommonConfigTemplate,
		cubridPath,
		cubridPath,
		cubridPath,
		cubridPath,
		cubridPath,
	)

	haReplicaListStr := fmt.Sprintf(
		settings.HaReplicaListTemplate,
		cubridPath,
		haReplicaList,
		cubridPath,
		haReplicaList,
		cubridPath,
	)

	logMaxArchivesStr := fmt.Sprintf(
		settings.HaLogMaxArchivesTemplate,
		cubridPath,
		cubridPath,
		cubridPath,
		cubridPath,
	)

	command := haModeReplicaStr + haCommonStr + haReplicaListStr + logMaxArchivesStr
	return []string{"sh", "-c", command}
}

func getMSNodeListCommand(haNodeList string, haCopySyncMode string) []string {
	cubridPath := getCubridPath()

	haNodeListStr := fmt.Sprintf(
		settings.HaNodeListTemplate,
		cubridPath, haNodeList,
		cubridPath, haNodeList,
		cubridPath,
	)

	haSyncModeStr := fmt.Sprintf(
		settings.HaSyncModeTemplate,
		cubridPath,
		haCopySyncMode,
		cubridPath,
		haCopySyncMode,
		cubridPath,
	)

	command := haNodeListStr + haSyncModeStr
	return []string{"sh", "-c", command}
}

func updateReplicaCommand(haReplicaList string) []string {
	cubridPath := getCubridPath()

	haReplicaListStr := fmt.Sprintf(
		settings.HaReplicaListTemplate,
		cubridPath,
		haReplicaList,
		cubridPath,
		haReplicaList,
		cubridPath,
	)

	command := haReplicaListStr
	return []string{"sh", "-c", command}
}

func deleteReplicaCommand() []string {
	cubridPath := getCubridPath()

	haReplicaListStr := fmt.Sprintf(settings.DelReplicaListTemplate, cubridPath)

	command := haReplicaListStr
	return []string{"sh", "-c", command}
}

func (r *HAHandler) execCommandsInPods(
	ctx context.Context,
	pods []v1.Pod,
	config *rest.Config,
	namespace string,
	command []string,
) error {
	for _, pod := range pods {
		isRunning, err := r.checkIfPodIsRunning(ctx, pod.Name, namespace)
		if err != nil {
			return err
		}

		if !isRunning {
			hahandlelog.V(1).Info("Pod is not in Running state", "Pod.Name", pod.Name)
			continue
		}

		isContainersRunning, err := allContainersRunning(&pod)
		if err != nil {
			hahandlelog.V(1).Info("container is not in Running state", "Pod.Name", pod.Name)
			continue
		}

		if !isContainersRunning {
			hahandlelog.V(1).Info("Not all containers are in Running state, requeueing", "Pod.Name", pod.Name)
			continue
		}

		for _, container := range pod.Spec.Containers {
			if err := execCommand(config, namespace, pod.Name, container.Name, command); err != nil {
				return err
			}
		}
	}
	return nil
}

func execCommand(config *rest.Config, namespace, podName string, containerNames string, command []string) error {
	var stdout, stderr bytes.Buffer

	podExecOptions := &corev1.PodExecOptions{
		Command:   command,
		Container: containerNames,
		Stdout:    true,
		Stderr:    true,
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(podExecOptions, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to init SPDY executor: %w", err)
	}

	err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	if err != nil {
		return fmt.Errorf("failed to execute command: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

func (r *HAHandler) checkIfPodIsRunning(ctx context.Context, podName string, namespace string) (bool, error) {
	var pod corev1.Pod
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: namespace}, &pod)
	if err != nil {
		return false, fmt.Errorf("failed to get Pod %s/%s: %v", namespace, podName, err)
	}

	return pod.Status.Phase == corev1.PodRunning, nil
}

func allContainersRunning(pod *corev1.Pod) (bool, error) {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running == nil {
			if containerStatus.State.Waiting != nil {
				return false, fmt.Errorf(
					"container %s is waiting: %v",
					containerStatus.Name,
					containerStatus.State.Waiting.Reason)
			} else if containerStatus.State.Terminated != nil {
				return false, fmt.Errorf(
					"container %s is terminated: %v",
					containerStatus.Name,
					containerStatus.State.Terminated.Reason,
				)
			}
			return false, nil
		}
	}
	return true, nil
}

func (r *HAHandler) getCubridDBPodList(
	ctx context.Context,
	name string,
	namespace string,
) (*v1.PodList, string, error) {
	var statefulSet appsv1.StatefulSet
	var serviceName string

	if err := r.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &statefulSet); err != nil {
		return nil, "", fmt.Errorf("unable to fetch StatefulSet: %w", err)
	}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(statefulSet.Spec.Selector.MatchLabels),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		return nil, "", fmt.Errorf("unable to list pods: %w", err)
	}

	serviceName = statefulSet.Spec.ServiceName

	return podList, serviceName, nil
}

func getCubridPath() string {
	cubridUserPath := os.Getenv("CUBRID")
	if cubridUserPath == "" {
		cubridUserPath = settings.DefaultCUBRIDPath
	}

	return cubridUserPath
}
