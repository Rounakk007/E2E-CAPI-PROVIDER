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

// E2EClusterTemplateSpec defines the desired state of E2EClusterTemplate.
type E2EClusterTemplateSpec struct {
	Template E2EClusterTemplateResource `json:"template"`
}

// E2EClusterTemplateResource defines the template structure.
type E2EClusterTemplateResource struct {
	// +optional
	ObjectMeta clusterv1ObjectMeta `json:"metadata,omitempty"`
	Spec       E2EClusterSpec     `json:"spec"`
}

// clusterv1ObjectMeta is a subset of metav1.ObjectMeta used in templates.
type clusterv1ObjectMeta struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=e2eclustertemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// E2EClusterTemplate is the Schema for the e2eclustertemplates API.
type E2EClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec E2EClusterTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// E2EClusterTemplateList contains a list of E2EClusterTemplate.
type E2EClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []E2EClusterTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&E2EClusterTemplate{}, &E2EClusterTemplateList{})
}
