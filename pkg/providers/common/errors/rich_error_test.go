package errors

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewRichError(t *testing.T) {
	baseErr := errors.New("test error")
	richErr := NewRichError(baseErr)

	if richErr == nil {
		t.Fatal("NewRichError returned nil")
	}

	if richErr.err != baseErr {
		t.Error("Expected base error to be wrapped")
	}

	if richErr.context == nil {
		t.Error("Expected context to be initialized")
	}

	if richErr.snapshotConfig == nil {
		t.Error("Expected snapshot config to be initialized")
	}
}

func TestRichError_Error(t *testing.T) {
	baseErr := errors.New("base error message")
	richErr := NewRichError(baseErr)

	if richErr.Error() != "base error message" {
		t.Errorf("Expected error message 'base error message', got: %s", richErr.Error())
	}

	// Test with nil error
	richErr2 := &RichError{}
	if richErr2.Error() != "unknown error" {
		t.Errorf("Expected 'unknown error' for nil error, got: %s", richErr2.Error())
	}
}

func TestRichError_Unwrap(t *testing.T) {
	baseErr := errors.New("base error")
	richErr := NewRichError(baseErr)

	unwrapped := richErr.Unwrap()
	if unwrapped != baseErr {
		t.Error("Expected Unwrap to return base error")
	}

	// Test errors.Is compatibility
	if !errors.Is(richErr, baseErr) {
		t.Error("Expected errors.Is to work with wrapped error")
	}
}

func TestRichError_Chaining(t *testing.T) {
	baseErr := errors.New("test error")
	req := httptest.NewRequest("POST", "https://api.example.com/v1/chat", bytes.NewBufferString(`{"test": "data"}`))
	req.Header.Set("Authorization", "Bearer secret")

	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(bytes.NewBufferString(`{"error": "internal error"}`)),
	}

	startTime := time.Now().Add(-100 * time.Millisecond)

	richErr := NewRichError(baseErr).
		WithRequestID("req-123").
		WithCorrelationID("corr-456").
		WithProvider("anthropic").
		WithModel("claude-3-opus").
		WithOperation("chat_completion").
		WithTimingStart(startTime).
		WithRequestSnapshot(req).
		WithResponseSnapshot(resp)

	ctx := richErr.Context()

	if ctx.RequestID != "req-123" {
		t.Errorf("Expected RequestID req-123, got: %s", ctx.RequestID)
	}

	if ctx.CorrelationID != "corr-456" {
		t.Errorf("Expected CorrelationID corr-456, got: %s", ctx.CorrelationID)
	}

	if string(ctx.Provider) != "anthropic" {
		t.Errorf("Expected Provider anthropic, got: %s", ctx.Provider)
	}

	if ctx.Model != "claude-3-opus" {
		t.Errorf("Expected Model claude-3-opus, got: %s", ctx.Model)
	}

	if ctx.Operation != "chat_completion" {
		t.Errorf("Expected Operation chat_completion, got: %s", ctx.Operation)
	}

	if ctx.Duration <= 0 {
		t.Errorf("Expected positive duration, got: %s", ctx.Duration)
	}

	if ctx.Request == nil {
		t.Error("Expected request snapshot to be set")
	}

	if ctx.Response == nil {
		t.Error("Expected response snapshot to be set")
	}
}

func TestRichError_WithTiming(t *testing.T) {
	baseErr := errors.New("test error")
	richErr := NewRichError(baseErr)

	duration := 250 * time.Millisecond
	_ = richErr.WithTiming(duration)

	if richErr.context.Duration != duration {
		t.Errorf("Expected duration %s, got: %s", duration, richErr.context.Duration)
	}
}

func TestRichError_Format(t *testing.T) {
	baseErr := errors.New("test error message")
	req := httptest.NewRequest("GET", "https://api.anthropic.com/v1/messages", nil)
	req.Header.Set("X-API-Key", "sk-ant-secret")

	resp := &http.Response{
		StatusCode: 401,
		Header:     http.Header{},
		Body:       io.NopCloser(bytes.NewBufferString(`{"error": {"message": "Invalid API key"}}`)),
	}
	resp.Header.Set("Content-Type", "application/json")

	richErr := NewRichError(baseErr).
		WithRequestID("req-abc-123").
		WithCorrelationID("corr-xyz-789").
		WithProvider(types.ProviderTypeAnthropic).
		WithModel("claude-3-opus-20240229").
		WithOperation("create_message").
		WithTiming(150 * time.Millisecond).
		WithRequestSnapshot(req).
		WithResponseSnapshot(resp)

	formatted := richErr.Format()

	// Check that all key information is present
	expectedContains := []string{
		"Error: test error message",
		"Request ID: req-abc-123",
		"Correlation ID: corr-xyz-789",
		"Provider: anthropic",
		"Model: claude-3-opus-20240229",
		"Operation: create_message",
		"Duration:",
		"Method: GET",
		"URL: https://api.anthropic.com/v1/messages",
		"Status Code: 401",
	}

	for _, expected := range expectedContains {
		if !strings.Contains(formatted, expected) {
			t.Errorf("Expected formatted output to contain %q, got:\n%s", expected, formatted)
		}
	}

	// Check that sensitive data is masked
	if strings.Contains(formatted, "sk-ant-secret") {
		t.Errorf("Expected API key to be masked, got:\n%s", formatted)
	}
}

func TestRichError_FormatMinimal(t *testing.T) {
	baseErr := errors.New("minimal error")
	richErr := NewRichError(baseErr)

	formatted := richErr.Format()

	if !strings.Contains(formatted, "Error: minimal error") {
		t.Errorf("Expected formatted output to contain error message, got:\n%s", formatted)
	}

	// Should still have Context section even if mostly empty
	if !strings.Contains(formatted, "Context:") {
		t.Errorf("Expected formatted output to have Context section, got:\n%s", formatted)
	}
}

func TestRichError_String(t *testing.T) {
	baseErr := errors.New("test error")
	richErr := NewRichError(baseErr).
		WithRequestID("req-123")

	str := richErr.String()
	formatted := richErr.Format()

	if str != formatted {
		t.Error("Expected String() to return same as Format()")
	}
}

func TestWrap(t *testing.T) {
	t.Run("wrap error", func(t *testing.T) {
		baseErr := errors.New("test error")
		richErr := Wrap(baseErr)

		if richErr == nil {
			t.Fatal("Expected non-nil rich error")
		}

		if richErr.err != baseErr {
			t.Error("Expected base error to be wrapped")
		}
	})

	t.Run("wrap nil error", func(t *testing.T) {
		richErr := Wrap(nil)

		if richErr != nil {
			t.Error("Expected nil for wrapping nil error")
		}
	})
}

func TestWrapWithContext(t *testing.T) {
	t.Run("wrap with context", func(t *testing.T) {
		baseErr := errors.New("test error")
		ctx := NewErrorContext().
			WithRequestID("req-123").
			WithProvider(types.ProviderTypeOpenAI)

		richErr := WrapWithContext(baseErr, ctx)

		if richErr == nil {
			t.Fatal("Expected non-nil rich error")
		}

		if richErr.context.RequestID != "req-123" {
			t.Errorf("Expected RequestID from context, got: %s", richErr.context.RequestID)
		}

		if richErr.context.Provider != types.ProviderTypeOpenAI {
			t.Errorf("Expected Provider from context, got: %s", richErr.context.Provider)
		}
	})

	t.Run("wrap nil error with context", func(t *testing.T) {
		ctx := NewErrorContext()
		richErr := WrapWithContext(nil, ctx)

		if richErr != nil {
			t.Error("Expected nil for wrapping nil error")
		}
	})
}

func TestRichError_WithContext(t *testing.T) {
	baseErr := errors.New("test error")
	richErr := NewRichError(baseErr)

	customCtx := NewErrorContext().
		WithRequestID("custom-123").
		WithModel("custom-model")

	_ = richErr.WithContext(customCtx)

	if richErr.context != customCtx {
		t.Error("Expected context to be replaced")
	}

	if richErr.context.RequestID != "custom-123" {
		t.Errorf("Expected RequestID from custom context, got: %s", richErr.context.RequestID)
	}
}

func TestRichError_NilSnapshots(t *testing.T) {
	baseErr := errors.New("test error")
	richErr := NewRichError(baseErr).
		WithRequestSnapshot(nil).
		WithResponseSnapshot(nil)

	if richErr.context.Request != nil {
		t.Error("Expected request snapshot to be nil")
	}

	if richErr.context.Response != nil {
		t.Error("Expected response snapshot to be nil")
	}

	// Format should still work
	formatted := richErr.Format()
	if !strings.Contains(formatted, "Error: test error") {
		t.Errorf("Expected formatted output to work with nil snapshots, got:\n%s", formatted)
	}
}

func TestNewRichErrorWithConfig(t *testing.T) {
	baseErr := errors.New("test error")
	customConfig := &SnapshotConfig{
		MaxBodySize:    1024,
		IncludeHeaders: false,
		IncludeBody:    true,
		Masker:         NewCredentialMasker(),
	}

	richErr := NewRichErrorWithConfig(baseErr, customConfig)

	if richErr.snapshotConfig != customConfig {
		t.Error("Expected custom config to be used")
	}

	if richErr.snapshotConfig.MaxBodySize != 1024 {
		t.Errorf("Expected MaxBodySize 1024, got: %d", richErr.snapshotConfig.MaxBodySize)
	}

	if richErr.snapshotConfig.IncludeHeaders {
		t.Error("Expected IncludeHeaders to be false")
	}
}

func TestRichError_BodyTruncationInFormat(t *testing.T) {
	baseErr := errors.New("test error")
	largeBody := strings.Repeat("x", 10000)

	req := httptest.NewRequest("POST", "https://api.example.com/v1/chat", bytes.NewBufferString(largeBody))

	config := &SnapshotConfig{
		MaxBodySize:    100,
		IncludeHeaders: false,
		IncludeBody:    true,
		Masker:         DefaultCredentialMasker(),
	}

	richErr := NewRichErrorWithConfig(baseErr, config).
		WithRequestSnapshot(req)

	formatted := richErr.Format()

	if !strings.Contains(formatted, "[... truncated]") {
		t.Errorf("Expected truncation indicator in formatted output, got:\n%s", formatted)
	}
}
