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
	info := ratelimit.MakeTestInfo("openai", "gpt-4",
		ratelimit.WithRequests(1000, 900, ""),
	)

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
	info := ratelimit.MakeTestInfo("openai", "gpt-4",
		ratelimit.WithRequests(1000, 100, ""),
		ratelimit.WithTokens(100000, 50000, ""),
	)

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
	info := ratelimit.MakeTestInfo("openai", "gpt-4",
		ratelimit.WithRequests(1000, 100, ""),
		ratelimit.WithTokens(100000, 50000, ""),
	)

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
	info := ratelimit.MakeTestInfo("test", "gpt-4",
		ratelimit.WithRequests(1000, 500, ""),
	)

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
	info := ratelimit.MakeTestInfo("test", "gpt-4",
		ratelimit.WithRequests(1000, 100, ""), // 10% remaining
	)

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
	info := ratelimit.MakeTestInfo("test", "gpt-4",
		ratelimit.WithRequests(1000, 500, ""),
	)

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
		info         func() *ratelimit.Info
	}{
		{
			providerName: "anthropic",
			info: func() *ratelimit.Info {
				i := ratelimit.MakeTestInfo("anthropic", "claude-3",
					ratelimit.WithInputTokens(100000, 10000, ""),
					ratelimit.WithOutputTokens(50000, 5000, ""),
				)
				return i
			},
		},
		{
			providerName: "openai",
			info: func() *ratelimit.Info {
				return ratelimit.MakeTestInfo("openai", "gpt-4",
					ratelimit.WithRequests(1000, 100, ""),
					ratelimit.WithTokens(100000, 50000, ""),
				)
			},
		},
		{
			providerName: "cerebras",
			info: func() *ratelimit.Info {
				return ratelimit.MakeTestInfo("cerebras", "llama3.1-70b",
					ratelimit.WithRequests(0, 50, ""),
					ratelimit.WithDailyRequests(0, 500, ""),
				)
			},
		},
		{
			providerName: "openrouter",
			info: func() *ratelimit.Info {
				return ratelimit.MakeTestInfo("openrouter", "auto",
					ratelimit.WithRequests(1000, 200, ""),
				)
			},
		},
		{
			providerName: "gemini",
			info: func() *ratelimit.Info {
				i := ratelimit.MakeTestInfo("gemini", "gemini-pro",
					ratelimit.WithRequests(500, 100, ""),
				)
				i.RequestsReset = time.Now().Add(time.Hour)
				return i
			},
		},
		{
			providerName: "qwen",
			info: func() *ratelimit.Info {
				return ratelimit.MakeTestInfo("qwen", "qwen-max",
					ratelimit.WithRequests(100, 50, ""),
					ratelimit.WithCustomData(map[string]interface{}{"test": "value"}),
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.providerName, func(t *testing.T) {
			parser := &MockParser{
				providerName: tt.providerName,
				info:         tt.info(),
			}

			helper := NewRateLimitHelper(parser)

			// This should not crash - just testing the logging paths
			helper.UpdateRateLimitInfo(tt.info())
		})
	}
}

func TestRateLimitHelper_ThreadSafety(t *testing.T) {
	parser := &MockParser{providerName: "test"}
	helper := NewRateLimitHelper(parser)

	info := ratelimit.MakeTestInfo("test", "gpt-4",
		ratelimit.WithRequests(1000, 500, ""),
	)

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
