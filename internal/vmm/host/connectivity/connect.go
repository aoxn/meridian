package connectivity

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian/internal/tool/mapping"
	"github.com/aoxn/meridian/internal/vmm/backend"
	"github.com/aoxn/meridian/internal/vmm/forward"
	"github.com/aoxn/meridian/internal/vmm/meta"
	gerrors "github.com/pkg/errors"
	"github.com/samber/lo"
	"golang.org/x/net/proxy"
	"strconv"
	"time"

	"k8s.io/klog/v2"

	v1 "github.com/aoxn/meridian/api/v1"
)

func NewConnectivity(
	fwd *forward.ForwardMgr,
	driver backend.Driver,
	vmMeta *meta.Machine,
) *Connectivity {
	conn := &Connectivity{fwd: fwd, driver: driver}
	return conn
}

type Connectivity struct {
	driver backend.Driver
	fwd    *forward.ForwardMgr
}

func (ha *Connectivity) F() *forward.ForwardMgr {
	return ha.fwd
}

func (ha *Connectivity) Forward(ctx context.Context, ports []v1.PortForward) error {
	dialer, err := ha.driver.Dialer(ctx)
	if err != nil {
		return gerrors.Wrap(err, "failed to get dialer")
	}
	for _, f := range ports {
		var dialers []proxy.Dialer
		if f.DstProto == "vsock" {
			dialers = append(dialers, dialer)
		}
		err = ha.fwd.AddBy(f.Rule(), dialers...)
		if err != nil {
			return fmt.Errorf("add forwarding rule[vsock]:[%s] %s", f.Rule(), err.Error())
		}
	}
	return nil
}

func (ha *Connectivity) ForwardMachine(ctx context.Context, vm *meta.Machine) error {
	var rules = lo.Map(vm.Spec.PortForwards, func(item v1.PortForward, index int) string {
		return item.Rule()
	})
	klog.Infof("forward machine connection: %s, %s", vm.Name, rules)
	return ha.Forward(ctx, vm.Spec.PortForwards)
}

func (ha *Connectivity) Remove(ports []v1.PortForward) error {
	for _, f := range ports {
		ha.fwd.Remove(f.Rule())
	}
	return nil
}

func buildRule(proto, source, dproto string, dest interface{}) string {
	return fmt.Sprintf("%s://%s->%s://%v", proto, source, dproto, dest)
}

func (ha *Connectivity) SetMappingRoute() {
	// todo: 需要实例化
	var req v1.RequestSpec
	klog.Infof("[mapping]start reconcile upnp port mapping")
	if !v1.HasFeature(
		req.Config.Features, v1.FeatureSupportNodeGroups) {
		klog.Infof("[mapping] nodegroups feature disabled, skip mapping")
		return
	}
	doMaping := func() {
		i := req
		port, err := strconv.Atoi(i.AccessPoint.APIPort)
		if err != nil {
			klog.Errorf("[mapping]failed to parse access point port: %s", err)
			return
		}
		tport, _ := strconv.Atoi(i.AccessPoint.TunnelPort)
		klog.Infof("[mapping]periodical mapping port: %d", port)
		items := []mapping.Item{
			{
				ExternalPort: port,
				InternalPort: port,
			},
			{
				ExternalPort: tport,
				InternalPort: tport,
			},
		}
		for _, item := range items {
			err = mapping.AddMapping([]mapping.Item{item})
			if err != nil {
				klog.Errorf("[mapping]failed to add mapping: %s", err)
				continue
			}
			klog.Infof("[mapping] port [%d] mapped", item.ExternalPort)
		}
	}
	doMaping()
	for {
		select {
		case <-time.After(5 * time.Minute):
			doMaping()
		}
	}
}
