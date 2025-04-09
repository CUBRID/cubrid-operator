package manager

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	"github.com/cubrid/cubrid-operator/pkg"
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

type HAManager struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

var halog = log.Log.WithName("HAMode")

func NewHAManager(client client.Client, scheme *runtime.Scheme, config *rest.Config) *HAManager {
	return &HAManager{
		Client: client,
		Scheme: scheme,
		Config: config,
	}
}

func (r *HAManager) ReconcileHAMode(
	ctx context.Context,
	cubridDB *cubridv1.CubridDB,
	req ctrl.Request,
) (ctrl.Result, error) {
	var result ctrl.Result
	var err error
	var msCubridDBName, replicaCubridDBName string

	switch cubridDB.HAmodeType() {
	case DEF.HA_MASTER_SLAVE_TYPE:
		msCubridDBName = cubridDB.Name

		// Ensure that CubridRef is initialized
		replicaRef := pkg.InitCubridRef(cubridDB)
		replicaCubridDBName = replicaRef.ReplicaLink

		halog.Info("Master-Slave info", "Master-Slave", msCubridDBName, "Replica", replicaCubridDBName)

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

	case DEF.HA_REPLICA_TYPE:
		halog.Info("Replica info", "Replica Name", cubridDB.Name)

		replicaCubridDBName = cubridDB.Name
		cubridRef := cubridDB.Spec.Replication.HAmodeType.CubridRef

		if cubridRef == nil || cubridRef.Name == "" {
			return ctrl.Result{}, nil
		}

		if result, err = r.SetReplicaConfig(ctx, cubridRef.Name, replicaCubridDBName, cubridDB.Namespace, req); err != nil {
			return result, err
		}

	default:
		return ctrl.Result{}, fmt.Errorf("it is an unknown HA Mode type. : %s", cubridDB.HAmodeType())
	}

	return result, nil
}

func (r *HAManager) SetMasterSlaveConfig(
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

func (r *HAManager) SetReplicaConfig(
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

func (r *HAManager) settingHAMasterSlave(
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

		commands := buildHAmodeCmds(haNodeList, haCopySyncMode)

		for _, command := range commands {
			if err := r.execCommandsInPods(ctx, msPodLists.Items, r.Config, namespace, command); err != nil {
				return err
			}
		}
	} else {
		halog.V(1).Info("Could not find a host to configure HA.")
	}

	return nil
}

func (r *HAManager) updateHANodeListAndSyncMode(
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
			return err
		}

		if len(msPodLists.Items) > 0 {
			msDNSList := CreateDNSList(msPodLists.Items, msServiceName, req.Namespace)
			msNodeList = fmt.Sprintf("cubrid@%s", strings.Join(msDNSList, ":"))
			msCopySyncMode = strings.TrimSuffix(strings.Repeat("sync:", len(msDNSList)), ":")

			commands := buildNodeListCmds(msNodeList, msCopySyncMode)

			for _, command := range commands {
				if err := r.execCommandsInPods(ctx, msPodLists.Items, r.Config, namespace, command); err != nil {
					return err
				}
			}
		} else {
			halog.V(1).Info("No pods found")
		}
	}

	if rName != "" {
		rList, _, err := r.getCubridDBPodList(ctx, rName, namespace)
		if err != nil {
			return err
		}

		commands := buildNodeListCmds(msNodeList, msCopySyncMode)

		for _, command := range commands {
			if err := r.execCommandsInPods(ctx, rList.Items, r.Config, namespace, command); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *HAManager) updateHAReplicaList(
	ctx context.Context,
	msCubridDBName string,
	replicaCubridDBName string,
	namespace string,
	req ctrl.Request,
) error {
	var commands map[string][]string
	var repDNSListStr []string
	var repNodeListStr string = ""
	var rList *corev1.PodList = &corev1.PodList{}

	if replicaCubridDBName != "" {
		var rServiceName string

		rList, rServiceName, _ = r.getCubridDBPodList(ctx, replicaCubridDBName, namespace)

		if rList != nil && len(rList.Items) > 0 {
			repDNSListStr = CreateDNSList(rList.Items, rServiceName, req.Namespace)
			repNodeListStr = fmt.Sprintf("cubrid@%s", strings.Join(repDNSListStr, ":"))
			commands = buildReplicaListCmds(repNodeListStr)

			for _, command := range commands {
				if err := r.execCommandsInPods(ctx, rList.Items, r.Config, namespace, command); err != nil {
					return err
				}
			}
		} else {
			halog.Info("No pods found", "repNodeListStr", repNodeListStr)
		}
	}

	if msCubridDBName != "" {
		var commands = make(map[string][]string)
		msPodLists, _, err := r.getCubridDBPodList(ctx, msCubridDBName, namespace)
		if err != nil {
			return err
		}

		if rList != nil && len(rList.Items) > 0 {
			commands = buildReplicaListCmds(repNodeListStr)
		} else {
			commands = buildReplicaDelCmds()
		}

		for _, command := range commands {
			if err := r.execCommandsInPods(ctx, msPodLists.Items, r.Config, namespace, command); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *HAManager) settingHAReplica(
	ctx context.Context,
	statefulSetName string,
	namespace string,
	req ctrl.Request,
) error {
	podList, serviceName, err := r.getCubridDBPodList(ctx, statefulSetName, namespace)
	if err != nil {
		return err
	}

	if len(podList.Items) > 0 {
		nodeDNSList := CreateDNSList(podList.Items, serviceName, req.Namespace)
		haNodeList := fmt.Sprintf("cubrid@%s", strings.Join(nodeDNSList, ":"))

		commands := buildReplicaCmds(haNodeList)

		for _, command := range commands {
			if err := r.execCommandsInPods(ctx, podList.Items, r.Config, namespace, command); err != nil {
				return err
			}
		}
	} else {
		halog.V(1).Info("Could not find a host to configure the replica.")
	}

	return nil
}

func CreateDNSList(pods []corev1.Pod, serviceName, namespace string) []string {
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

func buildHAmodeCmds(haNodeList, haCopySyncMode string) map[string][]string {
	cubridPath := getCubridPath()

	fullpath := cubridPath + "/" + DEF.HATemplateFilePath + " "

	commands := map[string][]string{
		"ha_mode":                      {"sh", "-c", fullpath + "ha_mode"},
		"ha_common_config":             {"sh", "-c", fullpath + "ha_common_config"},
		"ha_node_list":                 {"sh", "-c", fullpath + "ha_node_list" + " " + haNodeList},
		"ha_force_remove_log_archives": {"sh", "-c", fullpath + "ha_force_remove_log_archives"},
		"ha_copy_sync_mode":            {"sh", "-c", fullpath + "ha_copy_sync_mode" + " " + haCopySyncMode},
		"ha_log_max_archives":          {"sh", "-c", fullpath + "ha_log_max_archives"},
	}

	return commands
}

func buildReplicaCmds(haReplicaList string) map[string][]string {
	cubridPath := getCubridPath()

	fullpath := cubridPath + "/" + DEF.HATemplateFilePath + " "

	commands := map[string][]string{
		"ha_replica_mode":     {"sh", "-c", fullpath + "ha_replica_mode"},
		"ha_common_config":    {"sh", "-c", fullpath + "ha_common_config"},
		"ha_replica_list":     {"sh", "-c", fullpath + "ha_replica_list" + " " + haReplicaList},
		"ha_log_max_archives": {"sh", "-c", fullpath + "ha_log_max_archives"},
	}

	return commands
}

func buildNodeListCmds(haNodeList string, haCopySyncMode string) map[string][]string {
	cubridPath := getCubridPath()

	fullpath := cubridPath + "/" + DEF.HATemplateFilePath + " "

	commands := map[string][]string{
		"ha_node_list":      {"sh", "-c", fullpath + "ha_node_list" + " " + haNodeList},
		"ha_copy_sync_mode": {"sh", "-c", fullpath + "ha_copy_sync_mode" + " " + haCopySyncMode},
	}

	return commands
}

func buildReplicaListCmds(haReplicaList string) map[string][]string {
	cubridPath := getCubridPath()

	fullpath := cubridPath + "/" + DEF.HATemplateFilePath + " "

	commands := map[string][]string{
		"ha_replica_list": {"sh", "-c", fullpath + "ha_replica_list" + " " + haReplicaList},
	}

	return commands
}

func buildReplicaDelCmds() map[string][]string {
	cubridPath := getCubridPath()

	fullpath := cubridPath + "/" + DEF.HATemplateFilePath + " "
	commands := map[string][]string{
		"ha_del_replica_list": {"sh", "-c", fullpath + "ha_del_replica_list"},
	}

	return commands
}

func (r *HAManager) execCommandsInPods(
	ctx context.Context,
	pods []corev1.Pod,
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
			halog.V(1).Info("Pod is not in Running state", "Pod.Name", pod.Name)
			continue
		}

		isContainersRunning, err := allContainersRunning(&pod)
		if err != nil {
			halog.V(1).Info("container is not in Running state", "Pod.Name", pod.Name)
			continue
		}

		if !isContainersRunning {
			halog.V(1).Info("Not all containers are in Running state, requeueing", "Pod.Name", pod.Name)
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

func (r *HAManager) checkIfPodIsRunning(ctx context.Context, podName string, namespace string) (bool, error) {
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

func (r *HAManager) getCubridDBPodList(
	ctx context.Context,
	name string,
	namespace string,
) (*corev1.PodList, string, error) {
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
		cubridUserPath = DEF.DefaultCUBRIDPath
	}

	return cubridUserPath
}
