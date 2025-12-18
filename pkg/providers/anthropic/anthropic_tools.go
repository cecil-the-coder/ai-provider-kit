// Package anthropic provides tool calling conversion functions for Anthropic Claude.
package anthropic

import (
	"encoding/json"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// convertToAnthropicTools converts universal tools to Anthropic format
func convertToAnthropicTools(tools []types.Tool) []AnthropicTool {
	anthropicTools := make([]AnthropicTool, len(tools))
	for i, tool := range tools {
		anthropicTool := AnthropicTool{
			Name: tool.Name,
		}

		// Check if this is a native tool (has a type set)
		if tool.Type != "" && tool.Type != "custom" {
			// Native tool: preserve type, skip input_schema
			anthropicTool.Type = tool.Type
			// Native tools may have optional description and other fields
			if tool.Description != "" {
				anthropicTool.Description = tool.Description
			}
			// Extract native tool fields from InputSchema if present
			if tool.InputSchema != nil {
				if maxUses, ok := tool.InputSchema["max_uses"].(int); ok {
					anthropicTool.MaxUses = &maxUses
				}
				if allowedDomains, ok := tool.InputSchema["allowed_domains"].([]string); ok {
					anthropicTool.AllowedDomains = allowedDomains
				} else if allowedDomainsInterface, ok := tool.InputSchema["allowed_domains"].([]interface{}); ok {
					// Handle []interface{} conversion
					allowedDomains := make([]string, len(allowedDomainsInterface))
					for j, v := range allowedDomainsInterface {
						if str, ok := v.(string); ok {
							allowedDomains[j] = str
						}
					}
					anthropicTool.AllowedDomains = allowedDomains
				}
				if blockedDomains, ok := tool.InputSchema["blocked_domains"].([]string); ok {
					anthropicTool.BlockedDomains = blockedDomains
				} else if blockedDomainsInterface, ok := tool.InputSchema["blocked_domains"].([]interface{}); ok {
					// Handle []interface{} conversion
					blockedDomains := make([]string, len(blockedDomainsInterface))
					for j, v := range blockedDomainsInterface {
						if str, ok := v.(string); ok {
							blockedDomains[j] = str
						}
					}
					anthropicTool.BlockedDomains = blockedDomains
				}
				if userLocation, ok := tool.InputSchema["user_location"].(map[string]interface{}); ok {
					anthropicTool.UserLocation = userLocation
				}
			}
		} else {
			// Custom tool: include description and input_schema
			anthropicTool.Description = tool.Description
			anthropicTool.InputSchema = tool.InputSchema
		}

		anthropicTools[i] = anthropicTool
	}
	return anthropicTools
}

// convertToAnthropicToolChoice converts universal ToolChoice to Anthropic format
func convertToAnthropicToolChoice(toolChoice *types.ToolChoice) interface{} {
	if toolChoice == nil {
		return nil
	}

	switch toolChoice.Mode {
	case types.ToolChoiceAuto:
		// Anthropic format: {"type": "auto"}
		return map[string]string{
			"type": "auto",
		}
	case types.ToolChoiceRequired:
		// Anthropic format: {"type": "any"}
		return map[string]string{
			"type": "any",
		}
	case types.ToolChoiceNone:
		// Anthropic doesn't have explicit "none" mode
		// Just don't send tools if you don't want them used
		// For compatibility, we'll return auto
		return map[string]string{
			"type": "auto",
		}
	case types.ToolChoiceSpecific:
		// Anthropic format: {"type": "tool", "name": "tool_name"}
		return map[string]interface{}{
			"type": "tool",
			"name": toolChoice.FunctionName,
		}
	default:
		// Default to auto
		return map[string]string{
			"type": "auto",
		}
	}
}

// convertAnthropicContentToToolCalls converts Anthropic content blocks to universal tool calls
func convertAnthropicContentToToolCalls(content []AnthropicContentBlock) []types.ToolCall {
	var toolCalls []types.ToolCall
	for _, block := range content {
		if block.Type == "tool_use" {
			// Convert input map to JSON string for Arguments
			argsJSON, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, types.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      block.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}
	return toolCalls
}

// convertAnthropicResponseToChunk converts Anthropic response to universal chat completion chunk
func convertAnthropicResponseToChunk(response *AnthropicResponse) types.ChatCompletionChunk {
	toolCalls := convertAnthropicContentToToolCalls(response.Content)

	// Extract text content
	var textContent string
	for _, block := range response.Content {
		if block.Type == "text" {
			textContent = block.Text
			break
		}
	}

	chunk := types.ChatCompletionChunk{
		ID:      response.ID,
		Object:  "chat.completion",
		Model:   response.Model,
		Done:    true,
		Content: textContent,
		Usage: types.Usage{
			PromptTokens:             response.Usage.InputTokens,
			CompletionTokens:         response.Usage.OutputTokens,
			TotalTokens:              response.Usage.InputTokens + response.Usage.OutputTokens,
			CacheCreationInputTokens: response.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     response.Usage.CacheReadInputTokens,
		},
		Choices: []types.ChatChoice{
			{
				Index:        0,
				FinishReason: "stop",
				Message: types.ChatMessage{
					Role:      response.Role,
					Content:   textContent,
					ToolCalls: toolCalls,
				},
			},
		},
	}

	// Set proper finish reason for tool calls
	if len(toolCalls) > 0 {
		chunk.Choices[0].FinishReason = "tool_calls"
	}

	return chunk
}
