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

package v1

import (
	"context"
	"fmt"
	"reflect"

	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var cubriddblog = logf.Log.WithName("CubridDB-Webhook")
var cubClient client.Client

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (c *CubridDB) SetupWebhookWithManager(mgr ctrl.Manager) error {
	cubClient = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-k8s-cubrid-com-v1-cubriddb,mutating=true,failurePolicy=fail,sideEffects=None,groups=k8s.cubrid.com,resources=cubriddbs,verbs=create;update,versions=v1,name=mcubriddb.kb.io,admissionReviewVersions=v1
var _ webhook.Defaulter = &CubridDB{}

func (c *CubridDB) Default() {
	cubriddblog.Info("default", "name", c.Name)

	defaultFns := []func(){
		c.initReplication,
		c.initBroker,
		c.initImage,
		c.initStorages,
		c.initAffinity,
		c.initUpdateStrategy,
		c.initCMSService,
		c.initHAPort,
	}

	for _, fn := range defaultFns {
		fn()
	}
}

//+kubebuilder:webhook:path=/validate-k8s-cubrid-com-v1-cubriddb,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.cubrid.com,resources=cubriddbs,verbs=create;update,versions=v1,name=vcubriddb.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &CubridDB{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *CubridDB) ValidateCreate() (admission.Warnings, error) {
	cubriddblog.Info("validate create", "name", c.Name)

	validateFns := []func() error{
		c.validateHA,
		c.validateCubridDBByRefName,
		c.validateCMSService,
		c.validateBrokerServicePort,
		c.validateHAPort,
		c.validateStorageConfiguration,
	}

	for _, fn := range validateFns {
		if err := fn(); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (c *CubridDB) validateHA() error {
	cubriddblog.Info("validateHA", "name", c.Name)

	var allErrs field.ErrorList
	if !c.Replication().Enable && c.GetReplicasNum() != 1 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("replication").Child("replicas"),
			c.Spec.Replication.Replicas,
			"replicas must be 1 when replication is disabled"))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(schema.GroupKind{Group: "k8s.cubrid.com", Kind: "CubridDB"}, c.Name, allErrs)
}

func (c *CubridDB) validateCubridDBByRefName() error {
	cubriddblog.Info("validateCubridDBByRefName", "name", c.Name)

	if !c.Replication().Enable {
		return nil
	}

	// Check if CubridRef.Name exists for Replica type
	refName := c.Spec.Replication.HAmodeType.CubridRef.Name
	if c.HAmodeType() == DEF.HA_REPLICA_TYPE && c.Spec.Replication.HAmodeType.CubridRef.Name == "" {
		// Return error if ref.name is not specified
		return apierrors.NewInvalid(schema.GroupKind{Group: "k8s.cubrid.com", Kind: "CubridDB"}, c.Name, field.ErrorList{
			field.Invalid(field.NewPath("Spec").Child("Replication").Child("HAmodeType").Child("CubridRef").Child("Name"),
				refName, "CubridRef.Name must be specified"),
		})
	}

	// For Replica type, check if the referenced Master-Slave is not duplicated
	// Check if the Master-Slave referenced by Replica is duplicated
	if c.HAmodeType() == DEF.HA_REPLICA_TYPE {
		existingCubridDBList := &CubridDBList{} // Use CubridDBList to find multiple CubridDB resources
		err := cubClient.List(context.Background(), existingCubridDBList, &client.ListOptions{
			Namespace: c.Namespace,
		})

		if err != nil {
			return apierrors.NewInvalid(schema.GroupKind{Group: "k8s.cubrid.com", Kind: "CubridDB"}, c.Name, field.ErrorList{
				field.Invalid(field.NewPath("Spec").Child("Replication").Child("HAmodeType").Child("CubridRef").Child("Name"),
					refName, err.Error()),
			})
		}

		for _, existingCubridDB := range existingCubridDBList.Items {
			cubriddblog.Info("findCubridDBByRefName", "cubriddb name", existingCubridDB.Name, "CubridRef Name", existingCubridDB.Spec.Replication.HAmodeType.CubridRef.Name)
			if existingCubridDB.Spec.Replication.HAmodeType.CubridRef.Name == refName {
				return apierrors.NewInvalid(schema.GroupKind{Group: "k8s.cubrid.com", Kind: "CubridDB"}, c.Name, field.ErrorList{
					field.Invalid(field.NewPath("spec").Child("replication").Child("cubridRef").Child("name"), refName, "Master-Slave is already referenced by another Replica."),
				})
			}
		}
	}

	return nil
}

func (c *CubridDB) validateCMSService() error {
	cubriddblog.Info("validateCMSService", "name", c.Name)

	var allErrs field.ErrorList

	if c.Spec.CMSService != nil {
		// Validate CMS service type
		if c.Spec.CMSService.Type != nil {
			serviceType := *c.Spec.CMSService.Type
			if serviceType != DEF.CMSServiceTypeNodePort && serviceType != DEF.CMSServiceTypeIngress {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec", "cmsService", "type"),
					serviceType,
					"type must be either 'NodePort' or 'Ingress'",
				))
			}

			// Note: Ingress controller existence will be validated during reconciliation
			// to avoid import cycle issues in webhook
		}

		// Validate startPort (only for NodePort type)
		if c.Spec.CMSService.StartPort != nil {
			if *c.Spec.CMSService.StartPort < 30000 || *c.Spec.CMSService.StartPort > 32767 {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec", "cmsService", "startPort"),
					*c.Spec.CMSService.StartPort,
					"startPort must be between 30000 and 32767 (NodePort range)",
				))
			}
		}

		// Validate port
		if c.Spec.CMSService.Port != nil {
			if *c.Spec.CMSService.Port < 1 || *c.Spec.CMSService.Port > 65535 {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec", "cmsService", "port"),
					*c.Spec.CMSService.Port,
					"port must be between 1 and 65535",
				))
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "k8s.cubrid.com", Kind: "CubridDB"},
		c.Name,
		allErrs,
	)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *CubridDB) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	cubriddblog.Info("validate update", "name", c.Name)

	// Validate immutable fields
	oldCubridDB := old.(*CubridDB)
	if err := c.validateImmutableFields(oldCubridDB); err != nil {
		return nil, err
	}

	validateFns := []func() error{
		func() error { return c.validateUpdateHA(old) },
		func() error { return c.validateUpdateStorages(old) },
		func() error { return c.validateUpdateStartPort(old) },
		func() error { return c.validateBrokerServicePort() },
		c.validateStorageConfiguration,
	}

	for _, fn := range validateFns {
		if err := fn(); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (c *CubridDB) validateUpdateHA(old runtime.Object) error {
	cubriddblog.Info("validateUpdateHA", "name", c.Name)

	var allErrs field.ErrorList

	// TODO(user): fill in your validation logic upon object update.
	oldCubridDB := old.(*CubridDB)
	if c.Replication().Enable != oldCubridDB.Replication().Enable {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("replication").Child("enable"),
			c.Spec.Replication.Enable,
			"replication enable field is immutable"))
	}

	if c.Replication().Enable != oldCubridDB.Replication().Enable {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("replication").Child("enable"),
			c.Spec.Replication.Enable,
			"replication enable field is immutable"))
	}

	if c.Replication().Enable {
		if c.HAmodeType() != oldCubridDB.HAmodeType() {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec").Child("replication").Child("hamodetype").Child("type"),
					c.Spec.Replication.HAmodeType.Type,
					"HAModeType cannot be changed."))
		}

		if (c.HAmodeType() != DEF.HA_MASTER_SLAVE_TYPE) && (c.HAmodeType() != DEF.HA_REPLICA_TYPE) {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec").Child("replication").Child("hamodetype").Child("type"),
					c.Spec.Replication.HAmodeType.Type,
					"HAmodeType must be set to master-slave or replica only."))
		}

		if c.Replication().HAmodeType.Type == DEF.HA_REPLICA_TYPE {
			if c.Replication().HAmodeType.CubridRef.Name == "" {
				allErrs = append(allErrs,
					field.Invalid(field.NewPath("spec").Child("replication").Child("hamodetype").Child("cubridref").Child("name"),
						c.Spec.Replication.HAmodeType.CubridRef.Name,
						"replica must be written with master-slave names."))
			}
		}

		if !c.Affinity().EnableAntiAffinity {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec").Child("affinity").Child("enableAntiAffinity"),
					c.Spec.Replication.HAmodeType.CubridRef.Name,
					"In HA Mode, EnableAntiAffinity must be true."))
		}

	} else {
		if c.Replication().Replicas != 1 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("replication").Child("replicas"),
				c.Spec.Replication.Replicas,
				"replicas must be 1 when replication is disabled"))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "k8s.cubrid.com", Kind: "CubridDB"}, c.Name, allErrs)
}

func (c *CubridDB) validateUpdateStorages(old runtime.Object) error {
	cubriddblog.Info("validateUpdateStorages", "name", c.Name)
	var allErrs field.ErrorList

	oldCubridDB := old.(*CubridDB)

	if !storagesAreEqual(c.Spec.Storage, oldCubridDB.Spec.Storage) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("storage"),
			c.Spec.Replication.Enable,
			"Storages is immutable and cannot be updated"))
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "k8s.cubrid.com", Kind: "CubridDB"}, c.Name, allErrs)
}

// validateUpdateStartPort checks if the CMS StartPort has been changed
func (c *CubridDB) validateUpdateStartPort(old runtime.Object) error {
	cubriddblog.Info("validateUpdateStartPort", "name", c.Name)
	var allErrs field.ErrorList

	oldCubridDB := old.(*CubridDB)

	// Check if both old and new have CMSService configured
	if oldCubridDB.Spec.CMSService != nil && c.Spec.CMSService != nil {
		// If both have StartPort configured, check if they are different
		if oldCubridDB.Spec.CMSService.StartPort != nil && c.Spec.CMSService.StartPort != nil {
			if *oldCubridDB.Spec.CMSService.StartPort != *c.Spec.CMSService.StartPort {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec").Child("cmsService").Child("startPort"),
					*c.Spec.CMSService.StartPort,
					fmt.Sprintf("CMS StartPort cannot be changed after initial creation. Current port: %d", *oldCubridDB.Spec.CMSService.StartPort)))
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "k8s.cubrid.com", Kind: "CubridDB"}, c.Name, allErrs)
}

// validateImmutableFields validates that immutable fields have not been changed
func (c *CubridDB) validateImmutableFields(old *CubridDB) error {
	cubriddblog.Info("validateImmutableFields", "name", c.Name)
	var allErrs field.ErrorList

	// Validate CMS Service Type is immutable
	if old.Spec.CMSService != nil && c.Spec.CMSService != nil {
		if old.Spec.CMSService.Type != nil && c.Spec.CMSService.Type != nil {
			if *old.Spec.CMSService.Type != *c.Spec.CMSService.Type {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec").Child("cmsService").Child("type"),
					*c.Spec.CMSService.Type,
					fmt.Sprintf("CMS Service Type cannot be changed after initial creation. Current type: %s", *old.Spec.CMSService.Type)))
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "k8s.cubrid.com", Kind: "CubridDB"}, c.Name, allErrs)
}

// validateBrokerServicePort checks if the Broker ServicePort is within the valid NodePort range
func (c *CubridDB) validateBrokerServicePort() error {
	cubriddblog.Info("validateBrokerServicePort", "name", c.Name)
	var allErrs field.ErrorList

	for i, broker := range c.Spec.Broker {
		if broker.ServiceType == corev1.ServiceTypeNodePort && broker.ServicePort != 0 {
			if broker.ServicePort < 30000 || broker.ServicePort > 32767 {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec").Child("broker").Index(i).Child("servicePort"),
					broker.ServicePort,
					"servicePort must be between 30000 and 32767 (NodePort range)"))
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "k8s.cubrid.com", Kind: "CubridDB"}, c.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *CubridDB) ValidateDelete() (admission.Warnings, error) {
	cubriddblog.Info("validate delete", "name", c.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

func storagesAreEqual(newStorages, oldStorages []Storage) bool {
	cubriddblog.Info("storagesAreEqual")
	if len(newStorages) != len(oldStorages) {
		return false
	}

	for i := range newStorages {
		if !reflect.DeepEqual(newStorages[i], oldStorages[i]) {
			return false
		}
	}

	return true
}

func (c *CubridDB) initImage() {
	c.InitImage()
}

func (c *CubridDB) initReplication() {
	c.Replication()
}

func (c *CubridDB) initBroker() {
	c.InitBroker()
}

func (c *CubridDB) initStorages() {
	c.InitStorages()
}

func (c *CubridDB) initAffinity() {
	c.InitAffinity()
}

func (c *CubridDB) initUpdateStrategy() {
	c.InitUpdateStrategy()
}

func (c *CubridDB) initCMSService() {
	c.InitCMSService()
}

func (c *CubridDB) initHAPort() {
	c.InitHAPort()
}

func (c *CubridDB) validateHAPort() error {
	cubriddblog.Info("validateHAPort", "name", c.Name)

	var allErrs field.ErrorList

	// Only validate HA port when replication is enabled
	if c.Spec.Replication != nil && c.Spec.Replication.Enable {
		if c.Spec.Replication.HAPort != nil {
			if *c.Spec.Replication.HAPort < 1 || *c.Spec.Replication.HAPort > 65535 {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec", "replication", "haPort"),
					*c.Spec.Replication.HAPort,
					"haPort must be between 1 and 65535",
				))
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "k8s.cubrid.com", Kind: "CubridDB"},
		c.Name,
		allErrs,
	)
}

func (c *CubridDB) validateStorageConfiguration() error {
	cubriddblog.Info("validateStorageConfiguration", "name", c.Name)

	var allErrs field.ErrorList

	for i, storage := range c.Spec.Storage {
		if err := storage.ValidateStorage(); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec").Child("storage").Index(i),
				storage,
				err.Error(),
			))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "k8s.cubrid.com", Kind: "CubridDB"},
		c.Name,
		allErrs,
	)
}
