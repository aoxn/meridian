package post

import (
	"context"
	"fmt"
	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block"
	"github.com/aoxn/meridian/internal/node/block/post/addons"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/kubeclient"
	"k8s.io/klog/v2"
)

type postAddons struct {
	req  *api.Request
	host host.Host
}

// NewPostAddon returns a new postAddons for post kubernetes install
func NewPostAddon(req *api.Request, host host.Host) (block.Block, error) {
	return &postAddons{host: host, req: req}, nil
}

// Ensure runs the postAddons
func (a *postAddons) Ensure(ctx context.Context) error {

	klog.Infof("on waiting for kube-apiserver ok")
	err := waitBootstrap(a.req)
	if err != nil {
		return err
	}
	klog.Infof("on creating meridian operator")

	addons.SetDftClusterAddons(&a.req.Spec)
	data := &addons.RenderData{
		R: a.req,
	}
	var errList tool.Errors
	for _, v := range []string{
		addons.XDPIN.Name,
		addons.KUBEPROXY_MASTER.Name,
		addons.KUBEPROXY_WORKER.Name,
		addons.FLANNEL_MASTER.Name,
		addons.FLANNEL.Name,
		addons.KONNECTIVITY_AGENT_MASTER.Name,
		addons.KONNECTIVITY_AGENT_WORKER.Name,
		addons.CORDDNS.Name,
		addons.METRICS_SERVER.Name,
	} {

		addYaml, err := addons.RenderAddon(v, data)
		if err != nil {
			return err
		}
		homecfg, err := block.HomeKubeCfg()
		if err != nil {
			errList = append(errList, err)
			continue
		}
		err = kubeclient.ApplyBy(addYaml, homecfg)
		if err != nil {
			errList = append(errList, err)
			continue
		}
	}
	return errList.HasError()
}

func (a *postAddons) Name() string {
	return fmt.Sprintf("post addons: [%s]", a.host.NodeID())
}

func (a *postAddons) Purge(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (a *postAddons) CleanUp(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}
