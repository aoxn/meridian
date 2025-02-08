//go:build windows
// +build windows

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
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"strings"
)

// kubeadmInitAction implements actionInit for executing the kubadm init
// and a set of default post init operations like e.g. install the
// CNI network plugin.
type ActionInit struct{}

type ConfigTpl struct {
	*v1.Master
	*v1.ClusterSpec
	NodeName      string
	EtcdEndpoints []string
}

type Option func(tpl *ConfigTpl)

func NewConfigTpl(
	node *v1.Master,
	opt ...Option,
) *ConfigTpl {
	cfg := &ConfigTpl{
		Node: node, ClusterSpec: node.Status.BootCFG,
	}
	for _, o := range opt {
		o(cfg)
	}
	return cfg
}

func WithNodeName(tpl *ConfigTpl) {
	_, insid := BreakDownIDOrDie(tpl.Node.Spec.ID)
	tpl.NodeName = fmt.Sprintf("%s.%s", tpl.Node.Spec.IP, insid)
}

func WithEtcdEndpoints(tpl *ConfigTpl) {
	var etcds []string
	if tpl.Etcd.Endpoints != "" {
		for _, host := range strings.Split(tpl.Etcd.Endpoints, ",") {
			etcds = append(etcds, fmt.Sprintf("https://%s:2379", host))
		}
	} else {
		if tpl.Node.Spec.IP == "" {
			panic("host ip must be provided")
		}
		klog.Infof("single etcd addresses: %s", tpl.Node.Spec.IP)
		etcds = append(etcds, fmt.Sprintf("https://%s:2379", tpl.Node.Spec.IP))
	}
	klog.Infof("etcd endpoints configured[%t]: use self address %s", len(tpl.Etcd.Endpoints) != 0, etcds)

	tpl.EtcdEndpoints = etcds
}

func BreakDownIDOrDie(id string) (string, string) {
	tpl := strings.Split(id, ".")
	if len(tpl) != 2 {
		panic(fmt.Sprintf("id format error expect {region.instanceid} got %s", id))
	}
	return tpl[0], tpl[1]
}

// NewAction returns a new actionInit for kubeadm init
func NewActionInit() actions.Action {
	return &ActionInit{}
}

const (
	KUBEADM_CONFIG_DIR = "/etc/kubeadm/"
)

// Execute runs the actionInit
func (a *ActionInit) Execute(ctx *v1.Request) error { return fmt.Errorf("not implemented") }
