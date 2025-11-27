package qwen

import (
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// QwenExtension implements CoreProviderExtension for Qwen (Alibaba Cloud)
type QwenExtension struct {
	*types.BaseExtension
}

// NewQwenExtension creates a new Qwen extension
func NewQwenExtension() *QwenExtension {
	capabilities := []string{
		"chat",
		"streaming",
		"tool_calling",
		"function_calling",
		"system_messages",
		"temperature",
		"top_p",
		"max_tokens",
		"stop_sequences",
		"chinese_language",
		"multilingual",
		"code_generation",
		"long_context",
	}

	return &QwenExtension{
		BaseExtension: types.NewBaseExtension(
			"qwen",
			"1.0.0",
			"Qwen (Alibaba Cloud) API extension with multilingual support and tool calling",
			capabilities,
		),
	}
}

// StandardToProvider converts a standard request to Qwen format
func (e *QwenExtension) StandardToProvider(request types.StandardRequest) (interface{}, error) {
	qwenReq := e.createBaseQwenRequest(request)

	e.setDefaultValues(&qwenReq)
	e.convertMessages(&qwenReq, request.Messages)
	e.convertStopSequences(&qwenReq, request.Stop)
	e.convertTools(&qwenReq, request.Tools, request.ToolChoice)
	e.applyQwenMetadata(&qwenReq, request)

	return qwenReq, nil
}

// createBaseQwenRequest creates the base Qwen request structure
func (e *QwenExtension) createBaseQwenRequest(request types.StandardRequest) QwenRequest {
	return QwenRequest{
		Model:       request.Model,
		Stream:      request.Stream,
		MaxTokens:   request.MaxTokens,
		Temperature: request.Temperature,
	}
}

// setDefaultValues sets default values for max tokens and temperature if not provided
func (e *QwenExtension) setDefaultValues(req *QwenRequest) {
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
}

// convertMessages converts standard messages to Qwen message format
func (e *QwenExtension) convertMessages(req *QwenRequest, messages []types.ChatMessage) {
	req.Messages = make([]QwenMessage, len(messages))
	for i, msg := range messages {
		qwenMsg := QwenMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Convert tool calls if present
		if len(msg.ToolCalls) > 0 {
			qwenMsg.ToolCalls = convertToQwenToolCalls(msg.ToolCalls)
		}

		// Include tool call ID for tool response messages
		if msg.ToolCallID != "" {
			qwenMsg.ToolCallID = msg.ToolCallID
		}

		req.Messages[i] = qwenMsg
	}
}

// convertStopSequences converts stop sequences to Qwen format
func (e *QwenExtension) convertStopSequences(req *QwenRequest, stop []string) {
	if len(stop) > 0 {
		req.Stop = stop
	}
}

// convertTools converts tools and tool choice to Qwen format
func (e *QwenExtension) convertTools(req *QwenRequest, tools []types.Tool, toolChoice *types.ToolChoice) {
	if len(tools) > 0 {
		req.Tools = convertToQwenTools(tools)

		// Convert tool choice if specified
		if toolChoice != nil {
			// Tool choice conversion handled in main qwen.go file
			_ = toolChoice // Suppress unused variable warning
		}
	}
}

// applyQwenMetadata applies Qwen-specific settings from metadata
func (e *QwenExtension) applyQwenMetadata(req *QwenRequest, request types.StandardRequest) {
	if request.Metadata == nil {
		return
	}

	e.handleLanguageSettings(req, request)
	e.handleCodeGeneration(req, request)
	e.handleLongContext(req, request)
	e.handleChineseMode(req, request)
	e.handleMultilingualMode(req, request)
	e.handleCustomGenerationParams(req, request.Metadata)
}

// handleLanguageSettings handles language-specific optimizations
func (e *QwenExtension) handleLanguageSettings(req *QwenRequest, request types.StandardRequest) {
	language, ok := request.Metadata["language"].(string)
	if !ok {
		return
	}

	// Qwen-specific language handling
	if language == "chinese" || language == "zh" {
		// Optimize for Chinese language processing
		if request.Temperature == 0 {
			req.Temperature = 0.8 // Slightly higher temperature for Chinese
		}
	}
}

// handleCodeGeneration applies code generation optimizations
func (e *QwenExtension) handleCodeGeneration(req *QwenRequest, request types.StandardRequest) {
	codeGen, ok := request.Metadata["code_generation"].(bool)
	if !ok || !codeGen {
		return
	}

	// Optimize for code generation
	if request.Temperature == 0 {
		req.Temperature = 0.2 // Lower temperature for more deterministic code
	}
	if request.MaxTokens == 0 {
		req.MaxTokens = 8192 // Higher max tokens for code
	}
}

// handleLongContext applies long context optimizations
func (e *QwenExtension) handleLongContext(req *QwenRequest, request types.StandardRequest) {
	longContext, ok := request.Metadata["long_context"].(bool)
	if !ok || !longContext {
		return
	}

	// Optimize for long context
	if request.MaxTokens == 0 {
		req.MaxTokens = 32768 // Maximum context length
	}
}

// handleChineseMode applies Chinese language processing optimizations
func (e *QwenExtension) handleChineseMode(req *QwenRequest, request types.StandardRequest) {
	chineseMode, ok := request.Metadata["chinese_mode"].(bool)
	if !ok || !chineseMode {
		return
	}

	// Optimize for Chinese language processing
	if req.Temperature == 0.7 {
		req.Temperature = 0.8 // Better for Chinese text generation
	}
}

// handleMultilingualMode applies multilingual processing settings
func (e *QwenExtension) handleMultilingualMode(req *QwenRequest, request types.StandardRequest) {
	multilingual, ok := request.Metadata["multilingual"].(bool)
	if !ok || !multilingual {
		return
	}

	// Settings for multilingual processing
	if req.Temperature == 0 {
		req.Temperature = 0.6
	}
}

// handleCustomGenerationParams applies custom generation parameters
func (e *QwenExtension) handleCustomGenerationParams(req *QwenRequest, metadata map[string]interface{}) {
	genParams, ok := metadata["generation_params"].(map[string]interface{})
	if !ok {
		return
	}

	// Apply any custom generation parameters
	if temp, ok := genParams["temperature"].(float64); ok {
		req.Temperature = temp
	}
	if maxTokens, ok := genParams["max_tokens"].(int); ok {
		req.MaxTokens = maxTokens
	}
	if _, hasTopP := genParams["top_p"].(float64); hasTopP {
		// Qwen-specific top_p handling
		_ = hasTopP // suppress unused variable warning
	}
}

// ProviderToStandard converts a Qwen response to standard format
func (e *QwenExtension) ProviderToStandard(response interface{}) (*types.StandardResponse, error) {
	qwenResp, ok := response.(*QwenResponse)
	if !ok {
		return nil, fmt.Errorf("response is not a Qwen response")
	}

	if len(qwenResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in Qwen API response")
	}

	choice := qwenResp.Choices[0]

	// Convert message
	message := types.ChatMessage{
		Role:    choice.Message.Role,
		Content: choice.Message.Content,
	}

	// Convert tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		message.ToolCalls = convertQwenToolCallsToUniversal(choice.Message.ToolCalls)
	}

	standardChoice := types.StandardChoice{
		Index:        choice.Index,
		Message:      message,
		FinishReason: choice.FinishReason,
	}

	// Convert usage
	usage := types.Usage{
		PromptTokens:     qwenResp.Usage.PromptTokens,
		CompletionTokens: qwenResp.Usage.CompletionTokens,
		TotalTokens:      qwenResp.Usage.TotalTokens,
	}

	// Create provider metadata
	providerMetadata := map[string]interface{}{
		"provider":        "qwen",
		"multilingual":    true,
		"chinese_support": true,
	}

	return &types.StandardResponse{
		ID:               qwenResp.ID,
		Model:            qwenResp.Model,
		Object:           qwenResp.Object,
		Created:          qwenResp.Created,
		Choices:          []types.StandardChoice{standardChoice},
		Usage:            usage,
		ProviderMetadata: providerMetadata,
	}, nil
}

// ProviderToStandardChunk converts a Qwen stream chunk to standard format
func (e *QwenExtension) ProviderToStandardChunk(chunk interface{}) (*types.StandardStreamChunk, error) {
	qwenChunk, ok := chunk.(*QwenResponse)
	if !ok {
		return nil, fmt.Errorf("chunk is not a Qwen stream response")
	}

	if len(qwenChunk.Choices) == 0 {
		return nil, fmt.Errorf("no choices in Qwen stream chunk")
	}

	choice := qwenChunk.Choices[0]

	// Use Delta for streaming (not Message)
	delta := types.ChatMessage{
		Role:    choice.Delta.Role,
		Content: choice.Delta.Content,
	}

	// Convert tool calls if present in the delta
	if len(choice.Delta.ToolCalls) > 0 {
		delta.ToolCalls = convertQwenToolCallsToUniversal(choice.Delta.ToolCalls)
	}

	standardChoice := types.StandardStreamChoice{
		Index:        choice.Index,
		Delta:        delta,
		FinishReason: choice.FinishReason,
	}

	// Convert usage if present
	var usage *types.Usage
	if qwenChunk.Usage.TotalTokens > 0 {
		usage = &types.Usage{
			PromptTokens:     qwenChunk.Usage.PromptTokens,
			CompletionTokens: qwenChunk.Usage.CompletionTokens,
			TotalTokens:      qwenChunk.Usage.TotalTokens,
		}
	}

	// Create provider metadata
	providerMetadata := map[string]interface{}{
		"provider":        "qwen",
		"multilingual":    true,
		"chinese_support": true,
	}

	done := choice.FinishReason != ""

	return &types.StandardStreamChunk{
		ID:               qwenChunk.ID,
		Model:            qwenChunk.Model,
		Object:           qwenChunk.Object,
		Created:          qwenChunk.Created,
		Choices:          []types.StandardStreamChoice{standardChoice},
		Usage:            usage,
		Done:             done,
		ProviderMetadata: providerMetadata,
	}, nil
}

// ValidateOptions validates Qwen-specific options
func (e *QwenExtension) ValidateOptions(options map[string]interface{}) error {
	// Validate basic parameters
	if err := e.validateBasicParameters(options); err != nil {
		return err
	}

	// Validate language settings
	if err := e.validateLanguageSettings(options); err != nil {
		return err
	}

	// Validate mode-specific settings
	if err := e.validateModeSpecificSettings(options); err != nil {
		return err
	}

	// Call base validation
	return e.BaseExtension.ValidateOptions(options)
}

// validateBasicParameters validates temperature and max_tokens
func (e *QwenExtension) validateBasicParameters(options map[string]interface{}) error {
	// Validate temperature if provided
	if temperature, ok := options["temperature"].(float64); ok {
		if temperature < 0 || temperature > 2 {
			return fmt.Errorf("temperature must be between 0 and 2")
		}
	}

	// Validate max_tokens if provided
	if maxTokens, ok := options["max_tokens"].(int); ok {
		if maxTokens < 1 || maxTokens > 32768 {
			return fmt.Errorf("max_tokens must be between 1 and 32768")
		}
	}

	return nil
}

// validateLanguageSettings validates language-related options
func (e *QwenExtension) validateLanguageSettings(options map[string]interface{}) error {
	if language, ok := options["language"].(string); ok {
		validLanguages := []string{"chinese", "zh", "english", "en", "multilingual"}
		valid := false
		for _, validLang := range validLanguages {
			if language == validLang {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("language must be one of: %v", validLanguages)
		}
	}

	return nil
}

// validateModeSpecificSettings validates mode-specific options
func (e *QwenExtension) validateModeSpecificSettings(options map[string]interface{}) error {
	// Validate chinese_mode
	if chineseMode, ok := options["chinese_mode"].(bool); ok && chineseMode {
		if temperature, ok := options["temperature"].(float64); ok && temperature > 1.5 {
			return fmt.Errorf("temperature should be <= 1.5 for chinese mode")
		}
	}

	// Validate code_generation mode
	if codeGen, ok := options["code_generation"].(bool); ok && codeGen {
		if temperature, ok := options["temperature"].(float64); ok && temperature > 0.5 {
			return fmt.Errorf("temperature should be <= 0.5 for code generation mode")
		}
	}

	// Validate long_context mode
	if longContext, ok := options["long_context"].(bool); ok && longContext {
		if maxTokens, ok := options["max_tokens"].(int); ok && maxTokens < 8192 {
			return fmt.Errorf("max_tokens should be >= 8192 for long context mode")
		}
	}

	return nil
}

// Register the Qwen extension with the default registry
func init() {
	_ = types.RegisterDefaultExtension(types.ProviderTypeQwen, NewQwenExtension())
}

// Helper functions are defined in the main qwen.go file
// These extensions reference the existing functions
