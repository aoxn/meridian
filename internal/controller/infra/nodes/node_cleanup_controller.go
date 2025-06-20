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

package nodes

import (
	"context"
	v1 "github.com/aoxn/meridian/api/v1"
	"strings"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/tool"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func AddNodeCleanup(
	mgr manager.Manager,
) error {

	r := &nodeCleanupReconciler{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetEventRecorderFor("Node"),
	}
	// Create a new controller
	c, err := controller.New(
		"node-cleanup-controller", mgr,
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

// blank assignment to verify that nodeCleanupReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &nodeCleanupReconciler{}

// nodeCleanupReconciler reconciles a Infra object
type nodeCleanupReconciler struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

func (r *nodeCleanupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var node corev1.Node
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, &node); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if tool.IsNodeReady(node) || tool.NodeIsMaster(&node) {
		return ctrl.Result{}, nil
	}
	gid := tool.GetNodeGroupID(&node)
	if gid == "" {
		return ctrl.Result{}, nil
	}
	var ng v1.NodeGroup
	if err := r.Get(ctx, client.ObjectKey{Name: gid}, &ng); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	pd, err := cloud.NewCloud(r.Client, ng.Spec.Provider)
	if err != nil {
		klog.Errorf("init cloud provider[%s] with error: %s", tool.GetProviderName(&node), err.Error())
		return ctrl.Result{}, err
	}

	id := pd.GetInstanceId(&node)
	if id == "" {
		return ctrl.Result{}, nil
	}
	_, err = pd.FindInstance(ctx, cloud.Id{Id: id})
	if err != nil && (!apierrors.IsNotFound(err) ||
		strings.Contains(err.Error(), "NotFound")) {
		// clean up
		klog.Infof("instance [%s] not found, do ecs cleanup", id)
		return ctrl.Result{}, r.Delete(ctx, &node)
	}
	return ctrl.Result{}, nil
}
