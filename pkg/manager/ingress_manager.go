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

package manager

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
	"github.com/cubrid/cubrid-operator/pkg/util"
)

var ingresslog = log.Log.WithName("Ingress-Manager")

type IngressManager struct {
	client.Client
	Scheme *runtime.Scheme
}

func NewIngressManager(client client.Client, scheme *runtime.Scheme) *IngressManager {
	return &IngressManager{
		Client: client,
		Scheme: scheme,
	}
}

// ReconcileIngressWithServices manages both Ingress CMS services and Ingress resource
func (m *IngressManager) ReconcileIngressWithServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	ingresslog := log.Log.WithName("ingress_manager")

	// Validate that CMS service type is Ingress
	if !cubridDB.IsCMSEnabled() || cubridDB.GetCMSServiceType() != DEF.CMSServiceTypeIngress {
		ingresslog.Info("CMS service type is not Ingress, cleaning up ingress and services")
		return m.cleanupIngressAndServices(ctx, cubridDB)
	}

	// Check if nginx ingress controller exists
	exists, err := util.IsNginxIngressControllerExists(ctx, m.Client)
	if err != nil {
		return fmt.Errorf("error checking ingress controller: %v", err)
	}

	if !exists {
		ingresslog.Info("nginx ingress controller not found, cleaning up ingress and services")
		return m.cleanupIngressAndServices(ctx, cubridDB)
	}

	// Step 1: Create/Update Ingress CMS services
	if err := m.reconcileIngressCMSServices(ctx, cubridDB); err != nil {
		return fmt.Errorf("failed to reconcile Ingress CMS services: %v", err)
	}

	// Step 2: Create/Update Ingress resource
	if err := m.reconcileIngressResource(ctx, cubridDB); err != nil {
		return fmt.Errorf("failed to reconcile Ingress resource: %v", err)
	}

	return nil
}

// reconcileIngressCMSServices manages all Ingress CMS services for the CubridDB
func (m *IngressManager) reconcileIngressCMSServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	ingresslog.V(1).Info("Start reconcileIngressCMSServices", "cubridDB", cubridDB.Name)

	// Create services based on replicas count (not existing pods)
	// This ensures consistent service structure regardless of pod lifecycle
	for i := 0; i < int(cubridDB.Spec.Replication.Replicas); i++ {
		podName := fmt.Sprintf("%s-%d", cubridDB.Name, i)

		// Create service for this replica index
		if err := m.createIngressCMSService(ctx, cubridDB, podName); err != nil {
			return fmt.Errorf("failed to create ingress CMS service for replica %d: %v", i, err)
		}
	}

	// Clean up orphaned services (services for replicas that no longer exist)
	if err := m.cleanupOrphanedIngressCMSServices(ctx, cubridDB); err != nil {
		return fmt.Errorf("failed to cleanup orphaned ingress CMS services: %v", err)
	}

	return nil
}

// createIngressCMSService creates an Ingress CMS service for a specific replica
func (m *IngressManager) createIngressCMSService(
	ctx context.Context,
	cubridDB *cubridv1.CubridDB,
	podName string,
) error {
	ingresslog.V(1).Info("Creating ingress CMS service for replica", "cubridDB", cubridDB.Name, "podName", podName)

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

	// Create or update service
	return m.createOrUpdateService(ctx, desiredSvc)
}

// createOrUpdateService creates or updates a service
func (m *IngressManager) createOrUpdateService(ctx context.Context, desiredSvc *corev1.Service) error {
	existingSvc := &corev1.Service{}
	err := m.Get(ctx, types.NamespacedName{
		Name:      desiredSvc.Name,
		Namespace: desiredSvc.Namespace,
	}, existingSvc)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new service
			ingresslog.Info("Creating new service", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
			return m.Create(ctx, desiredSvc)
		}
		return fmt.Errorf("error checking service %s: %v", desiredSvc.Name, err)
	}

	// Check if there are any changes
	if reflect.DeepEqual(existingSvc.Spec, desiredSvc.Spec) &&
		reflect.DeepEqual(existingSvc.Labels, desiredSvc.Labels) {
		ingresslog.V(1).Info("Service unchanged, skipping update", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
		return nil
	}

	ingresslog.Info("Updating existing service", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)

	// Update existing service
	existingSvc.Spec = desiredSvc.Spec
	existingSvc.Labels = desiredSvc.Labels

	return m.Update(ctx, existingSvc)
}

// cleanupOrphanedIngressCMSServices removes services for replicas that no longer exist
func (m *IngressManager) cleanupOrphanedIngressCMSServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	ingresslog.V(1).Info("Cleaning up orphaned ingress CMS services", "cubridDB", cubridDB.Name)

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
								ingresslog.V(1).Info("Deleted orphaned ingress CMS service", "name", service.Name, "replica", podName)
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// isIngressCMSService checks if the service is an ingress CMS service
func isIngressCMSService(serviceName string) bool {
	// Check if service name ends with "-cms-svc" pattern
	return strings.HasSuffix(serviceName, "-cms-svc")
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

// reconcileIngressResource creates or updates the Ingress resource for CMS access
func (m *IngressManager) reconcileIngressResource(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	ingresslog.V(1).Info("Start reconcileIngressResource", "cubridDB", cubridDB.Name)

	// Define ingress name at the beginning
	ingressName := fmt.Sprintf(DEF.INGRESS_CMS_INGRESS_NAME, cubridDB.Name, cubridDB.Namespace)

	// Get CMS port from CR spec
	cmsPort := cubridDB.GetCMSPort()

	// Get all pods for this CubridDB
	podList := &corev1.PodList{}
	if err := m.List(
		ctx,
		podList,
		client.InNamespace(cubridDB.Namespace),
		client.MatchingLabels{"app": cubridDB.Name},
	); err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}

	// If no pods exist, don't create Ingress
	if len(podList.Items) == 0 {
		ingresslog.Info("No pods found, skipping Ingress creation", "cubridDB", cubridDB.Name)
		return nil
	}

	// Create initial rules based on existing pods
	ingresslog.V(1).Info("Creating initial rules based on existing pods")
	rules := make([]networkingv1.IngressRule, 0, len(podList.Items))
	for _, pod := range podList.Items {
		hostName := fmt.Sprintf(DEF.INGRESS_HOST_NAME, pod.Name, cubridDB.Namespace)
		serviceName := fmt.Sprintf(DEF.INGRESS_CMS_SVC_NAME, pod.Name, cubridDB.Namespace)

		rule := networkingv1.IngressRule{
			Host: hostName,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: pathTypePtr(networkingv1.PathTypePrefix),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: serviceName,
									Port: networkingv1.ServiceBackendPort{
										Number: cmsPort,
									},
								},
							},
						},
					},
				},
			},
		}
		rules = append(rules, rule)
	}

	// Create Ingress resource
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: cubridDB.Namespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
				"nginx.ingress.kubernetes.io/ssl-passthrough":  "true",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: stringPtr("nginx"), // use nginx ingress controller
			Rules:            rules,
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(cubridDB, ingress, m.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %v", err)
	}

	// Create or update Ingress
	if err := m.createOrUpdateIngress(ctx, ingress); err != nil {
		return fmt.Errorf("failed to create/update Ingress: %v", err)
	}

	return nil
}

// cleanupIngressAndServices deletes the Ingress and its related services
func (m *IngressManager) cleanupIngressAndServices(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	ingresslog := log.Log.WithName("ingress_manager")

	// Clean up Ingress
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(DEF.INGRESS_CMS_INGRESS_NAME, cubridDB.Name, cubridDB.Namespace),
			Namespace: cubridDB.Namespace,
		},
	}
	if err := m.Delete(ctx, ingress); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error deleting ingress: %v", err)
		}
		ingresslog.Info("Ingress already deleted", "name", ingress.Name)
	} else {
		ingresslog.Info("Deleted ingress", "name", ingress.Name)
	}

	// Clean up related services
	podList := &corev1.PodList{}
	if err := m.List(ctx, podList, client.InNamespace(cubridDB.Namespace), client.MatchingLabels(map[string]string{
		"app": cubridDB.Name,
	})); err != nil {
		return fmt.Errorf("error listing pods: %v", err)
	}

	ingresslog.Info("Deleted svc", "num of svc", len(podList.Items))

	for _, pod := range podList.Items {
		serviceName := fmt.Sprintf(DEF.INGRESS_CMS_SVC_NAME, pod.Name, pod.Namespace)
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: pod.Namespace,
			},
		}
		if err := m.Delete(ctx, service); err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("error deleting service: %v", err)
			}
			ingresslog.Info("Service already deleted", "name", serviceName)
		} else {
			ingresslog.Info("Deleted service", "name", serviceName)
		}
	}

	return nil
}

// ReconcileIngress creates or updates the Ingress resource for CMS access
// This method is kept for backward compatibility
func (m *IngressManager) ReconcileIngress(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	ingresslog := log.Log.WithName("ingress_manager")

	// Validate that CMS service type is Ingress
	if !cubridDB.IsCMSEnabled() || cubridDB.GetCMSServiceType() != DEF.CMSServiceTypeIngress {
		ingresslog.Info("CMS service type is not Ingress, cleaning up ingress and services")
		return m.cleanupIngressAndServices(ctx, cubridDB)
	}

	// Check if nginx ingress controller exists
	exists, err := util.IsNginxIngressControllerExists(ctx, m.Client)
	if err != nil {
		return fmt.Errorf("error checking ingress controller: %v", err)
	}

	if !exists {
		ingresslog.Info("nginx ingress controller not found, cleaning up ingress and services")
		return m.cleanupIngressAndServices(ctx, cubridDB)
	}

	// Define ingress name at the beginning
	ingressName := fmt.Sprintf(DEF.INGRESS_CMS_INGRESS_NAME, cubridDB.Name, cubridDB.Namespace)

	// Get CMS port from CR spec
	cmsPort := cubridDB.GetCMSPort()

	// Get all pods for this CubridDB
	podList := &corev1.PodList{}
	if err := m.List(
		ctx,
		podList,
		client.InNamespace(cubridDB.Namespace),
		client.MatchingLabels{"app": cubridDB.Name},
	); err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}

	// If no pods exist, don't create Ingress
	if len(podList.Items) == 0 {
		ingresslog.Info("No pods found, skipping Ingress creation", "cubridDB", cubridDB.Name)
		return nil
	}

	// Create initial rules based on existing pods
	ingresslog.V(1).Info("Creating initial rules based on existing pods")
	rules := make([]networkingv1.IngressRule, 0, len(podList.Items))
	for _, pod := range podList.Items {
		hostName := fmt.Sprintf(DEF.INGRESS_HOST_NAME, pod.Name, cubridDB.Namespace)
		serviceName := fmt.Sprintf(DEF.INGRESS_CMS_SVC_NAME, pod.Name, cubridDB.Namespace)

		rule := networkingv1.IngressRule{
			Host: hostName,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: pathTypePtr(networkingv1.PathTypePrefix),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: serviceName,
									Port: networkingv1.ServiceBackendPort{
										Number: cmsPort,
									},
								},
							},
						},
					},
				},
			},
		}
		rules = append(rules, rule)
	}

	// Create Ingress resource
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: cubridDB.Namespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
				"nginx.ingress.kubernetes.io/ssl-passthrough":  "true",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: stringPtr("nginx"), // use nginx ingress controller
			Rules:            rules,
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(cubridDB, ingress, m.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %v", err)
	}

	// Create or update Ingress
	if err := m.createOrUpdateIngress(ctx, ingress); err != nil {
		return fmt.Errorf("failed to create/update Ingress: %v", err)
	}

	return nil
}

// UpdateIngressRules updates the Ingress rules based on the current pods
func (m *IngressManager) UpdateIngressRules(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	ingresslog.V(1).Info("UpdateIngressRules", "cubridDB", cubridDB.Name)

	ingressName := fmt.Sprintf(DEF.INGRESS_CMS_INGRESS_NAME, cubridDB.Name, cubridDB.Namespace)

	ingress := &networkingv1.Ingress{}
	if err := m.Get(ctx, client.ObjectKey{
		Name:      ingressName,
		Namespace: cubridDB.Namespace,
	}, ingress); err != nil {
		return fmt.Errorf("failed to get Ingress: %v", err)
	}

	// Get CMS port from CR spec
	cmsPort := cubridDB.GetCMSPort()

	// Get all pods for this CubridDB
	podList := &corev1.PodList{}
	if err := m.List(
		ctx,
		podList,
		client.InNamespace(cubridDB.Namespace),
		client.MatchingLabels{"app": cubridDB.Name},
	); err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}

	// Update rules based on pods
	newRules := make([]networkingv1.IngressRule, 0, len(podList.Items))
	for _, pod := range podList.Items {
		hostName := fmt.Sprintf(DEF.INGRESS_HOST_NAME, pod.Name, cubridDB.Namespace)
		serviceName := fmt.Sprintf(DEF.INGRESS_CMS_SVC_NAME, pod.Name, cubridDB.Namespace)

		rule := networkingv1.IngressRule{
			Host: hostName,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: pathTypePtr(networkingv1.PathTypePrefix),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: serviceName,
									Port: networkingv1.ServiceBackendPort{
										Number: cmsPort,
									},
								},
							},
						},
					},
				},
			},
		}
		newRules = append(newRules, rule)
	}

	// Check if rules have changed
	if reflect.DeepEqual(ingress.Spec.Rules, newRules) {
		ingresslog.V(1).Info("Ingress rules unchanged, skipping update", "name", ingressName, "namespace", cubridDB.Namespace)
		return nil
	}

	ingress.Spec.Rules = newRules

	if err := m.Update(ctx, ingress); err != nil {
		return fmt.Errorf("failed to update Ingress rules: %v", err)
	}

	ingresslog.Info("Updated Ingress rules", "name", ingressName, "namespace", cubridDB.Namespace)
	return nil
}

// createOrUpdateIngress creates or updates an Ingress resource
func (m *IngressManager) createOrUpdateIngress(ctx context.Context, ingress *networkingv1.Ingress) error {
	ingresslog.Info("createOrUpdateIngress", "name", ingress.Name, "namespace", ingress.Namespace)
	existing := &networkingv1.Ingress{}
	err := m.Get(ctx, client.ObjectKey{
		Name:      ingress.Name,
		Namespace: ingress.Namespace,
	}, existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		// Create new Ingress
		ingresslog.Info("Creating new Ingress", "name", ingress.Name, "namespace", ingress.Namespace)
		return m.Create(ctx, ingress)
	}

	// Check if there are any changes
	if reflect.DeepEqual(existing.Spec, ingress.Spec) &&
		reflect.DeepEqual(existing.Labels, ingress.Labels) &&
		reflect.DeepEqual(existing.Annotations, ingress.Annotations) {
		ingresslog.V(1).Info("Ingress unchanged, skipping update", "name", ingress.Name, "namespace", ingress.Namespace)
		return nil
	}

	ingresslog.Info("Updating existing Ingress", "name", ingress.Name, "namespace", ingress.Namespace)

	// Update existing Ingress by patching only the changed fields
	patch := client.MergeFrom(existing.DeepCopy())

	// Update only the fields that have changed
	if !reflect.DeepEqual(existing.Spec, ingress.Spec) {
		existing.Spec = ingress.Spec
	}
	if !reflect.DeepEqual(existing.Labels, ingress.Labels) {
		existing.Labels = ingress.Labels
	}
	if !reflect.DeepEqual(existing.Annotations, ingress.Annotations) {
		existing.Annotations = ingress.Annotations
	}

	return m.Patch(ctx, existing, patch)
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func pathTypePtr(pt networkingv1.PathType) *networkingv1.PathType {
	return &pt
}
