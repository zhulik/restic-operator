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
	"github.com/zhulik/restic-operator/internal/conditions"
	"github.com/zhulik/restic-operator/internal/labels"
)

func CreateAddKeyJob(ctx context.Context, kubeclient client.Client, repo *v1.Repository, addedKey *v1.Key) (*batchv1.Job, error) {
	statusType, ok := conditions.ContainsAnyTrueCondition(repo.Status.Conditions, "Creating", "Locked")
	if ok {
		return nil, fmt.Errorf("repository is in %s status, cannot add key, should be retried", statusType)
	}

	_, ok = conditions.ContainsAnyTrueCondition(repo.Status.Conditions, "Secure")
	if !ok {
		return addFirstKey(repo, addedKey)
	}

	// If the repository already has keys, we pick the first one that came along to open the repo and add the new key
	var keyList v1.KeyList
	err := kubeclient.List(ctx, &keyList, client.InNamespace(repo.Namespace), client.MatchingLabels{
		"restic.zhulik.wtf/repository": repo.Name,
	})
	if err != nil {
		return nil, err
	}
	if len(keyList.Items) == 0 {
		return nil, fmt.Errorf("no keys found for repository %s", repo.Name)
	}

	openKey, ok := lo.Find(keyList.Items, func(key v1.Key) bool {
		// We don't want to use the key that we're adding
		return key.Name != addedKey.Name
	})
	if !ok {
		panic("open key not found")
	}
	return addKey(repo, addedKey, &openKey)

}

func addFirstKey(repo *v1.Repository, addedKey *v1.Key) (*batchv1.Job, error) {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "add-key-",
			Namespace:    repo.Namespace,
			Annotations: map[string]string{
				labels.FirstKey:   "true",
				labels.Key:        addedKey.Name,
				labels.Repository: repo.Name,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:   corev1.RestartPolicyNever,
					SecurityContext: podSecurityContext,
					Containers: []corev1.Container{
						{
							Name:            "restic-init",
							Image:           imageName(repo),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env:             jobEnv(repo, addedKey),
							Command:         []string{"/bin/sh", "-c"},
							Args:            []string{addFirstKeyScript},
							SecurityContext: containerSecurityContext,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "new-key",
									MountPath: "/new-key",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "new-key",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: addedKey.SecretName(),
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

func addKey(repo *v1.Repository, addedKey *v1.Key, openKey *v1.Key) (*batchv1.Job, error) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "add-key-",
			Namespace:    repo.Namespace,
			Annotations: map[string]string{
				labels.FirstKey:   "false",
				labels.Key:        addedKey.Name,
				labels.Repository: repo.Name,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:   corev1.RestartPolicyNever,
					SecurityContext: podSecurityContext,
					Containers: []corev1.Container{
						{
							Name:            "restic-init",
							Image:           imageName(repo),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: slices.Concat(jobEnv(repo, addedKey), []corev1.EnvVar{
								{
									Name:  "RESTIC_PASSWORD_FILE",
									Value: "/current-key/key.txt",
								},
							}),
							Args: []string{
								"key", "add",
								"--new-password-file", "$(NEW_KEY_FILE)",
								"--host", addedKey.Spec.Host,
								"--user", addedKey.Spec.User,
							},
							SecurityContext: containerSecurityContext,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "new-key",
									MountPath: "/new-key",
									ReadOnly:  true,
								},
								{
									Name:      "open-key",
									MountPath: "/current-key",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "new-key",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: addedKey.SecretName(),
									Items: []corev1.KeyToPath{
										{
											Key:  "key",
											Path: "key.txt",
										},
									},
								},
							},
						},
						{
							Name: "open-key",
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
	}

	return job, nil
}
