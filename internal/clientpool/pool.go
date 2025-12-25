// Package clientpool provides a shared HTTP client pool keyed by base URL
// This enables HTTP/2 connection sharing across providers targeting the same base URL
package clientpool

import (
	"net/http"
	"sync"
	"time"

	pkghttp "github.com/cecil-the-coder/ai-provider-kit/internal/http"
)

// ClientPool manages shared HTTP clients keyed by base URL
// This enables HTTP/2 connection sharing across providers targeting the same base URL
type ClientPool struct {
	clients map[string]*pkghttp.HTTPClient
	mu      sync.RWMutex
}

var globalPool = &ClientPool{
	clients: make(map[string]*pkghttp.HTTPClient),
}

// GetClient returns an HTTP client for the given base URL, creating if needed
// This ensures that providers targeting the same base URL share the same HTTP client,
// enabling HTTP/2 connection pooling and reducing resource usage
func GetClient(baseURL string) *pkghttp.HTTPClient {
	globalPool.mu.RLock()
	if client, ok := globalPool.clients[baseURL]; ok {
		globalPool.mu.RUnlock()
		return client
	}
	globalPool.mu.RUnlock()

	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()

	// Double-check in case another goroutine created it while we waited
	if client, ok := globalPool.clients[baseURL]; ok {
		return client
	}

	// Create new HTTP client with default configuration
	config := pkghttp.HTTPClientConfig{
		Timeout: 60 * time.Second,
		// Default connection pooling settings
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       0, // unlimited
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	client := pkghttp.NewHTTPClient(config)
	globalPool.clients[baseURL] = client
	return client
}

// GetClientWithConfig returns an HTTP client for the given base URL with custom config,
// creating if needed. If a client already exists for the base URL, it will be returned
// and the custom config will not be applied (first caller wins)
func GetClientWithConfig(baseURL string, config pkghttp.HTTPClientConfig) *pkghttp.HTTPClient {
	globalPool.mu.RLock()
	if client, ok := globalPool.clients[baseURL]; ok {
		globalPool.mu.RUnlock()
		return client
	}
	globalPool.mu.RUnlock()

	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()

	// Double-check in case another goroutine created it while we waited
	if client, ok := globalPool.clients[baseURL]; ok {
		return client
	}

	// Set default timeout if not specified
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}

	// Set default connection pooling if not specified
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
	if config.ResponseHeaderTimeout == 0 {
		config.ResponseHeaderTimeout = 15 * time.Second
	}

	client := pkghttp.NewHTTPClient(config)
	globalPool.clients[baseURL] = client
	return client
}

// GetClientWithTimeout returns an HTTP client for the given base URL with a specific timeout
// If a client already exists for the base URL, it will be returned regardless of timeout
func GetClientWithTimeout(baseURL string, timeout time.Duration) *pkghttp.HTTPClient {
	return GetClientWithConfig(baseURL, pkghttp.HTTPClientConfig{
		Timeout: timeout,
	})
}

// Clear removes all cached clients from the pool
// This is primarily useful for testing or when you need to reset connection state
func Clear() {
	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()
	globalPool.clients = make(map[string]*pkghttp.HTTPClient)
}

// RemoveClient removes a specific client from the pool
func RemoveClient(baseURL string) {
	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()
	delete(globalPool.clients, baseURL)
}

// Size returns the number of clients in the pool
func Size() int {
	globalPool.mu.RLock()
	defer globalPool.mu.RUnlock()
	return len(globalPool.clients)
}

// GetUnderlyingClient returns the underlying http.Client from a pooled HTTPClient
// This is provided for compatibility with code that needs the standard library client
func GetUnderlyingClient(baseURL string) *http.Client {
	return GetClient(baseURL).Client()
}
