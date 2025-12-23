// Package tools provides shared tool conversion utilities for AI provider implementations.
// This consolidates common patterns for converting between universal tool formats
// and provider-specific formats, reducing code duplication across providers.
package tools

import "github.com/cecil-the-coder/ai-provider-kit/pkg/types"

// ToolFormat represents the format used for tool definitions
type ToolFormat string

const (
	// ToolFormatOpenAI is the OpenAI-compatible tool format (used by OpenAI, Cerebras, OpenRouter, Qwen, etc.)
	ToolFormatOpenAI ToolFormat = "openai"

	// ToolFormatAnthropic is the Anthropic-specific tool format
	ToolFormatAnthropic ToolFormat = "anthropic"

	// ToolFormatGemini is the Google Gemini-specific tool format
	ToolFormatGemini ToolFormat = "gemini"

	// ToolFormatOllama is the Ollama-specific tool format (OpenAI-compatible with schema normalization)
	ToolFormatOllama ToolFormat = "ollama"
)

// OpenAIFormat represents the OpenAI tool format structure
type OpenAIFormat struct {
	Type     string            `json:"type"` // Always "function"
	Function OpenAIFunctionDef `json:"function"`
}

// OpenAIFunctionDef represents a function definition in OpenAI format
type OpenAIFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// OpenAIToolCall represents a tool call in OpenAI format
type OpenAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"` // "function"
	Function OpenAIToolCallFunction `json:"function"`
}

// OpenAIToolCallFunction represents a function call in a tool call
type OpenAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ConvertToOpenAIFormat converts universal tools to OpenAI format
// This is the most common format used by OpenAI-compatible providers
func ConvertToOpenAIFormat(tools []types.Tool) []OpenAIFormat {
	result := make([]OpenAIFormat, len(tools))
	for i, tool := range tools {
		result[i] = OpenAIFormat{
			Type: "function",
			Function: OpenAIFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}
	return result
}

// ConvertToOpenAIToolCalls converts universal tool calls to OpenAI format
func ConvertToOpenAIToolCalls(toolCalls []types.ToolCall) []OpenAIToolCall {
	result := make([]OpenAIToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		result[i] = OpenAIToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: OpenAIToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

// ConvertFromOpenAIToolCalls converts OpenAI format tool calls to universal format
func ConvertFromOpenAIToolCalls(toolCalls []OpenAIToolCall) []types.ToolCall {
	universal := make([]types.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		universal[i] = types.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: types.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return universal
}

// ConvertFromOpenAIToProviderFormat converts OpenAI format tools to provider-specific format
// This is useful when you have OpenAI format tools and need to convert to another provider's format
func ConvertFromOpenAIToProviderFormat(openaiTools []OpenAIFormat) []types.Tool {
	universal := make([]types.Tool, len(openaiTools))
	for i, tool := range openaiTools {
		universal[i] = types.Tool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.Parameters,
		}
	}
	return universal
}
