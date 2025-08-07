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
	"strconv"
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

//nolint:lll
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

func (b *BackupDB) initCommand() { // airnet check
	sc := b.Schedules()

	if sc.Schedule == "" {
		sc.Schedule = DEF.BackupDB_Schedule
	}

	if sc.FilePath == "" {
		sc.FilePath = DEF.BackupDB_Script_File_Path
	}

	if sc.Args == "" {
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
	if len(b.Status.BackupStatus) == 0 {
		b.Status.BackupStatus = make(map[string]PodsStatus, 0)
	}

	backupdblog.Info("initStatus", "CommandStatus", b.Status.BackupStatus)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//nolint:lll
//+kubebuilder:webhook:path=/validate-k8s-cubrid-com-v1-backupdb,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.cubrid.com,resources=backupdbs,verbs=create;update,versions=v1,name=vbackupdb.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &BackupDB{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (b *BackupDB) ValidateCreate() (admission.Warnings, error) {
	backupdblog.Info("validate create", "name", b.Name)

	var allErrs field.ErrorList

	// Check if CubridDBRef is provided
	if b.Spec.CubridDBRef == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("cubriddbref"),
			nil,
			"CubridDBRef is required."))
	} else if len(b.Spec.CubridDBRef.Names) == 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("cubriddbref").Child("names"),
			b.Spec.CubridDBRef.Names,
			"resource name may not be empty."))
	}

	// Check if StorageRef is provided
	if b.Spec.StorageRef == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("storageref"),
			nil,
			"StorageRef is required."))
	} else if !isValidStorageType(b.Spec.StorageRef.StorageType) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("storageref").Child("storagetype"),
			b.Spec.StorageRef.StorageType,
			"storagetype is incorrect. You must choose one of these four types: database-storage-type, conf-storage-type, backup-storage-type, logs-storage-type."))
	}

	// Check Schedules if provided
	if b.Spec.Schedules != nil {
		command := b.Spec.Schedules

		backupdblog.Info("validate create", "schedule.Args", command.Args)
		if command.Args == "" {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("schedules").Child("args"),
				command.Args,
				"The number of parameters passed to the backupdb script must be 4. start demodb 0 7"))
		}

		backupdblog.Info("validate create", "command.FilePath", command.FilePath)
		if command.FilePath == "" {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("schedules").Child("filepath"),
				command.FilePath,
				"You must write the path to the backupdb script file."))
		}

		backupdblog.Info("validate create", "command.Schedule", command.Schedule)
		if command.Schedule != "" && !isValidCronSchedule(command.Schedule) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("schedules").Child("schedule"),
				command.Schedule,
				"Invalid cron schedule format. Must be 5 fields: minute hour day month weekday. Example: '0 2 * * *'"))
		}
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

func isValidCronSchedule(schedule string) bool {
	backupdblog.Info("isValidCronSchedule", "schedule", schedule)

	// Basic cron format validation: 5 fields (minute hour day month weekday)
	// Format: "minute hour day month weekday"
	// Example: "0 2 * * *" (daily at 2 AM)

	if schedule == "" {
		return false
	}

	// Split the schedule into fields
	fields := strings.Fields(schedule)
	if len(fields) != 5 {
		return false
	}

	// Validate each field
	for i, field := range fields {
		if !isValidCronField(field, i) {
			return false
		}
	}

	return true
}

func isValidCronField(field string, fieldIndex int) bool {
	// Allow * for all fields
	if field == "*" {
		return true
	}

	// Check for step values (e.g., */15)
	if strings.Contains(field, "/") {
		parts := strings.Split(field, "/")
		if len(parts) != 2 {
			return false
		}
		if parts[0] != "*" {
			return false
		}
		step, err := strconv.Atoi(parts[1])
		if err != nil || step <= 0 {
			return false
		}
		return isValidStepValue(step, fieldIndex)
	}

	// Check for range values (e.g., 1-5)
	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		if len(parts) != 2 {
			return false
		}
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return false
		}
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			return false
		}
		return isValidRangeValue(start, end, fieldIndex)
	}

	// Check for list values (e.g., 1,3,5)
	if strings.Contains(field, ",") {
		parts := strings.Split(field, ",")
		for _, part := range parts {
			// For list values, we need to check each part individually
			// but avoid infinite recursion by handling single values directly
			if part == "*" {
				continue // * is always valid
			}

			// Check if it's a single number
			value, err := strconv.Atoi(part)
			if err != nil {
				return false
			}
			if !isValidSingleValue(value, fieldIndex) {
				return false
			}
		}
		return true
	}

	// Single value
	value, err := strconv.Atoi(field)
	if err != nil {
		return false
	}
	return isValidSingleValue(value, fieldIndex)
}

func isValidStepValue(step, fieldIndex int) bool {
	switch fieldIndex {
	case 0: // minute: 0-59
		return step >= 1 && step <= 59
	case 1: // hour: 0-23
		return step >= 1 && step <= 23
	case 2: // day: 1-31
		return step >= 1 && step <= 31
	case 3: // month: 1-12
		return step >= 1 && step <= 12
	case 4: // weekday: 0-6 (Sunday=0)
		return step >= 1 && step <= 6
	default:
		return false
	}
}

func isValidRangeValue(start, end, fieldIndex int) bool {
	if start > end {
		return false
	}

	switch fieldIndex {
	case 0: // minute: 0-59
		return start >= 0 && end <= 59
	case 1: // hour: 0-23
		return start >= 0 && end <= 23
	case 2: // day: 1-31
		return start >= 1 && end <= 31
	case 3: // month: 1-12
		return start >= 1 && end <= 12
	case 4: // weekday: 0-6 (Sunday=0)
		return start >= 0 && end <= 6
	default:
		return false
	}
}

func isValidSingleValue(value, fieldIndex int) bool {
	switch fieldIndex {
	case 0: // minute: 0-59
		return value >= 0 && value <= 59
	case 1: // hour: 0-23
		return value >= 0 && value <= 23
	case 2: // day: 1-31
		return value >= 1 && value <= 31
	case 3: // month: 1-12
		return value >= 1 && value <= 12
	case 4: // weekday: 0-6 (Sunday=0)
		return value >= 0 && value <= 6
	default:
		return false
	}
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (b *BackupDB) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	backupdblog.Info("validate update", "name", b.Name)

	var allErrs field.ErrorList
	oldBackupDB := old.(*BackupDB)

	// Check if CubridDBRef is provided
	if b.Spec.CubridDBRef == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("cubriddbref"),
			nil,
			"CubridDBRef is required."))
	} else {
		// Check if old BackupDB has CubridDBRef for immutability check
		if oldBackupDB.Spec.CubridDBRef != nil {
			if b.Spec.CubridDBRef.Namespace != oldBackupDB.Spec.CubridDBRef.Namespace {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("cubriddbref").Child("namespace"),
					b.Spec.CubridDBRef.Namespace,
					"CubridDBRef.Namespace cannot be changed."))
			}
		}

		if len(b.Spec.CubridDBRef.Names) == 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("cubriddbref").Child("names"),
				b.Spec.CubridDBRef.Names,
				"CubridDBRef.Names is required."))
		}
	}

	// Check if StorageRef is provided
	if b.Spec.StorageRef == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("storageref"),
			nil,
			"StorageRef is required."))
	} else if !isValidStorageType(b.Spec.StorageRef.StorageType) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("storageref").Child("storagetype"),
			b.Spec.StorageRef.StorageType,
			"storagetype is incorrect. You must choose one of these four types: database-storage-type, conf-storage-type, backup-storage-type, logs-storage-type."))
	}

	// Check Schedules if provided
	if b.Spec.Schedules != nil {
		command := b.Spec.Schedules

		if command.Args == "" {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("schedules").Child("args"),
				b.Spec.Schedules.Args,
				"The number of parameters passed to the backupdb script must be 4. start demodb 0 7"))
		}

		if command.FilePath == "" {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("schedules").Child("filepath"),
				b.Spec.Schedules.FilePath,
				"You must write the path to the backupdb script file."))
		}

		if command.Schedule != "" && !isValidCronSchedule(command.Schedule) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("schedules").Child("schedule"),
				b.Spec.Schedules.Schedule,
				"Invalid cron schedule format. Must be 5 fields: minute hour day month weekday. Example: '0 2 * * *'"))
		}
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
