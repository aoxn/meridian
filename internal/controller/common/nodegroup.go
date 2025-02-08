package common

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/node/block/post/addons"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/kubeclient"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type Client struct {
	client.Client
}

type NodeGroup struct {
	*Client
}

func NewNodeGroup(c client.Client) *NodeGroup {
	return &NodeGroup{Client: &Client{c}}
}

func breakId(id string) (string, string) {
	data := strings.Split(id, ".")
	if len(data) != 2 {
		return "", id
	}
	return data[0], data[1]
}

func (r *NodeGroup) ReconcileNodeGroupHosts(ctx context.Context, ng *v1.NodeGroup, pd cloud.Cloud, ip string) error {
	var (
		errList  = tool.Errors{}
		nodeList corev1.NodeList
	)
	selector := labels.SelectorFromSet(map[string]string{v1.MERIDIAN_NODEGROUP: ng.Name})
	err := r.Client.List(ctx, &nodeList, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		return errors.Wrap(err, "failed to list node")
	}
	for _, node := range nodeList.Items {
		_, id := breakId(node.Spec.ProviderID)
		content := fmt.Sprintf(hostFileCmd, v1.APIServerDomain, ip, v1.APIServerDomain)
		result, err := pd.RunCommand(ctx, cloud.Id{Id: id}, content)
		if err != nil {
			errList = append(errList, err)
			klog.Errorf("run command in [%s] error: %v, output: %s", id, err, result)
			continue
		}
		klog.Infof("apply instance [%s] host file: [%s %s]", id, ip, v1.APIServerDomain)
	}
	return errList.HasError()
}

func (r *NodeGroup) ReconcileNodeGroupAddons(ctx context.Context, ng *v1.NodeGroup, cfg cloud.Config) error {
	var (
		req        v1.Request
		needUpdate bool = false
		errs            = tool.Errors{}
	)
	err := r.Get(ctx, client.ObjectKey{Name: v1.KubernetesReq}, &req)
	if err != nil {
		return errors.Wrapf(err, "get kubernetes request")
	}
	// 如果集群没有安装节点组件，则安装，并更新组件status
	for _, addon := range ng.Spec.Addons {
		found := false
		for _, v := range ng.Status.Addons {
			if addon.Name == v.Name &&
				addon.Version == v.Version {
				found = true
				break
			}
		}
		if !found {
			data := &addons.RenderData{
				R:         &req,
				NodeGroup: ng.Name,
				AuthInfo:  cfg.AuthInfo,
			}
			// apply 组件
			ymlData, err := addons.RenderAddon(addon.Name, data)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			klog.V(8).Infof("debug addon yaml: %s", ymlData)
			err = kubeclient.ApplyInCluster(ymlData)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			found := false
			// merge status
			for i, t := range ng.Status.Addons {
				if t.Name == addon.Name {
					ng.Status.Addons[i] = addon
					found = true
					break
				}
			}
			if !found {
				ng.Status.Addons = append(ng.Status.Addons, addon)
			}
			needUpdate = true
		}
	}
	if needUpdate {
		err = r.Status().Update(ctx, &req)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs.HasError()
}

var hostFileCmd = `
sed -i '/%s/d' /etc/hosts
sed -i '$a\%s\t%s' /etc/hosts
`
