package common

import (
	"net/http"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
)

// MockParser implements ratelimit.Parser for testing
type MockParser struct {
	providerName string
	info         *ratelimit.Info
	err          error
}

func (m *MockParser) Parse(headers http.Header, model string) (*ratelimit.Info, error) {
	return m.info, m.err
}

func (m *MockParser) ProviderName() string {
	return m.providerName
}

func TestNewRateLimitHelper(t *testing.T) {
	parser := &MockParser{providerName: "test"}
	helper := NewRateLimitHelper(parser)

	if helper == nil {
		t.Fatal("expected non-nil helper")
	}

	if helper.parser != parser {
		t.Error("parser not set correctly")
	}

	if helper.tracker == nil {
		t.Error("tracker should be initialized")
	}
}

func TestRateLimitHelper_ParseAndUpdateRateLimits(t *testing.T) {
	info := &ratelimit.Info{
		Model:             "gpt-4",
		RequestsLimit:     1000,
		RequestsRemaining: 900,
	}

	parser := &MockParser{
		providerName: "openai",
		info:         info,
	}

	helper := NewRateLimitHelper(parser)

	headers := http.Header{}
	helper.ParseAndUpdateRateLimits(headers, "gpt-4")

	// Verify the info was stored
	retrievedInfo, exists := helper.GetRateLimitInfo("gpt-4")
	if !exists {
		t.Fatal("expected rate limit info to be stored")
	}

	if retrievedInfo.RequestsRemaining != 900 {
		t.Errorf("got %d requests remaining, expected 900", retrievedInfo.RequestsRemaining)
	}
}

func TestRateLimitHelper_CanMakeRequest(t *testing.T) {
	info := &ratelimit.Info{
		Model:             "gpt-4",
		RequestsLimit:     1000,
		RequestsRemaining: 100,
		TokensLimit:       100000,
		TokensRemaining:   50000,
	}

	parser := &MockParser{
		providerName: "openai",
		info:         info,
	}

	helper := NewRateLimitHelper(parser)
	helper.UpdateRateLimitInfo(info)

	// Should allow request
	canMake := helper.CanMakeRequest("gpt-4", 1000)
	if !canMake {
		t.Error("expected to be able to make request")
	}
}

func TestRateLimitHelper_GetWaitTime(t *testing.T) {
	parser := &MockParser{providerName: "test"}
	helper := NewRateLimitHelper(parser)

	// With no rate limit info, should return 0
	waitTime := helper.GetWaitTime("nonexistent")
	if waitTime != 0 {
		t.Errorf("expected 0 wait time, got %v", waitTime)
	}
}

func TestRateLimitHelper_CheckRateLimitAndWait(t *testing.T) {
	info := &ratelimit.Info{
		Model:             "gpt-4",
		RequestsLimit:     1000,
		RequestsRemaining: 100,
		TokensLimit:       100000,
		TokensRemaining:   50000,
	}

	parser := &MockParser{
		providerName: "openai",
		info:         info,
	}

	helper := NewRateLimitHelper(parser)
	helper.UpdateRateLimitInfo(info)

	// Should proceed without waiting
	canProceed := helper.CheckRateLimitAndWait("gpt-4", 1000)
	if !canProceed {
		t.Error("expected to be able to proceed")
	}
}

func TestRateLimitHelper_GetRateLimitInfo(t *testing.T) {
	info := &ratelimit.Info{
		Model:             "gpt-4",
		RequestsLimit:     1000,
		RequestsRemaining: 500,
	}

	parser := &MockParser{providerName: "test"}
	helper := NewRateLimitHelper(parser)

	// Non-existent model
	_, exists := helper.GetRateLimitInfo("nonexistent")
	if exists {
		t.Error("should not find info for nonexistent model")
	}

	// Add info
	helper.UpdateRateLimitInfo(info)

	// Should now exist
	retrieved, exists := helper.GetRateLimitInfo("gpt-4")
	if !exists {
		t.Fatal("expected to find rate limit info")
	}

	if retrieved.RequestsRemaining != 500 {
		t.Errorf("got %d requests remaining, expected 500", retrieved.RequestsRemaining)
	}
}

func TestRateLimitHelper_ShouldThrottle(t *testing.T) {
	info := &ratelimit.Info{
		Model:             "gpt-4",
		RequestsLimit:     1000,
		RequestsRemaining: 100, // 10% remaining
	}

	parser := &MockParser{providerName: "test"}
	helper := NewRateLimitHelper(parser)
	helper.UpdateRateLimitInfo(info)

	// Just test that ShouldThrottle doesn't crash
	// The actual logic is in ratelimit.Tracker which we're not testing here
	_ = helper.ShouldThrottle("gpt-4", 0.2)
	_ = helper.ShouldThrottle("gpt-4", 0.05)
	_ = helper.ShouldThrottle("nonexistent", 0.5)
}

func TestRateLimitHelper_UpdateRateLimitInfo(t *testing.T) {
	parser := &MockParser{providerName: "test"}
	helper := NewRateLimitHelper(parser)

	// Update with nil should not crash
	helper.UpdateRateLimitInfo(nil)

	// Update with valid info
	info := &ratelimit.Info{
		Model:             "gpt-4",
		RequestsLimit:     1000,
		RequestsRemaining: 500,
	}

	helper.UpdateRateLimitInfo(info)

	retrieved, exists := helper.GetRateLimitInfo("gpt-4")
	if !exists {
		t.Fatal("expected to find updated info")
	}

	if retrieved.RequestsLimit != 1000 {
		t.Errorf("got limit %d, expected 1000", retrieved.RequestsLimit)
	}
}

func TestRateLimitHelper_GetTracker(t *testing.T) {
	parser := &MockParser{providerName: "test"}
	helper := NewRateLimitHelper(parser)

	tracker := helper.GetTracker()
	if tracker == nil {
		t.Error("expected non-nil tracker")
	}
}

func TestRateLimitHelper_GetParser(t *testing.T) {
	parser := &MockParser{providerName: "test"}
	helper := NewRateLimitHelper(parser)

	retrievedParser := helper.GetParser()
	if retrievedParser != parser {
		t.Error("expected same parser instance")
	}

	if retrievedParser.ProviderName() != "test" {
		t.Errorf("got provider name %q, expected %q", retrievedParser.ProviderName(), "test")
	}
}

func TestRateLimitHelper_ProviderSpecificLogging(t *testing.T) {
	tests := []struct {
		providerName string
		info         *ratelimit.Info
	}{
		{
			providerName: "anthropic",
			info: &ratelimit.Info{
				Model:                  "claude-3",
				InputTokensRemaining:   10000,
				InputTokensLimit:       100000,
				OutputTokensRemaining:  5000,
				OutputTokensLimit:      50000,
			},
		},
		{
			providerName: "openai",
			info: &ratelimit.Info{
				Model:             "gpt-4",
				RequestsRemaining: 100,
				RequestsLimit:     1000,
				TokensRemaining:   50000,
				TokensLimit:       100000,
			},
		},
		{
			providerName: "cerebras",
			info: &ratelimit.Info{
				Model:                  "llama3.1-70b",
				DailyRequestsRemaining: 500,
				RequestsRemaining:      50,
			},
		},
		{
			providerName: "openrouter",
			info: &ratelimit.Info{
				Model:             "auto",
				RequestsRemaining: 200,
				RequestsLimit:     1000,
			},
		},
		{
			providerName: "gemini",
			info: &ratelimit.Info{
				Model:             "gemini-pro",
				RequestsRemaining: 100,
				RequestsLimit:     500,
				RequestsReset:     time.Now().Add(time.Hour),
			},
		},
		{
			providerName: "qwen",
			info: &ratelimit.Info{
				Model:             "qwen-max",
				RequestsRemaining: 50,
				RequestsLimit:     100,
				CustomData:        map[string]interface{}{"test": "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.providerName, func(t *testing.T) {
			parser := &MockParser{
				providerName: tt.providerName,
				info:         tt.info,
			}

			helper := NewRateLimitHelper(parser)

			// This should not crash - just testing the logging paths
			helper.UpdateRateLimitInfo(tt.info)
		})
	}
}

func TestRateLimitHelper_ThreadSafety(t *testing.T) {
	parser := &MockParser{providerName: "test"}
	helper := NewRateLimitHelper(parser)

	info := &ratelimit.Info{
		Model:             "gpt-4",
		RequestsLimit:     1000,
		RequestsRemaining: 500,
	}

	// Concurrent operations
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			helper.UpdateRateLimitInfo(info)
		}
		done <- true
	}()

	// Reader goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				helper.GetRateLimitInfo("gpt-4")
				helper.CanMakeRequest("gpt-4", 100)
				helper.GetWaitTime("gpt-4")
				helper.ShouldThrottle("gpt-4", 0.5)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 11; i++ {
		<-done
	}
}
