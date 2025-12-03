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
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/models"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/streaming"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Constants for OpenAI models
const (
	openAIDefaultModel  = "gpt-4o"        // Default model for chat completions
	openAIFallbackModel = "gpt-3.5-turbo" // Fallback model when no default specified
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
	authHelper        *auth.AuthHelper
	client            *http.Client
	baseURL           string
	useResponsesAPI   bool
	rateLimitHelper   *common.RateLimitHelper
	modelCache        *models.ModelCache
	modelRegistry     *models.ModelMetadataRegistry
	organizationID    string
	connectivityCache *common.ConnectivityCache
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(config types.ProviderConfig) *OpenAIProvider {
	// Use the shared config helper
	configHelper := commonconfig.NewConfigHelper("OpenAI", types.ProviderTypeOpenAI)

	// Merge with defaults and extract configuration
	mergedConfig := configHelper.MergeWithDefaults(config)

	client := &http.Client{
		Timeout: configHelper.ExtractTimeout(mergedConfig),
	}

	// Extract configuration using helper
	baseURL := configHelper.ExtractBaseURL(mergedConfig)
	organizationID := configHelper.ExtractStringField(mergedConfig, "organization_id", "")

	// Create auth helper
	authHelper := auth.NewAuthHelper("openai", mergedConfig, client)

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
		BaseProvider:      base.NewBaseProvider("openai", mergedConfig, client, log.Default()),
		authHelper:        authHelper,
		client:            client,
		baseURL:           baseURL,
		useResponsesAPI:   useResponsesAPI,
		rateLimitHelper:   common.NewRateLimitHelper(ratelimit.NewOpenAIParser()),
		organizationID:    organizationID,
		modelCache:        models.NewModelCache(24 * time.Hour), // 24 hour cache for OpenAI
		modelRegistry:     models.GetOpenAIMetadataRegistry(),
		connectivityCache: common.NewDefaultConnectivityCache(),
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
		return nil, types.NewAuthError(types.ProviderTypeOpenAI, "no OpenAI API key configured").
			WithOperation("fetchModelsFromAPI")
	}

	url := p.baseURL + "/models"

	// Use first available API key
	keys := p.authHelper.KeyManager.GetKeys()
	if len(keys) == 0 {
		return nil, types.NewAuthError(types.ProviderTypeOpenAI, "no API keys available").
			WithOperation("fetchModelsFromAPI")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenAI, "failed to create request").
			WithOperation("fetchModelsFromAPI").
			WithOriginalErr(err)
	}

	// Use auth helper to set headers
	p.authHelper.SetAuthHeaders(req, keys[0], "api_key")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenAI, "failed to fetch models").
			WithOperation("fetchModelsFromAPI").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, types.NewServerError(types.ProviderTypeOpenAI, resp.StatusCode, fmt.Sprintf("failed to fetch models: %s", string(body))).
			WithOperation("fetchModelsFromAPI")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenAI, "failed to read response").
			WithOperation("fetchModelsFromAPI").
			WithOriginalErr(err)
	}

	var modelsResp OpenAIModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeOpenAI, "failed to parse models response").
			WithOperation("fetchModelsFromAPI").
			WithOriginalErr(err)
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
	return models.GetStaticFallback(p.Type())
}

func (p *OpenAIProvider) GetDefaultModel() string {
	config := p.GetConfig()
	if config.DefaultModel != "" {
		return config.DefaultModel
	}
	return openAIDefaultModel
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

	return streaming.NewMockStream([]types.ChatCompletionChunk{chunk}), nil
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

	return nil, types.NewAuthError(types.ProviderTypeOpenAI, "no valid API key available for streaming").
		WithOperation("executeStreamWithAuth")
}

// buildOpenAIRequest builds the OpenAI API request from options
func (p *OpenAIProvider) buildOpenAIRequest(options types.GenerateOptions) OpenAIRequest {
	// Determine which model to use: options.Model takes precedence over default
	model := options.Model
	if model == "" {
		config := p.GetConfig()
		model = config.DefaultModel
		if model == "" {
			model = openAIFallbackModel
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
		return types.ChatMessage{}, nil, types.NewInvalidRequestError(types.ProviderTypeOpenAI, "failed to marshal request").
			WithOperation("makeAPICall").
			WithOriginalErr(err)
	}

	// Create HTTP request
	url := p.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return types.ChatMessage{}, nil, types.NewNetworkError(types.ProviderTypeOpenAI, "failed to create request").
			WithOperation("makeAPICall").
			WithOriginalErr(err)
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
		return types.ChatMessage{}, nil, types.NewNetworkError(types.ProviderTypeOpenAI, "request failed").
			WithOperation("makeAPICall").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.ChatMessage{}, nil, types.NewNetworkError(types.ProviderTypeOpenAI, "failed to read response body").
			WithOperation("makeAPICall").
			WithOriginalErr(err)
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
				return types.ChatMessage{}, nil, types.NewAuthError(types.ProviderTypeOpenAI, "invalid OpenAI API key").
					WithOperation("makeAPICall").
					WithStatusCode(resp.StatusCode)
			case "insufficient_quota":
				return types.ChatMessage{}, nil, &types.ProviderError{
					Code:       types.ErrCodeRateLimit,
					Message:    "OpenAI quota exceeded",
					Provider:   types.ProviderTypeOpenAI,
					StatusCode: resp.StatusCode,
					Operation:  "makeAPICall",
				}
			case "rate_limit_exceeded":
				return types.ChatMessage{}, nil, types.NewRateLimitError(types.ProviderTypeOpenAI, 0).
					WithOperation("makeAPICall").
					WithStatusCode(resp.StatusCode).
					WithOriginalErr(fmt.Errorf("OpenAI rate limit exceeded"))
			case "model_not_found":
				return types.ChatMessage{}, nil, types.NewNotFoundError(types.ProviderTypeOpenAI, fmt.Sprintf("OpenAI model not found: %s", errorResponse.Error.Message)).
					WithOperation("makeAPICall").
					WithStatusCode(resp.StatusCode)
			case "invalid_request_error":
				return types.ChatMessage{}, nil, types.NewInvalidRequestError(types.ProviderTypeOpenAI, fmt.Sprintf("invalid OpenAI request: %s", errorResponse.Error.Message)).
					WithOperation("makeAPICall").
					WithStatusCode(resp.StatusCode)
			default:
				return types.ChatMessage{}, nil, types.NewServerError(types.ProviderTypeOpenAI, resp.StatusCode, fmt.Sprintf("OpenAI API error (%s): %s", errorResponse.Error.Type, errorResponse.Error.Message)).
					WithOperation("makeAPICall")
			}
		}
		return types.ChatMessage{}, nil, types.NewServerError(types.ProviderTypeOpenAI, resp.StatusCode, fmt.Sprintf("OpenAI API error: %s", string(body))).
			WithOperation("makeAPICall")
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
		return nil, types.NewInvalidRequestError(types.ProviderTypeOpenAI, "failed to marshal request").
			WithOperation("makeStreamingAPICall").
			WithOriginalErr(err)
	}

	url := p.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenAI, "failed to create request").
			WithOperation("makeStreamingAPICall").
			WithOriginalErr(err)
	}

	req.Header.Set("Content-Type", "application/json")
	p.authHelper.SetAuthHeaders(req, apiKey, "api_key")
	p.authHelper.SetProviderSpecificHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenAI, "request failed").
			WithOperation("makeStreamingAPICall").
			WithOriginalErr(err)
	}

	// Parse and update rate limit info from response headers
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, requestData.Model)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }()
		return nil, types.NewServerError(types.ProviderTypeOpenAI, resp.StatusCode, fmt.Sprintf("OpenAI API error: %s", string(body))).
			WithOperation("makeStreamingAPICall")
	}

	// Use the shared streaming utility
	stream := streaming.CreateOpenAIStream(resp)
	return streaming.StreamFromContext(ctx, stream), nil
}

// InvokeServerTool invokes a server tool (not yet implemented)
func (p *OpenAIProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, types.NewInvalidRequestError(types.ProviderTypeOpenAI, "tool invocation not yet implemented for OpenAI provider").
		WithOperation("InvokeTool")
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
	configHelper := commonconfig.NewConfigHelper("OpenAI", types.ProviderTypeOpenAI)

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

// TestConnectivity performs a lightweight connectivity test using the /v1/models endpoint
// Results are cached for 30 seconds by default to prevent hammering the API during rapid health checks
// To bypass the cache and force a fresh check, use TestConnectivityWithOptions with bypassCache=true
func (p *OpenAIProvider) TestConnectivity(ctx context.Context) error {
	return p.TestConnectivityWithOptions(ctx, false)
}

// TestConnectivityWithOptions performs a connectivity test with optional cache bypass
// If bypassCache is true, the cache is bypassed and a fresh connectivity check is performed
func (p *OpenAIProvider) TestConnectivityWithOptions(ctx context.Context, bypassCache bool) error {
	return p.connectivityCache.TestConnectivity(
		ctx,
		types.ProviderTypeOpenAI,
		p.performConnectivityTest,
		bypassCache,
	)
}

// performConnectivityTest performs the actual connectivity test without caching
func (p *OpenAIProvider) performConnectivityTest(ctx context.Context) error {
	// Check if we have API keys configured
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		return types.NewAuthError(types.ProviderTypeOpenAI, "no API keys configured").
			WithOperation("test_connectivity")
	}

	// Use the first available API key for connectivity test
	keys := p.authHelper.KeyManager.GetKeys()
	apiKey := keys[0]

	// Create a request to the /v1/models endpoint
	url := p.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOpenAI, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Set authentication headers
	p.authHelper.SetAuthHeaders(req, apiKey, "api_key")

	// Make the request with a shorter timeout for connectivity testing
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOpenAI, "connectivity test failed").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		return types.NewAuthError(types.ProviderTypeOpenAI, "invalid API key").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(types.ProviderTypeOpenAI, "API key does not have access to models endpoint").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeOpenAI, resp.StatusCode,
			fmt.Sprintf("connectivity test failed: %s", string(body))).
			WithOperation("test_connectivity")
	}

	// Read entire response to support providers with many models (e.g., Groq with 20+ models)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOpenAI, "failed to read response body").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	var testResponse struct {
		Data []interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &testResponse); err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeOpenAI, "invalid response from models endpoint").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Successfully parsed response - connectivity verified
	// No Object field validation to support OpenAI-compatible providers (Groq, xAI, etc.)
	return nil
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
