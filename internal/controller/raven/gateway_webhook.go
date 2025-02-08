package raven

import (
	"context"
	ravenv1beta1 "github.com/openyurtio/openyurt/pkg/apis/raven/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func AddGatewayWebhook(
	mgr manager.Manager,
) error {
	hook := newGatewayDft()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&ravenv1beta1.Gateway{}).
		WithDefaulter(hook).
		WithValidator(hook).Complete()
}

// blank assignment to verify that ReconcileAutoRepair implements reconcile.Reconciler
var (
	_ webhook.CustomDefaulter = &gatewayWebhook{}
	_ webhook.CustomValidator = &gatewayWebhook{}
)

func newGatewayDft() *gatewayWebhook {
	return &gatewayWebhook{}
}

// gatewayWebhook reconciles a AutoHeal object
type gatewayWebhook struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	scheme *runtime.Scheme

	//record event recorder
	recorder record.EventRecorder
}

func (g gatewayWebhook) Default(ctx context.Context, obj runtime.Object) error {
	gw, ok := obj.(*ravenv1beta1.Gateway)
	if !ok {
		return nil
	}

	ravenv1beta1.SetDefaultsGateway(gw)
	klog.Infof("set gateway default")
	return nil
}

func (g gatewayWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (g gatewayWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (g gatewayWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}
