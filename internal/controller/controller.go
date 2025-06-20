package controller

import (
	"github.com/aoxn/meridian/internal/controller/infra"
	"github.com/aoxn/meridian/internal/controller/infra/nodes"
	"github.com/aoxn/meridian/internal/controller/raven"
	"github.com/aoxn/meridian/internal/controller/xdpin"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	modules []func(manager.Manager) error
)

func init() {
	modules = append(modules, raven.AddGateway)
	modules = append(modules, raven.AddGatewayPublicService)
	modules = append(modules, raven.AddGatewayInternalService)
	modules = append(modules, xdpin.AddPeriodical)
	//modules = append(modules, xdpin.Add)
	modules = append(modules, infra.AddNode)
	modules = append(modules, infra.AddNodeGroup)
	modules = append(modules, nodes.AddNodeCleanup)

	modules = append(modules, raven.AddGatewayWebhook)
	modules = append(modules, infra.AddApproveController)
}

func Add(
	mgr manager.Manager,
) error {
	for i, f := range modules {
		err := f(mgr)
		if err != nil {
			return err
		}
		klog.Infof("controller added [%d]", i)
	}
	return nil
}
