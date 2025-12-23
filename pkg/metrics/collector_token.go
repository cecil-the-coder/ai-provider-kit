package metrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// tokenMetrics tracks token usage and costs
type tokenMetrics struct {
	mu sync.Mutex

	totalTokens     atomic.Int64
	inputTokens     atomic.Int64
	outputTokens    atomic.Int64
	cachedTokens    atomic.Int64
	cacheReadTokens atomic.Int64
	reasoningTokens atomic.Int64

	// Cost tracking (protected by mu)
	totalCost  float64
	inputCost  float64
	outputCost float64
	currency   string

	lastUpdated time.Time
}

// newTokenMetrics creates a new tokenMetrics instance
func newTokenMetrics() *tokenMetrics {
	return &tokenMetrics{
		currency: "USD", // Default currency
	}
}

// RecordTokens records token usage with associated costs
func (tm *tokenMetrics) RecordTokens(event types.MetricEvent, cost *Cost) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if event.TokensUsed > 0 {
		tm.totalTokens.Add(event.TokensUsed)
	}
	if event.InputTokens > 0 {
		tm.inputTokens.Add(event.InputTokens)
	}
	if event.OutputTokens > 0 {
		tm.outputTokens.Add(event.OutputTokens)
	}

	// Record costs if provided
	if cost != nil {
		tm.inputCost += cost.InputCost
		tm.outputCost += cost.OutputCost
		tm.totalCost += cost.TotalCost
		if cost.Currency != "" {
			tm.currency = cost.Currency
		}
	}

	tm.lastUpdated = time.Now()
}

// GetSnapshot returns a snapshot of the token metrics
func (tm *tokenMetrics) GetSnapshot() types.TokenMetrics {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	totalTokens := tm.totalTokens.Load()
	inputTokens := tm.inputTokens.Load()
	outputTokens := tm.outputTokens.Load()

	return types.TokenMetrics{
		TotalTokens:         totalTokens,
		InputTokens:         inputTokens,
		OutputTokens:        outputTokens,
		CachedTokens:        tm.cachedTokens.Load(),
		CacheReadTokens:     tm.cacheReadTokens.Load(),
		ReasoningTokens:     tm.reasoningTokens.Load(),
		EstimatedCost:       tm.totalCost,
		Currency:            tm.currency,
		EstimatedInputCost:  tm.inputCost,
		EstimatedOutputCost: tm.outputCost,
		LastUpdated:         tm.lastUpdated,
	}
}
