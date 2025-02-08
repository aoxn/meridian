//go:build windows
// +build windows

package kubeadm

import v1 "github.com/aoxn/meridian/api/v1"

const (
	ObjectName        = "config"
	KUBELET_UNIT_FILE = "/etc/systemd/system/kubelet.service"
)

type ActionKubelet struct {
}

// NewAction returns a new actionInit for kubeadm init
func NewActionKubelet() actions.Action {
	return &ActionKubelet{}
}

// Execute runs the actionInit
func (a *ActionKubelet) Execute(ctx *v1.Request) error {

	//TODO:
	return nil
}
