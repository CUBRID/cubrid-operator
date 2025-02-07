package pkg

import (
	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func CreateService(
	svcName string,
	svcNamespace string,
	svcSelector string,
	svcType corev1.ServiceType,
	ports []corev1.ServicePort) *corev1.Service {

	label := map[string]string{"app": svcSelector}

	return &corev1.Service{
		ObjectMeta: NewObjectMeta(svcName, svcNamespace, label, label),
		Spec:       CreateServiceSpec(svcSelector, svcType, ports),
	}
}

func CreateServiceSpec(svcSelector string, svcType corev1.ServiceType, ports []corev1.ServicePort) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Type:     svcType,
		Selector: map[string]string{"app": svcSelector},
		Ports:    ports,
	}
}

// Setting up one port in SVC
func CreatePort(
	name string,
	servicePort int32,
	targetPort int32,
	protocol corev1.Protocol,
	serviceType corev1.ServiceType) []corev1.ServicePort {

	ports := []corev1.ServicePort{
		buildServicePort(name, servicePort, targetPort, protocol),
	}

	if serviceType == corev1.ServiceTypeNodePort {
		buildNodePort(&ports[0], servicePort)
	}

	return ports
}

// Setting up multiple ports in SVC
func CreateNodePorts(cubridDB *cubridv1.CubridDB) []corev1.ServicePort {
	servicePorts := make([]corev1.ServicePort, 0, len(cubridDB.Spec.Broker))

	for _, svc := range cubridDB.Spec.Broker {
		servicePorts = append(servicePorts, corev1.ServicePort{
			Name:       svc.Name,
			Protocol:   corev1.ProtocolTCP,
			Port:       svc.ServicePort,
			TargetPort: intstr.FromInt32(svc.Port),
			NodePort:   svc.ServicePort,
		})
	}

	return servicePorts
}

func buildServicePort(name string, servicePort int32, targetPort int32, protocol corev1.Protocol) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       name,
		Protocol:   protocol,
		Port:       servicePort,
		TargetPort: intstr.FromInt32(targetPort),
	}
}

func buildNodePort(servicePort *corev1.ServicePort, nodePort int32) {
	servicePort.NodePort = nodePort
}

func GetServiceType(svcType string) string {
	var serviceType string
	if svcType == "ClusterIP" {
		serviceType = string(corev1.ServiceTypeClusterIP)
	} else if svcType == "NodePort" {
		serviceType = string(corev1.ServiceTypeNodePort)
	} else {
		serviceType = "None"
	}
	return serviceType
}
