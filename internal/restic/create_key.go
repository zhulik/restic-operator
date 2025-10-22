package restic

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/samber/lo"
	v1 "github.com/zhulik/restic-operator/api/v1"
)

func CreateAddKeyJob(ctx context.Context, kubeclient client.Client, repo *v1.Repository, addedKey *v1.Key) (*batchv1.Job, error) {
	if repo.Status.Keys == 1 {
		return addFirstKey(repo, addedKey)
	}
	return addKey(ctx, kubeclient, repo, addedKey)

}

func addFirstKey(repo *v1.Repository, addedKey *v1.Key) (*batchv1.Job, error) {
	args := []string{"key", "add", "--new-password-file", "/new-key/key.txt", "--host", addedKey.Spec.Host, "--user", addedKey.Spec.User, "--insecure-no-password"}
	env := jobEnv(repo)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("add-key-%s-%s", repo.Name, addedKey.Name),
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
									Name:      addedKey.SecretName(),
									MountPath: "/new-key",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: addedKey.SecretName(),
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

func addKey(ctx context.Context, kubeclient client.Client, repo *v1.Repository, addedKey *v1.Key) (*batchv1.Job, error) {
	args := []string{"key", "add", "--new-password-file", "/new-key/key.txt", "--host", addedKey.Spec.Host, "--user", addedKey.Spec.User}
	env := jobEnv(repo)

	var keyList v1.KeyList
	err := kubeclient.List(ctx, &keyList, client.InNamespace(repo.Namespace))
	if err != nil {
		return nil, err
	}

	openKey, ok := lo.Find(keyList.Items, func(key v1.Key) bool {
		return lo.ContainsBy(key.OwnerReferences, func(owner metav1.OwnerReference) bool {
			return owner.UID == repo.UID
		}) && key.Name != addedKey.Name
	})
	if !ok {
		panic("open key not found")
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("add-key-%s-%s", repo.Name, addedKey.Name),
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
									Name:      addedKey.SecretName(),
									MountPath: "/new-key",
									ReadOnly:  true,
								},
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
							Name: addedKey.SecretName(),
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
	}

	return job, nil
}
