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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	KeyFailed  = "Failed"
	KeyCreated = "Created"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KeySpec defines the desired state of Key
type KeySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// +required
	// +kubebuilder:validation:MinLength=1
	Repository string `json:"repository"`

	// +required
	// +kubebuilder:validation:MinLength=1
	User string `json:"user"`

	// +required
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`
}

// KeyStatus defines the observed state of Key.
type KeyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the Key resource.
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

	// +optional
	KeyID *string `json:"keyID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Repository",type=string,description="repository",JSONPath=`.spec.repository`
// +kubebuilder:printcolumn:name="Status",type=string,description="status",JSONPath=`.status.conditions[0].type`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Key is the Schema for the keys API
type Key struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Key
	// +required
	Spec KeySpec `json:"spec"`

	// status defines the observed state of Key
	// +optional
	Status KeyStatus `json:"status,omitempty,omitzero"`
}

func (k *Key) SetDefaultConditions() bool {
	if k.Status.Conditions != nil {
		return false
	}

	k.Status.Conditions = []metav1.Condition{
		{
			Type:               KeyCreated,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "KeyNotCreated",
			Message:            "Key is not yet created",
		},
		{
			Type:               KeyFailed,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "KeyCreationNotStarted",
			Message:            "Key creation is not yet started",
		},
	}

	return true
}

func (k Key) SecretName() string {
	return fmt.Sprintf("restic-key-%s-%s", k.Spec.Repository, k.Name)
}

func (k Key) IsCreated() bool {
	_, ok := conditions.ContainsAnyTrueCondition(k.Status.Conditions, KeyCreated)
	return ok
}

func (k Key) IsFailed() bool {
	_, ok := conditions.ContainsAnyTrueCondition(k.Status.Conditions, KeyFailed)
	return ok
}

// +kubebuilder:object:root=true

// KeyList contains a list of Key
type KeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Key `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Key{}, &KeyList{})
}
