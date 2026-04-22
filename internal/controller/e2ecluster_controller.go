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
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

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
	apiServerPort    = 6443
	requeueAfterLong = 30 * time.Second
)

// E2EClusterReconciler reconciles an E2ECluster object.
type E2EClusterReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	E2EClient   *cloud.Client
}

// +kubebuilder:rbac:groups=infrastructure.e2enetworks.com,resources=e2eclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.e2enetworks.com,resources=e2eclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.e2enetworks.com,resources=e2eclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile handles E2ECluster reconciliation.
func (r *E2EClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, retErr error) {
	logger := log.FromContext(ctx)

	// Fetch the E2ECluster instance
	e2eCluster := &infrav1.E2ECluster{}
	if err := r.Get(ctx, req.NamespacedName, e2eCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the owning Cluster
	cluster, err := util.GetOwnerCluster(ctx, r.Client, e2eCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		logger.Info("Waiting for Cluster controller to set OwnerRef on E2ECluster")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("cluster", cluster.Name)

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(e2eCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch the E2ECluster when exiting this function
	defer func() {
		if err := patchHelper.Patch(
			ctx,
			e2eCluster,
			patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
				clusterv1.ReadyCondition,
				infrav1.LoadBalancerReadyCondition,
			}},
		); err != nil {
			if !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to patch E2ECluster")
				if retErr == nil {
					retErr = err
				}
			}
		}
	}()

	// Handle paused clusters
	if annotations.IsPaused(cluster, e2eCluster) {
		logger.Info("E2ECluster or owning Cluster is paused, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Handle deletion
	if !e2eCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, e2eCluster)
	}

	// Handle normal reconciliation
	return r.reconcileNormal(ctx, e2eCluster, cluster)
}

// reconcileNormal handles the normal (non-delete) reconciliation flow.
func (r *E2EClusterReconciler) reconcileNormal(ctx context.Context, e2eCluster *infrav1.E2ECluster, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(e2eCluster, infrav1.ClusterFinalizer) {
		controllerutil.AddFinalizer(e2eCluster, infrav1.ClusterFinalizer)
		return ctrl.Result{}, nil
	}

	// Reconcile the API server load balancer
	if err := r.reconcileLoadBalancer(ctx, e2eCluster); err != nil {
		conditions.MarkFalse(
			e2eCluster,
			infrav1.LoadBalancerReadyCondition,
			infrav1.LoadBalancerCreationFailedReason,
			clusterv1.ConditionSeverityError,
			err.Error(),
		)
		return ctrl.Result{}, fmt.Errorf("reconciling load balancer: %w", err)
	}

	// If the LB doesn't have an IP yet, requeue
	if e2eCluster.Status.Network.APIServerIP == "" {
		logger.Info("Waiting for load balancer IP to be assigned")
		return ctrl.Result{RequeueAfter: requeueAfterLong}, nil
	}

	conditions.MarkTrue(e2eCluster, infrav1.LoadBalancerReadyCondition)

	// Set the control plane endpoint
	e2eCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: e2eCluster.Status.Network.APIServerIP,
		Port: apiServerPort,
	}

	// Mark the cluster as ready
	e2eCluster.Status.Ready = true
	conditions.MarkTrue(e2eCluster, clusterv1.ReadyCondition)

	logger.Info("E2ECluster is ready",
		"endpoint", fmt.Sprintf("%s:%d", e2eCluster.Spec.ControlPlaneEndpoint.Host, e2eCluster.Spec.ControlPlaneEndpoint.Port),
	)

	return ctrl.Result{}, nil
}

// reconcileLoadBalancer ensures an API server load balancer exists for the cluster.
func (r *E2EClusterReconciler) reconcileLoadBalancer(ctx context.Context, e2eCluster *infrav1.E2ECluster) error {
	logger := log.FromContext(ctx)

	location := e2eCluster.Spec.Location

	// If an LB already exists, check its status
	if e2eCluster.Status.Network.LoadBalancerID != 0 {
		lb, err := r.E2EClient.GetLoadBalancer(ctx, e2eCluster.Status.Network.LoadBalancerID, location)
		if err != nil {
			if errors.Is(err, cloud.ErrNodeNotFound) || errors.Is(err, cloud.ErrLoadBalancerNotFound) {
				// LB was deleted externally, reset and recreate
				logger.Info("Load balancer not found, will recreate")
				e2eCluster.Status.Network.LoadBalancerID = 0
				e2eCluster.Status.Network.APIServerIP = ""
			} else {
				return err
			}
		} else {
			e2eCluster.Status.Network.APIServerIP = lb.GetPublicIP()
			return nil
		}
	}

	// Create a new load balancer
	lbName := fmt.Sprintf("%s-apiserver-lb", e2eCluster.Name)
	if e2eCluster.Spec.LoadBalancer.Name != "" {
		lbName = e2eCluster.Spec.LoadBalancer.Name
	}

	logger.Info("Creating API server load balancer", "name", lbName)

	apiServerPortStr := fmt.Sprintf("%d", apiServerPort)
	lb, err := r.E2EClient.CreateLoadBalancer(ctx, cloud.CreateLoadBalancerRequest{
		LBName:               lbName,
		LBType:               "external",
		LBMode:               "TCP",
		LBPort:               apiServerPortStr,
		PlanName:             "E2E-LB-2",
		NodeListType:         "D",
		ClientTimeout:        "60",
		ConnectionTimeout:    "60",
		ServerTimeout:        "60",
		HTTPKeepAliveTimeout: "60",
		SSLContext:           cloud.SSLContext{RedirectToHTTPS: false},
		Backends:             []interface{}{},
		VPCList:              []interface{}{},
		ACLList:              []interface{}{},
		ACLMap:               []interface{}{},
		TCPBackend: []cloud.TCPBackend{
			{
				Target:      "tcpNetworkMappingNode",
				BackendName: "apiserver-backend",
				Port:        apiServerPort,
				Balance:     "roundrobin",
				Servers:     []cloud.TCPBackendServer{},
			},
		},
		SecurityGroupID: e2eCluster.Spec.LoadBalancer.SecurityGroupID,
		Location:        location,
	})
	if err != nil {
		return fmt.Errorf("creating load balancer: %w", err)
	}

	e2eCluster.Status.Network.LoadBalancerID = lb.ID
	e2eCluster.Status.Network.APIServerIP = lb.IP

	logger.Info("Load balancer created", "id", lb.ID, "ip", lb.IP)
	return nil
}

// reconcileDelete handles the delete reconciliation flow.
func (r *E2EClusterReconciler) reconcileDelete(ctx context.Context, e2eCluster *infrav1.E2ECluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling E2ECluster deletion")

	// Delete the load balancer if it exists
	if e2eCluster.Status.Network.LoadBalancerID != 0 {
		logger.Info("Deleting API server load balancer", "id", e2eCluster.Status.Network.LoadBalancerID)

		// Drain all backend servers first — the E2E API may reject deletion on a non-empty LB.
		// This also handles cases where machine-side cleanup failed silently.
		if err := r.E2EClient.ClearBackendServers(ctx, e2eCluster.Status.Network.LoadBalancerID, e2eCluster.Spec.Location); err != nil {
			logger.V(1).Info("Could not clear LB backend servers before deletion", "error", err)
		}

		if err := r.E2EClient.DeleteLoadBalancer(ctx, e2eCluster.Status.Network.LoadBalancerID, e2eCluster.Spec.Location); err != nil {
			if !errors.Is(err, cloud.ErrLoadBalancerNotFound) && !errors.Is(err, cloud.ErrNodeNotFound) {
				conditions.MarkFalse(
					e2eCluster,
					infrav1.LoadBalancerReadyCondition,
					infrav1.LoadBalancerDeletionFailedReason,
					clusterv1.ConditionSeverityWarning,
					err.Error(),
				)
				return ctrl.Result{}, fmt.Errorf("deleting load balancer: %w", err)
			}
		}
		logger.Info("Load balancer deleted")
	}

	// Remove the finalizer
	controllerutil.RemoveFinalizer(e2eCluster, infrav1.ClusterFinalizer)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *E2EClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.E2ECluster{}).
		WithOptions(controller.Options{}).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(
				ctx,
				infrav1.GroupVersion.WithKind("E2ECluster"),
				mgr.GetClient(),
				&infrav1.E2ECluster{},
			)),
		).
		WithEventFilter(predicates.ResourceNotPaused(log.FromContext(ctx))).
		Complete(r)
}
