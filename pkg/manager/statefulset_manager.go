package manager

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	metapkg "github.com/cubrid/cubrid-operator/pkg/meta"
	"github.com/cubrid/cubrid-operator/pkg/rbac"
	res "github.com/cubrid/cubrid-operator/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var stslogger = log.Log.WithName("StatefulSet")

type StatefulSetManager struct {
	client.Client
	Scheme    *runtime.Scheme
	clientset *kubernetes.Clientset
}

func NewStatefulSetManager(client client.Client, scheme *runtime.Scheme) (*StatefulSetManager, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes clientset: %v", err)
	}

	return &StatefulSetManager{
		Client:    client,
		Scheme:    scheme,
		clientset: clientset,
	}, nil
}

func (r *StatefulSetManager) ReconcileStatefulSet(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	stslogger.V(1).Info("Handling StatefulSet for CubridDB")

	// Validate storage configuration
	if err := r.validateStorageConfiguration(cubridDB); err != nil {
		return fmt.Errorf("invalid storage configuration: %v", err)
	}

	var replicaNum int32 = 1
	serviceName := res.CreateHeadlessServiceName(cubridDB.Name)

	if cubridDB.Spec.Replication.Enable {
		replicaNum = cubridDB.Spec.Replication.Replicas
	}

	copyConfVolumeMountSpecs := []res.VolumeMountConfig{
		{Name: DEF.ConfBackupVolumeName, MountPath: DEF.ConfBackupMountPath},
		{Name: DEF.LogsBackupVolumeName, MountPath: DEF.LogsBackupMountPath},
		{Name: DEF.DBBackupVolumeName, MountPath: DEF.DBBackupMountPath},
	}

	// Get PVC names from storage configuration
	var confPVCName, databasePVCName, backupPVCName, logsPVCName string
	for _, storage := range cubridDB.Spec.Storage {
		switch storage.Type {
		case DEF.StorageType_conf:
			if storage.VolumeClaimTemplate != nil && storage.VolumeClaimTemplate.Metadata != nil {
				confPVCName = storage.VolumeClaimTemplate.Metadata.Name
			} else {
				confPVCName = storage.Name
			}
		case DEF.StorageType_Database:
			if storage.VolumeClaimTemplate != nil && storage.VolumeClaimTemplate.Metadata != nil {
				databasePVCName = storage.VolumeClaimTemplate.Metadata.Name
			} else {
				databasePVCName = storage.Name
			}
		case DEF.StorageType_backup:
			if storage.VolumeClaimTemplate != nil && storage.VolumeClaimTemplate.Metadata != nil {
				backupPVCName = storage.VolumeClaimTemplate.Metadata.Name
			} else {
				backupPVCName = storage.Name
			}
		case DEF.StorageType_Logs:
			if storage.VolumeClaimTemplate != nil && storage.VolumeClaimTemplate.Metadata != nil {
				logsPVCName = storage.VolumeClaimTemplate.Metadata.Name
			} else {
				logsPVCName = storage.Name
			}
		}
	}

	recoveryConfVolumeMountSpecs := []res.VolumeMountConfig{
		{Name: confPVCName, MountPath: DEF.ConfMountPath},
		{Name: databasePVCName, MountPath: DEF.DatabaseMountPath},
		{Name: backupPVCName, MountPath: DEF.BackupDBMountPath},
		{Name: logsPVCName, MountPath: DEF.LogsMountPath},
		{Name: DEF.ConfBackupVolumeName, MountPath: DEF.ConfBackupMountPath},
		{Name: DEF.LogsBackupVolumeName, MountPath: DEF.LogsBackupMountPath},
	}

	initContainers := res.CreateInitContainers(cubridDB, copyConfVolumeMountSpecs, recoveryConfVolumeMountSpecs)

	containers := []corev1.Container{
		res.CreateContainers(cubridDB.Name,
			cubridDB.Spec.Image,
			res.ConfigureSecurityContext(DEF.CubridUser, DEF.CubridGroup),
			res.CreateContainerPorts(ctx, cubridDB),
			res.CreateVolumeMountsForCubridDB(cubridDB)),
	}

	volumes := []corev1.Volume{
		{
			Name: DEF.ConfBackupVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: DEF.LogsBackupVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: DEF.DBBackupVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	var group_name string = cubridDB.Name
	var group_type string = ""
	if cubridDB.IsHAEnabled() {
		group_type = cubridDB.HAmodeType()
		if cubridDB.HAmodeType() == DEF.HA_REPLICA_TYPE {
			group_name = cubridDB.Spec.Replication.HAmodeType.CubridRef.Name
		}
	}

	// Configure ServiceAccount
	// Use one ServiceAccount per namespace
	serviceAccountName := fmt.Sprintf("cubrid-%s-sa", cubridDB.Namespace)

	// Create/Manage Role Based Access Control (RBAC) resources
	if err := rbac.ReconcileRBACResources(r.clientset, cubridDB.Namespace); err != nil {
		return fmt.Errorf("error reconciling RBAC resources: %v", err)
	}

	podTemplateSpec := res.CreatePodTemplateSpec(
		metapkg.NewLabelSelector(cubridDB.Name, group_name, group_type, serviceName),
		initContainers,
		containers,
		res.CreatePodSecurityContext(DEF.CubridUser, DEF.CubridGroup),
		volumes,
		res.CreateAffinity(cubridDB),
		serviceAccountName,
	)

	volumeClaimTemplates, err := res.NewPersistentVolumeClaims(cubridDB)
	if err != nil {
		return fmt.Errorf("error creating PVCs: %v", err)
	}

	// Create the StatefulSet
	desiredSTS := res.CreateStatefulSet(
		cubridDB, initContainers, containers, serviceName, replicaNum, podTemplateSpec, volumeClaimTemplates)

	if err := controllerutil.SetControllerReference(cubridDB, desiredSTS, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to StatefulSet: %v", err)
	}

	key := client.ObjectKeyFromObject(desiredSTS)
	var existingSts appsv1.StatefulSet
	if err := r.Get(ctx, key, &existingSts); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting StatefulSet: %v", err)
		}
		if err := r.Create(ctx, desiredSTS); err != nil {
			return fmt.Errorf("error creating StatefulSet: %v", err)
		}
		return nil
	}

	// Handle replicas synchronization
	if err := r.syncReplicas(ctx, cubridDB, &existingSts); err != nil {
		return fmt.Errorf("error synchronizing replicas: %v", err)
	}

	// Handle StatefulSet spec updates (image, updateStrategy, etc.)
	if err := r.syncStatefulSetSpec(ctx, &existingSts, desiredSTS); err != nil {
		return fmt.Errorf("error synchronizing StatefulSet spec: %v", err)
	}

	return nil
}

// syncReplicas handles the synchronization of replicas between StatefulSet and CubridDB CR
func (r *StatefulSetManager) syncReplicas(
	ctx context.Context,
	cubridDB *cubridv1.CubridDB,
	existingSts *appsv1.StatefulSet,
) error {
	currentReplicas := *existingSts.Spec.Replicas
	desiredReplicas := cubridDB.Spec.Replication.Replicas

	// Get the previous StatefulSet replicas from annotation
	lastReplicasStr := existingSts.Annotations["cubrid.io/last-replicas"]
	lastReplicas := currentReplicas
	if lastReplicasStr != "" {
		if val, err := strconv.ParseInt(lastReplicasStr, 10, 32); err == nil {
			lastReplicas = int32(val)
		}
	}

	// If replicas are different
	if currentReplicas != desiredReplicas {
		// Check which side changed
		if lastReplicas == desiredReplicas {
			// StatefulSet was changed (e.g., kubectl scale)
			// Validate replicas change
			if err := r.validateReplicasChange(cubridDB, currentReplicas); err != nil {
				return fmt.Errorf("invalid replicas change: %v", err)
			}

			// Update CubridDB CR replicas to match StatefulSet
			cubridDB.Spec.Replication.Replicas = currentReplicas
			if err := r.Client.Update(ctx, cubridDB); err != nil {
				return fmt.Errorf("error updating CubridDB replicas: %v", err)
			}

			stslogger.V(1).Info("Synchronized CubridDB replicas from StatefulSet change",
				"name", cubridDB.Name,
				"namespace", cubridDB.Namespace,
				"replicas", currentReplicas)
		} else {
			// CubridDB CR was changed
			// Validate replicas change
			if err := r.validateReplicasChange(cubridDB, desiredReplicas); err != nil {
				return fmt.Errorf("invalid replicas change: %v", err)
			}

			// Update StatefulSet replicas
			patch := client.MergeFrom(existingSts.DeepCopy())
			existingSts.Spec.Replicas = &desiredReplicas

			if err := r.Client.Patch(ctx, existingSts, patch); err != nil {
				return fmt.Errorf("error patching StatefulSet replicas: %v", err)
			}

			stslogger.V(1).Info("Updated StatefulSet replicas from CubridDB CR change",
				"name", existingSts.Name,
				"namespace", existingSts.Namespace,
				"oldReplicas", currentReplicas,
				"newReplicas", desiredReplicas)
		}
	}

	// Update the last replicas annotation
	patch := client.MergeFrom(existingSts.DeepCopy())
	if existingSts.Annotations == nil {
		existingSts.Annotations = make(map[string]string)
	}
	existingSts.Annotations["cubrid.io/last-replicas"] = strconv.FormatInt(int64(currentReplicas), 10)
	if err := r.Client.Patch(ctx, existingSts, patch); err != nil {
		return fmt.Errorf("error updating last replicas annotation: %v", err)
	}
	return nil
}

// validateReplicasChange validates the replicas change request
func (r *StatefulSetManager) validateReplicasChange(cubridDB *cubridv1.CubridDB, desiredReplicas int32) error {
	if cubridDB.IsHAEnabled() {
		// HA mode: replicas must be greater than 0
		if desiredReplicas <= 0 {
			return fmt.Errorf("HA mode replicas must be greater than 0")
		}
	} else {
		// Single mode: replicas must be exactly 1
		if desiredReplicas != 1 {
			return fmt.Errorf("single mode must have exactly 1 replica")
		}
	}

	return nil
}

// syncStatefulSetSpec handles StatefulSet spec updates like image, updateStrategy, etc.
func (r *StatefulSetManager) syncStatefulSetSpec(
	ctx context.Context,
	existingSts *appsv1.StatefulSet,
	desiredSts *appsv1.StatefulSet,
) error {
	// Check if any spec changes are needed
	needsUpdate := false
	needsRollingUpdate := false

	// Check image changes - this triggers rolling update
	if len(existingSts.Spec.Template.Spec.Containers) > 0 && len(desiredSts.Spec.Template.Spec.Containers) > 0 {
		if existingSts.Spec.Template.Spec.Containers[0].Image != desiredSts.Spec.Template.Spec.Containers[0].Image {
			stslogger.V(1).Info("Image change detected - will trigger rolling update",
				"name", existingSts.Name,
				"namespace", existingSts.Namespace,
				"oldImage", existingSts.Spec.Template.Spec.Containers[0].Image,
				"newImage", desiredSts.Spec.Template.Spec.Containers[0].Image)
			needsUpdate = true
			needsRollingUpdate = true
		}
	}

	// Check updateStrategy changes - this doesn't trigger rolling update
	if !reflect.DeepEqual(existingSts.Spec.UpdateStrategy, desiredSts.Spec.UpdateStrategy) {
		stslogger.V(1).Info("UpdateStrategy change detected",
			"name", existingSts.Name,
			"namespace", existingSts.Namespace)
		needsUpdate = true
	}

	// Check other template changes (excluding image, replicas, and affinity)
	existingTemplate := existingSts.Spec.Template.DeepCopy()
	desiredTemplate := desiredSts.Spec.Template.DeepCopy()

	// Remove image from comparison since it's handled separately
	if len(existingTemplate.Spec.Containers) > 0 && len(desiredTemplate.Spec.Containers) > 0 {
		existingTemplate.Spec.Containers[0].Image = ""
		desiredTemplate.Spec.Containers[0].Image = ""
	}

	// Remove affinity from comparison since it's handled separately
	existingTemplate.Spec.Affinity = nil
	desiredTemplate.Spec.Affinity = nil

	// Remove replicas from comparison as they are handled separately
	if !reflect.DeepEqual(existingTemplate, desiredTemplate) {
		// Add detailed debugging to identify what's changing
		stslogger.V(2).Info("Template difference detected - analyzing changes",
			"name", existingSts.Name,
			"namespace", existingSts.Namespace)

		// Compare containers
		if len(existingTemplate.Spec.Containers) > 0 && len(desiredTemplate.Spec.Containers) > 0 {
			existingContainer := existingTemplate.Spec.Containers[0]
			desiredContainer := desiredTemplate.Spec.Containers[0]

			if !reflect.DeepEqual(existingContainer.Ports, desiredContainer.Ports) {
				stslogger.V(2).Info("Container ports differ",
					"name", existingSts.Name,
					"existingPorts", existingContainer.Ports,
					"desiredPorts", desiredContainer.Ports)
			}

			if !reflect.DeepEqual(existingContainer.VolumeMounts, desiredContainer.VolumeMounts) {
				stslogger.V(2).Info("Container volume mounts differ",
					"name", existingSts.Name,
					"existingVolumeMounts", existingContainer.VolumeMounts,
					"desiredVolumeMounts", desiredContainer.VolumeMounts)
			}

			if !reflect.DeepEqual(existingContainer.SecurityContext, desiredContainer.SecurityContext) {
				stslogger.V(2).Info("Container security context differs",
					"name", existingSts.Name,
					"existingSecurityContext", existingContainer.SecurityContext,
					"desiredSecurityContext", desiredContainer.SecurityContext)
			}
		}

		// Compare init containers
		if !reflect.DeepEqual(existingTemplate.Spec.InitContainers, desiredTemplate.Spec.InitContainers) {
			stslogger.V(2).Info("Init containers differ",
				"name", existingSts.Name,
				"existingInitContainers", existingTemplate.Spec.InitContainers,
				"desiredInitContainers", desiredTemplate.Spec.InitContainers)
		}

		// Compare volumes
		if !reflect.DeepEqual(existingTemplate.Spec.Volumes, desiredTemplate.Spec.Volumes) {
			stslogger.V(2).Info("Volumes differ",
				"name", existingSts.Name,
				"existingVolumes", existingTemplate.Spec.Volumes,
				"desiredVolumes", desiredTemplate.Spec.Volumes)
		}

		// Compare security context
		if !reflect.DeepEqual(existingTemplate.Spec.SecurityContext, desiredTemplate.Spec.SecurityContext) {
			stslogger.V(2).Info("Pod security context differs",
				"name", existingSts.Name,
				"existingSecurityContext", existingTemplate.Spec.SecurityContext,
				"desiredSecurityContext", desiredTemplate.Spec.SecurityContext)
		}

		// Compare service account
		if existingTemplate.Spec.ServiceAccountName != desiredTemplate.Spec.ServiceAccountName {
			stslogger.V(2).Info("service account differs",
				"name", existingSts.Name,
				"existingServiceAccount", existingTemplate.Spec.ServiceAccountName,
				"desiredServiceAccount", desiredTemplate.Spec.ServiceAccountName)
		}

		if needsRollingUpdate {
			// Image change detected, include all template changes in rolling update
			stslogger.V(2).Info("Additional template changes detected - will be included in rolling update",
				"name", existingSts.Name,
				"namespace", existingSts.Namespace)
		} else {
			// Non-image template changes (like container ports) - log but don't update to prevent rolling updates
			stslogger.V(2).Info("Non-image template change detected - skipping to prevent rolling update",
				"name", existingSts.Name,
				"namespace", existingSts.Namespace,
				"reason", "Only image changes trigger rolling updates",
				"note", "Container ports and other template changes will be handled by service reconciliation")
		}
	}

	// Handle affinity changes separately
	if !reflect.DeepEqual(existingSts.Spec.Template.Spec.Affinity, desiredSts.Spec.Template.Spec.Affinity) {
		stslogger.V(1).Info("Affinity change detected - updating separately",
			"name", existingSts.Name,
			"namespace", existingSts.Namespace)
		needsUpdate = true
	}

	if needsUpdate {
		// Update StatefulSet spec
		patch := client.MergeFrom(existingSts.DeepCopy())

		if needsRollingUpdate {
			// Full template update - triggers rolling update
			existingSts.Spec.Template = desiredSts.Spec.Template
			stslogger.V(1).Info("Performing rolling update due to image change",
				"name", existingSts.Name,
				"namespace", existingSts.Namespace)
		} else {
			// Update only specific fields without triggering rolling update
			if !reflect.DeepEqual(existingSts.Spec.Template.Spec.Affinity, desiredSts.Spec.Template.Spec.Affinity) {
				existingSts.Spec.Template.Spec.Affinity = desiredSts.Spec.Template.Spec.Affinity
			}
		}

		// Update updateStrategy
		existingSts.Spec.UpdateStrategy = desiredSts.Spec.UpdateStrategy

		if err := r.Client.Patch(ctx, existingSts, patch); err != nil {
			return fmt.Errorf("error patching StatefulSet spec: %v", err)
		}

		stslogger.V(1).Info("Updated StatefulSet spec",
			"name", existingSts.Name,
			"namespace", existingSts.Namespace,
			"rollingUpdate", needsRollingUpdate)
	}

	stslogger.V(1).Info("syncStatefulSetSpec end")
	return nil
}

// validateStorageConfiguration validates the storage configuration for all storages
func (r *StatefulSetManager) validateStorageConfiguration(cubridDB *cubridv1.CubridDB) error {
	for _, storage := range cubridDB.Spec.Storage {
		if err := storage.ValidateStorage(); err != nil {
			return err
		}
	}
	return nil
}
