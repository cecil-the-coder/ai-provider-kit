// Package anthropic provides an Anthropic Claude AI provider implementation.
// It includes support for chat completions, streaming, tool calling, and OAuth authentication.
package anthropic

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/models"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// AnthropicProvider implements the Provider interface for Anthropic Claude
type AnthropicProvider struct {
	*base.BaseProvider
	authHelper        *auth.AuthHelper
	client            *http.Client
	lastUsage         *types.Usage
	displayName       string
	config            AnthropicConfig
	modelCache        *models.ModelCache
	connectivityCache *common.ConnectivityCache

	// Rate limiting (header-based tracking)
	rateLimitHelper *common.RateLimitHelper

	// Request/response handling
	requestHandler base.RequestHandler
	responseParser base.ResponseParser
}

// Name returns the display name of the provider
func (p *AnthropicProvider) Name() string {
	if p.displayName != "" {
		return p.displayName
	}
	return "Anthropic"
}

// Type returns the provider type
func (p *AnthropicProvider) Type() types.ProviderType {
	return types.ProviderTypeAnthropic
}

// Description returns a description of the provider
func (p *AnthropicProvider) Description() string {
	return "Anthropic Claude models with multi-key failover and load balancing"
}

// GetModels returns the list of available models
func (p *AnthropicProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	// Check if we're using OAuth - if so, use static list since models.list doesn't work with OAuth
	if p.authHelper.OAuthManager != nil && len(p.authHelper.OAuthManager.GetCredentials()) > 0 {
		// For OAuth, check if token starts with sk-ant-oat (Anthropic's OAuth prefix)
		creds := p.authHelper.OAuthManager.GetCredentials()
		if len(creds) > 0 && strings.HasPrefix(creds[0].AccessToken, "sk-ant-oat") {
			log.Printf("Anthropic: Using static model list for OAuth authentication")
			return p.getStaticFallback(), nil
		}
	}

	// For API keys or non-Anthropic OAuth tokens, use the model list endpoint
	// Use the shared model cache utility
	return p.modelCache.GetModels(
		func() ([]types.Model, error) {
			// Fetch from API
			models, err := p.fetchModelsFromAPI(ctx)
			if err != nil {
				log.Printf("Anthropic: Failed to fetch models from API: %v", err)
				return nil, err
			}
			// Enrich with provider-specific metadata
			return p.enrichModels(models), nil
		},
		func() []types.Model {
			// Fallback to static list
			return p.getStaticFallback()
		},
	)
}

// GetDefaultModel returns the default model for the provider
func (p *AnthropicProvider) GetDefaultModel() string {
	config := p.GetConfig()
	if config.DefaultModel != "" {
		return config.DefaultModel
	}

	// If using non-Anthropic OAuth (no sk-ant-oat prefix), don't provide a default
	// User must specify a model explicitly since we can't query the model list
	if p.authHelper.OAuthManager != nil && len(p.authHelper.OAuthManager.GetCredentials()) > 0 {
		creds := p.authHelper.OAuthManager.GetCredentials()
		if len(creds) > 0 && !strings.HasPrefix(creds[0].AccessToken, "sk-ant-oat") {
			// Third-party OAuth - no default model
			return ""
		}
	}

	return anthropicDefaultModel
}

// IsAuthenticated checks if the provider is authenticated
func (p *AnthropicProvider) IsAuthenticated() bool {
	return p.authHelper.IsAuthenticated()
}

// IsOAuthConfigured checks if OAuth authentication is properly configured
func (p *AnthropicProvider) IsOAuthConfigured() bool {
	return p.authHelper.IsOAuthConfigured()
}

// IsAPIKeyConfigured checks if API key authentication is properly configured
func (p *AnthropicProvider) IsAPIKeyConfigured() bool {
	return p.authHelper.IsAPIKeyConfigured()
}

// SetCredentialProvider sets a dynamic credential provider for OAuth credentials
// This allows external systems to manage credential storage and provide fresh
// credentials on-demand, rather than relying on cached credentials.
func (p *AnthropicProvider) SetCredentialProvider(provider types.CredentialProvider) {
	if p.authHelper.OAuthManager != nil {
		p.authHelper.OAuthManager.SetCredentialProvider(provider)
	}
}

// GetAuthStatus provides detailed authentication status using shared helper
func (p *AnthropicProvider) GetAuthStatus() map[string]interface{} {
	return p.authHelper.GetAuthStatus()
}

// Logout clears the API keys and resets configuration
func (p *AnthropicProvider) Logout(ctx context.Context) error {
	p.authHelper.ClearAuthentication()
	newConfig := p.authHelper.Config
	newConfig.APIKey = ""
	return p.Configure(newConfig)
}

// SupportsToolCalling returns true if the provider supports tool calling
func (p *AnthropicProvider) SupportsToolCalling() bool {
	return true
}

// SupportsStreaming returns true if the provider supports streaming
func (p *AnthropicProvider) SupportsStreaming() bool {
	return true
}

// SupportsResponsesAPI returns false as Anthropic doesn't support the Responses API
func (p *AnthropicProvider) SupportsResponsesAPI() bool {
	return false
}

// GetToolFormat returns the Anthropic tool format
func (p *AnthropicProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatAnthropic
}

// InvokeServerTool invokes a server tool (not yet implemented for Anthropic)
func (p *AnthropicProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, fmt.Errorf("tool invocation not yet implemented for Anthropic provider")
}

// TestConnectivity performs a lightweight connectivity test using the /v1/models endpoint
// Results are cached for 30 seconds by default to prevent hammering the API during rapid health checks
// To bypass the cache and force a fresh check, use TestConnectivityWithOptions with bypassCache=true
func (p *AnthropicProvider) TestConnectivity(ctx context.Context) error {
	return p.TestConnectivityWithOptions(ctx, false)
}

// TestConnectivityWithOptions performs a connectivity test with optional cache bypass
// If bypassCache is true, the cache is bypassed and a fresh connectivity check is performed
func (p *AnthropicProvider) TestConnectivityWithOptions(ctx context.Context, bypassCache bool) error {
	return p.connectivityCache.TestConnectivity(
		ctx,
		types.ProviderTypeAnthropic,
		p.performConnectivityTest,
		bypassCache,
	)
}

// RefreshAllOAuthTokens using shared helper
func (p *AnthropicProvider) RefreshAllOAuthTokens(ctx context.Context) error {
	return p.authHelper.RefreshAllOAuthTokens(ctx)
}
