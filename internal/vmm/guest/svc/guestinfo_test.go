package svc

import (
	"k8s.io/klog/v2"
	"testing"
)

func TestName(t *testing.T) {
	addrs, err := GetLocalIP()
	if err != nil {
		t.Fatal(err)
	}
	for i, addr := range addrs {
		klog.Infof("addr [%d]: %s", i, addr)
	}
}
