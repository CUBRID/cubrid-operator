package pkg

import (
	"context"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	v1 "github.com/cubrid/cubrid-operator/api/v1"
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VolumeMountConfig struct {
	Name      string
	MountPath string
}

func CreateInitContainer(
	name, image string,
	command []string,
	securityContext *corev1.SecurityContext,
	volumeMounts []corev1.VolumeMount,
) corev1.Container {
	return corev1.Container{
		Name:            name,
		Image:           image,
		Command:         command,
		SecurityContext: securityContext,
		VolumeMounts:    volumeMounts,
	}
}

func CreateInitContainers(
	cubridDB *v1.CubridDB,
	copyConfVolumeMountSpecs []VolumeMountConfig,
	recoveryConfVolumeMountSpecs []VolumeMountConfig,
) []corev1.Container {
	return []corev1.Container{
		CreateInitContainer(
			DEF.InitCopyConfContainerName,
			cubridDB.Spec.Image,
			[]string{"sh", "-c", DEF.InitCopyConfCommand},
			ConfigureSecurityContext(0, 0),
			CreateVolumeMounts(copyConfVolumeMountSpecs),
		),
		CreateInitContainer(
			DEF.InitRecoveryConfContainerName,
			DEF.BusyBoxImage,
			[]string{"sh", "-c", DEF.InitRecoveryConfCommand},
			ConfigureSecurityContext(0, 0),
			CreateVolumeMounts(recoveryConfVolumeMountSpecs),
		),
	}
}

func CreatePodTemplateSpec(
	selector *metav1.LabelSelector,
	initContainers []corev1.Container,
	containers []corev1.Container,
	securityContext *corev1.PodSecurityContext,
	volumes []corev1.Volume,
	affinity *corev1.Affinity) corev1.PodTemplateSpec {

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: selector.MatchLabels,
		},
		Spec: corev1.PodSpec{
			InitContainers:  initContainers,
			Containers:      containers,
			SecurityContext: securityContext,
			Volumes:         volumes,
			Affinity:        affinity,
		},
	}
}

func CreateContainers(
	name, image string,
	securityContext *corev1.SecurityContext,
	ports []corev1.ContainerPort,
	volumeMounts []corev1.VolumeMount) corev1.Container {

	return corev1.Container{
		Name:            name,
		Image:           image,
		SecurityContext: securityContext,
		Ports:           ports,
		VolumeMounts:    volumeMounts,
	}
}

func CreateVolumeMounts(specs []VolumeMountConfig) []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0, len(specs))

	for _, spec := range specs {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      spec.Name,
			MountPath: spec.MountPath,
		})
	}
	return volumeMounts
}

func ConfigureSecurityContext(runAsUser int64, runAsGroup int64) *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsUser:  &runAsUser,
		RunAsGroup: &runAsGroup,
	}
}

func CreatePodSecurityContext(runAsUser, runAsGroup int64) *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsUser:  &runAsUser,
		RunAsGroup: &runAsGroup,
	}
}

func CreateContainerPorts(ctx context.Context, cubridDB *cubridv1.CubridDB) []corev1.ContainerPort {
	containerPorts := make([]corev1.ContainerPort, 0, len(cubridDB.Spec.Broker))

	for _, br := range cubridDB.Spec.Broker {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			ContainerPort: br.Port,
			Name:          br.Name,
			Protocol:      corev1.ProtocolTCP,
		})
	}
	return containerPorts
}

func CreateVolumeMountsForCubridDB(cubridDB *cubridv1.CubridDB) []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0, len(cubridDB.Spec.Storage))

	for _, storage := range cubridDB.Spec.Storage {
		volumeMount := corev1.VolumeMount{
			Name:      storage.Name,
			MountPath: storage.MountPath,
		}
		volumeMounts = append(volumeMounts, volumeMount)
	}

	return volumeMounts
}

func CreateAffinity(cubridDB *cubridv1.CubridDB) *corev1.Affinity {
	if !cubridDB.Spec.Affinity.EnableAntiAffinity {
		return nil
	}

	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "app",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{cubridDB.Name},
							},
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}
}
