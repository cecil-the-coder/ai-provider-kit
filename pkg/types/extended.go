package types

import (
	"time"
)

// EnhancedRouterMetrics holds router performance metrics (for compatibility with enhanced router)
type EnhancedRouterMetrics struct {
	TotalRequests      int64 `json:"TotalRequests"`
	SuccessfulRequests int64 `json:"SuccessfulRequests"`
	FailedRequests     int64 `json:"FailedRequests"`
	FallbackAttempts   int64 `json:"FallbackAttempts"`
}

// OverallLatencyMetrics represents overall latency percentiles (for compatibility)
type OverallLatencyMetrics struct {
	MinLatency time.Duration `json:"MinLatency"`
	P50Latency time.Duration `json:"P50Latency"`
	P95Latency time.Duration `json:"P95Latency"`
	P99Latency time.Duration `json:"P99Latency"`
	MaxLatency time.Duration `json:"MaxLatency"`
}

// ProviderMetricsInfo represents detailed provider metrics for compatibility with router
type ProviderMetricsInfo struct {
	Name            string        `json:"name"`
	IsModel         bool          `json:"is_model"`
	RequestCount    int64         `json:"request_count"`
	SuccessCount    int64         `json:"success_count"`
	ErrorCount      int64         `json:"error_count"`
	AverageLatency  time.Duration `json:"average_latency"`
	LastRequestTime time.Time     `json:"last_request_time"`
	TokensUsed      int64         `json:"tokens_used"`
}

// MetricsTracker represents a metrics tracking interface (for compatibility)
type MetricsTracker interface {
	GetMetrics() ProviderMetricsInfo
	RecordRequest(success bool, latency time.Duration, usage *Usage)
}

// LatencyTracker represents a latency tracking interface (for compatibility)
type LatencyTracker interface {
	Add(latency time.Duration)
	GetPercentiles() (min, p50, p95, p99, max time.Duration)
}
