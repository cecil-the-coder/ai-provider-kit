package metrics

// CostCalculator calculates costs for AI requests.
//
// Example usage:
//
//	// Define your own cost calculator with your pricing
//	type MyCostCalculator struct {
//		// your pricing data
//	}
//
//	func (c *MyCostCalculator) CalculateCost(provider, model string, inputTokens, outputTokens int64) Cost {
//		// your pricing logic
//		return Cost{
//			InputCost:  float64(inputTokens) * inputPricePerToken,
//			OutputCost: float64(outputTokens) * outputPricePerToken,
//			TotalCost:  totalCost,
//			Currency:   "USD",
//		}
//	}
//
//	func (c *MyCostCalculator) GetPricing(provider, model string) (inputPer1K, outputPer1K float64, ok bool) {
//		// return your pricing per 1K tokens
//	}
//
//	// Use it with the collector
//	costCalc := &MyCostCalculator{}
//	collector := NewDefaultMetricsCollector(costCalc)
//
// CostCalculator calculates costs for AI requests.
// This is a pluggable interface - consumers provide their own implementation
// with their specific pricing models.
type CostCalculator interface {
	// CalculateCost returns the cost for a request
	CalculateCost(provider, model string, inputTokens, outputTokens int64) Cost

	// GetPricing returns pricing info (for display/debugging)
	// Returns inputPer1K, outputPer1K prices and ok=true if pricing exists
	GetPricing(provider, model string) (inputPer1K, outputPer1K float64, ok bool)
}

// Cost represents the calculated cost of an AI request
type Cost struct {
	InputCost  float64 `json:"input_cost"`
	OutputCost float64 `json:"output_cost"`
	TotalCost  float64 `json:"total_cost"`
	Currency   string  `json:"currency"`
}

// NullCostCalculator is a no-op cost calculator that returns zero costs.
// Use this when no pricing configuration is available.
type NullCostCalculator struct{}

// NewNullCostCalculator creates a new NullCostCalculator
func NewNullCostCalculator() *NullCostCalculator {
	return &NullCostCalculator{}
}

// CalculateCost returns zero cost
func (n *NullCostCalculator) CalculateCost(provider, model string, inputTokens, outputTokens int64) Cost {
	return Cost{
		InputCost:  0,
		OutputCost: 0,
		TotalCost:  0,
		Currency:   "USD",
	}
}

// GetPricing returns ok=false indicating no pricing is available
func (n *NullCostCalculator) GetPricing(provider, model string) (inputPer1K, outputPer1K float64, ok bool) {
	return 0, 0, false
}
