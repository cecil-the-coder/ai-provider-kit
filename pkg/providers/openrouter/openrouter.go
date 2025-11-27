// Package openrouter provides an OpenRouter AI provider implementation.
// It includes support for model selection, API key management, OAuth authentication,
// and specialized features for the OpenRouter platform.
package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// OpenRouterProvider implements the Provider interface for OpenRouter
type OpenRouterProvider struct {
	*base.BaseProvider
	config        ProviderConfig
	client        *http.Client
	authHelper    *common.AuthHelper
	modelSelector *ModelSelector
	lastUsedModel string
	lastUsage     *types.Usage
	mutex         sync.RWMutex
	modelCache    *common.ModelCache

	// Configuration fields
	apiKey        string
	baseURL       string
	siteURL       string
	siteName      string
	models        []string
	modelStrategy string
	freeOnly      bool

	// Rate limiting (header-based tracking)
	rateLimitHelper *common.RateLimitHelper
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

	models := providerConfig.Models
	if len(models) == 0 && providerConfig.Model != "" {
		models = []string{providerConfig.Model}
	}
	if len(models) == 0 && config.DefaultModel != "" {
		models = []string{config.DefaultModel}
	}
	if len(models) == 0 {
		models = []string{"qwen/qwen3-coder"}
	}

	modelStrategy := providerConfig.ModelStrategy
	if modelStrategy == "" {
		modelStrategy = "failover"
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// Create auth helper
	authHelper := common.NewAuthHelper("openrouter", config, client)

	// Setup API keys using shared helper
	authHelper.SetupAPIKeys()

	provider := &OpenRouterProvider{
		BaseProvider:    base.NewBaseProvider("openrouter", config, client, log.Default()),
		config:          providerConfig,
		client:          client,
		authHelper:      authHelper,
		modelSelector:   NewModelSelector(models, modelStrategy),
		apiKey:          apiKey,
		baseURL:         baseURL,
		siteURL:         siteURL,
		siteName:        siteName,
		models:          models,
		modelStrategy:   modelStrategy,
		freeOnly:        providerConfig.FreeOnly,
		modelCache:      common.NewModelCache(6 * time.Hour), // 6 hour cache for OpenRouter
		rateLimitHelper: common.NewRateLimitHelper(ratelimit.NewOpenRouterParser()),
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
		return nil, fmt.Errorf("no OpenRouter API key configured")
	}

	// Get API key from auth helper
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		return nil, fmt.Errorf("no valid API key available")
	}

	apiKey := p.authHelper.KeyManager.GetKeys()[0] // Simple for now, could be improved
	if apiKey == "" {
		return nil, fmt.Errorf("no valid API key available")
	}

	url := p.baseURL + "/v1/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Use auth helper to set headers
	p.authHelper.SetAuthHeaders(req, apiKey, "api_key")
	req.Header.Set("HTTP-Referer", p.siteURL)
	req.Header.Set("X-Title", p.siteName)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch models: HTTP %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var modelsResp OpenRouterModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
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
		return nil, fmt.Errorf("no OpenRouter API key configured")
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
			return nil, fmt.Errorf("OpenRouter: rate limit exceeded (free tier limit reached)")
		}
		// Check if we have credits remaining (for paid tiers)
		if rateLimits.Limit != nil && rateLimits.LimitRemaining != nil && *rateLimits.LimitRemaining <= 0 {
			return nil, fmt.Errorf("OpenRouter: credit limit exceeded")
		}
	}

	// Option 2: Check via header-based tracker (new approach)
	// This uses cached rate limit information from previous responses
	p.rateLimitHelper.CheckRateLimitAndWait(requestData.Model, 0)

	// Handle streaming
	if options.Stream {
		// Try to get an API key and make streaming call
		if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
			return nil, fmt.Errorf("no API key available for streaming")
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
		callErr = fmt.Errorf("no authentication manager available")
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
	return nil, fmt.Errorf("tool invocation not yet implemented for OpenRouter provider")
}

func (p *OpenRouterProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	if authConfig.Method != types.AuthMethodAPIKey {
		return fmt.Errorf("OpenRouter only supports API key authentication")
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
		return fmt.Errorf("invalid provider type for OpenRouter: %s", config.Type)
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

func (p *OpenRouterProvider) HealthCheck(ctx context.Context) error {
	if !p.IsAuthenticated() {
		return fmt.Errorf("not authenticated")
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

// prepareRequest prepares the API request payload
func (p *OpenRouterProvider) prepareRequest(options types.GenerateOptions) (OpenRouterRequest, error) {
	// Determine which model to use: options.Model takes precedence over modelSelector
	var modelName string
	if options.Model != "" {
		modelName = options.Model
	} else {
		var err error
		modelName, err = p.modelSelector.SelectModel()
		if err != nil {
			return OpenRouterRequest{}, fmt.Errorf("failed to select model: %w", err)
		}
	}

	if p.freeOnly && !strings.HasSuffix(modelName, ":free") {
		modelName += ":free"
	}

	p.mutex.Lock()
	p.lastUsedModel = modelName
	p.mutex.Unlock()

	// Convert to OpenRouter messages
	var messages []OpenRouterMessage

	if len(options.Messages) > 0 {
		// Use provided messages directly
		messages = convertMessagesToOpenRouter(options.Messages)
	} else if options.Prompt != "" {
		// Convert prompt to user message
		messages = append(messages, OpenRouterMessage{
			Role:    "user",
			Content: options.Prompt,
		})
	}

	requestData := OpenRouterRequest{
		Model:         modelName,
		Messages:      messages,
		Stream:        options.Stream,
		HTTPReferer:   p.siteURL,
		HTTPUserAgent: p.siteName,
	}

	if options.Temperature > 0 {
		requestData.Temperature = options.Temperature
	}
	if options.MaxTokens > 0 {
		requestData.MaxTokens = options.MaxTokens
	}

	// Convert tools if provided
	if len(options.Tools) > 0 {
		requestData.Tools = convertToOpenRouterTools(options.Tools)
		// ToolChoice defaults to "auto" when tools are provided (OpenAI's default behavior)
	}

	return requestData, nil
}

// makeAPICallWithKey makes the actual HTTP request to the OpenRouter API
func (p *OpenRouterProvider) makeAPICallWithKey(ctx context.Context, requestData OpenRouterRequest, apiKey string) (*OpenRouterResponse, error) {
	jsonBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(jsonBody)))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", p.siteURL)
	req.Header.Set("X-Title", p.siteName)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	// Parse rate limit headers from response
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, requestData.Model)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResponse OpenRouterErrorResponse
		if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil {
			return nil, fmt.Errorf("OpenRouter API error: %d - %s", resp.StatusCode, errorResponse.Error.Message)
		}
		return nil, fmt.Errorf("OpenRouter API error: %d - %s", resp.StatusCode, string(body))
	}

	var response OpenRouterResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no choices in API response")
	}

	return &response, nil
}

// GetRateLimits queries the OpenRouter API for current rate limit information via /api/v1/key endpoint.
// This is the existing approach that makes a dedicated API call to check rate limits.
func (p *OpenRouterProvider) GetRateLimits(ctx context.Context) (*OpenRouterRateLimits, error) {
	if !p.authHelper.IsAuthenticated() {
		return nil, fmt.Errorf("no OpenRouter API key configured")
	}

	// Get the current API key
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		return nil, fmt.Errorf("no valid API key available")
	}

	apiKey := p.authHelper.KeyManager.GetKeys()[0] // Simple for now, could be improved
	if apiKey == "" {
		return nil, fmt.Errorf("no valid API key available")
	}

	// Build the request URL
	url := p.baseURL + "/v1/key"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", p.siteURL)
	req.Header.Set("X-Title", p.siteName)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter rate limits API error: %d - %s", resp.StatusCode, string(body))
	}

	var rateLimits OpenRouterRateLimits
	if err := json.Unmarshal(body, &rateLimits); err != nil {
		return nil, fmt.Errorf("failed to parse rate limits response: %w", err)
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

// makeStreamingAPICallWithKey makes a streaming API call with a specific API key
func (p *OpenRouterProvider) makeStreamingAPICallWithKey(ctx context.Context, requestData OpenRouterRequest, apiKey string) (types.ChatCompletionStream, error) {
	jsonBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(jsonBody)))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", p.siteURL)
	req.Header.Set("X-Title", p.siteName)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Parse rate limit headers from streaming response
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, requestData.Model)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors
		return nil, fmt.Errorf("OpenRouter API error: %d - %s", resp.StatusCode, string(body))
	}

	return &OpenRouterStream{
		response: resp,
		reader:   bufio.NewReader(resp.Body),
		done:     false,
	}, nil
}

// OpenRouterStream implements ChatCompletionStream for real streaming responses
type OpenRouterStream struct {
	response *http.Response
	reader   *bufio.Reader
	done     bool
	mutex    sync.Mutex
}

func (s *OpenRouterStream) Next() (types.ChatCompletionChunk, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.done {
		return types.ChatCompletionChunk{Done: true}, io.EOF
	}

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				s.done = true
				return types.ChatCompletionChunk{Done: true}, io.EOF
			}
			return types.ChatCompletionChunk{}, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue // Skip empty lines
		}

		if !strings.HasPrefix(line, "data: ") {
			continue // Skip non-data lines
		}

		data := strings.TrimPrefix(line, "data: ")

		// Check for stream end
		if data == "[DONE]" {
			s.done = true
			return types.ChatCompletionChunk{Done: true}, io.EOF
		}

		var streamResp OpenRouterResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue // Skip malformed chunks
		}

		if len(streamResp.Choices) > 0 {
			choice := streamResp.Choices[0]
			chunk := types.ChatCompletionChunk{
				Content: choice.Message.Content,
				Done:    choice.FinishReason != "",
			}

			// Add usage if present
			if streamResp.Usage.TotalTokens > 0 {
				chunk.Usage = types.Usage{
					PromptTokens:     streamResp.Usage.PromptTokens,
					CompletionTokens: streamResp.Usage.CompletionTokens,
					TotalTokens:      streamResp.Usage.TotalTokens,
				}
			}

			// Add tool calls if present in the message
			if len(choice.Message.ToolCalls) > 0 {
				chunk.Choices = []types.ChatChoice{
					{
						Delta: types.ChatMessage{
							Role:      choice.Message.Role,
							Content:   choice.Message.Content,
							ToolCalls: convertOpenRouterToolCallsToUniversal(choice.Message.ToolCalls),
						},
						FinishReason: choice.FinishReason,
					},
				}
			}

			if chunk.Done {
				s.done = true
				return chunk, io.EOF
			}

			return chunk, nil
		}
	}
}

func (s *OpenRouterStream) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.done = true
	if s.response != nil {
		return s.response.Body.Close()
	}
	return nil
}

// OpenRouter data structures

// OpenRouterRequest represents the request payload for OpenRouter API
type OpenRouterRequest struct {
	Model         string              `json:"model"`
	Messages      []OpenRouterMessage `json:"messages"`
	Stream        bool                `json:"stream"`
	HTTPReferer   string              `json:"http_referer,omitempty"`
	HTTPUserAgent string              `json:"x-title,omitempty"`
	Temperature   float64             `json:"temperature,omitempty"`
	MaxTokens     int                 `json:"max_tokens,omitempty"`
	Tools         []OpenRouterTool    `json:"tools,omitempty"`
	ToolChoice    interface{}         `json:"tool_choice,omitempty"`
}

// OpenRouterTool represents a tool in the OpenRouter API (OpenAI-compatible format)
type OpenRouterTool struct {
	Type     string                `json:"type"` // Always "function"
	Function OpenRouterFunctionDef `json:"function"`
}

// OpenRouterFunctionDef represents a function definition in the OpenRouter API
type OpenRouterFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// OpenRouterMessage represents a message in the conversation
type OpenRouterMessage struct {
	Role       string               `json:"role"`
	Content    string               `json:"content"`
	ToolCalls  []OpenRouterToolCall `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

// OpenRouterToolCall represents a tool call in the OpenRouter API
type OpenRouterToolCall struct {
	ID       string                     `json:"id"`
	Type     string                     `json:"type"` // "function"
	Function OpenRouterToolCallFunction `json:"function"`
}

// OpenRouterToolCallFunction represents a function call in a tool call
type OpenRouterToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// OpenRouterResponse represents the response from OpenRouter API
type OpenRouterResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []OpenRouterChoice `json:"choices"`
	Usage   OpenRouterUsage    `json:"usage"`
}

// OpenRouterChoice represents a choice in the response
type OpenRouterChoice struct {
	Index        int               `json:"index"`
	Message      OpenRouterMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

// OpenRouterUsage represents token usage information
type OpenRouterUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenRouterErrorResponse represents an error response
type OpenRouterErrorResponse struct {
	Error OpenRouterError `json:"error"`
}

// OpenRouterError represents an error in the response
type OpenRouterError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    int    `json:"code"`
}

// OpenRouterRateLimits represents rate limit information from the /api/v1/key endpoint
type OpenRouterRateLimits struct {
	Limit          *float64 `json:"limit"`           // Credit limit (can be null)
	LimitReset     string   `json:"limit_reset"`     // Reset type for credits
	LimitRemaining *float64 `json:"limit_remaining"` // Remaining credits
	Usage          float64  `json:"usage"`           // Total credits used
	IsFreeTier     bool     `json:"is_free_tier"`    // Whether account is on free tier
	RateLimit      struct {
		RequestsPerMinute int `json:"requests_per_minute"`
		RequestsPerDay    int `json:"requests_per_day"`
	} `json:"rate_limit,omitempty"` // Rate limit information
}

// OpenRouterModelsResponse represents the response from /api/v1/models endpoint
type OpenRouterModelsResponse struct {
	Data []OpenRouterModelData `json:"data"`
}

// OpenRouterModelData represents a model in the models list
type OpenRouterModelData struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Pricing       OpenRouterPricing      `json:"pricing"`
	ContextLength int                    `json:"context_length"`
	Architecture  OpenRouterArchitecture `json:"architecture"`
	TopProvider   OpenRouterTopProvider  `json:"top_provider"`
}

// OpenRouterPricing represents pricing information
type OpenRouterPricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	Request    string `json:"request"`
	Image      string `json:"image"`
}

// OpenRouterArchitecture represents model architecture info
type OpenRouterArchitecture struct {
	Modality     string `json:"modality"`
	TokenizerID  string `json:"tokenizer"`
	InstructType string `json:"instruct_type"`
}

// OpenRouterTopProvider represents top provider info
type OpenRouterTopProvider struct {
	ContextLength       int  `json:"context_length"`
	MaxCompletionTokens int  `json:"max_completion_tokens"`
	IsModerated         bool `json:"is_moderated"`
}

// MockStream implements ChatCompletionStream for testing
type MockStream struct {
	chunks []types.ChatCompletionChunk
	index  int
}

func (ms *MockStream) Next() (types.ChatCompletionChunk, error) {
	if ms.index >= len(ms.chunks) {
		return types.ChatCompletionChunk{}, nil
	}
	chunk := ms.chunks[ms.index]
	ms.index++
	return chunk, nil
}

func (ms *MockStream) Close() error {
	ms.index = 0
	return nil
}

// OAuth Flow Methods

// StartOAuthFlow initiates the OAuth flow and returns the authorization URL
// OpenRouter OAuth provisions permanent API keys, not temporary tokens
func (p *OpenRouterProvider) StartOAuthFlow(ctx context.Context, callbackURL string) (authURL string, flowState *OAuthFlowState, err error) {
	if callbackURL == "" {
		// Use configured callback URL if available
		callbackURL = p.config.OAuthCallbackURL
		if callbackURL == "" {
			return "", nil, fmt.Errorf("callback URL is required for OAuth flow")
		}
	}

	// Create OAuth helper
	helper := NewOAuthHelper(p.baseURL)

	// Generate PKCE parameters
	pkceParams, err := helper.GeneratePKCEParams()
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate PKCE parameters: %w", err)
	}

	// Build authorization URL
	authURL, err = helper.BuildAuthURL(callbackURL, pkceParams)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build auth URL: %w", err)
	}

	// Create flow state to track the OAuth flow
	flowState = NewOAuthFlowState(pkceParams.CodeVerifier, callbackURL)

	return authURL, flowState, nil
}

// HandleOAuthCallback processes the OAuth callback and exchanges the code for an API key
// The obtained API key is automatically added to the APIKeyManager pool
func (p *OpenRouterProvider) HandleOAuthCallback(ctx context.Context, authCode string, flowState *OAuthFlowState) (apiKey string, err error) {
	if authCode == "" {
		return "", fmt.Errorf("authorization code is required")
	}
	if flowState == nil {
		return "", fmt.Errorf("OAuth flow state is required")
	}

	// Validate the flow state
	if err := flowState.Validate(); err != nil {
		return "", fmt.Errorf("invalid OAuth flow state: %w", err)
	}

	// Create OAuth helper
	helper := NewOAuthHelper(p.baseURL)

	// Exchange authorization code for API key
	apiKey, err = helper.ExchangeCodeForAPIKey(ctx, authCode, flowState.CodeVerifier)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code for API key: %w", err)
	}

	// Add the obtained API key to the key manager
	if err := p.AddAPIKey(apiKey); err != nil {
		return "", fmt.Errorf("failed to add API key to manager: %w", err)
	}

	return apiKey, nil
}

// AddAPIKey adds a new API key to the auth helper pool
// This can be used to add OAuth-obtained keys or manually configured keys
func (p *OpenRouterProvider) AddAPIKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
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
		return fmt.Errorf("API key cannot be empty")
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Get current keys
	var currentKeys []string
	if p.authHelper.KeyManager != nil {
		currentKeys = p.authHelper.KeyManager.GetKeys()
	} else {
		return fmt.Errorf("API key not found in manager")
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
		return fmt.Errorf("API key not found in manager")
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

// Helper functions

func convertMessagesToOpenRouter(messages []types.ChatMessage) []OpenRouterMessage {
	openrouterMessages := make([]OpenRouterMessage, len(messages))
	for i, msg := range messages {
		openrouterMsg := OpenRouterMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Convert tool calls if present
		if len(msg.ToolCalls) > 0 {
			openrouterMsg.ToolCalls = convertToOpenRouterToolCalls(msg.ToolCalls)
		}

		// Include tool call ID for tool response messages
		if msg.ToolCallID != "" {
			openrouterMsg.ToolCallID = msg.ToolCallID
		}

		openrouterMessages[i] = openrouterMsg
	}
	return openrouterMessages
}

// convertToOpenRouterTools converts universal tools to OpenRouter format (OpenAI-compatible)
func convertToOpenRouterTools(tools []types.Tool) []OpenRouterTool {
	openrouterTools := make([]OpenRouterTool, len(tools))
	for i, tool := range tools {
		openrouterTools[i] = OpenRouterTool{
			Type: "function",
			Function: OpenRouterFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}
	return openrouterTools
}

// convertToOpenRouterToolCalls converts universal tool calls to OpenRouter format
func convertToOpenRouterToolCalls(toolCalls []types.ToolCall) []OpenRouterToolCall {
	openrouterToolCalls := make([]OpenRouterToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		openrouterToolCalls[i] = OpenRouterToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: OpenRouterToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return openrouterToolCalls
}

// convertOpenRouterToolCallsToUniversal converts OpenRouter tool calls to universal format
func convertOpenRouterToolCallsToUniversal(toolCalls []OpenRouterToolCall) []types.ToolCall {
	universal := make([]types.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		universal[i] = types.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: types.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return universal
}
