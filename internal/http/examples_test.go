package http_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	httputil "github.com/cecil-the-coder/ai-provider-kit/internal/http"
)

// Example_defaultClient demonstrates creating an HTTP client with default transport settings
func Example_defaultClient() {
	client := httputil.NewHTTPClient(httputil.HTTPClientConfig{
		Timeout: 30 * time.Second,
	})

	req, _ := http.NewRequest("GET", "https://api.example.com/data", nil)
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // Example code

	fmt.Printf("Status: %d\n", resp.StatusCode)
}

// Example_customTransportConfig demonstrates creating an HTTP client with custom connection pool settings
func Example_customTransportConfig() {
	client := httputil.NewHTTPClient(httputil.HTTPClientConfig{
		Timeout:             30 * time.Second,
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 20,
		MaxConnsPerHost:     50,
		IdleConnTimeout:     120 * time.Second,
		TLSHandshakeTimeout: 15 * time.Second,
	})

	req, _ := http.NewRequest("GET", "https://api.example.com/data", nil)
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // Example code

	fmt.Printf("Status: %d\n", resp.StatusCode)
}

// Example_highConcurrencyClient demonstrates creating a client optimized for high concurrency
func Example_highConcurrencyClient() {
	// High concurrency client with 500 max idle connections
	client := httputil.NewHighConcurrencyHTTPClient(httputil.HTTPClientConfig{
		Timeout:    10 * time.Second,
		MaxRetries: 3,
	})

	req, _ := http.NewRequest("GET", "https://api.example.com/data", nil)
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // Example code

	fmt.Printf("Status: %d\n", resp.StatusCode)
}

// Example_builderWithTransport demonstrates using the builder pattern with transport configuration
func Example_builderWithTransport() {
	client := httputil.NewHTTPClientBuilder().
		WithTimeout(30*time.Second).
		WithRetry(3, time.Second).
		WithTransportConfig(300, 30, 100).
		WithIdleConnTimeout(60 * time.Second).
		WithTLSHandshakeTimeout(5 * time.Second).
		Build()

	req, _ := http.NewRequest("GET", "https://api.example.com/data", nil)
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // Example code

	fmt.Printf("Status: %d\n", resp.StatusCode)
}
