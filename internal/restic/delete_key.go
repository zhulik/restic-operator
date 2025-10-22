package restic

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	v1 "github.com/zhulik/restic-operator/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateDeleteKeyJob(ctx context.Context, kubeclient client.Client, repo *v1.Repository, deletedKey *v1.Key) (*batchv1.Job, error) {
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
			Name:      fmt.Sprintf("delete-key-%s-%s", repo.Name, deletedKey.Name),
			Namespace: repo.Namespace,
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
							Env:             env,
							Args:            args,
							SecurityContext: containerSecurityContext,
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
