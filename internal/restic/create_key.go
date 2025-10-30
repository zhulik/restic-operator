package restic

import (
	"slices"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resticv1 "github.com/zhulik/restic-operator/api/v1"
	"github.com/zhulik/restic-operator/internal/labels"
)

func CreateAddKeyJob(repo *resticv1.Repository, addedKey *resticv1.Key) (*batchv1.Job, error) {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "add-key-",
			Namespace:    repo.Namespace,
			Labels: map[string]string{
				labels.KeyOperation: labels.KeyOperationAdd,
				labels.Key:          addedKey.Name,
				labels.Repository:   repo.Name,
				labels.KeyType:      labels.KeyTypeOperator,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:   corev1.RestartPolicyNever,
					SecurityContext: podSecurityContext,
					NodeSelector:    repo.Spec.NodeSelector,
					Containers: []corev1.Container{
						{
							Name:            "restic-init",
							Image:           repo.ImageName(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: slices.Concat(jobEnv(repo, addedKey), []corev1.EnvVar{
								{
									Name:  "RESTIC_PASSWORD_FILE",
									Value: "/operator-key/key.txt",
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
									Name:      "operator-key",
									MountPath: "/operator-key",
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
							Name: "operator-key",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: repo.OperatorSecretName(),
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
