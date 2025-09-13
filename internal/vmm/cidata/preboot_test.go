package cidata

import "testing"

func TestPreBoot(t *testing.T) {
	at := "/Users/aoxn" + "/.meridian/vms/abc/cidata.iso"
	if !exist(at) {
		t.Fatalf("%s should exist", at)
	}
}
