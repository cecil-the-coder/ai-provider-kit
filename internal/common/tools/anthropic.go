// Package tools provides Anthropic-specific tool conversion utilities.
package tools

import (
	"encoding/json"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// AnthropicTool represents a tool in Anthropic's format
type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
	Type        string                 `json:"type,omitempty"` // For native tools (e.g., "web_search_20250305")

	// Native tool fields (for Anthropic's built-in tools)
	MaxUses        *int                   `json:"max_uses,omitempty"`
	AllowedDomains []string               `json:"allowed_domains,omitempty"`
	BlockedDomains []string               `json:"blocked_domains,omitempty"`
	UserLocation   map[string]interface{} `json:"user_location,omitempty"`
}

// AnthropicContentBlock represents a content block in Anthropic's format
type AnthropicContentBlock struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
	Text  string                 `json:"text,omitempty"`
}

// ConvertToAnthropicFormat converts universal tools to Anthropic format
// Handles both custom tools and Anthropic's native tools
func ConvertToAnthropicFormat(tools []types.Tool) []AnthropicTool {
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
				extractNativeToolFields(tool.InputSchema, &anthropicTool)
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

// extractNativeToolFields extracts Anthropic-native tool fields from InputSchema
func extractNativeToolFields(inputSchema map[string]interface{}, tool *AnthropicTool) {
	if maxUses, ok := inputSchema["max_uses"].(int); ok {
		tool.MaxUses = &maxUses
	}
	if allowedDomains, ok := inputSchema["allowed_domains"].([]string); ok {
		tool.AllowedDomains = allowedDomains
	} else if allowedDomainsInterface, ok := inputSchema["allowed_domains"].([]interface{}); ok {
		// Handle []interface{} conversion
		allowedDomains := make([]string, len(allowedDomainsInterface))
		for j, v := range allowedDomainsInterface {
			if str, ok := v.(string); ok {
				allowedDomains[j] = str
			}
		}
		tool.AllowedDomains = allowedDomains
	}
	if blockedDomains, ok := inputSchema["blocked_domains"].([]string); ok {
		tool.BlockedDomains = blockedDomains
	} else if blockedDomainsInterface, ok := inputSchema["blocked_domains"].([]interface{}); ok {
		// Handle []interface{} conversion
		blockedDomains := make([]string, len(blockedDomainsInterface))
		for j, v := range blockedDomainsInterface {
			if str, ok := v.(string); ok {
				blockedDomains[j] = str
			}
		}
		tool.BlockedDomains = blockedDomains
	}
	if userLocation, ok := inputSchema["user_location"].(map[string]interface{}); ok {
		tool.UserLocation = userLocation
	}
}

// ConvertAnthropicContentToToolCalls converts Anthropic content blocks to universal tool calls
func ConvertAnthropicContentToToolCalls(content []AnthropicContentBlock) []types.ToolCall {
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
