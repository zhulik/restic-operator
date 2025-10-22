package restic

import (
	"slices"

	"github.com/samber/lo"
	v1 "github.com/zhulik/restic-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	Image     = "restic/restic"
	LatestTag = "latest"
)

var podSecurityContext = &corev1.PodSecurityContext{
	RunAsNonRoot: lo.ToPtr(true),
	RunAsUser:    lo.ToPtr(int64(1000)),
	RunAsGroup:   lo.ToPtr(int64(1000)),
	SeccompProfile: &corev1.SeccompProfile{
		Type: corev1.SeccompProfileTypeRuntimeDefault,
	},
}

var containerSecurityContext = &corev1.SecurityContext{
	Capabilities: &corev1.Capabilities{
		Drop: []corev1.Capability{"ALL"},
	},
	AllowPrivilegeEscalation: lo.ToPtr(false),
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

func jobEnv(repo *v1.Repository, key *v1.Key) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "RESTIC_REPOSITORY",
			Value: repo.Spec.Repository,
		},
		{
			Name:  "RESTIC_HOST",
			Value: key.Spec.Host,
		},
		{
			Name:  "RESTIC_USER",
			Value: key.Spec.User,
		},
		{
			Name:  "NEW_KEY_FILE",
			Value: "/new-key/key.txt",
		},
	}

	return slices.Concat(envVars, repo.Spec.Env)
}
