package restic

import (
	"context"
	"fmt"
	"slices"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/samber/lo"

	v1 "github.com/zhulik/restic-operator/api/v1"
)

const (
	Image     = "restic/restic"
	LatestTag = "latest"
)

func CreateRepoInitJob(repo *v1.Repository) *batchv1.Job {
	var backoffLimit = int32(0)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("init-repo-%s-", repo.Name),
			Namespace:    repo.Namespace,
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

func CreateAddKeyJob(ctx context.Context, kubeclient client.Client, repo *v1.Repository, key *v1.Key) (*batchv1.Job, error) {
	var backoffLimit = int32(0)

	firstKey := repo.Status.Keys == 0

	args := []string{"key", "add", "--new-password-file", "/new-key/key.txt", "--host", key.Spec.Host, "--user", key.Spec.User}
	env := jobEnv(repo)

	if firstKey {
		args = append(args, "--insecure-no-password")
	} else {
		env = append(env, corev1.EnvVar{
			Name:  "RESTIC_PASSWORD_FILE",
			Value: "/current-key/key.txt",
		})
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("add-key-%s-%s-", repo.Name, key.Name),
			Namespace:    repo.Namespace,
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
							Env:             env,
							Args:            args,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      key.SecretName(),
									MountPath: "/new-key",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: key.SecretName(),
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: key.SecretName(),
									Items: []corev1.KeyToPath{
										{
											Key:  "key",
											Path: "key.txt",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if !firstKey {
		var keyList v1.KeyList
		err := kubeclient.List(ctx, &keyList, client.InNamespace(repo.Namespace))
		if err != nil {
			return nil, err
		}

		openKey, ok := lo.Find(keyList.Items, func(key v1.Key) bool {
			return lo.ContainsBy(key.OwnerReferences, func(owner metav1.OwnerReference) bool {
				return owner.UID == repo.UID
			})
		})
		if !ok {
			panic("open key not found")
		}

		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: openKey.SecretName(),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: openKey.SecretName(),
					Items: []corev1.KeyToPath{
						{
							Key:  "key",
							Path: "key.txt",
						},
					},
				},
			},
		})

		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      openKey.SecretName(),
			MountPath: "/current-key",
			ReadOnly:  true,
		})
	}

	return job, nil
}

func CreateDeleteKeyJob(ctx context.Context, kubeclient client.Client, repo *v1.Repository, deletedKey *v1.Key) (*batchv1.Job, error) {
	var backoffLimit = int32(0)

	var keyList v1.KeyList
	err := kubeclient.List(ctx, &keyList, client.InNamespace(repo.Namespace))
	if err != nil {
		return nil, err
	}

	openKey, ok := lo.Find(keyList.Items, func(key v1.Key) bool {
		return lo.ContainsBy(key.OwnerReferences, func(owner metav1.OwnerReference) bool {
			return owner.UID == repo.UID && key.Name != deletedKey.Name
		})
	})
	if !ok {
		panic("open key not found")
	}

	args := []string{"key", "remove", *deletedKey.Status.KeyID}
	env := jobEnv(repo)

	env = append(env, corev1.EnvVar{
		Name:  "RESTIC_PASSWORD_FILE",
		Value: "/current-key/key.txt",
	})

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("delete-key-%s-%s-", repo.Name, deletedKey.Name),
			Namespace:    repo.Namespace,
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
							Env:             env,
							Args:            args,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      openKey.SecretName(),
									MountPath: "/current-key",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: openKey.SecretName(),
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: openKey.SecretName(),
									Items: []corev1.KeyToPath{
										{
											Key:  "key",
											Path: "key.txt",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}, nil

}

func imageName(repo *v1.Repository) string {
	image := Image
	tag := LatestTag
	if repo.Spec.Version != "" {
		tag = repo.Spec.Version
	}
	image += ":" + tag
	return image
}

func jobEnv(repo *v1.Repository) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "RESTIC_REPOSITORY",
			Value: repo.Spec.Repository,
		},
	}

	return slices.Concat(envVars, repo.Spec.Env)
}
