package pkg

import (
	"strconv"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	DEF "github.com/cubrid/cubrid-operator/pkg/define"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func CreateStatefulSet(
	cubridDB *cubridv1.CubridDB,
	initContainers []corev1.Container,
	containers []corev1.Container,
	serviceName string,
	replicaNum int32,
	podTemplate corev1.PodTemplateSpec,
	volumeClaimTemplates []corev1.PersistentVolumeClaim,
) *appsv1.StatefulSet {
	var group_name string = cubridDB.Name
	var group_type string = ""
	if cubridDB.IsHAEnabled() {
		group_type = cubridDB.HAmodeType()
		if group_type == DEF.HA_REPLICA_TYPE {
			group_name = cubridDB.Spec.Replication.HAmodeType.CubridRef.Name
		}
	}

	labels := map[string]string{"app": cubridDB.Name}
	annotations := map[string]string{"lastReplicas": strconv.Itoa(int(replicaNum))}

	return &appsv1.StatefulSet{
		ObjectMeta: NewObjectMeta(cubridDB.Name, cubridDB.Namespace, labels, annotations),
		Spec: appsv1.StatefulSetSpec{
			Selector:             NewLabelSelector(cubridDB.Name, group_name, group_type, serviceName),
			ServiceName:          serviceName,
			Replicas:             &replicaNum,
			UpdateStrategy:       statefulSetUpdateStrategy(cubridDB.Spec.UpdateStrategy),
			Template:             podTemplate,
			VolumeClaimTemplates: volumeClaimTemplates,
		},
	}
}

func statefulSetUpdateStrategy(strategy *appsv1.StatefulSetUpdateStrategy) appsv1.StatefulSetUpdateStrategy {
	if strategy != nil {
		return *strategy
	}
	return appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.RollingUpdateStatefulSetStrategyType,
	}
}
