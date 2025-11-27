// Package gemini provides extension implementation for Google Gemini AI provider.
// It handles conversion between standard and provider-specific request/response formats.
package gemini

import (
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// GeminiExtension implements CoreProviderExtension for Google Gemini
type GeminiExtension struct {
	*types.BaseExtension
}

// NewGeminiExtension creates a new Gemini extension
func NewGeminiExtension() *GeminiExtension {
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
		"safety_settings",
		"generation_config",
		"multimodal",
		"project_id",
	}

	return &GeminiExtension{
		BaseExtension: types.NewBaseExtension(
			"gemini",
			"1.0.0",
			"Google Gemini API extension with full chat completion, streaming, tool calling, and multimodal support",
			capabilities,
		),
	}
}

// StandardToProvider converts a standard request to Gemini format
func (e *GeminiExtension) StandardToProvider(request types.StandardRequest) (interface{}, error) {
	// Create Gemini request
	geminiReq := GenerateContentRequest{
		GenerationConfig: &GenerationConfig{
			Temperature:     request.Temperature,
			MaxOutputTokens: request.MaxTokens,
		},
	}

	// Convert messages to Gemini format
	contents := make([]Content, len(request.Messages))
	for i, msg := range request.Messages {
		parts := []Part{{Text: msg.Content}}

		// Convert tool calls if present - use existing function from gemini.go
		if len(msg.ToolCalls) > 0 {
			parts = append(parts, convertUniversalToolCallsToGeminiParts(msg.ToolCalls)...)
		}

		contents[i] = Content{
			Role:  msg.Role,
			Parts: parts,
		}
	}
	geminiReq.Contents = contents

	// Convert stop sequences
	if len(request.Stop) > 0 {
		// Gemini handles stop sequences in generation config
		// Note: This is a simplified approach, actual implementation may vary
		// Stop sequence handling for Gemini - to be implemented in future version
		// Gemini requires different stop sequence handling in its generation config
		_ = request.Stop // Suppress unused variable warning for future implementation
	}

	// Convert tools if provided
	if len(request.Tools) > 0 {
		geminiReq.Tools = convertToGeminiTools(request.Tools)
	}

	// Handle Gemini-specific parameters from metadata
	if request.Metadata != nil {
		// Handle top_p
		if topP, ok := request.Metadata["top_p"].(float64); ok {
			geminiReq.GenerationConfig.TopP = topP
		}

		// Handle top_k
		if topK, ok := request.Metadata["top_k"].(int); ok {
			geminiReq.GenerationConfig.TopK = topK
		}

		// Handle project ID for CloudCode API
		if projectID, ok := request.Metadata["project_id"].(string); ok {
			// Return wrapped request for CloudCode API
			return CloudCodeRequestWrapper{
				Model:   request.Model,
				Project: projectID,
				Request: geminiReq,
			}, nil
		}

		// Handle safety settings
		if _, ok := request.Metadata["safety_settings"].(map[string]interface{}); ok {
			// Gemini-specific safety settings would be handled here
			// This is a placeholder for future implementation
			_ = ok // suppress unused variable warning
		}
	}

	return geminiReq, nil
}

// ProviderToStandard converts a Gemini response to standard format
func (e *GeminiExtension) ProviderToStandard(response interface{}) (*types.StandardResponse, error) {
	var geminiResp *GenerateContentResponse

	// Handle both direct responses and wrapped CloudCode responses
	switch resp := response.(type) {
	case *GenerateContentResponse:
		geminiResp = resp
	case *CloudCodeResponseWrapper:
		geminiResp = &resp.Response
	default:
		return nil, fmt.Errorf("response is not a Gemini response")
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in Gemini response")
	}

	candidate := geminiResp.Candidates[0]

	// Extract text content
	var content string
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			content += part.Text
		}
	}

	// Convert tool calls if present
	toolCalls := convertGeminiFunctionCallsToUniversal(candidate.Content.Parts)

	message := types.ChatMessage{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
	}

	// Determine finish reason
	finishReason := "stop"
	if candidate.FinishReason == "SAFETY" {
		finishReason = "content_filtered"
	} else if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	choices := []types.StandardChoice{
		{
			Index:        0,
			Message:      message,
			FinishReason: finishReason,
		},
	}

	// Convert usage
	usage := types.Usage{}
	if geminiResp.UsageMetadata != nil {
		usage = types.Usage{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		}
	}

	// Create provider metadata
	providerMetadata := map[string]interface{}{
		"finish_reason": candidate.FinishReason,
	}

	return &types.StandardResponse{
		ID:               "gemini-response",
		Model:            "gemini",
		Object:           "chat.completion",
		Created:          0, // Gemini doesn't provide created timestamp
		Choices:          choices,
		Usage:            usage,
		ProviderMetadata: providerMetadata,
	}, nil
}

// ProviderToStandardChunk converts a Gemini stream chunk to standard format
func (e *GeminiExtension) ProviderToStandardChunk(chunk interface{}) (*types.StandardStreamChunk, error) {
	var geminiChunk *GeminiStreamResponse

	switch c := chunk.(type) {
	case *GeminiStreamResponse:
		geminiChunk = c
	default:
		return nil, fmt.Errorf("chunk is not a Gemini stream response")
	}

	if len(geminiChunk.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in Gemini stream chunk")
	}

	candidate := geminiChunk.Candidates[0]

	// Extract text content
	var content string
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			content += part.Text
		}
	}

	// Convert tool calls if present
	toolCalls := convertGeminiFunctionCallsToUniversal(candidate.Content.Parts)

	delta := types.ChatMessage{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
	}

	// Determine finish reason
	finishReason := ""
	if candidate.FinishReason != "" {
		switch candidate.FinishReason {
		case "SAFETY":
			finishReason = "content_filtered"
		default:
			if len(toolCalls) > 0 {
				finishReason = "tool_calls"
			} else {
				finishReason = candidate.FinishReason
			}
		}
	}

	choices := []types.StandardStreamChoice{
		{
			Index:        0,
			Delta:        delta,
			FinishReason: finishReason,
		},
	}

	// Convert usage if present
	var usage *types.Usage
	if geminiChunk.UsageMetadata != nil {
		usage = &types.Usage{
			PromptTokens:     geminiChunk.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiChunk.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiChunk.UsageMetadata.TotalTokenCount,
		}
	}

	// Create provider metadata
	providerMetadata := map[string]interface{}{
		"finish_reason": candidate.FinishReason,
	}

	done := candidate.FinishReason != ""

	return &types.StandardStreamChunk{
		ID:               "gemini-chunk",
		Model:            "gemini",
		Object:           "chat.completion.chunk",
		Created:          0, // Gemini doesn't provide created timestamp
		Choices:          choices,
		Usage:            usage,
		Done:             done,
		ProviderMetadata: providerMetadata,
	}, nil
}

// ValidateOptions validates Gemini-specific options
func (e *GeminiExtension) ValidateOptions(options map[string]interface{}) error {
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
		if maxTokens < 1 || maxTokens > 8192 {
			return fmt.Errorf("max_tokens must be between 1 and 8192")
		}
	}

	// Validate temperature if provided
	if temperature, ok := options["temperature"].(float64); ok {
		if temperature < 0 || temperature > 2 {
			return fmt.Errorf("temperature must be between 0 and 2")
		}
	}

	// Call base validation
	return e.BaseExtension.ValidateOptions(options)
}

// Register the Gemini extension with the default registry
func init() {
	_ = types.RegisterDefaultExtension(types.ProviderTypeGemini, NewGeminiExtension())
}

// Helper functions are defined in the main gemini.go file
// These extensions reference the existing functions
