package loadbalance

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

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
		collector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventRequest,
			ProviderName: lb.name,
			ProviderType: lb.Type(),
			ModelID:      opts.Model,
			Timestamp:    time.Now(),
		})
	}

	provider := lb.selectProvider()

	chatProvider, ok := provider.(types.ChatProvider)
	if !ok {
		if collector != nil {
			collector.RecordEvent(ctx, types.MetricEvent{
				Type:         types.MetricEventError,
				ProviderName: lb.name,
				ProviderType: lb.Type(),
				ModelID:      opts.Model,
				Timestamp:    time.Now(),
				ErrorMessage: "selected provider does not support chat",
				ErrorType:    "provider_incompatible",
			})
		}
		return nil, fmt.Errorf("selected provider does not support chat")
	}

	start := time.Now()
	stream, err := chatProvider.GenerateChatCompletion(ctx, opts)
	latency := time.Since(start)

	if err != nil {
		if collector != nil {
			collector.RecordEvent(ctx, types.MetricEvent{
				Type:         types.MetricEventError,
				ProviderName: lb.name,
				ProviderType: lb.Type(),
				ModelID:      opts.Model,
				Timestamp:    time.Now(),
				ErrorMessage: err.Error(),
				ErrorType:    "provider_error",
				Latency:      latency,
			})
		}
		return nil, err
	}

	// Record success
	if collector != nil {
		collector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventSuccess,
			ProviderName: lb.name,
			ProviderType: lb.Type(),
			ModelID:      opts.Model,
			Timestamp:    time.Now(),
			Latency:      latency,
			Metadata: map[string]interface{}{
				"selected_provider": provider.Name(),
				"strategy":          string(lb.config.Strategy),
			},
		})
	}

	return &loadBalanceStream{
		inner:        stream,
		providerName: provider.Name(),
	}, nil
}

type loadBalanceStream struct {
	inner        types.ChatCompletionStream
	providerName string
}

func (s *loadBalanceStream) Next() (types.ChatCompletionChunk, error) {
	chunk, err := s.inner.Next()
	if chunk.Metadata == nil {
		chunk.Metadata = make(map[string]interface{})
	}
	chunk.Metadata["loadbalance_provider"] = s.providerName
	return chunk, err
}

func (s *loadBalanceStream) Close() error {
	return s.inner.Close()
}

func (lb *LoadBalanceProvider) selectProvider() types.Provider {
	switch lb.config.Strategy {
	case StrategyRandom:
		return lb.providers[randomInt(len(lb.providers))]
	default: // Round robin
		idx := atomic.AddUint64(&lb.counter, 1) - 1
		return lb.providers[idx%uint64(len(lb.providers))]
	}
}

func randomInt(max int) int {
	return int(time.Now().UnixNano() % int64(max))
}
