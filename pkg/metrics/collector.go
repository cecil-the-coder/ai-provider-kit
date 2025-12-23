// Package metrics provides a centralized metrics collection system for ai-provider-kit.
// It includes the DefaultMetricsCollector implementation, streaming metrics wrapper,
// cost calculation interface, and histogram for percentile calculations.
package metrics

import (
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

// calculateRate is a helper function to calculate success rate
func calculateRate(numerator, denominator int64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
