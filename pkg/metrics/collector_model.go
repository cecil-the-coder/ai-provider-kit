package metrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// modelMetrics holds per-model aggregated metrics
type modelMetrics struct {
	mu sync.RWMutex

	modelID      string
	providerName string
	providerType types.ProviderType

	totalRequests      atomic.Int64
	successfulRequests atomic.Int64
	failedRequests     atomic.Int64

	latencyHistogram *Histogram

	tokenMetrics  *tokenMetrics
	errorMetrics  *errorMetrics
	streamMetrics *streamMetrics

	// Cost calculation (shared reference from collector)
	costCalculator CostCalculator

	lastRequestTime time.Time
	lastUpdated     time.Time
}

// newModelMetrics creates a new modelMetrics instance
func newModelMetrics(modelID, providerName string, providerType types.ProviderType, costCalculator CostCalculator) *modelMetrics {
	return &modelMetrics{
		modelID:          modelID,
		providerName:     providerName,
		providerType:     providerType,
		latencyHistogram: NewHistogram(1000),
		tokenMetrics:     newTokenMetrics(),
		errorMetrics:     newErrorMetrics(),
		streamMetrics:    newStreamMetrics(),
		costCalculator:   costCalculator,
	}
}

// RecordEvent records a metric event for this model
func (mm *modelMetrics) RecordEvent(event types.MetricEvent) {
	now := time.Now()

	mm.mu.Lock()
	mm.lastRequestTime = event.Timestamp
	mm.lastUpdated = now
	mm.mu.Unlock()

	switch event.Type {
	case types.MetricEventRequest:
		mm.totalRequests.Add(1)
	case types.MetricEventSuccess:
		mm.successfulRequests.Add(1)
		if event.Latency > 0 {
			mm.latencyHistogram.Add(event.Latency)
		}
	case types.MetricEventError, types.MetricEventTimeout, types.MetricEventRateLimit:
		mm.failedRequests.Add(1)
		mm.errorMetrics.RecordError(event)
	case types.MetricEventStreamStart:
		mm.streamMetrics.RecordStreamStart(event)
	case types.MetricEventStreamEnd:
		mm.streamMetrics.RecordStreamEnd(event)
	case types.MetricEventStreamAbort:
		mm.streamMetrics.RecordStreamAbort()
	}

	// Update token metrics
	if event.TokensUsed > 0 || event.InputTokens > 0 || event.OutputTokens > 0 {
		var cost *Cost
		if event.InputTokens > 0 || event.OutputTokens > 0 {
			c := mm.costCalculator.CalculateCost(
				event.ProviderName,
				event.ModelID,
				event.InputTokens,
				event.OutputTokens,
			)
			cost = &c
		}
		mm.tokenMetrics.RecordTokens(event, cost)
	}
}

// GetSnapshot returns a snapshot of the model metrics
func (mm *modelMetrics) GetSnapshot() *types.ModelMetricsSnapshot {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	totalReq := mm.totalRequests.Load()
	successReq := mm.successfulRequests.Load()
	failedReq := mm.failedRequests.Load()

	totalTokens := mm.tokenMetrics.totalTokens.Load()
	avgTokensPerReq := float64(0)
	if totalReq > 0 {
		avgTokensPerReq = float64(totalTokens) / float64(totalReq)
	}

	// Calculate average cost per request
	mm.tokenMetrics.mu.Lock()
	totalCost := mm.tokenMetrics.totalCost
	mm.tokenMetrics.mu.Unlock()

	avgCostPerReq := float64(0)
	if totalReq > 0 {
		avgCostPerReq = totalCost / float64(totalReq)
	}

	return &types.ModelMetricsSnapshot{
		ModelID:                 mm.modelID,
		Provider:                mm.providerName,
		ProviderType:            mm.providerType,
		TotalRequests:           totalReq,
		SuccessfulRequests:      successReq,
		FailedRequests:          failedReq,
		SuccessRate:             calculateRate(successReq, totalReq),
		Latency:                 mm.latencyHistogram.GetLatencyMetrics(),
		Tokens:                  mm.tokenMetrics.GetSnapshot(),
		Errors:                  mm.errorMetrics.GetSnapshot(totalReq),
		Streaming:               mm.streamMetrics.GetSnapshot(),
		AverageTokensPerRequest: avgTokensPerReq,
		EstimatedCostPerRequest: avgCostPerReq,
		LastRequestTime:         mm.lastRequestTime,
		LastUpdated:             mm.lastUpdated,
	}
}
