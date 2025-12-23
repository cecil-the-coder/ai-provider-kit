// Package http provides HTTP client utilities and helpers for AI providers.
// It includes reusable HTTP clients with retry logic, metrics, and interceptors.
package http

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// SessionWarmup provides automatic session-aware connection warmup for HTTP clients.
// It tracks which base URLs have been recently used and sends periodic keep-alive
// requests to maintain warm connections, reducing cold start latency.
type SessionWarmup struct {
	mu            sync.RWMutex
	perHost       map[string]time.Time // baseURL -> lastSeen timestamp
	checkInterval time.Duration        // How often to send keep-alive (default: 30s)
	maxIdle       time.Duration        // Host 'inactive' after this (default: 30s)
	ticker        *time.Ticker
	done          chan struct{}
	client        *http.Client // HTTP client for warmup requests
}

// NewSessionWarmup creates a new SessionWarmup instance with the specified intervals.
// If checkInterval or maxIdle are zero, defaults of 30 seconds are used.
func NewSessionWarmup(checkInterval, maxIdle time.Duration) *SessionWarmup {
	if checkInterval == 0 {
		checkInterval = 30 * time.Second
	}
	if maxIdle == 0 {
		maxIdle = 30 * time.Second
	}

	return &SessionWarmup{
		perHost:       make(map[string]time.Time),
		checkInterval: checkInterval,
		maxIdle:       maxIdle,
		done:          make(chan struct{}),
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:          10,
				MaxIdleConnsPerHost:   2,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   5 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				DisableKeepAlives:     false,
				DisableCompression:    true,
				ForceAttemptHTTP2:     true,
			},
		},
	}
}

// MarkUsed marks a base URL as recently used, updating its lastSeen timestamp.
// This should be called on each request to track active sessions.
func (s *SessionWarmup) MarkUsed(baseURL string) {
	if s == nil {
		return
	}
	normalizedKey := normalizeBaseURL(baseURL)
	s.mu.Lock()
	s.perHost[normalizedKey] = time.Now()
	s.mu.Unlock()
}

// Start begins the background goroutine that sends periodic keep-alive requests
// to recently used hosts. It should be called once during application initialization.
func (s *SessionWarmup) Start() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.ticker != nil {
		// Already started
		s.mu.Unlock()
		return
	}
	s.ticker = time.NewTicker(s.checkInterval)
	s.mu.Unlock()

	go s.warmupLoop()
}

// Stop halts the background warmup goroutine and releases resources.
// It should be called during application shutdown.
func (s *SessionWarmup) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.ticker != nil {
		s.ticker.Stop()
		s.ticker = nil
	}
	s.mu.Unlock()

	select {
	case s.done <- struct{}{}:
		// Signal sent
	default:
		// Already stopping
	}
}

// warmupLoop runs in the background, sending periodic HEAD requests to active hosts.
func (s *SessionWarmup) warmupLoop() {
	// Get a local reference to the ticker channel to avoid race conditions
	var tickerCh <-chan time.Time
	s.mu.Lock()
	if s.ticker != nil {
		tickerCh = s.ticker.C
	}
	s.mu.Unlock()

	if tickerCh == nil {
		return
	}

	for {
		select {
		case <-tickerCh:
			s.sendKeepAlives()
		case <-s.done:
			return
		}
	}
}

// sendKeepAlives sends HEAD requests to all hosts that have been used recently.
func (s *SessionWarmup) sendKeepAlives() {
	s.mu.RLock()
	now := time.Now()
	activeHosts := make([]string, 0, len(s.perHost))
	for baseURL, lastSeen := range s.perHost {
		if now.Sub(lastSeen) < s.maxIdle {
			activeHosts = append(activeHosts, baseURL)
		}
	}
	s.mu.RUnlock()

	// Send keep-alive requests concurrently
	var wg sync.WaitGroup
	for _, baseURL := range activeHosts {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			s.sendKeepAlive(url)
		}(baseURL)
	}
	wg.Wait()

	// Clean up inactive hosts
	s.mu.Lock()
	for baseURL, lastSeen := range s.perHost {
		if now.Sub(lastSeen) >= s.maxIdle {
			delete(s.perHost, baseURL)
		}
	}
	s.mu.Unlock()
}

// sendKeepAlive sends a HEAD request to the specified URL to keep the connection warm.
func (s *SessionWarmup) sendKeepAlive(baseURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "HEAD", baseURL, nil)
	if err != nil {
		return
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return
	}
	// Immediately close the response body - we only care about establishing the connection
	_ = resp.Body.Close()
}

// GetActiveHosts returns a list of base URLs that are currently considered active
// (used within the maxIdle duration). This is primarily useful for testing and debugging.
func (s *SessionWarmup) GetActiveHosts() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	activeHosts := make([]string, 0, len(s.perHost))
	for baseURL, lastSeen := range s.perHost {
		if now.Sub(lastSeen) < s.maxIdle {
			activeHosts = append(activeHosts, baseURL)
		}
	}
	return activeHosts
}

// GetLastSeen returns the lastSeen timestamp for the given base URL.
// Returns zero time if the URL is not being tracked.
func (s *SessionWarmup) GetLastSeen(baseURL string) time.Time {
	if s == nil {
		return time.Time{}
	}
	normalizedKey := normalizeBaseURL(baseURL)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.perHost[normalizedKey]
}

// Clear removes all tracked hosts. This is primarily useful for testing.
func (s *SessionWarmup) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.perHost = make(map[string]time.Time)
}

// globalSessionWarmup is the singleton session warmup instance.
// It is lazily initialized on first use via getSessionWarmup().
var globalSessionWarmup *SessionWarmup
var globalSessionWarmupOnce sync.Once

// getSessionWarmup returns the global session warmup instance, initializing it if necessary.
func getSessionWarmup() *SessionWarmup {
	globalSessionWarmupOnce.Do(func() {
		globalSessionWarmup = NewSessionWarmup(30*time.Second, 30*time.Second)
		globalSessionWarmup.Start()
	})
	return globalSessionWarmup
}

// StartGlobalSessionWarmup explicitly starts the global session warmup.
// This is optional - the warmup will start automatically on the first MarkUsed call.
func StartGlobalSessionWarmup() {
	getSessionWarmup()
}

// StopGlobalSessionWarmup stops the global session warmup background goroutine.
// This should be called during application shutdown.
func StopGlobalSessionWarmup() {
	if globalSessionWarmup != nil {
		globalSessionWarmup.Stop()
	}
}
