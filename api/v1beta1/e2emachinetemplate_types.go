/*
Copyright 2024 E2E Networks Ltd.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// E2EMachineTemplateSpec defines the desired state of E2EMachineTemplate.
type E2EMachineTemplateSpec struct {
	Template E2EMachineTemplateResource `json:"template"`
}

// E2EMachineTemplateResource defines the template structure.
type E2EMachineTemplateResource struct {
	// +optional
	ObjectMeta clusterv1ObjectMeta `json:"metadata,omitempty"`
	Spec       E2EMachineSpec     `json:"spec"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=e2emachinetemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// E2EMachineTemplate is the Schema for the e2emachinetemplates API.
type E2EMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec E2EMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// E2EMachineTemplateList contains a list of E2EMachineTemplate.
type E2EMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []E2EMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&E2EMachineTemplate{}, &E2EMachineTemplateList{})
}
