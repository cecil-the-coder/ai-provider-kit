// Package connectivity provides shared connectivity testing utilities for AI providers.
package connectivity

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Helper functions for common provider patterns

// NewModelsEndpointTest creates a test configuration for the /models endpoint
// This is the most common connectivity test pattern for OpenAI-compatible providers
func NewModelsEndpointTest(providerType types.ProviderType, baseURL, authToken string) TestRequestConfig {
	return TestRequestConfig{
		ProviderType: providerType,
		BaseURL:      baseURL,
		EndpointType: EndpointTypeModels,
		AuthToken:    authToken,
		AuthMethod:   "api_key",
		Headers: map[string]string{
			"User-Agent": telemetry.GetUserAgent(),
		},
	}
}

// NewChatEndpointTest creates a test configuration for the /chat/completions endpoint
// This is used for providers that don't have a /models endpoint or prefer chat-based tests
func NewChatEndpointTest(providerType types.ProviderType, baseURL, authToken, model string) TestRequestConfig {
	return TestRequestConfig{
		ProviderType: providerType,
		BaseURL:      baseURL,
		EndpointType: EndpointTypeChat,
		AuthToken:    authToken,
		AuthMethod:   "api_key",
		TestModel:    model,
		Headers: map[string]string{
			"User-Agent": telemetry.GetUserAgent(),
		},
	}
}

// NewOAuthTest creates a test configuration for OAuth-based authentication
func NewOAuthTest(providerType types.ProviderType, baseURL, accessToken, model string) TestRequestConfig {
	return TestRequestConfig{
		ProviderType: providerType,
		BaseURL:      baseURL,
		EndpointType: EndpointTypeChat,
		AuthToken:    accessToken,
		AuthMethod:   "oauth",
		TestModel:    model,
		Headers: map[string]string{
			"User-Agent": telemetry.GetUserAgent(),
		},
	}
}

// NewCustomTest creates a test configuration with custom parameters
func NewCustomTest(providerType types.ProviderType, baseURL, authToken, authMethod string, endpointType TestEndpointType) TestRequestConfig {
	return TestRequestConfig{
		ProviderType: providerType,
		BaseURL:      baseURL,
		EndpointType: endpointType,
		AuthToken:    authToken,
		AuthMethod:   authMethod,
		Headers: map[string]string{
			"User-Agent": telemetry.GetUserAgent(),
		},
	}
}

// CheckAuthentication checks if authentication credentials are configured
// Returns an error if no credentials are available
func CheckAuthentication(hasAPIKeys, hasOAuth, hasContextOAuth bool, providerType types.ProviderType) error {
	if !hasAPIKeys && !hasOAuth && !hasContextOAuth {
		return types.NewAuthError(providerType, "no API keys or OAuth credentials configured").
			WithOperation("test_connectivity")
	}
	return nil
}

// ValidateStatusCode validates an HTTP status code and returns an appropriate error
func ValidateStatusCode(providerType types.ProviderType, statusCode int) error {
	switch statusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return types.NewAuthError(providerType, "invalid authentication credentials").
			WithOperation("test_connectivity").
			WithStatusCode(statusCode)
	case http.StatusForbidden:
		return types.NewAuthError(providerType, "authentication credentials do not have access").
			WithOperation("test_connectivity").
			WithStatusCode(statusCode)
	default:
		return types.NewServerError(providerType, statusCode, fmt.Sprintf("unexpected status code: %d", statusCode)).
			WithOperation("test_connectivity")
	}
}

// PerformSimpleHTTPTest performs a simple HTTP GET request for connectivity testing
// This is useful for providers that have simple health check endpoints
func PerformSimpleHTTPTest(ctx context.Context, client *http.Client, url, authToken, authMethod string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	// Set authentication
	if authMethod == "bearer" || authMethod == "oauth" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	} else if authMethod == "api_key" {
		if req.Header.Get("Authorization") == "" {
			req.Header.Set("Authorization", "Bearer "+authToken)
		}
	}

	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// NewTestClient creates a new HTTP client with a timeout for connectivity testing
func NewTestClient(timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
	}
}

// GetAuthStatus returns a map describing the authentication status
func GetAuthStatus(hasAPIKeys, hasOAuth bool, apiKeyCount, oauthCount int) map[string]interface{} {
	status := map[string]interface{}{
		"authenticated": false,
		"method":        "none",
	}

	if hasOAuth && oauthCount > 0 {
		status["authenticated"] = true
		status["method"] = "oauth"
		status["oauth_credential_count"] = oauthCount
	}

	if hasAPIKeys && apiKeyCount > 0 {
		status["authenticated"] = true
		status["method"] = "api_key"
		status["api_key_count"] = apiKeyCount
	}

	return status
}
