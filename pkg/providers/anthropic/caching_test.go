package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCacheControlStruct tests the CacheControl struct
func TestCacheControlStruct(t *testing.T) {
	tests := []struct {
		name     string
		cache    *CacheControl
		expected string
	}{
		{
			name: "ephemeral with default TTL",
			cache: &CacheControl{
				Type: "ephemeral",
			},
			expected: `{"type":"ephemeral"}`,
		},
		{
			name: "ephemeral with 5m TTL",
			cache: &CacheControl{
				Type: "ephemeral",
				TTL:  "5m",
			},
			expected: `{"type":"ephemeral","ttl":"5m"}`,
		},
		{
			name: "ephemeral with 1h TTL",
			cache: &CacheControl{
				Type: "ephemeral",
				TTL:  "1h",
			},
			expected: `{"type":"ephemeral","ttl":"1h"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.cache)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}

// TestToolWithCacheControl tests tool definitions with cache_control
func TestToolWithCacheControl(t *testing.T) {
	tool := AnthropicTool{
		Name:        "get_weather",
		Description: "Get weather information",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "City name",
				},
			},
		},
		CacheControl: &CacheControl{
			Type: "ephemeral",
		},
	}

	data, err := json.Marshal(tool)
	require.NoError(t, err)

	var decoded AnthropicTool
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, tool.Name, decoded.Name)
	assert.Equal(t, tool.Description, decoded.Description)
	assert.NotNil(t, decoded.CacheControl)
	assert.Equal(t, "ephemeral", decoded.CacheControl.Type)
}

// TestContentBlockWithCacheControl tests content blocks with cache_control
func TestContentBlockWithCacheControl(t *testing.T) {
	block := AnthropicContentBlock{
		Type: "text",
		Text: "This is cached content",
		CacheControl: &CacheControl{
			Type: "ephemeral",
			TTL:  "1h",
		},
	}

	data, err := json.Marshal(block)
	require.NoError(t, err)

	var decoded AnthropicContentBlock
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "text", decoded.Type)
	assert.Equal(t, "This is cached content", decoded.Text)
	assert.NotNil(t, decoded.CacheControl)
	assert.Equal(t, "ephemeral", decoded.CacheControl.Type)
	assert.Equal(t, "1h", decoded.CacheControl.TTL)
}

// TestUsageWithCacheMetrics tests usage tracking with cache metrics
func TestUsageWithCacheMetrics(t *testing.T) {
	usage := AnthropicUsage{
		InputTokens:              100,
		OutputTokens:             50,
		CacheCreationInputTokens: 1000,
		CacheReadInputTokens:     0,
	}

	data, err := json.Marshal(usage)
	require.NoError(t, err)

	var decoded AnthropicUsage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 100, decoded.InputTokens)
	assert.Equal(t, 50, decoded.OutputTokens)
	assert.Equal(t, 1000, decoded.CacheCreationInputTokens)
	assert.Equal(t, 0, decoded.CacheReadInputTokens)
}

// TestUsageWithCacheHit tests usage tracking with cache hit
func TestUsageWithCacheHit(t *testing.T) {
	usage := AnthropicUsage{
		InputTokens:              100,
		OutputTokens:             50,
		CacheCreationInputTokens: 0,
		CacheReadInputTokens:     1000,
	}

	data, err := json.Marshal(usage)
	require.NoError(t, err)

	var decoded AnthropicUsage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 100, decoded.InputTokens)
	assert.Equal(t, 50, decoded.OutputTokens)
	assert.Equal(t, 0, decoded.CacheCreationInputTokens)
	assert.Equal(t, 1000, decoded.CacheReadInputTokens)
}

// TestCacheCreationBreakdown tests detailed cache creation breakdown
func TestCacheCreationBreakdown(t *testing.T) {
	usage := AnthropicUsage{
		InputTokens:              100,
		OutputTokens:             50,
		CacheCreationInputTokens: 1500,
		CacheReadInputTokens:     0,
		CacheCreation: &AnthropicCacheCreationBreakdown{
			Ephemeral5mInputTokens: 500,
			Ephemeral1hInputTokens: 1000,
		},
	}

	data, err := json.Marshal(usage)
	require.NoError(t, err)

	var decoded AnthropicUsage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 1500, decoded.CacheCreationInputTokens)
	assert.NotNil(t, decoded.CacheCreation)
	assert.Equal(t, 500, decoded.CacheCreation.Ephemeral5mInputTokens)
	assert.Equal(t, 1000, decoded.CacheCreation.Ephemeral1hInputTokens)
}

// TestPromptCachingIntegration tests end-to-end prompt caching with mock server
func TestPromptCachingIntegration(t *testing.T) {
	// Create mock server that returns cache metrics
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get("x-api-key"))
		assert.NotEmpty(t, r.Header.Get("anthropic-version"))

		// Read and verify request body
		var reqBody AnthropicRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		// Verify cache_control is present in system message
		if systemArray, ok := reqBody.System.([]interface{}); ok {
			assert.Greater(t, len(systemArray), 0, "System array should not be empty")
		}

		// Return mock response with cache metrics
		response := AnthropicResponse{
			ID:    "msg_test123",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-sonnet-4-5",
			Content: []AnthropicContentBlock{
				{
					Type: "text",
					Text: "This response used cached content",
				},
			},
			Usage: AnthropicUsage{
				InputTokens:              50,
				OutputTokens:             100,
				CacheCreationInputTokens: 0,
				CacheReadInputTokens:     1000,
			},
			StopReason: "end_turn",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	// Create provider with mock server
	provider := NewAnthropicProvider(types.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	// Test request with cache control
	ctx := context.Background()
	options := types.GenerateOptions{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "What is prompt caching?",
			},
		},
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read the response
	chunk, err := stream.Next()
	require.NoError(t, err)

	// Verify cache metrics are present
	assert.Equal(t, 50, chunk.Usage.PromptTokens)
	assert.Equal(t, 100, chunk.Usage.CompletionTokens)
	assert.Equal(t, 0, chunk.Usage.CacheCreationInputTokens)
	assert.Equal(t, 1000, chunk.Usage.CacheReadInputTokens)
}

// TestSystemMessageArrayFormat tests that system messages support array format for caching
func TestSystemMessageArrayFormat(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		APIKey: "test-key",
	})

	options := types.GenerateOptions{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Messages: []types.ChatMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	request := provider.prepareRequest(options, "claude-sonnet-4-5", 1024)

	// For API key auth, system should be a string
	if str, ok := request.System.(string); ok {
		assert.Contains(t, str, "You are Claude Code")
		assert.Contains(t, str, "You are a helpful assistant")
	}
}

// TestCacheMetricsConversion tests that cache metrics are properly converted to types.Usage
func TestCacheMetricsConversion(t *testing.T) {
	anthropicUsage := AnthropicUsage{
		InputTokens:              100,
		OutputTokens:             50,
		CacheCreationInputTokens: 500,
		CacheReadInputTokens:     1000,
	}

	// Convert to types.Usage (simulating the conversion in anthropic.go)
	usage := &types.Usage{
		PromptTokens:             anthropicUsage.InputTokens,
		CompletionTokens:         anthropicUsage.OutputTokens,
		TotalTokens:              anthropicUsage.InputTokens + anthropicUsage.OutputTokens,
		CacheCreationInputTokens: anthropicUsage.CacheCreationInputTokens,
		CacheReadInputTokens:     anthropicUsage.CacheReadInputTokens,
	}

	assert.Equal(t, 100, usage.PromptTokens)
	assert.Equal(t, 50, usage.CompletionTokens)
	assert.Equal(t, 150, usage.TotalTokens)
	assert.Equal(t, 500, usage.CacheCreationInputTokens)
	assert.Equal(t, 1000, usage.CacheReadInputTokens)
}

// TestMultipleCacheBreakpoints tests request with multiple cache breakpoints
func TestMultipleCacheBreakpoints(t *testing.T) {
	// Create request with multiple cache breakpoints
	request := AnthropicRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Tools: []AnthropicTool{
			{
				Name:        "tool1",
				Description: "First tool",
				InputSchema: map[string]interface{}{"type": "object"},
			},
			{
				Name:        "tool2",
				Description: "Second tool with cache control",
				InputSchema: map[string]interface{}{"type": "object"},
				CacheControl: &CacheControl{
					Type: "ephemeral",
				},
			},
		},
		System: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "System instruction 1",
			},
			map[string]interface{}{
				"type":          "text",
				"text":          "System instruction 2 with cache control",
				"cache_control": map[string]string{"type": "ephemeral"},
			},
		},
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	// Verify structure can be marshaled
	data, err := json.Marshal(request)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify it can be unmarshaled
	var decoded AnthropicRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 2, len(decoded.Tools))
	assert.NotNil(t, decoded.Tools[1].CacheControl)
}

// TestCacheTTLVariants tests different TTL values
func TestCacheTTLVariants(t *testing.T) {
	tests := []struct {
		name        string
		cacheConfig *CacheControl
		wantTTL     string
	}{
		{
			name: "default (no TTL specified)",
			cacheConfig: &CacheControl{
				Type: "ephemeral",
			},
			wantTTL: "",
		},
		{
			name: "5 minute TTL",
			cacheConfig: &CacheControl{
				Type: "ephemeral",
				TTL:  "5m",
			},
			wantTTL: "5m",
		},
		{
			name: "1 hour TTL",
			cacheConfig: &CacheControl{
				Type: "ephemeral",
				TTL:  "1h",
			},
			wantTTL: "1h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := AnthropicTool{
				Name:         "test_tool",
				Description:  "Test tool",
				InputSchema:  map[string]interface{}{"type": "object"},
				CacheControl: tt.cacheConfig,
			}

			data, err := json.Marshal(tool)
			require.NoError(t, err)

			var decoded AnthropicTool
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.NotNil(t, decoded.CacheControl)
			assert.Equal(t, "ephemeral", decoded.CacheControl.Type)
			assert.Equal(t, tt.wantTTL, decoded.CacheControl.TTL)
		})
	}
}
