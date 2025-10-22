package restic

import (
	"fmt"

	v1 "github.com/zhulik/restic-operator/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateRepoInitJob(repo *v1.Repository) *batchv1.Job {
	var backoffLimit = int32(0)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("init-repo-%s", repo.Name),
			Namespace: repo.Namespace,
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
							Image:           imageName(repo),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name:  "RESTIC_REPOSITORY",
									Value: repo.Spec.Repository,
								},
							},
							Args:            []string{"init", "--insecure-no-password"},
							SecurityContext: containerSecurityContext,
						},
					},
				},
			},
		},
	}
}
