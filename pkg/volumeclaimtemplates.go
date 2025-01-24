package pkg

import (
	"fmt"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreatePersistentVolumeClaimSpec(storage cubridv1.Storage) (corev1.PersistentVolumeClaimSpec, error) {
	if len(storage.VolumeName) > 0 {
		return corev1.PersistentVolumeClaimSpec{
			AccessModes: CreatePersistentVolumeAccessModes(),
			Resources:   CreateVolumeResourceRequirements(storage.Size),
			VolumeName:  storage.VolumeName,
		}, nil
	} else if len(storage.StorageClassName) > 0 {
		return corev1.PersistentVolumeClaimSpec{
			AccessModes:      CreatePersistentVolumeAccessModes(),
			Resources:        CreateVolumeResourceRequirements(storage.Size),
			StorageClassName: &storage.StorageClassName,
		}, nil
	}
	return corev1.PersistentVolumeClaimSpec{},
		fmt.Errorf("both VolumeName and StorageClassName are empty for storage: %s", storage.Name)
}

func CreatePersistentVolumeAccessModes() []corev1.PersistentVolumeAccessMode {
	return []corev1.PersistentVolumeAccessMode{
		corev1.ReadWriteOnce,
	}
}

func CreateVolumeResourceRequirements(size *resource.Quantity) corev1.VolumeResourceRequirements {
	return corev1.VolumeResourceRequirements{
		Requests: CreateResourceList(size),
	}
}

func CreateResourceList(size *resource.Quantity) corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceStorage: *size,
	}
}

func NewPersistentVolumeClaims(cubridDB *cubridv1.CubridDB) ([]corev1.PersistentVolumeClaim, error) {
	pvcs := make([]corev1.PersistentVolumeClaim, 0, len(cubridDB.Spec.Storage))

	for _, storage := range cubridDB.Spec.Storage {
		spec, err := CreatePersistentVolumeClaimSpec(storage)
		if err != nil {
			return nil, fmt.Errorf("failed to create PVC spec for storage '%s': %w", storage.Name, err)
		}
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: storage.Name,
			},
			Spec: spec,
		}
		pvcs = append(pvcs, pvc)
	}

	return pvcs, nil
}
