/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	"github.com/cubrid/cubrid-operator/pkg"
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	manager "github.com/cubrid/cubrid-operator/pkg/manager"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// CubridDBReconciler reconciles a CubridDB object
type CubridDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

var cubriddblog = log.Log.WithName("CubridDB-Reconciler")

//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=cubriddbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=cubriddbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=cubriddbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CubridDB object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.2/pkg/reconcile
func (r *CubridDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cubriddblog.Info("CubridDB Resource Info")

	// Fetch the CubridDB Custom Resource
	var cubridDB cubridv1.CubridDB
	if err := r.Get(ctx, req.NamespacedName, &cubridDB); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.setSpecDefaults(ctx, &cubridDB); err != nil {
		return ctrl.Result{}, fmt.Errorf("error defaulting cubriddb: %v", err)
	}

	// Create service manager
	serviceManager := manager.NewServiceManager(r.Client, r.Scheme)

	statefulSetHandler := manager.NewStatefulSetManager(r.Client, r.Scheme)
	if err := statefulSetHandler.ReconcileStatefulSet(ctx, &cubridDB); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile all services
	if err := serviceManager.ReconcileServices(ctx, &cubridDB); err != nil {
		return ctrl.Result{}, err
	}

	if cubridDB.IsHAEnabled() {
		haManager := manager.NewHAManager(r.Client, r.Scheme, r.Config)

		result, err := haManager.ReconcileHAMode(ctx, &cubridDB, req)
		if err != nil {
			return result, err
		}
	}

	if result, err := r.UpdateCubridDBStatus(ctx, r.Client, &cubridDB); err != nil {
		return result, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CubridDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	go r.monitorCRStatus(context.Background())

	// Register the main controller for managing CubridDB, Pods, and StatefulSets
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&cubridv1.CubridDB{}).   // Watch CubridDB resources
		Owns(&corev1.Pod{}).         // Track related Pods
		Owns(&appsv1.StatefulSet{}). // Track related StatefulSets
		Complete(r); err != nil {
		return fmt.Errorf("failed to set up main controller: %w", err)
	}

	// Configure additional watchers for custom objects
	if err := r.SetupObjectWatcher(mgr); err != nil {
		return fmt.Errorf("failed to set up object watchers: %w", err)
	}

	return nil
}

func (r *CubridDBReconciler) SetupObjectWatcher(mgr ctrl.Manager) error {
	podInformer, err := mgr.GetCache().GetInformer(context.Background(), &corev1.Pod{})
	if err != nil {
		return err
	}

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    r.podAdded,
		UpdateFunc: r.podUpdated,
		DeleteFunc: r.podDeleted,
	})

	// StatefulSet Informer: Watches for StatefulSet deletion events
	statefulSetInformer, err := mgr.GetCache().GetInformer(context.Background(), &appsv1.StatefulSet{})
	if err != nil {
		return err
	}

	statefulSetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: r.statefulSetDeleted,
	})

	// CubridDB Informer: Watches for CubridDB events
	cubridDBInformer, err := mgr.GetCache().GetInformer(context.Background(), &cubridv1.CubridDB{})
	if err != nil {
		return err
	}

	cubridDBInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: r.cubridAdded,
		// UpdateFunc: r.cubridUpdated,
		DeleteFunc: r.cubridDeleted,
	})

	return nil
}

// Event handler function to handle Pod add event
func (r *CubridDBReconciler) podAdded(obj interface{}) {
	addPod, isPod := obj.(*corev1.Pod)
	if !isPod {
		cubriddblog.V(1).Info("podAdded: Received object is not a Pod, skipping.", "objectType", fmt.Sprintf("%T", obj))
		return
	}

	cubriddb, err := pkg.GetCubridDBFromPod(r.Client, addPod)
	if err != nil {
		cubriddblog.V(1).Info(fmt.Sprintf("podAdded: Failed to get CubridDB for Pod %s/%s", addPod.Namespace, addPod.Name), "error", err.Error())
		return
	}

	if cubriddb.IsHAEnabled() {
		if err := pkg.UpdateCubridDB(context.Background(), r.Client, addPod); err != nil {
			cubriddblog.V(1).Info(fmt.Sprintf("podAdded: Failed to update CubridDB for Pod %s/%s", addPod.Namespace, addPod.Name), "error", err.Error())
			return
		}
	}
}

func (r *CubridDBReconciler) podUpdated(oldObj, newObj interface{}) {
	oldPod := oldObj.(*corev1.Pod)
	newPod := newObj.(*corev1.Pod)

	if oldPod.Status.Phase != corev1.PodRunning && newPod.Status.Phase == corev1.PodRunning {
		cubriddb, err := pkg.GetCubridDBFromPod(r.Client, newPod)
		if err != nil {
			cubriddblog.V(1).Info(fmt.Sprintf("Failed to get CubridDB for updated Pod %s/%s", newPod.Namespace, newPod.Name), "error", err.Error())
			return
		}

		if cubriddb.IsHAEnabled() {
			groupName, exists := newPod.Labels["group"]
			if !exists {
				cubriddblog.V(1).Info("Pod does not have a 'group' label", "podName", newPod.Name)
				return
			}

			podsInGroup, err := r.getPodsInGroup(context.Background(), newPod.Namespace, groupName)
			if err != nil {
				cubriddblog.V(1).Info(fmt.Sprintf("Failed to list pods in group %s, %s/%s", groupName, newPod.Namespace, newPod.Name), "error", err.Error())
				return
			}

			masterPods := []string{}
			replicaPods := []string{}

			for _, pod := range podsInGroup {
				if pod.Labels["grouptype"] == DEF.HA_MASTER_SLAVE_TYPE {
					masterPods = append(masterPods, pod.Name)
				} else if pod.Labels["grouptype"] == DEF.HA_REPLICA_TYPE {
					replicaPods = append(replicaPods, pod.Name)
				}

				cubriddblog.V(1).Info("master", "pods", strings.Join(masterPods, ":"))
				cubriddblog.V(1).Info("replica", "pods", strings.Join(replicaPods, ":"))
			}
		}
	}
}

func (r *CubridDBReconciler) podDeleted(obj interface{}) {
	var err error

	deletedPod := obj.(*corev1.Pod)

	cubriddb, err := pkg.GetCubridDBFromPod(r.Client, deletedPod)
	if err != nil {
		cubriddblog.V(1).Info(fmt.Sprintf("Failed to get CubridDB for deleted pod %s/%s", deletedPod.Namespace, deletedPod.Name), "error", err.Error())
		return
	}

	if cubriddb.IsHAEnabled() {
		err = pkg.UpdateCubridDB(context.Background(), r.Client, deletedPod)
		if err != nil {
			cubriddblog.V(1).Info(fmt.Sprintf("Failed to update CubridDB for deleted pod %s/%s", deletedPod.Namespace, deletedPod.Name), "error", err.Error())
			return
		}

		groupName, exists := deletedPod.Labels["group"]
		if !exists {
			cubriddblog.V(1).Info(fmt.Sprintf("Pod does not have a 'group' label %s/%s", deletedPod.Namespace, deletedPod.Name))
			return
		}

		podsInGroup, err := r.getPodsInGroup(context.Background(), deletedPod.Namespace, groupName)
		if err != nil {
			cubriddblog.V(1).Info(fmt.Sprintf("Failed to list pods in group %s, %s/%s", groupName, deletedPod.Namespace, deletedPod.Name), "error", err.Error())
			return
		}

		cubriddbList, err := r.getCubridDBListFromPods(context.Background(), podsInGroup)
		if err != nil {
			cubriddblog.V(1).Info(fmt.Sprintf("Failed to list CubridDBs in pods %s/%s", deletedPod.Namespace, deletedPod.Name), "error", err.Error())
			return
		}

		for _, cubridDB := range cubriddbList {
			haManager := manager.NewHAManager(r.Client, r.Scheme, r.Config)

			req := reconcile.Request{
				NamespacedName: client.ObjectKey{
					Namespace: deletedPod.Namespace,
					Name:      deletedPod.Name,
				},
			}

			_, err := haManager.ReconcileHAMode(context.Background(), &cubridDB, req)
			if err != nil {
				cubriddblog.V(1).Info(fmt.Sprintf("Failed HAMode reconfiguration : %s/%s", deletedPod.Namespace, deletedPod.Name), "error", err.Error())
			}
		}
	}
}

func (r *CubridDBReconciler) statefulSetDeleted(obj interface{}) {
	statefulSet, isStatefulSet := obj.(*appsv1.StatefulSet)
	if !isStatefulSet {

		return
	}
	cubriddblog.V(1).Info("delete statefulset", "statefulset name", statefulSet.Name)
}

func (r *CubridDBReconciler) cubridAdded(obj interface{}) {
	var msCubridDB *cubridv1.CubridDB = nil
	var err error

	cubriddb, isCubridDB := obj.(*cubridv1.CubridDB)
	if !isCubridDB {
		cubriddblog.V(1).Info("Failed to assert object to CubridDB")
		return
	}

	cubriddblog.V(1).Info("cubridAdded", "CubridDB Name", cubriddb.Name)

	if cubriddb.Spec.Replication != nil && cubriddb.Spec.Replication.HAmodeType != nil {
		if cubriddb.Spec.Replication.HAmodeType.Type == DEF.HA_MASTER_SLAVE_TYPE {

		} else if cubriddb.Spec.Replication.HAmodeType.Type == DEF.HA_REPLICA_TYPE {
			msCubridDB, err = pkg.GetCubridDBByName(
				r.Client,
				cubriddb.Namespace,
				cubriddb.Spec.Replication.HAmodeType.CubridRef.Name)
			if err != nil {
				cubriddblog.V(1).Info(fmt.Sprintf("Failed to get master CubridDB %s/%s, CubridRef %s",
					cubriddb.Namespace, cubriddb.Name, cubriddb.Spec.Replication.HAmodeType.CubridRef.Name), "error", err.Error())
				return
			}

			err = pkg.UpdateReplicaLink(r.Client, msCubridDB, cubriddb.Name, DEF.ADD_REPLICALINK)
			if err != nil {
				cubriddblog.V(1).Info(fmt.Sprintf("Failed to update replica link : Master CubridDB Name %s, Replica CubridDB Name %s",
					msCubridDB.Name, cubriddb.Name), "error", err.Error())
				return
			}
		}
	}
}

func (r *CubridDBReconciler) cubridDeleted(obj interface{}) {
	var msCubridDB *cubridv1.CubridDB = nil
	var err error

	cubriddb, isCubridDB := obj.(*cubridv1.CubridDB)
	if !isCubridDB {
		return
	}

	if cubriddb.Replication().Enable && cubriddb.HAmodeType() == DEF.HA_REPLICA_TYPE {
		msCubridDB, err = pkg.GetCubridDBByName(
			r.Client,
			cubriddb.Namespace,
			cubriddb.Spec.Replication.HAmodeType.CubridRef.Name)

		if err != nil {
			cubriddblog.V(1).Info(fmt.Sprintf("Failed to get master CubridDB for replica deletion %s/%s, CubridRef Name %s",
				cubriddb.Namespace, cubriddb.Name, cubriddb.Spec.Replication.HAmodeType.CubridRef.Name), "error", err.Error())
			return
		}

		err = pkg.UpdateReplicaLink(r.Client, msCubridDB, cubriddb.Name, DEF.DELETE_REPLICALINK)
		if err != nil {
			cubriddblog.V(1).Info(fmt.Sprintf("Failed to delete replica link : Master CubridDB Name %s, Replica CubridDB Name %s",
				msCubridDB.Name, cubriddb.Name), "errro", err.Error())
			return
		}
	}
}

func (r *CubridDBReconciler) getCubridDBListFromPods(ctx context.Context, pods []corev1.Pod) ([]cubridv1.CubridDB, error) {
	cubridDBList := make([]cubridv1.CubridDB, 0, len(pods))
	seen := make(map[string]bool)

	for _, pod := range pods {
		var statefulSetName string
		for _, ownerRef := range pod.OwnerReferences {
			if ownerRef.Kind == "StatefulSet" {
				statefulSetName = ownerRef.Name
				break
			}
		}

		if statefulSetName == "" {
			cubriddblog.V(1).Info("No StatefulSet owner found for Pod", "PodName", pod.Name)
			continue
		}

		var statefulSet appsv1.StatefulSet
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: statefulSetName}, &statefulSet)
		if err != nil {
			return nil, fmt.Errorf("failed to get StatefulSet %s in namespace %s: %w", statefulSetName, pod.Namespace, err)
		}

		var cubridDBName string
		for _, ownerRef := range statefulSet.OwnerReferences {
			if ownerRef.Kind == "CubridDB" {
				cubridDBName = ownerRef.Name
				break
			}
		}

		if cubridDBName == "" {
			cubriddblog.V(1).Info("No CubridDB owner found for StatefulSet", "StatefulSetName", statefulSetName, "Namespace", pod.Namespace)
			continue
		}

		key := fmt.Sprintf("%s/%s", pod.Namespace, cubridDBName)
		if seen[key] {
			continue
		}
		seen[key] = true

		var cubridDB cubridv1.CubridDB
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: cubridDBName}, &cubridDB); err != nil {
			return nil, fmt.Errorf("failed to get CubridDB %s in namespace %s: %w", cubridDBName, pod.Namespace, err)
		}

		cubriddblog.V(1).Info("CubridDB added to list", "CubridDBName", cubridDBName, "Namespace", pod.Namespace)
		cubridDBList = append(cubridDBList, cubridDB)
	}

	return cubridDBList, nil
}

func (r *CubridDBReconciler) getPodsInGroup(ctx context.Context, namespace string, groupName string) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}

	listOptions := &client.ListOptions{
		Namespace: namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"group": groupName,
		}),
	}

	if err := r.List(ctx, podList, listOptions); err != nil {
		return nil, err
	}

	return podList.Items, nil
}

func (r *CubridDBReconciler) setSpecDefaults(ctx context.Context, cubriddb *cubridv1.CubridDB) error {
	return r.patch(ctx, cubriddb, func(cubdb *cubridv1.CubridDB) {
		cubdb.SetDefaults()
	})
}

func (r *CubridDBReconciler) patch(ctx context.Context, cubriddb *cubridv1.CubridDB,
	patcher func(*cubridv1.CubridDB)) error {
	patch := client.MergeFrom(cubriddb.DeepCopy())
	patcher(cubriddb)
	return r.Patch(ctx, cubriddb, patch)
}

func GetStatefulSetNameFromPod(pod *corev1.Pod) (string, error) {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "StatefulSet" {
			return ownerRef.Name, nil
		}
	}
	return "", fmt.Errorf("pod %s is not owned by any StatefulSet", pod.Name)
}

func (r *CubridDBReconciler) monitorCRStatus(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.updateSatatus(ctx)
		}
	}
}

func (r *CubridDBReconciler) updateSatatus(ctx context.Context) {
	var crList cubridv1.CubridDBList
	if err := r.List(ctx, &crList); err != nil {
		cubriddblog.Error(err, "Failed to list CubridDB resources")
		return
	}

	for _, cr := range crList.Items {
		if _, err := r.UpdateCubridDBStatus(ctx, r.Client, &cr); err != nil {
			cubriddblog.Error(err, "Failed to reconcile status for CubridDB resource", "name", cr.Name, "namespace", cr.Namespace)
			return
		}
	}
}
