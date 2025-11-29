package fallback

import (
	"context"
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// FallbackProvider tries providers in order until one succeeds
type FallbackProvider struct {
	name      string
	providers []types.Provider
	config    *Config
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

func (f *FallbackProvider) Name() string              { return f.name }
func (f *FallbackProvider) Type() types.ProviderType { return "fallback" }
func (f *FallbackProvider) Description() string      { return "Tries providers in order until one succeeds" }

func (f *FallbackProvider) GenerateChatCompletion(ctx context.Context, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
	var lastErr error

	for i, provider := range f.providers {
		chatProvider, ok := provider.(types.ChatProvider)
		if !ok {
			continue
		}

		stream, err := chatProvider.GenerateChatCompletion(ctx, opts)
		if err == nil {
			return &fallbackStream{
				inner:         stream,
				providerName:  provider.Name(),
				providerIndex: i,
			}, nil
		}

		lastErr = err
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
