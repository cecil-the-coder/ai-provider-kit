// Package gemini provides a Google Gemini AI provider implementation.
// It includes support for chat completions, streaming, tool calling, and OAuth authentication.
package gemini

import (
	"context"
	"net/http"
	"sync"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/auth"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"golang.org/x/time/rate"
)

// GeminiProvider implements the Provider interface for Google Gemini with OAuth support
type GeminiProvider struct {
	*base.BaseProvider
	authHelper        *auth.AuthHelper // Shared authentication helper
	client            *http.Client
	config            GeminiConfig
	projectID         string
	displayName       string
	rateLimitHelper   *common.RateLimitHelper
	rateLimitMutex    sync.RWMutex
	clientSideLimiter *rate.Limiter
	backendRouter     *BackendRouter // Backend router for Gemini API vs Vertex AI
}

// Name returns the display name of the provider
func (p *GeminiProvider) Name() string {
	if p.displayName != "" {
		return p.displayName
	}
	return "gemini"
}

// Type returns the provider type
func (p *GeminiProvider) Type() types.ProviderType {
	return types.ProviderTypeGemini
}

// Description returns a description of the provider
func (p *GeminiProvider) Description() string {
	return "Google Gemini with multi-OAuth failover and load balancing"
}

// GetModels returns the list of available models
func (p *GeminiProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	return []types.Model{
		// Gemini 3 Series (Preview)
		{ID: "gemini-3-pro-preview", Name: "Gemini 3 Pro Preview", Provider: p.Type(), MaxTokens: 2097152, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Google's latest Gemini 3 Pro model with 2M context (preview)"},
		{ID: "gemini-3-pro-image-preview", Name: "Gemini 3 Pro Image Preview", Provider: p.Type(), MaxTokens: 2097152, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Gemini 3 Pro with enhanced image understanding (preview)"},

		// Gemini 2.5 Series
		{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Provider: p.Type(), MaxTokens: 2097152, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "State-of-the-art thinking model for complex problems with 2M context"},
		{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: p.Type(), MaxTokens: 1048576, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Best price-performance for high-volume tasks and agentic use cases"},
		{ID: "gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Provider: p.Type(), MaxTokens: 524288, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Built for massive scale, optimized for efficiency"},

		// Gemini 2.0 Series
		{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Provider: p.Type(), MaxTokens: 1048576, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Multimodal model for general-purpose tasks"},
		{ID: "gemini-2.0-flash-lite", Name: "Gemini 2.0 Flash Lite", Provider: p.Type(), MaxTokens: 524288, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Ultra-efficient for simple, high-frequency tasks"},
	}, nil
}

// GetDefaultModel returns the default model for the provider
func (p *GeminiProvider) GetDefaultModel() string {
	if p.config.Model != "" {
		return p.config.Model
	}
	return geminiDefaultModel
}

// IsAuthenticated checks if the provider is authenticated
func (p *GeminiProvider) IsAuthenticated() bool {
	return p.authHelper.IsAuthenticated()
}

// IsOAuthConfigured checks if OAuth authentication is properly configured
func (p *GeminiProvider) IsOAuthConfigured() bool {
	return p.authHelper.IsOAuthConfigured()
}

// IsAPIKeyConfigured checks if API key authentication is properly configured
func (p *GeminiProvider) IsAPIKeyConfigured() bool {
	return p.authHelper.IsAPIKeyConfigured()
}

// SetCredentialProvider sets a dynamic credential provider for OAuth credentials
// This allows external systems to manage credential storage and provide fresh
// credentials on-demand, rather than relying on cached credentials.
func (p *GeminiProvider) SetCredentialProvider(provider types.CredentialProvider) {
	if p.authHelper.OAuthManager != nil {
		p.authHelper.OAuthManager.SetCredentialProvider(provider)
	}
}

// GetAuthStatus provides detailed authentication status using shared helper
func (p *GeminiProvider) GetAuthStatus() map[string]interface{} {
	return p.authHelper.GetAuthStatus()
}

// Logout clears the authentication credentials
func (p *GeminiProvider) Logout(ctx context.Context) error {
	// Use auth helper to clear authentication
	p.authHelper.ClearAuthentication()

	// Clear local config
	p.config.APIKey = ""
	p.config.APIKeys = nil

	newConfig := p.GetConfig()
	return p.Configure(newConfig)
}

// TestConnectivity performs a lightweight connectivity test to verify the provider can reach its service
func (p *GeminiProvider) TestConnectivity(ctx context.Context) error {
	// Check for OAuth token in context first (injected by caller)
	contextToken := auth.GetOAuthToken(ctx)

	// Check if we have API keys configured
	hasAPIKeys := p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0
	// Use GetCredentialsWithContext to fetch fresh credentials from dynamic provider
	hasOAuth := p.authHelper.OAuthManager != nil && len(p.authHelper.OAuthManager.GetCredentialsWithContext(ctx)) > 0
	hasContextOAuth := contextToken != ""

	if !hasAPIKeys && !hasOAuth && !hasContextOAuth {
		return types.NewAuthError(types.ProviderTypeGemini, "no API keys or OAuth credentials configured").
			WithOperation("test_connectivity")
	}

	// For API keys, make a minimal API call to test connectivity
	if hasAPIKeys {
		apiKey := p.authHelper.KeyManager.GetKeys()[0]
		if err := p.testConnectivityWithAPIKey(ctx, apiKey); err == nil {
			return nil
		}
	}

	// For OAuth, prefer context token, then stored credentials
	if hasContextOAuth {
		return p.testConnectivityWithOAuth(ctx, contextToken)
	}
	if hasOAuth {
		// Use GetCredentialsWithContext to fetch fresh credentials from dynamic provider
		creds := p.authHelper.OAuthManager.GetCredentialsWithContext(ctx)
		return p.testConnectivityWithOAuth(ctx, creds[0].AccessToken)
	}

	return types.NewAuthError(types.ProviderTypeGemini, "no valid authentication credentials available").
		WithOperation("test_connectivity")
}

// SupportsToolCalling returns true if the provider supports tool calling
func (p *GeminiProvider) SupportsToolCalling() bool {
	return true
}

// SupportsStreaming returns true if the provider supports streaming
func (p *GeminiProvider) SupportsStreaming() bool {
	return true
}

// SupportsResponsesAPI returns false as Gemini doesn't support the Responses API
func (p *GeminiProvider) SupportsResponsesAPI() bool {
	return false
}

// GetToolFormat returns the Gemini tool format
func (p *GeminiProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatGemini
}

// InvokeServerTool invokes a server tool (not yet implemented for Gemini)
func (p *GeminiProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "tool invocation not yet implemented for Gemini provider").
		WithOperation("invoke_tool")
}

// RefreshAllOAuthTokens using shared helper
func (p *GeminiProvider) RefreshAllOAuthTokens(ctx context.Context) error {
	return p.authHelper.RefreshAllOAuthTokens(ctx)
}
