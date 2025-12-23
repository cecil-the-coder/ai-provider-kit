// Package copilot provides a GitHub Copilot AI provider implementation.
// It includes support for chat completions, streaming, tool calling, and GitHub OAuth authentication.
package copilot

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/auth"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/models"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// CopilotProvider implements the Provider interface for GitHub Copilot
type CopilotProvider struct {
	*base.BaseProvider
	authHelper        *auth.AuthHelper
	client            *http.Client
	config            CopilotConfig
	displayName       string
	modelCache        *models.ModelCache
	connectivityCache *common.ConnectivityCache

	// Rate limiting
	rateLimitHelper *common.RateLimitHelper

	// Request/response handling
	requestHandler base.RequestHandler
	responseParser base.ResponseParser

	// Token management
	githubToken        string
	copilotToken       string
	copilotTokenExpiry time.Time
	refreshIn          int // seconds until recommended refresh
	tokenMutex         sync.RWMutex
	cancelRefresh      context.CancelFunc
}

// Name returns the display name of the provider
func (p *CopilotProvider) Name() string {
	if p.displayName != "" {
		return p.displayName
	}
	return "Copilot"
}

// Type returns the provider type
func (p *CopilotProvider) Type() types.ProviderType {
	return types.ProviderTypeCopilot
}

// Description returns a description of the provider
func (p *CopilotProvider) Description() string {
	return "GitHub Copilot with OpenAI-compatible API"
}

// GetModels returns the list of available models
func (p *CopilotProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	// Use the shared model cache utility
	return p.modelCache.GetModels(
		func() ([]types.Model, error) {
			// Fetch from API
			models, err := p.fetchModelsFromAPI(ctx)
			if err != nil {
				return nil, err
			}
			return models, nil
		},
		func() []types.Model {
			// Fallback to static list
			return p.getStaticFallback()
		},
	)
}

// GetDefaultModel returns the default model for the provider
func (p *CopilotProvider) GetDefaultModel() string {
	config := p.GetConfig()
	if config.DefaultModel != "" {
		return config.DefaultModel
	}
	return copilotDefaultModel
}

// IsAuthenticated checks if the provider is authenticated
func (p *CopilotProvider) IsAuthenticated() bool {
	p.tokenMutex.RLock()
	defer p.tokenMutex.RUnlock()

	if p.githubToken == "" {
		return false
	}

	// Check if Copilot token is valid
	if p.copilotToken == "" || time.Now().After(p.copilotTokenExpiry) {
		return false
	}

	return true
}

// IsOAuthConfigured checks if OAuth authentication is properly configured
func (p *CopilotProvider) IsOAuthConfigured() bool {
	p.tokenMutex.RLock()
	defer p.tokenMutex.RUnlock()
	return p.githubToken != ""
}

// IsAPIKeyConfigured checks if API key authentication is properly configured
// For Copilot, this means having a valid Copilot token
func (p *CopilotProvider) IsAPIKeyConfigured() bool {
	p.tokenMutex.RLock()
	defer p.tokenMutex.RUnlock()
	return p.copilotToken != "" && time.Now().Before(p.copilotTokenExpiry)
}

// SetCredentialProvider sets a dynamic credential provider for OAuth credentials
func (p *CopilotProvider) SetCredentialProvider(provider types.CredentialProvider) {
	if p.authHelper.OAuthManager != nil {
		p.authHelper.OAuthManager.SetCredentialProvider(provider)
	}
}

// GetAuthStatus provides detailed authentication status
func (p *CopilotProvider) GetAuthStatus() map[string]interface{} {
	p.tokenMutex.RLock()
	defer p.tokenMutex.RUnlock()

	status := make(map[string]interface{})

	hasGitHubToken := p.githubToken != ""
	status["github_authenticated"] = hasGitHubToken

	hasCopilotToken := p.copilotToken != ""
	status["copilot_authenticated"] = hasCopilotToken

	if hasCopilotToken {
		status["token_expires_at"] = p.copilotTokenExpiry
		status["refresh_in_seconds"] = p.refreshIn
		status["time_until_expiry"] = time.Until(p.copilotTokenExpiry).String()
	}

	return status
}

// Logout clears the authentication credentials
func (p *CopilotProvider) Logout(ctx context.Context) error {
	p.tokenMutex.Lock()
	defer p.tokenMutex.Unlock()

	// Stop token refresh loop
	if p.cancelRefresh != nil {
		p.cancelRefresh()
		p.cancelRefresh = nil
	}

	p.githubToken = ""
	p.copilotToken = ""
	p.copilotTokenExpiry = time.Time{}
	p.refreshIn = 0

	newConfig := p.GetConfig()
	newConfig.APIKey = ""
	return p.Configure(newConfig)
}

// SupportsToolCalling returns true if the provider supports tool calling
func (p *CopilotProvider) SupportsToolCalling() bool {
	return true
}

// SupportsStreaming returns true if the provider supports streaming
func (p *CopilotProvider) SupportsStreaming() bool {
	return true
}

// SupportsResponsesAPI returns false as Copilot doesn't support the Responses API
func (p *CopilotProvider) SupportsResponsesAPI() bool {
	return false
}

// GetToolFormat returns the OpenAI tool format (Copilot is OpenAI-compatible)
func (p *CopilotProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

// InvokeServerTool invokes a server tool (not yet implemented for Copilot)
func (p *CopilotProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, fmt.Errorf("tool invocation not yet implemented for Copilot provider")
}

// TestConnectivity performs a lightweight connectivity test
func (p *CopilotProvider) TestConnectivity(ctx context.Context) error {
	return p.TestConnectivityWithOptions(ctx, false)
}

// TestConnectivityWithOptions performs a connectivity test with optional cache bypass
func (p *CopilotProvider) TestConnectivityWithOptions(ctx context.Context, bypassCache bool) error {
	return p.connectivityCache.TestConnectivity(
		ctx,
		types.ProviderTypeCopilot,
		p.performConnectivityTest,
		bypassCache,
	)
}

// RefreshAllOAuthTokens refreshes the Copilot token
func (p *CopilotProvider) RefreshAllOAuthTokens(ctx context.Context) error {
	return p.refreshCopilotToken(ctx)
}

// GetBaseURL returns the appropriate base URL based on account type
func (p *CopilotProvider) GetBaseURL() string {
	switch p.config.AccountType {
	case AccountTypeBusiness:
		return CopilotBusinessBaseURL
	case AccountTypeEnterprise:
		return CopilotEnterpriseBaseURL
	default:
		return CopilotBaseURL
	}
}

// HealthCheck performs a health check on the provider
func (p *CopilotProvider) HealthCheck(ctx context.Context) error {
	if !p.IsAuthenticated() {
		return types.NewAuthError(types.ProviderTypeCopilot, "not authenticated").
			WithOperation("health_check")
	}

	// Try a minimal API call
	return p.performConnectivityTest(ctx)
}

// startTokenRefreshLoop starts a background goroutine to refresh the Copilot token
func (p *CopilotProvider) startTokenRefreshLoop(ctx context.Context) {
	p.tokenMutex.Lock()
	if p.cancelRefresh != nil {
		p.tokenMutex.Unlock()
		return // Already running
	}

	refreshCtx, cancel := context.WithCancel(context.Background())
	p.cancelRefresh = cancel
	p.tokenMutex.Unlock()

	go func() {
		ticker := time.NewTicker(p.calculateRefreshInterval())
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := p.refreshCopilotToken(refreshCtx); err != nil {
					// Log error but continue trying
					fmt.Printf("Copilot: Failed to refresh token: %v\n", err)
				}
				// Reset ticker with new interval
				ticker.Reset(p.calculateRefreshInterval())
			case <-refreshCtx.Done():
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// calculateRefreshInterval calculates the time until next token refresh
func (p *CopilotProvider) calculateRefreshInterval() time.Duration {
	p.tokenMutex.RLock()
	defer p.tokenMutex.RUnlock()

	if p.refreshIn > 0 {
		// Refresh before expiry with a buffer
		refreshSeconds := p.refreshIn - TokenRefreshBuffer
		if refreshSeconds < 60 {
			refreshSeconds = 60 // Minimum 1 minute
		}
		return time.Duration(refreshSeconds) * time.Second
	}

	// Default fallback: 19 minutes
	return 19 * time.Minute
}

// GetCopilotToken returns the current Copilot token, refreshing if necessary
func (p *CopilotProvider) GetCopilotToken(ctx context.Context) (string, error) {
	p.tokenMutex.RLock()
	token := p.copilotToken
	expiry := p.copilotTokenExpiry
	p.tokenMutex.RUnlock()

	// If token is valid and not expiring soon, return it
	if token != "" && time.Now().Add(30*time.Second).Before(expiry) {
		return token, nil
	}

	// Token needs refresh
	if err := p.refreshCopilotToken(ctx); err != nil {
		return "", err
	}

	p.tokenMutex.RLock()
	defer p.tokenMutex.RUnlock()
	return p.copilotToken, nil
}
