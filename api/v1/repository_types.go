/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"fmt"

	"github.com/zhulik/restic-operator/internal/conditions"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RepositoryCreated = "Created"
	RepositoryFailed  = "Failed"
	RepositorySecure  = "Secure"
)

const (
	Image     = "restic/restic"
	LatestTag = "latest"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RepositorySpec defines the desired state of Repository
type RepositorySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// URL is the URL of the repository
	// +required
	// +kubebuilder:validation:MinLength=1
	Repository string `json:"repository"`

	// Env is the map of environment variables to use when initializing the repository.
	// For instance, the s3 backend requires at least the following environment variables:
	// - AWS_ACCESS_KEY_ID
	// - AWS_SECRET_ACCESS_KEY
	// +optional
	Env []corev1.EnvVar `json:"env"`

	// Version is the version of restic to use. Leave it empty to use the latest version.
	// +optional
	Version string `json:"version,omitempty"`
}

// RepositoryStatus defines the observed state of Repository.
type RepositoryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the Repository resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Repository",type=string,description="repository",JSONPath=`.spec.repository`
// +kubebuilder:printcolumn:name="Created",type=string,description="created",JSONPath=`.status.conditions[?(@.type == 'Created')].status`
// +kubebuilder:printcolumn:name="Failed",type=string,description="failed",JSONPath=`.status.conditions[?(@.type == 'Failed')].status`
// +kubebuilder:printcolumn:name="Secure",type=string,description="secure",JSONPath=`.status.conditions[?(@.type == 'Secure')].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Repository is the Schema for the repositories API
type Repository struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Repository
	// +required
	Spec RepositorySpec `json:"spec"`

	// status defines the observed state of Repository
	// +optional
	Status RepositoryStatus `json:"status,omitempty,omitzero"`
}

func (r *Repository) SetDefaults() {
	if r.Spec.Version == "" {
		r.Spec.Version = LatestTag
	}
}

func (r *Repository) SetDefaultConditions() bool {
	if r.Status.Conditions != nil {
		return false
	}

	r.Status.Conditions = []metav1.Condition{
		{
			Type:               RepositoryCreated,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "RepositoryNotCreated",
			Message:            "Repository is not yet created",
		},
		{
			Type:               RepositorySecure,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "RepositoryHasNoKeys",
			Message:            "Repository created without keys. A key needs to be added to the repository to make it secure.",
		},
		{
			Type:               RepositoryFailed,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "RepositoryCreationNotStarted",
			Message:            "Repository creation is not yet started",
		},
	}

	return true
}

func (r *Repository) IsCreated() bool {
	_, ok := conditions.ContainsAnyTrueCondition(r.Status.Conditions, RepositoryCreated)
	return ok
}

func (r *Repository) OperatorSecretName() string {
	return fmt.Sprintf("operator-key-%s", r.Name)
}

func (r *Repository) IsSecure() bool {
	_, ok := conditions.ContainsAnyTrueCondition(r.Status.Conditions, RepositorySecure)
	return ok
}

func (r *Repository) IsFailed() bool {
	_, ok := conditions.ContainsAnyTrueCondition(r.Status.Conditions, RepositoryFailed)
	return ok
}

func (r *Repository) SetFailedCondition(message string) {
	r.Status.Conditions, _ = conditions.UpdateCondition(r.Status.Conditions, RepositoryFailed, metav1.Condition{
		Type:               RepositoryFailed,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "RepositoryInitializationFailed",
		Message:            "Repository initialization job failed: " + message,
	})

	r.Status.Conditions, _ = conditions.UpdateCondition(r.Status.Conditions, RepositoryCreated, metav1.Condition{
		Type:               RepositoryCreated,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             "RepositoryInitializationFailed",
		Message:            "Repository initialization job failed: " + message,
	})
}

func (r *Repository) SetCreatedCondition() {
	r.Status.Conditions, _ = conditions.UpdateCondition(r.Status.Conditions, RepositoryCreated, metav1.Condition{
		Type:               RepositoryCreated,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "RepositoryCreated",
		Message:            "Repository initialization job successfully completed",
	})

	r.Status.Conditions, _ = conditions.UpdateCondition(r.Status.Conditions, RepositoryFailed, metav1.Condition{
		Type:               RepositoryFailed,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             "RepositoryCreated",
		Message:            "Repository initialization job successfully completed",
	})
}

func (r *Repository) SetSecureCondition() {
	r.Status.Conditions, _ = conditions.UpdateCondition(r.Status.Conditions, RepositorySecure, metav1.Condition{
		Type:               RepositorySecure,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "RepositoryHasAtLeastOneKey",
		Message:            "Repository has at least one key added to it",
	})
}

func (r *Repository) ImageName() string {
	return fmt.Sprintf("%s:%s", Image, r.Spec.Version)
}

// +kubebuilder:object:root=true

// RepositoryList contains a list of Repository
type RepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Repository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Repository{}, &RepositoryList{})
}
