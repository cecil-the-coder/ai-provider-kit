package base

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
)

// TestNewBaseProvider tests the creation of a new base provider
func TestNewBaseProvider(t *testing.T) {
	t.Run("ValidCreation", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			Name:   "test-provider",
			APIKey: "test-key",
		}
		client := &http.Client{Timeout: 30 * time.Second}
		logger := log.New(bytes.NewBuffer(nil), "", log.LstdFlags)

		provider := NewBaseProvider("test-provider", config, client, logger)

		assert.Equal(t, "test-provider", provider.Name())
		assert.Equal(t, types.ProviderTypeOpenAI, provider.Type())
		assert.Equal(t, config, provider.GetConfig())
		assert.Equal(t, client, provider.client)
		assert.Equal(t, logger, provider.logger)
		assert.Equal(t, int64(0), provider.GetMetrics().RequestCount)
		assert.Equal(t, int64(0), provider.GetMetrics().SuccessCount)
		assert.Equal(t, int64(0), provider.GetMetrics().ErrorCount)
		assert.Equal(t, int64(0), provider.GetMetrics().TokensUsed)
	})

	t.Run("NilClient", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
			Name: "test-provider",
		}

		provider := NewBaseProvider("test-provider", config, nil, nil)

		assert.Equal(t, "test-provider", provider.Name())
		assert.Nil(t, provider.client)
		assert.Nil(t, provider.logger)
	})

	t.Run("NilLogger", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
			Name: "test-provider",
		}
		client := &http.Client{Timeout: 30 * time.Second}

		provider := NewBaseProvider("test-provider", config, client, nil)

		assert.Equal(t, "test-provider", provider.Name())
		assert.Equal(t, client, provider.client)
		assert.Nil(t, provider.logger)
	})
}

// TestBaseProvider_Name tests the Name method
func TestBaseProvider_Name(t *testing.T) {
	provider := NewBaseProvider("test-name", types.ProviderConfig{}, nil, nil)
	assert.Equal(t, "test-name", provider.Name())
}

// TestBaseProvider_Type tests the Type method
func TestBaseProvider_Type(t *testing.T) {
	tests := []struct {
		name     string
		provider *BaseProvider
		expected types.ProviderType
	}{
		{
			name: "OpenAI",
			provider: NewBaseProvider("test", types.ProviderConfig{
				Type: types.ProviderTypeOpenAI,
			}, nil, nil),
			expected: types.ProviderTypeOpenAI,
		},
		{
			name: "Anthropic",
			provider: NewBaseProvider("test", types.ProviderConfig{
				Type: types.ProviderTypeAnthropic,
			}, nil, nil),
			expected: types.ProviderTypeAnthropic,
		},
		{
			name: "Gemini",
			provider: NewBaseProvider("test", types.ProviderConfig{
				Type: types.ProviderTypeGemini,
			}, nil, nil),
			expected: types.ProviderTypeGemini,
		},
		{
			name: "Empty",
			provider: NewBaseProvider("test", types.ProviderConfig{
				Type: "",
			}, nil, nil),
			expected: types.ProviderType(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.provider.Type())
		})
	}
}

// TestBaseProvider_Description tests the Description method
func TestBaseProvider_Description(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)
	assert.Equal(t, "Base provider implementation", provider.Description())
}

// TestBaseProvider_Configure tests the Configure method
func TestBaseProvider_Configure(t *testing.T) {
	t.Run("ValidConfiguration", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := log.New(&logBuffer, "", log.LstdFlags)

		provider := NewBaseProvider("test", types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}, nil, logger)

		newConfig := types.ProviderConfig{
			Type:   types.ProviderTypeAnthropic,
			APIKey: "new-key",
		}

		err := provider.Configure(newConfig)

		assert.NoError(t, err)
		assert.Equal(t, newConfig, provider.GetConfig())
		assert.Contains(t, logBuffer.String(), "type changed from openai to anthropic")
	})

	t.Run("ConcurrentConfiguration", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := log.New(&logBuffer, "", log.LstdFlags)
		provider := NewBaseProvider("test", types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}, nil, logger)

		var wg sync.WaitGroup
		numGoroutines := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				config := types.ProviderConfig{
					Type:   types.ProviderType(fmt.Sprintf("provider-%d", index)),
					APIKey: fmt.Sprintf("key-%d", index),
				}
				_ = provider.Configure(config)
			}(i)
		}

		wg.Wait()

		// Verify that configuration was updated (should be one of the concurrent configs)
		finalConfig := provider.GetConfig()
		assert.NotEmpty(t, finalConfig.Type)
		assert.NotEmpty(t, finalConfig.APIKey)
	})
}

// TestBaseProvider_UpdateConfig tests the UpdateConfig method
func TestBaseProvider_UpdateConfig(t *testing.T) {
	t.Run("UpdateWithLogger", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := log.New(&logBuffer, "", log.LstdFlags)

		provider := NewBaseProvider("test", types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}, nil, logger)

		newConfig := types.ProviderConfig{
			Type:   types.ProviderTypeAnthropic,
			APIKey: "updated-key",
		}

		provider.UpdateConfig(newConfig)

		assert.Equal(t, newConfig, provider.GetConfig())
		assert.Contains(t, logBuffer.String(), "updated from openai to anthropic")
	})

	t.Run("UpdateWithoutLogger", func(t *testing.T) {
		provider := NewBaseProvider("test", types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}, nil, nil)

		newConfig := types.ProviderConfig{
			Type:   types.ProviderTypeAnthropic,
			APIKey: "updated-key",
		}

		provider.UpdateConfig(newConfig)

		assert.Equal(t, newConfig, provider.GetConfig())
	})
}

// TestBaseProvider_GetConfig tests the GetConfig method
func TestBaseProvider_GetConfig(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		Name:   "test-provider",
		APIKey: "test-key",
	}
	provider := NewBaseProvider("test", config, nil, nil)

	retrievedConfig := provider.GetConfig()
	assert.Equal(t, config, retrievedConfig)

	// Ensure returned config is a copy, not a reference
	retrievedConfig.APIKey = "modified-key"
	assert.Equal(t, "test-key", provider.GetConfig().APIKey)
}

// TestBaseProvider_GetModels tests the GetModels method
func TestBaseProvider_GetModels(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

	models, err := provider.GetModels(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, models)
}

// TestBaseProvider_GetDefaultModel tests the GetDefaultModel method
func TestBaseProvider_GetDefaultModel(t *testing.T) {
	t.Run("WithDefaultModelInConfig", func(t *testing.T) {
		config := types.ProviderConfig{
			DefaultModel: "gpt-4",
		}
		provider := NewBaseProvider("test", config, nil, nil)

		assert.Equal(t, "gpt-4", provider.GetDefaultModel())
	})

	t.Run("WithoutDefaultModelInConfig", func(t *testing.T) {
		provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

		assert.Equal(t, "default-model", provider.GetDefaultModel())
	})
}

// TestBaseProvider_Authenticate tests the Authenticate method
func TestBaseProvider_Authenticate(t *testing.T) {
	t.Run("WithAPIKey", func(t *testing.T) {
		provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

		authConfig := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: "test-api-key",
		}

		err := provider.Authenticate(context.Background(), authConfig)

		assert.NoError(t, err)
		assert.True(t, provider.IsAuthenticated())
		assert.Equal(t, "test-api-key", provider.GetConfig().APIKey)
	})

	t.Run("WithoutAPIKey", func(t *testing.T) {
		provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

		authConfig := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
		}

		err := provider.Authenticate(context.Background(), authConfig)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authentication not implemented")
		assert.False(t, provider.IsAuthenticated())
	})
}

// TestBaseProvider_IsAuthenticated tests the IsAuthenticated method
func TestBaseProvider_IsAuthenticated(t *testing.T) {
	t.Run("Authenticated", func(t *testing.T) {
		config := types.ProviderConfig{
			APIKey: "test-key",
		}
		provider := NewBaseProvider("test", config, nil, nil)

		assert.True(t, provider.IsAuthenticated())
	})

	t.Run("NotAuthenticated", func(t *testing.T) {
		provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

		assert.False(t, provider.IsAuthenticated())
	})
}

// TestBaseProvider_Logout tests the Logout method
func TestBaseProvider_Logout(t *testing.T) {
	config := types.ProviderConfig{
		APIKey: "test-key",
	}
	provider := NewBaseProvider("test", config, nil, nil)

	assert.True(t, provider.IsAuthenticated())

	err := provider.Logout(context.Background())

	assert.NoError(t, err)
	// Note: BaseProvider logout doesn't clear the API key, so it remains authenticated
	assert.True(t, provider.IsAuthenticated())
}

// TestBaseProvider_SupportsToolCalling tests the SupportsToolCalling method
func TestBaseProvider_SupportsToolCalling(t *testing.T) {
	t.Run("SupportsToolCalling", func(t *testing.T) {
		config := types.ProviderConfig{
			SupportsToolCalling: true,
		}
		provider := NewBaseProvider("test", config, nil, nil)

		assert.True(t, provider.SupportsToolCalling())
	})

	t.Run("DoesNotSupportToolCalling", func(t *testing.T) {
		provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

		assert.False(t, provider.SupportsToolCalling())
	})
}

// TestBaseProvider_SupportsStreaming tests the SupportsStreaming method
func TestBaseProvider_SupportsStreaming(t *testing.T) {
	t.Run("SupportsStreaming", func(t *testing.T) {
		config := types.ProviderConfig{
			SupportsStreaming: true,
		}
		provider := NewBaseProvider("test", config, nil, nil)

		assert.True(t, provider.SupportsStreaming())
	})

	t.Run("DoesNotSupportStreaming", func(t *testing.T) {
		provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

		assert.False(t, provider.SupportsStreaming())
	})
}

// TestBaseProvider_SupportsResponsesAPI tests the SupportsResponsesAPI method
func TestBaseProvider_SupportsResponsesAPI(t *testing.T) {
	t.Run("SupportsResponsesAPI", func(t *testing.T) {
		config := types.ProviderConfig{
			SupportsResponsesAPI: true,
		}
		provider := NewBaseProvider("test", config, nil, nil)

		assert.True(t, provider.SupportsResponsesAPI())
	})

	t.Run("DoesNotSupportResponsesAPI", func(t *testing.T) {
		provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

		assert.False(t, provider.SupportsResponsesAPI())
	})
}

// TestBaseProvider_InvokeServerTool tests the InvokeServerTool method
func TestBaseProvider_InvokeServerTool(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

	result, err := provider.InvokeServerTool(context.Background(), "test-tool", map[string]interface{}{
		"param": "value",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool invocation not implemented")
	assert.Nil(t, result)
}

// TestBaseProvider_GenerateChatCompletion tests the GenerateChatCompletion method
func TestBaseProvider_GenerateChatCompletion(t *testing.T) {
	provider := NewBaseProvider("test-provider", types.ProviderConfig{}, nil, nil)

	options := types.GenerateOptions{
		Prompt: "Test prompt",
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)

	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Test the mock stream
	chunk, err := stream.Next()
	assert.NoError(t, err)
	assert.Equal(t, "Mock response from test-provider", chunk.Content)
	assert.True(t, chunk.Done)

	// Test stream exhaustion
	chunk, err = stream.Next()
	assert.NoError(t, err)
	assert.Empty(t, chunk.Content)

	// Test closing the stream
	err = stream.Close()
	assert.NoError(t, err)
}

// TestBaseProvider_GetToolFormat tests the GetToolFormat method
func TestBaseProvider_GetToolFormat(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

	assert.Equal(t, types.ToolFormatOpenAI, provider.GetToolFormat())
}

// TestBaseProvider_HealthCheck tests the HealthCheck method
func TestBaseProvider_HealthCheck(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

	err := provider.HealthCheck(context.Background())

	assert.NoError(t, err)
}

// TestBaseProvider_GetMetrics tests the GetMetrics method
func TestBaseProvider_GetMetrics(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

	metrics := provider.GetMetrics()

	assert.Equal(t, int64(0), metrics.RequestCount)
	assert.Equal(t, int64(0), metrics.SuccessCount)
	assert.Equal(t, int64(0), metrics.ErrorCount)
	assert.Equal(t, int64(0), metrics.TokensUsed)
	assert.Equal(t, time.Duration(0), metrics.TotalLatency)
	assert.Equal(t, time.Duration(0), metrics.AverageLatency)
	assert.True(t, metrics.LastRequestTime.IsZero())
	assert.True(t, metrics.LastSuccessTime.IsZero())
	assert.True(t, metrics.LastErrorTime.IsZero())
	assert.Empty(t, metrics.LastError)
}

// TestBaseProvider_IncrementRequestCount tests the IncrementRequestCount method
func TestBaseProvider_IncrementRequestCount(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

	// Initially, request count should be 0
	metrics := provider.GetMetrics()
	assert.Equal(t, int64(0), metrics.RequestCount)
	assert.True(t, metrics.LastRequestTime.IsZero())

	// Increment request count
	provider.IncrementRequestCount()

	// Verify count incremented and timestamp updated
	metrics = provider.GetMetrics()
	assert.Equal(t, int64(1), metrics.RequestCount)
	assert.False(t, metrics.LastRequestTime.IsZero())

	// Increment again
	provider.IncrementRequestCount()

	metrics = provider.GetMetrics()
	assert.Equal(t, int64(2), metrics.RequestCount)
}

// TestBaseProvider_RecordSuccess tests the RecordSuccess method
func TestBaseProvider_RecordSuccess(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

	// Record first success
	latency1 := 100 * time.Millisecond
	tokens1 := int64(50)
	provider.RecordSuccess(latency1, tokens1)

	metrics := provider.GetMetrics()
	assert.Equal(t, int64(1), metrics.SuccessCount)
	assert.Equal(t, tokens1, metrics.TokensUsed)
	assert.Equal(t, latency1, metrics.TotalLatency)
	assert.Equal(t, latency1, metrics.AverageLatency)
	assert.False(t, metrics.LastSuccessTime.IsZero())

	// Record second success
	latency2 := 200 * time.Millisecond
	tokens2 := int64(75)
	provider.RecordSuccess(latency2, tokens2)

	metrics = provider.GetMetrics()
	assert.Equal(t, int64(2), metrics.SuccessCount)
	assert.Equal(t, tokens1+tokens2, metrics.TokensUsed)
	assert.Equal(t, latency1+latency2, metrics.TotalLatency)
	assert.Equal(t, (latency1+latency2)/2, metrics.AverageLatency)
}

// TestBaseProvider_RecordError tests the RecordError method
func TestBaseProvider_RecordError(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

	// Initially, error count should be 0
	metrics := provider.GetMetrics()
	assert.Equal(t, int64(0), metrics.ErrorCount)
	assert.True(t, metrics.LastErrorTime.IsZero())
	assert.Empty(t, metrics.LastError)

	// Record error with message
	err := fmt.Errorf("test error message")
	provider.RecordError(err)

	metrics = provider.GetMetrics()
	assert.Equal(t, int64(1), metrics.ErrorCount)
	assert.False(t, metrics.LastErrorTime.IsZero())
	assert.Equal(t, "test error message", metrics.LastError)

	// Record another error
	err2 := fmt.Errorf("second error")
	provider.RecordError(err2)

	metrics = provider.GetMetrics()
	assert.Equal(t, int64(2), metrics.ErrorCount)
	assert.Equal(t, "second error", metrics.LastError)

	// Record nil error
	provider.RecordError(nil)

	metrics = provider.GetMetrics()
	assert.Equal(t, int64(3), metrics.ErrorCount)
	// LastError should still be "second error" since nil doesn't update it
	assert.Equal(t, "second error", metrics.LastError)
}

// TestBaseProvider_UpdateHealthStatus tests the UpdateHealthStatus method
func TestBaseProvider_UpdateHealthStatus(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

	// Initially, health status should be zero values
	metrics := provider.GetMetrics()
	assert.False(t, metrics.HealthStatus.Healthy)
	assert.Empty(t, metrics.HealthStatus.Message)
	assert.True(t, metrics.HealthStatus.LastChecked.IsZero())

	// Update to healthy status
	provider.UpdateHealthStatus(true, "All systems operational")

	metrics = provider.GetMetrics()
	assert.True(t, metrics.HealthStatus.Healthy)
	assert.Equal(t, "All systems operational", metrics.HealthStatus.Message)
	assert.False(t, metrics.HealthStatus.LastChecked.IsZero())

	// Update to unhealthy status
	provider.UpdateHealthStatus(false, "Service unavailable")

	metrics = provider.GetMetrics()
	assert.False(t, metrics.HealthStatus.Healthy)
	assert.Equal(t, "Service unavailable", metrics.HealthStatus.Message)
	assert.False(t, metrics.HealthStatus.LastChecked.IsZero())
}

// TestBaseProvider_UpdateHealthStatusResponseTime tests the UpdateHealthStatusResponseTime method
func TestBaseProvider_UpdateHealthStatusResponseTime(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

	// Initially, response time should be 0
	metrics := provider.GetMetrics()
	assert.Equal(t, float64(0), metrics.HealthStatus.ResponseTime)

	// Update response time
	provider.UpdateHealthStatusResponseTime(123.45)

	metrics = provider.GetMetrics()
	assert.Equal(t, 123.45, metrics.HealthStatus.ResponseTime)

	// Update to different response time
	provider.UpdateHealthStatusResponseTime(456.78)

	metrics = provider.GetMetrics()
	assert.Equal(t, 456.78, metrics.HealthStatus.ResponseTime)
}

// TestBaseProvider_LogRequest tests the LogRequest method
func TestBaseProvider_LogRequest(t *testing.T) {
	t.Run("WithLogger", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := log.New(&logBuffer, "", log.LstdFlags)
		provider := NewBaseProvider("test-provider", types.ProviderConfig{}, nil, logger)

		headers := map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer token",
		}
		body := map[string]interface{}{
			"prompt": "test",
		}

		provider.LogRequest("POST", "https://api.example.com/v1/chat", headers, body)

		logOutput := logBuffer.String()
		assert.Contains(t, logOutput, "test-provider - POST https://api.example.com/v1/chat")
		assert.Contains(t, logOutput, "Header: Content-Type: application/json")
		assert.Contains(t, logOutput, "Header: Authorization: Bearer token")
		assert.Contains(t, logOutput, "Body: map[prompt:test]")
	})

	t.Run("WithoutLogger", func(t *testing.T) {
		provider := NewBaseProvider("test-provider", types.ProviderConfig{}, nil, nil)

		// This should not panic
		assert.NotPanics(t, func() {
			provider.LogRequest("POST", "https://api.example.com", nil, nil)
		})
	})
}

// TestBaseProvider_LogResponse tests the LogResponse method
func TestBaseProvider_LogResponse(t *testing.T) {
	t.Run("WithLogger", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := log.New(&logBuffer, "", log.LstdFlags)
		provider := NewBaseProvider("test-provider", types.ProviderConfig{}, nil, logger)

		resp := &http.Response{
			StatusCode: 200,
		}
		duration := 150 * time.Millisecond

		provider.LogResponse(resp, duration)

		logOutput := logBuffer.String()
		assert.Contains(t, logOutput, "test-provider response in 150ms - Status: 200")
	})

	t.Run("WithoutLogger", func(t *testing.T) {
		provider := NewBaseProvider("test-provider", types.ProviderConfig{}, nil, nil)

		resp := &http.Response{
			StatusCode: 200,
		}

		// This should not panic
		assert.NotPanics(t, func() {
			provider.LogResponse(resp, 100*time.Millisecond)
		})
	})
}

// TestMockStream tests the MockStream implementation
func TestMockStream(t *testing.T) {
	t.Run("SingleChunk", func(t *testing.T) {
		chunks := []types.ChatCompletionChunk{
			{Content: "Hello", Done: false},
			{Content: " World", Done: true},
		}
		stream := &MockStream{chunks: chunks}

		// First chunk
		chunk, err := stream.Next()
		assert.NoError(t, err)
		assert.Equal(t, "Hello", chunk.Content)
		assert.False(t, chunk.Done)

		// Second chunk
		chunk, err = stream.Next()
		assert.NoError(t, err)
		assert.Equal(t, " World", chunk.Content)
		assert.True(t, chunk.Done)

		// No more chunks
		chunk, err = stream.Next()
		assert.NoError(t, err)
		assert.Empty(t, chunk.Content)
	})

	t.Run("EmptyStream", func(t *testing.T) {
		stream := &MockStream{chunks: []types.ChatCompletionChunk{}}

		chunk, err := stream.Next()
		assert.NoError(t, err)
		assert.Empty(t, chunk.Content)
	})

	t.Run("CloseAndReuse", func(t *testing.T) {
		chunks := []types.ChatCompletionChunk{
			{Content: "Test", Done: true},
		}
		stream := &MockStream{chunks: chunks}

		// Read chunk
		chunk, err := stream.Next()
		assert.NoError(t, err)
		assert.Equal(t, "Test", chunk.Content)

		// Close stream
		err = stream.Close()
		assert.NoError(t, err)

		// Should be able to read from beginning again
		chunk, err = stream.Next()
		assert.NoError(t, err)
		assert.Equal(t, "Test", chunk.Content)
	})
}

// TestBaseProviderStub tests the BaseProviderStub wrapper
func TestBaseProviderStub(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "test-key",
	}
	stub := NewBaseProviderStub("test-stub", config, nil, nil)

	// Test basic methods
	assert.Equal(t, "test-stub", stub.Name())
	assert.Equal(t, types.ProviderTypeOpenAI, stub.Type())
	assert.Equal(t, "Base provider stub", stub.Description())
	assert.Equal(t, config, stub.GetConfig())

	// Test model methods
	models, err := stub.GetModels(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, models)

	assert.Equal(t, "default-model", stub.GetDefaultModel())

	// Test authentication
	authConfig := types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "new-key",
	}
	err = stub.Authenticate(context.Background(), authConfig)
	assert.NoError(t, err)
	assert.True(t, stub.IsAuthenticated())

	err = stub.Logout(context.Background())
	assert.NoError(t, err)

	// Test capabilities
	assert.False(t, stub.SupportsToolCalling())
	assert.False(t, stub.SupportsStreaming())
	assert.False(t, stub.SupportsResponsesAPI())
	assert.Equal(t, types.ToolFormatOpenAI, stub.GetToolFormat())

	// Test health and metrics
	err = stub.HealthCheck(context.Background())
	assert.NoError(t, err)

	metrics := stub.GetMetrics()
	assert.Equal(t, int64(0), metrics.RequestCount)

	// Test chat completion
	options := types.GenerateOptions{Prompt: "test"}
	stream, err := stub.GenerateChatCompletion(context.Background(), options)
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Test tool invocation
	result, err := stub.InvokeServerTool(context.Background(), "test", nil)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestBaseProvider_ConcurrentAccess tests concurrent access to provider methods
func TestBaseProvider_ConcurrentAccess(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{
		APIKey: "test-key",
	}, nil, nil)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Test concurrent configuration access
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = provider.GetConfig()
			_ = provider.IsAuthenticated()
			_ = provider.GetMetrics()
		}()
	}

	// Test concurrent configuration updates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			config := types.ProviderConfig{
				Type:   types.ProviderTypeOpenAI,
				APIKey: fmt.Sprintf("key-%d", index),
			}
			provider.UpdateConfig(config)
		}(i)
	}

	wg.Wait()

	// Verify final state
	assert.True(t, provider.IsAuthenticated())
	finalConfig := provider.GetConfig()
	assert.Equal(t, types.ProviderTypeOpenAI, finalConfig.Type)
	assert.NotEmpty(t, finalConfig.APIKey)
}

// TestBaseProvider_ConcurrentMetrics tests concurrent access to metrics methods
func TestBaseProvider_ConcurrentMetrics(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

	var wg sync.WaitGroup
	numGoroutines := 20

	// Test concurrent metric updates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			provider.IncrementRequestCount()
			provider.RecordSuccess(time.Duration(index)*time.Millisecond, int64(index))
			provider.RecordError(fmt.Errorf("error %d", index))
			provider.UpdateHealthStatus(index%2 == 0, fmt.Sprintf("status %d", index))
			provider.UpdateHealthStatusResponseTime(float64(index))
		}(i)
	}

	wg.Wait()

	// Verify final metrics
	metrics := provider.GetMetrics()
	assert.Equal(t, int64(numGoroutines), metrics.RequestCount)
	assert.Equal(t, int64(numGoroutines), metrics.SuccessCount)
	assert.Equal(t, int64(numGoroutines), metrics.ErrorCount)
	assert.Greater(t, metrics.TokensUsed, int64(0))
	assert.Greater(t, metrics.TotalLatency, time.Duration(0))
	assert.Greater(t, metrics.AverageLatency, time.Duration(0))
}

// TestBaseProvider_ErrorHandling tests error handling scenarios
func TestBaseProvider_ErrorHandling(t *testing.T) {
	provider := NewBaseProvider("test", types.ProviderConfig{}, nil, nil)

	t.Run("AuthenticationError", func(t *testing.T) {
		authConfig := types.AuthConfig{
			Method: types.AuthMethodOAuth, // Not supported by base provider
		}
		err := provider.Authenticate(context.Background(), authConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authentication not implemented")
	})

	t.Run("ToolInvocationError", func(t *testing.T) {
		result, err := provider.InvokeServerTool(context.Background(), "nonexistent-tool", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool invocation not implemented")
		assert.Nil(t, result)
	})
}

// BenchmarkBaseProvider_Creation benchmarks provider creation
func BenchmarkBaseProvider_Creation(b *testing.B) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		Name:   "benchmark-provider",
		APIKey: "benchmark-key",
	}
	client := &http.Client{Timeout: 30 * time.Second}
	logger := log.New(bytes.NewBuffer(nil), "", log.LstdFlags)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewBaseProvider("benchmark-provider", config, client, logger)
	}
}

// BenchmarkBaseProvider_Configuration benchmarks configuration operations
func BenchmarkBaseProvider_Configuration(b *testing.B) {
	provider := NewBaseProvider("benchmark", types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}, nil, nil)

	config := types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "new-key",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.UpdateConfig(config)
		_ = provider.GetConfig()
	}
}

// BenchmarkBaseProvider_Authentication benchmarks authentication operations
func BenchmarkBaseProvider_Authentication(b *testing.B) {
	provider := NewBaseProvider("benchmark", types.ProviderConfig{}, nil, nil)

	authConfig := types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "benchmark-key",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.Authenticate(context.Background(), authConfig)
		_ = provider.IsAuthenticated()
	}
}

// BenchmarkBaseProvider_Metrics benchmarks metrics operations
func BenchmarkBaseProvider_Metrics(b *testing.B) {
	provider := NewBaseProvider("benchmark", types.ProviderConfig{}, nil, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetMetrics()
	}
}

// BenchmarkBaseProvider_ChatCompletion benchmarks chat completion generation
func BenchmarkBaseProvider_ChatCompletion(b *testing.B) {
	provider := NewBaseProvider("benchmark", types.ProviderConfig{}, nil, nil)
	options := types.GenerateOptions{
		Prompt: "benchmark prompt",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		if err != nil {
			b.Fatal(err)
		}
		_ = stream.Close()
	}
}

// BenchmarkMockStream_Next benchmarks MockStream.Next operations
func BenchmarkMockStream_Next(b *testing.B) {
	chunks := make([]types.ChatCompletionChunk, 100)
	for i := range chunks {
		chunks[i] = types.ChatCompletionChunk{
			Content: fmt.Sprintf("Chunk %d", i),
			Done:    i == len(chunks)-1,
		}
	}
	stream := &MockStream{chunks: chunks}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset stream for each iteration
		stream.index = 0
		for {
			chunk, err := stream.Next()
			if err != nil {
				b.Fatal(err)
			}
			if chunk.Done || chunk.Content == "" {
				break
			}
		}
	}
}

// TestIntegration_ProviderInterface ensures BaseProvider implements all required interface methods
func TestIntegration_ProviderInterface(t *testing.T) {
	// This test ensures that BaseProvider (through BaseProviderStub)
	// correctly implements the Provider interface
	var logBuffer bytes.Buffer
	logger := log.New(&logBuffer, "", log.LstdFlags)
	var _ types.Provider = NewBaseProviderStub("test", types.ProviderConfig{}, nil, logger)

	provider := NewBaseProviderStub("test", types.ProviderConfig{
		Type:                 types.ProviderTypeOpenAI,
		Name:                 "test-provider",
		DefaultModel:         "gpt-4",
		APIKey:               "test-key",
		SupportsToolCalling:  true,
		SupportsStreaming:    true,
		SupportsResponsesAPI: false,
	}, nil, logger)

	// Test all interface methods
	assert.Equal(t, "test", provider.Name())
	assert.Equal(t, types.ProviderTypeOpenAI, provider.Type())
	assert.NotEmpty(t, provider.Description())

	// Model management
	models, err := provider.GetModels(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, models)
	assert.NotEmpty(t, provider.GetDefaultModel())

	// Authentication
	authConfig := types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "new-key",
	}
	assert.NoError(t, provider.Authenticate(context.Background(), authConfig))
	assert.True(t, provider.IsAuthenticated())
	assert.NoError(t, provider.Logout(context.Background()))

	// Configuration
	config := provider.GetConfig()
	assert.NoError(t, provider.Configure(config))

	// Core capabilities
	options := types.GenerateOptions{Prompt: "test"}
	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	_ = stream.Close()

	result, err := provider.InvokeServerTool(context.Background(), "test", nil)
	assert.Error(t, err) // Expected to fail in base implementation
	assert.Nil(t, result)

	// Feature flags
	assert.True(t, provider.SupportsToolCalling())
	assert.True(t, provider.SupportsStreaming())
	assert.False(t, provider.SupportsResponsesAPI())
	assert.Equal(t, types.ToolFormatOpenAI, provider.GetToolFormat())

	// Health and metrics
	assert.NoError(t, provider.HealthCheck(context.Background()))
	metrics := provider.GetMetrics()
	assert.NotNil(t, metrics)
}
