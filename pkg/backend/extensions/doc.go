// Package extensions provides a plugin system for extending backend functionality.
// It defines the Extension interface for lifecycle management, route registration,
// and hooks for generation events, along with an ExtensionRegistry for managing
// multiple extensions.
//
// # Interceptor Pattern
//
// The package provides a ProviderInterceptor pattern for wrapping provider calls
// with cross-cutting concerns. Unlike hooks (which observe), interceptors control
// the execution flow and can modify requests/responses or short-circuit calls.
//
// Example usage:
//
//	// Create a chain of interceptors
//	chain := NewInterceptorChain()
//	chain.Add(NewLoggingInterceptor())
//	chain.Add(NewTimeoutInterceptor(5 * time.Second))
//	chain.Add(NewCachingInterceptor())
//
//	// Execute with the chain
//	resp, err := chain.Execute(ctx, req, providerFunc)
//
// Interceptors execute in order, each calling next() to proceed to the next interceptor
// or the actual provider. An interceptor can skip calling next() to short-circuit
// execution (e.g., return a cached response).
//
// The InterceptorRegistry provides a way to manage named interceptors globally:
//
//	registry := NewInterceptorRegistry()
//	registry.Register("logger", NewLoggingInterceptor())
//	registry.Register("timeout", NewTimeoutInterceptor(5 * time.Second))
//
// Example interceptors included in the test suite:
//   - LoggingInterceptor: Logs before/after provider calls
//   - TimeoutInterceptor: Enforces timeouts on provider calls
//   - CachingInterceptor: Caches responses based on request prompts
//   - MetricsInterceptor: Tracks call counts and durations
//
// # Per-Request Extension Configuration
//
// Extensions can be configured or disabled on a per-request basis using the
// "extension_config" metadata key in GenerateRequest. This allows fine-grained
// control over extension behavior for specific requests.
//
// Example usage:
//
//	// Create a request with per-extension configuration
//	req := &GenerateRequest{
//	    Prompt: "Hello, world!",
//	    Metadata: map[string]interface{}{
//	        "extension_config": map[string]interface{}{
//	            "caching": map[string]interface{}{
//	                "enabled": false,  // Disable caching for this request
//	            },
//	            "logging": map[string]interface{}{
//	                "enabled": true,
//	                "level":   "debug", // Use debug level logging
//	            },
//	            "metrics": map[string]interface{}{
//	                "interval": 30, // Custom metrics interval (enabled implicitly)
//	            },
//	        },
//	    },
//	}
//
// Extensions can check their configuration using helper functions:
//
//	func (e *MyCachingExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error {
//	    // Check if extension is enabled for this request
//	    if !IsExtensionEnabled(req.Metadata, e.Name()) {
//	        return nil // Skip processing
//	    }
//
//	    // Get custom configuration
//	    config, enabled := GetExtensionConfig(req.Metadata, e.Name())
//	    if enabled && config != nil {
//	        // Use custom config values
//	        if ttl, ok := config["ttl"].(int); ok {
//	            // Use custom TTL
//	        }
//	    }
//
//	    // Process request...
//	    return nil
//	}
//
// # Default Behavior
//
// By default, extensions are enabled unless explicitly disabled. An extension
// is considered enabled in the following cases:
//   - No metadata is provided
//   - No "extension_config" key in metadata
//   - No configuration for the specific extension
//   - Extension config exists without an "enabled" field
//   - Extension config has "enabled": true
//
// An extension is disabled only when:
//   - Extension config has "enabled": false
//
// # Security Considerations
//
// Security-critical extensions (e.g., authentication, authorization, rate limiting)
// should IGNORE disable requests and always execute their logic. These extensions
// should check the configuration but enforce their own security policies:
//
//	func (e *AuthExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error {
//	    // Always run authentication, regardless of request config
//	    // Security extensions should ignore the enabled flag
//	    return e.authenticate(ctx, req)
//	}
//
// # Backward Compatibility
//
// Extensions opt-in to checking per-request configuration. Existing extensions
// that don't check IsExtensionEnabled() will continue to work normally, processing
// all requests as before. This ensures backward compatibility with existing
// extension implementations.
package extensions
