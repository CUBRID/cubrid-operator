package manager

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	var errs []error
	
	// Reconcile CMS services
	if err := m.reconcileCMSServices(ctx, cubridDB); err != nil {
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

	// If any errors occurred, return them as a single error
	if len(errs) > 0 {
		return fmt.Errorf("service reconciliation errors: %v", errs)
	}

	return nil
}

// reconcileHeadlessService creates or updates the headless service for StatefulSet
func (m *ServiceManager) reconcileHeadlessService(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	svclogger.V(1).Info("Start reconcileHeadlessService()")
	// Define service ports for headless service (HA port only)
	ports := []corev1.ServicePort{
		res.CreateServicePort(
			DEF.SVC_HEADLESS_PORT_NAME,
			DEF.SVC_HA_PORT_ID,
			DEF.SVC_HA_PORT_ID,
			0, // No NodePort needed for headless service
			corev1.ProtocolTCP,
		),
	}

	// Create service metadata
	meta := res.CreateServiceMeta(
		res.CreateHeadlessServiceName(cubridDB.Name),
		cubridDB.Namespace,
		res.CreateServiceLabels(cubridDB.Name, DEF.SVC_NAME_SUFFIX, nil),
	)

	// Create service spec
	spec := res.CreateServiceSpec(
		corev1.ServiceTypeClusterIP,
		ports,
		res.CreateHeadlessServiceSelector(cubridDB.Name),
	)

	// Create headless service
	headlessSvc := &corev1.Service{
		ObjectMeta: meta,
		Spec:       spec,
	}
	headlessSvc.Spec.ClusterIP = "None" // Make it a headless service

	// Set owner reference
	if err := controllerutil.SetControllerReference(cubridDB, headlessSvc, m.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %v", err)
	}

	// Create or update service
	if err := m.createOrUpdateService(ctx, headlessSvc); err != nil {
		return err
	}

	return nil
}

// reconcileCMSServices manages CMS NodePort services
func (m *ServiceManager) reconcileCMSServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	if !isCMSEnabled(cubridDB) {
		return m.cleanupServices(ctx, cubridDB, res.ServiceTypeCMS)
	}

	// Get the initial start port once
	startNodePort := getCMSStartPort(cubridDB)
	cmsPort := getCMSPort(cubridDB)

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

		serviceName := fmt.Sprintf("%s-cms-%d", cubridDB.Name, i)

		// Check if service already exists
		existingService := &corev1.Service{}
		err = m.Get(ctx, types.NamespacedName{
			Name:      serviceName,
			Namespace: cubridDB.Namespace,
		}, existingService)
		if err == nil {
			// Service exists, use its existing NodePort
			if len(existingService.Spec.Ports) > 0 {
				startNodePort = existingService.Spec.Ports[0].NodePort
			}
		} else if !errors.IsNotFound(err) {
			return fmt.Errorf("error checking existing service %s: %v", serviceName, err)
		} else {
			// Service doesn't exist, check if the port is available
			inUse, err := res.IsPortInUse(ctx, m.Client, startNodePort, serviceName)
			if err != nil {
				return fmt.Errorf("error checking port availability for CMS service %s: %v", serviceName, err)
			}

			// If port is in use, find the next available port
			if inUse {
				startNodePort, err = res.FindNextAvailablePort(ctx, m.Client, startNodePort, serviceName)
				if err != nil {
					return fmt.Errorf("error finding available port for CMS service %s: %v", serviceName, err)
				}
			}
		}

		// Create service ports
		ports := []corev1.ServicePort{
			res.CreateServicePort(
				"cms",
				cmsPort,
				cmsPort,
				startNodePort,
				corev1.ProtocolTCP,
			),
		}

		// Create service
		service := &corev1.Service{
			ObjectMeta: res.CreateServiceMeta(
				serviceName,
				cubridDB.Namespace,
				res.CreateServiceLabels(cubridDB.Name, res.ServiceTypeCMS, nil),
			),
			Spec: res.CreateServiceSpec(
				corev1.ServiceTypeNodePort,
				ports,
				res.CreateCMSSelector(podName),
			),
		}

		// Set Pod as the owner of the service with blockOwnerDeletion set to false
		util.SetOwnerReference(pod, service)

		// Create or update service to allow user modifications
		if err := m.createOrUpdateService(ctx, service); err != nil {
			return fmt.Errorf("failed to create/update CMS service %s: %v", serviceName, err)
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

		// Create service
		svc := &corev1.Service{
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
				res.CreateBrokerSelector(cubridDB.Name),
			),
		}

		// Set StatefulSet as the owner of the service
		if err := controllerutil.SetControllerReference(cubridDB, svc, m.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference for broker service %s: %v", broker.Name, err)
		}

		if err := m.createOrUpdateService(ctx, svc); err != nil {
			return fmt.Errorf("failed to reconcile broker service %s: %v", broker.Name, err)
		}
	}

	return nil
}

func (m *ServiceManager) createOrUpdateService(ctx context.Context, svc *corev1.Service) error {
	svclogger.V(1).Info("Create or Update service", "service name", svc.Name)
	existing := &corev1.Service{}
	err := m.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			return m.Create(ctx, svc)
		}
		return err
	}

	// Check if there are any changes
	needsUpdate := false

	// Compare Spec
	if existing.Spec.Type != svc.Spec.Type {
		needsUpdate = true
	} else if !reflect.DeepEqual(existing.Spec.Ports, svc.Spec.Ports) {
		needsUpdate = true
	} else if !reflect.DeepEqual(existing.Spec.Selector, svc.Spec.Selector) {
		needsUpdate = true
	}

	// Compare Labels
	if !reflect.DeepEqual(existing.Labels, svc.Labels) {
		needsUpdate = true
	}

	// Only update if there are changes
	if needsUpdate {
		existing.Spec = svc.Spec
		existing.Labels = svc.Labels
		return m.Update(ctx, existing)
	}

	return nil
}

func (m *ServiceManager) cleanupServices(ctx context.Context, cubridDB *cubridv1.CubridDB, serviceType res.ServiceType) error {
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

func getCMSPort(cubridDB *cubridv1.CubridDB) int32 {
	if cubridDB.Spec.CMSService != nil && cubridDB.Spec.CMSService.Port != nil {
		return *cubridDB.Spec.CMSService.Port
	}
	return DEF.SVC_CMS_PORT
}
