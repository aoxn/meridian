package common

import (
	"k8s.io/klog/v2"
	"testing"
)

func TestCluster(t *testing.T) {
	cluster := NewCluster(nil)
	_, err := cluster.enumerateNetwork([]string{})
	if err != nil {
		t.Fatalf("allocate fail: %s", err.Error())
	}

	network, err := cluster.allocate([]string{"192.168.72.0/21", "192.168.80.0/22"})
	if err != nil {
		t.Fatalf("allocate fail: %s", err.Error())
	}
	klog.Infof("allocated: %s", network)
}
