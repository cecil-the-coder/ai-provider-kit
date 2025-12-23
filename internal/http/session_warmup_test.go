// Package http provides HTTP client utilities and helpers for AI providers.
// It includes reusable HTTP clients with retry logic, metrics, and interceptors.
package http

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewSessionWarmup(t *testing.T) {
	tests := []struct {
		name          string
		checkInterval time.Duration
		maxIdle       time.Duration
		wantInterval  time.Duration
		wantMaxIdle   time.Duration
	}{
		{
			name:          "custom intervals",
			checkInterval: 10 * time.Second,
			maxIdle:       20 * time.Second,
			wantInterval:  10 * time.Second,
			wantMaxIdle:   20 * time.Second,
		},
		{
			name:          "zero intervals use defaults",
			checkInterval: 0,
			maxIdle:       0,
			wantInterval:  30 * time.Second,
			wantMaxIdle:   30 * time.Second,
		},
		{
			name:          "partial zero intervals",
			checkInterval: 15 * time.Second,
			maxIdle:       0,
			wantInterval:  15 * time.Second,
			wantMaxIdle:   30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sw := NewSessionWarmup(tt.checkInterval, tt.maxIdle)
			if sw == nil {
				t.Fatal("NewSessionWarmup returned nil")
			}
			if sw.checkInterval != tt.wantInterval {
				t.Errorf("checkInterval = %v, want %v", sw.checkInterval, tt.wantInterval)
			}
			if sw.maxIdle != tt.wantMaxIdle {
				t.Errorf("maxIdle = %v, want %v", sw.maxIdle, tt.wantMaxIdle)
			}
			if sw.perHost == nil {
				t.Error("perHost map is nil")
			}
			if sw.client == nil {
				t.Error("client is nil")
			}
		})
	}
}

func TestMarkUsed(t *testing.T) {
	sw := NewSessionWarmup(30*time.Second, 30*time.Second)
	sw.Start()
	defer sw.Stop()

	baseURL := "https://api.anthropic.com"

	// Mark a URL as used
	sw.MarkUsed(baseURL)

	// Check that it was tracked
	lastSeen := sw.GetLastSeen(baseURL)
	if lastSeen.IsZero() {
		t.Error("GetLastSeen returned zero time after MarkUsed")
	}

	// Verify it's in active hosts
	activeHosts := sw.GetActiveHosts()
	if len(activeHosts) != 1 {
		t.Errorf("GetActiveHosts returned %d hosts, want 1", len(activeHosts))
	}
	if len(activeHosts) > 0 && activeHosts[0] != baseURL {
		t.Errorf("GetActiveHosts[0] = %s, want %s", activeHosts[0], baseURL)
	}
}

func TestMarkUsedMultiple(t *testing.T) {
	sw := NewSessionWarmup(30*time.Second, 30*time.Second)
	sw.Start()
	defer sw.Stop()

	urls := []string{
		"https://api.anthropic.com",
		"https://api.openai.com",
		"https://api.gemini.com",
	}

	// Mark multiple URLs as used
	for _, url := range urls {
		sw.MarkUsed(url)
	}

	// Verify all are tracked
	activeHosts := sw.GetActiveHosts()
	if len(activeHosts) != len(urls) {
		t.Errorf("GetActiveHosts returned %d hosts, want %d", len(activeHosts), len(urls))
	}

	// Verify each URL has a non-zero lastSeen
	for _, url := range urls {
		lastSeen := sw.GetLastSeen(url)
		if lastSeen.IsZero() {
			t.Errorf("GetLastSeen(%s) returned zero time", url)
		}
	}
}

func TestMarkUsedConcurrent(t *testing.T) {
	sw := NewSessionWarmup(30*time.Second, 30*time.Second)
	sw.Start()
	defer sw.Stop()

	baseURL := "https://api.anthropic.com"
	const numGoroutines = 100
	var wg sync.WaitGroup

	// Concurrently mark the same URL
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sw.MarkUsed(baseURL)
		}()
	}
	wg.Wait()

	// Should still have only one entry
	activeHosts := sw.GetActiveHosts()
	if len(activeHosts) != 1 {
		t.Errorf("GetActiveHosts returned %d hosts, want 1", len(activeHosts))
	}
}

func TestMarkUsedNormalization(t *testing.T) {
	sw := NewSessionWarmup(30*time.Second, 30*time.Second)
	sw.Start()
	defer sw.Stop()

	// Mark the same host with different paths/variants
	urls := []string{
		"https://api.anthropic.com",
		"https://api.anthropic.com/",
		"https://api.anthropic.com/v1/messages",
		"https://api.anthropic.com/v1/chat",
	}

	for _, url := range urls {
		sw.MarkUsed(url)
	}

	// All should map to the same normalized URL
	activeHosts := sw.GetActiveHosts()
	if len(activeHosts) != 1 {
		t.Errorf("GetActiveHosts returned %d hosts, want 1 (all variants should normalize)", len(activeHosts))
	}
}

func TestInactiveDetection(t *testing.T) {
	// Use a short maxIdle for testing
	sw := NewSessionWarmup(100*time.Millisecond, 100*time.Millisecond)
	sw.Start()
	defer sw.Stop()

	baseURL := "https://api.anthropic.com"
	sw.MarkUsed(baseURL)

	// Should be active immediately
	activeHosts := sw.GetActiveHosts()
	if len(activeHosts) != 1 {
		t.Errorf("GetActiveHosts returned %d hosts immediately after MarkUsed, want 1", len(activeHosts))
	}

	// Wait for host to become inactive
	time.Sleep(200 * time.Millisecond)

	// Send keep-alives to trigger cleanup
	sw.sendKeepAlives()

	// Should now be inactive (cleaned up)
	activeHosts = sw.GetActiveHosts()
	if len(activeHosts) != 0 {
		t.Errorf("GetActiveHosts returned %d hosts after maxIdle elapsed, want 0", len(activeHosts))
	}

	lastSeen := sw.GetLastSeen(baseURL)
	if !lastSeen.IsZero() {
		t.Error("GetLastSeen returned non-zero time after cleanup")
	}
}

func TestActiveAfterRecentMarkUsed(t *testing.T) {
	// Use a short maxIdle for testing
	sw := NewSessionWarmup(100*time.Millisecond, 100*time.Millisecond)
	sw.Start()
	defer sw.Stop()

	baseURL := "https://api.anthropic.com"
	sw.MarkUsed(baseURL)

	// Wait half the maxIdle time
	time.Sleep(50 * time.Millisecond)

	// Mark as used again - this should refresh the lastSeen time
	sw.MarkUsed(baseURL)

	// Wait another 40ms (total 90ms from first mark, but only 40ms from second)
	// This should still be within the maxIdle window from the second mark
	time.Sleep(40 * time.Millisecond)

	// Should still be active because we re-marked it recently
	activeHosts := sw.GetActiveHosts()
	if len(activeHosts) != 1 {
		t.Errorf("GetActiveHosts returned %d hosts after re-mark, want 1", len(activeHosts))
	}

	// Wait longer to exceed maxIdle
	time.Sleep(70 * time.Millisecond)

	// Send keep-alives to trigger cleanup check
	sw.sendKeepAlives()

	// Should now be inactive
	activeHosts = sw.GetActiveHosts()
	if len(activeHosts) != 0 {
		t.Errorf("GetActiveHosts returned %d hosts after maxIdle from second mark, want 0", len(activeHosts))
	}
}

func TestStartStop(t *testing.T) {
	sw := NewSessionWarmup(30*time.Second, 30*time.Second)

	// Should not be started initially
	if sw.ticker != nil {
		t.Error("ticker is non-nil before Start")
	}

	// Start the warmup
	sw.Start()

	// Should now be started
	if sw.ticker == nil {
		t.Error("ticker is nil after Start")
	}

	// Starting again should be idempotent
	ticker1 := sw.ticker
	sw.Start()
	if sw.ticker != ticker1 {
		t.Error("Start created a new ticker (should be idempotent)")
	}

	// Stop the warmup
	sw.Stop()

	// Ticker should be stopped
	if sw.ticker != nil {
		t.Error("ticker is non-nil after Stop")
	}

	// Stopping again should be safe
	sw.Stop() // Should not panic
}

func TestSendKeepAlives(t *testing.T) {
	// Create a test server
	requestCount := 0
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		// Return 404 for HEAD requests to a non-existent path
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	sw := NewSessionWarmup(30*time.Second, 30*time.Second)
	sw.Start()
	defer sw.Stop()

	// Mark the test server URL as used
	sw.MarkUsed(server.URL)

	// Trigger keep-alive manually
	sw.sendKeepAlives()

	// Give time for the request to complete
	time.Sleep(100 * time.Millisecond)

	// At least one HEAD request should have been made
	mu.Lock()
	count := requestCount
	mu.Unlock()

	if count == 0 {
		t.Error("No HEAD request was sent to the server")
	}
}

func TestSessionWarmupClear(t *testing.T) {
	sw := NewSessionWarmup(30*time.Second, 30*time.Second)
	sw.Start()
	defer sw.Stop()

	// Add some hosts
	urls := []string{
		"https://api.anthropic.com",
		"https://api.openai.com",
		"https://api.gemini.com",
	}

	for _, url := range urls {
		sw.MarkUsed(url)
	}

	if len(sw.GetActiveHosts()) != len(urls) {
		t.Fatalf("GetActiveHosts returned %d hosts before Clear, want %d", len(sw.GetActiveHosts()), len(urls))
	}

	// Clear all hosts
	sw.Clear()

	// Should have no hosts
	if len(sw.GetActiveHosts()) != 0 {
		t.Errorf("GetActiveHosts returned %d hosts after Clear, want 0", len(sw.GetActiveHosts()))
	}

	// LastSeen should be zero for all
	for _, url := range urls {
		lastSeen := sw.GetLastSeen(url)
		if !lastSeen.IsZero() {
			t.Errorf("GetLastSeen(%s) returned non-zero time after Clear", url)
		}
	}
}

func TestNilSessionWarmup(t *testing.T) {
	var sw *SessionWarmup

	// These should all be safe to call on nil
	sw.MarkUsed("https://example.com")
	sw.Start()
	sw.Stop()
	_ = sw.GetActiveHosts()
	_ = sw.GetLastSeen("https://example.com")
	sw.Clear()
	// If we got here without panicking, the test passes
}

func TestGetLastSeenNonExistent(t *testing.T) {
	sw := NewSessionWarmup(30*time.Second, 30*time.Second)
	sw.Start()
	defer sw.Stop()

	// GetLastSeen for a URL that was never marked should return zero time
	lastSeen := sw.GetLastSeen("https://nonexistent.example.com")
	if !lastSeen.IsZero() {
		t.Error("GetLastSeen returned non-zero time for non-existent URL")
	}
}

func TestGetActiveHosts(t *testing.T) {
	sw := NewSessionWarmup(30*time.Second, 30*time.Second)
	sw.Start()
	defer sw.Stop()

	// Initially empty
	if len(sw.GetActiveHosts()) != 0 {
		t.Errorf("GetActiveHosts returned %d hosts initially, want 0", len(sw.GetActiveHosts()))
	}

	// Add some hosts
	urls := []string{
		"https://api.anthropic.com",
		"https://api.openai.com",
	}

	for _, url := range urls {
		sw.MarkUsed(url)
	}

	activeHosts := sw.GetActiveHosts()
	if len(activeHosts) != len(urls) {
		t.Errorf("GetActiveHosts returned %d hosts, want %d", len(activeHosts), len(urls))
	}

	// Verify the returned hosts are correct (order not guaranteed)
	hostMap := make(map[string]bool)
	for _, host := range activeHosts {
		hostMap[host] = true
	}

	for _, url := range urls {
		if !hostMap[url] {
			t.Errorf("GetActiveHosts did not include %s", url)
		}
	}
}

func TestGlobalSessionWarmup(t *testing.T) {
	// Reset global state for this test
	globalSessionWarmup = nil
	globalSessionWarmupOnce = *(new(sync.Once))
	defer func() {
		globalSessionWarmup = nil
		globalSessionWarmupOnce = *(new(sync.Once))
	}()

	// First call should initialize and start
	sw1 := getSessionWarmup()
	if sw1 == nil {
		t.Fatal("getSessionWarmup returned nil")
	}
	if sw1.ticker == nil {
		t.Error("Global session warmup not started")
	}

	// Second call should return the same instance
	sw2 := getSessionWarmup()
	if sw1 != sw2 {
		t.Error("getSessionWarmup returned different instances")
	}

	// Stop and reset
	StopGlobalSessionWarmup()
	globalSessionWarmup = nil
	globalSessionWarmupOnce = *(new(sync.Once))
}

func TestStartGlobalSessionWarmup(t *testing.T) {
	// Reset global state for this test
	globalSessionWarmup = nil
	globalSessionWarmupOnce = *(new(sync.Once))
	defer func() {
		StopGlobalSessionWarmup()
		globalSessionWarmup = nil
		globalSessionWarmupOnce = *(new(sync.Once))
	}()

	StartGlobalSessionWarmup()

	if globalSessionWarmup == nil {
		t.Fatal("StartGlobalSessionWarmup did not initialize globalSessionWarmup")
	}
	if globalSessionWarmup.ticker == nil {
		t.Error("StartGlobalSessionWarmup did not start ticker")
	}

	// Calling again should be safe
	StartGlobalSessionWarmup()
}

func TestSendKeepAlivesToMultipleHosts(t *testing.T) {
	requestCounts := make(map[string]int)
	var mu sync.Mutex

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCounts["server1"]++
		mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCounts["server2"]++
		mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server2.Close()

	sw := NewSessionWarmup(30*time.Second, 30*time.Second)
	sw.Start()
	defer sw.Stop()

	// Mark both servers as used
	sw.MarkUsed(server1.URL)
	sw.MarkUsed(server2.URL)

	// Trigger keep-alive manually
	sw.sendKeepAlives()

	// Give time for the requests to complete
	time.Sleep(100 * time.Millisecond)

	// Both servers should have received requests
	mu.Lock()
	defer mu.Unlock()

	if requestCounts["server1"] == 0 {
		t.Error("No request sent to server1")
	}
	if requestCounts["server2"] == 0 {
		t.Error("No request sent to server2")
	}
}
