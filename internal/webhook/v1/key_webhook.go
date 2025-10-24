/*
Copyright 2025.

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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	resticv1 "github.com/zhulik/restic-operator/api/v1"
)

// nolint:unused
// log is for logging in this package.
var keylog = logf.Log.WithName("key-resource")

// SetupKeyWebhookWithManager registers the webhook for Key in the manager.
func SetupKeyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&resticv1.Key{}).
		WithValidator(&KeyCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-restic-zhulik-wtf-v1-key,mutating=false,failurePolicy=fail,sideEffects=None,groups=restic.zhulik.wtf,resources=keys,verbs=create;update,versions=v1,name=vkey-v1.kb.io,admissionReviewVersions=v1

// KeyCustomValidator struct is responsible for validating the Key resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type KeyCustomValidator struct {
}

var _ webhook.CustomValidator = &KeyCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Key.
func (v *KeyCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Key.
func (v *KeyCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	old := oldObj.(*resticv1.Key)
	new := newObj.(*resticv1.Key)

	if old.Spec.Host != new.Spec.Host ||
		old.Spec.User != new.Spec.User ||
		old.Spec.Repository != new.Spec.Repository {
		return nil, fmt.Errorf("key spec is immutable")
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Key.
func (v *KeyCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
