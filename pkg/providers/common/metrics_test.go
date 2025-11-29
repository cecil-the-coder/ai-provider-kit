package common

import (
	"sync"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewProviderMetrics(t *testing.T) {
	metrics := NewProviderMetrics()

	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}

	// Verify initial values
	if metrics.totalRequests != 0 {
		t.Error("expected totalRequests to be 0")
	}
	if metrics.successRequests != 0 {
		t.Error("expected successRequests to be 0")
	}
	if metrics.errorsByType == nil {
		t.Error("expected errorsByType to be initialized")
	}
	if metrics.modelUsage == nil {
		t.Error("expected modelUsage to be initialized")
	}
}

func TestProviderMetrics_RecordRequest(t *testing.T) {
	metrics := NewProviderMetrics()

	metrics.RecordRequest(types.ProviderTypeOpenAI)

	if metrics.totalRequests != 1 {
		t.Errorf("expected totalRequests to be 1, got %d", metrics.totalRequests)
	}

	// Record multiple requests
	metrics.RecordRequest(types.ProviderTypeAnthropic)
	metrics.RecordRequest(types.ProviderTypeOpenAI)

	if metrics.totalRequests != 3 {
		t.Errorf("expected totalRequests to be 3, got %d", metrics.totalRequests)
	}
}

func TestProviderMetrics_RecordSuccess(t *testing.T) {
	metrics := NewProviderMetrics()

	responseTime := 100 * time.Millisecond
	tokens := int64(150)
	modelID := "gpt-4"

	metrics.RecordSuccess(types.ProviderTypeOpenAI, responseTime, tokens, modelID)

	if metrics.successRequests != 1 {
		t.Errorf("expected successRequests to be 1, got %d", metrics.successRequests)
	}

	if metrics.totalTokensUsed != tokens {
		t.Errorf("expected totalTokensUsed to be %d, got %d", tokens, metrics.totalTokensUsed)
	}

	// Check model usage
	metrics.mu.RLock()
	usage, exists := metrics.modelUsage[modelID]
	metrics.mu.RUnlock()

	if !exists {
		t.Error("expected model usage to be recorded")
	}
	if usage != 1 {
		t.Errorf("expected model usage to be 1, got %d", usage)
	}

	// Record another success
	metrics.RecordSuccess(types.ProviderTypeOpenAI, responseTime, tokens, modelID)

	if metrics.successRequests != 2 {
		t.Errorf("expected successRequests to be 2, got %d", metrics.successRequests)
	}
}

func TestProviderMetrics_RecordError(t *testing.T) {
	metrics := NewProviderMetrics()

	metrics.RecordError(types.ProviderTypeOpenAI, "rate_limit")

	if metrics.failedRequests != 1 {
		t.Errorf("expected failedRequests to be 1, got %d", metrics.failedRequests)
	}

	metrics.mu.RLock()
	errorCount := metrics.errorsByType["rate_limit"]
	providerErrors := metrics.errorsByProvider[types.ProviderTypeOpenAI]
	metrics.mu.RUnlock()

	if errorCount != 1 {
		t.Errorf("expected error count to be 1, got %d", errorCount)
	}
	if providerErrors != 1 {
		t.Errorf("expected provider errors to be 1, got %d", providerErrors)
	}
}

func TestProviderMetrics_RecordInitialization(t *testing.T) {
	metrics := NewProviderMetrics()

	metrics.RecordInitialization(types.ProviderTypeOpenAI)

	metrics.mu.RLock()
	count := metrics.initializations[types.ProviderTypeOpenAI]
	metrics.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected initialization count to be 1, got %d", count)
	}
}

func TestProviderMetrics_RecordHealthCheck(t *testing.T) {
	metrics := NewProviderMetrics()

	// Record successful health check
	metrics.RecordHealthCheck(types.ProviderTypeOpenAI, true)

	metrics.mu.RLock()
	healthChecks := metrics.healthChecks[types.ProviderTypeOpenAI]
	healthCheckFails := metrics.healthCheckFails[types.ProviderTypeOpenAI]
	metrics.mu.RUnlock()

	if healthChecks != 1 {
		t.Errorf("expected health checks to be 1, got %d", healthChecks)
	}
	if healthCheckFails != 0 {
		t.Errorf("expected health check fails to be 0, got %d", healthCheckFails)
	}

	// Record failed health check
	metrics.RecordHealthCheck(types.ProviderTypeOpenAI, false)

	metrics.mu.RLock()
	healthChecks = metrics.healthChecks[types.ProviderTypeOpenAI]
	healthCheckFails = metrics.healthCheckFails[types.ProviderTypeOpenAI]
	metrics.mu.RUnlock()

	if healthChecks != 2 {
		t.Errorf("expected health checks to be 2, got %d", healthChecks)
	}
	if healthCheckFails != 1 {
		t.Errorf("expected health check fails to be 1, got %d", healthCheckFails)
	}
}

func TestProviderMetrics_RecordRateLimitHit(t *testing.T) {
	metrics := NewProviderMetrics()

	metrics.RecordRateLimitHit(types.ProviderTypeOpenAI)

	metrics.mu.RLock()
	hits := metrics.rateLimitHits[types.ProviderTypeOpenAI]
	metrics.mu.RUnlock()

	if hits != 1 {
		t.Errorf("expected rate limit hits to be 1, got %d", hits)
	}
}

func TestProviderMetrics_GetSnapshot(t *testing.T) {
	metrics := NewProviderMetrics()

	// Record some data
	metrics.RecordRequest(types.ProviderTypeOpenAI)
	metrics.RecordSuccess(types.ProviderTypeOpenAI, 100*time.Millisecond, 50, "gpt-4")
	metrics.RecordError(types.ProviderTypeAnthropic, "api_error")

	snapshot := metrics.GetSnapshot()

	if snapshot.TotalRequests != 1 {
		t.Errorf("expected total requests to be 1, got %d", snapshot.TotalRequests)
	}

	if snapshot.SuccessRequests != 1 {
		t.Errorf("expected success requests to be 1, got %d", snapshot.SuccessRequests)
	}

	if snapshot.FailedRequests != 1 {
		t.Errorf("expected failed requests to be 1, got %d", snapshot.FailedRequests)
	}

	if snapshot.SuccessRate != 1.0 {
		t.Errorf("expected success rate to be 1.0, got %f", snapshot.SuccessRate)
	}

	if snapshot.TotalTokensUsed != 50 {
		t.Errorf("expected total tokens to be 50, got %d", snapshot.TotalTokensUsed)
	}
}

func TestProviderMetrics_GetMetricsForProvider(t *testing.T) {
	metrics := NewProviderMetrics()

	metrics.RecordError(types.ProviderTypeOpenAI, "test_error")
	metrics.RecordHealthCheck(types.ProviderTypeOpenAI, true)
	metrics.RecordRateLimitHit(types.ProviderTypeOpenAI)

	providerMetrics := metrics.GetMetricsForProvider(types.ProviderTypeOpenAI)

	if providerMetrics.Provider != string(types.ProviderTypeOpenAI) {
		t.Errorf("expected provider %s, got %s", types.ProviderTypeOpenAI, providerMetrics.Provider)
	}

	if providerMetrics.Errors != 1 {
		t.Errorf("expected 1 error, got %d", providerMetrics.Errors)
	}

	if providerMetrics.HealthChecks != 1 {
		t.Errorf("expected 1 health check, got %d", providerMetrics.HealthChecks)
	}

	if providerMetrics.RateLimitHits != 1 {
		t.Errorf("expected 1 rate limit hit, got %d", providerMetrics.RateLimitHits)
	}
}

func TestProviderMetrics_Reset(t *testing.T) {
	metrics := NewProviderMetrics()

	// Record some data
	metrics.RecordRequest(types.ProviderTypeOpenAI)
	metrics.RecordSuccess(types.ProviderTypeOpenAI, 100*time.Millisecond, 50, "gpt-4")
	metrics.RecordError(types.ProviderTypeAnthropic, "api_error")

	// Reset
	metrics.Reset()

	// Verify everything is reset
	if metrics.totalRequests != 0 {
		t.Errorf("expected total requests to be 0, got %d", metrics.totalRequests)
	}

	if metrics.successRequests != 0 {
		t.Errorf("expected success requests to be 0, got %d", metrics.successRequests)
	}

	if metrics.failedRequests != 0 {
		t.Errorf("expected failed requests to be 0, got %d", metrics.failedRequests)
	}

	if len(metrics.errorsByType) != 0 {
		t.Errorf("expected errorsByType to be empty, got %d", len(metrics.errorsByType))
	}

	if len(metrics.modelUsage) != 0 {
		t.Errorf("expected modelUsage to be empty, got %d", len(metrics.modelUsage))
	}
}

func TestProviderMetrics_GetTopModels(t *testing.T) {
	metrics := NewProviderMetrics()

	// Record model usage
	metrics.RecordSuccess(types.ProviderTypeOpenAI, 100*time.Millisecond, 50, "gpt-4")
	metrics.RecordSuccess(types.ProviderTypeOpenAI, 100*time.Millisecond, 50, "gpt-4")
	metrics.RecordSuccess(types.ProviderTypeOpenAI, 100*time.Millisecond, 50, "gpt-4")
	metrics.RecordSuccess(types.ProviderTypeOpenAI, 100*time.Millisecond, 50, "gpt-3.5-turbo")
	metrics.RecordSuccess(types.ProviderTypeOpenAI, 100*time.Millisecond, 50, "gpt-3.5-turbo")
	metrics.RecordSuccess(types.ProviderTypeOpenAI, 100*time.Millisecond, 50, "claude-3")

	topModels := metrics.GetTopModels(2)

	if len(topModels) != 2 {
		t.Errorf("expected 2 top models, got %d", len(topModels))
	}

	// First should be gpt-4 with count 3
	if topModels[0].ModelID != "gpt-4" {
		t.Errorf("expected first model to be gpt-4, got %s", topModels[0].ModelID)
	}
	if topModels[0].Count != 3 {
		t.Errorf("expected first model count to be 3, got %d", topModels[0].Count)
	}

	// Second should be gpt-3.5-turbo with count 2
	if topModels[1].ModelID != "gpt-3.5-turbo" {
		t.Errorf("expected second model to be gpt-3.5-turbo, got %s", topModels[1].ModelID)
	}
	if topModels[1].Count != 2 {
		t.Errorf("expected second model count to be 2, got %d", topModels[1].Count)
	}

	// Test with no limit
	allModels := metrics.GetTopModels(0)
	if len(allModels) != 3 {
		t.Errorf("expected 3 models with no limit, got %d", len(allModels))
	}
}

func TestProviderMetrics_GetErrorSummary(t *testing.T) {
	metrics := NewProviderMetrics()

	// Record various errors
	metrics.RecordError(types.ProviderTypeOpenAI, "rate_limit")
	metrics.RecordError(types.ProviderTypeOpenAI, "rate_limit")
	metrics.RecordError(types.ProviderTypeOpenAI, "api_error")

	summary := metrics.GetErrorSummary()

	if summary.TotalErrors != 3 {
		t.Errorf("expected total errors to be 3, got %d", summary.TotalErrors)
	}

	if len(summary.ErrorTypes) != 2 {
		t.Errorf("expected 2 error types, got %d", len(summary.ErrorTypes))
	}

	// Find rate_limit error
	var rateLimitError *ErrorType
	for i := range summary.ErrorTypes {
		if summary.ErrorTypes[i].Type == "rate_limit" {
			rateLimitError = &summary.ErrorTypes[i]
			break
		}
	}

	if rateLimitError == nil {
		t.Fatal("expected to find rate_limit error")
	}

	if rateLimitError.Count != 2 {
		t.Errorf("expected rate_limit count to be 2, got %d", rateLimitError.Count)
	}

	expectedPercentage := (2.0 / 3.0) * 100
	if rateLimitError.Percentage < expectedPercentage-0.1 || rateLimitError.Percentage > expectedPercentage+0.1 {
		t.Errorf("expected percentage around %.2f, got %.2f", expectedPercentage, rateLimitError.Percentage)
	}
}

func TestProviderMetrics_ResponseTime(t *testing.T) {
	metrics := NewProviderMetrics()

	// Record successes with different response times
	metrics.RecordSuccess(types.ProviderTypeOpenAI, 100*time.Millisecond, 50, "gpt-4")
	metrics.RecordSuccess(types.ProviderTypeOpenAI, 200*time.Millisecond, 50, "gpt-4")
	metrics.RecordSuccess(types.ProviderTypeOpenAI, 150*time.Millisecond, 50, "gpt-4")

	snapshot := metrics.GetSnapshot()

	// Average should be 150ms
	expectedAvg := 150 * time.Millisecond
	if snapshot.AvgResponseTime != expectedAvg {
		t.Errorf("expected avg response time to be %v, got %v", expectedAvg, snapshot.AvgResponseTime)
	}

	// Min should be 100ms
	expectedMin := 100 * time.Millisecond
	if snapshot.MinResponseTime != expectedMin {
		t.Errorf("expected min response time to be %v, got %v", expectedMin, snapshot.MinResponseTime)
	}

	// Max should be 200ms
	expectedMax := 200 * time.Millisecond
	if snapshot.MaxResponseTime != expectedMax {
		t.Errorf("expected max response time to be %v, got %v", expectedMax, snapshot.MaxResponseTime)
	}
}

func TestProviderMetrics_ThreadSafety(t *testing.T) {
	metrics := NewProviderMetrics()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent RecordRequest
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.RecordRequest(types.ProviderTypeOpenAI)
		}()
	}

	// Concurrent RecordSuccess
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.RecordSuccess(types.ProviderTypeOpenAI, 100*time.Millisecond, 50, "gpt-4")
		}()
	}

	// Concurrent RecordError
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.RecordError(types.ProviderTypeOpenAI, "test_error")
		}()
	}

	// Concurrent GetSnapshot
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = metrics.GetSnapshot()
		}()
	}

	wg.Wait()

	// Verify totals
	if metrics.totalRequests != int64(numGoroutines) {
		t.Errorf("expected %d requests, got %d", numGoroutines, metrics.totalRequests)
	}
}

func TestProviderMetrics_SuccessRate(t *testing.T) {
	metrics := NewProviderMetrics()

	// Record 7 successes and 3 failures
	for i := 0; i < 7; i++ {
		metrics.RecordRequest(types.ProviderTypeOpenAI)
		metrics.RecordSuccess(types.ProviderTypeOpenAI, 100*time.Millisecond, 50, "gpt-4")
	}

	for i := 0; i < 3; i++ {
		metrics.RecordRequest(types.ProviderTypeOpenAI)
		metrics.RecordError(types.ProviderTypeOpenAI, "error")
	}

	snapshot := metrics.GetSnapshot()

	expectedRate := 0.7 // 7/10
	if snapshot.SuccessRate < expectedRate-0.01 || snapshot.SuccessRate > expectedRate+0.01 {
		t.Errorf("expected success rate around %.2f, got %.2f", expectedRate, snapshot.SuccessRate)
	}
}

func TestProviderMetrics_MinResponseTime_NoRequests(t *testing.T) {
	metrics := NewProviderMetrics()

	snapshot := metrics.GetSnapshot()

	// When no requests, min response time should be 0
	if snapshot.MinResponseTime != 0 {
		t.Errorf("expected min response time to be 0 with no requests, got %v", snapshot.MinResponseTime)
	}
}

func TestProviderMetrics_Concurrency(t *testing.T) {
	metrics := NewProviderMetrics()
	
	// Test concurrent access to all methods
	done := make(chan bool)
	
	// Multiple goroutines recording metrics
	for i := 0; i < 20; i++ {
		go func(id int) {
			for j := 0; j < 50; j++ {
				metrics.RecordRequest(types.ProviderTypeOpenAI)
				metrics.RecordSuccess(types.ProviderTypeOpenAI, time.Duration(id)*time.Millisecond, int64(id), "gpt-4")
				metrics.RecordError(types.ProviderTypeAnthropic, "test_error")
				metrics.RecordInitialization(types.ProviderTypeGemini)
				metrics.RecordHealthCheck(types.ProviderTypeCerebras, true)
				metrics.RecordRateLimitHit(types.ProviderTypeOpenRouter)
				_ = metrics.GetSnapshot()
				_ = metrics.GetMetricsForProvider(types.ProviderTypeOpenAI)
				_ = metrics.GetTopModels(5)
				_ = metrics.GetErrorSummary()
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
	
	// Verify we have some data
	snapshot := metrics.GetSnapshot()
	if snapshot.TotalRequests == 0 {
		t.Error("expected some requests to be recorded")
	}
}

func TestProviderMetrics_EdgeCases(t *testing.T) {
	metrics := NewProviderMetrics()
	
	// Test with zero/empty values
	metrics.RecordSuccess(types.ProviderTypeOpenAI, 0, 0, "")
	
	snapshot := metrics.GetSnapshot()
	if snapshot.SuccessRequests != 1 {
		t.Errorf("expected 1 success request, got %d", snapshot.SuccessRequests)
	}
	
	// Test with very large values
	metrics.RecordSuccess(types.ProviderTypeAnthropic, 10*time.Second, 1000000, "large-model")
	
	topModels := metrics.GetTopModels(10)
	hasLargeModel := false
	for _, m := range topModels {
		if m.ModelID == "large-model" {
			hasLargeModel = true
		}
	}
	if !hasLargeModel {
		t.Error("expected large-model in top models")
	}
}
