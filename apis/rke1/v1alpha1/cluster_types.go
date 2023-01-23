/*
Copyright 2022 The Crossplane Authors.

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

package v1alpha1

import (
	"reflect"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// localClusterAuthEndpoint is the local cluster auth endpoint.
type LocalClusterAuthEndpoint struct {
	Enabled bool   `json:"enabled,omitempty"`
	FQDN    string `json:"fqdn,omitempty"`
}

// RKENodePoolSpec defines the desired state of RKENodePool
type RKENodePool struct {
	Annotations             map[string]string `json:"annotations,omitempty"`
	BaseType                string            `json:"baseType,omitempty"`
	ClusterID               string            `json:"clusterId,omitempty"`
	ControlPlane            bool              `json:"controlPlane,omitempty"`
	DeleteNotReadyAfterSecs int64             `json:"deleteNotReadyAfterSecs,omitempty"`
	DrainBeforeDelete       bool              `json:"drainBeforeDelete,omitempty"`
	Driver                  string            `json:"driver,omitempty"`
	ETCD                    bool              `json:"etcd,omitempty"`
	HostnamePrefix          string            `json:"hostnamePrefix,omitempty"`
	Labels                  map[string]string `json:"labels,omitempty"`
	Name                    string            `json:"name,omitempty"`
	NodeTemplateID          string            `json:"nodeTemplateId,omitempty"`
	NodeTemplateIDRef       string            `json:"nodeTemplateIdRef,omitempty"`
	Quantity                int64             `json:"quantity,omitempty"`
	Worker                  bool              `json:"worker,omitempty"`
}

// RKEClusterConfigSpec defines the desired state of RKEClusterConfig
type RKEClusterConfigSpec struct {
	RKEClusterSpec           RancherKubernetesEngineConfig `json:"rancherKubernetesEngineConfig,omitempty"`
	DockerRootDir            string                        `json:"dockerRootDir,omitempty"`
	EnableClusterAlerting    bool                          `json:"enableClusterAlerting,omitempty"`
	EnableClusterMonitoring  bool                          `json:"enableClusterMonitoring,omitempty"`
	EnableNetworkPolicy      bool                          `json:"enableNetworkPolicy,omitempty"`
	Labels                   map[string]string             `json:"labels,omitempty"`
	LocalClusterAuthEndpoint LocalClusterAuthEndpoint      `json:"localClusterAuthEndpoint,omitempty"`
	Name                     string                        `json:"name,omitempty"`
}

// ClusterParameters are the configurable fields of a Cluster.
type ClusterParameters struct {
	KubeconfigSecretNamespace string               `json:"kubeconfigSecretNamespace,omitempty"`
	Region                    string               `json:"region,omitempty"`
	RKE                       RKEClusterConfigSpec `json:"rke,omitempty"`
	NodePools                 []RKENodePool        `json:"nodePools,omitempty"`
}

// ClusterObservation are the observable fields of a Cluster.
type ClusterObservation struct {
	ID string `json:"id,omitempty"`
}

// A ClusterSpec defines the desired state of a Cluster.
type ClusterSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       ClusterParameters `json:"forProvider"`
}

// A ClusterStatus represents the observed state of a Cluster.
type ClusterStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ClusterObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Cluster is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,rancher}
type RKE1Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type RKE1ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RKE1Cluster `json:"items"`
}

// Cluster type metadata.
var (
	ClusterKind             = reflect.TypeOf(RKE1Cluster{}).Name()
	ClusterGroupKind        = schema.GroupKind{Group: Group, Kind: ClusterKind}.String()
	ClusterKindAPIVersion   = ClusterKind + "." + SchemeGroupVersion.String()
	ClusterGroupVersionKind = SchemeGroupVersion.WithKind(ClusterKind)
)

func init() {
	SchemeBuilder.Register(&RKE1Cluster{}, &RKE1ClusterList{})
}
