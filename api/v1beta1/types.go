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

// E2EResourceReference is a reference to a specific E2E resource by ID.
type E2EResourceReference struct {
	// ID is the E2E resource identifier.
	ID int `json:"id"`
}

// E2ENetwork contains network configuration for E2E infrastructure.
type E2ENetwork struct {
	// VPCID is the ID of the VPC to use.
	// +optional
	VPCID string `json:"vpcID,omitempty"`
}

// E2ELoadBalancer contains load balancer configuration.
type E2ELoadBalancer struct {
	// ID is the load balancer ID once created.
	// +optional
	ID int `json:"id,omitempty"`

	// Name is a custom name for the load balancer.
	// +optional
	Name string `json:"name,omitempty"`

	// SecurityGroupID is the security group to assign to the load balancer.
	// +optional
	SecurityGroupID int `json:"securityGroupID,omitempty"`
}
