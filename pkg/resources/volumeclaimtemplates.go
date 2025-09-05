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

package pkg

import (
	"fmt"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreatePersistentVolumeClaimSpec(storage cubridv1.Storage) (corev1.PersistentVolumeClaimSpec, error) {
	// 1. If volumeClaimTemplate exists
	if storage.VolumeClaimTemplate != nil {
		spec := corev1.PersistentVolumeClaimSpec{
			AccessModes: storage.VolumeClaimTemplate.AccessModes,
			Resources:   storage.VolumeClaimTemplate.Resources,
		}

		// Set default access modes if not specified
		if len(spec.AccessModes) == 0 {
			spec.AccessModes = CreatePersistentVolumeAccessModes()
		}

		// Determine provisioning type based on storageClassName
		if storage.VolumeClaimTemplate.StorageClassName == nil || *storage.VolumeClaimTemplate.StorageClassName == "" {
			// Static provisioning: use selector and empty storage class
			if storage.VolumeClaimTemplate.Selector == nil {
				return corev1.PersistentVolumeClaimSpec{},
					fmt.Errorf("selector is required for static provisioning in storage: %s", storage.Name)
			}
			spec.Selector = storage.VolumeClaimTemplate.Selector
			// Set empty storage class for static provisioning
			emptyStorageClass := ""
			spec.StorageClassName = &emptyStorageClass
		} else {
			// Dynamic provisioning: use storage class
			spec.StorageClassName = storage.VolumeClaimTemplate.StorageClassName
		}

		return spec, nil
	}

	// 2. If StorageClassName exists (Dynamic provisioning - legacy method)
	if len(storage.StorageClassName) > 0 {
		return corev1.PersistentVolumeClaimSpec{
			AccessModes:      CreatePersistentVolumeAccessModes(),
			Resources:        CreateVolumeResourceRequirements(storage.Size),
			StorageClassName: &storage.StorageClassName,
		}, nil
	}

	// 3. If VolumeName exists (Reference existing PV - deprecated)
	if len(storage.VolumeName) > 0 {
		return corev1.PersistentVolumeClaimSpec{
			AccessModes: CreatePersistentVolumeAccessModes(),
			Resources:   CreateVolumeResourceRequirements(storage.Size),
			VolumeName:  storage.VolumeName,
		}, nil
	}

	return corev1.PersistentVolumeClaimSpec{},
		fmt.Errorf("no storage configuration provided for storage: %s", storage.Name)
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

		// Determine PVC name
		pvcName := storage.Name
		if storage.VolumeClaimTemplate != nil && storage.VolumeClaimTemplate.Metadata != nil {
			pvcName = storage.VolumeClaimTemplate.Metadata.Name
		}

		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvcName,
			},
			Spec: spec,
		}
		pvcs = append(pvcs, pvc)
	}

	return pvcs, nil
}
