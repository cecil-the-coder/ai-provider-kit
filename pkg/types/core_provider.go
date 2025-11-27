package types

import (
	"context"
)

// CoreChatProvider defines the interface for providers that work with the standardized core API
// This is the new recommended interface that all providers should implement
type CoreChatProvider interface {
	// Core chat completion using standard request/response format
	GenerateStandardCompletion(ctx context.Context, request StandardRequest) (*StandardResponse, error)

	// Streaming chat completion using standard format
	GenerateStandardStream(ctx context.Context, request StandardRequest) (StandardStream, error)

	// Get the provider's core extension for format conversion
	GetCoreExtension() CoreProviderExtension

	// Provider capabilities using the standardized format
	GetStandardCapabilities() []string

	// Validate a standard request for this provider
	ValidateStandardRequest(request StandardRequest) error
}

// StandardStream represents a standardized streaming response
type StandardStream interface {
	// Next returns the next chunk from the stream
	Next() (*StandardStreamChunk, error)

	// Close closes the stream
	Close() error

	// Done returns true if the stream is finished
	Done() bool
}

// CoreProviderAdapter adapts existing providers to work with the new standardized API
type CoreProviderAdapter struct {
	provider  Provider
	extension CoreProviderExtension
	coreAPI   *CoreAPI
}

// NewCoreProviderAdapter creates a new adapter for an existing provider
func NewCoreProviderAdapter(provider Provider, extension CoreProviderExtension) *CoreProviderAdapter {
	return &CoreProviderAdapter{
		provider:  provider,
		extension: extension,
		coreAPI:   GetDefaultCoreAPI(),
	}
}

// GenerateStandardCompletion generates a completion using the standard format
func (a *CoreProviderAdapter) GenerateStandardCompletion(ctx context.Context, request StandardRequest) (*StandardResponse, error) {
	// Validate the request
	if err := a.ValidateStandardRequest(request); err != nil {
		return nil, err
	}

	// Convert to legacy GenerateOptions
	generateOptions := a.convertToLegacyOptions(request)

	// Execute using the provider's existing interface
	stream, err := a.provider.GenerateChatCompletion(ctx, generateOptions)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = stream.Close()
	}()

	// Collect all chunks to build the final response
	var finalChunk ChatCompletionChunk
	for {
		chunk, err := stream.Next()
		if err != nil {
			break // End of stream
		}

		if chunk.Done {
			finalChunk = chunk
			break
		}
	}

	// Convert the final chunk to standard response
	standardResponse, err := a.extension.ProviderToStandard(&finalChunk)
	if err != nil {
		return nil, err
	}

	return standardResponse, nil
}

// GenerateStandardStream generates a streaming completion using the standard format
func (a *CoreProviderAdapter) GenerateStandardStream(ctx context.Context, request StandardRequest) (StandardStream, error) {
	// Validate the request
	if err := a.ValidateStandardRequest(request); err != nil {
		return nil, err
	}

	// Convert to legacy GenerateOptions
	generateOptions := a.convertToLegacyOptions(request)

	// Execute using the provider's existing interface
	stream, err := a.provider.GenerateChatCompletion(ctx, generateOptions)
	if err != nil {
		return nil, err
	}

	// Return adapter stream
	return &StandardStreamAdapter{
		providerStream: stream,
		extension:      a.extension,
	}, nil
}

// GetCoreExtension returns the provider's core extension
func (a *CoreProviderAdapter) GetCoreExtension() CoreProviderExtension {
	return a.extension
}

// GetStandardCapabilities returns the provider's capabilities
func (a *CoreProviderAdapter) GetStandardCapabilities() []string {
	return a.extension.GetCapabilities()
}

// ValidateStandardRequest validates a standard request for this provider
func (a *CoreProviderAdapter) ValidateStandardRequest(request StandardRequest) error {
	// Basic validation
	if len(request.Messages) == 0 {
		return ErrNoMessages
	}

	// Provider-specific validation
	return a.extension.ValidateOptions(request.Metadata)
}

// convertToLegacyOptions converts a standard request to legacy GenerateOptions
func (a *CoreProviderAdapter) convertToLegacyOptions(request StandardRequest) GenerateOptions {
	return GenerateOptions{
		Messages:       request.Messages,
		Model:          request.Model,
		MaxTokens:      request.MaxTokens,
		Temperature:    request.Temperature,
		Stop:           request.Stop,
		Stream:         request.Stream,
		Tools:          request.Tools,
		ToolChoice:     request.ToolChoice,
		ResponseFormat: request.ResponseFormat,
		ContextObj:     request.Context,
		Timeout:        request.Timeout,
		Metadata:       request.Metadata,
	}
}

// StandardStreamAdapter adapts legacy streams to the standard stream interface
type StandardStreamAdapter struct {
	providerStream ChatCompletionStream
	extension      CoreProviderExtension
	done           bool
	lastChunk      *StandardStreamChunk
}

// Next returns the next chunk from the stream
func (s *StandardStreamAdapter) Next() (*StandardStreamChunk, error) {
	if s.done {
		return nil, nil // Stream is finished
	}

	chunk, err := s.providerStream.Next()
	if err != nil {
		s.done = true
		return nil, err
	}

	if chunk.Done {
		s.done = true
	}

	// Convert the chunk using the extension
	standardChunk, err := s.extension.ProviderToStandardChunk(&chunk)
	if err != nil {
		return nil, err
	}

	s.lastChunk = standardChunk
	return standardChunk, nil
}

// Close closes the stream
func (s *StandardStreamAdapter) Close() error {
	return s.providerStream.Close()
}

// Done returns true if the stream is finished
func (s *StandardStreamAdapter) Done() bool {
	return s.done
}

// MockStandardStream implements StandardStream for testing
type MockStandardStream struct {
	chunks []StandardStreamChunk
	index  int
}

// NewMockStandardStream creates a new mock standard stream
func NewMockStandardStream(chunks []StandardStreamChunk) *MockStandardStream {
	return &MockStandardStream{
		chunks: chunks,
		index:  0,
	}
}

// Next returns the next chunk from the mock stream
func (m *MockStandardStream) Next() (*StandardStreamChunk, error) {
	if m.index >= len(m.chunks) {
		return nil, nil // End of stream
	}

	chunk := m.chunks[m.index]
	m.index++
	return &chunk, nil
}

// Close closes the mock stream
func (m *MockStandardStream) Close() error {
	m.index = 0
	return nil
}

// Done returns true if the mock stream is finished
func (m *MockStandardStream) Done() bool {
	return m.index >= len(m.chunks)
}

// ProviderFactoryExtensions extends the existing ProviderFactory to work with the core API
type ProviderFactoryExtensions interface {
	ProviderFactory

	// Create a core provider adapter
	CreateCoreProvider(providerType ProviderType, config ProviderConfig) (CoreChatProvider, error)

	// Get supported provider types for core API
	GetSupportedCoreProviders() []ProviderType

	// Check if a provider type supports the core API
	SupportsCoreAPI(providerType ProviderType) bool
}

// DefaultProviderFactoryExtensions implements ProviderFactoryExtensions
type DefaultProviderFactoryExtensions struct {
	ProviderFactory
	coreAPI *CoreAPI
}

// NewDefaultProviderFactoryExtensions creates a new factory with core API support
func NewDefaultProviderFactoryExtensions(factory ProviderFactory) *DefaultProviderFactoryExtensions {
	return &DefaultProviderFactoryExtensions{
		ProviderFactory: factory,
		coreAPI:         GetDefaultCoreAPI(),
	}
}

// CreateCoreProvider creates a core provider adapter
func (f *DefaultProviderFactoryExtensions) CreateCoreProvider(providerType ProviderType, config ProviderConfig) (CoreChatProvider, error) {
	// Create the legacy provider
	provider, err := f.CreateProvider(providerType, config)
	if err != nil {
		return nil, err
	}

	// Get the extension for this provider type
	extension, err := f.coreAPI.GetExtension(providerType)
	if err != nil {
		return nil, err
	}

	// Return the adapter
	return NewCoreProviderAdapter(provider, extension), nil
}

// GetSupportedCoreProviders returns provider types that support the core API
func (f *DefaultProviderFactoryExtensions) GetSupportedCoreProviders() []ProviderType {
	supported := f.GetSupportedProviders()
	var coreSupported []ProviderType

	for _, providerType := range supported {
		if f.coreAPI.HasExtension(providerType) {
			coreSupported = append(coreSupported, providerType)
		}
	}

	return coreSupported
}

// SupportsCoreAPI checks if a provider type supports the core API
func (f *DefaultProviderFactoryExtensions) SupportsCoreAPI(providerType ProviderType) bool {
	return f.coreAPI.HasExtension(providerType)
}
