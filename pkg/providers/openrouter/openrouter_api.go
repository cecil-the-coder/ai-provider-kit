// Package openrouter provides an OpenRouter AI provider implementation.
package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// prepareRequest prepares the API request payload
func (p *OpenRouterProvider) prepareRequest(options types.GenerateOptions) (OpenRouterRequest, error) {
	// Determine which model to use: options.Model takes precedence over modelSelector
	var modelName string
	if options.Model != "" {
		modelName = options.Model
	} else {
		var err error
		modelName, err = p.modelSelector.SelectModel()
		if err != nil {
			return OpenRouterRequest{}, fmt.Errorf("failed to select model: %w", err)
		}
	}

	if p.freeOnly && !strings.HasSuffix(modelName, ":free") {
		modelName += ":free"
	}

	p.mutex.Lock()
	p.lastUsedModel = modelName
	p.mutex.Unlock()

	// Convert to OpenRouter messages
	var messages []OpenRouterMessage

	if len(options.Messages) > 0 {
		// Use provided messages directly
		messages = convertMessagesToOpenRouter(options.Messages)
	} else if options.Prompt != "" {
		// Convert prompt to user message
		messages = append(messages, OpenRouterMessage{
			Role:    "user",
			Content: options.Prompt,
		})
	}

	requestData := OpenRouterRequest{
		Model:         modelName,
		Messages:      messages,
		Stream:        options.Stream,
		HTTPReferer:   p.siteURL,
		HTTPUserAgent: p.siteName,
	}

	if options.Temperature > 0 {
		requestData.Temperature = options.Temperature
	}
	if options.MaxTokens > 0 {
		requestData.MaxTokens = options.MaxTokens
	}

	// Convert tools if provided
	if len(options.Tools) > 0 {
		requestData.Tools = convertToOpenRouterTools(options.Tools)
		// ToolChoice defaults to "auto" when tools are provided (OpenAI's default behavior)
	}

	// Handle structured outputs via ResponseFormat
	// OpenRouter uses OpenAI-compatible JSON mode (varies by underlying model)
	if options.ResponseFormat != "" {
		// Use shared utility to parse ResponseFormat
		result := types.ParseResponseFormatSchema(options.ResponseFormat)
		// For OpenAI-compatible providers like OpenRouter, wrap in {"type":"json_object"}
		// Note: Actual support depends on the underlying model selected
		requestData.ResponseFormat = map[string]interface{}{
			"type": "json_object",
		}
		if !result.IsSchema {
			// It's a string like "json_object", use it directly
			requestData.ResponseFormat = map[string]interface{}{
				"type": result.StringValue,
			}
		}
	}

	return requestData, nil
}

// makeAPICallWithKey makes the actual HTTP request to the OpenRouter API
func (p *OpenRouterProvider) makeAPICallWithKey(ctx context.Context, requestData OpenRouterRequest, apiKey string) (*OpenRouterResponse, error) {
	jsonBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(jsonBody)))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", p.siteURL)
	req.Header.Set("X-Title", p.siteName)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	// Track rate limit headers from response (single call replaces ParseAndUpdateRateLimits)
	p.rateLimitTracker.TrackResponse(resp.Header, requestData.Model)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResponse OpenRouterErrorResponse
		if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil {
			return nil, fmt.Errorf("OpenRouter API error: %d - %s", resp.StatusCode, errorResponse.Error.Message)
		}
		return nil, fmt.Errorf("OpenRouter API error: %d - %s", resp.StatusCode, string(body))
	}

	var response OpenRouterResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no choices in API response")
	}

	return &response, nil
}

// makeStreamingAPICallWithKey makes a streaming API call with a specific API key
func (p *OpenRouterProvider) makeStreamingAPICallWithKey(ctx context.Context, requestData OpenRouterRequest, apiKey string) (types.ChatCompletionStream, error) {
	jsonBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(jsonBody)))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", p.siteURL)
	req.Header.Set("X-Title", p.siteName)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Track rate limit headers from streaming response (single call replaces ParseAndUpdateRateLimits)
	p.rateLimitTracker.TrackResponse(resp.Header, requestData.Model)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors
		return nil, fmt.Errorf("OpenRouter API error: %d - %s", resp.StatusCode, string(body))
	}

	return &OpenRouterStream{
		response: resp,
		reader:   bufio.NewReader(resp.Body),
		done:     false,
	}, nil
}
