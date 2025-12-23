package common

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ProviderMetrics tracks performance and usage metrics for providers
type ProviderMetrics struct {
	mu sync.RWMutex

	// Request metrics
	totalRequests   int64
	successRequests int64
	failedRequests  int64

	// Response time metrics
	totalResponseTime int64 // Nanoseconds
	minResponseTime   int64 // Nanoseconds
	maxResponseTime   int64 // Nanoseconds

	// Error tracking
	errorsByType     map[string]int64
	errorsByProvider map[types.ProviderType]int64

	// Token usage
	totalTokensUsed int64

	// Initialization tracking
	initializations map[types.ProviderType]int64

	// Health check metrics
	healthChecks     map[types.ProviderType]int64
	healthCheckFails map[types.ProviderType]int64

	// Model usage
	modelUsage map[string]int64 // modelID -> count

	// Rate limiting
	rateLimitHits map[types.ProviderType]int64

	// Time tracking
	lastRequestTime  map[types.ProviderType]time.Time
	firstRequestTime time.Time
}

// MetricsSnapshot represents a snapshot of metrics at a point in time
type MetricsSnapshot struct {
	TotalRequests    int64                `json:"total_requests"`
	SuccessRequests  int64                `json:"success_requests"`
	FailedRequests   int64                `json:"failed_requests"`
	SuccessRate      float64              `json:"success_rate"`
	AvgResponseTime  time.Duration        `json:"avg_response_time"`
	MinResponseTime  time.Duration        `json:"min_response_time"`
	MaxResponseTime  time.Duration        `json:"max_response_time"`
	TotalTokensUsed  int64                `json:"total_tokens_used"`
	ErrorsByType     map[string]int64     `json:"errors_by_type"`
	ErrorsByProvider map[string]int64     `json:"errors_by_provider"`
	Initializations  map[string]int64     `json:"initializations"`
	HealthChecks     map[string]int64     `json:"health_checks"`
	HealthCheckFails map[string]int64     `json:"health_check_fails"`
	ModelUsage       map[string]int64     `json:"model_usage"`
	RateLimitHits    map[string]int64     `json:"rate_limit_hits"`
	LastRequestTime  map[string]time.Time `json:"last_request_time"`
	Uptime           time.Duration        `json:"uptime"`
}

// NewProviderMetrics creates a new provider metrics instance
func NewProviderMetrics() *ProviderMetrics {
	now := time.Now()
	return &ProviderMetrics{
		errorsByType:     make(map[string]int64),
		errorsByProvider: make(map[types.ProviderType]int64),
		initializations:  make(map[types.ProviderType]int64),
		healthChecks:     make(map[types.ProviderType]int64),
		healthCheckFails: make(map[types.ProviderType]int64),
		modelUsage:       make(map[string]int64),
		rateLimitHits:    make(map[types.ProviderType]int64),
		lastRequestTime:  make(map[types.ProviderType]time.Time),
		firstRequestTime: now,
		minResponseTime:  int64(^uint64(0) >> 1), // Max int64
	}
}

// RecordRequest records a request attempt
func (pm *ProviderMetrics) RecordRequest(providerType types.ProviderType) {
	atomic.AddInt64(&pm.totalRequests, 1)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.lastRequestTime[providerType] = time.Now()
}

// RecordSuccess records a successful request
func (pm *ProviderMetrics) RecordSuccess(providerType types.ProviderType, responseTime time.Duration, tokens int64, modelID string) {
	atomic.AddInt64(&pm.successRequests, 1)
	atomic.AddInt64(&pm.totalTokensUsed, tokens)

	responseTimeNanos := responseTime.Nanoseconds()
	atomic.AddInt64(&pm.totalResponseTime, responseTimeNanos)

	// Update min/max response times
	for {
		current := atomic.LoadInt64(&pm.minResponseTime)
		if responseTimeNanos >= current || atomic.CompareAndSwapInt64(&pm.minResponseTime, current, responseTimeNanos) {
			break
		}
	}

	for {
		current := atomic.LoadInt64(&pm.maxResponseTime)
		if responseTimeNanos <= current || atomic.CompareAndSwapInt64(&pm.maxResponseTime, current, responseTimeNanos) {
			break
		}
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if modelID != "" {
		pm.modelUsage[modelID]++
	}
}

// RecordError records a failed request
func (pm *ProviderMetrics) RecordError(providerType types.ProviderType, errorType string) {
	atomic.AddInt64(&pm.failedRequests, 1)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.errorsByType[errorType]++
	pm.errorsByProvider[providerType]++
}

// RecordInitialization records a provider initialization
func (pm *ProviderMetrics) RecordInitialization(providerType types.ProviderType) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.initializations[providerType]++
}

// RecordHealthCheck records a health check attempt
func (pm *ProviderMetrics) RecordHealthCheck(providerType types.ProviderType, success bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.healthChecks[providerType]++
	if !success {
		pm.healthCheckFails[providerType]++
	}
}

// RecordRateLimitHit records a rate limit occurrence
func (pm *ProviderMetrics) RecordRateLimitHit(providerType types.ProviderType) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.rateLimitHits[providerType]++
}

// GetSnapshot returns a snapshot of current metrics
func (pm *ProviderMetrics) GetSnapshot() MetricsSnapshot {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	totalReqs := atomic.LoadInt64(&pm.totalRequests)
	successReqs := atomic.LoadInt64(&pm.successRequests)
	failedReqs := atomic.LoadInt64(&pm.failedRequests)
	totalTime := atomic.LoadInt64(&pm.totalResponseTime)
	totalTokens := atomic.LoadInt64(&pm.totalTokensUsed)
	minTime := atomic.LoadInt64(&pm.minResponseTime)
	maxTime := atomic.LoadInt64(&pm.maxResponseTime)

	// Calculate success rate
	var successRate float64
	if totalReqs > 0 {
		successRate = float64(successReqs) / float64(totalReqs)
	}

	// Calculate average response time
	var avgResponseTime time.Duration
	if successReqs > 0 {
		avgResponseTime = time.Duration(totalTime / successReqs)
	}

	// Copy maps to avoid concurrent access issues
	errorsByType := make(map[string]int64)
	for k, v := range pm.errorsByType {
		errorsByType[k] = v
	}

	errorsByProvider := make(map[string]int64)
	for k, v := range pm.errorsByProvider {
		errorsByProvider[string(k)] = v
	}

	initializations := make(map[string]int64)
	for k, v := range pm.initializations {
		initializations[string(k)] = v
	}

	healthChecks := make(map[string]int64)
	for k, v := range pm.healthChecks {
		healthChecks[string(k)] = v
	}

	healthCheckFails := make(map[string]int64)
	for k, v := range pm.healthCheckFails {
		healthCheckFails[string(k)] = v
	}

	modelUsage := make(map[string]int64)
	for k, v := range pm.modelUsage {
		modelUsage[k] = v
	}

	rateLimitHits := make(map[string]int64)
	for k, v := range pm.rateLimitHits {
		rateLimitHits[string(k)] = v
	}

	lastRequestTime := make(map[string]time.Time)
	for k, v := range pm.lastRequestTime {
		lastRequestTime[string(k)] = v
	}

	uptime := time.Since(pm.firstRequestTime)

	// Handle min time case (if no requests yet)
	minResponseTime := time.Duration(minTime)
	if minTime == int64(^uint64(0)>>1) {
		minResponseTime = 0
	}

	return MetricsSnapshot{
		TotalRequests:    totalReqs,
		SuccessRequests:  successReqs,
		FailedRequests:   failedReqs,
		SuccessRate:      successRate,
		AvgResponseTime:  avgResponseTime,
		MinResponseTime:  minResponseTime,
		MaxResponseTime:  time.Duration(maxTime),
		TotalTokensUsed:  totalTokens,
		ErrorsByType:     errorsByType,
		ErrorsByProvider: errorsByProvider,
		Initializations:  initializations,
		HealthChecks:     healthChecks,
		HealthCheckFails: healthCheckFails,
		ModelUsage:       modelUsage,
		RateLimitHits:    rateLimitHits,
		LastRequestTime:  lastRequestTime,
		Uptime:           uptime,
	}
}

// GetMetricsForProvider returns metrics specific to a provider
func (pm *ProviderMetrics) GetMetricsForProvider(providerType types.ProviderType) ProviderMetricsSnapshot {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return ProviderMetricsSnapshot{
		Provider:         string(providerType),
		Errors:           pm.errorsByProvider[providerType],
		HealthChecks:     pm.healthChecks[providerType],
		HealthCheckFails: pm.healthCheckFails[providerType],
		RateLimitHits:    pm.rateLimitHits[providerType],
		Initializations:  pm.initializations[providerType],
		LastRequestTime:  pm.lastRequestTime[providerType],
	}
}

// ProviderMetricsSnapshot represents metrics for a specific provider
type ProviderMetricsSnapshot struct {
	Provider         string    `json:"provider"`
	Errors           int64     `json:"errors"`
	HealthChecks     int64     `json:"health_checks"`
	HealthCheckFails int64     `json:"health_check_fails"`
	RateLimitHits    int64     `json:"rate_limit_hits"`
	Initializations  int64     `json:"initializations"`
	LastRequestTime  time.Time `json:"last_request_time"`
}

// Reset resets all metrics
func (pm *ProviderMetrics) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	atomic.StoreInt64(&pm.totalRequests, 0)
	atomic.StoreInt64(&pm.successRequests, 0)
	atomic.StoreInt64(&pm.failedRequests, 0)
	atomic.StoreInt64(&pm.totalResponseTime, 0)
	atomic.StoreInt64(&pm.totalTokensUsed, 0)
	atomic.StoreInt64(&pm.minResponseTime, int64(^uint64(0)>>1))
	atomic.StoreInt64(&pm.maxResponseTime, 0)

	pm.errorsByType = make(map[string]int64)
	pm.errorsByProvider = make(map[types.ProviderType]int64)
	pm.initializations = make(map[types.ProviderType]int64)
	pm.healthChecks = make(map[types.ProviderType]int64)
	pm.healthCheckFails = make(map[types.ProviderType]int64)
	pm.modelUsage = make(map[string]int64)
	pm.rateLimitHits = make(map[types.ProviderType]int64)
	pm.lastRequestTime = make(map[types.ProviderType]time.Time)
	pm.firstRequestTime = time.Now()
}

// GetTopModels returns the most used models
func (pm *ProviderMetrics) GetTopModels(limit int) []ModelUsage {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	usage := make([]ModelUsage, 0, len(pm.modelUsage))
	for modelID, count := range pm.modelUsage {
		usage = append(usage, ModelUsage{
			ModelID: modelID,
			Count:   count,
		})
	}

	// Sort by count (descending) - simple implementation
	for i := 0; i < len(usage)-1; i++ {
		for j := i + 1; j < len(usage); j++ {
			if usage[j].Count > usage[i].Count {
				usage[i], usage[j] = usage[j], usage[i]
			}
		}
	}

	if limit > 0 && len(usage) > limit {
		usage = usage[:limit]
	}

	return usage
}

// ModelUsage represents model usage statistics
type ModelUsage struct {
	ModelID string `json:"model_id"`
	Count   int64  `json:"count"`
}

// GetErrorSummary returns a summary of errors by type
func (pm *ProviderMetrics) GetErrorSummary() ErrorSummary {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	totalErrors := int64(0)
	for _, count := range pm.errorsByType {
		totalErrors += count
	}

	errorTypes := make([]ErrorType, 0, len(pm.errorsByType))
	for errorType, count := range pm.errorsByType {
		percentage := float64(0)
		if totalErrors > 0 {
			percentage = float64(count) / float64(totalErrors) * 100
		}

		errorTypes = append(errorTypes, ErrorType{
			Type:       errorType,
			Count:      count,
			Percentage: percentage,
		})
	}

	return ErrorSummary{
		TotalErrors: totalErrors,
		ErrorTypes:  errorTypes,
	}
}

// ErrorSummary represents a summary of errors
type ErrorSummary struct {
	TotalErrors int64       `json:"total_errors"`
	ErrorTypes  []ErrorType `json:"error_types"`
}

// ErrorType represents an error type with count and percentage
type ErrorType struct {
	Type       string  `json:"type"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}
