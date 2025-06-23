package pkg

import (
	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	meta "github.com/cubrid/cubrid-operator/pkg/meta"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	return &appsv1.StatefulSet{
		ObjectMeta: meta.NewObjectMeta(cubridDB.Name, cubridDB.Namespace, labels, nil),
		Spec: appsv1.StatefulSetSpec{
			Selector:             meta.NewLabelSelector(cubridDB.Name, group_name, group_type, serviceName),
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
		if strategy.Type == appsv1.RollingUpdateStatefulSetStrategyType {
			if strategy.RollingUpdate == nil {
				strategy.RollingUpdate = &appsv1.RollingUpdateStatefulSetStrategy{}
			}
			if strategy.RollingUpdate.Partition == nil {
				partition := int32(0)
				strategy.RollingUpdate.Partition = &partition
			}
			if strategy.RollingUpdate.MaxUnavailable == nil {
				maxUnavailable := intstr.FromInt(1)
				strategy.RollingUpdate.MaxUnavailable = &maxUnavailable
			}
		}
		return *strategy
	}

	partition := int32(0)
	maxUnavailable := intstr.FromInt(1)
	return appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.RollingUpdateStatefulSetStrategyType,
		RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
			Partition:      &partition,
			MaxUnavailable: &maxUnavailable,
		},
	}
}
