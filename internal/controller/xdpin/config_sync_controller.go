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

package xdpin

import (
	"context"
	api "github.com/aoxn/meridian/api/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func Add(
	mgr manager.Manager,
) error {
	return addConfigSync(mgr)
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func addConfigSync(mgr manager.Manager) error {
	options := controller.Options{
		MaxConcurrentReconciles: 1,
	}
	recon := &configSyncReconciler{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetEventRecorderFor("config-sync-controller"),
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ConfigMap{}).WithOptions(options).Complete(recon)
}

// blank assignment to verify that ReconcileAutoRepair implements reconcile.Reconciler
var _ reconcile.Reconciler = &configSyncReconciler{}

type configSyncReconciler struct {
	client.Client
	scheme *runtime.Scheme

	//record event recorder
	recorder record.EventRecorder
}

func (r *configSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg v1.ConfigMap
	err := r.Get(context.TODO(), req.NamespacedName, &cfg)
	if err != nil {
		if errors.IsNotFound(err) {
			Remove(req.NamespacedName.String())
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			klog.Infof("configmap %s not found, might be delete option, do nothing.", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	if !hasLabel(&cfg) {
		return reconcile.Result{}, nil
	}
	Set(&cfg)
	klog.Infof("reconcile configmap: %s, set.", req)
	return ctrl.Result{}, nil
}

func hasLabel(cm *v1.ConfigMap) bool {
	if cm.Labels == nil {
		return false
	}
	_, ok := cm.Labels[api.XDPIN_BACKUP]
	return ok
}

// SetupWithManager sets up the controller with the Manager.
func (r *configSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	options := controller.Options{
		MaxConcurrentReconciles: 1,
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ConfigMap{}).WithOptions(options).Complete(r)
}
