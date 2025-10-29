package restic

import (
	"slices"
	"strconv"

	resticv1 "github.com/zhulik/restic-operator/api/v1"
	"github.com/zhulik/restic-operator/internal/labels"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateForgetJob(forgetSchedule *resticv1.ForgetSchedule, repo *resticv1.Repository) (*batchv1.CronJob, error) {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      forgetSchedule.JobName(),
			Namespace: forgetSchedule.Namespace,
			Labels: map[string]string{
				labels.Repository: repo.Name,
			},
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
									Image:           repo.ImageName(),
									ImagePullPolicy: corev1.PullIfNotPresent,
									Env: slices.Concat(repo.Spec.Env, []corev1.EnvVar{
										{
											Name:  "RESTIC_REPOSITORY",
											Value: repo.Spec.Repository,
										},
										{
											Name:  "RESTIC_PASSWORD_FILE",
											Value: "/operator-key/key.txt",
										},
									}),
									Args:            forgetResticArgs(forgetSchedule),
									SecurityContext: containerSecurityContext,
									VolumeMounts: []corev1.VolumeMount{
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
