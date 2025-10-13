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
	"io"
	"time"

	"github.com/zhulik/restic-operator/internal/restic"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;create;
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Repository object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.1/pkg/reconcile
func (r *RepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx).WithValues("resource", req)

	repo := &resticv1.Repository{}
	err := r.Get(ctx, req.NamespacedName, repo)
	if err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if repo.Status.Conditions != nil {
		for _, condition := range repo.Status.Conditions {
			if condition.Type == "Creating" {
				l.Info("Repo is in creating state, checking job status", "job", *repo.Status.CreateJobName)
				completed, err := r.checkCreateJobStatus(ctx, l, repo)
				if err != nil {
					return reconcile.Result{}, err
				}

				if !completed {
					return reconcile.Result{RequeueAfter: 3 * time.Second}, nil
				}

				return reconcile.Result{}, nil
			}

			if condition.Type == "Failed" || condition.Type == "Created" {
				l.Info(fmt.Sprintf("Repo is in %s state, job is done.", condition.Type))
				return reconcile.Result{}, nil
			}
		}
	}

	if repo.Status.CreateJobName == nil {
		err = r.startCreateRepoJob(ctx, l, repo)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{RequeueAfter: 3 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *RepositoryReconciler) checkCreateJobStatus(ctx context.Context, l logr.Logger, repo *resticv1.Repository) (bool, error) {
	job := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{Namespace: repo.Namespace, Name: *repo.Status.CreateJobName}, job)
	if err != nil {
		return false, err
	}

	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == v1.ConditionTrue {
			logs, err := getJobPodLogs(ctx, r.Client, r.Config, l, job)
			if err != nil {
				return false, err
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
			repo.Status.CreateJobName = nil
			repo.Status.Keys = 0
			err = r.Status().Update(ctx, repo)
			if err != nil {
				return false, err
			}
			return true, nil
		}

		if condition.Type == batchv1.JobComplete && condition.Status == v1.ConditionTrue {
			repo.Status.Conditions = []metav1.Condition{
				{
					Type:               "Created",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "RepositoryInitializationJobCompleted",
					Message:            "Repository initialization job successfully completed",
				},
			}

			repo.Status.CreateJobName = nil
			repo.Status.Keys = 0
			err = r.Status().Update(ctx, repo)
			if err != nil {
				return false, err
			}

			return true, nil
		}
	}

	return false, nil
}

func (r *RepositoryReconciler) startCreateRepoJob(ctx context.Context, l logr.Logger, repo *resticv1.Repository) error {
	job := restic.CreateRepoInitJob(repo)
	if err := ctrl.SetControllerReference(repo, job, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, job); err != nil {
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

	l.Info("Repository status updated", "status", repo.Status)

	l.Info("Created repository initialization job", "job", job.Name)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resticv1.Repository{}).
		Named("repository").
		Complete(r)
}

// SetupWithManager sets up the controller with the Manager.
func getJobPodLogs(ctx context.Context, kubeclient client.Client, config *rest.Config, l logr.Logger, job *batchv1.Job) (string, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}

	container := job.Spec.Template.Spec.Containers[0]

	// Find pods created by the job using controller-runtime client
	var podList v1.PodList
	err = kubeclient.List(ctx, &podList,
		client.InNamespace(job.Namespace),
		client.MatchingLabels{"batch.kubernetes.io/job-name": job.Name})
	if err != nil {
		return "", fmt.Errorf("failed to list pods for job %s: %w", job.Name, err)
	}

	pod := podList.Items[0]

	// Get logs using the clientset
	req := clientset.CoreV1().Pods(job.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{
		Container: container.Name,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to stream logs from pod %s: %w", pod.Name, err)
	}
	defer stream.Close() // nolint:errcheck

	logs, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("failed to read logs from pod %s: %w", pod.Name, err)
	}

	return string(logs), nil
}
