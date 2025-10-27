package restic

import (
	"slices"

	v1 "github.com/zhulik/restic-operator/api/v1"
	"github.com/zhulik/restic-operator/internal/labels"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateRepoInitJob(repo *v1.Repository) *batchv1.Job {
	var backoffLimit = int32(1)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      repo.Name,
			Namespace: repo.Namespace,
			Labels: map[string]string{
				labels.Repository: repo.Name,
			},
		},

		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:   corev1.RestartPolicyNever,
					SecurityContext: podSecurityContext,
					Containers: []corev1.Container{
						{
							Name:            "restic-init",
							Image:           repo.ImageName(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: slices.Concat([]corev1.EnvVar{
								{
									Name:  "RESTIC_REPOSITORY",
									Value: repo.Spec.Repository,
								},
							}, repo.Spec.Env),
							Args:            []string{"init", "--insecure-no-password"},
							SecurityContext: containerSecurityContext,
						},
					},
				},
			},
		},
	}
}
