package sshutil

import (
	"testing"
)

var cmd = `
pwd
ls -lhtr /
`

func TestSSHMgr(t *testing.T) {
	mgr := NewSSHMgr("abc", "192.168.64.71:22", 22)
	out, err := mgr.RunCommand(cmd)
	if err != nil {
		t.Fatalf("build command: %s", err)
	}
	t.Log(string(out))
}
