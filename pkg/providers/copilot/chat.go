// Package copilot provides chat completion logic for GitHub Copilot AI provider.
package copilot

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
	"time"

	"github.com/google/uuid"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/streaming"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// GenerateChatCompletion generates a chat completion
func (p *CopilotProvider) GenerateChatCompletion(
	ctx context.Context,
	options types.GenerateOptions,
) (types.ChatCompletionStream, error) {
	p.IncrementRequestCount()
	startTime := time.Now()

	// Get model
	model := options.Model
	if model == "" {
		model = p.GetDefaultModel()
	}
	if model == "" {
		model = copilotDefaultModel
	}

	// Handle streaming
	if options.Stream {
		stream, err := p.executeStreamWithAuth(ctx, options, model)
		if err != nil {
			p.RecordError(err)
			return nil, err
		}
		latency := time.Since(startTime)
		p.RecordSuccess(latency, 0) // Tokens will be counted as stream is consumed
		return stream, nil
	}

	// Non-streaming path
	token, err := p.GetCopilotToken(ctx)
	if err != nil {
		p.RecordError(err)
		return nil, types.NewAuthError(types.ProviderTypeCopilot, "failed to get Copilot token").
			WithOperation("GenerateChatCompletion").
			WithOriginalErr(err)
	}

	requestData, err := p.prepareRequest(options, model)
	if err != nil {
		p.RecordError(err)
		return nil, err
	}

	response, err := p.makeAPICall(ctx, requestData, token)
	if err != nil {
		p.RecordError(err)
		return nil, err
	}

	// Record success
	latency := time.Since(startTime)
	var tokensUsed int64
	tokensUsed = int64(response.Usage.TotalTokens)
	p.RecordSuccess(latency, tokensUsed)

	// Convert response to ChatCompletionChunk
	chunk := types.ChatCompletionChunk{
		ID:      response.ID,
		Object:  response.Object,
		Created: response.Created,
		Model:   response.Model,
		Content: extractContent(response),
		Done:    true,
		Usage: types.Usage{
			PromptTokens:     response.Usage.PromptTokens,
			CompletionTokens: response.Usage.CompletionTokens,
			TotalTokens:      response.Usage.TotalTokens,
		},
	}

	if len(response.Choices) > 0 {
		chunk.Choices = []types.ChatChoice{
			{
				Index: response.Choices[0].Index,
				Message: types.ChatMessage{
					Role:       response.Choices[0].Message.Role,
					Content:    extractMessageContent(response.Choices[0].Message.Content),
					ToolCalls:  convertToolCalls(response.Choices[0].Message.ToolCalls),
				},
				FinishReason: response.Choices[0].FinishReason,
			},
		}
	}

	return streaming.NewMockStream([]types.ChatCompletionChunk{chunk}), nil
}

// prepareRequest prepares the API request payload
func (p *CopilotProvider) prepareRequest(options types.GenerateOptions, model string) (*ChatCompletionRequest, error) {
	// Convert messages
	messages := make([]ChatMessage, 0, len(options.Messages))

	// Handle prompt field for backward compatibility
	if len(options.Messages) == 0 && options.Prompt != "" {
		messages = append(messages, ChatMessage{
			Role:    "user",
			Content: options.Prompt,
		})
	} else {
		for _, msg := range options.Messages {
			messages = append(messages, ChatMessage{
				Role:    msg.Role,
				Content: convertContent(msg.Content),
			})
		}
	}

	req := &ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}

	// Set temperature if provided
	if options.Temperature > 0 {
		req.Temperature = options.Temperature
	}

	// Set max_tokens
	if options.MaxTokens > 0 {
		req.MaxTokens = options.MaxTokens
	} else {
		// Default to 4096 if not available
		req.MaxTokens = DefaultMaxTokens
	}

	// Convert tools
	if len(options.Tools) > 0 {
		req.Tools = convertTools(options.Tools)
		if options.ToolChoice != nil {
			req.ToolChoice = convertToolChoice(options.ToolChoice)
		}
	}

	return req, nil
}

// makeAPICall makes the actual HTTP request to the Copilot API
func (p *CopilotProvider) makeAPICall(ctx context.Context, requestData *ChatCompletionRequest, token string) (*ChatCompletionResponse, error) {
	url := p.GetBaseURL() + "/chat/completions"

	jsonBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setCopilotHeaders(req, token)
	req.Header.Set("x-request-id", uuid.New().String())
	req.Header.Set("X-Initiator", p.getInitiator(requestData.Messages))

	if p.hasVisionContent(requestData.Messages) {
		req.Header.Set("copilot-vision-request", "true")
	}

	p.LogRequest("POST", url, map[string]string{
		"Authorization":         "Bearer ***",
		"Content-Type":          "application/json",
		"copilot-integration-id": CopilotIntegrationID,
	}, requestData)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, p.handleAPIError(resp.StatusCode, body)
	}

	var response ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// executeStreamWithAuth handles streaming requests
func (p *CopilotProvider) executeStreamWithAuth(ctx context.Context, options types.GenerateOptions, model string) (types.ChatCompletionStream, error) {
	token, err := p.GetCopilotToken(ctx)
	if err != nil {
		return nil, types.NewAuthError(types.ProviderTypeCopilot, "failed to get Copilot token").
			WithOperation("executeStreamWithAuth").
			WithOriginalErr(err)
	}

	requestData, err := p.prepareRequest(options, model)
	if err != nil {
		return nil, err
	}

	requestData.Stream = true

	return p.makeStreamingAPICall(ctx, requestData, token)
}

// makeStreamingAPICall makes a streaming API call
func (p *CopilotProvider) makeStreamingAPICall(ctx context.Context, requestData *ChatCompletionRequest, token string) (types.ChatCompletionStream, error) {
	url := p.GetBaseURL() + "/chat/completions"

	jsonBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setCopilotHeaders(req, token)
	req.Header.Set("x-request-id", uuid.New().String())
	req.Header.Set("X-Initiator", p.getInitiator(requestData.Messages))

	if p.hasVisionContent(requestData.Messages) {
		req.Header.Set("copilot-vision-request", "true")
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, p.handleAPIError(resp.StatusCode, body)
	}

	return NewCopilotStream(resp), nil
}

// handleAPIError handles API errors
func (p *CopilotProvider) handleAPIError(statusCode int, body []byte) error {
	var errResp ErrorResponse
	if json.Unmarshal(body, &errResp) == nil {
		message := errResp.Error.Message
		if message == "" {
			message = string(body)
		}

		switch statusCode {
		case 400:
			return types.NewInvalidRequestError(types.ProviderTypeCopilot, message).
				WithOperation("api_call").
				WithStatusCode(statusCode)
		case 401:
			return types.NewAuthError(types.ProviderTypeCopilot, "invalid or expired Copilot token").
				WithOperation("api_call").
				WithStatusCode(statusCode)
		case 429:
			return types.NewRateLimitError(types.ProviderTypeCopilot, 0).
				WithOperation("api_call")
		case 500, 502, 503, 504:
			return types.NewServerError(types.ProviderTypeCopilot, statusCode, message).
				WithOperation("api_call")
		default:
			return types.NewServerError(types.ProviderTypeCopilot, statusCode, message).
				WithOperation("api_call")
		}
	}

	return types.NewServerError(types.ProviderTypeCopilot, statusCode, string(body)).
		WithOperation("api_call")
}

// Helper functions for conversion

func convertContent(content interface{}) interface{} {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		// Convert to ContentPart slice
		parts := make([]ContentPart, 0, len(v))
		for _, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				part := ContentPart{Type: itemMap["type"].(string)}
				if text, ok := itemMap["text"].(string); ok {
					part.Text = text
				}
				if imageURL, ok := itemMap["image_url"].(map[string]interface{}); ok {
					if url, ok := imageURL["url"].(string); ok {
						part.ImageURL = &ImageURL{URL: url}
						if detail, ok := imageURL["detail"].(string); ok {
							part.ImageURL.Detail = detail
						}
					}
				}
				parts = append(parts, part)
			}
		}
		return parts
	default:
		return content
	}
}

func convertTools(tools []types.Tool) []Tool {
	converted := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		t := Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
			},
		}

		// Convert InputSchema to Parameters
		if tool.InputSchema != nil {
			t.Function.Parameters = tool.InputSchema
		}

		converted = append(converted, t)
	}
	return converted
}

func convertToolChoice(toolChoice *types.ToolChoice) interface{} {
	if toolChoice == nil {
		return "auto"
	}

	switch toolChoice.Mode {
	case types.ToolChoiceNone:
		return "none"
	case types.ToolChoiceAuto:
		return "auto"
	case types.ToolChoiceRequired, types.ToolChoiceSpecific:
		return "required"
	default:
		return "auto"
	}
}

func convertToolCalls(toolCalls []ToolCall) []types.ToolCall {
	if toolCalls == nil {
		return nil
	}

	converted := make([]types.ToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		converted = append(converted, types.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: types.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return converted
}

func extractContent(response *ChatCompletionResponse) string {
	if len(response.Choices) > 0 {
		msg := response.Choices[0].Message
		return extractMessageContent(msg.Content)
	}
	return ""
}

func extractMessageContent(content interface{}) string {
	// If content is a string, return it
	if contentStr, ok := content.(string); ok {
		return contentStr
	}

	// If content is a slice, extract text parts
	if parts, ok := content.([]ContentPart); ok {
		var text strings.Builder
		for _, part := range parts {
			if part.Type == "text" {
				text.WriteString(part.Text)
			}
		}
		return text.String()
	}

	return ""
}

// CopilotStream implements ChatCompletionStream for Copilot SSE streaming
type CopilotStream struct {
	resp    *http.Response
	scanner *bufio.Scanner
	closed  bool
}

// NewCopilotStream creates a new Copilot stream
func NewCopilotStream(resp *http.Response) *CopilotStream {
	return &CopilotStream{
		resp:    resp,
		scanner: newStreamScanner(resp),
	}
}

// newStreamScanner creates a scanner for streaming
func newStreamScanner(resp *http.Response) *bufio.Scanner {
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		// Find next newline
		for i := 0; i < len(data); i++ {
			if data[i] == '\n' {
				return i + 1, data[:i], nil
			}
		}

		if atEOF {
			return len(data), data, nil
		}

		return 0, nil, nil
	})
	return scanner
}

// Next returns the next chunk from the stream
func (s *CopilotStream) Next() (types.ChatCompletionChunk, error) {
	if s.closed {
		return types.ChatCompletionChunk{}, io.EOF
	}

	for s.scanner.Scan() {
		line := s.scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			s.Close()
			return types.ChatCompletionChunk{Done: true}, nil
		}

		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("Copilot: Failed to parse chunk: %v, data: %s", err, data)
			continue
		}

		// Convert to internal format
		internalChunk := types.ChatCompletionChunk{
			ID:      chunk.ID,
			Object:  chunk.Object,
			Created: chunk.Created,
			Model:   chunk.Model,
			Done:    false,
		}

		if len(chunk.Choices) > 0 {
			internalChunk.Choices = []types.ChatChoice{
				{
					Index: chunk.Choices[0].Index,
					Delta: types.ChatMessage{
						Role:       chunk.Choices[0].Delta.Role,
						Content:    chunk.Choices[0].Delta.Content,
						ToolCalls:  convertToolCalls(chunk.Choices[0].Delta.ToolCalls),
					},
					FinishReason: getFinishReason(chunk.Choices[0].FinishReason),
				},
			}

			// Extract content from delta
			if chunk.Choices[0].Delta.Content != "" {
				internalChunk.Content = chunk.Choices[0].Delta.Content
			}
		}

		if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
			internalChunk.Usage = types.Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}

		return internalChunk, nil
	}

	if err := s.scanner.Err(); err != nil {
		s.Close()
		return types.ChatCompletionChunk{}, err
	}

	s.Close()
	return types.ChatCompletionChunk{Done: true}, nil
}

// Close closes the stream
func (s *CopilotStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.resp.Body.Close()
}

func getFinishReason(fr *string) string {
	if fr != nil {
		return *fr
	}
	return ""
}
