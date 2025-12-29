// Package http provides HTTP client utilities and helpers for AI providers.
// It includes reusable HTTP clients with retry logic, metrics, and interceptors.
package http

import (
	"bytes"
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
	Timeout               time.Duration       `json:"timeout,omitempty"`
	MaxRetries            int                 `json:"max_retries,omitempty"`
	BaseRetryDelay        time.Duration       `json:"base_retry_delay,omitempty"`
	MaxRetryDelay         time.Duration       `json:"max_retry_delay,omitempty"`
	BackoffMultiplier     float64             `json:"backoff_multiplier,omitempty"`
	RetryableErrors       []string            `json:"retryable_errors,omitempty"`
	Headers               map[string]string   `json:"headers,omitempty"`
	UserAgent             string              `json:"user_agent,omitempty"`
	EnableMetrics         bool                `json:"enable_metrics,omitempty"`
	EnableConnectionTrace bool                `json:"enable_connection_trace,omitempty"` // Opt-in connection-level tracing
	RequestInterceptor    RequestInterceptor  `json:"-"`
	ResponseInterceptor   ResponseInterceptor `json:"-"`
	// Transport configuration
	MaxIdleConns          int           `json:"max_idle_conns,omitempty"`
	MaxIdleConnsPerHost   int           `json:"max_idle_conns_per_host,omitempty"`
	MaxConnsPerHost       int           `json:"max_conns_per_host,omitempty"`
	IdleConnTimeout       time.Duration `json:"idle_conn_timeout,omitempty"`
	TLSHandshakeTimeout   time.Duration `json:"tls_handshake_timeout,omitempty"`
	ExpectContinueTimeout time.Duration `json:"expect_continue_timeout,omitempty"`
	ResponseHeaderTimeout time.Duration `json:"response_header_timeout,omitempty"`
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
	// Connection-level metrics (aggregated across all requests)
	ConnectionMetricsSummary *ConnectionMetricsSummary `json:"connection_metrics_summary,omitempty"`
}

// ConnectionMetrics holds detailed connection timing for a single request
type ConnectionMetrics struct {
	// DNS lookup duration
	DNSLookupDuration time.Duration `json:"dns_lookup_duration"`
	// TCP connect duration
	TCPConnectDuration time.Duration `json:"tcp_connect_duration"`
	// TLS handshake duration (for HTTPS)
	TLSHandshakeDuration time.Duration `json:"tls_handshake_duration"`
	// Time to first byte (TTFB)
	TimeToFirstByte time.Duration `json:"time_to_first_byte"`
	// Total connection time
	TotalConnectionTime time.Duration `json:"total_connection_time"`
}

// ConnectionMetricsSummary aggregates connection metrics across multiple requests
type ConnectionMetricsSummary struct {
	// Number of requests with connection metrics
	TotalMeasurements int64 `json:"total_measurements"`
	// Average DNS lookup duration
	AvgDNSLookupDuration time.Duration `json:"avg_dns_lookup_duration"`
	// Average TCP connect duration
	AvgTCPConnectDuration time.Duration `json:"avg_tcp_connect_duration"`
	// Average TLS handshake duration
	AvgTLSHandshakeDuration time.Duration `json:"avg_tls_handshake_duration"`
	// Average TTFB
	AvgTimeToFirstByte time.Duration `json:"avg_time_to_first_byte"`
	// Average total connection time
	AvgTotalConnectionTime time.Duration `json:"avg_total_connection_time"`
	// Min/Max for each metric
	MinDNSLookupDuration    time.Duration `json:"min_dns_lookup_duration"`
	MaxDNSLookupDuration    time.Duration `json:"max_dns_lookup_duration"`
	MinTCPConnectDuration   time.Duration `json:"min_tcp_connect_duration"`
	MaxTCPConnectDuration   time.Duration `json:"max_tcp_connect_duration"`
	MinTLSHandshakeDuration time.Duration `json:"min_tls_handshake_duration"`
	MaxTLSHandshakeDuration time.Duration `json:"max_tls_handshake_duration"`
	MinTimeToFirstByte      time.Duration `json:"min_time_to_first_byte"`
	MaxTimeToFirstByte      time.Duration `json:"max_time_to_first_byte"`
	MinTotalConnectionTime  time.Duration `json:"min_total_connection_time"`
	MaxTotalConnectionTime  time.Duration `json:"max_total_connection_time"`
}

// connectionTrace holds per-request timing data for httptrace callbacks
type connectionTrace struct {
	mu                   sync.Mutex
	dnsStart             time.Time
	dnsDone              time.Time
	connectStart         time.Time
	connectDone          time.Time
	tlsHandshakeStart    time.Time
	tlsHandshakeDone     time.Time
	gotConn              time.Time
	gotFirstResponseByte time.Time
	requestStartTime     time.Time
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

	// Set transport defaults
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 100
	}
	if config.MaxIdleConnsPerHost == 0 {
		config.MaxIdleConnsPerHost = 10
	}
	if config.MaxConnsPerHost == 0 {
		config.MaxConnsPerHost = 0 // 0 means unlimited
	}
	if config.IdleConnTimeout == 0 {
		config.IdleConnTimeout = 90 * time.Second
	}
	if config.TLSHandshakeTimeout == 0 {
		config.TLSHandshakeTimeout = 10 * time.Second
	}
	if config.ExpectContinueTimeout == 0 {
		config.ExpectContinueTimeout = 1 * time.Second
	}
	if config.ResponseHeaderTimeout == 0 {
		config.ResponseHeaderTimeout = 15 * time.Second
	}

	// Default retryable HTTP status codes
	if len(config.RetryableErrors) == 0 {
		config.RetryableErrors = []string{"429", "500", "502", "503", "504"}
	}

	// Create custom transport with connection pooling settings
	transport := createTransport(config)

	client := &HTTPClient{
		client: &http.Client{
			Timeout:   config.Timeout,
			Transport: transport,
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

// createTransport creates an http.Transport with the specified configuration
func createTransport(config HTTPClientConfig) *http.Transport {
	return &http.Transport{
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		// Use http.DefaultTransport settings for other fields
		ForceAttemptHTTP2: true,
		// Additional sensible defaults from http.DefaultTransport
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		DisableKeepAlives:     false,
		DisableCompression:    true,
	}
}

// NewHighConcurrencyHTTPClient creates an HTTP client optimized for high-concurrency scenarios.
// It uses increased connection pool limits suitable for applications making many parallel requests.
func NewHighConcurrencyHTTPClient(config HTTPClientConfig) *HTTPClient {
	// Override transport settings for high concurrency
	config.MaxIdleConns = 500
	config.MaxIdleConnsPerHost = 100
	config.MaxConnsPerHost = 0 // unlimited
	config.IdleConnTimeout = 90 * time.Second
	config.TLSHandshakeTimeout = 10 * time.Second
	config.ExpectContinueTimeout = 1 * time.Second

	return NewHTTPClient(config)
}

// applyRequestInterceptor applies the request interceptor if configured.
func (c *HTTPClient) applyRequestInterceptor(req *http.Request) error {
	if c.config.RequestInterceptor != nil {
		if err := c.config.RequestInterceptor.Intercept(req); err != nil {
			return fmt.Errorf("request interceptor failed: %w", err)
		}
	}
	return nil
}

// captureRequestBody reads and closes the request body, returning the bytes.
func (c *HTTPClient) captureRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	_ = req.Body.Close() //nolint:errcheck // Best effort close
	return bodyBytes, nil
}

// waitForRetry waits for the retry delay or context cancellation.
// Returns an error if the context is cancelled.
func (c *HTTPClient) waitForRetry(ctx context.Context, attempts int) error {
	delay := c.retryHandler.calculateDelay(attempts)
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// applyResponseInterceptor applies the response interceptor if configured.
// Returns an error if the interceptor fails.
func (c *HTTPClient) applyResponseInterceptor(resp *http.Response) error {
	if c.config.ResponseInterceptor != nil {
		if interceptErr := c.config.ResponseInterceptor.Intercept(resp); interceptErr != nil {
			return fmt.Errorf("response interceptor failed: %w", interceptErr)
		}
	}
	return nil
}

// Do executes an HTTP request with retry logic and metrics
//
//nolint:gocyclo // Complexity is acceptable for the main request orchestration method
func (c *HTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	startTime := time.Now()
	atomic.AddInt64(&c.requestCount, 1)

	// Apply request interceptor
	if err := c.applyRequestInterceptor(req); err != nil {
		return nil, err
	}

	// Set default headers
	for key, value := range c.config.Headers {
		req.Header.Set(key, value)
	}

	// Capture the request body once so we can reuse it on retries
	bodyBytes, err := c.captureRequestBody(req)
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	var attempts int

	// Set up connection trace if enabled
	var connTrace *connectionTrace
	var tracedReq *http.Request
	if c.config.EnableConnectionTrace {
		tracedReq, connTrace = withConnectionTrace(req)
	} else {
		tracedReq = req
	}

	for attempts = 0; attempts <= c.config.MaxRetries; attempts++ {
		if attempts > 0 {
			if waitErr := c.waitForRetry(ctx, attempts); waitErr != nil {
				return nil, waitErr
			}
			atomic.AddInt64(&c.metrics.RetryCount, 1)
		}

		// Create new request for retry with fresh body
		retryReq := c.cloneRequestWithBody(tracedReq, bodyBytes)
		retryReq = retryReq.WithContext(ctx)

		// Make the request
		resp, err = c.client.Do(retryReq)
		if err != nil {
			if c.shouldRetryError(err, attempts) {
				continue
			}
			break
		}

		// Mark response received time for TTFB calculation
		if c.config.EnableConnectionTrace && connTrace != nil {
			connTrace.markResponseReceived()
		}

		// Apply response interceptor
		if interceptErr := c.applyResponseInterceptor(resp); interceptErr != nil {
			_ = resp.Body.Close() //nolint:errcheck // Best effort close
			return nil, interceptErr
		}

		// Check if we should retry based on status code
		if c.shouldRetryStatus(resp.StatusCode, attempts) {
			_ = resp.Body.Close() //nolint:errcheck // Best effort close
			continue
		}

		// Success! Collect connection metrics if tracing is enabled
		if c.config.EnableConnectionTrace && connTrace != nil && resp != nil {
			connMetrics := connTrace.toConnectionMetrics()
			c.updateConnectionMetricsSummary(connMetrics)
		}
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

// cloneRequest creates a copy of the request for retry (deprecated, use cloneRequestWithBody)
func (c *HTTPClient) cloneRequest(orig *http.Request) *http.Request {
	// This is a simplified clone - in production you'd want to handle body copying properly
	cloned := orig.Clone(orig.Context())
	return cloned
}

// cloneRequestWithBody creates a copy of the request with a fresh body for retry
func (c *HTTPClient) cloneRequestWithBody(orig *http.Request, bodyBytes []byte) *http.Request {
	cloned := orig.Clone(orig.Context())
	if len(bodyBytes) > 0 {
		cloned.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		cloned.ContentLength = int64(len(bodyBytes))
		// GetBody allows the http.Client to replay the request body if needed
		cloned.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	}
	return cloned
}

// shouldRetryError determines if an error should trigger a retry
func (c *HTTPClient) shouldRetryError(_ error, attempts int) bool {
	// Check for retryable error types
	// This could be extended with more sophisticated error detection
	return attempts < c.config.MaxRetries
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

// PreWarmConnections establishes connections to the given targets before actual requests
func (c *HTTPClient) PreWarmConnections(ctx context.Context, targets []string, count int) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for _, target := range targets {
		for i := 0; i < count; i++ {
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				// Make a lightweight HEAD request to establish connection
				req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
				if err != nil {
					return
				}

				resp, err := c.client.Do(req)
				if err == nil {
					_ = resp.Body.Close()
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}(target)
		}
	}

	wg.Wait()
	return nil
}

// calculateDelay calculates retry delay with exponential backoff
// Delegates to the shared CalculateBackoff function for backward compatibility
func (r *RetryHandler) calculateDelay(attempt int) time.Duration {
	config := BackoffConfig{
		BaseDelay:   r.config.BaseRetryDelay,
		MaxDelay:    r.config.MaxRetryDelay,
		Multiplier:  r.config.BackoffMultiplier,
		MaxAttempts: r.config.MaxRetries,
	}
	return CalculateBackoff(config, attempt)
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

// WithTransportConfig sets transport configuration for connection pooling
func (b *HTTPClientBuilder) WithTransportConfig(maxIdleConns, maxIdleConnsPerHost, maxConnsPerHost int) *HTTPClientBuilder {
	b.config.MaxIdleConns = maxIdleConns
	b.config.MaxIdleConnsPerHost = maxIdleConnsPerHost
	b.config.MaxConnsPerHost = maxConnsPerHost
	return b
}

// WithIdleConnTimeout sets the idle connection timeout
func (b *HTTPClientBuilder) WithIdleConnTimeout(timeout time.Duration) *HTTPClientBuilder {
	b.config.IdleConnTimeout = timeout
	return b
}

// WithTLSHandshakeTimeout sets the TLS handshake timeout
func (b *HTTPClientBuilder) WithTLSHandshakeTimeout(timeout time.Duration) *HTTPClientBuilder {
	b.config.TLSHandshakeTimeout = timeout
	return b
}

// WithExpectContinueTimeout sets the expect continue timeout
func (b *HTTPClientBuilder) WithExpectContinueTimeout(timeout time.Duration) *HTTPClientBuilder {
	b.config.ExpectContinueTimeout = timeout
	return b
}

// WithResponseHeaderTimeout sets the response header timeout
func (b *HTTPClientBuilder) WithResponseHeaderTimeout(timeout time.Duration) *HTTPClientBuilder {
	b.config.ResponseHeaderTimeout = timeout
	return b
}

// Build creates the HTTP client
func (b *HTTPClientBuilder) Build() *HTTPClient {
	return NewHTTPClient(b.config)
}

// Client returns the underlying http.Client
func (c *HTTPClient) Client() *http.Client {
	return c.client
}

// LastRequestInfo represents information about the last request for error handling.
// This can be used to enrich errors with request/response context.
type LastRequestInfo struct {
	Request      *http.Request
	Response     *http.Response
	Attempts     int
	TotalLatency time.Duration
	Error        error
}

// GetLastRequestInfo returns info about the last request made.
func (c *HTTPClient) GetLastRequestInfo() LastRequestInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return LastRequestInfo{
		TotalLatency: c.metrics.AvgLatency,
	}
}
