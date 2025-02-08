package alibaba

import (
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/alibaba/client"
	esspvd "github.com/aoxn/meridian/internal/cloud/alibaba/ess"
	oss2 "github.com/aoxn/meridian/internal/cloud/alibaba/oss"
	"github.com/aoxn/meridian/internal/cloud/alibaba/sgrp"
	slb2 "github.com/aoxn/meridian/internal/cloud/alibaba/slb"
	"github.com/aoxn/meridian/internal/cloud/alibaba/vpc"
)

const Key = "alibaba"

func init() {
	cloud.Add(Key, New)
}

func New(cfg cloud.Config) (cloud.Cloud, error) {
	mgr, err := client.NewClientMgr(cfg.AuthInfo)
	if err != nil {
		return nil, err
	}
	return &alibaba{
		cfg:                  cfg,
		IVSwitch:             vpc.NewVswitch(mgr),
		IVpc:                 vpc.NewVpc(mgr),
		ISlb:                 slb2.NewSLB(mgr),
		IElasticScalingGroup: esspvd.NewESS(mgr),
		IEip:                 vpc.NewEip(mgr),
		IRamRole:             vpc.NewRamrole(mgr),
		IInstance:            esspvd.NewInstance(mgr),
		IObjectStorage:       oss2.NewOSS(mgr),
		ISecurityGroup:       sgrp.NewSecurityGrp(mgr),
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

type alibaba struct {
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

func (a *alibaba) GetConfig() cloud.Config {
	return a.cfg
}
