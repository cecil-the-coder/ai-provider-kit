// Package copilot provides configuration and initialization for GitHub Copilot AI provider.
package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/clientpool"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/models"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// CopilotConfig represents Copilot-specific configuration
type CopilotConfig struct {
	DisplayName string `json:"display_name,omitempty"`
	// GitHub token (from OAuth device flow)
	GitHubToken string `json:"github_token,omitempty"`
	// Copilot API token (auto-refreshed)
	CopilotToken string `json:"copilot_token,omitempty"`
	// Account type: individual, business, or enterprise
	AccountType string `json:"account_type,omitempty"`
	// Base URL override
	BaseURL string `json:"base_url,omitempty"`
	// Default model
	Model string `json:"model,omitempty"`
}

// NewCopilotProvider creates a new Copilot provider
func NewCopilotProvider(config types.ProviderConfig) *CopilotProvider {
	// Track if BaseURL was explicitly provided (before setting defaults)
	explicitBaseURL := config.BaseURL != ""

	// Set defaults
	mergedConfig := config
	if mergedConfig.BaseURL == "" {
		mergedConfig.BaseURL = CopilotBaseURL
	}

	// Extract Copilot-specific config first, before setting DefaultModel
	var copilotConfig CopilotConfig
	if mergedConfig.ProviderConfig != nil {
		if displayName, ok := mergedConfig.ProviderConfig["display_name"].(string); ok {
			copilotConfig.DisplayName = displayName
		}
		if githubToken, ok := mergedConfig.ProviderConfig["github_token"].(string); ok {
			copilotConfig.GitHubToken = githubToken
		}
		if copilotToken, ok := mergedConfig.ProviderConfig["copilot_token"].(string); ok {
			copilotConfig.CopilotToken = copilotToken
		}
		if accountType, ok := mergedConfig.ProviderConfig["account_type"].(string); ok {
			copilotConfig.AccountType = accountType
		}
		if baseURL, ok := mergedConfig.ProviderConfig["base_url"].(string); ok {
			copilotConfig.BaseURL = baseURL
		}
		if model, ok := mergedConfig.ProviderConfig["model"].(string); ok {
			copilotConfig.Model = model
		}
	}

	// Set DefaultModel only if not already set and not in ProviderConfig
	if mergedConfig.DefaultModel == "" && copilotConfig.Model == "" {
		mergedConfig.DefaultModel = copilotDefaultModel
	}

	// Default account type
	if copilotConfig.AccountType == "" {
		copilotConfig.AccountType = AccountTypeIndividual
	}

	// Use base URL from config or Copilot config
	baseURL := copilotConfig.BaseURL
	if baseURL == "" {
		baseURL = mergedConfig.BaseURL
	}
	if baseURL == "" {
		// Use account type to determine base URL
		switch copilotConfig.AccountType {
		case AccountTypeBusiness:
			baseURL = CopilotBusinessBaseURL
		case AccountTypeEnterprise:
			baseURL = CopilotEnterpriseBaseURL
		default:
			baseURL = CopilotBaseURL
		}
	}

	// Store the resolved base URL in copilot config only if explicitly provided
	// This allows GetBaseURL() to fall back to account type defaults
	if copilotConfig.BaseURL != "" || explicitBaseURL {
		copilotConfig.BaseURL = baseURL
	}

	// Get shared HTTP client
	timeout := 120 * time.Second
	if mergedConfig.Timeout > 0 {
		timeout = mergedConfig.Timeout
	}
	httpClient := clientpool.GetClientWithTimeout(baseURL, timeout).Client()

	// Create provider
	provider := &CopilotProvider{
		BaseProvider:      base.NewBaseProvider("copilot", mergedConfig, httpClient, log.Default()),
		client:            httpClient,
		config:            copilotConfig,
		displayName:       copilotConfig.DisplayName,
		modelCache:        models.NewModelCache(1 * time.Hour), // 1 hour cache
		connectivityCache: common.NewDefaultConnectivityCache(),
		rateLimitHelper:   common.NewRateLimitHelper(ratelimit.NewOpenAIParser()),
		requestHandler:    base.NewDefaultRequestHandler(httpClient, nil, baseURL),
		responseParser:    base.NewDefaultResponseParser(&common.RateLimitHelper{}),
		githubToken:       copilotConfig.GitHubToken,
		copilotToken:      copilotConfig.CopilotToken,
	}

	// Initialize tokens if provided
	if copilotConfig.GitHubToken != "" && copilotConfig.CopilotToken == "" {
		// Exchange GitHub token for Copilot token
		ctx := context.Background()
		if err := provider.exchangeToken(ctx); err != nil {
			log.Printf("Copilot: Failed to exchange GitHub token for Copilot token: %v", err)
		}
	}

	// Set expiry for Copilot token if provided directly
	if provider.copilotToken != "" && provider.copilotTokenExpiry.IsZero() {
		provider.copilotTokenExpiry = time.Now().Add(24 * time.Hour)
	}

	// Start token refresh loop if we have a valid Copilot token
	if provider.copilotToken != "" {
		provider.startTokenRefreshLoop(context.Background())
	}

	return provider
}

// Authenticate handles authentication with GitHub OAuth device flow
func (p *CopilotProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	if authConfig.APIKey != "" {
		// Assume this is a pre-existing Copilot token (for testing)
		p.tokenMutex.Lock()
		p.copilotToken = authConfig.APIKey
		p.copilotTokenExpiry = time.Now().Add(24 * time.Hour) // Assume 24 hours
		p.tokenMutex.Unlock()
		return nil
	}

	// Otherwise, start OAuth device flow
	return p.startOAuthDeviceFlow(ctx)
}

// Configure updates the provider configuration
func (p *CopilotProvider) Configure(config types.ProviderConfig) error {
	// Update base provider
	if err := p.BaseProvider.Configure(config); err != nil {
		return err
	}

	// Extract Copilot-specific config
	var copilotConfig CopilotConfig
	if config.ProviderConfig != nil {
		if displayName, ok := config.ProviderConfig["display_name"].(string); ok {
			copilotConfig.DisplayName = displayName
		}
		if githubToken, ok := config.ProviderConfig["github_token"].(string); ok {
			copilotConfig.GitHubToken = githubToken
		}
		if copilotToken, ok := config.ProviderConfig["copilot_token"].(string); ok {
			copilotConfig.CopilotToken = copilotToken
		}
		if accountType, ok := config.ProviderConfig["account_type"].(string); ok {
			copilotConfig.AccountType = accountType
		}
		if baseURL, ok := config.ProviderConfig["base_url"].(string); ok {
			copilotConfig.BaseURL = baseURL
		}
		if model, ok := config.ProviderConfig["model"].(string); ok {
			copilotConfig.Model = model
		}
	}

	// Update config
	p.config = copilotConfig
	p.displayName = copilotConfig.DisplayName

	// Update tokens if provided
	if copilotConfig.GitHubToken != "" {
		p.tokenMutex.Lock()
		p.githubToken = copilotConfig.GitHubToken
		p.tokenMutex.Unlock()
	}

	if copilotConfig.CopilotToken != "" {
		p.tokenMutex.Lock()
		p.copilotToken = copilotConfig.CopilotToken
		// Set expiry if not already set or if current expiry is in the past
		if p.copilotTokenExpiry.IsZero() || time.Now().After(p.copilotTokenExpiry) {
			p.copilotTokenExpiry = time.Now().Add(24 * time.Hour)
		}
		p.tokenMutex.Unlock()
	}

	return nil
}

// GetConfig returns the current provider configuration
func (p *CopilotProvider) GetConfig() types.ProviderConfig {
	return p.BaseProvider.GetConfig()
}

// startOAuthDeviceFlow starts the GitHub OAuth device flow
func (p *CopilotProvider) startOAuthDeviceFlow(ctx context.Context) error {
	// Step 1: Request device code
	deviceCode, err := p.requestDeviceCode(ctx)
	if err != nil {
		return fmt.Errorf("failed to request device code: %w", err)
	}

	// Display instructions to user
	fmt.Printf("\n=== GitHub Copilot Authentication ===\n")
	fmt.Printf("1. Visit: %s\n", deviceCode.VerificationURI)
	fmt.Printf("2. Enter code: %s\n", deviceCode.UserCode)
	fmt.Printf("Waiting for authorization...\n\n")

	// Step 2: Poll for access token
	githubToken, err := p.pollForAccessToken(ctx, deviceCode)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Step 3: Store GitHub token and exchange for Copilot token
	p.tokenMutex.Lock()
	p.githubToken = githubToken
	p.tokenMutex.Unlock()

	if err := p.exchangeToken(ctx); err != nil {
		return fmt.Errorf("failed to exchange for Copilot token: %w", err)
	}

	// Start token refresh loop
	p.startTokenRefreshLoop(ctx)

	fmt.Printf("Authentication successful!\n")
	return nil
}

// requestDeviceCode requests a device code from GitHub
func (p *CopilotProvider) requestDeviceCode(ctx context.Context) (*GitHubDeviceCodeResponse, error) {
	reqBody := map[string]string{
		"client_id": GitHubClientID,
		"scope":     GitHubOAuthScopes,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", GitHubDeviceCodeURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var deviceCode GitHubDeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceCode); err != nil {
		return nil, err
	}

	return &deviceCode, nil
}

// pollForAccessToken polls for the access token
func (p *CopilotProvider) pollForAccessToken(ctx context.Context, deviceCode *GitHubDeviceCodeResponse) (string, error) {
	interval := time.Duration(deviceCode.Interval+1) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	expiry := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)

	for {
		select {
		case <-ticker.C:
			if time.Now().After(expiry) {
				return "", fmt.Errorf("device code expired")
			}

			token, err := p.checkAccessToken(ctx, deviceCode.DeviceCode)
			if err == nil {
				return token, nil
			}
			// Continue polling on error
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// checkAccessToken checks if the access token is available
func (p *CopilotProvider) checkAccessToken(ctx context.Context, deviceCode string) (string, error) {
	reqBody := map[string]interface{}{
		"client_id":   GitHubClientID,
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", GitHubAccessTokenURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var tokenResp GitHubAccessTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}

	if tokenResp.Error != "" {
		if tokenResp.Error == "authorization_pending" {
			return "", fmt.Errorf("pending")
		}
		return "", fmt.Errorf("oauth error: %s", tokenResp.Error)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}

	return tokenResp.AccessToken, nil
}

// exchangeToken exchanges GitHub token for Copilot token
func (p *CopilotProvider) exchangeToken(ctx context.Context) error {
	p.tokenMutex.RLock()
	githubToken := p.githubToken
	p.tokenMutex.RUnlock()

	if githubToken == "" {
		return fmt.Errorf("no GitHub token available")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", GitHubCopilotTokenURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("editor-version", "vscode/"+VSCodeVersion)
	req.Header.Set("editor-plugin-version", EditorPluginVersion)
	req.Header.Set("user-agent", UserAgent)
	req.Header.Set("x-github-api-version", GitHubAPIVersion)

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp CopilotTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}

	p.tokenMutex.Lock()
	p.copilotToken = tokenResp.Token
	p.copilotTokenExpiry = time.Unix(tokenResp.ExpiresAt, 0)
	p.refreshIn = tokenResp.RefreshIn
	p.tokenMutex.Unlock()

	return nil
}

// refreshCopilotToken refreshes the Copilot token
func (p *CopilotProvider) refreshCopilotToken(ctx context.Context) error {
	p.tokenMutex.RLock()
	githubToken := p.githubToken
	p.tokenMutex.RUnlock()

	if githubToken == "" {
		return fmt.Errorf("no GitHub token available for refresh")
	}

	return p.exchangeToken(ctx)
}

// fetchModelsFromAPI fetches models from Copilot API
func (p *CopilotProvider) fetchModelsFromAPI(ctx context.Context) ([]types.Model, error) {
	token, err := p.GetCopilotToken(ctx)
	if err != nil {
		return nil, err
	}

	url := p.GetBaseURL() + "/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	p.setCopilotHeaders(req, token)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, err
	}

	// Convert to internal Model format
	models := make([]types.Model, 0, len(modelsResp.Data))
	for _, model := range modelsResp.Data {
		if !model.ModelPickerEnabled {
			continue // Skip disabled models
		}

		models = append(models, types.Model{
			ID:                  model.ID,
			Name:                model.Name,
			Provider:            p.Type(),
			MaxTokens:           model.Capabilities.Limits.MaxOutputTokens,
			SupportsStreaming:   true,
			SupportsToolCalling: model.Capabilities.Supports.ToolCalls,
			Description:         fmt.Sprintf("%s model by %s", model.Name, model.Vendor),
		})
	}

	return models, nil
}

// getStaticFallback returns static model list
func (p *CopilotProvider) getStaticFallback() []types.Model {
	return []types.Model{
		{
			ID:                  "gpt-4o",
			Name:                "GPT-4o",
			Provider:            p.Type(),
			MaxTokens:           16384,
			SupportsStreaming:   true,
			SupportsToolCalling: true,
			Description:         "OpenAI GPT-4o multimodal model",
		},
		{
			ID:                  "gpt-4o-mini",
			Name:                "GPT-4o mini",
			Provider:            p.Type(),
			MaxTokens:           16384,
			SupportsStreaming:   true,
			SupportsToolCalling: true,
			Description:         "OpenAI GPT-4o mini model",
		},
	}
}

// setCopilotHeaders sets the required headers for Copilot API requests
func (p *CopilotProvider) setCopilotHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("copilot-integration-id", CopilotIntegrationID)
	req.Header.Set("editor-version", "vscode/"+VSCodeVersion)
	req.Header.Set("editor-plugin-version", EditorPluginVersion)
	req.Header.Set("user-agent", UserAgent)
	req.Header.Set("openai-intent", OpenAIIntent)
	req.Header.Set("x-github-api-version", GitHubAPIVersion)
	req.Header.Set("x-vscode-user-agent-library-version", VSCodeUserAgentLibrary)
	req.Header.Set("User-Agent", telemetry.GetUserAgent())
}

// performConnectivityTest performs a connectivity test
func (p *CopilotProvider) performConnectivityTest(ctx context.Context) error {
	token, err := p.GetCopilotToken(ctx)
	if err != nil {
		return types.NewAuthError(types.ProviderTypeCopilot, "failed to get Copilot token").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	url := p.GetBaseURL() + "/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeCopilot, "failed to create request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	p.setCopilotHeaders(req, token)

	resp, err := p.client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeCopilot, "connectivity test failed").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return types.NewAuthError(types.ProviderTypeCopilot, "invalid Copilot token").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeCopilot, resp.StatusCode,
			string(body)).
			WithOperation("test_connectivity")
	}

	return nil
}

// hasVisionContent checks if messages contain vision content
func (p *CopilotProvider) hasVisionContent(messages []ChatMessage) bool {
	for _, msg := range messages {
		if parts, ok := msg.Content.([]ContentPart); ok {
			for _, part := range parts {
				if part.Type == "image_url" {
					return true
				}
			}
		}
	}
	return false
}

// getInitiator returns the initiator based on message roles
func (p *CopilotProvider) getInitiator(messages []ChatMessage) string {
	for _, msg := range messages {
		if msg.Role == "assistant" || msg.Role == "tool" {
			return "agent"
		}
	}
	return "user"
}

// NewModelCache creates a new model cache
func NewModelCache(ttl time.Duration) *modelCache {
	return &modelCache{
		ttl: ttl,
	}
}

// modelCache caches models
type modelCache struct {
	models    []types.Model
	fetchedAt time.Time
	ttl       time.Duration
	mu        sync.RWMutex
}

// GetModels gets models from cache or fetches them
func (c *modelCache) GetModels(fetchFunc func() ([]types.Model, error), fallbackFunc func() []types.Model) ([]types.Model, error) {
	c.mu.RLock()
	if time.Since(c.fetchedAt) < c.ttl && len(c.models) > 0 {
		models := c.models
		c.mu.RUnlock()
		return models, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check
	if time.Since(c.fetchedAt) < c.ttl && len(c.models) > 0 {
		return c.models, nil
	}

	models, err := fetchFunc()
	if err != nil {
		// Return fallback on error
		return fallbackFunc(), nil
	}

	c.models = models
	c.fetchedAt = time.Now()
	return models, nil
}

// NewConnectivityCache creates a new connectivity cache
func NewConnectivityCache() *connectivityCache {
	return &connectivityCache{
		cache: make(map[string]*cacheEntry),
		ttl:   30 * time.Second,
	}
}

// connectivityCache caches connectivity test results
type connectivityCache struct {
	cache map[string]*cacheEntry
	mu    sync.RWMutex
	ttl   time.Duration
}

type cacheEntry struct {
	result   error
	lastSeen time.Time
}

// TestConnectivity tests connectivity with caching
func (c *connectivityCache) TestConnectivity(ctx context.Context, providerType types.ProviderType, testFunc func(context.Context) error, bypassCache bool) error {
	key := string(providerType)

	if !bypassCache {
		c.mu.RLock()
		entry, exists := c.cache[key]
		c.mu.RUnlock()

		if exists && time.Since(entry.lastSeen) < c.ttl {
			return entry.result
		}
	}

	result := testFunc(ctx)

	c.mu.Lock()
	c.cache[key] = &cacheEntry{
		result:   result,
		lastSeen: time.Now(),
	}
	c.mu.Unlock()

	return result
}
