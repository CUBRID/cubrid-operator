package rbac

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
)

// RBACNames holds the names for RBAC resources
type RBACNames struct {
	ServiceAccountName string
	RoleName           string
	RoleBindingName    string
}

// GenerateRBACNames generates RBAC resource names based on CubridDB instance
func GenerateRBACNames(cubridDB *cubridv1.CubridDB) RBACNames {
	return RBACNames{
		ServiceAccountName: fmt.Sprintf("cubrid-%s-%s-sa", cubridDB.Name, cubridDB.Namespace),
		RoleName:           fmt.Sprintf("cubrid-%s-%s-role", cubridDB.Name, cubridDB.Namespace),
		RoleBindingName:    fmt.Sprintf("cubrid-%s-%s-rolebinding", cubridDB.Name, cubridDB.Namespace),
	}
}

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

// ReconcileRBACResources creates or updates RBAC resources (ServiceAccount, Role, RoleBinding)
// for a given CubridDB instance. It ensures that all necessary RBAC resources exist and are properly configured.
func ReconcileRBACResources(clientset *kubernetes.Clientset, cubridDB *cubridv1.CubridDB) error {
	// Generate RBAC resource names based on CubridDB instance
	rbacNames := GenerateRBACNames(cubridDB)

	// Create ServiceAccount
	if err := reconcileServiceAccount(clientset, cubridDB, rbacNames.ServiceAccountName); err != nil {
		return fmt.Errorf("error reconciling ServiceAccount: %v", err)
	}

	// Create Role
	if err := reconcileRole(clientset, cubridDB, rbacNames.RoleName); err != nil {
		return fmt.Errorf("error reconciling Role: %v", err)
	}

	// Create RoleBinding
	if err := reconcileRoleBinding(clientset, rbacNames, cubridDB.Namespace); err != nil {
		return fmt.Errorf("error reconciling RoleBinding: %v", err)
	}

	return nil
}

// reconcileServiceAccount creates or updates ServiceAccount
func reconcileServiceAccount(clientset *kubernetes.Clientset, cubridDB *cubridv1.CubridDB, serviceAccountName string) error {
	// Check if ServiceAccount exists
	_, err := clientset.CoreV1().ServiceAccounts(cubridDB.Namespace).Get(context.TODO(), serviceAccountName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create ServiceAccount if it doesn't exist
			serviceAccount := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceAccountName,
					Namespace: cubridDB.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name":      "cubrid",
						"app.kubernetes.io/instance":  cubridDB.Name,
						"app.kubernetes.io/component": "database",
						"app.kubernetes.io/part-of":   "cubrid-operator",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cubridDB.APIVersion,
							Kind:       cubridDB.Kind,
							Name:       cubridDB.Name,
							UID:        cubridDB.UID,
						},
					},
				},
			}
			_, err := clientset.CoreV1().ServiceAccounts(cubridDB.Namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("error creating ServiceAccount: %v", err)
			}
		} else {
			return fmt.Errorf("error checking existing ServiceAccount: %v", err)
		}
	}
	return nil
}

// reconcileRole creates or updates Role with minimal permissions for CUBRID Pods
func reconcileRole(clientset *kubernetes.Clientset, cubridDB *cubridv1.CubridDB, roleName string) error {
	// Check if Role exists
	_, err := clientset.RbacV1().Roles(cubridDB.Namespace).Get(context.TODO(), roleName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create Role if it doesn't exist with minimal permissions
			rules := []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods", "services", "configmaps", "secrets"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods/log"},
					Verbs:     []string{"get", "list"},
				},
			}

			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: cubridDB.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name":      "cubrid",
						"app.kubernetes.io/instance":  cubridDB.Name,
						"app.kubernetes.io/component": "database",
						"app.kubernetes.io/part-of":   "cubrid-operator",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: cubridDB.APIVersion,
							Kind:       cubridDB.Kind,
							Name:       cubridDB.Name,
							UID:        cubridDB.UID,
						},
					},
				},
				Rules: rules,
			}

			_, err = clientset.RbacV1().Roles(cubridDB.Namespace).Create(context.TODO(), role, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("error creating Role: %v", err)
			}
		} else {
			return fmt.Errorf("error checking existing Role: %v", err)
		}
	}
	return nil
}

// reconcileRoleBinding creates or updates RoleBinding
func reconcileRoleBinding(clientset *kubernetes.Clientset, rbacNames RBACNames, namespace string) error {
	// Check if RoleBinding exists
	_, err := clientset.RbacV1().RoleBindings(namespace).Get(context.TODO(), rbacNames.RoleBindingName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create RoleBinding if it doesn't exist
			subjects := []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      rbacNames.ServiceAccountName,
					Namespace: namespace,
				},
			}
			roleRef := rbacv1.RoleRef{
				Kind:     "Role",
				Name:     rbacNames.RoleName,
				APIGroup: "rbac.authorization.k8s.io",
			}

			roleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rbacNames.RoleBindingName,
					Namespace: namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name":      "cubrid",
						"app.kubernetes.io/component": "database",
						"app.kubernetes.io/part-of":   "cubrid-operator",
					},
				},
				RoleRef:  roleRef,
				Subjects: subjects,
			}

			_, err = clientset.RbacV1().RoleBindings(namespace).Create(context.TODO(), roleBinding, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("error creating RoleBinding: %v", err)
			}
		} else {
			return fmt.Errorf("error checking existing RoleBinding: %v", err)
		}
	}
	return nil
}
