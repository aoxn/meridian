//go:build linux || darwin
// +build linux darwin

/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package kubeadmin implements the kubeadm join joinBlock
package kubeadm

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/aoxn/meridian/internal/tool/cmd"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"os"
	"time"
)

type joinBlock struct {
	req  *v1.Request
	host host.Host
}

// NewJoinBlock returns a new joinBlock for kubeadm init
func NewJoinBlock(req *v1.Request, host host.Host) (block.Block, error) {
	return &joinBlock{req: req, host: host}, nil
}

// Ensure runs the joinBlock
func (a *joinBlock) Ensure(ctx context.Context) error {
	cfg := NewConfigTpl(a.req, a.host)
	_ = os.MkdirAll("/etc/kubernetes/manifests", 0755)
	err := a.host.Service().DaemonReload()
	if err != nil {
		return fmt.Errorf("systecmctl daemon-reload,%s ", err.Error())
	}
	err = a.host.Service().Enable("kubelet")
	if err != nil {
		return fmt.Errorf("systecmctl enable kubelet error,%s ", err.Error())
	}
	err = a.host.Service().Restart("kubelet")
	if err != nil {
		return fmt.Errorf("systecmctl start kubelet error,%s ", err.Error())
	}
	port := "6443"
	endpoint := cfg.Spec.AccessPoint.Intranet
	if cfg.Spec.AccessPoint.APIDomain != "" {
		endpoint = cfg.Spec.AccessPoint.APIDomain
	}
	if cfg.Spec.AccessPoint.APIPort != "" {
		port = cfg.Spec.AccessPoint.APIPort
	}
	status := <-cmd.NewCmd(
		"/usr/local/bin/kubeadm", "join",
		// increase verbosity for debugging
		"--v=6",
		// preflight errors are expected, in particular for swap being enabled
		"--ignore-preflight-errors=all",
		"--node-name", cfg.NodeName,
		"--token", cfg.Spec.Config.Token,
		"--discovery-token-unsafe-skip-ca-verification",
		fmt.Sprintf("%s:%s", endpoint, port),
	).Start()
	if err := cmd.CmdError(status); err != nil {
		return fmt.Errorf("kubeadm join: %s", err.Error())
	}
	return WaitJoin(a.req)
}

func (a *joinBlock) Name() string {
	return fmt.Sprintf("kubelet join: [%s]", a.host.NodeID())
}

func (a *joinBlock) Purge(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (a *joinBlock) CleanUp(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func WaitJoin(ctx *v1.Request) error {
	return wait.Poll(
		2*time.Second,
		5*time.Minute,
		func() (done bool, err error) {
			status := <-cmd.NewCmd(
				"kubectl",
				"--kubeconfig", "/etc/kubernetes/kubelet.conf",
				"get", "no",
			).Start()
			if err := cmd.CmdError(status); err != nil {
				klog.Infof("wait for kubeadm join: %s", err.Error())
				return false, nil
			}
			return true, nil
		},
	)
}
