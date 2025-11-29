package racing

import (
	"context"
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Stub implementations for Provider interface methods not specific to racing

func (r *RacingProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	return nil, fmt.Errorf("GetModels not supported for virtual racing provider")
}

func (r *RacingProvider) GetDefaultModel() string {
	return ""
}

func (r *RacingProvider) SupportsToolCalling() bool {
	return false
}

func (r *RacingProvider) SupportsStreaming() bool {
	return true
}

func (r *RacingProvider) SupportsResponsesAPI() bool {
	return false
}

func (r *RacingProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

func (r *RacingProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return nil // Virtual providers don't need authentication
}

func (r *RacingProvider) IsAuthenticated() bool {
	return true
}

func (r *RacingProvider) Logout(ctx context.Context) error {
	return nil
}

func (r *RacingProvider) Configure(config types.ProviderConfig) error {
	// Update racing config from provider config if needed
	if config.ProviderConfig != nil {
		if timeout, ok := config.ProviderConfig["timeout_ms"].(int); ok {
			r.config.TimeoutMS = timeout
		}
		if gracePeriod, ok := config.ProviderConfig["grace_period_ms"].(int); ok {
			r.config.GracePeriodMS = gracePeriod
		}
		if strategy, ok := config.ProviderConfig["strategy"].(string); ok {
			r.config.Strategy = Strategy(strategy)
		}
		if providers, ok := config.ProviderConfig["providers"].([]string); ok {
			r.config.ProviderNames = providers
		}
	}
	return nil
}

func (r *RacingProvider) GetConfig() types.ProviderConfig {
	return types.ProviderConfig{
		Type: "racing",
		Name: r.name,
		ProviderConfig: map[string]interface{}{
			"timeout_ms":       r.config.TimeoutMS,
			"grace_period_ms":  r.config.GracePeriodMS,
			"strategy":         string(r.config.Strategy),
			"providers":        r.config.ProviderNames,
			"performance_file": r.config.PerformanceFile,
		},
	}
}

func (r *RacingProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, fmt.Errorf("tool calling not supported for virtual racing provider")
}

func (r *RacingProvider) HealthCheck(ctx context.Context) error {
	r.mu.RLock()
	providers := r.providers
	r.mu.RUnlock()

	if len(providers) == 0 {
		return fmt.Errorf("no providers configured")
	}

	// Check health of at least one provider
	var lastErr error
	for _, provider := range providers {
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

func (r *RacingProvider) GetMetrics() types.ProviderMetrics {
	return types.ProviderMetrics{
		RequestCount: 0,
		SuccessCount: 0,
		ErrorCount:   0,
	}
}
