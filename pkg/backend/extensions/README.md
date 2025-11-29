# Backend Extensions

Framework for adding custom functionality to the AI Provider Kit backend server.

## Overview

Extensions provide a way to hook into the request lifecycle and add custom behavior without modifying core backend code. Use extensions for:

- **Request/response modification** - Transform data before/after generation
- **Metrics and monitoring** - Track usage, performance, errors
- **Caching** - Cache responses to reduce API calls
- **Rate limiting** - Per-user or global rate limits
- **Content filtering** - Block inappropriate content
- **Custom routing** - Add domain-specific endpoints
- **Logging** - Advanced logging, audit trails
- **Provider selection** - Dynamic provider routing logic

## Extension Interface

All extensions implement the `Extension` interface:

```go
type Extension interface {
    // Metadata
    Name() string
    Version() string
    Description() string
    Dependencies() []string

    // Lifecycle
    Initialize(config map[string]interface{}) error
    Shutdown(ctx context.Context) error

    // Routes
    RegisterRoutes(registrar RouteRegistrar) error

    // Hooks - Generation lifecycle
    BeforeGenerate(ctx context.Context, req *GenerateRequest) error
    AfterGenerate(ctx context.Context, req *GenerateRequest, resp *GenerateResponse) error

    // Hooks - Provider events
    OnProviderError(ctx context.Context, provider types.Provider, err error) error
    OnProviderSelected(ctx context.Context, provider types.Provider) error
}
```

## Lifecycle Hooks

### Initialize

Called once during server startup with extension configuration.

```go
func (e *MyExtension) Initialize(config map[string]interface{}) error {
    e.apiKey = config["api_key"].(string)
    e.enabled = config["enabled"].(bool)
    return nil
}
```

**Use for:**
- Loading configuration
- Initializing clients/connections
- Setting up resources

### Shutdown

Called during graceful server shutdown with a timeout context.

```go
func (e *MyExtension) Shutdown(ctx context.Context) error {
    return e.db.Close()
}
```

**Use for:**
- Closing connections
- Flushing buffers
- Cleaning up resources

### RegisterRoutes

Called during server initialization to register custom HTTP routes.

```go
func (e *MyExtension) RegisterRoutes(r RouteRegistrar) error {
    r.HandleFunc("/api/metrics", e.handleMetrics)
    r.HandleFunc("/api/cache/clear", e.handleClearCache)
    return nil
}
```

**Use for:**
- Adding custom endpoints
- Exposing extension functionality via HTTP

## Generation Hooks

### BeforeGenerate

Called **before** sending the request to the provider. Can modify the request.

```go
func (e *MyExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error {
    // Add custom metadata
    if req.Metadata == nil {
        req.Metadata = make(map[string]interface{})
    }
    req.Metadata["request_id"] = generateID()
    req.Metadata["timestamp"] = time.Now()

    // Modify the prompt
    req.Prompt = sanitizePrompt(req.Prompt)

    return nil
}
```

**Use for:**
- Request validation
- Prompt modification
- Adding metadata
- Content filtering

**Return error to:** Abort the request

### AfterGenerate

Called **after** receiving response from provider. Can modify the response.

```go
func (e *MyExtension) AfterGenerate(ctx context.Context, req *GenerateRequest, resp *GenerateResponse) error {
    // Add metadata to response
    if resp.Metadata == nil {
        resp.Metadata = make(map[string]interface{})
    }
    resp.Metadata["cached"] = false
    resp.Metadata["processing_time_ms"] = time.Since(startTime).Milliseconds()

    // Modify content
    resp.Content = postProcess(resp.Content)

    return nil
}
```

**Use for:**
- Response modification
- Caching responses
- Recording metrics
- Content filtering

**Return error to:** Return error to client instead of response

## Provider Hooks

### OnProviderSelected

Called after a provider is selected but before the generation request.

```go
func (e *MyExtension) OnProviderSelected(ctx context.Context, provider types.Provider) error {
    // Record provider selection
    e.metrics.IncrementProviderUsage(provider.Name())

    // Check provider health
    if err := provider.HealthCheck(ctx); err != nil {
        return fmt.Errorf("selected provider unhealthy: %w", err)
    }

    return nil
}
```

**Use for:**
- Recording provider usage
- Provider health checks
- Provider-specific setup

**Return error to:** Abort the generation request

### OnProviderError

Called when a provider returns an error.

```go
func (e *MyExtension) OnProviderError(ctx context.Context, provider types.Provider, err error) error {
    // Log the error
    e.logger.Error("Provider error",
        "provider", provider.Name(),
        "error", err.Error())

    // Record metrics
    e.metrics.IncrementProviderErrors(provider.Name())

    // Send alert if error rate is high
    if e.metrics.ErrorRate(provider.Name()) > 0.5 {
        e.alerting.Send("High error rate for " + provider.Name())
    }

    return nil
}
```

**Use for:**
- Error logging
- Alerting
- Error metrics
- Error recovery

**Return value:** Not used (error already occurred)

## BaseExtension

Use `BaseExtension` to avoid implementing unused methods:

```go
type MyExtension struct {
    extensions.BaseExtension
    config map[string]interface{}
}

// Only implement what you need
func (e *MyExtension) Name() string { return "my-extension" }
func (e *MyExtension) Version() string { return "1.0.0" }
func (e *MyExtension) Description() string { return "My custom extension" }

func (e *MyExtension) Initialize(config map[string]interface{}) error {
    e.config = config
    return nil
}

func (e *MyExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error {
    // Custom logic
    return nil
}

// All other methods have default implementations from BaseExtension
```

## Example Extensions

### 1. Metrics Extension

Tracks request counts, latencies, and errors.

```go
package main

import (
    "context"
    "sync/atomic"
    "time"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/backend/extensions"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type MetricsExtension struct {
    extensions.BaseExtension
    requestCount  int64
    errorCount    int64
    totalLatency  int64
    providerStats map[string]*ProviderMetrics
}

type ProviderMetrics struct {
    Requests int64
    Errors   int64
}

func NewMetricsExtension() *MetricsExtension {
    return &MetricsExtension{
        providerStats: make(map[string]*ProviderMetrics),
    }
}

func (m *MetricsExtension) Name() string        { return "metrics" }
func (m *MetricsExtension) Version() string     { return "1.0.0" }
func (m *MetricsExtension) Description() string { return "Tracks request metrics" }

func (m *MetricsExtension) Initialize(config map[string]interface{}) error {
    return nil
}

func (m *MetricsExtension) BeforeGenerate(ctx context.Context, req *extensions.GenerateRequest) error {
    atomic.AddInt64(&m.requestCount, 1)

    // Store start time in metadata
    if req.Metadata == nil {
        req.Metadata = make(map[string]interface{})
    }
    req.Metadata["start_time"] = time.Now()

    return nil
}

func (m *MetricsExtension) AfterGenerate(ctx context.Context, req *extensions.GenerateRequest, resp *extensions.GenerateResponse) error {
    // Calculate latency
    if startTime, ok := req.Metadata["start_time"].(time.Time); ok {
        latency := time.Since(startTime).Milliseconds()
        atomic.AddInt64(&m.totalLatency, latency)
    }

    return nil
}

func (m *MetricsExtension) OnProviderSelected(ctx context.Context, provider types.Provider) error {
    stats, ok := m.providerStats[provider.Name()]
    if !ok {
        stats = &ProviderMetrics{}
        m.providerStats[provider.Name()] = stats
    }
    atomic.AddInt64(&stats.Requests, 1)
    return nil
}

func (m *MetricsExtension) OnProviderError(ctx context.Context, provider types.Provider, err error) error {
    atomic.AddInt64(&m.errorCount, 1)

    stats, ok := m.providerStats[provider.Name()]
    if ok {
        atomic.AddInt64(&stats.Errors, 1)
    }

    return nil
}

func (m *MetricsExtension) RegisterRoutes(r extensions.RouteRegistrar) error {
    r.HandleFunc("/api/metrics", m.handleMetrics)
    return nil
}

func (m *MetricsExtension) handleMetrics(w http.ResponseWriter, r *http.Request) {
    requests := atomic.LoadInt64(&m.requestCount)
    errors := atomic.LoadInt64(&m.errorCount)
    latency := atomic.LoadInt64(&m.totalLatency)

    avgLatency := int64(0)
    if requests > 0 {
        avgLatency = latency / requests
    }

    response := map[string]interface{}{
        "total_requests":       requests,
        "total_errors":         errors,
        "average_latency_ms":   avgLatency,
        "error_rate":           float64(errors) / float64(requests),
        "provider_stats":       m.providerStats,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

### 2. Caching Extension

Caches responses to reduce API calls and costs.

```go
package main

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "sync"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/backend/extensions"
)

type CachingExtension struct {
    extensions.BaseExtension
    cache map[string]*CacheEntry
    mu    sync.RWMutex
    ttl   time.Duration
}

type CacheEntry struct {
    Response  *extensions.GenerateResponse
    Timestamp time.Time
}

func NewCachingExtension() *CachingExtension {
    return &CachingExtension{
        cache: make(map[string]*CacheEntry),
        ttl:   5 * time.Minute,
    }
}

func (c *CachingExtension) Name() string        { return "cache" }
func (c *CachingExtension) Version() string     { return "1.0.0" }
func (c *CachingExtension) Description() string { return "Caches responses" }

func (c *CachingExtension) Initialize(config map[string]interface{}) error {
    if ttl, ok := config["ttl_seconds"].(int); ok {
        c.ttl = time.Duration(ttl) * time.Second
    }
    return nil
}

func (c *CachingExtension) BeforeGenerate(ctx context.Context, req *extensions.GenerateRequest) error {
    // Check cache
    key := c.cacheKey(req)

    c.mu.RLock()
    entry, exists := c.cache[key]
    c.mu.RUnlock()

    if exists && time.Since(entry.Timestamp) < c.ttl {
        // Cache hit - store in metadata to skip provider
        if req.Metadata == nil {
            req.Metadata = make(map[string]interface{})
        }
        req.Metadata["cached_response"] = entry.Response
    }

    return nil
}

func (c *CachingExtension) AfterGenerate(ctx context.Context, req *extensions.GenerateRequest, resp *extensions.GenerateResponse) error {
    // Check if response came from cache
    if cached, ok := req.Metadata["cached_response"].(*extensions.GenerateResponse); ok {
        // Use cached response
        *resp = *cached
        if resp.Metadata == nil {
            resp.Metadata = make(map[string]interface{})
        }
        resp.Metadata["from_cache"] = true
        return nil
    }

    // Store in cache
    key := c.cacheKey(req)
    c.mu.Lock()
    c.cache[key] = &CacheEntry{
        Response:  resp,
        Timestamp: time.Now(),
    }
    c.mu.Unlock()

    if resp.Metadata == nil {
        resp.Metadata = make(map[string]interface{})
    }
    resp.Metadata["from_cache"] = false

    return nil
}

func (c *CachingExtension) cacheKey(req *extensions.GenerateRequest) string {
    data, _ := json.Marshal(map[string]interface{}{
        "provider": req.Provider,
        "model":    req.Model,
        "prompt":   req.Prompt,
        "temp":     req.Temperature,
        "max":      req.MaxTokens,
    })

    hash := sha256.Sum256(data)
    return hex.EncodeToString(hash[:])
}

func (c *CachingExtension) RegisterRoutes(r extensions.RouteRegistrar) error {
    r.HandleFunc("/api/cache/clear", c.handleClearCache)
    r.HandleFunc("/api/cache/stats", c.handleStats)
    return nil
}

func (c *CachingExtension) handleClearCache(w http.ResponseWriter, r *http.Request) {
    c.mu.Lock()
    c.cache = make(map[string]*CacheEntry)
    c.mu.Unlock()

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

func (c *CachingExtension) handleStats(w http.ResponseWriter, r *http.Request) {
    c.mu.RLock()
    size := len(c.cache)
    c.mu.RUnlock()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "entries": size,
        "ttl_seconds": int(c.ttl.Seconds()),
    })
}
```

### 3. Content Filter Extension

Filters inappropriate content from requests and responses.

```go
package main

import (
    "context"
    "regexp"
    "strings"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/backend/extensions"
)

type ContentFilterExtension struct {
    extensions.BaseExtension
    blockedPatterns []*regexp.Regexp
    replacements    map[string]string
}

func NewContentFilterExtension() *ContentFilterExtension {
    return &ContentFilterExtension{
        blockedPatterns: []*regexp.Regexp{
            regexp.MustCompile(`(?i)offensive-word-1`),
            regexp.MustCompile(`(?i)offensive-word-2`),
        },
        replacements: map[string]string{
            "badword": "***",
        },
    }
}

func (f *ContentFilterExtension) Name() string        { return "content-filter" }
func (f *ContentFilterExtension) Version() string     { return "1.0.0" }
func (f *ContentFilterExtension) Description() string { return "Filters inappropriate content" }

func (f *ContentFilterExtension) Initialize(config map[string]interface{}) error {
    // Load custom patterns from config
    if patterns, ok := config["blocked_patterns"].([]string); ok {
        for _, pattern := range patterns {
            if re, err := regexp.Compile(pattern); err == nil {
                f.blockedPatterns = append(f.blockedPatterns, re)
            }
        }
    }
    return nil
}

func (f *ContentFilterExtension) BeforeGenerate(ctx context.Context, req *extensions.GenerateRequest) error {
    // Check for blocked content
    for _, pattern := range f.blockedPatterns {
        if pattern.MatchString(req.Prompt) {
            return fmt.Errorf("request contains inappropriate content")
        }
    }

    // Apply replacements
    for bad, good := range f.replacements {
        req.Prompt = strings.ReplaceAll(req.Prompt, bad, good)
    }

    return nil
}

func (f *ContentFilterExtension) AfterGenerate(ctx context.Context, req *extensions.GenerateRequest, resp *extensions.GenerateResponse) error {
    // Filter response content
    for _, pattern := range f.blockedPatterns {
        if pattern.MatchString(resp.Content) {
            return fmt.Errorf("response contains inappropriate content")
        }
    }

    // Apply replacements to response
    for bad, good := range f.replacements {
        resp.Content = strings.ReplaceAll(resp.Content, bad, good)
    }

    return nil
}
```

## Registration

### Programmatic Registration

```go
package main

import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/backend"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
)

func main() {
    config := backendtypes.BackendConfig{
        Server: backendtypes.ServerConfig{
            Host: "0.0.0.0",
            Port: 8080,
        },
    }

    server := backend.NewServer(config, providers)

    // Register extensions
    server.RegisterExtension(NewMetricsExtension())
    server.RegisterExtension(NewCachingExtension())
    server.RegisterExtension(NewContentFilterExtension())

    server.Start()
}
```

### Configuration-Based Registration

```yaml
extensions:
  metrics:
    enabled: true
    config: {}

  cache:
    enabled: true
    config:
      ttl_seconds: 300

  content-filter:
    enabled: true
    config:
      blocked_patterns:
        - "(?i)spam"
        - "(?i)inappropriate"
```

```go
config := backendtypes.BackendConfig{
    Extensions: map[string]backendtypes.ExtensionConfig{
        "metrics": {
            Enabled: true,
            Config:  map[string]interface{}{},
        },
        "cache": {
            Enabled: true,
            Config: map[string]interface{}{
                "ttl_seconds": 300,
            },
        },
    },
}

server := backend.NewServer(config, providers)
// Extensions are automatically initialized from config
```

## Extension Dependencies

Extensions can declare dependencies on other extensions:

```go
type MyExtension struct {
    extensions.BaseExtension
    metrics *MetricsExtension
}

func (e *MyExtension) Dependencies() []string {
    return []string{"metrics"}
}

func (e *MyExtension) Initialize(config map[string]interface{}) error {
    // Get metrics extension from registry
    registry := config["registry"].(extensions.ExtensionRegistry)
    metricsExt, _ := registry.Get("metrics")
    e.metrics = metricsExt.(*MetricsExtension)

    return nil
}
```

The registry ensures extensions are initialized in dependency order.

## Best Practices

1. **Keep extensions focused** - One responsibility per extension
2. **Use BaseExtension** - Only implement hooks you need
3. **Handle errors gracefully** - Don't crash the server
4. **Make extensions configurable** - Use Initialize() config
5. **Add custom routes** - Expose extension functionality
6. **Thread safety** - Use mutexes for shared state
7. **Document configuration** - Explain config options
8. **Test thoroughly** - Extensions can break the server
9. **Version your extensions** - Track compatibility
10. **Clean up resources** - Use Shutdown() properly

## Common Patterns

### Conditional Execution

```go
func (e *MyExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error {
    // Only run for specific providers
    if req.Provider != "openai" {
        return nil
    }

    // Only run for specific models
    if !strings.HasPrefix(req.Model, "gpt-4") {
        return nil
    }

    // Custom logic
    return nil
}
```

### Metadata Communication

```go
// In BeforeGenerate
req.Metadata["custom_flag"] = true

// In AfterGenerate
if req.Metadata["custom_flag"].(bool) {
    // Custom processing
}
```

### Error Handling

```go
func (e *MyExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error {
    if err := e.validate(req); err != nil {
        // Log but don't fail the request
        log.Printf("Validation warning: %v", err)
        return nil
    }

    if err := e.criticalCheck(req); err != nil {
        // Abort the request
        return fmt.Errorf("critical error: %w", err)
    }

    return nil
}
```

## Troubleshooting

### Extension not called

- Check if extension is registered
- Verify extension is enabled in config
- Ensure Initialize() succeeded
- Check logs for initialization errors

### Extension causing errors

- Add logging to track execution
- Return nil from hooks unless aborting is required
- Check for nil pointer dereferences
- Verify thread safety with concurrent requests

### Extension slowing down requests

- Profile extension code
- Minimize work in BeforeGenerate/AfterGenerate
- Use goroutines for non-blocking operations
- Consider caching expensive operations

### Dependencies not working

- Verify dependency names match exactly
- Check initialization order in logs
- Ensure dependent extension is registered
- Use registry.Get() to access dependencies

## Testing Extensions

```go
package main

import (
    "context"
    "testing"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/backend/extensions"
)

func TestMetricsExtension(t *testing.T) {
    ext := NewMetricsExtension()

    // Test initialization
    err := ext.Initialize(map[string]interface{}{})
    if err != nil {
        t.Fatalf("Initialize failed: %v", err)
    }

    // Test BeforeGenerate
    req := &extensions.GenerateRequest{
        Prompt: "test",
    }

    err = ext.BeforeGenerate(context.Background(), req)
    if err != nil {
        t.Fatalf("BeforeGenerate failed: %v", err)
    }

    // Verify request count
    if ext.requestCount != 1 {
        t.Errorf("Expected 1 request, got %d", ext.requestCount)
    }
}
```

## Further Reading

- [Backend README](../README.md) - Main backend documentation
- [Virtual Providers](../../providers/virtual/README.md) - Virtual provider documentation
- [Examples](../../../examples/extensions/) - Extension examples
