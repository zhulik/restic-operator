package restic

import (
	"fmt"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resticv1 "github.com/zhulik/restic-operator/api/v1"
)

func CreateJob(repo *resticv1.Repository) *batchv1.Job {
	image := "restic/restic"
	if repo.Spec.Version != nil && *repo.Spec.Version != "" {
		image += ":" + *repo.Spec.Version
	}

	// Create a Kubernetes Job to initialize the repository.
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "init-repo-" + repo.Name,
			Namespace: repo.Namespace,
			Labels: map[string]string{
				"app":        "restic-repository-init",
				"repository": repo.Name,
			},
		},

		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:  "restic-init",
							Image: image,
							Command: []string{
								"restic",
								"init",
								"--json",
							},
							Env: jobEnv(repo),
						},
					},
				},
			},
		},
	}
}

func jobEnv(repo *resticv1.Repository) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "RESTIC_REPOSITORY",
			Value: repo.Spec.Repository,
		},
	}

	if repo.Spec.Import != nil && *repo.Spec.Import {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "RESTIC_IMPORT",
			Value: strconv.FormatBool(*repo.Spec.Import),
		})
	}

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
	// Add RESTIC_PASSWORD from the "main" key
	if mainKey, ok := repo.Spec.Keys["main"]; ok {
		envVars = append(envVars, corev1.EnvVar{
			Name: "RESTIC_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: mainKey.Name,
					},
					Key: mainKey.Key,
				},
			},
		})
	}
	return envVars
}
