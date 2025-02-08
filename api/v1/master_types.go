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
	"encoding/json"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AddonKubelet   = "kubelet"
	AddonRuntime   = "runtime"
	AddonEtcd      = "etcd"
	AddonApiServer = "kube-apiserver"
	AddonCCM       = "ccm"
	AddonScheduler = "kube-scheduler"
	AddonKCM       = "kcm"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MasterSpec defines the desired state of Master
type MasterSpec struct {
	Ip string `json:"ip,omitempty" protobuf:"bytes,1,opt,name=ip"`
	// ID provider Id of type: region.instanceid
	Id string `json:"Identity,omitempty" protobuf:"bytes,2,opt,name=Identity"`

	Addon map[string]Addon `json:"addon,omitempty" protobuf:"bytes,1,opt,name=addon"`
}

type Addon struct {
	Name            string          `json:"name"`
	Version         string          `json:"version"`
	Replicas        int             `json:"replicas,omitempty"`
	Category        string          `json:"category,omitempty"`
	TemplateVersion string          `json:"templateVersion,omitempty"`
	Param           json.RawMessage `json:"param,omitempty"`
}

// MasterStatus defines the observed state of Master
type MasterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Peer       []Host   `json:"peer,omitempty" protobuf:"bytes,1,opt,name=peer"`
	BootCFG    *Cluster `json:"bootcfg,omitempty" protobuf:"bytes,2,opt,name=bootcfg"`
	InstanceId string   `json:"instanceId,omitempty" protobuf:"bytes,3,opt,name=instanceId"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Master is the Schema for the masters API
type Master struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MasterSpec   `json:"spec,omitempty"`
	Status MasterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MasterList contains a list of Master
type MasterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Master `json:"items"`
}

func ToMasterStringList(m []Master) []string {
	var result []string
	for _, i := range m {
		result = append(result, i.String())
	}
	return result
}

func (m *Master) String() string {
	return fmt.Sprintf("master://%s/%s/%s", m.Name, m.Spec.Id, m.Spec.Ip)
}

func init() {
	SchemeBuilder.Register(&Master{}, &MasterList{})
}
