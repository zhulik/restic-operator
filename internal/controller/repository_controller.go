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

	"github.com/zhulik/restic-operator/internal/restic"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	resticv1 "github.com/zhulik/restic-operator/api/v1"
)

// RepositoryReconciler reconciles a Repository object
type RepositoryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Config   *rest.Config
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=restic.zhulik.wtf,resources=repositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=restic.zhulik.wtf,resources=repositories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=restic.zhulik.wtf,resources=repositories/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;create;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods/log,verbs=get

func (r *RepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	repo := &resticv1.Repository{}
	err := r.Get(ctx, req.NamespacedName, repo)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if repo.Status.CreateJobName != nil {
		err = r.checkCreateJobStatus(ctx, l, repo)
		return ctrl.Result{}, err
	}

	if _, ok := containsAnyTrueCondition(repo.Status.Conditions, "Created", "Failed"); ok {
		return ctrl.Result{}, nil
	}

	err = r.startCreateRepoJob(ctx, l, repo)
	return ctrl.Result{}, err
}

func (r *RepositoryReconciler) checkCreateJobStatus(ctx context.Context, l logr.Logger, repo *resticv1.Repository) error {
	job := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{Namespace: repo.Namespace, Name: *repo.Status.CreateJobName}, job)
	if err != nil {
		return err
	}

	if conditionType, ok := jobHasAnyTrueCondition(job, batchv1.JobComplete, batchv1.JobFailed); ok {
		switch conditionType {
		case batchv1.JobFailed:
			l.Info("Create job failed, updating repository status")
			logs, err := getJobPodLogs(ctx, r.Client, r.Config, l, job)
			if err != nil {
				return err
			}

			repo.Status.Conditions = []metav1.Condition{
				{
					Type:               "Failed",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "RepositoryInitializationJobFailed",
					Message:            logs,
				},
			}

		case batchv1.JobComplete:
			l.Info("Create job successfully completed, updating repository status")
			repo.Status.Conditions = []metav1.Condition{
				{
					Type:               "Created",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "RepositoryInitializationJobCompleted",
					Message:            "Repository initialization job successfully completed",
				},
				{
					Type:               "Secure",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: metav1.Now(),
					Reason:             "RepositoryHasNoKeys",
					Message:            "Repository initialized without keys. A key needs to be added to the repository to make it secure.",
				},
			}
			repo.Status.Keys = 1 // When created with --insecure-no-password, a key is automatically created
		}

		repo.Status.CreateJobName = nil

		return r.Status().Update(ctx, repo)
	}

	return nil
}

func (r *RepositoryReconciler) startCreateRepoJob(ctx context.Context, l logr.Logger, repo *resticv1.Repository) error {
	job := restic.CreateRepoInitJob(repo)
	if err := ctrl.SetControllerReference(repo, job, r.Scheme); err != nil {
		return err
	}

	repo.Status.Conditions = []metav1.Condition{
		{
			Type:               "Creating",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "RepositoryInitializationStarted",
			Message:            "Repository initialization job created",
		},
	}
	repo.Status.CreateJobName = &job.Name

	err := r.Status().Update(ctx, repo)
	if err != nil {
		return err
	}

	if err := r.Create(ctx, job); err != nil {
		return err
	}

	l.Info("Repository initialization job created", "job", job.Name)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resticv1.Repository{}).
		Owns(&batchv1.Job{}).
		Named("repository").
		Complete(r)
}
