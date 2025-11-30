package openai

import (
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// OpenAIExtension implements CoreProviderExtension for OpenAI
type OpenAIExtension struct {
	*types.BaseExtension
}

// NewOpenAIExtension creates a new OpenAI extension
func NewOpenAIExtension() *OpenAIExtension {
	capabilities := []string{
		"chat",
		"streaming",
		"tool_calling",
		"function_calling",
		"json_mode",
		"system_messages",
		"temperature",
		"top_p",
		"max_tokens",
		"stop_sequences",
		"seed",
		"parallel_tool_calls",
	}

	return &OpenAIExtension{
		BaseExtension: types.NewBaseExtension(
			"openai",
			"1.0.0",
			"OpenAI API extension with full chat completion, streaming, and tool calling support",
			capabilities,
		),
	}
}

// StandardToProvider converts a standard request to OpenAI format
func (e *OpenAIExtension) StandardToProvider(request types.StandardRequest) (interface{}, error) {
	// Create OpenAI request
	openAIReq := OpenAIRequest{
		Model:       request.Model,
		MaxTokens:   request.MaxTokens,
		Temperature: request.Temperature,
		Stream:      request.Stream,
	}

	// Convert messages
	openAIReq.Messages = make([]OpenAIMessage, len(request.Messages))
	for i, msg := range request.Messages {
		openAIReq.Messages[i] = OpenAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Convert tool calls if present
		if len(msg.ToolCalls) > 0 {
			openAIReq.Messages[i].ToolCalls = convertToOpenAIToolCalls(msg.ToolCalls)
		}

		// Include tool call ID for tool response messages
		if msg.ToolCallID != "" {
			openAIReq.Messages[i].ToolCallID = msg.ToolCallID
		}
	}

	// Convert stop sequences
	if len(request.Stop) > 0 {
		openAIReq.Stop = request.Stop
	}

	// Convert tools if provided
	if len(request.Tools) > 0 {
		openAIReq.Tools = convertToOpenAITools(request.Tools)

		// Convert tool choice if specified
		if request.ToolChoice != nil {
			openAIReq.ToolChoice = convertToOpenAIToolChoice(request.ToolChoice)
		}
	}

	// Handle OpenAI-specific parameters from metadata
	if request.Metadata != nil {
		// Handle top_p
		if topP, ok := request.Metadata["top_p"].(float64); ok {
			openAIReq.TopP = topP
		}

		// Handle seed for reproducible results
		if seed, ok := request.Metadata["seed"].(int); ok {
			openAIReq.Seed = &seed
		}

		// Handle response format (e.g., for JSON mode)
		if responseFormat, ok := request.Metadata["response_format"].(map[string]interface{}); ok {
			openAIReq.ResponseFormat = responseFormat
		}

		// Handle parallel tool calls
		if parallelToolCalls, ok := request.Metadata["parallel_tool_calls"].(bool); ok {
			openAIReq.ParallelToolCalls = &parallelToolCalls
		}
	}

	return openAIReq, nil
}

// ProviderToStandard converts an OpenAI response to standard format
func (e *OpenAIExtension) ProviderToStandard(response interface{}) (*types.StandardResponse, error) {
	openAIResp, ok := response.(*OpenAIResponse)
	if !ok {
		return nil, fmt.Errorf("response is not an OpenAI response")
	}

	// Convert choices
	choices := make([]types.StandardChoice, len(openAIResp.Choices))
	for i, choice := range openAIResp.Choices {
		// Convert message content (handle both string and array formats)
		content := ""
		if contentStr, ok := choice.Message.Content.(string); ok {
			content = contentStr
		}

		message := types.ChatMessage{
			Role:    choice.Message.Role,
			Content: content,
		}

		// Convert tool calls if present
		if len(choice.Message.ToolCalls) > 0 {
			message.ToolCalls = convertOpenAIToolCallsToUniversal(choice.Message.ToolCalls)
		}

		choices[i] = types.StandardChoice{
			Index:        choice.Index,
			Message:      message,
			FinishReason: choice.FinishReason,
		}
	}

	// Convert usage
	usage := types.Usage{
		PromptTokens:     openAIResp.Usage.PromptTokens,
		CompletionTokens: openAIResp.Usage.CompletionTokens,
		TotalTokens:      openAIResp.Usage.TotalTokens,
	}

	// Create provider metadata
	providerMetadata := map[string]interface{}{
		"system_fingerprint": openAIResp.SystemFingerprint,
	}

	return &types.StandardResponse{
		ID:               openAIResp.ID,
		Model:            openAIResp.Model,
		Object:           openAIResp.Object,
		Created:          openAIResp.Created,
		Choices:          choices,
		Usage:            usage,
		ProviderMetadata: providerMetadata,
	}, nil
}

// ProviderToStandardChunk converts an OpenAI stream chunk to standard format
func (e *OpenAIExtension) ProviderToStandardChunk(chunk interface{}) (*types.StandardStreamChunk, error) {
	openAIChunk, ok := chunk.(*OpenAIStreamResponse)
	if !ok {
		return nil, fmt.Errorf("chunk is not an OpenAI stream response")
	}

	// Convert choices
	choices := make([]types.StandardStreamChoice, len(openAIChunk.Choices))
	for i, choice := range openAIChunk.Choices {
		// Convert delta
		delta := types.ChatMessage{
			Role:    choice.Delta.Role,
			Content: choice.Delta.Content,
		}

		// Convert tool calls if present
		if len(choice.Delta.ToolCalls) > 0 {
			delta.ToolCalls = convertOpenAIToolCallsToUniversal(choice.Delta.ToolCalls)
		}

		choices[i] = types.StandardStreamChoice{
			Index:        choice.Index,
			Delta:        delta,
			FinishReason: choice.FinishReason,
		}
	}

	// Convert usage if present
	var usage *types.Usage
	if openAIChunk.Usage != nil {
		usage = &types.Usage{
			PromptTokens:     openAIChunk.Usage.PromptTokens,
			CompletionTokens: openAIChunk.Usage.CompletionTokens,
			TotalTokens:      openAIChunk.Usage.TotalTokens,
		}
	}

	// Create provider metadata
	providerMetadata := map[string]interface{}{
		"system_fingerprint": openAIChunk.SystemFingerprint,
	}

	return &types.StandardStreamChunk{
		ID:               openAIChunk.ID,
		Model:            openAIChunk.Model,
		Object:           openAIChunk.Object,
		Created:          openAIChunk.Created,
		Choices:          choices,
		Usage:            usage,
		Done:             len(choices) > 0 && choices[0].FinishReason != "",
		ProviderMetadata: providerMetadata,
	}, nil
}

// ValidateOptions validates OpenAI-specific options
func (e *OpenAIExtension) ValidateOptions(options map[string]interface{}) error {
	// Validate top_p if provided
	if topP, ok := options["top_p"].(float64); ok {
		if topP < 0 || topP > 1 {
			return fmt.Errorf("top_p must be between 0 and 1")
		}
	}

	// Validate seed if provided
	if seed, ok := options["seed"].(int); ok {
		if seed < 0 {
			return fmt.Errorf("seed must be non-negative")
		}
	}

	// Validate response format if provided
	if responseFormat, ok := options["response_format"].(map[string]interface{}); ok {
		if formatType, ok := responseFormat["type"].(string); ok {
			validTypes := []string{"text", "json_object", "json_schema"}
			valid := false
			for _, validType := range validTypes {
				if formatType == validType {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("response_format.type must be one of: %v", validTypes)
			}
		}
	}

	// Call base validation
	return e.BaseExtension.ValidateOptions(options)
}

// Register the OpenAI extension with the default registry
func init() {
	_ = types.RegisterDefaultExtension(types.ProviderTypeOpenAI, NewOpenAIExtension())
}
