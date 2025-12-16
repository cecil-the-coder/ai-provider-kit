// Package azure provides middleware for Azure OpenAI API integration.
// It handles endpoint transformation, API version management, and Azure Identity authentication.
//
// Azure OpenAI uses a different endpoint format than OpenAI:
// - OpenAI: https://api.openai.com/v1/chat/completions
// - Azure: https://{resource}.openai.azure.com/openai/deployments/{deployment}/chat/completions?api-version=2024-10-21
//
// This middleware transforms OpenAI-compatible requests to Azure OpenAI format.
//
// References:
// - Azure OpenAI REST API: https://learn.microsoft.com/en-us/azure/ai-services/openai/reference
// - API Version Lifecycle: https://learn.microsoft.com/en-us/azure/ai-foundry/openai/api-version-lifecycle
// - Azure Identity: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity
package azure

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/middleware"
)

// AzureConfig contains Azure-specific configuration for the middleware
type AzureConfig struct {
	// ResourceName is the Azure OpenAI resource name (e.g., "my-resource")
	// Used to construct the endpoint: https://{ResourceName}.openai.azure.com
	ResourceName string

	// Endpoint is the full Azure OpenAI endpoint URL (optional)
	// If provided, it takes precedence over ResourceName
	// Example: https://my-resource.openai.azure.com
	Endpoint string

	// DeploymentID is the deployment name for the model in Azure
	// This is required for Azure OpenAI and appears in the URL path
	DeploymentID string

	// APIVersion is the Azure OpenAI API version (e.g., "2024-10-21")
	// See: https://learn.microsoft.com/en-us/azure/ai-services/openai/reference
	// Default: "2024-10-21" (latest stable as of December 2025)
	APIVersion string

	// APIKey for Azure API key authentication
	// Either APIKey or TokenProvider must be provided
	APIKey string

	// UseAzureAD indicates whether to use Azure Active Directory token authentication
	// When true, TokenProvider must be provided
	UseAzureAD bool

	// TokenProvider provides Azure AD access tokens when UseAzureAD is true
	// Integrate with github.com/Azure/azure-sdk-for-go/sdk/azidentity for production use
	TokenProvider TokenProvider

	// UseV1API indicates whether to use the new v1 API format (GA August 2025)
	// When true, uses /openai/v1/ base path without api-version parameter
	// When false (default), uses /openai/deployments/ path with api-version
	// See: https://learn.microsoft.com/en-us/azure/ai-foundry/openai/api-version-lifecycle
	UseV1API bool

	// Scope is the Azure OAuth scope for token authentication
	// Default: "https://cognitiveservices.azure.com/.default"
	Scope string

	// Debug enables debug logging for the middleware
	Debug bool
}

// TokenProvider provides Azure AD access tokens
// This interface is compatible with Azure Identity SDK token credentials
type TokenProvider interface {
	// GetToken returns an Azure access token for the configured scope
	// Returns the token string and any error encountered
	GetToken(ctx context.Context) (string, error)
}

// AzureTokenCredentialAdapter adapts Azure Identity SDK TokenCredential to TokenProvider
// This allows seamless integration with github.com/Azure/azure-sdk-for-go/sdk/azidentity
// Example:
//   cred, _ := azidentity.NewDefaultAzureCredential(nil)
//   provider := &AzureTokenCredentialAdapter{
//       Credential: cred,
//       Scope: "https://cognitiveservices.azure.com/.default",
//   }
type AzureTokenCredentialAdapter struct {
	// Credential is the Azure Identity TokenCredential
	// Example: azidentity.DefaultAzureCredential, azidentity.ManagedIdentityCredential
	Credential interface {
		GetToken(ctx context.Context, opts interface{}) (interface{}, error)
	}

	// Scope is the OAuth scope for the token request
	Scope string

	// cachedToken stores the last retrieved token
	cachedToken string
	// cachedExpiry stores when the cached token expires
	cachedExpiry time.Time
	// mu protects token cache access
	mu sync.RWMutex
}

// GetToken implements TokenProvider using Azure Identity SDK
func (a *AzureTokenCredentialAdapter) GetToken(ctx context.Context) (string, error) {
	// Check if we have a valid cached token (with 5-minute buffer)
	a.mu.RLock()
	if a.cachedToken != "" && time.Now().Before(a.cachedExpiry.Add(-5*time.Minute)) {
		token := a.cachedToken
		a.mu.RUnlock()
		return token, nil
	}
	a.mu.RUnlock()

	// Need to refresh token
	a.mu.Lock()
	defer a.mu.Unlock()

	// Double-check after acquiring write lock
	if a.cachedToken != "" && time.Now().Before(a.cachedExpiry.Add(-5*time.Minute)) {
		return a.cachedToken, nil
	}

	// Note: This is a placeholder for Azure SDK integration
	// In production, use azcore.TokenRequestOptions and azcore.AccessToken
	// Example:
	//   opts := policy.TokenRequestOptions{Scopes: []string{a.Scope}}
	//   token, err := a.Credential.GetToken(ctx, opts)
	//   a.cachedToken = token.Token
	//   a.cachedExpiry = token.ExpiresOn

	return "", fmt.Errorf("azure identity SDK integration required: use github.com/Azure/azure-sdk-for-go/sdk/azidentity")
}

// AzureMiddleware implements request transformation for Azure OpenAI
type AzureMiddleware struct {
	config        *AzureConfig
	tokenProvider TokenProvider
	tokenCache    struct {
		token      string
		expiry     time.Time
		lastRefresh time.Time
		mu         sync.RWMutex
	}
}

// NewAzureMiddleware creates a new Azure OpenAI middleware instance
func NewAzureMiddleware(config *AzureConfig) (*AzureMiddleware, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid azure config: %w", err)
	}

	// Set default API version
	if config.APIVersion == "" {
		config.APIVersion = "2024-10-21"
	}

	// Set default scope for token authentication
	if config.Scope == "" {
		config.Scope = "https://cognitiveservices.azure.com/.default"
	}

	return &AzureMiddleware{
		config:        config,
		tokenProvider: config.TokenProvider,
	}, nil
}

// Validate validates the Azure configuration
func (c *AzureConfig) Validate() error {
	// Either ResourceName or Endpoint must be provided
	if c.ResourceName == "" && c.Endpoint == "" {
		return fmt.Errorf("either ResourceName or Endpoint must be provided")
	}

	// DeploymentID is required for non-v1 API
	if !c.UseV1API && c.DeploymentID == "" {
		return fmt.Errorf("DeploymentID must be provided for traditional API format")
	}

	// Validate authentication configuration
	if !c.UseAzureAD {
		// API key authentication
		if c.APIKey == "" {
			return fmt.Errorf("APIKey must be provided when UseAzureAD is false")
		}
	} else {
		// Azure AD token authentication
		if c.TokenProvider == nil {
			return fmt.Errorf("TokenProvider must be provided when UseAzureAD is true")
		}
	}

	return nil
}

// GetEndpoint returns the full Azure OpenAI endpoint URL
func (c *AzureConfig) GetEndpoint() string {
	if c.Endpoint != "" {
		return strings.TrimSuffix(c.Endpoint, "/")
	}
	return fmt.Sprintf("https://%s.openai.azure.com", c.ResourceName)
}

// ProcessRequest implements middleware.RequestMiddleware
// Transforms OpenAI-compatible requests to Azure OpenAI format
func (m *AzureMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	// Only process OpenAI-compatible API requests
	if !m.isOpenAIRequest(req) {
		return ctx, req, nil
	}

	// Transform the URL for Azure OpenAI
	if err := m.transformURL(req); err != nil {
		return ctx, req, fmt.Errorf("azure: failed to transform URL: %w", err)
	}

	// Add Azure-specific query parameters (api-version for traditional API)
	if !m.config.UseV1API {
		m.addAPIVersionParam(req)
	}

	// Set authentication headers
	if err := m.setAuthHeaders(ctx, req); err != nil {
		return ctx, req, fmt.Errorf("azure: failed to set auth headers: %w", err)
	}

	// Remove OpenAI-specific headers that Azure doesn't use
	req.Header.Del("Authorization") // Remove if it was set (we use api-key or Bearer token)
	req.Header.Del("OpenAI-Organization")
	req.Header.Del("OpenAI-Beta")

	// Store provider info in context for response transformation
	ctx = context.WithValue(ctx, middleware.ContextKeyProvider, "azure")

	if m.config.Debug {
		fmt.Printf("azure: transformed request: %s %s\n", req.Method, req.URL.String())
	}

	return ctx, req, nil
}

// ProcessResponse implements middleware.ResponseMiddleware
// Azure OpenAI uses the same response format as OpenAI, so minimal transformation is needed
func (m *AzureMiddleware) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
	// Check if this response came from Azure
	provider := ctx.Value(middleware.ContextKeyProvider)
	if provider != "azure" {
		return ctx, resp, nil
	}

	// Azure OpenAI responses are compatible with OpenAI format
	// No transformation needed in most cases
	return ctx, resp, nil
}

// isOpenAIRequest checks if this is an OpenAI-compatible API request
func (m *AzureMiddleware) isOpenAIRequest(req *http.Request) bool {
	path := req.URL.Path

	// Check for common OpenAI API paths
	return strings.Contains(path, "/v1/chat/completions") ||
		strings.Contains(path, "/v1/completions") ||
		strings.Contains(path, "/v1/embeddings") ||
		strings.Contains(path, "/v1/models") ||
		strings.Contains(path, "/v1/audio") ||
		strings.Contains(path, "/v1/images")
}

// transformURL transforms OpenAI URL to Azure OpenAI format
func (m *AzureMiddleware) transformURL(req *http.Request) error {
	endpoint := m.config.GetEndpoint()

	// Parse the endpoint to get scheme and host
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint: %w", err)
	}

	// Get the original path
	originalPath := req.URL.Path

	var newPath string

	if m.config.UseV1API {
		// New v1 API format (GA August 2025)
		// https://{endpoint}/openai/v1/{operation}
		// Example: https://my-resource.openai.azure.com/openai/v1/chat/completions
		newPath = "/openai" + originalPath
	} else {
		// Traditional API format
		// https://{endpoint}/openai/deployments/{deployment-id}/{operation}
		// Example: https://my-resource.openai.azure.com/openai/deployments/gpt-4/chat/completions

		// Extract operation from path (e.g., /v1/chat/completions -> /chat/completions)
		operation := strings.TrimPrefix(originalPath, "/v1")

		// Handle special cases
		if operation == "/models" || originalPath == "/models" {
			// Models endpoint lists deployments in Azure
			newPath = "/openai/deployments"
		} else {
			// Standard operation with deployment ID
			newPath = fmt.Sprintf("/openai/deployments/%s%s", m.config.DeploymentID, operation)
		}
	}

	// Update the URL
	req.URL.Scheme = endpointURL.Scheme
	req.URL.Host = endpointURL.Host
	req.URL.Path = newPath
	req.Host = endpointURL.Host

	return nil
}

// addAPIVersionParam adds the api-version query parameter
func (m *AzureMiddleware) addAPIVersionParam(req *http.Request) {
	query := req.URL.Query()
	query.Set("api-version", m.config.APIVersion)
	req.URL.RawQuery = query.Encode()
}

// setAuthHeaders sets the appropriate authentication headers
func (m *AzureMiddleware) setAuthHeaders(ctx context.Context, req *http.Request) error {
	if m.config.UseAzureAD {
		// Azure AD token authentication
		token, err := m.getValidToken(ctx)
		if err != nil {
			return fmt.Errorf("failed to get Azure AD token: %w", err)
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	} else {
		// API key authentication
		req.Header.Set("api-key", m.config.APIKey)
	}

	return nil
}

// getValidToken returns a valid token, refreshing if necessary
func (m *AzureMiddleware) getValidToken(ctx context.Context) (string, error) {
	m.tokenCache.mu.RLock()
	cachedToken := m.tokenCache.token
	cachedExpiry := m.tokenCache.expiry
	m.tokenCache.mu.RUnlock()

	// Check if we have a valid cached token (with 5-minute buffer)
	if cachedToken != "" && time.Now().Before(cachedExpiry.Add(-5*time.Minute)) {
		return cachedToken, nil
	}

	// Need to refresh token
	m.tokenCache.mu.Lock()
	defer m.tokenCache.mu.Unlock()

	// Double-check after acquiring write lock
	if m.tokenCache.token != "" && time.Now().Before(m.tokenCache.expiry.Add(-5*time.Minute)) {
		return m.tokenCache.token, nil
	}

	// Get new token from provider
	token, err := m.tokenProvider.GetToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get token from provider: %w", err)
	}

	// Cache the token
	// Note: We don't have expiry info from the simple TokenProvider interface
	// In production, enhance the interface or use a more sophisticated token cache
	m.tokenCache.token = token
	m.tokenCache.expiry = time.Now().Add(1 * time.Hour) // Default 1-hour expiry
	m.tokenCache.lastRefresh = time.Now()

	return token, nil
}

// ClearTokenCache clears the cached token, forcing a refresh on the next request
func (m *AzureMiddleware) ClearTokenCache() {
	m.tokenCache.mu.Lock()
	defer m.tokenCache.mu.Unlock()
	m.tokenCache.token = ""
	m.tokenCache.expiry = time.Time{}
}

// GetConfig returns the middleware configuration
func (m *AzureMiddleware) GetConfig() *AzureConfig {
	return m.config
}

// GetLastTokenRefreshTime returns the time of the last token refresh
func (m *AzureMiddleware) GetLastTokenRefreshTime() time.Time {
	m.tokenCache.mu.RLock()
	defer m.tokenCache.mu.RUnlock()
	return m.tokenCache.lastRefresh
}

// RoundTrip implements http.RoundTripper for backward compatibility
// Use ProcessRequest/ProcessResponse with middleware.MiddlewareChain for new code
func (m *AzureMiddleware) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	newReq := req.Clone(req.Context())

	// Apply request transformation
	_, transformedReq, err := m.ProcessRequest(newReq.Context(), newReq)
	if err != nil {
		return nil, err
	}

	// Use the default transport to execute the request
	transport := http.DefaultTransport
	if t, ok := req.Context().Value("base_transport").(http.RoundTripper); ok {
		transport = t
	}

	resp, err := transport.RoundTrip(transformedReq)
	if err != nil {
		return nil, err
	}

	// Apply response transformation
	_, transformedResp, err := m.ProcessResponse(transformedReq.Context(), transformedReq, resp)
	if err != nil {
		return nil, err
	}

	return transformedResp, nil
}

// WrapTransport wraps an existing http.RoundTripper with Azure middleware
func (m *AzureMiddleware) WrapTransport(transport http.RoundTripper) http.RoundTripper {
	return &wrappedTransport{
		base:       transport,
		middleware: m,
	}
}

// wrappedTransport combines base transport with Azure middleware
type wrappedTransport struct {
	base       http.RoundTripper
	middleware *AzureMiddleware
}

// RoundTrip implements http.RoundTripper
func (wt *wrappedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Store the base transport in context for middleware to use
	ctx := context.WithValue(req.Context(), "base_transport", wt.base)
	newReq := req.WithContext(ctx)

	// Apply middleware transformation
	return wt.middleware.RoundTrip(newReq)
}
