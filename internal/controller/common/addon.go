package common

import (
	"context"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block/post/addons"
	"github.com/aoxn/meridian/internal/tool/kubeclient"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Addon struct {
	*Client
}

func NewAddon(c client.Client) *Addon {
	return &Addon{Client: &Client{c}}
}

func (r *Addon) ReconcileAddon(ctx context.Context, name string) error {
	var req v1.Request
	err := r.Get(ctx, client.ObjectKey{Name: v1.KubernetesReq}, &req)
	if err != nil {
		return errors.Wrapf(err, "get kubernetes request")
	}
	data := &addons.RenderData{
		R: &req,
		//AuthInfo: cfg.AuthInfo,
	}
	// apply 组件
	ymlData, err := addons.RenderAddon(name, data)
	if err != nil {
		return err
	}
	klog.V(8).Infof("addon: debug addon yaml, %s", ymlData)
	return kubeclient.ApplyInCluster(ymlData)
}
