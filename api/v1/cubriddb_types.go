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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CubridDBSpec defines the desired state of CubridDB
type CubridDBSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Replication        *Replication                      `json:"replication,omitempty"`
	Affinity           *Affinity                         `json:"affinty,omitempty"`
	Broker             []Broker                          `json:"broker,omitempty"`
	CMSService         *CMSServiceConfig                 `json:"cmsService,omitempty"`
	HAPort             *HAPortConfig                     `json:"haPort,omitempty"`
	Image              string                            `json:"image,omitempty"`
	InitContainerImage string                            `json:"initContainerImage,omitempty"`
	Storage            []Storage                         `json:"storage,omitempty"`
	Label              string                            `json:"label,omitempty"`
	UpdateStrategy     *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`
	ImagePullPolicy    corev1.PullPolicy                 `json:"imagePullPolicy,omitempty"`
}

// CubridDBStatus defines the observed state of CubridDB
type CubridDBStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions    []metav1.Condition `json:"conditions,omitempty"`
	HaMode        string             `json:"hamode,omitempty"`
	CurrentMaster string             `json:"currentMaster,omitempty"`
	NodeLists     []string           `json:"nodeLists,omitempty"`
	LastUpdated   metav1.Time        `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="HA Mode",type="string",JSONPath=".status.hamode"
//+kubebuilder:printcolumn:name="Master Server",type="string",JSONPath=".status.currentMaster"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// CubridDB is the Schema for the cubriddbs API
type CubridDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CubridDBSpec   `json:"spec,omitempty"`
	Status CubridDBStatus `json:"status,omitempty"`
}

type Replication struct {
	Enable     bool        `json:"enable,omitempty" webhook:"inmutable"`
	Replicas   int32       `json:"replicas,omitempty"`
	HAmodeType *HAmodeType `json:"hamodeType,omitempty" webhook:"inmutable"`
}

type HAmodeType struct {
	Type      string     `json:"type,omitempty"`
	CubridRef *CubridRef `json:"cubridRef,omitempty"`
}

type CubridRef struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	// ReplicaLink is for internal use only. Not user-settable.
	ReplicaLink string `json:"-"` // internal use only
}

type Affinity struct {
	EnableAntiAffinity bool `json:"enableAntiAffinity,omitempty"`
}

type Broker struct {
	// Name is the unique identifier for the broker
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Port is the broker port number
	Port int32 `json:"port,omitempty"`

	// ServiceType determines how the Service is exposed
	// +kubebuilder:validation:Enum=ClusterIP;NodePort
	ServiceType corev1.ServiceType `json:"serviceType"`

	// ServicePort is the port number that will be exposed when ServiceType is NodePort
	// +optional
	ServicePort int32 `json:"servicePort,omitempty"`
}

// CMSServiceConfig defines the configuration for CMS services
type CMSServiceConfig struct {
	// Enabled indicates whether CMS service should be created
	Enabled *bool `json:"enabled,omitempty"`
	// StartPort is the starting NodePort number for CMS services
	StartPort *int32 `json:"startPort,omitempty"`
	// Port is the container port number for CMS
	Port *int32 `json:"port,omitempty"`
}

// HAPortConfig defines the configuration for HA port
type HAPortConfig struct {
	// Port is the port number for the HA port
	// Default is 59901 (CUBRID HA port)
	Port *int32 `json:"port,omitempty"`
}

type Storage struct {
	Name             string             `json:"name,omitempty"`
	MountPath        string             `json:"mountPath,omitempty"`
	Type             string             `json:"type,omitempty"`
	Size             *resource.Quantity `json:"size,omitempty"`
	StorageClassName string             `json:"storageClassName,omitempty" webhook:"inmutable"`
	VolumeName       string             `json:"volumeName,omitempty" webhook:"inmutable"`
}

//+kubebuilder:object:root=true

// CubridDBList contains a list of CubridDB
type CubridDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CubridDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CubridDB{}, &CubridDBList{})
}

// SetDefaults sets reasonable defaults.
func (c *CubridDB) SetDefaults() {
	c.Replication()
	c.InitBroker()
	c.InitImage()
	c.InitStorages()
	c.InitAffinity()
	c.InitUpdateStrategy()
	c.InitCMSService()
	c.InitHAPort()
}

func (c *CubridDB) Replication() Replication {
	if c.Spec.Replication == nil {
		c.Spec.Replication = &Replication{}
		c.Spec.Replication.Replicas = 1
		c.Spec.Replication.Enable = false
	} else {
		if c.Spec.Replication.Replicas <= 0 {
			c.Spec.Replication.Replicas = 1
		}
	}

	c.Spec.Replication.FillWithDefaults()
	return *c.Spec.Replication
}

func (c *CubridDB) Affinity() Affinity {
	c.InitAffinity()
	return *c.Spec.Affinity
}

func (r *Replication) FillWithDefaults() {
	if r.HAmodeType == nil {
		r.HAmodeType = &HAmodeType{}
		r.HAmodeType.Type = ""
	} else {
		if r.HAmodeType.Type == "" {
			r.HAmodeType.Type = DEF.HA_MASTER_SLAVE_TYPE
		}
	}

	if r.HAmodeType.CubridRef == nil {
		r.HAmodeType.CubridRef = &CubridRef{}
		r.HAmodeType.CubridRef.Name = ""
		r.HAmodeType.CubridRef.Namespace = ""
		r.HAmodeType.CubridRef.ReplicaLink = ""
	} else {
		if r.HAmodeType.CubridRef.Namespace == "" {
			r.HAmodeType.CubridRef.Namespace = "default"
		}
	}
}

func (c *CubridDB) IsHAEnabled() bool {
	c.Replication()
	return c.Spec.Replication.Enable
}

func (c *CubridDB) GetReplicasNum() int32 {
	return c.Replication().Replicas
}

func (c *CubridDB) HAmodeType() string {
	var server_type string
	server_type = ""
	if c.Spec.Replication.HAmodeType != nil {
		server_type = c.Spec.Replication.HAmodeType.Type
	}

	return server_type
}

func (c *CubridDB) InitBroker() {
	// Do not change if Broker is already set
	if len(c.Spec.Broker) > 0 {
		return
	}

	broker1 := Broker{
		Name:        DEF.SVC_BR_NAME_QUERY_EDITOR,
		Port:        int32(30000),
		ServiceType: DEF.SVC_TYPE_NODE_PORT,
		ServicePort: int32(30000),
	}

	broker2 := Broker{
		Name:        DEF.SVC_BR_NAME_BROKER1,
		Port:        int32(33000),
		ServiceType: DEF.SVC_TYPE_NODE_PORT,
		ServicePort: int32(31000),
	}

	c.Spec.Broker = []Broker{broker1, broker2}
}

func (c *CubridDB) InitImage() {
	if c.Spec.Image == "" {
		c.Spec.Image = DEF.CubridDefaultImage
	}
	if c.Spec.InitContainerImage == "" {
		c.Spec.InitContainerImage = DEF.InitContainerDefaultImage
	}
}

func (c *CubridDB) InitStorages() {
	// Do not change if Storage is already set
	if len(c.Spec.Storage) > 0 {
		return
	}

	size := resource.MustParse(DEF.Default_Volume_Size)
	db_storage := Storage{
		Name:             DEF.DatabaseStorageVolumeName,
		Type:             DEF.StorageType_Database,
		MountPath:        "/home/cubrid/CUBRID/databases",
		StorageClassName: DEF.DefaultStorageClassName,
		Size:             &size,
		VolumeName:       "",
	}

	log_storage := Storage{
		Name:             DEF.LogsStorageVolumeName,
		Type:             DEF.StorageType_Logs,
		MountPath:        "/home/cubrid/CUBRID/log",
		StorageClassName: DEF.DefaultStorageClassName,
		Size:             &size,
		VolumeName:       "",
	}

	backupdb_storage := Storage{
		Name:             DEF.BackupDBStorageVolumeName,
		Type:             DEF.StorageType_backup,
		MountPath:        "/home/cubrid/CUBRID/backupdb",
		StorageClassName: DEF.DefaultStorageClassName,
		Size:             &size,
		VolumeName:       "",
	}

	conf_storage := Storage{
		Name:             DEF.ConfStorageVolumeName,
		Type:             DEF.StorageType_conf,
		MountPath:        "/home/cubrid/CUBRID/conf",
		StorageClassName: DEF.DefaultStorageClassName,
		Size:             &size,
		VolumeName:       "",
	}

	c.Spec.Storage = []Storage{db_storage, log_storage, backupdb_storage, conf_storage}
}

func (c *CubridDB) InitAffinity() {
	// Do not change if Affinity is already set
	if c.Spec.Affinity != nil {
		return
	}

	c.Spec.Affinity = &Affinity{}
	if c.Replication().Enable {
		c.Spec.Affinity.EnableAntiAffinity = true
	} else {
		c.Spec.Affinity.EnableAntiAffinity = false
	}
}

func (c *CubridDB) InitUpdateStrategy() {
	// Set default values only if UpdateStrategy is nil
	if c.Spec.UpdateStrategy == nil {
		partition := int32(0)
		maxUnavailable := intstr.FromInt(1)
		c.Spec.UpdateStrategy = &appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
			RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
				Partition:      &partition,
				MaxUnavailable: &maxUnavailable,
			},
		}
		return
	}

	// If UpdateStrategy already exists, set only the minimum required fields
	// Set Type only if it's empty
	if c.Spec.UpdateStrategy.Type == "" {
		c.Spec.UpdateStrategy.Type = appsv1.RollingUpdateStatefulSetStrategyType
	}

	// Set RollingUpdate only if Type is RollingUpdate but RollingUpdate field is nil
	if c.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType && c.Spec.UpdateStrategy.RollingUpdate == nil {
		partition := int32(0)
		maxUnavailable := intstr.FromInt(1)
		c.Spec.UpdateStrategy.RollingUpdate = &appsv1.RollingUpdateStatefulSetStrategy{
			Partition:      &partition,
			MaxUnavailable: &maxUnavailable,
		}
	}
}

// setCMSServiceDefaults sets defaults for CMS service configuration
func (c *CubridDB) InitCMSService() {
	// Create CMSService only if it is nil
	if c.Spec.CMSService == nil {
		cubriddblog.Info("InitCMSService CMSService is nil", "name", c.Name)
		c.Spec.CMSService = &CMSServiceConfig{}
	}

	// Set default values only if each field is nil
	if c.Spec.CMSService.Enabled == nil {
		enabled := DEF.SVC_CMS_ENABLED
		c.Spec.CMSService.Enabled = &enabled
	}

	if c.Spec.CMSService.StartPort == nil {
		startPort := int32(DEF.SVC_CMS_START_NODE_PORT)
		c.Spec.CMSService.StartPort = &startPort
	}

	if c.Spec.CMSService.Port == nil {
		port := int32(DEF.CMS_PORT)
		c.Spec.CMSService.Port = &port
	}
}

// GetCMSPort returns the CMS port number for all CMS-related services
// This ensures consistency across:
// - Ingress CMS services
// - Ingress backend services
// - CMS NodePort services
func (c *CubridDB) GetCMSPort() int32 {
	if c.Spec.CMSService == nil || c.Spec.CMSService.Port == nil {
		return DEF.CMS_PORT
	}
	return *c.Spec.CMSService.Port
}

// GetCMSStartPort returns the CMS start port number
func (c *CubridDB) GetCMSStartPort() int32 {
	if c.Spec.CMSService == nil || c.Spec.CMSService.StartPort == nil {
		return DEF.SVC_CMS_START_NODE_PORT
	}
	return *c.Spec.CMSService.StartPort
}

// IsCMSEnabled returns whether CMS service is enabled
func (c *CubridDB) IsCMSEnabled() bool {
	if c.Spec.CMSService == nil || c.Spec.CMSService.Enabled == nil {
		return DEF.SVC_CMS_ENABLED
	}
	return *c.Spec.CMSService.Enabled
}

func (c *CubridDB) InitHAPort() {
	// Create HAPort only if it is nil
	if c.Spec.HAPort == nil {
		cubriddblog.Info("InitHAPort HAPort is nil", "name", c.Name)
		c.Spec.HAPort = &HAPortConfig{}
	}

	// Set default value only if Port is nil
	if c.Spec.HAPort.Port == nil {
		port := int32(DEF.SVC_HA_PORT)
		c.Spec.HAPort.Port = &port
	}
}

// GetHAPort returns the HA port number
func (c *CubridDB) GetHAPort() int32 {
	if c.Spec.HAPort == nil || c.Spec.HAPort.Port == nil {
		return DEF.SVC_HA_PORT
	}
	return *c.Spec.HAPort.Port
}
