// Package gemini provides tool calling conversion functions for Google Gemini AI provider.
package gemini

import (
	"encoding/json"
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// convertToGeminiTools converts universal tools to Gemini's function_declarations format
func convertToGeminiTools(tools []types.Tool) []GeminiTool {
	if len(tools) == 0 {
		return nil
	}

	declarations := make([]GeminiFunctionDeclaration, len(tools))
	for i, tool := range tools {
		declarations[i] = GeminiFunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  convertToGeminiSchema(tool.InputSchema),
		}
	}

	return []GeminiTool{
		{
			FunctionDeclarations: declarations,
		},
	}
}

// convertToGeminiSchema converts a JSON schema to Gemini's schema format
func convertToGeminiSchema(inputSchema map[string]interface{}) GeminiSchema {
	schema := GeminiSchema{
		Type:       "object",
		Properties: make(map[string]GeminiProperty),
	}

	// Extract type if present
	if schemaType, ok := inputSchema["type"].(string); ok {
		schema.Type = schemaType
	}

	// Extract properties if present
	if props, ok := inputSchema["properties"].(map[string]interface{}); ok {
		for propName, propValue := range props {
			if propMap, ok := propValue.(map[string]interface{}); ok {
				property := GeminiProperty{}

				// Extract type
				if propType, ok := propMap["type"].(string); ok {
					property.Type = propType
				}

				// Extract description
				if desc, ok := propMap["description"].(string); ok {
					property.Description = desc
				}

				// Extract enum if present
				if enumValue, ok := propMap["enum"]; ok {
					if enumSlice, ok := enumValue.([]interface{}); ok {
						property.Enum = make([]string, len(enumSlice))
						for i, v := range enumSlice {
							if strVal, ok := v.(string); ok {
								property.Enum[i] = strVal
							}
						}
					}
				}

				schema.Properties[propName] = property
			}
		}
	}

	// Extract required fields if present
	if required, ok := inputSchema["required"].([]interface{}); ok {
		schema.Required = make([]string, len(required))
		for i, r := range required {
			if strVal, ok := r.(string); ok {
				schema.Required[i] = strVal
			}
		}
	}

	return schema
}

// convertGeminiFunctionCallsToUniversal converts Gemini function calls to universal format
func convertGeminiFunctionCallsToUniversal(parts []Part) []types.ToolCall {
	var toolCalls []types.ToolCall
	callIndex := 0

	for _, part := range parts {
		if part.FunctionCall != nil {
			// Convert arguments map to JSON string
			argsJSON, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				continue
			}

			toolCall := types.ToolCall{
				ID:   fmt.Sprintf("call_%d", callIndex),
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      part.FunctionCall.Name,
					Arguments: string(argsJSON),
				},
			}
			toolCalls = append(toolCalls, toolCall)
			callIndex++
		}
	}

	return toolCalls
}

// convertUniversalToolCallsToGeminiParts converts universal tool calls to Gemini parts
func convertUniversalToolCallsToGeminiParts(toolCalls []types.ToolCall) []Part {
	parts := make([]Part, len(toolCalls))
	for i, tc := range toolCalls {
		// Parse arguments JSON string to map
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			// If parsing fails, use empty map
			args = make(map[string]interface{})
		}

		parts[i] = Part{
			FunctionCall: &GeminiFunctionCall{
				Name: tc.Function.Name,
				Args: args,
			},
		}
	}
	return parts
}
