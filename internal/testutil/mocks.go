// Package testutil provides shared testing utilities, mocks, and fixtures
// for use across the ai-provider-kit test suite.
package testutil

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ConfigurableMockProvider is a mock Provider implementation with configurable behavior.
// It allows tests to simulate various provider responses and scenarios.
type ConfigurableMockProvider struct {
	mu sync.RWMutex

	// Configuration
	name              string
	providerType      types.ProviderType
	description       string
	authenticated     bool
	config            types.ProviderConfig
	defaultModel      string
	models            []types.Model
	supportsToolCall  bool
	supportsStreaming bool
	supportsResponses bool
	toolFormat        string

	// Behavior control
	authenticateError   error
	logoutError         error
	configureError      error
	generateError       error
	getModelsError      error
	healthCheckError    error
	createResponseError error
	createStreamError   error

	// Mock responses
	chatCompletionResponse types.ChatCompletionStream
	createResponseResponse *types.StandardResponse
	createStreamResponse   types.StandardStream

	// Metrics
	metrics types.ProviderMetrics

	// Call tracking
	authenticateCalled     int
	logoutCalled           int
	configureCalled        int
	generateChatCalled     int
	getModelsCalled        int
	healthCheckCalled      int
	createResponseCalled   int
	createStreamCalled     int
	invokeServerToolCalled int
}

// NewConfigurableMockProvider creates a new mock provider with default settings.
func NewConfigurableMockProvider(name string, providerType types.ProviderType) *ConfigurableMockProvider {
	return &ConfigurableMockProvider{
		name:              name,
		providerType:      providerType,
		description:       fmt.Sprintf("Mock %s provider for testing", name),
		authenticated:     true,
		defaultModel:      "mock-model",
		supportsToolCall:  true,
		supportsStreaming: true,
		supportsResponses: true,
		toolFormat:        "openai",
		models: []types.Model{
			{
				ID:          "mock-model",
				Name:        "Mock Model",
				Description: "A mock model for testing",
			},
		},
		config: types.ProviderConfig{
			Type:                 providerType,
			APIKey:               "mock-api-key",
			DefaultModel:         "mock-model",
			SupportsToolCalling:  true,
			SupportsStreaming:    true,
			SupportsResponsesAPI: true,
		},
		metrics: types.ProviderMetrics{},
	}
}

// SetAuthenticateError configures the provider to return an error on Authenticate
func (m *ConfigurableMockProvider) SetAuthenticateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authenticateError = err
}

// SetGenerateError configures the provider to return an error on GenerateChatCompletion
func (m *ConfigurableMockProvider) SetGenerateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.generateError = err
}

// SetChatCompletionStream configures the stream returned by GenerateChatCompletion
func (m *ConfigurableMockProvider) SetChatCompletionStream(stream types.ChatCompletionStream) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chatCompletionResponse = stream
}

// SetHealthCheckError configures the provider to return an error on HealthCheck
func (m *ConfigurableMockProvider) SetHealthCheckError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthCheckError = err
}

// SetModels configures the list of models returned by GetModels
func (m *ConfigurableMockProvider) SetModels(models []types.Model) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.models = models
}

// SetGetModelsError configures the provider to return an error on GetModels
func (m *ConfigurableMockProvider) SetGetModelsError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getModelsError = err
}

// GetAuthenticateCallCount returns the number of times Authenticate was called
func (m *ConfigurableMockProvider) GetAuthenticateCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.authenticateCalled
}

// GetGenerateChatCallCount returns the number of times GenerateChatCompletion was called
func (m *ConfigurableMockProvider) GetGenerateChatCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.generateChatCalled
}

// GetHealthCheckCallCount returns the number of times HealthCheck was called
func (m *ConfigurableMockProvider) GetHealthCheckCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthCheckCalled
}

// Provider interface implementation

func (m *ConfigurableMockProvider) Name() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.name
}

func (m *ConfigurableMockProvider) Type() types.ProviderType {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.providerType
}

func (m *ConfigurableMockProvider) Description() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.description
}

func (m *ConfigurableMockProvider) Authenticate(ctx context.Context, config types.AuthConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authenticateCalled++
	if m.authenticateError != nil {
		return m.authenticateError
	}
	m.authenticated = true
	return nil
}

func (m *ConfigurableMockProvider) IsAuthenticated() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.authenticated
}

func (m *ConfigurableMockProvider) Logout(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logoutCalled++
	if m.logoutError != nil {
		return m.logoutError
	}
	m.authenticated = false
	return nil
}

func (m *ConfigurableMockProvider) Configure(config types.ProviderConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configureCalled++
	if m.configureError != nil {
		return m.configureError
	}
	m.config = config
	return nil
}

func (m *ConfigurableMockProvider) GetConfig() types.ProviderConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

func (m *ConfigurableMockProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.generateChatCalled++
	m.metrics.RequestCount++

	if m.generateError != nil {
		m.metrics.ErrorCount++
		return nil, m.generateError
	}

	m.metrics.SuccessCount++
	if m.chatCompletionResponse != nil {
		return m.chatCompletionResponse, nil
	}

	// Return a default mock stream
	return NewConfigurableMockStream([]types.ChatCompletionChunk{
		{
			Choices: []types.ChatChoice{
				{Delta: types.ChatMessage{Content: "Mock "}},
			},
			Done: false,
		},
		{
			Choices: []types.ChatChoice{
				{Delta: types.ChatMessage{Content: "response"}},
			},
			Done: true,
			Usage: types.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
	}), nil
}

func (m *ConfigurableMockProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invokeServerToolCalled++

	// Return a mock tool result
	return map[string]interface{}{
		"tool":   toolName,
		"result": "mock result",
	}, nil
}

func (m *ConfigurableMockProvider) CreateResponse(ctx context.Context, request types.StandardRequest) (*types.StandardResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createResponseCalled++

	if m.createResponseError != nil {
		return nil, m.createResponseError
	}

	if m.createResponseResponse != nil {
		return m.createResponseResponse, nil
	}

	// Return a default standard response
	return &types.StandardResponse{
		ID:     "mock-response-id",
		Model:  m.defaultModel,
		Object: "chat.completion",
		Choices: []types.StandardChoice{
			{
				Index: 0,
				Message: types.ChatMessage{
					Role:    "assistant",
					Content: "Mock standard response",
				},
				FinishReason: "stop",
			},
		},
		Usage: types.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func (m *ConfigurableMockProvider) CreateStream(ctx context.Context, request types.StandardRequest) (types.StandardStream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createStreamCalled++

	if m.createStreamError != nil {
		return nil, m.createStreamError
	}

	if m.createStreamResponse != nil {
		return m.createStreamResponse, nil
	}

	// Return a default standard stream
	chunks := []types.StandardStreamChunk{
		{
			ID:     "mock-chunk-1",
			Model:  m.defaultModel,
			Object: "chat.completion.chunk",
			Choices: []types.StandardStreamChoice{
				{
					Index: 0,
					Delta: types.ChatMessage{Content: "Mock "},
				},
			},
			Done: false,
		},
		{
			ID:     "mock-chunk-2",
			Model:  m.defaultModel,
			Object: "chat.completion.chunk",
			Choices: []types.StandardStreamChoice{
				{
					Index: 0,
					Delta: types.ChatMessage{Content: "stream"},
				},
			},
			Done: true,
		},
	}

	return types.NewMockStandardStream(chunks), nil
}

func (m *ConfigurableMockProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getModelsCalled++

	if m.getModelsError != nil {
		return nil, m.getModelsError
	}

	return m.models, nil
}

func (m *ConfigurableMockProvider) GetDefaultModel() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defaultModel
}

func (m *ConfigurableMockProvider) SupportsToolCalling() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.supportsToolCall
}

func (m *ConfigurableMockProvider) SupportsStreaming() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.supportsStreaming
}

func (m *ConfigurableMockProvider) SupportsResponsesAPI() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.supportsResponses
}

func (m *ConfigurableMockProvider) GetToolFormat() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.toolFormat
}

func (m *ConfigurableMockProvider) HealthCheck(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthCheckCalled++

	if m.healthCheckError != nil {
		return m.healthCheckError
	}

	return nil
}

func (m *ConfigurableMockProvider) GetMetrics() types.ProviderMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

// ConfigurableMockStream is a mock ChatCompletionStream implementation with configurable behavior.
type ConfigurableMockStream struct {
	mu     sync.Mutex
	chunks []types.ChatCompletionChunk
	index  int
	err    error
}

// NewConfigurableMockStream creates a new mock stream with the given chunks.
func NewConfigurableMockStream(chunks []types.ChatCompletionChunk) *ConfigurableMockStream {
	return &ConfigurableMockStream{
		chunks: chunks,
		index:  0,
	}
}

// SetError configures the stream to return an error on the next Next() call.
func (s *ConfigurableMockStream) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

// Next returns the next chunk from the stream.
func (s *ConfigurableMockStream) Next() (types.ChatCompletionChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return types.ChatCompletionChunk{}, s.err
	}

	if s.index >= len(s.chunks) {
		return types.ChatCompletionChunk{}, io.EOF
	}

	chunk := s.chunks[s.index]
	s.index++
	return chunk, nil
}

// Close resets the stream to the beginning.
func (s *ConfigurableMockStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.index = 0
	return nil
}

// Reset resets the stream to the beginning without locking (useful in tests).
func (s *ConfigurableMockStream) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.index = 0
	s.err = nil
}
