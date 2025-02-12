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
	"reflect"

	DEF "github.com/cubrid/cubrid-operator/pkg/config"
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
	}

	for _, fn := range validateFns {
		if err := fn(); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *CubridDB) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	cubriddblog.Info("validate update", "name", c.Name)

	validateFns := []func() error{
		func() error { return c.validateUpdateHA(old) },
		func() error { return c.validateUpdateStorages(old) },
	}

	for _, fn := range validateFns {
		if err := fn(); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *CubridDB) ValidateDelete() (admission.Warnings, error) {
	cubriddblog.Info("validate delete", "name", c.Name)

	// TODO(user): fill in your validation logic upon object deletion.
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

func (c *CubridDB) validateCubridDBByRefName() error {
	cubriddblog.Info("validateCubridDBByRefName", "name", c.Name)

	if !c.Replication().Enable {
		return nil
	}

	// Replica type 인 경우, CubridRef.Name 이 있는지 체크한다.
	refName := c.Spec.Replication.HAmodeType.CubridRef.Name
	if c.HAmodeType() == DEF.HA_REPLICA_TYPE && c.Spec.Replication.HAmodeType.CubridRef.Name == "" {
		// ref.name이 없으면 오류를 반환합니다.
		return apierrors.NewInvalid(schema.GroupKind{Group: "k8s.cubrid.com", Kind: "CubridDB"}, c.Name, field.ErrorList{
			field.Invalid(field.NewPath("Spec").Child("Replication").Child("HAmodeType").Child("CubridRef").Child("Name"),
				refName, "CubridRef.Name must be specified"),
		})
	}

	// Replica type인 경우, Replica에서 참조 하는 Master-Slave type이 중복되면 안된다.
	// Replica에서 참조하는 Master-Slae 중복되는지 체크한다.
	if c.HAmodeType() == DEF.HA_REPLICA_TYPE {
		existingCubridDBList := &CubridDBList{} // CubridDBList를 사용하여 다수의 CubridDB 리소스를 찾습니다.
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
