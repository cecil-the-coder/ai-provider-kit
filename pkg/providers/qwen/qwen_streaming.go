// Package qwen provides integration with Qwen (Alibaba Cloud) AI models
// supporting both API key and OAuth authentication, streaming, and tool calling.
package qwen

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/streaming"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Note: QwenStreamWithMessage is already defined in types.go

// QwenRealStream implements ChatCompletionStream for real streaming responses
type QwenRealStream struct {
	response *http.Response
	reader   *bufio.Reader
	done     bool
	mutex    sync.Mutex
}

func (s *QwenRealStream) Next() (types.ChatCompletionChunk, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.done {
		return types.ChatCompletionChunk{Done: true}, io.EOF
	}

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				s.done = true
				return types.ChatCompletionChunk{Done: true}, io.EOF
			}
			return types.ChatCompletionChunk{}, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue // Skip empty lines
		}

		if !strings.HasPrefix(line, "data: ") {
			continue // Skip non-data lines
		}

		data := strings.TrimPrefix(line, "data: ")

		// Check for stream end
		if data == "[DONE]" {
			s.done = true
			return types.ChatCompletionChunk{Done: true}, io.EOF
		}

		var streamResp QwenResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue // Skip malformed chunks
		}

		if len(streamResp.Choices) > 0 {
			choice := streamResp.Choices[0]

			// Handle content - it can be a string or array of content parts
			var content string
			if contentStr, ok := choice.Delta.Content.(string); ok {
				content = contentStr
			} else if choice.Delta.Content != nil {
				content = fmt.Sprintf("%v", choice.Delta.Content)
			}

			chunk := types.ChatCompletionChunk{
				Content: content,
				Done:    choice.FinishReason != "",
			}

			// Add usage if present
			if streamResp.Usage.TotalTokens > 0 {
				chunk.Usage = types.Usage{
					PromptTokens:     streamResp.Usage.PromptTokens,
					CompletionTokens: streamResp.Usage.CompletionTokens,
					TotalTokens:      streamResp.Usage.TotalTokens,
				}
			}

			if chunk.Done {
				s.done = true
				return chunk, io.EOF
			}

			return chunk, nil
		}
	}
}

func (s *QwenRealStream) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.done = true
	if s.response != nil {
		return s.response.Body.Close()
	}
	return nil
}

// executeStreamWithAuth handles streaming requests with authentication
func (p *QwenProvider) executeStreamWithAuth(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	providerConfig := p.GetConfig()

	baseURL := providerConfig.BaseURL
	if baseURL == "" {
		baseURL = "https://portal.qwen.ai/v1"
	}

	// Build request using the new helper
	request := p.buildQwenRequest(options)
	request.Stream = true

	// Check for context-injected OAuth token first
	if contextToken := auth.GetOAuthToken(ctx); contextToken != "" {
		log.Printf("🟢 [Qwen] Using context-injected OAuth token for streaming")
		return p.makeStreamingAPICall(ctx, baseURL+"/chat/completions", request, contextToken)
	}

	// Try OAuth credentials first with token refresh support
	if p.authHelper.OAuthManager != nil {
		stream, err := p.authHelper.OAuthManager.ExecuteWithFailoverStream(ctx,
			func(ctx context.Context, cred *types.OAuthCredentialSet) (types.ChatCompletionStream, error) {
				return p.makeStreamingAPICall(ctx, baseURL+"/chat/completions", request, cred.AccessToken)
			},
		)
		if err != nil {
			return nil, types.NewAuthError(types.ProviderTypeQwen, err.Error()).
				WithOperation("executeStreamWithAuth").
				WithOriginalErr(err)
		}
		return stream, nil
	}

	// Try API keys
	if p.authHelper.KeyManager != nil {
		keys := p.authHelper.KeyManager.GetKeys()
		var lastErr error
		for _, apiKey := range keys {
			stream, err := p.makeStreamingAPICall(ctx, baseURL+"/chat/completions", request, apiKey)
			if err == nil {
				return stream, nil
			}
			lastErr = err
		}
		// Fall back to configured API key if available
		if providerConfig.APIKey != "" {
			stream, err := p.makeStreamingAPICall(ctx, baseURL+"/chat/completions", request, providerConfig.APIKey)
			if err == nil {
				return stream, nil
			}
			lastErr = err
		}
		return nil, types.NewAuthError(types.ProviderTypeQwen, "no valid API key available for streaming").
			WithOperation("executeStreamWithAuth").
			WithOriginalErr(lastErr)
	}

	return nil, types.NewAuthError(types.ProviderTypeQwen, "no authentication method configured for streaming").
		WithOperation("executeStreamWithAuth")
}

// makeStreamingAPICall makes a streaming API call to Qwen
func (p *QwenProvider) makeStreamingAPICall(ctx context.Context, url string, request QwenRequest, authToken string) (types.ChatCompletionStream, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))

	// Log the request
	p.LogRequest("POST", url, map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer ***",
	}, request)

	startTime := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	duration := time.Since(startTime)
	p.LogResponse(resp, duration)

	// Parse rate limit headers from streaming response (flexible multi-format parser)
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, request.Model)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors
		return nil, fmt.Errorf("qwen API error: %d - %s", resp.StatusCode, string(body))
	}

	return &QwenRealStream{
		response: resp,
		reader:   bufio.NewReader(resp.Body),
		done:     false,
	}, nil
}

// convertToQwenTools converts universal tools to Qwen format
// Qwen uses OpenAI-compatible format, so we use the shared implementation
func convertToQwenTools(tools []types.Tool) []QwenTool {
	// Convert using shared OpenAI-compatible converter
	compatibleTools := streaming.ConvertToOpenAICompatibleTools(tools)

	// Convert to Qwen-specific types (same structure, different type names)
	qwenTools := make([]QwenTool, len(compatibleTools))
	for i, ct := range compatibleTools {
		qwenTools[i] = QwenTool{
			Type: ct.Type,
			Function: QwenFunctionDef{
				Name:        ct.Function.Name,
				Description: ct.Function.Description,
				Parameters:  ct.Function.Parameters,
			},
		}
	}
	return qwenTools
}

// convertToQwenToolCalls converts universal tool calls to Qwen format
// Qwen uses OpenAI-compatible format, so we use the shared implementation
func convertToQwenToolCalls(toolCalls []types.ToolCall) []QwenToolCall {
	// Convert using shared OpenAI-compatible converter
	compatibleCalls := streaming.ConvertToOpenAICompatibleToolCalls(toolCalls)

	// Convert to Qwen-specific types (same structure, different type names)
	qwenToolCalls := make([]QwenToolCall, len(compatibleCalls))
	for i, cc := range compatibleCalls {
		qwenToolCalls[i] = QwenToolCall{
			ID:   cc.ID,
			Type: cc.Type,
			Function: QwenToolCallFunction{
				Name:      cc.Function.Name,
				Arguments: cc.Function.Arguments,
			},
		}
	}
	return qwenToolCalls
}

// convertQwenToolCallsToUniversal converts Qwen tool calls to universal format
// Qwen uses OpenAI-compatible format, so we use the shared implementation
func convertQwenToolCallsToUniversal(toolCalls []QwenToolCall) []types.ToolCall {
	// Convert to OpenAI-compatible format
	compatibleCalls := make([]streaming.OpenAICompatibleToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		compatibleCalls[i] = streaming.OpenAICompatibleToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: streaming.OpenAICompatibleToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}

	// Convert using shared converter
	return streaming.ConvertOpenAICompatibleToolCallsToUniversal(compatibleCalls)
}

// convertContentPartsToQwen converts ContentParts to Qwen format (OpenAI-compatible)
// Returns a string if there's only text, or []QwenContentPart if multimodal
func convertContentPartsToQwen(parts []types.ContentPart) interface{} {
	if len(parts) == 0 {
		return ""
	}

	// If only one part and it's text, return as string for backwards compatibility
	if len(parts) == 1 && parts[0].Type == types.ContentTypeText {
		return parts[0].Text
	}

	// Otherwise, build multimodal content array
	qwenParts := make([]QwenContentPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case types.ContentTypeText:
			qwenParts = append(qwenParts, QwenContentPart{
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
					qwenParts = append(qwenParts, QwenContentPart{
						Type: "image_url",
						ImageURL: &QwenImageURL{
							URL: url,
						},
					})
				}
			}
			// Note: Qwen uses OpenAI-compatible format
			// Documents/audio in chat would need separate handling
		case types.ContentTypeToolResult:
			// For tool results, extract text content
			if resultText, ok := part.Content.(string); ok {
				// Return as simple text for tool responses
				qwenParts = append(qwenParts, QwenContentPart{
					Type: "text",
					Text: resultText,
				})
			} else {
				qwenParts = append(qwenParts, QwenContentPart{
					Type: "text",
					Text: fmt.Sprintf("%v", part.Content),
				})
			}
		}
	}

	return qwenParts
}
