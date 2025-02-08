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
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	FAILED  = "Failed"
	Running = "Running"
	Stopped = "Stopped"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GuestInfoSpec defines the desired state of GuestInfo
type GuestInfoSpec struct {
	Address []string `yaml:"address,omitempty" json:"address,omitempty"`
}

func (t *GuestInfo) SetEvent(v Event) {
	if v.Resource == "" {
		klog.Infof("empty Event resource name: %+v", v)
		return
	}
	t.Status.Events = append(t.Status.Events, v)
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GuestInfo is the Schema for the tasks API
type GuestInfo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GuestInfoSpec   `json:"spec,omitempty"`
	Status GuestInfoStatus `json:"status,omitempty"`
}

// GuestInfoStatus defines the observed state of GuestInfo
type GuestInfoStatus struct {
	Events     []Event            `json:"events,omitempty"`
	Phase      string             `json:"phase,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true

// GuestInfoList contains a list of GuestInfo
type GuestInfoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GuestInfo `json:"items"`
}

func GenGuestInfoName(id, action string) string {
	return strings.ToUpper(fmt.Sprintf("%s_%s_%s", id, action, time.Now().Format("20060102150405")))
}

func (t *GuestInfo) SetTypeMeta() *GuestInfo {
	t.TypeMeta = metav1.TypeMeta{
		Kind:       "GuestInfo",
		APIVersion: GroupVersion.String(),
	}
	return t
}

func EmptyGI(k string) *GuestInfo {
	vm := &GuestInfo{
		ObjectMeta: metav1.ObjectMeta{Name: k},
	}
	vm.SetTypeMeta()
	return vm
}

func init() {
	SchemeBuilder.Register(&GuestInfo{}, &GuestInfoList{})
}
