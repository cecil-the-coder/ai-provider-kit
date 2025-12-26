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
	// activeRequests tracks the number of active requests per provider for fill-first strategy
	activeRequests []uint64
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
	StrategyFillFirst  Strategy = "fill_first"
)

func NewLoadBalanceProvider(name string, config *Config) *LoadBalanceProvider {
	return &LoadBalanceProvider{
		name:   name,
		config: config,
	}
}

func (lb *LoadBalanceProvider) SetProviders(providers []types.Provider) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.providers = providers
	// Reset active requests tracking when providers are reset
	lb.activeRequests = make([]uint64, len(providers))
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
			// Decrement active request count for fill-first strategy
			if lb.config.Strategy == StrategyFillFirst {
				lb.decrementActiveRequest(providerIdx)
			}
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
			// Decrement active request count for fill-first strategy
			if lb.config.Strategy == StrategyFillFirst {
				lb.decrementActiveRequest(providerIdx)
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
			lb:            lb,
			providerIdx:   providerIdx,
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
	lb           *LoadBalanceProvider
	providerIdx  int
	decremented atomic.Bool
}

func (s *loadBalanceStream) Close() error {
	// Decrement active request count on close for fill-first strategy
	if s.lb != nil && s.lb.config != nil && s.lb.config.Strategy == StrategyFillFirst {
		if s.decremented.CompareAndSwap(false, true) {
			s.lb.decrementActiveRequest(s.providerIdx)
		}
	}
	return s.StreamWrapper.Close()
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
	case StrategyFillFirst:
		return lb.selectFillFirstProviderExcluding(excludedIndices)
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
		// #nosec G115 -- Safe conversion: result is modded by n (slice length), which fits within int on 64-bit systems
		idx := int((atomic.AddUint64(&lb.counter, 1) - 1) % n)
		return lb.providers[idx], idx, nil
	}

	// Find the first available provider starting from current position
	n := uint64(len(lb.providers))
	for i := uint64(0); i < n; i++ {
		// #nosec G115 -- Safe conversion: result is modded by n (slice length), which fits within int on 64-bit systems
		idx := int((atomic.AddUint64(&lb.counter, 1) - 1) % n)
		if !excludedIndices[idx] {
			return lb.providers[idx], idx, nil
		}
	}

	return nil, -1, fmt.Errorf("no available providers after exclusions")
}

// selectFillFirstProviderExcluding selects the provider with the lowest number of active requests,
// excluding specified indices. If multiple providers have the same load, it selects the first
// among them for deterministic behavior. This strategy helps distribute load by filling
// underutilized providers first.
func (lb *LoadBalanceProvider) selectFillFirstProviderExcluding(excludedIndices map[int]bool) (types.Provider, int, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Ensure activeRequests slice is properly sized
	if len(lb.activeRequests) != len(lb.providers) {
		lb.activeRequests = make([]uint64, len(lb.providers))
	}

	// Find the provider with the lowest active request count that is not excluded
	var minActiveRequests uint64 = ^uint64(0) // Max uint64
	var selectedIdx int = -1

	for i := range lb.providers {
		if excludedIndices[i] {
			continue
		}

		if lb.activeRequests[i] < minActiveRequests {
			minActiveRequests = lb.activeRequests[i]
			selectedIdx = i
		}
	}

	if selectedIdx == -1 {
		return nil, -1, fmt.Errorf("no available providers after exclusions")
	}

	// Increment active request count for selected provider
	lb.activeRequests[selectedIdx]++

	return lb.providers[selectedIdx], selectedIdx, nil
}

func randomInt(max int) int {
	return int(time.Now().UnixNano() % int64(max))
}

// decrementActiveRequest decrements the active request count for a specific provider.
// This is called when a stream is closed.
func (lb *LoadBalanceProvider) decrementActiveRequest(idx int) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if idx >= 0 && idx < len(lb.activeRequests) {
		if lb.activeRequests[idx] > 0 {
			lb.activeRequests[idx]--
		}
	}
}
