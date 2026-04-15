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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://api.e2enetworks.com/myaccount/api/v1"
	defaultTimeout = 30 * time.Second
)

// Client is the E2E Networks API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	authToken  string
	projectID  string
}

// ClientConfig holds the configuration for creating an E2E client.
type ClientConfig struct {
	APIKey    string
	AuthToken string
	ProjectID string
	BaseURL   string
}

// NewClient creates a new E2E Networks API client.
func NewClient(cfg ClientConfig) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    baseURL,
		apiKey:     cfg.APIKey,
		authToken:  cfg.AuthToken,
		projectID:  cfg.ProjectID,
	}
}

// doRequest executes an HTTP request against the E2E API.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)

	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))

	// Add query params
	q := req.URL.Query()
	q.Set("apikey", c.apiKey)
	q.Set("project_id", c.projectID)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		return respBody, nil
	case http.StatusNotFound:
		return nil, ErrNodeNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusTooManyRequests:
		return nil, ErrRateLimited
	default:
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrAPIFailure, resp.StatusCode, string(respBody))
	}
}

// APIResponse wraps the standard E2E API response format.
type APIResponse struct {
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
	Errors  json.RawMessage `json:"errors,omitempty"`
}

// parseResponse parses a standard E2E API response.
func parseResponse(data []byte, target interface{}) error {
	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		// Try direct unmarshalling if it's not a wrapped response
		return json.Unmarshal(data, target)
	}
	if resp.Data != nil {
		return json.Unmarshal(resp.Data, target)
	}
	return json.Unmarshal(data, target)
}
