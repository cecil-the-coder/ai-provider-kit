// Package factory provides provider factory functionality for AI providers.
package factory

import (
	"context"
	"testing"
	"time"

	pkghttp "github.com/cecil-the-coder/ai-provider-kit/internal/http"
)

func TestFactoryGetHTTPClient(t *testing.T) {
	t.Parallel()
	// Clear the pool before testing
	pkghttp.Clear()

	factory := NewProviderFactory()

	// Test getting a client for a base URL
	client1 := factory.GetHTTPClient("https://api.anthropic.com")
	if client1 == nil {
		t.Fatal("GetHTTPClient returned nil")
	}

	// Test that getting the same URL returns the same client
	client2 := factory.GetHTTPClient("https://api.anthropic.com")
	if client1 != client2 {
		t.Error("GetHTTPClient returned different clients for the same URL")
	}

	// Test that different hosts return different clients
	client3 := factory.GetHTTPClient("https://api.openai.com")
	if client1 == client3 {
		t.Error("GetHTTPClient returned same client for different hosts")
	}

	// Clean up
	pkghttp.Clear()
}

func TestFactoryGetHTTPClientWithTimeout(t *testing.T) {
	t.Parallel()
	// Clear the pool before testing
	pkghttp.Clear()

	factory := NewProviderFactory()

	// Test getting a client with custom timeout
	client1 := factory.GetHTTPClientWithTimeout("https://api.anthropic.com", 30*time.Second)
	if client1 == nil {
		t.Fatal("GetHTTPClientWithTimeout returned nil")
	}

	// Test that getting the same URL with different timeout returns the same client
	client2 := factory.GetHTTPClientWithTimeout("https://api.anthropic.com", 60*time.Second)
	if client1 != client2 {
		t.Error("GetHTTPClientWithTimeout returned different clients for the same URL")
	}

	// Clean up
	pkghttp.Clear()
}

func TestFactoryGetHTTPClientWithConfig(t *testing.T) {
	t.Parallel()
	// Clear the pool before testing
	pkghttp.Clear()

	factory := NewProviderFactory()

	// Test getting a client with custom config
	config := pkghttp.HTTPClientConfig{
		Timeout:    30 * time.Second,
		MaxRetries: 5,
	}
	client1 := factory.GetHTTPClientWithConfig("https://api.anthropic.com", config)
	if client1 == nil {
		t.Fatal("GetHTTPClientWithConfig returned nil")
	}

	// Test that getting the same URL again returns the same client
	newConfig := pkghttp.HTTPClientConfig{
		Timeout: 120 * time.Second,
	}
	client2 := factory.GetHTTPClientWithConfig("https://api.anthropic.com", newConfig)
	if client1 != client2 {
		t.Error("GetHTTPClientWithConfig returned different clients for the same URL")
	}

	// Clean up
	pkghttp.Clear()
}

func TestFactoryShutdownHTTPClients(t *testing.T) {
	// Clear the pool before testing
	pkghttp.Clear()

	factory := NewProviderFactory()

	// Add some clients
	_ = factory.GetHTTPClient("https://api.anthropic.com")
	_ = factory.GetHTTPClient("https://api.openai.com")

	if pkghttp.Size() != 2 {
		t.Fatalf("Size() before Shutdown = %d, want 2", pkghttp.Size())
	}

	// Shutdown - should not panic
	err := factory.ShutdownHTTPClients(context.Background())
	if err != nil {
		t.Errorf("ShutdownHTTPClients() error = %v", err)
	}

	if pkghttp.Size() != 0 {
		t.Errorf("Size() after Shutdown = %d, want 0", pkghttp.Size())
	}
}

func TestFactoryGetHTTPClientStats(t *testing.T) {
	t.Parallel()
	// Clear the pool before testing
	pkghttp.Clear()

	factory := NewProviderFactory()

	// Get stats for empty pool
	stats := factory.GetHTTPClientStats()
	if stats.ClientCount != 0 {
		t.Errorf("ClientCount = %d, want 0", stats.ClientCount)
	}

	// Add some clients
	_ = factory.GetHTTPClient("https://api.anthropic.com")
	_ = factory.GetHTTPClient("https://api.openai.com")

	// Get stats
	stats = factory.GetHTTPClientStats()
	if stats.ClientCount != 2 {
		t.Errorf("ClientCount = %d, want 2", stats.ClientCount)
	}

	// Clean up
	pkghttp.Clear()
}

func TestFactoryIntegrationWithProviders(t *testing.T) {
	t.Parallel()
	// Clear the pool before testing
	pkghttp.Clear()

	factory := NewProviderFactory()
	RegisterDefaultProviders(factory)

	// Create providers - they should use the HTTP client pool internally
	anthropicConfig := getTestConfig("anthropic")
	openaiConfig := getTestConfig("openai")

	// Note: This test doesn't actually make API calls, it just verifies
	// that the factory can provide HTTP clients to the providers
	client1 := factory.GetHTTPClient(anthropicConfig.BaseURL)
	client2 := factory.GetHTTPClient(openaiConfig.BaseURL)

	if client1 == nil {
		t.Error("Failed to get HTTP client for Anthropic")
	}
	if client2 == nil {
		t.Error("Failed to get HTTP client for OpenAI")
	}

	// Clean up
	pkghttp.Clear()
}

func TestFactoryHTTPClientURLNormalization(t *testing.T) {
	t.Parallel()
	// Clear the pool before testing
	pkghttp.Clear()

	factory := NewProviderFactory()

	// Test that URLs with different paths return the same client
	client1 := factory.GetHTTPClient("https://api.anthropic.com")
	client2 := factory.GetHTTPClient("https://api.anthropic.com/")
	client3 := factory.GetHTTPClient("https://api.anthropic.com/v1/messages")

	if client1 != client2 {
		t.Error("Clients don't match for URL with trailing slash")
	}
	if client1 != client3 {
		t.Error("Clients don't match for URL with path")
	}

	// Note: Explicit ports like :443 are treated as different hosts
	// This is intentional since ports can matter for routing
	client4 := factory.GetHTTPClient("https://api.anthropic.com:443")
	if client1 == client4 {
		t.Error("Client with explicit port should be different")
	}

	// Clean up
	pkghttp.Clear()
}

// Helper function to get test config
func getTestConfig(provider string) struct {
	BaseURL string
	APIKey  string
} {
	switch provider {
	case "anthropic":
		return struct {
			BaseURL string
			APIKey  string
		}{
			BaseURL: "https://api.anthropic.com",
			APIKey:  "test-key",
		}
	case "openai":
		return struct {
			BaseURL string
			APIKey  string
		}{
			BaseURL: "https://api.openai.com",
			APIKey:  "test-key",
		}
	default:
		return struct {
			BaseURL string
			APIKey  string
		}{
			BaseURL: "https://api.example.com",
			APIKey:  "test-key",
		}
	}
}
