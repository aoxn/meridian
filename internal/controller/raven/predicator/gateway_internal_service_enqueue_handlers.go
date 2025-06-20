/*
Copyright 2023 The OpenYurt Authors.

Licensed under the Apache License, Version 2.0 (the License);
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an AS IS BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package predicator

import (
	"context"
	"github.com/aoxn/meridian/internal/tool"
	"k8s.io/apimachinery/pkg/types"
	"net"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"

	ravenv1beta1 "github.com/openyurtio/openyurt/pkg/apis/raven/v1beta1"
)

type EnqueueRequestForGatewayEventInternal struct{}

func (h *EnqueueRequestForGatewayEventInternal) Create(ctx context.Context, e event.CreateEvent, q workqueue.RateLimitingInterface) {
	gw, ok := e.Object.(*ravenv1beta1.Gateway)
	if !ok {
		klog.Errorf("could not assert runtime Object %s/%s to v1beta1.Gateway", e.Object.GetNamespace(), e.Object.GetName())
		return
	}
	if gw.Spec.ExposeType == "" {
		return
	}
	klog.V(4).Infof("enqueue service %s/%s due to gateway %s create event", tool.WorkingNamespace, tool.GatewayProxyInternalService, gw.GetName())
	AddGatewayProxyInternalService(q)
}

func (h *EnqueueRequestForGatewayEventInternal) Update(ctx context.Context, e event.UpdateEvent, q workqueue.RateLimitingInterface) {
	newGw, ok := e.ObjectNew.(*ravenv1beta1.Gateway)
	if !ok {
		klog.Errorf("could not assert runtime Object %s/%s to v1beta1.Gateway", e.ObjectNew.GetNamespace(), e.ObjectNew.GetName())
		return
	}
	oldGw, ok := e.ObjectOld.(*ravenv1beta1.Gateway)
	if !ok {
		klog.Errorf("could not assert runtime Object %s/%s to v1beta1.Gateway", e.ObjectOld.GetNamespace(), e.ObjectOld.GetName())
		return
	}
	if oldGw.Spec.ExposeType == "" && newGw.Spec.ExposeType == "" {
		return
	}
	klog.V(4).Infof("enqueue service %s/%s due to gateway %s update event", tool.WorkingNamespace, tool.GatewayProxyInternalService, newGw.GetName())
	AddGatewayProxyInternalService(q)
}

func (h *EnqueueRequestForGatewayEventInternal) Delete(ctx context.Context, e event.DeleteEvent, q workqueue.RateLimitingInterface) {
	gw, ok := e.Object.(*ravenv1beta1.Gateway)
	if !ok {
		klog.Errorf("could not assert runtime Object %s/%s to v1beta1.Gateway", e.Object.GetNamespace(), e.Object.GetName())
		return
	}
	if gw.Spec.ExposeType == "" {
		return
	}
	klog.V(4).Infof("enqueue service %s/%s due to gateway %s delete event", tool.WorkingNamespace, tool.GatewayProxyInternalService, gw.GetName())
	AddGatewayProxyInternalService(q)
}

func (h *EnqueueRequestForGatewayEventInternal) Generic(ctx context.Context, e event.GenericEvent, q workqueue.RateLimitingInterface) {
	return
}

type EnqueueRequestForConfigEventInternal struct{}

func AddGatewayProxyInternalService(q workqueue.RateLimitingInterface) {
	q.Add(reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: tool.WorkingNamespace, Name: tool.GatewayProxyInternalService},
	})
}

func (h *EnqueueRequestForConfigEventInternal) Create(ctx context.Context, e event.CreateEvent, q workqueue.RateLimitingInterface) {
	cm, ok := e.Object.(*corev1.ConfigMap)
	if !ok {
		klog.Errorf("could not assert runtime Object %s/%s to v1.Configmap", e.Object.GetNamespace(), e.Object.GetName())
		return
	}
	if cm.Data == nil {
		return
	}
	_, _, err := net.SplitHostPort(cm.Data[tool.ProxyServerInsecurePortKey])
	if err == nil {
		klog.V(4).Infof("enqueue service %s/%s due to config %s/%s create event",
			tool.WorkingNamespace, tool.GatewayProxyInternalService, tool.WorkingNamespace, tool.RavenAgentConfig)
		AddGatewayProxyInternalService(q)
		return
	}
	_, _, err = net.SplitHostPort(cm.Data[tool.ProxyServerSecurePortKey])
	if err == nil {
		klog.V(4).Infof("enqueue service %s/%s due to config %s/%s create event",
			tool.WorkingNamespace, tool.GatewayProxyInternalService, tool.WorkingNamespace, tool.RavenAgentConfig)
		AddGatewayProxyInternalService(q)
		return
	}
}

func (h *EnqueueRequestForConfigEventInternal) Update(ctx context.Context, e event.UpdateEvent, q workqueue.RateLimitingInterface) {
	newCm, ok := e.ObjectNew.(*corev1.ConfigMap)
	if !ok {
		klog.Errorf("could not assert runtime Object %s/%s to v1.Configmap", e.ObjectNew.GetNamespace(), e.ObjectNew.GetName())
		return
	}
	oldCm, ok := e.ObjectOld.(*corev1.ConfigMap)
	if !ok {
		klog.Errorf("could not assert runtime Object %s/%s to v1.Configmap", e.ObjectOld.GetNamespace(), e.ObjectOld.GetName())
		return
	}
	_, newInsecurePort, newErr := net.SplitHostPort(newCm.Data[tool.ProxyServerInsecurePortKey])
	_, oldInsecurePort, oldErr := net.SplitHostPort(oldCm.Data[tool.ProxyServerInsecurePortKey])
	if newErr == nil && oldErr == nil {
		if newInsecurePort != oldInsecurePort {
			klog.V(4).Infof("enqueue service %s/%s due to config %s/%s update event",
				tool.WorkingNamespace, tool.GatewayProxyInternalService, tool.WorkingNamespace, tool.RavenAgentConfig)
			AddGatewayProxyInternalService(q)
			return
		}
	}
	_, newSecurePort, newErr := net.SplitHostPort(newCm.Data[tool.ProxyServerSecurePortKey])
	_, oldSecurePort, oldErr := net.SplitHostPort(oldCm.Data[tool.ProxyServerSecurePortKey])
	if newErr == nil && oldErr == nil {
		if newSecurePort != oldSecurePort {
			klog.V(4).Infof("enqueue service %s/%s due to config %s/%s update event",
				tool.WorkingNamespace, tool.GatewayProxyInternalService, tool.WorkingNamespace, tool.RavenAgentConfig)
			AddGatewayProxyInternalService(q)
			return
		}
	}
}

func (h *EnqueueRequestForConfigEventInternal) Delete(ctx context.Context, e event.DeleteEvent, q workqueue.RateLimitingInterface) {
	return
}

func (h *EnqueueRequestForConfigEventInternal) Generic(ctx context.Context, e event.GenericEvent, q workqueue.RateLimitingInterface) {
	return
}
