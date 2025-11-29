package metrics

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

const floatTolerance = 1e-6

// Helper function to compare floats with tolerance
func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < floatTolerance
}

// MockCostCalculator is a test implementation of CostCalculator
type MockCostCalculator struct {
	pricing map[string]map[string]struct {
		inputPer1K  float64
		outputPer1K float64
	}
	currency string
}

func NewMockCostCalculator() *MockCostCalculator {
	mc := &MockCostCalculator{
		pricing: make(map[string]map[string]struct {
			inputPer1K  float64
			outputPer1K float64
		}),
		currency: "USD",
	}

	// Add some example pricing
	mc.SetPricing("anthropic", "claude-3-5-sonnet-20241022", 3.0/1000, 15.0/1000) // $3 per 1M input, $15 per 1M output
	mc.SetPricing("openai", "gpt-4", 30.0/1000, 60.0/1000)                        // $30 per 1M input, $60 per 1M output
	mc.SetPricing("openai", "gpt-3.5-turbo", 0.5/1000, 1.5/1000)                  // $0.50 per 1M input, $1.50 per 1M output

	return mc
}

func (m *MockCostCalculator) SetPricing(provider, model string, inputPer1K, outputPer1K float64) {
	if m.pricing[provider] == nil {
		m.pricing[provider] = make(map[string]struct {
			inputPer1K  float64
			outputPer1K float64
		})
	}
	m.pricing[provider][model] = struct {
		inputPer1K  float64
		outputPer1K float64
	}{inputPer1K, outputPer1K}
}

func (m *MockCostCalculator) CalculateCost(provider, model string, inputTokens, outputTokens int64) Cost {
	inputPer1K, outputPer1K, ok := m.GetPricing(provider, model)
	if !ok {
		return Cost{
			InputCost:  0,
			OutputCost: 0,
			TotalCost:  0,
			Currency:   m.currency,
		}
	}

	inputCost := float64(inputTokens) * inputPer1K / 1000
	outputCost := float64(outputTokens) * outputPer1K / 1000

	return Cost{
		InputCost:  inputCost,
		OutputCost: outputCost,
		TotalCost:  inputCost + outputCost,
		Currency:   m.currency,
	}
}

func (m *MockCostCalculator) GetPricing(provider, model string) (inputPer1K, outputPer1K float64, ok bool) {
	providerPricing, exists := m.pricing[provider]
	if !exists {
		return 0, 0, false
	}

	modelPricing, exists := providerPricing[model]
	if !exists {
		return 0, 0, false
	}

	return modelPricing.inputPer1K, modelPricing.outputPer1K, true
}

func TestNullCostCalculator(t *testing.T) {
	calc := NewNullCostCalculator()

	// Test CalculateCost
	cost := calc.CalculateCost("anthropic", "claude-3-5-sonnet-20241022", 1000, 500)
	if cost.InputCost != 0 {
		t.Errorf("Expected InputCost to be 0, got %f", cost.InputCost)
	}
	if cost.OutputCost != 0 {
		t.Errorf("Expected OutputCost to be 0, got %f", cost.OutputCost)
	}
	if cost.TotalCost != 0 {
		t.Errorf("Expected TotalCost to be 0, got %f", cost.TotalCost)
	}
	if cost.Currency != "USD" {
		t.Errorf("Expected Currency to be USD, got %s", cost.Currency)
	}

	// Test GetPricing
	inputPer1K, outputPer1K, ok := calc.GetPricing("anthropic", "claude-3-5-sonnet-20241022")
	if ok {
		t.Error("Expected ok to be false for NullCostCalculator")
	}
	if inputPer1K != 0 || outputPer1K != 0 {
		t.Errorf("Expected pricing to be 0, got inputPer1K=%f, outputPer1K=%f", inputPer1K, outputPer1K)
	}
}

func TestMockCostCalculator(t *testing.T) {
	calc := NewMockCostCalculator()

	// Test GetPricing
	inputPer1K, outputPer1K, ok := calc.GetPricing("anthropic", "claude-3-5-sonnet-20241022")
	if !ok {
		t.Fatal("Expected ok to be true")
	}
	if inputPer1K != 3.0/1000 {
		t.Errorf("Expected inputPer1K to be %f, got %f", 3.0/1000, inputPer1K)
	}
	if outputPer1K != 15.0/1000 {
		t.Errorf("Expected outputPer1K to be %f, got %f", 15.0/1000, outputPer1K)
	}

	// Test CalculateCost
	// 1000 input tokens at $3/1M = $0.003
	// 500 output tokens at $15/1M = $0.0075
	// Total = $0.0105
	cost := calc.CalculateCost("anthropic", "claude-3-5-sonnet-20241022", 1000, 500)
	if !floatEquals(cost.InputCost, 0.003) {
		t.Errorf("Expected InputCost to be 0.003, got %f", cost.InputCost)
	}
	if !floatEquals(cost.OutputCost, 0.0075) {
		t.Errorf("Expected OutputCost to be 0.0075, got %f", cost.OutputCost)
	}
	if !floatEquals(cost.TotalCost, 0.0105) {
		t.Errorf("Expected TotalCost to be 0.0105, got %f", cost.TotalCost)
	}
	if cost.Currency != "USD" {
		t.Errorf("Expected Currency to be USD, got %s", cost.Currency)
	}

	// Test unknown provider/model
	cost = calc.CalculateCost("unknown", "model", 1000, 500)
	if cost.TotalCost != 0 {
		t.Errorf("Expected TotalCost to be 0 for unknown provider/model, got %f", cost.TotalCost)
	}
}

func TestCollectorWithNullCostCalculator(t *testing.T) {
	collector := NewDefaultMetricsCollector()

	event := types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "anthropic",
		ModelID:      "claude-3-5-sonnet-20241022",
		InputTokens:  1000,
		OutputTokens: 500,
		TokensUsed:   1500,
		Timestamp:    time.Now(),
	}

	err := collector.RecordEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("Failed to record event: %v", err)
	}

	snapshot := collector.GetSnapshot()
	if snapshot.Tokens.EstimatedCost != 0 {
		t.Errorf("Expected EstimatedCost to be 0 with NullCostCalculator, got %f", snapshot.Tokens.EstimatedCost)
	}
	if snapshot.Tokens.EstimatedInputCost != 0 {
		t.Errorf("Expected EstimatedInputCost to be 0 with NullCostCalculator, got %f", snapshot.Tokens.EstimatedInputCost)
	}
	if snapshot.Tokens.EstimatedOutputCost != 0 {
		t.Errorf("Expected EstimatedOutputCost to be 0 with NullCostCalculator, got %f", snapshot.Tokens.EstimatedOutputCost)
	}
}

func TestCollectorWithMockCostCalculator(t *testing.T) {
	calc := NewMockCostCalculator()
	collector := NewDefaultMetricsCollector(calc)

	event := types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "anthropic",
		ModelID:      "claude-3-5-sonnet-20241022",
		InputTokens:  1000,
		OutputTokens: 500,
		TokensUsed:   1500,
		Timestamp:    time.Now(),
	}

	err := collector.RecordEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("Failed to record event: %v", err)
	}

	snapshot := collector.GetSnapshot()

	// Expected: 1000 * 0.003/1000 = 0.003 for input, 500 * 0.015/1000 = 0.0075 for output
	expectedInputCost := 0.003
	expectedOutputCost := 0.0075
	expectedTotalCost := 0.0105

	if !floatEquals(snapshot.Tokens.EstimatedInputCost, expectedInputCost) {
		t.Errorf("Expected EstimatedInputCost to be %f, got %f", expectedInputCost, snapshot.Tokens.EstimatedInputCost)
	}
	if !floatEquals(snapshot.Tokens.EstimatedOutputCost, expectedOutputCost) {
		t.Errorf("Expected EstimatedOutputCost to be %f, got %f", expectedOutputCost, snapshot.Tokens.EstimatedOutputCost)
	}
	if !floatEquals(snapshot.Tokens.EstimatedCost, expectedTotalCost) {
		t.Errorf("Expected EstimatedCost to be %f, got %f", expectedTotalCost, snapshot.Tokens.EstimatedCost)
	}
	if snapshot.Tokens.Currency != "USD" {
		t.Errorf("Expected Currency to be USD, got %s", snapshot.Tokens.Currency)
	}
}

func TestCollectorWithMultipleEvents(t *testing.T) {
	calc := NewMockCostCalculator()
	collector := NewDefaultMetricsCollector(calc)

	type eventPair struct {
		providerName string
		modelID      string
		inputTokens  int64
		outputTokens int64
		tokensUsed   int64
	}

	events := []eventPair{
		{
			providerName: "anthropic",
			modelID:      "claude-3-5-sonnet-20241022",
			inputTokens:  1000,
			outputTokens: 500,
			tokensUsed:   1500,
		},
		{
			providerName: "openai",
			modelID:      "gpt-4",
			inputTokens:  2000,
			outputTokens: 1000,
			tokensUsed:   3000,
		},
		{
			providerName: "openai",
			modelID:      "gpt-3.5-turbo",
			inputTokens:  5000,
			outputTokens: 2000,
			tokensUsed:   7000,
		},
	}

	for _, ep := range events {
		// Record request event
		requestEvent := types.MetricEvent{
			Type:         types.MetricEventRequest,
			ProviderName: ep.providerName,
			ModelID:      ep.modelID,
			Timestamp:    time.Now(),
		}
		err := collector.RecordEvent(context.Background(), requestEvent)
		if err != nil {
			t.Fatalf("Failed to record request event: %v", err)
		}

		// Record success event with tokens
		successEvent := types.MetricEvent{
			Type:         types.MetricEventSuccess,
			ProviderName: ep.providerName,
			ModelID:      ep.modelID,
			InputTokens:  ep.inputTokens,
			OutputTokens: ep.outputTokens,
			TokensUsed:   ep.tokensUsed,
			Timestamp:    time.Now(),
		}
		err = collector.RecordEvent(context.Background(), successEvent)
		if err != nil {
			t.Fatalf("Failed to record success event: %v", err)
		}
	}

	// Test aggregate snapshot
	snapshot := collector.GetSnapshot()

	// Total tokens: 1500 + 3000 + 7000 = 11500
	if snapshot.Tokens.TotalTokens != 11500 {
		t.Errorf("Expected TotalTokens to be 11500, got %d", snapshot.Tokens.TotalTokens)
	}

	// Total costs:
	// Event 1: 1000 * 0.003/1000 + 500 * 0.015/1000 = 0.003 + 0.0075 = 0.0105
	// Event 2: 2000 * 0.030/1000 + 1000 * 0.060/1000 = 0.06 + 0.06 = 0.12
	// Event 3: 5000 * 0.0005/1000 + 2000 * 0.0015/1000 = 0.0025 + 0.003 = 0.0055
	// Total: 0.0105 + 0.12 + 0.0055 = 0.136
	expectedTotalCost := 0.136
	if !floatEquals(snapshot.Tokens.EstimatedCost, expectedTotalCost) {
		t.Errorf("Expected EstimatedCost to be %f, got %f", expectedTotalCost, snapshot.Tokens.EstimatedCost)
	}

	// Test provider-specific snapshots
	anthropicSnapshot := collector.GetProviderMetrics("anthropic")
	if anthropicSnapshot == nil {
		t.Fatal("Expected anthropic provider snapshot")
	}
	if !floatEquals(anthropicSnapshot.Tokens.EstimatedCost, 0.0105) {
		t.Errorf("Expected anthropic cost to be 0.0105, got %f", anthropicSnapshot.Tokens.EstimatedCost)
	}

	openaiSnapshot := collector.GetProviderMetrics("openai")
	if openaiSnapshot == nil {
		t.Fatal("Expected openai provider snapshot")
	}
	expectedOpenAICost := 0.12 + 0.0055 // 0.1255
	if !floatEquals(openaiSnapshot.Tokens.EstimatedCost, expectedOpenAICost) {
		t.Errorf("Expected openai cost to be %f, got %f", expectedOpenAICost, openaiSnapshot.Tokens.EstimatedCost)
	}

	// Test model-specific snapshots
	claudeSnapshot := collector.GetModelMetrics("claude-3-5-sonnet-20241022")
	if claudeSnapshot == nil {
		t.Fatal("Expected claude model snapshot")
	}
	if !floatEquals(claudeSnapshot.Tokens.EstimatedCost, 0.0105) {
		t.Errorf("Expected claude cost to be 0.0105, got %f", claudeSnapshot.Tokens.EstimatedCost)
	}
	if !floatEquals(claudeSnapshot.EstimatedCostPerRequest, 0.0105) {
		t.Errorf("Expected claude EstimatedCostPerRequest to be 0.0105, got %f", claudeSnapshot.EstimatedCostPerRequest)
	}

	gpt4Snapshot := collector.GetModelMetrics("gpt-4")
	if gpt4Snapshot == nil {
		t.Fatal("Expected gpt-4 model snapshot")
	}
	if !floatEquals(gpt4Snapshot.Tokens.EstimatedCost, 0.12) {
		t.Errorf("Expected gpt-4 cost to be 0.12, got %f", gpt4Snapshot.Tokens.EstimatedCost)
	}
	if !floatEquals(gpt4Snapshot.EstimatedCostPerRequest, 0.12) {
		t.Errorf("Expected gpt-4 EstimatedCostPerRequest to be 0.12, got %f", gpt4Snapshot.EstimatedCostPerRequest)
	}
}

func TestCollectorWithZeroTokens(t *testing.T) {
	calc := NewMockCostCalculator()
	collector := NewDefaultMetricsCollector(calc)

	event := types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "anthropic",
		ModelID:      "claude-3-5-sonnet-20241022",
		InputTokens:  0,
		OutputTokens: 0,
		TokensUsed:   0,
		Timestamp:    time.Now(),
	}

	err := collector.RecordEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("Failed to record event: %v", err)
	}

	snapshot := collector.GetSnapshot()
	if snapshot.Tokens.EstimatedCost != 0 {
		t.Errorf("Expected EstimatedCost to be 0 for zero tokens, got %f", snapshot.Tokens.EstimatedCost)
	}
}

func TestCollectorCostAveragePerRequest(t *testing.T) {
	calc := NewMockCostCalculator()
	collector := NewDefaultMetricsCollector(calc)

	// Record 3 requests with the same model
	for i := 0; i < 3; i++ {
		// First record the request event
		requestEvent := types.MetricEvent{
			Type:         types.MetricEventRequest,
			ProviderName: "openai",
			ModelID:      "gpt-3.5-turbo",
			Timestamp:    time.Now(),
		}
		err := collector.RecordEvent(context.Background(), requestEvent)
		if err != nil {
			t.Fatalf("Failed to record request event: %v", err)
		}

		// Then record the success event with tokens
		successEvent := types.MetricEvent{
			Type:         types.MetricEventSuccess,
			ProviderName: "openai",
			ModelID:      "gpt-3.5-turbo",
			InputTokens:  1000,
			OutputTokens: 500,
			TokensUsed:   1500,
			Timestamp:    time.Now(),
		}
		err = collector.RecordEvent(context.Background(), successEvent)
		if err != nil {
			t.Fatalf("Failed to record success event: %v", err)
		}
	}

	modelSnapshot := collector.GetModelMetrics("gpt-3.5-turbo")
	if modelSnapshot == nil {
		t.Fatal("Expected gpt-3.5-turbo model snapshot")
	}

	// Each request: 1000 * 0.0005/1000 + 500 * 0.0015/1000 = 0.0005 + 0.00075 = 0.00125
	// Total for 3 requests: 0.00375
	expectedTotalCost := 0.00375
	if !floatEquals(modelSnapshot.Tokens.EstimatedCost, expectedTotalCost) {
		t.Errorf("Expected total cost to be %f, got %f", expectedTotalCost, modelSnapshot.Tokens.EstimatedCost)
	}

	// Average per request: 0.00375 / 3 = 0.00125
	expectedAvgCost := 0.00125
	if !floatEquals(modelSnapshot.EstimatedCostPerRequest, expectedAvgCost) {
		t.Errorf("Expected EstimatedCostPerRequest to be %f, got %f", expectedAvgCost, modelSnapshot.EstimatedCostPerRequest)
	}
}
