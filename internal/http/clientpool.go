// Package http provides HTTP client utilities and helpers for AI providers.
// It includes reusable HTTP clients with retry logic, metrics, and interceptors.
package http

import (
	"context"
	"net/url"
	"sync"
	"time"
)

// ClientPool manages shared HTTP clients keyed by base URL.
// This enables HTTP/2 connection sharing across providers targeting the same host,
// reducing TLS handshakes and improving resource utilization.
type ClientPool struct {
	clients map[string]*HTTPClient
	mu      sync.RWMutex
}

// globalPool is the singleton client pool instance
var globalPool = &ClientPool{
	clients: make(map[string]*HTTPClient),
}

// GetClient returns an HTTP client for the given base URL, creating if needed.
// This ensures that providers targeting the same base URL share the same HTTP client,
// enabling HTTP/2 connection pooling and reducing resource usage.
func GetClient(baseURL string) *HTTPClient {
	// Normalize the base URL to use as a key
	normalizedKey := normalizeBaseURL(baseURL)

	globalPool.mu.RLock()
	if client, ok := globalPool.clients[normalizedKey]; ok {
		globalPool.mu.RUnlock()
		return client
	}
	globalPool.mu.RUnlock()

	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()

	// Double-check in case another goroutine created it while we waited
	if client, ok := globalPool.clients[normalizedKey]; ok {
		return client
	}

	// Create new HTTP client with default configuration optimized for HTTP/2
	config := HTTPClientConfig{
		Timeout: 60 * time.Second,
		// Connection pooling settings optimized for HTTP/2 multiplexing
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       0, // unlimited - HTTP/2 can multiplex many requests on one connection
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		// Retry settings
		MaxRetries:        3,
		BaseRetryDelay:    time.Second,
		MaxRetryDelay:     60 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors:   []string{"429", "500", "502", "503", "504"},
	}

	client := NewHTTPClient(config)
	globalPool.clients[normalizedKey] = client
	return client
}

// GetClientWithConfig returns an HTTP client for the given base URL with custom config.
// If a client already exists for the base URL, the existing client is returned
// and the custom config is not applied (first caller wins).
// Use this when you need specific timeout or retry behavior.
func GetClientWithConfig(baseURL string, config HTTPClientConfig) *HTTPClient {
	normalizedKey := normalizeBaseURL(baseURL)

	globalPool.mu.RLock()
	if client, ok := globalPool.clients[normalizedKey]; ok {
		globalPool.mu.RUnlock()
		return client
	}
	globalPool.mu.RUnlock()

	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()

	// Double-check in case another goroutine created it while we waited
	if client, ok := globalPool.clients[normalizedKey]; ok {
		return client
	}

	// Set defaults if not specified
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 100
	}
	if config.MaxIdleConnsPerHost == 0 {
		config.MaxIdleConnsPerHost = 10
	}
	if config.MaxConnsPerHost == 0 {
		config.MaxConnsPerHost = 0 // unlimited
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
	if len(config.RetryableErrors) == 0 {
		config.RetryableErrors = []string{"429", "500", "502", "503", "504"}
	}

	client := NewHTTPClient(config)
	globalPool.clients[normalizedKey] = client
	return client
}

// GetClientWithTimeout returns an HTTP client for the given base URL with a specific timeout.
// If a client already exists for the base URL, it will be returned regardless of timeout.
// This is a convenience wrapper around GetClientWithConfig.
func GetClientWithTimeout(baseURL string, timeout time.Duration) *HTTPClient {
	return GetClientWithConfig(baseURL, HTTPClientConfig{
		Timeout: timeout,
	})
}

// Shutdown gracefully closes all connections in the pool.
// This should be called when shutting down the application to ensure
// all connections are properly closed.
func Shutdown(ctx context.Context) error {
	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()

	// Close all idle connections
	for _, client := range globalPool.clients {
		if client != nil && client.client != nil {
			client.client.CloseIdleConnections()
		}
	}

	// Clear the pool
	globalPool.clients = make(map[string]*HTTPClient)
	return nil
}

// Clear removes all cached clients from the pool.
// This is primarily useful for testing or when you need to reset connection state.
// Note: This does not close existing connections, just removes the references.
func Clear() {
	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()
	globalPool.clients = make(map[string]*HTTPClient)
}

// RemoveClient removes a specific client from the pool by base URL.
// Note: This does not close the client's connections, just removes the reference.
func RemoveClient(baseURL string) {
	normalizedKey := normalizeBaseURL(baseURL)
	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()
	delete(globalPool.clients, normalizedKey)
}

// Size returns the number of clients currently in the pool.
func Size() int {
	globalPool.mu.RLock()
	defer globalPool.mu.RUnlock()
	return len(globalPool.clients)
}

// GetClientForURL extracts the base URL from a full URL and returns the appropriate client.
// This is a convenience function that automatically extracts the scheme and host.
func GetClientForURL(fullURL string) *HTTPClient {
	u, err := url.Parse(fullURL)
	if err != nil {
		// If parsing fails, return the default client with the full URL as key
		return GetClient(fullURL)
	}

	// Reconstruct base URL with scheme and host
	baseURL := u.Scheme + "://" + u.Host
	return GetClient(baseURL)
}

// normalizeBaseURL normalizes a base URL for use as a map key.
// It extracts the scheme and host, removing any path, query, or fragment.
func normalizeBaseURL(baseURL string) string {
	if baseURL == "" {
		return ""
	}
	// Parse the URL to extract scheme and host
	u, err := url.Parse(baseURL)
	if err != nil {
		// If parsing fails, return the URL as-is
		return baseURL
	}

	// Return scheme://host as the normalized key
	// This ensures that https://api.anthropic.com and https://api.anthropic.com/
	// map to the same client
	normalized := u.Scheme + "://" + u.Host
	if normalized == "://" {
		// If scheme or host is missing, return original
		return baseURL
	}
	return normalized
}

// GetPool returns the global client pool instance.
// This can be used for advanced operations or testing.
func GetPool() *ClientPool {
	return globalPool
}

// CloseIdleConnections closes all idle connections in all clients in the pool.
// This is useful for releasing resources without removing the clients from the pool.
func CloseIdleConnections() {
	globalPool.mu.RLock()
	defer globalPool.mu.RUnlock()

	for _, client := range globalPool.clients {
		if client != nil && client.client != nil {
			client.client.CloseIdleConnections()
		}
	}
}

// ClientStats returns statistics about the client pool.
type ClientStats struct {
	ClientCount int               `json:"client_count"`
	Clients     []ClientStatEntry `json:"clients,omitempty"`
}

// ClientStatEntry represents statistics for a single client in the pool.
type ClientStatEntry struct {
	BaseURL      string        `json:"base_url"`
	RequestCount int64         `json:"request_count"`
	SuccessCount int64         `json:"success_count"`
	ErrorCount   int64         `json:"error_count"`
	AvgLatency   time.Duration `json:"avg_latency"`
}

// GetStats returns statistics about all clients in the pool.
func GetStats() ClientStats {
	globalPool.mu.RLock()
	defer globalPool.mu.RUnlock()

	stats := ClientStats{
		ClientCount: len(globalPool.clients),
		Clients:     make([]ClientStatEntry, 0, len(globalPool.clients)),
	}

	for baseURL, client := range globalPool.clients {
		if client != nil {
			metrics := client.GetMetrics()
			stats.Clients = append(stats.Clients, ClientStatEntry{
				BaseURL:      baseURL,
				RequestCount: metrics.TotalRequests,
				SuccessCount: metrics.SuccessfulReqs,
				ErrorCount:   metrics.FailedReqs,
				AvgLatency:   metrics.AvgLatency,
			})
		}
	}

	return stats
}
