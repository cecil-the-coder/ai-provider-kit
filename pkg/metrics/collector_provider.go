package metrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// providerMetrics holds per-provider aggregated metrics
type providerMetrics struct {
	mu sync.RWMutex

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

	initializations  atomic.Int64
	healthChecks     atomic.Int64
	healthCheckFails atomic.Int64
	rateLimitHits    atomic.Int64

	modelUsage map[string]*atomic.Int64

	lastRequestTime time.Time
	lastUpdated     time.Time
}

// newProviderMetrics creates a new providerMetrics instance
func newProviderMetrics(name string, providerType types.ProviderType, costCalculator CostCalculator) *providerMetrics {
	return &providerMetrics{
		providerName:     name,
		providerType:     providerType,
		latencyHistogram: NewHistogram(1000),
		tokenMetrics:     newTokenMetrics(),
		errorMetrics:     newErrorMetrics(),
		streamMetrics:    newStreamMetrics(),
		costCalculator:   costCalculator,
		modelUsage:       make(map[string]*atomic.Int64),
	}
}

// RecordEvent records a metric event for this provider
func (pm *providerMetrics) RecordEvent(event types.MetricEvent) {
	now := time.Now()

	pm.mu.Lock()
	pm.lastRequestTime = event.Timestamp
	pm.lastUpdated = now
	pm.mu.Unlock()

	switch event.Type {
	case types.MetricEventRequest:
		pm.totalRequests.Add(1)
	case types.MetricEventSuccess:
		pm.successfulRequests.Add(1)
		if event.Latency > 0 {
			pm.latencyHistogram.Add(event.Latency)
		}
	case types.MetricEventError, types.MetricEventTimeout, types.MetricEventRateLimit:
		pm.failedRequests.Add(1)
		pm.errorMetrics.RecordError(event)
	case types.MetricEventHealthCheck:
		pm.healthChecks.Add(1)
	case types.MetricEventInitialization:
		pm.initializations.Add(1)
	case types.MetricEventStreamStart:
		pm.streamMetrics.RecordStreamStart(event)
	case types.MetricEventStreamEnd:
		pm.streamMetrics.RecordStreamEnd(event)
	case types.MetricEventStreamAbort:
		pm.streamMetrics.RecordStreamAbort()
	}

	if event.Type == types.MetricEventRateLimit {
		pm.rateLimitHits.Add(1)
	}

	// Track model usage
	if event.ModelID != "" {
		pm.mu.Lock()
		if pm.modelUsage[event.ModelID] == nil {
			pm.modelUsage[event.ModelID] = &atomic.Int64{}
		}
		usage := pm.modelUsage[event.ModelID]
		pm.mu.Unlock()
		usage.Add(1)
	}

	// Update token metrics
	if event.TokensUsed > 0 || event.InputTokens > 0 || event.OutputTokens > 0 {
		var cost *Cost
		if event.InputTokens > 0 || event.OutputTokens > 0 {
			c := pm.costCalculator.CalculateCost(
				event.ProviderName,
				event.ModelID,
				event.InputTokens,
				event.OutputTokens,
			)
			cost = &c
		}
		pm.tokenMetrics.RecordTokens(event, cost)
	}
}

// GetSnapshot returns a snapshot of the provider metrics
func (pm *providerMetrics) GetSnapshot() *types.ProviderMetricsSnapshot {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	totalReq := pm.totalRequests.Load()
	successReq := pm.successfulRequests.Load()
	failedReq := pm.failedRequests.Load()
	healthChecks := pm.healthChecks.Load()
	healthCheckFails := pm.healthCheckFails.Load()

	modelUsage := make(map[string]int64)
	for model, counter := range pm.modelUsage {
		modelUsage[model] = counter.Load()
	}

	return &types.ProviderMetricsSnapshot{
		Provider:           pm.providerName,
		ProviderType:       pm.providerType,
		TotalRequests:      totalReq,
		SuccessfulRequests: successReq,
		FailedRequests:     failedReq,
		SuccessRate:        calculateRate(successReq, totalReq),
		Latency:            pm.latencyHistogram.GetLatencyMetrics(),
		Tokens:             pm.tokenMetrics.GetSnapshot(),
		Errors:             pm.errorMetrics.GetSnapshot(totalReq),
		Streaming:          pm.streamMetrics.GetSnapshot(),
		Initializations:    pm.initializations.Load(),
		HealthChecks:       healthChecks,
		HealthCheckFails:   healthCheckFails,
		HealthCheckRate:    calculateRate(healthChecks-healthCheckFails, healthChecks),
		RateLimitHits:      pm.rateLimitHits.Load(),
		ModelUsage:         modelUsage,
		LastRequestTime:    pm.lastRequestTime,
		LastUpdated:        pm.lastUpdated,
	}
}
