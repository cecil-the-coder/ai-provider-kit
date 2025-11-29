// Package factory provides provider factory functionality for AI providers.
// It includes registration, creation, and management of different AI provider implementations.
package factory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// DefaultProviderFactory is the default factory implementation
type DefaultProviderFactory struct {
	providers        map[types.ProviderType]func(types.ProviderConfig) types.Provider
	mutex            sync.RWMutex
	metricsCollector types.MetricsCollector
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory() *DefaultProviderFactory {
	return &DefaultProviderFactory{
		providers: make(map[types.ProviderType]func(types.ProviderConfig) types.Provider),
		mutex:     sync.RWMutex{},
	}
}

// SetMetricsCollector sets the metrics collector for the factory
// When set, all providers created by this factory will have the collector configured
func (f *DefaultProviderFactory) SetMetricsCollector(collector types.MetricsCollector) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.metricsCollector = collector
}

// RegisterProvider registers a new provider type
func (f *DefaultProviderFactory) RegisterProvider(providerType types.ProviderType, factoryFunc func(types.ProviderConfig) types.Provider) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.providers[providerType] = factoryFunc
}

// CreateProvider creates a provider instance
func (f *DefaultProviderFactory) CreateProvider(providerType types.ProviderType, config types.ProviderConfig) (types.Provider, error) {
	f.mutex.RLock()
	factoryFunc, exists := f.providers[providerType]
	collector := f.metricsCollector
	f.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("provider type %s not registered", providerType)
	}

	provider := factoryFunc(config)

	// If a metrics collector is configured and the provider supports it, set it
	if collector != nil {
		if metricProvider, ok := provider.(interface{ SetMetricsCollector(types.MetricsCollector) }); ok {
			metricProvider.SetMetricsCollector(collector)
		}
	}

	return provider, nil
}

// GetSupportedProviders returns all supported provider types
func (f *DefaultProviderFactory) GetSupportedProviders() []types.ProviderType {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	providerTypes := make([]types.ProviderType, 0, len(f.providers))
	for providerType := range f.providers {
		providerTypes = append(providerTypes, providerType)
	}

	return providerTypes
}

// InitializeDefaultProviders registers stub providers only for providers without real implementations
// This function is used for testing providers that don't have actual implementations yet.
// For providers with real implementations, use RegisterDefaultProviders instead.
func InitializeDefaultProviders(factory *DefaultProviderFactory) {
	// Register only stub providers for those without real implementations
	factory.RegisterProvider(types.ProviderTypeLMStudio, func(config types.ProviderConfig) types.Provider {
		return &SimpleProviderStub{name: "lmstudio", providerType: types.ProviderTypeLMStudio, config: config}
	})
	factory.RegisterProvider(types.ProviderTypeLlamaCpp, func(config types.ProviderConfig) types.Provider {
		return &SimpleProviderStub{name: "llamacpp", providerType: types.ProviderTypeLlamaCpp, config: config}
	})
	factory.RegisterProvider(types.ProviderTypeOllama, func(config types.ProviderConfig) types.Provider {
		return &SimpleProviderStub{name: "ollama", providerType: types.ProviderTypeOllama, config: config}
	})
}

// SimpleProviderStub implements types.Provider interface
type SimpleProviderStub struct {
	name         string
	providerType types.ProviderType
	config       types.ProviderConfig
	metrics      types.ProviderMetrics
	mutex        sync.RWMutex
}

// Name returns the provider name
func (p *SimpleProviderStub) Name() string             { return p.name }
func (p *SimpleProviderStub) Type() types.ProviderType { return p.providerType }
func (p *SimpleProviderStub) Description() string      { return fmt.Sprintf("%s provider", p.name) }
func (p *SimpleProviderStub) GetModels(ctx context.Context) ([]types.Model, error) {
	providerStr := string(p.providerType)
	return []types.Model{
		{
			ID:                   providerStr + "-model-1",
			Name:                 providerStr + " Default Model",
			Provider:             p.providerType,
			Description:          "Default model for " + providerStr,
			MaxTokens:            4096,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "completion"},
			Pricing: types.Pricing{
				InputTokenPrice:  0.001,
				OutputTokenPrice: 0.002,
				Unit:             "token",
			},
		},
		{
			ID:                   providerStr + "-model-2",
			Name:                 providerStr + " Large Model",
			Provider:             p.providerType,
			Description:          "Large model for " + providerStr,
			MaxTokens:            8192,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "completion", "analysis"},
			Pricing: types.Pricing{
				InputTokenPrice:  0.002,
				OutputTokenPrice: 0.004,
				Unit:             "token",
			},
		},
	}, nil
}
func (p *SimpleProviderStub) GetDefaultModel() string         { return "default-model" }
func (p *SimpleProviderStub) SupportsToolCalling() bool       { return true }
func (p *SimpleProviderStub) SupportsStreaming() bool         { return true }
func (p *SimpleProviderStub) SupportsResponsesAPI() bool      { return false }
func (p *SimpleProviderStub) GetToolFormat() types.ToolFormat { return types.ToolFormatOpenAI }
func (p *SimpleProviderStub) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return nil
}
func (p *SimpleProviderStub) IsAuthenticated() bool            { return true }
func (p *SimpleProviderStub) Logout(ctx context.Context) error { return nil }
func (p *SimpleProviderStub) Configure(config types.ProviderConfig) error {
	p.config = config
	return nil
}
func (p *SimpleProviderStub) GetConfig() types.ProviderConfig { return p.config }
func (p *SimpleProviderStub) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.metrics.RequestCount++
	p.metrics.SuccessCount++
	p.metrics.LastRequestTime = time.Now()
	p.metrics.LastSuccessTime = time.Now()
	p.metrics.TokensUsed += 25

	return &FactoryMockStream{}, nil
}
func (p *SimpleProviderStub) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, fmt.Errorf("tool calling not implemented")
}
func (p *SimpleProviderStub) HealthCheck(ctx context.Context) error { return nil }
func (p *SimpleProviderStub) GetMetrics() types.ProviderMetrics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.metrics
}

// FactoryMockStream implements types.ChatCompletionStream
type FactoryMockStream struct{}

func (m *FactoryMockStream) Next() (types.ChatCompletionChunk, error) {
	return types.ChatCompletionChunk{
		ID:      "mock-chunk-1",
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   "mock-model",
		Choices: []types.ChatChoice{
			{
				Index: 0,
				Delta: types.ChatMessage{
					Role:    "assistant",
					Content: "This is a mock response from the provider.",
				},
				FinishReason: "stop",
			},
		},
		Usage: types.Usage{
			PromptTokens:     10,
			CompletionTokens: 15,
			TotalTokens:      25,
		},
		Content: "This is a mock response from the provider.",
		Done:    true,
	}, nil
}
func (m *FactoryMockStream) Close() error {
	return nil
}
