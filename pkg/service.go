package pkg

import (
	"fmt"

	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Service type constants
const (
	ServiceTypeHeadless = "headless"
)

type ServiceType string

const (
	ServiceTypeCMS    ServiceType = "cms"
	ServiceTypeBroker ServiceType = "broker"
)

// CreateServiceMeta creates ObjectMeta for a service
func CreateServiceMeta(name, namespace string, labels map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    labels,
	}
}

// CreateServicePort creates a ServicePort configuration
func CreateServicePort(name string, port, targetPort, nodePort int32, protocol corev1.Protocol) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       name,
		Protocol:   protocol,
		Port:       port,
		TargetPort: intstr.FromInt32(targetPort),
		NodePort:   nodePort,
	}
}

// CreateServiceSpec creates a ServiceSpec configuration
func CreateServiceSpec(serviceType corev1.ServiceType, ports []corev1.ServicePort, selector map[string]string) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Type:     serviceType,
		Ports:    ports,
		Selector: selector,
	}
}

// CreateCMSSelector creates a selector for CMS service
func CreateCMSSelector(podName string) map[string]string {
	return map[string]string{
		"statefulset.kubernetes.io/pod-name": podName,
	}
}

// CreateBrokerSelector creates a selector for Broker service
func CreateBrokerSelector(appName string) map[string]string {
	return map[string]string{
		"app": appName,
	}
}

// CreateServiceLabels creates labels for a service
func CreateServiceLabels(appName string, serviceType ServiceType, additionalLabels map[string]string) map[string]string {
	labels := map[string]string{
		"app":     appName,
		"service": string(serviceType),
	}
	for k, v := range additionalLabels {
		labels[k] = v
	}
	return labels
}

// ValidateNodePortRange validates the NodePort range
func ValidateNodePortRange(startPort int32, count int32) error {
	if startPort < 30000 || startPort > 32767 {
		return fmt.Errorf("start port must be between 30000 and 32767")
	}
	endPort := startPort + count - 1
	if endPort > 32767 {
		return fmt.Errorf("port range exceeds maximum NodePort value (32767)")
	}
	return nil
}

// CreateHeadlessServicePort creates a service port for headless service
func CreateHeadlessServicePort(name string, port int32, targetPort int32, protocol corev1.Protocol) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       name,
		Port:       port,
		TargetPort: intstr.FromInt32(targetPort),
		Protocol:   protocol,
	}
}

// CreateHeadlessServiceSelector creates a selector for headless service
func CreateHeadlessServiceSelector(cubridDBName string) map[string]string {
	return map[string]string{
		// "app.kubernetes.io/name":     "cubrid",
		"app": cubridDBName,
	}
}

// CreateHeadlessService creates a headless service for StatefulSet
func CreateHeadlessService(cubridDBName string, namespace string, ports []corev1.ServicePort) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: CreateServiceMeta(
			CreateHeadlessServiceName(cubridDBName),
			namespace,
			CreateServiceLabels(cubridDBName, ServiceTypeHeadless, nil),
		),
		Spec: corev1.ServiceSpec{
			ClusterIP: "None", // Headless service
			Ports:     ports,
			Selector:  CreateHeadlessServiceSelector(cubridDBName),
		},
	}
}

func CreateHeadlessServiceName(cubridDBName string) string {
	return fmt.Sprintf("%s-%s", cubridDBName, DEF.SVC_NAME_SUFFIX)
}
