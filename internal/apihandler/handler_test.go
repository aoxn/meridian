package apihandler

import (
	"context"
	"fmt"
	"github.com/c-robinson/iplib"
	"k8s.io/klog/v2"
	"net"
	"testing"
)

func TestAPIHandler(t *testing.T) {
	err := RunDaemonAPI(context.TODO())
	if err != nil {
		t.Fatalf("failed to run daemon api handler: %v", err)
	}
}

func TestAddress(t *testing.T) {
	ip, _, err := net.ParseCIDR("192.168.64.1/24")
	if err != nil {
		t.Fatalf("error parsing default cidr %s", err)
	}
	n := iplib.NewNet4(ip, 24)
	for i := 0; i < 255; i++ {
		ip, err = n.NextIP(ip)
		if err != nil {
			t.Fatalf("xxx: %s", err)
		}
		klog.Infof("search for vm ip: %s", ip)
		if ip.String() == "192.168.64.1" || ip.String() == "192.168.64.0" {
			continue
		}

		fmt.Printf("%s/24\n", ip)
		break
	}
}
