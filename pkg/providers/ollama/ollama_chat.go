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
