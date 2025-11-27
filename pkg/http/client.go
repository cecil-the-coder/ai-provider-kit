// Package http provides HTTP client utilities and helpers for AI providers.
// It includes reusable HTTP clients with retry logic, metrics, and interceptors.
package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// HTTPClient provides a reusable HTTP client with common patterns for AI providers
type HTTPClient struct {
	client       *http.Client
	config       HTTPClientConfig
	metrics      *ClientMetrics
	requestCount int64
	successCount int64
	errorCount   int64
	totalLatency int64 // Nanoseconds
	mu           sync.RWMutex
	retryHandler *RetryHandler
}

// HTTPClientConfig configures the HTTP client
type HTTPClientConfig struct {
	Timeout             time.Duration       `json:"timeout,omitempty"`
	MaxRetries          int                 `json:"max_retries,omitempty"`
	BaseRetryDelay      time.Duration       `json:"base_retry_delay,omitempty"`
	MaxRetryDelay       time.Duration       `json:"max_retry_delay,omitempty"`
	BackoffMultiplier   float64             `json:"backoff_multiplier,omitempty"`
	RetryableErrors     []string            `json:"retryable_errors,omitempty"`
	Headers             map[string]string   `json:"headers,omitempty"`
	UserAgent           string              `json:"user_agent,omitempty"`
	EnableMetrics       bool                `json:"enable_metrics,omitempty"`
	RequestInterceptor  RequestInterceptor  `json:"-"`
	ResponseInterceptor ResponseInterceptor `json:"-"`
}

// ClientMetrics tracks HTTP client performance
type ClientMetrics struct {
	TotalRequests   int64         `json:"total_requests"`
	SuccessfulReqs  int64         `json:"successful_requests"`
	FailedReqs      int64         `json:"failed_requests"`
	AvgLatency      time.Duration `json:"avg_latency"`
	P95Latency      time.Duration `json:"p95_latency"`
	LastRequestTime time.Time     `json:"last_request_time"`
	RetryCount      int64         `json:"retry_count"`
	ErrorsByType    map[int]int64 `json:"errors_by_type"`
}

// RequestInterceptor allows modifying requests before sending
type RequestInterceptor interface {
	Intercept(req *http.Request) error
}

// ResponseInterceptor allows processing responses after receiving
type ResponseInterceptor interface {
	Intercept(resp *http.Response) error
}

// RetryHandler manages retry logic with exponential backoff
type RetryHandler struct {
	config HTTPClientConfig
	_      int64 // placeholder for future attempt tracking
}

// NewHTTPClient creates a new HTTP client with common configurations
func NewHTTPClient(config HTTPClientConfig) *HTTPClient {
	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.BaseRetryDelay == 0 {
		config.BaseRetryDelay = time.Second
	}
	if config.MaxRetryDelay == 0 {
		config.MaxRetryDelay = 60 * time.Second
	}
	if config.BackoffMultiplier == 0 {
		config.BackoffMultiplier = 2.0
	}

	// Default retryable HTTP status codes
	if len(config.RetryableErrors) == 0 {
		config.RetryableErrors = []string{"429", "500", "502", "503", "504"}
	}

	client := &HTTPClient{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		config:       config,
		metrics:      &ClientMetrics{ErrorsByType: make(map[int]int64)},
		retryHandler: &RetryHandler{config: config},
	}

	// Set default headers
	if config.Headers == nil {
		config.Headers = make(map[string]string)
	}
	if config.UserAgent != "" {
		config.Headers["User-Agent"] = config.UserAgent
	} else {
		config.Headers["User-Agent"] = "ai-provider-kit/1.0"
	}

	return client
}

// Do executes an HTTP request with retry logic and metrics
func (c *HTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	startTime := time.Now()
	atomic.AddInt64(&c.requestCount, 1)

	// Apply request interceptor
	if c.config.RequestInterceptor != nil {
		if err := c.config.RequestInterceptor.Intercept(req); err != nil {
			return nil, fmt.Errorf("request interceptor failed: %w", err)
		}
	}

	// Set default headers
	for key, value := range c.config.Headers {
		req.Header.Set(key, value)
	}

	var resp *http.Response
	var err error
	var attempts int

	for attempts = 0; attempts <= c.config.MaxRetries; attempts++ {
		if attempts > 0 {
			// Calculate delay and wait
			delay := c.retryHandler.calculateDelay(attempts)
			select {
			case <-time.After(delay):
				// Continue with retry
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			atomic.AddInt64(&c.metrics.RetryCount, 1)
		}

		// Create new request for retry (to avoid body reuse issues)
		retryReq := c.cloneRequest(req)
		retryReq = retryReq.WithContext(ctx)

		// Make the request
		resp, err = c.client.Do(retryReq)
		if err != nil {
			if c.shouldRetryError(err, attempts) {
				continue
			}
			break
		}

		// Apply response interceptor
		if c.config.ResponseInterceptor != nil {
			if interceptErr := c.config.ResponseInterceptor.Intercept(resp); interceptErr != nil {
				_ = resp.Body.Close() //nolint:errcheck // Best effort close
				return nil, fmt.Errorf("response interceptor failed: %w", interceptErr)
			}
		}

		// Check if we should retry based on status code
		if c.shouldRetryStatus(resp.StatusCode, attempts) {
			_ = resp.Body.Close() //nolint:errcheck // Best effort close
			continue
		}

		// Success!
		break
	}

	// Update metrics
	latency := time.Since(startTime)
	c.updateMetrics(resp, err, latency)

	return resp, err
}

// DoWithFullResponse executes request and returns response body as string
func (c *HTTPClient) DoWithFullResponse(ctx context.Context, req *http.Request) (string, *http.Response, error) {
	resp, err := c.Do(ctx, req)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp, fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), resp, nil
}

// PostJSON sends a JSON POST request
func (c *HTTPClient) PostJSON(ctx context.Context, url string, body interface{}) (*http.Response, error) {
	return c.DoJSON(ctx, "POST", url, body)
}

// DoJSON sends a JSON request with specified method
func (c *HTTPClient) DoJSON(ctx context.Context, method, url string, body interface{}) (*http.Response, error) {
	jsonReq, err := NewJSONRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create JSON request: %w", err)
	}
	return c.Do(ctx, jsonReq)
}

// cloneRequest creates a copy of the request for retry
func (c *HTTPClient) cloneRequest(orig *http.Request) *http.Request {
	// This is a simplified clone - in production you'd want to handle body copying properly
	cloned := orig.Clone(orig.Context())
	return cloned
}

// shouldRetryError determines if an error should trigger a retry
func (c *HTTPClient) shouldRetryError(_ error, attempts int) bool {
	if attempts >= c.config.MaxRetries {
		return false
	}

	// Check for retryable error types
	// This could be extended with more sophisticated error detection
	return true
}

// shouldRetryStatus determines if a status code should trigger a retry
func (c *HTTPClient) shouldRetryStatus(statusCode int, attempts int) bool {
	if attempts >= c.config.MaxRetries {
		return false
	}

	// Check if status code is in retryable list
	statusStr := fmt.Sprintf("%d", statusCode)
	for _, retryable := range c.config.RetryableErrors {
		if retryable == statusStr {
			return true
		}
	}

	return false
}

// updateMetrics updates client metrics after a request
func (c *HTTPClient) updateMetrics(resp *http.Response, err error, latency time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics.LastRequestTime = time.Now()
	c.metrics.TotalRequests++

	if err != nil {
		c.errorCount++
		c.metrics.FailedReqs++
	} else {
		c.successCount++
		c.metrics.SuccessfulReqs++
		if resp != nil {
			c.metrics.ErrorsByType[resp.StatusCode]++
		}
	}

	// Update latency metrics (simplified average)
	atomic.AddInt64(&c.totalLatency, latency.Nanoseconds())
	totalReqs := atomic.LoadInt64(&c.requestCount)
	if totalReqs > 0 {
		avgNanos := atomic.LoadInt64(&c.totalLatency) / totalReqs
		c.metrics.AvgLatency = time.Duration(avgNanos)
	}
}

// GetMetrics returns current client metrics
func (c *HTTPClient) GetMetrics() ClientMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics := *c.metrics
	metrics.TotalRequests = atomic.LoadInt64(&c.requestCount)
	metrics.SuccessfulReqs = atomic.LoadInt64(&c.successCount)
	metrics.FailedReqs = atomic.LoadInt64(&c.errorCount)

	return metrics
}

// ResetMetrics resets all metrics
func (c *HTTPClient) ResetMetrics() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics = &ClientMetrics{ErrorsByType: make(map[int]int64)}
	atomic.StoreInt64(&c.requestCount, 0)
	atomic.StoreInt64(&c.successCount, 0)
	atomic.StoreInt64(&c.errorCount, 0)
	atomic.StoreInt64(&c.totalLatency, 0)
}

// calculateDelay calculates retry delay with exponential backoff
func (r *RetryHandler) calculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return r.config.BaseRetryDelay
	}

	// Safe bit shifting to prevent overflow
	if attempt > 30 { // 1 << 30 would overflow int32
		attempt = 30
	}
	multiplier := float64(int(1)<<uint(attempt-1)) * r.config.BackoffMultiplier // #nosec G115 -- attempt is capped at 30, safe conversion
	delay := time.Duration(float64(r.config.BaseRetryDelay) * multiplier)

	if delay > r.config.MaxRetryDelay {
		delay = r.config.MaxRetryDelay
	}

	return delay
}

// HTTPClientBuilder provides a builder pattern for HTTPClient
type HTTPClientBuilder struct {
	config HTTPClientConfig
}

// NewHTTPClientBuilder creates a new builder
func NewHTTPClientBuilder() *HTTPClientBuilder {
	return &HTTPClientBuilder{
		config: HTTPClientConfig{},
	}
}

// WithTimeout sets the timeout
func (b *HTTPClientBuilder) WithTimeout(timeout time.Duration) *HTTPClientBuilder {
	b.config.Timeout = timeout
	return b
}

// WithRetry sets retry configuration
func (b *HTTPClientBuilder) WithRetry(maxRetries int, baseDelay time.Duration) *HTTPClientBuilder {
	b.config.MaxRetries = maxRetries
	b.config.BaseRetryDelay = baseDelay
	return b
}

// WithHeaders sets default headers
func (b *HTTPClientBuilder) WithHeaders(headers map[string]string) *HTTPClientBuilder {
	if b.config.Headers == nil {
		b.config.Headers = make(map[string]string)
	}
	for k, v := range headers {
		b.config.Headers[k] = v
	}
	return b
}

// WithUserAgent sets the user agent
func (b *HTTPClientBuilder) WithUserAgent(userAgent string) *HTTPClientBuilder {
	b.config.UserAgent = userAgent
	return b
}

// WithMetrics enables metrics collection
func (b *HTTPClientBuilder) WithMetrics(enabled bool) *HTTPClientBuilder {
	b.config.EnableMetrics = enabled
	return b
}

// WithRequestInterceptor sets a request interceptor
func (b *HTTPClientBuilder) WithRequestInterceptor(interceptor RequestInterceptor) *HTTPClientBuilder {
	b.config.RequestInterceptor = interceptor
	return b
}

// WithResponseInterceptor sets a response interceptor
func (b *HTTPClientBuilder) WithResponseInterceptor(interceptor ResponseInterceptor) *HTTPClientBuilder {
	b.config.ResponseInterceptor = interceptor
	return b
}

// Build creates the HTTP client
func (b *HTTPClientBuilder) Build() *HTTPClient {
	return NewHTTPClient(b.config)
}
