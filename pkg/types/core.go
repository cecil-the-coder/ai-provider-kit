// Package types defines the core types and interfaces for the AI provider kit.
// It includes standardized request/response formats, provider interfaces, and common
// data structures used across all AI providers.
package types

import (
	"context"
	"time"
)

// StandardRequest represents the core request format that all providers support
// This contains only the fields that are universally supported across all providers
type StandardRequest struct {
	// Core content
	Messages []ChatMessage `json:"messages"`

	// Model selection
	Model string `json:"model,omitempty"`

	// Generation parameters
	MaxTokens   int      `json:"max_tokens,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
	Stop        []string `json:"stop,omitempty"`

	// Streaming control
	Stream bool `json:"stream"`

	// Tool support (if provider supports it)
	Tools      []Tool      `json:"tools,omitempty"`
	ToolChoice *ToolChoice `json:"tool_choice,omitempty"`

	// Response format (for providers that support structured output)
	ResponseFormat string `json:"response_format,omitempty"`

	// Context and metadata
	Context  context.Context        `json:"-"`
	Timeout  time.Duration          `json:"-"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// StandardResponse represents the core response format that all providers return
// This contains only the fields that are universally supported across all providers
type StandardResponse struct {
	// Core response
	ID      string `json:"id"`
	Model   string `json:"model"`
	Object  string `json:"object"`
	Created int64  `json:"created"`

	// Content
	Choices []StandardChoice `json:"choices"`

	// Usage information
	Usage Usage `json:"usage"`

	// Provider-specific metadata
	ProviderMetadata map[string]interface{} `json:"provider_metadata,omitempty"`
}

// StandardChoice represents a choice in a standardized response
type StandardChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// StandardStreamChunk represents a chunk in a streaming response
type StandardStreamChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Object  string `json:"object"`
	Created int64  `json:"created"`

	// Content
	Choices []StandardStreamChoice `json:"choices"`

	// Usage information (may be nil for intermediate chunks)
	Usage *Usage `json:"usage,omitempty"`

	// Stream state
	Done bool `json:"done"`

	// Provider-specific metadata
	ProviderMetadata map[string]interface{} `json:"provider_metadata,omitempty"`
}

// StandardStreamChoice represents a choice in a streaming chunk
type StandardStreamChoice struct {
	Index        int         `json:"index"`
	Delta        ChatMessage `json:"delta"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// CoreProviderExtension defines the interface for provider-specific extensions
// This allows providers to add their own unique capabilities while maintaining
// compatibility with the standardized core API
type CoreProviderExtension interface {
	// Extension information
	Name() string
	Version() string
	Description() string

	// Convert between standard and provider-specific formats
	StandardToProvider(request StandardRequest) (interface{}, error)
	ProviderToStandard(response interface{}) (*StandardResponse, error)
	ProviderToStandardChunk(chunk interface{}) (*StandardStreamChunk, error)

	// Validate provider-specific options
	ValidateOptions(options map[string]interface{}) error

	// Get provider-specific capabilities
	GetCapabilities() []string
}

// ExtensionRegistry manages provider extensions
type ExtensionRegistry interface {
	// Register an extension for a provider type
	Register(providerType ProviderType, extension CoreProviderExtension) error

	// Get extension for a provider type
	Get(providerType ProviderType) (CoreProviderExtension, error)

	// List all registered extensions
	List() map[ProviderType]CoreProviderExtension

	// Check if a provider type has a registered extension
	Has(providerType ProviderType) bool
}

// CoreRequestBuilder helps build standard requests with validation
type CoreRequestBuilder struct {
	request *StandardRequest
}

// NewCoreRequestBuilder creates a new request builder
func NewCoreRequestBuilder() *CoreRequestBuilder {
	return &CoreRequestBuilder{
		request: &StandardRequest{
			Messages: make([]ChatMessage, 0),
			Stop:     make([]string, 0),
			Tools:    make([]Tool, 0),
			Metadata: make(map[string]interface{}),
		},
	}
}

// WithMessages sets the messages for the request
func (b *CoreRequestBuilder) WithMessages(messages []ChatMessage) *CoreRequestBuilder {
	b.request.Messages = messages
	return b
}

// WithModel sets the model for the request
func (b *CoreRequestBuilder) WithModel(model string) *CoreRequestBuilder {
	b.request.Model = model
	return b
}

// WithMaxTokens sets the maximum tokens for the request
func (b *CoreRequestBuilder) WithMaxTokens(maxTokens int) *CoreRequestBuilder {
	b.request.MaxTokens = maxTokens
	return b
}

// WithTemperature sets the temperature for the request
func (b *CoreRequestBuilder) WithTemperature(temperature float64) *CoreRequestBuilder {
	b.request.Temperature = temperature
	return b
}

// WithStop sets the stop sequences for the request
func (b *CoreRequestBuilder) WithStop(stop []string) *CoreRequestBuilder {
	b.request.Stop = stop
	return b
}

// WithStreaming enables or disables streaming
func (b *CoreRequestBuilder) WithStreaming(stream bool) *CoreRequestBuilder {
	b.request.Stream = stream
	return b
}

// WithTools sets the tools for the request
func (b *CoreRequestBuilder) WithTools(tools []Tool) *CoreRequestBuilder {
	b.request.Tools = tools
	return b
}

// WithToolChoice sets the tool choice for the request
func (b *CoreRequestBuilder) WithToolChoice(toolChoice *ToolChoice) *CoreRequestBuilder {
	b.request.ToolChoice = toolChoice
	return b
}

// WithResponseFormat sets the response format for the request
func (b *CoreRequestBuilder) WithResponseFormat(format string) *CoreRequestBuilder {
	b.request.ResponseFormat = format
	return b
}

// WithContext sets the context for the request
func (b *CoreRequestBuilder) WithContext(ctx context.Context) *CoreRequestBuilder {
	b.request.Context = ctx
	return b
}

// WithTimeout sets the timeout for the request
func (b *CoreRequestBuilder) WithTimeout(timeout time.Duration) *CoreRequestBuilder {
	b.request.Timeout = timeout
	return b
}

// WithMetadata adds metadata to the request
func (b *CoreRequestBuilder) WithMetadata(key string, value interface{}) *CoreRequestBuilder {
	b.request.Metadata[key] = value
	return b
}

// Build builds the standard request
func (b *CoreRequestBuilder) Build() (*StandardRequest, error) {
	// Validate the request
	if err := b.validate(); err != nil {
		return nil, err
	}

	// Return a copy to prevent modification after building
	requestCopy := *b.request
	return &requestCopy, nil
}

// validate validates the request
func (b *CoreRequestBuilder) validate() error {
	// Check if we have content
	if len(b.request.Messages) == 0 {
		return ErrNoMessages
	}

	// Validate temperature
	if b.request.Temperature < 0 || b.request.Temperature > 2 {
		return ErrInvalidTemperature
	}

	// Validate max tokens
	if b.request.MaxTokens < 0 {
		return ErrInvalidMaxTokens
	}

	// Validate tools and tool choice consistency
	if len(b.request.Tools) == 0 && b.request.ToolChoice != nil {
		return ErrToolChoiceWithoutTools
	}

	return nil
}

// FromGenerateOptions converts from legacy GenerateOptions to StandardRequest
func (b *CoreRequestBuilder) FromGenerateOptions(options GenerateOptions) *CoreRequestBuilder {
	b.WithMessages(options.Messages)
	b.WithModel(options.Model)
	b.WithMaxTokens(options.MaxTokens)
	b.WithTemperature(options.Temperature)
	b.WithStop(options.Stop)
	b.WithStreaming(options.Stream)
	b.WithTools(options.Tools)
	b.WithToolChoice(options.ToolChoice)
	b.WithResponseFormat(options.ResponseFormat)
	b.WithContext(options.ContextObj)
	b.WithTimeout(options.Timeout)

	// Copy metadata
	if options.Metadata != nil {
		for k, v := range options.Metadata {
			b.WithMetadata(k, v)
		}
	}

	return b
}

// ToGenerateOptions converts from StandardRequest to legacy GenerateOptions
func (r *StandardRequest) ToGenerateOptions() GenerateOptions {
	return GenerateOptions{
		Messages:       r.Messages,
		Model:          r.Model,
		MaxTokens:      r.MaxTokens,
		Temperature:    r.Temperature,
		Stop:           r.Stop,
		Stream:         r.Stream,
		Tools:          r.Tools,
		ToolChoice:     r.ToolChoice,
		ResponseFormat: r.ResponseFormat,
		ContextObj:     r.Context,
		Timeout:        r.Timeout,
		Metadata:       r.Metadata,
	}
}

// Common validation errors
var (
	ErrNoMessages             = NewValidationError("at least one message is required")
	ErrInvalidTemperature     = NewValidationError("temperature must be between 0 and 2")
	ErrInvalidMaxTokens       = NewValidationError("max_tokens must be non-negative")
	ErrToolChoiceWithoutTools = NewValidationError("tool_choice specified but no tools provided")
)

// ValidationError represents a validation error
type ValidationError struct {
	Message string
}

func NewValidationError(message string) *ValidationError {
	return &ValidationError{Message: message}
}

func (e *ValidationError) Error() string {
	return e.Message
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}
