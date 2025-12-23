// Package tools provides Gemini-specific tool conversion utilities.
package tools

import (
	"encoding/json"
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// GeminiTool represents a tool in Gemini's format
type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"function_declarations,omitempty"`
}

// GeminiFunctionDeclaration represents a function declaration in Gemini's format
type GeminiFunctionDeclaration struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Parameters  GeminiSchema `json:"parameters"`
}

// GeminiSchema represents JSON Schema in Gemini's format
type GeminiSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]GeminiProperty `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}

// GeminiProperty represents a property in Gemini's schema format
type GeminiProperty struct {
	Type        string   `json:"type,omitempty"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// GeminiPart represents a part in Gemini's content format
type GeminiPart struct {
	Text         string              `json:"text,omitempty"`
	FunctionCall *GeminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResp *GeminiFunctionResp `json:"functionResponse,omitempty"`
}

// GeminiFunctionCall represents a function call in Gemini's format
type GeminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// GeminiFunctionResp represents a function response in Gemini's format
type GeminiFunctionResp struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

// ConvertToGeminiFormat converts universal tools to Gemini's function_declarations format
func ConvertToGeminiFormat(tools []types.Tool) []GeminiTool {
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

// ConvertGeminiFunctionCallsToUniversal converts Gemini function calls to universal format
func ConvertGeminiFunctionCallsToUniversal(parts []GeminiPart) []types.ToolCall {
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

// ConvertUniversalToolCallsToGeminiParts converts universal tool calls to Gemini parts
func ConvertUniversalToolCallsToGeminiParts(toolCalls []types.ToolCall) []GeminiPart {
	parts := make([]GeminiPart, len(toolCalls))
	for i, tc := range toolCalls {
		// Parse arguments JSON string to map
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			// If parsing fails, use empty map
			args = make(map[string]interface{})
		}

		parts[i] = GeminiPart{
			FunctionCall: &GeminiFunctionCall{
				Name: tc.Function.Name,
				Args: args,
			},
		}
	}
	return parts
}
