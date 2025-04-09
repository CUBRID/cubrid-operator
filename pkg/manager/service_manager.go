package manager

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	"github.com/cubrid/cubrid-operator/pkg"
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
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

	// Reconcile CMS services
	if err := m.reconcileCMSServices(ctx, cubridDB); err != nil {
		return fmt.Errorf("failed to reconcile CMS services: %v", err)
	}

	// Reconcile Broker services
	if err := m.reconcileBrokerServices(ctx, cubridDB); err != nil {
		return fmt.Errorf("failed to reconcile broker services: %v", err)
	}

	if cubridDB.IsHAEnabled() {
		if err := m.reconcileHeadlessService(ctx, cubridDB); err != nil {
			return fmt.Errorf("failed to reconcile headless service: %v", err)
		}
	}

	return nil
}

// reconcileHeadlessService creates or updates the headless service for StatefulSet
func (m *ServiceManager) reconcileHeadlessService(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	fmt.Println("Start reconcileHeadlessService()")
	// Define service ports for headless service (HA port only)
	ports := []corev1.ServicePort{
		pkg.CreateServicePort(
			DEF.SVC_HEADLESS_PORT_NAME,
			DEF.SVC_HA_PORT,
			DEF.SVC_HA_PORT,
			0, // No NodePort needed for headless service
			corev1.ProtocolTCP,
		),
	}

	// Create service metadata
	meta := pkg.CreateServiceMeta(
		pkg.CreateHeadlessServiceName(cubridDB.Name),
		cubridDB.Namespace,
		pkg.CreateServiceLabels(cubridDB.Name, DEF.SVC_NAME_SUFFIX, nil),
	)

	// Create service spec
	spec := pkg.CreateServiceSpec(
		corev1.ServiceTypeClusterIP,
		ports,
		pkg.CreateHeadlessServiceSelector(cubridDB.Name),
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
		return m.cleanupServices(ctx, cubridDB, pkg.ServiceTypeCMS)
	}

	startPort := getCMSStartPort(cubridDB)
	cmsPort := getCMSPort(cubridDB)

	if err := pkg.ValidateNodePortRange(startPort, cubridDB.Spec.Replication.Replicas); err != nil {
		return err
	}

	for i := int32(0); i < cubridDB.Spec.Replication.Replicas; i++ {
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

		// Create CMS service
		svc := &corev1.Service{
			ObjectMeta: pkg.CreateServiceMeta(
				fmt.Sprintf("%s-cms", podName),
				cubridDB.Namespace,
				pkg.CreateServiceLabels(cubridDB.Name, pkg.ServiceTypeCMS, nil),
			),
			Spec: pkg.CreateServiceSpec(
				corev1.ServiceTypeNodePort,
				[]corev1.ServicePort{
					pkg.CreateServicePort(
						"cms",
						cmsPort,
						cmsPort,
						startPort+i,
						corev1.ProtocolTCP,
					),
				},
				pkg.CreateCMSSelector(podName),
			),
		}

		// Set Pod as the owner of the service
		if err := controllerutil.SetControllerReference(pod, svc, m.Scheme); err != nil {
			return fmt.Errorf("failed to set owner reference: %v", err)
		}

		// Create or update service
		if err := m.createOrUpdateService(ctx, svc); err != nil {
			return err
		}
	}

	return nil
}

// reconcileBrokerServices manages Broker services
func (m *ServiceManager) reconcileBrokerServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	if len(cubridDB.Spec.Broker) == 0 {
		return m.cleanupServices(ctx, cubridDB, pkg.ServiceTypeBroker)
	}

	for _, broker := range cubridDB.Spec.Broker {
		ports := []corev1.ServicePort{
			pkg.CreateServicePort(
				broker.Name,
				broker.Port,
				broker.Port,
				0,
				corev1.ProtocolTCP,
			),
		}

		// Set NodePort if service type is NodePort
		if broker.ServiceType == corev1.ServiceTypeNodePort && broker.ServicePort != 0 {
			ports[0].NodePort = broker.ServicePort
		}

		svc := &corev1.Service{
			ObjectMeta: pkg.CreateServiceMeta(
				broker.Name,
				cubridDB.Namespace,
				pkg.CreateServiceLabels(
					cubridDB.Name,
					pkg.ServiceTypeBroker,
					map[string]string{"broker": broker.Name},
				),
			),
			Spec: pkg.CreateServiceSpec(
				broker.ServiceType,
				ports,
				pkg.CreateBrokerSelector(cubridDB.Name),
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

	return m.cleanupUnusedBrokerServices(ctx, cubridDB)
}

func (m *ServiceManager) createOrUpdateService(ctx context.Context, svc *corev1.Service) error {
	fmt.Println(fmt.Sprintf("Create or Update service : %s", svc.Name))
	existing := &corev1.Service{}
	err := m.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			return m.Create(ctx, svc)
		}
		return err
	}

	existing.Spec = svc.Spec
	existing.Labels = svc.Labels
	return m.Update(ctx, existing)
}

func (m *ServiceManager) cleanupServices(ctx context.Context, cubridDB *cubridv1.CubridDB, serviceType pkg.ServiceType) error {
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

func (m *ServiceManager) cleanupUnusedBrokerServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	configuredServices := make(map[string]bool)
	for _, broker := range cubridDB.Spec.Broker {
		configuredServices[broker.Name] = true
	}

	services := &corev1.ServiceList{}
	if err := m.List(ctx, services,
		client.InNamespace(cubridDB.Namespace),
		client.MatchingLabels{
			"app":     cubridDB.Name,
			"service": string(pkg.ServiceTypeBroker),
		}); err != nil {
		return err
	}

	for _, svc := range services.Items {
		if !configuredServices[svc.Name] {
			if err := m.Delete(ctx, &svc); err != nil && !errors.IsNotFound(err) {
				return err
			}
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
