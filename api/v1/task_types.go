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

const (
	TypeCreate = "CreateCluster"
	TypeDelete = "DeleteCluster"
)

const (
	ResourceKindVM      = "vm"
	ResourceKindCluster = "cluster"
)

// TaskSpec defines the desired state of Task
type TaskSpec struct {
	Resource string `json:"resource"`
	Type     string `json:"type,omitempty"`
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ClusterName string `json:"clusterName,omitempty"`
}

const (
	TaskSuccess = "Success"
	TaskFail    = "Fail"
	TaskRunning = "Running"
)

// TaskStatus defines the observed state of Task
type TaskStatus struct {
	Events []Event `json:"events,omitempty"`
	Phase  string  `json:"phase,omitempty"`
}

type Event struct {
	RID      string      `json:"rid,omitempty"`
	Resource string      `json:"resource,omitempty"`
	Reason   string      `json:"reason,omitempty"`
	Time     metav1.Time `json:"time,omitempty"`
	Message  string      `json:"message,omitempty"`
}

func (t *Task) SetEvent(v Event) {
	if v.Resource == "" {
		klog.Infof("empty Event resource name: %+v", v)
		return
	}
	t.Status.Events = append(t.Status.Events, v)
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Task is the Schema for the tasks API
type Task struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TaskSpec   `json:"spec,omitempty"`
	Status TaskStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TaskList contains a list of Task
type TaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Task `json:"items"`
}

func GenTaskName(id, action string) string {
	return strings.ToUpper(fmt.Sprintf("%s_%s_%s", id, action, time.Now().Format("20060102150405")))
}

func (t *Task) Init() *Task {
	t.TypeMeta = metav1.TypeMeta{
		Kind:       "Task",
		APIVersion: GroupVersion.String(),
	}
	return t
}

func init() {
	SchemeBuilder.Register(&Task{}, &TaskList{})
}
