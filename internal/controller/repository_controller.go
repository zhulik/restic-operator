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
	"fmt"

	"github.com/sethvargo/go-password/password"
	"github.com/zhulik/restic-operator/internal/conditions"
	"github.com/zhulik/restic-operator/internal/labels"
	"github.com/zhulik/restic-operator/internal/restic"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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

	if repo.SetDefaultConditions() {
		err := r.Status().Update(ctx, repo)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if repo.IsCreated() || repo.IsFailed() {
		return ctrl.Result{}, nil
	}

	job, err := r.getCreateRepoJob(ctx, repo)
	if err != nil {
		return ctrl.Result{}, err
	}
	if job == nil {
		// Job was not found, repository does not exist yet
		return ctrl.Result{}, r.startCreateRepoJob(ctx, l, repo)
	}

	return ctrl.Result{}, r.checkCreateJobStatus(ctx, l, repo, job)
}

func (r *RepositoryReconciler) getCreateRepoJob(ctx context.Context, repo *resticv1.Repository) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{Namespace: repo.Namespace, Name: repo.Name}, job)
	if err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return job, nil
}

func (r *RepositoryReconciler) checkCreateJobStatus(ctx context.Context, l logr.Logger, repo *resticv1.Repository, job *batchv1.Job) error {
	conditionType, inCondition := conditions.JobHasAnyTrueCondition(job, batchv1.JobComplete, batchv1.JobFailed)
	if !inCondition {
		return nil
	}
	switch conditionType {
	case batchv1.JobFailed:
		l.Info("Create job failed, updating repository status")
		logs, err := getJobPodLogs(ctx, r.Client, r.Config, l, job)
		if err != nil {
			return err
		}

		repo.SetFailedCondition(logs)

		r.Recorder.Eventf(repo,
			"Warning", "RepositoryInitializationJobFailed",
			"Repository initialization job %s failed: %s", job.Name, logs,
		)

	case batchv1.JobComplete:
		l.Info("Create job successfully completed, updating repository status")
		repo.SetCreatedCondition()

		r.Recorder.Eventf(repo,
			"Normal", "RepositoryInitializationJobCompleted",
			"Repository initialization job %s successfully completed", job.Name,
		)
	}

	return r.Status().Update(ctx, repo)
}

func (r *RepositoryReconciler) startCreateRepoJob(ctx context.Context, l logr.Logger, repo *resticv1.Repository) error {
	err := r.createOperatorSecretIfNotExists(ctx, l, repo)
	if err != nil {
		return err
	}

	job := restic.CreateRepoInitJob(repo)
	if err := ctrl.SetControllerReference(repo, job, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, job); err != nil {
		return client.IgnoreAlreadyExists(err)
	}

	r.Recorder.Eventf(repo,
		"Normal", "RepositoryInitializationJobStarted",
		"Repository initialization job %s started", job.Name,
	)

	l.Info("Repository initialization job started", "job", job.Name)
	return nil
}

func (r *RepositoryReconciler) createOperatorSecretIfNotExists(ctx context.Context, l logr.Logger, repo *resticv1.Repository) error {
	secret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Namespace: repo.Namespace, Name: repo.OperatorSecretName()}, secret)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		res, err := password.Generate(64, 10, 10, false, true)
		if err != nil {
			return err
		}

		l.Info("Operator key secret not found, creating", "secret", repo.OperatorSecretName())
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      repo.OperatorSecretName(),
				Namespace: repo.Namespace,
				Labels: map[string]string{
					labels.KeyType:    labels.KeyTypeOperator,
					labels.Repository: repo.Name,
				},
			},
			Type: keySecretType,
			StringData: map[string]string{
				"key": res,
			},
		}

		if err := ctrl.SetControllerReference(repo, secret, r.Scheme); err != nil {
			return err
		}

		return r.Create(ctx, secret)
	}

	if len(secret.OwnerReferences) == 0 {
		return fmt.Errorf("secret %s already exists but has no owner references", repo.OperatorSecretName())
	}

	if len(secret.OwnerReferences) > 1 {
		return fmt.Errorf("secret %s already exists but has multiple owner references", repo.OperatorSecretName())
	}

	if secret.OwnerReferences[0].UID != repo.UID {
		return fmt.Errorf("secret %s is owned by another resource: %s/%s", repo.OperatorSecretName(), secret.OwnerReferences[0].Kind, secret.OwnerReferences[0].Name)
	}

	l.Info("Operator key secret already exists", "secret", repo.OperatorSecretName())
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
