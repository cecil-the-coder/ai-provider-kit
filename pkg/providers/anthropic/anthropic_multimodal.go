// Package anthropic provides multimodal content handling for Anthropic Claude.
package anthropic

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// oauthToolIDPattern matches the required srvtoolu_ pattern for OAuth
var oauthToolIDPattern = regexp.MustCompile(`^srvtoolu_[a-zA-Z0-9_]+$`)

// tooluPattern matches the non-OAuth toolu_ pattern
var tooluPattern = regexp.MustCompile(`^toolu_[a-zA-Z0-9_-]+$`)

// transformToOAuthToolID converts a tool_call ID to srvtulu_ format for Anthropic OAuth compatibility.
//
// Anthropic's OAuth API (anthropic-beta: oauth-2025-04-20) requires tool_use IDs to match pattern:
// ^srvtoolu_[a-zA-Z0-9_]+$
//
// This function ensures compatibility by:
// - Returning srvtoolu_ and toolu_ IDs as-is (both are valid)
// - Converting other formats (call_xxx, tool_xxx, etc.) to srvtoolu_ format
//
// The transformation preserves uniqueness using SHA256 hash of the original ID.
func transformToOAuthToolID(id string) string {
	if id == "" {
		return id
	}

	// srvtoolu_ and toolu_ prefixes are already valid
	if oauthToolIDPattern.MatchString(id) {
		return id
	}
	if tooluPattern.MatchString(id) {
		return id
	}

	// Convert other prefixes to srvtoolu_ format
	// Remove common prefixes
	sanitized := strings.TrimPrefix(id, "call_")
	sanitized = strings.TrimPrefix(sanitized, "tool_")

	// If ID already matches srvtoolu_ pattern after removing prefix, just add the prefix
	if regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(sanitized) {
		return "srvtoolu_" + sanitized
	}

	// For complex IDs (with special chars, etc.), generate a stable hash-based ID
	hash := sha256.Sum256([]byte(id))
	// Encode first 12 bytes of hash to get a base64-like string, then sanitize
	hashStr := base64.URLEncoding.EncodeToString(hash[:12])
	// Remove base64 filler/padding and non-alphanumeric chars
	hashStr = strings.ReplaceAll(hashStr, "=", "")
	hashStr = strings.ReplaceAll(hashStr, "-", "_")
	hashStr = strings.ReplaceAll(hashStr, ".", "_")

	return "srvtoolu_" + hashStr
}

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
		// Transform tool_use ID to srvtoolu_ format for OAuth compatibility
		transformedID := transformToOAuthToolID(part.ID)
		return map[string]interface{}{
			"type":  "tool_use",
			"id":    transformedID,
			"name":  part.Name,
			"input": input,
		}
	case types.ContentTypeToolResult:
		// Transform tool_use ID to srvtoolu_ format for OAuth compatibility
		transformedID := transformToOAuthToolID(part.ToolUseID)
		return AnthropicContentBlock{
			Type:      "tool_result",
			ToolUseID: transformedID,
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

			// Transform tool_use ID to srvtoolu_ format for OAuth compatibility
			transformedID := transformToOAuthToolID(tc.ID)

			// Use map[string]interface{} to ensure "input" is always serialized
			content = append(content, map[string]interface{}{
				"type":  "tool_use",
				"id":    transformedID,
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
		// Transform tool_use ID to srvtoolu_ format for OAuth compatibility
		transformedID := transformToOAuthToolID(msg.ToolCallID)
		return []AnthropicContentBlock{
			{
				Type:      "tool_result",
				ToolUseID: transformedID,
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

			// Transform tool_use ID to srvtoolu_ format for OAuth compatibility
			transformedID := transformToOAuthToolID(tc.ID)

			// Use map[string]interface{} to ensure "input" is always serialized
			content = append(content, map[string]interface{}{
				"type":  "tool_use",
				"id":    transformedID,
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
