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

package controller

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	resticv1 "github.com/zhulik/restic-operator/api/v1"
	"github.com/zhulik/restic-operator/internal/restic"
	batchv1 "k8s.io/api/batch/v1"
)

// ForgetScheduleReconciler reconciles a ForgetSchedule object
type ForgetScheduleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Config   *rest.Config
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=restic.zhulik.wtf,resources=forgetschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=restic.zhulik.wtf,resources=forgetschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=restic.zhulik.wtf,resources=forgetschedules/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ForgetSchedule object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.1/pkg/reconcile
func (r *ForgetScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	forgetSchedule := &resticv1.ForgetSchedule{}
	err := r.Get(ctx, req.NamespacedName, forgetSchedule)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !forgetSchedule.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	repo := &resticv1.Repository{}
	err = r.Get(ctx, client.ObjectKey{Namespace: forgetSchedule.Namespace, Name: forgetSchedule.Spec.Repository}, repo)
	if err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("Repository not found, will retry", "repository", forgetSchedule.Spec.Repository)
		}
		return ctrl.Result{}, err
	}

	if forgetSchedule.OwnerReferences == nil {
		if err := ctrl.SetControllerReference(repo, forgetSchedule, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		err = r.Update(ctx, forgetSchedule)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	cronJob := &batchv1.CronJob{}
	err = r.Get(ctx, client.ObjectKey{Namespace: forgetSchedule.Namespace, Name: forgetSchedule.JobName()}, cronJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.createForgetJob(ctx, l, forgetSchedule, repo)
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, r.updateForgetJob(ctx, l, forgetSchedule, repo)
}

func (r *ForgetScheduleReconciler) createForgetJob(ctx context.Context, l logr.Logger, forgetSchedule *resticv1.ForgetSchedule, repo *resticv1.Repository) error {
	l.Info("Creating forget job")
	job, err := restic.CreateForgetJob(forgetSchedule, repo)
	if err != nil {
		return err
	}

	if err := ctrl.SetControllerReference(forgetSchedule, job, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, job); err != nil {
		return err
	}

	forgetSchedule.Status.Conditions = []metav1.Condition{
		{
			Type:               resticv1.ForgetScheduleCreated,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "ForgetScheduleCreated",
			Message:            "Forget schedule CronJob created",
		},
	}

	return r.Status().Update(ctx, forgetSchedule)
}

func (r *ForgetScheduleReconciler) updateForgetJob(ctx context.Context, l logr.Logger, forgetSchedule *resticv1.ForgetSchedule, repo *resticv1.Repository) error {
	l.Info("Updating forget job")
	job, err := restic.CreateForgetJob(forgetSchedule, repo)
	if err != nil {
		return err
	}

	if err := ctrl.SetControllerReference(forgetSchedule, job, r.Scheme); err != nil {
		return err
	}

	return r.Update(ctx, job)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ForgetScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resticv1.ForgetSchedule{}).
		Named("forgetschedule").
		Complete(r)
}
