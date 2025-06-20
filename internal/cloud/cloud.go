package cloud

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type Config struct {
	v1.AuthInfo
}

type New func(config Config) (Cloud, error)

var cloud = map[string]New{}

func NewCloudBy(
	pv v1.Provider,
) (Cloud, error) {
	pdFunc, err := Get(pv.Spec.Type)
	if err != nil {
		return nil, err
	}
	return pdFunc(Config{AuthInfo: pv.Spec.AuthInfo})
}

func NewCloud(
	r client.Client,
	pvd string,
) (Cloud, error) {
	if pvd == "" {
		return nil, fmt.Errorf("empty provider specified")
	}
	var pv v1.Provider
	err := r.Get(context.TODO(), client.ObjectKey{Name: pvd}, &pv)
	if err != nil {
		return nil, err
	}
	return NewCloudBy(pv)
}

func Get(name string) (New, error) {
	fn, ok := cloud[name]
	if !ok {
		return nil, fmt.Errorf("cloud [%s] NotFound", name)
	}
	return fn, nil
}

func Add(key string, fn New) {
	if _, ok := cloud[key]; ok {
		klog.Infof("Warning: cloud service %s already registerd, override", key)
	}
	cloud[key] = fn
}

type Cloud interface {
	IConfig
	IVSwitch
	IVpc
	ISlb
	IElasticScalingGroup
	IEip
	IRamRole
	IInstance
	IObjectStorage
	ISecurityGroup
}

type IConfig interface {
	GetConfig() Config
}

type IElasticScalingGroup interface {
	FindESSBy(ctx context.Context, id Id) (ScalingGroupModel, error)
	ListESS(ctx context.Context, id Id) ([]ScalingGroupModel, error)
	CreateESS(ctx context.Context, id string, ess ScalingGroupModel) (string, error)
	UpdateESS(ctx context.Context, ess ScalingGroupModel) error
	DeleteESS(ctx context.Context, essid ScalingGroupModel) error

	ScaleNodeGroup(ctx context.Context, model ScalingGroupModel, desired uint) error

	FindScalingConfig(ctx context.Context, id Id) (ScalingConfig, error)
	FindScalingRule(ctx context.Context, id Id) (ScalingRule, error)

	CreateScalingConfig(ctx context.Context, id string, cfg ScalingConfig) (string, error)
	CreateScalingRule(ctx context.Context, id string, rule ScalingRule) (ScalingRule, error)
	ExecuteScalingRule(ctx context.Context, id string) (string, error)

	DeleteScalingConfig(ctx context.Context, cfgId string) error
	DeleteScalingRule(ctx context.Context, ruleId string) error

	EnableScalingGroup(ctx context.Context, gid, sid string) error
	//DescribeScalingGroups(ctx context.Context, id Tag) (ScalingGroupModel, error)
}

type IVpc interface {
	FindVPC(ctx context.Context, id Id) (VpcModel, error)
	ListVPC(ctx context.Context, id Id) ([]VpcModel, error)
	CreateVPC(ctx context.Context, vpc VpcModel) (string, error)
	UpdateVPC(ctx context.Context, vpc VpcModel) error
	DeleteVPC(ctx context.Context, id string) error
}

type IVSwitch interface {
	FindVSwitch(ctx context.Context, vpcid string, id Id) (VSwitchModel, error)
	ListVSwitch(ctx context.Context, vpcid string, id Id) ([]VSwitchModel, error)
	CreateVSwitch(ctx context.Context, vpcid string, model VSwitchModel) (string, error)
	UpdateVSwitch(ctx context.Context, vpcid string, model VSwitchModel) error
	DeleteVSwitch(ctx context.Context, vpcId string, id Id) error
}

type IEip interface {
	FindEIP(ctx context.Context, id Id) (EipModel, error)
	ListEIP(ctx context.Context, id Id) ([]EipModel, error)
	CreateEIP(ctx context.Context, m EipModel) (string, error)
	UpdateEIP(ctx context.Context, m EipModel) error
	DeleteEIP(ctx context.Context, id Id) error

	BindEIP(ctx context.Context, do EipModel) error
}

type IRamRole interface {
	FindRAM(ctx context.Context, id Id) (RamModel, error)
	ListRAM(ctx context.Context, id Id) ([]RamModel, error)
	CreateRAM(ctx context.Context, m RamModel) (string, error)
	UpdateRAM(ctx context.Context, m RamModel) error
	DeleteRAM(ctx context.Context, id Id, policyName string) error

	FindPolicy(ctx context.Context, m Id) (RamModel, error)
	CreatePolicy(ctx context.Context, m RamModel) (RamModel, error)
	AttachPolicyToRole(ctx context.Context, m RamModel) (RamModel, error)
	ListPoliciesForRole(ctx context.Context, m RamModel) (RamModel, error)
}

type ISecurityGroup interface {
	FindSecurityGroup(ctx context.Context, vpcid string, id Id) (SecurityGroupModel, error)
	ListSecurityGroup(ctx context.Context, vpcid string, id Id) ([]SecurityGroupModel, error)
	CreateSecurityGroup(ctx context.Context, vpcid string, grp SecurityGroupModel) (string, error)
	UpdateSecurityGroup(ctx context.Context, grp SecurityGroupModel) error
	DeleteSecurityGroup(ctx context.Context, id Id) error
}

type IInstance interface {
	GetInstanceId(node *corev1.Node) string
	FindInstance(ctx context.Context, id Id) (InstanceModel, error)
	ListInstance(ctx context.Context, i Id) ([]InstanceModel, error)
	CreateInstance(ctx context.Context, i InstanceModel) (string, error)
	UpdateInstance(ctx context.Context, i InstanceModel) error
	DeleteInstance(ctx context.Context, id Id) error

	RunCommand(ctx context.Context, id Id, command string) (string, error)
}

//type IRouteTable interface {
//	FindRouteTable(context.Context, *Request) (Response, error)
//	ListRouteTable(context.Context, *Request) (Response, error)
//	CreateRouteTable(context.Context, *Request) (Response, error)
//	UpdateRouteTable(context.Context, *Request) (Response, error)
//	DeleteRouteTable(context.Context, *Request) (Response, error)
//}
//
//type IRouteEntry interface {
//	FindRouteEntry(context.Context, *Request) (Response, error)
//	ListRouteEntry(context.Context, *Request) (Response, error)
//	CreateRouteEntry(context.Context, *Request) (Response, error)
//	UpdateRouteEntry(context.Context, *Request) (Response, error)
//	DeleteRouteEntry(context.Context, *Request) (Response, error)
//}

type ISlb interface {
	FindSLB(ctx context.Context, id Id) (SlbModel, error)
	ListSLB(ctx context.Context, id Id) ([]SlbModel, error)
	CreateSLB(ctx context.Context, b SlbModel) (string, error)
	UpdateSLB(ctx context.Context, b SlbModel) error
	DeleteSLB(ctx context.Context, id Id) error

	FindListener(ctx context.Context, id Id) (SlbModel, error)
	CreateListener(ctx context.Context, b SlbModel) (string, error)
	UpdateListener(ctx context.Context, b SlbModel) error
	DeleteListener(ctx context.Context, id Id) error
}

type IObjectStorage interface {
	BucketName() string
	EnsureBucket(name string) error
	GetFile(src, dst string) error
	PutFile(src, dst string) error
	DeleteObject(f string) error
	GetObject(src string) ([]byte, error)
	PutObject(b []byte, dst string) error
	ListObject(prefix string) ([][]byte, error)
}

var NotFound = fmt.Errorf("NotFound")

// IMetaData metadata interface
type IMetaData interface {
	HostName() (string, error)
	ImageID() (string, error)
	InstanceID() (string, error)
	Mac() (string, error)
	NetworkType() (string, error)
	OwnerAccountID() (string, error)
	PrivateIPv4() (string, error)
	Region() (string, error)
	SerialNumber() (string, error)
	SourceAddress() (string, error)
	VpcCIDRBlock() (string, error)
	VpcID() (string, error)
	VswitchCIDRBlock() (string, error)
	Zone() (string, error)
	NTPConfigServers() ([]string, error)
	RoleName() (string, error)
	RamRoleToken(role string) (RoleAuth, error)
	VswitchID() (string, error)
	// values from cloud config file
	ClusterID() string
}

type RoleAuth struct {
	AccessKeyId     string
	AccessKeySecret string
	Expiration      time.Time
	SecurityToken   string
	LastUpdated     time.Time
	Code            string
}
