// Package anthropic provides configuration and initialization for Anthropic Claude AI provider.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/clientpool"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/internal/common/config"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/models"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Constants for Anthropic models
const (
	anthropicDefaultModel = "claude-sonnet-4-5"       // Default model for chat completions
	anthropicTestModel    = "claude-3-haiku-20240307" // Lightweight model for connectivity testing
)

// AnthropicConfig represents Anthropic-specific configuration
type AnthropicConfig struct {
	DisplayName   string   `json:"display_name,omitempty"`
	APIKey        string   `json:"api_key"`
	APIKeys       []string `json:"api_keys,omitempty"`
	BaseURL       string   `json:"base_url,omitempty"`
	Model         string   `json:"model,omitempty"`
	OAuthClientID string   `json:"oauth_client_id,omitempty"`
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(config types.ProviderConfig) *AnthropicProvider {
	// Use the shared config helper
	configHelper := commonconfig.NewConfigHelper("Anthropic", types.ProviderTypeAnthropic)

	// Merge with defaults and extract configuration
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Extract Anthropic-specific config
	var anthropicConfig AnthropicConfig
	if err := configHelper.ExtractProviderSpecificConfig(mergedConfig, &anthropicConfig); err != nil {
		// If extraction fails, use empty config and let helper handle defaults
		anthropicConfig = AnthropicConfig{}
	}

	// Apply top-level overrides using helper
	if err := configHelper.ApplyTopLevelOverrides(mergedConfig, &anthropicConfig); err != nil {
		// In constructor, we log the error but continue with default config
		log.Printf("Warning: failed to apply top-level overrides in NewAnthropicProvider: %v", err)
	}

	// Set default OAuth client ID if not configured
	if anthropicConfig.OAuthClientID == "" {
		anthropicConfig.OAuthClientID = configHelper.ExtractDefaultOAuthClientID()
	}

	// Determine base URL
	baseURL := anthropicConfig.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	// Get shared HTTP client from pool keyed by base URL
	timeout := configHelper.ExtractTimeout(mergedConfig)
	httpClient := clientpool.GetClientWithTimeout(baseURL, timeout)

	// Extract the underlying http.Client for compatibility with existing code
	client := httpClient.Client()

	// Create auth helper
	authHelper := auth.NewAuthHelper("anthropic", mergedConfig, client)

	// Create rate limit helper
	rateLimitHelper := common.NewRateLimitHelper(ratelimit.NewAnthropicParser())

	provider := &AnthropicProvider{
		BaseProvider:      base.NewBaseProvider("anthropic", mergedConfig, client, log.Default()),
		authHelper:        authHelper,
		client:            client,
		displayName:       anthropicConfig.DisplayName,
		config:            anthropicConfig,
		modelCache:        models.NewModelCache(6 * time.Hour), // 6 hour cache for Anthropic
		connectivityCache: common.NewDefaultConnectivityCache(),
		rateLimitHelper:   rateLimitHelper,
		requestHandler:    base.NewDefaultRequestHandler(client, authHelper, baseURL),
		responseParser:    base.NewDefaultResponseParser(rateLimitHelper),
	}

	// Setup authentication with initial config
	provider.authHelper.SetupAPIKeys()
	refreshFactory := auth.NewRefreshFuncFactory("anthropic", client)
	provider.authHelper.SetupOAuth(refreshFactory.CreateAnthropicRefreshFunc())

	return provider
}

// Authenticate updates the provider's authentication configuration
func (p *AnthropicProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	if err := p.authHelper.ValidateAuthConfig(authConfig); err != nil {
		return err
	}

	// Update config with new authentication
	newConfig := p.authHelper.Config
	newConfig.APIKey = authConfig.APIKey
	newConfig.BaseURL = authConfig.BaseURL
	newConfig.DefaultModel = authConfig.DefaultModel

	return p.Configure(newConfig)
}

// Configure updates the provider configuration
func (p *AnthropicProvider) Configure(config types.ProviderConfig) error {
	// Use the shared config helper for validation and extraction
	configHelper := commonconfig.NewConfigHelper("Anthropic", types.ProviderTypeAnthropic)

	// Validate configuration
	validation := configHelper.ValidateProviderConfig(config)
	if !validation.Valid {
		return fmt.Errorf("configuration validation failed: %s", validation.Errors[0])
	}

	// Merge with defaults
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Extract Anthropic-specific config
	var anthropicConfig AnthropicConfig
	if err := configHelper.ExtractProviderSpecificConfig(mergedConfig, &anthropicConfig); err != nil {
		// If extraction fails, use empty config and let helper handle defaults
		anthropicConfig = AnthropicConfig{}
	}

	// Apply top-level overrides using helper
	if err := configHelper.ApplyTopLevelOverrides(mergedConfig, &anthropicConfig); err != nil {
		return fmt.Errorf("failed to apply top-level overrides: %w", err)
	}

	// Set default OAuth client ID if not configured
	if anthropicConfig.OAuthClientID == "" {
		anthropicConfig.OAuthClientID = configHelper.ExtractDefaultOAuthClientID()
	}

	// Update auth helper configuration
	p.authHelper.Config = mergedConfig

	// Re-setup authentication with new config
	p.authHelper.SetupAPIKeys()
	refreshFactory := auth.NewRefreshFuncFactory("anthropic", p.client)
	p.authHelper.SetupOAuth(refreshFactory.CreateAnthropicRefreshFunc())

	p.displayName = anthropicConfig.DisplayName
	p.config = anthropicConfig

	return p.BaseProvider.Configure(mergedConfig)
}

// fetchModelsFromAPI fetches models from Anthropic API
func (p *AnthropicProvider) fetchModelsFromAPI(ctx context.Context) ([]types.Model, error) {
	// Define OAuth operation for fetching models
	oauthOperation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
		return p.fetchModelsWithOAuth(ctx, cred.AccessToken)
	}

	// Define API key operation for fetching models
	apiKeyOperation := func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
		return p.fetchModelsWithAPIKey(ctx, apiKey)
	}

	// Use auth helper to execute with automatic failover
	modelsJSON, _, err := p.authHelper.ExecuteWithAuth(ctx, types.GenerateOptions{}, oauthOperation, apiKeyOperation)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeAnthropic, "failed to fetch models").
			WithOperation("GetModels").
			WithOriginalErr(err)
	}

	var modelsResp AnthropicModelsResponse
	if err := json.Unmarshal([]byte(modelsJSON), &modelsResp); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeAnthropic, "failed to parse models response").
			WithOperation("GetModels").
			WithOriginalErr(err)
	}

	// Convert to internal Model format
	var models []types.Model
	for _, model := range modelsResp.Data {
		if model.Type == "model" {
			models = append(models, types.Model{
				ID:       model.ID,
				Name:     model.DisplayName,
				Provider: p.Type(),
			})
		}
	}

	return models, nil
}

// fetchModelsHelper is a shared function to fetch models with any auth type
func (p *AnthropicProvider) fetchModelsHelper(ctx context.Context, authType string, credential string) (string, *types.Usage, error) {
	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/models?limit=1000"

	// Create request with proper authentication headers
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Use auth helper to set headers based on auth type
	p.authHelper.SetAuthHeaders(req, credential, authType)
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	resp, err := p.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Check status code using response parser
	if err := p.responseParser.CheckStatusCode(resp); err != nil {
		return "", nil, fmt.Errorf("failed to fetch models: %w", err)
	}

	// Read body using response parser
	body, err := p.responseParser.ReadBody(resp)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, &types.Usage{}, nil
}

// fetchModelsWithOAuth fetches models using OAuth authentication
func (p *AnthropicProvider) fetchModelsWithOAuth(ctx context.Context, accessToken string) (string, *types.Usage, error) {
	return p.fetchModelsHelper(ctx, "oauth", accessToken)
}

// fetchModelsWithAPIKey fetches models using API key authentication
func (p *AnthropicProvider) fetchModelsWithAPIKey(ctx context.Context, apiKey string) (string, *types.Usage, error) {
	return p.fetchModelsHelper(ctx, "api_key", apiKey)
}

// enrichModels adds metadata to models
func (p *AnthropicProvider) enrichModels(models []types.Model) []types.Model {
	// Static metadata for known models
	metadata := map[string]struct {
		maxTokens   int
		description string
	}{
		// Claude 4 models
		"claude-opus-4-5-20251101":   {200000, "Claude Opus 4.5 - Most powerful model for complex reasoning"},
		"claude-opus-4-5":            {200000, "Claude Opus 4.5 - Most powerful model for complex reasoning"},
		"claude-opus-4-1-20250805":   {200000, "Claude Opus 4.1 - Advanced reasoning model"},
		"claude-opus-4-1":            {200000, "Claude Opus 4.1 - Advanced reasoning model"},
		"claude-sonnet-4-5-20250929": {200000, "Claude Sonnet 4.5 - Best balance of intelligence and speed"},
		"claude-sonnet-4-5":          {200000, "Claude Sonnet 4.5 - Best balance of intelligence and speed"},
		"claude-sonnet-4-20250514":   {200000, "Claude Sonnet 4 - Balanced performance model"},
		"claude-sonnet-4":            {200000, "Claude Sonnet 4 - Balanced performance model"},
		"claude-haiku-4-5-20251001":  {200000, "Claude Haiku 4.5 - Fastest model for quick tasks"},
		"claude-haiku-4-5":           {200000, "Claude Haiku 4.5 - Fastest model for quick tasks"},

		// Claude 3.5 models
		"claude-3-5-sonnet-20241022": {200000, "Anthropic's most capable Sonnet model, updated for October 2024"},
		"claude-3-5-haiku-20241022":  {200000, "Anthropic's fastest Haiku model, updated for October 2024"},

		// Claude 3 models
		"claude-3-opus-20240229":   {200000, "Anthropic's most powerful model for complex tasks"},
		"claude-3-sonnet-20240229": {200000, "Anthropic's balanced model for workloads"},
		"claude-3-haiku-20240307":  {200000, "Anthropic's fastest and most compact model"},
	}

	enriched := make([]types.Model, len(models))
	for i, model := range models {
		enriched[i] = types.Model{
			ID:                  model.ID,
			Name:                model.Name,
			Provider:            p.Type(),
			SupportsStreaming:   true,
			SupportsToolCalling: true,
		}

		// Add metadata if available
		if meta, ok := metadata[model.ID]; ok {
			enriched[i].MaxTokens = meta.maxTokens
			enriched[i].Description = meta.description
		} else {
			// Default values for unknown models
			enriched[i].MaxTokens = 200000
			enriched[i].Description = "Anthropic Claude model"
		}
	}
	return enriched
}

// getStaticFallback returns static model list (used for OAuth tokens)
func (p *AnthropicProvider) getStaticFallback() []types.Model {
	return []types.Model{
		// Opus 4.5
		{ID: "claude-opus-4-5-20251101", Name: "Claude Opus 4.5", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Opus 4.5 - Most powerful model for complex reasoning"},
		{ID: "claude-opus-4-5", Name: "Claude Opus 4.5", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Opus 4.5 - Most powerful model for complex reasoning"},

		// Opus 4.1
		{ID: "claude-opus-4-1-20250805", Name: "Claude Opus 4.1", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Opus 4.1 - Advanced reasoning model"},
		{ID: "claude-opus-4-1", Name: "Claude Opus 4.1", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Opus 4.1 - Advanced reasoning model"},

		// Sonnet 4.5
		{ID: "claude-sonnet-4-5-20250929", Name: "Claude Sonnet 4.5", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Sonnet 4.5 - Best balance of intelligence and speed"},
		{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Sonnet 4.5 - Best balance of intelligence and speed"},

		// Sonnet 4
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Sonnet 4 - Balanced performance model"},
		{ID: "claude-sonnet-4", Name: "Claude Sonnet 4", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Sonnet 4 - Balanced performance model"},

		// Haiku 4.5
		{ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Haiku 4.5 - Fastest model for quick tasks"},
		{ID: "claude-haiku-4-5", Name: "Claude Haiku 4.5", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Haiku 4.5 - Fastest model for quick tasks"},

		// Legacy Claude 3.5 models (for backwards compatibility)
		{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet (Oct 2024)", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's most capable Sonnet model, updated for October 2024"},
		{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku (Oct 2024)", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's fastest Haiku model, updated for October 2024"},
		{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's most powerful model for complex tasks"},
		{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's balanced model for workloads"},
		{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's fastest and most compact model"},
	}
}

// performConnectivityTest performs the actual connectivity test without caching
func (p *AnthropicProvider) performConnectivityTest(ctx context.Context) error {
	// For OAuth tokens starting with sk-ant-oat (Anthropic's OAuth prefix),
	// the models endpoint doesn't work, so we need to test differently
	// Use GetCredentialsWithContext to fetch fresh credentials from dynamic provider
	if p.authHelper.OAuthManager != nil && len(p.authHelper.OAuthManager.GetCredentialsWithContext(ctx)) > 0 {
		creds := p.authHelper.OAuthManager.GetCredentialsWithContext(ctx)
		if len(creds) > 0 && strings.HasPrefix(creds[0].AccessToken, "sk-ant-oat") {
			// For Anthropic OAuth, test with a minimal messages API call
			return p.testConnectivityWithMessagesAPI(ctx, creds[0].AccessToken, "oauth")
		}
	}

	// For API keys or non-Anthropic OAuth tokens, use the models endpoint
	// Check if we have any authentication credentials
	hasAPIKeys := p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0
	// Use GetCredentialsWithContext to fetch fresh credentials from dynamic provider
	hasOAuth := p.authHelper.OAuthManager != nil && len(p.authHelper.OAuthManager.GetCredentialsWithContext(ctx)) > 0

	if !hasAPIKeys && !hasOAuth {
		return types.NewAuthError(types.ProviderTypeAnthropic, "no API keys or OAuth credentials configured").
			WithOperation("test_connectivity")
	}

	// Try API keys first
	if hasAPIKeys {
		keys := p.authHelper.KeyManager.GetKeys()
		if err := p.testConnectivityWithModelsEndpoint(ctx, keys[0], "api_key"); err == nil {
			return nil
		}
	}

	// Try OAuth credentials
	if hasOAuth {
		// Use GetCredentialsWithContext to fetch fresh credentials from dynamic provider
		creds := p.authHelper.OAuthManager.GetCredentialsWithContext(ctx)
		return p.testConnectivityWithModelsEndpoint(ctx, creds[0].AccessToken, "oauth")
	}

	return types.NewAuthError(types.ProviderTypeAnthropic, "no valid authentication credentials available").
		WithOperation("test_connectivity")
}

// testConnectivityWithModelsEndpoint tests connectivity using the /v1/models endpoint
func (p *AnthropicProvider) testConnectivityWithModelsEndpoint(ctx context.Context, credential, authType string) error {
	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/models?limit=1" // Limit to 1 model for lightweight test

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeAnthropic, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Set authentication headers
	p.authHelper.SetAuthHeaders(req, credential, authType)
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Make the request with a shorter timeout for connectivity testing
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeAnthropic, "connectivity test failed").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		return types.NewAuthError(types.ProviderTypeAnthropic, "invalid authentication credentials").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(types.ProviderTypeAnthropic, "credentials do not have access to models endpoint").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeAnthropic, resp.StatusCode,
			fmt.Sprintf("connectivity test failed: %s", string(body))).
			WithOperation("test_connectivity")
	}

	// Try to parse a small portion of the response to ensure it's valid JSON
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1024))
	var testResponse struct {
		Data []interface{} `json:"data"`
	}
	if err := decoder.Decode(&testResponse); err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeAnthropic, "invalid response from models endpoint").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	return nil
}

// testConnectivityWithMessagesAPI tests connectivity using a minimal messages API call
func (p *AnthropicProvider) testConnectivityWithMessagesAPI(ctx context.Context, accessToken, authType string) error {
	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/messages"

	// Create a minimal request for connectivity testing
	minimalRequest := map[string]interface{}{
		"model":      anthropicTestModel, // Use smallest model
		"max_tokens": 1,                  // Minimal token usage
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "Hi", // Minimal content
			},
		},
	}

	jsonBody, err := json.Marshal(minimalRequest)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeAnthropic, "failed to marshal test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeAnthropic, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Set authentication headers
	p.authHelper.SetAuthHeaders(req, accessToken, authType)
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Make the request with a shorter timeout for connectivity testing
	client := &http.Client{
		Timeout: 15 * time.Second, // Slightly longer for message API
	}

	resp, err := client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeAnthropic, "connectivity test failed").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		return types.NewAuthError(types.ProviderTypeAnthropic, "invalid OAuth token").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(types.ProviderTypeAnthropic, "OAuth token does not have access to messages endpoint").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeAnthropic, resp.StatusCode,
			fmt.Sprintf("connectivity test failed: %s", string(body))).
			WithOperation("test_connectivity")
	}

	// Try to parse a small portion of the response to ensure it's valid
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1024))
	var testResponse struct {
		ID      string        `json:"id"`
		Type    string        `json:"type"`
		Role    string        `json:"role"`
		Content []interface{} `json:"content"`
	}
	if err := decoder.Decode(&testResponse); err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeAnthropic, "invalid response from messages endpoint").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Verify we got a valid message response
	if testResponse.Type != "message" || testResponse.Role != "assistant" {
		return types.NewInvalidRequestError(types.ProviderTypeAnthropic, "unexpected response format from messages endpoint").
			WithOperation("test_connectivity")
	}

	return nil
}
