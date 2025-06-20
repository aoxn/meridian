package mapping

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	u "github.com/syncthing/syncthing/lib/upnp"
	"k8s.io/klog/v2"
	"testing"
	"time"
)

func TestUpnp(t *testing.T) {
	cnt := 0
	for {
		if cnt > 1 {
			break
		}
		cnt++
		ip, err := getAddr()
		if err != nil {
			t.Fatal(err)
		}
		klog.Infof("get ip: %s", ip)
	}
}

func getAddr() (string, error) {
	ctx := context.TODO()
	devices := u.Discover(ctx, 0, time.Second)
	if len(devices) <= 0 {
		return "", fmt.Errorf("no router device discoverd")
	}
	device := devices[0]
	klog.Infof("[mapping] total [%d] devices discovered, use the first one", len(devices))
	eip, err := device.GetExternalIPv4Address(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get external IP address")
	}
	return eip.String(), nil
}
