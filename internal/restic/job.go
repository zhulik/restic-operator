package restic

import (
	"fmt"
	"slices"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resticv1 "github.com/zhulik/restic-operator/api/v1"
)

const (
	Image     = "restic/restic"
	LatestTag = "latest"
)

func CreateRepoInitJob(repo *resticv1.Repository) *batchv1.Job {
	var backoffLimit = int32(0)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("init-repo-%s-", repo.Name),
			Namespace:    repo.Namespace,
			Labels: map[string]string{
				"app":        "restic-repository-init",
				"repository": repo.Name,
			},
		},

		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "restic-init",
							Image:           imageName(repo),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env:             jobEnv(repo),
							Args:            []string{"init", "--insecure-no-password"},
						},
					},
				},
			},
		},
	}
}

func imageName(repo *resticv1.Repository) string {
	image := Image
	tag := LatestTag
	if repo.Spec.Version != "" {
		tag = repo.Spec.Version
	}
	image += ":" + tag
	return image
}

func jobEnv(repo *resticv1.Repository) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "RESTIC_REPOSITORY",
			Value: repo.Spec.Repository,
		},
	}

	return slices.Concat(envVars, repo.Spec.Env)
}
