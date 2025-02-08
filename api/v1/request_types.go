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
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	GatewayMaster = "gw-master"
	KubernetesReq = "kubernetes"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RequestSpec defines the desired state of Request
type RequestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// InitType one of init | join
	//InitType string `json:"initType,omitempty"`

	// Role of init, one of master | worker
	//Role string `json:"role,omitempty"`

	// MasterSet is an example field of Request. Edit request_types.go to remove/update
	//MasterSet MasterSetSpec `json:"masterSet,omitempty"`

	Config ClusterConfig `json:"config"`
	// AccessPoint
	AccessPoint AccessPoint `json:"accessPoint,omitempty"`

	// Provider is the auth info for cloud provider
	Provider AuthInfo `json:"provider,omitempty"`
}

// RequestStatus defines the observed state of Request
type RequestStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	AddonInitialized bool `json:"addonInitialized,omitempty"`
}

type (
	NodeRole string
)

const (
	NodeRoleMaster NodeRole = "master"
	NodeRoleWorker NodeRole = "worker"

	ActionJoin = "join"
	ActionInit = "init"
)

const (
	APIServerDomain = "apiserver.xdpin.cn"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Request is the Schema for the requests API
type Request struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RequestSpec   `json:"spec,omitempty"`
	Status RequestStatus `json:"status,omitempty"`
}

func (req *Request) GetAddon(name string) *Addon {
	for i, _ := range req.Spec.Config.Addons {
		v := req.Spec.Config.Addons[i]
		if v.Name == name {
			return req.Spec.Config.Addons[i]
		}
	}
	return nil
}

func (req *Request) Validate() error {
	if req.Spec.AccessPoint.Intranet == "" {
		return errors.New("apiserver access point must not be empty")
	}
	if req.Spec.Config.TLS == nil {
		return errors.New("cluster tls certs must not be empty")
	}

	if req.Spec.Config.Etcd.Version == "" {
		req.Spec.Config.Etcd.Version = "v3.4.3"
	}
	if req.Spec.Config.Runtime.Version == "" {
		req.Spec.Config.Runtime.Version = "1.6.28"
	}
	if req.Spec.Config.Runtime.RuntimeType == "" {
		req.Spec.Config.Runtime.RuntimeType = "containerd"
	}
	if req.Spec.Config.Registry == "" {
		req.Spec.Config.Registry = "registry.cn-hangzhou.aliyuncs.com"
	}
	if req.Spec.Config.Kubernetes.Version == "" {
		req.Spec.Config.Kubernetes.Version = "1.31.1-aliyun.1"
	}
	if req.Spec.Config.Namespace == "" {
		req.Spec.Config.Namespace = "default"
	}
	if req.Spec.Config.CloudType == "" {
		req.Spec.Config.CloudType = "public"
	}
	if req.Spec.Config.Network.SVCCIDR == "" {
		req.Spec.Config.Network.SVCCIDR = "172.16.0.1/16"
	}
	if req.Spec.Config.Network.PodCIDR == "" {
		req.Spec.Config.Network.PodCIDR = "10.0.0.0/16"
	}
	if req.Spec.Config.Network.Domain == "" {
		req.Spec.Config.Network.Domain = "host.local"
	}
	return nil
}

func NewEmptyRequest(name string, spec RequestSpec) *Request {
	return &Request{
		TypeMeta: metav1.TypeMeta{
			APIVersion: GroupVersion.String(),
			Kind:       "Request",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: spec,
	}
}

//+kubebuilder:object:root=true

// RequestList contains a list of Request
type RequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Request `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Request{}, &RequestList{})
}
