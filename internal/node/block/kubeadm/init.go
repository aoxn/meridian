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

// Package kubeadminit implements the kubeadm init actionInit
package kubeadm

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/cmd"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"os"
	"path"

	"io/ioutil"
	"path/filepath"
)

func NewInitBlock(
	req *v1.Request,
	host host.Host,
) (block.Block, error) {
	return &actionInit{req: req, host: host}, nil
}

type actionInit struct {
	req  *v1.Request
	host host.Host
}

type ConfigTpl struct {
	*v1.Request
	NodeName      string
	EtcdEndpoints []string
}

type Option func(tpl *ConfigTpl)

func NewConfigTpl(
	node *v1.Request,
	host host.Host,
	opt ...Option,
) *ConfigTpl {
	addr := fmt.Sprintf("https://%s:2379", host.NodeIP())
	cfg := &ConfigTpl{
		Request:       node,
		NodeName:      host.NodeName(),
		EtcdEndpoints: []string{addr},
	}
	for _, o := range opt {
		o(cfg)
	}
	return cfg
}

const (
	KUBEADM_CONFIG_DIR = "/etc/kubeadm/"
)

// Ensure runs the actionInit
func (a *actionInit) Ensure(ctx context.Context) error {
	cfg := NewInitCfg(a.req, a.host)
	klog.V(5).Infof("Using kubeadm config:%v", cfg)
	err := os.MkdirAll(KUBEADM_CONFIG_DIR, 0755)
	if err != nil {
		return fmt.Errorf("mkdir %s error: %s", KUBEADM_CONFIG_DIR, err.Error())
	}
	// copy the config to the host
	if err := os.WriteFile(
		filepath.Join(KUBEADM_CONFIG_DIR, "kubeadm.conf"),
		[]byte(cfg),
		0755,
	); err != nil {
		return errors.Wrap(err, "failed to copy kubeadm config to host")
	}
	err = a.cleanUp()
	if err != nil {
		klog.Warningf("cleanUp failed: %v", err)
	}
	err = setOriginalPki(a.req)
	if err != nil {
		return err
	}
	err = a.createKonnectivityPod()
	if err != nil {
		return errors.Wrap(err, "failed to create konnectivity pod")
	}
	err = a.host.Service().DaemonReload()
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
	defer func() {
		_ = os.WriteFile(path.Join(KUBEADM_CONFIG_DIR, "request.yml"), []byte(tool.PrettyYaml(a.req)), 0755)
	}()
	status := <-cmd.NewCmd(
		"/usr/local/bin/kubeadm", "init",
		// preflight errors are expected, in particular for swap being enabled
		"--ignore-preflight-errors=all",
		// specify our generated config file
		fmt.Sprintf("--config=%s", filepath.Join(KUBEADM_CONFIG_DIR, "kubeadm.conf")),
		"--skip-token-print", "--v=6", // increase verbosity for debugging
	).Start()
	return cmd.CmdError(status)
}

func (a *actionInit) cleanUp() error {
	for _, v := range []string{
		"controller-manager.conf", "scheduler.conf",
	} {
		dir := filepath.Join("/etc/kubernetes", v)
		info, err := os.Stat(dir)
		if err != nil && os.IsNotExist(err) {
			continue
		}
		if info.IsDir() {
			_ = os.RemoveAll(dir)
		}
	}
	return nil
}

func (a *actionInit) Name() string {
	return fmt.Sprintf("kubeadm init [%s]", a.host.NodeID())
}

func (a *actionInit) Purge(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (a *actionInit) CleanUp(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func setOriginalPki(boot *v1.Request) error {
	err := os.MkdirAll("/etc/kubernetes/pki", 0755)
	if err != nil {
		return fmt.Errorf("ensure dir /etc/kubernetes for admin.local:%s", err.Error())
	}
	counts := map[string][]byte{}
	root := boot.Spec.Config.TLS["root"]
	if root != nil {
		counts["ca.crt"] = root.Cert
		counts["ca.key"] = root.Key
	}

	front := boot.Spec.Config.TLS["front-proxy"]
	if front != nil {
		counts["front-proxy-ca.crt"] = front.Cert
		counts["front-proxy-ca.key"] = front.Key
	}

	sa := boot.Spec.Config.TLS["svc"]
	if sa != nil {
		counts["sa.key"] = sa.Key
		counts["sa.pub"] = sa.Cert
	}
	for name, v := range counts {
		if err := ioutil.WriteFile(certHome(name), v, 0644); err != nil {
			return fmt.Errorf("write file %s: %s", name, err.Error())
		}
	}
	return nil
}
