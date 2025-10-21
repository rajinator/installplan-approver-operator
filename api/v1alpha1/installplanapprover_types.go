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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// InstallPlanApproverSpec defines the desired state of InstallPlanApprover.
type InstallPlanApproverSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// TargetNamespaces is a list of namespaces to watch for InstallPlans
	// If empty, watches all namespaces
	// +optional
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`

	// AutoApprove enables automatic approval of InstallPlans
	// +kubebuilder:default=true
	AutoApprove bool `json:"autoApprove,omitempty"`

	// OperatorNames is an optional list of operator names to approve
	// If empty, approves all operators
	// +optional
	OperatorNames []string `json:"operatorNames,omitempty"`
}

// InstallPlanApproverStatus defines the observed state of InstallPlanApprover.
type InstallPlanApproverStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ApprovedCount is the total number of InstallPlans approved
	// +optional
	ApprovedCount int32 `json:"approvedCount,omitempty"`

	// LastApprovedPlan contains the name of the last approved InstallPlan
	// +optional
	LastApprovedPlan string `json:"lastApprovedPlan,omitempty"`

	// LastApprovedTime is the timestamp of the last approval
	// +optional
	LastApprovedTime *metav1.Time `json:"lastApprovedTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ipa

// InstallPlanApprover is the Schema for the installplanapprovers API.
type InstallPlanApprover struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstallPlanApproverSpec   `json:"spec,omitempty"`
	Status InstallPlanApproverStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstallPlanApproverList contains a list of InstallPlanApprover.
type InstallPlanApproverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InstallPlanApprover `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InstallPlanApprover{}, &InstallPlanApproverList{})
}
