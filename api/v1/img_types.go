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
	"k8s.io/klog/v2"
	"strings"
	"time"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ImageSpec defines the desired state of GuestInfo
type ImageSpec struct {
	Name string `json:"name"`
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

func (t *Image) SetEvent(v Event) {
	if v.Resource == "" {
		klog.Infof("empty Event resource name: %+v", v)
		return
	}
	t.Status.Events = append(t.Status.Events, v)
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Image is the Schema for the tasks API
type Image struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ImageSpec   `json:"spec,omitempty"`
	Status ImageStatus `json:"status,omitempty"`
}

// ImageStatus defines the observed state of GuestInfo
type ImageStatus struct {
	Events  []Event  `json:"events,omitempty"`
	Phase   string   `json:"phase,omitempty"`
	Address []string `json:"address,omitempty"`
}

//+kubebuilder:object:root=true

// ImageList contains a list of GuestInfo
type ImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Image `json:"items"`
}

func GenImageName(id, action string) string {
	return strings.ToUpper(fmt.Sprintf("%s_%s_%s", id, action, time.Now().Format("20060102150405")))
}

func (t *Image) SetTypeMeta() *Image {
	t.TypeMeta = metav1.TypeMeta{
		Kind:       "Image",
		APIVersion: GroupVersion.String(),
	}
	return t
}

func EmptyImage(k string) *Image {
	vm := &Image{
		ObjectMeta: metav1.ObjectMeta{Name: k},
	}
	vm.SetTypeMeta()
	return vm
}

func init() {
	SchemeBuilder.Register(&Image{}, &ImageList{})
}
