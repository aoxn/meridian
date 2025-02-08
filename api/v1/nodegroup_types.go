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
	"math/rand"
	"strings"
	"time"
)

func RandomID(strlen int) string {
	const asciiA = 65
	const asciiZ = 90
	rand.Seed(time.Now().UTC().UnixNano())
	result := make([]byte, strlen)
	for i := 0; i < strlen; i++ {
		result[i] = byte(randInt(asciiA, asciiZ))
	}
	return strings.ToLower(string(result))
}

func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodeGroupSpec defines the desired state of GuestInfo
type NodeGroupSpec struct {
	Region       string `json:"region,omitempty"`
	Provider     string `json:"provider,omitempty"`
	VpcId        string `json:"vpcId,omitempty"`
	VpcName      string `json:"vpcName,omitempty"`
	Cidr         string `json:"cidr,omitempty"`
	RouterId     string `json:"routerId,omitempty"`
	RouteTableId string `json:"routeTableId,omitempty"`

	RamRole       RamRole             `json:"ram,omitempty"`
	Eip           []Eip               `json:"eip,omitempty"`
	VSwitch       []VSwitch           `json:"vswitch,omitempty"`
	SecurityGroup SecurityGroup       `json:"securityGroup,omitempty"`
	ScalingGroup  ElasticScalingGroup `json:"scalingGroup,omitempty"`
	NodeConfig    NodeConfig          `json:"nodeConfig,omitempty"`

	Replicas uint     `json:"replicas"`
	CPUs     int      `json:"cpus,omitempty"`
	Memory   int      `json:"memory,omitempty"`
	Addons   []*Addon `json:"addons,omitempty"`
}

type NodeConfig struct {
}

type VSwitch struct {
	VSwitchId   string `json:"vswitchId,omitempty"`
	VSwitchName string `json:"vswitchName,omitempty"`
	CidrBlock   string `json:"cidrBlock,omitempty"`
	ZoneId      string `json:"zoneId,omitempty"`
}

type Eip struct {
	EipId   string `json:"eipId,omitempty"`
	EipName string `json:"eipName,omitempty"`
	//Ref      string `json:"ref,omitempty"`
	Address string `json:"address,omitempty"`
}

type SecurityGroup struct {
	SecurityGroupId   string `json:"securityGroupId,omitempty"`
	SecurityGroupName string `json:"securityGroupName,omitempty"`
}

type ElasticScalingGroup struct {
	ScalingGroupId   string        `json:"scalingGroupId,omitempty"`
	ScalingGroupName string        `json:"scalingGroupName,omitempty"`
	Min              int           `json:"min,omitempty"`
	Max              int           `json:"max,omitempty"`
	ImageId          string        `json:"imageId,omitempty"`
	InstanceType     string        `json:"instanceType,omitempty"`
	ScalingConfig    ScalingConfig `json:"scalingConfig,omitempty"`
	ScalingRule      ScalingRule   `json:"scalingRule,omitempty"`
}

type ScalingConfig struct {
	ScalingCfgId   string `json:"scalingCfgId,omitempty"`
	ScalingCfgName string `json:"scalingCfgName,omitempty"`
}

type ScalingRule struct {
	ScalingRuleName string `json:"scalingRuleName,omitempty"`
	ScalingRuleId   string `json:"scalingRuleId,omitempty"`
	ScalingRuleAri  string `json:"scalingRuleAri,omitempty"`
}

type RamRole struct {
	RoleId   string `json:"ramId,omitempty"`
	RoleName string `json:"roleName,omitempty"`
	Arn      string `json:"arn,omitempty"`
	//AssumeRolePolicyDocument string `json:"document,omitempty"`
	PolicyName string `json:"policyName,omitempty"`
}

var Finalizer = "nodegroups"

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NodeGroup is the Schema for the tasks API
type NodeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeGroupSpec   `json:"spec,omitempty"`
	Status NodeGroupStatus `json:"status,omitempty"`
}

// NodeGroupStatus defines the observed state of GuestInfo
type NodeGroupStatus struct {
	Replicas uint     `json:"replicas,omitempty"`
	Events   []Event  `json:"events,omitempty"`
	Phase    string   `json:"phase,omitempty"`
	Address  []string `json:"address,omitempty"`
	Addons   []*Addon `json:"addons,omitempty"`
}

//+kubebuilder:object:root=true

// NodeGroupList contains a list of GuestInfo
type NodeGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeGroup `json:"items"`
}

func GenNodeGroupName(id, action string) string {
	return strings.ToUpper(fmt.Sprintf("%s_%s_%s", id, action, time.Now().Format("20060102150405")))
}

func (t *NodeGroup) SetTypeMeta() *NodeGroup {
	t.TypeMeta = metav1.TypeMeta{
		Kind:       "NodeGroup",
		APIVersion: GroupVersion.String(),
	}
	return t
}

func EmptyNG(k string) *NodeGroup {
	ng := &NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: k},
	}
	ng.SetTypeMeta()
	return ng
}

func init() {
	SchemeBuilder.Register(&NodeGroup{}, &NodeGroupList{})
}
