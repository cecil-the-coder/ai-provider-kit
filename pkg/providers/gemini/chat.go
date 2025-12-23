// Package gemini provides chat completion logic for Google Gemini AI provider.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/auth"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/google/uuid"
)

// GenerateChatCompletion generates a chat completion with OAuth/API key support
func (p *GeminiProvider) GenerateChatCompletion(
	ctx context.Context,
	options types.GenerateOptions,
) (types.ChatCompletionStream, error) {
	p.IncrementRequestCount()
	startTime := time.Now()

	// Check if streaming is requested
	if options.Stream {
		// Determine model for streaming with fallback priority
		model := common.ResolveModel(options.Model, p.config.Model, geminiDefaultModel)

		var stream types.ChatCompletionStream
		var err error

		// Check for context-injected OAuth token first
		if contextToken := auth.GetOAuthToken(ctx); contextToken != "" {
			stream, err = p.makeStreamingAPICallWithToken(ctx, options, model, contextToken)
		} else {
			// Use the unified executeStreamWithAuth method
			stream, err = p.executeStreamWithAuth(ctx, options)
		}

		if err != nil {
			p.RecordError(err)
			return nil, err
		}
		latency := time.Since(startTime)
		p.RecordSuccess(latency, 0) // Tokens will be counted as stream is consumed
		return stream, nil
	}

	// Non-streaming path - use ExecuteWithAuthMessage
	var responseMessage types.ChatMessage
	var usage *types.Usage
	var err error

	// Define OAuth operation (returns ChatMessage)
	oauthOperation := func(ctx context.Context, cred *types.OAuthCredentialSet) (types.ChatMessage, *types.Usage, error) {
		return p.makeAPICallWithTokenMessage(ctx, options, "", cred.AccessToken)
	}

	// Define API key operation (returns ChatMessage)
	apiKeyOperation := func(ctx context.Context, apiKey string) (types.ChatMessage, *types.Usage, error) {
		return p.makeAPICallWithAPIKeyMessage(ctx, options, "", apiKey)
	}

	// Use shared auth helper to execute with automatic failover (preserves tool calls)
	responseMessage, usage, err = p.authHelper.ExecuteWithAuthMessage(ctx, options, oauthOperation, apiKeyOperation)

	if err != nil {
		p.RecordError(err)
		return nil, err
	}

	// Record success metrics
	latency := time.Since(startTime)
	var tokensUsed int64
	if usage != nil {
		tokensUsed = int64(usage.TotalTokens)
	}
	p.RecordSuccess(latency, tokensUsed)

	// Return stream with full message support (including tool calls)
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

	return &MockStream{
		chunks: []types.ChatCompletionChunk{chunk},
	}, nil
}

// makeAPICallWithTokenMessage makes an API call with OAuth token (returns ChatMessage)
func (p *GeminiProvider) makeAPICallWithTokenMessage(ctx context.Context, options types.GenerateOptions, model string, accessToken string) (types.ChatMessage, *types.Usage, error) {
	// Apply rate limiting
	if err := p.applyRateLimiting(ctx); err != nil {
		return types.ChatMessage{}, nil, err
	}

	// Determine model to use
	model = p.resolveModel(model, options)

	// Use standard Gemini API with OAuth
	responseBody, err := p.makeStandardAPICallWithOAuth(ctx, model, accessToken, options)
	if err != nil {
		return types.ChatMessage{}, nil, err
	}

	// Parse standard API response to ChatMessage
	return p.parseStandardGeminiResponseMessage(responseBody, model)
}

// makeAPICallWithAPIKeyMessage makes an API call with API key (returns ChatMessage)
func (p *GeminiProvider) makeAPICallWithAPIKeyMessage(ctx context.Context, options types.GenerateOptions, model string, apiKey string) (types.ChatMessage, *types.Usage, error) {
	// Apply rate limiting
	if err := p.applyRateLimiting(ctx); err != nil {
		return types.ChatMessage{}, nil, err
	}

	// Determine model to use
	model = p.resolveModel(model, options)

	// Prepare request for standard Gemini API
	requestBody := p.prepareStandardRequest(options)

	// Execute API call
	responseBody, err := p.executeStandardAPIRequest(ctx, model, apiKey, requestBody)
	if err != nil {
		return types.ChatMessage{}, nil, err
	}

	// Parse standard Gemini API response to ChatMessage
	return p.parseStandardGeminiResponseMessage(responseBody, model)
}

// applyRateLimiting applies client-side rate limiting
func (p *GeminiProvider) applyRateLimiting(ctx context.Context) error {
	waitCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	p.rateLimitMutex.RLock()
	limiter := p.clientSideLimiter
	p.rateLimitMutex.RUnlock()

	return limiter.Wait(waitCtx)
}

// resolveModel determines which model to use based on precedence
func (p *GeminiProvider) resolveModel(_ string, options types.GenerateOptions) string {
	return common.ResolveModel(options.Model, p.config.Model, geminiDefaultModel)
}

// prepareStandardRequest prepares request body for standard Gemini API
func (p *GeminiProvider) prepareStandardRequest(options types.GenerateOptions) GenerateContentRequest {
	var contents []Content

	// Handle messages if provided
	if len(options.Messages) > 0 {
		contents = make([]Content, len(options.Messages))
		for i, msg := range options.Messages {
			// Use GetContentParts() helper for unified access
			contentParts := msg.GetContentParts()
			var parts []Part

			if len(contentParts) > 0 {
				// Convert multimodal content parts
				parts = convertContentPartsToGeminiParts(contentParts)
			} else {
				// Fallback to string content (should not happen with GetContentParts)
				parts = []Part{{Text: msg.Content}}
			}

			contents[i] = Content{
				Role:  msg.Role,
				Parts: parts,
			}
		}
	} else if options.Prompt != "" {
		// Convert prompt to user message
		contents = append(contents, Content{
			Role: "user",
			Parts: []Part{
				{Text: options.Prompt},
			},
		})
	}

	generationConfig := &GenerationConfig{
		Temperature:     0.7,
		TopP:            0.95,
		TopK:            40,
		MaxOutputTokens: 8192,
	}

	// Handle structured outputs via ResponseFormat
	// Gemini supports JSON schema validation with response_schema + response_mime_type="application/json"
	if options.ResponseFormat != "" {
		// Use shared utility to parse ResponseFormat
		result := types.ParseResponseFormatSchema(options.ResponseFormat)
		if result.IsSchema {
			// It's a valid JSON schema object
			generationConfig.ResponseSchema = result.Schema
			generationConfig.ResponseMimeType = "application/json"
		} else {
			// It's a string like "json", just set the mime type
			generationConfig.ResponseMimeType = "application/json"
		}
	}

	requestBody := GenerateContentRequest{
		Contents:         contents,
		GenerationConfig: generationConfig,
	}

	// Add tools if provided
	if len(options.Tools) > 0 {
		requestBody.Tools = convertToGeminiTools(options.Tools)
	}

	return requestBody
}

// wrapForCodeAssist wraps a GenerateContentRequest in the Code Assist API format
func (p *GeminiProvider) wrapForCodeAssist(chatRequest GenerateContentRequest, model string, projectID string) CodeAssistRequest {
	// Generate UUID for user_prompt_id
	userPromptID := uuid.New().String()

	// Convert the request to a map for the wrapped format
	requestMap := make(map[string]interface{})

	// Add contents
	if len(chatRequest.Contents) > 0 {
		requestMap["contents"] = chatRequest.Contents
	}

	// Add generation config if present
	if chatRequest.GenerationConfig != nil {
		requestMap["generationConfig"] = chatRequest.GenerationConfig
	}

	// Add tools if present
	if len(chatRequest.Tools) > 0 {
		requestMap["tools"] = chatRequest.Tools
	}

	// Add safety settings if present
	if len(chatRequest.SafetySettings) > 0 {
		requestMap["safetySettings"] = chatRequest.SafetySettings
	}

	return CodeAssistRequest{
		Model:        model,
		Project:      projectID,
		UserPromptID: userPromptID,
		Request:      requestMap,
	}
}

// executeStandardAPIRequest executes a standard Gemini API request
func (p *GeminiProvider) executeStandardAPIRequest(ctx context.Context, model string, apiKey string, requestBody GenerateContentRequest) ([]byte, error) {
	// Check if using Code Assist backend - wrap request if needed
	var requestToMarshal interface{}
	if p.backendRouter.GetBackend() == BackendCodeAssist {
		// Get project ID for Code Assist API
		projectID := p.getProjectID()
		if projectID == "" {
			return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "project ID is required for Code Assist API").
				WithOperation("chat_completion")
		}
		// Wrap the request in Code Assist format
		wrappedRequest := p.wrapForCodeAssist(requestBody, model, projectID)
		requestToMarshal = wrappedRequest
	} else {
		// Use backend router to convert request if needed
		convertedRequest, err := p.backendRouter.GetConverter().ConvertRequest(requestBody)
		if err != nil {
			return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to convert request").
				WithOperation("chat_completion").
				WithOriginalErr(err)
		}
		requestToMarshal = convertedRequest
	}

	// Serialize request
	jsonBody, err := json.Marshal(requestToMarshal)
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to marshal request").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	// Use backend router to build the URL
	url := p.backendRouter.BuildRequestURL(model, "generateContent", apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to create request").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Add X-Goog-Api-Key header if auth_method is "header"
	if p.backendRouter.ShouldUseHeaderAuth() && apiKey != "" {
		req.Header.Set("X-Goog-Api-Key", apiKey)
	}

	p.LogRequest("POST", url, map[string]string{
		"Content-Type": "application/json",
	}, requestBody)

	// Make the request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "request failed").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to read response body").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	// Handle rate limiting
	if resp.StatusCode == 429 {
		if info, err := p.rateLimitHelper.GetParser().Parse(resp.Header, model); err == nil && info.RetryAfter > 0 {
			// Update tracker with retry info
			p.rateLimitHelper.UpdateRateLimitInfo(info)
			return nil, types.NewRateLimitError(types.ProviderTypeGemini, int(info.RetryAfter.Seconds())).
				WithOperation("chat_completion")
		}
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeGemini, errCode, string(responseBody)).
			WithOperation("chat_completion").
			WithStatusCode(resp.StatusCode)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeGemini, errCode, string(responseBody)).
			WithOperation("chat_completion").
			WithStatusCode(resp.StatusCode)
	}

	return responseBody, nil
}

// makeStandardAPICallWithOAuth executes a non-streaming standard Gemini API request with OAuth
func (p *GeminiProvider) makeStandardAPICallWithOAuth(ctx context.Context, model string, accessToken string, options types.GenerateOptions) ([]byte, error) {
	// Prepare standard request
	requestBody := p.prepareStandardRequest(options)

	// Check if using Code Assist backend - wrap request if needed
	var requestToMarshal interface{}
	if p.backendRouter.GetBackend() == BackendCodeAssist {
		// Get project ID for Code Assist API
		projectID := p.getProjectID()
		if projectID == "" {
			return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "project ID is required for Code Assist API").
				WithOperation("chat_completion")
		}
		// Wrap the request in Code Assist format
		wrappedRequest := p.wrapForCodeAssist(requestBody, model, projectID)
		requestToMarshal = wrappedRequest
	} else {
		// Use backend router to convert request if needed
		convertedRequest, err := p.backendRouter.GetConverter().ConvertRequest(requestBody)
		if err != nil {
			return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to convert request").
				WithOperation("chat_completion").
				WithOriginalErr(err)
		}
		requestToMarshal = convertedRequest
	}

	// Serialize request
	jsonBody, err := json.Marshal(requestToMarshal)
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to marshal request").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	// Use backend router to build the URL (no API key for OAuth)
	url := p.backendRouter.BuildRequestURL(model, "generateContent", "")
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to create request").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	// Set headers with OAuth bearer token
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	p.LogRequest("POST", url, map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "***",
	}, requestBody)

	// Make the request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "request failed").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to read response body").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	// Handle rate limiting
	if resp.StatusCode == 429 {
		if info, err := p.rateLimitHelper.GetParser().Parse(resp.Header, model); err == nil && info.RetryAfter > 0 {
			// Update tracker with retry info
			p.rateLimitHelper.UpdateRateLimitInfo(info)
			return nil, types.NewRateLimitError(types.ProviderTypeGemini, int(info.RetryAfter.Seconds())).
				WithOperation("chat_completion")
		}
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeGemini, errCode, string(responseBody)).
			WithOperation("chat_completion").
			WithStatusCode(resp.StatusCode)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeGemini, errCode, string(responseBody)).
			WithOperation("chat_completion").
			WithStatusCode(resp.StatusCode)
	}

	return responseBody, nil
}

// parseStandardGeminiResponse parses a standard Gemini API response
func (p *GeminiProvider) parseStandardGeminiResponse(responseBody []byte, _ string) (string, *types.Usage, error) {
	// Use backend router to convert response if needed
	apiResp, err := p.backendRouter.GetConverter().ConvertResponse(responseBody)
	if err != nil {
		return "", nil, err
	}

	// Extract content
	if len(apiResp.Candidates) == 0 {
		return "", nil, types.NewServerError(types.ProviderTypeGemini, 0, "no candidates in Gemini response").
			WithOperation("convert_response")
	}

	candidate := apiResp.Candidates[0]
	// Check for error finish reasons
	if IsErrorFinishReason(candidate.FinishReason) {
		return "", nil, types.NewServerError(types.ProviderTypeGemini, 0,
			fmt.Sprintf("generation failed: %s", GetFinishReasonMessage(candidate.FinishReason))).
			WithOperation("convert_response")
	}

	if len(candidate.Content.Parts) == 0 {
		return "", nil, types.NewServerError(types.ProviderTypeGemini, 0, "no parts in candidate content").
			WithOperation("convert_response")
	}

	var fullText strings.Builder
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			fullText.WriteString(part.Text)
		}
	}

	result := fullText.String()
	if result == "" {
		return "", nil, types.NewServerError(types.ProviderTypeGemini, 0, "empty response from Gemini API").
			WithOperation("convert_response")
	}

	// Extract usage information
	var usage *types.Usage
	if apiResp.UsageMetadata != nil {
		usage = &types.Usage{
			PromptTokens:     apiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: apiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      apiResp.UsageMetadata.TotalTokenCount,
		}
	}

	return result, usage, nil
}

// parseStandardGeminiResponseMessage parses standard Gemini response and returns ChatMessage
func (p *GeminiProvider) parseStandardGeminiResponseMessage(responseBody []byte, _ string) (types.ChatMessage, *types.Usage, error) {
	// Use backend router to convert response if needed
	apiResp, err := p.backendRouter.GetConverter().ConvertResponse(responseBody)
	if err != nil {
		return types.ChatMessage{}, nil, err
	}

	// Extract content
	if len(apiResp.Candidates) == 0 {
		return types.ChatMessage{}, nil, types.NewServerError(types.ProviderTypeGemini, 0, "no candidates in Gemini response").
			WithOperation("convert_response")
	}

	candidate := apiResp.Candidates[0]
	// Check for error finish reasons
	if IsErrorFinishReason(candidate.FinishReason) {
		return types.ChatMessage{}, nil, types.NewServerError(types.ProviderTypeGemini, 0,
			fmt.Sprintf("generation failed: %s", GetFinishReasonMessage(candidate.FinishReason))).
			WithOperation("convert_response")
	}

	if len(candidate.Content.Parts) == 0 {
		return types.ChatMessage{}, nil, types.NewServerError(types.ProviderTypeGemini, 0, "no parts in candidate content").
			WithOperation("convert_response")
	}

	// Extract text content and tool calls
	var fullText strings.Builder
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			fullText.WriteString(part.Text)
		}
	}

	message := types.ChatMessage{
		Role:      candidate.Content.Role,
		Content:   fullText.String(),
		ToolCalls: convertGeminiFunctionCallsToUniversal(candidate.Content.Parts),
	}

	// Extract usage information
	var usage *types.Usage
	if apiResp.UsageMetadata != nil {
		usage = &types.Usage{
			PromptTokens:     apiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: apiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      apiResp.UsageMetadata.TotalTokenCount,
		}
	}

	return message, usage, nil
}
