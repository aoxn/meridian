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
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	//"k8s.io/klog/v2"
	"unicode"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	InfraSpec  InfraSpec     `json:"infraSpec,omitempty" protobuf:"bytes,1,opt,name=infraSpec"`
	MasterSpec MasterSetSpec `json:"masterSpec,omitempty" protobuf:"bytes,2,opt,name=masterSpec"`
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	InfraState *InfraSpec `json:"infraState,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Cluster is the Schema for the clusters API
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

type KeyCert struct {
	Key  []byte `json:"key,omitempty" protobuf:"bytes,1,opt,name=key"`
	Cert []byte `json:"cert,omitempty" protobuf:"bytes,2,opt,name=cert"`
}

type Host struct {
	ID string `json:"Identity,omitempty" protobuf:"bytes,2,opt,name=Identity"`
	IP string `json:"ip,omitempty" protobuf:"bytes,1,opt,name=ip"`
}

const (
	KUBERNETES_CLUSTER = "kubernetes-cluster"
)

type Kubernetes struct {
	Unit `json:"unit,omitempty" protobuf:"bytes,1,opt,name=unit"`
}

type Etcd struct {
	Unit      `json:"unit,omitempty" protobuf:"bytes,1,opt,name=unit"`
	Endpoints string `json:"endpoints,omitempty" protobuf:"bytes,1,opt,name=endpoints"`
	InitToken string `json:"initToken,omitempty" protobuf:"bytes,2,opt,name=initToken"`
}

type ContainerRuntime struct {
	Unit `json:"unit,omitempty" protobuf:"bytes,1,opt,name=unit"`
}
type Unit struct {
	Name    string            `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Version string            `json:"version,omitempty" protobuf:"bytes,2,opt,name=version"`
	Paras   map[string]string `json:"paras,omitempty" protobuf:"bytes,3,opt,name=paras"`
}

type NetworkCfg struct {
	Mode    string `json:"mode,omitempty" protobuf:"bytes,1,opt,name=mode"`
	PodCIDR string `json:"podcidr,omitempty" protobuf:"bytes,2,opt,name=podcidr"`
	SVCCIDR string `json:"svccidr,omitempty" protobuf:"bytes,3,opt,name=svccidr"`
	Domain  string `json:"domain,omitempty" protobuf:"bytes,4,opt,name=domain"`
	NetMask string `json:"netMask,omitempty" protobuf:"bytes,5,opt,name=netMask"`
}

//type Disk struct {
//	Size string `json:"size,omitempty" protobuf:"bytes,1,opt,name=size"`
//	Type string `json:"type,omitempty" protobuf:"bytes,2,opt,name=type"`
//}

type Secret struct {
	Type  string `json:"type,omitempty" protobuf:"bytes,1,opt,name=type"`
	Value Value  `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
}
type Value struct {
	Name     string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Password string `json:"password,omitempty" protobuf:"bytes,1,opt,name=password"`
}

//
//type Kernel struct {
//	Sysctl []string `json:"sysctl,omitempty" protobuf:"bytes,1,opt,name=sysctl"`
//}

type Immutable struct {
	CAs CA `json:"cas,omitempty" protobuf:"bytes,1,opt,name=cas"`
}

type CA struct {
	Root      KeyCert `json:"root,omitempty" protobuf:"bytes,1,opt,name=root"`
	FrontRoot KeyCert `json:"frontRoot,omitempty" protobuf:"bytes,2,opt,name=frontRoot"`
}

func NewDefaultCluster(
	name string,
	spec ClusterSpec,
) *Cluster {
	return &Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}
}

// NodePoolSpec defines the desired state of NodePool
type NodePoolSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "manager-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	NodePoolID string `json:"Identity,omitempty" protobuf:"bytes,1,opt,name=Identity"`
	AutoHeal   bool   `json:"autoHeal,omitempty" protobuf:"bytes,2,opt,name=autoHeal"`
	Infra      Infra  `json:"infra,omitempty" protobuf:"bytes,3,opt,name=infra"`
}

type OSConfiguration struct {
}

//type Infra struct {
//	// DesiredCapacity
//	DesiredCapacity int `json:"desiredCapacity" protobuf:"bytes,1,opt,name=desiredCapacity"`
//
//	ImageId string            `json:"imageId,omitempty" protobuf:"bytes,2,opt,name=imageId"`
//	CPU     int               `json:"cpu,omitempty" protobuf:"bytes,3,opt,name=cpu"`
//	Mem     int               `json:"memory,omitempty" protobuf:"bytes,4,opt,name=memory"`
//	Tags    map[string]string `json:"tags,omitempty" protobuf:"bytes,5,opt,name=tags"`
//
//	// Generated ref of generated infra ids configmap
//	// for provider
//	//Generated string
//
//	Bind *BindID `json:"bind,omitempty" protobuf:"bytes,6,opt,name=bind"`
//}

// BindID is the infrastructure ids loaded(created) from under BindInfra layer
type BindID struct {
	VswitchIDS      []string `json:"vswitchIDs,omitempty" protobuf:"bytes,1,opt,name=vswitchIDs"`
	ScalingGroupId  string   `json:"scalingGroupId,omitempty" protobuf:"bytes,2,opt,name=scalingGroupId"`
	ConfigurationId string   `json:"configurationId,omitempty" protobuf:"bytes,3,opt,name=configurationId"`
}

// NodePoolStatus defines the observed state of NodePool
type NodePoolStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "manager-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodePool is the Schema for the nodepools API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=nodepools,scope=Namespaced
type NodePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodePoolSpec   `json:"spec,omitempty"`
	Status NodePoolStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodePoolList contains a list of NodePool
type NodePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodePool `json:"items"`
}

const (
	NodePoolHashLabel = "alibabacloud.com/nodepool.hash"
	NodePoolIDLabel   = "alibabacloud.com/nodepool-Identity"
)

type ConfigTpl struct {
	ImageId    string  `json:"imageid,omitempty" protobuf:"bytes,1,opt,name=imageid"`
	Runtime    Runtime `json:"runtime,omitempty" protobuf:"bytes,2,opt,name=runtime"`
	Kubernetes Unit    `json:"kubernetes,omitempty" protobuf:"bytes,3,opt,name=kubernetes"`
}

type Runtime struct {
	Name                 string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	RuntimeType          string `json:"runtimeType,omitempty" protobuf:"bytes,1,opt,name=runtimeType"`
	Version              string `json:"version,omitempty" protobuf:"bytes,2,opt,name=version"`
	NvidiaToolKitVersion string `json:"nvidiaToolKitVersion,omitempty" protobuf:"bytes,3,opt,name=nvidiaToolKitVersion"`
}

type Progress struct {
	Step        string `json:"step,omitempty"`
	Description string `json:"description,omitempty"`
}

func (n *Cluster) GenName(rType string) string {
	return fmt.Sprintf("%s.%s", rType, n.Name)
}

func (n *Cluster) Init() *Cluster {
	n.SetGroupVersionKind(getGVK("Cluster"))
	return n
}

func getGVK(r string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "meridian.meridian.io", Version: "v1", Kind: toUpper(r)}
}

func toUpper(r string) string {
	if r == "" {
		return ""
	}
	k := []rune(r)
	k[0] = unicode.ToUpper(k[0])
	return string(k)
}
