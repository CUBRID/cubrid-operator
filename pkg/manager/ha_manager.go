/*
 * Copyright 2016 CUBRID Corporation
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package manager

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	"github.com/cubrid/cubrid-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GroupHAConfig represents the unified HA configuration for a group
type GroupHAConfig struct {
	// Common HA configuration for all pods in the group
	HaNodeList     string
	HaCopySyncMode string
	HaReplicaList  string

	// Pod-specific configurations
	PodConfigs map[string]PodHAConfig
}

// PodHAConfig represents HA configuration specific to a pod
type PodHAConfig struct {
	HAMode string // "on" for master-slave, "replica" for replica
}

// GroupHAManager handles HA configuration for groups of pods
type GroupHAManager struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

var groupHalog = log.Log.WithName("GroupHAMode")

func NewGroupHAManager(client client.Client, scheme *runtime.Scheme, config *rest.Config) *GroupHAManager {
	return &GroupHAManager{
		Client: client,
		Scheme: scheme,
		Config: config,
	}
}

// ReconcileGroupHAMode is the main entry point for group-based HA configuration
func (g *GroupHAManager) ReconcileGroupHAMode(
	ctx context.Context,
	cubridDB *cubridv1.CubridDB,
	req ctrl.Request,
) (ctrl.Result, error) {
	if !cubridDB.IsHAEnabled() {
		return ctrl.Result{}, nil
	}

	groupHalog.Info("Starting group HA reconciliation", "cubridDB", cubridDB.Name, "haMode", cubridDB.HAmodeType())

	// Get group name based on HA mode type
	groupName := g.getGroupName(cubridDB)

	// Get all pods in the group
	allPodsInGroup, err := g.getAllPodsInGroup(ctx, cubridDB.Namespace, groupName)
	for _, pod := range allPodsInGroup {
		groupHalog.Info("Pod in group", "podName", pod.Name, "namespace", pod.Namespace, "labels", pod.Labels)
	}

	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get pods in group %s: %v", groupName, err)
	}

	if len(allPodsInGroup) == 0 {
		groupHalog.V(1).Info("No pods found in group", "group", groupName)
		return ctrl.Result{}, nil
	}

	// Build unified HA configuration for the entire group
	haConfig := g.buildGroupHAConfig(ctx, allPodsInGroup)

	// Apply HA configuration to all pods in the group
	g.applyGroupHAConfig(allPodsInGroup, haConfig)

	groupHalog.Info("Group HA reconciliation completed", "group", groupName, "pods", len(allPodsInGroup))
	return ctrl.Result{}, nil
}

// getGroupName returns the group name based on HA mode type
func (g *GroupHAManager) getGroupName(cubridDB *cubridv1.CubridDB) string {
	var groupName string

	if cubridDB.HAmodeType() == DEF.HA_REPLICA_TYPE {
		// For replica type, use the master's name as group name
		if cubridDB.Spec.Replication.HAmodeType.CubridRef != nil {
			groupName = cubridDB.Spec.Replication.HAmodeType.CubridRef.Name + DEF.SELECTOR_SUFFIX
		}
	} else {
		groupName = cubridDB.Name + DEF.SELECTOR_SUFFIX
	}

	groupHalog.Info(
		"Generated group name",
		"cubridDB", cubridDB.Name,
		"haMode", cubridDB.HAmodeType(),
		"groupName", groupName,
	)
	return groupName
}

// getAllPodsInGroup gets all pods that belong to the same HA group
func (g *GroupHAManager) getAllPodsInGroup(ctx context.Context, namespace, groupName string) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}

	listOptions := &client.ListOptions{
		Namespace: namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"group": groupName,
		}),
	}

	groupHalog.Info("Searching for pods in group", "namespace", namespace, "groupName", groupName)

	if err := g.List(ctx, podList, listOptions); err != nil {
		return nil, err
	}

	groupHalog.Info("Found pods in group", "groupName", groupName, "podCount", len(podList.Items))
	for _, pod := range podList.Items {
		groupHalog.Info("Pod in group", "podName", pod.Name, "namespace", pod.Namespace, "labels", pod.Labels)
	}

	return podList.Items, nil
}

// buildGroupHAConfig builds unified HA configuration for the entire group
func (g *GroupHAManager) buildGroupHAConfig(ctx context.Context, allPods []corev1.Pod) *GroupHAConfig {
	config := &GroupHAConfig{
		PodConfigs: make(map[string]PodHAConfig),
	}

	// Separate pods by type
	masterSlavePods := []corev1.Pod{}
	replicaPods := []corev1.Pod{}

	groupHalog.Info("Processing pods for HA config", "totalPods", len(allPods))

	for _, pod := range allPods {
		groupType, exists := pod.Labels["grouptype"]
		if !exists {
			groupHalog.V(1).Info("Pod missing grouptype label", "pod", pod.Name)
			continue
		}

		groupHalog.Info(
			"Pod grouptype",
			"pod", pod.Name,
			"grouptype", groupType,
			"expectedMasterSlave", DEF.HA_MASTER_SLAVE_TYPE,
			"expectedReplica", DEF.HA_REPLICA_TYPE,
		)

		if groupType == DEF.HA_MASTER_SLAVE_TYPE {
			groupHalog.Info("Adding pod to master-slave list", "pod", pod.Name)
			masterSlavePods = append(masterSlavePods, pod)
			config.PodConfigs[pod.Name] = PodHAConfig{HAMode: "on"}
		} else if groupType == DEF.HA_REPLICA_TYPE {
			groupHalog.Info("Adding pod to replica list", "pod", pod.Name)
			replicaPods = append(replicaPods, pod)
			config.PodConfigs[pod.Name] = PodHAConfig{HAMode: "replica"}
		} else {
			groupHalog.V(1).Info("Pod has unknown grouptype", "pod", pod.Name, "grouptype", groupType)
		}
	}

	// Build ha_node_list from master-slave pods
	if len(masterSlavePods) > 0 {
		groupHalog.Info("Building ha_node_list from master-slave pods", "count", len(masterSlavePods))
		msDNSList := g.createDNSList(ctx, masterSlavePods)
		config.HaNodeList = fmt.Sprintf("cubrid@%s", strings.Join(msDNSList, ":"))
		config.HaCopySyncMode = strings.TrimSuffix(strings.Repeat("sync:", len(msDNSList)), ":")
		groupHalog.Info(
			"Built ha_node_list",
			"haNodeList", config.HaNodeList,
			"haCopySyncMode", config.HaCopySyncMode,
		)
	} else {
		groupHalog.Info("No master-slave pods found for ha_node_list")
	}

	// Build ha_replica_list from replica pods
	if len(replicaPods) > 0 {
		groupHalog.Info("Building ha_replica_list from replica pods", "count", len(replicaPods))
		replicaDNSList := g.createDNSList(ctx, replicaPods)
		config.HaReplicaList = fmt.Sprintf("cubrid@%s", strings.Join(replicaDNSList, ":"))
		groupHalog.Info("Built ha_replica_list", "haReplicaList", config.HaReplicaList)
	} else {
		groupHalog.Info("No replica pods found for ha_replica_list")
	}

	groupHalog.V(1).Info("Built group HA config",
		"masterSlavePods", len(masterSlavePods),
		"replicaPods", len(replicaPods),
		"haNodeList", config.HaNodeList,
		"haReplicaList", config.HaReplicaList)

	return config
}

// createDNSList creates DNS names for pods
func (g *GroupHAManager) createDNSList(ctx context.Context, pods []corev1.Pod) []string {
	dnsList := make([]string, 0, len(pods))

	for _, pod := range pods {
		hostname := pod.Spec.Hostname
		if hostname == "" {
			hostname = pod.Name
		}

		// Extract StatefulSet name from pod name
		// Pod names are like: ha-ms-0, ha-ms-1, ha-rep-0
		// StatefulSet names are like: ha-ms, ha-rep
		statefulSetName := g.extractStatefulSetNameFromPod(pod.Name)
		if statefulSetName == "" {
			groupHalog.V(1).Info("Failed to extract StatefulSet name from pod", "pod", pod.Name)
			continue
		}

		groupHalog.Info("Extracted StatefulSet name", "pod", pod.Name, "statefulSet", statefulSetName)

		// Get StatefulSet to get service name
		statefulSet, err := util.FetchStatefulSet(ctx, g.Client, statefulSetName, pod.Namespace)
		if err != nil {
			groupHalog.Error(err, "Failed to get StatefulSet", "pod", pod.Name, "statefulSet", statefulSetName)
			continue
		}
		serviceName := statefulSet.Spec.ServiceName
		dnsName := util.CreateDNSShortName(hostname, serviceName)
		dnsList = append(dnsList, dnsName)
	}

	return dnsList
}

// extractStatefulSetNameFromPod extracts StatefulSet name from pod name
// Pod names: ha-ms-0, ha-ms-1, ha-rep-0 -> StatefulSet names: ha-ms, ha-rep
func (g *GroupHAManager) extractStatefulSetNameFromPod(podName string) string {
	// Use regex to match the pattern: name-number
	// This will match the last occurrence of -number at the end
	re := regexp.MustCompile(`^(.*)-\d+$`)
	matches := re.FindStringSubmatch(podName)
	if len(matches) >= 2 {
		return matches[1]
	}

	// Fallback: if no pattern matches, return empty string
	return ""
}

// applyGroupHAConfig applies HA configuration to all pods in the group
func (g *GroupHAManager) applyGroupHAConfig(allPods []corev1.Pod, config *GroupHAConfig) {
	for _, pod := range allPods {
		podConfig, exists := config.PodConfigs[pod.Name]
		if !exists {
			groupHalog.V(1).Info("No config found for pod", "pod", pod.Name)
			continue
		}

		// Apply cubrid_ha.conf (same for all pods in group)
		if err := g.applyCubridHAConf(pod, config); err != nil {
			groupHalog.Error(err, "Failed to apply cubrid_ha.conf", "pod", pod.Name)
			continue
		}

		// Apply cubrid.conf (pod-specific)
		if err := g.applyCubridConf(pod, podConfig, config); err != nil {
			groupHalog.Error(err, "Failed to apply cubrid.conf", "pod", pod.Name)
			continue
		}
	}
}

// applyCubridHAConf applies cubrid_ha.conf configuration to a pod
func (g *GroupHAManager) applyCubridHAConf(pod corev1.Pod, config *GroupHAConfig) error {
	commands := g.buildCubridHAConfCommands(config)

	groupHalog.Info("Applying cubrid_ha.conf to pod", "pod", pod.Name, "commandCount", len(commands))

	for commandName, command := range commands {
		groupHalog.Info("Executing command", "pod", pod.Name, "command", commandName, "args", command)
		if err := g.execCommandInPod(pod, command); err != nil {
			return fmt.Errorf("failed to execute command %v in pod %s: %v", command, pod.Name, err)
		}
	}

	return nil
}

// applyCubridConf applies cubrid.conf configuration to a pod
func (g *GroupHAManager) applyCubridConf(pod corev1.Pod, podConfig PodHAConfig, groupConfig *GroupHAConfig) error {
	commands := g.buildCubridConfCommands(podConfig, groupConfig)

	groupHalog.Info("Applying cubrid.conf to pod", "pod", pod.Name, "commandCount", len(commands))

	for commandName, command := range commands {
		groupHalog.Info("Executing command", "pod", pod.Name, "command", commandName, "args", command)
		if err := g.execCommandInPod(pod, command); err != nil {
			return fmt.Errorf("failed to execute command %v in pod %s: %v", command, pod.Name, err)
		}
	}

	return nil
}

// buildCubridHAConfCommands builds commands for cubrid_ha.conf configuration
func (g *GroupHAManager) buildCubridHAConfCommands(config *GroupHAConfig) map[string][]string {
	cubridPath := g.getCubridPath()
	fullpath := cubridPath + "/" + DEF.HATemplateFilePath + " "

	groupHalog.Info("Building cubrid_ha.conf commands",
		"haNodeList", config.HaNodeList,
		"haCopySyncMode", config.HaCopySyncMode,
		"haReplicaList", config.HaReplicaList)

	commands := map[string][]string{
		"ha_common_config":    {"sh", "-c", fullpath + "ha_common_config"},
		"ha_log_max_archives": {"sh", "-c", fullpath + "ha_log_max_archives"},
	}

	// Add ha_node_list if master-slave pods exist
	if config.HaNodeList != "" {
		groupHalog.Info("Adding ha_node_list command", "haNodeList", config.HaNodeList)
		commands["ha_node_list"] = []string{"sh", "-c", fullpath + "ha_node_list" + " " + config.HaNodeList}
		commands["ha_copy_sync_mode"] = []string{"sh", "-c", fullpath + "ha_copy_sync_mode" + " " + config.HaCopySyncMode}
	} else {
		groupHalog.Info("ha_node_list is empty, skipping")
	}

	// Add ha_replica_list if replica pods exist
	if config.HaReplicaList != "" {
		groupHalog.Info("Adding ha_replica_list command", "haReplicaList", config.HaReplicaList)
		commands["ha_replica_list"] = []string{"sh", "-c", fullpath + "ha_replica_list" + " " + config.HaReplicaList}
	} else {
		groupHalog.Info("ha_replica_list is empty, skipping")
	}

	groupHalog.Info("Final commands for cubrid_ha.conf", "commands", commands)
	return commands
}

// buildCubridConfCommands builds commands for cubrid.conf configuration
func (g *GroupHAManager) buildCubridConfCommands(
	podConfig PodHAConfig,
	groupConfig *GroupHAConfig,
) map[string][]string {
	// Use local implementations copied from ha_manager.go with correct values
	if podConfig.HAMode == "on" {
		// For master-slave mode, use buildHAmodeCmds with correct node list
		groupHalog.Info(
			"Building master-slave commands for cubrid.conf",
			"haNodeList", groupConfig.HaNodeList,
			"haCopySyncMode", groupConfig.HaCopySyncMode,
		)
		return g.buildHAmodeCmds(groupConfig.HaNodeList, groupConfig.HaCopySyncMode)
	} else if podConfig.HAMode == "replica" {
		// For replica mode, use buildReplicaCmds with correct replica list
		groupHalog.Info(
			"Building replica commands for cubrid.conf",
			"haReplicaList", groupConfig.HaReplicaList,
		)
		return g.buildReplicaCmds(groupConfig.HaReplicaList)
	}

	// Default case: only common config
	cubridPath := g.getCubridPath()
	fullpath := cubridPath + "/" + DEF.HATemplateFilePath + " "

	return map[string][]string{
		"ha_log_max_archives": {"sh", "-c", fullpath + "ha_log_max_archives"},
	}
}

// buildHAmodeCmds builds commands for master-slave HA mode (copied from ha_manager.go)
func (g *GroupHAManager) buildHAmodeCmds(haNodeList, haCopySyncMode string) map[string][]string {
	cubridPath := g.getCubridPath()
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

// buildReplicaCmds builds commands for replica HA mode (copied from ha_manager.go)
func (g *GroupHAManager) buildReplicaCmds(haReplicaList string) map[string][]string {
	cubridPath := g.getCubridPath()
	fullpath := cubridPath + "/" + DEF.HATemplateFilePath + " "

	commands := map[string][]string{
		"ha_replica_mode":     {"sh", "-c", fullpath + "ha_replica_mode"},
		"ha_common_config":    {"sh", "-c", fullpath + "ha_common_config"},
		"ha_replica_list":     {"sh", "-c", fullpath + "ha_replica_list" + " " + haReplicaList},
		"ha_log_max_archives": {"sh", "-c", fullpath + "ha_log_max_archives"},
	}

	return commands
}

// execCommandInPod executes a command in a specific pod
func (g *GroupHAManager) execCommandInPod(pod corev1.Pod, command []string) error {
	// Check if pod is running
	if !g.isPodRunning(&pod) {
		groupHalog.V(1).Info("Pod is not running, skipping command execution", "pod", pod.Name)
		return nil
	}

	// Execute command in all containers of the pod
	for _, container := range pod.Spec.Containers {
		if err := g.execCommand(g.Config, pod.Namespace, pod.Name, container.Name, command); err != nil {
			return fmt.Errorf("failed to execute command %v in pod %s container %s: %v", command, pod.Name, container.Name, err)
		}
	}

	return nil
}

// execCommand executes a command in a specific container (copied from ha_manager.go)
func (g *GroupHAManager) execCommand(
	config *rest.Config,
	namespace,
	podName string,
	containerNames string,
	command []string,
) error {
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
		VersionedParams(podExecOptions, runtime.NewParameterCodec(g.Scheme))

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

// isPodRunning checks if a pod is in running state
func (g *GroupHAManager) isPodRunning(pod *corev1.Pod) bool {
	_, err := util.IsPodAndContainersRunning(pod)
	return err == nil
}

// getCubridPath returns the CUBRID installation path
func (g *GroupHAManager) getCubridPath() string {
	cubridUserPath := os.Getenv("CUBRID")
	if cubridUserPath == "" {
		cubridUserPath = DEF.DefaultCUBRIDPath
	}
	return cubridUserPath
}
