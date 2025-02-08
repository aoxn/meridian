package vpc

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	vxc "github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/alibaba/client"
)

var _ cloud.IVpc = &vpc{}

func NewVpc(mgr *client.ClientMgr) cloud.IVpc {
	return &vpc{ClientMgr: mgr}
}

type vpc struct {
	*client.ClientMgr
}

func (n *vpc) FindVPC(ctx context.Context, id cloud.Id) (cloud.VpcModel, error) {
	var (
		req   = vxc.CreateDescribeVpcsRequest()
		model = cloud.VpcModel{}
	)
	req.RegionId = id.Region
	req.VpcId = id.Id

	req.VpcName = id.Name

	if len(id.Tag) > 0 {
		var tagList []vxc.DescribeVpcsTag
		for _, m := range id.Tag {
			tag := vxc.DescribeVpcsTag{
				Key:   m.Key,
				Value: m.Value,
			}
			tagList = append(tagList, tag)
		}
		req.Tag = &tagList
	}
	klog.V(5).Infof("[service] find vpc[request]: %v", req)
	r, err := n.VPC.DescribeVpcs(req)
	if err != nil {
		return model, err
	}
	if r != nil {
		if len(r.Vpcs.Vpc) == 0 {
			return model, cloud.NotFound
		}
		if len(r.Vpcs.Vpc) > 1 {
			klog.Infof("warning: mutilple vpc found by name [%v]", id.Name)
		}
		m := r.Vpcs.Vpc[0]
		if m.VpcId == "" {
			return model, fmt.Errorf("UnexpectedResponse")
		}
		model.VpcId = m.VpcId
		model.VpcName = m.VpcName
		model.Cidr = m.CidrBlock
		klog.V(5).Infof("[service] vpc found with id=[%s]", model.VpcId)
	}

	return model, nil
}

func (n *vpc) ListVPC(ctx context.Context, id cloud.Id) ([]cloud.VpcModel, error) {
	//TODO implement me
	panic("implement me")
}

func (n *vpc) CreateVPC(ctx context.Context, vpc cloud.VpcModel) (string, error) {

	req := vxc.CreateCreateVpcRequest()
	req.VpcName = vpc.VpcName
	req.Tag = &[]vxc.CreateVpcTag{
		{
			Key:   vpc.VpcName,
			Value: "meridian",
		},
	}
	req.CidrBlock = vpc.Cidr
	req.RegionId = vpc.Region

	klog.Infof("[service] create vpc: %s,%s,%s", req.RegionId, req.VpcName, req.CidrBlock)
	r, err := n.VPC.CreateVpc(req)
	if err != nil {
		return "", err
	}
	if r == nil || r.VpcId == "" {
		return "", fmt.Errorf("unexpected empty response")
	}
	waitOn := func(ctx context.Context) (done bool, err error) {

		creq := vxc.CreateDescribeVpcsRequest()
		creq.VpcId = r.VpcId
		creq.VpcName = vpc.VpcName
		creq.RegionId = vpc.Region

		if len(vpc.Tag) > 0 {
			var tagList []vxc.DescribeVpcsTag
			for _, m := range vpc.Tag {
				tag := vxc.DescribeVpcsTag{
					Key:   m.Key,
					Value: m.Value,
				}
				tagList = append(tagList, tag)
			}
			creq.Tag = &tagList
		}
		r, err := n.VPC.DescribeVpcs(creq)
		if err != nil {
			return false, err
		}
		for _, m := range r.Vpcs.Vpc {
			if m.Status == "Available" {
				return true, nil
			}
			klog.Infof("vpc not available yet: %v", m.Status)
		}
		return false, nil
	}
	return r.VpcId, wait.PollUntilContextTimeout(ctx, 2*time.Second, 1*time.Minute, false, waitOn)
}

func (n *vpc) UpdateVPC(ctx context.Context, vpc cloud.VpcModel) error {
	panic("implement me")
}

func (n *vpc) DeleteVPC(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("vpc id can not be empty")
	}
	klog.V(5).Infof("delete vpc by id: %s", id)
	req := vxc.CreateDeleteVpcRequest()
	req.VpcId = id
	_, err := n.VPC.DeleteVpc(req)
	return err
}
