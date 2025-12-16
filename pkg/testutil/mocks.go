// Package testutil provides mock implementations and testing utilities for the AI Provider Kit.
// These mocks are used across the codebase to facilitate unit and integration testing.
package testutil

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// MockProvider is a comprehensive mock implementation of types.Provider for testing.
// It combines features from multiple test implementations to provide a flexible testing tool.
type MockProvider struct {
	name          string
	providerType  types.ProviderType
	description   string
	models        []types.Model
	config        types.ProviderConfig
	authenticated bool
	isHealthy     bool
	healthErr     error
	metrics       types.ProviderMetrics
}

// NewMockProvider creates a new MockProvider with sensible defaults.
func NewMockProvider(name string, providerType types.ProviderType) *MockProvider {
	return &MockProvider{
		name:          name,
		providerType:  providerType,
		description:   fmt.Sprintf("Mock provider: %s", name),
		authenticated: true,
		isHealthy:     true,
		config: types.ProviderConfig{
			Name: name,
			Type: providerType,
		},
		models: []types.Model{
			{ID: "mock-model", Name: "Mock Model"},
		},
		metrics: types.ProviderMetrics{
			HealthStatus: types.HealthStatus{
				Healthy:     true,
				LastChecked: time.Now(),
				Message:     "OK",
			},
		},
	}
}

// SetModels allows setting custom models for the mock provider.
func (m *MockProvider) SetModels(models []types.Model) {
	m.models = models
}

// SetHealthy allows setting the health status of the mock provider.
func (m *MockProvider) SetHealthy(healthy bool, err error) {
	m.isHealthy = healthy
	m.healthErr = err
}

// SetDescription allows setting a custom description.
func (m *MockProvider) SetDescription(description string) {
	m.description = description
}

// Name returns the name of the mock provider.
func (m *MockProvider) Name() string {
	return m.name
}

// Type returns the provider type of the mock provider.
func (m *MockProvider) Type() types.ProviderType {
	return m.providerType
}

// Description returns the description of the mock provider.
func (m *MockProvider) Description() string {
	return m.description
}

// GetConfig returns the configuration of the mock provider.
func (m *MockProvider) GetConfig() types.ProviderConfig {
	return m.config
}

// Configure configures the mock provider with the given configuration.
func (m *MockProvider) Configure(config types.ProviderConfig) error {
	m.config = config
	return nil
}

// GetModels returns the list of models supported by the mock provider.
func (m *MockProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	return m.models, nil
}

// GetDefaultModel returns the default model for the mock provider.
func (m *MockProvider) GetDefaultModel() string {
	if m.config.DefaultModel != "" {
		return m.config.DefaultModel
	}
	return "mock-default-model"
}

// Authenticate authenticates the mock provider with the given configuration.
func (m *MockProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	m.authenticated = true
	return nil
}

// IsAuthenticated returns whether the mock provider is authenticated.
func (m *MockProvider) IsAuthenticated() bool {
	return m.authenticated
}

// Logout logs out the mock provider.
func (m *MockProvider) Logout(ctx context.Context) error {
	m.authenticated = false
	return nil
}

// HealthCheck performs a health check on the mock provider.
func (m *MockProvider) HealthCheck(ctx context.Context) error {
	if m.healthErr != nil {
		return m.healthErr
	}
	if !m.isHealthy {
		return fmt.Errorf("provider is unhealthy")
	}
	return nil
}

// GetMetrics returns the metrics for the mock provider.
func (m *MockProvider) GetMetrics() types.ProviderMetrics {
	return m.metrics
}

// SupportsToolCalling returns whether the mock provider supports tool calling.
func (m *MockProvider) SupportsToolCalling() bool {
	return true
}

// SupportsStreaming returns whether the mock provider supports streaming.
func (m *MockProvider) SupportsStreaming() bool {
	return true
}

// SupportsResponsesAPI returns whether the mock provider supports the responses API.
func (m *MockProvider) SupportsResponsesAPI() bool {
	return false
}

// GetToolFormat returns the tool format supported by the mock provider.
func (m *MockProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

// GenerateChatCompletion generates a mock chat completion stream.
func (m *MockProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	m.metrics.RequestCount++
	m.metrics.SuccessCount++
	return &MockStream{}, nil
}

// InvokeServerTool invokes a server tool (not implemented in mock).
func (m *MockProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return "tool result", fmt.Errorf("tool calling not implemented in mock")
}

// MockStream is a mock implementation of types.ChatCompletionStream for testing.
type MockStream struct {
	closed bool
	chunks []types.ChatCompletionChunk
	index  int
	mu     sync.Mutex
}

// NewMockStream creates a new MockStream with default behavior.
func NewMockStream() *MockStream {
	return &MockStream{
		chunks: []types.ChatCompletionChunk{
			{Content: "test", Done: true},
		},
		index: 0,
	}
}

// NewMockStreamWithChunks creates a new MockStream with custom chunks.
func NewMockStreamWithChunks(chunks []types.ChatCompletionChunk) *MockStream {
	return &MockStream{
		chunks: chunks,
		index:  0,
	}
}

// Next returns the next chunk in the mock stream.
func (m *MockStream) Next() (types.ChatCompletionChunk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.index >= len(m.chunks) {
		return types.ChatCompletionChunk{}, nil
	}

	chunk := m.chunks[m.index]
	m.index++
	return chunk, nil
}

// Close closes the mock stream.
func (m *MockStream) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// IsClosed returns whether the stream has been closed.
func (m *MockStream) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// MockAuthenticator is a mock implementation of Authenticator for testing.
type MockAuthenticator struct {
	authenticated    bool
	logoutShouldFail bool
	logoutError      error
	authMethod       types.AuthMethod
	refreshError     error
}

// NewMockAuthenticator creates a new MockAuthenticator with default values.
func NewMockAuthenticator() *MockAuthenticator {
	return &MockAuthenticator{
		authenticated: true,
		authMethod:    types.AuthMethodAPIKey,
	}
}

// SetLogoutError configures the mock to fail on logout with the given error.
func (m *MockAuthenticator) SetLogoutError(err error) {
	m.logoutShouldFail = true
	m.logoutError = err
}

// SetRefreshError configures the mock to fail on token refresh with the given error.
func (m *MockAuthenticator) SetRefreshError(err error) {
	m.refreshError = err
}

// Authenticate authenticates using the mock authenticator.
func (m *MockAuthenticator) Authenticate(ctx context.Context, config types.AuthConfig) error {
	m.authenticated = true
	return nil
}

// IsAuthenticated returns whether the mock authenticator is authenticated.
func (m *MockAuthenticator) IsAuthenticated() bool {
	return m.authenticated
}

// GetToken returns the authentication token from the mock authenticator.
func (m *MockAuthenticator) GetToken() (string, error) {
	if !m.authenticated {
		return "", errors.New("not authenticated")
	}
	return "mock-token", nil
}

// RefreshToken refreshes the authentication token.
func (m *MockAuthenticator) RefreshToken(ctx context.Context) error {
	if m.refreshError != nil {
		return m.refreshError
	}
	if !m.authenticated {
		return errors.New("not authenticated")
	}
	return nil
}

// Logout logs out the mock authenticator.
func (m *MockAuthenticator) Logout(ctx context.Context) error {
	if m.logoutShouldFail {
		return m.logoutError
	}
	m.authenticated = false
	return nil
}

// GetAuthMethod returns the authentication method used by the mock authenticator.
func (m *MockAuthenticator) GetAuthMethod() types.AuthMethod {
	if m.authMethod == "" {
		return types.AuthMethodAPIKey
	}
	return m.authMethod
}

// MockTokenStorage is a mock implementation of TokenStorage for testing.
type MockTokenStorage struct {
	tokens            map[string]*types.OAuthConfig
	shouldFailCleanup bool
	cleanupError      error
	shouldFailDelete  bool
	deleteError       error
	mu                sync.RWMutex
}

// NewMockTokenStorage creates a new MockTokenStorage.
func NewMockTokenStorage() *MockTokenStorage {
	return &MockTokenStorage{
		tokens: make(map[string]*types.OAuthConfig),
	}
}

// SetCleanupError configures the mock to fail on cleanup with the given error.
func (m *MockTokenStorage) SetCleanupError(err error) {
	m.shouldFailCleanup = true
	m.cleanupError = err
}

// SetDeleteError configures the mock to fail on delete with the given error.
func (m *MockTokenStorage) SetDeleteError(err error) {
	m.shouldFailDelete = true
	m.deleteError = err
}

// StoreToken stores a token in the mock token storage.
func (m *MockTokenStorage) StoreToken(key string, config *types.OAuthConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.tokens == nil {
		m.tokens = make(map[string]*types.OAuthConfig)
	}
	m.tokens[key] = config
	return nil
}

// RetrieveToken retrieves a token from the mock token storage.
func (m *MockTokenStorage) RetrieveToken(key string) (*types.OAuthConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	token, exists := m.tokens[key]
	if !exists {
		return nil, errors.New("token not found")
	}
	return token, nil
}

// DeleteToken deletes a token from the mock token storage.
func (m *MockTokenStorage) DeleteToken(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFailDelete {
		return m.deleteError
	}
	delete(m.tokens, key)
	return nil
}

// ListTokens lists all tokens in the mock token storage.
func (m *MockTokenStorage) ListTokens() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.tokens))
	for key := range m.tokens {
		keys = append(keys, key)
	}
	return keys, nil
}

// IsTokenValid checks if a token is valid in the mock token storage.
func (m *MockTokenStorage) IsTokenValid(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	token, exists := m.tokens[key]
	if !exists {
		return false
	}
	// Check if token is expired
	return token.ExpiresAt.After(time.Now())
}

// CleanupExpired cleans up expired tokens from the mock token storage.
func (m *MockTokenStorage) CleanupExpired() error {
	if m.shouldFailCleanup {
		return m.cleanupError
	}
	return nil
}
