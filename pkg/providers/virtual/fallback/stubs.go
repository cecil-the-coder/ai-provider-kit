package fallback

import (
	"context"
	"fmt"
	"sync"

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

var (
	fallbackMetricsMu sync.RWMutex
	fallbackMetrics   = make(map[string]types.ProviderMetrics)
)

func (f *FallbackProvider) GetMetrics() types.ProviderMetrics {
	fallbackMetricsMu.RLock()
	defer fallbackMetricsMu.RUnlock()
	if metrics, ok := fallbackMetrics[f.name]; ok {
		return metrics
	}
	return types.ProviderMetrics{
		RequestCount: 0,
		SuccessCount: 0,
		ErrorCount:   0,
	}
}
