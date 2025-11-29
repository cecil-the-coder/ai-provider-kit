package metrics

import (
	"sync"
	"sync/atomic"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// subscription implements types.MetricsSubscription
type subscription struct {
	id            string
	events        chan types.MetricEvent
	filter        types.MetricFilter
	overflowCount atomic.Int64
	collector     *DefaultMetricsCollector
	closed        atomic.Bool
	mu            sync.Mutex
}

// Events returns the channel for receiving metrics events
func (s *subscription) Events() <-chan types.MetricEvent {
	return s.events
}

// Unsubscribe closes the subscription and stops event delivery
func (s *subscription) Unsubscribe() {
	if !s.closed.CompareAndSwap(false, true) {
		return // Already unsubscribed
	}

	// Remove from collector
	if s.collector != nil {
		s.collector.mu.Lock()
		delete(s.collector.subscriptions, s.id)
		s.collector.mu.Unlock()
	}

	// Close the channel
	s.close()
}

// ID returns the unique identifier for this subscription
func (s *subscription) ID() string {
	return s.id
}

// OverflowCount returns the number of events dropped due to buffer overflow
func (s *subscription) OverflowCount() int64 {
	return s.overflowCount.Load()
}

// publish sends an event to the subscription if it matches the filter
func (s *subscription) publish(event types.MetricEvent) {
	if s.closed.Load() {
		return
	}

	// Check filter
	if !s.filter.Matches(event) {
		return
	}

	// Non-blocking send
	select {
	case s.events <- event:
		// Event sent successfully
	default:
		// Buffer is full, drop event and increment overflow counter
		s.overflowCount.Add(1)
	}
}

// close closes the event channel
func (s *subscription) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close channel if not already closed
	select {
	case <-s.events:
		// Already closed
	default:
		close(s.events)
	}
}
