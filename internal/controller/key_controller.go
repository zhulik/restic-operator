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
	"regexp"
	"time"

	"github.com/samber/lo"
	"github.com/sethvargo/go-password/password"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	resticv1 "github.com/zhulik/restic-operator/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zhulik/restic-operator/internal/conditions"
	"github.com/zhulik/restic-operator/internal/labels"
	"github.com/zhulik/restic-operator/internal/restic"
)

var keyIDRegex = regexp.MustCompile(`saved new key with ID (\w+)`)

const (
	finalizer                       = "restic.zhulik.wtf/finalizer"
	keySecretType corev1.SecretType = "restic.zhulik.wtf/key"
)

// KeyReconciler reconciles a Key object
type KeyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Config   *rest.Config
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=restic.zhulik.wtf,resources=keys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=restic.zhulik.wtf,resources=keys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=restic.zhulik.wtf,resources=keys/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;create;watch

func (r *KeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	key := &resticv1.Key{}
	err := r.Get(ctx, req.NamespacedName, key)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	repo, err := r.getRepository(ctx, key)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if !key.DeletionTimestamp.IsZero() {
				controllerutil.RemoveFinalizer(key, finalizer)
				return ctrl.Result{}, r.Update(ctx, key)
			}
			return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	if key.SetDefaultConditions() {
		err := r.Status().Update(ctx, key)
		if err != nil {
			return ctrl.Result{}, err
		}

		err = ctrl.SetControllerReference(repo, key, r.Scheme)
		if err != nil {
			return ctrl.Result{}, err
		}

		key.Labels = map[string]string{
			labels.Repository: key.Spec.Repository,
		}

		controllerutil.AddFinalizer(key, finalizer)
		err = r.Update(ctx, key)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	keyJobs, err := r.getKeyJobs(ctx, key)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !key.DeletionTimestamp.IsZero() {
		err = r.deleteKeyOrCheckDeletionStatus(ctx, l, repo, key, keyJobs)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if key.IsCreated() || key.IsFailed() {
		return ctrl.Result{}, nil
	}

	if len(keyJobs) == 0 {
		if !repo.IsCreated() {
			l.Info("Repository is not yet created, will retry", "repository", repo.Name)
			return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
		}

		err = r.createKey(ctx, l, repo, key)
		return ctrl.Result{}, err
	}

	finishedJobs := lo.Filter(keyJobs, func(job batchv1.Job, _ int) bool {
		_, inCondition := conditions.JobHasAnyTrueCondition(&job, batchv1.JobComplete)
		return inCondition
	})

	if len(finishedJobs) > 0 {
		err = r.checkActiveJobStatus(ctx, l, repo, key, keyJobs)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KeyReconciler) createKey(ctx context.Context, l logr.Logger, repo *resticv1.Repository, key *resticv1.Key) error {
	err := r.createSecretIfNotExists(ctx, l, key)
	if err != nil {
		return err
	}

	job, err := restic.CreateAddKeyJob(repo, key)
	if err != nil {
		return err
	}

	if err := ctrl.SetControllerReference(key, job, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, job); err != nil {
		return err
	}

	l.Info("Created key creation job", "job", job.Name)
	return nil
}

func (r *KeyReconciler) deleteKeyOrCheckDeletionStatus(ctx context.Context, l logr.Logger, repo *resticv1.Repository, key *resticv1.Key, jobs []batchv1.Job) error {
	if !repo.DeletionTimestamp.IsZero() {
		// If the repository is being deleted, we consider the key already deleted
		controllerutil.RemoveFinalizer(key, finalizer)
		return r.Update(ctx, key)
	}

	for _, job := range jobs {
		if job.Labels[labels.KeyOperation] == labels.KeyOperationDelete {
			l.Info("Key deletion job already exists", "job", job.Name)
			return r.checkDeleteJobStatus(ctx, l, repo, key, &job)
		}
	}

	job, err := restic.CreateDeleteKeyJob(ctx, r.Client, repo, key)
	if err != nil {
		return err
	}

	if err := ctrl.SetControllerReference(key, job, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, job); err != nil {
		return err
	}

	// TODO: when delete key job is done, we need to update the repository conditions: unlock it and update the number of keys

	l.Info("Created key deletion job", "job", job.Name)
	return nil
}

func (r *KeyReconciler) createSecretIfNotExists(ctx context.Context, l logr.Logger, key *resticv1.Key) error {
	secret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: key.SecretName()}, secret)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		res, err := password.Generate(64, 10, 10, false, true)
		if err != nil {
			return err
		}

		l.Info("Key secret not found, creating", "secret", key.SecretName())
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.SecretName(),
				Namespace: key.Namespace,
				Labels: map[string]string{
					labels.Key:        key.Name,
					labels.Repository: key.Spec.Repository,
				},
			},
			Type: keySecretType,
			StringData: map[string]string{
				"key": res,
			},
		}

		if err := ctrl.SetControllerReference(key, secret, r.Scheme); err != nil {
			return err
		}

		return r.Create(ctx, secret)
	}

	if len(secret.OwnerReferences) == 0 {
		return fmt.Errorf("secret %s already exists but has no owner references", key.SecretName())
	}

	if len(secret.OwnerReferences) > 1 {
		return fmt.Errorf("secret %s already exists but has multiple owner references", key.SecretName())
	}

	if secret.OwnerReferences[0].UID != key.UID {
		return fmt.Errorf("secret %s is owned by another resource: %s/%s", key.SecretName(), secret.OwnerReferences[0].Kind, secret.OwnerReferences[0].Name)
	}

	l.Info("Key secret already exists", "secret", key.SecretName())
	return nil
}

func (r *KeyReconciler) checkActiveJobStatus(ctx context.Context, l logr.Logger, repo *resticv1.Repository, key *resticv1.Key, jobs []batchv1.Job) error {
	for _, job := range jobs {
		if job.Labels[labels.KeyOperation] == labels.KeyOperationAdd {
			return r.checkCreateJobStatus(ctx, l, repo, key, &job)
		}
	}

	return nil
}

func (r *KeyReconciler) checkCreateJobStatus(ctx context.Context, l logr.Logger, repo *resticv1.Repository, key *resticv1.Key, job *batchv1.Job) error {
	conditionType, inCondition := conditions.JobHasAnyTrueCondition(job, batchv1.JobComplete, batchv1.JobFailed)
	if !inCondition {
		return nil
	}
	logs, err := getJobPodLogs(ctx, r.Client, r.Config, l, job)
	if err != nil {
		return err
	}

	switch conditionType {
	case batchv1.JobFailed:
		key.SetFailedCondition(logs)
		return r.Status().Update(ctx, key)
	case batchv1.JobComplete:
		key.SetCreatedCondition(keyIDRegex.FindStringSubmatch(logs)[1])
		l.Info("Key creation job completed", "keyID", *key.Status.KeyID)
		err := r.Status().Update(ctx, key)
		if err != nil {
			return err
		}

		r.Recorder.Eventf(repo,
			"Normal", "KeyAdded",
			"Key added, id: %s", *key.Status.KeyID,
		)
	}

	return nil
}

func (r *KeyReconciler) checkDeleteJobStatus(ctx context.Context, l logr.Logger, repo *resticv1.Repository, key *resticv1.Key, job *batchv1.Job) error {
	conditionType, inCondition := conditions.JobHasAnyTrueCondition(job, batchv1.JobComplete, batchv1.JobFailed)
	if !inCondition {
		return nil
	}

	switch conditionType {
	case batchv1.JobFailed:
		// TODO: signal error through repository status
		logs, err := getJobPodLogs(ctx, r.Client, r.Config, l, job)
		if err != nil {
			return err
		}

		l.Error(err, "Key deletion job failed", "logs", logs, "key", key.Name)

		r.Recorder.Eventf(repo,
			"Warning", "KeyDeletionFailed",
			"Key deletion failed, id: %s", *key.Status.KeyID,
		)

		return nil
	case batchv1.JobComplete:
		l.Info("Key deletion successfully completed", "key", key.Name)
		controllerutil.RemoveFinalizer(key, finalizer)
		if err := r.Update(ctx, key); err != nil {
			return err
		}
		r.Recorder.Eventf(repo,
			"Normal", "KeyDeleted",
			"Key deleted, id: %s", *key.Status.KeyID,
		)

		return nil
	}

	return nil
}

func (r *KeyReconciler) getRepository(ctx context.Context, key *resticv1.Key) (*resticv1.Repository, error) {
	repo := &resticv1.Repository{}
	err := r.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: key.Spec.Repository}, repo)
	if err != nil {
		return nil, err
	}
	return repo, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resticv1.Key{}).
		Owns(&batchv1.Job{}).
		Named("key").
		Complete(r)
}

func (r *KeyReconciler) getKeyJobs(ctx context.Context, key *resticv1.Key) ([]batchv1.Job, error) {
	jobs := &batchv1.JobList{}
	err := r.List(ctx, jobs, client.InNamespace(key.Namespace), client.MatchingLabels{
		labels.Key: key.Name,
	})
	if err != nil {
		return nil, err
	}

	return jobs.Items, nil
}
