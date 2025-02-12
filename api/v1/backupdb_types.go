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
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BackupDBSpec defines the desired state of BackupDB
type BackupDBSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	CubridDBRef *CubridDBRef `json:"cubridDBRef,omitempty"`
	CommandArgs *CommandArgs `json:"commandArgs,omitempty"`
	StorageRef  *StorageRef  `json:"storageRef,omitempty"`
}

type CubridDBRef struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	DBName    string `json:"dbName,omitempty"`
}

type CommandArgs struct {
	FilePath string   `json:"filepath,omitempty"`
	Args     []string `json:"args,omitempty"`
}

type StorageRef struct {
	StorageType string `json:"storageType,omitempty"`
}

// BackupDBStatus defines the observed state of BackupDB
type BackupDBStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Command       string `json:"command,omitempty"`
	FilePath      string `json:"filePath,omitempty"`
	CommandStatus string `json:"commandStatus,omitempty"`
	Message       string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Command",type=string,description="Command of the BackupDB",JSONPath=".status.command"
//+kubebuilder:printcolumn:name="File Path",type=string,description="File path for the backup script",JSONPath=".status.filePath"
//+kubebuilder:printcolumn:name="Command Status",type=string,description="Status of the command execution",JSONPath=".status.commandStatus"
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

func (b *BackupDB) CommandArgs() *CommandArgs {
	if b.Spec.CommandArgs == nil {
		defaultCommand := &CommandArgs{
			FilePath: DEF.BackupDB_Script_File_Path,
			Args:     DEF.BackupdbArgs,
		}
		b.Spec.CommandArgs = defaultCommand
	}

	return b.Spec.CommandArgs
}

func (b *BackupDB) BackupDBStatus() *BackupDBStatus {
	if b.Status.Command == "" {
		b.Status.Command = ""
	}

	if b.Status.FilePath == "" {
		b.Status.FilePath = ""
	}

	if b.Status.CommandStatus == "" {
		b.Status.CommandStatus = DEF.BackupDB_IDLE
	}

	if b.Status.Message == "" {
		b.Status.Message = ""
	}

	return &b.Status
}
