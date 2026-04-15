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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// MachineFinalizer allows the controller to clean up resources before deletion.
	MachineFinalizer = "e2emachine.infrastructure.e2enetworks.com"
)

// E2EMachineSpec defines the desired state of E2EMachine.
type E2EMachineSpec struct {
	// ProviderID is the unique identifier as specified by the cloud provider.
	// Format: e2e://<region>/<node-id>
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Plan is the E2E Cloud compute plan/SKU (e.g. "C2-12GB", "C3.8GB", "GPU-A100").
	Plan string `json:"plan"`

	// Image is the OS image name (e.g. "Ubuntu-22.04", "CentOS-7").
	Image string `json:"image"`

	// Region is the E2E Cloud region for this machine (e.g. "ncr", "chennai").
	// If empty, defaults to the cluster's region.
	// +optional
	Region string `json:"region,omitempty"`

	// SSHKeyName is the name of the SSH key to inject into this node.
	// If empty, defaults to the cluster's SSH key.
	// +optional
	SSHKeyName string `json:"sshKeyName,omitempty"`

	// VPCID is the VPC to place this node in.
	// If empty, defaults to the cluster's VPC.
	// +optional
	VPCID string `json:"vpcID,omitempty"`

	// SecurityGroupIDs is a list of security group IDs to attach.
	// +optional
	SecurityGroupIDs []int `json:"securityGroupIDs,omitempty"`

	// EnablePublicIP controls whether a public IP is assigned.
	// +optional
	EnablePublicIP *bool `json:"enablePublicIP,omitempty"`

	// EnableIPv6 enables IPv6 on the node.
	// +optional
	EnableIPv6 bool `json:"enableIPv6,omitempty"`

	// EnableBackup enables backup on the node.
	// +optional
	EnableBackup bool `json:"enableBackup,omitempty"`
}

// E2EMachineStatus defines the observed state of E2EMachine.
type E2EMachineStatus struct {
	// Ready indicates the infrastructure is fully provisioned.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// InstanceID is the E2E Cloud node ID.
	// +optional
	InstanceID *int `json:"instanceID,omitempty"`

	// InstanceStatus is the current status of the E2E node.
	// +optional
	InstanceStatus string `json:"instanceStatus,omitempty"`

	// Addresses contains the addresses of the machine.
	// +optional
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// FailureReason indicates a fatal error during reconciliation.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage is a human-readable description of the failure.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the E2EMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=e2emachines,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine infrastructure is ready"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="E2E Cloud provider ID"
// +kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object that owns this resource"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation"

// E2EMachine is the Schema for the e2emachines API.
type E2EMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   E2EMachineSpec   `json:"spec,omitempty"`
	Status E2EMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// E2EMachineList contains a list of E2EMachine.
type E2EMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []E2EMachine `json:"items"`
}

// GetConditions returns the set of conditions for this object.
func (m *E2EMachine) GetConditions() clusterv1.Conditions {
	return m.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (m *E2EMachine) SetConditions(conditions clusterv1.Conditions) {
	m.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&E2EMachine{}, &E2EMachineList{})
}
