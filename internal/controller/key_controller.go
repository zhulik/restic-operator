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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	resticv1 "github.com/zhulik/restic-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KeyReconciler reconciles a Key object
type KeyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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

	if key.Status.KeyID == nil {
		l.Info("Key ID is not set, creating and assigning a key")
		keyID, err := r.createKey(ctx, l, key)
		if err != nil {
			return ctrl.Result{}, err
		}

		key.Status.KeyID = &keyID
		err = r.Status().Update(ctx, key)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *KeyReconciler) createKey(ctx context.Context, l logr.Logger, key *resticv1.Key) (string, error) {
	err := r.createSecretIfNotExists(ctx, l, key)
	if err != nil {
		return "", err
	}

	repo := &resticv1.Repository{}
	err = r.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: key.Spec.Repository}, repo)
	if err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("Repository not found, will retry", "repository", key.Spec.Repository)
		}
		return "", err
	}

	// TODO: set repo as owner of the key?

	return "", nil
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

// SetupWithManager sets up the controller with the Manager.
func (r *KeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resticv1.Key{}).
		Named("key").
		Complete(r)
}
