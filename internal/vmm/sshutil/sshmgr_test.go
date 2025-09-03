package sshutil

import (
	"context"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"testing"
)

var cmd = `
pwd
ls -lhtr /
`

func TestSSHMgr(t *testing.T) {
	mgr := NewSSHMgr("192.168.64.2", meta.Local.Config().Dir())
	out, err := mgr.RunCommand(context.TODO(), "abc", cmd)
	if err != nil {
		t.Fatalf("build command: %s", err)
	}
	t.Log(string(out))
}
