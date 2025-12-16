package openai

import (
	"context"
	"strings"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// BenchmarkOpenAIProvider_Creation benchmarks provider creation
func BenchmarkOpenAIProvider_Creation(b *testing.B) {
	config := types.ProviderConfig{
		Type:                 types.ProviderTypeOpenAI,
		APIKey:               "sk-benchmark-key",
		BaseURL:              "https://api.openai.com/v1",
		DefaultModel:         "gpt-4",
		SupportsStreaming:    true,
		SupportsToolCalling:  true,
		SupportsResponsesAPI: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewOpenAIProvider(config)
	}
}

// BenchmarkOpenAIProvider_GetModels benchmarks model retrieval
func BenchmarkOpenAIProvider_GetModels(b *testing.B) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "", // Use empty API key to avoid real API calls
	}
	provider := NewOpenAIProvider(config)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := provider.GetModels(ctx)
		// Handle expected errors gracefully in benchmarks
		if err != nil {
			if strings.Contains(err.Error(), "no OpenAI API key configured") {
				continue
			}
			b.Fatal(err)
		}
	}
}

// BenchmarkOpenAIProvider_GenerateChatCompletion benchmarks chat completion generation
func BenchmarkOpenAIProvider_GenerateChatCompletion(b *testing.B) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "", // Use empty API key to avoid real API calls
	}
	provider := NewOpenAIProvider(config)
	options := types.GenerateOptions{
		Prompt: "benchmark prompt",
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, err := provider.GenerateChatCompletion(ctx, options)
		// Handle expected errors gracefully in benchmarks
		if err != nil {
			if strings.Contains(err.Error(), "no API keys configured") {
				continue
			}
			b.Fatal(err)
		}
		if stream != nil {
			_ = stream.Close()
		}
	}
}

// BenchmarkOpenAIProvider_Authentication benchmarks authentication operations
func BenchmarkOpenAIProvider_Authentication(b *testing.B) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewOpenAIProvider(config)
	authConfig := types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "sk-local-benchmark-key", // Use fake key but avoid network calls
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Benchmark only the authentication method call, not the actual validation
		_ = provider.Authenticate(ctx, authConfig)
		_ = provider.IsAuthenticated()
	}
}
