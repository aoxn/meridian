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
	"net"
	"strings"
	"time"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VirtualMachineSpec defines the desired state of GuestInfo
type VirtualMachineSpec struct {
	GUI             bool              `yaml:"gui" json:"gui"`
	VMType          VMType            `yaml:"vmType,omitempty" json:"vmType,omitempty"`
	OS              OS                `yaml:"os,omitempty" json:"os,omitempty"`
	Arch            Arch              `yaml:"arch,omitempty" json:"arch,omitempty"`
	Image           ImageLocation     `yaml:"images" json:"images"` // REQUIRED
	CPUs            int               `yaml:"cpus,omitempty" json:"cpus,omitempty"`
	GuestVersion    string            `yaml:"guestVersion,omitempty" json:"guestVersion,omitempty"`
	Memory          string            `yaml:"memory,omitempty" json:"memory,omitempty"` // go-units.RAMInBytes
	Disk            string            `yaml:"disk,omitempty" json:"disk,omitempty"`     // go-units.RAMInBytes
	AdditionalDisks []Disk            `yaml:"additionalDisks,omitempty" json:"additionalDisks,omitempty"`
	Mounts          []Mount           `yaml:"mounts,omitempty" json:"mounts,omitempty"`
	MountInotify    bool              `yaml:"mountInotify,omitempty" json:"mountInotify,omitempty"`
	SSH             SSH               `yaml:"ssh,omitempty" json:"ssh,omitempty"`
	Firmware        Firmware          `yaml:"firmware,omitempty" json:"firmware,omitempty"`
	Audio           Audio             `yaml:"audio,omitempty" json:"audio,omitempty"`
	Video           Video             `yaml:"video,omitempty" json:"video,omitempty"`
	PortForwards    []PortForward     `yaml:"portForwards,omitempty" json:"portForwards,omitempty"`
	Message         string            `yaml:"message,omitempty" json:"message,omitempty"`
	Networks        []Network         `yaml:"networks,omitempty" json:"networks,omitempty"`
	Env             map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Param           map[string]string `yaml:"param,omitempty" json:"param,omitempty"`
	DNS             []net.IP          `yaml:"dns,omitempty" json:"dns,omitempty"`
	HostResolver    HostResolver      `yaml:"hostResolver,omitempty" json:"hostResolver,omitempty"`
	CACertificates  CACertificates    `yaml:"caCerts,omitempty" json:"caCerts,omitempty"`
	TimeZone        string            `yaml:"timezone,omitempty" json:"timezone,omitempty"`
}

func (s *VirtualMachineSpec) SetForward(ports ...PortForward) {
	for _, port := range ports {
		found := false
		for i := range s.PortForwards {
			if s.PortForwards[i].Rule() == port.Rule() {
				found = true
				s.PortForwards[i] = port
			}
		}
		if !found {
			s.PortForwards = append(s.PortForwards, port)
		}
	}
}

func (s *VirtualMachineSpec) RemoveForward(ports ...PortForward) {
	var forward []PortForward
	for _, v := range s.PortForwards {
		found := false
		for _, port := range ports {
			if v.Rule() == port.Rule() {
				found = true
				break
			}
		}
		if !found {
			forward = append(forward, v)
		}
	}
	s.PortForwards = forward
}

func (s *VirtualMachineSpec) SetMounts(p Mount) {
	found := false
	for i := range s.Mounts {
		if s.Mounts[i].Location == p.Location {
			found = true
			s.Mounts[i] = p
		}
	}
	if !found {
		s.Mounts = append(s.Mounts, p)
	}
}

const (
	VirtualMachineSuccess = "Success"
	VirtualMachineFail    = "Fail"
	VirtualMachineRunning = "Running"
)

func (t *VirtualMachine) SetEvent(v Event) {
	if v.Resource == "" {
		klog.Infof("empty Event resource name: %+v", v)
		return
	}
	t.Status.Events = append(t.Status.Events, v)
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// VirtualMachine is the Schema for the tasks API
type VirtualMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineSpec   `json:"spec,omitempty"`
	Status VirtualMachineStatus `json:"status,omitempty"`
}

// VirtualMachineStatus defines the observed state of GuestInfo
type VirtualMachineStatus struct {
	Events  []Event  `json:"events,omitempty"`
	Phase   string   `json:"phase,omitempty"`
	Message string   `json:"message,omitempty"`
	Address []string `json:"address,omitempty"`
}

//+kubebuilder:object:root=true

// VirtualMachineList contains a list of GuestInfo
type VirtualMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachine `json:"items"`
}

func GenVirtualMachineName(id, action string) string {
	return strings.ToUpper(fmt.Sprintf("%s_%s_%s", id, action, time.Now().Format("20060102150405")))
}

func (t *VirtualMachine) SetTypeMeta() *VirtualMachine {
	t.TypeMeta = metav1.TypeMeta{
		Kind:       "VirtualMachine",
		APIVersion: GroupVersion.String(),
	}
	return t
}

func MeridianClusterName(n string) string {
	return fmt.Sprintf("meridian.cluster.%s", n)
}

func MeridianUserName(n string) string {
	return fmt.Sprintf("meridian.user.%s", n)
}

func EmptyVM(k string) *VirtualMachine {
	vm := &VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Name: k},
	}
	vm.SetTypeMeta()
	return vm
}

func init() {
	SchemeBuilder.Register(&VirtualMachine{}, &VirtualMachineList{})
}
