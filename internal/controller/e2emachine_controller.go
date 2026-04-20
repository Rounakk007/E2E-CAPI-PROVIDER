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

package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ctrl "sigs.k8s.io/controller-runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"

	infrav1 "github.com/e2enetworks/cluster-api-provider-e2e/api/v1beta1"
	"github.com/e2enetworks/cluster-api-provider-e2e/internal/cloud"
)

const (
	requeueAfterShort = 15 * time.Second
	sshPort           = 22
	sshUser           = "root"
)

// E2EMachineReconciler reconciles an E2EMachine object.
type E2EMachineReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	E2EClient *cloud.Client
	SSHKey    *SSHKeyPair
}

// +kubebuilder:rbac:groups=infrastructure.e2enetworks.com,resources=e2emachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.e2enetworks.com,resources=e2emachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.e2enetworks.com,resources=e2emachines/finalizers,verbs=update
// +kubebuilder:rbac:groups=infrastructure.e2enetworks.com,resources=e2eclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile handles E2EMachine reconciliation.
func (r *E2EMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, retErr error) {
	logger := log.FromContext(ctx)

	// Fetch the E2EMachine instance
	e2eMachine := &infrav1.E2EMachine{}
	if err := r.Get(ctx, req.NamespacedName, e2eMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the owning Machine
	machine, err := util.GetOwnerMachine(ctx, r.Client, e2eMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		logger.Info("Waiting for Machine controller to set OwnerRef on E2EMachine")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("machine", machine.Name)

	// Fetch the Cluster
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Info("E2EMachine owner Machine is missing Cluster label or Cluster does not exist")
		return ctrl.Result{}, err
	}

	logger = logger.WithValues("cluster", cluster.Name)

	// Fetch the E2ECluster
	e2eCluster := &infrav1.E2ECluster{}
	e2eClusterName := types.NamespacedName{
		Namespace: e2eMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Get(ctx, e2eClusterName, e2eCluster); err != nil {
		logger.Info("E2ECluster not yet available")
		return ctrl.Result{}, nil
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(e2eMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch the E2EMachine when exiting this function
	defer func() {
		if err := patchHelper.Patch(
			ctx,
			e2eMachine,
			patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
				clusterv1.ReadyCondition,
				infrav1.InstanceReadyCondition,
			}},
		); err != nil {
			logger.Error(err, "failed to patch E2EMachine")
			if retErr == nil {
				retErr = err
			}
		}
	}()

	// Handle paused clusters
	if annotations.IsPaused(cluster, e2eMachine) {
		logger.Info("E2EMachine or owning Cluster is paused, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Handle deletion
	if !e2eMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, e2eMachine, e2eCluster)
	}

	// Handle normal reconciliation
	return r.reconcileNormal(ctx, e2eMachine, machine, e2eCluster, cluster)
}

// reconcileNormal handles the normal (non-delete) reconciliation flow.
func (r *E2EMachineReconciler) reconcileNormal(
	ctx context.Context,
	e2eMachine *infrav1.E2EMachine,
	machine *clusterv1.Machine,
	e2eCluster *infrav1.E2ECluster,
	cluster *clusterv1.Cluster,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(e2eMachine, infrav1.MachineFinalizer) {
		controllerutil.AddFinalizer(e2eMachine, infrav1.MachineFinalizer)
		return ctrl.Result{}, nil
	}

	// Wait for the infrastructure cluster to be ready
	if !e2eCluster.Status.Ready {
		logger.Info("Waiting for E2ECluster to be ready")
		conditions.MarkFalse(
			e2eMachine,
			infrav1.InstanceReadyCondition,
			infrav1.InstanceProvisioningReason,
			clusterv1.ConditionSeverityInfo,
			"Waiting for E2ECluster to be ready",
		)
		return ctrl.Result{}, nil
	}

	// If we already have an instance, check its status
	if e2eMachine.Status.InstanceID != nil {
		return r.reconcileInstanceStatus(ctx, e2eMachine, machine, e2eCluster)
	}

	// Create a new instance (without bootstrap data — we'll SSH it in later)
	return r.createInstance(ctx, e2eMachine, machine, e2eCluster)
}

// createInstance creates a new E2E compute node for the machine.
func (r *E2EMachineReconciler) createInstance(
	ctx context.Context,
	e2eMachine *infrav1.E2EMachine,
	machine *clusterv1.Machine,
	e2eCluster *infrav1.E2ECluster,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Resolve region: machine-level override or cluster default
	region := e2eMachine.Spec.Region
	if region == "" {
		region = e2eCluster.Spec.Region
	}

	// Resolve VPC — the API expects vpc_id as an integer
	vpcIDStr := e2eMachine.Spec.VPCID
	if vpcIDStr == "" {
		vpcIDStr = e2eCluster.Spec.Network.VPCID
	}
	var vpcID int
	if vpcIDStr != "" {
		vpcID, _ = strconv.Atoi(vpcIDStr)
	}

	// Build SSH keys list: user keys + provider key
	sshKeys := make([]string, 0, len(e2eMachine.Spec.SSHKeys)+1)
	sshKeys = append(sshKeys, e2eMachine.Spec.SSHKeys...)
	if r.SSHKey != nil {
		sshKeys = append(sshKeys, r.SSHKey.PublicKey)
	}

	// Determine public IP setting
	defaultPublicIP := true
	if e2eMachine.Spec.EnablePublicIP != nil {
		defaultPublicIP = *e2eMachine.Spec.EnablePublicIP
	}

	// Resolve security group
	securityGroupID := 0
	if len(e2eMachine.Spec.SecurityGroupIDs) > 0 {
		securityGroupID = e2eMachine.Spec.SecurityGroupIDs[0]
	}

	logger.Info("Creating E2E compute node",
		"plan", e2eMachine.Spec.Plan,
		"image", e2eMachine.Spec.Image,
		"region", region,
	)

	conditions.MarkFalse(
		e2eMachine,
		infrav1.InstanceReadyCondition,
		infrav1.InstanceProvisioningReason,
		clusterv1.ConditionSeverityInfo,
		"Creating E2E compute node",
	)

	// When VPC is set, subnet_id should be null; otherwise empty string
	var subnetID *string
	if vpcID == 0 {
		empty := ""
		subnetID = &empty
	}

	node, err := r.E2EClient.CreateNode(ctx, cloud.CreateNodeRequest{
		Name:              e2eMachine.Name,
		Plan:              e2eMachine.Spec.Plan,
		Image:             e2eMachine.Spec.Image,
		Region:            region,
		Location:          e2eMachine.Spec.Location,
		Label:             e2eMachine.Namespace,
		SSHKeys:           sshKeys,
		StartScripts:      []string{},
		DisablePassword:   true,
		NumberOfInstances: 1,
		DefaultPublicIP:   defaultPublicIP,
		IsIPv6Availed:     e2eMachine.Spec.EnableIPv6,
		Backups:           e2eMachine.Spec.EnableBackup,
		VPCID:             vpcID,
		SubnetID:          subnetID,
		SecurityGroupID:   securityGroupID,
	})
	if err != nil {
		conditions.MarkFalse(
			e2eMachine,
			infrav1.InstanceReadyCondition,
			infrav1.InstanceProvisionFailedReason,
			clusterv1.ConditionSeverityError,
			err.Error(),
		)
		return ctrl.Result{}, fmt.Errorf("creating E2E node: %w", err)
	}

	logger.Info("E2E compute node created", "nodeID", node.ID, "status", node.Status)

	e2eMachine.Status.InstanceID = &node.ID
	e2eMachine.Status.InstanceStatus = node.Status

	// Node is still provisioning, requeue to check status
	return ctrl.Result{RequeueAfter: requeueAfterLong}, nil
}

// reconcileInstanceStatus checks and updates the status of an existing instance.
func (r *E2EMachineReconciler) reconcileInstanceStatus(
	ctx context.Context,
	e2eMachine *infrav1.E2EMachine,
	machine *clusterv1.Machine,
	e2eCluster *infrav1.E2ECluster,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	nodeID := *e2eMachine.Status.InstanceID

	node, err := r.E2EClient.GetNode(ctx, nodeID, e2eMachine.Spec.Location)
	if err != nil {
		if errors.Is(err, cloud.ErrNodeNotFound) {
			logger.Info("E2E node not found, instance may have been deleted externally", "nodeID", nodeID)
			conditions.MarkFalse(
				e2eMachine,
				infrav1.InstanceReadyCondition,
				infrav1.InstanceNotFoundReason,
				clusterv1.ConditionSeverityError,
				"Instance not found",
			)
			reason := "instance not found"
			e2eMachine.Status.FailureReason = &reason
			e2eMachine.Status.FailureMessage = &reason
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting node %d: %w", nodeID, err)
	}

	e2eMachine.Status.InstanceStatus = node.Status

	// Update addresses
	e2eMachine.Status.Addresses = []clusterv1.MachineAddress{}
	if node.PublicIPAddress != "" {
		e2eMachine.Status.Addresses = append(e2eMachine.Status.Addresses, clusterv1.MachineAddress{
			Type:    clusterv1.MachineExternalIP,
			Address: node.PublicIPAddress,
		})
	}
	if node.PrivateIPAddress != "" {
		e2eMachine.Status.Addresses = append(e2eMachine.Status.Addresses, clusterv1.MachineAddress{
			Type:    clusterv1.MachineInternalIP,
			Address: node.PrivateIPAddress,
		})
	}

	// If node is still creating, requeue
	if cloud.NodeIsCreating(node) {
		logger.Info("E2E node is still provisioning", "nodeID", nodeID, "status", node.Status)
		conditions.MarkFalse(
			e2eMachine,
			infrav1.InstanceReadyCondition,
			infrav1.InstanceProvisioningReason,
			clusterv1.ConditionSeverityInfo,
			"Instance is provisioning",
		)
		return ctrl.Result{RequeueAfter: requeueAfterLong}, nil
	}

	// If node is running, bootstrap via SSH then mark as ready
	if cloud.NodeIsRunning(node) {
		logger.Info("E2E node is running",
			"nodeID", nodeID,
			"publicIP", node.PublicIPAddress,
			"privateIP", node.PrivateIPAddress,
		)

		// Step 1: Run bootstrap script via SSH if not already done
		if !e2eMachine.Status.Bootstrapped {
			// Need bootstrap data to be available
			if machine.Spec.Bootstrap.DataSecretName == nil {
				logger.Info("Waiting for bootstrap data to be available")
				conditions.MarkFalse(
					e2eMachine,
					infrav1.InstanceReadyCondition,
					infrav1.BootstrapDataNotReadyReason,
					clusterv1.ConditionSeverityInfo,
					"Waiting for bootstrap data",
				)
				return ctrl.Result{RequeueAfter: requeueAfterShort}, nil
			}

			// Need a public IP to SSH into
			if node.PublicIPAddress == "" {
				logger.Info("Waiting for node to get a public IP")
				return ctrl.Result{RequeueAfter: requeueAfterShort}, nil
			}

			// Check if SSH is reachable
			sshClient := cloud.NewSSHClient(r.SSHKey.PrivateKey)
			if !sshClient.IsSSHReady(node.PublicIPAddress, sshPort) {
				logger.Info("Waiting for SSH to become available", "host", node.PublicIPAddress)
				conditions.MarkFalse(
					e2eMachine,
					infrav1.InstanceReadyCondition,
					infrav1.WaitingForSSHReason,
					clusterv1.ConditionSeverityInfo,
					"Waiting for SSH to become available",
				)
				return ctrl.Result{RequeueAfter: requeueAfterShort}, nil
			}

			// Get the bootstrap data
			bootstrapData, err := r.getBootstrapData(ctx, machine)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("getting bootstrap data: %w", err)
			}

			// Write the bootstrap script to the node and execute it
			// The bootstrap data is base64 encoded cloud-init, we decode and run it
			logger.Info("Executing bootstrap script via SSH", "host", node.PublicIPAddress)
			cmd := fmt.Sprintf("echo '%s' | base64 -d > /tmp/bootstrap.sh && chmod +x /tmp/bootstrap.sh && /tmp/bootstrap.sh", bootstrapData)
			output, err := sshClient.RunCommand(node.PublicIPAddress, sshPort, sshUser, cmd)
			if err != nil {
				logger.Error(err, "Bootstrap script failed", "output", output)
				conditions.MarkFalse(
					e2eMachine,
					infrav1.InstanceReadyCondition,
					infrav1.BootstrapFailedReason,
					clusterv1.ConditionSeverityError,
					fmt.Sprintf("Bootstrap failed: %v", err),
				)
				// Requeue to retry — the script may fail transiently
				return ctrl.Result{RequeueAfter: requeueAfterLong}, nil
			}

			logger.Info("Bootstrap script completed successfully")
			e2eMachine.Status.Bootstrapped = true
		}

		// Step 2: Set the provider ID and mark ready
		providerID := fmt.Sprintf("e2e://%s/%d", e2eCluster.Spec.Region, node.ID)
		e2eMachine.Spec.ProviderID = &providerID

		e2eMachine.Status.Ready = true
		conditions.MarkTrue(e2eMachine, infrav1.InstanceReadyCondition)
		conditions.MarkTrue(e2eMachine, clusterv1.ReadyCondition)

		logger.Info("E2E machine is ready",
			"nodeID", nodeID,
			"providerID", providerID,
		)

		// Step 3: Register with the load balancer if this is a control plane node
		if util.IsControlPlaneMachine(r.machineFromE2EMachine(ctx, e2eMachine)) {
			if e2eCluster.Status.Network.LoadBalancerID != 0 && node.PrivateIPAddress != "" {
				if err := r.E2EClient.AddBackendServer(
					ctx,
					e2eCluster.Status.Network.LoadBalancerID,
					cloud.TCPBackendServer{
						Target:      "servers",
						BackendName: node.Name,
						BackendIP:   node.PrivateIPAddress,
						BackendPort: apiServerPort,
					},
					e2eMachine.Spec.Location,
				); err != nil {
					logger.Error(err, "Failed to register node with load balancer, will retry")
					return ctrl.Result{RequeueAfter: requeueAfterShort}, nil
				}
				logger.Info("Registered control plane node with API server load balancer")
			}
		}

		return ctrl.Result{}, nil
	}

	// Node is in an unexpected state
	logger.Info("E2E node is in unexpected state", "nodeID", nodeID, "status", node.Status)
	conditions.MarkFalse(
		e2eMachine,
		infrav1.InstanceReadyCondition,
		infrav1.InstanceStoppedReason,
		clusterv1.ConditionSeverityWarning,
		fmt.Sprintf("Instance is in state: %s", node.Status),
	)
	return ctrl.Result{RequeueAfter: requeueAfterLong}, nil
}

// reconcileDelete handles the delete reconciliation flow.
func (r *E2EMachineReconciler) reconcileDelete(
	ctx context.Context,
	e2eMachine *infrav1.E2EMachine,
	e2eCluster *infrav1.E2ECluster,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling E2EMachine deletion")

	if e2eMachine.Status.InstanceID != nil {
		nodeID := *e2eMachine.Status.InstanceID

		// Deregister from load balancer if it's a control plane node
		if e2eCluster.Status.Network.LoadBalancerID != 0 {
			// Find the node's private IP from the machine's stored addresses
			var privateIP string
			for _, addr := range e2eMachine.Status.Addresses {
				if addr.Type == clusterv1.MachineInternalIP {
					privateIP = addr.Address
					break
				}
			}
			if privateIP != "" {
				if err := r.E2EClient.RemoveBackendServer(
					ctx,
					e2eCluster.Status.Network.LoadBalancerID,
					privateIP,
					e2eMachine.Spec.Location,
				); err != nil {
					// Log but don't fail — the LB may already be gone
					logger.V(1).Info("Could not deregister node from load balancer", "error", err)
				}
			}
		}

		// Delete the node
		logger.Info("Deleting E2E compute node", "nodeID", nodeID)
		if err := r.E2EClient.DeleteNode(ctx, nodeID, e2eMachine.Spec.Location); err != nil {
			if !errors.Is(err, cloud.ErrNodeNotFound) {
				return ctrl.Result{}, fmt.Errorf("deleting E2E node %d: %w", nodeID, err)
			}
			logger.Info("E2E node already deleted", "nodeID", nodeID)
		} else {
			logger.Info("E2E compute node deleted", "nodeID", nodeID)
		}
	}

	// Remove the finalizer
	controllerutil.RemoveFinalizer(e2eMachine, infrav1.MachineFinalizer)

	return ctrl.Result{}, nil
}

// getBootstrapData fetches the bootstrap data secret and returns its content.
func (r *E2EMachineReconciler) getBootstrapData(ctx context.Context, machine *clusterv1.Machine) (string, error) {
	if machine.Spec.Bootstrap.DataSecretName == nil {
		return "", fmt.Errorf("bootstrap data secret name is nil")
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Namespace: machine.Namespace,
		Name:      *machine.Spec.Bootstrap.DataSecretName,
	}
	if err := r.Get(ctx, key, secret); err != nil {
		return "", fmt.Errorf("fetching bootstrap data secret %s/%s: %w", key.Namespace, key.Name, err)
	}

	value, ok := secret.Data["value"]
	if !ok {
		return "", fmt.Errorf("bootstrap data secret %s/%s has no 'value' key", key.Namespace, key.Name)
	}

	return base64.StdEncoding.EncodeToString(value), nil
}

// machineFromE2EMachine fetches the owning Machine for an E2EMachine.
func (r *E2EMachineReconciler) machineFromE2EMachine(ctx context.Context, e2eMachine *infrav1.E2EMachine) *clusterv1.Machine {
	machine, _ := util.GetOwnerMachine(ctx, r.Client, e2eMachine.ObjectMeta)
	return machine
}

// SetupWithManager sets up the controller with the Manager.
func (r *E2EMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.E2EMachine{}).
		WithOptions(controller.Options{}).
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(
				clusterv1.GroupVersion.WithKind("Machine"),
			)),
		).
		Watches(
			&infrav1.E2ECluster{},
			handler.EnqueueRequestsFromMapFunc(r.e2eClusterToE2EMachines(ctx)),
		).
		WithEventFilter(predicates.ResourceNotPaused(log.FromContext(ctx))).
		Complete(r)
}

// e2eClusterToE2EMachines maps E2ECluster events to E2EMachine reconciliations.
func (r *E2EMachineReconciler) e2eClusterToE2EMachines(ctx context.Context) handler.MapFunc {
	return func(ctx context.Context, o client.Object) []reconcile.Request {
		logger := log.FromContext(ctx)

		e2eCluster, ok := o.(*infrav1.E2ECluster)
		if !ok {
			logger.Error(fmt.Errorf("expected E2ECluster, got %T", o), "failed to map E2ECluster to E2EMachines")
			return nil
		}

		// Get the owning cluster
		cluster, err := util.GetOwnerCluster(ctx, r.Client, e2eCluster.ObjectMeta)
		if err != nil || cluster == nil {
			return nil
		}

		// List all E2EMachines in the same namespace with the matching cluster label
		machineList := &infrav1.E2EMachineList{}
		if err := r.List(ctx, machineList,
			client.InNamespace(e2eCluster.Namespace),
			client.MatchingLabels{clusterv1.ClusterNameLabel: cluster.Name},
		); err != nil {
			return nil
		}

		requests := make([]reconcile.Request, len(machineList.Items))
		for i, m := range machineList.Items {
			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: m.Namespace,
					Name:      m.Name,
				},
			}
		}
		return requests
	}
}
