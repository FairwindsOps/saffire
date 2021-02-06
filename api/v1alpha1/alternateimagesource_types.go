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
	"k8s.io/apimachinery/pkg/types"
)

// Target is a target for image replacement
type Target struct {
	Name string           `json:"name"`
	Type metav1.GroupKind `json:"type"`
	// Container is the container that matches our list
	Container string    `json:"container,omitempty"`
	UID       types.UID `json:"uid,omitempty"`
}

// ImageSourceReplacement is a single replacement
type ImageSourceReplacement struct {
	// EquivalentRepositories is a list of possible replacement repositories
	// they should each have the same set of tags available
	EquivalentRepositories []string `json:"equivalentRepositories"`
}

// AlternateImageSourceSpec defines the desired state of AlternateImageSource
type AlternateImageSourceSpec struct {
	ImageSourceReplacements []ImageSourceReplacement `json:"imageSourceReplacements"`
}

// SwitchStatus is a switch event
type SwitchStatus struct {
	Time     metav1.Time `json:"time"`
	OldImage string      `json:"oldImage"`
	NewImage string      `json:"newImage"`
	Target   Target      `json:"target"`
}

// AlternateImageSourceStatus defines the observed state of AlternateImageSource
type AlternateImageSourceStatus struct {
	// ObservedGeneration is the last observed generation of the object
	ObservedGeneration int64 `json:"observedGeneration"`
	// Switches is each occurence of an image switch
	Switches []SwitchStatus `json:"switches,omitempty"`
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
