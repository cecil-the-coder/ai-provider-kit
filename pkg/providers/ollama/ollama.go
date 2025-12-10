// Package ollama provides an Ollama AI provider implementation.
// It supports both local Ollama instances and cloud endpoints with authentication,
// streaming, and OpenAI-compatible tool calling.
package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/models"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Constants for Ollama models
const (
	ollamaDefaultModel = "llama3.1:8b" // Default model for chat completions
)

// OllamaProvider implements Provider interface for Ollama
type OllamaProvider struct {
	*base.BaseProvider
	config            types.ProviderConfig
	httpClient        *http.Client
	authHelper        *auth.AuthHelper
	connectivityCache *common.ConnectivityCache
	modelCache        *models.ModelCache
	streamEndpoint    StreamEndpoint // Endpoint format for streaming (ollama or openai)
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(config types.ProviderConfig) *OllamaProvider {
	// Use the shared config helper
	configHelper := commonconfig.NewConfigHelper("Ollama", types.ProviderTypeOllama)

	// Merge with defaults and extract configuration
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Set default base URL if not provided
	if mergedConfig.BaseURL == "" {
		mergedConfig.BaseURL = "http://localhost:11434"
	}

	// Set default timeout (30 seconds)
	timeout := 30 * time.Second
	if mergedConfig.ProviderConfig != nil {
		if timeoutVal, ok := mergedConfig.ProviderConfig["timeout"].(time.Duration); ok {
			timeout = timeoutVal
		}
	}

	client := &http.Client{
		Timeout: timeout,
	}

	// Create auth helper
	authHelper := auth.NewAuthHelper("ollama", mergedConfig, client)

	// Setup API keys using shared helper (for cloud endpoints)
	// Check OLLAMA_API_KEY environment variable
	authHelper.SetupAPIKeys()

	// Determine stream endpoint format from config
	streamEndpoint := StreamEndpointOllama // Default to native Ollama format
	if mergedConfig.ProviderConfig != nil {
		if endpoint, ok := mergedConfig.ProviderConfig["stream_endpoint"].(string); ok {
			streamEndpoint = StreamEndpoint(endpoint)
		}
	}

	return &OllamaProvider{
		BaseProvider:      base.NewBaseProvider("ollama", mergedConfig, client, log.Default()),
		config:            mergedConfig,
		httpClient:        client,
		authHelper:        authHelper,
		connectivityCache: common.NewDefaultConnectivityCache(),
		modelCache:        models.NewModelCache(5 * time.Minute), // 5 minute TTL
		streamEndpoint:    streamEndpoint,
	}
}

// Name returns the provider name
func (p *OllamaProvider) Name() string {
	return "Ollama"
}

// Type returns the provider type
func (p *OllamaProvider) Type() types.ProviderType {
	return types.ProviderTypeOllama
}

// Description returns the provider description
func (p *OllamaProvider) Description() string {
	return "Ollama local and cloud model inference"
}

// isCloudEndpoint determines if the provider is using a cloud endpoint
func (p *OllamaProvider) isCloudEndpoint() bool {
	return strings.Contains(p.config.BaseURL, "ollama.com")
}

// IsAuthenticated checks if the provider is authenticated
// For local Ollama, authentication is not required (returns true)
// For cloud endpoints, checks if API key is configured
func (p *OllamaProvider) IsAuthenticated() bool {
	// Local Ollama doesn't require authentication
	if !p.isCloudEndpoint() {
		return true
	}

	// Cloud endpoints require API key
	return p.authHelper.IsAuthenticated()
}

// Authenticate handles authentication
func (p *OllamaProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	if authConfig.Method != types.AuthMethodAPIKey {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "ollama only supports API key authentication for cloud endpoints").
			WithOperation("authenticate")
	}

	newConfig := p.GetConfig()
	newConfig.APIKey = authConfig.APIKey
	newConfig.BaseURL = authConfig.BaseURL
	newConfig.DefaultModel = authConfig.DefaultModel
	return p.Configure(newConfig)
}

// Logout handles logout
func (p *OllamaProvider) Logout(ctx context.Context) error {
	p.authHelper.ClearAuthentication()
	newConfig := p.GetConfig()
	newConfig.APIKey = ""
	return p.Configure(newConfig)
}

// Configure updates the provider configuration
func (p *OllamaProvider) Configure(config types.ProviderConfig) error {
	// Use the shared config helper for validation and extraction
	configHelper := commonconfig.NewConfigHelper("Ollama", types.ProviderTypeOllama)

	// Validate configuration
	validation := configHelper.ValidateProviderConfig(config)
	if !validation.Valid {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, validation.Errors[0]).
			WithOperation("configure")
	}

	// Merge with defaults
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Set default base URL if not provided
	if mergedConfig.BaseURL == "" {
		mergedConfig.BaseURL = "http://localhost:11434"
	}

	p.config = mergedConfig

	// Update auth helper configuration
	p.authHelper.Config = mergedConfig

	// Re-setup authentication with new config
	p.authHelper.SetupAPIKeys()

	// Update stream endpoint if specified
	if mergedConfig.ProviderConfig != nil {
		if endpoint, ok := mergedConfig.ProviderConfig["stream_endpoint"].(string); ok {
			p.streamEndpoint = StreamEndpoint(endpoint)
		}
	}

	return p.BaseProvider.Configure(mergedConfig)
}

// GetConfig returns the current configuration
func (p *OllamaProvider) GetConfig() types.ProviderConfig {
	return p.config
}

// GetDefaultModel returns the default model
func (p *OllamaProvider) GetDefaultModel() string {
	if p.config.DefaultModel != "" {
		return p.config.DefaultModel
	}
	return ollamaDefaultModel
}

// GetToolFormat returns the tool format used by this provider
func (p *OllamaProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

// SupportsToolCalling returns whether the provider supports tool calling
func (p *OllamaProvider) SupportsToolCalling() bool {
	return true
}

// SupportsStreaming returns whether the provider supports streaming
func (p *OllamaProvider) SupportsStreaming() bool {
	return true
}

// SupportsResponsesAPI returns whether the provider supports Responses API
func (p *OllamaProvider) SupportsResponsesAPI() bool {
	return false
}

// ollamaTagsResponse represents the response from /api/tags endpoint
type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

// ollamaModel represents a model in the Ollama API
type ollamaModel struct {
	Name       string             `json:"name"`
	Model      string             `json:"model"`
	ModifiedAt string             `json:"modified_at"`
	Size       int64              `json:"size"`
	Digest     string             `json:"digest"`
	Details    ollamaModelDetails `json:"details"`
}

// ollamaModelDetails contains detailed information about an Ollama model
type ollamaModelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// GetModels returns available models
func (p *OllamaProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	// Use model cache with fetch and fallback functions
	return p.modelCache.GetModels(
		func() ([]types.Model, error) {
			return p.fetchModelsFromAPI(ctx)
		},
		p.getStaticFallback,
	)
}

// fetchModelsFromAPI fetches models from the Ollama /api/tags endpoint
func (p *OllamaProvider) fetchModelsFromAPI(ctx context.Context) ([]types.Model, error) {
	url := fmt.Sprintf("%s/api/tags", strings.TrimSuffix(p.config.BaseURL, "/"))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "failed to create request").
			WithOperation("fetch_models").
			WithOriginalErr(err)
	}

	// Add authentication header if using cloud endpoint
	if p.isCloudEndpoint() && p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0 {
		apiKey := p.authHelper.KeyManager.GetKeys()[0]
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "failed to fetch models").
			WithOperation("fetch_models").
			WithOriginalErr(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, types.NewAuthError(types.ProviderTypeOllama, "invalid API key").
			WithOperation("fetch_models").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewServerError(types.ProviderTypeOllama, resp.StatusCode,
			fmt.Sprintf("failed to fetch models with status %d", resp.StatusCode)).
			WithOperation("fetch_models")
	}

	// Parse response
	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to parse models response").
			WithOperation("fetch_models").
			WithOriginalErr(err)
	}

	// Convert to types.Model
	result := make([]types.Model, 0, len(tagsResp.Models))
	for _, ollamaModel := range tagsResp.Models {
		model := p.convertOllamaModel(ollamaModel)
		result = append(result, model)
	}

	return result, nil
}

// convertOllamaModel converts an Ollama model to types.Model
func (p *OllamaProvider) convertOllamaModel(m ollamaModel) types.Model {
	// Use name as ID (e.g., "llama3.1:8b")
	modelID := m.Name
	if modelID == "" {
		modelID = m.Model
	}

	// Generate a friendly name
	modelName := modelID

	// Infer capabilities and features based on model family and name
	capabilities := []string{"chat", "completion"}
	supportsToolCalling := false
	maxTokens := 8192 // Default

	// Check model name/family for specific capabilities
	lowerName := strings.ToLower(modelID)
	family := strings.ToLower(m.Details.Family)

	// Vision models
	if strings.Contains(lowerName, "llava") || strings.Contains(lowerName, "vision") {
		capabilities = append(capabilities, "vision")
	}

	// Code models
	if strings.Contains(lowerName, "codellama") ||
		strings.Contains(lowerName, "deepseek-coder") ||
		strings.Contains(lowerName, "starcoder") ||
		strings.Contains(lowerName, "code") {
		capabilities = append(capabilities, "code")
	}

	// Embedding models
	if strings.Contains(lowerName, "embed") {
		capabilities = []string{"embeddings"}
		maxTokens = 8192
	} else {
		// Most chat models support tool calling
		// Llama 3.1+, Mistral, and other modern models support tool calling
		if strings.Contains(lowerName, "llama3") ||
			strings.Contains(lowerName, "mistral") ||
			strings.Contains(lowerName, "mixtral") ||
			strings.Contains(lowerName, "qwen") ||
			strings.Contains(lowerName, "deepseek") {
			supportsToolCalling = true
			capabilities = append(capabilities, "tool_calling")
		}

		// Infer max tokens from model family
		// Check specific models first, then general families
		switch {
		case strings.Contains(lowerName, "codellama"):
			maxTokens = 16384
		case family == "llama" || strings.Contains(lowerName, "llama"):
			maxTokens = 131072 // Llama 3.1 supports 128k context
		case strings.Contains(lowerName, "mistral") || strings.Contains(lowerName, "mixtral"):
			maxTokens = 32768
		}
	}

	// Create description
	description := fmt.Sprintf("%s model", modelID)
	if m.Details.ParameterSize != "" {
		description = fmt.Sprintf("%s (%s parameters)", modelID, m.Details.ParameterSize)
	}

	return types.Model{
		ID:                  modelID,
		Name:                modelName,
		Provider:            p.Type(),
		Description:         description,
		MaxTokens:           maxTokens,
		SupportsStreaming:   true, // All Ollama models support streaming
		SupportsToolCalling: supportsToolCalling,
		Capabilities:        capabilities,
	}
}

// getStaticFallback returns static model list
func (p *OllamaProvider) getStaticFallback() []types.Model {
	return []types.Model{
		{
			ID:                   "llama3.1:8b",
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
			ID:                   "llama3.1:70b",
			Name:                 "Llama 3.1 70B",
			Provider:             p.Type(),
			Description:          "Llama 3.1 70B parameter model",
			MaxTokens:            8192,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "completion", "analysis"},
		},
		{
			ID:                   "codellama:13b",
			Name:                 "Code Llama 13B",
			Provider:             p.Type(),
			Description:          "Code Llama 13B specialized for code generation",
			MaxTokens:            4096,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "code"},
		},
		{
			ID:                   "mistral:7b",
			Name:                 "Mistral 7B",
			Provider:             p.Type(),
			Description:          "Mistral 7B parameter model",
			MaxTokens:            8192,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "completion"},
		},
	}
}

// ollamaChatRequest represents a request to Ollama /api/chat endpoint
type ollamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []ollamaChatMessage    `json:"messages"`
	Stream   bool                   `json:"stream"`
	Tools    []ollamaTool           `json:"tools,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

// ollamaChatMessage represents a message in the Ollama chat API
type ollamaChatMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Images    []string         `json:"images,omitempty"` // base64 encoded for vision
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

// ollamaToolCall represents a tool call from the assistant
type ollamaToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"` // "function"
	Function ollamaFunctionCall `json:"function"`
}

// ollamaFunctionCall represents a function call
type ollamaFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ollamaTool represents a tool definition for Ollama
type ollamaTool struct {
	Type     string            `json:"type"` // "function"
	Function ollamaFunctionDef `json:"function"`
}

// ollamaFunctionDef represents a function definition
type ollamaFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ollamaChatResponse represents a streaming response from Ollama
type ollamaChatResponse struct {
	Model     string            `json:"model"`
	CreatedAt string            `json:"created_at"`
	Message   ollamaChatMessage `json:"message"`
	Done      bool              `json:"done"`

	// Usage information (only in final chunk when done=true)
	TotalDuration      int64 `json:"total_duration,omitempty"`
	LoadDuration       int64 `json:"load_duration,omitempty"`
	PromptEvalCount    int   `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64 `json:"prompt_eval_duration,omitempty"`
	EvalCount          int   `json:"eval_count,omitempty"`
	EvalDuration       int64 `json:"eval_duration,omitempty"`
}

// GenerateChatCompletion generates a chat completion
func (p *OllamaProvider) GenerateChatCompletion(
	ctx context.Context,
	options types.GenerateOptions,
) (types.ChatCompletionStream, error) {
	// Initialize request tracking
	p.IncrementRequestCount()
	startTime := time.Now()

	// Validate authentication for cloud endpoints
	if p.isCloudEndpoint() && !p.authHelper.IsAuthenticated() {
		p.RecordError(types.NewAuthError(types.ProviderTypeOllama, "no API key configured for cloud endpoint"))
		return nil, types.NewAuthError(types.ProviderTypeOllama, "no API key configured for cloud endpoint").
			WithOperation("chat_completion")
	}

	// Build the request
	request := p.buildOllamaChatRequest(options)

	// Determine the base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := fmt.Sprintf("%s/api/chat", strings.TrimSuffix(baseURL, "/"))

	// Log the request
	p.LogRequest("POST", url, map[string]string{
		"Content-Type": "application/json",
	}, request)

	// Make the API call
	stream, err := p.makeStreamingAPICall(ctx, url, request)
	if err != nil {
		p.RecordError(err)
		return nil, err
	}

	// Record success (tokens will be counted as stream is consumed)
	latency := time.Since(startTime)
	p.RecordSuccess(latency, 0)

	return stream, nil
}

// buildOllamaChatRequest builds an Ollama chat request from GenerateOptions
func (p *OllamaProvider) buildOllamaChatRequest(options types.GenerateOptions) ollamaChatRequest {
	// Determine model
	model := options.Model
	if model == "" {
		model = p.GetDefaultModel()
	}

	// Convert messages
	messages := p.convertMessages(options.Messages)

	// Build options map
	optionsMap := make(map[string]interface{})
	if options.Temperature != 0 {
		optionsMap["temperature"] = options.Temperature
	}
	if options.MaxTokens > 0 {
		optionsMap["num_predict"] = options.MaxTokens
	}
	if len(options.Stop) > 0 {
		optionsMap["stop"] = options.Stop
	}

	// Build request
	request := ollamaChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true, // Always stream for real-time responses
		Options:  optionsMap,
	}

	// Convert tools if provided
	if len(options.Tools) > 0 {
		request.Tools = p.convertTools(options.Tools)
	}

	return request
}

// convertMessages converts universal ChatMessages to Ollama format
func (p *OllamaProvider) convertMessages(messages []types.ChatMessage) []ollamaChatMessage {
	ollamaMessages := make([]ollamaChatMessage, 0, len(messages))

	for _, msg := range messages {
		ollamaMsg := ollamaChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Extract images from ContentParts if present
		if len(msg.Parts) > 0 {
			images := p.extractImagesFromParts(msg.Parts)
			if len(images) > 0 {
				ollamaMsg.Images = images
			}

			// If content is empty but we have text parts, concatenate them
			if msg.Content == "" {
				var textParts []string
				for _, part := range msg.Parts {
					if part.IsText() {
						textParts = append(textParts, part.Text)
					}
				}
				if len(textParts) > 0 {
					ollamaMsg.Content = strings.Join(textParts, "\n")
				}
			}
		}

		// Convert tool calls if present
		if len(msg.ToolCalls) > 0 {
			ollamaMsg.ToolCalls = p.convertToolCalls(msg.ToolCalls)
		}

		ollamaMessages = append(ollamaMessages, ollamaMsg)
	}

	return ollamaMessages
}

// extractImagesFromParts extracts base64 encoded images from ContentParts
func (p *OllamaProvider) extractImagesFromParts(parts []types.ContentPart) []string {
	var images []string

	for _, part := range parts {
		if part.Type == types.ContentTypeImage && part.Source != nil {
			if part.Source.Type == types.MediaSourceBase64 {
				images = append(images, part.Source.Data)
			}
			// Note: Ollama doesn't support image URLs directly, only base64
		}
	}

	return images
}

// convertToolCalls converts universal ToolCalls to Ollama format
func (p *OllamaProvider) convertToolCalls(toolCalls []types.ToolCall) []ollamaToolCall {
	ollamaToolCalls := make([]ollamaToolCall, 0, len(toolCalls))

	for _, tc := range toolCalls {
		ollamaToolCalls = append(ollamaToolCalls, ollamaToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: ollamaFunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	return ollamaToolCalls
}

// convertTools converts universal Tools to Ollama format
func (p *OllamaProvider) convertTools(tools []types.Tool) []ollamaTool {
	ollamaTools := make([]ollamaTool, 0, len(tools))

	for _, tool := range tools {
		ollamaTools = append(ollamaTools, ollamaTool{
			Type: "function",
			Function: ollamaFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	return ollamaTools
}

// makeStreamingAPICall makes a streaming API call to Ollama using the configured endpoint
func (p *OllamaProvider) makeStreamingAPICall(ctx context.Context, _ string, request ollamaChatRequest) (types.ChatCompletionStream, error) {
	// Use the new streaming implementation with endpoint format
	return p.makeStreamingRequest(ctx, p.streamEndpoint, request)
}

// makeHTTPStreamRequest makes the HTTP request and returns the response
func (p *OllamaProvider) makeHTTPStreamRequest(ctx context.Context, url string, request ollamaChatRequest) (*http.Response, error) {
	// Marshal request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to marshal request").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "failed to create request").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Add authentication header if using cloud endpoint
	if p.isCloudEndpoint() && p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0 {
		apiKey := p.authHelper.KeyManager.GetKeys()[0]
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	// Make the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "request failed").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		// Map HTTP status codes to appropriate errors
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return nil, types.NewAuthError(types.ProviderTypeOllama, "invalid API key").
				WithOperation("chat_completion").
				WithStatusCode(resp.StatusCode)
		case http.StatusNotFound:
			return nil, types.NewNotFoundError(types.ProviderTypeOllama, "model not found").
				WithOperation("chat_completion").
				WithStatusCode(resp.StatusCode)
		case http.StatusTooManyRequests:
			return nil, types.NewRateLimitError(types.ProviderTypeOllama, 0).
				WithOperation("chat_completion").
				WithStatusCode(resp.StatusCode)
		default:
			if resp.StatusCode >= 500 {
				return nil, types.NewServerError(types.ProviderTypeOllama, resp.StatusCode, string(body)).
					WithOperation("chat_completion")
			}
			return nil, types.NewProviderError(types.ProviderTypeOllama, types.ErrCodeInvalidRequest, string(body)).
				WithOperation("chat_completion").
				WithStatusCode(resp.StatusCode)
		}
	}

	return resp, nil
}

// convertOllamaToolCallsToUniversal converts Ollama tool calls to universal format
func (p *OllamaProvider) convertOllamaToolCallsToUniversal(ollamaToolCalls []ollamaToolCall) []types.ToolCall {
	toolCalls := make([]types.ToolCall, 0, len(ollamaToolCalls))

	for _, otc := range ollamaToolCalls {
		toolCalls = append(toolCalls, types.ToolCall{
			ID:   otc.ID,
			Type: otc.Type,
			Function: types.ToolCallFunction{
				Name:      otc.Function.Name,
				Arguments: otc.Function.Arguments,
			},
		})
	}

	return toolCalls
}

// InvokeServerTool invokes a server tool
func (p *OllamaProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, types.NewInvalidRequestError(types.ProviderTypeOllama, "tool invocation not yet implemented for Ollama provider").
		WithOperation("invoke_tool")
}

// HealthCheck performs a health check
func (p *OllamaProvider) HealthCheck(ctx context.Context) error {
	// For local instances, check if the service is reachable
	// For cloud instances, verify API key is valid
	return p.TestConnectivity(ctx)
}

// GetMetrics returns provider metrics
func (p *OllamaProvider) GetMetrics() types.ProviderMetrics {
	return p.BaseProvider.GetMetrics()
}

// TestConnectivity performs a lightweight connectivity test
// Results are cached for 30 seconds by default to prevent hammering the API during rapid health checks
func (p *OllamaProvider) TestConnectivity(ctx context.Context) error {
	return p.connectivityCache.TestConnectivity(
		ctx,
		types.ProviderTypeOllama,
		p.performConnectivityTest,
		false,
	)
}

// performConnectivityTest performs the actual connectivity test without caching
func (p *OllamaProvider) performConnectivityTest(ctx context.Context) error {
	// For cloud endpoints, verify authentication
	if p.isCloudEndpoint() && !p.authHelper.IsAuthenticated() {
		return types.NewAuthError(types.ProviderTypeOllama, "no API key configured for cloud endpoint").
			WithOperation("test_connectivity")
	}

	// Determine base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	// Try GET /api/version first (lightweight endpoint)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/version", nil)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Add authentication header if using cloud endpoint
	if p.isCloudEndpoint() && p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0 {
		apiKey := p.authHelper.KeyManager.GetKeys()[0]
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	// Make the request with a shorter timeout for connectivity testing
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		// If /api/version fails, try GET / as fallback
		req2, err2 := http.NewRequestWithContext(ctx, "GET", baseURL+"/", nil)
		if err2 != nil {
			return types.NewNetworkError(types.ProviderTypeOllama, "connectivity test failed").
				WithOperation("test_connectivity").
				WithOriginalErr(err)
		}

		if p.isCloudEndpoint() && p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0 {
			apiKey := p.authHelper.KeyManager.GetKeys()[0]
			req2.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
		}

		resp2, err2 := client.Do(req2)
		if err2 != nil {
			return types.NewNetworkError(types.ProviderTypeOllama, "connectivity test failed").
				WithOperation("test_connectivity").
				WithOriginalErr(err2)
		}
		defer func() {
			_ = resp2.Body.Close()
		}()

		// Any 2xx or 3xx response indicates the service is reachable
		if resp2.StatusCode >= 200 && resp2.StatusCode < 400 {
			return nil
		}

		return types.NewNetworkError(types.ProviderTypeOllama, fmt.Sprintf("connectivity test failed with status %d", resp2.StatusCode)).
			WithOperation("test_connectivity").
			WithStatusCode(resp2.StatusCode)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		return types.NewAuthError(types.ProviderTypeOllama, "invalid API key").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(types.ProviderTypeOllama, "API key does not have access").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	// Any 2xx response indicates success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return types.NewServerError(types.ProviderTypeOllama, resp.StatusCode,
		fmt.Sprintf("connectivity test failed with status %d", resp.StatusCode)).
		WithOperation("test_connectivity")
}
