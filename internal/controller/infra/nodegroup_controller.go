/*
Copyright 2023.

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

package infra

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/controller/common"
	"github.com/aoxn/meridian/internal/node/block/post/addons"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/c-robinson/iplib"
	ravenv1beta1 "github.com/openyurtio/openyurt/pkg/apis/raven/v1beta1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"net"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func AddNodeGroup(
	mgr manager.Manager,
) error {

	r := &nodeGroupReconciler{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetEventRecorderFor("NodeGroup"),
	}
	// Create a new controller
	c, err := controller.New(
		"nodegroup-controller", mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: 1,
		},
	)
	if err != nil {
		return err
	}
	err = c.Watch(
		source.Kind(mgr.GetCache(), &corev1.Node{}),
		&handler.EnqueueRequestForObject{},
	)

	return c.Watch(
		source.Kind(mgr.GetCache(), &v1.NodeGroup{}),
		&handler.EnqueueRequestForObject{},
	)
}

// blank assignment to verify that ReconcileAutoRepair implements reconcile.Reconciler
var _ reconcile.Reconciler = &nodeGroupReconciler{}

// nodeGroupReconciler reconciles a Infra object
type nodeGroupReconciler struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=xdpin.cn,resources=nodegroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=xdpin.cn,resources=nodegroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=xdpin.cn,resources=nodegroups/finalizers,verbs=update

func (r *nodeGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var ng v1.NodeGroup
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, &ng); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	pd, err := newCloud(r.Client, &ng)
	if err != nil {
		klog.Errorf("init cloud provider[%s] with error: %s", ng.Spec.Provider, err.Error())
		return ctrl.Result{}, err
	}

	if !ng.DeletionTimestamp.IsZero() {
		err := r.teardown(pd, &ng)
		if err != nil {
			return ctrl.Result{}, err
		}
		removeFinalizer(&ng, v1.Finalizer)
		return ctrl.Result{}, r.Update(context.Background(), &ng)
	}
	// todo
	// 1. create gateway on master init. label master with gateway label.
	// 1. 设置xdpin. autbackup label[nodegroup, request]
	// 2. install nodepool level addons. [flannel,csi,]
	// 3. reconcile api access endpoint from master gateway. add host resolve && restart kubelet from run command
	// 4. label worker with gw-group.
	err = r.SetNodeGroupDft(ctx, &ng)
	if err != nil {
		return reconcile.Result{}, err
	}
	addons.SetDftNodeGroupAddons(&ng)

	err = r.createInfra(pd, &ng)
	if err != nil {
		klog.Infof("ensure nodegroup infra: %s", err.Error())
		return ctrl.Result{}, r.Update(ctx, &ng)
	}

	err = r.Client.Update(ctx, &ng)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "update nodegroup %s", ng.Name)
	}

	err = r.createNodeGroupGateway(ctx, &ng)
	if err != nil {
		klog.Errorf("ensure nodegroup[gw-%s] gateway: %s", ng.Name, err.Error())
	}
	err = r.createNodeGroupAddons(ctx, &ng, pd.GetConfig())
	if err != nil {
		klog.Errorf("ensure nodegroup addons for %s, %s", ng.Name, err.Error())
	}
	return ctrl.Result{}, r.ScalingNodeGroup(ctx, pd, &ng)
}

func removeFinalizer(ng *v1.NodeGroup, name string) {
	var finalizers []string
	for _, f := range ng.ObjectMeta.Finalizers {
		if f == name {
			continue
		}
		finalizers = append(finalizers, f)
	}
	ng.Finalizers = finalizers
}

func buildTag(name string) []cloud.Tag {
	return []cloud.Tag{
		{Value: "meridian", Key: name},
	}
}

func toScalingModel(ng *v1.NodeGroup, ud string) cloud.ScalingGroupModel {

	scalingGrpModel := cloud.ScalingGroupModel{
		Region: ng.Spec.Region,
		ScalingRule: cloud.ScalingRule{
			Region:          ng.Spec.Region,
			ScalingRuleAri:  ng.Spec.ScalingGroup.ScalingRule.ScalingRuleAri,
			ScalingRuleId:   ng.Spec.ScalingGroup.ScalingRule.ScalingRuleId,
			ScalingRuleName: ng.Spec.ScalingGroup.ScalingRule.ScalingRuleName,
		},
		ScalingConfig: cloud.ScalingConfig{
			UserData:       ud,
			ScalingCfgId:   ng.Spec.ScalingGroup.ScalingConfig.ScalingCfgId,
			ScalingCfgName: ng.Spec.ScalingGroup.ScalingConfig.ScalingCfgName,
		},
		ScalingGroupId:   ng.Spec.ScalingGroup.ScalingGroupId,
		ScalingGroupName: ng.Spec.ScalingGroup.ScalingGroupName,
	}
	return scalingGrpModel
}

func (r *nodeGroupReconciler) createNodeGroupAddons(ctx context.Context, ng *v1.NodeGroup, cfg cloud.Config) error {
	return common.NewNodeGroup(r.Client).ReconcileNodeGroupAddons(ctx, ng, cfg)
}

func (r *nodeGroupReconciler) createNodeGroupGateway(ctx context.Context, ng *v1.NodeGroup) error {

	var (
		gwName      = "gw-" + ng.Name
		gwNodeGroup ravenv1beta1.Gateway
	)

	mergeEndpoint := func(gw *ravenv1beta1.Gateway, nodeList []corev1.Node) bool {
		klog.Infof("[%s]merge endpoints, total nodes=%d", gwName, len(nodeList))
		var endpoints []ravenv1beta1.Endpoint
		for _, v := range gw.Spec.Endpoints {
			found := false
			for _, node := range nodeList {
				if v.NodeName == node.Name {
					if !tool.UnknownCondition(node.Status.Conditions) {
						found = true
						break
					}
					klog.Warningf("gateway endpoint [%s] kubelet condition unknown, remove from endpoint", v.NodeName)
				}
			}
			if found {
				endpoints = append(endpoints, v)
			}
		}

		for _, node := range nodeList {
			klog.Infof("[%s]merge endpoints, for each node %s", gw.Name, node.Name)
			found := false
			for _, endpoint := range gw.Spec.Endpoints {
				if endpoint.NodeName == node.Name {
					found = true
					break
				}
			}
			if !found {
				ep := ravenv1beta1.Endpoint{
					NodeName: node.Name,
					UnderNAT: false,
					Type:     ravenv1beta1.Tunnel,
					//Port:     ravenv1beta1.DefaultTunnelServerExposedPort,
				}
				endpoints = append(endpoints, ep)
				klog.Infof("[%s]merge endpoints with new endpoint: %s", gw.Name, node.Name)
			}
		}
		end := len(endpoints)
		//end := int(math.Min(float64(len(endpoints)), 2.0))
		needUpgade := !reflect.DeepEqual(gw.Spec.Endpoints, endpoints[0:end])
		gw.Spec.Endpoints = endpoints[0:end]
		klog.Infof("[%s]merge gateway endpoints: end=%d, needUpdate=%t, length=%d", gw.Name, end, needUpgade, len(gw.Spec.Endpoints))
		return needUpgade
	}

	var nodeList corev1.NodeList
	lblSet := labels.Set{v1.MERIDIAN_NODEGROUP: ng.Name}
	err := r.List(ctx, &nodeList, &client.ListOptions{LabelSelector: labels.SelectorFromSet(lblSet)})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "get nodegroup nodes")
		}
	}

	err = r.Get(ctx, client.ObjectKey{Name: gwName}, &gwNodeGroup)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "get gateway nodegroup %s", ng.Name)
		}
		//nodeSelector := metav1.SetAsLabelSelector(lblSet)
		gwNodeGroup = ravenv1beta1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name: gwName,
			},
			Spec: ravenv1beta1.GatewaySpec{
				ProxyConfig: ravenv1beta1.ProxyConfiguration{
					Replicas: 1,
				},
				TunnelConfig: ravenv1beta1.TunnelConfiguration{
					Replicas: 1,
				},
				//NodeSelector: nodeSelector,
			},
		}
		mergeEndpoint(&gwNodeGroup, nodeList.Items)
		return r.Create(ctx, &gwNodeGroup)
	}

	if !mergeEndpoint(&gwNodeGroup, nodeList.Items) {
		return nil
	}
	return r.Update(ctx, &gwNodeGroup)
}

func (r *nodeGroupReconciler) ScalingNodeGroup(ctx context.Context, pd cloud.Cloud, ng *v1.NodeGroup, desired ...uint) error {

	id := cloud.Id{
		Id:   ng.Spec.ScalingGroup.ScalingGroupId,
		Name: ng.Spec.ScalingGroup.ScalingGroupName,
	}
	_, err := pd.FindESSBy(ctx, id)
	if err != nil {
		return err
	}
	ud, err := r.getUserData(ng, pd.GetConfig())
	if err != nil {
		return errors.Wrapf(err, "get userdata failed")
	}
	var replica = ng.Spec.Replicas
	if len(desired) > 0 {
		replica = desired[0]
	}
	if ng.Status.Replicas == replica {
		return nil
	}
	scalingGrpModel := toScalingModel(ng, ud)
	klog.Infof("scaling node group through ess by: replicas=%d", replica)
	err = pd.ScaleNodeGroup(ctx, scalingGrpModel, replica)
	if err != nil {
		return err
	}
	ng.Status.Replicas = replica
	return r.Status().Update(ctx, ng)
}

func (r *nodeGroupReconciler) teardown(pd cloud.Cloud, ng *v1.NodeGroup) error {
	ctx := context.Background()
	var gwWorker ravenv1beta1.Gateway
	err := r.Get(ctx, client.ObjectKey{Name: "gw-" + ng.Name}, &gwWorker)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "get gateway worker %s", ng.Name)
		}
		klog.Infof("gateway[gw-%s] already not exist", ng.Name)
	} else {
		err = r.Delete(ctx, &gwWorker)
		if err != nil {
			klog.Errorf("delete worker gateway[gw-%s], %s", ng.Name, err.Error())
		}
	}
	scgrpId := ng.Spec.ScalingGroup.ScalingGroupId
	if scgrpId != "" {
		klog.Infof("[%s]teardwon: scale ess down to zero, [%s]", ng.Name, ng.Spec.ScalingGroup.ScalingGroupId)
		err := r.ScalingNodeGroup(ctx, pd, ng, 0)
		if err != nil {
			if !errors.Is(err, cloud.NotFound) {
				return errors.Wrapf(err, "scale down ess to zero")
			}
			klog.Infof("ess [%s] not found", scgrpId)
		}
		scalingModel := toScalingModel(ng, "")
		err = pd.DeleteESS(ctx, scalingModel)
		if err != nil {
			return errors.Wrapf(err, "delete scaling group [%s]", scgrpId)
		}
	}
	secGrpId := ng.Spec.SecurityGroup.SecurityGroupId
	if secGrpId != "" {
		klog.Infof("delete security group: %s", ng.Spec.SecurityGroup.SecurityGroupId)
		err := pd.DeleteSecurityGroup(ctx, cloud.Id{Id: secGrpId, Region: ng.Spec.Region})
		if err != nil {
			return errors.Wrapf(err, "delete security group [%s]", secGrpId)
		}
	}
	for _, vsw := range ng.Spec.VSwitch {
		if vsw.VSwitchId == "" {
			continue
		}
		klog.Infof("delete vswitch: %s", ng.Spec.VSwitch)
		err := pd.DeleteVSwitch(ctx, ng.Spec.VpcId, cloud.Id{Id: vsw.VSwitchId})
		if err != nil {
			return errors.Wrapf(err, fmt.Sprintf("delete vsw: %s", vsw.VSwitchId))
		}
	}
	if ng.Spec.RamRole.RoleId != "" {
		id := cloud.Id{
			Id:   ng.Spec.RamRole.RoleId,
			Name: ng.Spec.RamRole.RoleName,
		}
		err := pd.DeleteRAM(ctx, id, ng.Spec.RamRole.PolicyName)
		if err != nil {
			return errors.Wrapf(err, "delete ram role")
		}
	}
	if ng.Spec.VpcId == "" {
		return nil
	}
	klog.Infof("delete vpc: %s", ng.Spec.VpcId)
	return pd.DeleteVPC(ctx, ng.Spec.VpcId)
}

func (r *nodeGroupReconciler) createInfra(pd cloud.Cloud, ng *v1.NodeGroup) error {
	ctx := context.Background()
	switch ng.Spec.VpcId {
	case "":
		id := cloud.Id{
			Name: ng.Spec.VpcName,
			Tag:  buildTag(ng.Spec.VpcName),
		}
		vp, err := pd.FindVPC(ctx, id)
		if err == nil {
			ng.Spec.VpcId = vp.VpcId
			break
		}
		if !errors.Is(err, cloud.NotFound) {
			return errors.Wrapf(err, "find vpc")
		}

		vpcModel := cloud.VpcModel{
			VpcName: id.Name,
			Cidr:    ng.Spec.Cidr,
			Region:  ng.Spec.Region,
			Tag:     id.Tag,
		}
		vpcid, err := pd.CreateVPC(ctx, vpcModel)
		if err != nil {
			return errors.Wrap(err, "create vpc")
		}
		ng.Spec.VpcId = vpcid
	}

	for i, v := range ng.Spec.VSwitch {
		switch v.VSwitchId {
		case "":
			id := cloud.Id{
				Name: v.VSwitchName,
				Tag:  buildTag(v.VSwitchName),
			}
			vs, err := pd.FindVSwitch(ctx, ng.Spec.VpcId, id)
			if err == nil {
				ng.Spec.VSwitch[i].VSwitchId = vs.VSwitchId
				break
			}
			if !errors.Is(err, cloud.NotFound) {
				return errors.Wrapf(err, "find vSwitch %s", id)
			}
			vswithModel := cloud.VSwitchModel{
				Tag:         id.Tag,
				ZoneId:      v.ZoneId,
				CidrBlock:   v.CidrBlock,
				VSwitchName: id.Name,
			}
			vswitchId, err := pd.CreateVSwitch(ctx, ng.Spec.VpcId, vswithModel)
			if err != nil {
				return errors.Wrapf(err, "create vswitch")
			}
			ng.Spec.VSwitch[i].VSwitchId = vswitchId
		}
		klog.Infof("ensure vswitch: %s", v.VSwitchName)
	}

	klog.Infof("ensure security group: %s", ng.Spec.SecurityGroup.SecurityGroupName)
	switch ng.Spec.SecurityGroup.SecurityGroupId {
	case "":
		id := cloud.Id{
			Name: ng.Spec.SecurityGroup.SecurityGroupName,
			Tag:  buildTag(ng.Spec.SecurityGroup.SecurityGroupName),
		}
		klog.Infof("create security group: %v", id)
		sg, err := pd.FindSecurityGroup(ctx, ng.Spec.VpcId, id)
		if err == nil {
			ng.Spec.SecurityGroup.SecurityGroupId = sg.SecurityGroupId
			break
		}
		if !errors.Is(err, cloud.NotFound) {
			return errors.Wrapf(err, "find security group %s", id)
		}
		sgrpModel := cloud.SecurityGroupModel{
			Tag:               id.Tag,
			Region:            ng.Spec.Region,
			SecurityGroupName: id.Name,
		}
		sgrpId, err := pd.CreateSecurityGroup(ctx, ng.Spec.VpcId, sgrpModel)
		if err != nil {
			return errors.Wrapf(err, "create security group")
		}
		ng.Spec.SecurityGroup.SecurityGroupId = sgrpId
	}

	klog.Infof("ensure ramrole: %s", ng.Spec.RamRole.RoleName)
	switch ng.Spec.RamRole.RoleId {
	case "":
		id := cloud.Id{
			Name: ng.Spec.RamRole.RoleName,
			Tag:  buildTag(ng.Spec.RamRole.RoleName),
		}
		rl, err := pd.FindRAM(ctx, id)
		if err == nil {
			ng.Spec.RamRole.Arn = rl.Arn
			ng.Spec.RamRole.RoleId = rl.RamId
			break
		}
		if !errors.Is(err, cloud.NotFound) {
			return errors.Wrapf(err, "find ram role %s", id)
		}
		ramModel := cloud.RamModel{
			RamName:    id.Name,
			PolicyName: ng.Spec.RamRole.PolicyName,
		}
		klog.Infof("create ramrole by name: %s", id.Name)
		ramId, err := pd.CreateRAM(ctx, ramModel)
		if err != nil {
			return errors.Wrapf(err, "create ram")
		}
		ng.Spec.RamRole.RoleId = ramId

		id = cloud.Id{
			Name: ng.Spec.RamRole.PolicyName,
			Tag:  buildTag(ng.Spec.RamRole.PolicyName),
		}
		_, err = pd.FindPolicy(ctx, id)
		if err == nil {
			break
		}
		if !errors.Is(err, cloud.NotFound) {
			return errors.Wrapf(err, "find ram role policy %s", id)
		}
		klog.Infof("create ramrole policy by name: %s", id.Id)
		_, err = pd.CreatePolicy(ctx, ramModel)
		if err != nil {
			return errors.Wrapf(err, "create ram policy")
		}
	}

	klog.Infof("ensure scaling group: %s", ng.Spec.ScalingGroup.ScalingGroupName)
	switch ng.Spec.ScalingGroup.ScalingGroupId {
	case "":
		id := cloud.Id{
			Name: ng.Spec.ScalingGroup.ScalingGroupName,
			Tag:  buildTag(ng.Spec.ScalingGroup.ScalingGroupName),
		}
		sg, err := pd.FindESSBy(ctx, id)
		if err == nil {
			ng.Spec.ScalingGroup.ScalingGroupId = sg.ScalingGroupId
			break
		}
		if !errors.Is(err, cloud.NotFound) {
			return errors.Wrapf(err, "find scaling group")
		}
		scalingModel := cloud.ScalingGroupModel{
			Region:           ng.Spec.Region,
			ScalingGroupName: id.Name,
			Min:              0,
			Max:              9,
			Tag:              id.Tag,
			VSwitchId:        toVSwitchModel(ng.Spec.VSwitch),
		}
		klog.Infof("create scaling group by name: %s", id.Name)
		scalingId, err := pd.CreateESS(ctx, ng.Spec.VpcId, scalingModel)
		if err != nil {
			return errors.Wrapf(err, "create elastic scaling group")
		}
		ng.Spec.ScalingGroup.ScalingGroupId = scalingId
	}

	klog.Infof("ensure scaling config: %s", ng.Spec.ScalingGroup.ScalingConfig.ScalingCfgName)
	switch ng.Spec.ScalingGroup.ScalingConfig.ScalingCfgId {
	case "":
		id := cloud.Id{
			Name: ng.Spec.ScalingGroup.ScalingConfig.ScalingCfgName,
			Tag:  buildTag(ng.Spec.ScalingGroup.ScalingConfig.ScalingCfgName),
		}
		sc, err := pd.FindScalingConfig(ctx, id)
		if err == nil {
			ng.Spec.ScalingGroup.ScalingConfig.ScalingCfgId = sc.ScalingCfgId
			break
		}
		if !errors.Is(err, cloud.NotFound) {
			return errors.Wrapf(err, "find scaling config")
		}
		ud, err := r.getUserData(ng, pd.GetConfig())
		if err != nil {
			return errors.Wrapf(err, "get user data")
		}
		scfgModel := cloud.ScalingConfig{
			ImageId:        ng.Spec.ScalingGroup.ImageId,
			InstanceType:   ng.Spec.ScalingGroup.InstanceType,
			SecurityGrpId:  ng.Spec.SecurityGroup.SecurityGroupId,
			ScalingCfgName: id.Name,
			Tag:            id.Tag,
			UserData:       ud,
			RamRole:        ng.Spec.RamRole.RoleName,
		}
		klog.Infof("create scaling config by name: %s", id.Name)
		scfgId, err := pd.CreateScalingConfig(ctx, ng.Spec.ScalingGroup.ScalingGroupId, scfgModel)
		if err != nil {
			return errors.Wrapf(err, "create scaling config")
		}
		ng.Spec.ScalingGroup.ScalingConfig.ScalingCfgId = scfgId
	}

	klog.Infof("ensure scaling rule: %s", ng.Spec.ScalingGroup.ScalingRule.ScalingRuleName)
	switch ng.Spec.ScalingGroup.ScalingRule.ScalingRuleAri {
	case "":
		id := cloud.Id{
			Name: ng.Spec.ScalingGroup.ScalingRule.ScalingRuleName,
			Tag:  buildTag(ng.Spec.ScalingGroup.ScalingRule.ScalingRuleName),
		}
		sr, err := pd.FindScalingRule(ctx, id)
		if err == nil {
			ng.Spec.ScalingGroup.ScalingRule.ScalingRuleAri = sr.ScalingRuleAri
			ng.Spec.ScalingGroup.ScalingRule.ScalingRuleId = sr.ScalingRuleId
			break
		}
		if !errors.Is(err, cloud.NotFound) {
			return errors.Wrapf(err, "find scaling rule")
		}
		sruleModel := cloud.ScalingRule{
			Region:          ng.Spec.Region,
			ScalingRuleName: id.Name,
		}
		klog.Infof("create scaling rule by name: %s", id.Name)
		sruleArn, err := pd.CreateScalingRule(ctx, ng.Spec.ScalingGroup.ScalingGroupId, sruleModel)
		if err != nil {
			return errors.Wrapf(err, "create scaling rule")
		}
		ng.Spec.ScalingGroup.ScalingRule.ScalingRuleAri = sruleArn.ScalingRuleAri
		ng.Spec.ScalingGroup.ScalingRule.ScalingRuleId = sruleArn.ScalingRuleId
		return pd.EnableScalingGroup(ctx, ng.Spec.ScalingGroup.ScalingGroupId,
			ng.Spec.ScalingGroup.ScalingConfig.ScalingCfgId)
	}
	return nil
}

func (r *nodeGroupReconciler) getUserData(ng *v1.NodeGroup, cfg cloud.Config) (string, error) {
	var (
		publicIp string
		req      v1.Request
	)

	var ctx = context.TODO()

	//nodeSelector, err := labels.Parse("node-role.kubernetes.io/control-plane=")
	//if err != nil {
	//	return "", errors.Wrap(err, "parse node selector")
	//}Ï
	//var nodeList corev1.NodeList
	//err = r.List(ctx, &nodeList, &client.ListOptions{LabelSelector: nodeSelector})
	//if err != nil {
	//	return "", errors.Wrapf(err, "list master")
	//}
	//if len(nodeList.Items) == 0 {
	//	return "", errors.New("no master node found")
	//}
	//var master = ""
	//for _, node := range nodeList.Items {
	//	master = node.Name
	//	break
	//}
	//
	//var gw       ravenv1beta1.GatewayList
	//err = r.List(context.TODO(), &gw)
	//if err != nil {
	//	return "", errors.Wrapf(err, "get gateways")
	//}
	//if len(gw.Items) == 0 {
	//	return "", errors.New("no gateway node found")
	//}
	//var publicIp = ""
	//for _, gateway := range gw.Items {
	//	for _, node := range gateway.Status.ActiveEndpoints {
	//		if node.NodeName == master {
	//			if node.PublicIP != "" {
	//				publicIp = node.PublicIP
	//				klog.Infof("use public ip: %s", publicIp)
	//				break
	//			}
	//		}
	//	}
	//}
	err := r.Get(ctx, client.ObjectKey{Name: v1.KubernetesReq}, &req)
	if err != nil {
		return "", errors.Wrapf(err, "get kubernetes request object")
	}

	publicIp = req.Spec.AccessPoint.Internet
	klog.Infof("use public ip from api access point [%s] for userdata", publicIp)
	return tool.RenderConfig(
		"userdata", userdata, struct {
			Token     string
			Endpoint  string
			Group     string
			CloudType string
		}{
			Endpoint:  fmt.Sprintf("%s:%s", publicIp, req.Spec.AccessPoint.APIPort),
			Group:     ng.Name,
			CloudType: cfg.Type,
			Token:     req.Spec.Config.Token,
		},
	)
}

func newCloud(
	r client.Client,
	ng *v1.NodeGroup,
) (cloud.Cloud, error) {
	provider := ng.Spec.Provider
	if provider == "" {
		return nil, fmt.Errorf("node provider is empty")
	}
	var pv v1.Provider
	err := r.Get(context.TODO(), client.ObjectKey{Name: provider}, &pv)
	if err != nil {
		return nil, err
	}
	pdFunc, err := cloud.Get(pv.Spec.Type)
	if err != nil {
		return nil, err
	}
	return pdFunc(cloud.Config{AuthInfo: pv.Spec.AuthInfo})
}

func toVSwitchModel(abc []v1.VSwitch) []cloud.VSwitchModel {
	var vsw []cloud.VSwitchModel
	for _, v := range abc {
		vsw = append(vsw, cloud.VSwitchModel{
			ZoneId:      v.ZoneId,
			VSwitchId:   v.VSwitchId,
			VSwitchName: v.VSwitchName,
			CidrBlock:   v.CidrBlock,
			Tag:         buildTag(v.VSwitchName),
		})
	}
	return vsw
}

func (r *nodeGroupReconciler) SetNodeGroupDft(ctx context.Context, ng *v1.NodeGroup) error {
	setBackupLabel(ng)
	setFinalizer(ng)
	rid := fmt.Sprintf("%s.%s", tool.RandomID(4), ng.Name)
	if ng.Spec.VpcName == "" {
		ng.Spec.VpcName = rName(rid, "vpc")
	}
	if ng.Spec.Cidr == "" {
		network, err := common.NewCluster(r.Client).AllocateNet(ctx, ng.Name)
		if err != nil {
			return err
		}
		ng.Spec.Cidr = network
	} else {
		_, _, err := net.ParseCIDR(ng.Spec.Cidr)
		if err != nil {
			return errors.Wrapf(err, "validate cidr %s", ng.Spec.Cidr)
		}
	}
	if ng.Spec.Region == "" {
		ng.Spec.Region = "cn-hangzhou"
	}
	if ng.Spec.CPUs <= 0 {
		ng.Spec.CPUs = 4
	}
	if ng.Spec.Memory <= 0 {
		ng.Spec.Memory = 8
	}
	if ng.Spec.RamRole.RoleName == "" {
		ng.Spec.RamRole.RoleName = rName(rid, "ram")
		ng.Spec.RamRole.PolicyName = rName(rid, "policy")
	}
	if ng.Spec.SecurityGroup.SecurityGroupName == "" {
		ng.Spec.SecurityGroup.SecurityGroupName = rName(rid, "security_group")
	}
	if ng.Spec.ScalingGroup.ScalingGroupName == "" {
		ng.Spec.ScalingGroup.ScalingGroupName = rName(rid, "ess")
		ng.Spec.ScalingGroup.ScalingRule.ScalingRuleName = rName(rid, "ess.scaling_rule")
		ng.Spec.ScalingGroup.ScalingConfig.ScalingCfgName = rName(rid, "ess.scaling_config")
	}
	if ng.Spec.ScalingGroup.ImageId == "" {
		ng.Spec.ScalingGroup.ImageId = "aliyun_3_x64_20G_alibase_20240819.vhd"
	}
	if ng.Spec.ScalingGroup.InstanceType == "" {
		ng.Spec.ScalingGroup.InstanceType = "ecs.c7.xlarge"
	}
	if ng.Spec.ScalingGroup.Max <= 0 {
		ng.Spec.ScalingGroup.Max = 10
	}
	ip, _, err := net.ParseCIDR(ng.Spec.Cidr)
	if err != nil {
		return errors.Wrapf(err, "error parsing vpc cidr %s", ng.Spec.Cidr)
	}
	if len(ng.Spec.Eip) == 0 {
		eip := []v1.Eip{
			{EipName: rName(rid, "eip", 0)},
		}
		ng.Spec.Eip = eip
	}
	for i, eip := range ng.Spec.Eip {
		if eip.EipName == "" {
			ng.Spec.Eip[i].EipName = rName(rid, "eip", i)
		}
	}
	if len(ng.Spec.VSwitch) == 0 {
		n := iplib.NewNet4(ip, 24)
		vsw := []v1.VSwitch{
			{VSwitchName: rName(rid, "vswitch", 0),
				ZoneId:    fmt.Sprintf("%s-b", ng.Spec.Region),
				CidrBlock: n.String(),
			},
		}
		ng.Spec.VSwitch = vsw
	}

	n := iplib.NewNet4(ip, 24)
	for i, vswitch := range ng.Spec.VSwitch {
		if vswitch.VSwitchName == "" {
			ng.Spec.VSwitch[i].VSwitchName = rName(rid, "vswitch", i)
		}
		if vswitch.ZoneId == "" {
			vswitch.ZoneId = fmt.Sprintf("%s-b", ng.Spec.Region)
		}
		if vswitch.CidrBlock == "" {
			vswitch.CidrBlock = n.String()
			n = n.NextNet(24)
		}
	}
	return nil
}

func setBackupLabel(ng *v1.NodeGroup) {
	if ng.Labels == nil {
		ng.Labels = map[string]string{}
	}
	_, ok := ng.Labels[v1.XDPIN_BACKUP]
	if !ok {
		ng.Labels[v1.XDPIN_BACKUP] = "true"
	}
}

func setFinalizer(ng *v1.NodeGroup) {
	var finalizers []string
	for _, f := range ng.Finalizers {
		if f == v1.Finalizer {
			continue
		}
		finalizers = append(finalizers, f)
	}
	finalizers = append(finalizers, v1.Finalizer)
	ng.Finalizers = finalizers
}

func rName(name, r string, idx ...int) string {
	if len(idx) > 0 {
		return fmt.Sprintf("%d.%s.%s.xdpin.cn", idx[0], name, r)
	}
	return fmt.Sprintf("%s.%s.xdpin.cn", name, r)
}

var userdata = `#!/bin/bash
set -e
version=0.1.0
OS=$(uname|tr '[:upper:]' '[:lower:]')
arch=$(uname -m|tr '[:upper:]' '[:lower:]')
case $arch in
"amd64")
        arch=x86_64
        ;;
"arm64")
        arch=aarch64
        ;;
"x86_64")
	;;
*)
        echo "unknown arch: ${arch} for ${OS}"; exit 1
        ;;
esac

server=http://host-wdrip-cn-hangzhou.oss-cn-hangzhou.aliyuncs.com

need_install=0
if [[ -f /usr/local/bin/meridian ]];
then
        wget -q -O /tmp/meridian.${OS}.${arch}.tar.gz.sum \
                $server/bin/${OS}/${arch}/${version}/meridian.${OS}.${arch}.tar.gz.sum
        m1=$(cat /tmp/meridian.${OS}.${arch}.tar.gz.sum |awk '{print $1}')
        m2=$(md5sum /usr/local/bin/meridian |awk '{print $1}')
        if [[ "$m1" == "$m2" ]];
        then
                need_install=0
        else
                need_install=1
        fi
else
        need_install=1
fi

if [[ "$need_install" == "1" ]];
then
        wget -q -O /tmp/meridian.${OS}.${arch}.tar.gz \
                $server/bin/${OS}/${arch}/${version}/meridian.${OS}.${arch}.tar.gz

        wget -q -O /tmp/meridian.${OS}.${arch}.tar.gz.sum \
                $server/bin/${OS}/${arch}/${version}/meridian.${OS}.${arch}.tar.gz.sum
        tar xf /tmp/meridian.${OS}.${arch}.tar.gz -C /tmp
        sudo mv -f /tmp/bin/meridian.${OS}.${arch} /usr/local/bin/meridian
        rm -rf /tmp/meridian.${OS}.${arch}.tar.gz /tmp/meridian.${OS}.${arch}.tar.gz.sum
fi

/usr/local/bin/meridian join --role worker --api-server {{ .Endpoint }} --token {{ .Token }} --group "{{ .Group }}" --cloud "{{ .CloudType}}"
`
