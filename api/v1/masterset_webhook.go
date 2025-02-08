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

package v1

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *MasterSet) SetupWebhookWithManager(mgr ctrl.Manager) error {
	validator, err := NewUGValidator()
	if err != nil {
		return err
	}
	return ctrl.NewWebhookManagedBy(mgr).For(r).WithValidator(validator).Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-knode-alibabacloud-com-v1-machine,mutating=true,failurePolicy=fail,sideEffects=None,groups=knode.alibabacloud.com,resources=machines,verbs=create;update,versions=v1,name=mmachine.kb.io,admissionReviewVersions=v1

var _ admission.CustomValidator = &UserGroupValidator{}

func NewUGValidator() (*UserGroupValidator, error) {
	return &UserGroupValidator{}, nil
}

type UserGroupValidator struct {
}

func Validate(ctx context.Context, o runtime.Object, verb string) (admission.Warnings, error) {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "no request context")
	}
	klog.Infof("validate: "+
		"verb=%s, "+
		"UID=%s, "+
		"Groups=%s, "+
		"Username=%s, "+
		"UserID=%s", verb, req.UID, req.UserInfo.Groups, req.UserInfo.Username, req.UserInfo.UID,
	)
	// webhook validate:
	// From user kubeconfig
	// {
	//      "verb": "update",
	//      "UID": "d4038044-527f-4682-a98a-f145cfbdf607",
	//      "Groups": ["system:users", "system:authenticated"],
	//      "Username": "11707238x09x921x-1686895511",
	//      "UserID": ""
	// }

	// From troopers
	// "user": {
	// 	"groups": [
	//              "system:masters",
	//              "system:authenticated"
	//      ],
	//      "username": "kubernetes-admin"
	// },
	// if Allowed(req.UserInfo.Groups, []string{req.UserInfo.Username}) {
	// 	return nil, nil
	// }
	klog.Infof("validate: %s %s CR is not allowed", verb, reflect.TypeOf(o))
	return nil, errors.New(fmt.Sprintf("%s not allowed", verb))
}

func (u *UserGroupValidator) ValidateCreate(
	ctx context.Context,
	o runtime.Object,
) (admission.Warnings, error) {
	return Validate(ctx, o, "create")
}

func (u *UserGroupValidator) ValidateUpdate(
	ctx context.Context,
	o, newObj runtime.Object,
) (admission.Warnings, error) {
	return Validate(ctx, o, "update")
}

func (u *UserGroupValidator) ValidateDelete(
	ctx context.Context,
	o runtime.Object,
) (admission.Warnings, error) {
	return Validate(ctx, o, "delete")
}
