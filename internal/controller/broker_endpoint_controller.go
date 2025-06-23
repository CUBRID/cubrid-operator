package controller

import (
	"bytes"
	"context"
	"fmt"
	"net"
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

	// Process each broker - collect all pod statuses for each broker
	for _, broker := range cubridDB.Spec.Broker {
		// Collect active pod IPs for this broker
		activePodIPs := []corev1.EndpointAddress{}
		notReadyPodIPs := []corev1.EndpointAddress{}

		for _, pod := range podList.Items {
			// Skip pods that are not in Running state or without valid IP addresses
			if pod.Status.Phase != corev1.PodRunning || !r.isValidIPAddress(pod.Status.PodIP) {
				brokerEPlog.V(1).Info("Skipping pod - not running or no valid IP",
					"pod", pod.Name,
					"broker", broker.Name,
					"phase", pod.Status.Phase,
					"podIP", pod.Status.PodIP)
				continue
			}

			// Check if the port is listening
			isActive, err := r.checkPortStatus(ctx, &pod, broker.Port)
			if err != nil {
				brokerEPlog.V(1).Info("Failed to check port status, treating as not ready", "pod", pod.Name, "broker", broker.Name, "error", err.Error())
				// If we can't check status, treat as not ready
				notReadyPodIPs = append(notReadyPodIPs, corev1.EndpointAddress{
					IP:       pod.Status.PodIP,
					NodeName: &pod.Spec.NodeName,
					TargetRef: &corev1.ObjectReference{
						Kind:      "Pod",
						Name:      pod.Name,
						Namespace: pod.Namespace,
						UID:       pod.UID,
					},
				})
				continue
			}

			endpointAddr := corev1.EndpointAddress{
				IP:       pod.Status.PodIP,
				NodeName: &pod.Spec.NodeName,
				TargetRef: &corev1.ObjectReference{
					Kind:      "Pod",
					Name:      pod.Name,
					Namespace: pod.Namespace,
					UID:       pod.UID,
				},
			}

			if isActive {
				activePodIPs = append(activePodIPs, endpointAddr)
			} else {
				notReadyPodIPs = append(notReadyPodIPs, endpointAddr)
			}
		}

		// Update endpoint with all collected information
		if err := r.updateBrokerEndpointComplete(ctx, cubridDB, &broker, activePodIPs, notReadyPodIPs, broker.Port); err != nil {
			brokerEPlog.Error(err, "Failed to update broker endpoint", "broker", broker.Name)
			continue
		}
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// isValidIPAddress checks if the given IP address is valid
func (r *BrokerEndpointReconciler) isValidIPAddress(ip string) bool {
	if ip == "" {
		return false
	}

	// Check if it's a valid IPv4 or IPv6 address
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil
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

// updateBrokerEndpointComplete updates the endpoint based on broker status
func (r *BrokerEndpointReconciler) updateBrokerEndpointComplete(ctx context.Context, cubridDB *cubridv1.CubridDB, broker *cubridv1.Broker, activePodIPs, notReadyPodIPs []corev1.EndpointAddress, port int32) error {
	endpointName := broker.Name

	// Get the existing endpoint
	existingEndpoint := &corev1.Endpoints{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      endpointName,
		Namespace: cubridDB.Namespace,
	}, existingEndpoint)

	// Create the desired endpoint state
	desiredEndpoint := &corev1.Endpoints{
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

	// Set subsets based on active and not ready pod IPs
	if len(activePodIPs) > 0 {
		desiredEndpoint.Subsets = []corev1.EndpointSubset{
			{
				Addresses: activePodIPs,
				Ports: []corev1.EndpointPort{
					{
						Name:     endpointName,
						Port:     port,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}
	} else if len(notReadyPodIPs) > 0 {
		desiredEndpoint.Subsets = []corev1.EndpointSubset{
			{
				NotReadyAddresses: notReadyPodIPs,
				Ports: []corev1.EndpointPort{
					{
						Name:     endpointName,
						Port:     port,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}
	} else {
		// No pods available
		desiredEndpoint.Subsets = []corev1.EndpointSubset{
			{
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

	// If endpoint doesn't exist, create it
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			brokerEPlog.Error(err, "Failed to get endpoint")
			return err
		}
		brokerEPlog.V(1).Info("Creating new endpoint", "endpoint", endpointName, "activePods", len(activePodIPs), "notReadyPods", len(notReadyPodIPs))
		return r.Create(ctx, desiredEndpoint)
	}

	// Compare existing and desired endpoint states
	if reflect.DeepEqual(existingEndpoint.Subsets, desiredEndpoint.Subsets) {
		brokerEPlog.V(2).Info("Endpoint state unchanged, skipping update", "endpoint", endpointName)
		return nil
	}

	brokerEPlog.V(1).Info("Updating existing endpoint", "endpoint", endpointName, "activePods", len(activePodIPs), "notReadyPods", len(notReadyPodIPs))

	// Update the existing endpoint with desired state
	existingEndpoint.Subsets = desiredEndpoint.Subsets
	return r.Update(ctx, existingEndpoint)
}
