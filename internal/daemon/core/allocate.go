package core

import (
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/c-robinson/iplib"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"k8s.io/klog/v2"
	"net"
)

var (
	defaultGateway = "192.168.64.1"
	defaultCIDR    = "192.168.64.1/24"
)

func allocateAddress(m *meta.Machine, total []*meta.Machine) error {
	_, needAllocate := lo.Find(m.Spec.Networks, func(item v1.Network) bool {
		return item.Address == ""
	})
	if !needAllocate {
		return nil
	}

	networks := lo.FlatMap(total, func(item *meta.Machine, index int) []v1.Network {
		if item.Name == m.Name {
			return nil
		}
		return item.Spec.Networks
	})

	var allocated map[string]string
	allocated = lo.FilterSliceToMap(networks, func(item v1.Network) (string, string, bool) {
		if item.Address == "" {
			return "", "", false
		}
		return item.Address, item.Address, true
	})

	klog.Infof("address has been allocated: %s", allocated)
	ip, _, err := net.ParseCIDR(defaultCIDR)
	if err != nil {
		return errors.Wrapf(err, "error parsing default cidr %s", defaultCIDR)
	}
	var addrKey = func(k string) string {
		return fmt.Sprintf("%s/24", k)
	}
	n := iplib.NewNet4(ip, 24)
	for index, _ := range m.Spec.Networks {
		succeed := false
		for i := 0; i < 255; i++ {
			ip, err = n.NextIP(ip)
			if err != nil {
				return err
			}
			klog.V(6).Infof("search for vm ip: %s", ip)
			if ip.String() == "192.168.64.1" || ip.String() == "192.168.64.0" {
				continue
			}
			_, ok := allocated[addrKey(ip.String())]
			if ok {
				continue
			}
			succeed = true
			m.Spec.Networks[index].IpGateway = defaultGateway
			m.Spec.Networks[index].Address = addrKey(ip.String())
			break
		}
		if !succeed {
			return fmt.Errorf("no available ip address")
		}
	}
	return nil
}
