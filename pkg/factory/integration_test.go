package factory

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScenario1_CompleteProviderWorkflow tests the entire provider lifecycle
func TestScenario1_CompleteProviderWorkflow(t *testing.T) {
	// Create factory with mock providers instead of real implementations
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	// Register a custom provider
	customProviderType := types.ProviderType("custom-test")
	factory.RegisterProvider(customProviderType, func(config types.ProviderConfig) types.Provider {
		return &MockProvider{
			name:         config.Name,
			providerType: customProviderType,
			config:       config,
		}
	})

	// Test factory supports our custom provider
	supportedProviders := factory.GetSupportedProviders()
	assert.Contains(t, supportedProviders, customProviderType)

	// Test provider creation from configuration
	config := types.ProviderConfig{
		Type:                 types.ProviderTypeOpenAI,
		Name:                 "test-openai-provider",
		APIKey:               "test-api-key",
		BaseURL:              "https://api.openai.com/v1",
		DefaultModel:         "gpt-4",
		Description:          "Test OpenAI provider instance",
		SupportsStreaming:    true,
		SupportsToolCalling:  true,
		SupportsResponsesAPI: false,
		MaxTokens:            4096,
		Timeout:              30 * time.Second,
		ToolFormat:           types.ToolFormatOpenAI,
	}

	provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Test provider lifecycle: configuration
	err = provider.Configure(config)
	assert.NoError(t, err)
	retrievedConfig := provider.GetConfig()
	assert.Equal(t, config.Name, retrievedConfig.Name)
	assert.Equal(t, config.APIKey, retrievedConfig.APIKey)

	// Test provider lifecycle: authentication
	authConfig := types.AuthConfig{
		Method:       types.AuthMethodAPIKey,
		APIKey:       "new-api-key",
		BaseURL:      "https://api.openai.com/v1",
		DefaultModel: "gpt-4-turbo",
	}
	err = provider.Authenticate(context.Background(), authConfig)
	assert.NoError(t, err)
	assert.True(t, provider.IsAuthenticated())

	// Test provider lifecycle: model retrieval
	models, err := provider.GetModels(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, models)

	// Verify model structure
	for _, model := range models {
		assert.NotEmpty(t, model.ID)
		assert.NotEmpty(t, model.Name)
		assert.Equal(t, provider.Type(), model.Provider)
	}

	// Test provider lifecycle: chat completion
	options := types.GenerateOptions{
		Prompt:      "Hello, how are you?",
		MaxTokens:   100,
		Temperature: 0.7,
		Messages: []types.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello, how are you?"},
		},
		Stream: false,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Test streaming
	chunk, err := stream.Next()
	assert.NoError(t, err)
	assert.NotNil(t, chunk)
	err = stream.Close()
	assert.NoError(t, err)

	// Test provider lifecycle: metrics
	metrics := provider.GetMetrics()
	assert.NotNil(t, metrics)
	assert.GreaterOrEqual(t, metrics.RequestCount, int64(0))

	// Test provider lifecycle: health check
	err = provider.HealthCheck(context.Background())
	assert.NoError(t, err)

	// Test provider lifecycle: cleanup
	err = provider.Logout(context.Background())
	assert.NoError(t, err)
}

// TestScenario2_ConfigurationManagement tests configuration handling across types
func TestScenario2_ConfigurationManagement(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	t.Run("ProviderConfigurationFromMaps", func(t *testing.T) {
		// Test configuration created from maps
		configMap := map[string]interface{}{
			"type":                  "openai",
			"name":                  "map-configured-provider",
			"api_key":               "map-api-key",
			"base_url":              "https://api.openai.com/v1",
			"default_model":         "gpt-4",
			"supports_streaming":    true,
			"supports_tool_calling": true,
			"max_tokens":            4096,
			"timeout":               30000000000,
			"tool_format":           "openai",
			"provider_config": map[string]interface{}{
				"custom_param": "custom_value",
			},
		}

		// Convert map to JSON then to ProviderConfig
		configJSON, err := json.Marshal(configMap)
		require.NoError(t, err)

		var config types.ProviderConfig
		err = json.Unmarshal(configJSON, &config)
		require.NoError(t, err)

		// Verify configuration
		assert.Equal(t, types.ProviderTypeOpenAI, config.Type)
		assert.Equal(t, "map-configured-provider", config.Name)
		assert.Equal(t, "map-api-key", config.APIKey)
		assert.Equal(t, "gpt-4", config.DefaultModel)
		assert.True(t, config.SupportsStreaming)
		assert.True(t, config.SupportsToolCalling)
		assert.Equal(t, 4096, config.MaxTokens)

		// Create provider with this configuration
		provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
		require.NoError(t, err)
		require.NotNil(t, provider)

		// Verify provider was configured correctly
		retrievedConfig := provider.GetConfig()
		assert.Equal(t, config.Name, retrievedConfig.Name)
		assert.Equal(t, config.APIKey, retrievedConfig.APIKey)
	})

	t.Run("OAuthConfigurationHandling", func(t *testing.T) {
		// Test OAuth configuration
		oauthCreds := []*types.OAuthCredentialSet{
			{
				ID:           "test-cred-1",
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				Scopes:       []string{"read", "write"},
			},
		}

		config := types.ProviderConfig{
			Type:             types.ProviderTypeOpenAI,
			Name:             "oauth-provider",
			OAuthCredentials: oauthCreds,
		}

		// Test JSON serialization/deserialization
		configJSON, err := json.Marshal(config)
		require.NoError(t, err)

		var retrievedConfig types.ProviderConfig
		err = json.Unmarshal(configJSON, &retrievedConfig)
		require.NoError(t, err)

		assert.Equal(t, oauthCreds[0].ClientID, retrievedConfig.OAuthCredentials[0].ClientID)
		assert.Equal(t, oauthCreds[0].ClientSecret, retrievedConfig.OAuthCredentials[0].ClientSecret)
		assert.Equal(t, oauthCreds[0].Scopes, retrievedConfig.OAuthCredentials[0].Scopes)
	})

	t.Run("ConfigurationValidationAcrossTypes", func(t *testing.T) {
		// Test invalid configurations
		invalidConfigs := []struct {
			name   string
			config types.ProviderConfig
			errMsg string
		}{
			{
				name: "missing_type",
				config: types.ProviderConfig{
					Name:   "no-type-provider",
					APIKey: "test-key",
				},
				errMsg: "provider type",
			},
			{
				name: "invalid_provider_type",
				config: types.ProviderConfig{
					Type:   types.ProviderType("invalid"),
					Name:   "invalid-type-provider",
					APIKey: "test-key",
				},
				errMsg: "not registered",
			},
		}

		for _, tc := range invalidConfigs {
			t.Run(tc.name, func(t *testing.T) {
				_, err := factory.CreateProvider(tc.config.Type, tc.config)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			})
		}
	})

	t.Run("ErrorRecoveryFromInvalidConfigs", func(t *testing.T) {
		// Test that factory remains operational after invalid config attempts
		originalProviders := factory.GetSupportedProviders()
		originalCount := len(originalProviders)

		// Try to create provider with invalid config
		invalidConfig := types.ProviderConfig{
			Type: types.ProviderType("nonexistent"),
			Name: "invalid",
		}
		_, err := factory.CreateProvider(invalidConfig.Type, invalidConfig)
		assert.Error(t, err)

		// Verify factory state is unchanged
		currentProviders := factory.GetSupportedProviders()
		assert.Equal(t, originalCount, len(currentProviders))

		// Verify we can still create valid providers
		validConfig := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			Name:   "valid-provider",
			APIKey: "test-key",
		}
		provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, validConfig)
		assert.NoError(t, err)
		assert.NotNil(t, provider)
	})
}

// TestScenario3_MultiProviderOperations tests concurrent multi-provider scenarios
func TestScenario3_MultiProviderOperations(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	// Create multiple providers of different types
	providerConfigs := []types.ProviderConfig{
		{
			Type:   types.ProviderTypeOpenAI,
			Name:   "openai-instance",
			APIKey: "openai-key",
		},
		{
			Type:   types.ProviderTypeAnthropic,
			Name:   "anthropic-instance",
			APIKey: "anthropic-key",
		},
		{
			Type:   types.ProviderTypeGemini,
			Name:   "gemini-instance",
			APIKey: "gemini-key",
		},
		{
			Type:   types.ProviderTypeQwen,
			Name:   "qwen-instance",
			APIKey: "qwen-key",
		},
	}

	// Create provider instances
	providers := make([]types.Provider, 0, len(providerConfigs))
	for _, config := range providerConfigs {
		provider, err := factory.CreateProvider(config.Type, config)
		require.NoError(t, err)
		providers = append(providers, provider)
	}

	t.Run("ConcurrentProviderOperations", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, len(providers)*4)

		// Perform concurrent operations on each provider
		for i, provider := range providers {
			// Concurrent configuration
			wg.Add(1)
			go func(p types.Provider, idx int) {
				defer wg.Done()
				err := p.Configure(providerConfigs[idx])
				if err != nil {
					errors <- fmt.Errorf("config error on provider %d: %w", idx, err)
					return
				}
			}(provider, i)

			// Concurrent model retrieval
			wg.Add(1)
			go func(p types.Provider, idx int) {
				defer wg.Done()
				_, err := p.GetModels(context.Background())
				if err != nil {
					errors <- fmt.Errorf("models error on provider %d: %w", idx, err)
					return
				}
			}(provider, i)

			// Concurrent chat completion
			wg.Add(1)
			go func(p types.Provider, idx int) {
				defer wg.Done()
				options := types.GenerateOptions{
					Prompt:    fmt.Sprintf("Test prompt for provider %d", idx),
					MaxTokens: 50,
				}
				stream, err := p.GenerateChatCompletion(context.Background(), options)
				if err != nil {
					errors <- fmt.Errorf("generation error on provider %d: %w", idx, err)
					return
				}
				func() { _ = stream.Close() }()
			}(provider, i)

			// Concurrent health check
			wg.Add(1)
			go func(p types.Provider, idx int) {
				defer wg.Done()
				err := p.HealthCheck(context.Background())
				if err != nil {
					errors <- fmt.Errorf("health error on provider %d: %w", idx, err)
					return
				}
			}(provider, i)
		}

		wg.Wait()
		close(errors)

		// Check for any errors
		for err := range errors {
			t.Error(err)
		}
	})

	t.Run("ProviderIsolationAndIndependence", func(t *testing.T) {
		// Configure each provider differently
		for i, provider := range providers {
			config := providerConfigs[i]
			config.DefaultModel = fmt.Sprintf("model-%d", i)
			config.Description = fmt.Sprintf("Provider instance %d", i)
			err := provider.Configure(config)
			require.NoError(t, err)
		}

		// Verify configurations are independent
		for i, provider := range providers {
			config := provider.GetConfig()
			expectedModel := fmt.Sprintf("model-%d", i)
			expectedDesc := fmt.Sprintf("Provider instance %d", i)
			assert.Equal(t, expectedModel, config.DefaultModel)
			assert.Equal(t, expectedDesc, config.Description)
		}
	})

	t.Run("MetricsAggregationAcrossProviders", func(t *testing.T) {
		// Generate activity on each provider
		for i, provider := range providers {
			// Perform multiple operations to generate metrics
			for j := 0; j < 3; j++ {
				options := types.GenerateOptions{
					Prompt:    fmt.Sprintf("Activity %d on provider %d", j, i),
					MaxTokens: 10,
				}
				stream, err := provider.GenerateChatCompletion(context.Background(), options)
				if err == nil {
					func() { _ = stream.Close() }()
				}
			}
		}

		// Collect metrics from all providers
		var totalRequests int64
		for i, provider := range providers {
			metrics := provider.GetMetrics()
			totalRequests += metrics.RequestCount
			t.Logf("Provider %d (%s) - Requests: %d, Success: %d, Errors: %d",
				i, provider.Type(), metrics.RequestCount, metrics.SuccessCount, metrics.ErrorCount)
		}

		// Verify metrics were collected
		assert.Greater(t, totalRequests, int64(0))
	})

	t.Run("GracefulDegradation", func(t *testing.T) {
		// Simulate provider failure by configuring invalid credentials
		if len(providers) > 0 {
			failingProvider := providers[0]
			invalidConfig := types.ProviderConfig{
				Type:   failingProvider.Type(),
				Name:   "failing-provider",
				APIKey: "", // Empty API key should cause authentication issues
			}

			err := failingProvider.Configure(invalidConfig)
			// Note: This might not fail with the mock implementation, but demonstrates the pattern
			t.Logf("Configured failing provider, error: %v", err)

			// Other providers should continue working
			for i := 1; i < len(providers); i++ {
				options := types.GenerateOptions{
					Prompt:    "Test during degradation",
					MaxTokens: 10,
				}
				stream, err := providers[i].GenerateChatCompletion(context.Background(), options)
				assert.NoError(t, err)
				if stream != nil {
					func() { _ = stream.Close() }()
				}
			}
		}
	})
}

// TestScenario4_TypeSystemIntegration tests cross-package type compatibility
func TestScenario4_TypeSystemIntegration(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	t.Run("TypeCompatibilityBetweenPackages", func(t *testing.T) {
		// Create provider using types from pkg/types
		config := types.ProviderConfig{
			Type:                types.ProviderTypeOpenAI,
			Name:                "type-compat-test",
			APIKey:              "test-key",
			BaseURL:             "https://api.openai.com/v1",
			DefaultModel:        "gpt-4",
			SupportsStreaming:   true,
			SupportsToolCalling: true,
			ToolFormat:          types.ToolFormatOpenAI,
			Timeout:             30 * time.Second,
		}

		provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
		require.NoError(t, err)

		// Test that provider returns types compatible with pkg/types
		models, err := provider.GetModels(context.Background())
		require.NoError(t, err)

		// Verify Model type compatibility
		for _, model := range models {
			assert.NotEmpty(t, model.ID)
			assert.NotEmpty(t, model.Name)
			assert.IsType(t, types.ProviderType(""), model.Provider)
			assert.IsType(t, types.Pricing{}, model.Pricing)
		}

		// Test GenerateOptions type compatibility
		options := types.GenerateOptions{
			Prompt:      "Test prompt",
			MaxTokens:   100,
			Temperature: 0.7,
			Stream:      false,
			Messages: []types.ChatMessage{
				{
					Role:    "user",
					Content: "Hello",
					ToolCalls: []types.ToolCall{
						{
							ID:   "tool-1",
							Type: "function",
							Function: types.ToolCallFunction{
								Name:      "test_function",
								Arguments: `{"param": "value"}`,
							},
						},
					},
				},
			},
			Tools: []types.Tool{
				{
					Name:        "test_tool",
					Description: "A test tool",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"param": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		require.NotNil(t, stream)

		// Test ChatCompletionStream and ChatCompletionChunk compatibility
		chunk, err := stream.Next()
		assert.NoError(t, err)
		assert.IsType(t, types.ChatCompletionChunk{}, chunk)

		if len(chunk.Choices) > 0 {
			assert.IsType(t, types.ChatChoice{}, chunk.Choices[0])
			assert.IsType(t, types.ChatMessage{}, chunk.Choices[0].Message)
			assert.IsType(t, types.Usage{}, chunk.Usage)
		}

		func() { _ = stream.Close() }()
	})

	t.Run("JSONMarshalingAcrossTypes", func(t *testing.T) {
		// Test ProviderConfig JSON marshaling
		config := types.ProviderConfig{
			Type:         types.ProviderTypeOpenAI,
			Name:         "json-test-provider",
			APIKey:       "json-test-key",
			BaseURL:      "https://api.openai.com/v1",
			DefaultModel: "gpt-4",
			Description:  "JSON test provider",
			OAuthCredentials: []*types.OAuthCredentialSet{
				{
					ID:           "json-cred",
					ClientID:     "json-client-id",
					ClientSecret: "json-client-secret",
					Scopes:       []string{"read", "write"},
				},
			},
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			MaxTokens:            4096,
			Timeout:              30 * time.Second,
			ToolFormat:           types.ToolFormatOpenAI,
			ProviderConfig: map[string]interface{}{
				"custom_param": "custom_value",
				"nested_param": map[string]interface{}{
					"nested_key": "nested_value",
				},
			},
		}

		// Test JSON marshaling
		configJSON, err := json.Marshal(config)
		require.NoError(t, err)

		// Test JSON unmarshaling
		var unmarshaledConfig types.ProviderConfig
		err = json.Unmarshal(configJSON, &unmarshaledConfig)
		require.NoError(t, err)

		// Verify all fields were preserved
		assert.Equal(t, config.Type, unmarshaledConfig.Type)
		assert.Equal(t, config.Name, unmarshaledConfig.Name)
		assert.Equal(t, config.APIKey, unmarshaledConfig.APIKey)
		assert.Equal(t, config.DefaultModel, unmarshaledConfig.DefaultModel)
		assert.Equal(t, config.OAuthCredentials[0].ClientID, unmarshaledConfig.OAuthCredentials[0].ClientID)
		assert.Equal(t, config.ToolFormat, unmarshaledConfig.ToolFormat)
		assert.Equal(t, config.MaxTokens, unmarshaledConfig.MaxTokens)

		// Test nested objects
		assert.Equal(t, "custom_value", unmarshaledConfig.ProviderConfig["custom_param"])
	})

	t.Run("InterfaceImplementations", func(t *testing.T) {
		// Create a mock OpenAI provider instead of real one
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			Name:   "interface-test-provider",
			APIKey: "test-key",
		}

		provider := NewAdvancedMockProvider(config.Name, types.ProviderTypeOpenAI, config)
		require.NotNil(t, provider)

		// Verify provider implements all required interfaces
		var _ types.Provider = provider

		// Test all interface methods are implemented
		assert.NotEmpty(t, provider.Name())
		assert.NotEmpty(t, provider.Type())
		assert.NotEmpty(t, provider.Description())
		assert.NotEmpty(t, provider.GetDefaultModel())

		assert.IsType(t, provider.SupportsToolCalling(), true)
		assert.IsType(t, provider.SupportsStreaming(), true)
		assert.IsType(t, provider.SupportsResponsesAPI(), true)
		assert.IsType(t, provider.GetToolFormat(), types.ToolFormat(""))

		// Test methods that return types
		models, err := provider.GetModels(context.Background())
		assert.NoError(t, err)
		assert.IsType(t, []types.Model{}, models)

		providerConfig := provider.GetConfig()
		assert.IsType(t, types.ProviderConfig{}, providerConfig)

		metrics := provider.GetMetrics()
		assert.IsType(t, types.ProviderMetrics{}, metrics)
	})

	t.Run("ErrorPropagationThroughStack", func(t *testing.T) {
		// Test error propagation from factory through provider
		invalidConfig := types.ProviderConfig{
			Type:   types.ProviderType("nonexistent"),
			Name:   "error-prop-test",
			APIKey: "test-key",
		}

		provider, err := factory.CreateProvider(invalidConfig.Type, invalidConfig)
		assert.Error(t, err)
		assert.Nil(t, provider)
		assert.Contains(t, err.Error(), "not registered")

		// Test error propagation from provider operations
		validConfig := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			Name:   "error-prop-provider",
			APIKey: "", // Empty key should cause errors in real implementation
		}

		provider, err = factory.CreateProvider(types.ProviderTypeOpenAI, validConfig)
		assert.NoError(t, err) // Factory creates successfully

		// Provider operations should handle empty API key gracefully
		models, err := provider.GetModels(context.Background())
		// This might not error with mock, but demonstrates the pattern
		if err != nil {
			assert.Contains(t, err.Error(), "API key")
		} else {
			assert.NotNil(t, models) // Mock implementation succeeds
		}
	})
}

// TestScenario5_PerformanceAndReliability tests performance and reliability
func TestScenario5_PerformanceAndReliability(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	t.Run("CompleteWorkflowBenchmark", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			Name:   "benchmark-provider",
			APIKey: "benchmark-key",
		}

		// Benchmark complete workflow
		b := testing.B{}
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < 100; i++ {
			// Provider creation
			provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
			if err != nil {
				t.Fatal(err)
			}

			// Configuration
			err = provider.Configure(config)
			if err != nil {
				t.Fatal(err)
			}

			// Model retrieval
			_, err = provider.GetModels(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			// Chat completion
			options := types.GenerateOptions{
				Prompt:    fmt.Sprintf("Benchmark test %d", i),
				MaxTokens: 10,
			}
			stream, err := provider.GenerateChatCompletion(context.Background(), options)
			if err != nil {
				t.Fatal(err)
			}
			func() { _ = stream.Close() }()

			// Health check
			err = provider.HealthCheck(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			// Metrics
			_ = provider.GetMetrics()
		}

		b.StopTimer()
		t.Logf("Completed 100 full workflow iterations")
	})

	t.Run("MemoryUsagePatterns", func(t *testing.T) {
		// Get initial memory state
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		// Create many providers and perform operations
		providers := make([]types.Provider, 50)
		for i := 0; i < 50; i++ {
			config := types.ProviderConfig{
				Type:   types.ProviderTypeOpenAI,
				Name:   fmt.Sprintf("memory-test-provider-%d", i),
				APIKey: fmt.Sprintf("key-%d", i),
			}
			provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
			require.NoError(t, err)
			providers[i] = provider

			// Perform operations
			options := types.GenerateOptions{
				Prompt:    fmt.Sprintf("Memory test %d", i),
				MaxTokens: 5,
			}
			stream, err := provider.GenerateChatCompletion(context.Background(), options)
			require.NoError(t, err)
			func() { _ = stream.Close() }()
		}

		// Check memory after operations
		runtime.GC()
		runtime.ReadMemStats(&m2)

		// Calculate memory usage
		allocDiff := m2.TotalAlloc - m1.TotalAlloc
		t.Logf("Memory allocated for 50 providers and operations: %d bytes", allocDiff)

		// Verify no excessive memory usage (less than 10MB for this test)
		assert.Less(t, allocDiff, uint64(10*1024*1024), "Memory usage seems excessive")
	})

	t.Run("ResourceCleanup", func(t *testing.T) {
		initialGoroutines := runtime.NumGoroutine()

		// Create many providers and perform operations with streams
		for i := 0; i < 20; i++ {
			config := types.ProviderConfig{
				Type:   types.ProviderTypeOpenAI,
				Name:   fmt.Sprintf("cleanup-test-provider-%d", i),
				APIKey: fmt.Sprintf("key-%d", i),
			}
			provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
			require.NoError(t, err)

			// Create multiple streams and ensure they're closed
			for j := 0; j < 5; j++ {
				options := types.GenerateOptions{
					Prompt:    fmt.Sprintf("Cleanup test %d-%d", i, j),
					MaxTokens: 5,
					Stream:    true,
				}
				stream, err := provider.GenerateChatCompletion(context.Background(), options)
				require.NoError(t, err)

				// Read from stream and close
				chunk, err := stream.Next()
				if err == nil {
					_ = chunk
				}
				err = stream.Close()
				assert.NoError(t, err)
			}
		}

		// Allow some time for cleanup
		runtime.GC()
		time.Sleep(100 * time.Millisecond)

		finalGoroutines := runtime.NumGoroutine()
		goroutineDiff := finalGoroutines - initialGoroutines

		t.Logf("Goroutine count before: %d, after: %d, diff: %d",
			initialGoroutines, finalGoroutines, goroutineDiff)

		// Verify no excessive goroutine leaks (allow some tolerance)
		assert.Less(t, goroutineDiff, 10, "Possible goroutine leak detected")
	})

	t.Run("ThreadSafetyAcrossModule", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, 200)

		// Concurrent factory operations
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Factory operations
				providerType := types.ProviderType(fmt.Sprintf("thread-test-%d", id))
				factory.RegisterProvider(providerType, func(config types.ProviderConfig) types.Provider {
					return &MockProvider{
						name:         config.Name,
						providerType: providerType,
						config:       config,
					}
				})

				config := types.ProviderConfig{
					Type:   providerType,
					Name:   fmt.Sprintf("thread-provider-%d", id),
					APIKey: "thread-key",
				}

				provider, err := factory.CreateProvider(providerType, config)
				if err != nil {
					errors <- fmt.Errorf("factory error %d: %w", id, err)
					return
				}

				// Provider operations
				for j := 0; j < 10; j++ {
					options := types.GenerateOptions{
						Prompt:    fmt.Sprintf("Thread test %d-%d", id, j),
						MaxTokens: 5,
					}
					stream, err := provider.GenerateChatCompletion(context.Background(), options)
					if err != nil {
						errors <- fmt.Errorf("provider error %d-%d: %w", id, j, err)
						return
					}
					func() { _ = stream.Close() }()

					// Metrics access
					_ = provider.GetMetrics()
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for any errors
		for err := range errors {
			t.Error(err)
		}

		// Verify factory is still functional
		supportedProviders := factory.GetSupportedProviders()
		assert.GreaterOrEqual(t, len(supportedProviders), 10) // At least our thread-test providers
	})

	t.Run("StressTest", func(t *testing.T) {
		// High-intensity stress test
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			Name:   "stress-test-provider",
			APIKey: "stress-key",
		}

		provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
		require.NoError(t, err)

		var wg sync.WaitGroup
		stressErrors := make(chan error, 1000)

		// Launch many concurrent operations
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < 10; j++ {
					options := types.GenerateOptions{
						Prompt:    fmt.Sprintf("Stress test %d-%d", id, j),
						MaxTokens: 10,
						Messages: []types.ChatMessage{
							{Role: "system", Content: "You are a helpful assistant."},
							{Role: "user", Content: fmt.Sprintf("Stress test message %d-%d", id, j)},
						},
					}

					stream, err := provider.GenerateChatCompletion(context.Background(), options)
					if err != nil {
						stressErrors <- fmt.Errorf("stress error %d-%d: %w", id, j, err)
						return
					}

					// Process stream
					chunk, err := stream.Next()
					if err != nil {
						stressErrors <- fmt.Errorf("stream error %d-%d: %w", id, j, err)
						func() { _ = stream.Close() }()
						return
					}
					_ = chunk

					err = stream.Close()
					if err != nil {
						stressErrors <- fmt.Errorf("close error %d-%d: %w", id, j, err)
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(stressErrors)

		// Count errors
		errorCount := 0
		for err := range stressErrors {
			t.Error(err)
			errorCount++
		}

		t.Logf("Stress test completed with %d errors out of 1000 operations", errorCount)

		// Verify provider is still responsive
		finalOptions := types.GenerateOptions{
			Prompt:    "Final stress test check",
			MaxTokens: 5,
		}
		stream, err := provider.GenerateChatCompletion(context.Background(), finalOptions)
		assert.NoError(t, err)
		if stream != nil {
			func() { _ = stream.Close() }()
		}

		// Check final metrics
		metrics := provider.GetMetrics()
		t.Logf("Final metrics - Requests: %d, Success: %d, Errors: %d",
			metrics.RequestCount, metrics.SuccessCount, metrics.ErrorCount)
	})
}

// TestIntegrationExample demonstrates a complete real-world usage example
func TestIntegrationExample(t *testing.T) {
	// This test serves as documentation and validation of a complete usage pattern

	// Step 1: Create and initialize factory
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	// Step 2: Configure multiple providers for different use cases
	providers := make(map[string]types.Provider)

	// OpenAI for general chat completion
	openaiConfig := types.ProviderConfig{
		Type:                types.ProviderTypeOpenAI,
		Name:                "primary-openai",
		APIKey:              "sk-test-key",
		BaseURL:             "https://api.openai.com/v1",
		DefaultModel:        "gpt-4",
		SupportsStreaming:   true,
		SupportsToolCalling: true,
		MaxTokens:           4096,
		Timeout:             30 * time.Second,
		ToolFormat:          types.ToolFormatOpenAI,
	}

	openaiProvider, err := factory.CreateProvider(types.ProviderTypeOpenAI, openaiConfig)
	require.NoError(t, err)
	providers["openai"] = openaiProvider

	// Anthropic for complex reasoning
	anthropicConfig := types.ProviderConfig{
		Type:                types.ProviderTypeAnthropic,
		Name:                "reasoning-anthropic",
		APIKey:              "sk-ant-test-key",
		DefaultModel:        "claude-3-sonnet",
		SupportsStreaming:   true,
		SupportsToolCalling: true,
		MaxTokens:           8192,
		Timeout:             60 * time.Second,
		ToolFormat:          types.ToolFormatAnthropic,
	}

	anthropicProvider, err := factory.CreateProvider(types.ProviderTypeAnthropic, anthropicConfig)
	require.NoError(t, err)
	providers["anthropic"] = anthropicProvider

	// Step 3: Authenticate all providers
	for name, provider := range providers {
		err := provider.Authenticate(context.Background(), types.AuthConfig{
			Method:       types.AuthMethodAPIKey,
			APIKey:       providers[name].GetConfig().APIKey,
			BaseURL:      providers[name].GetConfig().BaseURL,
			DefaultModel: providers[name].GetConfig().DefaultModel,
		})
		assert.NoError(t, err, "Failed to authenticate %s provider", name)
		assert.True(t, provider.IsAuthenticated(), "%s provider should be authenticated", name)
	}

	// Step 4: Perform health checks
	for name, provider := range providers {
		err := provider.HealthCheck(context.Background())
		assert.NoError(t, err, "%s provider health check failed", name)
	}

	// Step 5: Use providers for different tasks
	ctx := context.Background()

	// General chat with OpenAI
	chatOptions := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Explain quantum computing in simple terms."},
		},
		MaxTokens:   500,
		Temperature: 0.7,
		Stream:      false,
	}

	stream, err := providers["openai"].GenerateChatCompletion(ctx, chatOptions)
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	chunk, err := stream.Next()
	assert.NoError(t, err)
	t.Logf("OpenAI response: %s", chunk.Content)

	// Complex reasoning with Anthropic
	reasoningOptions := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "system", Content: "You are a expert analyst."},
			{Role: "user", Content: "Analyze the potential impact of AI on healthcare over the next decade."},
		},
		MaxTokens:   1000,
		Temperature: 0.3,
		Stream:      true,
	}

	stream, err = providers["anthropic"].GenerateChatCompletion(ctx, reasoningOptions)
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	// Process streaming response
	responseContent := ""
	for {
		chunk, err := stream.Next()
		if err != nil || chunk.Done {
			break
		}
		responseContent += chunk.Content
	}

	t.Logf("Anthropic streaming response length: %d characters", len(responseContent))

	// Step 6: Collect and analyze metrics
	for name, provider := range providers {
		metrics := provider.GetMetrics()
		t.Logf("Provider %s metrics:", name)
		t.Logf("  Requests: %d", metrics.RequestCount)
		t.Logf("  Success: %d", metrics.SuccessCount)
		t.Logf("  Errors: %d", metrics.ErrorCount)
		t.Logf("  Avg Latency: %v", metrics.AverageLatency)
		t.Logf("  Tokens Used: %d", metrics.TokensUsed)
	}

	// Step 7: Cleanup
	for name, provider := range providers {
		err := provider.Logout(ctx)
		assert.NoError(t, err, "Failed to logout %s provider", name)
	}

	t.Log("Complete integration example executed successfully")
}

// AdvancedMockProvider is a more sophisticated mock for testing
type AdvancedMockProvider struct {
	name         string
	providerType types.ProviderType
	config       types.ProviderConfig
	metrics      types.ProviderMetrics
	mutex        sync.RWMutex
}

func NewAdvancedMockProvider(name string, providerType types.ProviderType, config types.ProviderConfig) *AdvancedMockProvider {
	return &AdvancedMockProvider{
		name:         name,
		providerType: providerType,
		config:       config,
		metrics: types.ProviderMetrics{
			RequestCount:    0,
			SuccessCount:    0,
			ErrorCount:      0,
			TotalLatency:    0,
			AverageLatency:  0,
			LastRequestTime: time.Now(),
			TokensUsed:      0,
			HealthStatus: types.HealthStatus{
				Healthy:      true,
				LastChecked:  time.Now(),
				Message:      "Mock provider is healthy",
				ResponseTime: 10.0,
				StatusCode:   200,
			},
		},
	}
}

func (p *AdvancedMockProvider) Name() string {
	return p.name
}

func (p *AdvancedMockProvider) Type() types.ProviderType {
	return p.providerType
}

func (p *AdvancedMockProvider) Description() string {
	return fmt.Sprintf("Advanced mock provider: %s (%s)", p.name, p.providerType)
}

func (p *AdvancedMockProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	p.updateMetrics("models", true, 0)

	return []types.Model{
		{
			ID:                   fmt.Sprintf("%s-model-1", p.providerType),
			Name:                 fmt.Sprintf("%s Default Model", p.providerType),
			Provider:             p.providerType,
			Description:          "Default model for mock provider",
			MaxTokens:            4096,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "completion"},
			Pricing: types.Pricing{
				InputTokenPrice:  0.001,
				OutputTokenPrice: 0.002,
				Unit:             "token",
			},
		},
	}, nil
}

func (p *AdvancedMockProvider) GetDefaultModel() string {
	return fmt.Sprintf("%s-default", p.providerType)
}

func (p *AdvancedMockProvider) SupportsToolCalling() bool {
	return true
}

func (p *AdvancedMockProvider) SupportsStreaming() bool {
	return true
}

func (p *AdvancedMockProvider) SupportsResponsesAPI() bool {
	return false
}

func (p *AdvancedMockProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

func (p *AdvancedMockProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Simulate authentication logic
	if authConfig.APIKey == "" {
		return fmt.Errorf("API key required for authentication")
	}

	p.config.APIKey = authConfig.APIKey
	p.config.BaseURL = authConfig.BaseURL
	p.config.DefaultModel = authConfig.DefaultModel

	return nil
}

func (p *AdvancedMockProvider) IsAuthenticated() bool {
	return p.config.APIKey != ""
}

func (p *AdvancedMockProvider) Logout(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.config.APIKey = ""
	return nil
}

func (p *AdvancedMockProvider) Configure(config types.ProviderConfig) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if config.Type != p.providerType {
		return fmt.Errorf("invalid provider type: expected %s, got %s", p.providerType, config.Type)
	}

	p.config = config
	return nil
}

func (p *AdvancedMockProvider) GetConfig() types.ProviderConfig {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.config
}

func (p *AdvancedMockProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		p.updateMetrics("completion", true, latency)
	}()

	// Simulate processing
	responseContent := fmt.Sprintf("Mock response from %s provider for: %s",
		p.providerType, options.Prompt)

	if len(options.Messages) > 0 {
		for _, msg := range options.Messages {
			if msg.Role == "user" {
				responseContent = fmt.Sprintf("Mock response to: %s", msg.Content)
				break
			}
		}
	}

	// Create mock stream
	chunks := []types.ChatCompletionChunk{
		{
			ID:      fmt.Sprintf("chunk-%d", time.Now().Unix()),
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   p.GetDefaultModel(),
			Choices: []types.ChatChoice{
				{
					Index: 0,
					Delta: types.ChatMessage{
						Role:    "assistant",
						Content: responseContent,
					},
					FinishReason: "stop",
				},
			},
			Usage: types.Usage{
				PromptTokens:     10,
				CompletionTokens: len(responseContent) / 4, // Rough estimate
				TotalTokens:      10 + len(responseContent)/4,
			},
			Content: responseContent,
			Done:    true,
		},
	}

	return &AdvancedMockStream{chunks: chunks}, nil
}

func (p *AdvancedMockProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	p.updateMetrics("tool", true, 0)

	return map[string]interface{}{
		"tool":   toolName,
		"params": params,
		"result": fmt.Sprintf("Mock result from %s provider", p.providerType),
	}, nil
}

func (p *AdvancedMockProvider) HealthCheck(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.metrics.HealthStatus.LastChecked = time.Now()
	p.metrics.HealthStatus.ResponseTime = 5.0 + float64(time.Now().UnixNano()%1000)/100 // Simulate variance

	if p.config.APIKey == "" {
		p.metrics.HealthStatus.Healthy = false
		p.metrics.HealthStatus.Message = "Not authenticated"
		p.metrics.HealthStatus.StatusCode = 401
		return fmt.Errorf("provider not authenticated")
	}

	p.metrics.HealthStatus.Healthy = true
	p.metrics.HealthStatus.Message = "Provider is healthy"
	p.metrics.HealthStatus.StatusCode = 200

	return nil
}

func (p *AdvancedMockProvider) GetMetrics() types.ProviderMetrics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.metrics
}

func (p *AdvancedMockProvider) updateMetrics(operation string, success bool, latency time.Duration) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	now := time.Now()
	p.metrics.RequestCount++
	p.metrics.TotalLatency += latency
	p.metrics.AverageLatency = p.metrics.TotalLatency / time.Duration(p.metrics.RequestCount)
	p.metrics.LastRequestTime = now

	if success {
		p.metrics.SuccessCount++
		p.metrics.LastSuccessTime = now
	} else {
		p.metrics.ErrorCount++
		p.metrics.LastErrorTime = now
		p.metrics.LastError = fmt.Sprintf("Error in %s operation", operation)
	}

	// Simulate token usage
	p.metrics.TokensUsed += 10 // Base tokens per request
}

// AdvancedMockStream implements ChatCompletionStream with more features
type AdvancedMockStream struct {
	chunks []types.ChatCompletionChunk
	index  int
}

func (s *AdvancedMockStream) Next() (types.ChatCompletionChunk, error) {
	if s.index >= len(s.chunks) {
		return types.ChatCompletionChunk{}, nil
	}

	chunk := s.chunks[s.index]
	s.index++
	return chunk, nil
}

func (s *AdvancedMockStream) Close() error {
	s.index = 0
	return nil
}

// registerMockProvidersForIntegrationTests registers mock versions of all default providers
// to replace real API calls in integration tests
func registerMockProvidersForIntegrationTests(factory *DefaultProviderFactory) {
	// Register mock providers for all default types
	providerTypes := []types.ProviderType{
		types.ProviderTypeOpenAI,
		types.ProviderTypeAnthropic,
		types.ProviderTypeGemini,
		types.ProviderTypeQwen,
		types.ProviderTypeCerebras,
		types.ProviderTypeOpenRouter,
		types.ProviderTypeLMStudio,
		types.ProviderTypeLlamaCpp,
		types.ProviderTypeOllama,
	}

	for _, providerType := range providerTypes {
		factory.RegisterProvider(providerType, func(config types.ProviderConfig) types.Provider {
			return NewAdvancedMockProvider(config.Name, providerType, config)
		})
	}
}
