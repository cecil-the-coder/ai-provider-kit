package ollama

import (
	"context"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestNewOllamaProvider(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)

	assert.NotNil(t, provider)
	assert.Equal(t, "Ollama", provider.Name())
	assert.Equal(t, types.ProviderTypeOllama, provider.Type())
	assert.Equal(t, "Ollama local and cloud model inference", provider.Description())
	assert.Equal(t, "http://localhost:11434", provider.GetConfig().BaseURL)
}

func TestOllamaProvider_DefaultValues(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOllama,
		Name: "ollama-test",
	}

	provider := NewOllamaProvider(config)

	assert.NotNil(t, provider)
	assert.Equal(t, "http://localhost:11434", provider.GetConfig().BaseURL)
	assert.Equal(t, "llama3.1:8b", provider.GetDefaultModel())
}

func TestOllamaProvider_SupportsFeatures(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)

	assert.True(t, provider.SupportsToolCalling())
	assert.True(t, provider.SupportsStreaming())
	assert.False(t, provider.SupportsResponsesAPI())
	assert.Equal(t, types.ToolFormatOpenAI, provider.GetToolFormat())
}

func TestOllamaProvider_IsAuthenticated(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		apiKey      string
		expectAuth  bool
		description string
	}{
		{
			name:        "Local endpoint - no auth required",
			baseURL:     "http://localhost:11434",
			apiKey:      "",
			expectAuth:  true,
			description: "Local Ollama doesn't require authentication",
		},
		{
			name:        "Cloud endpoint - no API key",
			baseURL:     "https://api.ollama.com",
			apiKey:      "",
			expectAuth:  false,
			description: "Cloud endpoint requires API key",
		},
		{
			name:        "Cloud endpoint - with API key",
			baseURL:     "https://api.ollama.com",
			apiKey:      "test-key",
			expectAuth:  true,
			description: "Cloud endpoint with API key is authenticated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := types.ProviderConfig{
				Type:    types.ProviderTypeOllama,
				Name:    "ollama-test",
				BaseURL: tt.baseURL,
				APIKey:  tt.apiKey,
			}

			provider := NewOllamaProvider(config)
			assert.Equal(t, tt.expectAuth, provider.IsAuthenticated(), tt.description)
		})
	}
}

func TestOllamaProvider_isCloudEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		expectCloud bool
	}{
		{
			name:        "Localhost",
			baseURL:     "http://localhost:11434",
			expectCloud: false,
		},
		{
			name:        "127.0.0.1",
			baseURL:     "http://127.0.0.1:11434",
			expectCloud: false,
		},
		{
			name:        "Cloud endpoint",
			baseURL:     "https://api.ollama.com",
			expectCloud: true,
		},
		{
			name:        "Cloud endpoint with path",
			baseURL:     "https://cloud.ollama.com/v1",
			expectCloud: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := types.ProviderConfig{
				Type:    types.ProviderTypeOllama,
				Name:    "ollama-test",
				BaseURL: tt.baseURL,
			}

			provider := NewOllamaProvider(config)
			assert.Equal(t, tt.expectCloud, provider.isCloudEndpoint())
		})
	}
}

func TestOllamaProvider_Configure(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)

	// Update configuration
	newConfig := types.ProviderConfig{
		Type:         types.ProviderTypeOllama,
		Name:         "ollama-updated",
		BaseURL:      "http://192.168.1.100:11434",
		DefaultModel: "mistral:7b",
	}

	err := provider.Configure(newConfig)

	assert.NoError(t, err)
	assert.Equal(t, "http://192.168.1.100:11434", provider.GetConfig().BaseURL)
	assert.Equal(t, "mistral:7b", provider.GetDefaultModel())
}

func TestOllamaProvider_GetMetrics(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)
	metrics := provider.GetMetrics()

	// Initial metrics should be zero
	assert.Equal(t, int64(0), metrics.RequestCount)
	assert.Equal(t, int64(0), metrics.SuccessCount)
	assert.Equal(t, int64(0), metrics.ErrorCount)
}

func TestOllamaProvider_Authenticate(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "https://api.ollama.com",
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Test API key authentication
	authConfig := types.AuthConfig{
		Method:       types.AuthMethodAPIKey,
		APIKey:       "test-key-123",
		BaseURL:      "https://api.ollama.com",
		DefaultModel: "llama3.1:8b",
	}

	err := provider.Authenticate(ctx, authConfig)
	assert.NoError(t, err)
	assert.True(t, provider.IsAuthenticated())
	assert.Equal(t, "test-key-123", provider.GetConfig().APIKey)

	// Test invalid auth method
	invalidConfig := types.AuthConfig{
		Method: types.AuthMethodOAuth,
	}
	err = provider.Authenticate(ctx, invalidConfig)
	assert.Error(t, err)
}

func TestOllamaProvider_Logout(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "https://api.ollama.com",
		APIKey:  "test-key",
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Verify initially authenticated
	assert.True(t, provider.IsAuthenticated())

	// Logout
	err := provider.Logout(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "", provider.GetConfig().APIKey)
}

func TestOllamaProvider_InvokeServerTool(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Tool invocation should return not implemented error
	result, err := provider.InvokeServerTool(ctx, "test_tool", map[string]interface{}{"key": "value"})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestOllamaProvider_MessageConversion_WithImages(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)

	// Create message with image parts
	messages := []types.ChatMessage{
		{
			Role:    "user",
			Content: "What's in this image?",
			Parts: []types.ContentPart{
				{
					Type: types.ContentTypeText,
					Text: "What's in this image?",
				},
				{
					Type: types.ContentTypeImage,
					Source: &types.MediaSource{
						Type: types.MediaSourceBase64,
						Data: "base64encodedimagedata",
					},
				},
			},
		},
	}

	// Convert messages
	ollamaMessages := provider.convertMessages(messages)

	// Verify image was extracted
	assert.Len(t, ollamaMessages, 1)
	assert.Equal(t, "What's in this image?", ollamaMessages[0].Content)
	assert.Len(t, ollamaMessages[0].Images, 1)
	assert.Equal(t, "base64encodedimagedata", ollamaMessages[0].Images[0])
}

func TestOllamaProvider_MessageConversion_WithToolResults(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)

	// Create messages with tool calls
	messages := []types.ChatMessage{
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []types.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location":"SF"}`,
					},
				},
			},
		},
		{
			Role:    "tool",
			Content: `{"temperature": 72, "condition": "sunny"}`,
		},
	}

	// Convert messages
	ollamaMessages := provider.convertMessages(messages)

	// Verify tool calls were converted
	assert.Len(t, ollamaMessages, 2)
	assert.Len(t, ollamaMessages[0].ToolCalls, 1)
	assert.Equal(t, "call_123", ollamaMessages[0].ToolCalls[0].ID)
	assert.Equal(t, "get_weather", ollamaMessages[0].ToolCalls[0].Function.Name)
}
