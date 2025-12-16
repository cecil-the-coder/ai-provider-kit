package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

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
					// Hook panicked, ignore - we don't want a misbehaving hook to crash the collector
					_ = r // Explicitly ignore the panic value
				}
			}()

			done := make(chan struct{})
			go func() {
				entry.hook.OnEvent(hookCtx, event)
				close(done)
			}()

			select {
			case <-done:
				// Hook completed successfully - no action needed
			case <-hookCtx.Done():
				// Hook timed out or context cancelled - no action needed, hook goroutine will be abandoned
			}
		}()
	}
}
