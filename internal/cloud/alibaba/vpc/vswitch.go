package vpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	vxc "github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/alibaba/client"
)

var _ cloud.IVSwitch = &vswitchProvider{}

func NewVswitch(mgr *client.ClientMgr) cloud.IVSwitch {
	return &vswitchProvider{ClientMgr: mgr}
}

type vswitchProvider struct {
	*client.ClientMgr
}

func (n *vswitchProvider) FindVSwitch(ctx context.Context, vpcId string, id cloud.Id) (cloud.VSwitchModel, error) {
	var (
		vswitchId   = id.Id
		vswitchName = id.Name
		model       = cloud.VSwitchModel{}
	)
	if vpcId == "" {
		return model, fmt.Errorf("(find vswitch) vpcid must be provided")
	}

	req := vxc.CreateDescribeVSwitchesRequest()
	req.VpcId = id.Id
	req.VSwitchId = vswitchId
	req.VSwitchName = vswitchName
	if len(id.Tag) > 0 {
		var tagList []vxc.DescribeVSwitchesTag
		for _, m := range id.Tag {
			tag := vxc.DescribeVSwitchesTag{
				Key:   m.Key,
				Value: m.Value,
			}
			tagList = append(tagList, tag)
		}
		req.Tag = &tagList
	}

	r, err := n.VPC.DescribeVSwitches(req)
	if err != nil {
		return model, err
	}
	if r != nil {
		if len(r.VSwitches.VSwitch) == 0 {
			return model, cloud.NotFound
		}
		if len(r.VSwitches.VSwitch) > 1 {
			klog.Infof("[service]warning: mutilple vpc found by name [%s]", id)
		}
		m := r.VSwitches.VSwitch[0]
		if m.VSwitchId == "" {
			return model, fmt.Errorf("UnexpectedResponse")
		}
		model.VSwitchId = m.VSwitchId
		model.VSwitchName = m.VSwitchName
		model.CidrBlock = m.CidrBlock
		klog.V(5).Infof("[service] vswitch found with id=%s", id.Id)
	}
	return model, nil
}

func (n *vswitchProvider) ListVSwitch(ctx context.Context, vpcId string, id cloud.Id) ([]cloud.VSwitchModel, error) {
	//TODO implement me
	panic("implement me")
}

func (n *vswitchProvider) CreateVSwitch(ctx context.Context, vpcId string, v cloud.VSwitchModel) (string, error) {
	if vpcId == "" || v.VSwitchName == "" || v.CidrBlock == "" {
		return "", fmt.Errorf("[service]create vswitch, error: vpcid must be provided")
	}
	if v.VSwitchId != "" {
		klog.Infof("vswitch already exists: %s", v.VSwitchId)
		return "", nil
	}
	var (
		r     *vxc.CreateVSwitchResponse
		req   = vxc.CreateCreateVSwitchRequest()
		model = cloud.VSwitchModel{}
	)
	req.VpcId = vpcId
	req.VSwitchName = v.VSwitchName
	req.ZoneId = v.ZoneId
	req.CidrBlock = v.CidrBlock
	if len(v.Tag) > 0 {
		var tagList []vxc.CreateVSwitchTag
		for _, m := range v.Tag {
			tag := vxc.CreateVSwitchTag{
				Key:   m.Key,
				Value: m.Value,
			}
			tagList = append(tagList, tag)
		}
		req.Tag = &tagList
	}

	klog.Infof("[service] create vswitch: %s, %s", req.VSwitchName, req.CidrBlock)
	//err := wait.PollUntilContextTimeout(
	//	ctx, 1*time.Second, 30*time.Second, true,
	//	func(ctx context.Context) (done bool, err error) {
	//		r, err = n.VPC.CreateVSwitch(&req)
	//		if err != nil {
	//			if strings.Contains(err.Error(), "TaskConflict") {
	//				klog.Infof("retry on TaskConflict: %s", err.Error())
	//				return false, nil
	//			}
	//			return true, err
	//		}
	//		if r != nil && r.Body != nil {
	//			model.VSwitchId = tea.StringValue(r.Body.VSwitchId)
	//			return true, nil
	//		}
	//		klog.Errorf("[service]unexpected empty response in creating vswitch")
	//		return true, nil
	//	},
	//)

	r, err := n.VPC.CreateVSwitch(req)
	if err != nil {
		return "", err
	}
	time.Sleep(5 * time.Second)
	model.VSwitchId = r.VSwitchId
	waitOn := func(ctx context.Context) (done bool, err error) {
		req := vxc.CreateDescribeVSwitchesRequest()
		req.VpcId = vpcId
		req.VSwitchId = r.VSwitchId
		if len(model.Tag) > 0 {
			var tagList []vxc.DescribeVSwitchesTag
			for _, m := range model.Tag {
				tag := vxc.DescribeVSwitchesTag{
					Key:   m.Key,
					Value: m.Value,
				}
				tagList = append(tagList, tag)
			}
		}
		r, err := n.VPC.DescribeVSwitches(req)
		if err != nil {
			return false, err
		}
		if r != nil {
			for _, m := range r.VSwitches.VSwitch {
				if m.VSwitchId == req.VSwitchId {
					if m.Status == "Available" {
						return true, nil
					}
				}
			}
		}
		klog.Infof("vswitch [%s] still unavailable", req.VSwitchId)
		return false, nil
	}
	return model.VSwitchId, wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, false, waitOn)
}

func (n *vswitchProvider) UpdateVSwitch(ctx context.Context, vpcid string, model cloud.VSwitchModel) error {
	//TODO implement me
	panic("implement me")
}

func (n *vswitchProvider) DeleteVSwitch(ctx context.Context, vpcId string, id cloud.Id) error {
	if id.Id == "" {
		return fmt.Errorf("unexpected empty vswitchid")
	}
	klog.V(5).Infof("delete vswitch by id: %s", id.Id)
	var vsw = vxc.CreateDeleteVSwitchRequest()

	vsw.VSwitchId = id.Id

	_, err := n.VPC.DeleteVSwitch(vsw)
	if err != nil {
		if strings.Contains(err.Error(), "not exist") {
			return nil
		}
		return err
	}
	waitOn := func(ctx context.Context) (done bool, err error) {
		_, err = n.FindVSwitch(ctx, vpcId, id)
		if err != nil {
			if errors.Is(err, cloud.NotFound) {
				return true, nil
			}
			klog.Infof("find vswitch with error: %s", err.Error())
			return false, err
		}
		return false, nil
	}
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, false, waitOn)
}
