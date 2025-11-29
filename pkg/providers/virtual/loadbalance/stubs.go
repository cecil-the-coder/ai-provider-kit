package loadbalance

import (
	"context"
	"fmt"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Stub implementations for Provider interface methods not specific to loadbalance

func (lb *LoadBalanceProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	return nil, fmt.Errorf("GetModels not supported for virtual load balance provider")
}

func (lb *LoadBalanceProvider) GetDefaultModel() string {
	return ""
}

func (lb *LoadBalanceProvider) SupportsToolCalling() bool {
	return false
}

func (lb *LoadBalanceProvider) SupportsStreaming() bool {
	return true
}

func (lb *LoadBalanceProvider) SupportsResponsesAPI() bool {
	return false
}

func (lb *LoadBalanceProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

func (lb *LoadBalanceProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return nil // Virtual providers don't need authentication
}

func (lb *LoadBalanceProvider) IsAuthenticated() bool {
	return true
}

func (lb *LoadBalanceProvider) Logout(ctx context.Context) error {
	return nil
}

func (lb *LoadBalanceProvider) Configure(config types.ProviderConfig) error {
	// Update load balance config from provider config if needed
	if config.ProviderConfig != nil {
		if strategy, ok := config.ProviderConfig["strategy"].(string); ok {
			lb.config.Strategy = Strategy(strategy)
		}
		if providers, ok := config.ProviderConfig["providers"].([]string); ok {
			lb.config.ProviderNames = providers
		}
	}
	return nil
}

func (lb *LoadBalanceProvider) GetConfig() types.ProviderConfig {
	return types.ProviderConfig{
		Type: "loadbalance",
		Name: lb.name,
		ProviderConfig: map[string]interface{}{
			"strategy":  string(lb.config.Strategy),
			"providers": lb.config.ProviderNames,
		},
	}
}

func (lb *LoadBalanceProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, fmt.Errorf("tool calling not supported for virtual load balance provider")
}

func (lb *LoadBalanceProvider) HealthCheck(ctx context.Context) error {
	if len(lb.providers) == 0 {
		return fmt.Errorf("no providers configured")
	}

	// Check health of at least one provider
	var lastErr error
	for _, provider := range lb.providers {
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

func (lb *LoadBalanceProvider) SetMetricsCollector(collector types.MetricsCollector) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.metricsCollector = collector
}

func (lb *LoadBalanceProvider) GetMetrics() types.ProviderMetrics {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// Aggregate metrics from child providers
	var metrics types.ProviderMetrics
	for _, provider := range lb.providers {
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
