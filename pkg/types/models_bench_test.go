package types

import (
	"encoding/json"
	"testing"
)

// BenchmarkModelMarshal benchmarks JSON marshaling of Model struct
func BenchmarkModelMarshal(b *testing.B) {
	model := Model{
		ID:                  "gpt-4",
		Name:                "GPT-4",
		Provider:            ProviderTypeOpenAI,
		Description:         "Large language model",
		MaxTokens:           8192,
		SupportsStreaming:   true,
		SupportsToolCalling: true,
		Capabilities:        []string{"text", "vision", "function_calling"},
		Pricing: Pricing{
			InputTokenPrice:  0.001,
			OutputTokenPrice: 0.002,
			Unit:             "USD",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(model)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUsageCalculateTotal benchmarks token calculation
func BenchmarkUsageCalculateTotal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		usage := Usage{
			PromptTokens:     1000 + (i % 1000),
			CompletionTokens: 500 + (i % 500),
		}
		usage.CalculateTotal()
	}
}
