// Package cerebras provides type definitions for Cerebras AI provider.
// It includes request/response structures and streaming types compatible with OpenAI format.
package cerebras

import (
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// API Request/Response structures for Cerebras

// CerebrasRequest represents a request to the Cerebras chat completions API
type CerebrasRequest struct {
	Model          string                 `json:"model"`
	Messages       []CerebrasMessage      `json:"messages"`
	Temperature    *float64               `json:"temperature,omitempty"`
	MaxTokens      *int                   `json:"max_tokens,omitempty"`
	Stream         bool                   `json:"stream"`
	Stop           []string               `json:"stop,omitempty"`
	Tools          []CerebrasTool         `json:"tools,omitempty"`
	ToolChoice     interface{}            `json:"tool_choice,omitempty"`
	ResponseFormat map[string]interface{} `json:"response_format,omitempty"` // For structured outputs
}

// CerebrasTool represents a tool in the Cerebras API (OpenAI-compatible)
type CerebrasTool struct {
	Type     string              `json:"type"` // Always "function"
	Function CerebrasFunctionDef `json:"function"`
}

// CerebrasFunctionDef represents a function definition in the Cerebras API
type CerebrasFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// CerebrasMessage represents a message in the Cerebras API
type CerebrasMessage struct {
	Role       string             `json:"role"`
	Content    string             `json:"content"`
	Reasoning  string             `json:"reasoning,omitempty"` // GLM-4.6 specific field
	ToolCalls  []CerebrasToolCall `json:"tool_calls,omitempty"`
	ToolCallID string             `json:"tool_call_id,omitempty"`
}

// CerebrasToolCall represents a tool call in the Cerebras API
type CerebrasToolCall struct {
	ID       string                   `json:"id"`
	Type     string                   `json:"type"` // "function"
	Function CerebrasToolCallFunction `json:"function"`
}

// CerebrasToolCallFunction represents a function call in a tool call
type CerebrasToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// CerebrasResponse represents a response from the Cerebras API
type CerebrasResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []CerebrasChoice `json:"choices"`
	Usage   CerebrasUsage    `json:"usage"`
}

// CerebrasChoice represents a choice in the Cerebras API response
type CerebrasChoice struct {
	Index        int             `json:"index"`
	Message      CerebrasMessage `json:"message"` // Used in non-streaming responses
	Delta        CerebrasDelta   `json:"delta"`   // Used in streaming responses
	FinishReason string          `json:"finish_reason"`
}

// CerebrasDelta represents incremental updates in streaming responses
type CerebrasDelta struct {
	Role      string             `json:"role,omitempty"`
	Content   string             `json:"content,omitempty"`
	Reasoning string             `json:"reasoning,omitempty"` // GLM-4.6 specific field
	ToolCalls []CerebrasToolCall `json:"tool_calls,omitempty"`
}

// CerebrasUsage represents token usage information from Cerebras
type CerebrasUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CerebrasStream implements types.ChatCompletionStream for Cerebras responses
type CerebrasStream struct {
	content string
	usage   types.Usage
	model   string
	closed  bool
	index   int
	chunk   types.ChatCompletionChunk
}

// Next returns the next chunk from the stream
func (s *CerebrasStream) Next() (types.ChatCompletionChunk, error) {
	if s.closed || s.index > 0 {
		return types.ChatCompletionChunk{}, nil
	}

	// If a pre-built chunk is available (with tool calls), return it
	if len(s.chunk.Choices) > 0 {
		s.index++
		return s.chunk, nil
	}

	// Otherwise, build a simple content chunk
	chunk := types.ChatCompletionChunk{
		ID:      "cerebras-" + s.model,
		Object:  "chat.completion.chunk",
		Created: 0, // Not provided in non-streaming mode
		Model:   s.model,
		Choices: []types.ChatChoice{
			{
				Index: 0,
				Message: types.ChatMessage{
					Role:    "assistant",
					Content: s.content,
				},
				FinishReason: "stop",
			},
		},
		Usage:   s.usage,
		Content: s.content,
		Done:    true,
	}

	s.index++
	return chunk, nil
}

// Close closes the stream
func (s *CerebrasStream) Close() error {
	s.closed = true
	s.index = 0
	return nil
}

// CerebrasModelsResponse represents the response from /v1/models endpoint (OpenAI-compatible)
type CerebrasModelsResponse struct {
	Object string              `json:"object"`
	Data   []CerebrasModelData `json:"data"`
}

// CerebrasModelData represents a model in the models list
type CerebrasModelData struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}
