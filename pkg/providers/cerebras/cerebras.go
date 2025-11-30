// Package cerebras provides a Cerebras AI provider implementation.
// It includes support for chat completions, streaming, tool calling, and multi-key load balancing.
package cerebras

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/models"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// CerebrasProvider implements Provider interface for Cerebras
type CerebrasProvider struct {
	*base.BaseProvider
	config          types.ProviderConfig
	httpClient      *http.Client
	authHelper      *auth.AuthHelper
	modelCache      *models.ModelCache
	rateLimitHelper *common.RateLimitHelper
}

// NewCerebrasProvider creates a new Cerebras provider
func NewCerebrasProvider(config types.ProviderConfig) *CerebrasProvider {
	// Initialize common provider components using the factory pattern
	components, err := base.InitializeProviderComponents(base.ProviderInitConfig{
		ProviderType: types.ProviderTypeCerebras,
		ProviderName: "Cerebras",
		Config:       config,
		HTTPTimeout:  10 * time.Second,
	})
	if err != nil {
		// This should rarely happen, but log and return nil if it does
		log.Printf("Failed to initialize Cerebras provider: %v", err)
		return nil
	}

	return &CerebrasProvider{
		BaseProvider:    components.BaseProvider,
		config:          components.MergedConfig,
		httpClient:      components.HTTPClient,
		authHelper:      components.AuthHelper,
		modelCache:      models.NewModelCache(commonconfig.GetModelCacheTTL(types.ProviderTypeCerebras)),
		rateLimitHelper: components.RateLimitHelper,
	}
}

// Name returns the provider name
func (p *CerebrasProvider) Name() string {
	return "Cerebras"
}

// Type returns the provider type
func (p *CerebrasProvider) Type() types.ProviderType {
	return types.ProviderTypeCerebras
}

// Description returns the provider description
func (p *CerebrasProvider) Description() string {
	return "Cerebras ultra-fast inference with multi-key failover and load balancing"
}

// GetModels returns available models
func (p *CerebrasProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	// Use the shared model cache utility
	return p.modelCache.GetModels(
		func() ([]types.Model, error) {
			// Fetch from API
			models, err := p.fetchModelsFromAPI(ctx)
			if err != nil {
				log.Printf("Cerebras: Failed to fetch models from API: %v", err)
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

// fetchModelsFromAPI fetches models from Cerebras API
func (p *CerebrasProvider) fetchModelsFromAPI(ctx context.Context) ([]types.Model, error) {
	if !p.authHelper.IsAuthenticated() {
		return nil, types.NewAuthError(types.ProviderTypeCerebras, "no Cerebras API key configured").
			WithOperation("list_models")
	}

	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.cerebras.ai/v1"
	}
	url := baseURL + "/models"

	// Get API key from auth helper
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		return nil, types.NewAuthError(types.ProviderTypeCerebras, "no API keys available").
			WithOperation("list_models")
	}

	apiKey, err := p.authHelper.KeyManager.GetNextKey()
	if err != nil {
		return nil, types.NewAuthError(types.ProviderTypeCerebras, "failed to get API key").
			WithOperation("list_models").
			WithOriginalErr(err)
	}

	req, err := p.authHelper.CreateJSONRequest(ctx, "GET", url, nil, apiKey, "api_key")
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeCerebras, "failed to create request").
			WithOperation("list_models").
			WithOriginalErr(err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeCerebras, "failed to fetch models").
			WithOperation("list_models").
			WithOriginalErr(err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeCerebras, errCode,
			fmt.Sprintf("failed to fetch models: %s", string(body))).
			WithOperation("list_models").
			WithStatusCode(resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeCerebras, "failed to read response").
			WithOperation("list_models").
			WithOriginalErr(err)
	}

	var modelsResp CerebrasModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeCerebras, "failed to parse models response").
			WithOperation("list_models").
			WithOriginalErr(err)
	}

	// Convert to internal Model format
	models := make([]types.Model, 0, len(modelsResp.Data))
	for _, model := range modelsResp.Data {
		models = append(models, types.Model{
			ID:       model.ID,
			Provider: p.Type(),
		})
	}

	return models, nil
}

// enrichModels adds metadata to models
func (p *CerebrasProvider) enrichModels(models []types.Model) []types.Model {
	// Static metadata for known models
	metadata := map[string]struct {
		name         string
		maxTokens    int
		description  string
		capabilities []string
	}{
		"zai-glm-4.6":  {"ZAI GLM-4.6", 131072, "Ultra-fast ZAI GLM-4.6 model", []string{"chat", "completion", "code-generation"}},
		"llama3.1-8b":  {"Llama 3.1 8B", 8192, "Llama 3.1 8B parameter model", []string{"chat", "completion"}},
		"llama3.1-70b": {"Llama 3.1 70B", 8192, "Llama 3.1 70B parameter model", []string{"chat", "completion", "analysis"}},
	}

	enriched := make([]types.Model, len(models))
	for i, model := range models {
		enriched[i] = types.Model{
			ID:                  model.ID,
			Provider:            p.Type(),
			SupportsStreaming:   true,
			SupportsToolCalling: true,
		}

		// Add metadata if available
		if meta, ok := metadata[model.ID]; ok {
			enriched[i].Name = meta.name
			enriched[i].MaxTokens = meta.maxTokens
			enriched[i].Description = meta.description
			enriched[i].Capabilities = meta.capabilities
		} else {
			// Default values for unknown models
			enriched[i].Name = model.ID
			enriched[i].MaxTokens = 8192
			enriched[i].Description = "Cerebras model"
		}
	}
	return enriched
}

// getStaticFallback returns static model list
func (p *CerebrasProvider) getStaticFallback() []types.Model {
	return []types.Model{
		{
			ID:                   "zai-glm-4.6",
			Name:                 "ZAI GLM-4.6",
			Provider:             p.Type(),
			Description:          "Ultra-fast ZAI GLM-4.6 model",
			MaxTokens:            131072,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "completion", "code-generation"},
		},
		{
			ID:                   "llama3.1-8b",
			Name:                 "Llama 3.1 8B",
			Provider:             p.Type(),
			Description:          "Llama 3.1 8B parameter model",
			MaxTokens:            8192,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "completion"},
		},
		{
			ID:                   "llama3.1-70b",
			Name:                 "Llama 3.1 70B",
			Provider:             p.Type(),
			Description:          "Llama 3.1 70B parameter model",
			MaxTokens:            8192,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "completion", "analysis"},
		},
	}
}

// GetDefaultModel returns the default model
func (p *CerebrasProvider) GetDefaultModel() string {
	if p.config.DefaultModel != "" {
		return p.config.DefaultModel
	}
	return "zai-glm-4.6"
}

// GenerateChatCompletion generates a chat completion
func (p *CerebrasProvider) GenerateChatCompletion(
	ctx context.Context,
	options types.GenerateOptions,
) (types.ChatCompletionStream, error) {
	// Initialize request tracking
	p.IncrementRequestCount()
	startTime := time.Now()

	// Perform initial validation and setup
	if err := p.validateAndSetup(); err != nil {
		p.RecordError(err)
		return nil, err
	}

	// Prepare request components
	model := p.resolveModel(options.Model)
	p.rateLimitHelper.CheckRateLimitAndWait(model, options.MaxTokens)
	baseURL := p.getBaseURL()
	temperature := p.resolveTemperature(options.Temperature)
	messages := p.buildMessages(options)
	request := p.buildRequest(model, messages, temperature, options)

	// Log the request
	p.logRequest(baseURL, request)

	// Handle the request based on streaming preference
	if options.Stream {
		return p.handleStreamingRequest(ctx, baseURL, request, startTime)
	}

	return p.handleNonStreamingRequest(ctx, baseURL, request, model, startTime)
}

// validateAndSetup performs initial validation and setup
func (p *CerebrasProvider) validateAndSetup() error {
	if !p.authHelper.IsAuthenticated() {
		return types.NewAuthError(types.ProviderTypeCerebras, "no API key configured for Cerebras").
			WithOperation("chat_completion")
	}
	return nil
}

// resolveModel determines which model to use
func (p *CerebrasProvider) resolveModel(optionModel string) string {
	if optionModel != "" {
		return optionModel
	}
	return p.GetDefaultModel()
}

// getBaseURL returns the base URL for API calls
func (p *CerebrasProvider) getBaseURL() string {
	if p.config.BaseURL != "" {
		return p.config.BaseURL
	}
	return "https://api.cerebras.ai/v1"
}

// resolveTemperature determines the temperature to use
func (p *CerebrasProvider) resolveTemperature(optionTemp float64) float64 {
	// Default temperature
	temperature := 0.6

	// Get from config if available
	if p.config.ProviderConfig != nil {
		if temp, ok := p.config.ProviderConfig["temperature"].(float64); ok {
			temperature = temp
		}
	}

	// Override with option if provided
	if optionTemp != 0 {
		temperature = optionTemp
	}

	return temperature
}

// buildMessages constructs the message array for the request
func (p *CerebrasProvider) buildMessages(options types.GenerateOptions) []CerebrasMessage {
	// Estimate initial capacity: system message + user prompt + custom messages
	capacity := 2 + len(options.Messages)
	messages := make([]CerebrasMessage, 0, capacity)

	// Add system message if no custom messages are provided
	if len(options.Messages) == 0 {
		messages = append(messages, CerebrasMessage{
			Role:    "system",
			Content: "You are an expert programmer. Generate ONLY clean, functional code with no explanations or markdown formatting.",
		})
	}

	// Add user prompt if provided
	if options.Prompt != "" {
		messages = append(messages, CerebrasMessage{
			Role:    "user",
			Content: options.Prompt,
		})
	}

	// Add custom messages
	for _, msg := range options.Messages {
		cerebrasMsg := CerebrasMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Convert tool calls if present
		if len(msg.ToolCalls) > 0 {
			cerebrasMsg.ToolCalls = convertToCerebrasToolCalls(msg.ToolCalls)
		}

		// Include tool call ID for tool response messages
		if msg.ToolCallID != "" {
			cerebrasMsg.ToolCallID = msg.ToolCallID
		}

		messages = append(messages, cerebrasMsg)
	}

	return messages
}

// buildRequest constructs the complete API request
func (p *CerebrasProvider) buildRequest(model string, messages []CerebrasMessage, temperature float64, options types.GenerateOptions) CerebrasRequest {
	request := CerebrasRequest{
		Model:       model,
		Messages:    messages,
		Temperature: &temperature,
		Stream:      options.Stream,
	}

	// Convert tools if provided
	if len(options.Tools) > 0 {
		request.Tools = convertToCerebrasTools(options.Tools)

		// Convert tool choice if specified
		if options.ToolChoice != nil {
			request.ToolChoice = convertToCerebrasToolChoice(options.ToolChoice)
		}
	}

	// Set max tokens if specified
	maxTokens := p.resolveMaxTokens(options.MaxTokens)
	if maxTokens > 0 {
		request.MaxTokens = &maxTokens
	}

	return request
}

// resolveMaxTokens determines the max tokens to use
func (p *CerebrasProvider) resolveMaxTokens(optionMaxTokens int) int {
	if optionMaxTokens != 0 {
		return optionMaxTokens
	}

	if p.config.ProviderConfig != nil {
		if mt, ok := p.config.ProviderConfig["max_tokens"].(int); ok && mt > 0 {
			return mt
		}
	}

	return 0
}

// logRequest logs the API request
func (p *CerebrasProvider) logRequest(baseURL string, request CerebrasRequest) {
	p.LogRequest("POST", baseURL+"/chat/completions", map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer ***",
	}, request)
}

// handleStreamingRequest handles the streaming API request path
func (p *CerebrasProvider) handleStreamingRequest(ctx context.Context, baseURL string, request CerebrasRequest, startTime time.Time) (types.ChatCompletionStream, error) {
	url := baseURL + "/chat/completions"
	var lastErr error

	if p.authHelper.KeyManager != nil {
		keys := p.authHelper.KeyManager.GetKeys()
		for _, apiKey := range keys {
			stream, err := p.makeStreamingAPICall(ctx, url, request, apiKey)
			if err != nil {
				lastErr = err
				continue
			}
			latency := time.Since(startTime)
			p.RecordSuccess(latency, 0) // Tokens will be counted as stream is consumed
			return stream, nil
		}
	}

	p.RecordError(lastErr)
	return nil, lastErr
}

// handleNonStreamingRequest handles the non-streaming API request path
func (p *CerebrasProvider) handleNonStreamingRequest(ctx context.Context, baseURL string, request CerebrasRequest, model string, startTime time.Time) (types.ChatCompletionStream, error) {
	url := baseURL + "/chat/completions"
	var responseMessage types.ChatMessage
	var usage *types.Usage
	var responseContent string
	var callErr error

	if p.authHelper.KeyManager != nil {
		responseContent, usage, callErr = p.authHelper.KeyManager.ExecuteWithFailover(ctx, func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
			return p.executeAPICall(ctx, url, request, apiKey, &responseMessage)
		})
	} else {
		callErr = types.NewAuthError(types.ProviderTypeCerebras, "no authentication manager available").
			WithOperation("chat_completion")
	}

	if callErr != nil {
		p.RecordError(callErr)
		return nil, callErr
	}

	// Record success metrics
	p.recordSuccessMetrics(startTime, usage)

	// Create and return response stream
	stream := p.createResponseStream(responseContent, usage, model, responseMessage)
	return stream, nil
}

// executeAPICall executes a single API call and processes the response
func (p *CerebrasProvider) executeAPICall(ctx context.Context, url string, request CerebrasRequest, apiKey string, responseMessage *types.ChatMessage) (string, *types.Usage, error) {
	response, err := p.makeAPICall(ctx, url, request, apiKey)
	if err != nil {
		return "", nil, err
	}

	if len(response.Choices) == 0 {
		return "", nil, types.NewInvalidRequestError(types.ProviderTypeCerebras, "no choices in Cerebras API response").
			WithOperation("chat_completion")
	}

	cerebrasMsg := response.Choices[0].Message

	// Convert to universal format
	*responseMessage = types.ChatMessage{
		Role:    cerebrasMsg.Role,
		Content: cerebrasMsg.Content,
	}

	// Convert tool calls if present
	if len(cerebrasMsg.ToolCalls) > 0 {
		responseMessage.ToolCalls = convertCerebrasToolCallsToUniversal(cerebrasMsg.ToolCalls)
	}

	responseUsage := &types.Usage{
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		TotalTokens:      response.Usage.TotalTokens,
	}

	return cerebrasMsg.Content, responseUsage, nil
}

// recordSuccessMetrics records success metrics
func (p *CerebrasProvider) recordSuccessMetrics(startTime time.Time, usage *types.Usage) {
	latency := time.Since(startTime)
	var tokensUsed int64
	if usage != nil {
		tokensUsed = int64(usage.TotalTokens)
	}
	p.RecordSuccess(latency, tokensUsed)
}

// createResponseStream creates a response stream from the API response
func (p *CerebrasProvider) createResponseStream(responseContent string, usage *types.Usage, model string, responseMessage types.ChatMessage) types.ChatCompletionStream {
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

	return &CerebrasStream{
		content: responseContent,
		usage:   usageValue,
		model:   model,
		closed:  false,
		chunk:   chunk,
	}
}

// makeAPICall makes a single API call
func (p *CerebrasProvider) makeAPICall(ctx context.Context, url string, request CerebrasRequest, apiKey string) (*CerebrasResponse, error) {
	req, err := p.authHelper.CreateJSONRequest(ctx, "POST", url, request, apiKey, "api_key")
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeCerebras, "failed to create request").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	startTime := time.Now()
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeCerebras, "request failed").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	duration := time.Since(startTime)
	p.LogResponse(resp, duration)

	// Parse rate limit headers
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, request.Model)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeCerebras, "failed to read response body").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	if resp.StatusCode != http.StatusOK {
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeCerebras, errCode, string(body)).
			WithOperation("chat_completion").
			WithStatusCode(resp.StatusCode)
	}

	var response CerebrasResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeCerebras, "failed to parse API response").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	return &response, nil
}

// InvokeServerTool invokes a server tool
func (p *CerebrasProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, types.NewInvalidRequestError(types.ProviderTypeCerebras, "tool invocation not yet implemented for Cerebras provider").
		WithOperation("invoke_tool")
}

// Authenticate handles authentication
func (p *CerebrasProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	if authConfig.Method != types.AuthMethodAPIKey {
		return types.NewInvalidRequestError(types.ProviderTypeCerebras, "cerebras only supports API key authentication").
			WithOperation("authenticate")
	}

	newConfig := p.GetConfig()
	newConfig.APIKey = authConfig.APIKey
	newConfig.BaseURL = authConfig.BaseURL
	newConfig.DefaultModel = authConfig.DefaultModel
	return p.Configure(newConfig)
}

// IsAuthenticated checks if the provider is authenticated
func (p *CerebrasProvider) IsAuthenticated() bool {
	return p.authHelper.IsAuthenticated()
}

// Logout handles logout
func (p *CerebrasProvider) Logout(ctx context.Context) error {
	p.authHelper.ClearAuthentication()
	newConfig := p.GetConfig()
	newConfig.APIKey = ""
	return p.Configure(newConfig)
}

// Configure updates the provider configuration
func (p *CerebrasProvider) Configure(config types.ProviderConfig) error {
	// Use the shared config helper for validation and extraction
	configHelper := commonconfig.NewConfigHelper("Cerebras", types.ProviderTypeCerebras)

	// Validate configuration
	validation := configHelper.ValidateProviderConfig(config)
	if !validation.Valid {
		return types.NewInvalidRequestError(types.ProviderTypeCerebras, validation.Errors[0]).
			WithOperation("configure")
	}

	// Merge with defaults
	mergedConfig := configHelper.MergeWithDefaults(config)

	p.config = mergedConfig

	// Update auth helper configuration
	p.authHelper.Config = mergedConfig

	// Re-setup authentication with new config
	p.authHelper.SetupAPIKeys()

	return p.BaseProvider.Configure(mergedConfig)
}

// GetConfig returns the current configuration
func (p *CerebrasProvider) GetConfig() types.ProviderConfig {
	return p.config
}

// SupportsToolCalling returns whether the provider supports tool calling
func (p *CerebrasProvider) SupportsToolCalling() bool {
	return true
}

// SupportsStreaming returns whether the provider supports streaming
func (p *CerebrasProvider) SupportsStreaming() bool {
	return true
}

// SupportsResponsesAPI returns whether the provider supports Responses API
func (p *CerebrasProvider) SupportsResponsesAPI() bool {
	return false
}

// GetToolFormat returns the tool format used by this provider
func (p *CerebrasProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

// HealthCheck performs a health check
func (p *CerebrasProvider) HealthCheck(ctx context.Context) error {
	if !p.authHelper.IsAuthenticated() {
		err := types.NewAuthError(types.ProviderTypeCerebras, "no API keys configured").
			WithOperation("health_check")
		p.UpdateHealthStatus(false, err.Error())
		return err
	}

	// Simple health check by attempting to get models
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.cerebras.ai/v1"
	}

	// Get API key from auth helper
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		err := types.NewAuthError(types.ProviderTypeCerebras, "no API keys available for health check").
			WithOperation("health_check")
		p.UpdateHealthStatus(false, err.Error())
		return err
	}

	apiKey, err := p.authHelper.KeyManager.GetNextKey()
	if err != nil {
		providerErr := types.NewAuthError(types.ProviderTypeCerebras, "failed to get API key").
			WithOperation("health_check").
			WithOriginalErr(err)
		p.UpdateHealthStatus(false, providerErr.Error())
		return providerErr
	}

	req, err := p.authHelper.CreateJSONRequest(ctx, "GET", baseURL+"/models", nil, apiKey, "api_key")
	if err != nil {
		providerErr := types.NewInvalidRequestError(types.ProviderTypeCerebras, "failed to create health check request").
			WithOperation("health_check").
			WithOriginalErr(err)
		p.UpdateHealthStatus(false, providerErr.Error())
		return providerErr
	}

	startTime := time.Now()
	resp, err := p.httpClient.Do(req)
	if err != nil {
		providerErr := types.NewNetworkError(types.ProviderTypeCerebras, "health check request failed").
			WithOperation("health_check").
			WithOriginalErr(err)
		p.UpdateHealthStatus(false, providerErr.Error())
		return providerErr
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	if resp.StatusCode != http.StatusOK {
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		providerErr := types.NewProviderError(types.ProviderTypeCerebras, errCode,
			fmt.Sprintf("health check failed with status %d", resp.StatusCode)).
			WithOperation("health_check").
			WithStatusCode(resp.StatusCode)
		p.UpdateHealthStatus(false, providerErr.Error())
		return providerErr
	}

	// Update health status with response time
	responseTime := time.Since(startTime)
	p.UpdateHealthStatus(true, "healthy")

	// Update the response time in metrics via BaseProvider
	p.BaseProvider.UpdateHealthStatusResponseTime(responseTime.Seconds())

	return nil
}

// GetMetrics returns provider metrics
func (p *CerebrasProvider) GetMetrics() types.ProviderMetrics {
	return p.BaseProvider.GetMetrics()
}

// makeStreamingAPICall makes a streaming API call
func (p *CerebrasProvider) makeStreamingAPICall(ctx context.Context, url string, request CerebrasRequest, apiKey string) (types.ChatCompletionStream, error) {
	req, err := p.authHelper.CreateJSONRequest(ctx, "POST", url, request, apiKey, "api_key")
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeCerebras, "failed to create request").
			WithOperation("chat_completion_stream").
			WithOriginalErr(err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeCerebras, "request failed").
			WithOperation("chat_completion_stream").
			WithOriginalErr(err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() {
			//nolint:staticcheck // Empty branch is intentional - we ignore close errors
			_ = resp.Body.Close()
		}()
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeCerebras, errCode, string(body)).
			WithOperation("chat_completion_stream").
			WithStatusCode(resp.StatusCode)
	}

	// Parse rate limit headers for streaming responses
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, request.Model)

	return &CerebrasRealStream{
		response: resp,
		reader:   bufio.NewReader(resp.Body),
		done:     false,
	}, nil
}

// CerebrasRealStream implements ChatCompletionStream for real streaming responses
type CerebrasRealStream struct {
	response *http.Response
	reader   *bufio.Reader
	done     bool
	mutex    sync.Mutex
}

func (s *CerebrasRealStream) Next() (types.ChatCompletionChunk, error) {
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

		var streamResp CerebrasResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue // Skip malformed chunks
		}

		if len(streamResp.Choices) > 0 {
			choice := streamResp.Choices[0]

			// Use Delta for streaming (not Message)
			chunk := types.ChatCompletionChunk{
				Content: choice.Delta.Content,
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

			// Add tool calls if present in the delta (similar to OpenAI)
			if len(choice.Delta.ToolCalls) > 0 {
				chunk.Choices = []types.ChatChoice{
					{
						Delta: types.ChatMessage{
							Role:      choice.Delta.Role,
							Content:   choice.Delta.Content,
							ToolCalls: convertCerebrasToolCallsToUniversal(choice.Delta.ToolCalls),
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

func (s *CerebrasRealStream) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.done = true
	if s.response != nil {
		return s.response.Body.Close()
	}
	return nil
}

// convertToCerebrasTools converts universal tools to Cerebras format (OpenAI-compatible)
func convertToCerebrasTools(tools []types.Tool) []CerebrasTool {
	cerebrasTools := make([]CerebrasTool, len(tools))
	for i, tool := range tools {
		cerebrasTools[i] = CerebrasTool{
			Type: "function",
			Function: CerebrasFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}
	return cerebrasTools
}

// convertToCerebrasToolCalls converts universal tool calls to Cerebras format
func convertToCerebrasToolCalls(toolCalls []types.ToolCall) []CerebrasToolCall {
	cerebrasToolCalls := make([]CerebrasToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		cerebrasToolCalls[i] = CerebrasToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: CerebrasToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return cerebrasToolCalls
}

// convertCerebrasToolCallsToUniversal converts Cerebras tool calls to universal format
func convertCerebrasToolCallsToUniversal(toolCalls []CerebrasToolCall) []types.ToolCall {
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

// convertToCerebrasToolChoice converts universal ToolChoice to Cerebras format (OpenAI-compatible)
func convertToCerebrasToolChoice(toolChoice *types.ToolChoice) interface{} {
	if toolChoice == nil {
		return nil
	}

	switch toolChoice.Mode {
	case types.ToolChoiceAuto:
		return "auto"
	case types.ToolChoiceRequired:
		return "required"
	case types.ToolChoiceNone:
		return "none"
	case types.ToolChoiceSpecific:
		// OpenAI specific tool format: {"type": "function", "function": {"name": "tool_name"}}
		return map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name": toolChoice.FunctionName,
			},
		}
	default:
		return "auto" // Default to auto if mode is unknown
	}
}
