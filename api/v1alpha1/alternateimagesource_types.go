/*
Copyright 2020 Fairwinds

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Target is a target for image replacement
type Target struct {
	Name string           `json:"name"`
	Type metav1.GroupKind `json:"type"`
}

// ImageSourceReplacement is a single replacement
type ImageSourceReplacement struct {
	// InitialRepository is the repository that you deploy with.
	// For example: quay.io/fairwinds/polaris
	InitialRepository string `json:"initialRepository"`

	// ReplacementRepository is the 1:1 replacement for the InitialRepository
	ReplacementRepository string `json:"replacementRepository"`

	// Targets is a list of objects you want to target for
	// replacement of the image in the event of an ImagePullError
	Targets []Target `json:"targets"`
}

// AlternateImageSourceSpec defines the desired state of AlternateImageSource
type AlternateImageSourceSpec struct {
	ImageSourceReplacements []ImageSourceReplacement `json:"imageSourceReplacements"`
}

// AlternateImageSourceStatus defines the observed state of AlternateImageSource
type AlternateImageSourceStatus struct {
	ObservedGeneration int64 `json:"observedGeneration"`
	// TargetsActivated is a list of targets that have been switched over
	TargetsActivated []Target `json:"targetsActivated,omitempty"`
	// TargetsAvailable is a list of objects that are available to be switched
	TargetsAvailable []Target `json:"targetsAvailable,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// AlternateImageSource is the Schema for the alternateimagesources API
type AlternateImageSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AlternateImageSourceSpec   `json:"spec,omitempty"`
	Status AlternateImageSourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AlternateImageSourceList contains a list of AlternateImageSource
type AlternateImageSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AlternateImageSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AlternateImageSource{}, &AlternateImageSourceList{})
}
