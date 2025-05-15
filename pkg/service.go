package pkg

import (
	"context"
	"fmt"

	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// ValidateNodePort validates if the given port is within the NodePort range.
func ValidateNodePort(port int32) error {
	if port < DEF.NodePortRangeMin || port > DEF.NodePortRangeMax {
		return fmt.Errorf("port %d is not within the valid NodePort range (%d-%d)", port, DEF.NodePortRangeMin, DEF.NodePortRangeMax)
	}
	return nil
}

// ValidateNodePortRange validates if a range of ports is within the valid NodePort range
func ValidateNodePortRange(startPort int32, count int32) error {
	if err := ValidateNodePort(startPort); err != nil {
		return err
	}

	endPort := startPort + count - 1
	if endPort > DEF.NodePortRangeMax {
		return fmt.Errorf("port range %d-%d exceeds the maximum NodePort value %d", startPort, endPort, DEF.NodePortRangeMax)
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

// IsPortInUse checks if the given port is already in use by any service in any namespace.
func IsPortInUse(ctx context.Context, client client.Client, port int32, excludeServiceName string) (bool, error) {
	services := &corev1.ServiceList{}
	err := client.List(ctx, services)
	if err != nil {
		return false, fmt.Errorf("error listing services: %v", err)
	}

	for _, service := range services.Items {
		// Skip the service that we want to exclude
		if service.Name == excludeServiceName {
			continue
		}
		for _, servicePort := range service.Spec.Ports {
			if servicePort.NodePort == port {
				return true, nil
			}
		}
	}

	return false, nil
}

// FindNextAvailablePort finds the next available port starting from the given port.
// It checks if the port is already in use by any service in any namespace.
// If the specified port is available, it returns that port.
// Otherwise, it returns the next available port.
func FindNextAvailablePort(ctx context.Context, client client.Client, startPort int32, excludeServiceName string) (int32, error) {
	// First validate if the start port is within the NodePort range
	if err := ValidateNodePort(startPort); err != nil {
		return 0, err
	}

	// Get all services in all namespaces
	services := &corev1.ServiceList{}
	err := client.List(ctx, services)
	if err != nil {
		return 0, fmt.Errorf("error listing services: %v", err)
	}

	// Create a map of used ports
	usedPorts := make(map[int32]bool)
	for _, service := range services.Items {
		// Skip the service that we want to exclude
		if service.Name == excludeServiceName {
			continue
		}
		for _, port := range service.Spec.Ports {
			if port.NodePort != 0 {
				usedPorts[port.NodePort] = true
			}
		}
	}

	// Check if the start port is available
	if !usedPorts[startPort] {
		return startPort, nil
	}

	// Find the next available port
	port := startPort + 1
	for port <= DEF.NodePortRangeMax && usedPorts[port] {
		port++
	}

	// Check if we found a valid port
	if port > DEF.NodePortRangeMax {
		return 0, fmt.Errorf("no available ports found in the NodePort range (%d-%d)", DEF.NodePortRangeMin, DEF.NodePortRangeMax)
	}

	return port, nil
}
