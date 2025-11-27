# AI Provider Kit - Troubleshooting and FAQ

Comprehensive troubleshooting guide and frequently asked questions for the AI Provider Kit SDK.

## Table of Contents

- [Common Issues and Solutions](#common-issues-and-solutions)
- [Debugging Guide](#debugging-guide)
- [Error Reference](#error-reference)
- [Performance Issues](#performance-issues)
- [Configuration FAQ](#configuration-faq)
- [Authentication FAQ](#authentication-faq)
- [Provider-Specific FAQ](#provider-specific-faq)
- [Integration FAQ](#integration-faq)
- [Advanced Scenarios](#advanced-scenarios)
- [Getting Help](#getting-help)

---

## Common Issues and Solutions

### Authentication Failures

#### Issue: "invalid_credentials" error

**Symptoms:**
```
Provider authentication error: Invalid credentials (invalid_credentials)
```

**Causes:**
- Incorrect API key or OAuth credentials
- Expired access token
- Revoked credentials
- Wrong credential format

**Solutions:**

1. **Verify API Key Format:**
```go
// Check if API key is properly formatted
config := types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "sk-...", // OpenAI keys start with "sk-"
}

// For Anthropic
config := types.ProviderConfig{
    Type:   types.ProviderTypeAnthropic,
    APIKey: "sk-ant-...", // Anthropic keys start with "sk-ant-"
}
```

2. **Test credentials directly:**
```bash
# OpenAI
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"

# Anthropic
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01"
```

3. **Check environment variables:**
```bash
# Verify environment variable is set
echo $OPENAI_API_KEY

# Verify it's being loaded correctly
export OPENAI_API_KEY="your-key-here"
```

#### Issue: OAuth token expired

**Symptoms:**
```
Provider authentication error: Stored token expired and refresh failed (token_expired)
```

**Causes:**
- Access token has expired
- Refresh token is invalid or expired
- Token refresh is disabled

**Solutions:**

1. **Enable automatic token refresh:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/auth"

config := &auth.OAuthConfig{
    Refresh: auth.RefreshConfig{
        Enabled: true,
        Buffer:  5 * time.Minute, // Refresh 5min before expiry
    },
}

authenticator := auth.NewOAuthAuthenticator("gemini", storage, config)
```

2. **Manually refresh token:**
```go
if !authenticator.IsAuthenticated() {
    if err := authenticator.RefreshToken(ctx); err != nil {
        // Token refresh failed, need to re-authenticate
        log.Printf("Token refresh failed: %v", err)
        // Initiate new OAuth flow
        authURL, _ := authenticator.StartOAuthFlow(ctx, scopes)
        fmt.Printf("Please visit: %s\n", authURL)
    }
}
```

3. **Re-authenticate from scratch:**
```go
// Clear stored token and start fresh
authenticator.Logout(ctx)

// Start new OAuth flow
authURL, err := authenticator.StartOAuthFlow(ctx, []string{
    "https://www.googleapis.com/auth/cloud-platform",
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Visit this URL to authenticate: %s\n", authURL)
```

### Rate Limit Errors

#### Issue: 429 Too Many Requests

**Symptoms:**
- HTTP 429 status code
- "Rate limit exceeded" messages
- Requests failing intermittently

**Causes:**
- Exceeding provider rate limits
- Insufficient delay between requests
- No rate limiting configured

**Solutions:**

1. **Configure rate limiting:**
```go
config := types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "your-api-key",
    RateLimit: types.RateLimitConfig{
        RequestsPerMinute: 60,
        TokensPerMinute:   90000,
        Enabled:          true,
    },
}
```

2. **Use multiple API keys for higher throughput:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/keymanager"

keys := []string{
    "key-1",
    "key-2",
    "key-3",
}

manager := keymanager.NewKeyManager("OpenAI", keys)

// Keys are automatically rotated on each request
result, err := manager.ExecuteWithFailover(ctx,
    func(ctx context.Context, key string) (string, error) {
        return makeAPICall(ctx, key)
    })
```

3. **Implement exponential backoff:**
```go
import "time"

func retryWithBackoff(operation func() error, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        err := operation()
        if err == nil {
            return nil
        }

        // Check if it's a rate limit error
        if isRateLimitError(err) {
            backoff := time.Duration(1<<uint(i)) * time.Second
            if backoff > 60*time.Second {
                backoff = 60 * time.Second
            }
            time.Sleep(backoff)
            continue
        }

        return err
    }
    return fmt.Errorf("max retries exceeded")
}
```

### Timeout Issues

#### Issue: Request timeout

**Symptoms:**
```
context deadline exceeded
request timeout
```

**Causes:**
- Network latency
- Long-running model inference
- Small timeout value

**Solutions:**

1. **Increase timeout:**
```go
config := types.ProviderConfig{
    Type:    types.ProviderTypeOpenAI,
    APIKey:  "your-api-key",
    Timeout: 60 * time.Second, // Increase from default 30s
}
```

2. **Use context with timeout:**
```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()

stream, err := provider.GenerateChatCompletion(ctx, options)
```

3. **Enable streaming for long responses:**
```go
options := types.GenerateOptions{
    Prompt: "Long prompt...",
    Stream: true, // Get partial results immediately
}

stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

for {
    chunk, err := stream.Next()
    if err != nil {
        break
    }
    if chunk.Done {
        break
    }
    fmt.Print(chunk.Content)
}
```

### Connection Problems

#### Issue: Network connection failed

**Symptoms:**
```
dial tcp: i/o timeout
connection refused
no such host
```

**Causes:**
- Network connectivity issues
- Firewall blocking requests
- Incorrect base URL
- DNS resolution problems

**Solutions:**

1. **Test basic connectivity:**
```bash
# Test DNS resolution
nslookup api.openai.com

# Test HTTP connectivity
curl -I https://api.openai.com/v1/models

# Check for proxy issues
curl -x http://proxy:port https://api.openai.com/v1/models
```

2. **Configure custom HTTP client:**
```go
import (
    "net/http"
    "time"
)

httpClient := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        // Configure proxy if needed
        Proxy: http.ProxyFromEnvironment,
    },
}

// Use with provider (if supported by implementation)
```

3. **Verify base URL:**
```go
config := types.ProviderConfig{
    Type:    types.ProviderTypeOpenAI,
    APIKey:  "your-api-key",
    BaseURL: "https://api.openai.com/v1", // Ensure correct URL
}

// For custom endpoints
config := types.ProviderConfig{
    Type:    types.ProviderTypeOpenAI,
    APIKey:  "your-api-key",
    BaseURL: "https://your-custom-endpoint.com/v1",
}
```

### Token Refresh Failures

#### Issue: OAuth token refresh failed

**Symptoms:**
```
Token refresh failed with status 400
refresh_failed
```

**Causes:**
- Invalid refresh token
- Refresh token expired
- OAuth configuration incorrect
- Client credentials invalid

**Solutions:**

1. **Verify OAuth configuration:**
```go
config := &auth.OAuthConfig{
    Refresh: auth.RefreshConfig{
        Enabled: true,
        Buffer:  5 * time.Minute,
    },
    HTTP: auth.HTTPConfig{
        Timeout:   30 * time.Second,
        UserAgent: "AI-Provider-Kit/1.0",
    },
}
```

2. **Check credential validity:**
```go
tokenInfo, err := authenticator.GetTokenInfo()
if err != nil {
    log.Printf("Token info error: %v", err)
}

if tokenInfo.IsExpired {
    log.Println("Token is expired")
    if tokenInfo.RefreshToken == "" {
        log.Println("No refresh token available - need to re-authenticate")
    }
}
```

3. **Monitor refresh operations:**
```go
// Set up callback for token refresh events
cred := &types.OAuthCredentialSet{
    OnTokenRefresh: func(id, access, refresh string, expires time.Time) error {
        log.Printf("Token refreshed for %s, expires at %s", id, expires)
        // Persist new tokens
        return saveTokensToStorage(id, access, refresh, expires)
    },
}
```

### Configuration Errors

#### Issue: Invalid configuration

**Symptoms:**
```
invalid_config
required field missing
```

**Causes:**
- Missing required fields
- Invalid field values
- Type mismatches

**Solutions:**

1. **Validate configuration:**
```go
func validateConfig(config types.ProviderConfig) error {
    if config.Type == "" {
        return fmt.Errorf("provider type is required")
    }

    if config.APIKey == "" && len(config.OAuthCredentials) == 0 {
        return fmt.Errorf("either APIKey or OAuthCredentials required")
    }

    if config.Timeout < 0 {
        return fmt.Errorf("timeout must be positive")
    }

    return nil
}
```

2. **Use configuration builder:**
```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    Name:         "openai-primary",
    APIKey:       os.Getenv("OPENAI_API_KEY"),
    DefaultModel: "gpt-4",
    Timeout:      30 * time.Second,
    MaxRetries:   3,
}

if err := validateConfig(config); err != nil {
    log.Fatalf("Invalid config: %v", err)
}
```

3. **Load from YAML with validation:**
```yaml
# config.yaml
providers:
  openai:
    enabled: true
    api_key: ${OPENAI_API_KEY}
    model: gpt-4
    timeout: 30
    max_retries: 3
```

```go
import (
    "gopkg.in/yaml.v3"
    "os"
)

type Config struct {
    Providers map[string]ProviderSettings `yaml:"providers"`
}

var config Config
data, _ := os.ReadFile("config.yaml")
yaml.Unmarshal(data, &config)
```

### Provider-Specific Errors

#### OpenAI: Model not found

**Symptoms:**
```
model not found
invalid model
```

**Solution:**
```go
// Verify model name
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    DefaultModel: "gpt-4-turbo-preview", // Use exact model name
}

// Or override per request
options := types.GenerateOptions{
    Model:  "gpt-3.5-turbo", // Per-request override
    Prompt: "Hello world",
}
```

#### Anthropic: Missing required headers

**Symptoms:**
```
missing anthropic-version header
missing x-api-key header
```

**Solution:**
```go
config := types.ProviderConfig{
    Type:   types.ProviderTypeAnthropic,
    APIKey: "sk-ant-...",
    Headers: map[string]string{
        "anthropic-version": "2023-06-01",
    },
}
```

#### Gemini: Project ID required

**Symptoms:**
```
project not found
invalid project ID
```

**Solution:**
```go
config := types.ProviderConfig{
    Type: types.ProviderTypeGemini,
    Headers: map[string]string{
        "x-goog-user-project": "your-project-id",
    },
}
```

---

## Debugging Guide

### Enabling Debug Logging

#### Standard Library Logging

```go
import (
    "log"
    "os"
)

// Set log level
log.SetFlags(log.LstdFlags | log.Lshortfile)
log.SetOutput(os.Stdout)

// Enable verbose logging
log.SetPrefix("[DEBUG] ")
```

#### Structured Logging

```go
import (
    "go.uber.org/zap"
)

// Development logger (verbose)
logger, _ := zap.NewDevelopment()
defer logger.Sync()

logger.Info("Creating provider",
    zap.String("type", "openai"),
    zap.String("model", "gpt-4"),
)

// Production logger
logger, _ := zap.NewProduction()
```

### Request/Response Inspection

#### Log HTTP requests

```go
import (
    "net/http"
    "net/http/httputil"
)

type LoggingTransport struct {
    Transport http.RoundTripper
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    // Log request
    reqDump, _ := httputil.DumpRequestOut(req, true)
    log.Printf("REQUEST:\n%s", reqDump)

    // Make request
    resp, err := t.Transport.RoundTrip(req)
    if err != nil {
        return nil, err
    }

    // Log response
    respDump, _ := httputil.DumpResponse(resp, true)
    log.Printf("RESPONSE:\n%s", respDump)

    return resp, nil
}

// Use with HTTP client
client := &http.Client{
    Transport: &LoggingTransport{
        Transport: http.DefaultTransport,
    },
}
```

#### Inspect streaming responses

```go
stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

chunkCount := 0
for {
    chunk, err := stream.Next()
    if err != nil {
        log.Printf("Stream error: %v", err)
        break
    }

    chunkCount++
    log.Printf("Chunk %d: Done=%v, Content=%q",
        chunkCount, chunk.Done, chunk.Content)

    if chunk.Done {
        log.Printf("Total chunks: %d", chunkCount)
        log.Printf("Final usage: %+v", chunk.Usage)
        break
    }
}
```

### Error Message Interpretation

#### Understanding AuthError

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/auth"

func handleAuthError(err error) {
    if authErr, ok := err.(*auth.AuthError); ok {
        log.Printf("Provider: %s", authErr.Provider)
        log.Printf("Error Code: %s", authErr.Code)
        log.Printf("Message: %s", authErr.Message)
        log.Printf("Details: %s", authErr.Details)
        log.Printf("Retryable: %v", authErr.IsRetryable())

        switch authErr.Code {
        case auth.ErrCodeTokenExpired:
            // Handle token expiration
            log.Println("Token expired, attempting refresh...")
        case auth.ErrCodeNetworkError:
            // Handle network error
            if authErr.IsRetryable() {
                log.Println("Network error - will retry")
            }
        case auth.ErrCodeInvalidCredentials:
            // Handle invalid credentials
            log.Println("Invalid credentials - check configuration")
        }
    }
}
```

#### Extracting useful information from errors

```go
func diagnoseError(err error) {
    log.Printf("Error: %v", err)
    log.Printf("Error type: %T", err)

    // Check for wrapped errors
    if unwrapped := errors.Unwrap(err); unwrapped != nil {
        log.Printf("Underlying error: %v", unwrapped)
    }

    // Check for specific error types
    switch e := err.(type) {
    case *auth.AuthError:
        log.Printf("Auth error code: %s", e.Code)
    case *url.Error:
        log.Printf("URL error on: %s", e.URL)
    case net.Error:
        if e.Timeout() {
            log.Println("Network timeout occurred")
        }
        if e.Temporary() {
            log.Println("Temporary network error")
        }
    }
}
```

### Stack Trace Analysis

```go
import (
    "runtime/debug"
)

func captureStackTrace() {
    stack := debug.Stack()
    log.Printf("Stack trace:\n%s", stack)
}

// Defer stack trace on panic
defer func() {
    if r := recover(); r != nil {
        log.Printf("Panic: %v", r)
        log.Printf("Stack trace:\n%s", debug.Stack())
    }
}()
```

### Performance Debugging

#### Track request latency

```go
func trackLatency(provider types.Provider) {
    start := time.Now()

    stream, err := provider.GenerateChatCompletion(ctx, options)
    if err != nil {
        log.Printf("Request failed after %v: %v", time.Since(start), err)
        return
    }
    defer stream.Close()

    firstChunkTime := time.Time{}
    for {
        chunk, err := stream.Next()
        if err != nil {
            break
        }

        if firstChunkTime.IsZero() && chunk.Content != "" {
            firstChunkTime = time.Now()
            log.Printf("Time to first chunk: %v", time.Since(start))
        }

        if chunk.Done {
            log.Printf("Total request time: %v", time.Since(start))
            break
        }
    }
}
```

#### Monitor memory usage

```go
import "runtime"

func printMemStats() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    log.Printf("Alloc = %v MB", m.Alloc / 1024 / 1024)
    log.Printf("TotalAlloc = %v MB", m.TotalAlloc / 1024 / 1024)
    log.Printf("Sys = %v MB", m.Sys / 1024 / 1024)
    log.Printf("NumGC = %v", m.NumGC)
}

// Monitor periodically
ticker := time.NewTicker(10 * time.Second)
go func() {
    for range ticker.C {
        printMemStats()
    }
}()
```

### Memory Leak Detection

#### Use pprof for profiling

```go
import (
    _ "net/http/pprof"
    "net/http"
)

// Start pprof server
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

```bash
# Analyze heap profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Analyze goroutines
go tool pprof http://localhost:6060/debug/pprof/goroutine

# Generate heap profile
curl http://localhost:6060/debug/pprof/heap > heap.prof
go tool pprof -http=:8080 heap.prof
```

#### Check for goroutine leaks

```go
func checkGoroutines() {
    initial := runtime.NumGoroutine()
    log.Printf("Initial goroutines: %d", initial)

    // Run your code
    runTest()

    // Force garbage collection
    runtime.GC()
    time.Sleep(1 * time.Second)

    final := runtime.NumGoroutine()
    log.Printf("Final goroutines: %d", final)

    if final > initial {
        log.Printf("WARNING: Possible goroutine leak (+%d)", final-initial)
    }
}
```

---

## Error Reference

### Authentication Error Codes

| Error Code | Description | Retryable | Common Causes |
|------------|-------------|-----------|---------------|
| `invalid_credentials` | Invalid API key or OAuth credentials | No | Wrong credentials, expired keys |
| `token_expired` | Access token has expired | Yes | Token TTL reached, refresh needed |
| `refresh_failed` | Token refresh operation failed | Yes | Invalid refresh token, network error |
| `oauth_flow_failed` | OAuth authorization flow failed | No | Invalid state, callback error |
| `invalid_config` | Configuration validation failed | No | Missing fields, invalid values |
| `network_error` | Network communication error | Yes | Timeout, connection refused |
| `provider_unavailable` | Provider not available | Yes | Service outage, rate limiting |
| `scope_insufficient` | Insufficient OAuth scopes | No | Missing required scopes |
| `key_rotation_failed` | API key rotation failed | Yes | All keys unhealthy |
| `all_keys_exhausted` | All API keys are unavailable | Yes | Rate limits, invalid keys |
| `storage_error` | Token storage operation failed | Yes | File I/O error, permission denied |
| `encryption_error` | Token encryption/decryption failed | No | Invalid encryption key |

### Error Resolution Steps

#### invalid_credentials

**Steps to resolve:**
1. Verify credential format matches provider requirements
2. Check credential hasn't been revoked
3. Ensure environment variables are set correctly
4. Test credential with provider's API directly
5. Regenerate credential if necessary

**Prevention:**
- Use environment variables or secure storage
- Implement credential rotation
- Monitor credential expiration
- Set up alerts for authentication failures

#### token_expired

**Steps to resolve:**
1. Enable automatic token refresh
2. Check refresh token is valid
3. Verify token storage is working
4. Implement proper buffer time before expiry
5. Re-authenticate if refresh fails

**Prevention:**
- Set refresh buffer to 5-10 minutes
- Monitor token expiration times
- Persist refresh tokens securely
- Implement token refresh callbacks

#### network_error

**Steps to resolve:**
1. Check network connectivity
2. Verify firewall rules
3. Test DNS resolution
4. Check proxy settings
5. Increase timeout values
6. Implement retry logic with exponential backoff

**Prevention:**
- Configure appropriate timeouts
- Use connection pooling
- Implement circuit breakers
- Monitor network metrics

#### all_keys_exhausted

**Steps to resolve:**
1. Check health status of all keys
2. Verify rate limits aren't exceeded
3. Wait for backoff periods to expire
4. Add more API keys if available
5. Reduce request rate

**Prevention:**
- Use multiple API keys
- Implement rate limiting
- Monitor key health metrics
- Set up failover strategies

---

## Performance Issues

### Slow Response Times

#### Diagnosis

```go
// Measure end-to-end latency
func measureLatency(provider types.Provider) {
    start := time.Now()

    stream, err := provider.GenerateChatCompletion(ctx, options)
    if err != nil {
        log.Printf("Error: %v", err)
        return
    }
    defer stream.Close()

    var totalChunks int
    var firstByte time.Duration

    for {
        chunkStart := time.Now()
        chunk, err := stream.Next()
        if err != nil {
            break
        }

        if totalChunks == 0 {
            firstByte = time.Since(start)
        }

        totalChunks++
        log.Printf("Chunk %d: %v", totalChunks, time.Since(chunkStart))

        if chunk.Done {
            break
        }
    }

    log.Printf("Time to first byte: %v", firstByte)
    log.Printf("Total time: %v", time.Since(start))
    log.Printf("Average per chunk: %v", time.Since(start)/time.Duration(totalChunks))
}
```

#### Solutions

1. **Enable streaming:**
```go
options := types.GenerateOptions{
    Stream: true, // Get results as they're generated
    Prompt: "Your prompt",
}
```

2. **Reduce token count:**
```go
options := types.GenerateOptions{
    MaxTokens: 500, // Limit response length
    Prompt:    "Be concise: ...",
}
```

3. **Use faster models:**
```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    DefaultModel: "gpt-3.5-turbo", // Faster than gpt-4
}
```

4. **Optimize network:**
```go
httpClient := &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        DisableKeepAlives:   false, // Reuse connections
    },
}
```

### High Memory Usage

#### Diagnosis

```go
import "runtime"

func analyzeMemory() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    log.Printf("Allocated: %d MB", m.Alloc/1024/1024)
    log.Printf("Total allocated: %d MB", m.TotalAlloc/1024/1024)
    log.Printf("System: %d MB", m.Sys/1024/1024)
    log.Printf("Garbage collections: %d", m.NumGC)
    log.Printf("Heap objects: %d", m.HeapObjects)
}
```

#### Solutions

1. **Close streams properly:**
```go
stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    return err
}
defer stream.Close() // Always close streams
```

2. **Limit concurrent requests:**
```go
semaphore := make(chan struct{}, 10) // Max 10 concurrent

for _, request := range requests {
    semaphore <- struct{}{}
    go func(req Request) {
        defer func() { <-semaphore }()
        processRequest(req)
    }(request)
}
```

3. **Use object pooling:**
```go
import "sync"

var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func processData() {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer bufferPool.Put(buf)
    buf.Reset()

    // Use buffer
}
```

### CPU Spikes

#### Diagnosis

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof
```

#### Solutions

1. **Optimize JSON parsing:**
```go
// Use streaming JSON decoder for large responses
decoder := json.NewDecoder(resp.Body)
for decoder.More() {
    var chunk Chunk
    if err := decoder.Decode(&chunk); err != nil {
        break
    }
    processChunk(chunk)
}
```

2. **Reduce allocations:**
```go
// Pre-allocate slices when size is known
messages := make([]types.ChatMessage, 0, expectedCount)

// Reuse buffers
var buf bytes.Buffer
buf.Reset()
```

### Connection Pool Exhaustion

#### Symptoms
```
too many open files
connection refused
```

#### Solutions

1. **Configure connection limits:**
```go
http.DefaultTransport.(*http.Transport).MaxIdleConns = 100
http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 10
```

2. **Close idle connections:**
```go
client := &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}
```

3. **Monitor connection count:**
```bash
# Linux
lsof -p <pid> | grep TCP | wc -l

# Check system limits
ulimit -n
```

### Cache Misses

#### Implement response caching

```go
import (
    "sync"
    "time"
)

type Cache struct {
    mu    sync.RWMutex
    items map[string]CacheItem
}

type CacheItem struct {
    Response  string
    ExpiresAt time.Time
}

func (c *Cache) Get(key string) (string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    item, exists := c.items[key]
    if !exists || time.Now().After(item.ExpiresAt) {
        return "", false
    }
    return item.Response, true
}

func (c *Cache) Set(key, value string, ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.items[key] = CacheItem{
        Response:  value,
        ExpiresAt: time.Now().Add(ttl),
    }
}
```

### Optimization Techniques

#### 1. Batch Requests

```go
// Instead of sequential requests
for _, prompt := range prompts {
    result, _ := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
        Prompt: prompt,
    })
    results = append(results, result)
}

// Use concurrent processing
var wg sync.WaitGroup
results := make([]Result, len(prompts))

for i, prompt := range prompts {
    wg.Add(1)
    go func(idx int, p string) {
        defer wg.Done()
        result, _ := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
            Prompt: p,
        })
        results[idx] = result
    }(i, prompt)
}
wg.Wait()
```

#### 2. Use Appropriate Models

```go
// Use cheaper/faster models for simple tasks
simpleConfig := types.ProviderConfig{
    DefaultModel: "gpt-3.5-turbo", // Fast and cheap
}

// Reserve expensive models for complex tasks
complexConfig := types.ProviderConfig{
    DefaultModel: "gpt-4", // Powerful but slower
}
```

#### 3. Optimize Token Usage

```go
options := types.GenerateOptions{
    Prompt:      "Be concise and direct.",
    MaxTokens:   100, // Limit tokens
    Temperature: 0.3, // Lower temperature for focused responses
}
```

---

## Configuration FAQ

### How to set up multi-provider configuration?

**YAML Configuration:**

```yaml
# config.yaml
providers:
  openai:
    enabled: true
    api_key: ${OPENAI_API_KEY}
    model: gpt-4
    timeout: 30

  anthropic:
    enabled: true
    api_keys:
      - ${ANTHROPIC_API_KEY_1}
      - ${ANTHROPIC_API_KEY_2}
    model: claude-3-opus-20240229

  gemini:
    enabled: true
    auth_type: oauth
    client_id: ${GEMINI_CLIENT_ID}
    client_secret: ${GEMINI_CLIENT_SECRET}
    model: gemini-pro
```

**Programmatic Configuration:**

```go
factory := factory.NewProviderFactory()
factory.RegisterDefaultProviders(factory)

// OpenAI
openaiConfig := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    APIKey:       os.Getenv("OPENAI_API_KEY"),
    DefaultModel: "gpt-4",
}
openaiProvider, _ := factory.CreateProvider(types.ProviderTypeOpenAI, openaiConfig)

// Anthropic with multiple keys
anthropicConfig := types.ProviderConfig{
    Type: types.ProviderTypeAnthropic,
    APIKeys: []string{
        os.Getenv("ANTHROPIC_API_KEY_1"),
        os.Getenv("ANTHROPIC_API_KEY_2"),
    },
    DefaultModel: "claude-3-opus-20240229",
}
anthropicProvider, _ := factory.CreateProvider(types.ProviderTypeAnthropic, anthropicConfig)

// Use providers
providers := map[string]types.Provider{
    "openai":    openaiProvider,
    "anthropic": anthropicProvider,
}
```

### Environment variable precedence

**Precedence order (highest to lowest):**

1. Explicit configuration values
2. Environment variables referenced in YAML
3. Default values in code
4. Provider defaults

**Example:**

```yaml
# config.yaml
providers:
  openai:
    api_key: ${OPENAI_API_KEY} # 2. Environment variable
    timeout: 60 # 1. Explicit value
    # max_retries not specified, uses default (3)
```

```go
// Explicit config overrides everything
config := types.ProviderConfig{
    APIKey:     "explicit-key", // Highest precedence
    Timeout:    30 * time.Second,
    MaxRetries: 5,
}
```

### YAML formatting issues

**Common mistakes:**

1. **Incorrect indentation:**
```yaml
# Wrong
providers:
openai:
  api_key: sk-...

# Correct
providers:
  openai:
    api_key: sk-...
```

2. **Missing quotes for special characters:**
```yaml
# Wrong
password: p@ssw0rd!

# Correct
password: "p@ssw0rd!"
```

3. **Environment variable syntax:**
```yaml
# Wrong
api_key: $OPENAI_API_KEY

# Correct
api_key: ${OPENAI_API_KEY}
```

4. **Lists format:**
```yaml
# Both valid
api_keys:
  - key1
  - key2

# Or
api_keys: [key1, key2]
```

### Secret management

**1. Environment Variables:**

```bash
# .env file
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."

# Load in shell
source .env
```

**2. Secret Management Systems:**

```go
// HashiCorp Vault
import "github.com/hashicorp/vault/api"

func getSecret(key string) (string, error) {
    client, _ := api.NewClient(nil)
    secret, err := client.Logical().Read("secret/data/" + key)
    if err != nil {
        return "", err
    }
    return secret.Data["value"].(string), nil
}

// AWS Secrets Manager
import "github.com/aws/aws-sdk-go/service/secretsmanager"

func getAWSSecret(secretName string) (string, error) {
    svc := secretsmanager.New(session.New())
    result, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
        SecretId: aws.String(secretName),
    })
    if err != nil {
        return "", err
    }
    return *result.SecretString, nil
}
```

**3. Encrypted Configuration:**

```go
import "crypto/aes"

func loadEncryptedConfig(filename, key string) (*Config, error) {
    encryptedData, _ := os.ReadFile(filename)
    decryptedData := decrypt(encryptedData, key)

    var config Config
    yaml.Unmarshal(decryptedData, &config)
    return &config, nil
}
```

### Provider selection logic

**1. Round-robin across providers:**

```go
type MultiProvider struct {
    providers []types.Provider
    index     int32
}

func (m *MultiProvider) GetNext() types.Provider {
    idx := atomic.AddInt32(&m.index, 1)
    return m.providers[idx%int32(len(m.providers))]
}
```

**2. Fallback chain:**

```go
func tryProviders(providers []types.Provider, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    var lastErr error

    for _, provider := range providers {
        stream, err := provider.GenerateChatCompletion(ctx, options)
        if err == nil {
            return stream, nil
        }
        lastErr = err
        log.Printf("Provider %s failed: %v, trying next", provider.Name(), err)
    }

    return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
}
```

**3. Cost-optimized selection:**

```go
func selectCheapestProvider(providers map[string]types.Provider, estimatedTokens int) types.Provider {
    type providerCost struct {
        name     string
        provider types.Provider
        cost     float64
    }

    var costs []providerCost
    for name, provider := range providers {
        cost := estimateCost(provider, estimatedTokens)
        costs = append(costs, providerCost{name, provider, cost})
    }

    sort.Slice(costs, func(i, j int) bool {
        return costs[i].cost < costs[j].cost
    })

    return costs[0].provider
}
```

### Default values

**Provider defaults:**

```go
const (
    DefaultTimeout     = 30 * time.Second
    DefaultMaxRetries  = 3
    DefaultTemperature = 0.7
    DefaultMaxTokens   = 1000
)

func applyDefaults(config *types.ProviderConfig) {
    if config.Timeout == 0 {
        config.Timeout = DefaultTimeout
    }
    if config.MaxRetries == 0 {
        config.MaxRetries = DefaultMaxRetries
    }
}
```

---

## Authentication FAQ

### OAuth token refresh issues

**Q: Token refresh fails with 400 Bad Request**

**A:** Common causes:
1. Refresh token has expired
2. Invalid client credentials
3. Wrong token URL

**Solution:**
```go
// Enable detailed logging
config := &auth.OAuthConfig{
    Refresh: auth.RefreshConfig{
        Enabled: true,
        Buffer:  5 * time.Minute,
    },
    HTTP: auth.HTTPConfig{
        Timeout:   30 * time.Second,
        UserAgent: "AI-Provider-Kit/1.0",
    },
}

// Monitor refresh attempts
cred := &types.OAuthCredentialSet{
    OnTokenRefresh: func(id, access, refresh string, expires time.Time) error {
        log.Printf("Token refreshed successfully for %s", id)
        return saveTokens(id, access, refresh, expires)
    },
}
```

**Q: How often should tokens be refreshed?**

**A:** Configure based on token lifetime:
- Default: 5 minutes before expiry
- High traffic: 10-15 minutes before expiry
- Conservative: 30 minutes before expiry

```go
strategy := &oauthmanager.RefreshStrategy{
    BufferTime:        10 * time.Minute,
    AdaptiveBuffer:    true, // Adjusts based on usage
    PreemptiveRefresh: true, // Refresh early under load
}
manager.SetRefreshStrategy(strategy)
```

### API key rotation

**Q: How to rotate API keys without downtime?**

**A:** Use multi-key configuration:

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/keymanager"

// Start with current keys
manager := keymanager.NewKeyManager("OpenAI", []string{
    "old-key-1",
    "old-key-2",
})

// Add new key (both old and new are active)
manager.AddKey("new-key-1")

// Test new key
testKey := func(key string) error {
    // Make test API call
    return nil
}

if err := testKey("new-key-1"); err == nil {
    // New key works, remove old key
    manager.RemoveKey("old-key-1")
}
```

**Q: Automated rotation schedule?**

**A:** Implement rotation policy:

```go
policy := &oauthmanager.RotationPolicy{
    Enabled:          true,
    RotationInterval: 30 * 24 * time.Hour, // Every 30 days
    GracePeriod:      7 * 24 * time.Hour,  // 7-day overlap
    AutoDecommission: true,

    OnRotationNeeded: func(credentialID string) error {
        log.Printf("Credential %s needs rotation", credentialID)
        // Trigger rotation process
        return initiateRotation(credentialID)
    },
}
manager.SetRotationPolicy(policy)
```

### Multi-credential setup

**Q: How to configure multiple OAuth credentials?**

**A:**

```yaml
# config.yaml
providers:
  gemini:
    auth_type: oauth
    oauth_credentials:
      - id: team-account
        client_id: client-id-1
        client_secret: client-secret-1
        access_token: token-1
        refresh_token: refresh-1

      - id: personal-account
        client_id: client-id-2
        client_secret: client-secret-2
        access_token: token-2
        refresh_token: refresh-2
```

```go
credentials := []*types.OAuthCredentialSet{
    {
        ID:           "team-account",
        ClientID:     os.Getenv("GEMINI_CLIENT_ID_1"),
        ClientSecret: os.Getenv("GEMINI_CLIENT_SECRET_1"),
        OnTokenRefresh: func(id, access, refresh string, expires time.Time) error {
            return saveTokens(id, access, refresh, expires)
        },
    },
    {
        ID:           "personal-account",
        ClientID:     os.Getenv("GEMINI_CLIENT_ID_2"),
        ClientSecret: os.Getenv("GEMINI_CLIENT_SECRET_2"),
        OnTokenRefresh: func(id, access, refresh string, expires time.Time) error {
            return saveTokens(id, access, refresh, expires)
        },
    },
}

manager := oauthmanager.NewOAuthKeyManager("Gemini", credentials, refreshFunc)
```

### Token storage problems

**Q: Where are tokens stored?**

**A:** Default storage locations:
- File storage: `.tokens.json` in working directory
- Memory storage: In-memory only (lost on restart)

```go
// File storage
storage := auth.NewFileTokenStorage(".tokens.json")

// Memory storage (for testing)
storage := auth.NewMemoryTokenStorage()

// Custom storage
type CustomStorage struct{}

func (s *CustomStorage) StoreToken(key string, config *types.OAuthConfig) error {
    // Store in database, etc.
    return db.SaveToken(key, config)
}
```

**Q: Token storage fails with permission denied**

**A:** Check file permissions:

```bash
# Check permissions
ls -l .tokens.json

# Fix permissions
chmod 600 .tokens.json

# Ensure directory is writable
chmod 755 $(dirname .tokens.json)
```

### PKCE errors

**Q: PKCE challenge validation failed**

**A:** Ensure PKCE is configured correctly:

```go
config := &auth.OAuthConfig{
    PKCE: auth.PKCEConfig{
        Enabled:        true,
        Method:         "S256", // SHA-256
        VerifierLength: 128,    // Recommended length
    },
}
```

**Q: Which providers require PKCE?**

**A:**
- **Required:** Mobile apps, single-page apps
- **Recommended:** All public clients
- **Optional:** Server-to-server flows

### State validation failures

**Q: Invalid OAuth state error**

**A:** Common causes:
1. State mismatch between request and callback
2. State expired or reused
3. CSRF attack detected

**Solution:**

```go
config := &auth.OAuthConfig{
    State: auth.StateConfig{
        EnableValidation: true,
        Length:          32, // Secure random length
    },
}

// Store state securely
var stateStore = make(map[string]time.Time)

authURL, _ := authenticator.StartOAuthFlow(ctx, scopes)
// State is generated automatically

// On callback
err := authenticator.HandleCallback(ctx, code, state)
if err != nil {
    log.Printf("Callback failed: %v", err)
}
```

---

## Provider-Specific FAQ

### OpenAI

**Q: Model access denied error**

**A:** Check API key tier:
- Free tier: Limited model access
- Pay-as-you-go: Access to most models
- Enterprise: Full access

```go
// List available models
models := []string{
    "gpt-3.5-turbo",      // Available to all
    "gpt-4",              // Requires pay-as-you-go
    "gpt-4-turbo-preview", // Latest model
}
```

**Q: Context length exceeded**

**A:** Reduce input length:

```go
options := types.GenerateOptions{
    Messages: truncateMessages(messages, maxTokens),
    MaxTokens: 1000,
}

func truncateMessages(messages []types.ChatMessage, maxTokens int) []types.ChatMessage {
    // Keep system message and recent messages
    result := []types.ChatMessage{messages[0]} // System message

    for i := len(messages) - 1; i > 0; i-- {
        // Add recent messages until limit
        if estimateTokens(result) < maxTokens {
            result = append([]types.ChatMessage{messages[i]}, result...)
        }
    }

    return result
}
```

### Anthropic

**Q: Missing Claude Code headers**

**A:** Add required headers:

```go
config := types.ProviderConfig{
    Type:   types.ProviderTypeAnthropic,
    APIKey: "sk-ant-...",
    Headers: map[string]string{
        "anthropic-version": "2023-06-01",
        // For Claude Code
        "anthropic-client": "claude-code",
        "anthropic-client-version": "0.1.0",
    },
}
```

**Q: How to use Claude with tool calling?**

**A:**

```go
tools := []types.Tool{
    {
        Name:        "get_weather",
        Description: "Get weather for a location",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "location": map[string]interface{}{
                    "type": "string",
                    "description": "City name",
                },
            },
            "required": []string{"location"},
        },
    },
}

options := types.GenerateOptions{
    Messages: []types.ChatMessage{
        {Role: "user", Content: "What's the weather in Paris?"},
    },
    Tools: tools,
}

stream, _ := provider.GenerateChatCompletion(ctx, options)
```

### Gemini

**Q: OAuth setup for Gemini**

**A:** Complete OAuth flow:

1. **Get OAuth credentials from Google Cloud Console:**
   - Create project
   - Enable Gemini API
   - Create OAuth 2.0 credentials

2. **Configure OAuth:**
```go
config := &auth.OAuthConfig{
    ClientID:     "your-client-id.apps.googleusercontent.com",
    ClientSecret: "your-client-secret",
    AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
    TokenURL:     "https://oauth2.googleapis.com/token",
    RedirectURL:  "http://localhost:8080/callback",
    DefaultScopes: []string{
        "https://www.googleapis.com/auth/cloud-platform",
    },
}
```

3. **Run OAuth flow:**
```go
authenticator := auth.NewOAuthAuthenticator("gemini", storage, config)
authURL, _ := authenticator.StartOAuthFlow(ctx, config.DefaultScopes)
fmt.Printf("Visit: %s\n", authURL)

// After user authorizes, handle callback
err := authenticator.HandleCallback(ctx, code, state)
```

**Q: Project ID required error**

**A:**

```go
config := types.ProviderConfig{
    Type: types.ProviderTypeGemini,
    Headers: map[string]string{
        "x-goog-user-project": "your-project-id",
    },
}
```

### Cerebras

**Q: Daily request limit tracking**

**A:** Cerebras has daily limits. Track usage:

```go
// Use provider metrics
metrics := provider.GetMetrics()
log.Printf("Requests today: %d", metrics.RequestCount)

// Reset counter daily
ticker := time.NewTicker(24 * time.Hour)
go func() {
    for range ticker.C {
        // Reset metrics or track separately
        dailyCount = 0
    }
}()
```

**Q: Cerebras-specific rate limits**

**A:**

```go
config := types.ProviderConfig{
    Type:   types.ProviderTypeCerebras,
    APIKey: "your-api-key",
    RateLimit: types.RateLimitConfig{
        RequestsPerMinute: 60,
        RequestsPerDay:    1000, // Daily limit
        Enabled:          true,
    },
}
```

### Qwen

**Q: Device code flow setup**

**A:** Qwen uses device code flow for OAuth:

```go
// Start device code flow
deviceCode, err := startDeviceCodeFlow()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Visit: %s\n", deviceCode.VerificationURL)
fmt.Printf("Enter code: %s\n", deviceCode.UserCode)

// Poll for authorization
for {
    token, err := pollForToken(deviceCode.DeviceCode)
    if err == nil {
        // Authorization complete
        break
    }
    time.Sleep(5 * time.Second)
}
```

**Q: Qwen API key vs OAuth**

**A:** Qwen supports both:

```go
// API key (simpler)
config := types.ProviderConfig{
    Type:   types.ProviderTypeQwen,
    APIKey: "your-api-key",
}

// OAuth (recommended)
config := types.ProviderConfig{
    Type: types.ProviderTypeQwen,
    OAuthCredentials: []*types.OAuthCredentialSet{
        {
            ClientID:     "your-client-id",
            ClientSecret: "your-client-secret",
        },
    },
}
```

### OpenRouter

**Q: Credit balance management**

**A:**

```go
// Check credit balance
balance := checkOpenRouterBalance(apiKey)
log.Printf("Remaining credits: %.2f", balance)

// Set alerts
if balance < 10.0 {
    log.Println("WARNING: Low credit balance")
    sendAlert("OpenRouter credits low")
}
```

**Q: Model selection**

**A:** Use provider/model format:

```go
models := []string{
    "anthropic/claude-3-opus",
    "openai/gpt-4-turbo-preview",
    "google/gemini-pro",
    "meta-llama/llama-2-70b-chat",
}

config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenRouter,
    DefaultModel: "anthropic/claude-3-opus",
}
```

---

## Integration FAQ

### Docker deployment issues

**Q: How to deploy with Docker?**

**A:** Use the provided Dockerfile:

```dockerfile
# Build
docker build -t ai-provider-kit .

# Run
docker run -d \
  -e OPENAI_API_KEY=$OPENAI_API_KEY \
  -e ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -p 8080:8080 \
  ai-provider-kit
```

**Q: Environment variables in Docker**

**A:**

```yaml
# docker-compose.yml
version: '3.8'
services:
  ai-provider-kit:
    build: .
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - LOG_LEVEL=info
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./tokens:/app/.tokens
    ports:
      - "8080:8080"
```

### Kubernetes configuration

**Q: Kubernetes deployment**

**A:**

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-provider-kit
spec:
  replicas: 3
  selector:
    matchLabels:
      app: ai-provider-kit
  template:
    metadata:
      labels:
        app: ai-provider-kit
    spec:
      containers:
      - name: ai-provider-kit
        image: ai-provider-kit:latest
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: ai-provider-secrets
              key: openai-key
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: ai-provider-secrets
              key: anthropic-key
        ports:
        - containerPort: 8080
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

```yaml
# secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: ai-provider-secrets
type: Opaque
stringData:
  openai-key: sk-...
  anthropic-key: sk-ant-...
```

### CI/CD pipeline setup

**Q: GitHub Actions integration**

**A:**

```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2

      - name: Set up Go
        uses: actions/setup-go@v2
        with:
          go-version: 1.23

      - name: Run tests
        run: go test -v ./...
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}

      - name: Build
        run: go build -v ./...
```

### Testing strategies

**Q: How to test without API calls?**

**A:** Use mock providers (requires build tags):

```go
//go:build dev || debug

import "github.com/cecil-the-coder/ai-provider-kit/pkg/dev"

// Create mock provider
mockProvider := dev.NewMockProvider("test-provider")

// Set mock response
dev.SetMockResponse(mockProvider, types.ChatCompletionChunk{
    Content: "Mock response",
    Done:    true,
})

// Test your code
stream, err := mockProvider.GenerateChatCompletion(ctx, options)
```

**Q: Integration testing**

**A:**

```go
func TestProviderIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    provider := createTestProvider()

    stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
        Prompt: "Test prompt",
    })

    require.NoError(t, err)
    defer stream.Close()

    chunk, err := stream.Next()
    require.NoError(t, err)
    assert.NotEmpty(t, chunk.Content)
}

// Run with: go test -v (includes integration tests)
// Run without: go test -v -short (skips integration tests)
```

### Monitoring setup

**Q: Prometheus metrics**

**A:**

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ai_provider_requests_total",
            Help: "Total requests per provider",
        },
        []string{"provider", "status"},
    )

    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "ai_provider_request_duration_seconds",
            Help: "Request duration in seconds",
        },
        []string{"provider"},
    )
)

func init() {
    prometheus.MustRegister(requestsTotal)
    prometheus.MustRegister(requestDuration)
}

// Record metrics
func trackRequest(provider string, duration time.Duration, err error) {
    status := "success"
    if err != nil {
        status = "error"
    }

    requestsTotal.WithLabelValues(provider, status).Inc()
    requestDuration.WithLabelValues(provider).Observe(duration.Seconds())
}
```

### Logging configuration

**Q: Structured logging setup**

**A:**

```go
import "go.uber.org/zap"

// Development
logger, _ := zap.NewDevelopment()

// Production
logger, _ := zap.NewProduction()

// Custom config
config := zap.Config{
    Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
    Encoding:         "json",
    OutputPaths:      []string{"stdout", "/var/log/ai-provider.log"},
    ErrorOutputPaths: []string{"stderr"},
    EncoderConfig: zapcore.EncoderConfig{
        TimeKey:        "timestamp",
        LevelKey:       "level",
        MessageKey:     "message",
        EncodeTime:     zapcore.ISO8601TimeEncoder,
        EncodeLevel:    zapcore.LowercaseLevelEncoder,
    },
}

logger, _ := config.Build()
defer logger.Sync()

// Use logger
logger.Info("Provider created",
    zap.String("provider", "openai"),
    zap.String("model", "gpt-4"),
)
```

---

## Advanced Scenarios

### Handling provider outages

**Q: Automatic failover to backup provider**

**A:**

```go
type FailoverManager struct {
    primary   types.Provider
    secondary types.Provider
    tertiary  types.Provider
}

func (f *FailoverManager) GenerateWithFailover(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    providers := []types.Provider{f.primary, f.secondary, f.tertiary}

    for i, provider := range providers {
        stream, err := provider.GenerateChatCompletion(ctx, options)
        if err == nil {
            if i > 0 {
                log.Printf("Failed over to provider %d", i+1)
            }
            return stream, nil
        }

        log.Printf("Provider %d failed: %v", i+1, err)
    }

    return nil, fmt.Errorf("all providers failed")
}
```

**Q: Circuit breaker pattern**

**A:**

```go
import "github.com/sony/gobreaker"

type CircuitBreakerProvider struct {
    provider types.Provider
    breaker  *gobreaker.CircuitBreaker
}

func NewCircuitBreakerProvider(provider types.Provider) *CircuitBreakerProvider {
    settings := gobreaker.Settings{
        Name:        provider.Name(),
        MaxRequests: 3,
        Interval:    time.Minute,
        Timeout:     30 * time.Second,
        ReadyToTrip: func(counts gobreaker.Counts) bool {
            failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
            return counts.Requests >= 3 && failureRatio >= 0.6
        },
    }

    return &CircuitBreakerProvider{
        provider: provider,
        breaker:  gobreaker.NewCircuitBreaker(settings),
    }
}

func (c *CircuitBreakerProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    result, err := c.breaker.Execute(func() (interface{}, error) {
        return c.provider.GenerateChatCompletion(ctx, options)
    })

    if err != nil {
        return nil, err
    }

    return result.(types.ChatCompletionStream), nil
}
```

### Implementing custom providers

**Q: How to add a new provider?**

**A:**

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"

type CustomProvider struct {
    *base.BaseProvider
    client *http.Client
}

func NewCustomProvider(config types.ProviderConfig) *CustomProvider {
    client := &http.Client{Timeout: config.Timeout}

    provider := &CustomProvider{
        BaseProvider: base.NewBaseProvider("custom", config, client, nil),
        client:       client,
    }

    return provider
}

func (p *CustomProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    // Increment request count
    p.IncrementRequestCount()
    startTime := time.Now()

    // Make API request
    response, err := p.makeRequest(ctx, options)
    if err != nil {
        p.RecordError(err)
        return nil, err
    }

    // Record success
    p.RecordSuccess(time.Since(startTime), response.TokensUsed)

    return p.createStream(response), nil
}

// Register with factory
func init() {
    factory.RegisterProvider(types.ProviderTypeCustom, func(config types.ProviderConfig) types.Provider {
        return NewCustomProvider(config)
    })
}
```

### Extending the SDK

**Q: Custom middleware**

**A:**

```go
type MiddlewareFunc func(types.Provider) types.Provider

type LoggingMiddleware struct {
    provider types.Provider
    logger   *zap.Logger
}

func (m *LoggingMiddleware) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    m.logger.Info("Request started",
        zap.String("provider", m.provider.Name()),
        zap.String("model", options.Model),
    )

    stream, err := m.provider.GenerateChatCompletion(ctx, options)

    if err != nil {
        m.logger.Error("Request failed",
            zap.Error(err),
        )
    } else {
        m.logger.Info("Request succeeded")
    }

    return stream, err
}

func WithLogging(logger *zap.Logger) MiddlewareFunc {
    return func(provider types.Provider) types.Provider {
        return &LoggingMiddleware{
            provider: provider,
            logger:   logger,
        }
    }
}

// Use middleware
provider = WithLogging(logger)(provider)
```

### Custom rate limiters

**Q: Implement token bucket rate limiter**

**A:**

```go
import "golang.org/x/time/rate"

type TokenBucketRateLimiter struct {
    limiter *rate.Limiter
}

func NewTokenBucketRateLimiter(requestsPerSecond int) *TokenBucketRateLimiter {
    return &TokenBucketRateLimiter{
        limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), requestsPerSecond),
    }
}

func (r *TokenBucketRateLimiter) Wait(ctx context.Context) error {
    return r.limiter.Wait(ctx)
}

func (r *TokenBucketRateLimiter) Allow() bool {
    return r.limiter.Allow()
}

// Use with provider
rateLimiter := NewTokenBucketRateLimiter(10) // 10 req/sec

func makeRequest(ctx context.Context) error {
    if err := rateLimiter.Wait(ctx); err != nil {
        return err
    }

    return provider.GenerateChatCompletion(ctx, options)
}
```

### Custom authentication

**Q: Implement custom auth method**

**A:**

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/auth"

type CustomAuthenticator struct {
    provider string
    token    string
}

func (a *CustomAuthenticator) Authenticate(ctx context.Context, config types.AuthConfig) error {
    // Custom authentication logic
    token, err := a.performCustomAuth(ctx)
    if err != nil {
        return err
    }
    a.token = token
    return nil
}

func (a *CustomAuthenticator) IsAuthenticated() bool {
    return a.token != ""
}

func (a *CustomAuthenticator) GetToken() (string, error) {
    if !a.IsAuthenticated() {
        return "", &auth.AuthError{
            Provider: a.provider,
            Code:     auth.ErrCodeTokenExpired,
            Message:  "Not authenticated",
        }
    }
    return a.token, nil
}

func (a *CustomAuthenticator) RefreshToken(ctx context.Context) error {
    return a.Authenticate(ctx, types.AuthConfig{})
}

func (a *CustomAuthenticator) Logout(ctx context.Context) error {
    a.token = ""
    return nil
}

func (a *CustomAuthenticator) GetAuthMethod() types.AuthMethod {
    return types.AuthMethodCustom
}
```

### Plugin development

**Q: Create plugin system**

**A:**

```go
type Plugin interface {
    Name() string
    Init(config map[string]interface{}) error
    PreRequest(ctx context.Context, options *types.GenerateOptions) error
    PostResponse(ctx context.Context, response *types.ChatCompletionChunk) error
    Close() error
}

type PluginManager struct {
    plugins []Plugin
}

func (m *PluginManager) Register(plugin Plugin) error {
    m.plugins = append(m.plugins, plugin)
    return nil
}

func (m *PluginManager) ExecutePreRequest(ctx context.Context, options *types.GenerateOptions) error {
    for _, plugin := range m.plugins {
        if err := plugin.PreRequest(ctx, options); err != nil {
            return err
        }
    }
    return nil
}

// Example plugin
type CachePlugin struct {
    cache map[string]string
}

func (p *CachePlugin) Name() string {
    return "cache"
}

func (p *CachePlugin) PreRequest(ctx context.Context, options *types.GenerateOptions) error {
    // Check cache
    if cached, exists := p.cache[options.Prompt]; exists {
        // Return cached response
    }
    return nil
}
```

---

## Getting Help

### Community Resources

**GitHub Repository:**
- Repository: https://github.com/cecil-the-coder/ai-provider-kit
- Stars: Watch for updates
- Fork: Contribute improvements

**Documentation:**
- Main README: [README.md](../README.md)
- OAuth Manager: [OAUTH_MANAGER.md](./OAUTH_MANAGER.md)
- Metrics Guide: [METRICS.md](./METRICS.md)
- Multi-Key Strategies: [MULTI_KEY_STRATEGIES.md](./MULTI_KEY_STRATEGIES.md)
- Tool Calling: [TOOL_CALLING.md](../TOOL_CALLING.md)
- API Documentation: https://pkg.go.dev/github.com/cecil-the-coder/ai-provider-kit

**Examples:**
- Demo Client: [examples/demo-client](../examples/demo-client)
- Streaming Demo: [examples/demo-client-streaming](../examples/demo-client-streaming)
- Tool Calling Demo: [examples/tool-calling-demo](../examples/tool-calling-demo)
- Config Demo: [examples/config-demo](../examples/config-demo)

### Issue Reporting Guidelines

**Before reporting:**
1. Search existing issues
2. Check documentation
3. Try the latest version
4. Create minimal reproduction

**Issue template:**

```markdown
## Description
Clear description of the issue

## Environment
- Go version: 1.23
- OS: Linux/macOS/Windows
- SDK version: v1.0.0
- Provider: OpenAI/Anthropic/etc.

## Steps to Reproduce
1. Configure provider with...
2. Call GenerateChatCompletion...
3. Observe error...

## Expected Behavior
What should happen

## Actual Behavior
What actually happens

## Code Sample
```go
// Minimal reproduction code
```

## Error Output
```
Full error message and stack trace
```

## Additional Context
Any other relevant information
```

### Contributing to the Project

**How to contribute:**

1. **Fork and clone:**
```bash
git clone https://github.com/YOUR-USERNAME/ai-provider-kit.git
cd ai-provider-kit
```

2. **Create branch:**
```bash
git checkout -b feature/amazing-feature
```

3. **Make changes:**
- Write code
- Add tests
- Update documentation

4. **Run tests:**
```bash
go test ./...
go test -race ./...
go test -coverprofile=coverage.out ./...
```

5. **Commit:**
```bash
git commit -m "Add amazing feature"
```

6. **Push and create PR:**
```bash
git push origin feature/amazing-feature
```

**Code style:**
- Run `gofmt`
- Follow Go conventions
- Add godoc comments
- Include examples

**PR template:**

```markdown
## Description
What does this PR do?

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] All tests passing

## Checklist
- [ ] Code follows style guidelines
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Commits are signed off
```

### Support Channels

**GitHub Discussions:**
- Questions: Ask general questions
- Ideas: Share feature ideas
- Show and tell: Share your projects

**GitHub Issues:**
- Bug reports
- Feature requests
- Documentation improvements

**Email:**
- Contact maintainers for private inquiries
- Security issues: Report privately

**Stack Overflow:**
- Tag: `ai-provider-kit`
- Search existing questions
- Follow tag for updates

### Documentation Resources

**API Reference:**
- pkg.go.dev: Full API documentation
- Godoc comments: Inline documentation
- Examples: Working code samples

**Guides:**
- Quick Start: [README.md](../README.md#quick-start)
- Authentication: [pkg/auth/README.md](../pkg/auth/README.md)
- OAuth Setup: [OAUTH_MANAGER.md](./OAUTH_MANAGER.md)
- Metrics: [METRICS.md](./METRICS.md)
- Rate Limiting: Implementation guides

**Video Tutorials:**
- Coming soon

### Example Repositories

**Official Examples:**
- [examples/demo-client](../examples/demo-client): Complete demo application
- [examples/demo-client-streaming](../examples/demo-client-streaming): Streaming responses
- [examples/tool-calling-demo](../examples/tool-calling-demo): Tool calling examples
- [examples/config-demo](../examples/config-demo): Configuration examples
- [examples/model-discovery-demo](../examples/model-discovery-demo): Model discovery

**Community Examples:**
- Check GitHub for projects using ai-provider-kit
- Share your own examples

---

## Quick Reference

### Common Commands

```bash
# Install
go get github.com/cecil-the-coder/ai-provider-kit@latest

# Run tests
go test ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run benchmarks
go test -bench=. ./...

# Check for race conditions
go test -race ./...

# Build
go build ./...

# Format code
gofmt -w .

# Vet code
go vet ./...
```

### Environment Variables

```bash
# Provider API keys
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GEMINI_API_KEY="..."
export CEREBRAS_API_KEY="..."
export QWEN_API_KEY="..."
export OPENROUTER_API_KEY="..."

# OAuth credentials
export GEMINI_CLIENT_ID="..."
export GEMINI_CLIENT_SECRET="..."
export GEMINI_PROJECT_ID="..."

# Configuration
export LOG_LEVEL="info"
export TOKEN_STORAGE="file"
export CONFIG_FILE="config.yaml"
```

### Quick Troubleshooting Steps

1. **Verify credentials:** Check API keys are valid
2. **Check network:** Test connectivity to provider
3. **Enable logging:** Add debug output
4. **Review metrics:** Check provider metrics
5. **Test timeout:** Increase timeout value
6. **Try streaming:** Enable streaming for long requests
7. **Update SDK:** Ensure latest version
8. **Check documentation:** Review provider-specific docs
9. **Search issues:** Look for similar problems
10. **Report bug:** Create detailed issue if needed

---

## Version Information

**Document Version:** 1.0.0
**SDK Version:** Latest
**Last Updated:** 2025-01-18

For the latest version of this document, visit:
https://github.com/cecil-the-coder/ai-provider-kit/blob/master/docs/SDK_TROUBLESHOOTING_FAQ.md
