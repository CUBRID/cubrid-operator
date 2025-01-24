package pkg

import (
	"context"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewObjectMeta(
	name string,
	namespace string,
	labels map[string]string,
	annotations map[string]string,
) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   namespace,
		Labels:      labels,
		Annotations: annotations,
	}
}

func UpdateLastReplicasAnnotation(
	ctx context.Context,
	c client.Client,
	statefulSet *appsv1.StatefulSet,
	currentReplicas int32,
) error {
	currentReplicasStr := strconv.Itoa(int(currentReplicas))

	if statefulSet.Annotations["lastReplicas"] != currentReplicasStr {
		if statefulSet.Annotations == nil {
			statefulSet.Annotations = make(map[string]string)
		}
		statefulSet.Annotations["lastReplicas"] = currentReplicasStr

		if err := c.Update(ctx, statefulSet); err != nil {
			return err
		}
	}
	return nil
}
