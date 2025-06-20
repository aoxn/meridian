package raven

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/controller/raven/predicator"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/pkg/errors"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	calicov3 "github.com/openyurtio/openyurt/pkg/apis/calico/v3"
	"github.com/openyurtio/openyurt/pkg/apis/raven"
	ravenv1beta1 "github.com/openyurtio/openyurt/pkg/apis/raven/v1beta1"
)

const (
	ActiveEndpointsName     = "ActiveEndpointName"
	ActiveEndpointsPublicIP = "ActiveEndpointsPublicIP"
	ActiveEndpointsType     = "ActiveEndpointsType"
)

func AddGateway(
	mgr manager.Manager,
) error {
	return addGateway(mgr, newGatewayReconciler(mgr))
}

// newGatewayReconciler returns a new reconcile.Reconciler
func newGatewayReconciler(
	mgr manager.Manager,
) reconcile.Reconciler {
	recon := &reconcileGateway{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetEventRecorderFor("AutoHeal"),
	}
	return recon
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func addGateway(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(
		"gateway-controller", mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: 1,
		},
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		source.Kind(mgr.GetCache(), &ravenv1beta1.Gateway{}),
		&handler.EnqueueRequestForObject{},
	)
	if err != nil {
		return err
	}

	// Watch for changes to Nodes
	err = c.Watch(
		source.Kind(mgr.GetCache(), &corev1.Node{}),
		&predicator.EnqueueGatewayForNode{},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		source.Kind(mgr.GetCache(), &corev1.ConfigMap{}),
		predicator.NewRavenGateway(mgr.GetClient()),
		predicate.NewPredicateFuncs(
			func(object client.Object) bool {
				cm, ok := object.(*corev1.ConfigMap)
				if !ok {
					return false
				}
				if cm.GetNamespace() != tool.WorkingNamespace {
					return false
				}
				if cm.GetName() != tool.RavenGlobalConfig {
					return false
				}
				return true
			}))
}

// blank assignment to verify that ReconcileAutoRepair implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconcileGateway{}

// reconcileGateway reconciles a AutoHeal object
type reconcileGateway struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	scheme *runtime.Scheme

	//record event recorder
	recorder record.EventRecorder
}

var _ reconcile.Reconciler = &reconcileGateway{}

//+kubebuilder:rbac:groups=raven.openyurt.io,resources=gateways,verbs=get;create;delete;update
//+kubebuilder:rbac:groups=raven.openyurt.io,resources=gateways/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=raven.openyurt.io,resources=gateways/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get
//+kubebuilder:rbac:groups=crd.projectcalico.org,resources=blockaffinities,verbs=get

// Reconcile reads that state of the cluster for a Gateway object and makes changes based on the state read
// and what is in the Gateway.Spec
func (r *reconcileGateway) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {

	var gw ravenv1beta1.Gateway
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, &gw); err != nil {
		klog.Errorf("unable get gateway %s, error %s", req.String(), err.Error())
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	// get all managed nodes
	var nodeList corev1.NodeList
	nodeSelector, err := labels.Parse(fmt.Sprintf(raven.LabelCurrentGateway+"=%s", gw.Name))
	if err != nil {
		return reconcile.Result{}, err
	}
	err = r.List(ctx, &nodeList, &client.ListOptions{LabelSelector: nodeSelector})
	if err != nil {
		klog.Errorf("unable to list node error %s", err.Error())
		return reconcile.Result{}, err
	}

	// 1. try to elect an active endpoint if possible
	activeEp := r.electActiveEndpoint(nodeList, &gw)
	r.recordEndpointEvent(&gw, gw.Status.ActiveEndpoints, activeEp)
	gw.Status.ActiveEndpoints = activeEp
	r.configEndpoints(ctx, &gw)
	// 2. get nodeInfo list of nodes managed by the Gateway
	var nodes []ravenv1beta1.NodeInfo
	for _, v := range nodeList.Items {
		podCIDRs, err := r.getPodCIDRs(ctx, v)
		if err != nil {
			klog.Errorf("unable to get podCIDR for node %s error %s", v.GetName(), err.Error())
			return reconcile.Result{}, err
		}
		nodes = append(nodes, ravenv1beta1.NodeInfo{
			NodeName:  v.Name,
			PrivateIP: tool.GetNodeInternalIP(v),
			Subnets:   podCIDRs,
		})
	}

	klog.V(5).Infof("debug raven, gateway: %s, nodelist=%d, yaml: %s", req.Name, len(nodeList.Items), tool.PrettyYaml(gw))

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].NodeName < nodes[j].NodeName })
	gw.Status.Nodes = nodes
	r.addExtraAllowedSubnet(&gw)
	err = r.Status().Update(ctx, &gw)
	if err != nil {
		if apierrs.IsConflict(err) {
			klog.Warningf("unable to update gateway.status, error %s", err.Error())
			return reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		klog.Errorf("unable to update %s gateway.status, error %s", gw.GetName(), err.Error())
		return reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
	}
	return reconcile.Result{}, nil
}

func (r *reconcileGateway) setPublicIp(ctx context.Context, gw *ravenv1beta1.Gateway) error {
	var (
		req      v1.Request
		nodeList corev1.NodeList
	)
	nodeSelector, err := labels.Parse("node-role.kubernetes.io/control-plane=")
	if err != nil {
		return errors.Wrap(err, "parse node selector")
	}
	err = r.List(ctx, &nodeList, &client.ListOptions{LabelSelector: nodeSelector})
	if err != nil {
		return errors.Wrapf(err, "list master for gateway public ip")
	}
	if len(nodeList.Items) == 0 {
		return errors.New("no master node found")
	}
	var master = ""
	for _, node := range nodeList.Items {
		master = node.Name
		break
	}

	var publicIp = ""
	for _, node := range gw.Status.ActiveEndpoints {
		if node.NodeName == master {
			if node.PublicIP != "" {
				publicIp = node.PublicIP
				klog.Infof("use public ip: %s", publicIp)
				break
			}
		}
	}

	if publicIp == "" {
		return fmt.Errorf("public ip not found for master %s", master)
	}
	err = r.Get(context.TODO(), client.ObjectKey{Name: v1.KubernetesReq}, &req)
	if err != nil {
		return errors.Wrapf(err, "get kubernetes request object")
	}
	req.Spec.AccessPoint.Internet = publicIp
	klog.Infof("set public ip for request resource[%s]: %s", v1.KubernetesReq, publicIp)
	return r.Update(ctx, &req)
}

func (r *reconcileGateway) recordEndpointEvent(sourceObj *ravenv1beta1.Gateway, previous, current []*ravenv1beta1.Endpoint) {
	sort.Slice(previous, func(i, j int) bool { return previous[i].NodeName < previous[j].NodeName })
	sort.Slice(current, func(i, j int) bool { return current[i].NodeName < current[j].NodeName })
	if len(current) != 0 && !reflect.DeepEqual(previous, current) {
		eps, num := getActiveEndpointsInfo(current)
		for i := 0; i < num; i++ {
			r.recorder.Event(sourceObj.DeepCopy(), corev1.EventTypeNormal,
				ravenv1beta1.EventActiveEndpointElected,
				fmt.Sprintf("The endpoint hosted by node %s has been elected active endpoint, publicIP: %s, type: %s", eps[ActiveEndpointsName][i], eps[ActiveEndpointsPublicIP][i], eps[ActiveEndpointsType][i]))
		}

		klog.V(2).Infof("elected new active endpoint: %s, %s=%s, %s=%s", eps[ActiveEndpointsName], "publicIP", eps[ActiveEndpointsPublicIP], "type", eps[ActiveEndpointsType])
		return
	}
	if len(previous) != 0 && !reflect.DeepEqual(previous, current) {
		eps, num := getActiveEndpointsInfo(previous)
		for i := 0; i < num; i++ {
			r.recorder.Event(sourceObj.DeepCopy(), corev1.EventTypeWarning,
				ravenv1beta1.EventActiveEndpointLost,
				fmt.Sprintf("The active endpoint hosted by node %s was change, publicIP: %s, type :%s", eps[ActiveEndpointsName][i], eps[ActiveEndpointsPublicIP][i], eps[ActiveEndpointsType][i]))
		}
		klog.V(2).InfoS("active endpoint lost", "nodeName", eps[ActiveEndpointsName], "publicIP", eps[ActiveEndpointsPublicIP], "type", eps[ActiveEndpointsType])
		return
	}
}

// electActiveEndpoint tries to elect an active Endpoint.
// If the current active endpoint remains valid, then we don't change it.
// Otherwise, try to elect a new one.
func (r *reconcileGateway) electActiveEndpoint(nodeList corev1.NodeList, gw *ravenv1beta1.Gateway) []*ravenv1beta1.Endpoint {
	// get all ready nodes referenced by endpoints
	readyNodes := make(map[string]*corev1.Node)
	for _, v := range nodeList.Items {
		if tool.IsNodeReady(v) {
			readyNodes[v.Name] = &v
		}
	}
	// init a endpoints slice
	enableProxy, enableTunnel := CheckServer(context.TODO(), r.Client)

	klog.Infof("has %d ready nodes, node [%s], status: proxy=%t, tunnel=%t", len(readyNodes), tool.NodeArray(readyNodes), enableProxy, enableTunnel)
	eps := make([]*ravenv1beta1.Endpoint, 0)
	if enableProxy {
		eps = append(eps, electEndpoints(gw, ravenv1beta1.Proxy, readyNodes)...)
	}
	if enableTunnel {
		eps = append(eps, electEndpoints(gw, ravenv1beta1.Tunnel, readyNodes)...)
	}
	sort.Slice(eps, func(i, j int) bool { return eps[i].NodeName < eps[j].NodeName })
	return eps
}

func electEndpoints(gw *ravenv1beta1.Gateway, endpointType string, readyNodes map[string]*corev1.Node) []*ravenv1beta1.Endpoint {
	eps := make([]*ravenv1beta1.Endpoint, 0)
	var replicas int
	switch endpointType {
	case ravenv1beta1.Proxy:
		replicas = gw.Spec.ProxyConfig.Replicas
	case ravenv1beta1.Tunnel:
		replicas = gw.Spec.TunnelConfig.Replicas
	default:
		replicas = 1
	}
	if replicas == 0 {
		replicas = 1
	}
	checkCandidates := func(ep *ravenv1beta1.Endpoint) bool {
		if _, ok := readyNodes[ep.NodeName]; ok && ep.Type == endpointType {
			return true
		}
		return false
	}

	// the current active endpoint is still competent.
	candidates := make(map[string]*ravenv1beta1.Endpoint, 0)
	for _, activeEndpoint := range gw.Status.ActiveEndpoints {
		if checkCandidates(activeEndpoint) {
			for _, ep := range gw.Spec.Endpoints {
				if ep.NodeName == activeEndpoint.NodeName && ep.Type == activeEndpoint.Type {
					candidates[activeEndpoint.NodeName] = ep.DeepCopy()
				}
			}
		}
	}
	for _, aep := range candidates {
		if len(eps) == replicas {
			aepInfo, _ := getActiveEndpointsInfo(eps)
			klog.V(4).Infof("elect %d active endpoints %s for gateway %s/%s",
				len(eps), fmt.Sprintf("[%s]", strings.Join(aepInfo[ActiveEndpointsName], ",")), gw.GetNamespace(), gw.GetName())
			return eps
		}
		klog.V(1).Infof("node %s is active endpoints, type is %s", aep.NodeName, aep.Type)
		klog.V(1).Infof("add node %v", aep.DeepCopy())
		eps = append(eps, aep.DeepCopy())
	}
	klog.Infof("debug spec.Endpoints=%d, replicas=%d, eps=%d", len(gw.Spec.Endpoints), replicas, len(eps))
	for _, ep := range gw.Spec.Endpoints {
		if _, ok := candidates[ep.NodeName]; !ok && checkCandidates(&ep) {
			if len(eps) == replicas {
				aepInfo, _ := getActiveEndpointsInfo(eps)
				klog.Infof("elect %d active endpoints %s for gateway %s/%s",
					len(eps), fmt.Sprintf("[%s]", strings.Join(aepInfo[ActiveEndpointsName], ",")), gw.GetNamespace(), gw.GetName())
				return eps
			}
			klog.V(1).Infof("node %s is active endpoints, type is %s", ep.NodeName, ep.Type)
			klog.V(1).Infof("add node %v", ep.DeepCopy())
			eps = append(eps, ep.DeepCopy())
		}
	}
	return eps
}

// getPodCIDRs returns the pod IP ranges assigned to the node.
func (r *reconcileGateway) getPodCIDRs(ctx context.Context, node corev1.Node) ([]string, error) {
	podCIDRs := make([]string, 0)
	for key := range node.Annotations {
		if strings.Contains(key, "projectcalico.org") {
			var blockAffinityList calicov3.BlockAffinityList
			err := r.List(ctx, &blockAffinityList)
			if err != nil {
				err = fmt.Errorf("unable to list calico blockaffinity: %s", err)
				return nil, err
			}
			for _, v := range blockAffinityList.Items {
				if v.Spec.Node != node.Name || v.Spec.State != "confirmed" {
					continue
				}
				podCIDRs = append(podCIDRs, v.Spec.CIDR)
			}
			return podCIDRs, nil
		}
	}
	return append(podCIDRs, node.Spec.PodCIDR), nil
}

func getActiveEndpointsInfo(eps []*ravenv1beta1.Endpoint) (map[string][]string, int) {
	infos := make(map[string][]string)
	infos[ActiveEndpointsName] = make([]string, 0)
	infos[ActiveEndpointsPublicIP] = make([]string, 0)
	infos[ActiveEndpointsType] = make([]string, 0)
	if len(eps) == 0 {
		return infos, 0
	}
	for _, ep := range eps {
		infos[ActiveEndpointsName] = append(infos[ActiveEndpointsName], ep.NodeName)
		infos[ActiveEndpointsPublicIP] = append(infos[ActiveEndpointsPublicIP], ep.PublicIP)
		infos[ActiveEndpointsType] = append(infos[ActiveEndpointsType], ep.Type)
	}
	return infos, len(infos[ActiveEndpointsName])
}

func (r *reconcileGateway) configEndpoints(ctx context.Context, gw *ravenv1beta1.Gateway) {
	enableProxy, enableTunnel := CheckServer(ctx, r.Client)
	for idx, val := range gw.Status.ActiveEndpoints {
		if gw.Status.ActiveEndpoints[idx].Config == nil {
			gw.Status.ActiveEndpoints[idx].Config = make(map[string]string)
		}
		switch val.Type {
		case ravenv1beta1.Proxy:
			gw.Status.ActiveEndpoints[idx].Config[tool.RavenEnableProxy] = strconv.FormatBool(enableProxy)
		case ravenv1beta1.Tunnel:
			gw.Status.ActiveEndpoints[idx].Config[tool.RavenEnableTunnel] = strconv.FormatBool(enableTunnel)
		default:
		}
	}
	return
}

func (r *reconcileGateway) addExtraAllowedSubnet(gw *ravenv1beta1.Gateway) {
	if gw.Annotations == nil || gw.Annotations[tool.ExtraAllowedSourceCIDRs] == "" {
		return
	}
	subnets := strings.Split(gw.Annotations[tool.ExtraAllowedSourceCIDRs], ",")
	var gatewayName string
	for _, aep := range gw.Status.ActiveEndpoints {
		if aep.Type == ravenv1beta1.Tunnel {
			gatewayName = aep.NodeName
			break
		}
	}
	for idx, node := range gw.Status.Nodes {
		if node.NodeName == gatewayName {
			gw.Status.Nodes[idx].Subnets = append(gw.Status.Nodes[idx].Subnets, subnets...)
			break
		}
	}
}
