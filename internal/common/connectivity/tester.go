// Package connectivity provides shared connectivity testing utilities for AI providers.
// It abstracts common patterns used across multiple providers for testing API connectivity.
package connectivity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestEndpointType defines the type of endpoint to use for connectivity testing
type TestEndpointType string

const (
	// EndpointTypeModels uses the /models endpoint for connectivity testing
	EndpointTypeModels TestEndpointType = "models"
	// EndpointTypeChat uses the /chat/completions endpoint for connectivity testing
	EndpointTypeChat TestEndpointType = "chat"
	// EndpointTypeGenerate uses the provider-specific generate endpoint for connectivity testing
	EndpointTypeGenerate TestEndpointType = "generate"
)

// TestRequestConfig holds the configuration for a connectivity test request
type TestRequestConfig struct {
	// ProviderType is the type of provider being tested
	ProviderType types.ProviderType

	// BaseURL is the base URL of the provider API
	BaseURL string

	// EndpointType specifies which endpoint to use for the test
	EndpointType TestEndpointType

	// AuthToken is the authentication token to use
	AuthToken string

	// AuthMethod is the authentication method ("api_key", "oauth", "bearer")
	AuthMethod string

	// TestModel is the model to use for chat-based tests (defaults to provider default)
	TestModel string

	// Headers are additional headers to include in the request
	Headers map[string]string

	// Timeout is the timeout for the test request (defaults to 10 seconds)
	Timeout time.Duration

	// MinimalRequestBody is the request body for chat/generate tests
	// If nil, a default minimal request will be created
	MinimalRequestBody interface{}

	// SuccessValidator is a custom function to validate the response
	// If nil, default validation is used
	SuccessValidator func(*http.Response, []byte) error

	// CreateRequest is a custom function to create the HTTP request
	// If nil, default request creation is used
	CreateRequest func(ctx context.Context, baseURL string, config TestRequestConfig) (*http.Request, error)
}

// TestResult contains the result of a connectivity test
type TestResult struct {
	// Success indicates whether the connectivity test was successful
	Success bool

	// Error contains any error that occurred during the test
	Error error

	// StatusCode is the HTTP status code returned (0 if request failed)
	StatusCode int

	// Latency is the time taken to complete the test
	Latency time.Duration

	// ResponseBody contains the response body (truncated if too large)
	ResponseBody string
}

// Tester provides shared connectivity testing functionality
type Tester struct {
	cache *common.ConnectivityCache
}

// NewTester creates a new connectivity tester with default cache configuration
func NewTester() *Tester {
	return &Tester{
		cache: common.NewDefaultConnectivityCache(),
	}
}

// NewTesterWithCache creates a new connectivity tester with a custom cache
func NewTesterWithCache(cache *common.ConnectivityCache) *Tester {
	return &Tester{
		cache: cache,
	}
}

// TestConnectivity performs a connectivity test with caching
func (t *Tester) TestConnectivity(ctx context.Context, config TestRequestConfig, bypassCache bool) error {
	return t.cache.TestConnectivity(
		ctx,
		config.ProviderType,
		func(ctx context.Context) error {
			return t.performTest(ctx, config)
		},
		bypassCache,
	)
}

// TestConnectivityWithResult performs a connectivity test and returns detailed results
func (t *Tester) TestConnectivityWithResult(ctx context.Context, config TestRequestConfig, bypassCache bool) *TestResult {
	startTime := time.Now()
	result := &TestResult{}

	err := t.cache.TestConnectivity(
		ctx,
		config.ProviderType,
		func(ctx context.Context) error {
			return t.performTest(ctx, config)
		},
		bypassCache,
	)

	result.Latency = time.Since(startTime)
	result.Success = err == nil
	result.Error = err

	return result
}

// performTest performs the actual connectivity test without caching
func (t *Tester) performTest(ctx context.Context, config TestRequestConfig) error {
	// Set default timeout if not specified
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: config.Timeout,
	}

	// Create the request
	req, err := t.createRequest(ctx, config)
	if err != nil {
		return err
	}

	// Set authentication headers
	t.setAuthHeaders(req, config)

	// Set User-Agent
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", telemetry.GetUserAgent())
	}

	// Set additional headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Make the request
	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return types.NewNetworkError(config.ProviderType, "connectivity test failed").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	latency := time.Since(startTime)

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		return types.NewAuthError(config.ProviderType, "invalid authentication credentials").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(config.ProviderType, "authentication credentials do not have access").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(config.ProviderType, resp.StatusCode,
			fmt.Sprintf("connectivity test failed (latency: %v): %s", latency, string(body))).
			WithOperation("test_connectivity")
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewNetworkError(config.ProviderType, "failed to read response body").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Validate response
	if config.SuccessValidator != nil {
		if err := config.SuccessValidator(resp, body); err != nil {
			return err
		}
	} else {
		if err := t.defaultSuccessValidator(resp, body, config); err != nil {
			return err
		}
	}

	return nil
}

// createRequest creates the HTTP request for the connectivity test
func (t *Tester) createRequest(ctx context.Context, config TestRequestConfig) (*http.Request, error) {
	// Use custom request creator if provided
	if config.CreateRequest != nil {
		return config.CreateRequest(ctx, config.BaseURL, config)
	}

	switch config.EndpointType {
	case EndpointTypeModels:
		return t.createModelsRequest(ctx, config)
	case EndpointTypeChat, EndpointTypeGenerate:
		return t.createChatRequest(ctx, config)
	default:
		return nil, types.NewInvalidRequestError(config.ProviderType, fmt.Sprintf("unsupported endpoint type: %s", config.EndpointType)).
			WithOperation("test_connectivity")
	}
}

// createModelsRequest creates a request to the /models endpoint
func (t *Tester) createModelsRequest(ctx context.Context, config TestRequestConfig) (*http.Request, error) {
	url := config.BaseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, types.NewNetworkError(config.ProviderType, "failed to create request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}
	return req, nil
}

// createChatRequest creates a request to a chat/generate endpoint
func (t *Tester) createChatRequest(ctx context.Context, config TestRequestConfig) (*http.Request, error) {
	url := config.BaseURL + "/chat/completions"

	// Create minimal request body
	body := config.MinimalRequestBody
	if body == nil {
		body = t.createMinimalRequestBody(config.TestModel)
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, types.NewInvalidRequestError(config.ProviderType, "failed to marshal request body").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, types.NewNetworkError(config.ProviderType, "failed to create request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// createMinimalRequestBody creates a minimal request body for chat-based tests
func (t *Tester) createMinimalRequestBody(model string) interface{} {
	if model == "" {
		model = "gpt-3.5-turbo" // Default fallback model
	}

	return map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "Hi",
			},
		},
		"max_tokens": 1,
	}
}

// setAuthHeaders sets the authentication headers based on the auth method
func (t *Tester) setAuthHeaders(req *http.Request, config TestRequestConfig) {
	switch config.AuthMethod {
	case "bearer", "oauth":
		if config.AuthToken != "" {
			req.Header.Set("Authorization", "Bearer "+config.AuthToken)
		}
	case "api_key":
		if config.AuthToken != "" {
			// Try common API key header patterns
			if req.Header.Get("Authorization") == "" {
				req.Header.Set("Authorization", "Bearer "+config.AuthToken)
			}
			if req.Header.Get("x-api-key") == "" {
				req.Header.Set("x-api-key", config.AuthToken)
			}
		}
	}
}

// defaultSuccessValidator provides default response validation
func (t *Tester) defaultSuccessValidator(resp *http.Response, body []byte, config TestRequestConfig) error {
	switch config.EndpointType {
	case EndpointTypeModels:
		// Validate models endpoint response
		var testResponse struct {
			Data []interface{} `json:"data"`
		}
		if err := json.Unmarshal(body, &testResponse); err != nil {
			return types.NewInvalidRequestError(config.ProviderType, "invalid response from models endpoint").
				WithOperation("test_connectivity").
				WithOriginalErr(err)
		}
		// Successfully parsed response - connectivity verified
		return nil

	case EndpointTypeChat, EndpointTypeGenerate:
		// For chat endpoints, we just need a successful response
		// The body structure can vary significantly between providers
		return nil

	default:
		return nil
	}
}

// GetCache returns the underlying connectivity cache
func (t *Tester) GetCache() *common.ConnectivityCache {
	return t.cache
}

// ClearCache clears the connectivity cache for a specific provider
func (t *Tester) ClearCache(providerType types.ProviderType) {
	t.cache.Clear(providerType)
}

// ClearAllCache clears all connectivity cache entries
func (t *Tester) ClearAllCache() {
	t.cache.ClearAll()
}
