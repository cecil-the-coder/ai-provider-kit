// Package anthropic provides multimodal content handling for Anthropic Claude.
package anthropic

import (
	"encoding/json"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// convertContentPartToAnthropic converts a single ContentPart to Anthropic format
func convertContentPartToAnthropic(part types.ContentPart) interface{} {
	switch part.Type {
	case types.ContentTypeText:
		return AnthropicContentBlock{
			Type: "text",
			Text: part.Text,
		}
	case types.ContentTypeImage:
		if part.Source == nil {
			return nil
		}
		// Anthropic format needs source as a map, not using AnthropicContentBlock
		// We'll return a map[string]interface{} instead
		source := map[string]interface{}{
			"type":       part.Source.Type,
			"media_type": part.Source.MediaType,
		}
		if part.Source.Type == types.MediaSourceBase64 {
			source["data"] = part.Source.Data
		} else if part.Source.Type == types.MediaSourceURL {
			source["url"] = part.Source.URL
		}
		return map[string]interface{}{
			"type":   "image",
			"source": source,
		}
	case types.ContentTypeDocument:
		if part.Source == nil {
			return nil
		}
		// Anthropic format needs source as a map, not using AnthropicContentBlock
		source := map[string]interface{}{
			"type":       part.Source.Type,
			"media_type": part.Source.MediaType,
		}
		if part.Source.Type == types.MediaSourceBase64 {
			source["data"] = part.Source.Data
		} else if part.Source.Type == types.MediaSourceURL {
			source["url"] = part.Source.URL
		}
		return map[string]interface{}{
			"type":   "document",
			"source": source,
		}
	case types.ContentTypeToolUse:
		// Use map[string]interface{} instead of AnthropicContentBlock to ensure
		// "input" field is always present (Anthropic API requires it for tool_use)
		input := part.Input
		if input == nil {
			input = map[string]interface{}{}
		}
		return map[string]interface{}{
			"type":  "tool_use",
			"id":    part.ID,
			"name":  part.Name,
			"input": input,
		}
	case types.ContentTypeToolResult:
		return AnthropicContentBlock{
			Type:      "tool_result",
			ToolUseID: part.ToolUseID,
			Content:   part.Content,
		}
	case types.ContentTypeThinking:
		return AnthropicContentBlock{
			Type: "thinking",
			Text: part.Thinking,
		}
	default:
		return nil
	}
}

// convertToAnthropicContent converts a universal chat message to Anthropic content blocks
func convertToAnthropicContent(msg types.ChatMessage) interface{} {
	// Check if message has multimodal Parts (explicit multimodal content)
	if len(msg.Parts) > 0 {
		// New multimodal path: convert Parts to Anthropic format
		var content []interface{}

		for _, part := range msg.Parts {
			if converted := convertContentPartToAnthropic(part); converted != nil {
				content = append(content, converted)
			}
		}

		// Add tool calls if present (backwards compatibility)
		for _, tc := range msg.ToolCalls {
			input := map[string]interface{}{}
			// Ignore JSON unmarshal errors - empty map is used by default
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)

			// Use map[string]interface{} to ensure "input" is always serialized
			content = append(content, map[string]interface{}{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Function.Name,
				"input": input,
			})
		}

		// If only one text part and no tool calls, return as string for simplicity
		if len(content) == 1 && len(msg.ToolCalls) == 0 {
			if block, ok := content[0].(AnthropicContentBlock); ok && block.Type == "text" {
				return block.Text
			}
		}

		return content
	}

	// Legacy handling for messages using Content field (backwards compatibility)
	switch {
	case msg.Role == "tool" || msg.ToolCallID != "":
		// Tool result message - return as content array
		// Check both role=="tool" (OpenAI format) and ToolCallID!="" (Anthropic native format)
		return []AnthropicContentBlock{
			{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   msg.Content,
			},
		}
	case len(msg.ToolCalls) > 0:
		// Assistant message with tool calls
		// Use []interface{} to allow mixing struct and map types
		var content []interface{}

		// Add text content if present
		if msg.Content != "" {
			content = append(content, AnthropicContentBlock{
				Type: "text",
				Text: msg.Content,
			})
		}

		// Add tool use blocks
		for _, tc := range msg.ToolCalls {
			input := map[string]interface{}{}
			// Ignore JSON unmarshal errors - empty map is used by default
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)

			// Use map[string]interface{} to ensure "input" is always serialized
			content = append(content, map[string]interface{}{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Function.Name,
				"input": input,
			})
		}
		return content
	default:
		// Regular text message - return as string
		return msg.Content
	}
}
