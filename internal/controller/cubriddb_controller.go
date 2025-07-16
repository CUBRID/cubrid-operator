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
	"time"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	manager "github.com/cubrid/cubrid-operator/pkg/manager"
	"github.com/cubrid/cubrid-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CubridDBReconciler reconciles a CubridDB object
type CubridDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

var cubriddblog = log.Log.WithName("CubridDB-Reconciler")

//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=cubriddbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=cubriddbs/status,verbs=get;update;patch;list;watch;create;delete
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=cubriddbs/finalizers,verbs=update;get;list;watch;create;patch;delete
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=backupdbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=backupdbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.cubrid.com,resources=backupdbs/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get
//+kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
//+kubebuilder:rbac:groups="",resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services/status,verbs=get
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=endpoints/restricted,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=endpoints/status,verbs=get
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete;bind
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingressclasses,verbs=get;list;watch;create;update;patch;delete

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
	cubriddblog.V(1).Info("Reconcile Start", "request", req)
	var cubridDB cubridv1.CubridDB

	if err := r.Get(ctx, req.NamespacedName, &cubridDB); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.setSpecDefaults(ctx, &cubridDB); err != nil {
		return ctrl.Result{}, fmt.Errorf("error defaulting cubriddb: %v", err)
	}

	statefulSetHandler, err := manager.NewStatefulSetManager(r.Client, r.Scheme)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create StatefulSet manager: %v", err)
	}

	if err := statefulSetHandler.ReconcileStatefulSet(ctx, &cubridDB); err != nil {
		return ctrl.Result{}, err
	}

	// Create service manager
	serviceManager := manager.NewServiceManager(r.Client, r.Scheme)
	if serviceManager == nil {
		return ctrl.Result{}, fmt.Errorf("failed to create service manager")
	}

	// Reconcile services based on CMS type
	if err := serviceManager.ReconcileServices(ctx, &cubridDB); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile CMS services based on type
	if cubridDB.IsCMSEnabled() {
		switch cubridDB.GetCMSServiceType() {
		case DEF.CMSServiceTypeNodePort:
			if err := serviceManager.ReconcileNodePortCMSServices(ctx, &cubridDB); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to reconcile NodePort CMS services: %v", err)
			}
		case DEF.CMSServiceTypeIngress:
			ingressManager := manager.NewIngressManager(r.Client, r.Scheme)
			if err := ingressManager.ReconcileIngressWithServices(ctx, &cubridDB); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to reconcile Ingress with services: %v", err)
			}
		default:
			return ctrl.Result{}, fmt.Errorf("unsupported CMS service type: %s", cubridDB.GetCMSServiceType())
		}
	}

	if cubridDB.IsHAEnabled() {
		groupHaManager := manager.NewGroupHAManager(r.Client, r.Scheme, r.Config)

		result, err := groupHaManager.ReconcileGroupHAMode(ctx, &cubridDB, req)
		if err != nil {
			return result, err
		}
	}

	if result, err := r.UpdateCubridDBStatus(ctx, r.Client, &cubridDB); err != nil {
		return result, err
	}

	cubriddblog.V(1).Info("Reconcile End", "request", req)
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
		Owns(&corev1.Service{}).     // Track related Services
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
	// StatefulSet Informer: Watches for StatefulSet events
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
		AddFunc:    r.cubridAdded,
		DeleteFunc: r.cubridDeleted,
	})

	return nil
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
			msCubridDB, err = util.GetCubridDBByName(
				r.Client,
				cubriddb.Namespace,
				cubriddb.Spec.Replication.HAmodeType.CubridRef.Name)
			if err != nil {
				cubriddblog.V(1).Info(fmt.Sprintf("Failed to get master CubridDB %s/%s, CubridRef %s",
					cubriddb.Namespace, cubriddb.Name, cubriddb.Spec.Replication.HAmodeType.CubridRef.Name), "error", err.Error())
				return
			}

			err = util.UpdateReplicaLink(r.Client, msCubridDB, cubriddb.Name, DEF.ADD_REPLICALINK)
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
		msCubridDB, err = util.GetCubridDBByName(
			r.Client,
			cubriddb.Namespace,
			cubriddb.Spec.Replication.HAmodeType.CubridRef.Name)

		if err != nil {
			cubriddblog.V(1).Info(fmt.Sprintf("Failed to get master CubridDB for replica deletion %s/%s, CubridRef Name %s",
				cubriddb.Namespace, cubriddb.Name, cubriddb.Spec.Replication.HAmodeType.CubridRef.Name), "error", err.Error())
			return
		}

		err = util.UpdateReplicaLink(r.Client, msCubridDB, cubriddb.Name, DEF.DELETE_REPLICALINK)
		if err != nil {
			cubriddblog.V(1).Info(fmt.Sprintf("Failed to delete replica link : Master CubridDB Name %s, Replica CubridDB Name %s",
				msCubridDB.Name, cubriddb.Name), "errro", err.Error())
			return
		}
	}
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
