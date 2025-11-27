//go:build dev || debug

package dev

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// MockProvider creates a mock provider for testing
type MockProvider struct {
	Name         string               `json:"name"`
	ProviderType types.ProviderType   `json:"provider_type"`
	Responses    []MockResponse       `json:"responses"`
	RequestCount int                  `json:"request_count"`
	Metrics      *ProviderTestMetrics `json:"metrics"`
	Config       types.ProviderConfig `json:"config"`
}

// MockResponse represents a mock API response
type MockResponse struct {
	StatusCode int               `json:"status_code"`
	Body       interface{}       `json:"body"`
	Delay      time.Duration     `json:"delay,omitempty"`
	Error      string            `json:"error,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// ProviderTestMetrics tracks test metrics for providers
type ProviderTestMetrics struct {
	TotalRequests    int           `json:"total_requests"`
	SuccessResponses int           `json:"success_responses"`
	ErrorResponses   int           `json:"error_responses"`
	AverageLatency   time.Duration `json:"average_latency"`
	LastRequestTime  time.Time     `json:"last_request_time"`
}

// TestSuite provides testing utilities for AI providers
type TestSuite struct {
	Providers map[string]*MockProvider
	Server    *httptest.Server
	Mocks     map[string][]MockResponse
	Config    TestSuiteConfig
}

// TestSuiteConfig configures the test suite
type TestSuiteConfig struct {
	EnableLogging      bool          `json:"enable_logging"`
	LogRequests        bool          `json:"log_requests"`
	LogResponses       bool          `json:"log_responses"`
	DefaultTimeout     time.Duration `json:"default_timeout"`
	RecordInteractions bool          `json:"record_interactions"`
	OutputDir          string        `json:"output_dir,omitempty"`
}

// NewTestSuite creates a new test suite
func NewTestSuite(config TestSuiteConfig) *TestSuite {
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 30 * time.Second
	}

	suite := &TestSuite{
		Providers: make(map[string]*MockProvider),
		Mocks:     make(map[string][]MockResponse),
		Config:    config,
	}

	// Create mock server
	suite.Server = httptest.NewServer(http.HandlerFunc(suite.handleMockRequest))

	return suite
}

// AddMockProvider adds a mock provider to the test suite
func (ts *TestSuite) AddMockProvider(name string, providerType types.ProviderType, config types.ProviderConfig) *MockProvider {
	provider := &MockProvider{
		Name:         name,
		ProviderType: providerType,
		Responses:    []MockResponse{},
		Metrics:      &ProviderTestMetrics{},
		Config:       config,
	}

	// Update base URL to point to mock server
	config.BaseURL = ts.Server.URL

	ts.Providers[name] = provider
	return provider
}

// AddMockResponse adds a mock response for a provider
func (ts *TestSuite) AddMockResponse(providerName string, response MockResponse) {
	if ts.Mocks[providerName] == nil {
		ts.Mocks[providerName] = []MockResponse{}
	}
	ts.Mocks[providerName] = append(ts.Mocks[providerName], response)
}

// handleMockRequest handles mock HTTP requests
func (ts *TestSuite) handleMockRequest(w http.ResponseWriter, r *http.Request) {
	// Log request if enabled
	if ts.Config.LogRequests {
		fmt.Printf("[MOCK] %s %s\n", r.Method, r.URL.Path)
	}

	// Determine which provider this request is for
	providerName := ts.getProviderFromRequest(r)
	if providerName == "" {
		http.Error(w, "Unknown provider", http.StatusNotFound)
		return
	}

	// Get mock responses for this provider
	responses := ts.Mocks[providerName]
	if len(responses) == 0 {
		// Default success response
		ts.sendDefaultResponse(w, r)
		return
	}

	// Get the next response (cycle through them)
	provider := ts.Providers[providerName]
	response := responses[provider.RequestCount%len(responses)]
	provider.RequestCount++

	// Update metrics
	provider.Metrics.TotalRequests++
	provider.Metrics.LastRequestTime = time.Now()

	// Apply delay if specified
	if response.Delay > 0 {
		time.Sleep(response.Delay)
	}

	// Set headers
	if response.Headers != nil {
		for key, value := range response.Headers {
			w.Header().Set(key, value)
		}
	}

	// Send response
	if response.Error != "" {
		provider.Metrics.ErrorResponses++
		http.Error(w, response.Error, response.StatusCode)
	} else {
		provider.Metrics.SuccessResponses++
		w.WriteHeader(response.StatusCode)

		if response.Body != nil {
			if bodyStr, ok := response.Body.(string); ok {
				_, _ = w.Write([]byte(bodyStr))
			} else {
				_ = json.NewEncoder(w).Encode(response.Body)
			}
		}
	}

	// Log response if enabled
	if ts.Config.LogResponses {
		fmt.Printf("[MOCK] Response: %d\n", response.StatusCode)
	}
}

// getProviderFromRequest determines the provider from the request
func (ts *TestSuite) getProviderFromRequest(r *http.Request) string {
	// Simple heuristic based on URL and headers
	if strings.Contains(r.URL.Path, "openai") || strings.Contains(r.Header.Get("Authorization"), "Bearer") {
		for name, provider := range ts.Providers {
			if provider.ProviderType == types.ProviderTypeOpenAI ||
				provider.ProviderType == types.ProviderTypeOpenRouter {
				return name
			}
		}
	}

	if strings.Contains(r.Header.Get("x-api-key"), "") && strings.Contains(r.URL.Path, "anthropic") {
		for name, provider := range ts.Providers {
			if provider.ProviderType == types.ProviderTypeAnthropic {
				return name
			}
		}
	}

	// Default to first provider
	for name := range ts.Providers {
		return name
	}

	return ""
}

// sendDefaultResponse sends a default successful response
func (ts *TestSuite) sendDefaultResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	defaultResponse := map[string]interface{}{
		"id":      "test-response",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "This is a mock response from the test suite.",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     10,
			"completion_tokens": 5,
			"total_tokens":      15,
		},
	}

	_ = json.NewEncoder(w).Encode(defaultResponse)
}

// GetTestHTTPClient returns an HTTP client configured for testing
func (ts *TestSuite) GetTestHTTPClient() *http.Client {
	return &http.Client{
		Timeout: ts.Config.DefaultTimeout,
	}
}

// GetProviderConfig returns test configuration for a provider
func (ts *TestSuite) GetProviderConfig(name string) (types.ProviderConfig, error) {
	provider, exists := ts.Providers[name]
	if !exists {
		return types.ProviderConfig{}, fmt.Errorf("provider %s not found", name)
	}

	return provider.Config, nil
}

// RunProviderTest runs a test against a specific provider
func (ts *TestSuite) RunProviderTest(t *testing.T, providerName string, testFunc func(*testing.T, *MockProvider)) {
	provider, exists := ts.Providers[providerName]
	if !exists {
		t.Fatalf("Provider %s not found", providerName)
	}

	testFunc(t, provider)
}

// Cleanup cleans up the test suite
func (ts *TestSuite) Cleanup() {
	if ts.Server != nil {
		ts.Server.Close()
	}
}

// PerformanceTest provides performance testing utilities
type PerformanceTest struct {
	Config PerformanceTestConfig
}

// PerformanceTestConfig configures performance tests
type PerformanceTestConfig struct {
	ConcurrentRequests int           `json:"concurrent_requests"`
	RequestsPerSecond  int           `json:"requests_per_second"`
	TestDuration       time.Duration `json:"test_duration"`
	WarmupRequests     int           `json:"warmup_requests"`
	EnableMetrics      bool          `json:"enable_metrics"`
}

// PerformanceResult contains performance test results
type PerformanceResult struct {
	TotalRequests      int           `json:"total_requests"`
	SuccessfulRequests int           `json:"successful_requests"`
	FailedRequests     int           `json:"failed_requests"`
	AverageLatency     time.Duration `json:"average_latency"`
	P95Latency         time.Duration `json:"p95_latency"`
	P99Latency         time.Duration `json:"p99_latency"`
	RequestsPerSecond  float64       `json:"requests_per_second"`
	ErrorRate          float64       `json:"error_rate"`
}

// NewPerformanceTest creates a new performance test
func NewPerformanceTest(config PerformanceTestConfig) *PerformanceTest {
	return &PerformanceTest{
		Config: config,
	}
}

// RunPerformanceTest runs a performance test against a function
func (pt *PerformanceTest) RunPerformanceTest(ctx context.Context, testFunc func(context.Context) error) (*PerformanceResult, error) {
	result := &PerformanceResult{}
	var latencies []time.Duration
	startTime := time.Now()

	// Warmup
	for i := 0; i < pt.Config.WarmupRequests; i++ {
		_ = testFunc(ctx)
	}

	// Main test
	testCtx, cancel := context.WithTimeout(ctx, pt.Config.TestDuration)
	defer cancel()

	// Simple single-threaded implementation for now
	for {
		select {
		case <-testCtx.Done():
			goto done
		default:
			reqStart := time.Now()
			err := testFunc(testCtx)
			latency := time.Since(reqStart)

			result.TotalRequests++
			latencies = append(latencies, latency)

			if err != nil {
				result.FailedRequests++
			} else {
				result.SuccessfulRequests++
			}
		}
	}

done:
	duration := time.Since(startTime)

	// Calculate metrics
	if result.TotalRequests > 0 {
		result.ErrorRate = float64(result.FailedRequests) / float64(result.TotalRequests)
		result.RequestsPerSecond = float64(result.TotalRequests) / duration.Seconds()

		// Calculate average latency
		var totalLatency time.Duration
		for _, latency := range latencies {
			totalLatency += latency
		}
		result.AverageLatency = totalLatency / time.Duration(len(latencies))

		// Calculate percentiles
		if len(latencies) > 0 {
			result.P95Latency = pt.calculatePercentile(latencies, 0.95)
			result.P99Latency = pt.calculatePercentile(latencies, 0.99)
		}
	}

	return result, nil
}

// calculatePercentile calculates the nth percentile of latencies
func (pt *PerformanceTest) calculatePercentile(latencies []time.Duration, percentile float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Convert to nanoseconds for sorting
	nanos := make([]int64, len(latencies))
	for i, latency := range latencies {
		nanos[i] = latency.Nanoseconds()
	}

	// Simple bubble sort (for small datasets)
	for i := 0; i < len(nanos)-1; i++ {
		for j := 0; j < len(nanos)-i-1; j++ {
			if nanos[j] > nanos[j+1] {
				nanos[j], nanos[j+1] = nanos[j+1], nanos[j]
			}
		}
	}

	index := int(float64(len(nanos)) * percentile)
	if index >= len(nanos) {
		index = len(nanos) - 1
	}

	return time.Duration(nanos[index])
}

// DebugLogger provides debugging utilities
type DebugLogger struct {
	Enabled bool
	Output  *os.File
}

// NewDebugLogger creates a new debug logger
func NewDebugLogger(enabled bool, outputFile string) *DebugLogger {
	logger := &DebugLogger{
		Enabled: enabled,
		Output:  os.Stdout,
	}

	if outputFile != "" {
		if file, err := os.Create(outputFile); err == nil {
			logger.Output = file
		}
	}

	return logger
}

// Log logs a debug message
func (dl *DebugLogger) Log(message string, args ...interface{}) {
	if !dl.Enabled {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	prefix := fmt.Sprintf("[%s] [DEBUG] ", timestamp)
	_, _ = fmt.Fprintf(dl.Output, prefix+message+"\n", args...)
}

// LogRequest logs an HTTP request
func (dl *DebugLogger) LogRequest(method, url string, headers map[string]string, body interface{}) {
	if !dl.Enabled {
		return
	}

	dl.Log("REQUEST: %s %s", method, url)
	for key, value := range headers {
		dl.Log("  Header: %s: %s", key, value)
	}
	if body != nil {
		dl.Log("  Body: %+v", body)
	}
}

// LogResponse logs an HTTP response
func (dl *DebugLogger) LogResponse(statusCode int, headers map[string]string, body interface{}) {
	if !dl.Enabled {
		return
	}

	dl.Log("RESPONSE: %d", statusCode)
	for key, value := range headers {
		dl.Log("  Header: %s: %s", key, value)
	}
	if body != nil {
		dl.Log("  Body: %+v", body)
	}
}

// Close closes the debug logger
func (dl *DebugLogger) Close() error {
	if dl.Output != nil && dl.Output != os.Stdout {
		return dl.Output.Close()
	}
	return nil
}
