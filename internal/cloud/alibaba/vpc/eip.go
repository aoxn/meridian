package vpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	vxc "github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/alibaba/client"
	"k8s.io/klog/v2"
)

var _ cloud.IEip = &eip{}

func NewEip(mgr *client.ClientMgr) cloud.IEip {
	return &eip{ClientMgr: mgr}
}

type eip struct {
	*client.ClientMgr
}

func (n *eip) FindEIP(ctx context.Context, id cloud.Id) (cloud.EipModel, error) {
	model := cloud.EipModel{}
	if id.Name == "" && id.Id == "" {
		return model, fmt.Errorf("empty IEip name or id")
	}

	req := vxc.CreateDescribeEipAddressesRequest()
	req.EipName = id.Name

	req.AllocationId = id.Id

	r, err := n.VPC.DescribeEipAddresses(req)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return model, cloud.NotFound
		}
		return model, nil
	}
	if r != nil {

		if len(r.EipAddresses.EipAddress) == 0 {
			return model, cloud.NotFound
		}
		addr := r.EipAddresses.EipAddress[0]
		model.EipId = addr.AllocationId
		model.Address = addr.IpAddress
		model.InstanceId = addr.InstanceId
		return model, nil
	}
	return model, cloud.NotFound
}

func (n *eip) ListEIP(ctx context.Context, id cloud.Id) ([]cloud.EipModel, error) {
	//TODO implement me
	panic("implement me")
}

func (n *eip) CreateEIP(ctx context.Context, m cloud.EipModel) (string, error) {
	req := vxc.CreateAllocateEipAddressRequest()
	req.Name = m.EipName

	r, err := n.VPC.AllocateEipAddress(req)
	if err != nil {
		return "", err
	}
	if r != nil {
		m.Address = r.EipAddress
		m.EipId = r.AllocationId
		return m.EipId, nil
	}
	klog.Infof("unexpected allocate eip address: %s, %s", m.EipName, m.EipId)
	return m.EipId, nil
}

func (n *eip) BindEIP(ctx context.Context, v cloud.EipModel) error {
	req := vxc.CreateAssociateEipAddressRequest()
	req.InstanceId = v.InstanceId
	req.RegionId = v.Region
	req.Mode = v.BindMode             //"NAT"
	req.InstanceType = v.InstanceType // "SlbInstance", "Nat"
	req.AllocationId = v.EipId
	klog.Infof("[service]bind eip for instance: %+v", req)
	_, err := n.VPC.AssociateEipAddress(req)
	if err != nil {
		klog.Infof("[service]bind eip address: %s", err.Error())
		return err
	}
	reqe := vxc.CreateDescribeEipAddressesRequest()
	reqe.AllocationId = v.EipId

	tick := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-tick.C:
			r, err := n.VPC.DescribeEipAddresses(reqe)
			if err != nil {
				return err
			}
			eip := r.EipAddresses.EipAddress[0]
			if eip.Status == "InUse" {
				klog.Infof("[service]eip %s is InUse status", v.EipId)
				return nil
			}
		case <-time.After(1 * time.Minute):
			return fmt.Errorf("time out on waiting eip binding nat gateway")
		}
	}
}

func (n *eip) UpdateEIP(ctx context.Context, m cloud.EipModel) error {
	//TODO implement me
	panic("implement me")
}

func (n *eip) DeleteEIP(ctx context.Context, id cloud.Id) error {
	//TODO implement me
	panic("implement me")
}
