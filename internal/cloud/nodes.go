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

// Node represents an E2E Cloud compute node.
type Node struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Label           string `json:"label"`
	Plan            string `json:"plan"`
	Image           string `json:"image"`
	Region          string `json:"region"`
	Status          string `json:"status"`
	CreatedAt       string `json:"created_at"`
	PublicIPAddress  string `json:"public_ip_address"`
	PrivateIPAddress string `json:"private_ip_address"`
	Memory          string `json:"memory"`
	Disk            string `json:"disk"`
	Price           string `json:"price"`
	IsActive        bool   `json:"is_active"`
	VPCID           string `json:"vpc_id"`
}

// CreateNodeRequest is the request payload for creating a node.
type CreateNodeRequest struct {
	Name                 string   `json:"name"`
	Plan                 string   `json:"plan"`
	Image                string   `json:"image"`
	Region               string   `json:"region"`
	Label                string   `json:"label,omitempty"`
	SSHKeys              []string `json:"ssh_keys"`
	StartScripts         []string `json:"start_scripts"`
	Backups              bool     `json:"backups"`
	EnableBitninja       bool     `json:"enable_bitninja"`
	DisablePassword      bool     `json:"disable_password"`
	IsSavedImage         bool     `json:"is_saved_image"`
	SavedImageTemplateID *int     `json:"saved_image_template_id"`
	ReserveIP            string   `json:"reserve_ip"`
	IsIPv6Availed        bool     `json:"is_ipv6_availed"`
	VPCID                int      `json:"vpc_id,omitempty"`
	SubnetID             *string  `json:"subnet_id"`
	DefaultPublicIP      bool     `json:"default_public_ip"`
	IsPrivate            bool     `json:"is_private"`
	NgcContainerID       *int     `json:"ngc_container_id"`
	NumberOfInstances    int      `json:"number_of_instances"`
	SecurityGroupID      int      `json:"security_group_id,omitempty"`
	IsEncryptionEnabled  bool     `json:"isEncryptionEnabled"`

	// Location is not sent in the body — it's passed as a query parameter.
	Location string `json:"-"`
}

// NodeActionRequest is the request payload for performing an action on a node.
type NodeActionRequest struct {
	Type string `json:"type"`
}

// NodeActionResponse is the response from a node action.
type NodeActionResponse struct {
	ID         int    `json:"id"`
	CreatedAt  string `json:"created_at"`
	Status     string `json:"status"`
	ActionType string `json:"action_type"`
	ResourceID int    `json:"resource_id"`
}

// CreateNode creates a new compute node.
func (c *Client) CreateNode(ctx context.Context, req CreateNodeRequest) (*Node, error) {
	if req.NumberOfInstances == 0 {
		req.NumberOfInstances = 1
	}

	extra := map[string]string{}
	if req.Location != "" {
		extra["location"] = req.Location
	}

	data, err := c.doRequest(ctx, http.MethodPost, "/nodes/", req, extra)
	if err != nil {
		return nil, fmt.Errorf("creating node: %w", err)
	}

	var node Node
	if err := parseResponse(data, &node); err != nil {
		return nil, fmt.Errorf("parsing create node response: %w", err)
	}
	return &node, nil
}

// GetNode retrieves a node by ID.
func (c *Client) GetNode(ctx context.Context, nodeID int, location string) (*Node, error) {
	extra := map[string]string{}
	if location != "" {
		extra["location"] = location
	}

	data, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/nodes/%d/", nodeID), nil, extra)
	if err != nil {
		return nil, fmt.Errorf("getting node %d: %w", nodeID, err)
	}

	var node Node
	if err := parseResponse(data, &node); err != nil {
		return nil, fmt.Errorf("parsing get node response: %w", err)
	}
	return &node, nil
}

// ListNodes retrieves all nodes.
func (c *Client) ListNodes(ctx context.Context, location string, pageNo int, perPage int) ([]Node, error) {
	extra := map[string]string{}
	if location != "" {
		extra["location"] = location
	}
	if pageNo > 0 {
		extra["page_no"] = fmt.Sprintf("%d", pageNo)
	}
	if perPage > 0 {
		extra["per_page"] = fmt.Sprintf("%d", perPage)
	}

	data, err := c.doRequest(ctx, http.MethodGet, "/nodes/", nil, extra)
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}

	var nodes []Node
	if err := parseResponse(data, &nodes); err != nil {
		return nil, fmt.Errorf("parsing list nodes response: %w", err)
	}
	return nodes, nil
}

// DeleteNode deletes a node by ID.
func (c *Client) DeleteNode(ctx context.Context, nodeID int, location string) error {
	extra := map[string]string{
		"reserve_ip_required":      "",
		"reserve_ip_pool_required": "",
	}
	if location != "" {
		extra["location"] = location
	}

	_, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/nodes/%d/", nodeID), nil, extra)
	if err != nil {
		return fmt.Errorf("deleting node %d: %w", nodeID, err)
	}
	return nil
}

// PowerOnNode powers on a node.
func (c *Client) PowerOnNode(ctx context.Context, nodeID int) error {
	return c.performNodeAction(ctx, nodeID, "power_on")
}

// PowerOffNode powers off a node.
func (c *Client) PowerOffNode(ctx context.Context, nodeID int) error {
	return c.performNodeAction(ctx, nodeID, "power_off")
}

// RebootNode reboots a node.
func (c *Client) RebootNode(ctx context.Context, nodeID int) error {
	return c.performNodeAction(ctx, nodeID, "reboot")
}

// performNodeAction performs a power action on a node.
func (c *Client) performNodeAction(ctx context.Context, nodeID int, action string) error {
	req := NodeActionRequest{Type: action}
	_, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/nodes/%d/actions/", nodeID), req)
	if err != nil {
		return fmt.Errorf("performing %s on node %d: %w", action, nodeID, err)
	}
	return nil
}

// NodeIsRunning returns true if the node status indicates it is running.
func NodeIsRunning(node *Node) bool {
	return node.Status == "Running" || node.Status == "running"
}

// NodeIsCreating returns true if the node is still being created.
func NodeIsCreating(node *Node) bool {
	return node.Status == "Creating" || node.Status == "creating"
}
