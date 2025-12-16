// Package anthropic provides type definitions for Anthropic Claude AI provider.
// It includes request/response structures, streaming types, and model definitions.
package anthropic

// CacheControl defines prompt caching behavior for request blocks
// Supports ephemeral caching with 5-minute (default) or 1-hour TTL
type CacheControl struct {
	Type string `json:"type"`          // "ephemeral" - only supported type
	TTL  string `json:"ttl,omitempty"` // "5m" (default) or "1h"
}

// AnthropicRequest represents the request payload for Anthropic API
type AnthropicRequest struct {
	Model          string             `json:"model"`
	MaxTokens      int                `json:"max_tokens"`
	System         interface{}        `json:"system,omitempty"` // Can be string or []interface{} for OAuth
	Messages       []AnthropicMessage `json:"messages"`
	Stream         bool               `json:"stream,omitempty"`
	Tools          []AnthropicTool    `json:"tools,omitempty"`
	ToolChoice     interface{}        `json:"tool_choice,omitempty"`     // Can be map[string]string or map[string]interface{}
	ResponseFormat interface{}        `json:"response_format,omitempty"` // For structured outputs
	StopSequences  []string           `json:"stop_sequences,omitempty"`
	TopP           *float64           `json:"top_p,omitempty"`
	TopK           *int               `json:"top_k,omitempty"`
}

// AnthropicTool represents a tool definition in the Anthropic API
type AnthropicTool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	InputSchema  map[string]interface{} `json:"input_schema"`
	CacheControl *CacheControl          `json:"cache_control,omitempty"`
}

// AnthropicMessage represents a message in the conversation
type AnthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []AnthropicContent
}

// AnthropicResponse represents the response from Anthropic API
type AnthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []AnthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	Usage        AnthropicUsage          `json:"usage"`
	StopReason   string                  `json:"stop_reason,omitempty"`
	StopSequence string                  `json:"stop_sequence,omitempty"`
}

// AnthropicContentBlock represents a content block in the response or request
type AnthropicContentBlock struct {
	Type         string                 `json:"type"` // "text", "tool_use", "tool_result"
	Text         string                 `json:"text,omitempty"`
	ID           string                 `json:"id,omitempty"`            // for tool_use
	Name         string                 `json:"name,omitempty"`          // for tool_use
	Input        map[string]interface{} `json:"input,omitempty"`         // for tool_use
	ToolUseID    string                 `json:"tool_use_id,omitempty"`   // for tool_result
	Content      interface{}            `json:"content,omitempty"`       // for tool_result, can be string or array
	CacheControl *CacheControl          `json:"cache_control,omitempty"` // for prompt caching
}

// AnthropicUsage represents token usage information
type AnthropicUsage struct {
	InputTokens              int                              `json:"input_tokens"`
	OutputTokens             int                              `json:"output_tokens"`
	CacheCreationInputTokens int                              `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int                              `json:"cache_read_input_tokens,omitempty"`
	CacheCreation            *AnthropicCacheCreationBreakdown `json:"cache_creation,omitempty"`
}

// AnthropicCacheCreationBreakdown provides detailed breakdown of cache writes by TTL
type AnthropicCacheCreationBreakdown struct {
	Ephemeral5mInputTokens int `json:"ephemeral_5m_input_tokens,omitempty"`
	Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens,omitempty"`
}

// AnthropicErrorResponse represents an error response
type AnthropicErrorResponse struct {
	Type  string         `json:"type"`
	Error AnthropicError `json:"error"`
}

// AnthropicError represents an error in the response
type AnthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// AnthropicStreamResponse represents a streaming response chunk from Anthropic API
type AnthropicStreamResponse struct {
	Type         string                       `json:"type"`
	Index        int                          `json:"index,omitempty"`
	Delta        *AnthropicStreamDelta        `json:"delta,omitempty"`
	ContentBlock *AnthropicStreamContentBlock `json:"content_block,omitempty"`
	Message      *AnthropicStreamMessage      `json:"message,omitempty"`
	Usage        *AnthropicUsage              `json:"usage,omitempty"`
}

// AnthropicStreamDelta represents delta content in streaming
type AnthropicStreamDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"` // for tool_use streaming
}

// AnthropicStreamContentBlock represents a content block in streaming
type AnthropicStreamContentBlock struct {
	Type  string                 `json:"type"` // "text" or "tool_use"
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`    // for tool_use
	Name  string                 `json:"name,omitempty"`  // for tool_use
	Input map[string]interface{} `json:"input,omitempty"` // for tool_use
}

// AnthropicStreamMessage represents a message in streaming
type AnthropicStreamMessage struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Role  string `json:"role"`
	Model string `json:"model"`
}

// AnthropicStreamChunk represents a complete streaming chunk from Anthropic API
type AnthropicStreamChunk struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Model        string                  `json:"model"`
	Content      []AnthropicContentBlock `json:"content"`
	Usage        *AnthropicUsage         `json:"usage,omitempty"`
	StopReason   string                  `json:"stop_reason,omitempty"`
	StopSequence string                  `json:"stop_sequence,omitempty"`
}

// AnthropicModelsResponse represents the response from /v1/models endpoint
type AnthropicModelsResponse struct {
	Data    []AnthropicModelData `json:"data"`
	HasMore bool                 `json:"has_more"`
}

// AnthropicModelData represents a model in the models list
type AnthropicModelData struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
	Type        string `json:"type"`
}
