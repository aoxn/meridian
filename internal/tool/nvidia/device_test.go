package nvidia

import (
	"k8s.io/klog/v2"
	"testing"
)

func TestDev(t *testing.T) {
	exist, err := HasNvidiaDevice()
	if err != nil {
		t.Fatalf("Failed to check if nvidia device exists: %v", err)
	}
	klog.Infof("Nvidia device exists?: %v", exist)
}
