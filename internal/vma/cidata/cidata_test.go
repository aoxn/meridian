package cidata

import (
	"k8s.io/klog/v2"
	"testing"
)

func TestCIDATA(t *testing.T) {
	ci := NewCloudInit(nil, nil)
	err := ci.GenCIISO()
	if err != nil {
		t.Fatal(err)
		return
	}
	klog.Infof("gen cloudinit data finished")
}
