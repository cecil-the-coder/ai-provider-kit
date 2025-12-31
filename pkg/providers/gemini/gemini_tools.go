// Package gemini provides tool calling conversion functions for Google Gemini AI provider.
package gemini

import (
	"encoding/json"
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// unsupportedSchemaKeywords contains JSON Schema keywords that Gemini does not support.
// These will be removed from schemas before sending to the Gemini API.
var unsupportedSchemaKeywords = map[string]bool{
	"$ref":     true,
	"$defs":    true,
	"$schema":  true,
	"$id":      true,
	"const":    true,
	"default":  true,
	"examples": true,
}

// CleanSchemaForGemini removes unsupported JSON Schema keywords from a schema
// before sending it to the Gemini API. It returns a cleaned copy without
// mutating the original schema.
//
// The following keywords are removed:
//   - $ref, $defs, $schema, $id (JSON Schema references/metadata)
//   - const, default, examples (value constraints)
//   - title fields nested inside properties (not supported by Gemini)
//
// The function recursively processes nested objects, arrays, and composition
// keywords (anyOf, allOf, oneOf).
func CleanSchemaForGemini(schema map[string]interface{}) map[string]interface{} {
	return cleanSchemaRecursive(schema, false)
}

// cleanSchemaRecursive is the internal recursive implementation that tracks
// whether we're inside a properties block (where title should be removed).
func cleanSchemaRecursive(schema map[string]interface{}, insideProperties bool) map[string]interface{} {
	if schema == nil {
		return nil
	}

	cleaned := make(map[string]interface{})

	for key, value := range schema {
		// Skip unsupported keywords
		if unsupportedSchemaKeywords[key] {
			continue
		}

		// Skip title when inside properties (nested title not supported)
		if key == "title" && insideProperties {
			continue
		}

		// Handle nested structures
		switch key {
		case "properties":
			if props, ok := value.(map[string]interface{}); ok {
				cleanedProps := make(map[string]interface{})
				for propName, propValue := range props {
					if propMap, ok := propValue.(map[string]interface{}); ok {
						cleanedProps[propName] = cleanSchemaRecursive(propMap, true)
					} else {
						cleanedProps[propName] = propValue
					}
				}
				cleaned[key] = cleanedProps
			} else {
				cleaned[key] = value
			}

		case "items":
			if itemsMap, ok := value.(map[string]interface{}); ok {
				cleaned[key] = cleanSchemaRecursive(itemsMap, insideProperties)
			} else {
				cleaned[key] = value
			}

		case "anyOf", "allOf", "oneOf":
			if arr, ok := value.([]interface{}); ok {
				cleanedArr := make([]interface{}, 0, len(arr))
				for _, item := range arr {
					if itemMap, ok := item.(map[string]interface{}); ok {
						cleanedArr = append(cleanedArr, cleanSchemaRecursive(itemMap, insideProperties))
					} else {
						cleanedArr = append(cleanedArr, item)
					}
				}
				cleaned[key] = cleanedArr
			} else {
				cleaned[key] = value
			}

		case "additionalProperties":
			if addPropsMap, ok := value.(map[string]interface{}); ok {
				cleaned[key] = cleanSchemaRecursive(addPropsMap, insideProperties)
			} else {
				cleaned[key] = value
			}

		default:
			// For other values, check if they're nested objects that might contain schemas
			if nestedMap, ok := value.(map[string]interface{}); ok {
				cleaned[key] = cleanSchemaRecursive(nestedMap, insideProperties)
			} else {
				cleaned[key] = value
			}
		}
	}

	return cleaned
}

// convertToGeminiTools converts universal tools to Gemini's function_declarations format.
// It automatically cleans the input schemas to remove unsupported JSON Schema keywords.
func convertToGeminiTools(tools []types.Tool) []GeminiTool {
	if len(tools) == 0 {
		return nil
	}

	declarations := make([]GeminiFunctionDeclaration, len(tools))
	for i, tool := range tools {
		// Clean the schema before conversion to remove unsupported keywords
		cleanedSchema := CleanSchemaForGemini(tool.InputSchema)
		declarations[i] = GeminiFunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  convertToGeminiSchema(cleanedSchema),
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
