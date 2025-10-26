package restic

import (
	"context"
	"slices"
	"strconv"

	resticv1 "github.com/zhulik/restic-operator/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateForgetJob(ctx context.Context, forgetSchedule *resticv1.ForgetSchedule, repo *resticv1.Repository, keySecret *corev1.Secret) (*batchv1.CronJob, error) {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      forgetSchedule.JobName(),
			Namespace: forgetSchedule.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:          forgetSchedule.Spec.Schedule,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			JobTemplate: batchv1.JobTemplateSpec{
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
									Env: slices.Concat(repo.Spec.Env, []corev1.EnvVar{
										{
											Name:  "RESTIC_REPOSITORY",
											Value: repo.Spec.Repository,
										},
										{
											Name:  "RESTIC_PASSWORD_FILE",
											Value: "/current-key/key.txt",
										},
									}),
									Args:            forgetResticArgs(forgetSchedule),
									SecurityContext: containerSecurityContext,
									VolumeMounts: []corev1.VolumeMount{
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
									Name: "open-key",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: keySecret.Name,
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
			},
		},
	}, nil
}

func forgetResticArgs(forgetSchedule *resticv1.ForgetSchedule) []string {
	args := []string{"forget"}
	if forgetSchedule.Spec.Prune {
		args = append(args, "--prune")
	}
	args = append(args, "--keep-last", strconv.Itoa(forgetSchedule.Spec.KeepLast))
	args = append(args, "--keep-hourly", strconv.Itoa(forgetSchedule.Spec.KeepHourly))
	args = append(args, "--keep-daily", strconv.Itoa(forgetSchedule.Spec.KeepDaily))
	args = append(args, "--keep-weekly", strconv.Itoa(forgetSchedule.Spec.KeepWeekly))
	args = append(args, "--keep-monthly", strconv.Itoa(forgetSchedule.Spec.KeepMonthly))
	args = append(args, "--keep-yearly", strconv.Itoa(forgetSchedule.Spec.KeepYearly))

	return args
}
