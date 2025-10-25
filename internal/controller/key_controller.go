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

	"github.com/sethvargo/go-password/password"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	v1 "github.com/zhulik/restic-operator/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zhulik/restic-operator/internal/conditions"
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

	key := &v1.Key{}
	err := r.Get(ctx, req.NamespacedName, key)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !key.DeletionTimestamp.IsZero() && key.Status.ActiveJobName == nil {
		l.Info("Key is being deleted")

		err = r.deleteKey(ctx, l, key)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	repo := &v1.Repository{}
	err = r.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: key.Spec.Repository}, repo)
	if err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("Repository not found, will retry", "repository", key.Spec.Repository)
		}
		return ctrl.Result{}, err
	}

	if key.Status.ActiveJobName != nil {
		err = r.checkActiveJobStatus(ctx, l, repo, key)
		return ctrl.Result{}, err
	}

	if key.Status.KeyID != nil {
		return ctrl.Result{}, nil
	}

	if controllerutil.AddFinalizer(key, finalizer) {
		l.Info("add finalizer")

		err = r.Update(ctx, key)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	l.Info("Key ID is not set, creating a new key")

	err = ctrl.SetControllerReference(repo, key, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}

	key.Labels = map[string]string{
		"restic.zhulik.wtf/repository": key.Spec.Repository,
	}

	err = r.Update(ctx, key)
	if err != nil {
		return ctrl.Result{}, err
	}

	if _, ok := conditions.ContainsAnyTrueCondition(key.Status.Conditions, "Failed", "Created"); ok {
		return ctrl.Result{}, nil
	}

	err = r.createKey(ctx, l, repo, key)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.Status().Update(ctx, key)
	return ctrl.Result{}, err
}

func (r *KeyReconciler) createKey(ctx context.Context, l logr.Logger, repo *v1.Repository, key *v1.Key) error {
	err := r.createSecretIfNotExists(ctx, l, key)
	if err != nil {
		return err
	}

	job, err := restic.CreateAddKeyJob(ctx, r.Client, repo, key)
	if err != nil {
		return err
	}

	if err := ctrl.SetControllerReference(key, job, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, job); err != nil {
		return err
	}

	key.Status.Conditions = []metav1.Condition{
		{
			Type:               "Creating",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "KeyCreationStarted",
			Message:            "Key creation job created",
		},
	}

	key.Status.ActiveJobName = &job.Name
	err = r.Status().Update(ctx, key)
	if err != nil {
		return err
	}

	l.Info("Created key creation job", "job", job.Name)
	return nil
}

func (r *KeyReconciler) deleteKey(ctx context.Context, l logr.Logger, key *v1.Key) error {
	repo := &v1.Repository{}
	err := r.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: key.Spec.Repository}, repo)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		// If the repository is not found, we consider the key already deleted
		controllerutil.RemoveFinalizer(key, finalizer)
		return r.Update(ctx, key)
	}

	if !repo.DeletionTimestamp.IsZero() {
		// If the repository is being deleted, we consider the key already deleted
		controllerutil.RemoveFinalizer(key, finalizer)
		return r.Update(ctx, key)
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

	repo.Status.Keys--
	err = r.Status().Update(ctx, repo)
	if err != nil {
		return err
	}

	key.Status.Conditions = []metav1.Condition{
		{
			Type:               "Deleting",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "KeyDeletionStarted",
			Message:            "Key deletion job created",
		},
	}

	key.Status.ActiveJobName = &job.Name
	err = r.Status().Update(ctx, key)
	if err != nil {
		return err
	}

	l.Info("Created key deletion job", "job", job.Name)
	return nil
}

func (r *KeyReconciler) createSecretIfNotExists(ctx context.Context, l logr.Logger, key *v1.Key) error {
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
					"restic.zhulik.wtf/key":        key.Name,
					"restic.zhulik.wtf/repository": key.Spec.Repository,
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

func (r *KeyReconciler) checkActiveJobStatus(ctx context.Context, l logr.Logger, repo *v1.Repository, key *v1.Key) error {
	job := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: *key.Status.ActiveJobName}, job)
	if err != nil {
		return err
	}

	if conditionType, ok := conditions.ContainsAnyTrueCondition(key.Status.Conditions, "Creating", "Deleting"); ok {
		switch conditionType {
		case "Creating":
			return r.checkCreateJobStatus(ctx, l, repo, key)
		case "Deleting":
			return r.checkDeleteJobStatus(ctx, l, key)
		}
	}

	return nil
}

func (r *KeyReconciler) checkCreateJobStatus(ctx context.Context, l logr.Logger, repo *v1.Repository, key *v1.Key) error {
	job := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: *key.Status.ActiveJobName}, job)
	if err != nil {
		return err
	}

	if conditionType, ok := conditions.JobHasAnyTrueCondition(job, batchv1.JobComplete, batchv1.JobFailed); ok {
		logs, err := getJobPodLogs(ctx, r.Client, r.Config, l, job)
		if err != nil {
			return err
		}

		switch conditionType {
		case batchv1.JobFailed:
			key.Status.Conditions = []metav1.Condition{
				{
					Type:               "Failed",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "KeyCreationJobFailed",
					Message:            logs,
				},
			}

			key.Status.ActiveJobName = nil
		case batchv1.JobComplete:
			key.Status.Conditions = []metav1.Condition{
				{
					Type:               "Created",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "KeyCreationJobCompleted",
					Message:            "Key creation job successfully completed",
				},
			}

			key.Status.ActiveJobName = nil
			key.Status.KeyID = &keyIDRegex.FindStringSubmatch(logs)[1]
			l.Info("Key creation job completed", "keyID", *key.Status.KeyID)

			// First key was added, so we can mark the repository as secure
			firstKey := job.Annotations["restic.zhulik.wtf/first-key"] == "true"
			if firstKey {
				repo.Status.Conditions, _ = conditions.UpdateCondition(repo.Status.Conditions, "Secure", metav1.Condition{
					Type:               "Secure",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "RepositoryHasAtLeastOneKey",
					Message:            "Repository has at least one secure key",
				})
			} else {
				// Other key was added, so we increment the number of keys
				repo.Status.Keys++
			}
		}
		repo.Status.Conditions, _ = conditions.UpdateCondition(repo.Status.Conditions, "Locked", metav1.Condition{
			Type:               "Locked",
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "RepositoryIsNotLocked",
			Message:            "Repository is not locked, this has nothing to with restic repository locking, it's used for restic-operator internal concurrency control",
		})
		err = r.Status().Update(ctx, key)
		if err != nil {
			return err
		}
		return r.Status().Update(ctx, repo)
	}

	return nil
}

func (r *KeyReconciler) checkDeleteJobStatus(ctx context.Context, l logr.Logger, key *v1.Key) error {
	job := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: *key.Status.ActiveJobName}, job)
	if err != nil {
		return err
	}

	if conditionType, ok := conditions.JobHasAnyTrueCondition(job, batchv1.JobComplete, batchv1.JobFailed); ok {
		switch conditionType {
		case batchv1.JobFailed:
			// TODO: signal error through repository status
			logs, err := getJobPodLogs(ctx, r.Client, r.Config, l, job)
			if err != nil {
				return err
			}

			l.Error(err, "Key deletion job failed", "logs", logs, "key", key.Name)

			return nil
		case batchv1.JobComplete:
			l.Info("Key deletion successfully completed", "key", key.Name)
			controllerutil.RemoveFinalizer(key, finalizer)
			if err := r.Update(ctx, key); err != nil {
				return err
			}

			return nil
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Key{}).
		Owns(&batchv1.Job{}).
		Named("key").
		Complete(r)
}
