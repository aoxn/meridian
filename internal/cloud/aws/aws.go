package aws

import (
	"time"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/aws/client"
	"github.com/aoxn/meridian/internal/cloud/aws/ec2"
	"github.com/aoxn/meridian/internal/cloud/aws/elb"
	"github.com/aoxn/meridian/internal/cloud/aws/iam"
	"github.com/aoxn/meridian/internal/cloud/aws/s3"
	"github.com/aoxn/meridian/internal/cloud/aws/sg"
)

const Key = "aws"

func init() {
	cloud.Add(Key, New)
}

func New(cfg cloud.Config) (cloud.Cloud, error) {
	mgr, err := client.NewClientMgr(cfg.AuthInfo)
	if err != nil {
		return nil, err
	}
	return &aws{
		cfg:                  cfg,
		IVSwitch:             ec2.NewSubnet(mgr),
		IVpc:                 ec2.NewVpc(mgr),
		ISlb:                 elb.NewELB(mgr),
		IElasticScalingGroup: ec2.NewAutoScalingGroup(mgr),
		IEip:                 ec2.NewEIP(mgr),
		IRamRole:             iam.NewIAMRole(mgr),
		IInstance:            ec2.NewInstance(mgr),
		IObjectStorage:       s3.NewS3(mgr),
		ISecurityGroup:       sg.NewSecurityGroup(mgr),
	}, nil
}

var (
	NotFound           = ErrorMsg{msg: "NotFound"}
	UnexpectedResponse = ErrorMsg{msg: "UnexpectedResponse"}
)

type ErrorMsg struct {
	msg string
}

func (e ErrorMsg) Error() string {
	return e.msg
}

type aws struct {
	cfg cloud.Config
	cloud.IVSwitch
	cloud.IVpc
	cloud.ISlb
	cloud.IElasticScalingGroup
	cloud.IEip
	cloud.IRamRole
	cloud.IInstance
	cloud.IObjectStorage
	cloud.ISecurityGroup
}

func (a *aws) GetConfig() cloud.Config {
	return a.cfg
}

// AWSNodeGroup represents an AWS Node Group configuration
type AWSNodeGroup struct {
	Name               string            `json:"name"`
	Region             string            `json:"region"`
	SubnetIDs          []string          `json:"subnetIds"`
	SecurityGroupIDs   []string          `json:"securityGroupIds"`
	AMIID              string            `json:"amiId"`
	InstanceTypes      []string          `json:"instanceTypes"`
	CapacityType       string            `json:"capacityType"` // on-demand, spot
	MinSize            int               `json:"minSize"`
	MaxSize            int               `json:"maxSize"`
	DesiredCapacity    int               `json:"desiredCapacity"`
	Tags               map[string]string `json:"tags"`
	UserData           string            `json:"userData"`
	KeyName            string            `json:"keyName"`
	VolumeSize         int               `json:"volumeSize"`
	VolumeType         string            `json:"volumeType"`
	IamInstanceProfile string            `json:"iamInstanceProfile"`
	KubeletConfig      *KubeletConfig    `json:"kubeletConfig"`
	LaunchTemplate     *LaunchTemplate   `json:"launchTemplate"`
}

type LaunchTemplate struct {
	Name                string               `json:"name"`
	Version             string               `json:"version"`
	InstanceType        string               `json:"instanceType"`
	AMIID               string               `json:"amiId"`
	KeyName             string               `json:"keyName"`
	SecurityGroupIDs    []string             `json:"securityGroupIds"`
	UserData            string               `json:"userData"`
	IamInstanceProfile  string               `json:"iamInstanceProfile"`
	BlockDeviceMappings []BlockDeviceMapping `json:"blockDeviceMappings"`
	Tags                map[string]string    `json:"tags"`
}

type BlockDeviceMapping struct {
	DeviceName string         `json:"deviceName"`
	EBS        EBSBlockDevice `json:"ebs"`
}

type EBSBlockDevice struct {
	VolumeSize          int    `json:"volumeSize"`
	VolumeType          string `json:"volumeType"`
	DeleteOnTermination bool   `json:"deleteOnTermination"`
	Encrypted           bool   `json:"encrypted"`
}

type KubeletConfig struct {
	ClusterDNS              []string                 `json:"clusterDNS"`
	MaxPods                 *int32                   `json:"maxPods"`
	PodsPerCore             *int32                   `json:"podsPerCore"`
	SystemReserved          map[string]string        `json:"systemReserved"`
	KubeReserved            map[string]string        `json:"kubeReserved"`
	EvictionHard            map[string]string        `json:"evictionHard"`
	EvictionSoft            map[string]string        `json:"evictionSoft"`
	EvictionSoftGracePeriod map[string]time.Duration `json:"evictionSoftGracePeriod"`
}

// AWSLoadBalancer represents an AWS Load Balancer configuration
type AWSLoadBalancer struct {
	Name           string            `json:"name"`
	Type           string            `json:"type"`   // application, network, classic
	Scheme         string            `json:"scheme"` // internet-facing, internal
	Subnets        []string          `json:"subnets"`
	SecurityGroups []string          `json:"securityGroups"`
	TargetGroups   []TargetGroup     `json:"targetGroups"`
	Listeners      []Listener        `json:"listeners"`
	Tags           map[string]string `json:"tags"`
}

type TargetGroup struct {
	Name                string            `json:"name"`
	Port                int32             `json:"port"`
	Protocol            string            `json:"protocol"`
	TargetType          string            `json:"targetType"` // instance, ip, lambda
	VpcID               string            `json:"vpcId"`
	HealthCheckPath     string            `json:"healthCheckPath"`
	HealthCheckPort     string            `json:"healthCheckPort"`
	HealthCheckProtocol string            `json:"healthCheckProtocol"`
	Tags                map[string]string `json:"tags"`
}

type Listener struct {
	Port           int32    `json:"port"`
	Protocol       string   `json:"protocol"`
	DefaultActions []Action `json:"defaultActions"`
	CertificateARN string   `json:"certificateArn"`
}

type Action struct {
	Type           string         `json:"type"`
	TargetGroupARN string         `json:"targetGroupArn"`
	ForwardConfig  *ForwardConfig `json:"forwardConfig"`
}

type ForwardConfig struct {
	TargetGroups []TargetGroupTuple `json:"targetGroups"`
}

type TargetGroupTuple struct {
	TargetGroupARN string `json:"targetGroupArn"`
	Weight         int32  `json:"weight"`
}
