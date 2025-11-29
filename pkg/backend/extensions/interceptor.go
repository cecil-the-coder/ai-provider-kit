package extensions

import (
	"context"
	"fmt"
	"sync"
)

// ProviderFunc is the signature for a provider call that can be intercepted.
// It takes a context and request, and returns a response or error.
type ProviderFunc func(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)

// ProviderInterceptor defines the interface for intercepting provider calls.
// Interceptors can modify requests, responses, or control the flow of execution.
// They must call next() to proceed to the next interceptor or the actual provider.
type ProviderInterceptor interface {
	// Intercept processes a provider call and optionally calls next to proceed.
	// Interceptors can:
	// - Modify the request before calling next
	// - Modify the response after calling next
	// - Skip calling next to short-circuit (e.g., return cached response)
	// - Handle errors from next
	Intercept(ctx context.Context, req *GenerateRequest, next ProviderFunc) (*GenerateResponse, error)
}

// InterceptorChain manages a chain of interceptors and executes them in order.
type InterceptorChain struct {
	mu           sync.RWMutex
	interceptors []ProviderInterceptor
}

// NewInterceptorChain creates a new empty interceptor chain.
func NewInterceptorChain() *InterceptorChain {
	return &InterceptorChain{
		interceptors: make([]ProviderInterceptor, 0),
	}
}

// Add appends an interceptor to the chain.
// Interceptors are executed in the order they are added.
func (c *InterceptorChain) Add(interceptor ProviderInterceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.interceptors = append(c.interceptors, interceptor)
}

// Execute runs the interceptor chain followed by the provider function.
// Each interceptor is called in order, and each must call next() to proceed.
// If an interceptor doesn't call next(), execution is short-circuited.
func (c *InterceptorChain) Execute(ctx context.Context, req *GenerateRequest, provider ProviderFunc) (*GenerateResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.interceptors) == 0 {
		return provider(ctx, req)
	}

	// Build the chain from the end backwards
	chain := provider
	for i := len(c.interceptors) - 1; i >= 0; i-- {
		interceptor := c.interceptors[i]
		currentChain := chain
		chain = func(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
			return interceptor.Intercept(ctx, req, currentChain)
		}
	}

	return chain(ctx, req)
}

// InterceptorRegistry manages named interceptors for the application.
type InterceptorRegistry interface {
	// Register adds an interceptor to the registry with a unique name.
	Register(name string, interceptor ProviderInterceptor) error

	// Get retrieves an interceptor by name.
	Get(name string) (ProviderInterceptor, bool)

	// List returns all registered interceptor names.
	List() []string

	// Unregister removes an interceptor from the registry.
	Unregister(name string) error
}

// interceptorRegistry implements InterceptorRegistry.
type interceptorRegistry struct {
	mu           sync.RWMutex
	interceptors map[string]ProviderInterceptor
	order        []string
}

// NewInterceptorRegistry creates a new interceptor registry.
func NewInterceptorRegistry() InterceptorRegistry {
	return &interceptorRegistry{
		interceptors: make(map[string]ProviderInterceptor),
		order:        make([]string, 0),
	}
}

// Register adds an interceptor to the registry with a unique name.
func (r *interceptorRegistry) Register(name string, interceptor ProviderInterceptor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.interceptors[name]; exists {
		return fmt.Errorf("interceptor %s already registered", name)
	}

	r.interceptors[name] = interceptor
	r.order = append(r.order, name)
	return nil
}

// Get retrieves an interceptor by name.
func (r *interceptorRegistry) Get(name string) (ProviderInterceptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	interceptor, ok := r.interceptors[name]
	return interceptor, ok
}

// List returns all registered interceptor names in registration order.
func (r *interceptorRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, len(r.order))
	copy(result, r.order)
	return result
}

// Unregister removes an interceptor from the registry.
func (r *interceptorRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.interceptors[name]; !exists {
		return fmt.Errorf("interceptor %s not found", name)
	}

	delete(r.interceptors, name)

	// Remove from order
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}

	return nil
}
