/*
Copyright 2023 aoxn.

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
	"crypto/x509"
	"encoding/pem"
	"github.com/pkg/errors"
	certv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AddApproveController(
	mgr manager.Manager,
) error {
	set, err := clientset.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrapf(err, "build certificate client")
	}
	r := &approverReconciler{
		clientset: set,
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		record:    mgr.GetEventRecorderFor("ApproveController"),
	}
	// Create a new controller
	c, err := controller.New(
		"approve-controller", mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: 1,
		},
	)
	if err != nil {
		return err
	}
	return c.Watch(
		source.Kind(mgr.GetCache(), &certv1.CertificateSigningRequest{}),
		&handler.EnqueueRequestForObject{},
	)
}

// blank assignment to verify that ReconcileAutoRepair implements reconcile.Reconciler
var _ reconcile.Reconciler = &approverReconciler{}

// approverReconciler reconciles a Machine object
type approverReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	clientset clientset.Interface
	record    record.EventRecorder
}

// +kubebuilder:rbac:groups=knode.alibabacloud.com,resources=machines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=knode.alibabacloud.com,resources=machines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=knode.alibabacloud.com,resources=machines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Machine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *approverReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var m certv1.CertificateSigningRequest
	if !isTarget(req.Name) {
		return ctrl.Result{}, nil
	}
	if err := r.Get(ctx, req.NamespacedName, &m); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if done(&m) {
		return ctrl.Result{}, nil
	}
	klog.Infof("[%s]start to process csr ", req.Name)
	x509cr, err := parseCSR(m.Spec.Request)
	if err != nil {
		klog.Errorf("[%s]parse csr error: %v", req.Name, err)
		return ctrl.Result{}, nil
	}

	if !validate(&m, x509cr) {
		return ctrl.Result{}, nil
	}

	// do approve csr
	approve(&m)
	klog.Infof("[%s]csr approved", req.Name)
	if _, err := r.clientset.CertificatesV1().
		CertificateSigningRequests().
		UpdateApproval(ctx, req.Name, &m, metav1.UpdateOptions{}); err != nil {
		r.record.Eventf(&m, corev1.EventTypeWarning, "ApproveFailed", "csr approve for %s: %s", m.Name, err.Error())
		// if err := r.Status().Update(ctx, &m); err != nil {
		klog.Infof("[%s]approve csr failed, %s", m.Name, err.Error())
		return ctrl.Result{RequeueAfter: 4 * time.Second}, nil
	}
	klog.Infof("[%s]reconcile finished", req.Name)
	r.record.Eventf(&m, corev1.EventTypeNormal, "Approved", "csr approved for %s", m.Name)
	return ctrl.Result{}, nil
}

func deny(csr *certv1.CertificateSigningRequest) {
	condition := certv1.CertificateSigningRequestCondition{
		Type:    certv1.CertificateDenied,
		Status:  corev1.ConditionTrue,
		Reason:  "CertDeny By xdpin operator",
		Message: "certificate approve by [approve controller]",
	}
	csr.Status.Conditions = append(csr.Status.Conditions, condition)
}

func approve(csr *certv1.CertificateSigningRequest) {
	condition := certv1.CertificateSigningRequestCondition{
		Type:    certv1.CertificateApproved,
		Status:  corev1.ConditionTrue,
		Reason:  "AutoApproved",
		Message: "certificate approve by [approve operator]",
	}
	csr.Status.Conditions = append(csr.Status.Conditions, condition)
}

func done(csr *certv1.CertificateSigningRequest) bool {
	if csr.Spec.SignerName != certv1.KubeAPIServerClientSignerName {
		return false
	}

	if len(csr.Status.Certificate) > 0 {
		return true
	}
	for _, c := range csr.Status.Conditions {
		switch c.Type {
		case certv1.CertificateApproved, certv1.CertificateDenied:
			return true
		}
	}
	return false
}

func parseCSR(pemBytes []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, errors.New("PEM block type must be CERTIFICATE REQUEST")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, err
	}
	return csr, nil
}

var requiredUsage = sets.NewString(
	string(certv1.UsageDigitalSignature),
	string(certv1.UsageKeyEncipherment),
	string(certv1.UsageClientAuth),
)

func validate(csr *certv1.CertificateSigningRequest, req *x509.CertificateRequest) bool {
	if !reflect.DeepEqual([]string{"system:meridian"}, req.Subject.Organization) {
		return false
	}

	if len(req.DNSNames) > 0 {
		klog.Info("dns not allowed")
		return false
	}
	if len(req.EmailAddresses) > 0 {
		klog.Info("email addresses not allowed")
		return false
	}
	if len(req.IPAddresses) > 0 {
		klog.Info("ip addresses not allowed")
		return false
	}
	if len(req.URIs) > 0 {
		klog.Info("URIs not allowed")
		return false
	}

	if !strings.HasPrefix(req.Subject.CommonName, "system:meridian:") {
		klog.Info("CN must be system:meridian:xxx", "current", req.Subject.CommonName)
		return false
	}

	if !requiredUsage.Equal(toset(csr.Spec.Usages)) {
		klog.Info("usage not equal. required ", "usage", requiredUsage)
		return false
	}

	return true

}

func isTarget(name string) bool {
	return strings.HasPrefix(name, "meridian-csr")
}

func toset(usages []certv1.KeyUsage) sets.String {
	result := sets.NewString()
	for _, usage := range usages {
		result.Insert(string(usage))
	}
	return result
}
