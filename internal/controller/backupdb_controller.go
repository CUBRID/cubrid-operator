/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	"github.com/cubrid/cubrid-operator/pkg"
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
)

// BackupDBReconciler reconciles a BackupDB object
type BackupDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=backupdbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=backupdbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=backupdbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BackupDB object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.2/pkg/reconcile
func (r *BackupDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("======= BackupDBReconciler: Start ========")

	var backupDB cubridv1.BackupDB
	if err := r.Get(ctx, req.NamespacedName, &backupDB); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !backupDB.ObjectMeta.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(&backupDB, backupDB.Name)
		if err := r.Update(ctx, &backupDB); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.addFinalizer(ctx, &backupDB); err != nil {
		return ctrl.Result{}, err
	}

	if backupDB.Status.CommandStatus == DEF.BackupDB_PENDING ||
		backupDB.Status.CommandStatus == DEF.BackupDB_FAILED ||
		isCommandChanged(&backupDB) {

		backupDB.Status.Command = getCommandArgs(&backupDB)
		backupDB.Status.FilePath = getCommandFile(&backupDB)
		backupDB.Status.CommandStatus = DEF.BackupDB_INPROGRESS
		backupDB.Status.Message = "Command is being sent"
		if err := r.Status().Update(ctx, &backupDB); err != nil {
			return ctrl.Result{}, err
		}

		err := r.sendCommand(ctx, &backupDB)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cubridv1.BackupDB{}).
		Complete(r)
}

func (r *BackupDBReconciler) sendCommand(ctx context.Context, backupDB *cubridv1.BackupDB) error {
	podName := backupDB.Spec.CubridDBRef.Name
	namespace := backupDB.Spec.CubridDBRef.Namespace
	storageType := backupDB.Spec.StorageRef.StorageType

	var pod corev1.Pod
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: podName}, &pod); err != nil {
		r.updateCommandStatus(ctx, backupDB, DEF.BackupDB_FAILED, "Pod not found or inaccessible")
		return fmt.Errorf("failed to get pod %s: %v", podName, err)
	}

	_, err := r.getMountPathFromCubriddb(ctx, backupDB, podName, namespace, storageType)
	if err != nil {
		r.updateCommandStatus(ctx, backupDB, DEF.BackupDB_FAILED, "Could not find the path to store backupdb")
		return err
	}

	if pod.Status.Phase != corev1.PodRunning {
		r.updateCommandStatus(ctx, backupDB, DEF.BackupDB_FAILED, string(pod.Status.Phase))
		return fmt.Errorf("pod(%s) is not Running state", podName)
	}

	isContainersRunning, err := pkg.AllContainersRunning(&pod)
	if err != nil {
		r.updateCommandStatus(ctx, backupDB, DEF.BackupDB_FAILED, err.Error())
		return err
	}

	if !isContainersRunning {
		return fmt.Errorf("not all containers are in Running state. %s: %v", podName, err)
	}

	if err := r.execCommandInPod(r.Config, namespace, podName, pod.Spec.Containers[0].Name, getBackupDBCommand(backupDB)); err != nil {
		r.updateCommandStatus(ctx, backupDB, DEF.BackupDB_FAILED, err.Error())
		return fmt.Errorf("command failed: %v", err)
	}

	r.updateCommandStatus(ctx, backupDB, DEF.BackupDB_COMPLETED, "Command sent successfully")
	return nil
}

func (r *BackupDBReconciler) getMountPathFromCubriddb(
	ctx context.Context,
	backupdb *cubridv1.BackupDB,
	podName string,
	namespace string,
	storageType string,
) (string, error) {
	var podBaseName string

	lastDashIndex := strings.LastIndex(podName, "-")

	if lastDashIndex != -1 {
		podBaseName = podName[:lastDashIndex]
	} else {
		fmt.Println("No dash found in the string")
	}

	var cubridDB cubridv1.CubridDB
	err := r.Get(ctx, client.ObjectKey{Name: podBaseName, Namespace: namespace}, &cubridDB)
	if err != nil {
		return "", fmt.Errorf("not found cubridDB object, err : %v", err)
	}

	for _, storage := range cubridDB.Spec.Storage {
		if storage.Type == backupdb.StorageRef().StorageType {
			return storage.MountPath, nil
		}
	}

	return "", fmt.Errorf("no storage of type %s found", storageType)
}

func (r *BackupDBReconciler) execCommandInPod(config *rest.Config, namespace, podName string, containerNames string, command []string) error {
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

func (r *BackupDBReconciler) addFinalizer(ctx context.Context, backupDB *cubridv1.BackupDB) error {
	if !controllerutil.ContainsFinalizer(backupDB, backupDB.Name) {
		controllerutil.AddFinalizer(backupDB, backupDB.Name)
		return r.Update(ctx, backupDB)
	}
	return nil
}

func (r *BackupDBReconciler) updateCommandStatus(ctx context.Context, backupdb *cubridv1.BackupDB, status, message string) {
	log := log.FromContext(ctx)

	updatedBackupDB := &cubridv1.BackupDB{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      backupdb.Name,
		Namespace: backupdb.Namespace,
	}, updatedBackupDB); err != nil {
		log.Error(err, "Failed to fetch latest BackupDB object")
		return
	}

	updatedBackupDB.Status.Command = getCommandArgs(backupdb)
	updatedBackupDB.Status.FilePath = getCommandFile(backupdb)
	updatedBackupDB.Status.CommandStatus = status
	updatedBackupDB.Status.Message = message

	if err := r.Status().Update(ctx, updatedBackupDB); err != nil {
		if errors.IsConflict(err) {
			return
		}
		log.Error(err, "Failed to update CommandStatus", "Status", status, "Message", message)
		return
	}
}

func getBackupDBCommand(backupDB *cubridv1.BackupDB) []string {
	filePath := getCommandFile(backupDB)
	args := getCommandArgs(backupDB)

	commandStr := fmt.Sprintf("$CUBRID/%s %s", filePath, args)

	return []string{"sh", "-c", commandStr}
}

func getCommandArgs(backupDB *cubridv1.BackupDB) string {
	if backupDB.Spec.CommandArgs == nil {
		backupDB.Spec.CommandArgs = backupDB.CommandArgs()
	}

	args := backupDB.Spec.CommandArgs.Args
	if args == nil {
		args = []string{}
	}

	argsStr := strings.Join(args, " ")

	return argsStr
}

func getCommandFile(backupDB *cubridv1.BackupDB) string {
	if backupDB.Spec.CommandArgs == nil {
		backupDB.Spec.CommandArgs = backupDB.CommandArgs()
	}

	filePath := backupDB.Spec.CommandArgs.FilePath
	if filePath == "" {
		filePath = DEF.BackupDB_Script_File_Path
	}

	return filePath
}

func isCommandChanged(backupDB *cubridv1.BackupDB) bool {
	status := backupDB.BackupDBStatus()
	command := backupDB.CommandArgs()

	fileChanged := status.FilePath != command.FilePath
	argsChanged := status.Command != strings.Join(command.Args, " ")

	fmt.Printf("status.File : %s, command.FilePath : %s\n", status.FilePath, command.FilePath)
	fmt.Printf("status.Command : %s, command.Args : %s\n", status.Command, strings.Join(command.Args, " "))

	return fileChanged || argsChanged
}
