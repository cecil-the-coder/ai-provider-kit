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
