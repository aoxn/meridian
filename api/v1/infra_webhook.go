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

package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var infralog = logf.Log.WithName("infra-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *Infra) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-meridian-meridian-io-v1-infra,mutating=true,failurePolicy=fail,sideEffects=None,groups=meridian.meridian.io,resources=infras,verbs=create;update,versions=v1,name=minfra.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Infra{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Infra) Default() {
	infralog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-meridian-meridian-io-v1-infra,mutating=false,failurePolicy=fail,sideEffects=None,groups=meridian.meridian.io,resources=infras,verbs=create;update,versions=v1,name=vinfra.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Infra{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Infra) ValidateCreate() (admission.Warnings, error) {
	infralog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Infra) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	infralog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Infra) ValidateDelete() (admission.Warnings, error) {
	infralog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
