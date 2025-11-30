// Package openai provides integration with OpenAI's GPT models including
// chat completions, streaming, tool calling, and authentication support.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// OpenAI API Request/Response Structures

// OpenAIRequest represents a request to the OpenAI chat completions API
type OpenAIRequest struct {
	Model             string                 `json:"model"`
	Messages          []OpenAIMessage        `json:"messages"`
	MaxTokens         int                    `json:"max_tokens,omitempty"`
	Temperature       float64                `json:"temperature,omitempty"`
	Stream            bool                   `json:"stream,omitempty"`
	TopP              float64                `json:"top_p,omitempty"`
	Tools             []OpenAITool           `json:"tools,omitempty"`
	ToolChoice        interface{}            `json:"tool_choice,omitempty"`
	Stop              []string               `json:"stop,omitempty"`
	Seed              *int                   `json:"seed,omitempty"`
	ResponseFormat    map[string]interface{} `json:"response_format,omitempty"`
	ParallelToolCalls *bool                  `json:"parallel_tool_calls,omitempty"`
}

// OpenAITool represents a tool in the OpenAI API
type OpenAITool struct {
	Type     string            `json:"type"` // Always "function"
	Function OpenAIFunctionDef `json:"function"`
}

// OpenAIFunctionDef represents a function definition in the OpenAI API
type OpenAIFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// OpenAIMessage represents a message in the OpenAI API
type OpenAIMessage struct {
	Role             string           `json:"role"`
	Content          interface{}      `json:"content"`                     // string or []OpenAIContentPart
	Reasoning        string           `json:"reasoning,omitempty"`         // Reasoning content (GLM-4.6, OpenCode/Zen)
	ReasoningContent string           `json:"reasoning_content,omitempty"` // Alternative reasoning field (vLLM/Synthetic)
	ToolCalls        []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
}

// OpenAIContentPart represents a content part in OpenAI's multimodal format
type OpenAIContentPart struct {
	Type     string          `json:"type"`                // "text" or "image_url"
	Text     string          `json:"text,omitempty"`      // Text content
	ImageURL *OpenAIImageURL `json:"image_url,omitempty"` // Image URL content
}

// OpenAIImageURL represents an image URL in OpenAI format
type OpenAIImageURL struct {
	URL    string `json:"url"`              // URL or data URL
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

// OpenAIToolCall represents a tool call in the OpenAI API
type OpenAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"` // "function"
	Function OpenAIToolCallFunction `json:"function"`
}

// OpenAIToolCallFunction represents a function call in a tool call
type OpenAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// OpenAIResponse represents a response from the OpenAI API
type OpenAIResponse struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []OpenAIChoice `json:"choices"`
	Usage             OpenAIUsage    `json:"usage"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
}

// OpenAIChoice represents a choice in the OpenAI API response
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAIUsage represents token usage information from OpenAI
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIStreamResponse represents a streaming response chunk
type OpenAIStreamResponse struct {
	ID                string               `json:"id"`
	Object            string               `json:"object"`
	Created           int64                `json:"created"`
	Model             string               `json:"model"`
	Choices           []OpenAIStreamChoice `json:"choices"`
	Usage             *OpenAIUsage         `json:"usage,omitempty"`
	SystemFingerprint string               `json:"system_fingerprint,omitempty"`
}

// OpenAIStreamChoice represents a choice in the streaming response
type OpenAIStreamChoice struct {
	Index        int         `json:"index"`
	Delta        OpenAIDelta `json:"delta"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// OpenAIDelta represents the delta content in a streaming response
type OpenAIDelta struct {
	Role             string           `json:"role,omitempty"`
	Content          string           `json:"content,omitempty"`
	Reasoning        string           `json:"reasoning,omitempty"`         // Reasoning content (GLM-4.6, OpenCode/Zen)
	ReasoningContent string           `json:"reasoning_content,omitempty"` // Alternative reasoning field (vLLM/Synthetic)
	ToolCalls        []OpenAIToolCall `json:"tool_calls,omitempty"`
}

// OpenAIErrorResponse represents an error response from OpenAI
type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

// OpenAIError represents an error in the OpenAI response
type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// OpenAIModelsResponse represents the response from the models API
type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// OpenAIModel represents a model in the OpenAI API
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	*base.BaseProvider
	authHelper      *common.AuthHelper
	client          *http.Client
	baseURL         string
	useResponsesAPI bool
	rateLimitHelper *common.RateLimitHelper
	modelCache      *common.ModelCache
	modelRegistry   *common.ModelMetadataRegistry
	organizationID  string
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(config types.ProviderConfig) *OpenAIProvider {
	// Use the shared config helper
	configHelper := common.NewConfigHelper("OpenAI", types.ProviderTypeOpenAI)

	// Merge with defaults and extract configuration
	mergedConfig := configHelper.MergeWithDefaults(config)

	client := &http.Client{
		Timeout: configHelper.ExtractTimeout(mergedConfig),
	}

	// Extract configuration using helper
	baseURL := configHelper.ExtractBaseURL(mergedConfig)
	organizationID := configHelper.ExtractStringField(mergedConfig, "organization_id", "")

	// Create auth helper
	authHelper := common.NewAuthHelper("openai", mergedConfig, client)

	// Setup API keys using shared helper
	authHelper.SetupAPIKeys()

	// OpenAI doesn't use OAuth, so no OAuth setup needed

	// Handle capability flags properly - preserve explicit values from config
	useResponsesAPI := mergedConfig.SupportsResponsesAPI
	if !config.SupportsResponsesAPI {
		useResponsesAPI = false
	} else if config.SupportsResponsesAPI {
		useResponsesAPI = true
	}

	provider := &OpenAIProvider{
		BaseProvider:    base.NewBaseProvider("openai", mergedConfig, client, log.Default()),
		authHelper:      authHelper,
		client:          client,
		baseURL:         baseURL,
		useResponsesAPI: useResponsesAPI,
		rateLimitHelper: common.NewRateLimitHelper(ratelimit.NewOpenAIParser()),
		organizationID:  organizationID,
		modelCache:      common.NewModelCache(24 * time.Hour), // 24 hour cache for OpenAI
		modelRegistry:   common.GetOpenAIMetadataRegistry(),
	}

	return provider
}

func (p *OpenAIProvider) Name() string {
	return "OpenAI"
}

func (p *OpenAIProvider) Type() types.ProviderType {
	return types.ProviderTypeOpenAI
}

func (p *OpenAIProvider) Description() string {
	return "OpenAI - GPT models with native API access"
}

func (p *OpenAIProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	// Use the shared model cache utility
	return p.modelCache.GetModels(
		func() ([]types.Model, error) {
			// Fetch from API
			models, err := p.fetchModelsFromAPI(ctx)
			if err != nil {
				log.Printf("OpenAI: Failed to fetch models from API: %v", err)
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

// fetchModelsFromAPI fetches models from OpenAI API
func (p *OpenAIProvider) fetchModelsFromAPI(ctx context.Context) ([]types.Model, error) {
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		return nil, fmt.Errorf("no OpenAI API key configured")
	}

	url := p.baseURL + "/models"

	// Use first available API key
	keys := p.authHelper.KeyManager.GetKeys()
	if len(keys) == 0 {
		return nil, fmt.Errorf("no API keys available")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Use auth helper to set headers
	p.authHelper.SetAuthHeaders(req, keys[0], "api_key")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch models: HTTP %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var modelsResp OpenAIModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	// Convert to internal Model format - no filtering to support OpenAI-compatible providers
	models := make([]types.Model, 0, len(modelsResp.Data))
	for _, model := range modelsResp.Data {
		models = append(models, types.Model{
			ID:       model.ID,
			Provider: p.Type(),
		})
	}

	return models, nil
}

// enrichModels adds metadata to models using the shared registry
func (p *OpenAIProvider) enrichModels(models []types.Model) []types.Model {
	return p.modelRegistry.EnrichModels(models)
}

// getStaticFallback returns static model list using the shared fallback
func (p *OpenAIProvider) getStaticFallback() []types.Model {
	return common.GetStaticFallback(p.Type())
}

func (p *OpenAIProvider) GetDefaultModel() string {
	config := p.GetConfig()
	if config.DefaultModel != "" {
		return config.DefaultModel
	}
	return "gpt-4o"
}

// GenerateChatCompletion generates a chat completion
func (p *OpenAIProvider) GenerateChatCompletion(
	ctx context.Context,
	options types.GenerateOptions,
) (types.ChatCompletionStream, error) {
	// Increment request count at the start
	p.IncrementRequestCount()

	// Track start time for latency measurement
	startTime := time.Now()

	// Build OpenAI request
	requestData := p.buildOpenAIRequest(options)

	// Check rate limits before making request
	model := requestData.Model
	p.rateLimitHelper.CheckRateLimitAndWait(model, options.MaxTokens)

	// Check if streaming is requested
	if options.Stream {
		stream, err := p.executeStreamWithAuth(ctx, requestData)
		if err != nil {
			p.RecordError(err)
			return nil, err
		}
		latency := time.Since(startTime)
		p.RecordSuccess(latency, 0) // Tokens will be counted as stream is consumed
		return stream, nil
	}

	// Non-streaming path - use auth helper
	var responseMessage types.ChatMessage
	var usage *types.Usage
	var responseContent string
	var callErr error

	// Define API key operation (OpenAI only supports API key auth)
	apiKeyOperation := func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
		msg, u, err := p.makeAPICall(ctx, requestData, apiKey)
		if err != nil {
			return "", nil, err
		}
		responseMessage = msg
		return msg.Content, u, nil
	}

	// Use auth helper to execute with API key only (no OAuth support)
	if p.authHelper.KeyManager != nil {
		responseContent, usage, callErr = p.authHelper.KeyManager.ExecuteWithFailover(ctx, apiKeyOperation)
	} else {
		callErr = fmt.Errorf("no API keys configured for OpenAI")
	}

	if callErr != nil {
		p.RecordError(callErr)
		return nil, callErr
	}

	// Record success metrics
	latency := time.Since(startTime)
	tokensUsed := int64(0)
	if usage != nil {
		tokensUsed = int64(usage.TotalTokens)
	}
	p.RecordSuccess(latency, tokensUsed)

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

	return common.NewMockStream([]types.ChatCompletionChunk{chunk}), nil
}

// executeStreamWithAuth handles streaming requests with authentication
func (p *OpenAIProvider) executeStreamWithAuth(ctx context.Context, requestData OpenAIRequest) (types.ChatCompletionStream, error) {
	requestData.Stream = true

	// Try API keys (OpenAI doesn't use OAuth)
	if p.authHelper.KeyManager != nil {
		keys := p.authHelper.KeyManager.GetKeys()
		for _, apiKey := range keys {
			stream, err := p.makeStreamingAPICall(ctx, requestData, apiKey)
			if err == nil {
				return stream, nil
			}
		}
	}

	return nil, fmt.Errorf("no valid API key available for streaming")
}

// buildOpenAIRequest builds the OpenAI API request from options
func (p *OpenAIProvider) buildOpenAIRequest(options types.GenerateOptions) OpenAIRequest {
	// Determine which model to use: options.Model takes precedence over default
	model := options.Model
	if model == "" {
		config := p.GetConfig()
		model = config.DefaultModel
		if model == "" {
			model = "gpt-3.5-turbo"
		}
	}

	// Convert messages to OpenAI format
	var messages []OpenAIMessage

	if len(options.Messages) > 0 {
		// Use provided messages directly
		messages = make([]OpenAIMessage, len(options.Messages))
		for i, msg := range options.Messages {
			// Use GetContentParts() to get unified content access
			parts := msg.GetContentParts()
			var content interface{}
			if len(parts) > 0 {
				content = convertContentPartsToOpenAI(parts)
			} else {
				content = msg.Content
			}

			openaiMsg := OpenAIMessage{
				Role:    msg.Role,
				Content: content,
			}

			// Convert tool calls if present
			if len(msg.ToolCalls) > 0 {
				openaiMsg.ToolCalls = convertToOpenAIToolCalls(msg.ToolCalls)
			}

			// Include tool call ID for tool response messages
			if msg.ToolCallID != "" {
				openaiMsg.ToolCallID = msg.ToolCallID
			}

			messages[i] = openaiMsg
		}
	} else if options.Prompt != "" {
		// Convert prompt to user message
		messages = append(messages, OpenAIMessage{
			Role:    "user",
			Content: options.Prompt,
		})
	}

	request := OpenAIRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   options.MaxTokens,
		Temperature: options.Temperature,
		Stream:      options.Stream,
	}

	// Convert tools if provided
	if len(options.Tools) > 0 {
		request.Tools = convertToOpenAITools(options.Tools)

		// Convert tool choice if specified
		if options.ToolChoice != nil {
			request.ToolChoice = convertToOpenAIToolChoice(options.ToolChoice)
		}
		// Otherwise, ToolChoice defaults to "auto" (OpenAI's default behavior)
	}

	return request
}

// makeAPICall makes a single API call to OpenAI
func (p *OpenAIProvider) makeAPICall(ctx context.Context, requestData OpenAIRequest, apiKey string) (types.ChatMessage, *types.Usage, error) {
	// Serialize request
	jsonBody, err := json.Marshal(requestData)
	if err != nil {
		return types.ChatMessage{}, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := p.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return types.ChatMessage{}, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Use auth helper to set headers
	req.Header.Set("Content-Type", "application/json")
	p.authHelper.SetAuthHeaders(req, apiKey, "api_key")
	p.authHelper.SetProviderSpecificHeaders(req)

	// Log request (for debugging)
	p.LogRequest("POST", url, map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer ***",
	}, requestData)

	// Make the request
	resp, err := p.client.Do(req)
	if err != nil {
		return types.ChatMessage{}, nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.ChatMessage{}, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse and update rate limit info
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, requestData.Model)

	// Check status code
	if resp.StatusCode != http.StatusOK {
		var errorResponse OpenAIErrorResponse
		if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil {
			// Handle specific error types
			switch errorResponse.Error.Type {
			case "invalid_api_key":
				return types.ChatMessage{}, nil, fmt.Errorf("invalid OpenAI API key")
			case "insufficient_quota":
				return types.ChatMessage{}, nil, fmt.Errorf("OpenAI quota exceeded")
			case "rate_limit_exceeded":
				return types.ChatMessage{}, nil, fmt.Errorf("OpenAI rate limit exceeded")
			case "model_not_found":
				return types.ChatMessage{}, nil, fmt.Errorf("OpenAI model not found: %s", errorResponse.Error.Message)
			case "invalid_request_error":
				return types.ChatMessage{}, nil, fmt.Errorf("invalid OpenAI request: %s", errorResponse.Error.Message)
			default:
				return types.ChatMessage{}, nil, fmt.Errorf("OpenAI API error (%s): %s", errorResponse.Error.Type, errorResponse.Error.Message)
			}
		}
		return types.ChatMessage{}, nil, fmt.Errorf("OpenAI API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var response OpenAIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return types.ChatMessage{}, nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(response.Choices) == 0 {
		return types.ChatMessage{}, nil, fmt.Errorf("no choices in API response")
	}

	// Extract message from response
	openaiMsg := response.Choices[0].Message

	// Determine effective content - fallback to reasoning fields if content is empty
	effectiveContent := ""
	if contentStr, ok := openaiMsg.Content.(string); ok {
		effectiveContent = contentStr
	}
	if effectiveContent == "" || effectiveContent == "\n" {
		// Try reasoning_content first (vLLM/Synthetic style)
		if openaiMsg.ReasoningContent != "" {
			effectiveContent = openaiMsg.ReasoningContent
		} else if openaiMsg.Reasoning != "" {
			// Fall back to reasoning field (GLM-4.6, OpenCode/Zen style)
			effectiveContent = openaiMsg.Reasoning
		}
	}

	// Convert to universal format
	message := types.ChatMessage{
		Role:             openaiMsg.Role,
		Content:          effectiveContent,
		Reasoning:        openaiMsg.Reasoning,
		ReasoningContent: openaiMsg.ReasoningContent,
	}

	// Convert tool calls if present
	if len(openaiMsg.ToolCalls) > 0 {
		message.ToolCalls = convertOpenAIToolCallsToUniversal(openaiMsg.ToolCalls)
	}

	// Convert usage
	usage := &types.Usage{
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		TotalTokens:      response.Usage.TotalTokens,
	}

	return message, usage, nil
}

// makeStreamingAPICall makes a streaming API call to OpenAI
func (p *OpenAIProvider) makeStreamingAPICall(ctx context.Context, requestData OpenAIRequest, apiKey string) (types.ChatCompletionStream, error) {
	requestData.Stream = true
	jsonBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	p.authHelper.SetAuthHeaders(req, apiKey, "api_key")
	p.authHelper.SetProviderSpecificHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Parse and update rate limit info from response headers
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, requestData.Model)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }()
		return nil, fmt.Errorf("OpenAI API error: %d - %s", resp.StatusCode, string(body))
	}

	// Use the shared streaming utility
	stream := common.CreateOpenAIStream(resp)
	return common.StreamFromContext(ctx, stream), nil
}

// InvokeServerTool invokes a server tool (not yet implemented)
func (p *OpenAIProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, fmt.Errorf("tool invocation not yet implemented for OpenAI provider")
}

func (p *OpenAIProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	// OpenAI only supports API key authentication
	if authConfig.Method != types.AuthMethodAPIKey {
		return fmt.Errorf("OpenAI only supports API key authentication")
	}

	// Update config with new authentication, preserving capability flags
	newConfig := p.authHelper.Config
	newConfig.APIKey = authConfig.APIKey
	// Only update BaseURL and DefaultModel if they're provided in authConfig
	if authConfig.BaseURL != "" {
		newConfig.BaseURL = authConfig.BaseURL
	}
	if authConfig.DefaultModel != "" {
		newConfig.DefaultModel = authConfig.DefaultModel
	}
	// Preserve current capability flags
	newConfig.SupportsResponsesAPI = p.useResponsesAPI

	return p.Configure(newConfig)
}

func (p *OpenAIProvider) IsAuthenticated() bool {
	return p.authHelper.IsAuthenticated()
}

// GetAuthStatus provides detailed authentication status using shared helper
func (p *OpenAIProvider) GetAuthStatus() map[string]interface{} {
	return p.authHelper.GetAuthStatus()
}

// Logout clears the API keys and resets configuration
func (p *OpenAIProvider) Logout(ctx context.Context) error {
	p.authHelper.ClearAuthentication()
	newConfig := p.authHelper.Config
	newConfig.APIKey = ""
	// Preserve current capability flags
	newConfig.SupportsResponsesAPI = p.useResponsesAPI
	return p.Configure(newConfig)
}

func (p *OpenAIProvider) Configure(config types.ProviderConfig) error {
	// Use the shared config helper for validation and extraction
	configHelper := common.NewConfigHelper("OpenAI", types.ProviderTypeOpenAI)

	// Validate configuration
	validation := configHelper.ValidateProviderConfig(config)
	if !validation.Valid {
		return fmt.Errorf("configuration validation failed: %s", validation.Errors[0])
	}

	// Merge with defaults
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Extract configuration using helper
	p.baseURL = configHelper.ExtractBaseURL(mergedConfig)
	p.organizationID = configHelper.ExtractStringField(mergedConfig, "organization_id", "")

	// Handle capability flags properly - preserve existing values for minimal configs
	// If this appears to be a minimal config (only auth changes), preserve existing flags
	isMinimalConfig := config.Type == types.ProviderTypeOpenAI &&
		config.BaseURL == "" &&
		config.DefaultModel == "" &&
		!config.SupportsToolCalling &&
		!config.SupportsStreaming &&
		!config.SupportsResponsesAPI &&
		config.MaxTokens == 0 &&
		config.Timeout == 0

	switch {
	case isMinimalConfig:
		// This is likely a configuration update for authentication only
		// Preserve the existing useResponsesAPI value
		// Don't change it unless explicitly specified
	case !config.SupportsResponsesAPI:
		// Explicitly set to false in the config
		p.useResponsesAPI = false
	default:
		// Use the merged value (either from config or defaults)
		p.useResponsesAPI = mergedConfig.SupportsResponsesAPI
	}

	// Update auth helper configuration
	p.authHelper.Config = mergedConfig

	// Re-setup authentication with new config
	p.authHelper.SetupAPIKeys()

	return p.BaseProvider.Configure(mergedConfig)
}

func (p *OpenAIProvider) SupportsToolCalling() bool {
	return true
}

func (p *OpenAIProvider) SupportsStreaming() bool {
	return true
}

func (p *OpenAIProvider) SupportsResponsesAPI() bool {
	return p.useResponsesAPI
}

func (p *OpenAIProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

// convertToOpenAITools converts universal tools to OpenAI format
func convertToOpenAITools(tools []types.Tool) []OpenAITool {
	openaiTools := make([]OpenAITool, len(tools))
	for i, tool := range tools {
		openaiTools[i] = OpenAITool{
			Type: "function",
			Function: OpenAIFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}
	return openaiTools
}

// convertToOpenAIToolCalls converts universal tool calls to OpenAI format
func convertToOpenAIToolCalls(toolCalls []types.ToolCall) []OpenAIToolCall {
	openaiToolCalls := make([]OpenAIToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		openaiToolCalls[i] = OpenAIToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: OpenAIToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return openaiToolCalls
}

// convertOpenAIToolCallsToUniversal converts OpenAI tool calls to universal format
func convertOpenAIToolCallsToUniversal(toolCalls []OpenAIToolCall) []types.ToolCall {
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

// convertToOpenAIToolChoice converts universal ToolChoice to OpenAI format
func convertToOpenAIToolChoice(toolChoice *types.ToolChoice) interface{} {
	if toolChoice == nil {
		return nil
	}

	switch toolChoice.Mode {
	case types.ToolChoiceAuto:
		return "auto"
	case types.ToolChoiceRequired:
		// OpenAI uses "required" for newer models (gpt-4-turbo and later)
		// For older models, "any" was used, but "required" is now the standard
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

// convertContentPartsToOpenAI converts ContentParts to OpenAI format
// Returns a string if there's only text, or []OpenAIContentPart if multimodal
func convertContentPartsToOpenAI(parts []types.ContentPart) interface{} {
	if len(parts) == 0 {
		return ""
	}

	// If only one part and it's text, return as string for backwards compatibility
	if len(parts) == 1 && parts[0].Type == types.ContentTypeText {
		return parts[0].Text
	}

	// Otherwise, build multimodal content array
	openaiParts := make([]OpenAIContentPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case types.ContentTypeText:
			openaiParts = append(openaiParts, OpenAIContentPart{
				Type: "text",
				Text: part.Text,
			})
		case types.ContentTypeImage:
			if part.Source != nil {
				url := ""
				if part.Source.Type == types.MediaSourceBase64 {
					// Build data URL for base64
					url = fmt.Sprintf("data:%s;base64,%s", part.Source.MediaType, part.Source.Data)
				} else if part.Source.Type == types.MediaSourceURL {
					url = part.Source.URL
				}
				if url != "" {
					openaiParts = append(openaiParts, OpenAIContentPart{
						Type: "image_url",
						ImageURL: &OpenAIImageURL{
							URL: url,
						},
					})
				}
			}
			// Note: OpenAI doesn't have native support for documents/audio in chat completions
			// These would need to be handled separately (e.g., via file uploads or transcription)
			// For now, we skip them
		}
	}

	return openaiParts
}
