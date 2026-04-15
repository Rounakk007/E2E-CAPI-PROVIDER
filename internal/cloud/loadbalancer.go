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

package cloud

import (
	"context"
	"fmt"
	"net/http"
)

// LoadBalancer represents an E2E Cloud load balancer.
type LoadBalancer struct {
	ID                 int                `json:"id"`
	Name               string             `json:"name"`
	Status             string             `json:"status"`
	IPAddress          string             `json:"ip_address"`
	Region             string             `json:"region"`
	VPCID              string             `json:"vpc_id"`
	BackendNodes       []BackendNode      `json:"backend_nodes,omitempty"`
	Listeners          []Listener         `json:"listeners,omitempty"`
	HealthCheck        *HealthCheck       `json:"health_check,omitempty"`
}

// BackendNode represents a backend node in a load balancer.
type BackendNode struct {
	NodeID int `json:"node_id"`
	Port   int `json:"port"`
}

// Listener represents a load balancer listener.
type Listener struct {
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	TargetPort int  `json:"target_port"`
}

// HealthCheck defines load balancer health check configuration.
type HealthCheck struct {
	Protocol           string `json:"protocol"`
	Port               int    `json:"port"`
	Path               string `json:"path,omitempty"`
	CheckInterval      int    `json:"check_interval"`
	ResponseTimeout    int    `json:"response_timeout"`
	UnhealthyThreshold int    `json:"unhealthy_threshold"`
	HealthyThreshold   int    `json:"healthy_threshold"`
}

// CreateLoadBalancerRequest is the request payload for creating a load balancer.
type CreateLoadBalancerRequest struct {
	Name        string       `json:"name"`
	Region      string       `json:"region"`
	VPCID       string       `json:"vpc_id,omitempty"`
	Listeners   []Listener   `json:"listeners"`
	HealthCheck *HealthCheck `json:"health_check,omitempty"`
}

// CreateLoadBalancer creates a new load balancer for the API server.
func (c *Client) CreateLoadBalancer(ctx context.Context, req CreateLoadBalancerRequest) (*LoadBalancer, error) {
	data, err := c.doRequest(ctx, http.MethodPost, "/loadbalancers/", req)
	if err != nil {
		return nil, fmt.Errorf("creating load balancer: %w", err)
	}

	var lb LoadBalancer
	if err := parseResponse(data, &lb); err != nil {
		return nil, fmt.Errorf("parsing create load balancer response: %w", err)
	}
	return &lb, nil
}

// GetLoadBalancer retrieves a load balancer by ID.
func (c *Client) GetLoadBalancer(ctx context.Context, lbID int) (*LoadBalancer, error) {
	data, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/loadbalancers/%d/", lbID), nil)
	if err != nil {
		return nil, fmt.Errorf("getting load balancer %d: %w", lbID, err)
	}

	var lb LoadBalancer
	if err := parseResponse(data, &lb); err != nil {
		return nil, fmt.Errorf("parsing get load balancer response: %w", err)
	}
	return &lb, nil
}

// DeleteLoadBalancer deletes a load balancer by ID.
func (c *Client) DeleteLoadBalancer(ctx context.Context, lbID int) error {
	_, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/loadbalancers/%d/", lbID), nil)
	if err != nil {
		return fmt.Errorf("deleting load balancer %d: %w", lbID, err)
	}
	return nil
}

// AddBackendNode adds a backend node to a load balancer.
func (c *Client) AddBackendNode(ctx context.Context, lbID int, nodeID int, port int) error {
	req := BackendNode{NodeID: nodeID, Port: port}
	_, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/loadbalancers/%d/backends/", lbID), req)
	if err != nil {
		return fmt.Errorf("adding backend node %d to lb %d: %w", nodeID, lbID, err)
	}
	return nil
}

// RemoveBackendNode removes a backend node from a load balancer.
func (c *Client) RemoveBackendNode(ctx context.Context, lbID int, nodeID int) error {
	_, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/loadbalancers/%d/backends/%d/", lbID, nodeID), nil)
	if err != nil {
		return fmt.Errorf("removing backend node %d from lb %d: %w", nodeID, lbID, err)
	}
	return nil
}
