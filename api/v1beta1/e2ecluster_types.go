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
	// ClusterFinalizer allows the controller to clean up resources before deletion.
	ClusterFinalizer = "e2ecluster.infrastructure.e2enetworks.com"
)

// E2EClusterSpec defines the desired state of E2ECluster.
type E2EClusterSpec struct {
	// Region is the E2E Cloud region for this cluster (e.g. "ncr", "chennai").
	Region string `json:"region"`

	// Location is the E2E Cloud location name (e.g. "Delhi", "Chennai").
	// Required for API calls as a query parameter.
	// +optional
	Location string `json:"location,omitempty"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`

	// Network contains network configuration for this cluster.
	// +optional
	Network E2ENetwork `json:"network,omitempty"`

	// LoadBalancer contains load balancer configuration for the API server.
	// +optional
	LoadBalancer E2ELoadBalancer `json:"loadBalancer,omitempty"`
}

// E2EClusterStatus defines the observed state of E2ECluster.
type E2EClusterStatus struct {
	// Ready indicates the infrastructure is fully provisioned.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// FailureReason indicates a fatal error during reconciliation.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage is a human-readable description of the failure.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Network contains the status of the cluster's network resources.
	// +optional
	Network E2ENetworkStatus `json:"network,omitempty"`

	// Conditions defines current service state of the E2ECluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// E2ENetworkStatus contains the status of network resources.
type E2ENetworkStatus struct {
	// VPCID is the ID of the VPC being used.
	// +optional
	VPCID string `json:"vpcID,omitempty"`

	// LoadBalancerID is the ID of the control plane load balancer.
	// +optional
	LoadBalancerID int `json:"loadBalancerID,omitempty"`

	// APIServerIP is the IP address of the API server load balancer.
	// +optional
	APIServerIP string `json:"apiServerIP,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=e2eclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster infrastructure is ready"
// +kubebuilder:printcolumn:name="Region",type="string",JSONPath=".spec.region",description="E2E Cloud region"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.controlPlaneEndpoint.host",description="API Server endpoint"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation"

// E2ECluster is the Schema for the e2eclusters API.
type E2ECluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   E2EClusterSpec   `json:"spec,omitempty"`
	Status E2EClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// E2EClusterList contains a list of E2ECluster.
type E2EClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []E2ECluster `json:"items"`
}

// GetConditions returns the set of conditions for this object.
func (c *E2ECluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (c *E2ECluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&E2ECluster{}, &E2EClusterList{})
}
