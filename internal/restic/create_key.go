package restic

import (
	"context"
	"fmt"
	"slices"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resticv1 "github.com/zhulik/restic-operator/api/v1"
	"github.com/zhulik/restic-operator/internal/labels"
)

func CreateAddKeyJob(ctx context.Context, kubeclient client.Client, repo *resticv1.Repository, addedKey *resticv1.Key) (*batchv1.Job, error) {
	var keyList resticv1.KeyList
	err := kubeclient.List(ctx, &keyList, client.InNamespace(repo.Namespace), client.MatchingLabels{
		labels.Repository: repo.Name,
		labels.KeyType:    labels.KeyTypeOperator,
	})
	if err != nil {
		return nil, err
	}
	if len(keyList.Items) == 0 {
		return nil, fmt.Errorf("no operator keys found for repository %s", repo.Name)
	}

	openKey := keyList.Items[0]

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "add-key-",
			Namespace:    repo.Namespace,
			Labels: map[string]string{
				labels.FirstKey:   "false",
				labels.Key:        addedKey.Name,
				labels.Repository: repo.Name,
				labels.KeyType:    labels.KeyTypeOperator,
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
							Image:           repo.ImageName(),
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
	}, nil
}
