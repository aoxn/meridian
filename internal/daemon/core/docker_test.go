package core

import (
	"context"
	"encoding/json"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/client"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"k8s.io/klog/v2"
	"testing"
)

func TestSandBox(t *testing.T) {

	vm, err := meta.Local.Machine().Get("aoxn")
	if err != nil {
		return
	}
	fwd := newDockerForward(vm)
	data, err := json.Marshal(fwd)
	if err != nil {
		t.Fatalf("marshal fwd: %s", err)
	}
	klog.Infof("data: %s", data)
	var port []v1.PortForward
	err = DecodeBody(data, &port)
	if err != nil {
		t.Fatalf("decode fwd: %s", err)
	}
	klog.Infof("port: %+v", tool.PrettyJson(port))
	sdbx, err := client.Client(vm.SandboxSock())
	if err != nil {
		t.Fatalf("get client sandbox sdbx: %s", err)
	}
	err = sdbx.Create(context.TODO(), "forward", "docker", newDockerForward(vm))
	if err != nil {
		t.Fatalf("forward: %s", err.Error())
	}
}

func DecodeBody(data []byte, v interface{}) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, v)
}
