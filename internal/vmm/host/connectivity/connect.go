package connectivity

import (
	"context"
	"fmt"
	user "github.com/aoxn/meridian/client"
	"github.com/aoxn/meridian/internal/tool/mapping"
	"github.com/aoxn/meridian/internal/vmm/backend"
	"github.com/aoxn/meridian/internal/vmm/forward"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"net"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	v1 "github.com/aoxn/meridian/api/v1"
)

func NewConnectivity(fwd *forward.ForwardMgr, driver backend.Driver, vm *meta.Machine) *Connectivity {
	return &Connectivity{
		fwd: fwd, driver: driver, vmMeta: vm,
	}
}

type Connectivity struct {
	fwd    *forward.ForwardMgr
	driver backend.Driver
	vmMeta *meta.Machine
}

func (ha *Connectivity) Forward(ctx context.Context) error {
	addr, err := ha.WaitVMAddr(ctx)
	if err != nil {
		return err
	}
	dialer, err := ha.driver.Dialer(ctx)
	if err != nil {
		return err
	}
	forwards := ha.vmMeta.Spec.PortForwards
	v1.SetPortForward(forwards, v1.PortForward{
		Proto:       "tcp",
		Source:      fmt.Sprintf("0.0.0.0:%s", ha.vmMeta.Spec.Request.AccessPoint.TunnelPort),
		Destination: fmt.Sprintf("%s:8132", addr),
	})
	for _, f := range forwards {
		if f.VSockPort > 0 {
			rule := buildRule(f.Proto, f.Source, "vsock", f.VSockPort)
			err = ha.fwd.AddBy(rule, dialer)
			if err != nil {
				return fmt.Errorf("add forwarding rule[vsock]:[%s] %s", rule, err.Error())
			}
		} else {
			rule := buildRule(f.Proto, f.Source, f.Proto, f.Destination)
			err = ha.fwd.AddBy(rule)
			if err != nil {
				return fmt.Errorf("add forwarding rule:[%s] %s", rule, err.Error())
			}
		}
	}
	return nil
}

func buildRule(proto, source, dproto string, dest interface{}) string {
	return fmt.Sprintf("%s://%s->%s://%v", proto, source, dproto, dest)
}

func (ha *Connectivity) WaitVMAddr(ctx context.Context) (string, error) {
	client, err := user.ClientWith(func(ctx context.Context, network, addr string) (net.Conn, error) {
		return ha.driver.GuestAgentConn(ctx)
	})
	if err != nil {
		return "", err
	}

	var addr = ""
	waitFunc := func(ctx context.Context) (bool, error) {
		var guest = v1.EmptyGI(ha.vmMeta.Name)
		err = client.Get(ctx, guest)
		if err != nil {
			klog.Errorf("wait guest info: %s", err.Error())
			return false, nil
		}
		klog.Infof("[%s]wait guest server: %s, [%s]", ha.vmMeta.Name, guest.Spec.Address, guest.Status.Phase)
		if guest.Status.Phase != v1.Running {
			klog.Infof("guest status is not running: [%s]", guest.Status.Phase)
			return false, nil
		}
		for _, ad := range guest.Spec.Address {
			if strings.Contains(ad, "192.168") {
				addr = ad
				return true, nil
			}
		}
		return false, nil
	}

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, false, waitFunc)
	return addr, err
}

func (ha *Connectivity) SetMappingRoute() {
	klog.Infof("[mapping]start reconcile upnp port mapping")
	if !v1.HasFeature(
		ha.vmMeta.Spec.Request.Config.Features, v1.FeatureSupportNodeGroups) {
		klog.Infof("[mapping] nodegroups feature disabled, skip mapping")
		return
	}
	doMaping := func() {
		i := ha.vmMeta.Spec.Request
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
