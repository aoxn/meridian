package sgrp

import (
	"context"
	"fmt"
	"k8s.io/klog/v2"
	"strings"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/alibaba/client"
)

var _ cloud.ISecurityGroup = &securityGrpProvider{}

func NewSecurityGrp(mgr *client.ClientMgr) cloud.ISecurityGroup {
	return &securityGrpProvider{ClientMgr: mgr}
}

type securityGrpProvider struct {
	*client.ClientMgr
}

func (n *securityGrpProvider) FindSecurityGroup(ctx context.Context, vpcId string, id cloud.Id) (cloud.SecurityGroupModel, error) {
	model := cloud.SecurityGroupModel{}
	if vpcId == "" {
		return model, fmt.Errorf("[find securityGrp]vpcid must be provided")
	}
	req := ecs.CreateDescribeSecurityGroupsRequest()
	req.VpcId = vpcId

	req.SecurityGroupId = id.Id

	r, err := n.ECS.DescribeSecurityGroups(req)
	if err != nil {
		return model, err
	}
	if len(r.SecurityGroups.SecurityGroup) == 0 {
		return model, cloud.NotFound
	}
	if len(r.SecurityGroups.SecurityGroup) > 1 {
		klog.Infof("[service]warning: mutilple securityGrp found by name[%s]", id)
	}
	s := r.SecurityGroups.SecurityGroup[0]
	model.SecurityGroupId = s.SecurityGroupId
	klog.Infof("[service] find securityGrp: %s", id)
	return model, nil
}

func (n *securityGrpProvider) ListSecurityGroup(ctx context.Context, vpcId string, id cloud.Id) ([]cloud.SecurityGroupModel, error) {
	//TODO implement me
	panic("implement me")
}

func (n *securityGrpProvider) CreateSecurityGroup(ctx context.Context, vpcId string, grp cloud.SecurityGroupModel) (string, error) {
	if vpcId == "" {
		return "", fmt.Errorf("[create securityGrp]vpcid must be provided")
	}
	req := ecs.CreateCreateSecurityGroupRequest()
	req.VpcId = vpcId
	req.SecurityGroupName = grp.SecurityGroupName
	req.RegionId = grp.Region

	klog.Infof("[service] create securityGrp: %s", req.SecurityGroupName)
	r, err := n.ECS.CreateSecurityGroup(req)
	if err != nil {
		return "", err
	}
	return r.SecurityGroupId, err
}

func (n *securityGrpProvider) UpdateSecurityGroup(ctx context.Context, grp cloud.SecurityGroupModel) error {
	//TODO implement me
	panic("implement me")
}

func (n *securityGrpProvider) DeleteSecurityGroup(ctx context.Context, id cloud.Id) error {
	if id.Region == "" || id.Id == "" {
		return fmt.Errorf("[delete securityGrp]id or region is empty")
	}
	req := ecs.CreateDeleteSecurityGroupRequest()
	req.RegionId = id.Region
	req.SecurityGroupId = id.Id
	_, err := n.ECS.DeleteSecurityGroup(req)
	if err != nil {
		if strings.Contains(err.Error(), "is not found") {
			return nil
		}
	}
	return err
}
