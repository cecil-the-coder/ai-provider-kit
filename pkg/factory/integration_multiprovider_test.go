package factory

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScenario3_MultiProviderOperations tests concurrent multi-provider scenarios
func TestScenario3_MultiProviderOperations(t *testing.T) {
	t.Parallel()
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
