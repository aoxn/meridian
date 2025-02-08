package slb

import (
	"context"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/alibaba/client"
)

var _ cloud.ISlb = &slbProvider{}

func NewSLB(mgr *client.ClientMgr) cloud.ISlb {
	return &slbProvider{ClientMgr: mgr}
}

type slbProvider struct {
	*client.ClientMgr
}

func (n *slbProvider) FindSLB(ctx context.Context, id cloud.Id) (cloud.SlbModel, error) {
	//req := slb.DescribeLoadBalancersArgs{}
	//if v.VpcId == "" {
	//	return v, fmt.Errorf("[find slb]vpcid must be provided")
	//}
	//req.VpcId = v.VpcId
	//if v.Id != "" {
	//	req.LoadBalancerId = v.Id
	//}
	//if v.Name != "" {
	//	req.LoadBalancerName = v.Name
	//}
	//
	//r, err := n.SLB.DescribeLoadBalancers(&req)
	//if err != nil {
	//	return v, err
	//}
	//if len(r) == 0 {
	//	return v, NotFound
	//}
	//if len(r) > 1 {
	//	klog.Infof("[service]warning: mutilple slb found by name[%s][count=%d]", v.Name, len(r))
	//}
	//s := r[0]
	//v.Id = s.LoadBalancerId
	//v.Name = s.LoadBalancerName
	//v.IpAddr = s.Address
	//klog.Infof("[service] find slb: %v", v)
	//return v, nil
	panic("implement me")
}

func (n *slbProvider) ListSLB(ctx context.Context, id cloud.Id) ([]cloud.SlbModel, error) {
	//TODO implement me
	panic("implement me")
}

func (n *slbProvider) CreateSLB(ctx context.Context, b cloud.SlbModel) (string, error) {
	//if v.VpcId == "" {
	//	return v, fmt.Errorf("[create slb]vpcid must be provided")
	//}
	//req := slb.CreateLoadBalancerArgs{
	//	LoadBalancerName: v.Name,
	//	VSwitchId:        v.VswitchId,
	//	LoadBalancerSpec: slb.S1Small,
	//	AddressType:      slb.IntranetAddressType,
	//	RegionId:         common.Region(v.Region),
	//}
	//klog.Infof("[service] create slb: %v", req)
	//r, err := n.SLB.CreateLoadBalancer(&req)
	//if err != nil {
	//	return v, err
	//}
	//if r != nil {
	//	v.Id = r.LoadBalancerId
	//	v.Name = r.LoadBalancerName
	//	v.IpAddr = r.Address
	//	v.VswitchId = r.VSwitchId
	//	return v, nil
	//}
	//return v, err
	panic("implement me")
}

func (n *slbProvider) UpdateSLB(ctx context.Context, b cloud.SlbModel) error {
	//TODO implement me
	panic("implement me")
}

func (n *slbProvider) DeleteSLB(ctx context.Context, id cloud.Id) error {
	//TODO implement me
	panic("implement me")
}

func (n *slbProvider) FindListener(ctx context.Context, id cloud.Id) (cloud.SlbModel, error) {
	//r, err := n.SLB.DescribeLoadBalancerAttribute(v.Id)
	//if err != nil {
	//	return v, err
	//}
	//if r == nil {
	//	return v, NotFound
	//}
	//hasPort := func(p int) bool {
	//	for _, v := range r.ListenerPorts.ListenerPort {
	//		if v == p {
	//			return true
	//		}
	//	}
	//	return false
	//}
	//
	//for _, m := range v.Listener {
	//	if !hasPort(m.Port) {
	//		return v, NotFound
	//	}
	//}
	//klog.Infof("[service] find slb listener: %v", v)
	//return v, nil
	panic("implement me")
}

func (n *slbProvider) CreateListener(ctx context.Context, b cloud.SlbModel) (string, error) {
	//for _, p := range v.Listener {
	//	req := slb.CreateLoadBalancerTCPListenerArgs{
	//		LoadBalancerId:    v.Id,
	//		ListenerPort:      p.Port,
	//		BackendServerPort: p.Port,
	//		Bandwidth:         p.Bandwidth,
	//	}
	//
	//	err := n.SLB.CreateLoadBalancerTCPListener(&req)
	//	if err != nil && !strings.Contains(err.Error(), "ListenerAlreadyExists") {
	//		return v, err
	//	}
	//	err = n.SLB.StartLoadBalancerListener(v.Id, p.Port)
	//	if err != nil {
	//		klog.Errorf("[service]start listener: %d %s", p.Port, err.Error())
	//	}
	//}
	//return v, nil
	panic("implement me")
}

func (n *slbProvider) UpdateListener(ctx context.Context, b cloud.SlbModel) error {
	//TODO implement me
	panic("implement me")
}

func (n *slbProvider) DeleteListener(ctx context.Context, id cloud.Id) error {
	//TODO implement me
	panic("implement me")
}
