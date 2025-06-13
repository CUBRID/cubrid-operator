package controller

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BrokerEndpointReconciler reconciles a CubridDB object's broker endpoints
type BrokerEndpointReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Config    *rest.Config
	clientset *kubernetes.Clientset
}

var brokerEPlog = log.Log.WithName("Broker-Endpoint-Reconciler")

//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=cubriddbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=cubriddbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=cubriddbs/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=endpoints/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=pods/exec,verbs=create

// SetupWithManager sets up the controller with the Manager.
func (r *BrokerEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create clientset
	clientset, err := kubernetes.NewForConfig(r.Config)
	if err != nil {
		return err
	}
	r.clientset = clientset

	return ctrl.NewControllerManagedBy(mgr).
		For(&cubridv1.CubridDB{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldDB := e.ObjectOld.(*cubridv1.CubridDB)
				newDB := e.ObjectNew.(*cubridv1.CubridDB)
				if !reflect.DeepEqual(oldDB.Spec.Broker, newDB.Spec.Broker) {
					return true
				}
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		}).
		Complete(r)
}

// BrokerStatus represents the status of a CUBRID broker
type BrokerStatus struct {
	Name     string
	PID      string
	Port     int32
	IsActive bool
}

// checkPortStatus checks if the Broker port is listening
func (r *BrokerEndpointReconciler) checkPortStatus(ctx context.Context, pod *corev1.Pod, port int32) (bool, error) {
	// Use netstat to check if the Broker port is listening
	cmd := []string{"netstat", "-tln", "|", "grep", fmt.Sprintf(":%d", port)}
	output, err := r.execCommand(ctx, pod, cmd)
	if err != nil {
		// If grep doesn't find anything, it returns error
		if strings.Contains(err.Error(), "exit status 1") {
			return false, nil
		}
		return false, err
	}

	// If output contains the port, it means the port is listening
	isListening := strings.Contains(output, fmt.Sprintf(":%d ", port))
	return isListening, nil
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *BrokerEndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	brokerEPlog.V(1).Info("Starting reconciliation", "namespace", req.Namespace, "name", req.Name)

	// Fetch the CubridDB instance
	cubridDB := &cubridv1.CubridDB{}
	if err := r.Get(ctx, req.NamespacedName, cubridDB); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get all pods for this CubridDB instance
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(cubridDB.Namespace), client.MatchingLabels{"app": cubridDB.Name}); err != nil {
		return ctrl.Result{}, err
	}

	if len(podList.Items) == 0 {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check port status for each broker on each pod
	for _, pod := range podList.Items {
		for _, broker := range cubridDB.Spec.Broker {
			// Check if the port is listening
			isActive, err := r.checkPortStatus(ctx, &pod, broker.Port)
			if err != nil {
				continue
			}

			// Update endpoint based on port status
			if err := r.updateBrokerEndpoint(ctx, cubridDB, &broker, &pod, isActive, broker.Port); err != nil {
				brokerEPlog.Error(err, "Failed to update broker endpoint", "broker", broker.Name, "pod", pod.Name)
				continue
			}
		}
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// execCommand executes a command in the pod
func (r *BrokerEndpointReconciler) execCommand(ctx context.Context, pod *corev1.Pod, cmd []string) (string, error) {
	brokerEPlog.V(1).Info("Executing command in pod", "pod", pod.Name, "command", strings.Join(cmd, " "))

	// Create exec request
	req := r.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: cmd,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, scheme.ParameterCodec)

	// Create SPDY executor
	exec, err := remotecommand.NewSPDYExecutor(r.Config, "POST", req.URL())
	if err != nil {
		return "", err
	}

	// Create buffers for stdout and stderr
	var stdout, stderr bytes.Buffer

	// Execute command
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %v, stderr: %s", err, stderr.String())
	}

	brokerEPlog.V(2).Info("Command executed successfully", "stdout", stdout.String())
	return stdout.String(), nil
}

// updateBrokerEndpoint updates the endpoint based on broker status
func (r *BrokerEndpointReconciler) updateBrokerEndpoint(ctx context.Context, cubridDB *cubridv1.CubridDB, broker *cubridv1.Broker, pod *corev1.Pod, isActive bool, port int32) error {
	endpointName := broker.Name

	// Get the endpoint
	endpoint := &corev1.Endpoints{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      endpointName,
		Namespace: cubridDB.Namespace,
	}, endpoint)

	// If endpoint doesn't exist, create it
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			brokerEPlog.Error(err, "Failed to get endpoint")
			return err
		}
		brokerEPlog.V(1).Info("Creating new endpoint", "endpoint", endpointName)
		endpoint = &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      endpointName,
				Namespace: cubridDB.Namespace,
				Labels: map[string]string{
					"app":    cubridDB.Name,
					"broker": endpointName,
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
	}

	// Update endpoint based on broker status
	if isActive {
		// Add pod IP to endpoint if not exists
		found := false
		for _, subset := range endpoint.Subsets {
			for _, addr := range subset.Addresses {
				if addr.IP == pod.Status.PodIP {
					found = true
					break
				}
			}
		}
		if !found {
			brokerEPlog.V(1).Info("Adding pod IP to endpoint", "endpoint", endpointName, "podIP", pod.Status.PodIP)
			// Check if there's an existing subset with the same port
			portFound := false
			for i, subset := range endpoint.Subsets {
				for _, p := range subset.Ports {
					if p.Port == port {
						// Add to existing subset
						endpoint.Subsets[i].Addresses = append(subset.Addresses, corev1.EndpointAddress{
							IP:       pod.Status.PodIP,
							NodeName: &pod.Spec.NodeName,
							TargetRef: &corev1.ObjectReference{
								Kind:      "Pod",
								Name:      pod.Name,
								Namespace: pod.Namespace,
								UID:       pod.UID,
							},
						})
						portFound = true
						break
					}
				}
				if portFound {
					break
				}
			}
			if !portFound {
				// Create new subset
				endpoint.Subsets = append(endpoint.Subsets, corev1.EndpointSubset{
					Addresses: []corev1.EndpointAddress{
						{
							IP:       pod.Status.PodIP,
							NodeName: &pod.Spec.NodeName,
							TargetRef: &corev1.ObjectReference{
								Kind:      "Pod",
								Name:      pod.Name,
								Namespace: pod.Namespace,
								UID:       pod.UID,
							},
						},
					},
					Ports: []corev1.EndpointPort{
						{
							Name:     endpointName,
							Port:     port,
							Protocol: corev1.ProtocolTCP,
						},
					},
				})
			}
		}
	} else {
		// Remove pod IP from endpoint if exists
		brokerEPlog.V(1).Info("Removing pod IP from endpoint", "endpoint", endpointName, "podIP", pod.Status.PodIP)
		newSubsets := []corev1.EndpointSubset{}
		for _, subset := range endpoint.Subsets {
			newAddresses := []corev1.EndpointAddress{}
			for _, addr := range subset.Addresses {
				if addr.IP != pod.Status.PodIP {
					newAddresses = append(newAddresses, addr)
				}
			}
			// Only add subset if it has addresses
			if len(newAddresses) > 0 {
				newSubsets = append(newSubsets, corev1.EndpointSubset{
					Addresses: newAddresses,
					Ports:     subset.Ports,
				})
			}
		}
		endpoint.Subsets = newSubsets
	}

	// If no subsets left, create an empty subset with notReadyAddresses
	if len(endpoint.Subsets) == 0 {
		endpoint.Subsets = []corev1.EndpointSubset{
			{
				NotReadyAddresses: []corev1.EndpointAddress{
					{
						IP: pod.Status.PodIP,
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Name:      pod.Name,
							Namespace: pod.Namespace,
							UID:       pod.UID,
						},
					},
				},
				Ports: []corev1.EndpointPort{
					{
						Name:     endpointName,
						Port:     port,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}
	}

	// Create or Update endpoint
	existingEndpoint := &corev1.Endpoints{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      endpointName,
		Namespace: cubridDB.Namespace,
	}, existingEndpoint); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		brokerEPlog.V(1).Info("Creating new endpoint", "endpoint", endpointName)
		return r.Create(ctx, endpoint)
	}

	// Compare existing and new endpoint states
	if reflect.DeepEqual(existingEndpoint.Subsets, endpoint.Subsets) {
		brokerEPlog.V(2).Info("Endpoint state unchanged, skipping update", "endpoint", endpointName)
		return nil
	}

	brokerEPlog.V(1).Info("Updating existing endpoint", "endpoint", endpointName)
	return r.Update(ctx, endpoint)
}
