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
	"strings"

	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var validStorageTypes = []string{
	DEF.StorageType_Database,
	DEF.StorageType_Logs,
	DEF.StorageType_backup,
	DEF.StorageType_conf,
}

// log is for logging in this package.
var backupdblog = logf.Log.WithName("BackupDB-Webhook")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *BackupDB) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-k8s-cubrid-com-v1-backupdb,mutating=true,failurePolicy=fail,sideEffects=None,groups=k8s.cubrid.com,resources=backupdbs,verbs=create;update,versions=v1,name=mbackupdb.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &BackupDB{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (b *BackupDB) Default() {
	backupdblog.Info("default", "name", b.Name)

	defaultFns := []func(){
		b.initCburidRef,
		b.initCommand,
		b.initStroageRef,
		b.initStatus,
	}

	for _, fn := range defaultFns {
		fn()
	}
}

func (b *BackupDB) initCburidRef() {
	cref := b.CubridDBRef()
	if cref.Namespace == "" {
		cref.Namespace = "default"
	}
}

func (b *BackupDB) initCommand() {
	sc := b.CommandArgs()
	if sc.FilePath == "" {
		sc.FilePath = DEF.BackupDB_Script_File_Path
	}

	if len(sc.Args) == 0 {
		sc.Args = DEF.BackupdbArgs
	}
}

func (b *BackupDB) initStroageRef() {
	sr := b.StorageRef()
	if sr.StorageType == "" {
		sr.StorageType = DEF.BackupDB_StorageType
	}
	backupdblog.Info("initStroageRef", "sr.StorageType", sr.StorageType)
}

func (b *BackupDB) initStatus() {
	if b.Status.Command == "" {
		b.Status.Command = strings.Join(DEF.BackupdbArgs, "")
	}
	backupdblog.Info("initStatus", "Command", b.Status.Command)

	if b.Status.FilePath == "" {
		b.Status.FilePath = DEF.BackupDB_Script_File_Path
	}

	backupdblog.Info("initStatus", "File", b.Status.FilePath)
	if b.Status.CommandStatus == "" {
		b.Status.CommandStatus = DEF.BackupDB_IDLE
	}
	backupdblog.Info("initStatus", "CommandStatus", b.Status.CommandStatus)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-k8s-cubrid-com-v1-backupdb,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.cubrid.com,resources=backupdbs,verbs=create;update,versions=v1,name=vbackupdb.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &BackupDB{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (b *BackupDB) ValidateCreate() (admission.Warnings, error) {
	backupdblog.Info("validate create", "name", b.Name)

	var allErrs field.ErrorList

	command := b.Spec.CommandArgs

	backupdblog.Info("validate create", "schedule.Args", strings.Join(command.Args, ""))
	if len(command.Args) != 4 {

		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("command").Child("args"),
			command.Args,
			"The number of parameters passed to the backupdb script must be 4. start demodb 0 7"))
	}

	backupdblog.Info("validate create", "command.FilePath", command.FilePath)
	if command.FilePath == "" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("command").Child("filepath"),
			command.FilePath,
			"You must write the path to the backupdb script file."))
	}

	if b.Spec.CubridDBRef == nil || b.Spec.CubridDBRef.Name == "" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("cubriddbref").Child("name"),
			b.Spec.CubridDBRef.Name,
			"resource name may not be empty."))
	}

	if b.Spec.StorageRef == nil || !isValidStorageType(b.Spec.StorageRef.StorageType) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("storageref").Child("storagetype"),
			b.Spec.StorageRef.StorageType,
			"storagetype is incorrect. You must choose one of these four types: database-storage-type, conf-storage-type, backup-storage-type, logs-storage-type."))
	}

	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(schema.GroupKind{Group: "k8s.cubrid.com", Kind: "BackupDB"}, b.Name, allErrs)
}

func isValidStorageType(storageType string) bool {
	backupdblog.Info("isValidStorageType", "storageType", storageType)
	for _, validType := range validStorageTypes {
		if storageType == validType {
			return true
		}
	}
	return false
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (b *BackupDB) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	backupdblog.Info("validate update", "name", b.Name)

	var allErrs field.ErrorList
	oldBackupDB := old.(*BackupDB)

	command := b.Spec.CommandArgs

	if command == nil {

	} else {
		if len(command.Args) != 4 {

			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("CommandArgs").Child("args"),
				b.Spec.CommandArgs.Args,
				"The number of parameters passed to the backupdb script must be 4. start demodb 0 7"))
		}

		if command.FilePath == "" {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("CommandArgs").Child("filepath"),
				b.Spec.CommandArgs.FilePath,
				"You must write the path to the backupdb script file."))
		}
	}

	if b.Spec.CubridDBRef == nil {

	} else {

		if b.Spec.CubridDBRef.Namespace != oldBackupDB.Spec.CubridDBRef.Namespace {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("cubriddbref").Child("namespace"),
				b.Spec.CubridDBRef.Namespace,
				"CubridDBRef.Namespace cannot be changed."))
		}

		if b.Spec.CubridDBRef.Name == "" {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("cubriddbref").Child("name"),
				b.Spec.CubridDBRef.Name,
				"CubridDBRef.Name is required."))
		}
	}

	if b.Spec.StorageRef == nil || !isValidStorageType(b.Spec.StorageRef.StorageType) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("storageref").Child("storagetype"),
			b.Spec.StorageRef.StorageType,
			"storagetype is incorrect. You must choose one of these four types: database-storage-type, conf-storage-type, backup-storage-type, logs-storage-type."))
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(schema.GroupKind{Group: "k8s.cubrid.com", Kind: "BackupDB"}, b.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (b *BackupDB) ValidateDelete() (admission.Warnings, error) {
	backupdblog.Info("validate delete", "name", b.Name)
	return nil, nil
}
