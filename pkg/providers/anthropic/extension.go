// Package anthropic provides extension implementation for Anthropic Claude AI provider.
// It handles conversion between standard and provider-specific request/response formats.
package anthropic

import (
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// AnthropicExtension implements CoreProviderExtension for Anthropic Claude
type AnthropicExtension struct {
	*types.BaseExtension
}

// NewAnthropicExtension creates a new Anthropic extension
func NewAnthropicExtension() *AnthropicExtension {
	capabilities := []string{
		"chat",
		"streaming",
		"tool_calling",
		"function_calling",
		"system_messages",
		"temperature",
		"top_p",
		"top_k",
		"max_tokens",
		"stop_sequences",
		"thinking_mode",
		"tool_choice",
		"xml_tool_format",
	}

	return &AnthropicExtension{
		BaseExtension: types.NewBaseExtension(
			"anthropic",
			"1.0.0",
			"Anthropic Claude API extension with full chat completion, streaming, and tool calling support",
			capabilities,
		),
	}
}

// StandardToProvider converts a standard request to Anthropic format
func (e *AnthropicExtension) StandardToProvider(request types.StandardRequest) (interface{}, error) {
	// Create Anthropic request
	anthropicReq := AnthropicRequest{
		Model:     request.Model,
		MaxTokens: request.MaxTokens,
		Stream:    request.Stream,
	}

	// Set default max tokens if not provided
	if anthropicReq.MaxTokens == 0 {
		anthropicReq.MaxTokens = 4096
	}

	// Convert messages
	anthropicReq.Messages = make([]AnthropicMessage, len(request.Messages))
	for i, msg := range request.Messages {
		anthropicReq.Messages[i] = AnthropicMessage{
			Role:    msg.Role,
			Content: convertToAnthropicContent(msg),
		}
	}

	// Convert stop sequences
	if len(request.Stop) > 0 {
		anthropicReq.StopSequences = request.Stop
	}

	// Convert tools if provided
	if len(request.Tools) > 0 {
		anthropicReq.Tools = convertToAnthropicTools(request.Tools)

		// Convert tool choice if specified
		if request.ToolChoice != nil {
			anthropicReq.ToolChoice = convertToAnthropicToolChoice(request.ToolChoice)
		}
	}

	// Handle Anthropic-specific parameters from metadata
	if request.Metadata != nil {
		// Handle top_p
		if topP, ok := request.Metadata["top_p"].(float64); ok {
			anthropicReq.TopP = &topP
		}

		// Handle top_k
		if topK, ok := request.Metadata["top_k"].(int); ok {
			anthropicReq.TopK = &topK
		}

		// Handle system prompt
		if systemPrompt, ok := request.Metadata["system"].(string); ok {
			anthropicReq.System = systemPrompt
		}

		// Handle thinking mode
		if thinking, ok := request.Metadata["thinking"].(bool); ok && thinking {
			if anthropicReq.ToolChoice == nil {
				anthropicReq.ToolChoice = map[string]string{
					"type": "auto",
				}
			}
		}

		// Handle tool format (XML vs JSON)
		if toolFormat, ok := request.Metadata["tool_format"].(string); ok && toolFormat == "xml" {
			// Anthropic uses XML format by default for tool calling
			// This is mainly for documentation purposes - no action needed
			_ = ok // Suppress unused variable warning
		}
	}

	return anthropicReq, nil
}

// ProviderToStandard converts an Anthropic response to standard format
func (e *AnthropicExtension) ProviderToStandard(response interface{}) (*types.StandardResponse, error) {
	anthropicResp, ok := response.(*AnthropicResponse)
	if !ok {
		return nil, fmt.Errorf("response is not an Anthropic response")
	}

	// Convert choices
	choices := make([]types.StandardChoice, 1) // Anthropic returns single choice

	// Extract text content
	var content string
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			content = block.Text
			break
		}
	}

	// Convert tool calls if present
	toolCalls := convertAnthropicContentToToolCalls(anthropicResp.Content)

	message := types.ChatMessage{
		Role:      anthropicResp.Role,
		Content:   content,
		ToolCalls: toolCalls,
	}

	// Determine finish reason
	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	choices[0] = types.StandardChoice{
		Index:        0,
		Message:      message,
		FinishReason: finishReason,
	}

	// Convert usage
	usage := types.Usage{
		PromptTokens:     anthropicResp.Usage.InputTokens,
		CompletionTokens: anthropicResp.Usage.OutputTokens,
		TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
	}

	// Create provider metadata
	providerMetadata := map[string]interface{}{
		"stop_reason":   anthropicResp.StopReason,
		"stop_sequence": anthropicResp.StopSequence,
	}

	return &types.StandardResponse{
		ID:               anthropicResp.ID,
		Model:            anthropicResp.Model,
		Object:           "chat.completion",
		Created:          0, // Anthropic doesn't provide created timestamp
		Choices:          choices,
		Usage:            usage,
		ProviderMetadata: providerMetadata,
	}, nil
}

// ProviderToStandardChunk converts an Anthropic stream chunk to standard format
func (e *AnthropicExtension) ProviderToStandardChunk(chunk interface{}) (*types.StandardStreamChunk, error) {
	anthropicChunk, ok := chunk.(*AnthropicStreamChunk)
	if !ok {
		return nil, fmt.Errorf("chunk is not an Anthropic stream chunk")
	}

	// Convert choices
	choices := make([]types.StandardStreamChoice, 1)

	// Extract content
	var content string
	for _, block := range anthropicChunk.Content {
		if block.Type == "text" {
			content = block.Text
			break
		}
	}

	// Convert tool calls if present
	toolCalls := convertAnthropicContentToToolCalls(anthropicChunk.Content)

	delta := types.ChatMessage{
		Role:      anthropicChunk.Role,
		Content:   content,
		ToolCalls: toolCalls,
	}

	// Determine finish reason
	finishReason := ""
	if anthropicChunk.StopReason != "" {
		finishReason = anthropicChunk.StopReason
	}

	choices[0] = types.StandardStreamChoice{
		Index:        0,
		Delta:        delta,
		FinishReason: finishReason,
	}

	// Convert usage if present
	var usage *types.Usage
	if anthropicChunk.Usage != nil {
		usage = &types.Usage{
			PromptTokens:     anthropicChunk.Usage.InputTokens,
			CompletionTokens: anthropicChunk.Usage.OutputTokens,
			TotalTokens:      anthropicChunk.Usage.InputTokens + anthropicChunk.Usage.OutputTokens,
		}
	}

	// Create provider metadata
	providerMetadata := map[string]interface{}{
		"stop_reason":   anthropicChunk.StopReason,
		"stop_sequence": anthropicChunk.StopSequence,
	}

	return &types.StandardStreamChunk{
		ID:               anthropicChunk.ID,
		Model:            anthropicChunk.Model,
		Object:           "chat.completion.chunk",
		Created:          0, // Anthropic doesn't provide created timestamp
		Choices:          choices,
		Usage:            usage,
		Done:             anthropicChunk.StopReason != "",
		ProviderMetadata: providerMetadata,
	}, nil
}

// ValidateOptions validates Anthropic-specific options
func (e *AnthropicExtension) ValidateOptions(options map[string]interface{}) error {
	// Validate top_p if provided
	if topP, ok := options["top_p"].(float64); ok {
		if topP < 0 || topP > 1 {
			return fmt.Errorf("top_p must be between 0 and 1")
		}
	}

	// Validate top_k if provided
	if topK, ok := options["top_k"].(int); ok {
		if topK < 0 {
			return fmt.Errorf("top_k must be non-negative")
		}
	}

	// Validate max_tokens if provided
	if maxTokens, ok := options["max_tokens"].(int); ok {
		if maxTokens < 1 || maxTokens > 200000 {
			return fmt.Errorf("max_tokens must be between 1 and 200000")
		}
	}

	// Call base validation
	return e.BaseExtension.ValidateOptions(options)
}

// Register the Anthropic extension with the default registry
func init() {
	_ = types.RegisterDefaultExtension(types.ProviderTypeAnthropic, NewAnthropicExtension())
}
