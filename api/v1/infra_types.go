/*
Copyright 2023.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// InfraSpec defines the desired state of Infra
type InfraSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Infra. Edit infra_types.go to remove/update
	Region        string        `json:"region,omitempty"`
	Vswitch       []VSwitch     `json:"vswitch,omitempty"`
	Eip           []Eip         `json:"eip,omitempty"`
	SLB           SLB           `json:"slb,omitempty"`
	SecurityGroup SecurityGroup `json:"securityGroup,omitempty"`
	NatGateway    NatGateway    `json:"natGateway,omitempty"`
	Ram           RamRole       `json:"ram,omitempty"`
}

type Identity struct {
	Name string `json:"name,omitempty"`
	Id   string `json:"rid,omitempty"`
	// LifeCycle managed or detached
	Lifecycle string `json:"lifecycle,omitempty"`
}

type NatGateway struct {
	Identity    `json:"identity,omitempty"`
	RefEip      string `json:"refEip,omitempty"`
	SnatTableId string `json:"snatTableId,omitempty"`
}

type SLB struct {
	Identity   `json:"identity,omitempty"`
	RefEip     string     `json:"refEip,omitempty"`
	RefVswitch []string   `json:"refVswitch,omitempty"`
	IpAddr     string     `json:"ipAddr,omitempty"`
	Listener   []Listener `json:"listener,omitempty"`
}

type Listener struct {
	Port      int    `json:"port,omitempty"`
	Proto     string `json:"proto,omitempty"`
	Bandwidth int    `json:"bandwidth,omitempty"`
}

// InfraStatus defines the observed state of Infra
type InfraStatus struct {
	Phase string     `json:"phase,omitempty"`
	State *InfraSpec `json:"state,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Infra is the Schema for the infras API
type Infra struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfraSpec   `json:"spec,omitempty"`
	Status InfraStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// InfraList contains a list of Infra
type InfraList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Infra `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Infra{}, &InfraList{})
}
