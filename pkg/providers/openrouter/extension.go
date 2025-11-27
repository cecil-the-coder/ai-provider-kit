// Package openrouter provides an OpenRouter AI provider implementation.
// It includes support for model selection, API key management, OAuth authentication,
// and specialized features for the OpenRouter platform.
package openrouter

import (
	"fmt"
	"strings"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// OpenRouterExtension implements CoreProviderExtension for OpenRouter
type OpenRouterExtension struct {
	*types.BaseExtension
}

// NewOpenRouterExtension creates a new OpenRouter extension
func NewOpenRouterExtension() *OpenRouterExtension {
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
		"model_routing",
		"fallback_models",
		"cost_optimization",
		"multi_provider",
		"site_referer",
		"free_tier",
	}

	return &OpenRouterExtension{
		BaseExtension: types.NewBaseExtension(
			"openrouter",
			"1.0.0",
			"OpenRouter API extension with universal model gateway and intelligent routing capabilities",
			capabilities,
		),
	}
}

// StandardToProvider converts a standard request to OpenRouter format
func (e *OpenRouterExtension) StandardToProvider(request types.StandardRequest) (interface{}, error) {
	openrouterReq := e.createBaseRequest(request)

	e.setBasicParameters(&openrouterReq, request)
	e.convertMessages(&openrouterReq, request.Messages)
	e.convertTools(&openrouterReq, request.Tools, request.ToolChoice)
	e.applyMetadataSettings(&openrouterReq, request)

	return openrouterReq, nil
}

// createBaseRequest creates the base OpenRouter request structure
func (e *OpenRouterExtension) createBaseRequest(request types.StandardRequest) OpenRouterRequest {
	return OpenRouterRequest{
		Model:  request.Model,
		Stream: request.Stream,
	}
}

// setBasicParameters sets temperature and max_tokens if provided
func (e *OpenRouterExtension) setBasicParameters(req *OpenRouterRequest, request types.StandardRequest) {
	if request.Temperature > 0 {
		req.Temperature = request.Temperature
	}

	if request.MaxTokens > 0 {
		req.MaxTokens = request.MaxTokens
	}
}

// convertMessages converts standard messages to OpenRouter message format
func (e *OpenRouterExtension) convertMessages(req *OpenRouterRequest, messages []types.ChatMessage) {
	req.Messages = make([]OpenRouterMessage, len(messages))
	for i, msg := range messages {
		openrouterMsg := OpenRouterMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Convert tool calls if present
		if len(msg.ToolCalls) > 0 {
			openrouterMsg.ToolCalls = convertToOpenRouterToolCalls(msg.ToolCalls)
		}

		// Include tool call ID for tool response messages
		if msg.ToolCallID != "" {
			openrouterMsg.ToolCallID = msg.ToolCallID
		}

		req.Messages[i] = openrouterMsg
	}
}

// convertTools converts tools and tool choice to OpenRouter format
func (e *OpenRouterExtension) convertTools(req *OpenRouterRequest, tools []types.Tool, toolChoice *types.ToolChoice) {
	if len(tools) > 0 {
		req.Tools = convertToOpenRouterTools(tools)

		// Convert tool choice if specified
		if toolChoice != nil {
			// Tool choice conversion handled in main openrouter.go file
			_ = toolChoice // Suppress unused variable warning
		}
	}
}

// applyMetadataSettings applies OpenRouter-specific settings from metadata
func (e *OpenRouterExtension) applyMetadataSettings(req *OpenRouterRequest, request types.StandardRequest) {
	if request.Metadata == nil {
		return
	}

	e.handleModelRouting(req, request)
	e.handleCostOptimization(req, request)
	e.handleFreeTierPreference(req, request)
	e.handleProviderRouting(req, request)
	e.handleHTTPSettings(req, request.Metadata)
	e.handleCustomProviderParams(req, request.Metadata)
}

// handleModelRouting handles model routing preferences from metadata
func (e *OpenRouterExtension) handleModelRouting(req *OpenRouterRequest, request types.StandardRequest) {
	routing, ok := request.Metadata["model_routing"].(string)
	if !ok {
		return
	}

	// Only set model if not already specified
	if request.Model != "" {
		return
	}

	switch routing {
	case "fastest":
		req.Model = "meta-llama/llama-3.1-8b-instruct"
	case "cheapest":
		req.Model = "meta-llama/llama-3.1-8b-instruct:free"
	case "balanced":
		req.Model = "meta-llama/llama-3.1-70b-instruct"
	case "best":
		req.Model = "anthropic/claude-3.5-sonnet"
	}
}

// handleCostOptimization applies cost optimization settings
func (e *OpenRouterExtension) handleCostOptimization(req *OpenRouterRequest, request types.StandardRequest) {
	costOpt, ok := request.Metadata["cost_optimization"].(bool)
	if !ok || !costOpt {
		return
	}

	// Optimize for cost
	if request.Temperature == 0 {
		req.Temperature = 0.7
	}
	if request.MaxTokens == 0 {
		req.MaxTokens = 1024 // Lower max tokens for cost savings
	}
}

// handleFreeTierPreference ensures free tier model selection
func (e *OpenRouterExtension) handleFreeTierPreference(req *OpenRouterRequest, request types.StandardRequest) {
	freeOnly, ok := request.Metadata["free_only"].(bool)
	if !ok || !freeOnly {
		return
	}

	// Ensure model selection from free tier
	if request.Model == "" {
		req.Model = "meta-llama/llama-3.1-8b-instruct:free"
	} else if !strings.HasSuffix(req.Model, ":free") {
		req.Model += ":free"
	}
}

// handleProviderRouting routes to specific providers
func (e *OpenRouterExtension) handleProviderRouting(req *OpenRouterRequest, request types.StandardRequest) {
	provider, ok := request.Metadata["provider"].(string)
	if !ok || request.Model != "" {
		return
	}

	switch provider {
	case "anthropic":
		req.Model = "anthropic/claude-3.5-sonnet"
	case "openai":
		req.Model = "openai/gpt-4o"
	case "google":
		req.Model = "google/gemini-pro-1.5"
	case "meta":
		req.Model = "meta-llama/llama-3.1-70b-instruct"
	}
}

// handleHTTPSettings handles HTTP-related settings
func (e *OpenRouterExtension) handleHTTPSettings(req *OpenRouterRequest, metadata map[string]interface{}) {
	// Handle site referer
	if siteURL, ok := metadata["site_url"].(string); ok {
		req.HTTPReferer = siteURL
	}

	// Handle user agent
	if userAgent, ok := metadata["user_agent"].(string); ok {
		req.HTTPUserAgent = userAgent
	}
}

// handleCustomProviderParams applies custom provider parameters
func (e *OpenRouterExtension) handleCustomProviderParams(req *OpenRouterRequest, metadata map[string]interface{}) {
	providerParams, ok := metadata["provider_params"].(map[string]interface{})
	if !ok {
		return
	}

	// Apply any provider-specific parameters
	if temp, ok := providerParams["temperature"].(float64); ok {
		req.Temperature = temp
	}
	if maxTokens, ok := providerParams["max_tokens"].(int); ok {
		req.MaxTokens = maxTokens
	}
	if _, hasTopP := providerParams["top_p"].(float64); hasTopP {
		// OpenRouter-specific top_p handling
		_ = hasTopP // suppress unused variable warning
	}
}

// ProviderToStandard converts an OpenRouter response to standard format
func (e *OpenRouterExtension) ProviderToStandard(response interface{}) (*types.StandardResponse, error) {
	openrouterResp, ok := response.(*OpenRouterResponse)
	if !ok {
		return nil, fmt.Errorf("response is not an OpenRouter response")
	}

	if len(openrouterResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in OpenRouter API response")
	}

	choice := openrouterResp.Choices[0]

	// Convert message
	message := types.ChatMessage{
		Role:    choice.Message.Role,
		Content: choice.Message.Content,
	}

	// Convert tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		message.ToolCalls = convertOpenRouterToolCallsToUniversal(choice.Message.ToolCalls)
	}

	standardChoice := types.StandardChoice{
		Index:        choice.Index,
		Message:      message,
		FinishReason: choice.FinishReason,
	}

	// Convert usage
	usage := types.Usage{
		PromptTokens:     openrouterResp.Usage.PromptTokens,
		CompletionTokens: openrouterResp.Usage.CompletionTokens,
		TotalTokens:      openrouterResp.Usage.TotalTokens,
	}

	// Create provider metadata
	providerMetadata := map[string]interface{}{
		"provider":      "openrouter",
		"model_gateway": true,
		"routed_model":  openrouterResp.Model,
	}

	return &types.StandardResponse{
		ID:               openrouterResp.ID,
		Model:            openrouterResp.Model,
		Object:           openrouterResp.Object,
		Created:          openrouterResp.Created,
		Choices:          []types.StandardChoice{standardChoice},
		Usage:            usage,
		ProviderMetadata: providerMetadata,
	}, nil
}

// ProviderToStandardChunk converts an OpenRouter stream chunk to standard format
func (e *OpenRouterExtension) ProviderToStandardChunk(chunk interface{}) (*types.StandardStreamChunk, error) {
	openrouterChunk, ok := chunk.(*OpenRouterResponse)
	if !ok {
		return nil, fmt.Errorf("chunk is not an OpenRouter stream response")
	}

	if len(openrouterChunk.Choices) == 0 {
		return nil, fmt.Errorf("no choices in OpenRouter stream chunk")
	}

	choice := openrouterChunk.Choices[0]

	// Use Message for streaming (OpenRouter uses Message instead of Delta)
	delta := types.ChatMessage{
		Role:    choice.Message.Role,
		Content: choice.Message.Content,
	}

	// Convert tool calls if present in the message
	if len(choice.Message.ToolCalls) > 0 {
		delta.ToolCalls = convertOpenRouterToolCallsToUniversal(choice.Message.ToolCalls)
	}

	standardChoice := types.StandardStreamChoice{
		Index:        choice.Index,
		Delta:        delta,
		FinishReason: choice.FinishReason,
	}

	// Convert usage if present
	var usage *types.Usage
	if openrouterChunk.Usage.TotalTokens > 0 {
		usage = &types.Usage{
			PromptTokens:     openrouterChunk.Usage.PromptTokens,
			CompletionTokens: openrouterChunk.Usage.CompletionTokens,
			TotalTokens:      openrouterChunk.Usage.TotalTokens,
		}
	}

	// Create provider metadata
	providerMetadata := map[string]interface{}{
		"provider":      "openrouter",
		"model_gateway": true,
		"routed_model":  openrouterChunk.Model,
	}

	done := choice.FinishReason != ""

	return &types.StandardStreamChunk{
		ID:               openrouterChunk.ID,
		Model:            openrouterChunk.Model,
		Object:           openrouterChunk.Object,
		Created:          openrouterChunk.Created,
		Choices:          []types.StandardStreamChoice{standardChoice},
		Usage:            usage,
		Done:             done,
		ProviderMetadata: providerMetadata,
	}, nil
}

// ValidateOptions validates OpenRouter-specific options
func (e *OpenRouterExtension) ValidateOptions(options map[string]interface{}) error {
	// Validate basic parameters
	if err := e.validateBasicOptions(options); err != nil {
		return err
	}

	// Validate routing options
	if err := e.validateRoutingOptions(options); err != nil {
		return err
	}

	// Validate mode-specific options
	if err := e.validateModeOptions(options); err != nil {
		return err
	}

	// Call base validation
	return e.BaseExtension.ValidateOptions(options)
}

// validateBasicOptions validates temperature and max_tokens
func (e *OpenRouterExtension) validateBasicOptions(options map[string]interface{}) error {
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

	return nil
}

// validateRoutingOptions validates model routing and provider selection
func (e *OpenRouterExtension) validateRoutingOptions(options map[string]interface{}) error {
	// Validate model routing
	if routing, ok := options["model_routing"].(string); ok {
		if err := e.validateModelRouting(routing); err != nil {
			return err
		}
	}

	// Validate provider selection
	if provider, ok := options["provider"].(string); ok {
		if err := e.validateProvider(provider); err != nil {
			return err
		}
	}

	return nil
}

// validateModeOptions validates cost optimization and free tier modes
func (e *OpenRouterExtension) validateModeOptions(options map[string]interface{}) error {
	// Validate cost optimization
	if costOpt, ok := options["cost_optimization"].(bool); ok && costOpt {
		if err := e.validateCostOptimization(options); err != nil {
			return err
		}
	}

	// Validate free tier
	if freeOnly, ok := options["free_only"].(bool); ok && freeOnly {
		if err := e.validateFreeTier(options); err != nil {
			return err
		}
	}

	return nil
}

// validateModelRouting checks if the routing option is valid
func (e *OpenRouterExtension) validateModelRouting(routing string) error {
	validRouting := []string{"fastest", "cheapest", "balanced", "best"}
	for _, validRoute := range validRouting {
		if routing == validRoute {
			return nil
		}
	}
	return fmt.Errorf("model_routing must be one of: %v", validRouting)
}

// validateProvider checks if the provider option is valid
func (e *OpenRouterExtension) validateProvider(provider string) error {
	validProviders := []string{"anthropic", "openai", "google", "meta", "mistral", "cohere"}
	for _, validProvider := range validProviders {
		if provider == validProvider {
			return nil
		}
	}
	return fmt.Errorf("provider must be one of: %v", validProviders)
}

// validateCostOptimization validates cost optimization specific settings
func (e *OpenRouterExtension) validateCostOptimization(options map[string]interface{}) error {
	if maxTokens, ok := options["max_tokens"].(int); ok && maxTokens > 4096 {
		return fmt.Errorf("max_tokens should be <= 4096 for cost optimization mode")
	}
	return nil
}

// validateFreeTier validates free tier specific settings
func (e *OpenRouterExtension) validateFreeTier(options map[string]interface{}) error {
	if model, ok := options["model"].(string); ok && !strings.HasSuffix(model, ":free") {
		return fmt.Errorf("model should end with ':free' for free tier mode")
	}
	return nil
}

// Register the OpenRouter extension with the default registry
func init() {
	_ = types.RegisterDefaultExtension(types.ProviderTypeOpenRouter, NewOpenRouterExtension())
}

// Helper functions are defined in the main openrouter.go file
// These extensions reference the existing functions
