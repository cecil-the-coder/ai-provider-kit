// Package ollama provides an Ollama AI provider implementation.
// It supports both local Ollama instances and cloud endpoints with authentication,
// streaming, and OpenAI-compatible tool calling.
package ollama

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	pkghttp "github.com/cecil-the-coder/ai-provider-kit/internal/http"
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

	// Create HTTP client using internal/http package
	httpClient := pkghttp.NewHTTPClient(pkghttp.HTTPClientConfig{
		Timeout: timeout,
	})

	// Extract the underlying http.Client for compatibility with existing code
	client := httpClient.Client()

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

// ollamaPsResponse represents the response from /api/ps endpoint
type ollamaPsResponse struct {
	Models []ollamaRunningModel `json:"models"`
}

// ollamaRunningModel represents a running model in the Ollama API
type ollamaRunningModel struct {
	Name      string             `json:"name"`
	Model     string             `json:"model"`
	Size      int64              `json:"size"`
	Digest    string             `json:"digest"`
	Details   ollamaModelDetails `json:"details"`
	ExpiresAt string             `json:"expires_at"`
	SizeVRAM  int64              `json:"size_vram"`
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

// GetRunningModels returns currently loaded/running models
func (p *OllamaProvider) GetRunningModels(ctx context.Context) ([]types.RunningModel, error) {
	url := fmt.Sprintf("%s/api/ps", strings.TrimSuffix(p.config.BaseURL, "/"))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "failed to create request").
			WithOperation("get_running_models").
			WithOriginalErr(err)
	}

	// Add authentication header if using cloud endpoint
	if p.isCloudEndpoint() && p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0 {
		apiKey := p.authHelper.KeyManager.GetKeys()[0]
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "failed to fetch running models").
			WithOperation("get_running_models").
			WithOriginalErr(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, types.NewAuthError(types.ProviderTypeOllama, "invalid API key").
			WithOperation("get_running_models").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewServerError(types.ProviderTypeOllama, resp.StatusCode,
			fmt.Sprintf("failed to fetch running models with status %d", resp.StatusCode)).
			WithOperation("get_running_models")
	}

	// Parse response
	var psResp ollamaPsResponse
	if err := json.NewDecoder(resp.Body).Decode(&psResp); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to parse running models response").
			WithOperation("get_running_models").
			WithOriginalErr(err)
	}

	// Convert to types.RunningModel
	result := make([]types.RunningModel, 0, len(psResp.Models))
	for _, ollamaModel := range psResp.Models {
		result = append(result, p.convertOllamaRunningModel(ollamaModel))
	}

	return result, nil
}

// convertOllamaRunningModel converts an Ollama running model to types.RunningModel
func (p *OllamaProvider) convertOllamaRunningModel(m ollamaRunningModel) types.RunningModel {
	// Parse expires_at timestamp
	expiresAt, err := time.Parse(time.RFC3339, m.ExpiresAt)
	if err != nil {
		// If parsing fails, use zero time
		expiresAt = time.Time{}
	}

	return types.RunningModel{
		Name:      m.Name,
		Model:     m.Model,
		Size:      m.Size,
		Digest:    m.Digest,
		ExpiresAt: expiresAt,
		SizeVRAM:  m.SizeVRAM,
	}
}

// ollamaChatRequest represents a request to Ollama /api/chat endpoint
type ollamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []ollamaChatMessage    `json:"messages"`
	Stream   bool                   `json:"stream"`
	Tools    []ollamaTool           `json:"tools,omitempty"`
	Format   interface{}            `json:"format,omitempty"` // Can be "json" string or JSON schema object
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
	// Determine model with fallback priority
	model := common.ResolveModel(options.Model, p.config.DefaultModel, ollamaDefaultModel)

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

	// Handle structured outputs via ResponseFormat
	// ResponseFormat can be:
	// - "json" for basic JSON mode
	// - A JSON schema object for structured output with schema validation
	if options.ResponseFormat != "" {
		// Try to parse as JSON schema first
		var schemaObj map[string]interface{}
		if err := json.Unmarshal([]byte(options.ResponseFormat), &schemaObj); err == nil {
			// It's a valid JSON object, use it as the schema
			request.Format = schemaObj
		} else {
			// It's a string like "json", use it directly
			request.Format = options.ResponseFormat
		}
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

// ollamaEmbeddingsRequest represents a request to Ollama /api/embeddings endpoint
type ollamaEmbeddingsRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// ollamaEmbeddingsResponse represents a response from Ollama /api/embeddings endpoint
type ollamaEmbeddingsResponse struct {
	Embedding []float64 `json:"embedding"`
}

// GenerateEmbeddings generates embeddings for the given text
func (p *OllamaProvider) GenerateEmbeddings(ctx context.Context, model string, text string) ([]float64, error) {
	// Initialize request tracking
	p.IncrementRequestCount()
	startTime := time.Now()

	// Validate authentication for cloud endpoints
	if p.isCloudEndpoint() && !p.authHelper.IsAuthenticated() {
		p.RecordError(types.NewAuthError(types.ProviderTypeOllama, "no API key configured for cloud endpoint"))
		return nil, types.NewAuthError(types.ProviderTypeOllama, "no API key configured for cloud endpoint").
			WithOperation("generate_embeddings")
	}

	// Use default model if not specified
	if model == "" {
		model = "nomic-embed-text"
	}

	// Build the request
	request := ollamaEmbeddingsRequest{
		Model:  model,
		Prompt: text,
	}

	// Determine the base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := fmt.Sprintf("%s/api/embeddings", strings.TrimSuffix(baseURL, "/"))

	// Marshal request
	reqBody, err := json.Marshal(request)
	if err != nil {
		p.RecordError(types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to marshal request"))
		return nil, types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to marshal request").
			WithOperation("generate_embeddings").
			WithOriginalErr(err)
	}

	// Log the request
	p.LogRequest("POST", url, map[string]string{
		"Content-Type": "application/json",
	}, request)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		p.RecordError(types.NewNetworkError(types.ProviderTypeOllama, "failed to create request"))
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "failed to create request").
			WithOperation("generate_embeddings").
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
		p.RecordError(types.NewNetworkError(types.ProviderTypeOllama, "request failed"))
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "request failed").
			WithOperation("generate_embeddings").
			WithOriginalErr(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		// Map HTTP status codes to appropriate errors
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			err := types.NewAuthError(types.ProviderTypeOllama, "invalid API key").
				WithOperation("generate_embeddings").
				WithStatusCode(resp.StatusCode)
			p.RecordError(err)
			return nil, err
		case http.StatusNotFound:
			err := types.NewNotFoundError(types.ProviderTypeOllama, "model not found").
				WithOperation("generate_embeddings").
				WithStatusCode(resp.StatusCode)
			p.RecordError(err)
			return nil, err
		case http.StatusTooManyRequests:
			err := types.NewRateLimitError(types.ProviderTypeOllama, 0).
				WithOperation("generate_embeddings").
				WithStatusCode(resp.StatusCode)
			p.RecordError(err)
			return nil, err
		default:
			if resp.StatusCode >= 500 {
				err := types.NewServerError(types.ProviderTypeOllama, resp.StatusCode, string(body)).
					WithOperation("generate_embeddings")
				p.RecordError(err)
				return nil, err
			}
			err := types.NewProviderError(types.ProviderTypeOllama, types.ErrCodeInvalidRequest, string(body)).
				WithOperation("generate_embeddings").
				WithStatusCode(resp.StatusCode)
			p.RecordError(err)
			return nil, err
		}
	}

	// Parse response
	var embeddingsResp ollamaEmbeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingsResp); err != nil {
		p.RecordError(types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to parse embeddings response"))
		return nil, types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to parse embeddings response").
			WithOperation("generate_embeddings").
			WithOriginalErr(err)
	}

	// Record success
	latency := time.Since(startTime)
	p.RecordSuccess(latency, 0)

	return embeddingsResp.Embedding, nil
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

// Model Management APIs
// These operations are only supported on local Ollama instances, not cloud endpoints

// ollamaModelRequest represents a request for model operations (pull/push)
type ollamaModelRequest struct {
	Name   string `json:"name"`
	Stream bool   `json:"stream"`
}

// ollamaProgressResponse represents a streaming progress response from pull/push operations
type ollamaProgressResponse struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

// executeStreamingModelOperation executes a streaming model operation (pull/push)
func (p *OllamaProvider) executeStreamingModelOperation(ctx context.Context, model, endpoint, operation string) error {
	// Check if this is a cloud endpoint
	if p.isCloudEndpoint() {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "model management operations are not supported on cloud endpoints").
			WithOperation(operation)
	}

	// Build the request
	request := ollamaModelRequest{
		Name:   model,
		Stream: true,
	}

	// Determine the base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := fmt.Sprintf("%s/api/%s", strings.TrimSuffix(baseURL, "/"), endpoint)

	// Marshal request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, fmt.Sprintf("failed to marshal %s request", endpoint)).
			WithOperation(operation).
			WithOriginalErr(err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, fmt.Sprintf("failed to create %s request", endpoint)).
			WithOperation(operation).
			WithOriginalErr(err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, fmt.Sprintf("%s request failed", endpoint)).
			WithOperation(operation).
			WithOriginalErr(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeOllama, resp.StatusCode, string(body)).
			WithOperation(operation)
	}

	// Stream the progress updates
	logPrefix := strings.ToUpper(endpoint)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var progressResp ollamaProgressResponse
		if err := json.Unmarshal(line, &progressResp); err != nil {
			// Skip malformed lines
			continue
		}

		// Log progress
		if p.BaseProvider != nil {
			p.BaseProvider.LogRequest(logPrefix, "progress", nil, progressResp)
		}
	}

	if err := scanner.Err(); err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, fmt.Sprintf("failed to read %s response", endpoint)).
			WithOperation(operation).
			WithOriginalErr(err)
	}

	return nil
}

// PullModel pulls a model from the Ollama registry
// This operation is only supported on local Ollama instances
func (p *OllamaProvider) PullModel(ctx context.Context, model string) error {
	return p.executeStreamingModelOperation(ctx, model, "pull", "pull_model")
}

// PushModel pushes a model to the Ollama registry
// This operation is only supported on local Ollama instances
func (p *OllamaProvider) PushModel(ctx context.Context, model string) error {
	return p.executeStreamingModelOperation(ctx, model, "push", "push_model")
}

// ollamaDeleteRequest represents a request to delete a model
type ollamaDeleteRequest struct {
	Name string `json:"name"`
}

// DeleteModel deletes a local model
// This operation is only supported on local Ollama instances
func (p *OllamaProvider) DeleteModel(ctx context.Context, model string) error {
	// Check if this is a cloud endpoint
	if p.isCloudEndpoint() {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "model management operations are not supported on cloud endpoints").
			WithOperation("delete_model")
	}

	// Build the request
	request := ollamaDeleteRequest{
		Name: model,
	}

	// Determine the base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := fmt.Sprintf("%s/api/delete", strings.TrimSuffix(baseURL, "/"))

	// Marshal request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to marshal delete request").
			WithOperation("delete_model").
			WithOriginalErr(err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "failed to create delete request").
			WithOperation("delete_model").
			WithOriginalErr(err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "delete request failed").
			WithOperation("delete_model").
			WithOriginalErr(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusNotFound {
			return types.NewNotFoundError(types.ProviderTypeOllama, "model not found").
				WithOperation("delete_model").
				WithStatusCode(resp.StatusCode)
		}
		return types.NewServerError(types.ProviderTypeOllama, resp.StatusCode, string(body)).
			WithOperation("delete_model")
	}

	return nil
}

// ollamaCopyRequest represents a request to copy a model
type ollamaCopyRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// CopyModel copies a model locally
// This operation is only supported on local Ollama instances
func (p *OllamaProvider) CopyModel(ctx context.Context, source, destination string) error {
	// Check if this is a cloud endpoint
	if p.isCloudEndpoint() {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "model management operations are not supported on cloud endpoints").
			WithOperation("copy_model")
	}

	// Build the request
	request := ollamaCopyRequest{
		Source:      source,
		Destination: destination,
	}

	// Determine the base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := fmt.Sprintf("%s/api/copy", strings.TrimSuffix(baseURL, "/"))

	// Marshal request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to marshal copy request").
			WithOperation("copy_model").
			WithOriginalErr(err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "failed to create copy request").
			WithOperation("copy_model").
			WithOriginalErr(err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "copy request failed").
			WithOperation("copy_model").
			WithOriginalErr(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusNotFound {
			return types.NewNotFoundError(types.ProviderTypeOllama, "source model not found").
				WithOperation("copy_model").
				WithStatusCode(resp.StatusCode)
		}
		return types.NewServerError(types.ProviderTypeOllama, resp.StatusCode, string(body)).
			WithOperation("copy_model")
	}

	return nil
}

// ollamaCreateRequest represents a request to create a model from Modelfile
type ollamaCreateRequest struct {
	Name      string `json:"name"`
	Modelfile string `json:"modelfile"`
	Stream    bool   `json:"stream"`
}

// ollamaCreateResponse represents a streaming response from /api/create
type ollamaCreateResponse struct {
	Status string `json:"status"`
}

// CreateModel creates a model from a Modelfile
// This operation is only supported on local Ollama instances
func (p *OllamaProvider) CreateModel(ctx context.Context, name string, modelfile string) error {
	// Check if this is a cloud endpoint
	if p.isCloudEndpoint() {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "model management operations are not supported on cloud endpoints").
			WithOperation("create_model")
	}

	// Build the request
	request := ollamaCreateRequest{
		Name:      name,
		Modelfile: modelfile,
		Stream:    true,
	}

	// Determine the base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := fmt.Sprintf("%s/api/create", strings.TrimSuffix(baseURL, "/"))

	// Marshal request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to marshal create request").
			WithOperation("create_model").
			WithOriginalErr(err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "failed to create create request").
			WithOperation("create_model").
			WithOriginalErr(err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "create request failed").
			WithOperation("create_model").
			WithOriginalErr(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeOllama, resp.StatusCode, string(body)).
			WithOperation("create_model")
	}

	// Stream the progress updates
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var createResp ollamaCreateResponse
		if err := json.Unmarshal(line, &createResp); err != nil {
			// Skip malformed lines
			continue
		}

		// Log progress
		if p.BaseProvider != nil {
			p.BaseProvider.LogRequest("CREATE", "progress", nil, createResp)
		}
	}

	if err := scanner.Err(); err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "failed to read create response").
			WithOperation("create_model").
			WithOriginalErr(err)
	}

	return nil
}
