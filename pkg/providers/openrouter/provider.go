// Package openrouter provides an OpenRouter AI provider implementation.
// It includes support for model selection, API key management, OAuth authentication,
// and specialized features for the OpenRouter platform.
package openrouter

import (
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
	pkghttp "github.com/cecil-the-coder/ai-provider-kit/internal/http"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/internal/common/config"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/models"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// OpenRouterProvider implements the Provider interface for OpenRouter
type OpenRouterProvider struct {
	*base.BaseProvider
	config            ProviderConfig
	client            *http.Client
	httpClient        *pkghttp.HTTPClient
	authHelper        *auth.AuthHelper
	modelSelector     *ModelSelector
	lastUsedModel     string
	lastUsage         *types.Usage
	mutex             sync.RWMutex
	modelCache        *models.ModelCache
	connectivityCache *common.ConnectivityCache

	// Configuration fields
	apiKey        string
	baseURL       string
	siteURL       string
	siteName      string
	models        []string
	modelStrategy string
	freeOnly      bool

	// Rate limiting (header-based tracking)
	rateLimitHelper  *common.RateLimitHelper
	rateLimitTracker *common.RateLimitTracker
}

// ProviderConfig holds OpenRouter-specific configuration
type ProviderConfig struct {
	APIKey        string   `json:"api_key"`
	APIKeys       []string `json:"api_keys,omitempty"`
	Model         string   `json:"model,omitempty"`
	Models        []string `json:"models,omitempty"`
	ModelStrategy string   `json:"model_strategy,omitempty"`
	FreeOnly      bool     `json:"free_only,omitempty"`
	SiteURL       string   `json:"site_url,omitempty"`
	SiteName      string   `json:"site_name,omitempty"`
	BaseURL       string   `json:"base_url,omitempty"`

	// OAuth configuration
	OAuthCallbackURL string `json:"oauth_callback_url,omitempty"`
}

// NewOpenRouterProvider creates a new OpenRouter provider
func NewOpenRouterProvider(config types.ProviderConfig) *OpenRouterProvider {
	// Extract OpenRouter-specific config
	var providerConfig ProviderConfig
	if config.ProviderConfig != nil {
		configBytes, _ := json.Marshal(config.ProviderConfig)
		_ = json.Unmarshal(configBytes, &providerConfig)
	}

	// Set defaults
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = config.APIKeyEnv
	}
	if apiKey == "" {
		apiKey = providerConfig.APIKey
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = providerConfig.BaseURL
	}
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api"
	}

	siteURL := providerConfig.SiteURL
	if siteURL == "" {
		siteURL = "https://github.com/cecil-the-coder/mcp-code-api"
	}

	siteName := providerConfig.SiteName
	if siteName == "" {
		siteName = "MCP Code API"
	}

	modelList := providerConfig.Models
	if len(modelList) == 0 && providerConfig.Model != "" {
		modelList = []string{providerConfig.Model}
	}
	if len(modelList) == 0 && config.DefaultModel != "" {
		modelList = []string{config.DefaultModel}
	}
	if len(modelList) == 0 {
		modelList = []string{"qwen/qwen3-coder"}
	}

	modelStrategy := providerConfig.ModelStrategy
	if modelStrategy == "" {
		modelStrategy = "failover"
	}

	// Get shared HTTP client from pool keyed by base URL
	httpClient := clientpool.GetClientWithTimeout(baseURL, 60*time.Second)
	client := httpClient.Client()

	// Create auth helper
	authHelper := auth.NewAuthHelper("openrouter", config, client)

	// Setup API keys using shared helper
	authHelper.SetupAPIKeys()

	// Create parser once for both helper and tracker
	openRouterParser := ratelimit.NewOpenRouterParser()

	provider := &OpenRouterProvider{
		BaseProvider:      base.NewBaseProvider("openrouter", config, client, log.Default()),
		config:            providerConfig,
		client:            client,
		httpClient:        httpClient,
		authHelper:        authHelper,
		modelSelector:     NewModelSelector(modelList, modelStrategy),
		apiKey:            apiKey,
		baseURL:           baseURL,
		siteURL:           siteURL,
		siteName:          siteName,
		models:            modelList,
		modelStrategy:     modelStrategy,
		freeOnly:          providerConfig.FreeOnly,
		modelCache:        models.NewModelCache(commonconfig.GetModelCacheTTL(types.ProviderTypeOpenRouter)),
		connectivityCache: common.NewDefaultConnectivityCache(),
		rateLimitHelper:   common.NewRateLimitHelper(openRouterParser),
		rateLimitTracker:  common.NewRateLimitTracker(openRouterParser),
	}

	return provider
}

func (p *OpenRouterProvider) Name() string {
	return "OpenRouter"
}

func (p *OpenRouterProvider) Type() types.ProviderType {
	return types.ProviderTypeOpenRouter
}

func (p *OpenRouterProvider) Description() string {
	return "OpenRouter - Universal AI model gateway with access to all major models"
}

func (p *OpenRouterProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	// Use the shared model cache utility
	return p.modelCache.GetModels(
		func() ([]types.Model, error) {
			// Fetch from API (already enriched in fetchModelsFromAPI)
			models, err := p.fetchModelsFromAPI(ctx)
			if err != nil {
				log.Printf("OpenRouter: Failed to fetch models from API: %v", err)
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

// fetchModelsFromAPI fetches models from OpenRouter API
func (p *OpenRouterProvider) fetchModelsFromAPI(ctx context.Context) ([]types.Model, error) {
	if !p.authHelper.IsAuthenticated() {
		return nil, types.NewAuthError(types.ProviderTypeOpenRouter, "no OpenRouter API key configured").
			WithOperation("fetch_models")
	}

	// Get API key from auth helper
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		return nil, types.NewAuthError(types.ProviderTypeOpenRouter, "no valid API key available").
			WithOperation("fetch_models")
	}

	apiKey := p.authHelper.KeyManager.GetKeys()[0] // Simple for now, could be improved
	if apiKey == "" {
		return nil, types.NewAuthError(types.ProviderTypeOpenRouter, "no valid API key available").
			WithOperation("fetch_models")
	}

	url := p.baseURL + "/v1/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenRouter, "failed to create request").
			WithOperation("fetch_models").
			WithOriginalErr(err)
	}

	// Use auth helper to set headers
	p.authHelper.SetAuthHeaders(req, apiKey, "api_key")
	req.Header.Set("HTTP-Referer", p.siteURL)
	req.Header.Set("X-Title", p.siteName)

	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenRouter, "failed to fetch models").
			WithOperation("fetch_models").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, types.NewProviderError(types.ProviderTypeOpenRouter, types.ClassifyHTTPError(resp.StatusCode), fmt.Sprintf("failed to fetch models: HTTP %d - %s", resp.StatusCode, string(body))).
			WithOperation("fetch_models").
			WithStatusCode(resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenRouter, "failed to read response").
			WithOperation("fetch_models").
			WithOriginalErr(err)
	}

	var modelsResp OpenRouterModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "failed to parse models response").
			WithOperation("fetch_models").
			WithOriginalErr(err)
	}

	// Convert to internal Model format with pricing data
	models := make([]types.Model, 0, len(modelsResp.Data))
	for _, model := range modelsResp.Data {
		models = append(models, types.Model{
			ID:                  model.ID,
			Name:                model.Name,
			Provider:            p.Type(),
			MaxTokens:           model.ContextLength,
			SupportsStreaming:   true,
			SupportsToolCalling: true,
			Description:         model.Description,
		})
	}

	return models, nil
}

// getStaticFallback returns static model list
func (p *OpenRouterProvider) getStaticFallback() []types.Model {
	return []types.Model{
		{ID: "qwen/qwen3-coder", Name: "Qwen 3 Coder", Provider: p.Type(), MaxTokens: 32768, SupportsStreaming: true, SupportsToolCalling: true, Description: "Qwen's coding model"},
		{ID: "qwen/qwen2.5-coder-32b-instruct", Name: "Qwen 2.5 Coder 32B", Provider: p.Type(), MaxTokens: 32768, SupportsStreaming: true, SupportsToolCalling: true, Description: "Qwen's advanced coding model"},
		{ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's Claude 3.5 Sonnet"},
		{ID: "openai/gpt-4o", Name: "GPT-4o", Provider: p.Type(), MaxTokens: 128000, SupportsStreaming: true, SupportsToolCalling: true, Description: "OpenAI's GPT-4o"},
		{ID: "google/gemini-pro-1.5", Name: "Gemini Pro 1.5", Provider: p.Type(), MaxTokens: 2097152, SupportsStreaming: true, SupportsToolCalling: true, Description: "Google's Gemini Pro 1.5"},
		{ID: "meta-llama/llama-3.1-70b-instruct", Name: "Llama 3.1 70B", Provider: p.Type(), MaxTokens: 131072, SupportsStreaming: true, SupportsToolCalling: true, Description: "Meta's Llama 3.1 70B"},
		{ID: "deepseek/deepseek-coder-v3.1:free", Name: "Deepseek Coder v3.1 (Free)", Provider: p.Type(), MaxTokens: 128000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Deepseek's free coding model"},
	}
}

func (p *OpenRouterProvider) GetDefaultModel() string {
	config := p.GetConfig()
	if config.DefaultModel != "" {
		return config.DefaultModel
	}
	if p.config.Model != "" {
		return p.config.Model
	}
	if len(p.models) > 0 {
		return p.models[0]
	}
	return "qwen/qwen3-coder"
}

func (p *OpenRouterProvider) GenerateChatCompletion(
	ctx context.Context,
	options types.GenerateOptions,
) (types.ChatCompletionStream, error) {
	if !p.authHelper.IsAuthenticated() {
		return nil, types.NewAuthError(types.ProviderTypeOpenRouter, "no OpenRouter API key configured").
			WithOperation("generate_chat_completion")
	}

	// Prepare the request first to get the model name
	requestData, err := p.prepareRequest(options)
	if err != nil {
		return nil, err
	}

	// Option 1: Check via /api/v1/key endpoint (existing approach)
	rateLimits, err := p.GetRateLimits(ctx)
	if err != nil {
		// Continue anyway, but log the warning
	} else {
		// Check if we're on free tier with limited requests
		if rateLimits.IsFreeTier && rateLimits.LimitRemaining != nil && *rateLimits.LimitRemaining <= 0 {
			return nil, types.NewRateLimitError(types.ProviderTypeOpenRouter, 0).
				WithOperation("generate_chat_completion").
				WithMessage("rate limit exceeded (free tier limit reached)")
		}
		// Check if we have credits remaining (for paid tiers)
		if rateLimits.Limit != nil && rateLimits.LimitRemaining != nil && *rateLimits.LimitRemaining <= 0 {
			return nil, types.NewRateLimitError(types.ProviderTypeOpenRouter, 0).
				WithOperation("generate_chat_completion").
				WithMessage("credit limit exceeded")
		}
	}

	// Option 2: Check via header-based tracker (new approach)
	// This uses cached rate limit information from previous responses
	p.rateLimitHelper.CheckRateLimitAndWait(requestData.Model, 0)

	// Handle streaming
	if options.Stream {
		// Try to get an API key and make streaming call
		if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
			return nil, types.NewAuthError(types.ProviderTypeOpenRouter, "no API key available for streaming").
				WithOperation("generate_chat_completion")
		}

		apiKey := p.authHelper.KeyManager.GetKeys()[0] // Simple for now, could be improved
		stream, err := p.makeStreamingAPICallWithKey(ctx, requestData, apiKey)
		if err != nil {
			return nil, err
		}
		return stream, nil
	}

	// Non-streaming path: Execute with failover
	var responseMessage types.ChatMessage
	var usage *types.Usage
	var responseContent string
	var callErr error

	if p.authHelper.KeyManager != nil {
		responseContent, usage, callErr = p.authHelper.KeyManager.ExecuteWithFailover(ctx, func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
			response, err := p.makeAPICallWithKey(ctx, requestData, apiKey)
			if err != nil {
				return "", nil, err
			}

			openrouterMsg := response.Choices[0].Message

			// Convert to universal format
			responseMessage = types.ChatMessage{
				Role:    openrouterMsg.Role,
				Content: openrouterMsg.Content,
			}

			// Convert tool calls if present
			if len(openrouterMsg.ToolCalls) > 0 {
				responseMessage.ToolCalls = convertOpenRouterToolCallsToUniversal(openrouterMsg.ToolCalls)
			}

			usage := &types.Usage{
				PromptTokens:     response.Usage.PromptTokens,
				CompletionTokens: response.Usage.CompletionTokens,
				TotalTokens:      response.Usage.TotalTokens,
			}

			return openrouterMsg.Content, usage, nil
		})
	} else {
		callErr = types.NewAuthError(types.ProviderTypeOpenRouter, "no authentication manager available").
			WithOperation("generate_chat_completion")
	}

	if callErr != nil {
		return nil, callErr
	}

	// Store usage information
	p.mutex.Lock()
	p.lastUsage = usage
	p.mutex.Unlock()

	// Return streaming response with tool calls if present
	var usageValue types.Usage
	if usage != nil {
		usageValue = *usage
	}

	chunk := types.ChatCompletionChunk{
		Content: responseContent,
		Done:    true,
		Usage:   usageValue,
	}

	// Include tool calls if present
	if len(responseMessage.ToolCalls) > 0 {
		chunk.Choices = []types.ChatChoice{
			{
				Message: responseMessage,
			},
		}
	}

	return &MockStream{
		chunks: []types.ChatCompletionChunk{chunk},
	}, nil
}

func (p *OpenRouterProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "tool invocation not yet implemented for OpenRouter provider").
		WithOperation("invoke_server_tool")
}

func (p *OpenRouterProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	if authConfig.Method != types.AuthMethodAPIKey {
		return types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "OpenRouter only supports API key authentication").
			WithOperation("authenticate")
	}

	newConfig := p.GetConfig()
	newConfig.APIKey = authConfig.APIKey
	newConfig.BaseURL = authConfig.BaseURL
	newConfig.DefaultModel = authConfig.DefaultModel

	return p.Configure(newConfig)
}

func (p *OpenRouterProvider) IsAuthenticated() bool {
	return p.authHelper.IsAuthenticated()
}

func (p *OpenRouterProvider) Logout(ctx context.Context) error {
	p.authHelper.ClearAuthentication()
	newConfig := p.GetConfig()
	newConfig.APIKey = ""
	return p.Configure(newConfig)
}

func (p *OpenRouterProvider) Configure(config types.ProviderConfig) error {
	if config.Type != types.ProviderTypeOpenRouter {
		return types.NewInvalidRequestError(types.ProviderTypeOpenRouter, fmt.Sprintf("invalid provider type for OpenRouter: %s", config.Type)).
			WithOperation("configure")
	}

	// Extract OpenRouter-specific config
	var providerConfig ProviderConfig
	if config.ProviderConfig != nil {
		configBytes, _ := json.Marshal(config.ProviderConfig)
		_ = json.Unmarshal(configBytes, &providerConfig)
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Update configuration
	p.config = providerConfig
	p.apiKey = config.APIKey
	if p.apiKey == "" {
		p.apiKey = config.APIKeyEnv
	}
	if p.apiKey == "" {
		p.apiKey = providerConfig.APIKey
	}

	if config.BaseURL != "" {
		p.baseURL = config.BaseURL
	} else if providerConfig.BaseURL != "" {
		p.baseURL = providerConfig.BaseURL
	}

	if providerConfig.SiteURL != "" {
		p.siteURL = providerConfig.SiteURL
	}
	if providerConfig.SiteName != "" {
		p.siteName = providerConfig.SiteName
	}
	if providerConfig.Models != nil {
		p.models = providerConfig.Models
	}
	if providerConfig.ModelStrategy != "" {
		p.modelStrategy = providerConfig.ModelStrategy
	}
	p.freeOnly = providerConfig.FreeOnly

	// Update auth helper configuration
	p.authHelper.Config = config

	// Re-setup authentication with new config
	p.authHelper.SetupAPIKeys()

	// Recreate model selector
	p.modelSelector = NewModelSelector(p.models, p.modelStrategy)

	return p.BaseProvider.Configure(config)
}

func (p *OpenRouterProvider) SupportsToolCalling() bool {
	return true
}

func (p *OpenRouterProvider) SupportsStreaming() bool {
	return true
}

func (p *OpenRouterProvider) SupportsResponsesAPI() bool {
	return false
}

func (p *OpenRouterProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

// TestConnectivity performs a lightweight connectivity test using the /v1/models endpoint
// Results are cached for 30 seconds by default to prevent hammering the API during rapid health checks
// To bypass the cache and force a fresh check, use TestConnectivityWithOptions with bypassCache=true
func (p *OpenRouterProvider) TestConnectivity(ctx context.Context) error {
	return p.TestConnectivityWithOptions(ctx, false)
}

// TestConnectivityWithOptions performs a connectivity test with optional cache bypass
// If bypassCache is true, the cache is bypassed and a fresh connectivity check is performed
func (p *OpenRouterProvider) TestConnectivityWithOptions(ctx context.Context, bypassCache bool) error {
	return p.connectivityCache.TestConnectivity(
		ctx,
		types.ProviderTypeOpenRouter,
		p.performConnectivityTest,
		bypassCache,
	)
}

// performConnectivityTest performs the actual connectivity test without caching
func (p *OpenRouterProvider) performConnectivityTest(ctx context.Context) error {
	// Check if we have API keys configured
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		return types.NewAuthError(types.ProviderTypeOpenRouter, "no API keys configured").
			WithOperation("test_connectivity")
	}

	// Use the first available API key for connectivity test
	apiKey := p.authHelper.KeyManager.GetKeys()[0]
	if apiKey == "" {
		return types.NewAuthError(types.ProviderTypeOpenRouter, "no valid API key available").
			WithOperation("test_connectivity")
	}

	// Create a request to the /v1/models endpoint
	url := p.baseURL + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create connectivity test request: %w", err)
	}

	// Set authentication headers
	p.authHelper.SetAuthHeaders(req, apiKey, "api_key")
	req.Header.Set("HTTP-Referer", p.siteURL)
	req.Header.Set("X-Title", p.siteName)

	// Make the request with a shorter timeout for connectivity testing
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connectivity test failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		return types.NewAuthError(types.ProviderTypeOpenRouter, "invalid API key").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(types.ProviderTypeOpenRouter, "API key does not have access to models endpoint").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("connectivity test failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	// Try to parse the response to ensure it's valid JSON
	decoder := json.NewDecoder(resp.Body)
	var testResponse struct {
		Data []interface{} `json:"data"`
	}
	if err := decoder.Decode(&testResponse); err != nil {
		return fmt.Errorf("invalid response from models endpoint: %w", err)
	}

	// Verify we got a valid models list response
	if len(testResponse.Data) == 0 {
		return fmt.Errorf("no models returned from models endpoint")
	}

	return nil
}

func (p *OpenRouterProvider) HealthCheck(ctx context.Context) error {
	if !p.IsAuthenticated() {
		return types.NewAuthError(types.ProviderTypeOpenRouter, "not authenticated").
			WithOperation("health_check")
	}

	// Try to get rate limits as a health check
	_, err := p.GetRateLimits(ctx)
	return err
}

func (p *OpenRouterProvider) GetMetrics() types.ProviderMetrics {
	metrics := p.BaseProvider.GetMetrics()

	p.mutex.RLock()
	if p.lastUsage != nil {
		metrics.TokensUsed += int64(p.lastUsage.TotalTokens)
	}
	p.mutex.RUnlock()

	return metrics
}

// GetLastUsedModel returns the model name that was used in the last API call
func (p *OpenRouterProvider) GetLastUsedModel() string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.lastUsedModel
}

// GetRateLimits queries the OpenRouter API for current rate limit information via /api/v1/key endpoint.
// This is the existing approach that makes a dedicated API call to check rate limits.
func (p *OpenRouterProvider) GetRateLimits(ctx context.Context) (*OpenRouterRateLimits, error) {
	if !p.authHelper.IsAuthenticated() {
		return nil, types.NewAuthError(types.ProviderTypeOpenRouter, "no OpenRouter API key configured").
			WithOperation("get_rate_limits")
	}

	// Get the current API key
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		return nil, types.NewAuthError(types.ProviderTypeOpenRouter, "no valid API key available").
			WithOperation("get_rate_limits")
	}

	apiKey := p.authHelper.KeyManager.GetKeys()[0] // Simple for now, could be improved
	if apiKey == "" {
		return nil, types.NewAuthError(types.ProviderTypeOpenRouter, "no valid API key available").
			WithOperation("get_rate_limits")
	}

	// Build the request URL
	url := p.baseURL + "/v1/key"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenRouter, "failed to create request").
			WithOperation("get_rate_limits").
			WithOriginalErr(err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", p.siteURL)
	req.Header.Set("X-Title", p.siteName)

	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenRouter, "request failed").
			WithOperation("get_rate_limits").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenRouter, "failed to read response body").
			WithOperation("get_rate_limits").
			WithOriginalErr(err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewProviderError(types.ProviderTypeOpenRouter, types.ClassifyHTTPError(resp.StatusCode), fmt.Sprintf("OpenRouter rate limits API error: %d - %s", resp.StatusCode, string(body))).
			WithOperation("get_rate_limits").
			WithStatusCode(resp.StatusCode)
	}

	var rateLimits OpenRouterRateLimits
	if err := json.Unmarshal(body, &rateLimits); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "failed to parse rate limits response").
			WithOperation("get_rate_limits").
			WithOriginalErr(err)
	}

	return &rateLimits, nil
}

// GetTrackedRateLimits returns the rate limit information tracked from response headers.
// This provides the most up-to-date rate limit information for a specific model
// without making an additional API call. Returns nil if no rate limit data has been
// tracked for the given model yet.
func (p *OpenRouterProvider) GetTrackedRateLimits(model string) *ratelimit.Info {
	if info, exists := p.rateLimitHelper.GetRateLimitInfo(model); exists {
		return info
	}
	return nil
}

// GetCombinedRateLimits provides unified rate limit information by combining:
// 1. API endpoint data (/api/v1/key) - provides account-level credit information
// 2. Header-based tracking - provides per-model request/token limits
// This method returns the most comprehensive and up-to-date rate limit information.
func (p *OpenRouterProvider) GetCombinedRateLimits(ctx context.Context, model string) (*OpenRouterRateLimits, *ratelimit.Info, error) {
	// Get account-level rate limits from API endpoint
	apiLimits, apiErr := p.GetRateLimits(ctx)

	// Get model-specific rate limits from header tracking
	var headerInfo *ratelimit.Info
	if info, exists := p.rateLimitHelper.GetRateLimitInfo(model); exists {
		headerInfo = info
	}

	// If we have header info, we can merge it with API limits
	if apiErr == nil && headerInfo != nil && apiLimits != nil {
		// Header info may have more recent data for this specific model
		// API limits provide account-wide credit information
		// Both are valuable and complement each other
		_ = headerInfo // Suppress unused variable warning for future logic
	}

	return apiLimits, headerInfo, apiErr
}

// OAuth Flow Methods

// StartOAuthFlow initiates the OAuth flow and returns the authorization URL
// OpenRouter OAuth provisions permanent API keys, not temporary tokens
func (p *OpenRouterProvider) StartOAuthFlow(ctx context.Context, callbackURL string) (authURL string, flowState *OAuthFlowState, err error) {
	if callbackURL == "" {
		// Use configured callback URL if available
		callbackURL = p.config.OAuthCallbackURL
		if callbackURL == "" {
			return "", nil, types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "callback URL is required for OAuth flow").
				WithOperation("start_oauth_flow")
		}
	}

	// Create OAuth helper
	helper := NewOAuthHelper(p.baseURL)

	// Generate PKCE parameters
	pkceParams, err := helper.GeneratePKCEParams()
	if err != nil {
		return "", nil, types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "failed to generate PKCE parameters").
			WithOperation("start_oauth_flow").
			WithOriginalErr(err)
	}

	// Build authorization URL
	authURL, err = helper.BuildAuthURL(callbackURL, pkceParams)
	if err != nil {
		return "", nil, types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "failed to build auth URL").
			WithOperation("start_oauth_flow").
			WithOriginalErr(err)
	}

	// Create flow state to track the OAuth flow
	flowState = NewOAuthFlowState(pkceParams.CodeVerifier, callbackURL)

	return authURL, flowState, nil
}

// HandleOAuthCallback processes the OAuth callback and exchanges the code for an API key
// The obtained API key is automatically added to the APIKeyManager pool
func (p *OpenRouterProvider) HandleOAuthCallback(ctx context.Context, authCode string, flowState *OAuthFlowState) (apiKey string, err error) {
	if authCode == "" {
		return "", types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "authorization code is required").
			WithOperation("handle_oauth_callback")
	}
	if flowState == nil {
		return "", types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "OAuth flow state is required").
			WithOperation("handle_oauth_callback")
	}

	// Validate the flow state
	if err := flowState.Validate(); err != nil {
		return "", types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "invalid OAuth flow state").
			WithOperation("handle_oauth_callback").
			WithOriginalErr(err)
	}

	// Create OAuth helper
	helper := NewOAuthHelper(p.baseURL)

	// Exchange authorization code for API key
	apiKey, err = helper.ExchangeCodeForAPIKey(ctx, authCode, flowState.CodeVerifier)
	if err != nil {
		return "", types.NewNetworkError(types.ProviderTypeOpenRouter, "failed to exchange code for API key").
			WithOperation("handle_oauth_callback").
			WithOriginalErr(err)
	}

	// Add the obtained API key to the key manager
	if err := p.AddAPIKey(apiKey); err != nil {
		return "", types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "failed to add API key to manager").
			WithOperation("handle_oauth_callback").
			WithOriginalErr(err)
	}

	return apiKey, nil
}

// AddAPIKey adds a new API key to the auth helper pool
// This can be used to add OAuth-obtained keys or manually configured keys
func (p *OpenRouterProvider) AddAPIKey(apiKey string) error {
	if apiKey == "" {
		return types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "API key cannot be empty").
			WithOperation("add_api_key")
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Get current keys from the auth helper
	var currentKeys []string
	if p.authHelper.KeyManager != nil {
		currentKeys = p.authHelper.KeyManager.GetKeys()
	}

	// Check if key already exists
	for _, key := range currentKeys {
		if key == apiKey {
			return nil // Key already exists
		}
	}

	// Add the new key to the list
	currentKeys = append(currentKeys, apiKey)

	// Update the base provider config to use only the api_keys array
	baseConfig := p.GetConfig()
	if baseConfig.ProviderConfig == nil {
		baseConfig.ProviderConfig = make(map[string]interface{})
	}

	// Store keys only in the api_keys array to avoid duplication
	baseConfig.ProviderConfig["api_keys"] = currentKeys

	// Clear the single APIKey field to prevent duplication
	baseConfig.APIKey = ""

	// Update provider config
	p.config.APIKeys = currentKeys

	// Re-setup auth helper with updated keys
	p.authHelper.Config = baseConfig
	p.authHelper.SetupAPIKeys()

	// Update primary API key if this is the first key
	if p.apiKey == "" {
		p.apiKey = apiKey
	}

	return nil
}

// RemoveAPIKey removes an API key from the auth helper pool
func (p *OpenRouterProvider) RemoveAPIKey(apiKey string) error {
	if apiKey == "" {
		return types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "API key cannot be empty").
			WithOperation("remove_api_key")
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Get current keys
	var currentKeys []string
	if p.authHelper.KeyManager != nil {
		currentKeys = p.authHelper.KeyManager.GetKeys()
	} else {
		return types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "API key not found in manager").
			WithOperation("remove_api_key")
	}

	// Remove the key from the list
	var newKeys []string
	found := false
	for _, key := range currentKeys {
		if key != apiKey {
			newKeys = append(newKeys, key)
		} else {
			found = true
		}
	}

	if !found {
		return types.NewInvalidRequestError(types.ProviderTypeOpenRouter, "API key not found in manager").
			WithOperation("remove_api_key")
	}

	// Update the base provider config to use only the api_keys array
	baseConfig := p.GetConfig()
	if baseConfig.ProviderConfig == nil {
		baseConfig.ProviderConfig = make(map[string]interface{})
	}

	// Store keys only in the api_keys array to avoid duplication
	baseConfig.ProviderConfig["api_keys"] = newKeys

	// Clear the single APIKey field to prevent duplication
	baseConfig.APIKey = ""

	// Update provider config
	p.config.APIKeys = newKeys

	// Re-setup auth helper with updated keys
	p.authHelper.Config = baseConfig
	p.authHelper.SetupAPIKeys()

	// Update primary API key if we removed it
	if p.apiKey == apiKey {
		if len(newKeys) > 0 {
			p.apiKey = newKeys[0]
		} else {
			p.apiKey = ""
		}
	}

	return nil
}

// GetAPIKeys returns a masked list of all API keys in the auth helper
func (p *OpenRouterProvider) GetAPIKeys() []string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.authHelper.KeyManager == nil {
		return []string{}
	}

	keys := p.authHelper.KeyManager.GetKeys()
	if len(keys) > 0 {
		// Return masked placeholder for each key
		maskedKeys := make([]string, len(keys))
		for i := range maskedKeys {
			maskedKeys[i] = "sk-***-***"
		}
		return maskedKeys
	}

	return []string{}
}

// IsOAuthConfigured checks if OAuth authentication is properly configured
// OpenRouter supports OAuth flow which provisions permanent API keys
func (p *OpenRouterProvider) IsOAuthConfigured() bool {
	// OpenRouter's OAuth is unique - it provisions API keys rather than tokens
	// So we check if we have an OAuth callback URL configured and OAuth helper available
	return p.config.OAuthCallbackURL != ""
}

// IsAPIKeyConfigured checks if API key authentication is properly configured
func (p *OpenRouterProvider) IsAPIKeyConfigured() bool {
	return p.authHelper.IsAPIKeyConfigured()
}
