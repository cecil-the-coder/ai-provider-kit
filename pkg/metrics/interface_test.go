package metrics

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestDefaultMetricsCollectorImplementsInterface verifies that DefaultMetricsCollector
// implements the MetricsCollector interface at compile time.
func TestDefaultMetricsCollectorImplementsInterface(t *testing.T) {
	// This test verifies that DefaultMetricsCollector implements types.MetricsCollector
	var _ types.MetricsCollector = (*DefaultMetricsCollector)(nil)
}

// TestSubscriptionImplementsInterface verifies that subscription
// implements the MetricsSubscription interface at compile time.
func TestSubscriptionImplementsInterface(t *testing.T) {
	// This test verifies that subscription implements types.MetricsSubscription
	var _ types.MetricsSubscription = (*subscription)(nil)
}
