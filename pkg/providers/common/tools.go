// Package common provides shared utilities for AI provider implementations.
// This file contains tool conversion functions for OpenAI-compatible providers.
package common

import "github.com/cecil-the-coder/ai-provider-kit/pkg/types"

// OpenAICompatibleTool represents a tool in OpenAI-compatible API format
// Used by OpenAI, Qwen, and other OpenAI-compatible providers
type OpenAICompatibleTool struct {
	Type     string                        `json:"type"` // Always "function"
	Function OpenAICompatibleFunctionDef `json:"function"`
}

// OpenAICompatibleFunctionDef represents a function definition in OpenAI-compatible format
type OpenAICompatibleFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// OpenAICompatibleToolCall represents a tool call in OpenAI-compatible format
type OpenAICompatibleToolCall struct {
	ID       string                              `json:"id"`
	Type     string                              `json:"type"` // "function"
	Function OpenAICompatibleToolCallFunction `json:"function"`
}

// OpenAICompatibleToolCallFunction represents a function call in a tool call
type OpenAICompatibleToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ConvertToOpenAICompatibleTools converts universal tools to OpenAI-compatible format
// This function can be used by any provider that uses OpenAI's tool format (OpenAI, Qwen, etc.)
func ConvertToOpenAICompatibleTools(tools []types.Tool) []OpenAICompatibleTool {
	result := make([]OpenAICompatibleTool, len(tools))
	for i, tool := range tools {
		result[i] = OpenAICompatibleTool{
			Type: "function",
			Function: OpenAICompatibleFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}
	return result
}

// ConvertToOpenAICompatibleToolCalls converts universal tool calls to OpenAI-compatible format
// This function can be used by any provider that uses OpenAI's tool call format (OpenAI, Qwen, etc.)
func ConvertToOpenAICompatibleToolCalls(toolCalls []types.ToolCall) []OpenAICompatibleToolCall {
	result := make([]OpenAICompatibleToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		result[i] = OpenAICompatibleToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: OpenAICompatibleToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

// ConvertOpenAICompatibleToolCallsToUniversal converts OpenAI-compatible tool calls to universal format
// This function can be used by any provider that uses OpenAI's tool call format (OpenAI, Qwen, etc.)
func ConvertOpenAICompatibleToolCallsToUniversal(toolCalls []OpenAICompatibleToolCall) []types.ToolCall {
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
