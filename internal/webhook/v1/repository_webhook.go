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
var repositorylog = logf.Log.WithName("repository-resource")

// SetupRepositoryWebhookWithManager registers the webhook for Repository in the manager.
func SetupRepositoryWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&resticv1.Repository{}).
		WithValidator(&RepositoryCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-restic-zhulik-wtf-v1-repository,mutating=false,failurePolicy=fail,sideEffects=None,groups=restic.zhulik.wtf,resources=repositories,verbs=create;update,versions=v1,name=vrepository-v1.kb.io,admissionReviewVersions=v1

// RepositoryCustomValidator struct is responsible for validating the Repository resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type RepositoryCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &RepositoryCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Repository.
func (v *RepositoryCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	repository, ok := obj.(*resticv1.Repository)
	if !ok {
		return nil, fmt.Errorf("expected a Repository object but got %T", obj)
	}
	repositorylog.Info("Validation for Repository upon creation", "name", repository.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Repository.
func (v *RepositoryCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	repository, ok := newObj.(*resticv1.Repository)
	if !ok {
		return nil, fmt.Errorf("expected a Repository object for the newObj but got %T", newObj)
	}
	repositorylog.Info("Validation for Repository upon update", "name", repository.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Repository.
func (v *RepositoryCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	repository, ok := obj.(*resticv1.Repository)
	if !ok {
		return nil, fmt.Errorf("expected a Repository object but got %T", obj)
	}
	repositorylog.Info("Validation for Repository upon deletion", "name", repository.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
