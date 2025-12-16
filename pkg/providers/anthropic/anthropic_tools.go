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
		anthropicTools[i] = AnthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
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
