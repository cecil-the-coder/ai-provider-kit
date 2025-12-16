package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPricing tests the Pricing struct
func TestPricing(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var pricing Pricing
		assert.Equal(t, 0.0, pricing.InputTokenPrice)
		assert.Equal(t, 0.0, pricing.OutputTokenPrice)
		assert.Empty(t, pricing.Unit)
	})

	t.Run("FullPricing", func(t *testing.T) {
		pricing := Pricing{
			InputTokenPrice:  0.001,
			OutputTokenPrice: 0.002,
			Unit:             "USD",
		}

		assert.Equal(t, 0.001, pricing.InputTokenPrice)
		assert.Equal(t, 0.002, pricing.OutputTokenPrice)
		assert.Equal(t, "USD", pricing.Unit)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		pricing := Pricing{
			InputTokenPrice:  0.003,
			OutputTokenPrice: 0.015,
			Unit:             "USD",
		}

		data, err := json.Marshal(pricing)
		require.NoError(t, err)

		var result Pricing
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, pricing, result)
	})

	t.Run("CalculateCost", func(t *testing.T) {
		pricing := Pricing{
			InputTokenPrice:  0.001, // $0.001 per 1K tokens
			OutputTokenPrice: 0.002, // $0.002 per 1K tokens
			Unit:             "USD",
		}

		// Calculate cost for 1000 input tokens and 500 output tokens
		cost := pricing.CalculateCost(1000, 500)
		expectedCost := 0.001*1.0 + 0.002*0.5 // $0.001 + $0.001 = $0.002
		assert.Equal(t, expectedCost, cost)

		// Calculate cost for 2000 input tokens and 1000 output tokens
		cost = pricing.CalculateCost(2000, 1000)
		expectedCost = 0.001*2.0 + 0.002*1.0 // $0.002 + $0.002 = $0.004
		assert.Equal(t, expectedCost, cost)
	})

	t.Run("CalculateCostWithUsage", func(t *testing.T) {
		pricing := Pricing{
			InputTokenPrice:  0.002,
			OutputTokenPrice: 0.006,
			Unit:             "USD",
		}

		usage := Usage{
			PromptTokens:     1500,
			CompletionTokens: 800,
			TotalTokens:      2300,
		}

		cost := pricing.CalculateCostWithUsage(usage)
		expectedCost := 0.002*1.5 + 0.006*0.8 // $0.003 + $0.0048 = $0.0078
		assert.Equal(t, expectedCost, cost)
	})

	t.Run("EdgeCases", func(t *testing.T) {
		pricing := Pricing{
			InputTokenPrice:  0.001,
			OutputTokenPrice: 0.002,
			Unit:             "USD",
		}

		// Zero tokens
		cost := pricing.CalculateCost(0, 0)
		assert.Equal(t, 0.0, cost)

		// Negative tokens (should handle gracefully)
		cost = pricing.CalculateCost(-100, 50)
		assert.Equal(t, 0.0, cost) // Should return 0 for invalid input

		// Very large numbers
		cost = pricing.CalculateCost(1000000, 500000)
		expectedCost := 0.001*1000.0 + 0.002*500.0 // $1.0 + $1.0 = $2.0
		assert.Equal(t, expectedCost, cost)
	})
}

// TestUsage tests the Usage struct
func TestUsage(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var usage Usage
		assert.Equal(t, 0, usage.PromptTokens)
		assert.Equal(t, 0, usage.CompletionTokens)
		assert.Equal(t, 0, usage.TotalTokens)
	})

	t.Run("Creation", func(t *testing.T) {
		usage := Usage{
			PromptTokens:     150,
			CompletionTokens: 75,
			TotalTokens:      225,
		}

		assert.Equal(t, 150, usage.PromptTokens)
		assert.Equal(t, 75, usage.CompletionTokens)
		assert.Equal(t, 225, usage.TotalTokens)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		usage := Usage{
			PromptTokens:     1200,
			CompletionTokens: 800,
			TotalTokens:      2000,
		}

		data, err := json.Marshal(usage)
		require.NoError(t, err)

		var result Usage
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, usage, result)
	})

	t.Run("CalculateTotal", func(t *testing.T) {
		usage := Usage{
			PromptTokens:     500,
			CompletionTokens: 300,
		}

		// Calculate total tokens
		usage.CalculateTotal()
		assert.Equal(t, 800, usage.TotalTokens)

		// Update tokens and recalculate
		usage.PromptTokens = 750
		usage.CompletionTokens = 250
		usage.CalculateTotal()
		assert.Equal(t, 1000, usage.TotalTokens)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name  string
			usage Usage
			valid bool
		}{
			{
				name:  "ZeroUsage",
				usage: Usage{},
				valid: true,
			},
			{
				name: "ValidUsage",
				usage: Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				},
				valid: true,
			},
			{
				name: "IncorrectTotal",
				usage: Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      200, // Should be 150
				},
				valid: false,
			},
			{
				name: "NegativeTokens",
				usage: Usage{
					PromptTokens:     -10,
					CompletionTokens: 50,
					TotalTokens:      40,
				},
				valid: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				valid := tt.usage.Validate()
				assert.Equal(t, tt.valid, valid)
			})
		}
	})

	t.Run("Add", func(t *testing.T) {
		usage1 := Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		}

		usage2 := Usage{
			PromptTokens:     200,
			CompletionTokens: 75,
			TotalTokens:      275,
		}

		combined := usage1.Add(usage2)
		assert.Equal(t, 300, combined.PromptTokens)
		assert.Equal(t, 125, combined.CompletionTokens)
		assert.Equal(t, 425, combined.TotalTokens)
	})
}
