package racing

import (
	"context"
	"fmt"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Stub implementations for Provider interface methods not specific to racing


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
	r.mu.Lock()
	defer r.mu.Unlock()

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
		if defaultVM, ok := config.ProviderConfig["default_virtual_model"].(string); ok {
			r.config.DefaultVirtualModel = defaultVM
		}
		if providers, ok := config.ProviderConfig["providers"].([]string); ok {
			r.config.ProviderNames = providers
		}
		if performanceFile, ok := config.ProviderConfig["performance_file"].(string); ok {
			r.config.PerformanceFile = performanceFile
		}

		// Handle virtual models configuration
		if virtualModels, ok := config.ProviderConfig["virtual_models"].(map[string]interface{}); ok {
			r.config.VirtualModels = make(map[string]VirtualModelConfig)
			for vmName, vmData := range virtualModels {
				if vmMap, ok := vmData.(map[string]interface{}); ok {
					vmConfig := VirtualModelConfig{
						DisplayName: getStringOrDefault(vmMap, "display_name", vmName),
						Description: getStringOrDefault(vmMap, "description", ""),
						Strategy:    Strategy(getStringOrDefault(vmMap, "strategy", string(r.config.Strategy))),
						TimeoutMS:   getIntOrDefault(vmMap, "timeout_ms", r.config.TimeoutMS),
					}

					// Parse providers
					if providers, ok := vmMap["providers"].([]interface{}); ok {
						for _, provData := range providers {
							if provMap, ok := provData.(map[string]interface{}); ok {
								providerRef := ProviderReference{
									Name:     getStringOrDefault(provMap, "name", ""),
									Model:    getStringOrDefault(provMap, "model", ""),
									Priority: getIntOrDefault(provMap, "priority", 0),
								}
								vmConfig.Providers = append(vmConfig.Providers, providerRef)
							}
						}
					}

					r.config.VirtualModels[vmName] = vmConfig
				}
			}
		}
	}
	return nil
}

// Helper functions for configuration parsing
func getStringOrDefault(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return defaultValue
}

func getIntOrDefault(m map[string]interface{}, key string, defaultValue int) int {
	if val, ok := m[key].(int); ok {
		return val
	}
	if val, ok := m[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

func (r *RacingProvider) GetConfig() types.ProviderConfig {
	providerConfig := map[string]interface{}{
		"timeout_ms":           r.config.TimeoutMS,
		"grace_period_ms":      r.config.GracePeriodMS,
		"strategy":             string(r.config.Strategy),
		"default_virtual_model": r.config.DefaultVirtualModel,
		"virtual_models":       r.config.VirtualModels,
	}

	if len(r.config.ProviderNames) > 0 {
		providerConfig["providers"] = r.config.ProviderNames
	}

	if r.config.PerformanceFile != "" {
		providerConfig["performance_file"] = r.config.PerformanceFile
	}

	return types.ProviderConfig{
		Type: "racing",
		Name: r.name,
		ProviderConfig: providerConfig,
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

func (r *RacingProvider) SetMetricsCollector(collector types.MetricsCollector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metricsCollector = collector
}

func (r *RacingProvider) GetMetrics() types.ProviderMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Start with racing provider's own request count
	var metrics types.ProviderMetrics
	metrics.RequestCount = r.requestCount
	metrics.LastRequestTime = time.Now()

	// Aggregate metrics from child providers
	for _, provider := range r.providers {
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
