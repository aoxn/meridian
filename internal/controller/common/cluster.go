package common

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/node/block/post/addons"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/kubeclient"
	"github.com/c-robinson/iplib"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"net"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
)

type Cluster struct {
	*Client
}

func NewCluster(c client.Client) *Cluster {
	return &Cluster{Client: &Client{c}}
}

func (r *Cluster) AllocateNet(ctx context.Context, name string) (string, error) {
	var nodeGroupList v1.NodeGroupList
	err := r.List(ctx, &nodeGroupList)
	if err != nil {
		return "", errors.Wrapf(err, "get kubernetes request")
	}
	var allocated []string
	for _, nodeGroup := range nodeGroupList.Items {
		if nodeGroup.Name == name {
			continue
		}
		allocated = append(allocated, nodeGroup.Spec.Cidr)
	}
	return r.allocate(allocated)
}

func (r *Cluster) allocate(allocated []string) (string, error) {

	sort.Strings(allocated)

	initial := fmt.Sprintf("192.168.64.0/%d", Mask)
	ip, _, err := net.ParseCIDR(initial)
	if err != nil {
		return "", errors.Wrapf(err, "parse initial")
	}
	n := iplib.NewNet4(ip, Mask)
	for {
		n = n.NextNet(Mask)
		if n.FirstAddress().String() == "192.169.0.1" {
			break
		}
		found := false
		for _, cidr := range allocated {
			_, existing, err := iplib.ParseCIDR(cidr)
			if err != nil {
				return "", errors.Wrapf(err, "parse cidr: %s", cidr)
			}
			if n.ContainsNet(existing) {
				found = true
				klog.Infof("target network [%s] contains existing network %s, %t, "+
					"abandon", n.String(), existing.String(), true)
				break
			}
		}
		if !found {
			klog.Infof("found available cidr: [%s]", n.String())
			return n.String(), nil
		}
	}
	return "", errors.Errorf("no available cidr in [%s]", initial)
}

const Mask = 21

func (r *Cluster) enumerateNetwork(allocated []string) (string, error) {

	sort.Strings(allocated)

	initial := "192.168.64.0/21"
	ip, _, err := net.ParseCIDR(initial)
	if err != nil {
		return "", errors.Wrapf(err, "parse initial")
	}
	n := iplib.NewNet4(ip, Mask)
	for {
		n = n.NextNet(Mask)
		if n.FirstAddress().String() == "192.169.0.1" {
			break
		}
		klog.Infof("next network: %s, [%s]", n.String(), n.FirstAddress())
	}
	return "", nil
}

func (r *Cluster) ReconcileClusterAddons(ctx context.Context, cfg cloud.Config) error {
	var (
		req  v1.Request
		errs = tool.Errors{}
	)
	err := r.Get(ctx, client.ObjectKey{Name: v1.KubernetesReq}, &req)
	if err != nil {
		return errors.Wrapf(err, "get kubernetes request")
	}
	if req.Status.AddonInitialized {
		return nil
	}
	// 如果集群没有安装节点组件，则安装，并更新组件status
	for _, addon := range req.Spec.Config.Addons {

		data := &addons.RenderData{
			R:        &req,
			AuthInfo: cfg.AuthInfo,
		}
		// apply 组件
		ymlData, err := addons.RenderAddon(addon.Name, data)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		klog.V(8).Infof("debug cluster addon[%s] yaml: %s", addon.Name, ymlData)
		err = kubeclient.ApplyInCluster(ymlData)
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}
	if err := errs.HasError(); err != nil {
		return err
	}
	req.Status.AddonInitialized = true
	return r.Status().Update(ctx, &req)
}
