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
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/controller/common"
	"github.com/aoxn/meridian/internal/tool"
	ravenv1beta1 "github.com/openyurtio/openyurt/pkg/apis/raven/v1beta1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func AddNode(
	mgr manager.Manager,
) error {

	r := &nodeReconciler{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetEventRecorderFor("Node"),
	}
	// Create a new controller
	c, err := controller.New(
		"node-controller", mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: 1,
		},
	)
	if err != nil {
		return err
	}
	return c.Watch(
		source.Kind(mgr.GetCache(), &corev1.Node{}),
		&handler.EnqueueRequestForObject{},
	)
}

// blank assignment to verify that ReconcileAutoRepair implements reconcile.Reconciler
var _ reconcile.Reconciler = &nodeReconciler{}

// nodeReconciler reconciles a Infra object
type nodeReconciler struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=xdpin.cn,resources=nodegroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=xdpin.cn,resources=nodegroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=xdpin.cn,resources=nodegroups/finalizers,verbs=update

func (r *nodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var node corev1.Node
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, &node); err != nil {
		klog.Errorf("unable get node %s, error %s", req.String(), err.Error())
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if node.Labels == nil {
		return reconcile.Result{}, nil
	}

	if tool.NodeIsMaster(&node) {
		return reconcile.Result{}, nil
	}

	if !node.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	var errList = tool.Errors{}
	//err := r.createMasterGateway(ctx, &node)
	//if err != nil {
	//	errList = append(errList, err)
	//}
	err := common.NewCluster(r.Client).ReconcileClusterAddons(ctx, cloud.Config{})
	if err != nil {
		errList = append(errList, err)
	}
	return ctrl.Result{}, errList.HasError()
}

func (r *nodeReconciler) createMasterGateway(ctx context.Context, node *corev1.Node) error {

	var (
		gwName      = "gw-master"
		gwNodeGroup ravenv1beta1.Gateway
	)

	err := r.Get(ctx, client.ObjectKey{Name: gwName}, &gwNodeGroup)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "get gateway nodegroup %s", node.Name)
		}
		gwNodeGroup = ravenv1beta1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name: gwName,
			},
			Spec: ravenv1beta1.GatewaySpec{
				Endpoints: []ravenv1beta1.Endpoint{{
					NodeName: node.Name,
					UnderNAT: true,
					Type:     ravenv1beta1.Tunnel,
				}},
				ProxyConfig: ravenv1beta1.ProxyConfiguration{
					Replicas: 1,
				},
				TunnelConfig: ravenv1beta1.TunnelConfiguration{
					Replicas: 1,
				},
			},
		}

		klog.Infof("create gateway gw-master")
		return r.Create(ctx, &gwNodeGroup)
	}
	return nil
}
