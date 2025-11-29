package types

import "time"

// MetricsSnapshot represents a complete point-in-time snapshot of all aggregate metrics
// This is the top-level snapshot returned by GetSnapshot()
type MetricsSnapshot struct {
	// Request metrics
	TotalRequests      int64   `json:"total_requests"`
	SuccessfulRequests int64   `json:"successful_requests"`
	FailedRequests     int64   `json:"failed_requests"`
	SuccessRate        float64 `json:"success_rate"` // Calculated: successful/total

	// Latency metrics
	Latency LatencyMetrics `json:"latency"`

	// Token usage metrics
	Tokens TokenMetrics `json:"tokens"`

	// Error metrics
	Errors ErrorMetrics `json:"errors"`

	// Streaming metrics (aggregated across all streaming requests)
	Streaming *StreamMetrics `json:"streaming,omitempty"`

	// Provider breakdown
	ProviderBreakdown map[string]*ProviderMetricsSnapshot `json:"provider_breakdown"`

	// Model breakdown
	ModelBreakdown map[string]*ModelMetricsSnapshot `json:"model_breakdown"`

	// Timestamps
	LastUpdated      time.Time `json:"last_updated"`
	FirstRequestTime time.Time `json:"first_request_time"`
	Uptime           int64     `json:"uptime_seconds"` // Seconds since first request
}

// ProviderMetricsSnapshot represents per-provider breakdown metrics
// Returned by GetProviderMetrics(providerType)
type ProviderMetricsSnapshot struct {
	// Provider identification
	Provider     string       `json:"provider"`
	ProviderType ProviderType `json:"provider_type"`

	// Request metrics
	TotalRequests      int64   `json:"total_requests"`
	SuccessfulRequests int64   `json:"successful_requests"`
	FailedRequests     int64   `json:"failed_requests"`
	SuccessRate        float64 `json:"success_rate"` // Calculated: successful/total

	// Latency metrics for this provider
	Latency LatencyMetrics `json:"latency"`

	// Token usage for this provider
	Tokens TokenMetrics `json:"tokens"`

	// Error metrics for this provider
	Errors ErrorMetrics `json:"errors"`

	// Streaming metrics for this provider
	Streaming *StreamMetrics `json:"streaming,omitempty"`

	// Provider-specific operations
	Initializations  int64   `json:"initializations"`
	HealthChecks     int64   `json:"health_checks"`
	HealthCheckFails int64   `json:"health_check_fails"`
	HealthCheckRate  float64 `json:"health_check_success_rate"` // Calculated: (checks - fails) / checks
	RateLimitHits    int64   `json:"rate_limit_hits"`

	// Model usage breakdown for this provider
	ModelUsage map[string]int64 `json:"model_usage"`

	// Timestamps
	LastRequestTime time.Time `json:"last_request_time"`
	LastUpdated     time.Time `json:"last_updated"`
}

// ModelMetricsSnapshot represents per-model breakdown metrics
// Returned by GetModelMetrics(modelID)
type ModelMetricsSnapshot struct {
	// Model identification
	ModelID      string       `json:"model_id"`
	Provider     string       `json:"provider"`      // Provider name (instance identifier)
	ProviderType ProviderType `json:"provider_type"` // Provider type

	// Request metrics
	TotalRequests      int64   `json:"total_requests"`
	SuccessfulRequests int64   `json:"successful_requests"`
	FailedRequests     int64   `json:"failed_requests"`
	SuccessRate        float64 `json:"success_rate"` // Calculated: successful/total

	// Latency metrics for this model
	Latency LatencyMetrics `json:"latency"`

	// Token usage for this model
	Tokens TokenMetrics `json:"tokens"`

	// Error metrics for this model
	Errors ErrorMetrics `json:"errors"`

	// Streaming metrics for this model
	Streaming *StreamMetrics `json:"streaming,omitempty"`

	// Model-specific metrics
	AverageTokensPerRequest float64 `json:"average_tokens_per_request"` // Calculated: total_tokens / requests
	EstimatedCostPerRequest float64 `json:"estimated_cost_per_request"` // Calculated: total_cost / requests

	// Timestamps
	LastRequestTime time.Time `json:"last_request_time"`
	LastUpdated     time.Time `json:"last_updated"`
}

// LatencyMetrics represents latency statistics including percentiles
type LatencyMetrics struct {
	// Summary statistics
	TotalRequests  int64         `json:"total_requests"`  // Number of requests measured
	TotalLatency   time.Duration `json:"total_latency"`   // Sum of all latencies
	AverageLatency time.Duration `json:"average_latency"` // Calculated: total_latency / total_requests

	// Min/Max
	MinLatency time.Duration `json:"min_latency"`
	MaxLatency time.Duration `json:"max_latency"`

	// Percentiles (for detailed latency distribution)
	P50Latency time.Duration `json:"p50_latency"` // Median
	P75Latency time.Duration `json:"p75_latency"`
	P90Latency time.Duration `json:"p90_latency"`
	P95Latency time.Duration `json:"p95_latency"`
	P99Latency time.Duration `json:"p99_latency"`

	// Last updated
	LastUpdated time.Time `json:"last_updated"`
}

// StreamMetrics represents streaming-specific metrics
type StreamMetrics struct {
	// Streaming request counts
	TotalStreamRequests      int64   `json:"total_stream_requests"`
	SuccessfulStreamRequests int64   `json:"successful_stream_requests"`
	FailedStreamRequests     int64   `json:"failed_stream_requests"`
	StreamSuccessRate        float64 `json:"stream_success_rate"` // Calculated: successful/total

	// Time to First Token (TTFT) metrics
	TimeToFirstToken TimeToFirstTokenMetrics `json:"time_to_first_token"`

	// Throughput metrics
	AverageTokensPerSecond float64 `json:"average_tokens_per_second"` // Calculated: total_tokens / total_duration
	MinTokensPerSecond     float64 `json:"min_tokens_per_second"`
	MaxTokensPerSecond     float64 `json:"max_tokens_per_second"`
	MedianTokensPerSecond  float64 `json:"median_tokens_per_second"`

	// Stream duration statistics
	AverageStreamDuration time.Duration `json:"average_stream_duration"`
	MinStreamDuration     time.Duration `json:"min_stream_duration"`
	MaxStreamDuration     time.Duration `json:"max_stream_duration"`

	// Token statistics for streaming
	TotalStreamedTokens    int64   `json:"total_streamed_tokens"`
	AverageTokensPerStream float64 `json:"average_tokens_per_stream"` // Calculated: total_streamed_tokens / successful_streams

	// Chunk statistics
	TotalChunks            int64   `json:"total_chunks"`
	AverageChunksPerStream float64 `json:"average_chunks_per_stream"` // Calculated: total_chunks / successful_streams
	AverageChunkSize       float64 `json:"average_chunk_size"`        // Calculated: total_streamed_tokens / total_chunks

	// Last updated
	LastUpdated time.Time `json:"last_updated"`
}

// TimeToFirstTokenMetrics represents TTFT (Time to First Token) statistics
type TimeToFirstTokenMetrics struct {
	// Summary statistics
	TotalMeasurements int64         `json:"total_measurements"`
	AverageTTFT       time.Duration `json:"average_ttft"`

	// Min/Max
	MinTTFT time.Duration `json:"min_ttft"`
	MaxTTFT time.Duration `json:"max_ttft"`

	// Percentiles for TTFT distribution
	P50TTFT time.Duration `json:"p50_ttft"` // Median
	P75TTFT time.Duration `json:"p75_ttft"`
	P90TTFT time.Duration `json:"p90_ttft"`
	P95TTFT time.Duration `json:"p95_ttft"`
	P99TTFT time.Duration `json:"p99_ttft"`

	// Last updated
	LastUpdated time.Time `json:"last_updated"`
}

// TokenMetrics represents comprehensive token usage breakdown
type TokenMetrics struct {
	// Total token counts
	TotalTokens  int64 `json:"total_tokens"`
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`

	// Cached token usage (for providers that support prompt caching)
	CachedTokens        int64 `json:"cached_tokens,omitempty"`
	CacheReadTokens     int64 `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens int64 `json:"cache_creation_tokens,omitempty"`

	// Reasoning tokens (for models with extended thinking)
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`

	// Average token usage per request
	AverageInputTokens  float64 `json:"average_input_tokens"`  // Calculated: input_tokens / requests
	AverageOutputTokens float64 `json:"average_output_tokens"` // Calculated: output_tokens / requests
	AverageTotalTokens  float64 `json:"average_total_tokens"`  // Calculated: total_tokens / requests

	// Token efficiency metrics
	InputOutputRatio float64 `json:"input_output_ratio"`       // Calculated: input_tokens / output_tokens
	CacheHitRate     float64 `json:"cache_hit_rate,omitempty"` // Calculated: cache_read_tokens / input_tokens

	// Cost estimation
	EstimatedCost       float64 `json:"estimated_cost"`
	Currency            string  `json:"currency"`
	EstimatedInputCost  float64 `json:"estimated_input_cost"`
	EstimatedOutputCost float64 `json:"estimated_output_cost"`
	EstimatedCachedCost float64 `json:"estimated_cached_cost,omitempty"`

	// Cost per token (for reference)
	InputCostPerToken  float64 `json:"input_cost_per_token,omitempty"`
	OutputCostPerToken float64 `json:"output_cost_per_token,omitempty"`
	CachedCostPerToken float64 `json:"cached_cost_per_token,omitempty"`

	// Last updated
	LastUpdated time.Time `json:"last_updated"`
}

// ErrorMetrics represents comprehensive error categorization and tracking
type ErrorMetrics struct {
	// Total error counts
	TotalErrors int64   `json:"total_errors"`
	ErrorRate   float64 `json:"error_rate"` // Calculated: errors / total_requests

	// Error categorization by type
	ErrorsByType map[string]int64 `json:"errors_by_type"` // e.g., "rate_limit", "timeout", "authentication", "invalid_request"

	// Error categorization by HTTP status
	ErrorsByStatus map[string]int64 `json:"errors_by_status"` // e.g., "400", "401", "429", "500", "503"

	// Error categorization by provider
	ErrorsByProvider map[string]int64 `json:"errors_by_provider"`

	// Error categorization by model
	ErrorsByModel map[string]int64 `json:"errors_by_model,omitempty"`

	// Specific error categories
	RateLimitErrors      int64 `json:"rate_limit_errors"`
	TimeoutErrors        int64 `json:"timeout_errors"`
	AuthenticationErrors int64 `json:"authentication_errors"`
	InvalidRequestErrors int64 `json:"invalid_request_errors"`
	ServerErrors         int64 `json:"server_errors"` // 5xx errors
	NetworkErrors        int64 `json:"network_errors"`
	UnknownErrors        int64 `json:"unknown_errors"`

	// Error percentages (for quick analysis)
	RateLimitErrorRate      float64 `json:"rate_limit_error_rate"`     // Calculated: rate_limit_errors / total_errors
	TimeoutErrorRate        float64 `json:"timeout_error_rate"`        // Calculated: timeout_errors / total_errors
	AuthenticationErrorRate float64 `json:"authentication_error_rate"` // Calculated: auth_errors / total_errors
	ServerErrorRate         float64 `json:"server_error_rate"`         // Calculated: server_errors / total_errors

	// Recent error tracking
	ConsecutiveErrors int64     `json:"consecutive_errors"` // Current consecutive error count
	LastError         string    `json:"last_error"`
	LastErrorType     string    `json:"last_error_type"`
	LastErrorTime     time.Time `json:"last_error_time"`

	// Error recovery metrics
	TotalRetries      int64   `json:"total_retries"`
	SuccessfulRetries int64   `json:"successful_retries"`
	FailedRetries     int64   `json:"failed_retries"`
	RetrySuccessRate  float64 `json:"retry_success_rate"` // Calculated: successful_retries / total_retries

	// Last updated
	LastUpdated time.Time `json:"last_updated"`
}
