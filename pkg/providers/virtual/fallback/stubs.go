package fallback

import (
	"context"
	"fmt"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Stub implementations for Provider interface methods not specific to fallback

func (f *FallbackProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	return nil, fmt.Errorf("GetModels not supported for virtual fallback provider")
}

func (f *FallbackProvider) GetDefaultModel() string {
	return ""
}

func (f *FallbackProvider) SupportsToolCalling() bool {
	return false
}

func (f *FallbackProvider) SupportsStreaming() bool {
	return true
}

func (f *FallbackProvider) SupportsResponsesAPI() bool {
	return false
}

func (f *FallbackProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

func (f *FallbackProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return nil // Virtual providers don't need authentication
}

func (f *FallbackProvider) IsAuthenticated() bool {
	return true
}

func (f *FallbackProvider) Logout(ctx context.Context) error {
	return nil
}

func (f *FallbackProvider) Configure(config types.ProviderConfig) error {
	// Update fallback config from provider config if needed
	if config.ProviderConfig != nil {
		if maxRetries, ok := config.ProviderConfig["max_retries"].(int); ok {
			f.config.MaxRetries = maxRetries
		}
		if providers, ok := config.ProviderConfig["providers"].([]string); ok {
			f.config.ProviderNames = providers
		}
	}
	return nil
}

func (f *FallbackProvider) GetConfig() types.ProviderConfig {
	return types.ProviderConfig{
		Type: "fallback",
		Name: f.name,
		ProviderConfig: map[string]interface{}{
			"max_retries": f.config.MaxRetries,
			"providers":   f.config.ProviderNames,
		},
	}
}

func (f *FallbackProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, fmt.Errorf("tool calling not supported for virtual fallback provider")
}

func (f *FallbackProvider) HealthCheck(ctx context.Context) error {
	if len(f.providers) == 0 {
		return fmt.Errorf("no providers configured")
	}

	// Check health of at least one provider
	var lastErr error
	for _, provider := range f.providers {
		if healthProvider, ok := provider.(types.HealthCheckProvider); ok {
			if err := healthProvider.HealthCheck(ctx); err == nil {
				return nil // At least one provider is healthy
			} else {
				lastErr = err
			}
		}
	}

	if lastErr != nil {
		return fmt.Errorf("all providers unhealthy: %w", lastErr)
	}
	return nil
}

func (f *FallbackProvider) SetMetricsCollector(collector types.MetricsCollector) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metricsCollector = collector
}

func (f *FallbackProvider) GetMetrics() types.ProviderMetrics {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Aggregate metrics from child providers
	var metrics types.ProviderMetrics
	for _, provider := range f.providers {
		childMetrics := provider.GetMetrics()
		metrics.RequestCount += childMetrics.RequestCount
		metrics.SuccessCount += childMetrics.SuccessCount
		metrics.ErrorCount += childMetrics.ErrorCount
		metrics.TokensUsed += childMetrics.TokensUsed
		metrics.TotalLatency += childMetrics.TotalLatency

		// Track latest times
		if childMetrics.LastRequestTime.After(metrics.LastRequestTime) {
			metrics.LastRequestTime = childMetrics.LastRequestTime
		}
		if childMetrics.LastSuccessTime.After(metrics.LastSuccessTime) {
			metrics.LastSuccessTime = childMetrics.LastSuccessTime
		}
		if childMetrics.LastErrorTime.After(metrics.LastErrorTime) {
			metrics.LastErrorTime = childMetrics.LastErrorTime
			metrics.LastError = childMetrics.LastError
		}
	}

	// Calculate average latency
	if metrics.SuccessCount > 0 {
		metrics.AverageLatency = metrics.TotalLatency / time.Duration(metrics.SuccessCount)
	}

	return metrics
}
