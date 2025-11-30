package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouterHealthStatus tests the RouterHealthStatus struct
func TestRouterHealthStatus(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var status RouterHealthStatus
		assert.False(t, status.IsHealthy)
		assert.True(t, status.LastChecked.IsZero())
		assert.Empty(t, status.ErrorMessage)
		assert.Equal(t, time.Duration(0), status.ResponseTime)
	})

	t.Run("HealthyStatus", func(t *testing.T) {
		now := time.Now()
		status := RouterHealthStatus{
			IsHealthy:    true,
			LastChecked:  now,
			ErrorMessage: "",
			ResponseTime: 50 * time.Millisecond,
		}

		assert.True(t, status.IsHealthy)
		assert.Equal(t, now, status.LastChecked)
		assert.Empty(t, status.ErrorMessage)
		assert.Equal(t, 50*time.Millisecond, status.ResponseTime)
	})

	t.Run("UnhealthyStatus", func(t *testing.T) {
		now := time.Now()
		status := RouterHealthStatus{
			IsHealthy:    false,
			LastChecked:  now,
			ErrorMessage: "Connection timeout",
			ResponseTime: 5 * time.Second,
		}

		assert.False(t, status.IsHealthy)
		assert.Equal(t, now, status.LastChecked)
		assert.Equal(t, "Connection timeout", status.ErrorMessage)
		assert.Equal(t, 5*time.Second, status.ResponseTime)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		status := RouterHealthStatus{
			IsHealthy:    true,
			LastChecked:  time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			ErrorMessage: "",
			ResponseTime: 75 * time.Millisecond,
		}

		data, err := json.Marshal(status)
		require.NoError(t, err)

		var result RouterHealthStatus
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, status.IsHealthy, result.IsHealthy)
		assert.Equal(t, status.LastChecked.Unix(), result.LastChecked.Unix())
		assert.Equal(t, status.ErrorMessage, result.ErrorMessage)
		assert.Equal(t, status.ResponseTime, result.ResponseTime)
	})

	t.Run("UpdateHealth", func(t *testing.T) {
		status := RouterHealthStatus{}

		// Update to healthy
		status.UpdateHealth(true, 100*time.Millisecond, "")
		assert.True(t, status.IsHealthy)
		assert.False(t, status.LastChecked.IsZero())
		assert.Empty(t, status.ErrorMessage)
		assert.Equal(t, 100*time.Millisecond, status.ResponseTime)

		// Update to unhealthy
		status.UpdateHealth(false, 2*time.Second, "Service unavailable")
		assert.False(t, status.IsHealthy)
		assert.Equal(t, "Service unavailable", status.ErrorMessage)
		assert.Equal(t, 2*time.Second, status.ResponseTime)
	})

	t.Run("ToHealthStatus", func(t *testing.T) {
		routerStatus := RouterHealthStatus{
			IsHealthy:    true,
			LastChecked:  time.Now(),
			ErrorMessage: "",
			ResponseTime: 80 * time.Millisecond,
		}

		healthStatus := routerStatus.ToHealthStatus()
		assert.Equal(t, routerStatus.IsHealthy, healthStatus.Healthy)
		assert.Equal(t, routerStatus.LastChecked, healthStatus.LastChecked)
		assert.Equal(t, routerStatus.ErrorMessage, healthStatus.Message)
		assert.Equal(t, float64(routerStatus.ResponseTime.Nanoseconds())/1e6, healthStatus.ResponseTime)
		assert.Equal(t, 200, healthStatus.StatusCode) // Default status code
	})
}

// UpdateHealth updates the health status
func (rhs *RouterHealthStatus) UpdateHealth(isHealthy bool, responseTime time.Duration, errorMessage string) {
	rhs.IsHealthy = isHealthy
	rhs.LastChecked = time.Now()
	rhs.ResponseTime = responseTime
	rhs.ErrorMessage = errorMessage
}

// ToHealthStatus converts RouterHealthStatus to HealthStatus
func (rhs *RouterHealthStatus) ToHealthStatus() HealthStatus {
	statusCode := 200
	if !rhs.IsHealthy {
		statusCode = 503 // Service Unavailable
	}

	return HealthStatus{
		Healthy:      rhs.IsHealthy,
		LastChecked:  rhs.LastChecked,
		Message:      rhs.ErrorMessage,
		ResponseTime: float64(rhs.ResponseTime.Nanoseconds()) / 1e6, // Convert to milliseconds
		StatusCode:   statusCode,
	}
}

// TestRouterMetrics tests the RouterMetrics struct
func TestRouterMetrics(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var metrics RouterMetrics
		assert.Equal(t, int64(0), metrics.TotalRequests)
		assert.Equal(t, int64(0), metrics.SuccessfulRequests)
		assert.Equal(t, int64(0), metrics.FailedRequests)
		assert.Empty(t, metrics.ProviderUsage)
		assert.Equal(t, time.Duration(0), metrics.AverageResponseTime)
		assert.Empty(t, metrics.ProviderMetrics)
		assert.True(t, metrics.LastReset.IsZero())
	})

	t.Run("Initialization", func(t *testing.T) {
		now := time.Now()
		metrics := RouterMetrics{
			TotalRequests:      100,
			SuccessfulRequests: 95,
			FailedRequests:     5,
			ProviderUsage: map[string]int64{
				"openai":    60,
				"anthropic": 40,
			},
			AverageResponseTime: 150 * time.Millisecond,
			ProviderMetrics: map[string]*ProviderMetrics{
				"openai": {
					RequestCount: 60,
					SuccessCount: 58,
					ErrorCount:   2,
				},
				"anthropic": {
					RequestCount: 40,
					SuccessCount: 37,
					ErrorCount:   3,
				},
			},
			LastReset: now,
		}

		assert.Equal(t, int64(100), metrics.TotalRequests)
		assert.Equal(t, int64(95), metrics.SuccessfulRequests)
		assert.Equal(t, int64(5), metrics.FailedRequests)
		assert.Equal(t, int64(60), metrics.ProviderUsage["openai"])
		assert.Equal(t, int64(40), metrics.ProviderUsage["anthropic"])
		assert.Equal(t, 150*time.Millisecond, metrics.AverageResponseTime)
		assert.Equal(t, now, metrics.LastReset)
		assert.Equal(t, int64(60), metrics.ProviderMetrics["openai"].RequestCount)
		assert.Equal(t, int64(40), metrics.ProviderMetrics["anthropic"].RequestCount)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		metrics := RouterMetrics{
			TotalRequests:      50,
			SuccessfulRequests: 45,
			FailedRequests:     5,
			ProviderUsage: map[string]int64{
				"openai": 30,
				"test":   20,
			},
			AverageResponseTime: 200 * time.Millisecond,
			LastReset:           time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		}

		data, err := json.Marshal(metrics)
		require.NoError(t, err)

		var result RouterMetrics
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, metrics.TotalRequests, result.TotalRequests)
		assert.Equal(t, metrics.SuccessfulRequests, result.SuccessfulRequests)
		assert.Equal(t, metrics.FailedRequests, result.FailedRequests)
		assert.Equal(t, metrics.ProviderUsage, result.ProviderUsage)
		assert.Equal(t, metrics.AverageResponseTime, result.AverageResponseTime)
	})

	t.Run("RecordRequest", func(t *testing.T) {
		metrics := RouterMetrics{
			ProviderMetrics: make(map[string]*ProviderMetrics),
		}

		// Record successful request
		metrics.RecordRequest("openai", true, 100*time.Millisecond)
		assert.Equal(t, int64(1), metrics.TotalRequests)
		assert.Equal(t, int64(1), metrics.SuccessfulRequests)
		assert.Equal(t, int64(0), metrics.FailedRequests)
		assert.Equal(t, int64(1), metrics.ProviderUsage["openai"])
		assert.NotNil(t, metrics.ProviderMetrics["openai"])
		assert.Equal(t, int64(1), metrics.ProviderMetrics["openai"].RequestCount)
		assert.Equal(t, int64(1), metrics.ProviderMetrics["openai"].SuccessCount)

		// Record failed request
		metrics.RecordRequest("anthropic", false, 500*time.Millisecond)
		assert.Equal(t, int64(2), metrics.TotalRequests)
		assert.Equal(t, int64(1), metrics.SuccessfulRequests)
		assert.Equal(t, int64(1), metrics.FailedRequests)
		assert.Equal(t, int64(1), metrics.ProviderUsage["anthropic"])
		assert.NotNil(t, metrics.ProviderMetrics["anthropic"])
		assert.Equal(t, int64(1), metrics.ProviderMetrics["anthropic"].RequestCount)
		assert.Equal(t, int64(1), metrics.ProviderMetrics["anthropic"].ErrorCount)
	})

	t.Run("CalculateAverageResponseTime", func(t *testing.T) {
		metrics := RouterMetrics{
			ProviderMetrics: make(map[string]*ProviderMetrics),
		}

		// Record multiple requests with different response times
		responseTimes := []time.Duration{
			100 * time.Millisecond,
			200 * time.Millisecond,
			300 * time.Millisecond,
		}

		for i, rt := range responseTimes {
			provider := "test-provider"
			if i%2 == 0 {
				provider = "openai"
			}
			metrics.RecordRequest(provider, true, rt)
		}

		metrics.CalculateAverageResponseTime()
		expectedAverage := (100 + 200 + 300) / 3 * time.Millisecond
		assert.Equal(t, expectedAverage, metrics.AverageResponseTime)
	})

	t.Run("Reset", func(t *testing.T) {
		metrics := RouterMetrics{
			TotalRequests:      100,
			SuccessfulRequests: 80,
			FailedRequests:     20,
			ProviderUsage: map[string]int64{
				"openai": 60,
				"test":   40,
			},
			ProviderMetrics: map[string]*ProviderMetrics{
				"openai": {
					RequestCount: 60,
					SuccessCount: 50,
					ErrorCount:   10,
				},
			},
			LastReset: time.Now().Add(-time.Hour),
		}

		oldReset := metrics.LastReset
		time.Sleep(10 * time.Millisecond) // Ensure time difference

		metrics.Reset()
		assert.Equal(t, int64(0), metrics.TotalRequests)
		assert.Equal(t, int64(0), metrics.SuccessfulRequests)
		assert.Equal(t, int64(0), metrics.FailedRequests)
		assert.Empty(t, metrics.ProviderUsage)
		assert.Equal(t, time.Duration(0), metrics.AverageResponseTime)
		assert.True(t, metrics.LastReset.After(oldReset))
	})

	t.Run("GetSuccessRate", func(t *testing.T) {
		metrics := RouterMetrics{
			TotalRequests:      100,
			SuccessfulRequests: 95,
			FailedRequests:     5,
		}

		successRate := metrics.GetSuccessRate()
		assert.Equal(t, 0.95, successRate)

		// Test with zero requests
		metrics.TotalRequests = 0
		successRate = metrics.GetSuccessRate()
		assert.Equal(t, 0.0, successRate)
	})
}

// RecordRequest records a request in the router metrics
func (rm *RouterMetrics) RecordRequest(providerName string, success bool, responseTime time.Duration) {
	rm.TotalRequests++
	if success {
		rm.SuccessfulRequests++
	} else {
		rm.FailedRequests++
	}

	// Update provider usage
	if rm.ProviderUsage == nil {
		rm.ProviderUsage = make(map[string]int64)
	}
	rm.ProviderUsage[providerName]++

	// Update provider-specific metrics
	if rm.ProviderMetrics == nil {
		rm.ProviderMetrics = make(map[string]*ProviderMetrics)
	}
	if rm.ProviderMetrics[providerName] == nil {
		rm.ProviderMetrics[providerName] = &ProviderMetrics{}
	}
	rm.ProviderMetrics[providerName].RecordRequest(success, responseTime, nil)
}

// CalculateAverageResponseTime calculates the average response time across all providers
func (rm *RouterMetrics) CalculateAverageResponseTime() {
	if rm.TotalRequests == 0 {
		rm.AverageResponseTime = 0
		return
	}

	var totalLatency time.Duration
	for _, providerMetrics := range rm.ProviderMetrics {
		totalLatency += providerMetrics.TotalLatency
	}
	rm.AverageResponseTime = totalLatency / time.Duration(rm.TotalRequests)
}

// Reset resets all metrics
func (rm *RouterMetrics) Reset() {
	rm.TotalRequests = 0
	rm.SuccessfulRequests = 0
	rm.FailedRequests = 0
	rm.ProviderUsage = make(map[string]int64)
	rm.AverageResponseTime = 0
	rm.ProviderMetrics = make(map[string]*ProviderMetrics)
	rm.LastReset = time.Now()
}

// GetSuccessRate returns the success rate as a percentage (0.0 to 1.0)
func (rm *RouterMetrics) GetSuccessRate() float64 {
	if rm.TotalRequests == 0 {
		return 0.0
	}
	return float64(rm.SuccessfulRequests) / float64(rm.TotalRequests)
}
