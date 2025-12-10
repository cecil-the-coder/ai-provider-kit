package qwen

import (
	"sync"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// QwenRequest represents the API request to Qwen
type QwenRequest struct {
	Model       string        `json:"model"`
	Messages    []QwenMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
	Tools       []QwenTool    `json:"tools,omitempty"`
	ToolChoice  interface{}   `json:"tool_choice,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
}

// QwenTool represents a tool in Qwen API (OpenAI-compatible format)
type QwenTool struct {
	Type     string          `json:"type"`
	Function QwenFunctionDef `json:"function"`
}

// QwenFunctionDef represents a function definition in Qwen API
type QwenFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// QwenMessage represents a message in the conversation
type QwenMessage struct {
	Role       string         `json:"role"`
	Content    interface{}    `json:"content"`             // string or []QwenContentPart for multimodal
	ToolCalls  []QwenToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// QwenContentPart represents a content part in Qwen's multimodal format (OpenAI-compatible)
type QwenContentPart struct {
	Type     string         `json:"type"`                // "text" or "image_url"
	Text     string         `json:"text,omitempty"`      // Text content
	ImageURL *QwenImageURL  `json:"image_url,omitempty"` // Image URL content
}

// QwenImageURL represents an image URL in Qwen format (OpenAI-compatible)
type QwenImageURL struct {
	URL    string `json:"url"`              // URL or data URL
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

// QwenToolCall represents a tool call in Qwen API (OpenAI-compatible format)
type QwenToolCall struct {
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function QwenToolCallFunction `json:"function"`
}

// QwenToolCallFunction represents a function call in a tool call
type QwenToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// QwenResponse represents the API response from Qwen
type QwenResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []QwenChoice `json:"choices"`
	Usage   QwenUsage    `json:"usage"`
}

// QwenChoice represents a choice in the response
type QwenChoice struct {
	Index        int         `json:"index"`
	Message      QwenMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
	Delta        QwenMessage `json:"delta"`
}

// QwenUsage represents token usage information
type QwenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// QwenStream implements ChatCompletionStream for Qwen responses (legacy)
type QwenStream struct {
	content string
	usage   *types.Usage
	model   string
	closed  bool
	index   int
}

// Next returns the next chunk from the stream
func (qs *QwenStream) Next() (types.ChatCompletionChunk, error) {
	if qs.closed || qs.index > 0 {
		return types.ChatCompletionChunk{}, nil
	}

	// Convert usage if available
	var usage types.Usage
	if qs.usage != nil {
		usage = types.Usage{
			PromptTokens:     qs.usage.PromptTokens,
			CompletionTokens: qs.usage.CompletionTokens,
			TotalTokens:      qs.usage.TotalTokens,
		}
	}

	chunk := types.ChatCompletionChunk{
		ID:      "qwen-response",
		Object:  "chat.completion",
		Created: 0,
		Model:   qs.model,
		Choices: []types.ChatChoice{
			{
				Index: 0,
				Message: types.ChatMessage{
					Role:    "assistant",
					Content: qs.content,
				},
				FinishReason: "stop",
			},
		},
		Usage:   usage,
		Done:    true,
		Content: qs.content,
	}

	qs.index++
	return chunk, nil
}

// Close closes the stream
func (qs *QwenStream) Close() error {
	qs.closed = true
	qs.index = 0
	return nil
}

// QwenStreamWithMessage implements ChatCompletionStream with full message support (tool calls)
type QwenStreamWithMessage struct {
	chunk  types.ChatCompletionChunk
	closed bool
	index  int
}

// Next returns the next chunk from the stream
func (qs *QwenStreamWithMessage) Next() (types.ChatCompletionChunk, error) {
	if qs.closed || qs.index > 0 {
		return types.ChatCompletionChunk{}, nil
	}

	qs.index++
	return qs.chunk, nil
}

// Close closes the stream
func (qs *QwenStreamWithMessage) Close() error {
	qs.closed = true
	qs.index = 0
	return nil
}

// APIKeyManager manages multiple API keys with rotation
type APIKeyManager struct {
	mu   sync.RWMutex
	keys []string
}

// NewAPIKeyManager creates a new API key manager
func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{}
}

// addKey adds an API key to the manager
func (m *APIKeyManager) addKey(key string) {
	if key == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if key already exists
	for _, existingKey := range m.keys {
		if existingKey == key {
			return
		}
	}
	m.keys = append(m.keys, key)
}

// getKeys returns all API keys
func (m *APIKeyManager) getKeys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, len(m.keys))
	copy(keys, m.keys)
	return keys
}

// clear removes all API keys
func (m *APIKeyManager) clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys = nil
}

// getCurrentKey returns the current API key (first in the list)
func (m *APIKeyManager) getCurrentKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.keys) == 0 {
		return ""
	}
	return m.keys[0]
}

// rotateKey rotates to the next API key
func (m *APIKeyManager) rotateKey() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.keys) <= 1 {
		return m.getCurrentKey()
	}
	// Move first key to the end
	firstKey := m.keys[0]
	m.keys = append(m.keys[1:], firstKey)
	return m.keys[0] // This is now the next key
}

// MockStream implements ChatCompletionStream for testing
type MockStream struct {
	chunks []types.ChatCompletionChunk
	index  int
}

// Next returns the next chunk from the mock stream
func (ms *MockStream) Next() (types.ChatCompletionChunk, error) {
	if ms.index >= len(ms.chunks) {
		return types.ChatCompletionChunk{}, nil
	}
	chunk := ms.chunks[ms.index]
	ms.index++
	return chunk, nil
}

// Close closes the mock stream
func (ms *MockStream) Close() error {
	ms.index = 0
	return nil
}
