//go:build windows
// +build windows

package runtime

import v1 "github.com/aoxn/meridian/api/v1"

type action struct{}

// NewDockerBlock returns a new action for kubeadm init
func NewAction() actions.Action {
	return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *v1.Request) error {
	return nil
}

var daemonjson = `
{
    "exec-opts": ["native.cgroupdriver=systemd"],
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "100m",
        "max-file": "10"
    },
    "bip": "169.254.123.1/24",
    "oom-score-adjust": -1000,
    "registry-mirrors": [],
    "storage-driver": "overlay2",
    "storage-opts":["overlay2.override_kernel_check=true"],
    "live-restore": true
}
`

var nvidiadaemonjson = `
{
    "default-runtime": "nvidia",
    "runtimes": {
        "nvidia": {
            "path": "/usr/bin/nvidia-container-runtime",
            "runtimeArgs": []
        }
    },
    "exec-opts": ["native.cgroupdriver=systemd"],
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "100m",
        "max-file": "10"
    },
    "bip": "169.254.123.1/24",
    "oom-score-adjust": -1000,
    "registry-mirrors": [""],
    "storage-driver": "overlay2",
    "storage-opts":["overlay2.override_kernel_check=true"],
    "live-restore": true
}
`

type dockerDaemonJson struct {
	runtime string

	execOpts        []string
	logDriver       string
	logOpts         logOpt
	bip             string
	oomScore        int
	registryMirrors []string
	storageDriver   string
	storageOpts     []string
	liveRestore     bool
}

type logOpt struct {
	maxSize string
	maxFile string
}
