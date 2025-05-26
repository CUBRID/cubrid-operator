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
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	meta "github.com/cubrid/cubrid-operator/pkg/meta"
	"github.com/cubrid/cubrid-operator/pkg/rbac"
	res "github.com/cubrid/cubrid-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// log is for logging in this package.
var stslogger = log.Log.WithName("StatufulSet")

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

	recoveryConfVolumeMountSpecs := []res.VolumeMountConfig{
		{Name: DEF.ConfStorageVolumeName, MountPath: DEF.ConfMountPath},
		{Name: DEF.DatabaseStorageVolumeName, MountPath: DEF.DatabaseMountPath},
		{Name: DEF.BackupDBStorageVolumeName, MountPath: DEF.BackupDBMountPath},
		{Name: DEF.LogsStorageVolumeName, MountPath: DEF.LogsMountPath},
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
		meta.NewLabelSelector(cubridDB.Name, group_name, group_type, serviceName),
		initContainers,
		containers,
		res.CreatePodSecurityContext(DEF.CubridUser, DEF.CubridGroup),
		volumes,
		res.CreateAffinity(cubridDB),
		serviceAccountName,
	)

	volumeClaimTemplates, err := res.NewPersistentVolumeClaims(cubridDB)
	if err != nil {
		return fmt.Errorf("error createing PVCs: %v", err)
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

			if err := meta.UpdateLastReplicasAnnotation(ctx, r.Client, &existingSts, currentReplicas); err != nil {
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
