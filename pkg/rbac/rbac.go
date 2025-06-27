package rbac

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateClusterRole creates a ClusterRole with the given name and rules
func CreateClusterRole(
	clientset *kubernetes.Clientset,
	name string,
	rules []rbacv1.PolicyRule,
) (*rbacv1.ClusterRole, error) {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: rules,
	}

	return clientset.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
}

// CreateClusterRoleBinding creates a ClusterRoleBinding with the given name, roleRef, and subjects
func CreateClusterRoleBinding(
	clientset *kubernetes.Clientset,
	name string,
	roleRef rbacv1.RoleRef,
	subjects []rbacv1.Subject,
) (*rbacv1.ClusterRoleBinding, error) {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		RoleRef:  roleRef,
		Subjects: subjects,
	}

	return clientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
}

// CreateServiceAccount creates a ServiceAccount with the given name and namespace
func CreateServiceAccount(clientset *kubernetes.Clientset, name, namespace string) (*corev1.ServiceAccount, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	return clientset.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
}

// ReconcileRBACResources creates or updates RBAC resources (ServiceAccount, ClusterRole, ClusterRoleBinding)
// for a given namespace. It ensures that all necessary RBAC resources exist and are properly configured.
func ReconcileRBACResources(clientset *kubernetes.Clientset, namespace string) error {
	// Create ServiceAccount namespace
	serviceAccountName := fmt.Sprintf("cubrid-%s-sa", namespace)

	// Check if ServiceAccount exists
	_, err := clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), serviceAccountName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create ServiceAccount if it doesn't exist
			_, err := CreateServiceAccount(clientset, serviceAccountName, namespace)
			if err != nil {
				return fmt.Errorf("error creating ServiceAccount: %v", err)
			}
		} else {
			return fmt.Errorf("error checking existing ServiceAccount: %v", err)
		}
	}

	// Create ClusterRole name
	clusterRoleName := fmt.Sprintf("cubrid-%s-role", namespace)

	// Check if ClusterRole exists
	_, err = clientset.RbacV1().ClusterRoles().Get(context.TODO(), clusterRoleName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create ClusterRole if it doesn't exist
			rules := []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods", "services", "configmaps", "secrets"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"statefulsets"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{"k8s.cubrid.com"},
					Resources: []string{"cubriddbs", "cubriddbs/status", "cubriddbs/finalizers"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
			}
			_, err = CreateClusterRole(clientset, clusterRoleName, rules)
			if err != nil {
				return fmt.Errorf("error creating ClusterRole: %v", err)
			}
		} else {
			return fmt.Errorf("error checking existing ClusterRole: %v", err)
		}
	}

	// Create ClusterRoleBinding name
	clusterRoleBindingName := fmt.Sprintf("cubrid-%s-rolebinding", namespace)

	// Check if ClusterRoleBinding exists
	_, err = clientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterRoleBindingName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create ClusterRoleBinding if it doesn't exist
			subjects := []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      serviceAccountName,
					Namespace: namespace,
				},
			}
			roleRef := rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     clusterRoleName,
				APIGroup: "rbac.authorization.k8s.io",
			}
			_, err = CreateClusterRoleBinding(clientset, clusterRoleBindingName, roleRef, subjects)
			if err != nil {
				return fmt.Errorf("error creating ClusterRoleBinding: %v", err)
			}
		} else {
			return fmt.Errorf("error checking existing ClusterRoleBinding: %v", err)
		}
	}

	return nil
}
