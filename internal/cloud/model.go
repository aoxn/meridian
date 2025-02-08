package cloud

import (
	"fmt"
)

type VpcModel struct {
	Region       string `yaml:"region"`
	VpcId        string `json:"vpcId,omitempty"`
	VpcName      string `json:"vpcName,omitempty"`
	Cidr         string `json:"cidr,omitempty"`
	RouterId     string `json:"routerId,omitempty"`
	RouteTableId string `json:"routeTableId,omitempty"`
	Tag          []Tag  `json:"tag,omitempty"`
}

type VSwitchModel struct {
	ZoneId      string `json:"zoneId,omitempty"`
	VSwitchId   string `json:"vswitchId,omitempty"`
	VSwitchName string `json:"vswitchName,omitempty"`
	CidrBlock   string `json:"cidrBlock,omitempty"`

	Tag []Tag `json:"tag,omitempty"`
}

type InstanceModel struct {
	Tag []Tag `json:"tag,omitempty"`
}

type SecurityGroupModel struct {
	Region            string `yaml:"region"`
	SecurityGroupId   string `json:"securityGroupId,omitempty"`
	SecurityGroupName string `json:"securityGroupName,omitempty"`
	Tag               []Tag  `json:"tag,omitempty"`
}

type ScalingGroupModel struct {
	Region           string         `json:"region,omitempty"`
	VSwitchId        []VSwitchModel `json:"vswitchId,omitempty"`
	ScalingGroupId   string         `json:"scalingGroupId,omitempty"`
	ScalingGroupName string         `json:"scalingGroupName,omitempty"`
	Min              int            `json:"min,omitempty"`
	Max              int            `json:"max,omitempty"`
	ScalingConfig    ScalingConfig  `json:"scalingConfig,omitempty"`
	ScalingRule      ScalingRule    `json:"scalingRule,omitempty"`
	Tag              []Tag          `json:"tag,omitempty"`
}

type ScalingConfig struct {
	ScalingCfgId   string `json:"scalingCfgId,omitempty"`
	ScalingCfgName string `json:"scalingCfgName,omitempty"`

	Tag      []Tag  `json:"tag,omitempty"`
	UserData string `json:"userData,omitempty"`

	RamRole       string `json:"ramRole,omitempty"`
	ImageId       string `json:"imageId,omitempty"`
	SecurityGrpId string `json:"securityGrpId,omitempty"`
	InstanceType  string `json:"instanceType,omitempty"`
}

type ScalingRule struct {
	Region          string `json:"region,omitempty"`
	ScalingRuleId   string `json:"scalingRuleId,omitempty"`
	ScalingRuleName string `json:"scalingRuleName,omitempty"`
	ScalingRuleAri  string `json:"ScalingRuleAri,omitempty"`
}

type Id struct {
	Id     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Tag    []Tag  `json:"tag,omitempty"`
	Region string `json:"region,omitempty"`
}

func (id *Id) String() string {
	return fmt.Sprintf("[%s:%s]", id.Name, id.Id)
}

type Tag struct {
	Key   string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type SlbModel struct {
	VSwitchId string
	Tag       []Tag `json:"tag,omitempty"`
}

type RamModel struct {
	RamId   string `json:"ramId,omitempty"`
	RamName string `json:"ramName,omitempty"`
	Arn     string `json:"arn,omitempty"`
	//AssumeRolePolicyDocument string `json:"document,omitempty"`
	//Policy                   string `json:"policy,omitempty"`
	PolicyName string `json:"policyName,omitempty"`
}

type EipModel struct {
	Region       string `yaml:"region"`
	EipId        string `json:"eipId,omitempty"`
	EipName      string `json:"eipName,omitempty"`
	Address      string `json:"address,omitempty"`
	BindMode     string // "NAT"
	InstanceId   string
	InstanceType string // "SlbInstance", "Nat"
	Tag          []Tag  `json:"tag,omitempty"`
}
