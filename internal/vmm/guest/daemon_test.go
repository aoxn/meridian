package guest

import (
	"k8s.io/klog/v2"
	"testing"
)

func TestAPI(t *testing.T) {
	err := RunDaemonAPI()
	if err != nil {
		t.Fatal(err)
	}
	klog.Infof("daemon api ok")
}
