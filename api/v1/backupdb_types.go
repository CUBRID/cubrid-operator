/*
 * Copyright 2016 CUBRID Corporation
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package v1

import (
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BackupDBSpec defines the desired state of BackupDB
type BackupDBSpec struct {
	CubridDBRef *CubridDBRef `json:"cubridDBRef,omitempty"`
	Schedules   *Schedules   `json:"schedules,omitempty"`
	StorageRef  *StorageRef  `json:"storageRef,omitempty"`
}

type CubridDBRef struct {
	Namespace string   `json:"namespace,omitempty"`
	Names     []string `json:"names,omitempty"`
	DBName    string   `json:"dbName,omitempty"` // airnet check
}

type Schedules struct {
	Schedule string `json:"schedule,omitempty"`
	FilePath string `json:"filepath,omitempty"`
	Args     string `json:"args,omitempty"`
}

type StorageRef struct {
	StorageType string `json:"storageType,omitempty"`
}

// BackupDBStatus defines the observed state of BackupDB
type BackupDBStatus struct {
	BackupStatus  map[string]PodsStatus `json:"backupStatus,omitempty"`
	OverallStatus string                `json:"overallStatus,omitempty"`
}

type PodsStatus struct {
	Status      string      `json:"status,omitempty" yaml:"status"`
	Message     string      `json:"message,omitempty" yaml:"message"`
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Schedule",type=string,description="Command of the BackupDB",JSONPath=".spec.schedules.schedule"
//+kubebuilder:printcolumn:name="Command",type=string,description="Command of the BackupDB",JSONPath=".spec.schedules.args"
//+kubebuilder:printcolumn:name="File Path",type=string,description="File path for the backup script",JSONPath=".spec.schedules.filepath"
//+kubebuilder:printcolumn:name="Status",type=string,description="Status of the command execution",JSONPath=".status.overallStatus"
//+kubebuilder:printcolumn:name="Age",type=date,description="Time duration since creation of the resource",JSONPath=".metadata.creationTimestamp"

// BackupDB is the Schema for the backupdbs API
type BackupDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupDBSpec   `json:"spec,omitempty"`
	Status BackupDBStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BackupDBList contains a list of BackupDB
type BackupDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackupDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupDB{}, &BackupDBList{})
}

func (b *BackupDB) CubridDBRef() *CubridDBRef {
	if b.Spec.CubridDBRef == nil {
		b.Spec.CubridDBRef = &CubridDBRef{}
	}
	return b.Spec.CubridDBRef
}

func (b *BackupDB) StorageRef() *StorageRef {
	if b.Spec.StorageRef == nil {
		b.Spec.StorageRef = &StorageRef{}
	}

	return b.Spec.StorageRef
}

func (b *BackupDB) Schedules() *Schedules {
	if b.Spec.Schedules == nil {
		defaultSchedules := &Schedules{
			Schedule: "0 0 * * 0",
			FilePath: DEF.BackupDB_Script_File_Path,
			Args:     DEF.BackupdbArgs,
		}
		b.Spec.Schedules = defaultSchedules
	}

	return b.Spec.Schedules
}

func (b *BackupDB) BackupDBStatus() *BackupDBStatus {
	if len(b.Status.BackupStatus) == 0 {
		b.Status.BackupStatus = make(map[string]PodsStatus, 0)
	}

	return &b.Status
}
