package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodDisposalScheduleSpec defines the desired state of PodDisposalSchedule
type PodDisposalScheduleSpec struct {
	// Selector defines target pods to disposal
	// +kubebuilder:validation:Required
	Selector Selector `json:"selector"`
	// Schedule is a period duration of disposal, written in "cron" format
	// +kubebuilder:validation:Required
	Schedule string `json:"schedule"`
	// Strategy defines a policies of pod disposal
	// +kubebuilder:validation:Required
	Strategy Strategy `json:"strategy,omitempty"`
}

// Selector needs one of "Deployment" or "NamePrefix".
type Selector struct {
	// Namespase is a pod's namespace, default is the same namespace.
	Namespase string `json:"namespase,omitempty"`
	// Type is a selector type
	// +kubebuilder:validation:Required
	Type SelectorType `json:"type"`
	// Name is a Deployment or a prefix of pod's name
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// SelectorType is a type of selector
// +kubebuilder:validation:Enum=Deployment
type SelectorType string

const (
	// DeploymentSelectorType select target pods by Deployment name
	DeploymentSelectorType SelectorType = "Deployment"
)

// Strategy defines a policies of pod disposal
type Strategy struct {
	// DisposalConcurrency is a number of pods deleted at the same time.
	// +kubebuilder:validation:Minimum:=1
	DisposalConcurrency int `json:"disposalConcurrency,omitempty"`
	// Lifespan is a period of pod alive, written in "time.Duration"
	// +kubebuilder:validation:Type:=string
	// +kubebuilder:validation:Required
	Lifespan string `json:"lifespan"`
	// MinAvailable is a number of pods must be available.
	// +kubebuilder:validation:Minimum:=0
	MinAvailable int `json:"minAvailable,omitempty"`
}

// PodDisposalScheduleStatus defines the observed state of PodDisposalSchedule
type PodDisposalScheduleStatus struct {
	// LastDisposalCounts is a number of deleted pods in last reconciled time
	LastDisposalCounts int `json:"lastDisposalCounts"`
	// LastDisposalTime is a last reconciled time
	LastDisposalTime metav1.Time `json:"lastDisposalTime"`
	// NextDisposalTime is a next reconcile time
	NextDisposalTime metav1.Time `json:"nextDisposalTime"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=pds
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="TargetType",type=string,JSONPath=`.spec.selector.type`
// +kubebuilder:printcolumn:name="TargetName",type=string,JSONPath=`.spec.selector.name`
// +kubebuilder:printcolumn:name="LastDisposalCounts",type=integer,JSONPath=`.status.lastDisposalCounts`
// +kubebuilder:printcolumn:name="LastDisposalTime",type=string,JSONPath=`.status.lastDisposalTime`
// +kubebuilder:printcolumn:name="NextDisposalTime",type=string,JSONPath=`.status.nextDisposalTime`

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
