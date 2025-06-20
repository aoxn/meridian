package mapping

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/syncthing/syncthing/lib/nat"
	u "github.com/syncthing/syncthing/lib/upnp"
	"k8s.io/klog/v2"
	"strings"
	"time"
)

type Item struct {
	ExternalPort int    `json:"externalPort"`
	InternalPort int    `json:"internalPort"`
	Protocol     string `json:"protocol"`
	Description  string `json:"description"`
}

func AddMapping(items []Item) error {
	klog.Infof("[mapping] start to discover mapping device")
	ctx := context.TODO()
	devices := u.Discover(ctx, 0, 10*time.Second)
	if len(devices) <= 0 {
		return fmt.Errorf("no router device discoverd")
	}
	device := devices[0]
	klog.Infof("[mapping] total [%d] devices discovered, use the first one", len(devices))
	eip, err := device.GetExternalIPv4Address(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get external IP address")
	}
	klog.Infof("[mapping] external router ip: %v", eip)
	for _, v := range items {
		proto := nat.Protocol(strings.ToUpper(v.Protocol))
		code, err := device.AddPortMapping(
			ctx, proto, v.InternalPort,
			v.ExternalPort, v.Description, 2*time.Hour,
		)
		if err != nil {
			klog.Errorf("[mapping] add port mapping failed: [%d=>%d(%s)], %s",
				v.ExternalPort, v.InternalPort, proto, err.Error())
			continue
		}
		klog.Infof("[mapping] port, [%d=>%d(%s)] with code %d, [%s]",
			v.ExternalPort, v.InternalPort, proto, code, v.Description)
	}
	return nil
}
