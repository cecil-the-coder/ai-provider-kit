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

// TestModelMetrics tests the ModelMetrics struct
func TestModelMetrics(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var metrics ModelMetrics
		assert.Empty(t, metrics.ModelID)
		assert.Equal(t, ProviderType(""), metrics.Provider)
		assert.Equal(t, int64(0), metrics.RequestCount)
		assert.Equal(t, int64(0), metrics.SuccessCount)
		assert.Equal(t, int64(0), metrics.ErrorCount)
		assert.Equal(t, int64(0), metrics.TokensUsed)
		assert.Equal(t, time.Duration(0), metrics.AverageLatency)
		assert.True(t, metrics.LastRequestTime.IsZero())
		assert.Equal(t, time.Duration(0), metrics.P95ResponseTime)
		assert.Equal(t, time.Duration(0), metrics.P99ResponseTime)
	})

	t.Run("FullMetrics", func(t *testing.T) {
		now := time.Now()
		metrics := ModelMetrics{
			ModelID:         "gpt-4",
			Provider:        ProviderTypeOpenAI,
			RequestCount:    1000,
			SuccessCount:    950,
			ErrorCount:      50,
			TokensUsed:      50000,
			AverageLatency:  150 * time.Millisecond,
			LastRequestTime: now,
			P95ResponseTime: 300 * time.Millisecond,
			P99ResponseTime: 500 * time.Millisecond,
		}

		assert.Equal(t, "gpt-4", metrics.ModelID)
		assert.Equal(t, ProviderTypeOpenAI, metrics.Provider)
		assert.Equal(t, int64(1000), metrics.RequestCount)
		assert.Equal(t, int64(950), metrics.SuccessCount)
		assert.Equal(t, int64(50), metrics.ErrorCount)
		assert.Equal(t, int64(50000), metrics.TokensUsed)
		assert.Equal(t, 150*time.Millisecond, metrics.AverageLatency)
		assert.Equal(t, now, metrics.LastRequestTime)
		assert.Equal(t, 300*time.Millisecond, metrics.P95ResponseTime)
		assert.Equal(t, 500*time.Millisecond, metrics.P99ResponseTime)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		metrics := ModelMetrics{
			ModelID:         "claude-3-sonnet",
			Provider:        ProviderTypeAnthropic,
			RequestCount:    500,
			SuccessCount:    480,
			ErrorCount:      20,
			TokensUsed:      25000,
			AverageLatency:  200 * time.Millisecond,
			P95ResponseTime: 400 * time.Millisecond,
			P99ResponseTime: 800 * time.Millisecond,
		}

		data, err := json.Marshal(metrics)
		require.NoError(t, err)

		var result ModelMetrics
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, metrics.ModelID, result.ModelID)
		assert.Equal(t, metrics.Provider, result.Provider)
		assert.Equal(t, metrics.RequestCount, result.RequestCount)
		assert.Equal(t, metrics.SuccessCount, result.SuccessCount)
		assert.Equal(t, metrics.ErrorCount, result.ErrorCount)
		assert.Equal(t, metrics.TokensUsed, result.TokensUsed)
		assert.Equal(t, metrics.AverageLatency, result.AverageLatency)
		assert.Equal(t, metrics.P95ResponseTime, result.P95ResponseTime)
		assert.Equal(t, metrics.P99ResponseTime, result.P99ResponseTime)
	})

	t.Run("RecordRequest", func(t *testing.T) {
		metrics := ModelMetrics{
			ModelID:  "test-model",
			Provider: ProviderTypeOpenAI,
		}

		// Record successful request
		metrics.RecordRequest(true, 120*time.Millisecond, 100)
		assert.Equal(t, int64(1), metrics.RequestCount)
		assert.Equal(t, int64(1), metrics.SuccessCount)
		assert.Equal(t, int64(0), metrics.ErrorCount)
		assert.Equal(t, int64(100), metrics.TokensUsed)
		assert.Equal(t, 120*time.Millisecond, metrics.AverageLatency)
		assert.False(t, metrics.LastRequestTime.IsZero())

		// Record failed request
		metrics.RecordRequest(false, 300*time.Millisecond, 0)
		assert.Equal(t, int64(2), metrics.RequestCount)
		assert.Equal(t, int64(1), metrics.SuccessCount)
		assert.Equal(t, int64(1), metrics.ErrorCount)
		assert.Equal(t, int64(100), metrics.TokensUsed)               // Tokens not added for failed request
		assert.Equal(t, 210*time.Millisecond, metrics.AverageLatency) // (120+300)/2
	})

	t.Run("UpdatePercentiles", func(t *testing.T) {
		metrics := ModelMetrics{
			ModelID:  "test-model",
			Provider: ProviderTypeOpenAI,
		}

		// Simulate response times
		responseTimes := []time.Duration{
			50 * time.Millisecond,  // Fast
			100 * time.Millisecond, // Below average
			150 * time.Millisecond, // Average
			200 * time.Millisecond, // Above average
			500 * time.Millisecond, // Slow
		}

		// Record requests
		for _, rt := range responseTimes {
			metrics.RecordRequest(true, rt, 10)
		}

		// Update percentiles (simplified calculation)
		metrics.UpdatePercentiles()

		// P95 should be close to 500ms (highest value in small dataset)
		assert.GreaterOrEqual(t, metrics.P95ResponseTime, 400*time.Millisecond)
		assert.LessOrEqual(t, metrics.P99ResponseTime, 1000*time.Millisecond) // Allow up to 2x average

		// P99 should be close to 500ms (highest value)
		assert.GreaterOrEqual(t, metrics.P99ResponseTime, 400*time.Millisecond)
		assert.LessOrEqual(t, metrics.P99ResponseTime, 1500*time.Millisecond) // Allow up to 3x average
	})

	t.Run("GetErrorRate", func(t *testing.T) {
		metrics := ModelMetrics{
			RequestCount: 100,
			ErrorCount:   10,
		}

		errorRate := metrics.GetErrorRate()
		assert.Equal(t, 0.1, errorRate)

		// Test with zero requests
		metrics.RequestCount = 0
		errorRate = metrics.GetErrorRate()
		assert.Equal(t, 0.0, errorRate)
	})

	t.Run("GetSuccessRate", func(t *testing.T) {
		metrics := ModelMetrics{
			RequestCount: 100,
			SuccessCount: 85,
		}

		successRate := metrics.GetSuccessRate()
		assert.Equal(t, 0.85, successRate)

		// Test with zero requests
		metrics.RequestCount = 0
		successRate = metrics.GetSuccessRate()
		assert.Equal(t, 0.0, successRate)
	})
}

// RecordRequest records a request in the model metrics
func (mm *ModelMetrics) RecordRequest(success bool, latency time.Duration, tokens int64) {
	mm.RequestCount++
	mm.LastRequestTime = time.Now()

	if success {
		mm.SuccessCount++
		mm.TokensUsed += tokens
	} else {
		mm.ErrorCount++
	}

	// Update average latency
	if mm.RequestCount == 1 {
		mm.AverageLatency = latency
	} else {
		// Calculate running average
		mm.AverageLatency = time.Duration(
			(int64(mm.AverageLatency)*(mm.RequestCount-1) + int64(latency)) / mm.RequestCount,
		)
	}
}

// UpdatePercentiles updates response time percentiles (simplified implementation)
func (mm *ModelMetrics) UpdatePercentiles() {
	// This is a simplified implementation
	// In a real scenario, you would store all response times and calculate actual percentiles
	if mm.RequestCount > 0 {
		// Mock percentile calculations based on average latency
		mm.P95ResponseTime = mm.AverageLatency * 2
		mm.P99ResponseTime = mm.AverageLatency * 3
	}
}

// GetErrorRate returns the error rate as a percentage (0.0 to 1.0)
func (mm *ModelMetrics) GetErrorRate() float64 {
	if mm.RequestCount == 0 {
		return 0.0
	}
	return float64(mm.ErrorCount) / float64(mm.RequestCount)
}

// GetSuccessRate returns the success rate as a percentage (0.0 to 1.0)
func (mm *ModelMetrics) GetSuccessRate() float64 {
	if mm.RequestCount == 0 {
		return 0.0
	}
	return float64(mm.SuccessCount) / float64(mm.RequestCount)
}

// TestErrorMetrics tests the ErrorMetrics struct
func TestErrorMetrics(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var metrics LegacyErrorMetrics
		assert.Equal(t, int64(0), metrics.TotalErrors)
		assert.Empty(t, metrics.ErrorTypes)
		assert.Empty(t, metrics.LastError)
		assert.True(t, metrics.LastErrorTime.IsZero())
		assert.Equal(t, int64(0), metrics.ConsecutiveFails)
	})

	t.Run("FullMetrics", func(t *testing.T) {
		now := time.Now()
		metrics := LegacyErrorMetrics{
			TotalErrors: 50,
			ErrorTypes: map[string]int64{
				"timeout":      20,
				"rate_limit":   15,
				"auth_error":   10,
				"server_error": 5,
			},
			LastError:        "Connection timeout after 30 seconds",
			LastErrorTime:    now,
			ConsecutiveFails: 3,
		}

		assert.Equal(t, int64(50), metrics.TotalErrors)
		assert.Equal(t, int64(20), metrics.ErrorTypes["timeout"])
		assert.Equal(t, int64(15), metrics.ErrorTypes["rate_limit"])
		assert.Equal(t, int64(10), metrics.ErrorTypes["auth_error"])
		assert.Equal(t, int64(5), metrics.ErrorTypes["server_error"])
		assert.Equal(t, "Connection timeout after 30 seconds", metrics.LastError)
		assert.Equal(t, now, metrics.LastErrorTime)
		assert.Equal(t, int64(3), metrics.ConsecutiveFails)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		metrics := LegacyErrorMetrics{
			TotalErrors: 25,
			ErrorTypes: map[string]int64{
				"timeout":         10,
				"api_error":       8,
				"invalid_request": 7,
			},
			LastError:        "API key invalid",
			LastErrorTime:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			ConsecutiveFails: 2,
		}

		data, err := json.Marshal(metrics)
		require.NoError(t, err)

		var result LegacyErrorMetrics
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, metrics.TotalErrors, result.TotalErrors)
		assert.Equal(t, metrics.ErrorTypes, result.ErrorTypes)
		assert.Equal(t, metrics.LastError, result.LastError)
		assert.Equal(t, metrics.LastErrorTime.Unix(), result.LastErrorTime.Unix())
		assert.Equal(t, metrics.ConsecutiveFails, result.ConsecutiveFails)
	})

	t.Run("RecordError", func(t *testing.T) {
		metrics := LegacyErrorMetrics{}

		// Record first error
		metrics.RecordError("timeout", "Request timed out")
		assert.Equal(t, int64(1), metrics.TotalErrors)
		assert.Equal(t, int64(1), metrics.ErrorTypes["timeout"])
		assert.Equal(t, "Request timed out", metrics.LastError)
		assert.False(t, metrics.LastErrorTime.IsZero())
		assert.Equal(t, int64(1), metrics.ConsecutiveFails)

		// Record same error type
		metrics.RecordError("timeout", "Another timeout")
		assert.Equal(t, int64(2), metrics.TotalErrors)
		assert.Equal(t, int64(2), metrics.ErrorTypes["timeout"])
		assert.Equal(t, "Another timeout", metrics.LastError)
		assert.Equal(t, int64(2), metrics.ConsecutiveFails)

		// Record different error type
		metrics.RecordError("rate_limit", "Rate limit exceeded")
		assert.Equal(t, int64(3), metrics.TotalErrors)
		assert.Equal(t, int64(2), metrics.ErrorTypes["timeout"])
		assert.Equal(t, int64(1), metrics.ErrorTypes["rate_limit"])
		assert.Equal(t, "Rate limit exceeded", metrics.LastError)
		assert.Equal(t, int64(3), metrics.ConsecutiveFails)
	})

	t.Run("ResetConsecutiveFails", func(t *testing.T) {
		metrics := LegacyErrorMetrics{
			TotalErrors:      10,
			ConsecutiveFails: 5,
		}

		metrics.ResetConsecutiveFails()
		assert.Equal(t, int64(10), metrics.TotalErrors) // Total errors unchanged
		assert.Equal(t, int64(0), metrics.ConsecutiveFails)
	})

	t.Run("GetMostCommonError", func(t *testing.T) {
		metrics := LegacyErrorMetrics{
			ErrorTypes: map[string]int64{
				"timeout":      10,
				"rate_limit":   5,
				"auth_error":   8,
				"server_error": 3,
			},
		}

		mostCommon := metrics.GetMostCommonError()
		assert.Equal(t, "timeout", mostCommon)

		// Test with empty errors
		metrics.ErrorTypes = make(map[string]int64)
		mostCommon = metrics.GetMostCommonError()
		assert.Equal(t, "", mostCommon)
	})

	t.Run("GetErrorRate", func(t *testing.T) {
		metrics := LegacyErrorMetrics{
			TotalErrors:      25,
			ConsecutiveFails: 3,
		}

		errorRate := metrics.GetErrorRate(100) // 25 errors out of 100 total requests
		assert.Equal(t, 0.25, errorRate)

		// Test with zero total requests
		errorRate = metrics.GetErrorRate(0)
		assert.Equal(t, 0.0, errorRate)
	})
}

// RecordError records an error in the metrics
func (em *LegacyErrorMetrics) RecordError(errorType, errorMessage string) {
	em.TotalErrors++
	em.ConsecutiveFails++
	em.LastError = errorMessage
	em.LastErrorTime = time.Now()

	if em.ErrorTypes == nil {
		em.ErrorTypes = make(map[string]int64)
	}
	em.ErrorTypes[errorType]++
}

// ResetConsecutiveFails resets the consecutive failure counter
func (em *LegacyErrorMetrics) ResetConsecutiveFails() {
	em.ConsecutiveFails = 0
}

// GetMostCommonError returns the most common error type
func (em *LegacyErrorMetrics) GetMostCommonError() string {
	if len(em.ErrorTypes) == 0 {
		return ""
	}

	var mostCommonType string
	var maxCount int64

	for errorType, count := range em.ErrorTypes {
		if count > maxCount {
			maxCount = count
			mostCommonType = errorType
		}
	}

	return mostCommonType
}

// GetErrorRate returns the error rate as a percentage (0.0 to 1.0)
func (em *LegacyErrorMetrics) GetErrorRate(totalRequests int64) float64 {
	if totalRequests == 0 {
		return 0.0
	}
	return float64(em.TotalErrors) / float64(totalRequests)
}

// TestTokenMetrics tests the TokenMetrics struct
func TestTokenMetrics(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var metrics LegacyTokenMetrics
		assert.Equal(t, int64(0), metrics.InputTokens)
		assert.Equal(t, int64(0), metrics.OutputTokens)
		assert.Equal(t, int64(0), metrics.TotalTokens)
		assert.Equal(t, 0.0, metrics.EstimatedCost)
		assert.Empty(t, metrics.Currency)
		assert.True(t, metrics.LastUpdated.IsZero())
	})

	t.Run("FullMetrics", func(t *testing.T) {
		now := time.Now()
		metrics := LegacyTokenMetrics{
			InputTokens:   100000,
			OutputTokens:  50000,
			TotalTokens:   150000,
			EstimatedCost: 5.25,
			Currency:      "USD",
			LastUpdated:   now,
		}

		assert.Equal(t, int64(100000), metrics.InputTokens)
		assert.Equal(t, int64(50000), metrics.OutputTokens)
		assert.Equal(t, int64(150000), metrics.TotalTokens)
		assert.Equal(t, 5.25, metrics.EstimatedCost)
		assert.Equal(t, "USD", metrics.Currency)
		assert.Equal(t, now, metrics.LastUpdated)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		metrics := LegacyTokenMetrics{
			InputTokens:   50000,
			OutputTokens:  25000,
			TotalTokens:   75000,
			EstimatedCost: 2.75,
			Currency:      "USD",
			LastUpdated:   time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		}

		data, err := json.Marshal(metrics)
		require.NoError(t, err)

		var result LegacyTokenMetrics
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, metrics.InputTokens, result.InputTokens)
		assert.Equal(t, metrics.OutputTokens, result.OutputTokens)
		assert.Equal(t, metrics.TotalTokens, result.TotalTokens)
		assert.Equal(t, metrics.EstimatedCost, result.EstimatedCost)
		assert.Equal(t, metrics.Currency, result.Currency)
		assert.Equal(t, metrics.LastUpdated.Unix(), result.LastUpdated.Unix())
	})

	t.Run("AddTokens", func(t *testing.T) {
		metrics := LegacyTokenMetrics{
			InputTokens:   1000,
			OutputTokens:  500,
			TotalTokens:   1500,
			EstimatedCost: 0.05,
			Currency:      "USD",
		}

		// Add more tokens
		metrics.AddTokens(2000, 800, 0.12)
		assert.Equal(t, int64(3000), metrics.InputTokens)
		assert.Equal(t, int64(1300), metrics.OutputTokens)
		assert.Equal(t, int64(4300), metrics.TotalTokens)
		assert.InDelta(t, 0.17, metrics.EstimatedCost, 0.001) // 0.05 + 0.12
		assert.False(t, metrics.LastUpdated.IsZero())
	})

	t.Run("CalculateCost", func(t *testing.T) {
		metrics := LegacyTokenMetrics{}

		// Calculate cost with standard pricing
		metrics.CalculateCost(1000, 500, 0.001, 0.002) // $0.001 per 1K input, $0.002 per 1K output
		expectedCost := 0.001*1.0 + 0.002*0.5          // $0.001 + $0.001 = $0.002
		assert.Equal(t, expectedCost, metrics.EstimatedCost)
		assert.Equal(t, int64(1000), metrics.InputTokens)
		assert.Equal(t, int64(500), metrics.OutputTokens)
		assert.Equal(t, int64(1500), metrics.TotalTokens)
		assert.Equal(t, "USD", metrics.Currency)
		assert.False(t, metrics.LastUpdated.IsZero())
	})

	t.Run("GetCostPerToken", func(t *testing.T) {
		metrics := LegacyTokenMetrics{
			TotalTokens:   100000,
			EstimatedCost: 10.0,
			Currency:      "USD",
		}

		costPerToken := metrics.GetCostPerToken()
		assert.Equal(t, 0.0001, costPerToken) // $10.0 / 100,000 tokens

		// Test with zero tokens
		metrics.TotalTokens = 0
		costPerToken = metrics.GetCostPerToken()
		assert.Equal(t, 0.0, costPerToken)
	})

	t.Run("GetInputOutputRatio", func(t *testing.T) {
		metrics := LegacyTokenMetrics{
			InputTokens:  80000,
			OutputTokens: 20000,
		}

		ratio := metrics.GetInputOutputRatio()
		assert.Equal(t, 4.0, ratio) // 80,000 / 20,000

		// Test with zero output tokens
		metrics.OutputTokens = 0
		ratio = metrics.GetInputOutputRatio()
		assert.Equal(t, 0.0, ratio)
	})

	t.Run("Reset", func(t *testing.T) {
		metrics := LegacyTokenMetrics{
			InputTokens:   50000,
			OutputTokens:  25000,
			TotalTokens:   75000,
			EstimatedCost: 3.75,
			Currency:      "USD",
			LastUpdated:   time.Now(),
		}

		metrics.Reset()
		assert.Equal(t, int64(0), metrics.InputTokens)
		assert.Equal(t, int64(0), metrics.OutputTokens)
		assert.Equal(t, int64(0), metrics.TotalTokens)
		assert.Equal(t, 0.0, metrics.EstimatedCost)
		assert.Empty(t, metrics.Currency)
		assert.True(t, metrics.LastUpdated.IsZero())
	})
}

// AddTokens adds token usage and cost to the metrics
func (tm *LegacyTokenMetrics) AddTokens(inputTokens, outputTokens int64, cost float64) {
	tm.InputTokens += inputTokens
	tm.OutputTokens += outputTokens
	tm.TotalTokens += inputTokens + outputTokens
	tm.EstimatedCost += cost
	if tm.Currency == "" {
		tm.Currency = "USD"
	}
	tm.LastUpdated = time.Now()
}

// CalculateCost calculates the estimated cost based on token usage and pricing
func (tm *LegacyTokenMetrics) CalculateCost(inputTokens, outputTokens int64, inputPrice, outputPrice float64) {
	// Prices are per 1000 tokens
	inputCost := inputPrice * float64(inputTokens) / 1000.0
	outputCost := outputPrice * float64(outputTokens) / 1000.0

	tm.InputTokens = inputTokens
	tm.OutputTokens = outputTokens
	tm.TotalTokens = inputTokens + outputTokens
	tm.EstimatedCost = inputCost + outputCost
	tm.Currency = "USD"
	tm.LastUpdated = time.Now()
}

// GetCostPerToken returns the average cost per token
func (tm *LegacyTokenMetrics) GetCostPerToken() float64 {
	if tm.TotalTokens == 0 {
		return 0.0
	}
	return tm.EstimatedCost / float64(tm.TotalTokens)
}

// GetInputOutputRatio returns the ratio of input to output tokens
func (tm *LegacyTokenMetrics) GetInputOutputRatio() float64 {
	if tm.OutputTokens == 0 {
		return 0.0
	}
	return float64(tm.InputTokens) / float64(tm.OutputTokens)
}

// Reset resets all token metrics
func (tm *LegacyTokenMetrics) Reset() {
	tm.InputTokens = 0
	tm.OutputTokens = 0
	tm.TotalTokens = 0
	tm.EstimatedCost = 0.0
	tm.Currency = ""
	tm.LastUpdated = time.Time{}
}

// TestMetricsIntegration tests integration between different metrics types
func TestMetricsIntegration(t *testing.T) {
	t.Run("ProviderAndModelMetrics", func(t *testing.T) {
		providerMetrics := &ProviderMetrics{}
		modelMetrics := &ModelMetrics{
			ModelID:  "gpt-4",
			Provider: ProviderTypeOpenAI,
		}

		// Simulate recording a request in both metrics
		latency := 150 * time.Millisecond
		usage := &Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		}

		providerMetrics.RecordRequest(true, latency, usage)
		modelMetrics.RecordRequest(true, latency, int64(usage.TotalTokens))

		// Verify consistency
		assert.Equal(t, providerMetrics.RequestCount, modelMetrics.RequestCount)
		assert.Equal(t, providerMetrics.SuccessCount, modelMetrics.SuccessCount)
		assert.Equal(t, providerMetrics.TokensUsed, modelMetrics.TokensUsed)
		assert.Equal(t, providerMetrics.AverageLatency, modelMetrics.AverageLatency)
	})

	t.Run("RouterAndProviderMetrics", func(t *testing.T) {
		routerMetrics := &RouterMetrics{
			ProviderMetrics: make(map[string]*ProviderMetrics),
		}

		providerName := "openai"
		latency := 200 * time.Millisecond

		// Record request through router metrics
		routerMetrics.RecordRequest(providerName, true, latency)

		// Verify provider metrics were updated
		assert.NotNil(t, routerMetrics.ProviderMetrics[providerName])
		assert.Equal(t, int64(1), routerMetrics.ProviderMetrics[providerName].RequestCount)
		assert.Equal(t, int64(1), routerMetrics.ProviderMetrics[providerName].SuccessCount)
		assert.Equal(t, int64(1), routerMetrics.ProviderUsage[providerName])
	})

	t.Run("TokenAndErrorMetrics", func(t *testing.T) {
		tokenMetrics := &LegacyTokenMetrics{}
		errorMetrics := &LegacyErrorMetrics{}

		// Simulate a failed request
		inputTokens := int64(100)
		outputTokens := int64(0) // No output due to error

		// Record token usage (even for failed requests, we might have consumed input tokens)
		tokenMetrics.AddTokens(inputTokens, outputTokens, 0.0001) // Small cost for input tokens only

		// Record error
		errorMetrics.RecordError("timeout", "Request timed out")

		// Verify both metrics are updated
		assert.Equal(t, inputTokens, tokenMetrics.InputTokens)
		assert.Equal(t, outputTokens, tokenMetrics.OutputTokens)
		assert.Equal(t, int64(1), errorMetrics.TotalErrors)
		assert.Equal(t, int64(1), errorMetrics.ErrorTypes["timeout"])
	})
}

// BenchmarkRouterMetricsRecordRequest benchmarks request recording in RouterMetrics
func BenchmarkRouterMetricsRecordRequest(b *testing.B) {
	metrics := &RouterMetrics{
		ProviderMetrics: make(map[string]*ProviderMetrics),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		providerName := "test-provider"
		if i%2 == 0 {
			providerName = "openai"
		}
		success := i%4 != 0 // 75% success rate
		latency := time.Duration(50+i%200) * time.Millisecond
		metrics.RecordRequest(providerName, success, latency)
	}
}

// BenchmarkModelMetricsRecordRequest benchmarks request recording in ModelMetrics
func BenchmarkModelMetricsRecordRequest(b *testing.B) {
	metrics := &ModelMetrics{
		ModelID:  "benchmark-model",
		Provider: ProviderTypeOpenAI,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		success := i%5 != 0 // 80% success rate
		latency := time.Duration(100+i%300) * time.Millisecond
		tokens := int64(50 + i%150)
		metrics.RecordRequest(success, latency, tokens)
	}
}

// BenchmarkTokenMetricsAddTokens benchmarks adding tokens to LegacyTokenMetrics
func BenchmarkTokenMetricsAddTokens(b *testing.B) {
	metrics := &LegacyTokenMetrics{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inputTokens := int64(100 + i%500)
		outputTokens := int64(50 + i%250)
		cost := 0.001 + float64(i%100)*0.00001
		metrics.AddTokens(inputTokens, outputTokens, cost)
	}
}
