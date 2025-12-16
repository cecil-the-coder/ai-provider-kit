package metrics

import (
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Subscribe creates a new subscription with the given buffer size
func (c *DefaultMetricsCollector) Subscribe(bufferSize int) types.MetricsSubscription {
	return c.SubscribeFiltered(bufferSize, types.MetricFilter{})
}

// SubscribeFiltered creates a filtered subscription
func (c *DefaultMetricsCollector) SubscribeFiltered(bufferSize int, filter types.MetricFilter) types.MetricsSubscription {
	if c.closed.Load() {
		// Return a closed subscription
		sub := &subscription{
			id:     fmt.Sprintf("sub-closed-%d", c.nextSubID.Add(1)),
			events: make(chan types.MetricEvent),
		}
		close(sub.events)
		return sub
	}

	id := fmt.Sprintf("sub-%d", c.nextSubID.Add(1))
	sub := &subscription{
		id:        id,
		events:    make(chan types.MetricEvent, bufferSize),
		filter:    filter,
		collector: c,
	}

	c.mu.Lock()
	c.subscriptions[id] = sub
	c.mu.Unlock()

	return sub
}

// RegisterHook registers a hook and returns its ID
func (c *DefaultMetricsCollector) RegisterHook(hook types.MetricsHook) types.HookID {
	id := types.HookID(fmt.Sprintf("hook-%d", c.nextHookID.Add(1)))

	entry := &hookEntry{
		hook:   hook,
		id:     id,
		filter: hook.Filter(),
	}

	c.mu.Lock()
	c.hooks[id] = entry
	c.mu.Unlock()

	return id
}

// UnregisterHook removes a hook
func (c *DefaultMetricsCollector) UnregisterHook(id types.HookID) {
	c.mu.Lock()
	delete(c.hooks, id)
	c.mu.Unlock()
}
