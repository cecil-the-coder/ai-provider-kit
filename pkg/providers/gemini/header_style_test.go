// Package gemini provides header style tests for dual quota management.
package gemini

import (
	"net/http"
	"testing"
	"time"
)

func TestHeaderStyle_Constants(t *testing.T) {
	// Verify constant values
	if HeaderStyleAntigravity != "antigravity" {
		t.Errorf("HeaderStyleAntigravity = %q, want %q", HeaderStyleAntigravity, "antigravity")
	}
	if HeaderStyleGeminiCLI != "gemini-cli" {
		t.Errorf("HeaderStyleGeminiCLI = %q, want %q", HeaderStyleGeminiCLI, "gemini-cli")
	}
}

func TestGetHeaderConfig(t *testing.T) {
	tests := []struct {
		name     string
		style    HeaderStyle
		wantUA   string
		wantHdrs map[string]string
	}{
		{
			name:   "Antigravity style",
			style:  HeaderStyleAntigravity,
			wantUA: "google-cloud-sdk gcloud/526.0.0 command/gcloud.ai.models.describe invocation-id/placeholder-id environment/devshell environment-version/None interactive/True from-script/False python/3.11.8 term/xterm-256color (Linux 6.1.85+)",
			wantHdrs: map[string]string{
				"x-goog-api-client": "cred-type/u",
			},
		},
		{
			name:   "Gemini CLI style",
			style:  HeaderStyleGeminiCLI,
			wantUA: "gemini-cli/0.1.0",
			wantHdrs: map[string]string{
				"x-goog-api-client": "gemini-cli/0.1.0 gl-go/1.22.0",
			},
		},
		{
			name:   "Unknown style defaults to Antigravity",
			style:  HeaderStyle("unknown"),
			wantUA: "google-cloud-sdk gcloud/526.0.0 command/gcloud.ai.models.describe invocation-id/placeholder-id environment/devshell environment-version/None interactive/True from-script/False python/3.11.8 term/xterm-256color (Linux 6.1.85+)",
			wantHdrs: map[string]string{
				"x-goog-api-client": "cred-type/u",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GetHeaderConfig(tt.style)
			if config.UserAgent != tt.wantUA {
				t.Errorf("UserAgent = %q, want %q", config.UserAgent, tt.wantUA)
			}
			for key, wantVal := range tt.wantHdrs {
				if gotVal, ok := config.ExtraHeaders[key]; !ok || gotVal != wantVal {
					t.Errorf("ExtraHeaders[%q] = %q, want %q", key, gotVal, wantVal)
				}
			}
		})
	}
}

func TestApplyHeaderStyle(t *testing.T) {
	tests := []struct {
		name  string
		style HeaderStyle
	}{
		{name: "Antigravity", style: HeaderStyleAntigravity},
		{name: "GeminiCLI", style: HeaderStyleGeminiCLI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "https://example.com", nil)
			ApplyHeaderStyle(req, tt.style)

			config := GetHeaderConfig(tt.style)

			// Check User-Agent
			if got := req.Header.Get("User-Agent"); got != config.UserAgent {
				t.Errorf("User-Agent = %q, want %q", got, config.UserAgent)
			}

			// Check extra headers
			for key, wantVal := range config.ExtraHeaders {
				if got := req.Header.Get(key); got != wantVal {
					t.Errorf("Header[%q] = %q, want %q", key, got, wantVal)
				}
			}
		})
	}
}

func TestNewDualQuotaManager(t *testing.T) {
	mgr := NewDualQuotaManager()

	if mgr == nil {
		t.Fatal("NewDualQuotaManager returned nil")
	}

	if mgr.preferredStyle != HeaderStyleAntigravity {
		t.Errorf("preferredStyle = %q, want %q", mgr.preferredStyle, HeaderStyleAntigravity)
	}

	if !mgr.fallbackEnabled {
		t.Error("fallbackEnabled should be true by default")
	}

	if mgr.rateLimitCooldown != 60*time.Second {
		t.Errorf("rateLimitCooldown = %v, want %v", mgr.rateLimitCooldown, 60*time.Second)
	}
}

func TestNewDualQuotaManagerWithOptions(t *testing.T) {
	mgr := NewDualQuotaManagerWithOptions(HeaderStyleGeminiCLI, false, 30*time.Second)

	if mgr.preferredStyle != HeaderStyleGeminiCLI {
		t.Errorf("preferredStyle = %q, want %q", mgr.preferredStyle, HeaderStyleGeminiCLI)
	}

	if mgr.fallbackEnabled {
		t.Error("fallbackEnabled should be false")
	}

	if mgr.rateLimitCooldown != 30*time.Second {
		t.Errorf("rateLimitCooldown = %v, want %v", mgr.rateLimitCooldown, 30*time.Second)
	}
}

func TestDualQuotaManager_GetSetPreferredStyle(t *testing.T) {
	mgr := NewDualQuotaManager()

	if got := mgr.GetPreferredStyle(); got != HeaderStyleAntigravity {
		t.Errorf("GetPreferredStyle() = %q, want %q", got, HeaderStyleAntigravity)
	}

	mgr.SetPreferredStyle(HeaderStyleGeminiCLI)

	if got := mgr.GetPreferredStyle(); got != HeaderStyleGeminiCLI {
		t.Errorf("GetPreferredStyle() = %q, want %q", got, HeaderStyleGeminiCLI)
	}
}

func TestDualQuotaManager_GetSetFallbackEnabled(t *testing.T) {
	mgr := NewDualQuotaManager()

	if !mgr.IsFallbackEnabled() {
		t.Error("IsFallbackEnabled() should be true by default")
	}

	mgr.SetFallbackEnabled(false)

	if mgr.IsFallbackEnabled() {
		t.Error("IsFallbackEnabled() should be false after SetFallbackEnabled(false)")
	}
}

func TestDualQuotaManager_GetActiveStyle_NoRateLimits(t *testing.T) {
	mgr := NewDualQuotaManager()

	// Should return preferred style when no rate limits
	if got := mgr.GetActiveStyle(); got != HeaderStyleAntigravity {
		t.Errorf("GetActiveStyle() = %q, want %q", got, HeaderStyleAntigravity)
	}
}

func TestDualQuotaManager_GetActiveStyle_WithRateLimit(t *testing.T) {
	mgr := NewDualQuotaManager()

	// Rate limit the preferred style
	mgr.RecordRateLimit(HeaderStyleAntigravity, 60*time.Second)

	// Should return alternate style when preferred is rate limited
	if got := mgr.GetActiveStyle(); got != HeaderStyleGeminiCLI {
		t.Errorf("GetActiveStyle() = %q, want %q", got, HeaderStyleGeminiCLI)
	}
}

func TestDualQuotaManager_GetActiveStyle_BothRateLimited(t *testing.T) {
	mgr := NewDualQuotaManager()

	// Rate limit both styles with different retry times
	mgr.RecordRateLimit(HeaderStyleAntigravity, 120*time.Second)
	mgr.RecordRateLimit(HeaderStyleGeminiCLI, 30*time.Second)

	// Should return the style with shorter cooldown (GeminiCLI)
	got := mgr.GetActiveStyle()
	if got != HeaderStyleGeminiCLI {
		t.Errorf("GetActiveStyle() = %q, want %q (shorter cooldown)", got, HeaderStyleGeminiCLI)
	}
}

func TestDualQuotaManager_GetActiveStyle_FallbackDisabled(t *testing.T) {
	mgr := NewDualQuotaManager()
	mgr.SetFallbackEnabled(false)

	// Rate limit the preferred style
	mgr.RecordRateLimit(HeaderStyleAntigravity, 60*time.Second)

	// Should still return preferred style when fallback is disabled
	if got := mgr.GetActiveStyle(); got != HeaderStyleAntigravity {
		t.Errorf("GetActiveStyle() = %q, want %q (fallback disabled)", got, HeaderStyleAntigravity)
	}
}

func TestDualQuotaManager_RecordSuccess(t *testing.T) {
	mgr := NewDualQuotaManager()

	// Rate limit the preferred style
	mgr.RecordRateLimit(HeaderStyleAntigravity, 60*time.Second)

	// Record success on alternate style
	mgr.RecordSuccess(HeaderStyleGeminiCLI)

	// Check that GeminiCLI became the preferred style
	if got := mgr.GetPreferredStyle(); got != HeaderStyleGeminiCLI {
		t.Errorf("GetPreferredStyle() = %q, want %q after success on alternate", got, HeaderStyleGeminiCLI)
	}

	// Check that GeminiCLI is not rate limited
	state := mgr.GetRateLimitState(HeaderStyleGeminiCLI)
	if state.IsRateLimited {
		t.Error("GeminiCLI should not be rate limited after RecordSuccess")
	}
}

func TestDualQuotaManager_ShouldTryAlternateStyle(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(*DualQuotaManager)
		currentStyle HeaderStyle
		expectedTry  bool
	}{
		{
			name:         "No rate limits - alternate available",
			setupFunc:    func(m *DualQuotaManager) {},
			currentStyle: HeaderStyleAntigravity,
			expectedTry:  true,
		},
		{
			name: "Alternate rate limited - should not try",
			setupFunc: func(m *DualQuotaManager) {
				m.RecordRateLimit(HeaderStyleGeminiCLI, 60*time.Second)
			},
			currentStyle: HeaderStyleAntigravity,
			expectedTry:  false,
		},
		{
			name: "Fallback disabled - should not try",
			setupFunc: func(m *DualQuotaManager) {
				m.SetFallbackEnabled(false)
			},
			currentStyle: HeaderStyleAntigravity,
			expectedTry:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewDualQuotaManager()
			tt.setupFunc(mgr)

			if got := mgr.ShouldTryAlternateStyle(tt.currentStyle); got != tt.expectedTry {
				t.Errorf("ShouldTryAlternateStyle(%q) = %v, want %v", tt.currentStyle, got, tt.expectedTry)
			}
		})
	}
}

func TestDualQuotaManager_GetAlternateStyle(t *testing.T) {
	mgr := NewDualQuotaManager()

	if got := mgr.GetAlternateStyle(HeaderStyleAntigravity); got != HeaderStyleGeminiCLI {
		t.Errorf("GetAlternateStyle(Antigravity) = %q, want %q", got, HeaderStyleGeminiCLI)
	}

	if got := mgr.GetAlternateStyle(HeaderStyleGeminiCLI); got != HeaderStyleAntigravity {
		t.Errorf("GetAlternateStyle(GeminiCLI) = %q, want %q", got, HeaderStyleAntigravity)
	}
}

func TestDualQuotaManager_Reset(t *testing.T) {
	mgr := NewDualQuotaManager()

	// Configure some state
	mgr.SetPreferredStyle(HeaderStyleGeminiCLI)
	mgr.RecordRateLimit(HeaderStyleAntigravity, 60*time.Second)

	// Reset
	mgr.Reset()

	// Check state is reset
	if got := mgr.GetPreferredStyle(); got != HeaderStyleAntigravity {
		t.Errorf("GetPreferredStyle() = %q, want %q after Reset", got, HeaderStyleAntigravity)
	}

	state := mgr.GetRateLimitState(HeaderStyleAntigravity)
	if state.IsRateLimited {
		t.Error("Antigravity should not be rate limited after Reset")
	}
}

func TestDualQuotaManager_GetStatus(t *testing.T) {
	mgr := NewDualQuotaManager()

	status := mgr.GetStatus()

	if status == nil {
		t.Fatal("GetStatus() returned nil")
	}

	// Check required fields
	if _, ok := status["preferred_style"]; !ok {
		t.Error("Status missing 'preferred_style'")
	}
	if _, ok := status["fallback_enabled"]; !ok {
		t.Error("Status missing 'fallback_enabled'")
	}
	if _, ok := status["cooldown_duration"]; !ok {
		t.Error("Status missing 'cooldown_duration'")
	}
	if _, ok := status["active_style"]; !ok {
		t.Error("Status missing 'active_style'")
	}
}

func TestDualQuotaManager_CooldownExpiry(t *testing.T) {
	mgr := NewDualQuotaManagerWithOptions(HeaderStyleAntigravity, true, 100*time.Millisecond)

	// Rate limit the preferred style
	mgr.RecordRateLimit(HeaderStyleAntigravity, 100*time.Millisecond)

	// Immediately, it should be rate limited
	if mgr.GetActiveStyle() != HeaderStyleGeminiCLI {
		t.Error("Should use alternate style immediately after rate limit")
	}

	// Wait for cooldown to expire
	time.Sleep(150 * time.Millisecond)

	// After cooldown, preferred style should be available again
	if mgr.GetActiveStyle() != HeaderStyleAntigravity {
		t.Error("Should use preferred style after cooldown expires")
	}
}

func TestRateLimitState_Fields(t *testing.T) {
	mgr := NewDualQuotaManager()

	// Record a rate limit
	beforeLimit := time.Now()
	mgr.RecordRateLimit(HeaderStyleAntigravity, 30*time.Second)
	afterLimit := time.Now()

	state := mgr.GetRateLimitState(HeaderStyleAntigravity)

	if !state.IsRateLimited {
		t.Error("IsRateLimited should be true")
	}

	if state.RateLimitedAt.Before(beforeLimit) || state.RateLimitedAt.After(afterLimit) {
		t.Error("RateLimitedAt should be between before and after record")
	}

	if state.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v, want %v", state.RetryAfter, 30*time.Second)
	}

	// Record success
	beforeSuccess := time.Now()
	mgr.RecordSuccess(HeaderStyleAntigravity)
	afterSuccess := time.Now()

	state = mgr.GetRateLimitState(HeaderStyleAntigravity)

	if state.IsRateLimited {
		t.Error("IsRateLimited should be false after success")
	}

	if state.LastSuccessAt.Before(beforeSuccess) || state.LastSuccessAt.After(afterSuccess) {
		t.Error("LastSuccessAt should be between before and after success")
	}
}
