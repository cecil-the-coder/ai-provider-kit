package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// StreamEndpoint represents the streaming endpoint type
type StreamEndpoint string

const (
	// StreamEndpointOllama uses the native Ollama /api/chat endpoint (newline-delimited JSON)
	StreamEndpointOllama StreamEndpoint = "ollama"
	// StreamEndpointOpenAI uses the OpenAI-compatible /v1/chat/completions endpoint (SSE format)
	StreamEndpointOpenAI StreamEndpoint = "openai"
)

// OllamaStream implements ChatCompletionStream for Ollama streaming responses.
// It supports both native Ollama newline-delimited JSON format and OpenAI-compatible SSE format.
type OllamaStream struct {
	reader   *bufio.Reader
	body     io.ReadCloser
	done     bool
	model    string
	provider *OllamaProvider
	ctx      context.Context

	// Metadata tracking
	startTime    time.Time
	totalTokens  int
	promptTokens int
	deltaTokens  int

	// Endpoint format
	endpoint StreamEndpoint

	// Buffer for accumulating tool calls in OpenAI format
	toolCallBuffer map[int]*types.ToolCall
}

// StreamMetadata contains metadata about the stream including token usage and timing information.
type StreamMetadata struct {
	TotalTokens   int
	PromptTokens  int
	OutputTokens  int
	Duration      time.Duration
	FirstByteTime time.Duration
}

// GetMetadata returns the current stream metadata including token counts and duration.
func (s *OllamaStream) GetMetadata() StreamMetadata {
	return StreamMetadata{
		TotalTokens:  s.totalTokens,
		PromptTokens: s.promptTokens,
		OutputTokens: s.deltaTokens,
		Duration:     time.Since(s.startTime),
	}
}

// Next returns the next chunk from the stream.
// Returns io.EOF when the stream is complete.
func (s *OllamaStream) Next() (types.ChatCompletionChunk, error) {
	if s.done {
		return types.ChatCompletionChunk{Done: true}, io.EOF
	}

	// Check if context is cancelled
	select {
	case <-s.ctx.Done():
		s.done = true
		return types.ChatCompletionChunk{Done: true}, s.ctx.Err()
	default:
	}

	// Read based on endpoint format
	switch s.endpoint {
	case StreamEndpointOpenAI:
		return s.nextOpenAI()
	default:
		return s.nextOllama()
	}
}

// nextOllama reads from native Ollama endpoint (newline-delimited JSON)
func (s *OllamaStream) nextOllama() (types.ChatCompletionChunk, error) {
	// Read next line (Ollama uses newline-delimited JSON, not SSE)
	line, err := s.reader.ReadBytes('\n')
	if err != nil {
		if err == io.EOF {
			s.done = true
			return types.ChatCompletionChunk{Done: true}, io.EOF
		}
		return types.ChatCompletionChunk{}, fmt.Errorf("failed to read stream: %w", err)
	}

	// Handle empty lines
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return s.nextOllama() // Skip empty lines
	}

	// Parse the JSON response
	var resp ollamaChatResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		// Skip malformed lines and continue
		return s.nextOllama()
	}

	// Build the chunk
	chunk := types.ChatCompletionChunk{
		Model:   resp.Model,
		Content: resp.Message.Content,
		Done:    resp.Done,
	}

	// Track tokens
	if resp.EvalCount > 0 {
		s.deltaTokens = resp.EvalCount
	}

	// If this is the final chunk, include usage information
	if resp.Done {
		s.done = true
		s.promptTokens = resp.PromptEvalCount
		s.totalTokens = resp.PromptEvalCount + resp.EvalCount
		chunk.Usage = types.Usage{
			PromptTokens:     resp.PromptEvalCount,
			CompletionTokens: resp.EvalCount,
			TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
		}
	}

	// Handle tool calls if present
	if len(resp.Message.ToolCalls) > 0 {
		chunk.Choices = []types.ChatChoice{
			{
				Index: 0,
				Delta: types.ChatMessage{
					Role:      resp.Message.Role,
					Content:   resp.Message.Content,
					ToolCalls: s.provider.convertOllamaToolCallsToUniversal(resp.Message.ToolCalls),
				},
			},
		}
	} else if resp.Message.Content != "" {
		// Regular content chunk
		chunk.Choices = []types.ChatChoice{
			{
				Index: 0,
				Delta: types.ChatMessage{
					Role:    resp.Message.Role,
					Content: resp.Message.Content,
				},
			},
		}
	}

	return chunk, nil
}

// nextOpenAI reads from OpenAI-compatible endpoint (SSE format)
func (s *OllamaStream) nextOpenAI() (types.ChatCompletionChunk, error) {
	for {
		// Read next line
		line, err := s.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				s.done = true
				return types.ChatCompletionChunk{Done: true}, io.EOF
			}
			return types.ChatCompletionChunk{}, fmt.Errorf("failed to read stream: %w", err)
		}

		// Handle SSE format: "data: {...}"
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue // Skip empty lines
		}

		// Check for SSE data prefix
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue // Skip non-data lines
		}

		// Extract JSON data
		data := bytes.TrimPrefix(line, []byte("data: "))
		data = bytes.TrimSpace(data)

		// Check for [DONE] marker
		if bytes.Equal(data, []byte("[DONE]")) {
			s.done = true
			return types.ChatCompletionChunk{Done: true}, io.EOF
		}

		// Parse OpenAI-compatible response
		var resp openAIStreamResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			// Skip malformed chunks
			continue
		}

		// Build the chunk
		chunk := types.ChatCompletionChunk{
			Model: resp.Model,
			Done:  false,
		}

		// Process choices
		if len(resp.Choices) > 0 {
			choice := resp.Choices[0]
			chunk.Content = choice.Delta.Content

			// Convert delta to ChatMessage
			deltaMsg := types.ChatMessage{
				Role:    choice.Delta.Role,
				Content: choice.Delta.Content,
			}

			// Handle tool calls - OpenAI streams them incrementally
			if len(choice.Delta.ToolCalls) > 0 {
				// Initialize buffer if needed
				if s.toolCallBuffer == nil {
					s.toolCallBuffer = make(map[int]*types.ToolCall)
				}

				// Accumulate tool calls
				for _, tc := range choice.Delta.ToolCalls {
					if tc.Index != nil {
						idx := *tc.Index
						if s.toolCallBuffer[idx] == nil {
							s.toolCallBuffer[idx] = &types.ToolCall{
								ID:   tc.ID,
								Type: tc.Type,
								Function: types.ToolCallFunction{
									Name:      tc.Function.Name,
									Arguments: tc.Function.Arguments,
								},
							}
						} else {
							// Append incremental data
							if tc.ID != "" {
								s.toolCallBuffer[idx].ID = tc.ID
							}
							if tc.Function.Name != "" {
								s.toolCallBuffer[idx].Function.Name = tc.Function.Name
							}
							if tc.Function.Arguments != "" {
								s.toolCallBuffer[idx].Function.Arguments += tc.Function.Arguments
							}
						}
					}
				}

				// Update delta message with accumulated tool calls
				accumulated := make([]types.ToolCall, 0, len(s.toolCallBuffer))
				for _, tc := range s.toolCallBuffer {
					accumulated = append(accumulated, *tc)
				}
				deltaMsg.ToolCalls = accumulated
			}

			chunk.Choices = []types.ChatChoice{
				{
					Index:        choice.Index,
					Delta:        deltaMsg,
					FinishReason: choice.FinishReason,
				},
			}

			// Check for finish
			if choice.FinishReason != "" {
				s.done = true
				chunk.Done = true

				// Include usage if present
				if resp.Usage != nil {
					s.promptTokens = resp.Usage.PromptTokens
					s.deltaTokens = resp.Usage.CompletionTokens
					s.totalTokens = resp.Usage.TotalTokens
					chunk.Usage = types.Usage{
						PromptTokens:     resp.Usage.PromptTokens,
						CompletionTokens: resp.Usage.CompletionTokens,
						TotalTokens:      resp.Usage.TotalTokens,
					}
				}
			}
		}

		return chunk, nil
	}
}

// Close closes the stream and releases resources.
// It is safe to call Close multiple times.
func (s *OllamaStream) Close() error {
	s.done = true
	if s.body != nil {
		return s.body.Close()
	}
	return nil
}

// openAIStreamResponse represents an OpenAI-compatible streaming response
type openAIStreamResponse struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []openAIStreamChoice `json:"choices"`
	Usage   *types.Usage         `json:"usage,omitempty"`
}

// openAIStreamChoice represents a choice in OpenAI streaming
type openAIStreamChoice struct {
	Index        int               `json:"index"`
	Delta        openAIStreamDelta `json:"delta"`
	FinishReason string            `json:"finish_reason,omitempty"`
}

// openAIStreamDelta represents the delta in OpenAI streaming
type openAIStreamDelta struct {
	Role      string                 `json:"role,omitempty"`
	Content   string                 `json:"content,omitempty"`
	ToolCalls []openAIStreamToolCall `json:"tool_calls,omitempty"`
}

// openAIStreamToolCall represents a tool call in OpenAI streaming (with index)
type openAIStreamToolCall struct {
	Index    *int                     `json:"index,omitempty"`
	ID       string                   `json:"id,omitempty"`
	Type     string                   `json:"type,omitempty"`
	Function openAIStreamFunctionCall `json:"function"`
}

// openAIStreamFunctionCall represents a function call in streaming
type openAIStreamFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// makeStreamingRequest creates a streaming request and returns the stream
func (p *OllamaProvider) makeStreamingRequest(
	ctx context.Context,
	endpoint StreamEndpoint,
	request ollamaChatRequest,
) (types.ChatCompletionStream, error) {
	// Determine URL based on endpoint type
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	var url string
	switch endpoint {
	case StreamEndpointOpenAI:
		url = fmt.Sprintf("%s/v1/chat/completions", baseURL)
	default:
		url = fmt.Sprintf("%s/api/chat", baseURL)
	}

	// Make the API call
	resp, err := p.makeHTTPStreamRequest(ctx, url, request)
	if err != nil {
		return nil, err
	}

	// Create and return streaming response
	return &OllamaStream{
		reader:         bufio.NewReader(resp.Body),
		body:           resp.Body,
		done:           false,
		model:          request.Model,
		provider:       p,
		ctx:            ctx,
		startTime:      time.Now(),
		endpoint:       endpoint,
		toolCallBuffer: make(map[int]*types.ToolCall),
	}, nil
}
