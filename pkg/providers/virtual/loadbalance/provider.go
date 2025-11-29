package loadbalance

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// LoadBalanceProvider distributes requests across providers
type LoadBalanceProvider struct {
	name      string
	providers []types.Provider
	config    *Config
	counter   uint64
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

func (lb *LoadBalanceProvider) Name() string              { return lb.name }
func (lb *LoadBalanceProvider) Type() types.ProviderType { return "loadbalance" }
func (lb *LoadBalanceProvider) Description() string      { return "Distributes requests across providers" }

func (lb *LoadBalanceProvider) GenerateChatCompletion(ctx context.Context, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
	if len(lb.providers) == 0 {
		return nil, fmt.Errorf("no providers configured")
	}

	provider := lb.selectProvider()

	chatProvider, ok := provider.(types.ChatProvider)
	if !ok {
		return nil, fmt.Errorf("selected provider does not support chat")
	}

	return chatProvider.GenerateChatCompletion(ctx, opts)
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
