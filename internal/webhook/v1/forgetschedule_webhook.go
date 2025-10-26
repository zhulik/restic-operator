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

	"github.com/hashicorp/cronexpr"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	resticv1 "github.com/zhulik/restic-operator/api/v1"
)

// nolint:unused
// log is for logging in this package.
var forgetSchedulelog = logf.Log.WithName("forgetschedule-resource")

// SetupForgetScheduleWebhookWithManager registers the webhook for ForgetSchedule in the manager.
func SetupForgetScheduleWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&resticv1.ForgetSchedule{}).
		WithValidator(&ForgetScheduleCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-restic-zhulik-wtf-v1-forgetschedule,mutating=false,failurePolicy=fail,sideEffects=None,groups=restic.zhulik.wtf,resources=forgetschedules,verbs=create;update,versions=v1,name=vforgetschedule-v1.kb.io,admissionReviewVersions=v1

// ForgetScheduleCustomValidator struct is responsible for validating the ForgetSchedule resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ForgetScheduleCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &ForgetScheduleCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ForgetSchedule.
func (v *ForgetScheduleCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	forgetSchedule, ok := obj.(*resticv1.ForgetSchedule)
	if !ok {
		return nil, fmt.Errorf("expected a ForgetSchedule object but got %T", obj)
	}
	forgetSchedulelog.Info("Validation for ForgetSchedule upon creation", "name", forgetSchedule.GetName())

	return nil, v.validateForgetScheduleSpec(forgetSchedule.Spec)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ForgetSchedule.
func (v *ForgetScheduleCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	forgetSchedule, ok := newObj.(*resticv1.ForgetSchedule)
	if !ok {
		return nil, fmt.Errorf("expected a ForgetSchedule object for the newObj but got %T", newObj)
	}
	forgetSchedulelog.Info("Validation for ForgetSchedule upon update", "name", forgetSchedule.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, v.validateForgetScheduleSpec(forgetSchedule.Spec)
}

func (v *ForgetScheduleCustomValidator) validateForgetScheduleSpec(spec resticv1.ForgetScheduleSpec) error {
	if _, err := cronexpr.Parse(spec.Schedule); err != nil {
		return fmt.Errorf("schedule is invalid: %w", err)
	}
	if spec.KeepLast+spec.KeepHourly+spec.KeepDaily+
		spec.KeepWeekly+spec.KeepMonthly+spec.KeepYearly == 0 {
		return fmt.Errorf("at least one of keepLast, keepHourly, keepDaily, keepWeekly, keepMonthly, keepYearly must be set")
	}
	return nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ForgetSchedule.
func (v *ForgetScheduleCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	forgetSchedule, ok := obj.(*resticv1.ForgetSchedule)
	if !ok {
		return nil, fmt.Errorf("expected a ForgetSchedule object but got %T", obj)
	}
	forgetSchedulelog.Info("Validation for ForgetSchedule upon deletion", "name", forgetSchedule.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
