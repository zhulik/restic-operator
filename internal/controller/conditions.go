package controller

import (
	"slices"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func jobHasAnyTrueCondition(job *batchv1.Job, conditionTypes ...batchv1.JobConditionType) (batchv1.JobConditionType, bool) {
	for _, condition := range job.Status.Conditions {
		if slices.Contains(conditionTypes, condition.Type) && condition.Status == corev1.ConditionTrue {
			return condition.Type, true
		}
	}
	return "", false
}

func containsAnyTrueCondition(conditions []metav1.Condition, conditionTypes ...string) (string, bool) {
	if conditions == nil {
		return "", false
	}

	for _, condition := range conditions {
		if slices.Contains(conditionTypes, condition.Type) && condition.Status == metav1.ConditionTrue {
			return condition.Type, true
		}
	}
	return "", false
}
