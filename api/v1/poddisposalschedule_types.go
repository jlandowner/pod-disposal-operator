package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PodDisposalScheduleSpec defines the desired state of PodDisposalSchedule
type PodDisposalScheduleSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of PodDisposalSchedule. Edit PodDisposalSchedule_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// PodDisposalScheduleStatus defines the observed state of PodDisposalSchedule
type PodDisposalScheduleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// PodDisposalSchedule is the Schema for the poddisposalschedules API
type PodDisposalSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodDisposalScheduleSpec   `json:"spec,omitempty"`
	Status PodDisposalScheduleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PodDisposalScheduleList contains a list of PodDisposalSchedule
type PodDisposalScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodDisposalSchedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodDisposalSchedule{}, &PodDisposalScheduleList{})
}
