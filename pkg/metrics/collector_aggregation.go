package metrics

import (
	"sort"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

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
