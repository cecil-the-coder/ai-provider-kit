package backend

import (
	"net/http"
	"time"
)

// SharedTransport is a reusable HTTP transport with optimized connection pooling
// settings for backend operations. It reduces allocation overhead and improves
// performance by reusing TCP connections across requests.
var SharedTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// SharedClient is a reusable HTTP client that uses the SharedTransport.
// This client should be used by all backend components that need to make
// HTTP requests to reduce memory allocations and improve connection reuse.
var SharedClient = &http.Client{
	Transport: SharedTransport,
	Timeout:   30 * time.Second,
}
