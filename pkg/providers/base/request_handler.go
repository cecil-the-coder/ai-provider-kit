// Package base provides common functionality and utilities for AI providers.
package base

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
)

// RequestHandler interface for executing HTTP requests with authentication
// Provides a unified way to make API calls across different providers
type RequestHandler interface {
	// ExecuteRequest makes an HTTP request with custom headers
	ExecuteRequest(ctx context.Context, method, url string, body interface{}, headers map[string]string) (*http.Response, error)

	// ExecuteAuthenticatedRequest makes an HTTP request with automatic authentication
	// useOAuth determines whether to use OAuth or API key authentication
	ExecuteAuthenticatedRequest(ctx context.Context, method, url string, body interface{}, useOAuth bool) (*http.Response, error)

	// PrepareJSONBody marshals the body into JSON and returns a buffer
	PrepareJSONBody(body interface{}) (*bytes.Buffer, error)
}

// DefaultRequestHandler provides a default implementation of RequestHandler
type DefaultRequestHandler struct {
	client     *http.Client
	authHelper *common.AuthHelper
	baseURL    string
}

// NewDefaultRequestHandler creates a new DefaultRequestHandler
func NewDefaultRequestHandler(client *http.Client, authHelper *common.AuthHelper, baseURL string) *DefaultRequestHandler {
	return &DefaultRequestHandler{
		client:     client,
		authHelper: authHelper,
		baseURL:    baseURL,
	}
}

// ExecuteRequest makes an HTTP request with custom headers
func (h *DefaultRequestHandler) ExecuteRequest(ctx context.Context, method, url string, body interface{}, headers map[string]string) (*http.Response, error) {
	var reqBody *bytes.Buffer
	var err error

	if body != nil {
		reqBody, err = h.PrepareJSONBody(body)
		if err != nil {
			return nil, err
		}
	}

	var req *http.Request
	if reqBody != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, reqBody)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Set Content-Type if body is present and not already set
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// ExecuteAuthenticatedRequest makes an HTTP request with automatic authentication
func (h *DefaultRequestHandler) ExecuteAuthenticatedRequest(ctx context.Context, method, url string, body interface{}, useOAuth bool) (*http.Response, error) {
	var reqBody *bytes.Buffer
	var err error

	if body != nil {
		reqBody, err = h.PrepareJSONBody(body)
		if err != nil {
			return nil, err
		}
	}

	var req *http.Request
	if reqBody != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, reqBody)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication headers based on method
	var authToken string
	var authType string

	if useOAuth {
		if h.authHelper.OAuthManager != nil {
			creds := h.authHelper.OAuthManager.GetCredentials()
			if len(creds) > 0 {
				authToken = creds[0].AccessToken
				authType = "oauth"
			}
		}
	} else {
		if h.authHelper.KeyManager != nil {
			keys := h.authHelper.KeyManager.GetKeys()
			if len(keys) > 0 {
				authToken = keys[0]
				authType = "api_key"
			}
		}
	}

	if authToken == "" {
		return nil, fmt.Errorf("no authentication credentials available")
	}

	// Use auth helper to set headers
	h.authHelper.SetAuthHeaders(req, authToken, authType)
	h.authHelper.SetProviderSpecificHeaders(req)

	// Set Content-Type
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// PrepareJSONBody marshals the body into JSON and returns a buffer
func (h *DefaultRequestHandler) PrepareJSONBody(body interface{}) (*bytes.Buffer, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	return bytes.NewBuffer(jsonData), nil
}

// GetBaseURL returns the base URL for API requests
func (h *DefaultRequestHandler) GetBaseURL() string {
	return h.baseURL
}

// SetBaseURL updates the base URL for API requests
func (h *DefaultRequestHandler) SetBaseURL(baseURL string) {
	h.baseURL = baseURL
}
