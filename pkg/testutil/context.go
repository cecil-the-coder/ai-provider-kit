package testutil

import (
	"context"
	"testing"
	"time"
)

// TestContext creates a context with a reasonable timeout for tests.
// The default timeout is 30 seconds, which should be sufficient for most tests.
// Returns a context and a cancel function that should be deferred.
func TestContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// TestContextWithTimeout creates a context with a custom timeout for tests.
// Returns a context and a cancel function that should be deferred.
func TestContextWithTimeout(t *testing.T, timeout time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), timeout)
}

// TestContextWithDeadline creates a context with a specific deadline for tests.
// Returns a context and a cancel function that should be deferred.
func TestContextWithDeadline(t *testing.T, deadline time.Time) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithDeadline(context.Background(), deadline)
}

// TestContextWithCancel creates a cancellable context for tests.
// Returns a context and a cancel function that should be deferred.
func TestContextWithCancel(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithCancel(context.Background())
}

// ShortTestContext creates a context with a short timeout (5 seconds) for quick tests.
// Returns a context and a cancel function that should be deferred.
func ShortTestContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 5*time.Second)
}

// LongTestContext creates a context with a long timeout (2 minutes) for slow tests.
// Returns a context and a cancel function that should be deferred.
func LongTestContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 2*time.Minute)
}

// BackgroundContext returns a background context for tests that don't need timeouts.
// This is useful for tests that use manual cancellation or don't need timeout protection.
func BackgroundContext(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}

// ContextWithValue creates a test context with a key-value pair attached.
func ContextWithValue(t *testing.T, key, value interface{}) context.Context {
	t.Helper()
	return context.WithValue(context.Background(), key, value)
}

// ContextWithTimeout is a non-test-specific helper that creates a context with timeout.
// Unlike TestContext, this doesn't require a *testing.T parameter and can be used
// in helper functions or test utilities.
func ContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
