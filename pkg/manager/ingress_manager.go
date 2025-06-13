package manager

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
		serviceName := fmt.Sprintf(DEF.SVC_INGRESS_CMS_NAME, pod.Name, pod.Namespace)
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
func (m *IngressManager) ReconcileIngress(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	ingresslog := log.Log.WithName("ingress_manager")

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

	// Get all pods for this CubridDB
	podList := &corev1.PodList{}
	if err := m.List(ctx, podList, client.InNamespace(cubridDB.Namespace), client.MatchingLabels{"app": cubridDB.Name}); err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}

	// Create initial rules based on existing pods
	ingresslog.V(1).Info("Creating initial rules based on existing pods")
	var rules []networkingv1.IngressRule
	for _, pod := range podList.Items {
		hostName := fmt.Sprintf(DEF.INGRESS_HOST_NAME, pod.Name, cubridDB.Namespace)
		serviceName := fmt.Sprintf(DEF.SVC_INGRESS_CMS_NAME, pod.Name, cubridDB.Namespace)

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
										Number: 8001,
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

	// Get all pods for this CubridDB
	podList := &corev1.PodList{}
	if err := m.List(ctx, podList, client.InNamespace(cubridDB.Namespace), client.MatchingLabels{"app": cubridDB.Name}); err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}

	// Update rules based on pods
	var newRules []networkingv1.IngressRule
	for _, pod := range podList.Items {
		hostName := fmt.Sprintf(DEF.INGRESS_HOST_NAME, pod.Name, cubridDB.Namespace)
		serviceName := fmt.Sprintf(DEF.SVC_INGRESS_CMS_NAME, pod.Name, cubridDB.Namespace)

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
										Number: 8001,
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
		return m.Create(ctx, ingress)
	}

	// Check if there are any changes
	if !reflect.DeepEqual(existing.Spec, ingress.Spec) || !reflect.DeepEqual(existing.Labels, ingress.Labels) {
		ingresslog.Info("Update existing Ingress", "name", ingress.Name, "namespace", ingress.Namespace)
		// Update existing Ingress
		ingress.ResourceVersion = existing.ResourceVersion
		return m.Update(ctx, ingress)
	}
	return nil
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func pathTypePtr(pt networkingv1.PathType) *networkingv1.PathType {
	return &pt
}
