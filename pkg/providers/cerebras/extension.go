// Package cerebras provides extension implementation for Cerebras AI provider.
// It handles conversion between standard and provider-specific request/response formats.
package cerebras

import (
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// CerebrasExtension implements CoreProviderExtension for Cerebras
type CerebrasExtension struct {
	*types.BaseExtension
}

// NewCerebrasExtension creates a new Cerebras extension
func NewCerebrasExtension() *CerebrasExtension {
	capabilities := []string{
		"chat",
		"streaming",
		"tool_calling",
		"function_calling",
		"system_messages",
		"temperature",
		"max_tokens",
		"stop_sequences",
		"fast_inference",
		"high_throughput",
		"code_generation",
	}

	return &CerebrasExtension{
		BaseExtension: types.NewBaseExtension(
			"cerebras",
			"1.0.0",
			"Cerebras API extension with ultra-fast inference and high-throughput capabilities",
			capabilities,
		),
	}
}

// StandardToProvider converts a standard request to Cerebras format
func (e *CerebrasExtension) StandardToProvider(request types.StandardRequest) (interface{}, error) {
	// Create Cerebras request
	cerebrasReq := e.createBaseRequest(request)

	e.setBasicParameters(&cerebrasReq, request)
	e.convertMessages(&cerebrasReq, request.Messages)
	e.convertStopSequences(&cerebrasReq, request.Stop)
	e.convertTools(&cerebrasReq, request.Tools, request.ToolChoice)
	e.applyCerebrasMetadata(&cerebrasReq, request)

	return cerebrasReq, nil
}

// createBaseRequest creates the base Cerebras request structure
func (e *CerebrasExtension) createBaseRequest(request types.StandardRequest) CerebrasRequest {
	return CerebrasRequest{
		Model:  request.Model,
		Stream: request.Stream,
	}
}

// setBasicParameters sets temperature and max tokens with defaults
func (e *CerebrasExtension) setBasicParameters(req *CerebrasRequest, request types.StandardRequest) {
	// Set temperature if provided
	if request.Temperature > 0 {
		req.Temperature = &request.Temperature
	} else {
		// Default temperature for Cerebras
		defaultTemp := 0.6
		req.Temperature = &defaultTemp
	}

	// Set max tokens if provided
	if request.MaxTokens > 0 {
		req.MaxTokens = &request.MaxTokens
	}
}

// convertMessages converts standard messages to Cerebras format
func (e *CerebrasExtension) convertMessages(req *CerebrasRequest, messages []types.ChatMessage) {
	req.Messages = make([]CerebrasMessage, len(messages))
	for i, msg := range messages {
		cerebrasMsg := CerebrasMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Convert tool calls if present
		if len(msg.ToolCalls) > 0 {
			cerebrasMsg.ToolCalls = convertToCerebrasToolCalls(msg.ToolCalls)
		}

		// Include tool call ID for tool response messages
		if msg.ToolCallID != "" {
			cerebrasMsg.ToolCallID = msg.ToolCallID
		}

		req.Messages[i] = cerebrasMsg
	}
}

// convertStopSequences converts stop sequences to Cerebras format
func (e *CerebrasExtension) convertStopSequences(req *CerebrasRequest, stop []string) {
	if len(stop) > 0 {
		req.Stop = stop
	}
}

// convertTools converts tools and tool choice to Cerebras format
func (e *CerebrasExtension) convertTools(req *CerebrasRequest, tools []types.Tool, toolChoice *types.ToolChoice) {
	if len(tools) > 0 {
		req.Tools = convertToCerebrasTools(tools)

		// Convert tool choice if specified
		if toolChoice != nil {
			req.ToolChoice = convertToCerebrasToolChoice(toolChoice)
		}
	}
}

// applyCerebrasMetadata applies Cerebras-specific settings from metadata
func (e *CerebrasExtension) applyCerebrasMetadata(req *CerebrasRequest, request types.StandardRequest) {
	if request.Metadata == nil {
		return
	}

	e.handleFastInferenceMode(req, request)
	e.handleHighThroughputMode(req, request)
	e.handleCodeGenerationMode(req, request)
	e.handleCustomInferenceParams(req, request.Metadata)
}

// handleFastInferenceMode applies fast inference settings
func (e *CerebrasExtension) handleFastInferenceMode(req *CerebrasRequest, request types.StandardRequest) {
	fastMode, ok := request.Metadata["fast_inference"].(bool)
	if !ok || !fastMode {
		return
	}

	// Cerebras-specific fast inference settings
	if request.Temperature == 0 {
		defaultTemp := 0.3 // Lower temperature for faster responses
		req.Temperature = &defaultTemp
	}
}

// handleHighThroughputMode applies high throughput settings
func (e *CerebrasExtension) handleHighThroughputMode(req *CerebrasRequest, request types.StandardRequest) {
	throughputMode, ok := request.Metadata["high_throughput"].(bool)
	if !ok || !throughputMode {
		return
	}

	// Optimize for throughput over latency
	if request.MaxTokens == 0 {
		defaultMaxTokens := 2048 // Smaller max tokens for faster throughput
		req.MaxTokens = &defaultMaxTokens
	}
}

// handleCodeGenerationMode applies code generation settings
func (e *CerebrasExtension) handleCodeGenerationMode(req *CerebrasRequest, request types.StandardRequest) {
	codeGen, ok := request.Metadata["code_generation"].(bool)
	if !ok || !codeGen {
		return
	}

	// Optimize settings for code generation
	if request.Temperature == 0 {
		defaultTemp := 0.2 // Lower temperature for more deterministic code
		req.Temperature = &defaultTemp
	}

	// Add system message for code generation if not already present
	if len(req.Messages) == 0 || req.Messages[0].Role != "system" {
		systemMsg := CerebrasMessage{
			Role:    "system",
			Content: "You are an expert programmer. Generate clean, functional code with minimal explanations.",
		}
		req.Messages = append([]CerebrasMessage{systemMsg}, req.Messages...)
	}
}

// handleCustomInferenceParams applies custom inference parameters
func (e *CerebrasExtension) handleCustomInferenceParams(req *CerebrasRequest, metadata map[string]interface{}) {
	inferenceParams, ok := metadata["inference_params"].(map[string]interface{})
	if !ok {
		return
	}

	// Apply any custom inference parameters
	if temp, ok := inferenceParams["temperature"].(float64); ok {
		req.Temperature = &temp
	}
	if maxTokens, ok := inferenceParams["max_tokens"].(int); ok {
		req.MaxTokens = &maxTokens
	}
}

// ProviderToStandard converts a Cerebras response to standard format
func (e *CerebrasExtension) ProviderToStandard(response interface{}) (*types.StandardResponse, error) {
	cerebrasResp, ok := response.(*CerebrasResponse)
	if !ok {
		return nil, fmt.Errorf("response is not a Cerebras response")
	}

	if len(cerebrasResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in Cerebras API response")
	}

	choice := cerebrasResp.Choices[0]

	// Convert message
	message := types.ChatMessage{
		Role:    choice.Message.Role,
		Content: choice.Message.Content,
	}

	// Convert tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		message.ToolCalls = convertCerebrasToolCallsToUniversal(choice.Message.ToolCalls)
	}

	standardChoice := types.StandardChoice{
		Index:        choice.Index,
		Message:      message,
		FinishReason: choice.FinishReason,
	}

	// Convert usage
	usage := types.Usage{
		PromptTokens:     cerebrasResp.Usage.PromptTokens,
		CompletionTokens: cerebrasResp.Usage.CompletionTokens,
		TotalTokens:      cerebrasResp.Usage.TotalTokens,
	}

	// Create provider metadata
	providerMetadata := map[string]interface{}{
		"inference_speed": "fast", // Cerebras characteristic
		"provider":        "cerebras",
	}

	return &types.StandardResponse{
		ID:               cerebrasResp.ID,
		Model:            cerebrasResp.Model,
		Object:           cerebrasResp.Object,
		Created:          cerebrasResp.Created,
		Choices:          []types.StandardChoice{standardChoice},
		Usage:            usage,
		ProviderMetadata: providerMetadata,
	}, nil
}

// ProviderToStandardChunk converts a Cerebras stream chunk to standard format
func (e *CerebrasExtension) ProviderToStandardChunk(chunk interface{}) (*types.StandardStreamChunk, error) {
	cerebrasChunk, ok := chunk.(*CerebrasResponse)
	if !ok {
		return nil, fmt.Errorf("chunk is not a Cerebras stream response")
	}

	if len(cerebrasChunk.Choices) == 0 {
		return nil, fmt.Errorf("no choices in Cerebras stream chunk")
	}

	choice := cerebrasChunk.Choices[0]

	// Use Delta for streaming (not Message)
	delta := types.ChatMessage{
		Role:    choice.Delta.Role,
		Content: choice.Delta.Content,
	}

	// Convert tool calls if present in the delta
	if len(choice.Delta.ToolCalls) > 0 {
		delta.ToolCalls = convertCerebrasToolCallsToUniversal(choice.Delta.ToolCalls)
	}

	standardChoice := types.StandardStreamChoice{
		Index:        choice.Index,
		Delta:        delta,
		FinishReason: choice.FinishReason,
	}

	// Convert usage if present
	var usage *types.Usage
	if cerebrasChunk.Usage.TotalTokens > 0 {
		usage = &types.Usage{
			PromptTokens:     cerebrasChunk.Usage.PromptTokens,
			CompletionTokens: cerebrasChunk.Usage.CompletionTokens,
			TotalTokens:      cerebrasChunk.Usage.TotalTokens,
		}
	}

	// Create provider metadata
	providerMetadata := map[string]interface{}{
		"inference_speed": "fast", // Cerebras characteristic
		"provider":        "cerebras",
		"reasoning":       choice.Delta.Reasoning, // GLM-4.6 specific field
	}

	done := choice.FinishReason != ""

	return &types.StandardStreamChunk{
		ID:               cerebrasChunk.ID,
		Model:            cerebrasChunk.Model,
		Object:           cerebrasChunk.Object,
		Created:          cerebrasChunk.Created,
		Choices:          []types.StandardStreamChoice{standardChoice},
		Usage:            usage,
		Done:             done,
		ProviderMetadata: providerMetadata,
	}, nil
}

// ValidateOptions validates Cerebras-specific options
func (e *CerebrasExtension) ValidateOptions(options map[string]interface{}) error {
	// Validate temperature if provided
	if temperature, ok := options["temperature"].(float64); ok {
		if temperature < 0 || temperature > 2 {
			return fmt.Errorf("temperature must be between 0 and 2")
		}
	}

	// Validate max_tokens if provided
	if maxTokens, ok := options["max_tokens"].(int); ok {
		if maxTokens < 1 || maxTokens > 131072 {
			return fmt.Errorf("max_tokens must be between 1 and 131072")
		}
	}

	// Validate fast_inference mode
	if fastMode, ok := options["fast_inference"].(bool); ok && fastMode {
		// Fast inference mode specific validations
		if temperature, ok := options["temperature"].(float64); ok && temperature > 1.0 {
			return fmt.Errorf("temperature should be <= 1.0 for fast inference mode")
		}
	}

	// Validate code_generation mode
	if codeGen, ok := options["code_generation"].(bool); ok && codeGen {
		// Code generation specific validations
		if temperature, ok := options["temperature"].(float64); ok && temperature > 0.5 {
			return fmt.Errorf("temperature should be <= 0.5 for code generation mode")
		}
	}

	// Call base validation
	return e.BaseExtension.ValidateOptions(options)
}

// Register the Cerebras extension with the default registry
func init() {
	_ = types.RegisterDefaultExtension(types.ProviderTypeCerebras, NewCerebrasExtension())
}

// Helper functions are defined in the main cerebras.go file
// These extensions reference the existing functions
