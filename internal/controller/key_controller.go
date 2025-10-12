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
	"regexp"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	resticv1 "github.com/zhulik/restic-operator/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zhulik/restic-operator/internal/restic"
)

var keyIDRegex = regexp.MustCompile(`saved new key with ID (\w+)`)

// KeyReconciler reconciles a Key object
type KeyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

// +kubebuilder:rbac:groups=restic.zhulik.wtf,resources=keys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=restic.zhulik.wtf,resources=keys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=restic.zhulik.wtf,resources=keys/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Key object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.1/pkg/reconcile
func (r *KeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx).WithValues("resource", req)

	key := &resticv1.Key{}
	err := r.Get(ctx, req.NamespacedName, key)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// TODO: set finalizer

	if key.Status.KeyID == nil {
		l.Info("Key ID is not set")

		repo := &resticv1.Repository{}
		err = r.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: key.Spec.Repository}, repo)
		if err != nil {
			if apierrors.IsNotFound(err) {
				l.Info("Repository not found, will retry", "repository", key.Spec.Repository)
			}
			return ctrl.Result{}, err
		}

		for _, condition := range key.Status.Conditions {
			if condition.Type == "Creating" {
				l.Info("Key is in creating state, checking job status", "job", *key.Status.CreateJobName)
				completed, err := r.checkCreateJobStatus(ctx, l, repo, key)
				if err != nil {
					return ctrl.Result{}, err
				}

				if !completed {
					return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
				}

				return ctrl.Result{}, nil
			}

			if condition.Type == "Failed" || condition.Type == "Created" {
				return ctrl.Result{}, nil
			}
		}

		err = r.createKey(ctx, l, repo, key)
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.Status().Update(ctx, key)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	l.Info("Key ID is set, job is done")

	return ctrl.Result{}, nil
}

func (r *KeyReconciler) createKey(ctx context.Context, l logr.Logger, repo *resticv1.Repository, key *resticv1.Key) error {
	err := r.createSecretIfNotExists(ctx, l, key)
	if err != nil {
		return err
	}

	job := restic.CreateAddKeyJob(repo, key)
	if err := ctrl.SetControllerReference(key, job, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, job); err != nil {
		return err
	}

	repo.Status.Keys++
	err = r.Status().Update(ctx, repo)
	if err != nil {
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

	key.Status.CreateJobName = &job.Name
	err = r.Status().Update(ctx, key)
	if err != nil {
		return err
	}

	l.Info("Created key job", "job", job.Name)
	return nil
}

func (r *KeyReconciler) createSecretIfNotExists(ctx context.Context, l logr.Logger, key *resticv1.Key) error {
	secret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: key.SecretName()}, secret)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		l.Info("Key secret not found, creating", "secret", key.SecretName())
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.SecretName(),
				Namespace: key.Namespace,
			},
			StringData: map[string]string{
				"key": "SomePasssowrd", // TODO: Generate a random password
			},
		}

		if err := ctrl.SetControllerReference(key, secret, r.Scheme); err != nil {
			return err
		}

		return r.Create(ctx, secret)
	}

	l.Info("Key secret already exists", "secret", key.SecretName())
	return nil
}

func (r *KeyReconciler) checkCreateJobStatus(ctx context.Context, l logr.Logger, repo *resticv1.Repository, key *resticv1.Key) (bool, error) {
	job := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: *key.Status.CreateJobName}, job)
	if err != nil {
		return false, err
	}

	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == v1.ConditionTrue {
			logs, err := getJobPodLogs(ctx, r.Client, r.Config, l, job)
			if err != nil {
				return false, err
			}

			key.Status.Conditions = []metav1.Condition{
				{
					Type:               "Failed",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "KeyCreationJobFailed",
					Message:            logs,
				},
			}

			key.Status.CreateJobName = nil

			err = r.Status().Update(ctx, key)
			if err != nil {
				return false, err
			}

			repo.Status.Keys--
			err = r.Status().Update(ctx, repo)
			if err != nil {
				return false, err
			}
			return true, nil
		}

		if condition.Type == batchv1.JobComplete && condition.Status == v1.ConditionTrue {
			key.Status.Conditions = []metav1.Condition{
				{
					Type:               "Created",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "KeyCreationJobCompleted",
					Message:            "Key creation job successfully completed",
				},
			}

			logs, err := getJobPodLogs(ctx, r.Client, r.Config, l, job)
			if err != nil {
				return false, err
			}

			key.Status.CreateJobName = nil
			key.Status.KeyID = &keyIDRegex.FindStringSubmatch(logs)[1]

			err = r.Status().Update(ctx, key)
			if err != nil {
				return false, err
			}

			return true, nil
		}
	}

	return false, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resticv1.Key{}).
		Named("key").
		Complete(r)
}
