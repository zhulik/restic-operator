package restic

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	v1 "github.com/zhulik/restic-operator/api/v1"
	"github.com/zhulik/restic-operator/internal/conditions"
	"github.com/zhulik/restic-operator/internal/labels"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateDeleteKeyJob(ctx context.Context, kubeclient client.Client, repo *v1.Repository, deletedKey *v1.Key) (*batchv1.Job, error) {
	statusType, ok := conditions.ContainsAnyTrueCondition(repo.Status.Conditions, "Creating", "Locked")
	if ok {
		return nil, fmt.Errorf("repository is in %s status, cannot delete key, should be retried", statusType)
	}

	repo.Status.Conditions, _ = conditions.UpdateCondition(repo.Status.Conditions, "Locked", metav1.Condition{
		Type:               "Locked",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "RepositoryIsLocked",
		Message:            "Repository is locked, this has nothing to do with restic repository locking, it's used for restic-operator internal concurrency control",
	})

	// TODO: when delete key job is done, we need to update the repository conditions: unlock it and update the number of keys

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
	env := jobEnv(repo, deletedKey)

	env = append(env, corev1.EnvVar{
		Name:  "RESTIC_PASSWORD_FILE",
		Value: "/current-key/key.txt",
	})

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "delete-key-",
			Namespace:    repo.Namespace,
			Annotations: map[string]string{
				labels.FirstKey:   "true",
				labels.Key:        deletedKey.Name,
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
