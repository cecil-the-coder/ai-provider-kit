package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

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

	// Handle KeepAlive from metadata if provided
	// Supports various formats:
	// - "5m" (keep alive for 5 minutes)
	// - "300s" (keep alive for 300 seconds)
	// - "-1" (keep model loaded forever)
	// - "0" (unload immediately after request)
	if options.Metadata != nil {
		if keepAliveValue, ok := options.Metadata["keep_alive"]; ok {
			duration := parseKeepAlive(keepAliveValue)
			if duration != nil {
				request.KeepAlive = duration
			}
		}
	}

	return request
}

// parseKeepAlive converts various keep_alive formats to Duration
func parseKeepAlive(value interface{}) *Duration {
	switch v := value.(type) {
	case string:
		// Handle string formats: "5m", "300s", "-1", "0"
		if v == "-1" {
			return &Duration{Duration: -1}
		}
		if v == "0" {
			return &Duration{Duration: 0}
		}
		if dur, err := time.ParseDuration(v); err == nil {
			return &Duration{Duration: dur}
		}
	case int:
		// Handle integer (seconds)
		if v == -1 {
			return &Duration{Duration: -1}
		}
		return &Duration{Duration: time.Duration(v) * time.Second}
	case int64:
		// Handle int64 (seconds)
		if v == -1 {
			return &Duration{Duration: -1}
		}
		return &Duration{Duration: time.Duration(v) * time.Second}
	case float64:
		// Handle float64 (seconds)
		if v == -1 {
			return &Duration{Duration: -1}
		}
		return &Duration{Duration: time.Duration(v * float64(time.Second))}
	case time.Duration:
		// Handle native time.Duration
		return &Duration{Duration: v}
	case Duration:
		// Handle our Duration type
		return &v
	case *Duration:
		// Handle pointer to our Duration type
		return v
	}
	return nil
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
		// Normalize the input schema to ensure compatibility with Ollama
		// This converts array-typed "type" fields (e.g., ["string", "null"]) to strings
		normalizedSchema := NormalizeJSONSchema(tool.InputSchema)

		ollamaTools = append(ollamaTools, ollamaTool{
			Type: "function",
			Function: ollamaFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  normalizedSchema,
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
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

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
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

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

// GenerateBatchEmbeddings generates embeddings for multiple texts using the new /api/embed endpoint
// This method supports batching and is more efficient than calling GenerateEmbeddings multiple times.
// It will automatically fall back to the legacy /api/embeddings endpoint if configured to do so or if the new endpoint fails.
func (p *OllamaProvider) GenerateBatchEmbeddings(ctx context.Context, model string, texts []string) ([][]float64, error) {
	// Initialize request tracking
	p.IncrementRequestCount()
	startTime := time.Now()

	// Validate authentication for cloud endpoints
	if p.isCloudEndpoint() && !p.authHelper.IsAuthenticated() {
		p.RecordError(types.NewAuthError(types.ProviderTypeOllama, "no API key configured for cloud endpoint"))
		return nil, types.NewAuthError(types.ProviderTypeOllama, "no API key configured for cloud endpoint").
			WithOperation("generate_batch_embeddings")
	}

	// Use default model if not specified
	if model == "" {
		model = "nomic-embed-text"
	}

	// Validate input
	if len(texts) == 0 {
		return [][]float64{}, nil
	}

	// Determine which endpoint to use based on configuration
	var embeddings [][]float64
	var err error

	switch p.embeddingsEndpoint {
	case EmbeddingsEndpointEmbed:
		// Use new /api/embed endpoint
		embeddings, err = p.generateEmbeddingsWithEmbedEndpoint(ctx, model, texts)
	case EmbeddingsEndpointLegacy:
		// Use legacy /api/embeddings endpoint
		embeddings, err = p.generateEmbeddingsWithLegacyEndpoint(ctx, model, texts)
	case EmbeddingsEndpointAuto:
		// Try new endpoint first, fall back to legacy if it fails
		if !p.embedEndpointFailed {
			embeddings, err = p.generateEmbeddingsWithEmbedEndpoint(ctx, model, texts)
			if err != nil {
				// Check if it's a 404 or method not allowed error (endpoint doesn't exist)
				if providerErr, ok := err.(*types.ProviderError); ok {
					if providerErr.StatusCode == http.StatusNotFound || providerErr.StatusCode == http.StatusMethodNotAllowed {
						// Mark the endpoint as failed and try legacy
						p.embedEndpointFailed = true
						embeddings, err = p.generateEmbeddingsWithLegacyEndpoint(ctx, model, texts)
					}
				}
			}
		} else {
			// Already know the new endpoint doesn't work, use legacy
			embeddings, err = p.generateEmbeddingsWithLegacyEndpoint(ctx, model, texts)
		}
	default:
		// Fallback to auto mode
		embeddings, err = p.generateEmbeddingsWithEmbedEndpoint(ctx, model, texts)
		if err != nil {
			if providerErr, ok := err.(*types.ProviderError); ok {
				if providerErr.StatusCode == http.StatusNotFound || providerErr.StatusCode == http.StatusMethodNotAllowed {
					embeddings, err = p.generateEmbeddingsWithLegacyEndpoint(ctx, model, texts)
				}
			}
		}
	}

	if err != nil {
		p.RecordError(err)
		return nil, err
	}

	// Record success
	latency := time.Since(startTime)
	p.RecordSuccess(latency, 0)

	return embeddings, nil
}

// generateEmbeddingsWithEmbedEndpoint uses the new /api/embed endpoint (supports batching)
func (p *OllamaProvider) generateEmbeddingsWithEmbedEndpoint(ctx context.Context, model string, texts []string) ([][]float64, error) {
	// Determine the base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := fmt.Sprintf("%s/api/embed", strings.TrimSuffix(baseURL, "/"))

	// Build the request - use array for batch or string for single
	var input interface{}
	if len(texts) == 1 {
		input = texts[0]
	} else {
		input = texts
	}

	request := ollamaEmbedRequest{
		Model: model,
		Input: input,
	}

	// Marshal request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to marshal request").
			WithOperation("generate_embeddings_embed").
			WithOriginalErr(err)
	}

	// Log the request
	p.LogRequest("POST", url, map[string]string{
		"Content-Type": "application/json",
	}, request)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "failed to create request").
			WithOperation("generate_embeddings_embed").
			WithOriginalErr(err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Add authentication header if using cloud endpoint
	if p.isCloudEndpoint() && p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0 {
		apiKey := p.authHelper.KeyManager.GetKeys()[0]
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	// Make the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "request failed").
			WithOperation("generate_embeddings_embed").
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
			return nil, types.NewAuthError(types.ProviderTypeOllama, "invalid API key").
				WithOperation("generate_embeddings_embed").
				WithStatusCode(resp.StatusCode)
		case http.StatusNotFound:
			return nil, types.NewNotFoundError(types.ProviderTypeOllama, "endpoint or model not found").
				WithOperation("generate_embeddings_embed").
				WithStatusCode(resp.StatusCode)
		case http.StatusMethodNotAllowed:
			return nil, types.NewProviderError(types.ProviderTypeOllama, types.ErrCodeInvalidRequest, "method not allowed").
				WithOperation("generate_embeddings_embed").
				WithStatusCode(resp.StatusCode)
		case http.StatusTooManyRequests:
			return nil, types.NewRateLimitError(types.ProviderTypeOllama, 0).
				WithOperation("generate_embeddings_embed").
				WithStatusCode(resp.StatusCode)
		default:
			if resp.StatusCode >= 500 {
				return nil, types.NewServerError(types.ProviderTypeOllama, resp.StatusCode, string(body)).
					WithOperation("generate_embeddings_embed")
			}
			return nil, types.NewProviderError(types.ProviderTypeOllama, types.ErrCodeInvalidRequest, string(body)).
				WithOperation("generate_embeddings_embed").
				WithStatusCode(resp.StatusCode)
		}
	}

	// Parse response
	var embedResp ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to parse embed response").
			WithOperation("generate_embeddings_embed").
			WithOriginalErr(err)
	}

	// Convert [][]float32 to [][]float64
	result := make([][]float64, len(embedResp.Embeddings))
	for i, embedding := range embedResp.Embeddings {
		result[i] = make([]float64, len(embedding))
		for j, val := range embedding {
			result[i][j] = float64(val)
		}
	}

	return result, nil
}

// generateEmbeddingsWithLegacyEndpoint uses the legacy /api/embeddings endpoint (single text only, batched sequentially)
func (p *OllamaProvider) generateEmbeddingsWithLegacyEndpoint(ctx context.Context, model string, texts []string) ([][]float64, error) {
	result := make([][]float64, len(texts))

	// Process each text sequentially
	for i, text := range texts {
		embedding, err := p.generateSingleEmbeddingLegacy(ctx, model, text)
		if err != nil {
			return nil, err
		}
		result[i] = embedding
	}

	return result, nil
}

// generateSingleEmbeddingLegacy generates a single embedding using the legacy /api/embeddings endpoint
func (p *OllamaProvider) generateSingleEmbeddingLegacy(ctx context.Context, model string, text string) ([]float64, error) {
	// Determine the base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := fmt.Sprintf("%s/api/embeddings", strings.TrimSuffix(baseURL, "/"))

	// Build the request
	request := ollamaEmbeddingsRequest{
		Model:  model,
		Prompt: text,
	}

	// Marshal request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to marshal request").
			WithOperation("generate_embeddings_legacy").
			WithOriginalErr(err)
	}

	// Log the request
	p.LogRequest("POST", url, map[string]string{
		"Content-Type": "application/json",
	}, request)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "failed to create request").
			WithOperation("generate_embeddings_legacy").
			WithOriginalErr(err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Add authentication header if using cloud endpoint
	if p.isCloudEndpoint() && p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0 {
		apiKey := p.authHelper.KeyManager.GetKeys()[0]
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	// Make the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "request failed").
			WithOperation("generate_embeddings_legacy").
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
			return nil, types.NewAuthError(types.ProviderTypeOllama, "invalid API key").
				WithOperation("generate_embeddings_legacy").
				WithStatusCode(resp.StatusCode)
		case http.StatusNotFound:
			return nil, types.NewNotFoundError(types.ProviderTypeOllama, "model not found").
				WithOperation("generate_embeddings_legacy").
				WithStatusCode(resp.StatusCode)
		case http.StatusTooManyRequests:
			return nil, types.NewRateLimitError(types.ProviderTypeOllama, 0).
				WithOperation("generate_embeddings_legacy").
				WithStatusCode(resp.StatusCode)
		default:
			if resp.StatusCode >= 500 {
				return nil, types.NewServerError(types.ProviderTypeOllama, resp.StatusCode, string(body)).
					WithOperation("generate_embeddings_legacy")
			}
			return nil, types.NewProviderError(types.ProviderTypeOllama, types.ErrCodeInvalidRequest, string(body)).
				WithOperation("generate_embeddings_legacy").
				WithStatusCode(resp.StatusCode)
		}
	}

	// Parse response
	var embeddingsResp ollamaEmbeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingsResp); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to parse embeddings response").
			WithOperation("generate_embeddings_legacy").
			WithOriginalErr(err)
	}

	return embeddingsResp.Embedding, nil
}
