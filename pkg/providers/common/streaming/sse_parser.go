package streaming

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// SSELineParser defines provider-specific parsing behavior for SSE streams.
// This interface allows different providers to implement their own parsing logic
// while using the common SSE stream infrastructure.
type SSELineParser interface {
	// ParseLine parses a single SSE data line into a chunk.
	// Returns the parsed chunk and any error that occurred during parsing.
	ParseLine(line string) (types.ChatCompletionChunk, error)

	// IsDone checks if this line indicates stream completion.
	// Common completion signals include "[DONE]" or specific event types.
	IsDone(line string) bool
}

// GenericSSEStream wraps SSE parsing for any provider that implements SSELineParser.
// It handles the low-level SSE protocol details (reading lines, handling "data:" prefix)
// while delegating provider-specific parsing to the SSELineParser implementation.
type GenericSSEStream struct {
	response *http.Response
	reader   *bufio.Reader
	parser   SSELineParser
	done     bool
	mu       sync.Mutex
}

// NewGenericSSEStream creates a new generic SSE stream with the given response and parser.
// The parser parameter defines how individual SSE lines are parsed into ChatCompletionChunk objects.
func NewGenericSSEStream(resp *http.Response, parser SSELineParser) *GenericSSEStream {
	return &GenericSSEStream{
		response: resp,
		reader:   bufio.NewReader(resp.Body),
		parser:   parser,
		done:     false,
	}
}

// Next returns the next chunk from the SSE stream.
// It reads lines from the stream, extracts SSE data, and uses the parser to convert them to chunks.
// Returns io.EOF when the stream is complete or an error if parsing fails.
func (s *GenericSSEStream) Next() (types.ChatCompletionChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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
			return types.ChatCompletionChunk{}, fmt.Errorf("error reading stream: %w", err)
		}

		line = strings.TrimSpace(line)

		// Skip empty lines (SSE uses empty lines as delimiters)
		if line == "" {
			continue
		}

		// Check for SSE comment lines (start with ':')
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Extract SSE data
		var data string
		switch {
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case strings.HasPrefix(line, "data:"):
			data = strings.TrimPrefix(line, "data:")
		default:
			// Skip non-data SSE fields (event:, id:, retry:)
			continue
		}

		// Check if parser indicates this is a completion signal
		if s.parser.IsDone(data) {
			s.done = true
			return types.ChatCompletionChunk{Done: true}, io.EOF
		}

		// Parse the data line using provider-specific parser
		chunk, err := s.parser.ParseLine(data)
		if err != nil {
			// Log error but continue to next line (some providers send malformed chunks)
			continue
		}

		// Check if chunk indicates completion
		if chunk.Done {
			s.done = true
			return chunk, io.EOF
		}

		// Return the parsed chunk
		return chunk, nil
	}
}

// Close closes the underlying HTTP response body and cleans up resources.
func (s *GenericSSEStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.done = true
	if s.response != nil && s.response.Body != nil {
		return s.response.Body.Close()
	}
	return nil
}

// OpenAICompatibleParser handles OpenAI-style SSE responses.
// This parser works with the standard OpenAI streaming format and can be used
// by any provider that follows OpenAI's streaming API conventions.
type OpenAICompatibleParser struct {
	// SkipEmptyContent determines whether to skip chunks with empty content
	SkipEmptyContent bool
}

// NewOpenAICompatibleParser creates a new OpenAI-compatible SSE parser with default settings.
func NewOpenAICompatibleParser() *OpenAICompatibleParser {
	return &OpenAICompatibleParser{
		SkipEmptyContent: true, // By default, skip chunks with no content
	}
}

// ParseLine parses a single SSE data line in OpenAI format into a ChatCompletionChunk.
// The OpenAI format is JSON with the structure:
//
//	{
//	  "id": "chatcmpl-...",
//	  "object": "chat.completion.chunk",
//	  "created": 1234567890,
//	  "model": "gpt-4",
//	  "choices": [{
//	    "index": 0,
//	    "delta": {
//	      "role": "assistant",
//	      "content": "Hello"
//	    },
//	    "finish_reason": null
//	  }]
//	}
func (p *OpenAICompatibleParser) ParseLine(line string) (types.ChatCompletionChunk, error) {
	// Parse the JSON response
	var streamResp struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index        int    `json:"index"`
			FinishReason string `json:"finish_reason"`
			Delta        struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					Index    int    `json:"index"`
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := unmarshalJSON([]byte(line), &streamResp); err != nil {
		return types.ChatCompletionChunk{}, fmt.Errorf("failed to parse OpenAI stream response: %w", err)
	}

	// Build the chunk
	chunk := types.ChatCompletionChunk{
		ID:      streamResp.ID,
		Object:  streamResp.Object,
		Created: streamResp.Created,
		Model:   streamResp.Model,
	}

	// Extract choices
	if len(streamResp.Choices) > 0 {
		choice := streamResp.Choices[0]

		// Set content from delta
		chunk.Content = choice.Delta.Content

		// Build ChatChoice for the chunk
		chatChoice := types.ChatChoice{
			Index:        choice.Index,
			FinishReason: choice.FinishReason,
			Delta: types.ChatMessage{
				Role:    choice.Delta.Role,
				Content: choice.Delta.Content,
			},
		}

		// Convert tool calls if present
		if len(choice.Delta.ToolCalls) > 0 {
			chatChoice.Delta.ToolCalls = make([]types.ToolCall, 0, len(choice.Delta.ToolCalls))
			for _, tc := range choice.Delta.ToolCalls {
				chatChoice.Delta.ToolCalls = append(chatChoice.Delta.ToolCalls, types.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: types.ToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
		}

		chunk.Choices = []types.ChatChoice{chatChoice}

		// Check if stream is done based on finish_reason
		if choice.FinishReason != "" && choice.FinishReason != "null" {
			chunk.Done = true
		}
	}

	// Extract usage if present
	if streamResp.Usage != nil {
		chunk.Usage = types.Usage{
			PromptTokens:     streamResp.Usage.PromptTokens,
			CompletionTokens: streamResp.Usage.CompletionTokens,
			TotalTokens:      streamResp.Usage.TotalTokens,
		}
	}

	return chunk, nil
}

// IsDone checks if the given line indicates stream completion.
// OpenAI sends "[DONE]" as the final message in the stream.
func (p *OpenAICompatibleParser) IsDone(line string) bool {
	return strings.TrimSpace(line) == "[DONE]"
}

// unmarshalJSON is a helper function to unmarshal JSON with error handling.
// This can be replaced with encoding/json.Unmarshal but is abstracted here
// for potential future customization (e.g., strict parsing, custom decoders).
func unmarshalJSON(data []byte, v interface{}) error {
	// Using the standard library's json package
	// In the future, this could be replaced with a faster parser if needed
	return json.Unmarshal(data, v)
}
