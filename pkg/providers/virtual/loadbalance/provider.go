package loadbalance

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/virtual/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// LoadBalanceProvider distributes requests across providers
type LoadBalanceProvider struct {
	name             string
	providers        []types.Provider
	config           *Config
	counter          uint64
	metricsCollector types.MetricsCollector
	mu               sync.RWMutex
}

type Config struct {
	Strategy      Strategy `yaml:"strategy"`
	ProviderNames []string `yaml:"providers"`
	// EnableFailover enables automatic retry with other providers on failure.
	// When enabled, if the selected provider fails, the load balancer will
	// try other available providers (excluding already-tried ones) before
	// returning an error. This combines load distribution with high availability.
	// Default is false (backward compatible).
	EnableFailover bool `yaml:"enable_failover"`
}

type Strategy string

const (
	StrategyRoundRobin Strategy = "round_robin"
	StrategyRandom     Strategy = "random"
	StrategyWeighted   Strategy = "weighted"
)

func NewLoadBalanceProvider(name string, config *Config) *LoadBalanceProvider {
	return &LoadBalanceProvider{
		name:   name,
		config: config,
	}
}

func (lb *LoadBalanceProvider) SetProviders(providers []types.Provider) {
	lb.providers = providers
}

func (lb *LoadBalanceProvider) Name() string             { return lb.name }
func (lb *LoadBalanceProvider) Type() types.ProviderType { return "loadbalance" }
func (lb *LoadBalanceProvider) Description() string      { return "Distributes requests across providers" }

func (lb *LoadBalanceProvider) GenerateChatCompletion(ctx context.Context, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
	lb.mu.RLock()
	providers := lb.providers
	collector := lb.metricsCollector
	lb.mu.RUnlock()

	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers configured")
	}

	// Record request
	if collector != nil {
		_ = collector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventRequest,
			ProviderName: lb.name,
			ProviderType: lb.Type(),
			ModelID:      opts.Model,
			Timestamp:    time.Now(),
		})
	}

	// Track tried providers to avoid retrying the same one
	triedProviderIndices := make(map[int]bool)
	var lastErr error

	for attempt := 0; attempt < len(providers); attempt++ {
		// Select a provider that hasn't been tried yet
		provider, providerIdx, err := lb.selectProviderExcluding(triedProviderIndices)
		if err != nil {
			break // No more providers to try
		}
		triedProviderIndices[providerIdx] = true

		chatProvider, ok := provider.(types.ChatProvider)
		if !ok {
			if collector != nil {
				_ = collector.RecordEvent(ctx, types.MetricEvent{
					Type:         types.MetricEventError,
					ProviderName: lb.name,
					ProviderType: lb.Type(),
					ModelID:      opts.Model,
					Timestamp:    time.Now(),
					ErrorMessage: "selected provider does not support chat",
					ErrorType:    "provider_incompatible",
				})
			}
			lastErr = fmt.Errorf("selected provider does not support chat")
			// Try next provider if failover is enabled
			if !lb.config.EnableFailover {
				return nil, lastErr
			}
			continue
		}

		start := time.Now()
		stream, err := chatProvider.GenerateChatCompletion(ctx, opts)
		latency := time.Since(start)

		if err != nil {
			lastErr = err
			// Record error for this attempt
			if collector != nil {
				_ = collector.RecordEvent(ctx, types.MetricEvent{
					Type:         types.MetricEventError,
					ProviderName: lb.name,
					ProviderType: lb.Type(),
					ModelID:      opts.Model,
					Timestamp:    time.Now(),
					ErrorMessage: err.Error(),
					ErrorType:    "provider_error",
					Latency:      latency,
					Metadata: map[string]interface{}{
						"attempt":           attempt + 1,
						"selected_provider": provider.Name(),
					},
				})
			}
			// Try next provider if failover is enabled
			if !lb.config.EnableFailover {
				return nil, lastErr
			}
			// When failover is enabled, continue to try next provider
			// The loop will exit when all providers have been tried
			continue
		}

		// Record success
		if collector != nil {
			_ = collector.RecordEvent(ctx, types.MetricEvent{
				Type:         types.MetricEventSuccess,
				ProviderName: lb.name,
				ProviderType: lb.Type(),
				ModelID:      opts.Model,
				Timestamp:    time.Now(),
				Latency:      latency,
				Metadata: map[string]interface{}{
					"selected_provider": provider.Name(),
					"strategy":          string(lb.config.Strategy),
					"attempt":           attempt + 1,
					"failover_used":     attempt > 0,
				},
			})
		}

		return &loadBalanceStream{
			StreamWrapper: common.NewStreamWrapper(stream, "loadbalance_provider", provider.Name()),
		}, nil
	}

	// All providers failed
	if collector != nil {
		_ = collector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventError,
			ProviderName: lb.name,
			ProviderType: lb.Type(),
			ModelID:      opts.Model,
			Timestamp:    time.Now(),
			ErrorMessage: fmt.Sprintf("all %d providers failed, last error: %v", len(triedProviderIndices), lastErr),
			ErrorType:    "all_providers_failed",
		})
	}

	return nil, fmt.Errorf("all %d providers failed, last error: %w", len(triedProviderIndices), lastErr)
}

type loadBalanceStream struct {
	*common.StreamWrapper
}

// selectProviderExcluding selects a provider using the configured strategy,
// excluding providers at the specified indices. Returns the provider, its index,
// and an error if no providers are available.
func (lb *LoadBalanceProvider) selectProviderExcluding(excludedIndices map[int]bool) (types.Provider, int, error) {
	if len(lb.providers) == 0 {
		return nil, -1, fmt.Errorf("no providers available")
	}

	switch lb.config.Strategy {
	case StrategyRandom:
		return lb.selectRandomProviderExcluding(excludedIndices)
	default: // Round robin (and Weighted, which falls back to round-robin)
		return lb.selectRoundRobinProviderExcluding(excludedIndices)
	}
}

// selectRandomProviderExcluding selects a random provider excluding specified indices.
func (lb *LoadBalanceProvider) selectRandomProviderExcluding(excludedIndices map[int]bool) (types.Provider, int, error) {
	// Collect available provider indices
	availableIndices := make([]int, 0, len(lb.providers))
	for i := range lb.providers {
		if !excludedIndices[i] {
			availableIndices = append(availableIndices, i)
		}
	}

	if len(availableIndices) == 0 {
		return nil, -1, fmt.Errorf("no available providers")
	}

	// Select randomly from available
	selectedIdx := availableIndices[randomInt(len(availableIndices))]
	return lb.providers[selectedIdx], selectedIdx, nil
}

// selectRoundRobinProviderExcluding selects a provider in round-robin fashion excluding specified indices.
func (lb *LoadBalanceProvider) selectRoundRobinProviderExcluding(excludedIndices map[int]bool) (types.Provider, int, error) {
	if len(excludedIndices) == 0 {
		// No exclusions, use standard round-robin (simpler and preserves existing behavior when no failover)
		n := uint64(len(lb.providers))
		idx := int((atomic.AddUint64(&lb.counter, 1) - 1) % n)
		return lb.providers[idx], idx, nil
	}

	// Find the first available provider starting from current position
	n := uint64(len(lb.providers))
	for i := uint64(0); i < n; i++ {
		idx := int((atomic.AddUint64(&lb.counter, 1) - 1) % n)
		if !excludedIndices[idx] {
			return lb.providers[idx], idx, nil
		}
	}

	return nil, -1, fmt.Errorf("no available providers after exclusions")
}

func randomInt(max int) int {
	return int(time.Now().UnixNano() % int64(max))
}
