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

	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/streaming"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Constants for OpenAI models
const (
	openAIDefaultModel  = "gpt-4o"        // Default model for chat completions
	openAIFallbackModel = "gpt-3.5-turbo" // Fallback model when no default specified
)

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

// buildOpenAIRequest builds the OpenAI API request from options
func (p *OpenAIProvider) buildOpenAIRequest(options types.GenerateOptions) OpenAIRequest {
	// Determine which model to use with fallback priority
	config := p.GetConfig()
	model := common.ResolveModel(options.Model, config.DefaultModel, openAIFallbackModel)

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

	// Handle structured outputs via ResponseFormat
	// OpenAI supports JSON schema validation with {"type":"json_schema", "json_schema": {...}}
	if options.ResponseFormat != "" {
		// Try to parse as JSON schema first
		var schemaObj map[string]interface{}
		if err := json.Unmarshal([]byte(options.ResponseFormat), &schemaObj); err == nil {
			// It's a valid JSON object - wrap it in OpenAI's json_schema format
			request.ResponseFormat = map[string]interface{}{
				"type":        "json_schema",
				"json_schema": schemaObj,
			}
		} else {
			// It's a string like "json_object", use it directly
			request.ResponseFormat = map[string]interface{}{
				"type": options.ResponseFormat,
			}
		}
	}

	// Handle thinking mode (GLM-4.6, DeepSeek)
	if options.Thinking != nil {
		request.Thinking = map[string]interface{}{
			"type": options.Thinking.Type,
		}
		// Note: BudgetTokens is Claude-specific, GLM only supports type
	}

	// Handle reasoning effort (OpenAI o1/o3 models)
	if options.ReasoningEffort != "" {
		request.ReasoningEffort = string(options.ReasoningEffort)
	}

	return request
}

// makeAPICall makes a single API call to OpenAI
func (p *OpenAIProvider) makeAPICall(ctx context.Context, requestData OpenAIRequest, apiKey string) (types.ChatMessage, *types.Usage, error) {
	return p.makeAPICallInternal(ctx, requestData, apiKey, false)
}

// handleOpenAIError converts an OpenAI error response to a typed error
func handleOpenAIError(body []byte, statusCode int) error {
	var errorResponse OpenAIErrorResponse
	if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil {
		switch errorResponse.Error.Type {
		case "invalid_api_key":
			return types.NewAuthError(types.ProviderTypeOpenAI, "invalid OpenAI API key").
				WithOperation("makeAPICall").
				WithStatusCode(statusCode)
		case "insufficient_quota":
			return &types.ProviderError{
				Code:       types.ErrCodeRateLimit,
				Message:    "OpenAI quota exceeded",
				Provider:   types.ProviderTypeOpenAI,
				StatusCode: statusCode,
				Operation:  "makeAPICall",
			}
		case "rate_limit_exceeded":
			return types.NewRateLimitError(types.ProviderTypeOpenAI, 0).
				WithOperation("makeAPICall").
				WithStatusCode(statusCode).
				WithOriginalErr(fmt.Errorf("OpenAI rate limit exceeded"))
		case "model_not_found":
			return types.NewNotFoundError(types.ProviderTypeOpenAI, fmt.Sprintf("OpenAI model not found: %s", errorResponse.Error.Message)).
				WithOperation("makeAPICall").
				WithStatusCode(statusCode)
		case "invalid_request_error":
			return types.NewInvalidRequestError(types.ProviderTypeOpenAI, fmt.Sprintf("invalid OpenAI request: %s", errorResponse.Error.Message)).
				WithOperation("makeAPICall").
				WithStatusCode(statusCode)
		default:
			return types.NewServerError(types.ProviderTypeOpenAI, statusCode, fmt.Sprintf("OpenAI API error (%s): %s", errorResponse.Error.Type, errorResponse.Error.Message)).
				WithOperation("makeAPICall")
		}
	}
	return types.NewServerError(types.ProviderTypeOpenAI, statusCode, fmt.Sprintf("OpenAI API error: %s", string(body))).
		WithOperation("makeAPICall")
}

// extractEffectiveContent extracts the effective content from an OpenAI message,
// falling back to reasoning fields if content is empty
func extractEffectiveContent(msg OpenAIMessage) string {
	effectiveContent := ""
	if contentStr, ok := msg.Content.(string); ok {
		effectiveContent = contentStr
	}
	if effectiveContent == "" || effectiveContent == "\n" {
		if msg.ReasoningContent != "" {
			effectiveContent = msg.ReasoningContent
		} else if msg.Reasoning != "" {
			effectiveContent = msg.Reasoning
		}
	}
	return effectiveContent
}

// makeAPICallInternal is the internal implementation that supports retry logic
func (p *OpenAIProvider) makeAPICallInternal(ctx context.Context, requestData OpenAIRequest, apiKey string, isRetry bool) (types.ChatMessage, *types.Usage, error) {
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
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

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
		return types.ChatMessage{}, nil, handleOpenAIError(body, resp.StatusCode)
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
	effectiveContent := extractEffectiveContent(openaiMsg)

	// GLM-4.6 "No response requested" retry logic
	// When GLM models return this specific response, retry with thinking enabled
	// This is an intermittent issue with complex agentic prompts
	if !isRetry && isGLMModel(requestData.Model) && isNoResponseRequested(effectiveContent) {
		log.Printf("[WARN] GLM model returned 'No response requested', retrying with thinking enabled (model: %s)", requestData.Model)
		retryRequest := requestData
		retryRequest.Thinking = map[string]interface{}{"type": "enabled"}
		return p.makeAPICallInternal(ctx, retryRequest, apiKey, true)
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
		ReasoningTokens:  response.Usage.ReasoningTokens,
	}

	return message, usage, nil
}
