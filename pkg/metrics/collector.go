package metrics

import (
	"context"
	"fmt"
	"sort"
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

	tokenMetrics *tokenMetrics
	errorMetrics *errorMetrics
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
	totalCost       float64
	inputCost       float64
	outputCost      float64
	currency        string

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

	minTokensPerSecond float64
	maxTokensPerSecond float64
	totalTokensPerSecond float64
	tpsCount int64

	minStreamDuration time.Duration
	maxStreamDuration time.Duration
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

// GetSnapshot returns a complete snapshot of all metrics
func (c *DefaultMetricsCollector) GetSnapshot() types.MetricsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalReq := c.totalRequests.Load()
	successReq := c.successfulRequests.Load()
	failedReq := c.failedRequests.Load()

	snapshot := types.MetricsSnapshot{
		TotalRequests:      totalReq,
		SuccessfulRequests: successReq,
		FailedRequests:     failedReq,
		SuccessRate:        calculateRate(successReq, totalReq),
		Latency:            c.latencyHistogram.GetLatencyMetrics(),
		Tokens:             c.tokenMetrics.GetSnapshot(),
		Errors:             c.errorMetrics.GetSnapshot(totalReq),
		Streaming:          c.streamMetrics.GetSnapshot(),
		ProviderBreakdown:  make(map[string]*types.ProviderMetricsSnapshot),
		ModelBreakdown:     make(map[string]*types.ModelMetricsSnapshot),
		LastUpdated:        c.lastUpdated,
		FirstRequestTime:   c.firstRequestTime,
	}

	if !c.firstRequestTime.IsZero() {
		snapshot.Uptime = int64(time.Since(c.firstRequestTime).Seconds())
	}

	// Add provider breakdowns
	for name, pm := range c.providerMetrics {
		snapshot.ProviderBreakdown[name] = pm.GetSnapshot()
	}

	// Add model breakdowns
	for modelID, mm := range c.modelMetrics {
		snapshot.ModelBreakdown[modelID] = mm.GetSnapshot()
	}

	return snapshot
}

// GetProviderMetrics returns metrics for a specific provider
func (c *DefaultMetricsCollector) GetProviderMetrics(providerName string) *types.ProviderMetricsSnapshot {
	c.mu.RLock()
	pm, exists := c.providerMetrics[providerName]
	c.mu.RUnlock()

	if !exists {
		return nil
	}

	return pm.GetSnapshot()
}

// GetModelMetrics returns metrics for a specific model
func (c *DefaultMetricsCollector) GetModelMetrics(modelID string) *types.ModelMetricsSnapshot {
	c.mu.RLock()
	mm, exists := c.modelMetrics[modelID]
	c.mu.RUnlock()

	if !exists {
		return nil
	}

	return mm.GetSnapshot()
}

// GetProviderNames returns a sorted list of all provider names
func (c *DefaultMetricsCollector) GetProviderNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.providerMetrics))
	for name := range c.providerMetrics {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetModelIDs returns a sorted list of all model IDs
func (c *DefaultMetricsCollector) GetModelIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]string, 0, len(c.modelMetrics))
	for id := range c.modelMetrics {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Subscribe creates a new subscription with the given buffer size
func (c *DefaultMetricsCollector) Subscribe(bufferSize int) types.MetricsSubscription {
	return c.SubscribeFiltered(bufferSize, types.MetricFilter{})
}

// SubscribeFiltered creates a filtered subscription
func (c *DefaultMetricsCollector) SubscribeFiltered(bufferSize int, filter types.MetricFilter) types.MetricsSubscription {
	if c.closed.Load() {
		// Return a closed subscription
		sub := &subscription{
			id:     fmt.Sprintf("sub-closed-%d", c.nextSubID.Add(1)),
			events: make(chan types.MetricEvent),
		}
		close(sub.events)
		return sub
	}

	id := fmt.Sprintf("sub-%d", c.nextSubID.Add(1))
	sub := &subscription{
		id:         id,
		events:     make(chan types.MetricEvent, bufferSize),
		filter:     filter,
		collector:  c,
	}

	c.mu.Lock()
	c.subscriptions[id] = sub
	c.mu.Unlock()

	return sub
}

// RegisterHook registers a hook and returns its ID
func (c *DefaultMetricsCollector) RegisterHook(hook types.MetricsHook) types.HookID {
	id := types.HookID(fmt.Sprintf("hook-%d", c.nextHookID.Add(1)))

	entry := &hookEntry{
		hook:   hook,
		id:     id,
		filter: hook.Filter(),
	}

	c.mu.Lock()
	c.hooks[id] = entry
	c.mu.Unlock()

	return id
}

// UnregisterHook removes a hook
func (c *DefaultMetricsCollector) UnregisterHook(id types.HookID) {
	c.mu.Lock()
	delete(c.hooks, id)
	c.mu.Unlock()
}

// RecordEvent records a single metrics event
func (c *DefaultMetricsCollector) RecordEvent(ctx context.Context, event types.MetricEvent) error {
	if c.closed.Load() {
		return fmt.Errorf("collector is closed")
	}

	// Check context
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Update aggregate metrics
	c.updateAggregateMetrics(event)

	// Update provider metrics
	c.updateProviderMetrics(event)

	// Update model metrics
	if event.ModelID != "" {
		c.updateModelMetrics(event)
	}

	// Publish to subscriptions
	c.publishToSubscriptions(event)

	// Call hooks
	c.callHooks(ctx, event)

	return nil
}

// RecordEvents records multiple events in a batch
func (c *DefaultMetricsCollector) RecordEvents(ctx context.Context, events []types.MetricEvent) error {
	for _, event := range events {
		if err := c.RecordEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// Reset clears all metrics data
func (c *DefaultMetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Reset atomic counters
	c.totalRequests.Store(0)
	c.successfulRequests.Store(0)
	c.failedRequests.Store(0)

	// Clear maps
	c.providerMetrics = make(map[string]*providerMetrics)
	c.modelMetrics = make(map[string]*modelMetrics)

	// Reset histograms
	c.latencyHistogram = NewHistogram(1000)

	// Reset metrics
	c.streamMetrics = newStreamMetrics()
	c.tokenMetrics = newTokenMetrics()
	c.errorMetrics = newErrorMetrics()

	// Reset timestamps
	c.firstRequestTime = time.Time{}
	c.lastUpdated = time.Time{}
}

// Close shuts down the collector
func (c *DefaultMetricsCollector) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Close all subscriptions
	for _, sub := range c.subscriptions {
		sub.close()
	}
	c.subscriptions = make(map[string]*subscription)

	// Clear all hooks
	c.hooks = make(map[types.HookID]*hookEntry)

	return nil
}

// calculateCost calculates the cost for an event if token usage is present
func (c *DefaultMetricsCollector) calculateCost(event types.MetricEvent) *Cost {
	if event.InputTokens == 0 && event.OutputTokens == 0 {
		return nil
	}

	cost := c.costCalculator.CalculateCost(
		event.ProviderName,
		event.ModelID,
		event.InputTokens,
		event.OutputTokens,
	)
	return &cost
}

// updateAggregateMetrics updates the top-level aggregate metrics
func (c *DefaultMetricsCollector) updateAggregateMetrics(event types.MetricEvent) {
	now := time.Now()

	c.mu.Lock()
	if c.firstRequestTime.IsZero() {
		c.firstRequestTime = event.Timestamp
	}
	c.lastUpdated = now
	c.mu.Unlock()

	switch event.Type {
	case types.MetricEventRequest:
		c.totalRequests.Add(1)
	case types.MetricEventSuccess:
		c.successfulRequests.Add(1)
		if event.Latency > 0 {
			c.latencyHistogram.Add(event.Latency)
		}
	case types.MetricEventError, types.MetricEventTimeout, types.MetricEventRateLimit:
		c.failedRequests.Add(1)
		c.errorMetrics.RecordError(event)
	case types.MetricEventStreamStart:
		c.streamMetrics.RecordStreamStart(event)
	case types.MetricEventStreamEnd:
		c.streamMetrics.RecordStreamEnd(event)
	case types.MetricEventStreamAbort:
		c.streamMetrics.RecordStreamAbort()
	}

	// Update token metrics
	if event.TokensUsed > 0 || event.InputTokens > 0 || event.OutputTokens > 0 {
		cost := c.calculateCost(event)
		c.tokenMetrics.RecordTokens(event, cost)
	}
}

// updateProviderMetrics updates per-provider metrics
func (c *DefaultMetricsCollector) updateProviderMetrics(event types.MetricEvent) {
	c.mu.Lock()
	pm, exists := c.providerMetrics[event.ProviderName]
	if !exists {
		pm = newProviderMetrics(event.ProviderName, event.ProviderType, c.costCalculator)
		c.providerMetrics[event.ProviderName] = pm
	}
	c.mu.Unlock()

	pm.RecordEvent(event)
}

// updateModelMetrics updates per-model metrics
func (c *DefaultMetricsCollector) updateModelMetrics(event types.MetricEvent) {
	c.mu.Lock()
	mm, exists := c.modelMetrics[event.ModelID]
	if !exists {
		mm = newModelMetrics(event.ModelID, event.ProviderName, event.ProviderType, c.costCalculator)
		c.modelMetrics[event.ModelID] = mm
	}
	c.mu.Unlock()

	mm.RecordEvent(event)
}

// publishToSubscriptions publishes an event to all subscriptions
func (c *DefaultMetricsCollector) publishToSubscriptions(event types.MetricEvent) {
	c.mu.RLock()
	subs := make([]*subscription, 0, len(c.subscriptions))
	for _, sub := range c.subscriptions {
		subs = append(subs, sub)
	}
	c.mu.RUnlock()

	for _, sub := range subs {
		sub.publish(event)
	}
}

// callHooks calls all registered hooks
func (c *DefaultMetricsCollector) callHooks(ctx context.Context, event types.MetricEvent) {
	c.mu.RLock()
	hooks := make([]*hookEntry, 0, len(c.hooks))
	for _, h := range c.hooks {
		hooks = append(hooks, h)
	}
	c.mu.RUnlock()

	for _, entry := range hooks {
		// Check filter
		if entry.filter != nil && !entry.filter.Matches(event) {
			continue
		}

		// Call hook with timeout protection
		hookCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		func() {
			defer cancel()
			defer func() {
				if r := recover(); r != nil {
					// Hook panicked, ignore
				}
			}()

			done := make(chan struct{})
			go func() {
				entry.hook.OnEvent(hookCtx, event)
				close(done)
			}()

			select {
			case <-done:
				// Hook completed
			case <-hookCtx.Done():
				// Hook timed out or context cancelled
			}
		}()
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
