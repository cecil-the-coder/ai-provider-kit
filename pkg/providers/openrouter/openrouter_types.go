// Package openrouter provides an OpenRouter AI provider implementation.
package openrouter

import (
	commontools "github.com/cecil-the-coder/ai-provider-kit/internal/common/tools"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// OpenRouterRequest represents the request payload for OpenRouter API
type OpenRouterRequest struct {
	Model          string                 `json:"model"`
	Messages       []OpenRouterMessage    `json:"messages"`
	Stream         bool                   `json:"stream"`
	HTTPReferer    string                 `json:"http_referer,omitempty"`
	HTTPUserAgent  string                 `json:"x-title,omitempty"`
	Temperature    float64                `json:"temperature,omitempty"`
	MaxTokens      int                    `json:"max_tokens,omitempty"`
	Tools          []OpenRouterTool       `json:"tools,omitempty"`
	ToolChoice     interface{}            `json:"tool_choice,omitempty"`
	ResponseFormat map[string]interface{} `json:"response_format,omitempty"` // For structured outputs
}

// OpenRouterTool represents a tool in the OpenRouter API (OpenAI-compatible format)
type OpenRouterTool struct {
	Type     string                `json:"type"` // Always "function"
	Function OpenRouterFunctionDef `json:"function"`
}

// OpenRouterFunctionDef represents a function definition in the OpenRouter API
type OpenRouterFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// OpenRouterMessage represents a message in the conversation
type OpenRouterMessage struct {
	Role       string               `json:"role"`
	Content    string               `json:"content"`
	ToolCalls  []OpenRouterToolCall `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

// OpenRouterToolCall represents a tool call in the OpenRouter API
type OpenRouterToolCall struct {
	ID       string                     `json:"id"`
	Type     string                     `json:"type"` // "function"
	Function OpenRouterToolCallFunction `json:"function"`
}

// OpenRouterToolCallFunction represents a function call in a tool call
type OpenRouterToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// OpenRouterResponse represents the response from OpenRouter API
type OpenRouterResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []OpenRouterChoice `json:"choices"`
	Usage   OpenRouterUsage    `json:"usage"`
}

// OpenRouterChoice represents a choice in the response
type OpenRouterChoice struct {
	Index        int               `json:"index"`
	Message      OpenRouterMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

// OpenRouterUsage represents token usage information
type OpenRouterUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenRouterErrorResponse represents an error response
type OpenRouterErrorResponse struct {
	Error OpenRouterError `json:"error"`
}

// OpenRouterError represents an error in the response
type OpenRouterError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    int    `json:"code"`
}

// OpenRouterRateLimits represents rate limit information from the /api/v1/key endpoint
type OpenRouterRateLimits struct {
	Limit          *float64 `json:"limit"`           // Credit limit (can be null)
	LimitReset     string   `json:"limit_reset"`     // Reset type for credits
	LimitRemaining *float64 `json:"limit_remaining"` // Remaining credits
	Usage          float64  `json:"usage"`           // Total credits used
	IsFreeTier     bool     `json:"is_free_tier"`    // Whether account is on free tier
	RateLimit      struct {
		RequestsPerMinute int `json:"requests_per_minute"`
		RequestsPerDay    int `json:"requests_per_day"`
	} `json:"rate_limit,omitempty"` // Rate limit information
}

// OpenRouterModelsResponse represents the response from /api/v1/models endpoint
type OpenRouterModelsResponse struct {
	Data []OpenRouterModelData `json:"data"`
}

// OpenRouterModelData represents a model in the models list
type OpenRouterModelData struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Pricing       OpenRouterPricing      `json:"pricing"`
	ContextLength int                    `json:"context_length"`
	Architecture  OpenRouterArchitecture `json:"architecture"`
	TopProvider   OpenRouterTopProvider  `json:"top_provider"`
}

// OpenRouterPricing represents pricing information
type OpenRouterPricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	Request    string `json:"request"`
	Image      string `json:"image"`
}

// OpenRouterArchitecture represents model architecture info
type OpenRouterArchitecture struct {
	Modality     string `json:"modality"`
	TokenizerID  string `json:"tokenizer"`
	InstructType string `json:"instruct_type"`
}

// OpenRouterTopProvider represents top provider info
type OpenRouterTopProvider struct {
	ContextLength       int  `json:"context_length"`
	MaxCompletionTokens int  `json:"max_completion_tokens"`
	IsModerated         bool `json:"is_moderated"`
}

// MockStream implements ChatCompletionStream for testing
type MockStream struct {
	chunks []types.ChatCompletionChunk
	index  int
}

func (ms *MockStream) Next() (types.ChatCompletionChunk, error) {
	if ms.index >= len(ms.chunks) {
		return types.ChatCompletionChunk{}, nil
	}
	chunk := ms.chunks[ms.index]
	ms.index++
	return chunk, nil
}

func (ms *MockStream) Close() error {
	ms.index = 0
	return nil
}

// Helper functions for type conversions

// convertMessagesToOpenRouter converts universal messages to OpenRouter format
func convertMessagesToOpenRouter(messages []types.ChatMessage) []OpenRouterMessage {
	openrouterMessages := make([]OpenRouterMessage, len(messages))
	for i, msg := range messages {
		openrouterMsg := OpenRouterMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Convert tool calls if present
		if len(msg.ToolCalls) > 0 {
			openrouterMsg.ToolCalls = convertToOpenRouterToolCalls(msg.ToolCalls)
		}

		// Include tool call ID for tool response messages
		if msg.ToolCallID != "" {
			openrouterMsg.ToolCallID = msg.ToolCallID
		}

		openrouterMessages[i] = openrouterMsg
	}
	return openrouterMessages
}

// convertToOpenRouterTools converts universal tools to OpenRouter format (OpenAI-compatible) using shared utility
func convertToOpenRouterTools(tools []types.Tool) []OpenRouterTool {
	// Use shared utility for conversion
	sharedTools := commontools.ConvertToOpenAIFormat(tools)

	// Convert to OpenRouter-specific types (same structure, different type names)
	openrouterTools := make([]OpenRouterTool, len(sharedTools))
	for i, st := range sharedTools {
		openrouterTools[i] = OpenRouterTool{
			Type: st.Type,
			Function: OpenRouterFunctionDef{
				Name:        st.Function.Name,
				Description: st.Function.Description,
				Parameters:  st.Function.Parameters,
			},
		}
	}
	return openrouterTools
}

// convertToOpenRouterToolCalls converts universal tool calls to OpenRouter format using shared utility
func convertToOpenRouterToolCalls(toolCalls []types.ToolCall) []OpenRouterToolCall {
	// Use shared utility for conversion
	sharedCalls := commontools.ConvertToOpenAIToolCalls(toolCalls)

	// Convert to OpenRouter-specific types (same structure, different type names)
	openrouterToolCalls := make([]OpenRouterToolCall, len(sharedCalls))
	for i, sc := range sharedCalls {
		openrouterToolCalls[i] = OpenRouterToolCall{
			ID:   sc.ID,
			Type: sc.Type,
			Function: OpenRouterToolCallFunction{
				Name:      sc.Function.Name,
				Arguments: sc.Function.Arguments,
			},
		}
	}
	return openrouterToolCalls
}

// convertOpenRouterToolCallsToUniversal converts OpenRouter tool calls to universal format using shared utility
func convertOpenRouterToolCallsToUniversal(toolCalls []OpenRouterToolCall) []types.ToolCall {
	// Convert to shared format first
	sharedCalls := make([]commontools.OpenAIToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		sharedCalls[i] = commontools.OpenAIToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: commontools.OpenAIToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}

	// Use shared utility for conversion
	return commontools.ConvertFromOpenAIToolCalls(sharedCalls)
}
