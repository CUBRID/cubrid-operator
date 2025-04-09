package manager

import (
	"context"
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	"github.com/cubrid/cubrid-operator/pkg"
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// log is for logging in this package.
var stslogger = log.Log.WithName("StatufulSet")

type StatefulSetManager struct {
	client.Client
	Scheme *runtime.Scheme
}

func NewStatefulSetManager(client client.Client, scheme *runtime.Scheme) *StatefulSetManager {
	return &StatefulSetManager{
		Client: client,
		Scheme: scheme,
	}
}

func (r *StatefulSetManager) ReconcileStatefulSet(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	stslogger.V(1).Info("Handling StatefulSet for CubridDB")

	var replicaNum int32 = 1
	serviceName := pkg.CreateHeadlessServiceName(cubridDB.Name)

	if cubridDB.Spec.Replication.Enable {
		replicaNum = cubridDB.Spec.Replication.Replicas
	}

	copyConfVolumeMountSpecs := []pkg.VolumeMountConfig{
		{Name: DEF.ConfBackupVolumeName, MountPath: DEF.ConfBackupMountPath},
		{Name: DEF.LogsBackupVolumeName, MountPath: DEF.LogsBackupMountPath},
		{Name: DEF.DBBackupVolumeName, MountPath: DEF.DBBackupMountPath},
	}

	recoveryConfVolumeMountSpecs := []pkg.VolumeMountConfig{
		{Name: DEF.ConfStorageVolumeName, MountPath: DEF.ConfMountPath},
		{Name: DEF.DatabaseStorageVolumeName, MountPath: DEF.DatabaseMountPath},
		{Name: DEF.BackupDBStorageVolumeName, MountPath: DEF.BackupDBMountPath},
		{Name: DEF.LogsStorageVolumeName, MountPath: DEF.LogsMountPath},
		{Name: DEF.ConfBackupVolumeName, MountPath: DEF.ConfBackupMountPath},
		{Name: DEF.LogsBackupVolumeName, MountPath: DEF.LogsBackupMountPath},
	}

	initContainers := pkg.CreateInitContainers(cubridDB, copyConfVolumeMountSpecs, recoveryConfVolumeMountSpecs)

	containers := []corev1.Container{
		pkg.CreateContainers(cubridDB.Name,
			cubridDB.Spec.Image,
			pkg.ConfigureSecurityContext(DEF.CubridUser, DEF.CubridGroup),
			pkg.CreateContainerPorts(ctx, cubridDB),
			pkg.CreateVolumeMountsForCubridDB(cubridDB)),
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

	podTemplateSpec := pkg.CreatePodTemplateSpec(
		pkg.NewLabelSelector(cubridDB.Name, group_name, group_type, serviceName),
		initContainers,
		containers,
		pkg.CreatePodSecurityContext(DEF.CubridUser, DEF.CubridGroup),
		volumes,
		pkg.CreateAffinity(cubridDB),
	)

	volumeClaimTemplates, err := pkg.NewPersistentVolumeClaims(cubridDB)
	if err != nil {
		return fmt.Errorf("error createing PVCs: %v", err)
	}

	// Create the StatefulSet
	desiredSTS := pkg.CreateStatefulSet(
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

	if cubridDB.IsHAEnabled() {
		lastReplicasAnnotation := existingSts.Annotations["lastReplicas"]
		currentReplicas := *existingSts.Spec.Replicas
		desiredReplicas := cubridDB.Spec.Replication.Replicas

		if desiredReplicas != currentReplicas && strconv.Itoa(int(desiredReplicas)) != lastReplicasAnnotation {
			patch := client.MergeFrom(existingSts.DeepCopy())
			existingSts.Spec.Replicas = &desiredReplicas
			existingSts.Annotations["lastReplicas"] = strconv.Itoa(int(desiredReplicas))

			if err := r.Client.Patch(ctx, &existingSts, patch); err != nil {
				return fmt.Errorf("error patching StatefulSet replicas from CR update: %v", err)
			}
			return nil
		}

		if strconv.Itoa(int(currentReplicas)) != lastReplicasAnnotation {
			cubridDB.Spec.Replication.Replicas = currentReplicas
			if err := r.Client.Update(ctx, cubridDB); err != nil {
				return fmt.Errorf("error updating CR replicas from StatefulSet scale: %v", err)
			}

			if err := pkg.UpdateLastReplicasAnnotation(ctx, r.Client, &existingSts, currentReplicas); err != nil {
				return fmt.Errorf("error updating StatefulSet annotation(LastReplicas): %v", err)
			}

			return nil
		}
	} else {
		desiredReplicas := cubridDB.Spec.Replication.Replicas
		patch := client.MergeFrom(existingSts.DeepCopy())
		existingSts.Spec.Replicas = &desiredReplicas
		existingSts.Annotations["lastReplicas"] = strconv.Itoa(int(desiredReplicas))

		if err := r.Client.Patch(ctx, &existingSts, patch); err != nil {
			return fmt.Errorf("error patching StatefulSet replicas from CR update: %v", err)
		}
	}

	return nil
}
