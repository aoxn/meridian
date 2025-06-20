package mock

import (
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/mock/client"
	"github.com/aoxn/meridian/internal/cloud/mock/ess"
	"github.com/aoxn/meridian/internal/cloud/mock/oss"
	"github.com/aoxn/meridian/internal/cloud/mock/ram"
	"github.com/aoxn/meridian/internal/cloud/mock/sg"
	"github.com/aoxn/meridian/internal/cloud/mock/slb"
	"github.com/aoxn/meridian/internal/cloud/mock/vpc"
)

const Key = "mock"

func init() {
	cloud.Add(Key, New)
}

func New(cfg cloud.Config) (cloud.Cloud, error) {
	mgr := client.NewClientMgr()
	return &mockCloud{
		cfg:                  cfg,
		IVSwitch:             vpc.NewVswitch(mgr),
		IVpc:                 vpc.NewVpc(mgr),
		ISlb:                 slb.NewSLB(mgr),
		IElasticScalingGroup: ess.NewESS(mgr),
		IEip:                 vpc.NewEip(mgr),
		IRamRole:             ram.NewRAM(mgr),
		IInstance:            ess.NewInstance(mgr),
		IObjectStorage:       oss.NewOSS(mgr),
		ISecurityGroup:       sg.NewSecurityGrp(mgr),
	}, nil
}

type mockCloud struct {
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

func (m *mockCloud) GetConfig() cloud.Config {
	return m.cfg
}
