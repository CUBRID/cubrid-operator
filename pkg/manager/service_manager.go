package manager

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	res "github.com/cubrid/cubrid-operator/pkg/resources"
	"github.com/cubrid/cubrid-operator/pkg/util"
)

type ServiceManager struct {
	client.Client
	Scheme *runtime.Scheme
}

var (
	svclogger = log.Log.WithName("Service")
)

func NewServiceManager(client client.Client, scheme *runtime.Scheme) *ServiceManager {
	return &ServiceManager{
		Client: client,
		Scheme: scheme,
	}
}

// ReconcileServices reconciles all services
func (m *ServiceManager) ReconcileServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	// Reconcile CMS services creates or updates CMS services for CUBRID Manager Server
	// Reconcile Broker services creates or updates broker services for CUBRID Broker
	// Reconcile Headless service creates or updates headless service for HA mode
	// Reconcile Ingress CMS services creates or updates services for Ingress backend
	var errs []error

	// Reconcile CMS services
	if err := m.reconcileNodePortCMSServices(ctx, cubridDB); err != nil {
		errs = append(errs, fmt.Errorf("failed to reconcile CMS services: %v", err))
	}

	// Reconcile Broker services
	if err := m.reconcileBrokerServices(ctx, cubridDB); err != nil {
		errs = append(errs, fmt.Errorf("failed to reconcile broker services: %v", err))
	}

	// Reconcile Headless service if HA is enabled
	if cubridDB.IsHAEnabled() {
		if err := m.reconcileHeadlessService(ctx, cubridDB); err != nil {
			errs = append(errs, fmt.Errorf("failed to reconcile headless service: %v", err))
		}
	}

	// Reconcile Ingress CMS services
	if err := m.reconcileAllIngressCMSServices(ctx, cubridDB); err != nil {
		errs = append(errs, fmt.Errorf("failed to reconcile ingress CMS services: %v", err))
	}

	// If any errors occurred, return them as a single error
	if len(errs) > 0 {
		return fmt.Errorf("service reconciliation errors: %v", errs)
	}

	return nil
}

// ServiceManagementType defines how a service should be managed
type ServiceManagementType string

const (
	// ServiceManagedByCR means the service is managed through CR spec
	ServiceManagedByCR ServiceManagementType = "CR"
	// ServiceManagedByUser means the service is created by operator but managed by user
	ServiceManagedByUser ServiceManagementType = "USER"
)

// getServiceManagementType determines how a service should be managed
func getServiceManagementType(serviceName string, cubridDB *cubridv1.CubridDB) ServiceManagementType {
	if isBrokerService(serviceName, cubridDB) {
		return ServiceManagedByCR
	}

	// Headless service is managed by CR
	if isHeadlessService(serviceName, cubridDB) {
		return ServiceManagedByCR
	}

	// Ingress CMS services are managed by CR
	if isIngressCMSService(serviceName) {
		return ServiceManagedByCR
	}

	// NodePort CMS services are managed by CR
	if isNodePortCMSService(serviceName, cubridDB) {
		return ServiceManagedByCR
	}

	return ServiceManagedByUser
}

// isBrokerService checks if the service is a broker service
func isBrokerService(serviceName string, cubridDB *cubridv1.CubridDB) bool {
	for _, broker := range cubridDB.Spec.Broker {
		if broker.Name == serviceName {
			return true
		}
	}
	return false
}

// isHeadlessService checks if the service is a headless service
func isHeadlessService(serviceName string, cubridDB *cubridv1.CubridDB) bool {
	return serviceName == res.CreateHeadlessServiceName(cubridDB.Name)
}

// isIngressCMSService checks if the service is an ingress CMS service
func isIngressCMSService(serviceName string) bool {
	// Check if service name ends with "-cms-svc" pattern
	return strings.HasSuffix(serviceName, "-cms-svc")
}

// isNodePortCMSService checks if the service is a NodePort CMS service
func isNodePortCMSService(serviceName string, cubridDB *cubridv1.CubridDB) bool {
	return strings.HasPrefix(serviceName, fmt.Sprintf("%s-%s-cms", cubridDB.Name, cubridDB.Namespace))
}

// getServiceModificationGuide returns guidance for users on how to modify a service
func getServiceModificationGuide(serviceName string, cubridDB *cubridv1.CubridDB) string {
	managementType := getServiceManagementType(serviceName, cubridDB)

	switch managementType {
	case ServiceManagedByCR:
		return fmt.Sprintf("Service '%s' is managed by CR. Use 'kubectl edit cubriddb %s' to modify this service.",
			serviceName, cubridDB.Name)
	case ServiceManagedByUser:
		return fmt.Sprintf("Service '%s' can be modified directly. Use 'kubectl edit svc %s' to modify this service.",
			serviceName, serviceName)
	default:
		return fmt.Sprintf("Service '%s' management type is unknown.", serviceName)
	}
}

// protectCRManagedService ensures CR-managed services are not modified directly by user
// If user modifies a CR-managed service directly, it will be restored to CR state
func (m *ServiceManager) protectCRManagedService(
	ctx context.Context,
	serviceName string,
	cubridDB *cubridv1.CubridDB,
	desiredSvc *corev1.Service,
) error {
	managementType := getServiceManagementType(serviceName, cubridDB)

	if managementType == ServiceManagedByUser {
		// User-managed services can be modified directly
		return nil
	}

	// For CR-managed services, check if they have been modified by user
	existingSvc := &corev1.Service{}
	err := m.Get(ctx, types.NamespacedName{
		Name:      serviceName,
		Namespace: cubridDB.Namespace,
	}, existingSvc)

	if err != nil {
		if errors.IsNotFound(err) {
			// Service doesn't exist, create it
			return m.Create(ctx, desiredSvc)
		}
		return err
	}

	// Check if service has been modified by user (not matching CR spec)
	needsRestore := false

	// Check if service type matches CR spec
	if existingSvc.Spec.Type != desiredSvc.Spec.Type {
		needsRestore = true
		svclogger.Info("Service type modified by user, restoring to CR state",
			"service", serviceName, "current", existingSvc.Spec.Type, "desired", desiredSvc.Spec.Type)
	}

	// Check if ports match CR spec
	if !reflect.DeepEqual(existingSvc.Spec.Ports, desiredSvc.Spec.Ports) {
		needsRestore = true
		svclogger.Info("Service ports modified by user, restoring to CR state",
			"service", serviceName)
	}

	// Check if selector matches CR spec
	if !reflect.DeepEqual(existingSvc.Spec.Selector, desiredSvc.Spec.Selector) {
		needsRestore = true
		svclogger.Info("Service selector modified by user, restoring to CR state",
			"service", serviceName)
	}

	// Check if labels match CR spec
	if !reflect.DeepEqual(existingSvc.Labels, desiredSvc.Labels) {
		needsRestore = true
		svclogger.Info("Service labels modified by user, restoring to CR state",
			"service", serviceName)
	}

	// Restore service to CR state if needed
	if needsRestore {
		existingSvc.Spec = desiredSvc.Spec
		existingSvc.Labels = desiredSvc.Labels

		if err := m.Update(ctx, existingSvc); err != nil {
			return fmt.Errorf("failed to restore CR-managed service %s: %v", serviceName, err)
		}

		guide := getServiceModificationGuide(serviceName, cubridDB)
		svclogger.Info("Restored CR-managed service to CR state",
			"service", serviceName,
			"note", guide)
	}

	return nil
}

// reconcileHeadlessService creates or updates the headless service for StatefulSet
func (m *ServiceManager) reconcileHeadlessService(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	svclogger.V(1).Info("Start reconcileHeadlessService()")

	serviceName := res.CreateHeadlessServiceName(cubridDB.Name)

	// Get headless service port from CR spec
	headlessPort := cubridDB.GetHAPort()

	// Define desired service ports for headless service (HA port only)
	desiredPorts := []corev1.ServicePort{
		res.CreateServicePort(
			DEF.SVC_HEADLESS_PORT_NAME,
			headlessPort,
			headlessPort,
			0, // No NodePort needed for headless service
			corev1.ProtocolTCP,
		),
	}

	// Create desired service metadata
	desiredMeta := res.CreateServiceMeta(
		serviceName,
		cubridDB.Namespace,
		res.CreateServiceLabels(cubridDB.Name, DEF.SVC_NAME_SUFFIX, nil),
	)

	// Create desired service spec
	desiredSpec := res.CreateServiceSpec(
		corev1.ServiceTypeClusterIP,
		desiredPorts,
		res.CreateHeadlessServiceSelector(cubridDB.Name),
	)
	desiredSpec.ClusterIP = "None" // Make it a headless service

	// Create desired headless service
	desiredSvc := &corev1.Service{
		ObjectMeta: desiredMeta,
		Spec:       desiredSpec,
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(cubridDB, desiredSvc, m.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %v", err)
	}

	// Use protection logic for CR-managed services
	return m.protectCRManagedService(ctx, serviceName, cubridDB, desiredSvc)
}

// reconcileNodePortService handles the common pattern for NodePort services
// It includes special logic for NodePort management
func (m *ServiceManager) reconcileNodePortService(
	ctx context.Context,
	serviceName, namespace string,
	desiredSvc *corev1.Service,
	startNodePort int32,
) error {
	// Check if service exists and get its current state
	existingSvc := &corev1.Service{}
	err := m.Get(ctx, types.NamespacedName{
		Name:      serviceName,
		Namespace: namespace,
	}, existingSvc)

	// If service doesn't exist, create it
	if err != nil {
		if errors.IsNotFound(err) {
			// Check if the port is available
			inUse, err := res.IsPortInUse(ctx, m.Client, startNodePort, serviceName)
			if err != nil {
				return fmt.Errorf("error checking port availability for service %s: %v", serviceName, err)
			}

			// If port is in use, find the next available port
			if inUse {
				startNodePort, err = res.FindNextAvailablePort(ctx, m.Client, startNodePort, serviceName)
				if err != nil {
					return fmt.Errorf("error finding available port for service %s: %v", serviceName, err)
				}
			}

			// Update the desired service with the correct NodePort
			if len(desiredSvc.Spec.Ports) > 0 {
				desiredSvc.Spec.Ports[0].NodePort = startNodePort
			}

			if err := m.Create(ctx, desiredSvc); err != nil {
				return fmt.Errorf("failed to create service %s: %v", serviceName, err)
			}
			svclogger.V(1).Info("Created service", "name", serviceName, "nodePort", startNodePort)
			return nil
		}
		return fmt.Errorf("error checking service %s: %v", serviceName, err)
	}

	// Service exists, use its existing NodePort
	if len(existingSvc.Spec.Ports) > 0 {
		startNodePort = existingSvc.Spec.Ports[0].NodePort
	}

	// Update the desired service with the existing NodePort
	if len(desiredSvc.Spec.Ports) > 0 {
		desiredSvc.Spec.Ports[0].NodePort = startNodePort
	}

	// Check if service needs update
	needsUpdate := false

	// Check service type
	if existingSvc.Spec.Type != desiredSvc.Spec.Type {
		needsUpdate = true
	}

	// Check ports
	if !reflect.DeepEqual(existingSvc.Spec.Ports, desiredSvc.Spec.Ports) {
		needsUpdate = true
	}

	// Check selector
	if !reflect.DeepEqual(existingSvc.Spec.Selector, desiredSvc.Spec.Selector) {
		needsUpdate = true
	}

	// Check labels
	if !reflect.DeepEqual(existingSvc.Labels, desiredSvc.Labels) {
		needsUpdate = true
	}

	// Only update if needed
	if needsUpdate {
		existingSvc.Spec = desiredSvc.Spec
		existingSvc.Labels = desiredSvc.Labels

		if err := m.Update(ctx, existingSvc); err != nil {
			return fmt.Errorf("failed to update service %s: %v", serviceName, err)
		}
		svclogger.V(1).Info("Updated service", "name", serviceName, "nodePort", startNodePort)
	}

	return nil
}

// reconcileNodePortCMSServices  manages CMS NodePort services
func (m *ServiceManager) reconcileNodePortCMSServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	if !isCMSEnabled(cubridDB) {
		return m.cleanupServices(ctx, cubridDB, res.ServiceTypeCMS)
	}

	// Check if nginx ingress controller exists
	exists, err := util.IsNginxIngressControllerExists(ctx, m.Client)
	if err != nil {
		return fmt.Errorf("error checking ingress controller: %v", err)
	}

	// If nginx ingress controller exists, clean up NodePort services
	if exists {
		// svclogger.Info("nginx ingress controller found, cleaning up NodePort services")
		// Get all pods for this CubridDB
		podList := &corev1.PodList{}
		if err := m.List(
			ctx, podList,
			client.InNamespace(cubridDB.Namespace),
			client.MatchingLabels{"app": cubridDB.Name},
		); err != nil {
			return fmt.Errorf("failed to list pods: %v", err)
		}

		// Delete NodePort service for each pod
		for i := 0; i < int(cubridDB.Spec.Replication.Replicas); i++ {
			serviceName := fmt.Sprintf(DEF.SVC_CMS_NODEPORT, cubridDB.Name, cubridDB.Namespace, i)
			service := &corev1.Service{}
			err := m.Get(ctx, types.NamespacedName{
				Name:      serviceName,
				Namespace: cubridDB.Namespace,
			}, service)
			if err != nil {
				if errors.IsNotFound(err) {
					continue // Service doesn't exist, skip
				}
				return fmt.Errorf("error getting service: %v", err)
			}
			if err := m.Delete(ctx, service); err != nil {
				return fmt.Errorf("error deleting service: %v", err)
			} else {
				svclogger.V(1).Info("Deleted service", "name", serviceName)
			}
		}
		return nil
	}

	// Get the initial start port once
	startNodePort := getCMSStartPort(cubridDB)
	cmsPort := cubridDB.GetCMSPort()

	// Validate the initial start port
	if err := res.ValidateNodePort(startNodePort); err != nil {
		return fmt.Errorf("invalid initial CMS start port: %v", err)
	}

	// Create CMS service for connection for each pod in Cubrid Admin
	for i := 0; i < int(cubridDB.Spec.Replication.Replicas); i++ {
		podName := fmt.Sprintf("%s-%d", cubridDB.Name, i)

		// Check if Pod exists
		pod := &corev1.Pod{}
		err := m.Get(ctx, types.NamespacedName{
			Name:      podName,
			Namespace: cubridDB.Namespace,
		}, pod)
		if err != nil {
			if errors.IsNotFound(err) {
				// Skip service creation if Pod does not exist
				continue
			}
			return err
		}

		serviceName := fmt.Sprintf(DEF.SVC_CMS_NODEPORT, cubridDB.Name, cubridDB.Namespace, i)

		// Define desired service ports
		desiredPorts := []corev1.ServicePort{
			res.CreateServicePort(
				"cms",
				cmsPort,
				cmsPort,
				startNodePort,
				corev1.ProtocolTCP,
			),
		}

		// Define desired service metadata
		desiredMeta := res.CreateServiceMeta(
			serviceName,
			cubridDB.Namespace,
			res.CreateServiceLabels(cubridDB.Name, res.ServiceTypeCMS, nil),
		)

		// Define desired service spec
		desiredSpec := res.CreateServiceSpec(
			corev1.ServiceTypeNodePort,
			desiredPorts,
			res.CreateCMSSelector(podName),
		)

		// Create desired service
		desiredSvc := &corev1.Service{
			ObjectMeta: desiredMeta,
			Spec:       desiredSpec,
		}

		// Set Pod as the owner of the service with blockOwnerDeletion set to false
		util.SetOwnerReference(pod, desiredSvc)

		// Use the helper function to reconcile the NodePort service
		if err := m.reconcileNodePortService(ctx, serviceName, cubridDB.Namespace, desiredSvc, startNodePort); err != nil {
			return fmt.Errorf("failed to reconcile CMS service %s: %v", serviceName, err)
		}

		// Increment the port for the next service
		startNodePort++
	}

	return nil
}

// reconcileBrokerServices manages Broker services
func (m *ServiceManager) reconcileBrokerServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	if len(cubridDB.Spec.Broker) == 0 {
		return m.cleanupServices(ctx, cubridDB, res.ServiceTypeBroker)
	}

	for _, broker := range cubridDB.Spec.Broker {
		if broker.ServiceType != corev1.ServiceTypeNodePort {
			continue
		}

		// Create service port
		servicePort := res.CreateServicePort(broker.Name, broker.Port, broker.Port, broker.ServicePort, corev1.ProtocolTCP)

		// Create desired service
		desiredSvc := &corev1.Service{
			ObjectMeta: res.CreateServiceMeta(
				broker.Name,
				cubridDB.Namespace,
				res.CreateServiceLabels(
					cubridDB.Name,
					res.ServiceTypeBroker,
					map[string]string{"broker": broker.Name},
				),
			),
			Spec: res.CreateServiceSpec(
				broker.ServiceType,
				[]corev1.ServicePort{servicePort},
				nil,
			),
		}

		// Set StatefulSet as the owner of the service
		if err := controllerutil.SetControllerReference(cubridDB, desiredSvc, m.Scheme); err != nil {
			return fmt.Errorf(
				"failed to set controller reference for broker service %s: %v",
				broker.Name,
				err,
			)
		}

		// Use protection logic for CR-managed services
		if err := m.protectCRManagedService(ctx, broker.Name, cubridDB, desiredSvc); err != nil {
			return fmt.Errorf("failed to reconcile broker service %s: %v", broker.Name, err)
		}
	}

	return nil
}

func (m *ServiceManager) cleanupServices(
	ctx context.Context,
	cubridDB *cubridv1.CubridDB,
	serviceType res.ServiceType,
) error {
	services := &corev1.ServiceList{}
	if err := m.List(ctx, services,
		client.InNamespace(cubridDB.Namespace),
		client.MatchingLabels{
			"app":     cubridDB.Name,
			"service": string(serviceType),
		}); err != nil {
		return err
	}

	for _, svc := range services.Items {
		if err := m.Delete(ctx, &svc); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// reconcileAllIngressCMSServices manages all Ingress CMS services for the CubridDB
func (m *ServiceManager) reconcileAllIngressCMSServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	svclogger.V(1).Info("Start reconcileAllIngressCMSServices", "cubridDB", cubridDB.Name)

	// Check if nginx ingress controller exists
	exists, err := util.IsNginxIngressControllerExists(ctx, m.Client)
	if err != nil {
		return fmt.Errorf("error checking ingress controller: %v", err)
	}

	// If nginx ingress controller doesn't exist, clean up all ingress CMS services
	if !exists {
		svclogger.Info("nginx ingress controller not found, cleaning up ingress CMS services")
		return m.cleanupIngressCMSServices(ctx, cubridDB)
	}

	// Create services based on replicas count (not existing pods)
	// This ensures consistent service structure regardless of pod lifecycle
	for i := 0; i < int(cubridDB.Spec.Replication.Replicas); i++ {
		podName := fmt.Sprintf("%s-%d", cubridDB.Name, i)

		// Create service for this replica index
		if err := m.createIngressCMSServiceForReplica(ctx, cubridDB, podName); err != nil {
			return fmt.Errorf("failed to create ingress CMS service for replica %d: %v", i, err)
		}
	}

	// Clean up orphaned services (services for replicas that no longer exist)
	if err := m.cleanupOrphanedIngressCMSServices(ctx, cubridDB); err != nil {
		return fmt.Errorf("failed to cleanup orphaned ingress CMS services: %v", err)
	}

	return nil
}

// cleanupIngressCMSServices removes all ingress CMS services for the CubridDB
func (m *ServiceManager) cleanupIngressCMSServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	svclogger.V(1).Info("Cleaning up ingress CMS services", "cubridDB", cubridDB.Name)

	// List all services in the namespace
	serviceList := &corev1.ServiceList{}
	if err := m.List(ctx, serviceList, client.InNamespace(cubridDB.Namespace)); err != nil {
		return fmt.Errorf("failed to list services: %v", err)
	}

	// Delete services that match the ingress CMS service pattern and belong to this CubridDB
	for _, service := range serviceList.Items {
		if isIngressCMSService(service.Name) {
			// Extract pod name from service name
			// Service name format: {pod-name}-{namespace}-cms-svc
			// Example: my-cubrid-0-default-cms-svc
			parts := strings.Split(service.Name, "-")
			if len(parts) >= 4 {
				// Find the namespace part (second to last)
				namespaceIndex := len(parts) - 3
				if namespaceIndex > 0 {
					// Pod name is everything before the namespace part
					podName := strings.Join(parts[:namespaceIndex], "-")

					// Check if this service belongs to our CubridDB by checking owner references
					if hasPodOwnerReference(&service, podName) {
						if err := m.Delete(ctx, &service); err != nil {
							if !errors.IsNotFound(err) {
								return fmt.Errorf("failed to delete ingress CMS service %s: %v", service.Name, err)
							}
						} else {
							svclogger.V(1).Info("Deleted ingress CMS service", "name", service.Name)
						}
					}
				}
			}
		}
	}

	return nil
}

// createIngressCMSServiceForReplica creates an Ingress CMS service for a specific replica
func (m *ServiceManager) createIngressCMSServiceForReplica(
	ctx context.Context,
	cubridDB *cubridv1.CubridDB,
	podName string,
) error {
	svclogger.V(1).Info("Creating ingress CMS service for replica", "cubridDB", cubridDB.Name, "podName", podName)

	// Create CMS service for the replica
	serviceName := fmt.Sprintf(DEF.INGRESS_CMS_SVC_NAME, podName, cubridDB.Namespace)

	// Get CMS port from CR spec (unified port for all CMS services)
	cmsPort := cubridDB.GetCMSPort()

	// Define desired service metadata
	desiredMeta := metav1.ObjectMeta{
		Name:      serviceName,
		Namespace: cubridDB.Namespace,
		Labels: map[string]string{
			"app": "cubrid",
		},
	}

	// Define desired service spec
	desiredSpec := corev1.ServiceSpec{
		Type:      corev1.ServiceTypeClusterIP, // Explicitly set service type
		ClusterIP: "None",                      // Headless service
		Selector: map[string]string{
			"statefulset.kubernetes.io/pod-name": podName,
		},
		Ports: []corev1.ServicePort{
			{
				Name:       "cms-port",
				Protocol:   corev1.ProtocolTCP,
				Port:       cmsPort,
				TargetPort: intstr.FromInt(int(cmsPort)),
			},
		},
	}

	// Create desired service
	desiredSvc := &corev1.Service{
		ObjectMeta: desiredMeta,
		Spec:       desiredSpec,
	}

	// Set CubridDB as the owner of the service for automatic cleanup
	if err := controllerutil.SetControllerReference(cubridDB, desiredSvc, m.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference for ingress CMS service %s: %v", serviceName, err)
	}

	// Use protection logic for CR-managed services
	return m.protectCRManagedService(ctx, serviceName, cubridDB, desiredSvc)
}

// cleanupOrphanedIngressCMSServices removes services for replicas that no longer exist
func (m *ServiceManager) cleanupOrphanedIngressCMSServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	svclogger.V(1).Info("Cleaning up orphaned ingress CMS services", "cubridDB", cubridDB.Name)

	// Create a map of expected replica names
	expectedReplicaNames := make(map[string]bool)
	for i := 0; i < int(cubridDB.Spec.Replication.Replicas); i++ {
		podName := fmt.Sprintf("%s-%d", cubridDB.Name, i)
		expectedReplicaNames[podName] = true
	}

	// List all services in the namespace
	serviceList := &corev1.ServiceList{}
	if err := m.List(ctx, serviceList, client.InNamespace(cubridDB.Namespace)); err != nil {
		return fmt.Errorf("failed to list services: %v", err)
	}

	// Check each ingress CMS service
	for _, service := range serviceList.Items {
		if isIngressCMSService(service.Name) {
			// Extract pod name from service name
			// Service name format: {pod-name}-{namespace}-cms-svc
			// Example: my-cubrid-0-default-cms-svc
			parts := strings.Split(service.Name, "-")
			if len(parts) >= 4 {
				// Find the namespace part (second to last)
				namespaceIndex := len(parts) - 3
				if namespaceIndex > 0 {
					// Pod name is everything before the namespace part
					podName := strings.Join(parts[:namespaceIndex], "-")

					// Check if this service belongs to our CubridDB by checking owner references
					if hasOwnerReference(&service, cubridDB) {
						// If replica no longer exists, delete the service
						if !expectedReplicaNames[podName] {
							if err := m.Delete(ctx, &service); err != nil {
								if !errors.IsNotFound(err) {
									return fmt.Errorf("failed to delete orphaned ingress CMS service %s: %v", service.Name, err)
								}
							} else {
								svclogger.V(1).Info("Deleted orphaned ingress CMS service", "name", service.Name, "replica", podName)
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// hasPodOwnerReference checks if a service has a pod as its owner
func hasPodOwnerReference(service *corev1.Service, podName string) bool {
	for _, ownerRef := range service.OwnerReferences {
		if ownerRef.Kind == "Pod" && ownerRef.Name == podName {
			return true
		}
	}
	return false
}

// hasOwnerReference checks if a service has the CubridDB as its owner
func hasOwnerReference(service *corev1.Service, cubridDB *cubridv1.CubridDB) bool {
	for _, ownerRef := range service.OwnerReferences {
		if ownerRef.Kind == "CubridDB" && ownerRef.Name == cubridDB.Name {
			return true
		}
	}
	return false
}

func isCMSEnabled(cubridDB *cubridv1.CubridDB) bool {
	return cubridDB.Spec.CMSService != nil &&
		(cubridDB.Spec.CMSService.Enabled == nil || *cubridDB.Spec.CMSService.Enabled)
}

func getCMSStartPort(cubridDB *cubridv1.CubridDB) int32 {
	if cubridDB.Spec.CMSService != nil && cubridDB.Spec.CMSService.StartPort != nil {
		return *cubridDB.Spec.CMSService.StartPort
	}
	return DEF.SVC_CMS_START_NODE_PORT
}
