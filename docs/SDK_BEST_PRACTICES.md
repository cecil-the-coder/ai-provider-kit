# AI Provider Kit - Best Practices and Performance Optimization

A comprehensive guide to building production-grade applications with the AI Provider Kit SDK, covering architecture patterns, performance optimization, security, and operational best practices.

## Table of Contents

1. [Architecture Best Practices](#1-architecture-best-practices)
2. [Performance Optimization](#2-performance-optimization)
3. [Security Best Practices](#3-security-best-practices)
4. [Production Deployment](#4-production-deployment)
5. [Multi-Tenancy](#5-multi-tenancy)
6. [Scalability Patterns](#6-scalability-patterns)
7. [Testing Strategies](#7-testing-strategies)
8. [Cost Optimization](#8-cost-optimization)
9. [Debugging and Troubleshooting](#9-debugging-and-troubleshooting)
10. [Code Organization](#10-code-organization)
11. [Shared Utilities Best Practices (Phase 2)](#11-shared-utilities-best-practices-phase-2)
    - [Streaming Utilities](#streaming-utilities)
    - [Authentication Helpers](#authentication-helpers)
    - [Configuration Helper](#configuration-helper)
12. [Interface Segregation Best Practices (Phase 3)](#12-interface-segregation-best-practices-phase-3)
    - [When to Use Specific Interfaces](#when-to-use-specific-interfaces)
    - [Interface Composition Patterns](#interface-composition-patterns)
    - [Testing with Segregated Interfaces](#testing-with-segregated-interfaces)
13. [Standardized Core API Best Practices (Phase 3)](#13-standardized-core-api-best-practices-phase-3)
    - [Request Builder Patterns](#request-builder-patterns)
    - [Extension Development](#extension-development)
    - [Migration Strategies](#migration-strategies)

---

## 1. Architecture Best Practices

### 1.1 Provider Selection Strategies

#### Dynamic Provider Selection

Implement intelligent provider selection based on workload characteristics:

```go
package aiservice

import (
    "context"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type ProviderSelector struct {
    factory *factory.DefaultProviderFactory
    config  *SelectionConfig
}

type SelectionConfig struct {
    PreferredProviders map[string][]string // workload -> providers
    FallbackOrder      []string
    CostThreshold      float64
    LatencyThreshold   time.Duration
}

func (s *ProviderSelector) SelectProvider(ctx context.Context, workload Workload) (types.Provider, error) {
    // 1. Check workload requirements
    requirements := workload.GetRequirements()

    // 2. Filter providers by capabilities
    candidates := s.filterByCapabilities(requirements)

    // 3. Rank by performance metrics
    ranked := s.rankByMetrics(candidates)

    // 4. Select best available provider
    for _, providerType := range ranked {
        provider, err := s.factory.CreateProvider(providerType, s.getConfig(providerType))
        if err != nil {
            continue
        }

        // Health check before using
        if err := provider.HealthCheck(ctx); err != nil {
            continue
        }

        return provider, nil
    }

    return nil, fmt.Errorf("no suitable provider available")
}

// Example: Route by model capabilities
func (s *ProviderSelector) SelectByCapability(needsToolCalling, needsStreaming bool) types.ProviderType {
    if needsToolCalling {
        // All providers support tool calling, but some are better
        return types.ProviderTypeAnthropic // Claude excels at tool use
    }

    if needsStreaming {
        return types.ProviderTypeCerebras // Fastest streaming
    }

    return types.ProviderTypeOpenAI // Default choice
}
```

#### Multi-Provider Failover

```go
type MultiProviderService struct {
    providers map[types.ProviderType]types.Provider
    priority  []types.ProviderType
}

func (s *MultiProviderService) ExecuteWithFailover(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    var lastErr error

    for _, providerType := range s.priority {
        provider, exists := s.providers[providerType]
        if !exists {
            continue
        }

        stream, err := provider.GenerateChatCompletion(ctx, options)
        if err == nil {
            return stream, nil
        }

        lastErr = err
        log.Printf("Provider %s failed: %v, trying next", providerType, err)
    }

    return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
}
```

### 1.2 Configuration Management

#### Environment-Based Configuration

```go
package config

import (
    "os"
    "time"
    "gopkg.in/yaml.v3"
)

type AppConfig struct {
    Environment string            `yaml:"environment"`
    Providers   ProvidersConfig   `yaml:"providers"`
    Security    SecurityConfig    `yaml:"security"`
    Performance PerformanceConfig `yaml:"performance"`
}

type ProvidersConfig struct {
    OpenAI    *ProviderEntry `yaml:"openai,omitempty"`
    Anthropic *ProviderEntry `yaml:"anthropic,omitempty"`
    Gemini    *ProviderEntry `yaml:"gemini,omitempty"`
    Cerebras  *ProviderEntry `yaml:"cerebras,omitempty"`
}

type ProviderEntry struct {
    Enabled          bool                     `yaml:"enabled"`
    APIKey           string                   `yaml:"api_key,omitempty"`
    APIKeys          []string                 `yaml:"api_keys,omitempty"`
    OAuthCredentials []*OAuthCredentialEntry  `yaml:"oauth_credentials,omitempty"`
    Model            string                   `yaml:"model"`
    Timeout          int                      `yaml:"timeout"`
    MaxRetries       int                      `yaml:"max_retries"`
}

// Load configuration with environment variable substitution
func LoadConfig(path string) (*AppConfig, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    // Expand environment variables
    expanded := os.ExpandEnv(string(data))

    var config AppConfig
    if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
        return nil, err
    }

    // Validate configuration
    if err := config.Validate(); err != nil {
        return nil, err
    }

    return &config, nil
}

func (c *AppConfig) Validate() error {
    if c.Environment == "" {
        return fmt.Errorf("environment must be specified")
    }

    // Ensure at least one provider is enabled
    hasProvider := false
    if c.Providers.OpenAI != nil && c.Providers.OpenAI.Enabled {
        hasProvider = true
    }
    if c.Providers.Anthropic != nil && c.Providers.Anthropic.Enabled {
        hasProvider = true
    }

    if !hasProvider {
        return fmt.Errorf("at least one provider must be enabled")
    }

    return nil
}
```

**Example configuration:**

```yaml
environment: production

providers:
  anthropic:
    enabled: true
    api_keys:
      - ${ANTHROPIC_API_KEY_1}
      - ${ANTHROPIC_API_KEY_2}
    model: claude-3-5-sonnet-20241022
    timeout: 30
    max_retries: 3

  gemini:
    enabled: true
    oauth_credentials:
      - id: primary-account
        client_id: ${GEMINI_CLIENT_ID}
        client_secret: ${GEMINI_CLIENT_SECRET}
        refresh_token: ${GEMINI_REFRESH_TOKEN}

security:
  encryption_key: ${ENCRYPTION_KEY}
  token_rotation_days: 30
  audit_logging: true

performance:
  connection_pool_size: 100
  request_timeout: 60s
  enable_caching: true
  cache_ttl: 300s
```

### 1.3 Dependency Injection Pattern

```go
package service

type AIService struct {
    factory      ProviderFactory
    config       *Config
    cache        Cache
    metrics      MetricsCollector
    logger       Logger
}

// Constructor with dependency injection
func NewAIService(
    factory ProviderFactory,
    config *Config,
    cache Cache,
    metrics MetricsCollector,
    logger Logger,
) *AIService {
    return &AIService{
        factory: factory,
        config:  config,
        cache:   cache,
        metrics: metrics,
        logger:  logger,
    }
}

// Interface-based design for testability
type ProviderFactory interface {
    CreateProvider(types.ProviderType, types.ProviderConfig) (types.Provider, error)
}

type Cache interface {
    Get(key string) (interface{}, bool)
    Set(key string, value interface{}, ttl time.Duration)
}

type MetricsCollector interface {
    RecordLatency(operation string, duration time.Duration)
    RecordError(operation string, err error)
}
```

### 1.4 Service Layer Design

```go
package service

type CompletionService struct {
    providers map[types.ProviderType]types.Provider
    cache     *ResponseCache
    limiter   *RateLimiter
    monitor   *HealthMonitor
}

func (s *CompletionService) Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
    // 1. Validate request
    if err := req.Validate(); err != nil {
        return nil, fmt.Errorf("invalid request: %w", err)
    }

    // 2. Check cache
    if req.EnableCache {
        if cached, found := s.cache.Get(req.CacheKey()); found {
            return cached.(*CompletionResponse), nil
        }
    }

    // 3. Check rate limits
    if !s.limiter.Allow(req.UserID) {
        return nil, ErrRateLimitExceeded
    }

    // 4. Select provider
    provider := s.selectProvider(req)

    // 5. Execute with timeout
    ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
    defer cancel()

    stream, err := provider.GenerateChatCompletion(ctx, req.ToGenerateOptions())
    if err != nil {
        s.monitor.RecordFailure(provider.Type())
        return nil, err
    }
    defer stream.Close()

    // 6. Collect response
    response, err := s.collectResponse(stream)
    if err != nil {
        return nil, err
    }

    // 7. Cache if enabled
    if req.EnableCache {
        s.cache.Set(req.CacheKey(), response, 5*time.Minute)
    }

    // 8. Record metrics
    s.monitor.RecordSuccess(provider.Type())

    return response, nil
}
```

### 1.5 Error Boundary Implementation

```go
package middleware

type ErrorBoundary struct {
    logger Logger
    alerts AlertManager
}

func (eb *ErrorBoundary) Wrap(handler func(context.Context) error) func(context.Context) error {
    return func(ctx context.Context) error {
        defer func() {
            if r := recover(); r != nil {
                eb.logger.Error("panic recovered", "panic", r, "stack", debug.Stack())
                eb.alerts.Send("panic_recovered", fmt.Sprintf("%v", r))
            }
        }()

        if err := handler(ctx); err != nil {
            eb.handleError(err)
            return err
        }

        return nil
    }
}

func (eb *ErrorBoundary) handleError(err error) {
    // Classify error
    switch {
    case isRateLimitError(err):
        eb.logger.Warn("rate limit exceeded", "error", err)

    case isAuthError(err):
        eb.logger.Error("authentication failed", "error", err)
        eb.alerts.Send("auth_failure", err.Error())

    case isProviderError(err):
        eb.logger.Error("provider error", "error", err)
        eb.alerts.Send("provider_error", err.Error())

    default:
        eb.logger.Error("unexpected error", "error", err)
    }
}
```

### 1.6 Logging Strategies

```go
package logging

import (
    "context"
    "log/slog"
    "os"
)

// Structured logging setup
func NewLogger(env string) *slog.Logger {
    var handler slog.Handler

    if env == "production" {
        // JSON logging for production
        handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
            Level: slog.LevelInfo,
        })
    } else {
        // Human-readable for development
        handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
            Level: slog.LevelDebug,
        })
    }

    return slog.New(handler)
}

// Request logging middleware
type RequestLogger struct {
    logger *slog.Logger
}

func (rl *RequestLogger) LogRequest(ctx context.Context, provider types.ProviderType, options types.GenerateOptions) {
    rl.logger.InfoContext(ctx, "ai_request",
        "provider", provider,
        "model", options.Model,
        "messages", len(options.Messages),
        "max_tokens", options.MaxTokens,
        "stream", options.Stream,
    )
}

func (rl *RequestLogger) LogResponse(ctx context.Context, provider types.ProviderType, duration time.Duration, tokensUsed int64, err error) {
    if err != nil {
        rl.logger.ErrorContext(ctx, "ai_request_failed",
            "provider", provider,
            "duration_ms", duration.Milliseconds(),
            "error", err,
        )
    } else {
        rl.logger.InfoContext(ctx, "ai_request_success",
            "provider", provider,
            "duration_ms", duration.Milliseconds(),
            "tokens_used", tokensUsed,
        )
    }
}
```

---

## 2. Performance Optimization

### 2.1 Connection Pooling

The SDK uses HTTP client with connection pooling by default:

```go
package http

func NewOptimizedHTTPClient() *http.Client {
    transport := &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        DisableKeepAlives:   false,

        // TCP tuning
        DialContext: (&net.Dialer{
            Timeout:   30 * time.Second,
            KeepAlive: 30 * time.Second,
        }).DialContext,

        // TLS optimization
        TLSHandshakeTimeout: 10 * time.Second,

        // Enable compression
        DisableCompression: false,
    }

    return &http.Client{
        Transport: transport,
        Timeout:   60 * time.Second,
    }
}
```

### 2.2 Request Batching

```go
package batch

type RequestBatcher struct {
    maxBatchSize int
    maxWaitTime  time.Duration
    processor    BatchProcessor
    queue        chan *Request
    mu           sync.Mutex
}

func NewRequestBatcher(maxSize int, maxWait time.Duration, processor BatchProcessor) *RequestBatcher {
    b := &RequestBatcher{
        maxBatchSize: maxSize,
        maxWaitTime:  maxWait,
        processor:    processor,
        queue:        make(chan *Request, 1000),
    }

    go b.run()
    return b
}

func (b *RequestBatcher) Submit(req *Request) <-chan *Response {
    responseChan := make(chan *Response, 1)
    req.responseChan = responseChan
    b.queue <- req
    return responseChan
}

func (b *RequestBatcher) run() {
    batch := make([]*Request, 0, b.maxBatchSize)
    timer := time.NewTimer(b.maxWaitTime)

    for {
        select {
        case req := <-b.queue:
            batch = append(batch, req)

            if len(batch) >= b.maxBatchSize {
                b.processBatch(batch)
                batch = make([]*Request, 0, b.maxBatchSize)
                timer.Reset(b.maxWaitTime)
            }

        case <-timer.C:
            if len(batch) > 0 {
                b.processBatch(batch)
                batch = make([]*Request, 0, b.maxBatchSize)
            }
            timer.Reset(b.maxWaitTime)
        }
    }
}

func (b *RequestBatcher) processBatch(batch []*Request) {
    responses := b.processor.ProcessBatch(batch)

    for i, req := range batch {
        req.responseChan <- responses[i]
        close(req.responseChan)
    }
}
```

### 2.3 Caching Strategies

#### Response Caching

```go
package cache

import (
    "context"
    "crypto/sha256"
    "encoding/json"
    "time"
)

type ResponseCache struct {
    store    map[string]*CachedResponse
    ttl      time.Duration
    maxSize  int
    mu       sync.RWMutex
}

type CachedResponse struct {
    Content   string
    Usage     types.Usage
    CreatedAt time.Time
    ExpiresAt time.Time
}

func NewResponseCache(ttl time.Duration, maxSize int) *ResponseCache {
    c := &ResponseCache{
        store:   make(map[string]*CachedResponse),
        ttl:     ttl,
        maxSize: maxSize,
    }

    // Start cleanup goroutine
    go c.cleanup()

    return c
}

func (c *ResponseCache) Get(key string) (*CachedResponse, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    cached, exists := c.store[key]
    if !exists {
        return nil, false
    }

    if time.Now().After(cached.ExpiresAt) {
        return nil, false
    }

    return cached, true
}

func (c *ResponseCache) Set(key string, content string, usage types.Usage) {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Evict oldest if at capacity
    if len(c.store) >= c.maxSize {
        c.evictOldest()
    }

    c.store[key] = &CachedResponse{
        Content:   content,
        Usage:     usage,
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(c.ttl),
    }
}

func (c *ResponseCache) GenerateKey(messages []types.ChatMessage, options types.GenerateOptions) string {
    data := struct {
        Messages    []types.ChatMessage
        Model       string
        Temperature float64
        MaxTokens   int
    }{
        Messages:    messages,
        Model:       options.Model,
        Temperature: options.Temperature,
        MaxTokens:   options.MaxTokens,
    }

    jsonData, _ := json.Marshal(data)
    hash := sha256.Sum256(jsonData)
    return fmt.Sprintf("%x", hash)
}

func (c *ResponseCache) cleanup() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        c.mu.Lock()
        now := time.Now()
        for key, cached := range c.store {
            if now.After(cached.ExpiresAt) {
                delete(c.store, key)
            }
        }
        c.mu.Unlock()
    }
}
```

#### Semantic Caching

```go
package cache

// Semantic cache using embeddings for similar queries
type SemanticCache struct {
    embeddings EmbeddingService
    vectorDB   VectorDB
    threshold  float64
}

func (sc *SemanticCache) FindSimilar(ctx context.Context, query string) (*CachedResponse, bool) {
    // Generate embedding for query
    embedding, err := sc.embeddings.Embed(ctx, query)
    if err != nil {
        return nil, false
    }

    // Search for similar queries
    results, err := sc.vectorDB.Search(ctx, embedding, 1)
    if err != nil || len(results) == 0 {
        return nil, false
    }

    // Check similarity threshold
    if results[0].Similarity < sc.threshold {
        return nil, false
    }

    return results[0].Response, true
}
```

### 2.4 Memory Management

```go
package pool

import (
    "sync"
)

// Object pool for reducing allocations
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 0, 4096)
    },
}

func GetBuffer() []byte {
    return bufferPool.Get().([]byte)
}

func PutBuffer(buf []byte) {
    bufferPool.Put(buf[:0])
}

// Stream chunk pool
var chunkPool = sync.Pool{
    New: func() interface{} {
        return &types.ChatCompletionChunk{}
    },
}

func GetChunk() *types.ChatCompletionChunk {
    chunk := chunkPool.Get().(*types.ChatCompletionChunk)
    // Reset fields
    chunk.Content = ""
    chunk.Done = false
    return chunk
}

func PutChunk(chunk *types.ChatCompletionChunk) {
    chunkPool.Put(chunk)
}
```

### 2.5 Goroutine Optimization

```go
package worker

type WorkerPool struct {
    workers   int
    taskQueue chan Task
    wg        sync.WaitGroup
}

func NewWorkerPool(workers int) *WorkerPool {
    p := &WorkerPool{
        workers:   workers,
        taskQueue: make(chan Task, workers*2),
    }

    // Start workers
    for i := 0; i < workers; i++ {
        p.wg.Add(1)
        go p.worker()
    }

    return p
}

func (p *WorkerPool) Submit(task Task) {
    p.taskQueue <- task
}

func (p *WorkerPool) worker() {
    defer p.wg.Done()

    for task := range p.taskQueue {
        task.Execute()
    }
}

func (p *WorkerPool) Shutdown() {
    close(p.taskQueue)
    p.wg.Wait()
}
```

### 2.6 Latency Reduction

#### Circuit Breaker Pattern

```go
package resilience

type CircuitBreaker struct {
    maxFailures  int
    resetTimeout time.Duration
    state        State
    failures     int
    lastFailTime time.Time
    mu           sync.Mutex
}

type State int

const (
    StateClosed State = iota
    StateOpen
    StateHalfOpen
)

func (cb *CircuitBreaker) Execute(fn func() error) error {
    cb.mu.Lock()

    // Check state
    if cb.state == StateOpen {
        if time.Since(cb.lastFailTime) > cb.resetTimeout {
            cb.state = StateHalfOpen
            cb.failures = 0
        } else {
            cb.mu.Unlock()
            return ErrCircuitOpen
        }
    }

    cb.mu.Unlock()

    // Execute function
    err := fn()

    cb.mu.Lock()
    defer cb.mu.Unlock()

    if err != nil {
        cb.failures++
        cb.lastFailTime = time.Now()

        if cb.failures >= cb.maxFailures {
            cb.state = StateOpen
        }

        return err
    }

    // Success - reset or close
    if cb.state == StateHalfOpen {
        cb.state = StateClosed
    }
    cb.failures = 0

    return nil
}
```

### 2.7 Benchmark Results

Based on internal testing with the AI Provider Kit:

```
Provider Performance Benchmarks (1000 requests, 100 tokens avg)
================================================================

Provider      | Avg Latency | P95 Latency | Throughput | Success Rate
--------------|-------------|-------------|------------|-------------
Cerebras      | 145ms       | 180ms       | 500 req/s  | 99.8%
Gemini        | 280ms       | 350ms       | 350 req/s  | 99.5%
Anthropic     | 320ms       | 420ms       | 300 req/s  | 99.9%
OpenAI        | 380ms       | 510ms       | 250 req/s  | 99.7%
Qwen          | 410ms       | 550ms       | 220 req/s  | 99.3%

Multi-Key Failover Overhead
---------------------------
Single key:     320ms avg
2 keys:         325ms avg (+1.5%)
5 keys:         335ms avg (+4.7%)

Caching Impact
--------------
No cache:       350ms avg
Response cache: 85ms avg (cache hit), 360ms (miss)
Hit rate 40%:   220ms avg effective latency

Connection Pooling Impact
-------------------------
No pooling:     450ms avg (includes handshake)
With pooling:   320ms avg (-29%)
```

**Key Optimization Recommendations:**

1. Use Cerebras for latency-sensitive applications
2. Enable response caching for 20-40% latency reduction
3. Use connection pooling (default in SDK)
4. Multi-key failover adds <5% overhead
5. Batch requests when possible for 2-3x throughput

---

## 3. Security Best Practices

### 3.1 Credential Management

#### Secure Storage

```go
package security

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
)

type CredentialVault struct {
    cipher cipher.AEAD
}

func NewCredentialVault(encryptionKey []byte) (*CredentialVault, error) {
    block, err := aes.NewCipher(encryptionKey)
    if err != nil {
        return nil, err
    }

    aead, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    return &CredentialVault{cipher: aead}, nil
}

func (v *CredentialVault) Encrypt(plaintext string) (string, error) {
    nonce := make([]byte, v.cipher.NonceSize())
    if _, err := rand.Read(nonce); err != nil {
        return "", err
    }

    ciphertext := v.cipher.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (v *CredentialVault) Decrypt(encrypted string) (string, error) {
    data, err := base64.StdEncoding.DecodeString(encrypted)
    if err != nil {
        return "", err
    }

    nonceSize := v.cipher.NonceSize()
    if len(data) < nonceSize {
        return "", fmt.Errorf("ciphertext too short")
    }

    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    plaintext, err := v.cipher.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }

    return string(plaintext), nil
}
```

#### Environment Variables

```go
// Use environment variables for sensitive data
func loadCredentials() map[string]string {
    return map[string]string{
        "ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
        "GEMINI_CLIENT_SECRET": os.Getenv("GEMINI_CLIENT_SECRET"),
        "OPENAI_API_KEY": os.Getenv("OPENAI_API_KEY"),
    }
}

// Validate all required credentials are present
func validateCredentials(creds map[string]string) error {
    required := []string{"ANTHROPIC_API_KEY", "GEMINI_CLIENT_SECRET"}

    for _, key := range required {
        if creds[key] == "" {
            return fmt.Errorf("missing required credential: %s", key)
        }
    }

    return nil
}
```

### 3.2 Token Storage

#### Secure Token Persistence

```go
package auth

type TokenStore struct {
    vault     *CredentialVault
    storage   Storage
}

func (ts *TokenStore) SaveToken(credID string, token *types.OAuthCredentialSet) error {
    // Encrypt sensitive fields
    encryptedAccess, err := ts.vault.Encrypt(token.AccessToken)
    if err != nil {
        return err
    }

    encryptedRefresh, err := ts.vault.Encrypt(token.RefreshToken)
    if err != nil {
        return err
    }

    // Store encrypted tokens
    data := &StoredToken{
        ID:           credID,
        AccessToken:  encryptedAccess,
        RefreshToken: encryptedRefresh,
        ExpiresAt:    token.ExpiresAt,
        UpdatedAt:    time.Now(),
    }

    return ts.storage.Save(credID, data)
}

func (ts *TokenStore) LoadToken(credID string) (*types.OAuthCredentialSet, error) {
    data, err := ts.storage.Load(credID)
    if err != nil {
        return nil, err
    }

    // Decrypt tokens
    accessToken, err := ts.vault.Decrypt(data.AccessToken)
    if err != nil {
        return nil, err
    }

    refreshToken, err := ts.vault.Decrypt(data.RefreshToken)
    if err != nil {
        return nil, err
    }

    return &types.OAuthCredentialSet{
        ID:           credID,
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresAt:    data.ExpiresAt,
    }, nil
}
```

### 3.3 Encryption at Rest and in Transit

```go
// TLS configuration for production
func createSecureHTTPClient() *http.Client {
    tlsConfig := &tls.Config{
        MinVersion: tls.VersionTLS13,
        CipherSuites: []uint16{
            tls.TLS_AES_256_GCM_SHA384,
            tls.TLS_AES_128_GCM_SHA256,
        },
    }

    transport := &http.Transport{
        TLSClientConfig: tlsConfig,
    }

    return &http.Client{
        Transport: transport,
        Timeout:   60 * time.Second,
    }
}
```

### 3.4 Audit Logging

```go
package audit

type AuditLogger struct {
    logger *slog.Logger
    writer io.Writer
}

func (al *AuditLogger) LogAPICall(ctx context.Context, event APICallEvent) {
    entry := AuditEntry{
        Timestamp:  time.Now(),
        EventType:  "api_call",
        UserID:     event.UserID,
        Provider:   string(event.Provider),
        Model:      event.Model,
        TokensUsed: event.TokensUsed,
        Success:    event.Error == nil,
        IPAddress:  getClientIP(ctx),
    }

    if event.Error != nil {
        entry.ErrorMessage = event.Error.Error()
    }

    al.writeAuditEntry(entry)
}

func (al *AuditLogger) LogTokenRefresh(credID string, success bool) {
    entry := AuditEntry{
        Timestamp:    time.Now(),
        EventType:    "token_refresh",
        CredentialID: credID,
        Success:      success,
    }

    al.writeAuditEntry(entry)
}

func (al *AuditLogger) writeAuditEntry(entry AuditEntry) {
    data, _ := json.Marshal(entry)
    al.writer.Write(append(data, '\n'))
}
```

### 3.5 Compliance Considerations

#### GDPR Compliance

```go
// Implement data minimization
type CompletionRequest struct {
    Messages    []types.ChatMessage
    Options     types.GenerateOptions

    // Don't log PII
    UserID      string `json:"-"` // Excluded from logs
    SessionID   string `json:"-"`
}

// Data retention policy
type DataRetentionPolicy struct {
    RetentionDays int
    AutoDelete    bool
}

func (drp *DataRetentionPolicy) CleanupOldData(ctx context.Context) error {
    cutoff := time.Now().AddDate(0, 0, -drp.RetentionDays)

    // Delete old audit logs
    if err := deleteAuditLogsBefore(ctx, cutoff); err != nil {
        return err
    }

    // Delete old cached responses
    if err := deleteCachedResponsesBefore(ctx, cutoff); err != nil {
        return err
    }

    return nil
}
```

#### SOC2 Controls

```go
// Access control
type AccessControl struct {
    rbac *RBACManager
}

func (ac *AccessControl) CheckPermission(userID string, resource string, action string) bool {
    return ac.rbac.HasPermission(userID, resource, action)
}

// Change tracking
type ChangeTracker struct {
    auditLog AuditLogger
}

func (ct *ChangeTracker) TrackConfigChange(userID string, field string, oldValue, newValue interface{}) {
    ct.auditLog.LogConfigChange(ConfigChangeEvent{
        Timestamp: time.Now(),
        UserID:    userID,
        Field:     field,
        OldValue:  oldValue,
        NewValue:  newValue,
    })
}
```

### 3.6 Security Scanning Integration

```go
// Dependency scanning in CI/CD
// .github/workflows/security.yml

name: Security Scan

on: [push, pull_request]

jobs:
  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Run Gosec Security Scanner
        uses: securego/gosec@master
        with:
          args: ./...

      - name: Run Trivy vulnerability scanner
        uses: aquasecurity/trivy-action@master
        with:
          scan-type: 'fs'
          scan-ref: '.'
```

---

## 4. Production Deployment

### 4.1 Environment Configuration

```go
package deployment

type Environment string

const (
    EnvDevelopment Environment = "development"
    EnvStaging    Environment = "staging"
    EnvProduction Environment = "production"
)

type DeploymentConfig struct {
    Environment Environment
    LogLevel    string
    Providers   map[string]ProviderDeploymentConfig
    Security    SecurityConfig
    Monitoring  MonitoringConfig
}

type ProviderDeploymentConfig struct {
    Replicas      int
    MaxConcurrent int
    Timeout       time.Duration
    HealthCheck   HealthCheckConfig
}

func LoadDeploymentConfig(env Environment) (*DeploymentConfig, error) {
    var configPath string

    switch env {
    case EnvDevelopment:
        configPath = "config/dev.yaml"
    case EnvStaging:
        configPath = "config/staging.yaml"
    case EnvProduction:
        configPath = "config/prod.yaml"
    default:
        return nil, fmt.Errorf("unknown environment: %s", env)
    }

    return loadConfigFromFile(configPath)
}
```

### 4.2 Secret Management

#### HashiCorp Vault Integration

```go
package secrets

import (
    vault "github.com/hashicorp/vault/api"
)

type VaultSecretManager struct {
    client *vault.Client
    path   string
}

func NewVaultSecretManager(address, token, path string) (*VaultSecretManager, error) {
    config := vault.DefaultConfig()
    config.Address = address

    client, err := vault.NewClient(config)
    if err != nil {
        return nil, err
    }

    client.SetToken(token)

    return &VaultSecretManager{
        client: client,
        path:   path,
    }, nil
}

func (vsm *VaultSecretManager) GetSecret(key string) (string, error) {
    secret, err := vsm.client.Logical().Read(fmt.Sprintf("%s/%s", vsm.path, key))
    if err != nil {
        return "", err
    }

    if secret == nil || secret.Data == nil {
        return "", fmt.Errorf("secret not found: %s", key)
    }

    value, ok := secret.Data["value"].(string)
    if !ok {
        return "", fmt.Errorf("invalid secret format for %s", key)
    }

    return value, nil
}

func (vsm *VaultSecretManager) LoadProviderCredentials(provider string) (*types.ProviderConfig, error) {
    config := &types.ProviderConfig{
        Type: types.ProviderType(provider),
    }

    // Load API key
    apiKey, err := vsm.GetSecret(fmt.Sprintf("%s/api_key", provider))
    if err == nil {
        config.APIKey = apiKey
    }

    // Load OAuth credentials if applicable
    if provider == "gemini" || provider == "qwen" {
        clientID, _ := vsm.GetSecret(fmt.Sprintf("%s/client_id", provider))
        clientSecret, _ := vsm.GetSecret(fmt.Sprintf("%s/client_secret", provider))
        refreshToken, _ := vsm.GetSecret(fmt.Sprintf("%s/refresh_token", provider))

        config.OAuthCredentials = []*types.OAuthCredentialSet{
            {
                ID:           "vault-managed",
                ClientID:     clientID,
                ClientSecret: clientSecret,
                RefreshToken: refreshToken,
            },
        }
    }

    return config, nil
}
```

#### AWS Secrets Manager Integration

```go
package secrets

import (
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/secretsmanager"
)

type AWSSecretManager struct {
    client *secretsmanager.SecretsManager
}

func NewAWSSecretManager(region string) (*AWSSecretManager, error) {
    sess, err := session.NewSession(&aws.Config{
        Region: aws.String(region),
    })
    if err != nil {
        return nil, err
    }

    return &AWSSecretManager{
        client: secretsmanager.New(sess),
    }, nil
}

func (asm *AWSSecretManager) GetSecret(secretName string) (string, error) {
    input := &secretsmanager.GetSecretValueInput{
        SecretId: aws.String(secretName),
    }

    result, err := asm.client.GetSecretValue(input)
    if err != nil {
        return "", err
    }

    return *result.SecretString, nil
}
```

### 4.3 Container Deployment

**Dockerfile:**

```dockerfile
# Multi-stage build
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/api

# Runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/main .

# Copy config templates
COPY --from=builder /app/config ./config

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./main"]
```

**docker-compose.yml:**

```yaml
version: '3.8'

services:
  ai-provider-api:
    build: .
    ports:
      - "8080:8080"
    environment:
      - ENVIRONMENT=production
      - LOG_LEVEL=info
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - GEMINI_CLIENT_ID=${GEMINI_CLIENT_ID}
      - GEMINI_CLIENT_SECRET=${GEMINI_CLIENT_SECRET}
    volumes:
      - ./config:/root/config:ro
    restart: unless-stopped
    deploy:
      replicas: 3
      resources:
        limits:
          cpus: '2'
          memory: 1G
        reservations:
          cpus: '1'
          memory: 512M
    healthcheck:
      test: ["CMD", "wget", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    restart: unless-stopped

volumes:
  redis-data:
```

### 4.4 Kubernetes Configuration

**deployment.yaml:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-provider-api
  labels:
    app: ai-provider-api
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  selector:
    matchLabels:
      app: ai-provider-api
  template:
    metadata:
      labels:
        app: ai-provider-api
    spec:
      containers:
      - name: api
        image: ai-provider-api:latest
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: ENVIRONMENT
          value: "production"
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: ai-provider-secrets
              key: anthropic-api-key
        - name: GEMINI_CLIENT_ID
          valueFrom:
            secretKeyRef:
              name: ai-provider-secrets
              key: gemini-client-id
        - name: GEMINI_CLIENT_SECRET
          valueFrom:
            secretKeyRef:
              name: ai-provider-secrets
              key: gemini-client-secret
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "2000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 2
---
apiVersion: v1
kind: Service
metadata:
  name: ai-provider-api
spec:
  selector:
    app: ai-provider-api
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: ai-provider-api-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: ai-provider-api
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### 4.5 Health Check Endpoints

```go
package api

type HealthHandler struct {
    providers map[types.ProviderType]types.Provider
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    health := HealthResponse{
        Status:    "healthy",
        Timestamp: time.Now(),
        Providers: make(map[string]ProviderHealth),
    }

    // Check each provider
    for providerType, provider := range h.providers {
        providerHealth := ProviderHealth{
            Status: "unknown",
        }

        if err := provider.HealthCheck(ctx); err != nil {
            providerHealth.Status = "unhealthy"
            providerHealth.Error = err.Error()
            health.Status = "degraded"
        } else {
            providerHealth.Status = "healthy"
        }

        // Add metrics
        metrics := provider.GetMetrics()
        providerHealth.Metrics = MetricsSummary{
            RequestCount: metrics.RequestCount,
            SuccessRate:  calculateSuccessRate(metrics),
            AvgLatency:   metrics.AverageLatency.String(),
        }

        health.Providers[string(providerType)] = providerHealth
    }

    statusCode := http.StatusOK
    if health.Status == "degraded" {
        statusCode = http.StatusServiceUnavailable
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(health)
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
    // Lightweight check for Kubernetes readiness
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("ready"))
}
```

### 4.6 Monitoring Setup

#### Prometheus Metrics

```go
package monitoring

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    requestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ai_provider_requests_total",
            Help: "Total number of AI provider requests",
        },
        []string{"provider", "model", "status"},
    )

    requestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "ai_provider_request_duration_seconds",
            Help:    "AI provider request duration in seconds",
            Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
        },
        []string{"provider", "model"},
    )

    tokensUsed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ai_provider_tokens_used_total",
            Help: "Total tokens used per provider",
        },
        []string{"provider", "model", "type"},
    )
)

type MetricsMiddleware struct {
    next http.Handler
}

func (m *MetricsMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    start := time.Now()

    // Wrap response writer to capture status
    wrapped := &statusWriter{ResponseWriter: w}

    m.next.ServeHTTP(wrapped, r)

    duration := time.Since(start)

    // Extract provider and model from context
    provider := getProviderFromContext(r.Context())
    model := getModelFromContext(r.Context())

    // Record metrics
    status := "success"
    if wrapped.status >= 400 {
        status = "error"
    }

    requestsTotal.WithLabelValues(provider, model, status).Inc()
    requestDuration.WithLabelValues(provider, model).Observe(duration.Seconds())
}
```

### 4.7 Alerting Strategies

```go
package alerting

type AlertManager struct {
    notifiers []Notifier
    rules     []AlertRule
}

type AlertRule struct {
    Name      string
    Condition func(*types.ProviderMetrics) bool
    Severity  Severity
    Message   string
}

type Severity string

const (
    SeverityInfo     Severity = "info"
    SeverityWarning  Severity = "warning"
    SeverityCritical Severity = "critical"
)

func (am *AlertManager) CheckAlerts(metrics map[types.ProviderType]*types.ProviderMetrics) {
    for providerType, providerMetrics := range metrics {
        for _, rule := range am.rules {
            if rule.Condition(providerMetrics) {
                alert := Alert{
                    Name:      rule.Name,
                    Provider:  providerType,
                    Severity:  rule.Severity,
                    Message:   rule.Message,
                    Timestamp: time.Now(),
                    Metrics:   providerMetrics,
                }

                am.sendAlert(alert)
            }
        }
    }
}

// Example alert rules
var DefaultAlertRules = []AlertRule{
    {
        Name: "high_error_rate",
        Condition: func(m *types.ProviderMetrics) bool {
            if m.RequestCount == 0 {
                return false
            }
            errorRate := float64(m.ErrorCount) / float64(m.RequestCount)
            return errorRate > 0.1 // 10% error rate
        },
        Severity: SeverityCritical,
        Message:  "Error rate exceeded 10%",
    },
    {
        Name: "high_latency",
        Condition: func(m *types.ProviderMetrics) bool {
            return m.AverageLatency > 5*time.Second
        },
        Severity: SeverityWarning,
        Message:  "Average latency exceeded 5 seconds",
    },
    {
        Name: "provider_unhealthy",
        Condition: func(m *types.ProviderMetrics) bool {
            return !m.HealthStatus.Healthy
        },
        Severity: SeverityCritical,
        Message:  "Provider health check failing",
    },
}
```

---

## 5. Multi-Tenancy

### 5.1 Tenant Isolation

```go
package tenancy

type TenantManager struct {
    tenants map[string]*Tenant
    mu      sync.RWMutex
}

type Tenant struct {
    ID          string
    Name        string
    Providers   map[types.ProviderType]*TenantProviderConfig
    RateLimits  RateLimitConfig
    CostLimits  CostLimitConfig
    CreatedAt   time.Time
}

type TenantProviderConfig struct {
    Enabled     bool
    Credentials *types.ProviderConfig
    Quota       QuotaConfig
}

func (tm *TenantManager) GetTenant(tenantID string) (*Tenant, error) {
    tm.mu.RLock()
    defer tm.mu.RUnlock()

    tenant, exists := tm.tenants[tenantID]
    if !exists {
        return nil, fmt.Errorf("tenant not found: %s", tenantID)
    }

    return tenant, nil
}

func (tm *TenantManager) CreateProvider(ctx context.Context, tenantID string, providerType types.ProviderType) (types.Provider, error) {
    tenant, err := tm.GetTenant(tenantID)
    if err != nil {
        return nil, err
    }

    providerConfig, exists := tenant.Providers[providerType]
    if !exists || !providerConfig.Enabled {
        return nil, fmt.Errorf("provider not enabled for tenant: %s", providerType)
    }

    // Create isolated provider instance for tenant
    factory := factory.NewProviderFactory()
    factory.RegisterDefaultProviders(factory)

    provider, err := factory.CreateProvider(providerType, *providerConfig.Credentials)
    if err != nil {
        return nil, err
    }

    return provider, nil
}
```

### 5.2 Credential Segregation

```go
package tenancy

type TenantCredentialStore struct {
    vault      *CredentialVault
    tenantData map[string]map[string]*types.OAuthCredentialSet
    mu         sync.RWMutex
}

func (tcs *TenantCredentialStore) GetCredentials(tenantID string, provider string) ([]*types.OAuthCredentialSet, error) {
    tcs.mu.RLock()
    defer tcs.mu.RUnlock()

    tenantCreds, exists := tcs.tenantData[tenantID]
    if !exists {
        return nil, fmt.Errorf("no credentials for tenant: %s", tenantID)
    }

    creds, exists := tenantCreds[provider]
    if !exists {
        return nil, fmt.Errorf("no %s credentials for tenant: %s", provider, tenantID)
    }

    return []*types.OAuthCredentialSet{creds}, nil
}

func (tcs *TenantCredentialStore) UpdateCredentials(tenantID string, provider string, creds *types.OAuthCredentialSet) error {
    tcs.mu.Lock()
    defer tcs.mu.Unlock()

    if _, exists := tcs.tenantData[tenantID]; !exists {
        tcs.tenantData[tenantID] = make(map[string]*types.OAuthCredentialSet)
    }

    tcs.tenantData[tenantID][provider] = creds

    // Persist to secure storage
    return tcs.vault.SaveTenantCredentials(tenantID, provider, creds)
}
```

### 5.3 Rate Limit Management Per Tenant

```go
package tenancy

type TenantRateLimiter struct {
    limiters map[string]*rate.Limiter
    configs  map[string]RateLimitConfig
    mu       sync.RWMutex
}

type RateLimitConfig struct {
    RequestsPerSecond int
    BurstSize         int
    TokensPerMinute   int
}

func (trl *TenantRateLimiter) Allow(tenantID string) bool {
    trl.mu.RLock()
    limiter, exists := trl.limiters[tenantID]
    trl.mu.RUnlock()

    if !exists {
        // Create limiter for new tenant
        trl.mu.Lock()
        config := trl.configs[tenantID]
        limiter = rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.BurstSize)
        trl.limiters[tenantID] = limiter
        trl.mu.Unlock()
    }

    return limiter.Allow()
}

func (trl *TenantRateLimiter) CheckTokenQuota(tenantID string, tokensNeeded int) bool {
    trl.mu.RLock()
    config := trl.configs[tenantID]
    trl.mu.RUnlock()

    // Check if tenant has enough token quota remaining
    used := trl.getTokensUsedThisMinute(tenantID)
    return used+tokensNeeded <= config.TokensPerMinute
}
```

### 5.4 Cost Attribution

```go
package billing

type CostTracker struct {
    usage map[string]*TenantUsage
    mu    sync.RWMutex
}

type TenantUsage struct {
    TenantID      string
    Provider      types.ProviderType
    Model         string
    TokensUsed    int64
    RequestCount  int64
    Cost          float64
    Period        time.Time
}

func (ct *CostTracker) RecordUsage(tenantID string, provider types.ProviderType, model string, tokens int64) {
    ct.mu.Lock()
    defer ct.mu.Unlock()

    key := fmt.Sprintf("%s:%s:%s", tenantID, provider, model)

    if _, exists := ct.usage[key]; !exists {
        ct.usage[key] = &TenantUsage{
            TenantID: tenantID,
            Provider: provider,
            Model:    model,
            Period:   time.Now().Truncate(24 * time.Hour),
        }
    }

    usage := ct.usage[key]
    usage.TokensUsed += tokens
    usage.RequestCount++
    usage.Cost += ct.calculateCost(provider, model, tokens)
}

func (ct *CostTracker) calculateCost(provider types.ProviderType, model string, tokens int64) float64 {
    // Pricing per 1K tokens
    prices := map[string]float64{
        "claude-3-5-sonnet-20241022": 0.015,  // $15/1M tokens
        "gpt-4":                       0.030,  // $30/1M tokens
        "gemini-2.0-flash-exp":       0.0003, // $0.30/1M tokens
        "llama3.1-8b":                0.0001, // $0.10/1M tokens (Cerebras)
    }

    pricePerToken := prices[model] / 1000.0
    return float64(tokens) * pricePerToken
}

func (ct *CostTracker) GetTenantCosts(tenantID string, period time.Time) []TenantUsage {
    ct.mu.RLock()
    defer ct.mu.RUnlock()

    var costs []TenantUsage

    for key, usage := range ct.usage {
        if usage.TenantID == tenantID && usage.Period.Equal(period) {
            costs = append(costs, *usage)
        }
    }

    return costs
}
```

### 5.5 Usage Tracking

```go
package analytics

type UsageAnalytics struct {
    store AnalyticsStore
}

type UsageMetrics struct {
    TenantID        string
    Provider        types.ProviderType
    RequestCount    int64
    TokensUsed      int64
    AverageLatency  time.Duration
    ErrorRate       float64
    TopModels       []ModelUsage
    TimeRange       TimeRange
}

type ModelUsage struct {
    Model        string
    RequestCount int64
    TokensUsed   int64
    Cost         float64
}

func (ua *UsageAnalytics) GetTenantMetrics(tenantID string, timeRange TimeRange) (*UsageMetrics, error) {
    records, err := ua.store.QueryUsage(tenantID, timeRange)
    if err != nil {
        return nil, err
    }

    metrics := &UsageMetrics{
        TenantID:  tenantID,
        TimeRange: timeRange,
        TopModels: make([]ModelUsage, 0),
    }

    modelMap := make(map[string]*ModelUsage)

    for _, record := range records {
        metrics.RequestCount += record.RequestCount
        metrics.TokensUsed += record.TokensUsed

        // Track by model
        if _, exists := modelMap[record.Model]; !exists {
            modelMap[record.Model] = &ModelUsage{Model: record.Model}
        }

        modelMap[record.Model].RequestCount += record.RequestCount
        modelMap[record.Model].TokensUsed += record.TokensUsed
        modelMap[record.Model].Cost += record.Cost
    }

    // Convert to sorted list
    for _, usage := range modelMap {
        metrics.TopModels = append(metrics.TopModels, *usage)
    }

    // Sort by request count
    sort.Slice(metrics.TopModels, func(i, j int) bool {
        return metrics.TopModels[i].RequestCount > metrics.TopModels[j].RequestCount
    })

    return metrics, nil
}
```

### 5.6 Billing Integration

```go
package billing

type BillingService struct {
    costTracker *CostTracker
    stripe      *stripe.Client
}

func (bs *BillingService) GenerateInvoice(tenantID string, period time.Time) (*Invoice, error) {
    costs := bs.costTracker.GetTenantCosts(tenantID, period)

    invoice := &Invoice{
        TenantID:  tenantID,
        Period:    period,
        Items:     make([]InvoiceItem, 0),
        TotalCost: 0,
    }

    for _, cost := range costs {
        item := InvoiceItem{
            Provider:     string(cost.Provider),
            Model:        cost.Model,
            TokensUsed:   cost.TokensUsed,
            RequestCount: cost.RequestCount,
            Cost:         cost.Cost,
        }

        invoice.Items = append(invoice.Items, item)
        invoice.TotalCost += cost.Cost
    }

    return invoice, nil
}

func (bs *BillingService) ChargeCustomer(tenantID string, invoice *Invoice) error {
    // Get Stripe customer ID for tenant
    customerID, err := bs.getStripeCustomerID(tenantID)
    if err != nil {
        return err
    }

    // Create Stripe invoice
    params := &stripe.InvoiceParams{
        Customer: stripe.String(customerID),
    }

    for _, item := range invoice.Items {
        params.AddInvoiceItem(&stripe.InvoiceItemParams{
            Customer:    stripe.String(customerID),
            Amount:      stripe.Int64(int64(item.Cost * 100)), // Convert to cents
            Currency:    stripe.String("usd"),
            Description: stripe.String(fmt.Sprintf("%s - %s (%d tokens)", item.Provider, item.Model, item.TokensUsed)),
        })
    }

    _, err = bs.stripe.Invoices.New(params)
    return err
}
```

---

## 6. Scalability Patterns

### 6.1 Horizontal Scaling

```go
package scaling

type LoadBalancer struct {
    instances []ServiceInstance
    strategy  LoadBalancingStrategy
    mu        sync.RWMutex
}

type ServiceInstance struct {
    ID       string
    Address  string
    Healthy  bool
    Load     int
}

type LoadBalancingStrategy interface {
    SelectInstance(instances []ServiceInstance) *ServiceInstance
}

// Round-robin strategy
type RoundRobinStrategy struct {
    current uint32
}

func (rr *RoundRobinStrategy) SelectInstance(instances []ServiceInstance) *ServiceInstance {
    if len(instances) == 0 {
        return nil
    }

    idx := atomic.AddUint32(&rr.current, 1) % uint32(len(instances))
    return &instances[idx]
}

// Least connections strategy
type LeastConnectionsStrategy struct{}

func (lc *LeastConnectionsStrategy) SelectInstance(instances []ServiceInstance) *ServiceInstance {
    if len(instances) == 0 {
        return nil
    }

    minLoad := instances[0].Load
    selected := &instances[0]

    for i := range instances {
        if instances[i].Load < minLoad && instances[i].Healthy {
            minLoad = instances[i].Load
            selected = &instances[i]
        }
    }

    return selected
}
```

### 6.2 Load Balancing Across Providers

```go
package loadbalancing

type ProviderLoadBalancer struct {
    providers map[types.ProviderType][]types.Provider
    selector  ProviderSelector
}

func (plb *ProviderLoadBalancer) Route(ctx context.Context, req Request) (types.Provider, error) {
    // Get provider type based on requirements
    providerType := plb.selector.Select(req)

    instances, exists := plb.providers[providerType]
    if !exists || len(instances) == 0 {
        return nil, fmt.Errorf("no instances for provider: %s", providerType)
    }

    // Select instance with lowest load
    var selected types.Provider
    minLoad := int64(math.MaxInt64)

    for _, provider := range instances {
        metrics := provider.GetMetrics()
        currentLoad := metrics.RequestCount - metrics.SuccessCount - metrics.ErrorCount

        if currentLoad < minLoad {
            minLoad = currentLoad
            selected = provider
        }
    }

    return selected, nil
}
```

### 6.3 Queue-Based Processing

```go
package queue

import (
    "github.com/rabbitmq/amqp091-go"
)

type MessageQueue struct {
    conn    *amqp091.Connection
    channel *amqp091.Channel
    queue   string
}

func NewMessageQueue(url, queueName string) (*MessageQueue, error) {
    conn, err := amqp091.Dial(url)
    if err != nil {
        return nil, err
    }

    ch, err := conn.Channel()
    if err != nil {
        return nil, err
    }

    _, err = ch.QueueDeclare(
        queueName,
        true,  // durable
        false, // auto-delete
        false, // exclusive
        false, // no-wait
        nil,   // arguments
    )
    if err != nil {
        return nil, err
    }

    return &MessageQueue{
        conn:    conn,
        channel: ch,
        queue:   queueName,
    }, nil
}

func (mq *MessageQueue) Publish(ctx context.Context, message []byte) error {
    return mq.channel.PublishWithContext(
        ctx,
        "",       // exchange
        mq.queue, // routing key
        false,    // mandatory
        false,    // immediate
        amqp091.Publishing{
            ContentType:  "application/json",
            Body:         message,
            DeliveryMode: amqp091.Persistent,
        },
    )
}

func (mq *MessageQueue) Consume(handler func([]byte) error) error {
    msgs, err := mq.channel.Consume(
        mq.queue,
        "",    // consumer
        false, // auto-ack
        false, // exclusive
        false, // no-local
        false, // no-wait
        nil,   // args
    )
    if err != nil {
        return err
    }

    for msg := range msgs {
        if err := handler(msg.Body); err != nil {
            msg.Nack(false, true) // Requeue on error
        } else {
            msg.Ack(false)
        }
    }

    return nil
}
```

### 6.4 Async Operations

```go
package async

type AsyncProcessor struct {
    workers  int
    queue    chan Task
    results  map[string]chan Result
    mu       sync.RWMutex
}

type Task struct {
    ID      string
    Request types.GenerateOptions
}

type Result struct {
    ID      string
    Content string
    Usage   types.Usage
    Error   error
}

func NewAsyncProcessor(workers int) *AsyncProcessor {
    ap := &AsyncProcessor{
        workers: workers,
        queue:   make(chan Task, workers*10),
        results: make(map[string]chan Result),
    }

    for i := 0; i < workers; i++ {
        go ap.worker()
    }

    return ap
}

func (ap *AsyncProcessor) Submit(task Task) <-chan Result {
    resultChan := make(chan Result, 1)

    ap.mu.Lock()
    ap.results[task.ID] = resultChan
    ap.mu.Unlock()

    ap.queue <- task

    return resultChan
}

func (ap *AsyncProcessor) worker() {
    for task := range ap.queue {
        result := ap.processTask(task)

        ap.mu.RLock()
        resultChan := ap.results[task.ID]
        ap.mu.RUnlock()

        if resultChan != nil {
            resultChan <- result
            close(resultChan)

            ap.mu.Lock()
            delete(ap.results, task.ID)
            ap.mu.Unlock()
        }
    }
}

func (ap *AsyncProcessor) processTask(task Task) Result {
    // Process task with provider
    // ... implementation
    return Result{ID: task.ID}
}
```

### 6.5 Batch Processing

```go
package batch

type BatchProcessor struct {
    provider     types.Provider
    maxBatchSize int
}

func (bp *BatchProcessor) ProcessBatch(ctx context.Context, requests []types.GenerateOptions) ([]Response, error) {
    responses := make([]Response, len(requests))
    var wg sync.WaitGroup

    // Process in parallel with concurrency limit
    semaphore := make(chan struct{}, 10)

    for i, req := range requests {
        wg.Add(1)

        go func(idx int, request types.GenerateOptions) {
            defer wg.Done()

            semaphore <- struct{}{}
            defer func() { <-semaphore }()

            stream, err := bp.provider.GenerateChatCompletion(ctx, request)
            if err != nil {
                responses[idx] = Response{Error: err}
                return
            }
            defer stream.Close()

            chunk, err := stream.Next()
            if err != nil {
                responses[idx] = Response{Error: err}
                return
            }

            responses[idx] = Response{
                Content: chunk.Content,
                Usage:   chunk.Usage,
            }
        }(i, req)
    }

    wg.Wait()

    return responses, nil
}
```

### 6.6 Stream Processing

```go
package streaming

type StreamProcessor struct {
    provider types.Provider
    buffer   chan StreamChunk
}

type StreamChunk struct {
    ID      string
    Content string
    Done    bool
}

func (sp *StreamProcessor) ProcessStream(ctx context.Context, options types.GenerateOptions, callback func(StreamChunk)) error {
    stream, err := sp.provider.GenerateChatCompletion(ctx, options)
    if err != nil {
        return err
    }
    defer stream.Close()

    for {
        chunk, err := stream.Next()
        if err != nil {
            if err.Error() == "EOF" || strings.Contains(err.Error(), "EOF") {
                break
            }
            return err
        }

        streamChunk := StreamChunk{
            ID:      chunk.ID,
            Content: chunk.Content,
            Done:    chunk.Done,
        }

        callback(streamChunk)

        if chunk.Done {
            break
        }
    }

    return nil
}

// WebSocket streaming
func (sp *StreamProcessor) StreamToWebSocket(ctx context.Context, conn *websocket.Conn, options types.GenerateOptions) error {
    return sp.ProcessStream(ctx, options, func(chunk StreamChunk) {
        data, _ := json.Marshal(chunk)
        conn.WriteMessage(websocket.TextMessage, data)
    })
}

// SSE (Server-Sent Events) streaming
func (sp *StreamProcessor) StreamToSSE(ctx context.Context, w http.ResponseWriter, options types.GenerateOptions) error {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        return fmt.Errorf("streaming not supported")
    }

    return sp.ProcessStream(ctx, options, func(chunk StreamChunk) {
        data, _ := json.Marshal(chunk)
        fmt.Fprintf(w, "data: %s\n\n", data)
        flusher.Flush()
    })
}
```

---

## 7. Testing Strategies

### 7.1 Unit Testing with Mocks

```go
package service_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// Mock provider
type MockProvider struct {
    mock.Mock
}

func (m *MockProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    args := m.Called(ctx, options)
    return args.Get(0).(types.ChatCompletionStream), args.Error(1)
}

func (m *MockProvider) GetMetrics() types.ProviderMetrics {
    args := m.Called()
    return args.Get(0).(types.ProviderMetrics)
}

func (m *MockProvider) HealthCheck(ctx context.Context) error {
    args := m.Called(ctx)
    return args.Error(0)
}

func (m *MockProvider) Type() types.ProviderType {
    return types.ProviderTypeOpenAI
}

// Mock stream
type MockStream struct {
    chunks []types.ChatCompletionChunk
    index  int
}

func (ms *MockStream) Next() (types.ChatCompletionChunk, error) {
    if ms.index >= len(ms.chunks) {
        return types.ChatCompletionChunk{}, fmt.Errorf("EOF")
    }

    chunk := ms.chunks[ms.index]
    ms.index++
    return chunk, nil
}

func (ms *MockStream) Close() error {
    return nil
}

// Test case
func TestCompletionService_Generate(t *testing.T) {
    // Setup
    mockProvider := new(MockProvider)
    mockStream := &MockStream{
        chunks: []types.ChatCompletionChunk{
            {
                Content: "Hello, world!",
                Done:    true,
                Usage: types.Usage{
                    TotalTokens: 10,
                },
            },
        },
    }

    mockProvider.On("GenerateChatCompletion", mock.Anything, mock.Anything).
        Return(mockStream, nil)

    service := &CompletionService{
        provider: mockProvider,
    }

    // Execute
    result, err := service.Generate(context.Background(), CompletionRequest{
        Messages: []types.ChatMessage{
            {Role: "user", Content: "Hello"},
        },
    })

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, "Hello, world!", result.Content)
    assert.Equal(t, int64(10), result.TokensUsed)

    mockProvider.AssertExpectations(t)
}
```

### 7.2 Integration Testing

```go
package integration_test

import (
    "context"
    "os"
    "testing"
)

func TestAnthropicProvider_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    apiKey := os.Getenv("ANTHROPIC_API_KEY")
    if apiKey == "" {
        t.Skip("ANTHROPIC_API_KEY not set")
    }

    config := types.ProviderConfig{
        Type:   types.ProviderTypeAnthropic,
        APIKey: apiKey,
    }

    provider := anthropic.NewAnthropicProvider(config)

    t.Run("HealthCheck", func(t *testing.T) {
        err := provider.HealthCheck(context.Background())
        assert.NoError(t, err)
    })

    t.Run("GenerateCompletion", func(t *testing.T) {
        stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
            Messages: []types.ChatMessage{
                {Role: "user", Content: "Say hello"},
            },
            MaxTokens: 50,
        })

        assert.NoError(t, err)
        assert.NotNil(t, stream)

        chunk, err := stream.Next()
        assert.NoError(t, err)
        assert.NotEmpty(t, chunk.Content)
    })
}
```

### 7.3 Load Testing

```go
package loadtest

import (
    "context"
    "sync"
    "testing"
    "time"
)

func BenchmarkProvider_Concurrent(b *testing.B) {
    provider := setupProvider()

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            ctx := context.Background()
            stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
                Messages: []types.ChatMessage{
                    {Role: "user", Content: "Hello"},
                },
                MaxTokens: 50,
            })

            if err != nil {
                b.Error(err)
                continue
            }

            stream.Close()
        }
    })
}

// Load test with vegeta
func TestLoadTest(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping load test")
    }

    const (
        duration  = 60 * time.Second
        rps       = 100
        workers   = 50
    )

    provider := setupProvider()

    var wg sync.WaitGroup
    results := make(chan LoadTestResult, rps*int(duration.Seconds()))

    // Start workers
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()

            for start := time.Now(); time.Since(start) < duration; {
                result := executeRequest(provider)
                results <- result
            }
        }()
    }

    // Wait for completion
    wg.Wait()
    close(results)

    // Analyze results
    var successful, failed int
    var totalLatency time.Duration

    for result := range results {
        if result.Error != nil {
            failed++
        } else {
            successful++
            totalLatency += result.Latency
        }
    }

    avgLatency := totalLatency / time.Duration(successful)

    t.Logf("Load test results:")
    t.Logf("  Successful: %d", successful)
    t.Logf("  Failed: %d", failed)
    t.Logf("  Average latency: %v", avgLatency)
    t.Logf("  Success rate: %.2f%%", float64(successful)/float64(successful+failed)*100)
}
```

### 7.4 Chaos Engineering

```go
package chaos

import (
    "context"
    "math/rand"
    "time"
)

// Chaos middleware
type ChaosMiddleware struct {
    failureRate    float64 // 0.0 to 1.0
    latencyInjection time.Duration
}

func (cm *ChaosMiddleware) Wrap(provider types.Provider) types.Provider {
    return &ChaosProvider{
        inner:     provider,
        chaos:     cm,
    }
}

type ChaosProvider struct {
    inner types.Provider
    chaos *ChaosMiddleware
}

func (cp *ChaosProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    // Inject random failures
    if rand.Float64() < cp.chaos.failureRate {
        return nil, fmt.Errorf("chaos: injected failure")
    }

    // Inject latency
    if cp.chaos.latencyInjection > 0 {
        time.Sleep(cp.chaos.latencyInjection)
    }

    return cp.inner.GenerateChatCompletion(ctx, options)
}

// Test with chaos
func TestWithChaos(t *testing.T) {
    provider := setupProvider()

    chaos := &ChaosMiddleware{
        failureRate:      0.1,  // 10% failure rate
        latencyInjection: 100 * time.Millisecond,
    }

    chaosProvider := chaos.Wrap(provider)

    // Test resilience
    var successful, failed int

    for i := 0; i < 100; i++ {
        _, err := chaosProvider.GenerateChatCompletion(context.Background(), testOptions)
        if err != nil {
            failed++
        } else {
            successful++
        }
    }

    t.Logf("Chaos test: %d successful, %d failed", successful, failed)

    // Should have ~90% success rate with 10% failure injection
    assert.InDelta(t, 90, successful, 10)
}
```

### 7.5 A/B Testing Providers

```go
package abtest

type ABTest struct {
    providerA types.Provider
    providerB types.Provider
    traffic   float64 // 0.0 to 1.0 - percentage to provider B
    metrics   *ABMetrics
}

type ABMetrics struct {
    providerA MetricSet
    providerB MetricSet
    mu        sync.RWMutex
}

type MetricSet struct {
    RequestCount int64
    SuccessCount int64
    TotalLatency time.Duration
    TotalCost    float64
}

func (ab *ABTest) Execute(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    // Route based on traffic split
    useProviderB := rand.Float64() < ab.traffic

    var provider types.Provider
    var metrics *MetricSet

    if useProviderB {
        provider = ab.providerB
        metrics = &ab.metrics.providerB
    } else {
        provider = ab.providerA
        metrics = &ab.metrics.providerA
    }

    start := time.Now()
    stream, err := provider.GenerateChatCompletion(ctx, options)
    latency := time.Since(start)

    // Record metrics
    ab.metrics.mu.Lock()
    metrics.RequestCount++
    if err == nil {
        metrics.SuccessCount++
    }
    metrics.TotalLatency += latency
    ab.metrics.mu.Unlock()

    return stream, err
}

func (ab *ABTest) GetResults() ABTestResults {
    ab.metrics.mu.RLock()
    defer ab.metrics.mu.RUnlock()

    return ABTestResults{
        ProviderA: ProviderResults{
            RequestCount: ab.metrics.providerA.RequestCount,
            SuccessRate:  float64(ab.metrics.providerA.SuccessCount) / float64(ab.metrics.providerA.RequestCount),
            AvgLatency:   ab.metrics.providerA.TotalLatency / time.Duration(ab.metrics.providerA.RequestCount),
            TotalCost:    ab.metrics.providerA.TotalCost,
        },
        ProviderB: ProviderResults{
            RequestCount: ab.metrics.providerB.RequestCount,
            SuccessRate:  float64(ab.metrics.providerB.SuccessCount) / float64(ab.metrics.providerB.RequestCount),
            AvgLatency:   ab.metrics.providerB.TotalLatency / time.Duration(ab.metrics.providerB.RequestCount),
            TotalCost:    ab.metrics.providerB.TotalCost,
        },
    }
}
```

### 7.6 Cost Optimization Testing

```go
package costtest

func TestCostOptimization(t *testing.T) {
    providers := map[string]types.Provider{
        "gpt-4":          setupOpenAI("gpt-4"),
        "claude-3-opus":  setupAnthropic("claude-3-opus-20240229"),
        "gemini-pro":     setupGemini("gemini-pro"),
        "cerebras-llama": setupCerebras("llama3.1-8b"),
    }

    testPrompt := "Explain quantum computing"

    results := make(map[string]CostTestResult)

    for name, provider := range providers {
        result := runCostTest(provider, testPrompt)
        results[name] = result

        t.Logf("%s: Cost=$%.4f, Latency=%v, Quality=%d/10",
            name, result.Cost, result.Latency, result.QualityScore)
    }

    // Find best cost/performance ratio
    bestRatio := math.MaxFloat64
    bestProvider := ""

    for name, result := range results {
        ratio := result.Cost / float64(result.QualityScore)
        if ratio < bestRatio {
            bestRatio = ratio
            bestProvider = name
        }
    }

    t.Logf("Best cost/quality ratio: %s", bestProvider)
}

func runCostTest(provider types.Provider, prompt string) CostTestResult {
    start := time.Now()

    stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
        Messages: []types.ChatMessage{
            {Role: "user", Content: prompt},
        },
        MaxTokens: 500,
    })

    if err != nil {
        return CostTestResult{Error: err}
    }
    defer stream.Close()

    var content string
    var usage types.Usage

    chunk, _ := stream.Next()
    content = chunk.Content
    usage = chunk.Usage

    cost := calculateCost(provider.Type(), usage.TotalTokens)
    quality := assessQuality(content)

    return CostTestResult{
        Cost:         cost,
        Latency:      time.Since(start),
        TokensUsed:   int64(usage.TotalTokens),
        QualityScore: quality,
    }
}
```

---

## 8. Cost Optimization

### 8.1 Provider Cost Comparison

```go
package cost

type CostAnalyzer struct {
    pricing map[types.ProviderType]map[string]ModelPricing
}

type ModelPricing struct {
    InputPer1K  float64
    OutputPer1K float64
}

func NewCostAnalyzer() *CostAnalyzer {
    return &CostAnalyzer{
        pricing: map[types.ProviderType]map[string]ModelPricing{
            types.ProviderTypeAnthropic: {
                "claude-3-5-sonnet-20241022": {
                    InputPer1K:  0.003,  // $3/M input
                    OutputPer1K: 0.015,  // $15/M output
                },
            },
            types.ProviderTypeOpenAI: {
                "gpt-4": {
                    InputPer1K:  0.030,  // $30/M
                    OutputPer1K: 0.060,  // $60/M
                },
                "gpt-4-turbo": {
                    InputPer1K:  0.010,  // $10/M
                    OutputPer1K: 0.030,  // $30/M
                },
            },
            types.ProviderTypeGemini: {
                "gemini-2.0-flash-exp": {
                    InputPer1K:  0.0000, // Free during preview
                    OutputPer1K: 0.0000,
                },
                "gemini-pro": {
                    InputPer1K:  0.00025, // $0.25/M
                    OutputPer1K: 0.00050, // $0.50/M
                },
            },
            types.ProviderTypeCerebras: {
                "llama3.1-8b": {
                    InputPer1K:  0.0001,  // $0.10/M
                    OutputPer1K: 0.0001,
                },
                "llama3.1-70b": {
                    InputPer1K:  0.0006,  // $0.60/M
                    OutputPer1K: 0.0006,
                },
            },
        },
    }
}

func (ca *CostAnalyzer) CalculateCost(provider types.ProviderType, model string, inputTokens, outputTokens int) float64 {
    pricing, exists := ca.pricing[provider][model]
    if !exists {
        return 0
    }

    inputCost := float64(inputTokens) / 1000.0 * pricing.InputPer1K
    outputCost := float64(outputTokens) / 1000.0 * pricing.OutputPer1K

    return inputCost + outputCost
}

func (ca *CostAnalyzer) RecommendCheapest(requirements Requirements) Recommendation {
    var recommendations []Recommendation

    for providerType, models := range ca.pricing {
        for model, pricing := range models {
            estimatedCost := ca.estimateCost(pricing, requirements)

            recommendations = append(recommendations, Recommendation{
                Provider:      providerType,
                Model:         model,
                EstimatedCost: estimatedCost,
                Pricing:       pricing,
            })
        }
    }

    // Sort by cost
    sort.Slice(recommendations, func(i, j int) bool {
        return recommendations[i].EstimatedCost < recommendations[j].EstimatedCost
    })

    return recommendations[0]
}
```

**Cost comparison for common tasks:**

```
Task: Generate 1000-word article (input: 100 tokens, output: 1500 tokens)
============================================================================

Provider/Model                    | Input Cost | Output Cost | Total
----------------------------------|------------|-------------|--------
Cerebras/llama3.1-8b             | $0.00001   | $0.00015    | $0.00016
Gemini/gemini-2.0-flash-exp      | $0.00000   | $0.00000    | $0.00000
Gemini/gemini-pro                | $0.00003   | $0.00075    | $0.00078
Anthropic/claude-3-5-sonnet      | $0.00030   | $0.02250    | $0.02280
OpenAI/gpt-4-turbo               | $0.00100   | $0.04500    | $0.04600
OpenAI/gpt-4                     | $0.00300   | $0.09000    | $0.09300

Monthly cost (1M articles/month):
- Cerebras: $160
- Gemini Flash: $0 (during preview)
- Gemini Pro: $780
- Claude 3.5: $22,800
- GPT-4 Turbo: $46,000
- GPT-4: $93,000
```

### 8.2 Token Usage Optimization

```go
package optimization

type TokenOptimizer struct {
    encoder TokenEncoder
}

// Estimate tokens before sending
func (to *TokenOptimizer) EstimateTokens(text string) int {
    // Rough estimate: 1 token ≈ 4 characters for English
    return len(text) / 4
}

// Optimize prompt to reduce tokens
func (to *TokenOptimizer) OptimizePrompt(prompt string) string {
    // Remove unnecessary whitespace
    optimized := strings.TrimSpace(prompt)
    optimized = regexp.MustCompile(`\s+`).ReplaceAllString(optimized, " ")

    // Use abbreviations where appropriate
    optimized = strings.ReplaceAll(optimized, "for example", "e.g.")
    optimized = strings.ReplaceAll(optimized, "that is", "i.e.")

    return optimized
}

// Truncate context to fit within budget
func (to *TokenOptimizer) TruncateContext(messages []types.ChatMessage, maxTokens int) []types.ChatMessage {
    var result []types.ChatMessage
    totalTokens := 0

    // Keep system message
    if len(messages) > 0 && messages[0].Role == "system" {
        result = append(result, messages[0])
        totalTokens += to.EstimateTokens(messages[0].Content)
        messages = messages[1:]
    }

    // Add messages from most recent, staying within budget
    for i := len(messages) - 1; i >= 0; i-- {
        tokens := to.EstimateTokens(messages[i].Content)

        if totalTokens+tokens > maxTokens {
            break
        }

        result = append([]types.ChatMessage{messages[i]}, result...)
        totalTokens += tokens
    }

    return result
}
```

### 8.3 Caching to Reduce API Calls

```go
// Intelligent caching with cost awareness
type CostAwareCache struct {
    cache        *ResponseCache
    costTracker  *CostTracker
    savingsGoal  float64
}

func (cac *CostAwareCache) Get(key string) (*CachedResponse, bool, float64) {
    cached, found := cac.cache.Get(key)
    if !found {
        return nil, false, 0
    }

    // Calculate savings
    provider := getProviderFromKey(key)
    model := getModelFromKey(key)
    savings := cac.costTracker.CalculateCost(provider, model, cached.Usage.PromptTokens, cached.Usage.CompletionTokens)

    return cached, true, savings
}

func (cac *CostAwareCache) ReportSavings() CostSavings {
    return CostSavings{
        TotalAPICallsSaved: cac.cache.HitCount(),
        TotalCostSaved:     cac.getTotalSavings(),
        CacheHitRate:       cac.cache.HitRate(),
    }
}
```

### 8.4 Batch vs Streaming Trade-offs

```
Performance Comparison: Batch vs Streaming
===========================================

Scenario: 100 requests

Batch Processing:
- Latency: 45s total (process all at once)
- Throughput: 2.2 req/s
- Memory: High (all responses in memory)
- Cost: Same as streaming
- Best for: Background jobs, bulk processing

Streaming:
- Latency: 350ms avg per request
- Throughput: Varies (depends on concurrency)
- Memory: Low (one response at a time)
- Cost: Same as batch
- Best for: Real-time UX, incremental display

Parallel Streaming:
- Latency: 8s total (10 concurrent streams)
- Throughput: 12.5 req/s
- Memory: Medium (10 responses in memory)
- Cost: Same as others
- Best for: Balanced approach
```

### 8.5 Multi-Key Strategies for Quotas

```go
package quota

type QuotaManager struct {
    keys         []string
    quotas       map[string]*KeyQuota
    mu           sync.RWMutex
}

type KeyQuota struct {
    RequestsPerMinute int
    TokensPerMinute   int
    UsedRequests      int
    UsedTokens        int
    ResetAt           time.Time
}

func (qm *QuotaManager) SelectKey(tokensNeeded int) (string, error) {
    qm.mu.RLock()
    defer qm.mu.RUnlock()

    now := time.Now()

    for _, key := range qm.keys {
        quota := qm.quotas[key]

        // Reset if needed
        if now.After(quota.ResetAt) {
            quota.UsedRequests = 0
            quota.UsedTokens = 0
            quota.ResetAt = now.Add(1 * time.Minute)
        }

        // Check availability
        if quota.UsedRequests < quota.RequestsPerMinute &&
           quota.UsedTokens+tokensNeeded <= quota.TokensPerMinute {
            return key, nil
        }
    }

    return "", fmt.Errorf("all keys exhausted")
}

func (qm *QuotaManager) RecordUsage(key string, tokensUsed int) {
    qm.mu.Lock()
    defer qm.mu.Unlock()

    quota := qm.quotas[key]
    quota.UsedRequests++
    quota.UsedTokens += tokensUsed
}
```

---

## 9. Debugging and Troubleshooting

### 9.1 Debug Logging

```go
package debug

type DebugLogger struct {
    logger        *slog.Logger
    enableRequest bool
    enableResponse bool
}

func (dl *DebugLogger) LogRequest(ctx context.Context, provider types.ProviderType, options types.GenerateOptions) {
    if !dl.enableRequest {
        return
    }

    dl.logger.DebugContext(ctx, "api_request",
        "provider", provider,
        "model", options.Model,
        "messages", len(options.Messages),
        "max_tokens", options.MaxTokens,
        "temperature", options.Temperature,
        "stream", options.Stream,
    )

    // Log message content in development
    if dl.logger.Enabled(ctx, slog.LevelDebug) {
        for i, msg := range options.Messages {
            dl.logger.DebugContext(ctx, "message",
                "index", i,
                "role", msg.Role,
                "content", truncate(msg.Content, 200),
            )
        }
    }
}

func (dl *DebugLogger) LogResponse(ctx context.Context, provider types.ProviderType, response string, usage types.Usage, err error) {
    if !dl.enableResponse {
        return
    }

    if err != nil {
        dl.logger.ErrorContext(ctx, "api_response_error",
            "provider", provider,
            "error", err,
        )
    } else {
        dl.logger.DebugContext(ctx, "api_response",
            "provider", provider,
            "content_length", len(response),
            "content_preview", truncate(response, 100),
            "prompt_tokens", usage.PromptTokens,
            "completion_tokens", usage.CompletionTokens,
            "total_tokens", usage.TotalTokens,
        )
    }
}
```

### 9.2 Request/Response Logging

```go
package logging

type RequestLogger struct {
    storage RequestStorage
}

type RequestLog struct {
    ID               string
    Timestamp        time.Time
    Provider         types.ProviderType
    Model            string
    Request          types.GenerateOptions
    Response         string
    Usage            types.Usage
    Latency          time.Duration
    Error            error
}

func (rl *RequestLogger) Log(log RequestLog) {
    // Sanitize sensitive data
    sanitized := log
    sanitized.Request = sanitizeRequest(log.Request)

    // Store
    rl.storage.Save(sanitized)
}

func (rl *RequestLogger) Query(criteria QueryCriteria) []RequestLog {
    return rl.storage.Query(criteria)
}

// Example: Debug specific request
func debugRequest(requestID string) {
    logger := NewRequestLogger(storage)
    logs := logger.Query(QueryCriteria{
        RequestID: requestID,
    })

    if len(logs) == 0 {
        fmt.Println("Request not found")
        return
    }

    log := logs[0]
    fmt.Printf("Request ID: %s\n", log.ID)
    fmt.Printf("Timestamp: %s\n", log.Timestamp)
    fmt.Printf("Provider: %s\n", log.Provider)
    fmt.Printf("Model: %s\n", log.Model)
    fmt.Printf("Latency: %v\n", log.Latency)
    fmt.Printf("Tokens: %d\n", log.Usage.TotalTokens)

    if log.Error != nil {
        fmt.Printf("Error: %v\n", log.Error)
    }
}
```

### 9.3 Distributed Tracing

```go
package tracing

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ai-provider-kit")

func TraceCompletion(ctx context.Context, provider types.ProviderType, options types.GenerateOptions) (context.Context, trace.Span) {
    ctx, span := tracer.Start(ctx, "generate_completion",
        trace.WithAttributes(
            attribute.String("provider", string(provider)),
            attribute.String("model", options.Model),
            attribute.Int("max_tokens", options.MaxTokens),
            attribute.Float64("temperature", options.Temperature),
        ),
    )

    return ctx, span
}

// Usage
func (s *Service) Generate(ctx context.Context, options types.GenerateOptions) (*Response, error) {
    ctx, span := TraceCompletion(ctx, s.provider.Type(), options)
    defer span.End()

    // Add events
    span.AddEvent("starting_generation")

    result, err := s.provider.GenerateChatCompletion(ctx, options)
    if err != nil {
        span.RecordError(err)
        return nil, err
    }

    span.AddEvent("generation_complete")
    span.SetAttributes(
        attribute.Int64("tokens_used", result.Usage.TotalTokens),
    )

    return result, nil
}
```

### 9.4 Performance Profiling

```go
// CPU profiling
import _ "net/http/pprof"

func main() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()

    // Your application code
}

// Memory profiling
func profileMemory() {
    f, _ := os.Create("mem.prof")
    defer f.Close()

    runtime.GC()
    pprof.WriteHeapProfile(f)
}

// Benchmark with profiling
func BenchmarkGenerate(b *testing.B) {
    provider := setupProvider()

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        provider.GenerateChatCompletion(context.Background(), testOptions)
    }
}

// Run with: go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof
// Analyze with: go tool pprof cpu.prof
```

### 9.5 Common Issues and Solutions

```go
package troubleshooting

// Issue 1: Rate limit errors
func handleRateLimitError(err error) {
    if isRateLimitError(err) {
        // Parse retry-after header
        retryAfter := parseRetryAfter(err)

        log.Printf("Rate limited, retrying after %v", retryAfter)
        time.Sleep(retryAfter)

        // Retry request
    }
}

// Issue 2: Token refresh failures
func handleTokenRefreshFailure(cred *types.OAuthCredentialSet, err error) {
    log.Printf("Token refresh failed for credential %s: %v", cred.ID, err)

    // Check if refresh token is expired
    if isRefreshTokenExpired(err) {
        // Alert for manual re-authentication
        sendAlert("refresh_token_expired", cred.ID)
    }

    // Failover to next credential
}

// Issue 3: High latency
func diagnoseHighLatency(metrics types.ProviderMetrics) {
    if metrics.AverageLatency > 5*time.Second {
        log.Println("High latency detected:")

        // Check network
        if err := checkNetworkLatency(); err != nil {
            log.Printf("Network issue: %v", err)
        }

        // Check provider status
        if !metrics.HealthStatus.Healthy {
            log.Println("Provider unhealthy")
        }

        // Check concurrent requests
        if getConcurrentRequests() > 100 {
            log.Println("High concurrency may be causing delays")
        }
    }
}

// Issue 4: Memory leaks
func detectMemoryLeaks() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    if m.Alloc > 1*1024*1024*1024 { // 1GB
        log.Printf("High memory usage: %d MB", m.Alloc/1024/1024)

        // Force GC
        runtime.GC()

        // Check for goroutine leaks
        if runtime.NumGoroutine() > 1000 {
            log.Printf("Possible goroutine leak: %d goroutines", runtime.NumGoroutine())
        }
    }
}
```

---

## 10. Code Organization

### 10.1 Package Structure

```
ai-provider-kit/
├── cmd/
│   └── api/
│       └── main.go                 # Application entry point
├── pkg/
│   ├── auth/                       # Authentication
│   │   ├── apikey.go
│   │   ├── oauth.go
│   │   └── manager.go
│   ├── factory/                    # Provider factory
│   │   ├── factory.go
│   │   └── registry.go
│   ├── providers/                  # Provider implementations
│   │   ├── anthropic/
│   │   ├── cerebras/
│   │   ├── gemini/
│   │   ├── openai/
│   │   └── qwen/
│   ├── oauthmanager/              # OAuth management
│   ├── keymanager/                # API key management
│   ├── ratelimit/                 # Rate limiting
│   ├── http/                      # HTTP client utilities
│   ├── types/                     # Common types
│   └── response/                  # Response processing
├── internal/                      # Internal packages
│   ├── service/                   # Business logic
│   ├── cache/                     # Caching
│   ├── monitoring/                # Monitoring
│   └── config/                    # Configuration
├── examples/                      # Example applications
├── docs/                         # Documentation
└── tests/                        # Integration tests
```

### 10.2 Interface Design

```go
// Core provider interface
type Provider interface {
    // Essential methods
    GenerateChatCompletion(context.Context, GenerateOptions) (ChatCompletionStream, error)
    GetModels(context.Context) ([]Model, error)
    HealthCheck(context.Context) error
    GetMetrics() ProviderMetrics

    // Metadata
    Type() ProviderType
    Description() string
    GetDefaultModel() string

    // Capabilities
    SupportsStreaming() bool
    SupportsToolCalling() bool
    SupportsResponsesAPI() bool
    GetToolFormat() ToolFormat

    // Configuration
    Configure(ProviderConfig) error
    GetConfig() ProviderConfig

    // Authentication
    Authenticate(context.Context, AuthConfig) error
    IsAuthenticated() bool
    Logout(context.Context) error
}

// Separate interfaces for optional features
type ToolCaller interface {
    InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error)
}

type Streamer interface {
    StreamCompletion(ctx context.Context, options GenerateOptions) (<-chan StreamChunk, error)
}
```

### 10.3 Dependency Management

```go
// go.mod best practices

module github.com/cecil-the-coder/ai-provider-kit

go 1.23

require (
    // Direct dependencies
    github.com/stretchr/testify v1.8.4
    golang.org/x/oauth2 v0.15.0
    gopkg.in/yaml.v3 v3.0.1
)

require (
    // Indirect dependencies (managed by go mod tidy)
    github.com/davecgh/go-spew v1.1.1 // indirect
    github.com/pmezard/go-difflib v1.0.0 // indirect
)

// Use specific versions, not 'latest'
// Update regularly: go get -u ./...
// Verify: go mod verify
```

### 10.4 Using Shared Utilities

The SDK provides a comprehensive set of shared utilities in `pkg/providers/common` to reduce code duplication and ensure consistent behavior across providers.

#### Language Detection

Use the shared language detection utility for consistent file type identification:

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"

// Detect language from filename
language := common.DetectLanguage("main.go")     // "go"
language = common.DetectLanguage("script.py")    // "python"
language = common.DetectLanguage("Dockerfile")  // "dockerfile"

// Use in context processing
func processFile(filename string) error {
    language := common.DetectLanguage(filename)

    switch language {
    case "go", "python", "javascript":
        return processCodeFile(filename, language)
    case "markdown":
        return processDocumentation(filename)
    default:
        return processTextFile(filename)
    }
}
```

#### File Operations

Use shared file utilities for consistent file handling:

```go
// Read file content safely
content, err := common.ReadFileContent("config.yaml")
if err != nil {
    return fmt.Errorf("failed to read config: %w", err)
}

// Filter context files to avoid duplication
contextFiles := []string{"context1.txt", "context2.txt", "output.txt"}
filteredFiles := common.FilterContextFiles(contextFiles, "output.txt")
// Result: ["context1.txt", "context2.txt"]

// Read configuration files
configData, err := common.ReadConfigFile("app.yaml")
if err != nil {
    log.Fatal(err)
}
```

#### Rate Limiting

Use the shared rate limit helper for consistent rate limit management:

```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
)

// Initialize rate limit helper for your provider
type MyProvider struct {
    rateLimitHelper *common.RateLimitHelper
    // ... other fields
}

func NewMyProvider(config types.ProviderConfig) *MyProvider {
    parser := ratelimit.NewMyProviderParser() // Your provider's parser
    return &MyProvider{
        rateLimitHelper: common.NewRateLimitHelper(parser),
    }
}

// Check rate limits before making requests
func (p *MyProvider) makeRequest(ctx context.Context, req request) (*response, error) {
    // Estimate tokens (rough calculation)
    estimatedTokens := len(req.prompt) / 4

    // Check if we can make the request
    if !p.rateLimitHelper.CanMakeRequest(req.model, estimatedTokens) {
        // Wait if rate limited
        p.rateLimitHelper.CheckRateLimitAndWait(req.model, estimatedTokens)
    }

    // Make the actual request
    resp, err := p.doHTTPRequest(req)
    if err != nil {
        return nil, err
    }

    // Update rate limits from response headers
    p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, req.model)

    return resp, nil
}

// Implement smart throttling
func (p *MyProvider) shouldThrottle(model string) bool {
    // Start throttling at 80% of limits
    return p.rateLimitHelper.ShouldThrottle(model, 0.8)
}

// Get rate limit information for monitoring
func (p *MyProvider) GetRateLimitStatus(model string) *RateLimitStatus {
    if info, exists := p.rateLimitHelper.GetRateLimitInfo(model); exists {
        return &RateLimitStatus{
            RequestsRemaining: info.RequestsRemaining,
            RequestsLimit:     info.RequestsLimit,
            TokensRemaining:   info.TokensRemaining,
            TokensLimit:       info.TokensLimit,
            ResetTime:         info.RequestsReset,
        }
    }
    return nil
}
```

#### Model Caching

Implement efficient model list caching using the shared model cache:

```go
type MyProvider struct {
    modelCache *common.ModelCache
    // ... other fields
}

func NewMyProvider(config types.ProviderConfig) *MyProvider {
    return &MyProvider{
        modelCache: common.NewModelCache(1 * time.Hour), // Cache for 1 hour
    }
}

func (p *MyProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    return p.modelCache.GetModels(
        // Fetch fresh models from API
        func() ([]types.Model, error) {
            return p.fetchModelsFromAPI(ctx)
        },
        // Fallback to static list if API fails
        func() []types.Model {
            return p.getStaticModelList()
        },
    )
}

// Manual cache management
func (p *MyProvider) refreshModelCache(ctx context.Context) error {
    models, err := p.fetchModelsFromAPI(ctx)
    if err != nil {
        return err
    }
    p.modelCache.Update(models)
    return nil
}

func (p *MyProvider) clearModelCache() {
    p.modelCache.Clear()
}
```

#### Testing with Shared Helpers

Use the comprehensive testing helpers for provider development:

```go
func TestMyProvider(t *testing.T) {
    // Create provider with test configuration
    config := types.ProviderConfig{
        Type:   types.ProviderTypeMyProvider,
        APIKey: "test-key",
    }
    provider := NewMyProvider(config)

    // Use comprehensive test helpers
    helper := common.NewProviderTestHelpers(t, provider)

    // Test basic provider functionality
    t.Run("Basics", func(t *testing.T) {
        helper.AssertProviderBasics("myprovider", "myprovider")
        helper.AssertAuthenticated(true)
        helper.AssertSupportsFeatures(true, true, false)
    })

    // Test model availability
    t.Run("Models", func(t *testing.T) {
        helper.AssertModelExists("my-model-v1")
        helper.AssertDefaultModel("my-model-v1")
    })

    // Run complete test suite
    t.Run("Comprehensive", func(t *testing.T) {
        common.RunProviderTests(t, provider, config, []string{
            "my-model-v1",
            "my-model-v2",
        })
    })

    // Test tool calling if supported
    if provider.SupportsToolCalling() {
        t.Run("ToolCalling", func(t *testing.T) {
            toolHelper := common.NewToolCallTestHelper(t)
            toolHelper.StandardToolTestSuite(provider)
        })
    }
}

// Integration test with mock server
func TestMyProviderWithMock(t *testing.T) {
    // Create mock server
    server := common.NewMockServer(`{
        "choices": [{"message": {"content": "Hello, world!"}}],
        "usage": {"prompt_tokens": 10, "completion_tokens": 5}
    }`, 200)
    server.SetHeader("Content-Type", "application/json")
    url := server.Start()
    defer server.Close()

    // Configure provider to use mock server
    config := types.ProviderConfig{
        Type:    types.ProviderTypeMyProvider,
        BaseURL: url,
        APIKey:  "test-key",
    }

    provider := NewMyProvider(config)
    ctx := context.Background()

    // Test the provider
    stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
        Prompt: "Hello",
        Model:  "test-model",
    })
    require.NoError(t, err)
    defer stream.Close()

    chunk, err := stream.Next()
    require.NoError(t, err)
    assert.Equal(t, "Hello, world!", chunk.Content)
}
```

#### Provider Development Best Practices

When developing new providers, follow these patterns using shared utilities:

```go
// Provider structure with shared utilities
type MyProvider struct {
    config          types.ProviderConfig
    client          *http.Client
    rateLimitHelper *common.RateLimitHelper
    modelCache      *common.ModelCache
    mutex           sync.RWMutex
}

// Constructor pattern
func NewMyProvider(config types.ProviderConfig) *MyProvider {
    // Create rate limit helper with provider-specific parser
    parser := ratelimit.NewMyProviderParser()

    return &MyProvider{
        config:          config,
        client:          createHTTPClient(),
        rateLimitHelper: common.NewRateLimitHelper(parser),
        modelCache:      common.NewModelCache(30 * time.Minute),
    }
}

// Consistent request pattern
func (p *MyProvider) makeRequest(ctx context.Context, req request) (*response, error) {
    // 1. Check rate limits
    estimatedTokens := p.estimateTokens(req)
    p.rateLimitHelper.CheckRateLimitAndWait(req.model, estimatedTokens)

    // 2. Make HTTP request
    httpReq := p.buildHTTPRequest(req)
    resp, err := p.client.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // 3. Update rate limits
    p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, req.model)

    // 4. Process response
    return p.processResponse(resp)
}

// Consistent model retrieval pattern
func (p *MyProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    return p.modelCache.GetModels(
        func() ([]types.Model, error) {
            return p.fetchModelsFromAPI(ctx)
        },
        func() []types.Model {
            return p.getStaticModels()
        },
    )
}

// File processing with language detection
func (p *MyProvider) processContextFiles(files []string) error {
    for _, file := range files {
        // Detect language for syntax highlighting or special processing
        language := common.DetectLanguage(file)

        // Read file content
        content, err := common.ReadFileContent(file)
        if err != nil {
            return fmt.Errorf("failed to read %s: %w", file, err)
        }

        // Process based on language
        err = p.processFileContent(file, content, language)
        if err != nil {
            return err
        }
    }
    return nil
}
```

#### Benefits of Using Shared Utilities

1. **Consistency**: All providers behave consistently for common operations
2. **Reduced Duplication**: Common code is implemented once and reused
3. **Easier Testing**: Shared test utilities ensure comprehensive coverage
4. **Faster Development**: New providers can be implemented more quickly
5. **Bug Fixes**: Fixes in shared utilities benefit all providers
6. **Rate Limiting**: Consistent rate limit behavior across providers
7. **Caching**: Efficient model caching with proper TTL management
8. **Testing**: Comprehensive test helpers reduce boilerplate code

### 10.5 Version Management

```go
// Semantic versioning: MAJOR.MINOR.PATCH

const (
    // Major version: Incompatible API changes
    // Minor version: Backwards-compatible functionality
    // Patch version: Backwards-compatible bug fixes

    Version = "1.2.3"

    MajorVersion = 1
    MinorVersion = 2
    PatchVersion = 3
)

// Version compatibility check
func CheckCompatibility(clientVersion string) bool {
    clientMajor := parseVersion(clientVersion).Major
    return clientMajor == MajorVersion
}
```

### 10.5 Documentation Standards

```go
// Package documentation
// Package service provides business logic for AI completions.
//
// The service layer sits between the HTTP handlers and provider implementations,
// providing a clean interface for common operations while handling cross-cutting
// concerns like caching, rate limiting, and metrics collection.
//
// Example usage:
//
//  service := service.NewCompletionService(providers, cache, limiter)
//  response, err := service.Generate(ctx, request)
//
package service

// Type documentation
// CompletionService orchestrates AI completion requests across multiple providers.
//
// It handles:
//   - Provider selection based on requirements
//   - Response caching
//   - Rate limiting
//   - Metrics collection
//   - Error handling and retries
type CompletionService struct {
    // providers maps provider types to configured instances
    providers map[types.ProviderType]types.Provider

    // cache stores recent responses for reuse
    cache *ResponseCache

    // limiter enforces rate limits per user/tenant
    limiter *RateLimiter
}

// Function documentation
// Generate creates an AI completion based on the provided request.
//
// It performs the following steps:
//  1. Validates the request
//  2. Checks cache for existing response
//  3. Verifies rate limits
//  4. Selects appropriate provider
//  5. Executes completion with timeout
//  6. Caches successful response
//
// Returns an error if validation fails, rate limit is exceeded, or all
// providers fail to complete the request.
func (s *CompletionService) Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
    // Implementation
}
```

---

## 11. Shared Utilities Best Practices (Phase 2)

Phase 2 of the AI Provider Kit introduces powerful shared utilities that significantly reduce code duplication and provide consistent patterns across all providers. This section covers best practices for using these utilities effectively.

### Streaming Utilities

#### Use the Standard Stream Processors

**Best Practice**: Always use the built-in stream processors for consistent behavior across providers.

```go
package service

import (
    "context"
    "fmt"
    "io"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type StreamingService struct {
    // Use standard HTTP client with connection pooling
    client *http.Client
}

// Good: Use built-in streaming utilities
func (s *StreamingService) StreamCompletion(ctx context.Context, provider types.Provider, options types.GenerateOptions) error {
    // Get streaming response from provider
    resp, err := s.makeStreamingRequest(ctx, provider, options)
    if err != nil {
        return fmt.Errorf("failed to make streaming request: %w", err)
    }
    defer resp.Body.Close()

    // Use appropriate stream parser based on provider type
    var stream types.ChatCompletionStream
    switch provider.GetType() {
    case types.ProviderTypeOpenAI, types.ProviderTypeCerebras, types.ProviderTypeOpenRouter:
        stream = common.CreateOpenAIStream(resp)
    case types.ProviderTypeAnthropic:
        stream = common.CreateAnthropicStream(resp)
    default:
        return fmt.Errorf("unsupported provider for streaming: %s", provider.GetType())
    }
    defer stream.Close()

    // Process stream with proper error handling
    for {
        chunk, err := stream.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return fmt.Errorf("stream error: %w", err)
        }

        // Handle chunk content
        if chunk.Content != "" {
            fmt.Print(chunk.Content)
        }

        // Handle tool calls
        for _, choice := range chunk.Choices {
            for _, toolCall := range choice.Delta.ToolCalls {
                if err := s.handleToolCall(toolCall); err != nil {
                    return fmt.Errorf("failed to handle tool call: %w", err)
                }
            }
        }

        // Check for completion
        if chunk.Done {
            if chunk.Usage.TotalTokens > 0 {
                log.Printf("Stream completed. Tokens used: %d", chunk.Usage.TotalTokens)
            }
            break
        }
    }

    return nil
}

// Bad: Manual stream parsing (error-prone and provider-specific)
func (s *StreamingService) ManualStreamParsing(resp *http.Response) error {
    // This is what we want to avoid - manual parsing logic
    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        if strings.HasPrefix(line, "data: ") {
            data := strings.TrimPrefix(line, "data: ")
            // Complex, error-prone parsing logic here...
            // Provider-specific format handling...
            // Error handling...
        }
    }
    return scanner.Err()
}
```

#### Create Custom Stream Parsers

**Best Practice**: For providers with unique streaming formats, implement the `StreamParser` interface.

```go
// Custom stream parser for a provider with unique format
type CustomStreamParser struct {
    // Configuration for parsing
    contentPath  string
    metadataPath string
}

func (p *CustomStreamParser) ParseLine(data string) (types.ChatCompletionChunk, bool, error) {
    var response map[string]interface{}
    if err := json.Unmarshal([]byte(data), &response); err != nil {
        return types.ChatCompletionChunk{}, false, fmt.Errorf("failed to parse stream data: %w", err)
    }

    chunk := types.ChatCompletionChunk{}

    // Extract content using custom path
    if content := p.extractNestedValue(response, p.contentPath); content != nil {
        if contentStr, ok := content.(string); ok {
            chunk.Content = contentStr
        }
    }

    // Extract metadata
    if metadata := p.extractNestedValue(response, p.metadataPath); metadata != nil {
        // Process provider-specific metadata
        chunk.Usage = p.extractUsage(metadata)
    }

    return chunk, chunk.Done, nil
}

// Use custom parser
stream := common.CreateCustomStream(resp, &CustomStreamParser{
    contentPath:  "response.text",
    metadataPath: "usage",
})
```

### Authentication Helpers

#### Centralize Authentication Logic

**Best Practice**: Use `AuthHelper` to handle all authentication concerns consistently.

```go
package provider

import (
    "context"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type CustomProvider struct {
    authHelper *common.AuthHelper
    client     *http.Client
    config     types.ProviderConfig
}

func NewCustomProvider(config types.ProviderConfig) (*CustomProvider, error) {
    client := &http.Client{Timeout: 30 * time.Second}

    // Create auth helper
    authHelper := common.NewAuthHelper("custom", config, client)

    // Setup authentication methods
    authHelper.SetupAPIKeys()

    provider := &CustomProvider{
        authHelper: authHelper,
        client:     client,
        config:     config,
    }

    return provider, nil
}

// Good: Use ExecuteWithAuth for automatic failover
func (p *CustomProvider) Generate(ctx context.Context, options types.GenerateOptions) (string, *types.Usage, error) {
    return p.authHelper.ExecuteWithAuth(ctx, options,
        // OAuth operation
        func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
            return p.makeOAuthRequest(ctx, cred, options)
        },
        // API key operation
        func(ctx context.Context, key string) (string, *types.Usage, error) {
            return p.makeAPIKeyRequest(ctx, key, options)
        },
    )
}

// Good: Use helper methods for HTTP requests
func (p *CustomProvider) makeAPIKeyRequest(ctx context.Context, apiKey string, options types.GenerateOptions) (string, *types.Usage, error) {
    req, err := http.NewRequestWithContext(ctx, "POST", p.getEndpoint(), nil)
    if err != nil {
        return "", nil, err
    }

    // Set auth headers using helper
    p.authHelper.SetAuthHeaders(req, apiKey, "api_key")

    // Set provider-specific headers
    p.authHelper.SetProviderSpecificHeaders(req)

    // Add content type and request body
    req.Header.Set("Content-Type", "application/json")
    // ... set request body ...

    resp, err := p.client.Do(req)
    if err != nil {
        return "", nil, err
    }
    defer resp.Body.Close()

    // Use helper for error handling
    if resp.StatusCode >= 400 {
        return "", nil, p.authHelper.HandleAuthError(fmt.Errorf("request failed"), resp.StatusCode)
    }

    return p.parseResponse(resp)
}
```

#### Implement OAuth with Refresh Support

**Best Practice**: Use the OAuth refresh helpers for seamless token management.

```go
func (p *CustomProvider) setupOAuth() error {
    // Create refresh function factory
    factory := common.NewRefreshFuncFactory("custom", p.client)

    // Use appropriate refresh function
    var refreshFunc oauthmanager.RefreshFunc
    switch p.config.Type {
    case types.ProviderTypeAnthropic:
        refreshFunc = factory.CreateAnthropicRefreshFunc()
    case types.ProviderTypeOpenAI:
        refreshFunc = factory.CreateOpenAIRefreshFunc()
    case types.ProviderTypeGemini:
        refreshFunc = factory.CreateGeminiRefreshFunc()
    case types.ProviderTypeQwen:
        refreshFunc = factory.CreateQwenRefreshFunc()
    default:
        // Use generic refresh for custom providers
        refreshFunc = factory.CreateGenericRefreshFunc("https://api.example.com/oauth/token")
    }

    // Setup OAuth manager
    p.authHelper.SetupOAuth(refreshFunc)

    return nil
}

// OAuth request implementation
func (p *CustomProvider) makeOAuthRequest(ctx context.Context, cred *types.OAuthCredentialSet, options types.GenerateOptions) (string, *types.Usage, error) {
    req, err := http.NewRequestWithContext(ctx, "POST", p.getEndpoint(), nil)
    if err != nil {
        return "", nil, err
    }

    // Use OAuth token
    p.authHelper.SetAuthHeaders(req, cred.AccessToken, "oauth")
    p.authHelper.SetProviderSpecificHeaders(req)

    // ... make request ...

    return p.parseResponse(resp)
}
```

#### Monitor Authentication Health

**Best Practice**: Regularly check authentication status and handle failures gracefully.

```go
type AuthMonitor struct {
    helpers map[string]*common.AuthHelper
}

func (m *AuthMonitor) CheckAllProviders(ctx context.Context) map[string]interface{} {
    status := make(map[string]interface{})

    for provider, helper := range m.helpers {
        providerStatus := helper.GetAuthStatus()

        // Check if OAuth tokens need refresh
        if providerStatus["method"] == "oauth" && helper.OAuthManager != nil {
            creds := helper.OAuthManager.GetCredentials()
            for _, cred := range creds {
                if time.Now().Add(10 * time.Minute).After(cred.ExpiresAt) {
                    // Token expires soon, attempt refresh
                    if err := helper.RefreshAllOAuthTokens(ctx); err != nil {
                        providerStatus["refresh_error"] = err.Error()
                    }
                }
            }
        }

        status[provider] = providerStatus
    }

    return status
}
```


### Configuration Helper

#### Standardize Configuration Handling

**Best Practice**: Use `ConfigHelper` for all provider configuration needs.

```go
package provider

import (
    "time"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type CustomProvider struct {
    configHelper *common.ConfigHelper
    baseConfig   CustomProviderConfig
}

type CustomProviderConfig struct {
    APIKey        string            `json:"api_key"`
    BaseURL       string            `json:"base_url"`
    Model         string            `json:"model"`
    Timeout       time.Duration     `json:"timeout"`
    MaxTokens     int               `json:"max_tokens"`
    CustomHeaders map[string]string `json:"custom_headers"`
}

func NewCustomProvider(config types.ProviderConfig) (*CustomProvider, error) {
    // Create config helper
    configHelper := common.NewConfigHelper("custom", types.ProviderTypeCustom)

    // Validate configuration
    validation := configHelper.ValidateProviderConfig(config)
    if !validation.Valid {
        return nil, fmt.Errorf("invalid configuration: %v", validation.Errors)
    }

    // Extract provider-specific configuration
    var baseConfig CustomProviderConfig
    if err := configHelper.ExtractProviderSpecificConfig(config, &baseConfig); err != nil {
        return nil, fmt.Errorf("failed to extract provider config: %w", err)
    }

    // Apply defaults and top-level overrides
    if err := configHelper.ApplyTopLevelOverrides(config, &baseConfig); err != nil {
        return nil, fmt.Errorf("failed to apply overrides: %w", err)
    }

    // Merge with provider defaults
    mergedConfig := configHelper.MergeWithDefaults(config)

    provider := &CustomProvider{
        configHelper: configHelper,
        baseConfig:   baseConfig,
    }

    return provider, nil
}

// Good: Use helper methods for configuration access
func (p *CustomProvider) getEndpoint() string {
    return p.configHelper.ExtractBaseURL(p.config) + "/chat/completions"
}

func (p *CustomProvider) getDefaultModel() string {
    return p.configHelper.ExtractDefaultModel(p.config)
}

func (p *CustomProvider) getTimeout() time.Duration {
    return p.configHelper.ExtractTimeout(p.config)
}

func (p *CustomProvider) getMaxTokens() int {
    return p.configHelper.ExtractMaxTokens(p.config)
}

// Get provider capabilities
func (p *CustomProvider) GetCapabilities() map[string]bool {
    toolCalling, streaming, responsesAPI := p.configHelper.GetProviderCapabilities()
    return map[string]bool{
        "tool_calling":   toolCalling,
        "streaming":      streaming,
        "responses_api":  responsesAPI,
    }
}
```

#### Configuration Validation and Sanitization

**Best Practice**: Implement comprehensive configuration validation and safe logging.

```go
func (p *CustomProvider) ValidateConfiguration() error {
    // Use helper validation
    validation := p.configHelper.ValidateProviderConfig(p.config)
    if !validation.Valid {
        return fmt.Errorf("configuration validation failed: %v", validation.Errors)
    }

    // Provider-specific validations
    if p.baseConfig.Timeout < 5*time.Second {
        return fmt.Errorf("timeout must be at least 5 seconds")
    }

    if p.baseConfig.MaxTokens > 100000 {
        return fmt.Errorf("max_tokens cannot exceed 100000")
    }

    return nil
}

// Safe configuration logging
func (p *CustomProvider) LogConfiguration() {
    // Sanitize config for logging (removes sensitive data)
    safeConfig := p.configHelper.SanitizeConfigForLogging(p.config)

    // Get human-readable summary
    summary := p.configHelper.ConfigSummary(p.config)

    log.Printf("CustomProvider configured: %+v", summary)
    log.Printf("Safe config (for debugging): %+v", safeConfig)
}

// Export configuration for monitoring
func (p *CustomProvider) GetConfigurationMetrics() map[string]interface{} {
    summary := p.configHelper.ConfigSummary(p.config)

    // Add custom metrics
    summary["custom_headers_count"] = len(p.baseConfig.CustomHeaders)
    summary["config_hash"] = p.calculateConfigHash()

    return summary
}
```

### Integration Example: Complete Provider Implementation

Here's a complete example showing how to use all Phase 2 utilities together:

```go
package provider

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type ExampleProvider struct {
    authHelper   *common.AuthHelper
    configHelper *common.ConfigHelper
    customizer   common.ProviderCustomizer
    client       *http.Client
    config       types.ProviderConfig
}

func NewExampleProvider(config types.ProviderConfig) (*ExampleProvider, error) {
    // Initialize HTTP client with connection pooling
    client := &http.Client{
        Timeout: 60 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:        100,
            MaxIdleConnsPerHost: 10,
            IdleConnTimeout:     90 * time.Second,
        },
    }

    // Setup configuration helper
    configHelper := common.NewConfigHelper("example", types.ProviderTypeCustom)

    // Validate configuration
    if validation := configHelper.ValidateProviderConfig(config); !validation.Valid {
        return nil, fmt.Errorf("invalid config: %v", validation.Errors)
    }

    // Setup authentication helper
    authHelper := common.NewAuthHelper("example", config, client)
    authHelper.SetupAPIKeys()

    // Setup OAuth if configured
    if len(config.OAuthCredentials) > 0 {
        factory := common.NewRefreshFuncFactory("example", client)
        refreshFunc := factory.CreateGenericRefreshFunc("https://api.example.com/oauth/token")
        authHelper.SetupOAuth(refreshFunc)
    }

    // Create provider customizer
    customizer := &ExampleCustomizer{}

    provider := &ExampleProvider{
        authHelper:   authHelper,
        configHelper: configHelper,
        customizer:   customizer,
        client:       client,
        config:       config,
    }

    return provider, nil
}

// Generate using all Phase 2 utilities
func (p *ExampleProvider) Generate(ctx context.Context, options types.GenerateOptions) (string, *types.Usage, error) {
    // Direct prompt handling - no prompt builder abstraction
    // Applications construct prompts directly
    finalOptions := options

    // Execute with authentication failover
    return p.authHelper.ExecuteWithAuth(ctx, finalOptions,
        p.executeWithOAuth,
        p.executeWithAPIKey,
    )
}

func (p *ExampleProvider) executeWithOAuth(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
    return p.makeRequest(ctx, "oauth", cred.AccessToken)
}

func (p *ExampleProvider) executeWithAPIKey(ctx context.Context, apiKey string) (string, *types.Usage, error) {
    return p.makeRequest(ctx, "api_key", apiKey)
}

func (p *ExampleProvider) makeRequest(ctx context.Context, authType, authToken string) (string, *types.Usage, error) {
    endpoint := p.configHelper.ExtractBaseURL(p.config) + "/v1/chat/completions"

    // Create request
    req, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
    if err != nil {
        return "", nil, err
    }

    // Set headers using helpers
    p.authHelper.SetAuthHeaders(req, authToken, authType)
    p.authHelper.SetProviderSpecificHeaders(req)
    req.Header.Set("Content-Type", "application/json")

    // Make request and parse response
    resp, err := p.client.Do(req)
    if err != nil {
        return "", nil, err
    }
    defer resp.Body.Close()

    // Handle auth errors with helper
    if resp.StatusCode >= 400 {
        return "", nil, p.authHelper.HandleAuthError(fmt.Errorf("request failed"), resp.StatusCode)
    }

    return p.parseResponse(resp)
}

// Streaming with Phase 2 utilities
func (p *ExampleProvider) Stream(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    // Similar to Generate but returns stream
    resp, err := p.makeStreamingRequest(ctx, options)
    if err != nil {
        return nil, err
    }

    // Use standard stream parser
    return common.CreateOpenAIStream(resp), nil
}

// Health check using helpers
func (p *ExampleProvider) HealthCheck(ctx context.Context) error {
    status := p.authHelper.GetAuthStatus()
    if !status["authenticated"].(bool) {
        return fmt.Errorf("not authenticated")
    }

    // Additional health checks
    endpoint := p.configHelper.ExtractBaseURL(p.config) + "/health"
    req, _ := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
    p.authHelper.SetProviderSpecificHeaders(req)

    resp, err := p.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("health check failed: %d", resp.StatusCode)
    }

    return nil
}
```

---

## Summary

This comprehensive guide covers best practices for building production-grade applications with the AI Provider Kit SDK. Key takeaways:

1. **Architecture**: Use dependency injection, service layers, and error boundaries for maintainable code
2. **Performance**: Enable connection pooling, implement caching, and optimize goroutine usage
3. **Security**: Encrypt credentials, use secure storage, implement audit logging, and follow compliance requirements
4. **Production**: Use secret management, containerize applications, implement health checks, and set up comprehensive monitoring
5. **Multi-Tenancy**: Isolate tenants, segregate credentials, manage per-tenant rate limits, and track costs accurately
6. **Scalability**: Implement horizontal scaling, load balancing, queue-based processing, and async operations
7. **Testing**: Use mocks for unit tests, run integration tests, perform load testing, and practice chaos engineering
8. **Cost**: Compare provider costs, optimize token usage, leverage caching, and use multi-key strategies
9. **Debugging**: Enable debug logging, trace requests, profile performance, and understand common issues
10. **Organization**: Follow clear package structure, design clean interfaces, manage dependencies, and document thoroughly
11. **Phase 2 Shared Utilities**: Leverage streaming processors, authentication helpers, and configuration helpers to reduce code duplication and ensure consistency across providers

For specific implementations and examples, refer to the SDK's example applications in `/examples` and comprehensive documentation in `/docs`.

---

## 12. Interface Segregation Best Practices (Phase 3)

Phase 3 introduced interface segregation, breaking down the monolithic Provider interface into smaller, focused interfaces that follow the Interface Segregation Principle. This section covers best practices for using these interfaces effectively.

### When to Use Specific Interfaces

**Best Practice**: Choose the smallest interface that meets your needs to reduce coupling and improve testability.

#### Use CoreProvider for Basic Information

```go
package service

import "github.com/cecil-the-coder/ai-provider-kit/pkg/types"

// Good: Only need basic provider information
type ProviderRegistry struct {
    providers []types.CoreProvider
}

func (r *ProviderRegistry) RegisterProvider(provider types.CoreProvider) {
    r.providers = append(r.providers, provider)
}

func (r *ProviderRegistry) ListProviders() []string {
    var names []string
    for _, provider := range r.providers {
        names = append(names, provider.Name())
    }
    return names
}

func (r *ProviderRegistry) GetProviderInfo(name string) (string, string, string) {
    for _, provider := range r.providers {
        if provider.Name() == name {
            return provider.Name(), string(provider.Type()), provider.Description()
        }
    }
    return "", "", ""
}
```

#### Use ModelProvider for Model Discovery

```go
// Good: Service that only needs to discover and compare models
type ModelComparisonService struct {
    providers []types.ModelProvider
}

func (s *ModelComparisonService) FindBestModel(requirements ModelRequirements) (*types.Model, error) {
    var candidates []*types.Model

    for _, provider := range s.providers {
        models, err := provider.GetModels(context.Background())
        if err != nil {
            log.Printf("Failed to get models from %s: %v", provider.GetDefaultModel(), err)
            continue
        }

        for _, model := range models {
            if s.meetsRequirements(model, requirements) {
                candidates = append(candidates, &model)
            }
        }
    }

    if len(candidates) == 0 {
        return nil, fmt.Errorf("no models meet requirements")
    }

    // Select best model based on criteria
    return s.selectBestModel(candidates), nil
}
```

#### Use AuthenticatedProvider for Authentication Management

```go
// Good: Service that only manages authentication
type AuthenticationService struct {
    providers map[string]types.AuthenticatedProvider
}

func (s *AuthenticationService) AuthenticateProvider(providerName string, authConfig types.AuthConfig) error {
    provider, exists := s.providers[providerName]
    if !exists {
        return fmt.Errorf("provider not found: %s", providerName)
    }

    return provider.Authenticate(context.Background(), authConfig)
}

func (s *AuthenticationService) CheckAllAuthenticationStatus() map[string]bool {
    status := make(map[string]bool)
    for name, provider := range s.providers {
        status[name] = provider.IsAuthenticated()
    }
    return status
}

func (s *AuthenticationService) LogoutProvider(providerName string) error {
    provider, exists := s.providers[providerName]
    if !exists {
        return fmt.Errorf("provider not found: %s", providerName)
    }

    return provider.Logout(context.Background())
}
```

#### Use HealthCheckProvider for Monitoring

```go
// Good: Service that only needs health monitoring
type HealthMonitoringService struct {
    providers []types.HealthCheckProvider
    interval  time.Duration
}

func (s *HealthMonitoringService) StartMonitoring(ctx context.Context) {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.checkAllProviders(ctx)
        }
    }
}

func (s *HealthMonitoringService) checkAllProviders(ctx context.Context) {
    for _, provider := range s.providers {
        go func(p types.HealthCheckProvider) {
            start := time.Now()
            err := p.HealthCheck(ctx)
            latency := time.Since(start)

            metrics := p.GetMetrics()

            if err != nil {
                log.Printf("Provider %s unhealthy: %v (latency: %v)",
                    p.GetDefaultModel(), err, latency)
                s.sendAlert(p.GetDefaultModel(), err, metrics)
            } else {
                log.Printf("Provider %s healthy (latency: %v, requests: %d)",
                    p.GetDefaultModel(), latency, metrics.RequestCount)
            }
        }(provider)
    }
}
```

### Interface Composition Patterns

**Best Practice**: Compose interfaces to create focused service abstractions.

#### Focused Service Interfaces

```go
// Define service-specific interfaces
type ChatService interface {
    Generate(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    Stream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
}

type ModelDiscoveryService interface {
    ListModels(ctx context.Context) ([]types.Model, error)
    FindModel(ctx context.Context, criteria ModelCriteria) (*types.Model, error)
}

type HealthService interface {
    CheckHealth(ctx context.Context) error
    GetMetrics() (types.ProviderMetrics, error)
}

// Implement services using segregated interfaces
type DefaultChatService struct {
    provider types.ChatProvider
    config   ChatServiceConfig
}

func NewChatService(provider types.ChatProvider, config ChatServiceConfig) ChatService {
    return &DefaultChatService{
        provider: provider,
        config:   config,
    }
}

func (s *DefaultChatService) Generate(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    options := types.GenerateOptions{
        Messages:   req.Messages,
        Model:      req.Model,
        MaxTokens:  req.MaxTokens,
        Temperature: req.Temperature,
        Stream:     false,
    }

    stream, err := s.provider.GenerateChatCompletion(ctx, options)
    if err != nil {
        return nil, fmt.Errorf("failed to generate completion: %w", err)
    }
    defer stream.Close()

    chunk, err := stream.Next()
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }

    return &ChatResponse{
        Content: chunk.Content,
        Usage:   chunk.Usage,
        Model:   req.Model,
    }, nil
}
```

#### Capability-Based Interfaces

```go
// Create capability-specific interfaces
type ToolCapable interface {
    types.ChatProvider
    types.ToolCallingProvider
}

type StreamingCapable interface {
    types.ChatProvider
    types.CapabilityProvider
}

type FullFeatured interface {
    types.ChatProvider
    types.ToolCallingProvider
    types.CapabilityProvider
}

// Service that adapts based on capabilities
type AdaptiveChatService struct {
    provider interface{}
    config   AdaptiveConfig
}

func NewAdaptiveChatService(provider interface{}, config AdaptiveConfig) *AdaptiveChatService {
    return &AdaptiveChatService{
        provider: provider,
        config:   config,
    }
}

func (s *AdaptiveChatService) Generate(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // Use type assertions to check capabilities
    switch p := s.provider.(type) {
    case FullFeatured:
        return s.generateWithFullFeatures(ctx, p, req)
    case ToolCapable:
        return s.generateWithTools(ctx, p, req)
    case StreamingCapable:
        return s.generateWithStreaming(ctx, p, req)
    case types.ChatProvider:
        return s.generateBasic(ctx, p, req)
    default:
        return nil, fmt.Errorf("provider does not support chat generation")
    }
}

func (s *AdaptiveChatService) generateWithFullFeatures(ctx context.Context, provider FullFeatured, req ChatRequest) (*ChatResponse, error) {
    // Use all available features
    options := types.GenerateOptions{
        Messages:     req.Messages,
        Model:        req.Model,
        MaxTokens:    req.MaxTokens,
        Temperature:  req.Temperature,
        Stream:       false,
        Tools:        req.Tools,
        ToolChoice:   req.ToolChoice,
    }

    // Handle provider-specific features
    if provider.SupportsResponsesAPI() {
        options.Metadata = map[string]interface{}{
            "use_responses_api": true,
        }
    }

    return s.executeGeneration(ctx, provider, options)
}
```

### Testing with Segregated Interfaces

**Best Practice**: Create focused mocks that implement only the interfaces you need to test.

#### Focused Mock Implementations

```go
package testutil

import "github.com/cecil-the-coder/ai-provider-kit/pkg/types"

// Mock for testing model discovery
type MockModelProvider struct {
    models []types.Model
    err    error
}

func (m *MockModelProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    return m.models, m.err
}

func (m *MockModelProvider) GetDefaultModel() string {
    if len(m.models) > 0 {
        return m.models[0].ID
    }
    return "default-model"
}

// Mock for testing health monitoring
type MockHealthProvider struct {
    healthy bool
    metrics types.ProviderMetrics
    err     error
}

func (m *MockHealthProvider) HealthCheck(ctx context.Context) error {
    return m.err
}

func (m *MockHealthProvider) GetMetrics() types.ProviderMetrics {
    return m.metrics
}

// Mock for testing authentication
type MockAuthProvider struct {
    authenticated bool
    err           error
}

func (m *MockAuthProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
    return m.err
}

func (m *MockAuthProvider) IsAuthenticated() bool {
    return m.authenticated
}

func (m *MockAuthProvider) Logout(ctx context.Context) error {
    m.authenticated = false
    return nil
}

// Mock for testing chat generation
type MockChatProvider struct {
    response string
    usage    types.Usage
    err      error
}

func (m *MockChatProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    if m.err != nil {
        return nil, m.err
    }

    stream := &MockStream{
        chunks: []types.ChatCompletionChunk{
            {
                Content: m.response,
                Usage:   m.usage,
                Done:    true,
            },
        },
    }

    return stream, nil
}
```

#### Interface-Specific Test Helpers

```go
package testutil

// Test helpers for specific interfaces
func TestModelProvider(t *testing.T, provider types.ModelProvider) {
    ctx := context.Background()

    models, err := provider.GetModels(ctx)
    require.NoError(t, err, "GetModels should not return error")
    assert.NotEmpty(t, models, "Should return at least one model")

    defaultModel := provider.GetDefaultModel()
    assert.NotEmpty(t, defaultModel, "Should have a default model")

    // Verify default model exists in model list
    found := false
    for _, model := range models {
        if model.ID == defaultModel {
            found = true
            break
        }
    }
    assert.True(t, found, "Default model should exist in model list")
}

func TestHealthProvider(t *testing.T, provider types.HealthCheckProvider) {
    ctx := context.Background()

    // Test health check
    err := provider.HealthCheck(ctx)
    assert.NoError(t, err, "HealthCheck should not return error")

    // Test metrics
    metrics := provider.GetMetrics()
    assert.NotNil(t, metrics, "GetMetrics should return metrics")
    assert.GreaterOrEqual(t, metrics.RequestCount, int64(0), "RequestCount should be non-negative")
}

func TestAuthProvider(t *testing.T, provider types.AuthenticatedProvider, authConfig types.AuthConfig) {
    ctx := context.Background()

    // Test authentication
    err := provider.Authenticate(ctx, authConfig)
    require.NoError(t, err, "Authenticate should not return error")

    // Test authentication status
    assert.True(t, provider.IsAuthenticated(), "Should be authenticated after successful auth")

    // Test logout
    err = provider.Logout(ctx)
    assert.NoError(t, err, "Logout should not return error")
    assert.False(t, provider.IsAuthenticated(), "Should not be authenticated after logout")
}
```

#### Integration Testing with Interface Composition

```go
package test

// Integration test that combines multiple interfaces
func TestProviderCapabilities(t *testing.T, provider types.Provider) {
    t.Run("ModelDiscovery", func(t *testing.T) {
        testutil.TestModelProvider(t, provider)
    })

    t.Run("HealthCheck", func(t *testing.T) {
        testutil.TestHealthProvider(t, provider)
    })

    t.Run("Authentication", func(t *testing.T) {
        if authConfig := getTestAuthConfig(provider.Type()); authConfig != nil {
            testutil.TestAuthProvider(t, provider, *authConfig)
        }
    })

    t.Run("ChatGeneration", func(t *testing.T) {
        testChatGeneration(t, provider)
    })

    // Test optional capabilities
    if provider.SupportsToolCalling() {
        t.Run("ToolCalling", func(t *testing.T) {
            testToolCalling(t, provider)
        })
    }

    if provider.SupportsStreaming() {
        t.Run("Streaming", func(t *testing.T) {
            testStreaming(t, provider)
        })
    }
}

func testChatGeneration(t *testing.T, provider types.ChatProvider) {
    ctx := context.Background()
    options := types.GenerateOptions{
        Messages: []types.ChatMessage{
            {Role: "user", Content: "Hello, test!"},
        },
        MaxTokens: 50,
        Model:     provider.GetDefaultModel(),
    }

    stream, err := provider.GenerateChatCompletion(ctx, options)
    require.NoError(t, err, "GenerateChatCompletion should not return error")
    defer stream.Close()

    chunk, err := stream.Next()
    require.NoError(t, err, "Next should not return error")
    assert.NotEmpty(t, chunk.Content, "Response should not be empty")
}
```

### Migration Strategies

**Best Practice**: Gradually migrate from the full Provider interface to segregated interfaces.

#### Step 1: Identify Interface Dependencies

```go
// Before: Using full Provider interface
type OldService struct {
    provider types.Provider
}

// Step 1: Analyze what methods are actually used
func (s *OldService) ProcessRequest(ctx context.Context, req Request) error {
    // Used methods:
    // - provider.GenerateChatCompletion()
    // - provider.GetMetrics()
    // - provider.GetDefaultModel()

    // Not used:
    // - provider.GetModels()
    // - provider.Authenticate()
    // - provider.Configure()
    // - etc.
}
```

#### Step 2: Create Focused Interfaces

```go
// Step 2: Create interface with only used methods
type ChatWithMetricsProvider interface {
    types.ChatProvider
    GetDefaultModel() string
    GetMetrics() types.ProviderMetrics
}

// Step 3: Update service to use focused interface
type NewService struct {
    provider ChatWithMetricsProvider
}

func NewService(provider ChatWithMetricsProvider) *NewService {
    return &NewService{provider: provider}
}

func (s *NewService) ProcessRequest(ctx context.Context, req Request) error {
    // Implementation unchanged, but now dependencies are explicit
    stream, err := s.provider.GenerateChatCompletion(ctx, options)
    // ...
}
```

#### Step 3: Gradual Interface Refinement

```go
// Step 4: Further refine interfaces as needed
type MetricsProvider interface {
    GetMetrics() types.ProviderMetrics
}

type ModelProvider interface {
    GetDefaultModel() string
}

// Compose interfaces as needed
type ChatServiceDependencies interface {
    types.ChatProvider
    ModelProvider
    MetricsProvider
}

// This allows services to depend only on what they need
type FocusedChatService struct {
    provider ChatServiceDependencies
}
```

---

## 13. Standardized Core API Best Practices (Phase 3)

Phase 3 also introduced a standardized core API with provider-specific extensions. This section covers best practices for using the standardized API while maintaining provider-specific capabilities.

### Request Builder Patterns

**Best Practice**: Use the CoreRequestBuilder for type-safe, validated request construction.

#### Basic Request Building

```go
package service

import (
    "context"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type ChatService struct {
    coreProvider types.CoreChatProvider
}

func (s *ChatService) GenerateResponse(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // Build request using CoreRequestBuilder
    request, err := types.NewCoreRequestBuilder().
        WithMessages(req.Messages).
        WithModel(req.Model).
        WithMaxTokens(req.MaxTokens).
        WithTemperature(req.Temperature).
        Build()
    if err != nil {
        return nil, fmt.Errorf("failed to build request: %w", err)
    }

    // Generate standardized response
    response, err := s.coreProvider.GenerateStandardCompletion(ctx, *request)
    if err != nil {
        return nil, fmt.Errorf("failed to generate completion: %w", err)
    }

    return &ChatResponse{
        Content: response.Choices[0].Message.Content,
        Usage:   response.Usage,
        Model:   response.Model,
        ID:      response.ID,
    }, nil
}
```

#### Advanced Request Building with Validation

```go
// Good: Use builder for complex requests with validation
func (s *ChatService) GenerateAdvancedResponse(ctx context.Context, req AdvancedChatRequest) (*ChatResponse, error) {
    builder := types.NewCoreRequestBuilder().
        WithMessages(req.Messages).
        WithModel(req.Model).
        WithMaxTokens(req.MaxTokens).
        WithTemperature(req.Temperature)

    // Add tools if provided
    if len(req.Tools) > 0 {
        builder = builder.WithTools(req.Tools).
            WithToolChoice(req.ToolChoice)
    }

    // Add provider-specific metadata
    if req.ProviderSpecific != nil {
        for key, value := range req.ProviderSpecific {
            builder = builder.WithMetadata(key, value)
        }
    }

    // Validate request before building
    request, err := builder.Build()
    if err != nil {
        return nil, fmt.Errorf("invalid request: %w", err)
    }

    // Additional validation
    if err := s.coreProvider.ValidateStandardRequest(*request); err != nil {
        return nil, fmt.Errorf("provider validation failed: %w", err)
    }

    response, err := s.coreProvider.GenerateStandardCompletion(ctx, *request)
    if err != nil {
        return nil, fmt.Errorf("generation failed: %w", err)
    }

    return s.convertToChatResponse(response), nil
}
```

#### Streaming with Standardized API

```go
func (s *ChatService) StreamResponse(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
    request, err := types.NewCoreRequestBuilder().
        WithMessages(req.Messages).
        WithModel(req.Model).
        WithMaxTokens(req.MaxTokens).
        WithStreaming(true).
        Build()
    if err != nil {
        return nil, fmt.Errorf("failed to build request: %w", err)
    }

    stream, err := s.coreProvider.GenerateStandardStream(ctx, *request)
    if err != nil {
        return nil, fmt.Errorf("failed to create stream: %w", err)
    }

    // Convert standardized stream to application format
    chunks := make(chan StreamChunk, 10)
    go func() {
        defer close(chunks)
        defer stream.Close()

        for {
            chunk, err := stream.Next()
            if err == io.EOF {
                break
            }
            if err != nil {
                log.Printf("Stream error: %v", err)
                break
            }

            // Convert standardized chunk
            appChunk := StreamChunk{
                Content:   chunk.Choices[0].Delta.Content,
                Done:      chunk.Choices[0].FinishReason != "",
                Timestamp: time.Now(),
            }

            select {
            case chunks <- appChunk:
            case <-ctx.Done():
                return
            }
        }
    }()

    return chunks, nil
}
```

### Extension Development

**Best Practice**: Create provider-specific extensions to maintain unique capabilities while using the standardized API.

#### Creating Custom Extensions

```go
package extension

import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Example: Custom provider with unique features
type CustomProviderExtension struct {
    name        string
    version     string
    description string
    features    map[string]interface{}
}

func NewCustomProviderExtension() *CustomProviderExtension {
    return &CustomProviderExtension{
        name:        "custom-provider",
        version:     "1.0.0",
        description: "Custom provider with unique features",
        features: map[string]interface{}{
            "supports_custom_format": true,
            "max_context":           32000,
            "special_tokens":        []string{"<begin>", "<end>"},
        },
    }
}

func (e *CustomProviderExtension) Name() string { return e.name }
func (e *CustomProviderExtension) Version() string { return e.version }
func (e *CustomProviderExtension) Description() string { return e.description }

func (e *CustomProviderExtension) StandardToProvider(request types.StandardRequest) (interface{}, error) {
    // Convert standardized request to provider-specific format
    providerReq := CustomRequest{
        Messages:     e.convertMessages(request.Messages),
        Model:        request.Model,
        MaxTokens:    request.MaxTokens,
        Temperature:  request.Temperature,
        Stream:       request.Stream,
    }

    // Handle provider-specific features from metadata
    if customFormat, ok := request.Metadata["custom_format"].(bool); ok && customFormat {
        providerReq.Format = "custom"
    }

    if customTokens, ok := request.Metadata["custom_tokens"].([]string); ok {
        providerReq.SpecialTokens = customTokens
    }

    return providerReq, nil
}

func (e *CustomProviderExtension) ProviderToStandard(response interface{}) (*types.StandardResponse, error) {
    // Convert provider response to standardized format
    customResp := response.(CustomResponse)

    return &types.StandardResponse{
        ID:      customResp.ID,
        Object:  "chat.completion",
        Created: customResp.Created,
        Model:   customResp.Model,
        Choices: []types.StandardChoice{
            {
                Index: 0,
                Message: types.ChatMessage{
                    Role:    "assistant",
                    Content: customResp.Content,
                },
                FinishReason: customResp.Reason,
            },
        },
        Usage: types.Usage{
            PromptTokens:     customResp.PromptTokens,
            CompletionTokens: customResp.CompletionTokens,
            TotalTokens:      customResp.PromptTokens + customResp.CompletionTokens,
        },
    }, nil
}

func (e *CustomProviderExtension) ProviderToStandardChunk(chunk interface{}) (*types.StandardStreamChunk, error) {
    // Convert streaming chunk to standardized format
    customChunk := chunk.(CustomStreamChunk)

    return &types.StandardStreamChunk{
        ID: customChunk.ID,
        Choices: []types.StandardChoice{
            {
                Delta: types.ChatMessage{
                    Role:    "assistant",
                    Content: customChunk.Content,
                },
                FinishReason: customChunk.Reason,
            },
        },
    }, nil
}

func (e *CustomProviderExtension) ValidateOptions(options map[string]interface{}) error {
    // Validate provider-specific options
    if format, ok := options["format"].(string); ok {
        if format != "standard" && format != "custom" {
            return fmt.Errorf("invalid format: %s", format)
        }
    }

    return nil
}

func (e *CustomProviderExtension) GetCapabilities() []string {
    return []string{
        "chat",
        "streaming",
        "custom_format",
        "special_tokens",
        "large_context",
    }
}
```

#### Registering Extensions

```go
package provider

import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/extension"
)

func InitializeExtensions() {
    registry := types.NewExtensionRegistry()

    // Register built-in extensions
    registry.RegisterExtension(types.ProviderTypeOpenAI, extension.NewOpenAIExtension())
    registry.RegisterExtension(types.ProviderTypeAnthropic, extension.NewAnthropicExtension())
    registry.RegisterExtension(types.ProviderTypeCustom, extension.NewCustomProviderExtension())

    // Make registry globally available
    SetGlobalExtensionRegistry(registry)
}

// Factory function that creates core providers
func CreateCoreProvider(providerType types.ProviderType, config types.ProviderConfig) (types.CoreChatProvider, error) {
    // Create legacy provider
    legacyProvider, err := factory.CreateProvider(providerType, config)
    if err != nil {
        return nil, fmt.Errorf("failed to create legacy provider: %w", err)
    }

    // Get extension for provider type
    extension, err := GetGlobalExtensionRegistry().GetExtension(providerType)
    if err != nil {
        return nil, fmt.Errorf("no extension found for provider %s: %w", providerType, err)
    }

    // Create core provider adapter
    return types.NewCoreProviderAdapter(legacyProvider, extension), nil
}
```

### Migration Strategies

**Best Practice**: Gradually migrate from provider-specific APIs to the standardized core API.

#### Step 1: Adapter Pattern for Existing Code

```go
// Before: Direct provider usage
func (s *OldService) ProcessRequest(ctx context.Context, req Request) error {
    stream, err := s.provider.GenerateChatCompletion(ctx, req.Options)
    // ... process response
}

// Step 1: Wrap with adapter for gradual migration
type MigrationService struct {
    legacyProvider types.Provider
    coreProvider   types.CoreChatProvider
    useStandardAPI bool
}

func NewMigrationService(provider types.Provider) *MigrationService {
    // Create core provider adapter
    extension, _ := GetExtension(provider.Type())
    coreProvider := types.NewCoreProviderAdapter(provider, extension)

    return &MigrationService{
        legacyProvider: provider,
        coreProvider:   coreProvider,
        useStandardAPI: false, // Start with legacy API
    }
}

func (s *MigrationService) ProcessRequest(ctx context.Context, req Request) error {
    if s.useStandardAPI {
        return s.processWithStandardAPI(ctx, req)
    } else {
        return s.processWithLegacyAPI(ctx, req)
    }
}

func (s *MigrationService) processWithLegacyAPI(ctx context.Context, req Request) error {
    stream, err := s.legacyProvider.GenerateChatCompletion(ctx, req.Options)
    // ... legacy processing
}

func (s *MigrationService) processWithStandardAPI(ctx context.Context, req Request) error {
    request, err := types.NewCoreRequestBuilder().
        WithMessages(req.Options.Messages).
        WithModel(req.Options.Model).
        Build()
    if err != nil {
        return err
    }

    response, err := s.coreProvider.GenerateStandardCompletion(ctx, *request)
    // ... standardized processing
}

// Feature flag to switch between APIs
func (s *MigrationService) EnableStandardAPI(enable bool) {
    s.useStandardAPI = enable
    log.Printf("Standard API %s", map[bool]string{true: "enabled", false: "disabled"}[enable])
}
```

#### Step 2: Gradual Feature Migration

```go
// Step 2: Migrate specific features first
type HybridService struct {
    legacyProvider types.Provider
    coreProvider   types.CoreChatProvider
}

func (s *HybridService) GenerateChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // Use standardized API for basic chat
    request, _ := types.NewCoreRequestBuilder().
        WithMessages(req.Messages).
        WithModel(req.Model).
        Build()

    response, err := s.coreProvider.GenerateStandardCompletion(ctx, *request)
    if err != nil {
        return nil, err
    }

    return s.convertResponse(response), nil
}

func (s *HybridService) GenerateWithTools(ctx context.Context, req ToolChatRequest) (*ToolChatResponse, error) {
    // Keep using legacy API for complex tool calling until standardized version is ready
    stream, err := s.legacyProvider.GenerateChatCompletion(ctx, req.Options)
    if err != nil {
        return nil, err
    }
    defer stream.Close()

    // ... legacy tool calling processing
}

func (s *HybridService) StreamChat(ctx context.Context, req StreamRequest) (<-chan StreamChunk, error) {
    // Use standardized API for streaming
    request, _ := types.NewCoreRequestBuilder().
        WithMessages(req.Messages).
        WithModel(req.Model).
        WithStreaming(true).
        Build()

    stream, err := s.coreProvider.GenerateStandardStream(ctx, *request)
    if err != nil {
        return nil, err
    }

    return s.convertStream(stream), nil
}
```

#### Step 3: Complete Migration

```go
// Step 3: Full migration to standardized API
type ModernService struct {
    coreProvider types.CoreChatProvider
}

func NewModernService(providerType types.ProviderType, config types.ProviderConfig) (*ModernService, error) {
    coreProvider, err := CreateCoreProvider(providerType, config)
    if err != nil {
        return nil, err
    }

    return &ModernService{
        coreProvider: coreProvider,
    }, nil
}

func (s *ModernService) GenerateChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    request, err := types.NewCoreRequestBuilder().
        WithMessages(req.Messages).
        WithModel(req.Model).
        WithMaxTokens(req.MaxTokens).
        WithTemperature(req.Temperature).
        WithTools(req.Tools).
        WithToolChoice(req.ToolChoice).
        Build()
    if err != nil {
        return nil, fmt.Errorf("invalid request: %w", err)
    }

    response, err := s.coreProvider.GenerateStandardCompletion(ctx, *request)
    if err != nil {
        return nil, fmt.Errorf("generation failed: %w", err)
    }

    return &ChatResponse{
        Content: response.Choices[0].Message.Content,
        Usage:   response.Usage,
        Model:   response.Model,
        ID:      response.ID,
    }, nil
}

func (s *ModernService) GetProviderCapabilities() []string {
    return s.coreProvider.GetStandardCapabilities()
}
```

### Best Practices Summary

1. **Interface Segregation**:
   - Choose the smallest interface that meets your needs
   - Compose interfaces to create focused abstractions
   - Create focused mocks for testing
   - Migrate gradually from full Provider interface

2. **Standardized Core API**:
   - Use CoreRequestBuilder for type-safe request construction
   - Create provider-specific extensions for unique capabilities
   - Implement proper validation at multiple levels
   - Migrate gradually using adapter patterns

3. **Testing**:
   - Test interfaces in isolation
   - Create comprehensive integration tests
   - Validate extension behavior
   - Test migration paths thoroughly

4. **Migration**:
   - Start with feature flags for gradual rollout
   - Maintain backward compatibility during transition
   - Monitor performance and behavior changes
   - Document migration paths for users

By following these practices, you can leverage the benefits of interface segregation and the standardized core API while maintaining provider-specific capabilities and ensuring smooth migration paths.
