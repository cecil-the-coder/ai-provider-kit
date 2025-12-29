// Package anthropic provides chat completion logic for Anthropic Claude AI provider.
package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/streaming"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// GenerateChatCompletion generates a chat completion
func (p *AnthropicProvider) GenerateChatCompletion(
	ctx context.Context,
	options types.GenerateOptions,
) (types.ChatCompletionStream, error) {
	log.Printf("🟣 [Anthropic] GenerateChatCompletion ENTRY - options.Model=%s, options.Stream=%v", options.Model, options.Stream)
	log.Printf("🟣 [Anthropic] authHelper=%p, OAuthManager=%p, KeyManager=%p",
		p.authHelper,
		func() interface{} {
			if p.authHelper != nil {
				return p.authHelper.OAuthManager
			} else {
				return nil
			}
		}(),
		func() interface{} {
			if p.authHelper != nil {
				return p.authHelper.KeyManager
			} else {
				return nil
			}
		}())

	p.IncrementRequestCount()
	startTime := time.Now()

	// Determine which model to use with fallback priority
	// Note: GetDefaultModel() returns empty for non-Anthropic OAuth to force model specification
	model := common.ResolveModel(options.Model, p.GetConfig().DefaultModel, p.GetDefaultModel())
	if model == "" {
		log.Printf("🔴 [Anthropic] ERROR: No model specified and no default available")
		return nil, types.NewInvalidRequestError(types.ProviderTypeAnthropic, "no model specified and no default model available (required when using third-party OAuth tokens)").
			WithOperation("GenerateChatCompletion")
	}
	if options.Model == "" {
		log.Printf("🟣 [Anthropic] Using default model: %s", model)
	} else {
		log.Printf("🟣 [Anthropic] Using specified model: %s", model)
	}

	// Check rate limits before making request
	maxTokens := options.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096 // Default max tokens
	}

	p.rateLimitHelper.CheckRateLimitAndWait(model, maxTokens)

	// Handle streaming
	if options.Stream {
		log.Printf("🟣 [Anthropic] Taking STREAMING path")
		stream, err := p.executeStreamWithAuth(ctx, options, model)
		if err != nil {
			log.Printf("🔴 [Anthropic] Streaming error: %v", err)
			p.RecordError(err)
			return nil, err
		}
		latency := time.Since(startTime)
		p.RecordSuccess(latency, 0) // Tokens will be counted as stream is consumed
		log.Printf("🟢 [Anthropic] Streaming SUCCESS")
		return stream, nil
	}

	log.Printf("🟣 [Anthropic] Taking NON-STREAMING path")
	// Non-streaming path - use auth helper with message support
	requestData := p.prepareRequest(options, model, maxTokens)
	log.Printf("🟣 [Anthropic] Request prepared")

	// Define OAuth operation (returns ChatMessage)
	oauthOperation := func(ctx context.Context, cred *types.OAuthCredentialSet) (types.ChatMessage, *types.Usage, error) {
		log.Printf("🟣 [Anthropic] OAuth operation called with cred.ID=%s", cred.ID)
		response, responseUsage, err := p.makeAPICallWithOAuthMessage(ctx, requestData, cred.AccessToken)
		if err != nil {
			return types.ChatMessage{}, nil, err
		}
		return response, responseUsage, nil
	}

	// Define API key operation (returns ChatMessage)
	apiKeyOperation := func(ctx context.Context, apiKey string) (types.ChatMessage, *types.Usage, error) {
		log.Printf("🟣 [Anthropic] API key operation called")
		response, responseUsage, err := p.makeAPICallWithKeyMessage(ctx, requestData, apiKey)
		if err != nil {
			return types.ChatMessage{}, nil, err
		}
		return response, responseUsage, nil
	}

	log.Printf("🟣 [Anthropic] About to call authHelper.ExecuteWithAuthMessage")
	// Use auth helper to execute with automatic failover (preserves tool calls)
	responseMessage, usage, err := p.authHelper.ExecuteWithAuthMessage(ctx, options, oauthOperation, apiKeyOperation)
	if err != nil {
		log.Printf("🔴 [Anthropic] ExecuteWithAuthMessage returned ERROR: %v", err)
		p.RecordError(err)
		return nil, err
	}
	log.Printf("🟢 [Anthropic] ExecuteWithAuthMessage returned SUCCESS")

	// Record success
	latency := time.Since(startTime)
	var tokensUsed int64
	if usage != nil {
		tokensUsed = int64(usage.TotalTokens)
	}
	p.RecordSuccess(latency, tokensUsed)
	p.lastUsage = usage

	// Return full message with tool call support
	var usageValue types.Usage
	if usage != nil {
		usageValue = *usage
	}

	chunk := types.ChatCompletionChunk{
		Content: responseMessage.Content,
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

// prepareRequest prepares the API request payload
func (p *AnthropicProvider) prepareRequest(options types.GenerateOptions, model string, maxTokens int) AnthropicRequest {
	log.Printf("🔧 [Anthropic] prepareRequest ENTRY - model=%s, Messages count=%d, Prompt=%q", model, len(options.Messages), options.Prompt)

	// Use the model parameter passed from GenerateChatCompletion
	// (already determined with options.Model fallback logic)

	// REQUIRED: All Anthropic requests via Claude Code must start with this system prompt
	claudeCodePrompt := "You are Claude Code, Anthropic's official CLI for Claude."

	var messages []AnthropicMessage
	var systemPrompts []string

	// Extract system messages from the Messages array and collect their content
	// Anthropic API requires system prompts in a separate System field, not in Messages
	if len(options.Messages) > 0 {
		log.Printf("🔧 [Anthropic] Processing %d messages", len(options.Messages))
		for i, msg := range options.Messages {
			log.Printf("🔧 [Anthropic] Message %d: role=%s, content_length=%d", i, msg.Role, len(msg.Content))
			if msg.Role == "system" {
				// Collect system message content to add to System field
				systemPrompts = append(systemPrompts, msg.Content)
				log.Printf("🟠 [Anthropic] Extracted system message from Messages array: %.100s...", msg.Content)
			} else {
				// Add non-system messages to the messages array
				// Determine the role - tool messages must be converted to "user" role for Anthropic API
				msgRole := msg.Role
				if msg.Role == "tool" {
					msgRole = "user"
				}

				messages = append(messages, AnthropicMessage{
					Role:    msgRole,
					Content: convertToAnthropicContent(msg),
				})
				log.Printf("🔧 [Anthropic] Added %s message to array", msg.Role)
			}
		}
	} else if options.Prompt != "" {
		log.Printf("🔧 [Anthropic] Using legacy Prompt field: %q", options.Prompt)
		// Convert prompt to user message
		messages = []AnthropicMessage{
			{
				Role:    "user",
				Content: options.Prompt,
			},
		}
	}

	log.Printf("🔧 [Anthropic] After processing: %d system prompts, %d messages", len(systemPrompts), len(messages))

	// Build the System field: Handle Claude Code identifier and system messages
	var fullSystemPrompt string

	// Check if any system prompt already contains the Claude Code identifier
	hasClaudeCodeIdentifier := false
	for _, sysPrompt := range systemPrompts {
		if strings.Contains(sysPrompt, "Claude Code") || strings.Contains(sysPrompt, "Anthropic's official CLI") {
			hasClaudeCodeIdentifier = true
			log.Printf("🔍 [Anthropic] Detected Claude Code identifier in system prompt")
			break
		}
	}

	if len(systemPrompts) > 0 {
		if hasClaudeCodeIdentifier {
			// Claude Code prompt is already in the system prompts, just join them
			fullSystemPrompt = systemPrompts[0]
			for i := 1; i < len(systemPrompts); i++ {
				fullSystemPrompt += "\n\n" + systemPrompts[i]
			}
			log.Printf("🔧 [Anthropic] Using existing Claude Code identifier from system prompts")
		} else {
			// Prepend claudeCodePrompt to the system prompts
			fullSystemPrompt = claudeCodePrompt + "\n\n" + systemPrompts[0]
			for i := 1; i < len(systemPrompts); i++ {
				fullSystemPrompt += "\n\n" + systemPrompts[i]
			}
			log.Printf("🔧 [Anthropic] Prepended Claude Code identifier to system prompts")
		}
	} else {
		// No system prompts provided, use claudeCodePrompt alone (no expert programmer fallback)
		fullSystemPrompt = claudeCodePrompt
		log.Printf("🔧 [Anthropic] Using only Claude Code identifier (no system prompts provided)")
	}

	// For OAuth, use array format to allow Claude Code beta headers
	// For API key, use string format
	var systemField interface{}
	authMethod := p.authHelper.GetAuthMethod()
	if authMethod == "oauth" {
		systemField = []interface{}{
			map[string]string{
				"type": "text",
				"text": fullSystemPrompt,
			},
		}
		log.Printf("🔧 [Anthropic] System field format: OAuth array, prompt_length=%d", len(fullSystemPrompt))
	} else {
		systemField = fullSystemPrompt
		log.Printf("🔧 [Anthropic] System field format: API key string, prompt_length=%d", len(fullSystemPrompt))
	}

	request := AnthropicRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    systemField,
		Messages:  messages,
	}

	log.Printf("🔧 [Anthropic] Request prepared: model=%s, messages_count=%d, has_system=%v", model, len(messages), systemField != nil)

	// Convert tools if provided
	if len(options.Tools) > 0 {
		request.Tools = convertToAnthropicTools(options.Tools)
		log.Printf("🔧 [Anthropic] Added %d tools to request", len(options.Tools))

		// Convert tool choice if specified
		if options.ToolChoice != nil {
			request.ToolChoice = convertToAnthropicToolChoice(options.ToolChoice)
		}
	}

	// Handle structured outputs via ResponseFormat
	// Anthropic supports structured outputs using tool-calling fallback with beta header
	// The beta header "anthropic-beta: structured-outputs-2025-11-13" enables JSON schema validation
	if options.ResponseFormat != "" {
		// Use shared utility to parse ResponseFormat
		result := types.ParseResponseFormatSchema(options.ResponseFormat)
		if result.IsSchema {
			// For structured outputs, we use a tool-calling approach with a special tool
			// The model will be forced to call this tool with the structured output
			structuredTool := AnthropicTool{
				Name:        "structured_output",
				Description: "Generate a structured output matching the required schema",
				InputSchema: result.Schema,
			}

			// Add the structured output tool (replacing any existing tools for now)
			// In a more sophisticated implementation, we could merge with existing tools
			request.Tools = []AnthropicTool{structuredTool}

			// Force the model to use this specific tool
			request.ToolChoice = map[string]interface{}{
				"type": "tool",
				"name": "structured_output",
			}

			log.Printf("🔧 [Anthropic] Added structured output tool with schema")
		} else {
			// If it's not a valid JSON schema, just set it as response_format
			// This allows for simple string modes like "json"
			request.ResponseFormat = result.StringValue
			log.Printf("🔧 [Anthropic] Set response_format to: %s", result.StringValue)
		}
	}

	log.Printf("🔧 [Anthropic] prepareRequest EXIT - returning request")
	return request
}

// makeAPICallWithKey makes the actual HTTP request to the Anthropic API with a specific API key
func (p *AnthropicProvider) makeAPICallWithKey(ctx context.Context, requestData AnthropicRequest, apiKey string) (*AnthropicResponse, *types.Usage, error) {
	// Get base URL
	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/messages"

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Prepare JSON body
	jsonBody, err := p.requestHandler.PrepareJSONBody(requestData)
	if err != nil {
		return nil, nil, err
	}
	req.Body = io.NopCloser(jsonBody)

	// Use auth helper to set headers
	p.authHelper.SetAuthHeaders(req, apiKey, "api_key")
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	p.LogRequest("POST", url, map[string]string{
		"Content-Type":      "application/json",
		"x-api-key":         "***",
		"anthropic-version": "2023-06-01",
	}, requestData)

	// Make the request
	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Parse rate limit headers from response
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, requestData.Model)

	// Check status code using response parser
	if resp.StatusCode != http.StatusOK {
		// Read body for error message
		body, _ := io.ReadAll(resp.Body)
		var errorResponse AnthropicErrorResponse
		if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil {
			return nil, nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, errorResponse.Error.Message)
		}
		return nil, nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse successful response using response parser
	var response AnthropicResponse
	if err := p.responseParser.ParseJSON(resp, &response); err != nil {
		return nil, nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	// Note: Content may be empty for token counting requests with maxTokens=1
	// Still return the response with usage info

	// Convert usage to standard format
	usage := &types.Usage{
		PromptTokens:             response.Usage.InputTokens,
		CompletionTokens:         response.Usage.OutputTokens,
		TotalTokens:              response.Usage.InputTokens + response.Usage.OutputTokens,
		CacheCreationInputTokens: response.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     response.Usage.CacheReadInputTokens,
	}

	return &response, usage, nil
}

// makeAPICallWithOAuth makes API call with OAuth authentication (legacy - returns string)
func (p *AnthropicProvider) makeAPICallWithOAuth(ctx context.Context, requestData AnthropicRequest, accessToken string) (string, *types.Usage, error) {
	message, usage, err := p.makeAPICallWithOAuthMessage(ctx, requestData, accessToken)
	if err != nil {
		return "", nil, err
	}
	return message.Content, usage, nil
}

// makeAPICallWithOAuthMessage makes API call with OAuth authentication (returns ChatMessage)
func (p *AnthropicProvider) makeAPICallWithOAuthMessage(ctx context.Context, requestData AnthropicRequest, accessToken string) (types.ChatMessage, *types.Usage, error) {
	// CRITICAL: Add Claude Code system prompt as FIRST element
	claudeCodePrompt := map[string]string{
		"type": "text",
		"text": "You are Claude Code, Anthropic's official CLI for Claude.",
	}

	// Prepend to existing system prompts
	if systemArray, ok := requestData.System.([]interface{}); ok {
		requestData.System = append([]interface{}{claudeCodePrompt}, systemArray...)
	} else {
		// If not already an array, create one with Claude Code prompt
		requestData.System = []interface{}{claudeCodePrompt}
	}

	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/messages"

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return types.ChatMessage{}, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Prepare JSON body using request handler
	jsonBody, err := p.requestHandler.PrepareJSONBody(requestData)
	if err != nil {
		return types.ChatMessage{}, nil, err
	}
	req.Body = io.NopCloser(jsonBody)

	// Use auth helper to set OAuth headers
	p.authHelper.SetAuthHeaders(req, accessToken, "oauth")
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	p.LogRequest("POST", url, map[string]string{
		"Content-Type":      "application/json",
		"Authorization":     "Bearer ***",
		"anthropic-version": "2023-06-01",
		"anthropic-beta":    "oauth-2025-04-20,claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14",
	}, requestData)

	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return types.ChatMessage{}, nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Parse rate limit headers from response
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, requestData.Model)

	// Check status code and parse response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errorResponse AnthropicErrorResponse
		if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil {
			return types.ChatMessage{}, nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, errorResponse.Error.Message)
		}
		return types.ChatMessage{}, nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse successful response using response parser
	var response AnthropicResponse
	if err := p.responseParser.ParseJSON(resp, &response); err != nil {
		return types.ChatMessage{}, nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	// Convert response to ChatMessage with tool call support
	message := types.ChatMessage{
		Role: response.Role,
	}

	// Extract text content (may be empty for token counting requests with maxTokens=1)
	for _, block := range response.Content {
		if block.Type == "text" {
			message.Content = block.Text
			break
		}
	}

	// Extract tool calls
	message.ToolCalls = convertAnthropicContentToToolCalls(response.Content)

	usage := &types.Usage{
		PromptTokens:             response.Usage.InputTokens,
		CompletionTokens:         response.Usage.OutputTokens,
		TotalTokens:              response.Usage.InputTokens + response.Usage.OutputTokens,
		CacheCreationInputTokens: response.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     response.Usage.CacheReadInputTokens,
	}

	return message, usage, nil
}

// makeAPICallWithKeyMessage makes API call with API key authentication (returns ChatMessage)
func (p *AnthropicProvider) makeAPICallWithKeyMessage(ctx context.Context, requestData AnthropicRequest, apiKey string) (types.ChatMessage, *types.Usage, error) {
	response, usage, err := p.makeAPICallWithKey(ctx, requestData, apiKey)
	if err != nil {
		return types.ChatMessage{}, nil, err
	}

	// Convert response to ChatMessage with tool call support
	message := types.ChatMessage{
		Role: response.Role,
	}

	// Extract text content
	for _, block := range response.Content {
		if block.Type == "text" {
			message.Content = block.Text
			break
		}
	}

	// Extract tool calls
	message.ToolCalls = convertAnthropicContentToToolCalls(response.Content)

	return message, usage, nil
}
