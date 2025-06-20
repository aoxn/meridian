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

// MasterSetSpec defines the desired state of MasterSet
type MasterSetSpec struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`

	// Label selector for pods. Existing ReplicaSets whose pods are
	// selected by this will be the ones affected by this deployment.
	// It must match the pod template's labels.
	Selector *metav1.LabelSelector `json:"selector" protobuf:"bytes,2,opt,name=selector"`

	// Template describes the pods that will be created.
	// The only allowed template.spec.restartPolicy value is "Always".
	Template MasterTemplateSpec `json:"template" protobuf:"bytes,3,opt,name=template"`

	// The deployment strategy to use to replace existing pods with new ones.
	// +optional
	// +patchStrategy=retainKeys
	//Strategy DeploymentStrategy `json:"strategy,omitempty" patchStrategy:"retainKeys" protobuf:"bytes,4,opt,name=strategy"`

	// Minimum number of seconds for which a newly created pod should be ready
	// without any of its container crashing, for it to be considered available.
	// Defaults to 0 (pod will be considered available as soon as it is ready)
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty" protobuf:"varint,5,opt,name=minReadySeconds"`

	// The number of old ReplicaSets to retain to allow rollback.
	// This is a pointer to distinguish between explicit zero and not specified.
	// Defaults to 10.
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty" protobuf:"varint,6,opt,name=revisionHistoryLimit"`

	// Indicates that the deployment is paused.
	// +optional
	Paused bool `json:"paused,omitempty" protobuf:"varint,7,opt,name=paused"`

	// The maximum time in seconds for a deployment to make progress before it
	// is considered to be failed. The deployment controller will continue to
	// process failed deployments and a condition with a ProgressDeadlineExceeded
	// reason will be surfaced in the deployment status. Note that progress will
	// not be estimated during the time a deployment is paused. Defaults to 600s.
	ProgressDeadlineSeconds *int32 `json:"progressDeadlineSeconds,omitempty" protobuf:"varint,9,opt,name=progressDeadlineSeconds"`

	Config ClusterConfig `json:"config,omitempty" protobuf:"bytes,10,opt,name=config"`
}

const (
	FeatureSupportNodeGroups = "nodegroups"
)

type ClusterConfig struct {
	Features map[string]string `json:"features,omitempty" protobuf:"bytes,1,rep,name=features"`
	// Token bootstrap with expiration of 2h
	Token       string              `json:"token,omitempty"`
	Etcd        Etcd                `json:"etcd,omitempty"`
	Kubernetes  Kubernetes          `json:"kubernetes,omitempty"`
	Runtime     Runtime             `json:"runtime,omitempty"`
	Registry    string              `json:"registry,omitempty" `
	Sans        []string            `json:"sans,omitempty" `
	ImageId     string              `json:"imageId,omitempty" `
	Network     NetworkCfg          `json:"network,omitempty"`
	InfraRef    string              `json:"infraRef,omitempty"`
	Description string              `json:"description,omitempty"`
	TLS         map[string]*KeyCert `json:"tls,omitempty"`
	Addons      []*Addon            `json:"addons,omitempty"`
	Namespace   string              `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
	CloudType   string              `json:"cloudType,omitempty" protobuf:"bytes,4,opt,name=cloudType"`
}

func (cfg *ClusterConfig) HasFeature(feature string) bool {
	return HasFeature(cfg.Features, feature)
}

func HasFeature(features map[string]string, feature string) bool {
	_, ok := features[feature]
	return ok
}

func (cfg *ClusterConfig) SetAddon(addon *Addon) {
	var addons []*Addon
	for _, a := range cfg.Addons {
		if a.Name == addon.Name {
			continue
		}
		addons = append(addons, a)
	}
	addons = append(addons, addon)
	cfg.Addons = addons
}

// MasterTemplateSpec describes the data a master should have when created from a template
type MasterTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the master.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec MasterSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// MasterSetStatus defines the observed state of MasterSet
type MasterSetStatus struct {
	AddonInitialized bool `json:"addonInitialized,omitempty" protobuf:"bytes,1,opt,name=addonInitialized"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	AccessPoint AccessPoint `json:"accessPoint,omitempty" protobuf:"bytes,2,opt,name=accessPoint"`
}

type AccessPoint struct {
	APIDomain  string     `json:"apiDomain,omitempty"`
	Internet   string     `json:"internet,omitempty"`
	Intranet   string     `json:"intranet,omitempty"`
	APIPort    string     `json:"apiPort,omitempty"`
	TunnelPort string     `json:"tunnelPort,omitempty"`
	Backends   []Endpoint `json:"backends,omitempty"`
}

type Endpoint struct {
	Id string `json:"id,omitempty"`
	Ip string `json:"ip,omitempty"`
}

func NewMasterSet(cluster *Cluster) *MasterSet {

	return &MasterSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "MasterSet",
			APIVersion: GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: cluster.Name,
		},
		Spec: cluster.Spec.MasterSpec,
	}
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MasterSet is the Schema for the mastersets API
type MasterSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MasterSetSpec   `json:"spec,omitempty"`
	Status MasterSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MasterSetList contains a list of MasterSet
type MasterSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MasterSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MasterSet{}, &MasterSetList{})
}
