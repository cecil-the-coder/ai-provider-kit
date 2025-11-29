package fallback

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// FallbackProvider tries providers in order until one succeeds
type FallbackProvider struct {
	name             string
	providers        []types.Provider
	config           *Config
	metricsCollector types.MetricsCollector
	mu               sync.RWMutex
}

type Config struct {
	ProviderNames []string `yaml:"providers"`
	MaxRetries    int      `yaml:"max_retries"`
}

func NewFallbackProvider(name string, config *Config) *FallbackProvider {
	return &FallbackProvider{
		name:   name,
		config: config,
	}
}

func (f *FallbackProvider) SetProviders(providers []types.Provider) {
	f.providers = providers
}

func (f *FallbackProvider) Name() string             { return f.name }
func (f *FallbackProvider) Type() types.ProviderType { return "fallback" }
func (f *FallbackProvider) Description() string      { return "Tries providers in order until one succeeds" }

func (f *FallbackProvider) GenerateChatCompletion(ctx context.Context, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
	f.mu.RLock()
	collector := f.metricsCollector
	providers := f.providers
	f.mu.RUnlock()

	// Record request
	if collector != nil {
		_ = collector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventRequest,
			ProviderName: f.name,
			ProviderType: f.Type(),
			ModelID:      opts.Model,
			Timestamp:    time.Now(),
		})
	}

	var lastErr error
	var previousProvider string

	for i, provider := range providers {
		chatProvider, ok := provider.(types.ChatProvider)
		if !ok {
			continue
		}

		start := time.Now()
		stream, err := chatProvider.GenerateChatCompletion(ctx, opts)
		latency := time.Since(start)

		if err == nil {
			// Success - emit provider switch event if not the first provider
			if collector != nil {
				if i > 0 {
					_ = collector.RecordEvent(ctx, types.MetricEvent{
						Type:          types.MetricEventProviderSwitch,
						ProviderName:  f.name,
						ProviderType:  f.Type(),
						ModelID:       opts.Model,
						Timestamp:     time.Now(),
						FromProvider:  previousProvider,
						ToProvider:    provider.Name(),
						SwitchReason:  "fallback_success",
						AttemptNumber: i + 1,
						Latency:       latency,
					})
				}

				// Record success
				_ = collector.RecordEvent(ctx, types.MetricEvent{
					Type:         types.MetricEventSuccess,
					ProviderName: f.name,
					ProviderType: f.Type(),
					ModelID:      opts.Model,
					Timestamp:    time.Now(),
					Latency:      latency,
				})
			}

			return &fallbackStream{
				inner:         stream,
				providerName:  provider.Name(),
				providerIndex: i,
			}, nil
		}

		// Record fallback attempt failure
		if collector != nil && i > 0 {
			_ = collector.RecordEvent(ctx, types.MetricEvent{
				Type:          types.MetricEventProviderSwitch,
				ProviderName:  f.name,
				ProviderType:  f.Type(),
				ModelID:       opts.Model,
				Timestamp:     time.Now(),
				FromProvider:  previousProvider,
				ToProvider:    provider.Name(),
				SwitchReason:  "fallback_attempt",
				AttemptNumber: i + 1,
				ErrorMessage:  err.Error(),
				Latency:       latency,
			})
		}

		previousProvider = provider.Name()
		lastErr = err
	}

	// All providers failed
	if collector != nil {
		errorMsg := "no providers available"
		if lastErr != nil {
			errorMsg = fmt.Sprintf("all providers failed, last error: %v", lastErr)
		}

		_ = collector.RecordEvent(ctx, types.MetricEvent{
			Type:          types.MetricEventError,
			ProviderName:  f.name,
			ProviderType:  f.Type(),
			ModelID:       opts.Model,
			Timestamp:     time.Now(),
			ErrorMessage:  errorMsg,
			ErrorType:     "fallback_all_failed",
			AttemptNumber: len(providers),
		})
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
	}
	return nil, fmt.Errorf("no providers available")
}

type fallbackStream struct {
	inner         types.ChatCompletionStream
	providerName  string
	providerIndex int
}

func (s *fallbackStream) Next() (types.ChatCompletionChunk, error) {
	chunk, err := s.inner.Next()
	if chunk.Metadata == nil {
		chunk.Metadata = make(map[string]interface{})
	}
	chunk.Metadata["fallback_provider"] = s.providerName
	chunk.Metadata["fallback_index"] = s.providerIndex
	return chunk, err
}

func (s *fallbackStream) Close() error {
	return s.inner.Close()
}
