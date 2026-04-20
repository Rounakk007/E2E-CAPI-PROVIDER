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

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

// E2ECluster conditions.
const (
	// LoadBalancerReadyCondition reports on the status of the load balancer.
	LoadBalancerReadyCondition clusterv1.ConditionType = "LoadBalancerReady"

	// LoadBalancerCreationFailedReason indicates the load balancer failed to create.
	LoadBalancerCreationFailedReason = "LoadBalancerCreationFailed"

	// LoadBalancerDeletionFailedReason indicates the load balancer failed to delete.
	LoadBalancerDeletionFailedReason = "LoadBalancerDeletionFailed"
)

// E2EMachine conditions.
const (
	// InstanceReadyCondition reports on the status of the E2E compute node.
	InstanceReadyCondition clusterv1.ConditionType = "InstanceReady"

	// InstanceProvisioningReason indicates the instance is being provisioned.
	InstanceProvisioningReason = "InstanceProvisioning"

	// InstanceProvisionFailedReason indicates instance creation failed.
	InstanceProvisionFailedReason = "InstanceProvisionFailed"

	// InstanceTerminatedReason indicates the instance was terminated.
	InstanceTerminatedReason = "InstanceTerminated"

	// InstanceNotFoundReason indicates the instance was not found.
	InstanceNotFoundReason = "InstanceNotFound"

	// InstanceStoppedReason indicates the instance is stopped.
	InstanceStoppedReason = "InstanceStopped"

	// BootstrapDataNotReadyReason indicates bootstrap data is not yet available.
	BootstrapDataNotReadyReason = "BootstrapDataNotReady"

	// BootstrapSucceededReason indicates bootstrap was applied via SSH.
	BootstrapSucceededReason = "BootstrapSucceeded"

	// BootstrapFailedReason indicates bootstrap via SSH failed.
	BootstrapFailedReason = "BootstrapFailed"

	// WaitingForSSHReason indicates the node is running but SSH is not ready yet.
	WaitingForSSHReason = "WaitingForSSH"
)
