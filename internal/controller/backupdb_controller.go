/*
 * Copyright 2025 CUBRID Corporation
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

package controller

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/manager"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	"github.com/cubrid/cubrid-operator/pkg/util"
	"github.com/robfig/cron/v3"
	"k8s.io/client-go/util/exec"
)

// BackupDBReconciler reconciles a BackupDB object
type BackupDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config

	taskScheduler       *cron.Cron
	backupSchedule      map[string]ScheduledTask
	backupCommandCancel map[string]context.CancelFunc
	backupRetryCancel   map[string]context.CancelFunc
	Mutex               sync.Mutex
}

type ScheduledTask struct {
	EntryID  cron.EntryID
	Schedule string
}

type ScheduleInfo struct {
	Name      string
	Namespace string
}

var (
	backupdblog   = log.Log.WithName("BackupDB-Reconciler")
	cronExprRegex = regexp.MustCompile(`^(\*|(\d+|(\*/\d+))) (\*|(\d+|(\*/\d+))) (\*|(\d+|(\*/\d+))) (\*|(\d+|(\*/\d+))) (\*|(\d+|(\*/\d+)))$`)
)

func init() {
}

//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=backupdbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=backupdbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=backupdbs/finalizers,verbs=update
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=cubriddbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=cubriddbs/status,verbs=get;update;patch;list;watch;create;delete
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=cubriddbs/finalizers,verbs=update;get;list;watch;create;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get
//+kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
//+kubebuilder:rbac:groups="",resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services/status,verbs=get
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=endpoints/restricted,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=endpoints/status,verbs=get
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete;bind
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingressclasses,verbs=get;list;watch;create;update;patch;delete

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
	var backupDB cubridv1.BackupDB

	// Fetch BackupDB resource
	if err := r.Get(ctx, req.NamespacedName, &backupDB); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle resource deletion
	if !backupDB.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleResourceDeletion(ctx, req.NamespacedName.String(), &backupDB)
	}

	// Add finalizer if not already added
	if err := r.addFinalizer(ctx, &backupDB); err != nil {
		return ctrl.Result{}, err
	}

	// Update backup schedule
	if err := r.updateBackupSchedule(ctx, req, &backupDB); err != nil {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cubridv1.BackupDB{}).
		Complete(r)
}

func NewBackupDBReconciler(mgr manager.Manager) *BackupDBReconciler {
	scheduler := cron.New(cron.WithChain())
	scheduler.Start()

	return &BackupDBReconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		Config:              mgr.GetConfig(),
		taskScheduler:       scheduler,
		backupSchedule:      make(map[string]ScheduledTask),
		backupCommandCancel: make(map[string]context.CancelFunc),
		backupRetryCancel:   make(map[string]context.CancelFunc),
	}
}

// Handle resource deletion logic
func (r *BackupDBReconciler) handleResourceDeletion(ctx context.Context, key string, backupDB *cubridv1.BackupDB) (ctrl.Result, error) {
	// Remove backup schedule and cancel commands
	r.removeBackupSchedule(key)
	r.cancelBackupRoutines(key, backupDB.Spec.CubridDBRef.Names)

	// Remove finalizer to allow resource deletion
	controllerutil.RemoveFinalizer(backupDB, backupDB.Name)
	if err := r.Update(ctx, backupDB); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *BackupDBReconciler) addFinalizer(ctx context.Context, backupDB *cubridv1.BackupDB) error {
	if !controllerutil.ContainsFinalizer(backupDB, backupDB.Name) {
		controllerutil.AddFinalizer(backupDB, backupDB.Name)
		return r.Update(ctx, backupDB)
	}
	return nil
}

func (r *BackupDBReconciler) removeBackupSchedule(key string) {
	// Locking the mutex to ensure safe concurrent access
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	// Check if a scheduled task exists for the provided key
	if scheduledTask, exists := r.backupSchedule[key]; exists {
		// Remove the scheduled task from the task scheduler (cron)
		r.taskScheduler.Remove(scheduledTask.EntryID)
		// Delete the schedule information from the backupSchedule map
		delete(r.backupSchedule, key)
	}
}

func (r *BackupDBReconciler) cancelBackupRoutines(key string, podNames []string) {
	// Locking the mutex to ensure safe concurrent access
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	// Iterate over the pod names and cancel any associated commands
	for _, podName := range podNames {
		// Construct a unique podKey by combining key and podName
		podKey := key + "/" + podName

		// Cancel retry goroutine if it exists for the pod
		if cancel, exists := r.backupRetryCancel[podKey]; exists {
			cancel()                            // Cancel the retry goroutine
			delete(r.backupRetryCancel, podKey) // Remove from backupRetryCancel  map
		}

		// Cancel sendCommandToPod goroutine if it exists for the pod
		if cancel, exists := r.backupCommandCancel[podKey]; exists {
			cancel()                              // Cancel the sendCommandToPod goroutine
			delete(r.backupCommandCancel, podKey) // Remove from backupCommandCancel  map
		}
	}
}

func (r *BackupDBReconciler) checkAndTriggerPodCommand(ctx context.Context, key string, scheduleInfo ScheduleInfo) {
	// Fetch the latest BackupDB resource
	var latestBackupDB cubridv1.BackupDB
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      scheduleInfo.Name,
		Namespace: scheduleInfo.Namespace,
	}, &latestBackupDB); err != nil {
		if errors.IsNotFound(err) {
			backupdblog.Info("BackupDB resource not found. Skipping execution.")
		} else {
			backupdblog.Error(err, "Failed to get latest BackupDB resource")
		}
		return
	}

	// Process each pod and trigger the backup command
	for _, podName := range latestBackupDB.Spec.CubridDBRef.Names {
		// Cancel existing goroutines for the current pod
		r.cancelBackupRoutines(key, []string{podName})

		// Trigger the backup command for the pod in a new goroutine
		r.triggerBackupCommandForPod(ctx, key, podName, &latestBackupDB)
	}
}

func (r *BackupDBReconciler) triggerBackupCommandForPod(ctx context.Context, key, podName string, latestBackupDB *cubridv1.BackupDB) {
	// Create a new context with cancelation support
	ctx, cancel := context.WithCancel(context.Background())

	// Safely store the cancel function for the pod
	r.Mutex.Lock()
	r.backupCommandCancel[key+"/"+podName] = cancel
	r.Mutex.Unlock()

	// Execute the command in a separate goroutine
	go func() {
		defer r.cleanupPodCommandCancelFunc(key, podName)

		// Send command to the pod
		if err := r.sendCommandToPod(ctx, podName, latestBackupDB); err != nil {
			// Schedule a retry if the command fails
			go r.scheduleCommandRetry(key, podName, latestBackupDB)
		}
	}()
}

// Cleans up the cancel function for the pod after execution
func (r *BackupDBReconciler) cleanupPodCommandCancelFunc(key, backupPodName string) {
	// Ensure safe removal of the cancel function
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	// Remove the cancel function for the pod
	delete(r.backupCommandCancel, key+"/"+backupPodName)
}

func (r *BackupDBReconciler) scheduleCommandRetry(key string, backupPodName string, backupDB *cubridv1.BackupDB) {
	// Define retry parameters
	const (
		timeout       = 10 * time.Minute
		retryInterval = 10 * time.Second
		maxAttempts   = 60 // Max attempts before the 10-minute timeout
	)

	// Create a context with timeout and defer cancel
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// Ensure cleanup of cancel function when exiting
	defer func() {
		delete(r.backupRetryCancel, key+"/"+backupPodName)
		backupdblog.Info("Removed cancel function from backupRetryCancel", "key", key+"/"+backupPodName)
	}()

	// Store cancel function for retry
	r.backupRetryCancel[key+"/"+backupPodName] = cancel

	// Create ticker for retry interval
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()

	// Attempt counter
	attempts := 0

	// Retry loop with a maximum of 10 minutes
	for {
		select {
		case <-ctx.Done(): // Context canceled (e.g., CR deleted)
			backupdblog.Info("Retry command canceled due to CR deletion")
			return

		case <-ticker.C: // Proceed with retry
			attempts++

			backupdblog.Info("Retry command", "count", attempts)

			// Send command to pod
			err := r.sendCommandToPod(ctx, backupPodName, backupDB)
			if err != nil {
				// Log error if command fails
				backupdblog.Error(err, "Failed to send command")
			} else {
				// Success, exit retry loop
				backupdblog.Info("Successfully sent command")
				return
			}

			// Stop retrying if the max number of attempts is reached
			if attempts >= maxAttempts {
				backupdblog.Info("max retry attempts reached")
				return
			}
		}
	}
}

func (r *BackupDBReconciler) sendCommandToPod(ctx context.Context, backupPodName string, backupDB *cubridv1.BackupDB) error {
	namespace := backupDB.Spec.CubridDBRef.Namespace
	storageType := backupDB.Spec.StorageRef.StorageType

	// 1. Retrieve Pod information
	pod, err := r.getPod(ctx, namespace, backupPodName)
	if err != nil {
		return r.failStatus(ctx, backupPodName, backupDB, "Failed to get pod", err)
	}

	// 2. Check Pod status
	if err := r.checkPodState(pod, backupPodName); err != nil {
		return r.failStatus(ctx, backupPodName, backupDB, "Pod is not in a valid state", err)
	}

	// 3. Verify the backup storage path
	if _, err := r.getMountPathFromCubriddb(ctx, backupDB, backupPodName, namespace, storageType); err != nil {
		return r.failStatus(ctx, backupPodName, backupDB, "Could not find the path to store backupdb", err)
	}

	// 4. Execute backup command in the Pod
	command := getBackupDBCommand(backupDB.Spec.Schedules.FilePath, backupDB.Spec.Schedules.Args)
	if err := r.execCommandInPod(r.Config, namespace, backupPodName, pod.Spec.Containers[0].Name, command); err != nil {
		return r.failStatus(ctx, backupPodName, backupDB, "Command execution failed", err)
	}

	// 5. Update status to indicate successful execution
	if err := r.updateStatus(ctx, backupPodName, backupDB, DEF.BackupDB_COMPLETED, "Backup completed successfully"); err != nil {
		backupdblog.Error(err, "Failed to update status after sending command", "Pod Name", backupPodName)
	}

	return nil
}

func (r *BackupDBReconciler) getPod(ctx context.Context, namespace, backupPodName string) (*corev1.Pod, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: backupPodName}, &pod); err != nil {
		return nil, fmt.Errorf("failed to get pod %s: %v", backupPodName, err)
	}
	return &pod, nil
}

func (r *BackupDBReconciler) checkPodState(pod *corev1.Pod, backupPodName string) error {
	_, err := util.IsPodAndContainersRunning(pod)
	if err != nil {
		return fmt.Errorf("%s: %v", backupPodName, err)
	}
	return nil
}

func (r *BackupDBReconciler) failStatus(ctx context.Context, backupPodName string, backupDB *cubridv1.BackupDB, msg string, err error) error {
	statusMsg := fmt.Sprintf("%s: %v", msg, err)
	if err := r.updateStatus(ctx, backupPodName, backupDB, DEF.BackupDB_FAILED, statusMsg); err != nil {
		return fmt.Errorf("failed to update status after sending command: %v", err)
	}
	return nil
}

func (r *BackupDBReconciler) updateBackupSchedule(ctx context.Context, req ctrl.Request, backupDB *cubridv1.BackupDB) error {
	var entryID cron.EntryID
	var err error

	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	newSchedule := backupDB.Spec.Schedules.Schedule
	key := req.NamespacedName.String()

	if !isValidCronExpression(newSchedule) {
		return fmt.Errorf("invalid cron expression: %s", newSchedule)
	}

	// Check existing schedule
	if existingTask, exists := r.backupSchedule[key]; exists {
		if existingTask.Schedule == newSchedule {
			fmt.Printf("updateBackupSchedule unchange schedule\n")
			return nil
		}

		// If the existing schedule is different, delete and update
		r.taskScheduler.Remove(existingTask.EntryID)
		delete(r.backupSchedule, key)
	}

	// Add new schedule
	scheduleInfo := ScheduleInfo{
		Name:      backupDB.Name,
		Namespace: backupDB.Namespace,
	}

	if entryID, err = r.taskScheduler.AddFunc(newSchedule, func() {
		r.checkAndTriggerPodCommand(ctx, key, scheduleInfo)
	}); err != nil {
		return err
	}

	// Save the new schedule
	r.backupSchedule[key] = ScheduledTask{
		EntryID:  entryID,
		Schedule: newSchedule,
	}
	return nil
}

func isValidCronExpression(expr string) bool {
	return cronExprRegex.MatchString(expr)
}

func (r *BackupDBReconciler) getMountPathFromCubriddb(
	ctx context.Context,
	backupdb *cubridv1.BackupDB,
	podName string,
	namespace string,
	storageType string,
) (string, error) {
	// Extract the prefix (base name) of the pod by removing the suffix after the last dash
	podNamePrefix := extractPodNamePrefix(podName)
	if podNamePrefix == "" {
		return "", fmt.Errorf("invalid pod name format: no dash found")
	}

	// Fetch the CubridDB object using the pod name prefix
	var cubridDB cubridv1.CubridDB
	err := r.Get(ctx, client.ObjectKey{Name: podNamePrefix, Namespace: namespace}, &cubridDB)
	if err != nil {
		return "", fmt.Errorf("failed to get CubridDB object: %v", err)
	}

	// Search for the matching storage type in the CubridDB spec
	for _, storage := range cubridDB.Spec.Storage {
		if storage.Type == backupdb.StorageRef().StorageType {
			return storage.MountPath, nil
		}
	}

	return "", fmt.Errorf("storage of type %s not found in CubridDB", storageType)
}

// Helper function to extract the base name of the pod (prefix before the last dash)
func extractPodNamePrefix(podName string) string {
	lastDashIndex := strings.LastIndex(podName, "-")
	if lastDashIndex == -1 {
		return ""
	}
	return podName[:lastDashIndex]
}

func (r *BackupDBReconciler) execCommandInPod(config *rest.Config, namespace, podName string, containerNames string, command []string) error {
	// Initialize buffers for capturing command output and error output
	var stdout, stderr bytes.Buffer

	// Set up the pod execution options
	podExecOptions := &corev1.PodExecOptions{
		Command:   command,
		Container: containerNames,
		Stdout:    true,
		Stderr:    true,
	}

	// Create a Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Build the request to execute the command inside the pod
	req := r.buildPodExecRequest(clientset, namespace, podName, podExecOptions)

	// Execute the command using SPDYExecutor
	err = r.executeCommand(req, config, &stdout, &stderr)
	if err != nil {
		// Log error and return
		return fmt.Errorf("%w, %s", err, stderr.String())
	}

	// Log the command output
	backupdblog.V(1).Info("Command Output", "stdout", stdout.String(), "stderr", stderr.String())
	return nil
}

// buildPodExecRequest constructs the Kubernetes REST request for executing the command in the pod
func (r *BackupDBReconciler) buildPodExecRequest(
	clientset *kubernetes.Clientset,
	namespace, podName string,
	podExecOptions *corev1.PodExecOptions,
) *rest.Request {
	return clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(podExecOptions, scheme.ParameterCodec)
}

// executeCommand executes the command in the pod using the given request and SPDYExecutor
func (r *BackupDBReconciler) executeCommand(req *rest.Request, config *rest.Config, stdout, stderr *bytes.Buffer) error {
	_exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("error executing SPDY request: %w", err)
	}

	err = _exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		if exitErr, ok := err.(exec.CodeExitError); ok {
			// Log exit code if available
			exitCode := exitErr.Code
			backupdblog.Info("Exit Code:", "code", exitCode)
		} else {
			// Log unexpected error types
			backupdblog.Error(err, "Unknown error type")
		}
		return err
	}
	return nil
}

func (r *BackupDBReconciler) updateStatus(ctx context.Context, backupPod string, backupdb *cubridv1.BackupDB, status, message string) error {
	const maxRetries = 5
	const retryDelay = 100 * time.Millisecond

	backupdblog.Info("updateStatus start")

	for i := 0; i < maxRetries; i++ {
		// Fetch the latest BackupDB object
		updatedBackupDB := &cubridv1.BackupDB{}
		err := r.Get(ctx, types.NamespacedName{Name: backupdb.Name, Namespace: backupdb.Namespace}, updatedBackupDB)
		if err != nil {
			backupdblog.Error(err, "Failed to fetch latest BackupDB object")
			return err
		}

		// Initialize BackupStatus map if nil
		if updatedBackupDB.Status.BackupStatus == nil {
			updatedBackupDB.Status.BackupStatus = make(map[string]cubridv1.PodsStatus)
		}

		// Update pod status
		currentTime := metav1.Time{Time: time.Now()}
		updatedBackupDB.Status.BackupStatus[backupPod] = cubridv1.PodsStatus{
			Status:      status,
			Message:     strings.TrimSuffix(message, "\n"),
			LastUpdated: currentTime,
		}

		// Calculate success count
		completeCount := 0
		totalPods := len(updatedBackupDB.Spec.CubridDBRef.Names)

		for _, podStatus := range updatedBackupDB.Status.BackupStatus {
			if podStatus.Status == DEF.BackupDB_COMPLETED {
				completeCount++
			}
		}

		// Update OverallStatus
		updatedBackupDB.Status.OverallStatus = fmt.Sprintf("%d/%d", completeCount, totalPods)

		// Attempt to update status
		if err := r.Status().Update(ctx, updatedBackupDB); err != nil {
			if errors.IsConflict(err) {
				// Retry on conflict
				backupdblog.V(1).Info("Conflict detected, retrying...", "Attempt", i+1)
				time.Sleep(retryDelay)
				continue
			}
			backupdblog.Error(err, "Failed to update BackupDB status", "Pod Name", backupPod, "Status", status, "Message", message)
			return err
		}
		return nil
	}

	// If all retries failed, return an error
	err := fmt.Errorf("failed to update BackupDB status after %d attempts", maxRetries)
	backupdblog.Info("Giving up on status update", "Pod Name", backupdb.Name, "error", err)
	return err
}

func getBackupDBCommand(filePath, args string) []string {
	command := fmt.Sprintf("$CUBRID/%s %s", filePath, args)
	return []string{"sh", "-c", command}
}
