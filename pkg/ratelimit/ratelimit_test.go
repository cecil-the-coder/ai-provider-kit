package ratelimit

import (
	"sync"
	"testing"
	"time"
)

// TestTracker_Update tests the Update method of the Tracker.
func TestTracker_Update(t *testing.T) {
	tracker := NewTracker()

	// Test updating with nil info
	tracker.Update(nil)
	if len(tracker.info) != 0 {
		t.Error("Expected tracker to ignore nil info")
	}

	// Test updating with valid info
	now := time.Now()
	info1 := &Info{
		Provider:          "anthropic",
		Model:             "claude-3-opus-20240229",
		Timestamp:         now,
		RequestsLimit:     1000,
		RequestsRemaining: 999,
		RequestsReset:     now.Add(time.Hour),
	}

	tracker.Update(info1)

	if len(tracker.info) != 1 {
		t.Errorf("Expected 1 model in tracker, got %d", len(tracker.info))
	}

	retrieved, exists := tracker.info[info1.Model]
	if !exists {
		t.Error("Expected info to be stored in tracker")
	}

	if retrieved.Provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got '%s'", retrieved.Provider)
	}

	if retrieved.RequestsRemaining != 999 {
		t.Errorf("Expected 999 requests remaining, got %d", retrieved.RequestsRemaining)
	}

	// Test updating existing model
	info2 := &Info{
		Provider:          "anthropic",
		Model:             "claude-3-opus-20240229",
		Timestamp:         now.Add(time.Minute),
		RequestsLimit:     1000,
		RequestsRemaining: 998,
		RequestsReset:     now.Add(time.Hour),
	}

	tracker.Update(info2)

	if len(tracker.info) != 1 {
		t.Errorf("Expected 1 model in tracker after update, got %d", len(tracker.info))
	}

	retrieved, exists = tracker.info[info2.Model]
	if !exists {
		t.Error("Expected updated info to be in tracker")
	}

	if retrieved.RequestsRemaining != 998 {
		t.Errorf("Expected 998 requests remaining after update, got %d", retrieved.RequestsRemaining)
	}

	// Test updating different models
	info3 := &Info{
		Provider:          "openai",
		Model:             "gpt-4",
		Timestamp:         now,
		RequestsLimit:     500,
		RequestsRemaining: 450,
		RequestsReset:     now.Add(time.Hour),
	}

	tracker.Update(info3)

	if len(tracker.info) != 2 {
		t.Errorf("Expected 2 models in tracker, got %d", len(tracker.info))
	}
}

// TestTracker_Get tests the Get method of the Tracker.
func TestTracker_Get(t *testing.T) {
	tracker := NewTracker()
	now := time.Now()

	// Test getting from empty tracker
	info, exists := tracker.Get("nonexistent-model")
	if exists {
		t.Error("Expected false for nonexistent model")
	}
	if info != nil {
		t.Error("Expected nil info for nonexistent model")
	}

	// Add some data
	testInfo := &Info{
		Provider:          "anthropic",
		Model:             "claude-3-opus-20240229",
		Timestamp:         now,
		RequestsLimit:     1000,
		RequestsRemaining: 999,
		RequestsReset:     now.Add(time.Hour),
		TokensLimit:       100000,
		TokensRemaining:   99500,
		TokensReset:       now.Add(time.Hour),
	}

	tracker.Update(testInfo)

	// Test getting existing model
	info, exists = tracker.Get("claude-3-opus-20240229")
	if !exists {
		t.Error("Expected true for existing model")
	}
	if info == nil {
		t.Fatal("Expected non-nil info for existing model")
	}

	if info.Provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got '%s'", info.Provider)
	}

	if info.RequestsRemaining != 999 {
		t.Errorf("Expected 999 requests remaining, got %d", info.RequestsRemaining)
	}

	if info.TokensRemaining != 99500 {
		t.Errorf("Expected 99500 tokens remaining, got %d", info.TokensRemaining)
	}

	// Test getting nonexistent model after adding data
	info, exists = tracker.Get("gpt-4")
	if exists {
		t.Error("Expected false for different model")
	}
	if info != nil {
		t.Error("Expected nil info for different model")
	}
}

// TestTracker_CanMakeRequest tests the CanMakeRequest method.
func TestTracker_CanMakeRequest(t *testing.T) {
	tracker := NewTracker()
	now := time.Now()

	// Test with no rate limit info (should allow request)
	if !tracker.CanMakeRequest("unknown-model", 1000) {
		t.Error("Expected to allow request when no rate limit info exists")
	}

	tests := []struct {
		name            string
		info            *Info
		estimatedTokens int
		want            bool
	}{
		{
			name: "has requests and tokens remaining",
			info: &Info{
				Provider:          "anthropic",
				Model:             "claude-3-opus-20240229",
				RequestsLimit:     1000,
				RequestsRemaining: 100,
				RequestsReset:     now.Add(time.Hour),
				TokensLimit:       100000,
				TokensRemaining:   50000,
				TokensReset:       now.Add(time.Hour),
			},
			estimatedTokens: 1000,
			want:            true,
		},
		{
			name: "no requests remaining",
			info: &Info{
				Provider:          "anthropic",
				Model:             "claude-3-opus-no-requests",
				RequestsLimit:     1000,
				RequestsRemaining: 0,
				RequestsReset:     now.Add(time.Hour),
				TokensLimit:       100000,
				TokensRemaining:   50000,
				TokensReset:       now.Add(time.Hour),
			},
			estimatedTokens: 1000,
			want:            false,
		},
		{
			name: "not enough tokens remaining",
			info: &Info{
				Provider:          "anthropic",
				Model:             "claude-3-opus-low-tokens",
				RequestsLimit:     1000,
				RequestsRemaining: 100,
				RequestsReset:     now.Add(time.Hour),
				TokensLimit:       100000,
				TokensRemaining:   500,
				TokensReset:       now.Add(time.Hour),
			},
			estimatedTokens: 1000,
			want:            false,
		},
		{
			name: "requests reset time passed",
			info: &Info{
				Provider:          "anthropic",
				Model:             "claude-3-opus-reset",
				RequestsLimit:     1000,
				RequestsRemaining: 0,
				RequestsReset:     now.Add(-time.Minute), // Already passed
				TokensLimit:       100000,
				TokensRemaining:   0,
				TokensReset:       now.Add(-time.Minute),
			},
			estimatedTokens: 1000,
			want:            true,
		},
		{
			name: "anthropic input tokens insufficient",
			info: &Info{
				Provider:             "anthropic",
				Model:                "claude-3-opus-input-limit",
				RequestsLimit:        1000,
				RequestsRemaining:    100,
				RequestsReset:        now.Add(time.Hour),
				InputTokensLimit:     10000,
				InputTokensRemaining: 500,
				InputTokensReset:     now.Add(time.Hour),
			},
			estimatedTokens: 1000,
			want:            false,
		},
		{
			name: "cerebras daily limit exceeded",
			info: &Info{
				Provider:               "cerebras",
				Model:                  "cerebras-model",
				RequestsLimit:          1000,
				RequestsRemaining:      100,
				RequestsReset:          now.Add(time.Hour),
				DailyRequestsLimit:     100,
				DailyRequestsRemaining: 0,
				DailyRequestsReset:     now.Add(time.Hour),
			},
			estimatedTokens: 0,
			want:            false,
		},
		{
			name: "openrouter credits exhausted",
			info: &Info{
				Provider:          "openrouter",
				Model:             "openrouter-model",
				RequestsLimit:     1000,
				RequestsRemaining: 100,
				RequestsReset:     now.Add(time.Hour),
				CreditsLimit:      100.0,
				CreditsRemaining:  0.0,
			},
			estimatedTokens: 0,
			want:            false,
		},
		{
			name: "openrouter has credits",
			info: &Info{
				Provider:          "openrouter",
				Model:             "openrouter-model-ok",
				RequestsLimit:     1000,
				RequestsRemaining: 100,
				RequestsReset:     now.Add(time.Hour),
				CreditsLimit:      100.0,
				CreditsRemaining:  50.0,
			},
			estimatedTokens: 0,
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker.Update(tt.info)
			got := tracker.CanMakeRequest(tt.info.Model, tt.estimatedTokens)
			if got != tt.want {
				t.Errorf("CanMakeRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTracker_GetWaitTime tests the GetWaitTime method.
func TestTracker_GetWaitTime(t *testing.T) {
	tracker := NewTracker()
	now := time.Now()

	// Test with no rate limit info
	waitTime := tracker.GetWaitTime("unknown-model")
	if waitTime != 0 {
		t.Errorf("Expected 0 wait time for unknown model, got %v", waitTime)
	}

	// Test with RetryAfter specified
	info1 := &Info{
		Provider:   "anthropic",
		Model:      "model-with-retry-after",
		RetryAfter: 30 * time.Second,
	}
	tracker.Update(info1)
	waitTime = tracker.GetWaitTime("model-with-retry-after")
	if waitTime != 30*time.Second {
		t.Errorf("Expected 30s wait time, got %v", waitTime)
	}

	// Test with future reset time
	info2 := &Info{
		Provider:          "anthropic",
		Model:             "model-with-reset",
		RequestsLimit:     1000,
		RequestsRemaining: 0,
		RequestsReset:     now.Add(5 * time.Minute),
	}
	tracker.Update(info2)
	waitTime = tracker.GetWaitTime("model-with-reset")
	if waitTime <= 0 || waitTime > 6*time.Minute {
		t.Errorf("Expected wait time around 5 minutes, got %v", waitTime)
	}

	// Test with past reset time
	info3 := &Info{
		Provider:          "anthropic",
		Model:             "model-reset-passed",
		RequestsLimit:     1000,
		RequestsRemaining: 0,
		RequestsReset:     now.Add(-time.Minute),
	}
	tracker.Update(info3)
	waitTime = tracker.GetWaitTime("model-reset-passed")
	if waitTime != 0 {
		t.Errorf("Expected 0 wait time for past reset, got %v", waitTime)
	}

	// Test with multiple reset times (should return earliest)
	info4 := &Info{
		Provider:         "anthropic",
		Model:            "model-multiple-resets",
		RequestsReset:    now.Add(10 * time.Minute),
		TokensReset:      now.Add(3 * time.Minute), // Earliest
		InputTokensReset: now.Add(7 * time.Minute),
	}
	tracker.Update(info4)
	waitTime = tracker.GetWaitTime("model-multiple-resets")
	if waitTime <= 2*time.Minute || waitTime > 4*time.Minute {
		t.Errorf("Expected wait time around 3 minutes (earliest reset), got %v", waitTime)
	}
}

// TestTracker_ShouldThrottle tests the ShouldThrottle method.
func TestTracker_ShouldThrottle(t *testing.T) {
	tracker := NewTracker()
	now := time.Now()

	// Test with no rate limit info
	if tracker.ShouldThrottle("unknown-model", 0.8) {
		t.Error("Expected no throttling for unknown model")
	}

	tests := []struct {
		name      string
		info      *Info
		threshold float64
		want      bool
	}{
		{
			name: "below threshold - requests",
			info: &Info{
				Provider:          "anthropic",
				Model:             "model-ok",
				RequestsLimit:     1000,
				RequestsRemaining: 500, // 50% used
				RequestsReset:     now.Add(time.Hour),
			},
			threshold: 0.8,
			want:      false,
		},
		{
			name: "above threshold - requests",
			info: &Info{
				Provider:          "anthropic",
				Model:             "model-high-usage",
				RequestsLimit:     1000,
				RequestsRemaining: 100, // 90% used
				RequestsReset:     now.Add(time.Hour),
			},
			threshold: 0.8,
			want:      true,
		},
		{
			name: "at threshold - requests",
			info: &Info{
				Provider:          "anthropic",
				Model:             "model-at-threshold",
				RequestsLimit:     1000,
				RequestsRemaining: 200, // 80% used
				RequestsReset:     now.Add(time.Hour),
			},
			threshold: 0.8,
			want:      true,
		},
		{
			name: "above threshold - tokens",
			info: &Info{
				Provider:        "anthropic",
				Model:           "model-tokens-high",
				TokensLimit:     100000,
				TokensRemaining: 5000, // 95% used
				TokensReset:     now.Add(time.Hour),
			},
			threshold: 0.8,
			want:      true,
		},
		{
			name: "above threshold - input tokens",
			info: &Info{
				Provider:             "anthropic",
				Model:                "model-input-tokens-high",
				InputTokensLimit:     50000,
				InputTokensRemaining: 5000, // 90% used
				InputTokensReset:     now.Add(time.Hour),
			},
			threshold: 0.8,
			want:      true,
		},
		{
			name: "above threshold - output tokens",
			info: &Info{
				Provider:              "anthropic",
				Model:                 "model-output-tokens-high",
				OutputTokensLimit:     50000,
				OutputTokensRemaining: 1000, // 98% used
				OutputTokensReset:     now.Add(time.Hour),
			},
			threshold: 0.8,
			want:      true,
		},
		{
			name: "above threshold - daily requests",
			info: &Info{
				Provider:               "cerebras",
				Model:                  "cerebras-daily-high",
				DailyRequestsLimit:     1000,
				DailyRequestsRemaining: 150, // 85% used
				DailyRequestsReset:     now.Add(time.Hour),
			},
			threshold: 0.8,
			want:      true,
		},
		{
			name: "above threshold - credits",
			info: &Info{
				Provider:         "openrouter",
				Model:            "openrouter-credits-low",
				CreditsLimit:     100.0,
				CreditsRemaining: 10.0, // 90% used
			},
			threshold: 0.8,
			want:      true,
		},
		{
			name: "reset time passed - should not throttle",
			info: &Info{
				Provider:          "anthropic",
				Model:             "model-reset-passed",
				RequestsLimit:     1000,
				RequestsRemaining: 0, // 100% used but reset passed
				RequestsReset:     now.Add(-time.Minute),
			},
			threshold: 0.8,
			want:      false,
		},
		{
			name: "invalid threshold - uses default",
			info: &Info{
				Provider:          "anthropic",
				Model:             "model-invalid-threshold",
				RequestsLimit:     1000,
				RequestsRemaining: 100, // 90% used
				RequestsReset:     now.Add(time.Hour),
			},
			threshold: 1.5, // Invalid, will use 0.8 default
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker.Update(tt.info)
			got := tracker.ShouldThrottle(tt.info.Model, tt.threshold)
			if got != tt.want {
				t.Errorf("ShouldThrottle() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTracker_Concurrent tests thread safety of the Tracker.
func TestTracker_Concurrent(t *testing.T) {
	tracker := NewTracker()
	now := time.Now()

	// Number of concurrent goroutines
	numGoroutines := 100
	numOperations := 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3) // 3 types of operations

	// Concurrent updates
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				info := &Info{
					Provider:          "test-provider",
					Model:             "test-model",
					Timestamp:         now,
					RequestsLimit:     1000,
					RequestsRemaining: 1000 - (id*numOperations + j),
					RequestsReset:     now.Add(time.Hour),
				}
				tracker.Update(info)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				tracker.Get("test-model")
			}
		}()
	}

	// Concurrent CanMakeRequest checks
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				tracker.CanMakeRequest("test-model", 100)
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify tracker is still functional
	info, exists := tracker.Get("test-model")
	if !exists {
		t.Error("Expected model to exist after concurrent operations")
	}
	if info == nil {
		t.Error("Expected non-nil info after concurrent operations")
	}

	// Verify CanMakeRequest still works
	_ = tracker.CanMakeRequest("test-model", 100)

	// Verify GetWaitTime still works
	_ = tracker.GetWaitTime("test-model")

	// Verify ShouldThrottle still works
	_ = tracker.ShouldThrottle("test-model", 0.8)
}

// TestNewTracker tests the NewTracker constructor.
func TestNewTracker(t *testing.T) {
	tracker := NewTracker()

	if tracker == nil {
		t.Fatal("Expected non-nil tracker")
	}

	if tracker.info == nil {
		t.Error("Expected initialized info map")
	}

	if len(tracker.info) != 0 {
		t.Errorf("Expected empty info map, got %d entries", len(tracker.info))
	}

	if !tracker.lastUpdate.IsZero() {
		t.Error("Expected zero lastUpdate time for new tracker")
	}
}

// TestInfo_ProviderSpecificFields tests that provider-specific fields are properly stored.
func TestInfo_ProviderSpecificFields(t *testing.T) {
	tracker := NewTracker()
	now := time.Now()

	testAnthropicProviderFields(t, tracker, now)
	testCerebrasProviderFields(t, tracker, now)
	testOpenRouterProviderFields(t, tracker)
	testGenericProviderFields(t, tracker)
}

// testAnthropicProviderFields tests Anthropic-specific fields
func testAnthropicProviderFields(t *testing.T, tracker *Tracker, now time.Time) {
	anthropicInfo := &Info{
		Provider:              "anthropic",
		Model:                 "claude-3-opus-20240229",
		InputTokensLimit:      10000,
		InputTokensRemaining:  9500,
		InputTokensReset:      now.Add(time.Hour),
		OutputTokensLimit:     20000,
		OutputTokensRemaining: 19000,
		OutputTokensReset:     now.Add(time.Hour),
	}
	tracker.Update(anthropicInfo)

	retrieved, exists := tracker.Get("claude-3-opus-20240229")
	if !exists || retrieved == nil {
		t.Fatal("Expected to retrieve Anthropic info")
	}

	if retrieved.InputTokensLimit != 10000 {
		t.Errorf("Expected InputTokensLimit 10000, got %d", retrieved.InputTokensLimit)
	}
	if retrieved.OutputTokensRemaining != 19000 {
		t.Errorf("Expected OutputTokensRemaining 19000, got %d", retrieved.OutputTokensRemaining)
	}
}

// testCerebrasProviderFields tests Cerebras-specific fields
func testCerebrasProviderFields(t *testing.T, tracker *Tracker, now time.Time) {
	cerebrasInfo := &Info{
		Provider:               "cerebras",
		Model:                  "cerebras-model",
		DailyRequestsLimit:     1000,
		DailyRequestsRemaining: 950,
		DailyRequestsReset:     now.Add(24 * time.Hour),
	}
	tracker.Update(cerebrasInfo)

	retrieved, exists := tracker.Get("cerebras-model")
	if !exists || retrieved == nil {
		t.Fatal("Expected to retrieve Cerebras info")
	}

	if retrieved.DailyRequestsLimit != 1000 {
		t.Errorf("Expected DailyRequestsLimit 1000, got %d", retrieved.DailyRequestsLimit)
	}
}

// testOpenRouterProviderFields tests OpenRouter-specific fields
func testOpenRouterProviderFields(t *testing.T, tracker *Tracker) {
	openrouterInfo := &Info{
		Provider:         "openrouter",
		Model:            "openrouter-model",
		CreditsLimit:     100.5,
		CreditsRemaining: 75.25,
		IsFreeTier:       true,
	}
	tracker.Update(openrouterInfo)

	retrieved, exists := tracker.Get("openrouter-model")
	if !exists || retrieved == nil {
		t.Fatal("Expected to retrieve OpenRouter info")
	}

	if retrieved.CreditsLimit != 100.5 {
		t.Errorf("Expected CreditsLimit 100.5, got %f", retrieved.CreditsLimit)
	}
	if retrieved.CreditsRemaining != 75.25 {
		t.Errorf("Expected CreditsRemaining 75.25, got %f", retrieved.CreditsRemaining)
	}
	if !retrieved.IsFreeTier {
		t.Error("Expected IsFreeTier to be true")
	}
}

// testGenericProviderFields tests generic fields
func testGenericProviderFields(t *testing.T, tracker *Tracker) {
	genericInfo := &Info{
		Provider:   "generic",
		Model:      "generic-model",
		RequestID:  "req-12345",
		RetryAfter: 60 * time.Second,
		CustomData: map[string]interface{}{
			"custom_field":  "custom_value",
			"numeric_field": 42,
		},
	}
	tracker.Update(genericInfo)

	retrieved, exists := tracker.Get("generic-model")
	if !exists || retrieved == nil {
		t.Fatal("Expected to retrieve generic info")
	}

	if retrieved.RequestID != "req-12345" {
		t.Errorf("Expected RequestID 'req-12345', got '%s'", retrieved.RequestID)
	}
	if retrieved.RetryAfter != 60*time.Second {
		t.Errorf("Expected RetryAfter 60s, got %v", retrieved.RetryAfter)
	}
	if retrieved.CustomData == nil {
		t.Fatal("Expected non-nil CustomData")
	}
	if retrieved.CustomData["custom_field"] != "custom_value" {
		t.Errorf("Expected custom_field 'custom_value', got '%v'", retrieved.CustomData["custom_field"])
	}
}
