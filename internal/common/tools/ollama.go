// Package tools provides Ollama-specific tool conversion utilities.
// Ollama uses OpenAI-compatible format but requires JSON schema normalization
// for compatibility with its validation.
package tools

import (
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// OllamaTool represents a tool in Ollama's format (OpenAI-compatible)
type OllamaTool struct {
	Type     string            `json:"type"` // Always "function"
	Function OllamaFunctionDef `json:"function"`
}

// OllamaFunctionDef represents a function definition in Ollama's format
type OllamaFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// OllamaToolCall represents a tool call in Ollama's format (OpenAI-compatible)
type OllamaToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"` // "function"
	Function OllamaToolCallFunction `json:"function"`
}

// OllamaToolCallFunction represents a function call in a tool call
type OllamaToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ConvertToOllamaFormat converts universal tools to Ollama format with schema normalization
// Ollama requires that JSON schema "type" fields be strings, not arrays
func ConvertToOllamaFormat(tools []types.Tool) []OllamaTool {
	result := make([]OllamaTool, len(tools))
	for i, tool := range tools {
		// Normalize the input schema to ensure compatibility with Ollama
		// This converts array-typed "type" fields (e.g., ["string", "null"]) to strings
		normalizedSchema := NormalizeJSONSchema(tool.InputSchema)

		result[i] = OllamaTool{
			Type: "function",
			Function: OllamaFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  normalizedSchema,
			},
		}
	}
	return result
}

// ConvertToOllamaToolCalls converts universal tool calls to Ollama format
func ConvertToOllamaToolCalls(toolCalls []types.ToolCall) []OllamaToolCall {
	result := make([]OllamaToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		result[i] = OllamaToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: OllamaToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

// ConvertFromOllamaToolCalls converts Ollama tool calls to universal format
func ConvertFromOllamaToolCalls(toolCalls []OllamaToolCall) []types.ToolCall {
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

// NormalizeJSONSchema normalizes a JSON schema for Ollama compatibility
// Converts array-typed "type" fields to single string types
// For example: {"type": ["string", "null"]} becomes {"type": "string"}
func NormalizeJSONSchema(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		return nil
	}

	normalized := make(map[string]interface{})
	for key, value := range schema {
		switch key {
		case "type":
			// Handle type field - convert arrays to strings
			normalized[key] = normalizeTypeField(value)
		case "properties":
			// Recursively normalize properties
			if props, ok := value.(map[string]interface{}); ok {
				normalized[key] = normalizeProperties(props)
			} else {
				normalized[key] = value
			}
		case "items":
			// Recursively normalize items (for array types)
			if items, ok := value.(map[string]interface{}); ok {
				normalized[key] = NormalizeJSONSchema(items)
			} else {
				normalized[key] = value
			}
		default:
			// Copy other fields as-is
			normalized[key] = value
		}
	}

	return normalized
}

// normalizeTypeField converts array types to string types
func normalizeTypeField(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return v
	case []interface{}:
		// For array types, use the first non-null type
		for _, item := range v {
			if str, ok := item.(string); ok && str != "null" {
				return str
			}
		}
		// If all are null or empty, return "string" as default
		return "string"
	case []string:
		// For typed string arrays, use the first non-null type
		for _, item := range v {
			if item != "null" {
				return item
			}
		}
		return "string"
	default:
		return "string" // Default fallback
	}
}

// normalizeProperties normalizes all properties in a schema
func normalizeProperties(props map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{})
	for key, value := range props {
		if propSchema, ok := value.(map[string]interface{}); ok {
			normalized[key] = NormalizeJSONSchema(propSchema)
		} else {
			normalized[key] = value
		}
	}
	return normalized
}
