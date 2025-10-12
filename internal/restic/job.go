package restic

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resticv1 "github.com/zhulik/restic-operator/api/v1"
)

const (
	Image     = "zhulik/restic"
	LatestTag = "latest"
)

func CreateJob(repo *resticv1.Repository, command string) (*batchv1.Job, error) {
	image := Image
	tag := LatestTag
	if repo.Spec.Version != "" {
		tag = repo.Spec.Version
	}
	image += ":" + tag

	env, err := jobEnv(repo, command)
	if err != nil {
		return nil, fmt.Errorf("failed to get job environment variables: %w", err)
	}

	// Create a Kubernetes Job to initialize the repository.
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
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env:             env,
						},
					},
				},
			},
		},
	}, nil
}

func jobEnv(repo *resticv1.Repository, command string) ([]corev1.EnvVar, error) {
	envVars := []corev1.EnvVar{
		{
			Name:  "RESTIC_REPOSITORY",
			Value: repo.Spec.Repository,
		},
		{
			Name:  "COMMAND",
			Value: command,
		},
	}

	// TODO: mount secrets as volumes instead of using environment variables
	for k, v := range repo.Spec.Env {
		envVars = append(envVars, corev1.EnvVar{
			Name: fmt.Sprintf("RESTIC_KEY_%s", k),
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: v.Name,
					},
					Key: v.Key,
				},
			},
		})
	}

	return envVars, nil
}
