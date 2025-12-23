// Package http provides HTTP client utilities and helpers for AI providers.
package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple URL",
			input:    "https://api.anthropic.com",
			expected: "https://api.anthropic.com",
		},
		{
			name:     "URL with trailing slash",
			input:    "https://api.anthropic.com/",
			expected: "https://api.anthropic.com",
		},
		{
			name:     "URL with path",
			input:    "https://api.anthropic.com/v1/messages",
			expected: "https://api.anthropic.com",
		},
		{
			name:     "URL with port",
			input:    "https://api.anthropic.com:443",
			expected: "https://api.anthropic.com:443",
		},
		{
			name:     "http URL",
			input:    "http://localhost:8080",
			expected: "http://localhost:8080",
		},
		{
			name:     "http with path",
			input:    "http://localhost:8080/v1/messages",
			expected: "http://localhost:8080",
		},
		{
			name:     "http with trailing slash",
			input:    "http://localhost:8080/",
			expected: "http://localhost:8080",
		},
		{
			name:     "invalid URL returns as-is",
			input:    "not-a-url",
			expected: "not-a-url",
		},
		{
			name:     "empty URL returns empty",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeBaseURL(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeBaseURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetClient(t *testing.T) {
	// Clear the pool before testing
	Clear()

	// Test getting a client for a base URL
	client1 := GetClient("https://api.anthropic.com")
	if client1 == nil {
		t.Fatal("GetClient returned nil")
	}

	// Test that getting the same URL returns the same client
	client2 := GetClient("https://api.anthropic.com")
	if client1 != client2 {
		t.Error("GetClient returned different clients for the same URL")
	}

	// Test that URLs with different paths return the same client
	client3 := GetClient("https://api.anthropic.com/v1/messages")
	if client1 != client3 {
		t.Error("GetClient returned different clients for same host with different paths")
	}

	// Test that URLs with trailing slash return the same client
	client4 := GetClient("https://api.anthropic.com/")
	if client1 != client4 {
		t.Error("GetClient returned different clients for same host with trailing slash")
	}

	// Test that different hosts return different clients
	client5 := GetClient("https://api.openai.com")
	if client1 == client5 {
		t.Error("GetClient returned same client for different hosts")
	}

	// Test pool size
	if Size() != 2 {
		t.Errorf("Size() = %d, want 2", Size())
	}

	// Clean up
	Clear()
}

func TestGetClientWithConfig(t *testing.T) {
	// Clear the pool before testing
	Clear()

	// Test getting a client with custom config
	config := HTTPClientConfig{
		Timeout:             30 * time.Second,
		MaxRetries:          5,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 5,
	}
	client1 := GetClientWithConfig("https://api.anthropic.com", config)
	if client1 == nil {
		t.Fatal("GetClientWithConfig returned nil")
	}

	// Test that getting the same URL again returns the same client
	// (ignoring the new config)
	newConfig := HTTPClientConfig{
		Timeout: 120 * time.Second,
	}
	client2 := GetClientWithConfig("https://api.anthropic.com", newConfig)
	if client1 != client2 {
		t.Error("GetClientWithConfig returned different clients for the same URL")
	}

	// Test that URL with path returns the same client
	client3 := GetClientWithConfig("https://api.anthropic.com/v1", config)
	if client1 != client3 {
		t.Error("GetClientWithConfig returned different clients for same host with path")
	}

	// Clean up
	Clear()
}

func TestGetClientWithTimeout(t *testing.T) {
	// Clear the pool before testing
	Clear()

	// Test getting a client with custom timeout
	client1 := GetClientWithTimeout("https://api.anthropic.com", 30*time.Second)
	if client1 == nil {
		t.Fatal("GetClientWithTimeout returned nil")
	}

	// Test that getting the same URL with different timeout returns the same client
	client2 := GetClientWithTimeout("https://api.anthropic.com", 60*time.Second)
	if client1 != client2 {
		t.Error("GetClientWithTimeout returned different clients for the same URL")
	}

	// Clean up
	Clear()
}

func TestGetClientForURL(t *testing.T) {
	// Clear the pool before testing
	Clear()

	// Test getting a client for a full URL
	client1 := GetClientForURL("https://api.anthropic.com/v1/messages")
	if client1 == nil {
		t.Fatal("GetClientForURL returned nil")
	}

	// Test that the client is keyed by base URL
	client2 := GetClient("https://api.anthropic.com")
	if client1 != client2 {
		t.Error("GetClientForURL and GetClient returned different clients for the same host")
	}

	// Test with URL that has query parameters
	client3 := GetClientForURL("https://api.anthropic.com/v1/messages?limit=100")
	if client1 != client3 {
		t.Error("GetClientForURL returned different clients for same host with query params")
	}

	// Clean up
	Clear()
}

func TestRemoveClient(t *testing.T) {
	// Clear the pool before testing
	Clear()

	// Add a client
	_ = GetClient("https://api.anthropic.com")
	if Size() != 1 {
		t.Fatalf("Size() = %d, want 1", Size())
	}

	// Remove the client
	RemoveClient("https://api.anthropic.com")
	if Size() != 0 {
		t.Errorf("Size() after Remove = %d, want 0", Size())
	}

	// Test that removing with path also works
	_ = GetClient("https://api.openai.com")
	RemoveClient("https://api.openai.com/v1")
	if Size() != 0 {
		t.Errorf("Size() after Remove with path = %d, want 0", Size())
	}

	// Clean up
	Clear()
}

func TestClear(t *testing.T) {
	// Add some clients
	_ = GetClient("https://api.anthropic.com")
	_ = GetClient("https://api.openai.com")
	_ = GetClient("https://api.gemini.com")

	if Size() != 3 {
		t.Fatalf("Size() before Clear = %d, want 3", Size())
	}

	// Clear the pool
	Clear()

	if Size() != 0 {
		t.Errorf("Size() after Clear = %d, want 0", Size())
	}

	// Verify that new clients are created after clear
	client1 := GetClient("https://api.anthropic.com")
	client2 := GetClient("https://api.anthropic.com")
	if client1 != client2 {
		t.Error("Clients don't match after Clear")
	}

	// Clean up
	Clear()
}

func TestCloseIdleConnections(t *testing.T) {
	// Clear the pool before testing
	Clear()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Add a client and make a request to establish connections
	client := GetClient(server.URL)
	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	_, _ = client.Do(ctx, req)

	// Close idle connections - should not panic
	CloseIdleConnections()

	// Clean up
	Clear()
}

func TestShutdown(t *testing.T) {
	// Clear the pool before testing
	Clear()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Add clients and make requests
	client1 := GetClient(server.URL)
	_ = GetClient("https://api.anthropic.com")

	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	_, _ = client1.Do(ctx, req)

	if Size() != 2 {
		t.Fatalf("Size() before Shutdown = %d, want 2", Size())
	}

	// Shutdown - should not panic
	err := Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}

	if Size() != 0 {
		t.Errorf("Size() after Shutdown = %d, want 0", Size())
	}
}

func TestGetStats(t *testing.T) {
	// Clear the pool before testing
	Clear()

	// Get stats for empty pool
	stats := GetStats()
	if stats.ClientCount != 0 {
		t.Errorf("ClientCount = %d, want 0", stats.ClientCount)
	}
	if len(stats.Clients) != 0 {
		t.Errorf("Clients length = %d, want 0", len(stats.Clients))
	}

	// Add some clients
	_ = GetClient("https://api.anthropic.com")
	_ = GetClient("https://api.openai.com")

	// Get stats
	stats = GetStats()
	if stats.ClientCount != 2 {
		t.Errorf("ClientCount = %d, want 2", stats.ClientCount)
	}
	if len(stats.Clients) != 2 {
		t.Errorf("Clients length = %d, want 2", len(stats.Clients))
	}

	// Verify client entries
	foundAnthropic := false
	foundOpenAI := false
	for _, client := range stats.Clients {
		if client.BaseURL == "https://api.anthropic.com" {
			foundAnthropic = true
		}
		if client.BaseURL == "https://api.openai.com" {
			foundOpenAI = true
		}
	}
	if !foundAnthropic {
		t.Error("Anthropic client not found in stats")
	}
	if !foundOpenAI {
		t.Error("OpenAI client not found in stats")
	}

	// Clean up
	Clear()
}

func TestConcurrency(t *testing.T) {
	// Clear the pool before testing
	Clear()

	// Test concurrent access
	const numGoroutines = 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(url string) {
			_ = GetClient(url)
			done <- true
		}("https://api.anthropic.com")
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Should only have one client
	if Size() != 1 {
		t.Errorf("Size() = %d, want 1", Size())
	}

	// Clean up
	Clear()
}

func TestIntegrationWithMakeRequest(t *testing.T) {
	// Clear the pool before testing
	Clear()

	// Create a test server
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"response":"ok"}`))
	}))
	defer server.Close()

	// Get a client from the pool
	client := GetClient(server.URL)

	// Make multiple requests using the same client
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		resp, err := client.Do(ctx, req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		resp.Body.Close()
	}

	if requestCount != 3 {
		t.Errorf("Request count = %d, want 3", requestCount)
	}

	// Clean up
	Clear()
}
