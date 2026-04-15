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

import "errors"

var (
	// ErrNodeNotFound is returned when a node is not found.
	ErrNodeNotFound = errors.New("node not found")

	// ErrLoadBalancerNotFound is returned when a load balancer is not found.
	ErrLoadBalancerNotFound = errors.New("load balancer not found")

	// ErrUnauthorized is returned when the API key or token is invalid.
	ErrUnauthorized = errors.New("unauthorized: invalid API key or token")

	// ErrRateLimited is returned when the API rate limit is exceeded.
	ErrRateLimited = errors.New("rate limited: too many requests")

	// ErrAPIFailure is returned for general API failures.
	ErrAPIFailure = errors.New("E2E API request failed")
)
