package xdpin

import (
	"context"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/controller/common"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/address"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func NewPortMapping(mgr manager.Manager) Periodical {
	return &portMapping{mgr: mgr}
}

type portMapping struct {
	mgr manager.Manager
}

func (s *portMapping) Name() string {
	return "api.accesspoint.reconciler"
}

func (s *portMapping) Schedule() string {
	return "*/3 * * * *"
}

func (s *portMapping) Run(options Options) error {
	klog.Infof("1.port mapping controller %s", s.Name())
	//cfg, err := LoadCfg()
	//if err != nil {
	//	return errors.Wrapf(err, "failed to build config: ")
	//}
	return newMapMgr(s.mgr.GetClient()).Reconcile()
}

func newMapMgr(client client.Client) *accessPoint {
	return &accessPoint{client: client}
}

type accessPoint struct {
	client client.Client
}

//func (m *accessPoint) Reconcile() error {
//	klog.Infof("[mapping] start to reconcile api accesspoint")
//	var req v1.Request
//	err := m.client.Get(
//		context.TODO(),
//		client.ObjectKey{Name: v1.KubernetesReq}, &req,
//	)
//	if err != nil {
//		return errors.Wrapf(err, "failed to get mapping device %s", v1.KubernetesReq)
//	}
//
//	need := setBackupLabel(&req)
//
//	var (
//		addr string
//		ctx  context.Context
//		gw   ravenv1beta1.Gateway
//	)
//	err = m.client.Get(ctx, client.ObjectKey{Name: v1.GatewayMaster}, &gw)
//	if err == nil {
//		var node v13.NodeList
//		// get master
//		selector := labels.SelectorFromSet(labels.Set{"node-role.kubernetes.io/control-plane": ""})
//		err = m.client.List(ctx, &node, &client.ListOptions{LabelSelector: selector})
//		if err == nil {
//			var masterName string
//			for _, nodeItem := range node.Items {
//				masterName = nodeItem.Name
//			}
//			for _, k := range gw.Spec.Endpoints {
//				if k.NodeName == masterName {
//					addr = k.PublicIP
//					break
//				}
//			}
//		} else {
//			klog.Infof("failed to find master node: %s", err.Error())
//		}
//	} else {
//		klog.Infof("failed to find master gateway %s, %s", v1.GatewayMaster, err.Error())
//	}
//
//	if addr != "" && req.Spec.AccessPoint.Internet != addr {
//		need = true
//		req.Spec.AccessPoint.Internet = addr
//	}
//	if !need {
//		return nil
//	}
//	klog.Infof("set public ip for request resource[%s]: %s", v1.KubernetesReq, addr)
//	err = m.client.Update(context.TODO(), &req)
//	if err != nil {
//		return err
//	}
//	// todo:
//	// 	1. set host resolve first; restart kubelet
//	//      2. update raven-worker deploy.pod.spec.hostAlias
//	ds := &v12.DaemonSet{}
//	err = m.client.Get(
//		context.TODO(),
//		client.ObjectKey{Name: "raven-agent-ds-worker", Namespace: "kube-system"}, ds,
//	)
//	if err != nil {
//		return err
//	}
//	ds.Spec.Template.Spec.HostAliases = []v13.HostAlias{
//		{
//			IP:        addr,
//			Hostnames: []string{req.Spec.AccessPoint.APIDomain},
//		},
//	}
//	return m.client.Update(context.TODO(), ds)
//}

func (m *accessPoint) Reconcile() error {
	klog.Infof("[mapping] start to reconcile api accesspoint")
	var (
		req v1.Request
		ctx context.Context = context.Background()
	)
	err := m.client.Get(
		context.TODO(), client.ObjectKey{Name: v1.KubernetesReq}, &req,
	)
	if err != nil {
		return errors.Wrapf(err, "failed to get mapping device %s", v1.KubernetesReq)
	}

	need := setBackupLabel(&req)
	klog.Infof("need to set backup label for %s: %t", v1.KubernetesReq, need)
	ip, err := address.GetAddress(address.POLL)
	if err != nil {
		return errors.Wrapf(err, "reconcile public ip")
	}
	addr := ip.IPv4.String()
	if addr != "" && req.Spec.AccessPoint.Internet != addr {
		need = true
		req.Spec.AccessPoint.Internet = addr
	}
	if !need {
		klog.Infof("public ip consist, no need to update. access.point.ip=%s, public.ip=%s", req.Spec.AccessPoint.Internet, addr)
		return nil
	}
	klog.Infof("set public ip for request resource[%s]: %s", v1.KubernetesReq, addr)
	err = m.client.Update(ctx, &req)
	if err != nil {
		return err
	}
	// todo:
	// 	1. set host resolve first; restart kubelet
	//      2. update raven-worker deploy.pod.spec.hostAlias
	var (
		errList    = tool.Errors{}
		nodeGroups v1.NodeGroupList
	)
	for _, addon := range []string{"konnectivity-worker"} {
		err = common.NewAddon(m.client).ReconcileAddon(context.TODO(), addon)
		if err != nil {
			errList = append(errList, err)
			continue
		}
		klog.Infof("reconcile api accesspoint for addon[%s]", addon)
	}
	err = m.client.List(ctx, &nodeGroups)
	if err != nil {
		return errors.Wrapf(err, "failed to list node groups")
	}
	for _, ng := range nodeGroups.Items {
		pd, err := cloud.NewCloud(m.client, ng.Spec.Provider)
		if err != nil {
			errList = append(errList, err)
			continue
		}
		group := common.NewNodeGroup(m.client)
		err = group.ReconcileNodeGroupAddons(ctx, &ng, pd.GetConfig())
		if err != nil {
			errList = append(errList, err)
			continue
		}
		klog.Infof("reconcile api accesspoint for nodegroup[%s]", ng.Name)

		err = group.ReconcileNodeGroupHosts(ctx, &ng, pd, addr)
		if err != nil {
			errList = append(errList, err)
			continue
		}
	}
	return errList.HasError()
}

func setBackupLabel(ng *v1.Request) bool {
	if ng.Labels == nil {
		ng.Labels = map[string]string{}
	}
	_, ok := ng.Labels[v1.XDPIN_BACKUP]
	if ok {
		// not changed
		return false
	}
	ng.Labels[v1.XDPIN_BACKUP] = "true"
	return true // changed
}
