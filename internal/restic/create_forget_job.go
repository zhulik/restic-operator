package restic

import (
	"bytes"
	"slices"
	"text/template"

	resticv1 "github.com/zhulik/restic-operator/api/v1"
	"github.com/zhulik/restic-operator/internal/labels"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	forgetJobScript = template.Must(template.New("forget").Parse(`
	set -eu

	{{ if (gt .CheckPercentage 0) }}
	restic check --read-data-subset {{ .CheckPercentage }}%
	{{ end }}

	restic forget \
	{{ if .Prune }}--prune{{ end }} \
	{{ if (gt .KeepLast 0) }}   --keep-last {{ .KeepLast }}{{ end }} \
	{{ if (gt .KeepHourly 0) }} --keep-hourly {{ .KeepHourly }}{{ end }} \
	{{ if (gt .KeepDaily 0) }}  --keep-daily {{ .KeepDaily }}{{ end }} \
	{{ if (gt .KeepWeekly 0) }} --keep-weekly {{ .KeepWeekly }}{{ end }} \
	{{ if (gt .KeepMonthly 0) }}--keep-monthly {{ .KeepMonthly }}{{ end }} \
	{{ if (gt .KeepYearly 0) }} --keep-yearly {{ .KeepYearly }}{{ end }} # Yes, you must pass it=)
	`))
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
									Command:         []string{"/bin/sh", "-c"},
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
	var buf bytes.Buffer
	if err := forgetJobScript.Execute(&buf, forgetSchedule.Spec); err != nil {
		panic(err)
	}

	return []string{buf.String()}
}
