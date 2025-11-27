package types

import "time"

// RouterHealthStatus represents the health status of a provider (router-specific for compatibility)
type RouterHealthStatus struct {
	IsHealthy    bool          `json:"IsHealthy"`
	LastChecked  time.Time     `json:"LastChecked"`
	ErrorMessage string        `json:"ErrorMessage,omitempty"`
	ResponseTime time.Duration `json:"ResponseTime"`
}

// RouterMetrics represents metrics for the router
type RouterMetrics struct {
	TotalRequests       int64                       `json:"total_requests"`
	SuccessfulRequests  int64                       `json:"successful_requests"`
	FailedRequests      int64                       `json:"failed_requests"`
	ProviderUsage       map[string]int64            `json:"provider_usage"`
	AverageResponseTime time.Duration               `json:"average_response_time"`
	ProviderMetrics     map[string]*ProviderMetrics `json:"provider_metrics"`
	LastReset           time.Time                   `json:"last_reset"`
}

// ModelMetrics represents metrics for a specific model
type ModelMetrics struct {
	ModelID         string        `json:"model_id"`
	Provider        ProviderType  `json:"provider"`
	RequestCount    int64         `json:"request_count"`
	SuccessCount    int64         `json:"success_count"`
	ErrorCount      int64         `json:"error_count"`
	TokensUsed      int64         `json:"tokens_used"`
	AverageLatency  time.Duration `json:"average_latency"`
	LastRequestTime time.Time     `json:"last_request_time"`
	P95ResponseTime time.Duration `json:"p95_response_time"`
	P99ResponseTime time.Duration `json:"p99_response_time"`
}

// ErrorMetrics represents error-related metrics
type ErrorMetrics struct {
	TotalErrors      int64            `json:"total_errors"`
	ErrorTypes       map[string]int64 `json:"error_types"`
	LastError        string           `json:"last_error"`
	LastErrorTime    time.Time        `json:"last_error_time"`
	ConsecutiveFails int64            `json:"consecutive_fails"`
}

// TokenMetrics represents token usage metrics
type TokenMetrics struct {
	InputTokens   int64     `json:"input_tokens"`
	OutputTokens  int64     `json:"output_tokens"`
	TotalTokens   int64     `json:"total_tokens"`
	EstimatedCost float64   `json:"estimated_cost"`
	Currency      string    `json:"currency"`
	LastUpdated   time.Time `json:"last_updated"`
}
