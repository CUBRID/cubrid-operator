package manager

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
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

func (r *ServiceManager) HandleCMSService(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	var existingSvc corev1.Service

	ports := pkg.CreatePort(DEF.SVC_CMS_PORT_NAME, DEF.SVC_CMS_SVC_PORT, DEF.SVC_CMS_PORT, corev1.ProtocolTCP, corev1.ServiceTypeNodePort)
	cms_svc := pkg.CreateService(
		cubridDB.Name+DEF.SVC_CMS_SUFFIX,
		cubridDB.Namespace,
		cubridDB.Name,
		corev1.ServiceTypeNodePort,
		ports,
	)

	if err := controllerutil.SetControllerReference(cubridDB, cms_svc, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to headless service : %v", err)
	}

	err := r.Get(ctx, types.NamespacedName{Name: cms_svc.Name, Namespace: cms_svc.Namespace}, &existingSvc)
	if err != nil {
		if errors.IsNotFound(err) {
			if createErr := r.Create(ctx, cms_svc); createErr != nil {
				return fmt.Errorf("error creating Service %s/%s: %v", cms_svc.Namespace, cms_svc.Name, createErr)
			}
			return nil
		}
		return fmt.Errorf("error getting headless service %s/%s: %v", cms_svc.Namespace, cms_svc.Name, err)
	}

	patch := client.MergeFrom(existingSvc.DeepCopy())
	existingSvc.Spec = cms_svc.Spec

	if err := r.Patch(ctx, &existingSvc, patch); err != nil {
		return fmt.Errorf("error patching headless service %s/%s: %v", existingSvc.Namespace, existingSvc.Name, err)
	}

	return nil
}

func (r *ServiceManager) HandleService(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	svcLabel := map[string]string{"app": cubridDB.Name}
	err := r.DeleteBrokerService(ctx, cubridDB, svcLabel)
	if err != nil {
		svclogger.V(1).Info(
			fmt.Sprintf("Failed to delete broker service: %s/%s", cubridDB.Namespace, cubridDB.Name),
			"error", err.Error(),
		)
	}

	// Create Broker Service
	for _, bs := range cubridDB.Spec.Broker {
		ports := pkg.CreatePort(
			bs.Name+DEF.SVC_PORT_SUFFIX,
			bs.ServicePort,
			bs.Port,
			corev1.ProtocolTCP,
			corev1.ServiceType(pkg.GetServiceType(bs.ServiceType)),
		)

		desiredSvc := pkg.CreateService(
			bs.Name,
			cubridDB.Namespace,
			cubridDB.Name,
			corev1.ServiceType(pkg.GetServiceType(bs.ServiceType)),
			ports,
		)

		if err := controllerutil.SetControllerReference(cubridDB, desiredSvc, r.Scheme); err != nil {
			return fmt.Errorf("error setting controller reference to service : %v", err)
		}

		var existingSvc corev1.Service
		err := r.Get(ctx, types.NamespacedName{Name: bs.Name, Namespace: cubridDB.Namespace}, &existingSvc)
		if err != nil {
			if errors.IsNotFound(err) {
				svclogger.Info("Creating new Service", "Service.Name", bs.Name)
				if createErr := r.Create(ctx, desiredSvc); createErr != nil {
					return fmt.Errorf("failed to create Service %s/%s: %v", cubridDB.Namespace, bs.Name, createErr)
				}
				continue
			}
			return fmt.Errorf("failed to get Service %s/%s: %v", cubridDB.Namespace, bs.Name, err)
		}

		patch := client.MergeFrom(existingSvc.DeepCopy())
		existingSvc.Spec = desiredSvc.Spec

		if err := r.Patch(ctx, &existingSvc, patch); err != nil {
			return fmt.Errorf("failed to patch Service %s/%s: %v", cubridDB.Namespace, bs.Name, err)
		}
	}

	return nil
}

func (r *ServiceManager) HandleHeadlessService(ctx context.Context, cubridDB *cubridv1.CubridDB) error {
	var existingSvc corev1.Service

	ports := pkg.CreatePort(DEF.SVC_HEADLESS_PORT_NAME, DEF.SVC_HA_PORT, DEF.SVC_HA_PORT, corev1.ProtocolTCP, corev1.ServiceTypeClusterIP)
	headlessService := pkg.CreateService(
		cubridDB.Name+DEF.SVC_NAME_SUFFIX,
		cubridDB.Namespace,
		cubridDB.Name,
		corev1.ServiceTypeClusterIP,
		ports,
	)
	headlessService.Spec.ClusterIP = corev1.ClusterIPNone

	if err := controllerutil.SetControllerReference(cubridDB, headlessService, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to headless service : %v", err)
	}

	err := r.Get(ctx, types.NamespacedName{Name: headlessService.Name, Namespace: headlessService.Namespace}, &existingSvc)
	if err != nil {
		if errors.IsNotFound(err) {
			if createErr := r.Create(ctx, headlessService); createErr != nil {
				return fmt.Errorf("error creating Service %s/%s: %v", headlessService.Namespace, headlessService.Name, createErr)
			}
			return nil
		}
		return fmt.Errorf("error getting headless service %s/%s: %v", headlessService.Namespace, headlessService.Name, err)
	}

	patch := client.MergeFrom(existingSvc.DeepCopy())
	existingSvc.Spec = headlessService.Spec

	if err := r.Patch(ctx, &existingSvc, patch); err != nil {
		return fmt.Errorf("error patching headless service %s/%s: %v", existingSvc.Namespace, existingSvc.Name, err)
	}

	return nil
}

func (r *ServiceManager) DeleteBrokerService(
	ctx context.Context,
	cubridDB *cubridv1.CubridDB,
	selector map[string]string,
) error {
	labelSelector := labels.SelectorFromSet(selector)

	listOptions := &client.ListOptions{
		Namespace:     cubridDB.Namespace,
		LabelSelector: labelSelector,
	}

	svcList := &corev1.ServiceList{}

	if err := r.List(ctx, svcList, listOptions); err != nil {
		return err
	}

	nodePortServices := make(map[string]string)
	clusterIPServices := make(map[string]string)
	for _, svcType := range cubridDB.Spec.Broker {
		if svcType.ServiceType == "NodePort" {
			nodePortServices[svcType.Name] = svcType.ServiceType
		} else if svcType.ServiceType == "ClusterIP" {
			clusterIPServices[svcType.Name] = svcType.ServiceType
		}
	}

	for _, svc := range svcList.Items {
		if svc.Spec.Type == corev1.ServiceTypeNodePort {
			if _, exists := nodePortServices[svc.Name]; !exists {
				if err := r.Delete(ctx, &svc); err != nil {
					return err
				}
			}
		} else if svc.Spec.Type == corev1.ServiceTypeClusterIP {
			if _, exists := clusterIPServices[svc.Name]; !exists {
				if err := r.Delete(ctx, &svc); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
