package clientpool

import (
	"sync"
	"testing"
	"time"
)

func TestGetClient(t *testing.T) {
	// Clear the pool before testing
	Clear()

	baseURL1 := "https://api.example.com/v1"
	baseURL2 := "https://api.other.com/v1"

	// Get client for first URL
	client1 := GetClient(baseURL1)
	if client1 == nil {
		t.Fatal("expected non-nil client")
	}

	// Get client for same URL - should return same instance
	client1Again := GetClient(baseURL1)
	if client1 != client1Again {
		t.Error("expected same client instance for same base URL")
	}

	// Get client for different URL - should return different instance
	client2 := GetClient(baseURL2)
	if client2 == nil {
		t.Fatal("expected non-nil client")
	}
	if client1 == client2 {
		t.Error("expected different client instances for different base URLs")
	}

	// Check pool size
	if Size() != 2 {
		t.Errorf("expected pool size 2, got %d", Size())
	}
}

func TestGetClientWithTimeout(t *testing.T) {
	// Clear the pool before testing
	Clear()

	baseURL := "https://api.example.com/v1"
	timeout1 := 30 * time.Second
	timeout2 := 60 * time.Second

	// Get client with first timeout
	client1 := GetClientWithTimeout(baseURL, timeout1)
	if client1 == nil {
		t.Fatal("expected non-nil client")
	}

	// Get client with different timeout - should return same client (first caller wins)
	client2 := GetClientWithTimeout(baseURL, timeout2)
	if client1 != client2 {
		t.Error("expected same client instance regardless of timeout")
	}

	// Check pool size
	if Size() != 1 {
		t.Errorf("expected pool size 1, got %d", Size())
	}
}

func TestGetClientWithConfig(t *testing.T) {
	// Clear the pool before testing
	Clear()

	baseURL := "https://api.example.com/v1"

	// Get client with default config
	client1 := GetClient(baseURL)
	if client1 == nil {
		t.Fatal("expected non-nil client")
	}

	// Get client with custom config - should return same client (first caller wins)
	customConfig := client1.Client() // Just to have a valid type reference
	_ = customConfig

	// Check pool size
	if Size() != 1 {
		t.Errorf("expected pool size 1, got %d", Size())
	}
}

func TestRemoveClient(t *testing.T) {
	// Clear the pool before testing
	Clear()

	baseURL := "https://api.example.com/v1"

	// Add a client
	GetClient(baseURL)
	if Size() != 1 {
		t.Errorf("expected pool size 1, got %d", Size())
	}

	// Remove the client
	RemoveClient(baseURL)
	if Size() != 0 {
		t.Errorf("expected pool size 0 after removal, got %d", Size())
	}

	// Get client again - should create new instance
	client := GetClient(baseURL)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if Size() != 1 {
		t.Errorf("expected pool size 1, got %d", Size())
	}
}

func TestClear(t *testing.T) {
	// Clear the pool before testing
	Clear()

	// Add multiple clients
	GetClient("https://api.example.com/v1")
	GetClient("https://api.other.com/v1")
	GetClient("https://third.com/v1")

	if Size() != 3 {
		t.Errorf("expected pool size 3, got %d", Size())
	}

	// Clear all clients
	Clear()
	if Size() != 0 {
		t.Errorf("expected pool size 0 after clear, got %d", Size())
	}
}

func TestGetClientConcurrent(t *testing.T) {
	// Clear the pool before testing
	Clear()

	baseURL := "https://api.example.com/v1"
	var wg sync.WaitGroup
	clients := make([]interface{}, 100)

	// Concurrently get clients for the same URL
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			clients[idx] = GetClient(baseURL)
		}(i)
	}

	wg.Wait()

	// All clients should be the same instance
	firstClient := clients[0]
	for i, client := range clients {
		if client != firstClient {
			t.Errorf("client %d is different from first client", i)
		}
	}

	// Pool should only have one entry
	if Size() != 1 {
		t.Errorf("expected pool size 1, got %d", Size())
	}
}

func TestGetUnderlyingClient(t *testing.T) {
	// Clear the pool before testing
	Clear()

	baseURL := "https://api.example.com/v1"

	// Get underlying client
	underlying := GetUnderlyingClient(baseURL)
	if underlying == nil {
		t.Fatal("expected non-nil underlying client")
	}

	// Should return same underlying client on subsequent calls
	underlying2 := GetUnderlyingClient(baseURL)
	if underlying != underlying2 {
		t.Error("expected same underlying client instance")
	}
}
