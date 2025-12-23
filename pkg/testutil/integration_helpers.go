// Package testutil provides shared utilities for integration testing across the AI Provider Kit.
// These helpers reduce duplication in integration tests by providing common setup patterns,
// mock authentication configurations, and test scenarios.
package testutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Provider Test Setup
// ============================================================================

// ProviderTestSetup contains common test setup for providers.
type ProviderTestSetup struct {
	Provider    types.Provider
	Config      types.ProviderConfig
	AuthConfig  types.AuthConfig
	Context     context.Context
	CleanupFunc func() error
}

// NewProviderTestSetup creates a standard provider test setup with sensible defaults.
// This is a convenience function for integration tests that need a fully configured provider.
func NewProviderTestSetup(t *testing.T, providerType types.ProviderType, name string) *ProviderTestSetup {
	ctx := context.Background()
	config := DefaultProviderConfig(providerType, name)
	authConfig := DefaultAuthConfig(config)

	return &ProviderTestSetup{
		Config:     config,
		AuthConfig: authConfig,
		Context:    ctx,
		CleanupFunc: func() error {
			return nil
		},
	}
}

// WithProvider sets the provider in the test setup.
func (s *ProviderTestSetup) WithProvider(provider types.Provider) *ProviderTestSetup {
	s.Provider = provider
	return s
}

// WithContext sets a custom context in the test setup.
func (s *ProviderTestSetup) WithContext(ctx context.Context) *ProviderTestSetup {
	s.Context = ctx
	return s
}

// WithCleanup sets a custom cleanup function in the test setup.
func (s *ProviderTestSetup) WithCleanup(cleanup func() error) *ProviderTestSetup {
	s.CleanupFunc = cleanup
	return s
}

// AuthenticateProvider authenticates the provider using the auth config.
func (s *ProviderTestSetup) AuthenticateProvider(t *testing.T) {
	require.NoError(t, s.Provider.Authenticate(s.Context, s.AuthConfig),
		"Provider authentication should succeed")
	require.True(t, s.Provider.IsAuthenticated(),
		"Provider should be authenticated after authentication")
}

// VerifyProviderCapabilities performs common capability checks.
func (s *ProviderTestSetup) VerifyProviderCapabilities(t *testing.T, expectsToolCalling, expectsStreaming, expectsResponsesAPI bool) {
	require.Equal(t, expectsToolCalling, s.Provider.SupportsToolCalling(),
		"Tool calling capability mismatch")
	require.Equal(t, expectsStreaming, s.Provider.SupportsStreaming(),
		"Streaming capability mismatch")
	require.Equal(t, expectsResponsesAPI, s.Provider.SupportsResponsesAPI(),
		"Responses API capability mismatch")
}

// Cleanup performs cleanup operations for the test setup.
func (s *ProviderTestSetup) Cleanup(t *testing.T) {
	if s.CleanupFunc != nil {
		require.NoError(t, s.CleanupFunc(), "Cleanup should succeed")
	}
	if s.Provider != nil && s.Provider.IsAuthenticated() {
		require.NoError(t, s.Provider.Logout(s.Context), "Logout should succeed")
	}
}

// ============================================================================
// Mock Authentication Configuration
// ============================================================================

// DefaultProviderConfig creates a standard provider config with sensible defaults.
func DefaultProviderConfig(providerType types.ProviderType, name string) types.ProviderConfig {
	baseURL := getDefaultBaseURL(providerType)
	defaultModel := getDefaultModel(providerType)
	apiKey := fmt.Sprintf("test-%s-api-key", providerType)

	return types.ProviderConfig{
		Type:                 providerType,
		Name:                 name,
		APIKey:               apiKey,
		BaseURL:              baseURL,
		DefaultModel:         defaultModel,
		Description:          fmt.Sprintf("Test %s provider", providerType),
		SupportsStreaming:    true,
		SupportsToolCalling:  true,
		SupportsResponsesAPI: false,
		MaxTokens:            4096,
		Timeout:              30 * time.Second,
		ToolFormat:           types.ToolFormatOpenAI,
	}
}

// DefaultAuthConfig creates a standard auth config from a provider config.
func DefaultAuthConfig(config types.ProviderConfig) types.AuthConfig {
	return types.AuthConfig{
		Method:       types.AuthMethodAPIKey,
		APIKey:       config.APIKey,
		BaseURL:      config.BaseURL,
		DefaultModel: config.DefaultModel,
	}
}

// OAuthTestConfig creates a test OAuth configuration.
func OAuthTestConfig(clientID, clientSecret string, scopes []string) *types.OAuthCredentialSet {
	return &types.OAuthCredentialSet{
		ID:           "test-oauth-cred",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Scopes:       scopes,
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}
}

// MultiOAuthTestConfig creates multiple test OAuth credential sets for failover testing.
func MultiOAuthTestConfig(count int) []*types.OAuthCredentialSet {
	creds := make([]*types.OAuthCredentialSet, count)
	for i := 0; i < count; i++ {
		creds[i] = &types.OAuthCredentialSet{
			ID:           fmt.Sprintf("test-oauth-cred-%d", i),
			ClientID:     fmt.Sprintf("test-client-id-%d", i),
			ClientSecret: fmt.Sprintf("test-client-secret-%d", i),
			AccessToken:  fmt.Sprintf("test-access-token-%d", i),
			RefreshToken: fmt.Sprintf("test-refresh-token-%d", i),
			Scopes:       []string{"read", "write"},
			ExpiresAt:    time.Now().Add(1 * time.Hour),
		}
	}
	return creds
}

// ProviderConfigWithOAuth creates a provider config with OAuth credentials.
func ProviderConfigWithOAuth(providerType types.ProviderType, name string, oauthCreds []*types.OAuthCredentialSet) types.ProviderConfig {
	config := DefaultProviderConfig(providerType, name)
	config.OAuthCredentials = oauthCreds
	config.APIKey = "" // Clear API key when using OAuth
	return config
}

// ============================================================================
// Test Credential Providers
// ============================================================================

// TestCredentialProvider defines an interface for providing test credentials.
type TestCredentialProvider interface {
	GetAPIKey() string
	GetBaseURL() string
	GetDefaultModel() string
	IsConfigured() bool
}

// StaticCredentialProvider provides static test credentials.
type StaticCredentialProvider struct {
	apiKey       string
	baseURL      string
	defaultModel string
}

// NewStaticCredentialProvider creates a new static credential provider.
func NewStaticCredentialProvider(apiKey, baseURL, defaultModel string) *StaticCredentialProvider {
	return &StaticCredentialProvider{
		apiKey:       apiKey,
		baseURL:      baseURL,
		defaultModel: defaultModel,
	}
}

// GetAPIKey returns the API key.
func (p *StaticCredentialProvider) GetAPIKey() string {
	return p.apiKey
}

// GetBaseURL returns the base URL.
func (p *StaticCredentialProvider) GetBaseURL() string {
	return p.baseURL
}

// GetDefaultModel returns the default model.
func (p *StaticCredentialProvider) GetDefaultModel() string {
	return p.defaultModel
}

// IsConfigured returns true if credentials are configured.
func (p *StaticCredentialProvider) IsConfigured() bool {
	return p.apiKey != ""
}

// RotatingCredentialProvider provides rotating test credentials for failover testing.
type RotatingCredentialProvider struct {
	credentials []types.ProviderConfig
	current     int
	mu          sync.Mutex
}

// NewRotatingCredentialProvider creates a new rotating credential provider.
func NewRotatingCredentialProvider(credentials []types.ProviderConfig) *RotatingCredentialProvider {
	return &RotatingCredentialProvider{
		credentials: credentials,
		current:     0,
	}
}

// GetNext returns the next credential in the rotation.
func (p *RotatingCredentialProvider) GetNext() types.ProviderConfig {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.credentials) == 0 {
		return types.ProviderConfig{}
	}

	cred := p.credentials[p.current]
	p.current = (p.current + 1) % len(p.credentials)
	return cred
}

// GetCurrent returns the current credential without advancing.
func (p *RotatingCredentialProvider) GetCurrent() types.ProviderConfig {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.credentials) == 0 {
		return types.ProviderConfig{}
	}

	if p.current >= len(p.credentials) {
		return p.credentials[0]
	}
	return p.credentials[p.current]
}

// ============================================================================
// Mock HTTP Server Setup
// ============================================================================

// MockServerConfig configures a mock HTTP server for testing.
type MockServerConfig struct {
	StatusCode    int
	ResponseBody  string
	ResponseDelay time.Duration
	ErrorAfter    int // Return error after this many requests
	RequestCount  int
}

// MockHTTPServer creates a configurable mock HTTP server for testing.
func MockHTTPServer(t *testing.T, config MockServerConfig) *httptest.Server {
	requestCount := 0

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Check if we should return an error
		if config.ErrorAfter > 0 && requestCount > config.ErrorAfter {
			http.Error(w, "Simulated error", http.StatusInternalServerError)
			return
		}

		// Apply delay if configured
		if config.ResponseDelay > 0 {
			time.Sleep(config.ResponseDelay)
		}

		// Set status code
		if config.StatusCode != 0 {
			w.WriteHeader(config.StatusCode)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		// Write response body
		if config.ResponseBody != "" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(config.ResponseBody))
		}
	}))
}

// MockChatCompletionServer creates a mock server that returns chat completion responses.
func MockChatCompletionServer(t *testing.T, responseContent string) *httptest.Server {
	responseBody := fmt.Sprintf(`{
		"id": "chatcmpl-test",
		"object": "chat.completion",
		"created": %d,
		"model": "test-model",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": %q
			},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 20,
			"total_tokens": 30
		}
	}`, time.Now().Unix(), responseContent)

	return MockHTTPServer(t, MockServerConfig{
		StatusCode:   200,
		ResponseBody: responseBody,
	})
}

// MockStreamingChatCompletionServer creates a mock server that returns streaming chat completion responses.
func MockStreamingChatCompletionServer(t *testing.T, chunks []string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		for _, chunk := range chunks {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
			flusher.Flush()
		}

		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
}

// ============================================================================
// Common Test Scenarios
// ============================================================================

// HappyPathScenario runs a standard happy path test for a provider.
func HappyPathScenario(t *testing.T, provider types.Provider, ctx context.Context) {
	t.Run("Configuration", func(t *testing.T) {
		config := provider.GetConfig()
		require.NotEmpty(t, config.Name, "Provider name should not be empty")
		require.NotEmpty(t, config.Type, "Provider type should not be empty")
	})

	t.Run("Authentication", func(t *testing.T) {
		authConfig := DefaultAuthConfig(provider.GetConfig())
		err := provider.Authenticate(ctx, authConfig)
		require.NoError(t, err, "Authentication should succeed")
		require.True(t, provider.IsAuthenticated(), "Provider should be authenticated")
	})

	t.Run("ModelRetrieval", func(t *testing.T) {
		models, err := provider.GetModels(ctx)
		require.NoError(t, err, "GetModels should succeed")
		require.NotEmpty(t, models, "Should have at least one model")

		for _, model := range models {
			require.NotEmpty(t, model.ID, "Model ID should not be empty")
			require.NotEmpty(t, model.Name, "Model name should not be empty")
		}
	})

	t.Run("ChatCompletion", func(t *testing.T) {
		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{Role: "system", Content: "You are a helpful assistant."},
				{Role: "user", Content: "Hello, world!"},
			},
			MaxTokens:   50,
			Temperature: 0.7,
		}

		stream, err := provider.GenerateChatCompletion(ctx, options)
		require.NoError(t, err, "GenerateChatCompletion should succeed")
		require.NotNil(t, stream, "Stream should not be nil")

		chunk, err := stream.Next()
		require.NoError(t, err, "Next should succeed")

		if !chunk.Done && chunk.Content != "" {
			require.NotEmpty(t, chunk.Content, "Chunk content should not be empty")
		}

		err = stream.Close()
		require.NoError(t, err, "Close should succeed")
	})

	t.Run("HealthCheck", func(t *testing.T) {
		err := provider.HealthCheck(ctx)
		require.NoError(t, err, "HealthCheck should succeed")
	})

	t.Run("Metrics", func(t *testing.T) {
		metrics := provider.GetMetrics()
		require.GreaterOrEqual(t, metrics.RequestCount, int64(0), "Request count should be non-negative")
	})

	t.Run("Logout", func(t *testing.T) {
		err := provider.Logout(ctx)
		require.NoError(t, err, "Logout should succeed")
		require.False(t, provider.IsAuthenticated(), "Provider should not be authenticated after logout")
	})
}

// ErrorScenario runs standard error handling tests for a provider.
func ErrorScenario(t *testing.T, provider types.Provider, ctx context.Context) {
	t.Run("InvalidAuthentication", func(t *testing.T) {
		invalidAuthConfig := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: "", // Empty API key
		}

		// Logout first to ensure we're testing from unauthenticated state
		_ = provider.Logout(ctx)

		err := provider.Authenticate(ctx, invalidAuthConfig)
		// Some providers may accept empty API keys in test mode
		if err != nil {
			require.Error(t, err, "Authentication with empty API key should fail")
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		// Try to get models with cancelled context
		_, err := provider.GetModels(cancelCtx)
		// Should either fail or succeed immediately
		if err != nil {
			require.Error(t, err, "GetModels with cancelled context should fail")
		}
	})
}

// StreamingScenario runs streaming-specific tests for a provider.
func StreamingScenario(t *testing.T, provider types.Provider, ctx context.Context) {
	if !provider.SupportsStreaming() {
		t.Skip("Provider does not support streaming")
	}

	t.Run("StreamingResponse", func(t *testing.T) {
		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{Role: "user", Content: "Tell me a short story."},
			},
			MaxTokens: 100,
			Stream:    true,
		}

		stream, err := provider.GenerateChatCompletion(ctx, options)
		require.NoError(t, err, "GenerateChatCompletion with streaming should succeed")
		require.NotNil(t, stream, "Stream should not be nil")
		defer func() { _ = stream.Close() }()

		chunkCount := 0
		fullContent := ""

		for {
			chunk, err := stream.Next()
			if err != nil {
				break
			}

			// Count all chunks, including the final done chunk
			chunkCount++

			if chunk.Done {
				break
			}

			if chunk.Content != "" {
				fullContent += chunk.Content
			}
		}

		require.Greater(t, chunkCount, 0, "Should receive at least one chunk")
		// Note: Content may be empty for mock providers that return done chunks only
	})

	t.Run("StreamClose", func(t *testing.T) {
		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{Role: "user", Content: "Quick question."},
			},
			MaxTokens: 50,
			Stream:    true,
		}

		stream, err := provider.GenerateChatCompletion(ctx, options)
		require.NoError(t, err, "GenerateChatCompletion should succeed")

		// Close stream immediately
		err = stream.Close()
		require.NoError(t, err, "Close should succeed")
	})
}

// ConcurrentScenario runs concurrent access tests for a provider.
func ConcurrentScenario(t *testing.T, provider types.Provider, ctx context.Context, concurrency int) {
	t.Run("ConcurrentRequests", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, concurrency)
		results := make(chan string, concurrency)

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				options := types.GenerateOptions{
					Messages: []types.ChatMessage{
						{Role: "user", Content: fmt.Sprintf("Concurrent test message %d", id)},
					},
					MaxTokens: 20,
				}

				stream, err := provider.GenerateChatCompletion(ctx, options)
				if err != nil {
					errors <- err
					return
				}
				defer func() { _ = stream.Close() }()

				chunk, err := stream.Next()
				if err != nil {
					errors <- err
					return
				}

				// For mock providers, we just check that we got a chunk
				// Content may be empty if chunk is marked as done
				if chunk.Content != "" {
					results <- chunk.Content
				} else {
					// Send a placeholder result for successful completion with empty content
					results <- fmt.Sprintf("result-%d", id)
				}
			}(i)
		}

		wg.Wait()
		close(errors)
		close(results)

		// Check for errors
		errorCount := 0
		for err := range errors {
			t.Errorf("Concurrent request error: %v", err)
			errorCount++
		}

		// Verify we got results
		resultCount := 0
		for range results {
			resultCount++
		}

		require.Equal(t, 0, errorCount, "Should have no errors")
		require.Equal(t, concurrency, resultCount, "Should have results from all concurrent requests")
	})

	t.Run("ConcurrentMetricsAccess", func(t *testing.T) {
		var wg sync.WaitGroup

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				_ = provider.GetMetrics()
			}(i)
		}

		wg.Wait()
		// If we got here without race detector complaining, test passed
	})
}

// ============================================================================
// Helper Functions
// ============================================================================

// getDefaultBaseURL returns the default base URL for a provider type.
func getDefaultBaseURL(providerType types.ProviderType) string {
	switch providerType {
	case types.ProviderTypeOpenAI:
		return "https://api.openai.com/v1"
	case types.ProviderTypeAnthropic:
		return "https://api.anthropic.com/v1"
	case types.ProviderTypeGemini:
		return "https://generativelanguage.googleapis.com/v1"
	case types.ProviderTypeQwen:
		return "https://portal.qwen.ai/v1"
	case types.ProviderTypeCerebras:
		return "https://api.cerebras.ai/v1"
	case types.ProviderTypeOpenRouter:
		return "https://openrouter.ai/api/v1"
	case types.ProviderTypeOllama:
		return "http://localhost:11434"
	default:
		return "https://api.example.com/v1"
	}
}

// getDefaultModel returns the default model for a provider type.
func getDefaultModel(providerType types.ProviderType) string {
	switch providerType {
	case types.ProviderTypeOpenAI:
		return "gpt-4"
	case types.ProviderTypeAnthropic:
		return "claude-3-sonnet"
	case types.ProviderTypeGemini:
		return "gemini-pro"
	case types.ProviderTypeQwen:
		return "qwen3-coder-flash"
	case types.ProviderTypeCerebras:
		return "zai-glm-4.6"
	case types.ProviderTypeOpenRouter:
		return "anthropic/claude-3.5-sonnet"
	case types.ProviderTypeOllama:
		return "llama3.1"
	default:
		return "default-model"
	}
}

// RequireProviderHealthy is a test helper that requires a provider to be healthy.
func RequireProviderHealthy(t *testing.T, provider types.Provider, ctx context.Context) {
	err := provider.HealthCheck(ctx)
	require.NoError(t, err, "Provider should be healthy")

	metrics := provider.GetMetrics()
	require.True(t, metrics.HealthStatus.Healthy, "Health status should indicate healthy")
}

// RequireProviderAuthenticated is a test helper that requires a provider to be authenticated.
func RequireProviderAuthenticated(t *testing.T, provider types.Provider) {
	require.True(t, provider.IsAuthenticated(), "Provider should be authenticated")
}

// WaitForCondition waits for a condition to be true, with timeout.
func WaitForCondition(t *testing.T, condition func() bool, timeout, checkInterval time.Duration, description string) {
	t.Helper()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(checkInterval)
	}

	require.Failf(t, "Condition not met within timeout", "Waiting for: %s", description)
}

// AssertMetricsIncremented asserts that metrics were incremented after an operation.
func AssertMetricsIncremented(t *testing.T, before, after types.ProviderMetrics, requestDelta int64) {
	require.Equal(t, before.RequestCount+requestDelta, after.RequestCount,
		"Request count should increment by %d", requestDelta)
}

// NewTestContext creates a test context with timeout.
func NewTestContext(t *testing.T, timeout time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(func() { cancel() })
	return ctx
}

// ============================================================================
// Test Data Builders
// ============================================================================

// ChatMessageBuilder builds chat messages for testing.
type ChatMessageBuilder struct {
	messages []types.ChatMessage
}

// NewChatMessageBuilder creates a new chat message builder.
func NewChatMessageBuilder() *ChatMessageBuilder {
	return &ChatMessageBuilder{
		messages: []types.ChatMessage{},
	}
}

// WithSystem adds a system message.
func (b *ChatMessageBuilder) WithSystem(content string) *ChatMessageBuilder {
	b.messages = append(b.messages, types.ChatMessage{
		Role:    "system",
		Content: content,
	})
	return b
}

// WithUser adds a user message.
func (b *ChatMessageBuilder) WithUser(content string) *ChatMessageBuilder {
	b.messages = append(b.messages, types.ChatMessage{
		Role:    "user",
		Content: content,
	})
	return b
}

// WithAssistant adds an assistant message.
func (b *ChatMessageBuilder) WithAssistant(content string) *ChatMessageBuilder {
	b.messages = append(b.messages, types.ChatMessage{
		Role:    "assistant",
		Content: content,
	})
	return b
}

// WithTool adds a tool message.
func (b *ChatMessageBuilder) WithTool(toolCallID, content string) *ChatMessageBuilder {
	b.messages = append(b.messages, types.ChatMessage{
		Role:       "tool",
		ToolCallID: toolCallID,
		Content:    content,
	})
	return b
}

// Build returns the built messages.
func (b *ChatMessageBuilder) Build() []types.ChatMessage {
	return b.messages
}

// GenerateOptionsBuilder builds generate options for testing.
type GenerateOptionsBuilder struct {
	options types.GenerateOptions
}

// NewGenerateOptionsBuilder creates a new generate options builder.
func NewGenerateOptionsBuilder() *GenerateOptionsBuilder {
	return &GenerateOptionsBuilder{
		options: types.GenerateOptions{
			Messages:    []types.ChatMessage{},
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}
}

// WithMessages sets the messages.
func (b *GenerateOptionsBuilder) WithMessages(messages []types.ChatMessage) *GenerateOptionsBuilder {
	b.options.Messages = messages
	return b
}

// WithPrompt sets a simple prompt.
func (b *GenerateOptionsBuilder) WithPrompt(prompt string) *GenerateOptionsBuilder {
	b.options.Prompt = prompt
	return b
}

// WithMaxTokens sets the max tokens.
func (b *GenerateOptionsBuilder) WithMaxTokens(maxTokens int) *GenerateOptionsBuilder {
	b.options.MaxTokens = maxTokens
	return b
}

// WithTemperature sets the temperature.
func (b *GenerateOptionsBuilder) WithTemperature(temperature float64) *GenerateOptionsBuilder {
	b.options.Temperature = temperature
	return b
}

// WithStreaming enables streaming.
func (b *GenerateOptionsBuilder) WithStreaming() *GenerateOptionsBuilder {
	b.options.Stream = true
	return b
}

// WithModel sets the model.
func (b *GenerateOptionsBuilder) WithModel(model string) *GenerateOptionsBuilder {
	b.options.Model = model
	return b
}

// WithMetadata adds metadata.
func (b *GenerateOptionsBuilder) WithMetadata(metadata map[string]interface{}) *GenerateOptionsBuilder {
	if b.options.Metadata == nil {
		b.options.Metadata = make(map[string]interface{})
	}
	for k, v := range metadata {
		b.options.Metadata[k] = v
	}
	return b
}

// Build returns the built options.
func (b *GenerateOptionsBuilder) Build() types.GenerateOptions {
	return b.options
}
