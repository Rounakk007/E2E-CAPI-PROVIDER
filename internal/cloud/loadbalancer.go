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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// CreateLoadBalancerResponse is the response from the LB create API.
type CreateLoadBalancerResponse struct {
	ApplianceID int    `json:"appliance_id"`
	ID          int    `json:"id"`
	IP          string `json:"IP"`
}

// LoadBalancerNodeDetail contains the node info of an LB appliance.
type LoadBalancerNodeDetail struct {
	NodeID    int    `json:"node_id"`
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
	PlanName  string `json:"plan_name"`
}

// LoadBalancerContext contains the config from appliance_instance[].context.
type LoadBalancerContext struct {
	HostTarget      string       `json:"host_target"`
	ManagementIP    string       `json:"management_ip"`
	LBName          string       `json:"lb_name"`
	LBType          string       `json:"lb_type"`
	LBMode          string       `json:"lb_mode"`
	LBPort          string       `json:"lb_port"`
	PlanName        string       `json:"plan_name"`
	SecurityGroupID int          `json:"security_group_id"`
	TCPBackend      []TCPBackend `json:"tcp_backend"`
}

// LoadBalancerInstance represents one entry in appliance_instance[].
type LoadBalancerInstance struct {
	ID      string              `json:"id"`
	Context LoadBalancerContext `json:"context"`
}

// LoadBalancer represents an E2E Cloud load balancer from the list API response.
type LoadBalancer struct {
	ID                int                    `json:"id"`
	Name              string                 `json:"name"`
	Status            string                 `json:"status"`
	NodeDetail        LoadBalancerNodeDetail `json:"node_detail"`
	ApplianceInstance []LoadBalancerInstance  `json:"appliance_instance"`
}

// GetPublicIP returns the public IP of the load balancer.
func (lb *LoadBalancer) GetPublicIP() string {
	if lb.NodeDetail.PublicIP != "" {
		return lb.NodeDetail.PublicIP
	}
	if len(lb.ApplianceInstance) > 0 {
		return lb.ApplianceInstance[0].Context.HostTarget
	}
	return ""
}

// SSLContext defines SSL configuration for a load balancer.
type SSLContext struct {
	SSLCertificateID *int `json:"ssl_certificate_id"`
	RedirectToHTTPS  bool `json:"redirect_to_https"`
}

// TCPBackendServer represents a server in a TCP backend group.
type TCPBackendServer struct {
	Target      string `json:"target"`
	BackendName string `json:"backend_name"`
	BackendIP   string `json:"backend_ip"`
	BackendPort int    `json:"backend_port"`
}

// TCPBackend represents a TCP backend group in a load balancer.
type TCPBackend struct {
	Target          string             `json:"target,omitempty"`
	BackendName     string             `json:"backend_name"`
	Port            int                `json:"port"`
	Balance         string             `json:"balance"`
	Servers         []TCPBackendServer `json:"servers"`
	SecurityGroupID int                `json:"security_group_id,omitempty"`
}

// CreateLoadBalancerRequest is the request payload for creating a load balancer.
type CreateLoadBalancerRequest struct {
	ClientTimeout        string        `json:"client_timeout"`
	ConnectionTimeout    string        `json:"connection_timeout"`
	ServerTimeout        string        `json:"server_timeout"`
	HTTPKeepAliveTimeout string        `json:"http_keep_alive_timeout"`
	PlanName             string        `json:"plan_name"`
	LBName               string        `json:"lb_name"`
	LBType               string        `json:"lb_type"`
	LBMode               string        `json:"lb_mode"`
	LBPort               string        `json:"lb_port"`
	NodeListType         string        `json:"node_list_type"`
	CheckboxEnable       string        `json:"checkbox_enable"`
	LBReserveIP          string        `json:"lb_reserve_ip"`
	SSLCertificateID     *int          `json:"ssl_certificate_id"`
	DefaultBackend       string        `json:"default_backend"`
	IsIPv6Attached       bool          `json:"is_ipv6_attached"`
	EnableBitninja       bool          `json:"enable_bitninja"`
	SSLContext           SSLContext     `json:"ssl_context"`
	Backends             []interface{} `json:"backends"`
	VPCList              []interface{} `json:"vpc_list"`
	TCPBackend           []TCPBackend  `json:"tcp_backend"`
	SecurityGroupID      int           `json:"security_group_id,omitempty"`
	ACLList              []interface{} `json:"acl_list"`
	ACLMap               []interface{} `json:"acl_map"`

	// Location is passed as a query parameter, not in the body.
	Location string `json:"-"`
}

// CreateLoadBalancer creates a new load balancer.
// POST /appliances/load-balancers/
func (c *Client) CreateLoadBalancer(ctx context.Context, req CreateLoadBalancerRequest) (*CreateLoadBalancerResponse, error) {
	extra := map[string]string{}
	if req.Location != "" {
		extra["location"] = req.Location
	}

	data, err := c.doRequest(ctx, http.MethodPost, "/appliances/load-balancers/", req, extra)
	if err != nil {
		return nil, fmt.Errorf("creating load balancer: %w", err)
	}

	var resp CreateLoadBalancerResponse
	if err := parseResponse(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing create load balancer response: %w", err)
	}
	return &resp, nil
}

// GetLoadBalancer retrieves a load balancer by ID from the appliances list.
// GET /appliances/
func (c *Client) GetLoadBalancer(ctx context.Context, lbID int, location string) (*LoadBalancer, error) {
	extra := map[string]string{}
	if location != "" {
		extra["location"] = location
	}

	data, err := c.doRequest(ctx, http.MethodGet, "/appliances/", nil, extra)
	if err != nil {
		return nil, fmt.Errorf("listing appliances: %w", err)
	}

	var appliances []LoadBalancer
	if err := parseResponse(data, &appliances); err != nil {
		return nil, fmt.Errorf("parsing appliances response: %w", err)
	}

	for i := range appliances {
		if appliances[i].ID == lbID {
			return &appliances[i], nil
		}
	}
	return nil, ErrLoadBalancerNotFound
}

// GetLoadBalancerRaw retrieves the full raw JSON config of a load balancer.
// This is needed for update operations (add/remove backend) since the API
// requires a full PUT of the entire LB config.
func (c *Client) GetLoadBalancerRaw(ctx context.Context, lbID int, location string) (map[string]interface{}, error) {
	extra := map[string]string{}
	if location != "" {
		extra["location"] = location
	}

	data, err := c.doRequest(ctx, http.MethodGet, "/appliances/", nil, extra)
	if err != nil {
		return nil, fmt.Errorf("listing appliances: %w", err)
	}

	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing appliances response wrapper: %w", err)
	}

	var appliances []map[string]interface{}
	raw := resp.Data
	if raw == nil {
		raw = data
	}
	if err := json.Unmarshal(raw, &appliances); err != nil {
		return nil, fmt.Errorf("parsing appliances list: %w", err)
	}

	for _, a := range appliances {
		id, ok := a["id"].(float64)
		if ok && int(id) == lbID {
			// The config we need is inside appliance_instance[0].context
			instances, ok := a["appliance_instance"].([]interface{})
			if !ok || len(instances) == 0 {
				return nil, fmt.Errorf("load balancer %d has no appliance_instance", lbID)
			}
			instance, ok := instances[0].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid appliance_instance format for LB %d", lbID)
			}
			context, ok := instance["context"].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid context format for LB %d", lbID)
			}
			return context, nil
		}
	}
	return nil, ErrLoadBalancerNotFound
}

// UpdateLoadBalancer updates a load balancer with the full config.
// PUT /appliances/load-balancers/{id}/
func (c *Client) UpdateLoadBalancer(ctx context.Context, lbID int, body map[string]interface{}, location string) error {
	extra := map[string]string{}
	if location != "" {
		extra["location"] = location
	}

	path := fmt.Sprintf("/appliances/load-balancers/%d/", lbID)
	_, err := c.doRequest(ctx, http.MethodPut, path, body, extra)
	if err != nil {
		return fmt.Errorf("updating load balancer %d: %w", lbID, err)
	}
	return nil
}

// DeleteLoadBalancer deletes a load balancer by ID.
// DELETE /appliances/{id}/
func (c *Client) DeleteLoadBalancer(ctx context.Context, lbID int, location string) error {
	extra := map[string]string{
		"reserve_ip_required": "false",
	}
	if location != "" {
		extra["location"] = location
	}

	path := fmt.Sprintf("/appliances/%d/", lbID)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil, extra)
	if err != nil {
		return fmt.Errorf("deleting load balancer %d: %w", lbID, err)
	}
	return nil
}

// AddBackendServer adds a server to the LB's tcp_backend by doing a GET-modify-PUT.
func (c *Client) AddBackendServer(ctx context.Context, lbID int, server TCPBackendServer, location string) error {
	config, err := c.GetLoadBalancerRaw(ctx, lbID, location)
	if err != nil {
		return fmt.Errorf("getting LB config for add backend: %w", err)
	}

	tcpBackend, err := getTCPBackend(config)
	if err != nil {
		return err
	}

	if len(tcpBackend) == 0 {
		return fmt.Errorf("load balancer %d has no tcp_backend groups", lbID)
	}

	// Add the server to the first backend group
	group, ok := tcpBackend[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid tcp_backend group format")
	}

	servers, _ := group["servers"].([]interface{})

	// Check if server already registered (idempotent)
	for _, s := range servers {
		srv, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		if ip, _ := srv["backend_ip"].(string); ip == server.BackendIP {
			// Already registered, nothing to do
			return nil
		}
	}

	newServer := map[string]interface{}{
		"target":       server.Target,
		"backend_name": server.BackendName,
		"backend_ip":   server.BackendIP,
		"backend_port": server.BackendPort,
	}
	group["servers"] = append(servers, newServer)

	return c.UpdateLoadBalancer(ctx, lbID, config, location)
}

// RemoveBackendServer removes a server from the LB's tcp_backend by IP, using GET-modify-PUT.
func (c *Client) RemoveBackendServer(ctx context.Context, lbID int, backendIP string, location string) error {
	config, err := c.GetLoadBalancerRaw(ctx, lbID, location)
	if err != nil {
		return fmt.Errorf("getting LB config for remove backend: %w", err)
	}

	tcpBackend, err := getTCPBackend(config)
	if err != nil {
		return err
	}

	if len(tcpBackend) == 0 {
		return nil
	}

	group, ok := tcpBackend[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid tcp_backend group format")
	}

	servers, _ := group["servers"].([]interface{})
	filtered := make([]interface{}, 0, len(servers))
	for _, s := range servers {
		srv, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		if ip, _ := srv["backend_ip"].(string); ip != backendIP {
			filtered = append(filtered, srv)
		}
	}
	group["servers"] = filtered

	return c.UpdateLoadBalancer(ctx, lbID, config, location)
}

// ClearBackendServers removes all servers from every tcp_backend group of the
// load balancer. This is called before DeleteLoadBalancer to ensure the E2E API
// does not reject the delete request due to active backends.
func (c *Client) ClearBackendServers(ctx context.Context, lbID int, location string) error {
	config, err := c.GetLoadBalancerRaw(ctx, lbID, location)
	if err != nil {
		// If the LB is already gone, nothing to clear.
		if errors.Is(err, ErrLoadBalancerNotFound) {
			return nil
		}
		return fmt.Errorf("getting LB config for clear backends: %w", err)
	}

	tcpBackend, err := getTCPBackend(config)
	if err != nil || len(tcpBackend) == 0 {
		return nil
	}

	changed := false
	for _, b := range tcpBackend {
		group, ok := b.(map[string]interface{})
		if !ok {
			continue
		}
		servers, _ := group["servers"].([]interface{})
		if len(servers) > 0 {
			group["servers"] = []interface{}{}
			changed = true
		}
	}

	if !changed {
		return nil
	}

	return c.UpdateLoadBalancer(ctx, lbID, config, location)
}

// getTCPBackend extracts the tcp_backend array from a raw LB config.
func getTCPBackend(config map[string]interface{}) ([]interface{}, error) {
	tcpBackend, ok := config["tcp_backend"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("tcp_backend field missing or invalid in LB config")
	}
	return tcpBackend, nil
}
