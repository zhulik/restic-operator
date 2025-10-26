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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ForgetScheduleCreated = "Created"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ForgetScheduleSpec defines the desired state of ForgetSchedule
type ForgetScheduleSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// +required
	// +kubebuilder:validation:MinLength=9
	Schedule string `json:"schedule"`

	// +required
	// +kubebuilder:validation:MinLength=1
	Repository string `json:"repository"`

	// +required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`

	// +optional
	Prune bool `json:"prune,omitempty"`

	// +optional
	// +kubebuilder:validation:Minimum=1
	KeepLast int `json:"keepLast,omitempty"`

	// +optional
	// +kubebuilder:validation:Minimum=1
	KeepHourly int `json:"keepHourly,omitempty"`

	// +optional
	// +kubebuilder:validation:Minimum=1
	KeepDaily int `json:"keepDaily,omitempty"`

	// +optional
	// +kubebuilder:validation:Minimum=1
	KeepWeekly int `json:"keepWeekly,omitempty"`

	// +optional
	// +kubebuilder:validation:Minimum=1
	KeepMonthly int `json:"keepMonthly,omitempty"`

	// +optional
	// +kubebuilder:validation:Minimum=1
	KeepYearly int `json:"keepYearly,omitempty"`
}

// ForgetScheduleStatus defines the observed state of ForgetSchedule.
type ForgetScheduleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the ForgetSchedule resource.
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
// +kubebuilder:printcolumn:name="Status",type=string,description="status",JSONPath=`.status.conditions[0].type`
// +kubebuilder:printcolumn:name="Key",type=string,description="key",JSONPath=`.spec.key`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ForgetSchedule is the Schema for the forgetschedules API
type ForgetSchedule struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of ForgetSchedule
	// +required
	Spec ForgetScheduleSpec `json:"spec"`

	// status defines the observed state of ForgetSchedule
	// +optional
	Status ForgetScheduleStatus `json:"status,omitempty,omitzero"`
}

func (f *ForgetSchedule) JobName() string {
	return fmt.Sprintf("forget-%s", f.Name)
}

// +kubebuilder:object:root=true

// ForgetScheduleList contains a list of ForgetSchedule
type ForgetScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ForgetSchedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ForgetSchedule{}, &ForgetScheduleList{})
}
