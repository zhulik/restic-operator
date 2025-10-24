package conditions

import (
	"slices"

	"github.com/samber/lo"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func JobHasAnyTrueCondition(job *batchv1.Job, conditionTypes ...batchv1.JobConditionType) (batchv1.JobConditionType, bool) {
	for _, condition := range job.Status.Conditions {
		if slices.Contains(conditionTypes, condition.Type) && condition.Status == corev1.ConditionTrue {
			return condition.Type, true
		}
	}
	return "", false
}

func ContainsAnyTrueCondition(conditions []metav1.Condition, conditionTypes ...string) (string, bool) {
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

func UpdateCondition(conditions []metav1.Condition, conditionType string, newCondition metav1.Condition) ([]metav1.Condition, bool) {
	newConditions := lo.Filter(conditions, func(condition metav1.Condition, _ int) bool {
		return condition.Type != conditionType
	})

	if len(newConditions) == len(conditions) {
		return conditions, false
	}

	newConditions = append(newConditions, newCondition)
	return newConditions, true
}
