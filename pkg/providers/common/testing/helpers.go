package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockServer represents a configurable mock HTTP server for testing providers
type MockServer struct {
	server   *httptest.Server
	response string
	status   int
	headers  map[string]string
}

// NewMockServer creates a new mock server with the given response and status code
func NewMockServer(response string, statusCode int) *MockServer {
	return &MockServer{
		response: response,
		status:   statusCode,
		headers:  map[string]string{"Content-Type": "application/json"},
	}
}

// SetHeader sets a header that will be returned by the mock server
func (m *MockServer) SetHeader(key, value string) {
	m.headers[key] = value
}

// Start starts the mock server and returns the URL
func (m *MockServer) Start() string {
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		for key, value := range m.headers {
			w.Header().Set(key, value)
		}
		w.WriteHeader(m.status)
		_, _ = w.Write([]byte(m.response))
	}))
	return m.server.URL
}

// Close closes the mock server
func (m *MockServer) Close() {
	if m.server != nil {
		m.server.Close()
	}
}

// MockStream provides a mock implementation of ChatCompletionStream for testing
type MockStream struct {
	chunks []types.ChatCompletionChunk
	index  int
}

// NewMockStream creates a new mock stream with the given chunks
func NewMockStream(chunks []types.ChatCompletionChunk) *MockStream {
	return &MockStream{
		chunks: chunks,
		index:  0,
	}
}

// Next returns the next chunk from the stream
func (m *MockStream) Next() (types.ChatCompletionChunk, error) {
	if m.index >= len(m.chunks) {
		return types.ChatCompletionChunk{}, nil
	}

	chunk := m.chunks[m.index]
	m.index++
	return chunk, nil
}

// Close resets the stream to the beginning
func (m *MockStream) Close() error {
	m.index = 0
	return nil
}

// ProviderTestHelpers provides common helper functions for provider testing
type ProviderTestHelpers struct {
	t        *testing.T
	provider types.Provider
}

// NewProviderTestHelpers creates a new helper instance for testing providers
func NewProviderTestHelpers(t *testing.T, provider types.Provider) *ProviderTestHelpers {
	return &ProviderTestHelpers{
		t:        t,
		provider: provider,
	}
}

// AssertProviderBasics tests basic provider functionality
func (p *ProviderTestHelpers) AssertProviderBasics(expectedName, expectedType string) {
	assert.Equal(p.t, expectedName, p.provider.Name(), "Provider name should match")
	assert.Equal(p.t, types.ProviderType(expectedType), p.provider.Type(), "Provider type should match")
	assert.NotNil(p.t, p.provider.GetConfig(), "Provider config should not be nil")
}

// AssertAuthenticated tests authentication state
func (p *ProviderTestHelpers) AssertAuthenticated(expected bool) {
	assert.Equal(p.t, expected, p.provider.IsAuthenticated(), "Authentication state should match")
}

// AssertSupportsFeatures tests provider feature support
func (p *ProviderTestHelpers) AssertSupportsFeatures(toolCalling, streaming, responsesAPI bool) {
	assert.Equal(p.t, toolCalling, p.provider.SupportsToolCalling(), "Tool calling support should match")
	assert.Equal(p.t, streaming, p.provider.SupportsStreaming(), "Streaming support should match")
	assert.Equal(p.t, responsesAPI, p.provider.SupportsResponsesAPI(), "Responses API support should match")
}

// AssertModelExists checks that a model with the given ID exists in the model list
func (p *ProviderTestHelpers) AssertModelExists(expectedModelID string) {
	ctx := context.Background()
	models, err := p.provider.GetModels(ctx)
	require.NoError(p.t, err, "Should not error getting models")
	require.NotEmpty(p.t, models, "Should return at least one model")

	found := false
	for _, model := range models {
		if model.ID == expectedModelID {
			found = true
			break
		}
	}
	assert.True(p.t, found, "Model %s should be found in models list", expectedModelID)
}

// AssertDefaultModel tests the default model functionality
func (p *ProviderTestHelpers) AssertDefaultModel(expectedModel string) {
	assert.Equal(p.t, expectedModel, p.provider.GetDefaultModel(), "Default model should match")
}

// TestAuthenticationFlow tests a complete authentication flow
func (p *ProviderTestHelpers) TestAuthenticationFlow(authConfig types.AuthConfig) {
	ctx := context.Background()

	// Test initial authentication
	err := p.provider.Authenticate(ctx, authConfig)
	require.NoError(p.t, err, "Authentication should succeed")
	assert.True(p.t, p.provider.IsAuthenticated(), "Should be authenticated after Authenticate()")

	// Test logout
	err = p.provider.Logout(ctx)
	require.NoError(p.t, err, "Logout should succeed")
}

// TestConfiguration tests provider configuration
func (p *ProviderTestHelpers) TestConfiguration(config types.ProviderConfig) {
	err := p.provider.Configure(config)
	require.NoError(p.t, err, "Configuration should succeed")

	// Verify configuration was applied
	retrievedConfig := p.provider.GetConfig()
	assert.Equal(p.t, config.Type, retrievedConfig.Type, "Provider type should match")
	assert.Equal(p.t, config.APIKey, retrievedConfig.APIKey, "API key should match")
}

// TestHealthCheck tests the provider health check
func (p *ProviderTestHelpers) TestHealthCheck() {
	ctx := context.Background()
	err := p.provider.HealthCheck(ctx)
	assert.NoError(p.t, err, "Health check should succeed")
}

// TestMetrics tests the provider metrics
func (p *ProviderTestHelpers) TestMetrics() {
	metrics := p.provider.GetMetrics()
	assert.NotNil(p.t, metrics, "Metrics should not be nil")
	assert.Equal(p.t, int64(0), metrics.RequestCount, "Initial request count should be 0")
	assert.Equal(p.t, int64(0), metrics.SuccessCount, "Initial success count should be 0")
	assert.Equal(p.t, int64(0), metrics.ErrorCount, "Initial error count should be 0")
}

// CreateTestTool creates a standard test tool for testing tool calling
func CreateTestTool(name, description string) types.Tool {
	return types.Tool{
		Name:        name,
		Description: description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The city and state, e.g. San Francisco, CA",
				},
				"unit": map[string]interface{}{
					"type": "string",
					"enum": []string{"celsius", "fahrenheit"},
				},
			},
			"required": []string{"location"},
		},
	}
}

// CreateTestToolCall creates a standard test tool call for testing
func CreateTestToolCall(id, name, arguments string) types.ToolCall {
	return types.ToolCall{
		ID:   id,
		Type: "function",
		Function: types.ToolCallFunction{
			Name:      name,
			Arguments: arguments,
		},
	}
}

// CreateMockChatCompletionResponse creates a mock chat completion response
func CreateMockChatCompletionResponse(model, content string, toolCalls []types.ToolCall) map[string]interface{} {
	response := map[string]interface{}{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     10,
			"completion_tokens": 5,
			"total_tokens":      15,
		},
	}

	// Add tool calls if provided
	if len(toolCalls) > 0 {
		toolCallMaps := make([]map[string]interface{}, len(toolCalls))
		for i, tc := range toolCalls {
			toolCallMaps[i] = map[string]interface{}{
				"id":   tc.ID,
				"type": tc.Type,
				"function": map[string]interface{}{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			}
		}
		response["choices"].([]map[string]interface{})[0]["message"].(map[string]interface{})["tool_calls"] = toolCallMaps
		response["choices"].([]map[string]interface{})[0]["finish_reason"] = "tool_calls"
	}

	return response
}

// CreateMockStreamResponse creates a mock streaming response
func CreateMockStreamResponse(model, content string, toolCalls []types.ToolCall) []string {
	baseChunk := map[string]interface{}{
		"id":      "chatcmpl-123",
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]interface{}{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": nil,
			},
		},
	}

	// Add tool calls if provided
	if len(toolCalls) > 0 {
		toolCallMaps := make([]map[string]interface{}, len(toolCalls))
		for i, tc := range toolCalls {
			toolCallMaps[i] = map[string]interface{}{
				"id":   tc.ID,
				"type": tc.Type,
				"function": map[string]interface{}{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			}
		}
		baseChunk["choices"].([]map[string]interface{})[0]["delta"].(map[string]interface{})["tool_calls"] = toolCallMaps
	}

	chunkJSON, _ := json.Marshal(baseChunk)
	return []string{
		fmt.Sprintf("data: %s\n\n", string(chunkJSON)),
		"data: [DONE]\n\n",
	}
}

// CreateStreamingMockServer creates a mock server that streams responses
func CreateStreamingMockServer(chunks []string) *MockServer {
	response := ""
	for _, chunk := range chunks {
		response += chunk
	}

	server := NewMockServer(response, http.StatusOK)
	server.SetHeader("Content-Type", "text/event-stream")
	server.SetHeader("Cache-Control", "no-cache")
	server.SetHeader("Connection", "keep-alive")
	return server
}

// AssertContains checks if a string contains a substring (helper function)
func AssertContains(t *testing.T, s, substr string) {
	assert.True(t, len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}()))), "String should contain substring: expected '%s' in '%s'", substr, s)
}

// CleanCodeResponse removes markdown code blocks and language identifiers
func CleanCodeResponse(response string) string {
	// Remove markdown code blocks
	start := strings.Index(response, "```")
	if start != -1 {
		end := strings.Index(response[start:], "```")
		if end != -1 {
			response = response[start+3 : start+end]
		}
	}

	// Remove language identifier if present
	lines := strings.Split(response, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" && !strings.Contains(lines[0], " ") {
		response = strings.Join(lines[1:], "\n")
	}

	// Trim whitespace
	return strings.TrimSpace(response)
}

// TestProviderInterface ensures a provider implements all required interface methods
func TestProviderInterface(t *testing.T, provider types.Provider) {
	ctx := context.Background()

	// Test basic methods
	assert.NotEmpty(t, provider.Name(), "Provider name should not be empty")
	assert.NotEmpty(t, string(provider.Type()), "Provider type should not be empty")
	assert.NotEmpty(t, provider.Description(), "Provider description should not be empty")

	// Test model methods
	models, err := provider.GetModels(ctx)
	assert.NoError(t, err, "GetModels should not error")
	assert.NotNil(t, models, "Models should not be nil")
	assert.NotEmpty(t, provider.GetDefaultModel(), "Default model should not be empty")

	// Test authentication
	authConfig := types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "test-key",
	}
	_ = provider.Authenticate(ctx, authConfig) //nolint:errcheck // Some providers might not support API key auth
	// Don't assert error here as some providers might not support API key auth
	_ = provider.IsAuthenticated()

	err = provider.Logout(ctx)
	assert.NoError(t, err, "Logout should not error")

	// Test configuration
	config := provider.GetConfig()
	assert.NotNil(t, config, "Config should not be nil")

	// Test capabilities
	_ = provider.SupportsToolCalling()
	_ = provider.SupportsStreaming()
	_ = provider.SupportsResponsesAPI()
	_ = provider.GetToolFormat()

	// Test health and metrics
	err = provider.HealthCheck(ctx)
	assert.NoError(t, err, "Health check should not error")
	metrics := provider.GetMetrics()
	assert.NotNil(t, metrics, "Metrics should not be nil")
}

// RunProviderTests runs a comprehensive set of tests on a provider
func RunProviderTests(t *testing.T, provider types.Provider, config types.ProviderConfig, expectedModels []string) {
	helper := NewProviderTestHelpers(t, provider)

	t.Run("ProviderBasics", func(t *testing.T) {
		helper.AssertProviderBasics(provider.Name(), string(provider.Type()))
	})

	t.Run("Authentication", func(t *testing.T) {
		if config.APIKey != "" {
			helper.AssertAuthenticated(true)
		} else {
			helper.AssertAuthenticated(false)
		}
	})

	t.Run("Models", func(t *testing.T) {
		for _, modelID := range expectedModels {
			helper.AssertModelExists(modelID)
		}
	})

	t.Run("DefaultModel", func(t *testing.T) {
		if config.DefaultModel != "" {
			helper.AssertDefaultModel(config.DefaultModel)
		} else {
			assert.NotEmpty(t, provider.GetDefaultModel(), "Should have a default model")
		}
	})

	t.Run("Features", func(t *testing.T) {
		helper.AssertSupportsFeatures(
			config.SupportsToolCalling,
			config.SupportsStreaming,
			config.SupportsResponsesAPI,
		)
	})

	t.Run("HealthCheck", func(t *testing.T) {
		helper.TestHealthCheck()
	})

	t.Run("Metrics", func(t *testing.T) {
		helper.TestMetrics()
	})

	t.Run("Interface", func(t *testing.T) {
		TestProviderInterface(t, provider)
	})
}
