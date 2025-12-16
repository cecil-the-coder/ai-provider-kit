// Package metrics provides a centralized metrics collection system for ai-provider-kit.
// It includes the DefaultMetricsCollector implementation, streaming metrics wrapper,
// cost calculation interface, and histogram for percentile calculations.
package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// DefaultMetricsCollector is the default implementation of types.MetricsCollector.
// It provides thread-safe metrics collection with support for subscriptions and hooks.
type DefaultMetricsCollector struct {
	// Mutex for protecting maps and aggregate data
	mu sync.RWMutex

	// Aggregate metrics
	totalRequests      atomic.Int64
	successfulRequests atomic.Int64
	failedRequests     atomic.Int64

	// Per-provider metrics
	providerMetrics map[string]*providerMetrics

	// Per-model metrics
	modelMetrics map[string]*modelMetrics

	// Latency tracking
	latencyHistogram *Histogram

	// Streaming metrics
	streamMetrics *streamMetrics

	// Token metrics
	tokenMetrics *tokenMetrics

	// Error tracking
	errorMetrics *errorMetrics

	// Cost calculation (optional)
	costCalculator CostCalculator

	// Subscriptions
	subscriptions map[string]*subscription
	nextSubID     atomic.Int64

	// Hooks
	hooks      map[types.HookID]*hookEntry
	nextHookID atomic.Int64

	// Lifecycle
	firstRequestTime time.Time
	lastUpdated      time.Time
	closed           atomic.Bool
}

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

// errorMetrics tracks error statistics
type errorMetrics struct {
	mu sync.Mutex

	totalErrors atomic.Int64

	errorsByType   map[string]*atomic.Int64
	errorsByStatus map[string]*atomic.Int64

	rateLimitErrors      atomic.Int64
	timeoutErrors        atomic.Int64
	authenticationErrors atomic.Int64
	invalidRequestErrors atomic.Int64
	serverErrors         atomic.Int64
	networkErrors        atomic.Int64
	unknownErrors        atomic.Int64

	consecutiveErrors atomic.Int64
	lastError         string
	lastErrorType     string
	lastErrorTime     time.Time

	lastUpdated time.Time
}

// streamMetrics tracks streaming-specific metrics
type streamMetrics struct {
	mu sync.Mutex

	totalStreamRequests      atomic.Int64
	successfulStreamRequests atomic.Int64
	failedStreamRequests     atomic.Int64

	ttftHistogram *Histogram // Time to first token

	totalStreamedTokens atomic.Int64
	totalChunks         atomic.Int64

	minTokensPerSecond   float64
	maxTokensPerSecond   float64
	totalTokensPerSecond float64
	tpsCount             int64

	minStreamDuration   time.Duration
	maxStreamDuration   time.Duration
	totalStreamDuration time.Duration

	lastUpdated time.Time
}

// hookEntry wraps a hook with its metadata
type hookEntry struct {
	hook   types.MetricsHook
	id     types.HookID
	filter *types.MetricFilter
}

// NewDefaultMetricsCollector creates a new DefaultMetricsCollector instance.
// If no CostCalculator is provided, a NullCostCalculator will be used.
func NewDefaultMetricsCollector(costCalculator ...CostCalculator) *DefaultMetricsCollector {
	var cc CostCalculator
	if len(costCalculator) > 0 && costCalculator[0] != nil {
		cc = costCalculator[0]
	} else {
		cc = NewNullCostCalculator()
	}

	return &DefaultMetricsCollector{
		providerMetrics:  make(map[string]*providerMetrics),
		modelMetrics:     make(map[string]*modelMetrics),
		subscriptions:    make(map[string]*subscription),
		hooks:            make(map[types.HookID]*hookEntry),
		latencyHistogram: NewHistogram(1000),
		streamMetrics:    newStreamMetrics(),
		tokenMetrics:     newTokenMetrics(),
		errorMetrics:     newErrorMetrics(),
		costCalculator:   cc,
	}
}

// Helper function to calculate success rate
func calculateRate(numerator, denominator int64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

// providerMetrics methods

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

// modelMetrics methods

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

// tokenMetrics methods

func newTokenMetrics() *tokenMetrics {
	return &tokenMetrics{
		currency: "USD", // Default currency
	}
}

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

// errorMetrics methods

func newErrorMetrics() *errorMetrics {
	return &errorMetrics{
		errorsByType:   make(map[string]*atomic.Int64),
		errorsByStatus: make(map[string]*atomic.Int64),
	}
}

func (em *errorMetrics) RecordError(event types.MetricEvent) {
	em.mu.Lock()
	defer em.mu.Unlock()

	em.totalErrors.Add(1)
	em.lastError = event.ErrorMessage
	em.lastErrorType = event.ErrorType
	em.lastErrorTime = event.Timestamp
	em.lastUpdated = time.Now()

	// Track consecutive errors
	if event.Type.IsError() {
		em.consecutiveErrors.Add(1)
	} else {
		em.consecutiveErrors.Store(0)
	}

	// Track by type
	if event.ErrorType != "" {
		if em.errorsByType[event.ErrorType] == nil {
			em.errorsByType[event.ErrorType] = &atomic.Int64{}
		}
		em.errorsByType[event.ErrorType].Add(1)
	}

	// Track by status
	if event.StatusCode > 0 {
		statusStr := fmt.Sprintf("%d", event.StatusCode)
		if em.errorsByStatus[statusStr] == nil {
			em.errorsByStatus[statusStr] = &atomic.Int64{}
		}
		em.errorsByStatus[statusStr].Add(1)
	}

	// Categorize errors
	switch event.Type {
	case types.MetricEventRateLimit:
		em.rateLimitErrors.Add(1)
	case types.MetricEventTimeout:
		em.timeoutErrors.Add(1)
	}

	// Categorize by error type
	switch event.ErrorType {
	case "authentication":
		em.authenticationErrors.Add(1)
	case "invalid_request":
		em.invalidRequestErrors.Add(1)
	case "network":
		em.networkErrors.Add(1)
	}

	// Categorize by status code
	if event.StatusCode >= 500 {
		em.serverErrors.Add(1)
	}
}

func (em *errorMetrics) GetSnapshot(totalRequests int64) types.ErrorMetrics {
	em.mu.Lock()
	defer em.mu.Unlock()

	totalErrors := em.totalErrors.Load()

	errorsByType := make(map[string]int64)
	for errType, counter := range em.errorsByType {
		errorsByType[errType] = counter.Load()
	}

	errorsByStatus := make(map[string]int64)
	for status, counter := range em.errorsByStatus {
		errorsByStatus[status] = counter.Load()
	}

	return types.ErrorMetrics{
		TotalErrors:             totalErrors,
		ErrorRate:               calculateRate(totalErrors, totalRequests),
		ErrorsByType:            errorsByType,
		ErrorsByStatus:          errorsByStatus,
		ErrorsByProvider:        make(map[string]int64),
		ErrorsByModel:           make(map[string]int64),
		RateLimitErrors:         em.rateLimitErrors.Load(),
		TimeoutErrors:           em.timeoutErrors.Load(),
		AuthenticationErrors:    em.authenticationErrors.Load(),
		InvalidRequestErrors:    em.invalidRequestErrors.Load(),
		ServerErrors:            em.serverErrors.Load(),
		NetworkErrors:           em.networkErrors.Load(),
		UnknownErrors:           em.unknownErrors.Load(),
		RateLimitErrorRate:      calculateRate(em.rateLimitErrors.Load(), totalErrors),
		TimeoutErrorRate:        calculateRate(em.timeoutErrors.Load(), totalErrors),
		AuthenticationErrorRate: calculateRate(em.authenticationErrors.Load(), totalErrors),
		ServerErrorRate:         calculateRate(em.serverErrors.Load(), totalErrors),
		ConsecutiveErrors:       em.consecutiveErrors.Load(),
		LastError:               em.lastError,
		LastErrorType:           em.lastErrorType,
		LastErrorTime:           em.lastErrorTime,
		LastUpdated:             em.lastUpdated,
	}
}

// streamMetrics methods

func newStreamMetrics() *streamMetrics {
	return &streamMetrics{
		ttftHistogram: NewHistogram(1000),
	}
}

func (sm *streamMetrics) RecordStreamStart(event types.MetricEvent) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.totalStreamRequests.Add(1)

	if event.TimeToFirstToken > 0 {
		sm.ttftHistogram.Add(event.TimeToFirstToken)
	}

	sm.lastUpdated = time.Now()
}

func (sm *streamMetrics) RecordStreamEnd(event types.MetricEvent) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.successfulStreamRequests.Add(1)

	if event.TokensUsed > 0 {
		sm.totalStreamedTokens.Add(event.TokensUsed)
	}

	if event.TokensPerSecond > 0 {
		if sm.minTokensPerSecond == 0 || event.TokensPerSecond < sm.minTokensPerSecond {
			sm.minTokensPerSecond = event.TokensPerSecond
		}
		if event.TokensPerSecond > sm.maxTokensPerSecond {
			sm.maxTokensPerSecond = event.TokensPerSecond
		}
		sm.totalTokensPerSecond += event.TokensPerSecond
		sm.tpsCount++
	}

	if event.Latency > 0 {
		if sm.minStreamDuration == 0 || event.Latency < sm.minStreamDuration {
			sm.minStreamDuration = event.Latency
		}
		if event.Latency > sm.maxStreamDuration {
			sm.maxStreamDuration = event.Latency
		}
		sm.totalStreamDuration += event.Latency
	}

	sm.lastUpdated = time.Now()
}

func (sm *streamMetrics) RecordStreamAbort() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.failedStreamRequests.Add(1)
	sm.lastUpdated = time.Now()
}

func (sm *streamMetrics) GetSnapshot() *types.StreamMetrics {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	totalStream := sm.totalStreamRequests.Load()
	successStream := sm.successfulStreamRequests.Load()
	failedStream := sm.failedStreamRequests.Load()

	if totalStream == 0 {
		return nil // No streaming data
	}

	avgTPS := float64(0)
	if sm.tpsCount > 0 {
		avgTPS = sm.totalTokensPerSecond / float64(sm.tpsCount)
	}

	avgDuration := time.Duration(0)
	if successStream > 0 {
		avgDuration = sm.totalStreamDuration / time.Duration(successStream)
	}

	totalTokens := sm.totalStreamedTokens.Load()
	totalChunks := sm.totalChunks.Load()

	avgTokensPerStream := float64(0)
	if successStream > 0 {
		avgTokensPerStream = float64(totalTokens) / float64(successStream)
	}

	avgChunksPerStream := float64(0)
	if successStream > 0 {
		avgChunksPerStream = float64(totalChunks) / float64(successStream)
	}

	avgChunkSize := float64(0)
	if totalChunks > 0 {
		avgChunkSize = float64(totalTokens) / float64(totalChunks)
	}

	ttftMetrics := sm.ttftHistogram.GetLatencyMetrics()

	return &types.StreamMetrics{
		TotalStreamRequests:      totalStream,
		SuccessfulStreamRequests: successStream,
		FailedStreamRequests:     failedStream,
		StreamSuccessRate:        calculateRate(successStream, totalStream),
		TimeToFirstToken: types.TimeToFirstTokenMetrics{
			TotalMeasurements: ttftMetrics.TotalRequests,
			AverageTTFT:       ttftMetrics.AverageLatency,
			MinTTFT:           ttftMetrics.MinLatency,
			MaxTTFT:           ttftMetrics.MaxLatency,
			P50TTFT:           ttftMetrics.P50Latency,
			P75TTFT:           ttftMetrics.P75Latency,
			P90TTFT:           ttftMetrics.P90Latency,
			P95TTFT:           ttftMetrics.P95Latency,
			P99TTFT:           ttftMetrics.P99Latency,
			LastUpdated:       ttftMetrics.LastUpdated,
		},
		AverageTokensPerSecond: avgTPS,
		MinTokensPerSecond:     sm.minTokensPerSecond,
		MaxTokensPerSecond:     sm.maxTokensPerSecond,
		MedianTokensPerSecond:  avgTPS, // Simplified: use average as median
		AverageStreamDuration:  avgDuration,
		MinStreamDuration:      sm.minStreamDuration,
		MaxStreamDuration:      sm.maxStreamDuration,
		TotalStreamedTokens:    totalTokens,
		AverageTokensPerStream: avgTokensPerStream,
		TotalChunks:            totalChunks,
		AverageChunksPerStream: avgChunksPerStream,
		AverageChunkSize:       avgChunkSize,
		LastUpdated:            sm.lastUpdated,
	}
}
