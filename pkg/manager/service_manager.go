/*
 * Copyright 2016 CUBRID Corporation
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

package manager

import (
	"context"
	"fmt"
	"reflect"
	"strings"

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

var svclogger = log.Log.WithName("Service")

func NewServiceManager(client client.Client, scheme *runtime.Scheme) *ServiceManager {
	return &ServiceManager{
		Client: client,
		Scheme: scheme,
	}
}

// ReconcileServices reconciles all services
func (m *ServiceManager) ReconcileServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	// Reconcile Broker services creates or updates broker services for CUBRID Broker
	// Reconcile Headless service creates or updates headless service for HA mode
	var errs []error

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
	if aisIngressCMSService(serviceName) {
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

// aisIngressCMSService checks if the service is an ingress CMS service
func aisIngressCMSService(serviceName string) bool {
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

// ReconcileNodePortCMSServices manages NodePort CMS services
func (m *ServiceManager) ReconcileNodePortCMSServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	if !isCMSEnabled(cubridDB) {
		return m.cleanupServices(ctx, cubridDB, res.ServiceTypeCMS)
	}

	serviceType := cubridDB.GetCMSServiceType()
	if serviceType != DEF.CMSServiceTypeNodePort {
		return fmt.Errorf("CMS service type is not NodePort: %s", serviceType)
	}

	return m.reconcileNodePortCMSServices(ctx, cubridDB)
}

// ReconcileIngressCMSServices manages Ingress CMS services
// This method is deprecated. Use IngressManager.ReconcileIngressWithServices instead.
func (m *ServiceManager) ReconcileIngressCMSServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	return fmt.Errorf("ReconcileIngressCMSServices is deprecated. Use IngressManager.ReconcileIngressWithServices instead")
}

// reconcileNodePortCMSServices manages CMS NodePort services
func (m *ServiceManager) reconcileNodePortCMSServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
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

func isCMSEnabled(cubridDB *cubridv1.CubridDB) bool {
	return cubridDB.Spec.CMSService != nil
}

func getCMSStartPort(cubridDB *cubridv1.CubridDB) int32 {
	if cubridDB.Spec.CMSService != nil && cubridDB.Spec.CMSService.StartPort != nil {
		return *cubridDB.Spec.CMSService.StartPort
	}
	return DEF.SVC_CMS_START_NODE_PORT
}
