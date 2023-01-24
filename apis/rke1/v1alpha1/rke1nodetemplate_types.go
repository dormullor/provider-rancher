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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// RKE1NodeTemplateParameters are the configurable fields of a RKE1NodeTemplate.
type RKE1NodeTemplateParameters struct {
	Name                 string            `json:"name,omitempty"`
	CloudCredentialId    string            `json:"cloudCredentialId,omitempty"`
	DisplayName          string            `json:"displayName,omitempty"`
	Driver               string            `json:"driver,omitempty"`
	EngineInstallURL     string            `json:"engineInstallURL,omitempty"`
	UseInternalIPAddress bool              `json:"useInternalIPAddress,omitempty"`
	Amazonec2Config      Amazonec2Config   `json:"amazonec2Config,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
}

// Amazonec2Config contains the parameters for the amazonec2 driver.
type Amazonec2Config struct {
	AMI                     string   `json:"ami,omitempty"`
	BlockDurationMinutes    int      `json:"blockDurationMinutes,omitempty"`
	DeviceName              string   `json:"deviceName,omitempty"`
	EncryptEBSVolume        bool     `json:"encryptEbsVolume,omitempty"`
	Endpoint                string   `json:"endpoint,omitempty"`
	HttpEndpoint            string   `json:"httpEndpoint,omitempty"`
	HTTPTokens              string   `json:"httpTokens,omitempty"`
	IAMInstanceProfile      string   `json:"iamInstanceProfile,omitempty"`
	InsecureTransport       bool     `json:"insecureTransport,omitempty"`
	InstanceType            string   `json:"instanceType,omitempty"`
	KeypairName             string   `json:"keypairName,omitempty"`
	KMSKey                  string   `json:"kmsKey,omitempty"`
	Monitoring              bool     `json:"monitoring,omitempty"`
	PrivateAddressOnly      bool     `json:"privateAddressOnly,omitempty"`
	Region                  string   `json:"region,omitempty"`
	RequestSpotInstance     bool     `json:"requestSpotInstance,omitempty"`
	Retries                 int      `json:"retries,omitempty"`
	RootSize                int      `json:"rootSize,omitempty"`
	SecurityGroup           []string `json:"securityGroup,omitempty"`
	SecurityGroupReadonly   bool     `json:"securityGroupReadonly,omitempty"`
	SessionToken            string   `json:"sessionToken,omitempty"`
	SpotPrice               string   `json:"spotPrice,omitempty"`
	SSHKeyContents          string   `json:"sshKeyContents,omitempty"`
	SSHUser                 string   `json:"sshUser,omitempty"`
	SubnetID                string   `json:"subnetId,omitempty"`
	SubnetIDRef             string   `json:"subnetIdRef,omitempty"`
	Tags                    string   `json:"tags,omitempty"`
	UseEBSOptimizedInstance bool     `json:"useEbsOptimizedInstance,omitempty"`
	UsePrivateAddress       bool     `json:"usePrivateAddress,omitempty"`
	UserData                string   `json:"userdata,omitempty"`
	VolumeType              string   `json:"volumeType,omitempty"`
	VpcID                   string   `json:"vpcId,omitempty"`
	VpcIDRef                string   `json:"vpcIdRef,omitempty"`
	Zone                    string   `json:"zone,omitempty"`
}

// RKE1NodeTemplateObservation are the observable fields of a RKE1NodeTemplate.
type RKE1NodeTemplateObservation struct {
	ID string `json:"id,omitempty"`
}

// A RKE1NodeTemplateSpec defines the desired state of a RKE1NodeTemplate.
type RKE1NodeTemplateSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       RKE1NodeTemplateParameters `json:"forProvider"`
}

// A RKE1NodeTemplateStatus represents the observed state of a RKE1NodeTemplate.
type RKE1NodeTemplateStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          RKE1NodeTemplateObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A RKE1NodeTemplate is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,rancher}
type RKE1NodeTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RKE1NodeTemplateSpec   `json:"spec"`
	Status RKE1NodeTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RKE1NodeTemplateList contains a list of RKE1NodeTemplate
type RKE1NodeTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RKE1NodeTemplate `json:"items"`
}

// RKE1NodeTemplate type metadata.
var (
	RKE1NodeTemplateKind             = reflect.TypeOf(RKE1NodeTemplate{}).Name()
	RKE1NodeTemplateGroupKind        = schema.GroupKind{Group: Group, Kind: RKE1NodeTemplateKind}.String()
	RKE1NodeTemplateKindAPIVersion   = RKE1NodeTemplateKind + "." + SchemeGroupVersion.String()
	RKE1NodeTemplateGroupVersionKind = SchemeGroupVersion.WithKind(RKE1NodeTemplateKind)
)

func init() {
	SchemeBuilder.Register(&RKE1NodeTemplate{}, &RKE1NodeTemplateList{})
}
