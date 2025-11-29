# AI Provider Kit, VelocityCode & Cortex Rewrite Plan

## Architectural Overview

This document outlines the parallel implementation plan for:
1. Adding a backend API and virtual providers to **ai-provider-kit**
2. Refactoring **VelocityCode** to use it as a foundation with code-generation extensions
3. Creating **Cortex** as an LLM proxy with OpenAI/Anthropic-compatible APIs

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         AI-PROVIDER-KIT (Library)                          │
├─────────────────────────────────────────────────────────────────────────────┤
│  pkg/                                                                       │
│  ├── types/              # Core interfaces (existing)                       │
│  ├── providers/          # Provider implementations (existing)              │
│  │   ├── openai/                                                           │
│  │   ├── anthropic/                                                        │
│  │   ├── gemini/                                                           │
│  │   └── virtual/        # NEW: Virtual/composite providers                │
│  │       ├── racing/           # Racing provider                           │
│  │       ├── fallback/         # Failover provider                         │
│  │       └── loadbalance/      # Load balancing provider                   │
│  ├── factory/            # Provider factory (existing)                      │
│  ├── auth/               # Authentication (existing)                        │
│  ├── oauthmanager/       # OAuth management (existing)                      │
│  ├── keymanager/         # API key management (existing)                    │
│  ├── http/               # HTTP client utilities (existing)                 │
│  │                                                                         │
│  ├── backend/            # NEW: Backend server infrastructure              │
│  │   ├── server.go             # HTTP server setup                         │
│  │   ├── router.go             # Route management                          │
│  │   ├── middleware/           # Reusable middleware                       │
│  │   ├── handlers/             # Core API handlers                         │
│  │   └── extensions/           # Extension framework                       │
│  │                                                                         │
│  └── backendtypes/       # NEW: Backend-specific types                      │
│      ├── requests.go           # API request types                         │
│      ├── responses.go          # API response types                        │
│      └── config.go             # Backend configuration                     │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
                    ▼               ▼               ▼
┌───────────────────────┐ ┌───────────────────────┐ ┌───────────────────────┐
│     VELOCITYCODE      │ │        CORTEX         │ │     FUTURE APPS       │
│    (Code Gen App)     │ │     (LLM Proxy)       │ │                       │
├───────────────────────┤ ├───────────────────────┤ ├───────────────────────┤
│ internal/             │ │ internal/             │ │                       │
│ ├── extensions/       │ │ ├── extensions/       │ │                       │
│ │   ├── validation/   │ │ │   ├── openai_api/   │ │                       │
│ │   ├── bestofn/      │ │ │   ├── anthropic_api/│ │                       │
│ │   └── dynamic/      │ │ │   ├── apikeys/      │ │                       │
│ │                     │ │ │   ├── routing/      │ │                       │
│ │                     │ │ │   └── usage/        │ │                       │
│ ├── config/           │ │ ├── config/           │ │                       │
│ └── cmd/              │ │ └── cmd/              │ │                       │
│                       │ │                       │ │                       │
│ Uses:                 │ │ Uses:                 │ │                       │
│ • racing (virtual)    │ │ • racing (virtual)    │ │                       │
│ • fallback (virtual)  │ │ • fallback (virtual)  │ │                       │
│                       │ │ • loadbalance         │ │                       │
│                       │ │                       │ │                       │
│ Unique Features:      │ │ Unique Features:      │ │                       │
│ • Best-of-N synthesis │ │ • OpenAI-compat API   │ │                       │
│ • Code validation     │ │ • Anthropic-compat API│ │                       │
│ • MCP Server          │ │ • Multi-tenant keys   │ │                       │
│ • Web UI              │ │ • Usage tracking      │ │                       │
└───────────────────────┘ └───────────────────────┘ └───────────────────────┘
```

---

## Phase 1: Foundation (Week 1-2)

### Goal
Establish the backend infrastructure and virtual providers in ai-provider-kit.

---

### AI-PROVIDER-KIT TASKS

#### Task 1.1: Create Package Structure
**Priority:** P0 - Blocking
**Estimated Effort:** 2 hours

Create the new package structure:

```bash
mkdir -p pkg/backend/{handlers,middleware,extensions}
mkdir -p pkg/backendtypes
mkdir -p pkg/providers/virtual/{racing,fallback,loadbalance}
```

**Files to create:**
- [ ] `pkg/backend/doc.go` - Package documentation
- [ ] `pkg/backendtypes/doc.go` - Package documentation
- [ ] `pkg/providers/virtual/doc.go` - Virtual providers documentation

**Acceptance Criteria:**
- Package structure created
- Documentation explains purpose of each package

---

#### Task 1.2: Define Backend Configuration Types
**Priority:** P0 - Blocking
**Estimated Effort:** 4 hours

**File:** `pkg/backendtypes/config.go`

```go
package backendtypes

import (
    "time"
    "github.com/anthropics/ai-provider-kit/pkg/types"
)

// BackendConfig defines the configuration for the backend server
type BackendConfig struct {
    Server     ServerConfig                      `yaml:"server"`
    Auth       AuthConfig                        `yaml:"auth"`
    Logging    LoggingConfig                     `yaml:"logging"`
    CORS       CORSConfig                        `yaml:"cors"`
    Providers  map[string]*types.ProviderConfig  `yaml:"providers"`
    Extensions map[string]ExtensionConfig        `yaml:"extensions"`
}

type ServerConfig struct {
    Host            string        `yaml:"host"`
    Port            int           `yaml:"port"`
    Version         string        `yaml:"version"`
    ReadTimeout     time.Duration `yaml:"read_timeout"`
    WriteTimeout    time.Duration `yaml:"write_timeout"`
    ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type AuthConfig struct {
    Enabled     bool     `yaml:"enabled"`
    APIPassword string   `yaml:"api_password"`
    APIKeyEnv   string   `yaml:"api_key_env"`
    PublicPaths []string `yaml:"public_paths"`
}

type LoggingConfig struct {
    Level  string `yaml:"level"`
    Format string `yaml:"format"` // "json" or "text"
}

type CORSConfig struct {
    Enabled        bool     `yaml:"enabled"`
    AllowedOrigins []string `yaml:"allowed_origins"`
    AllowedMethods []string `yaml:"allowed_methods"`
    AllowedHeaders []string `yaml:"allowed_headers"`
}

type ExtensionConfig struct {
    Enabled bool                   `yaml:"enabled"`
    Config  map[string]interface{} `yaml:"config"`
}
```

**Acceptance Criteria:**
- [ ] All configuration types defined
- [ ] YAML tags for serialization
- [ ] Default values documented
- [ ] Unit tests for config validation

---

#### Task 1.3: Define API Request/Response Types
**Priority:** P0 - Blocking
**Estimated Effort:** 4 hours

**File:** `pkg/backendtypes/requests.go`

```go
package backendtypes

import "github.com/anthropics/ai-provider-kit/pkg/types"

// GenerateRequest represents a code/chat generation request
type GenerateRequest struct {
    Provider    string                 `json:"provider,omitempty"`
    Model       string                 `json:"model,omitempty"`
    Prompt      string                 `json:"prompt"`
    Messages    []types.ChatMessage    `json:"messages,omitempty"`
    MaxTokens   int                    `json:"max_tokens,omitempty"`
    Temperature float64                `json:"temperature,omitempty"`
    Stream      bool                   `json:"stream,omitempty"`
    Tools       []types.Tool           `json:"tools,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ProviderConfigRequest for updating provider configuration
type ProviderConfigRequest struct {
    Type         string   `json:"type"`
    APIKey       string   `json:"api_key,omitempty"`
    APIKeys      []string `json:"api_keys,omitempty"`
    DefaultModel string   `json:"default_model,omitempty"`
    BaseURL      string   `json:"base_url,omitempty"`
    Enabled      bool     `json:"enabled"`
}
```

**File:** `pkg/backendtypes/responses.go`

```go
package backendtypes

import "time"

// APIResponse is the standard response wrapper
type APIResponse struct {
    Success   bool        `json:"success"`
    Data      interface{} `json:"data,omitempty"`
    Error     *APIError   `json:"error,omitempty"`
    RequestID string      `json:"request_id,omitempty"`
    Timestamp time.Time   `json:"timestamp"`
}

type APIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}

// GenerateResponse for code/chat generation
type GenerateResponse struct {
    Content  string                 `json:"content"`
    Model    string                 `json:"model"`
    Provider string                 `json:"provider"`
    Usage    *UsageInfo             `json:"usage,omitempty"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type UsageInfo struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

// ProviderInfo for provider listing
type ProviderInfo struct {
    Name        string   `json:"name"`
    Type        string   `json:"type"`
    Enabled     bool     `json:"enabled"`
    Healthy     bool     `json:"healthy"`
    Models      []string `json:"models,omitempty"`
    Description string   `json:"description,omitempty"`
}

// HealthResponse for health endpoints
type HealthResponse struct {
    Status    string                    `json:"status"`
    Version   string                    `json:"version"`
    Uptime    string                    `json:"uptime"`
    Providers map[string]ProviderHealth `json:"providers,omitempty"`
}

type ProviderHealth struct {
    Status  string `json:"status"`
    Latency int64  `json:"latency_ms"`
    Message string `json:"message,omitempty"`
}
```

**Acceptance Criteria:**
- [ ] All request types defined with validation tags
- [ ] All response types defined with JSON tags
- [ ] Consistent with existing types.* types
- [ ] Unit tests for serialization/deserialization

---

#### Task 1.4: Implement Racing Virtual Provider
**Priority:** P0 - Blocking
**Estimated Effort:** 8 hours

**File:** `pkg/providers/virtual/racing/provider.go`

```go
package racing

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/anthropics/ai-provider-kit/pkg/types"
)

// RacingProvider races multiple providers and returns the first successful response
type RacingProvider struct {
    name        string
    providers   []types.Provider
    config      *Config
    performance *PerformanceTracker
    mu          sync.RWMutex
}

type Config struct {
    TimeoutMS       int      `yaml:"timeout_ms"`
    GracePeriodMS   int      `yaml:"grace_period_ms"`
    Strategy        Strategy `yaml:"strategy"`
    ProviderNames   []string `yaml:"providers"`
    PerformanceFile string   `yaml:"performance_file,omitempty"`
}

type Strategy string

const (
    StrategyFirstWins Strategy = "first_wins"
    StrategyWeighted  Strategy = "weighted"
    StrategyQuality   Strategy = "quality"
)

type raceResult struct {
    index    int
    provider types.Provider
    stream   types.ChatCompletionStream
    err      error
    latency  time.Duration
}

func NewRacingProvider(name string, config *Config) *RacingProvider {
    return &RacingProvider{
        name:        name,
        config:      config,
        performance: NewPerformanceTracker(),
    }
}

// SetProviders sets the providers to race
func (r *RacingProvider) SetProviders(providers []types.Provider) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.providers = providers
}

// Provider interface implementation
func (r *RacingProvider) Name() string                  { return r.name }
func (r *RacingProvider) Type() types.ProviderType     { return "racing" }
func (r *RacingProvider) Description() string          { return "Races multiple providers for fastest response" }

// ChatProvider interface implementation
func (r *RacingProvider) GenerateChatCompletion(ctx context.Context, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
    r.mu.RLock()
    providers := r.providers
    r.mu.RUnlock()

    if len(providers) == 0 {
        return nil, fmt.Errorf("no providers configured for racing")
    }

    timeout := time.Duration(r.config.TimeoutMS) * time.Millisecond
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    results := make(chan *raceResult, len(providers))
    var wg sync.WaitGroup

    // Start all providers racing
    for i, provider := range providers {
        wg.Add(1)
        go func(idx int, p types.Provider) {
            defer wg.Done()
            start := time.Now()

            chatProvider, ok := p.(types.ChatProvider)
            if !ok {
                results <- &raceResult{index: idx, provider: p, err: fmt.Errorf("provider does not support chat")}
                return
            }

            stream, err := chatProvider.GenerateChatCompletion(ctx, opts)
            results <- &raceResult{
                index:    idx,
                provider: p,
                stream:   stream,
                err:      err,
                latency:  time.Since(start),
            }
        }(i, provider)
    }

    // Close results channel when all done
    go func() {
        wg.Wait()
        close(results)
    }()

    // Select winner based on strategy
    return r.selectWinner(ctx, results)
}

func (r *RacingProvider) selectWinner(ctx context.Context, results chan *raceResult) (types.ChatCompletionStream, error) {
    switch r.config.Strategy {
    case StrategyWeighted:
        return r.weightedStrategy(ctx, results)
    case StrategyQuality:
        return r.qualityStrategy(ctx, results)
    default:
        return r.firstWinsStrategy(ctx, results)
    }
}

func (r *RacingProvider) firstWinsStrategy(ctx context.Context, results chan *raceResult) (types.ChatCompletionStream, error) {
    var lastErr error
    for result := range results {
        if result.err == nil && result.stream != nil {
            r.performance.RecordWin(result.provider.Name(), result.latency)
            return &racingStream{
                inner:    result.stream,
                provider: result.provider.Name(),
                latency:  result.latency,
            }, nil
        }
        if result.err != nil {
            r.performance.RecordLoss(result.provider.Name(), result.latency)
            lastErr = result.err
        }
    }
    if lastErr != nil {
        return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
    }
    return nil, fmt.Errorf("all providers failed")
}

func (r *RacingProvider) weightedStrategy(ctx context.Context, results chan *raceResult) (types.ChatCompletionStream, error) {
    // Collect results during grace period, pick best based on historical performance
    gracePeriod := time.Duration(r.config.GracePeriodMS) * time.Millisecond
    timer := time.NewTimer(gracePeriod)
    defer timer.Stop()

    var candidates []*raceResult

    for {
        select {
        case result, ok := <-results:
            if !ok {
                // All results received
                return r.pickBestCandidate(candidates)
            }
            if result.err == nil && result.stream != nil {
                candidates = append(candidates, result)
                // If we have a result and grace period hasn't started, start it
                if len(candidates) == 1 {
                    timer.Reset(gracePeriod)
                }
            }
        case <-timer.C:
            // Grace period expired, pick from candidates
            if len(candidates) > 0 {
                return r.pickBestCandidate(candidates)
            }
            // No candidates yet, continue waiting
        case <-ctx.Done():
            if len(candidates) > 0 {
                return r.pickBestCandidate(candidates)
            }
            return nil, ctx.Err()
        }
    }
}

func (r *RacingProvider) qualityStrategy(ctx context.Context, results chan *raceResult) (types.ChatCompletionStream, error) {
    // Wait for all results (up to timeout), then pick best
    return r.weightedStrategy(ctx, results)
}

func (r *RacingProvider) pickBestCandidate(candidates []*raceResult) (types.ChatCompletionStream, error) {
    if len(candidates) == 0 {
        return nil, fmt.Errorf("no successful candidates")
    }

    // Score candidates based on historical performance
    var best *raceResult
    var bestScore float64 = -1

    for _, c := range candidates {
        score := r.performance.GetScore(c.provider.Name())
        // Adjust score by latency (faster is better)
        latencyFactor := 1.0 / (1.0 + c.latency.Seconds())
        adjustedScore := score * latencyFactor

        if adjustedScore > bestScore {
            bestScore = adjustedScore
            best = c
        }
    }

    if best != nil {
        r.performance.RecordWin(best.provider.Name(), best.latency)
        return &racingStream{
            inner:    best.stream,
            provider: best.provider.Name(),
            latency:  best.latency,
        }, nil
    }

    // Fallback to first candidate
    r.performance.RecordWin(candidates[0].provider.Name(), candidates[0].latency)
    return &racingStream{
        inner:    candidates[0].stream,
        provider: candidates[0].provider.Name(),
        latency:  candidates[0].latency,
    }, nil
}

// GetPerformanceStats returns performance statistics for all providers
func (r *RacingProvider) GetPerformanceStats() map[string]*ProviderStats {
    return r.performance.GetAllStats()
}

// racingStream wraps a stream with racing metadata
type racingStream struct {
    inner    types.ChatCompletionStream
    provider string
    latency  time.Duration
}

func (s *racingStream) Next() (types.ChatCompletionChunk, error) {
    chunk, err := s.inner.Next()
    // Add racing metadata to first chunk
    if chunk.Metadata == nil {
        chunk.Metadata = make(map[string]interface{})
    }
    chunk.Metadata["racing_winner"] = s.provider
    chunk.Metadata["racing_latency_ms"] = s.latency.Milliseconds()
    return chunk, err
}

func (s *racingStream) Close() error {
    return s.inner.Close()
}
```

**File:** `pkg/providers/virtual/racing/performance.go`

```go
package racing

import (
    "sync"
    "time"
)

type PerformanceTracker struct {
    mu    sync.RWMutex
    stats map[string]*ProviderStats
}

type ProviderStats struct {
    TotalRaces   int64         `json:"total_races"`
    Wins         int64         `json:"wins"`
    Losses       int64         `json:"losses"`
    AvgLatency   time.Duration `json:"avg_latency"`
    TotalLatency time.Duration `json:"-"`
    WinRate      float64       `json:"win_rate"`
    LastUpdated  time.Time     `json:"last_updated"`
}

func NewPerformanceTracker() *PerformanceTracker {
    return &PerformanceTracker{
        stats: make(map[string]*ProviderStats),
    }
}

func (pt *PerformanceTracker) RecordWin(provider string, latency time.Duration) {
    pt.mu.Lock()
    defer pt.mu.Unlock()

    stats := pt.getOrCreate(provider)
    stats.TotalRaces++
    stats.Wins++
    stats.TotalLatency += latency
    stats.AvgLatency = stats.TotalLatency / time.Duration(stats.TotalRaces)
    stats.WinRate = float64(stats.Wins) / float64(stats.TotalRaces)
    stats.LastUpdated = time.Now()
}

func (pt *PerformanceTracker) RecordLoss(provider string, latency time.Duration) {
    pt.mu.Lock()
    defer pt.mu.Unlock()

    stats := pt.getOrCreate(provider)
    stats.TotalRaces++
    stats.Losses++
    stats.TotalLatency += latency
    stats.AvgLatency = stats.TotalLatency / time.Duration(stats.TotalRaces)
    stats.WinRate = float64(stats.Wins) / float64(stats.TotalRaces)
    stats.LastUpdated = time.Now()
}

func (pt *PerformanceTracker) GetScore(provider string) float64 {
    pt.mu.RLock()
    defer pt.mu.RUnlock()

    stats, ok := pt.stats[provider]
    if !ok {
        return 0.5 // Default score for new providers
    }

    // Score based on win rate (0.0 to 1.0)
    return stats.WinRate
}

func (pt *PerformanceTracker) GetAllStats() map[string]*ProviderStats {
    pt.mu.RLock()
    defer pt.mu.RUnlock()

    result := make(map[string]*ProviderStats)
    for k, v := range pt.stats {
        statsCopy := *v
        result[k] = &statsCopy
    }
    return result
}

func (pt *PerformanceTracker) getOrCreate(provider string) *ProviderStats {
    stats, ok := pt.stats[provider]
    if !ok {
        stats = &ProviderStats{}
        pt.stats[provider] = stats
    }
    return stats
}
```

**Acceptance Criteria:**
- [ ] Racing provider implements types.Provider and types.ChatProvider
- [ ] First-wins strategy implemented
- [ ] Weighted strategy with grace period implemented
- [ ] Performance tracking with win rates
- [ ] Thread-safe implementation
- [ ] Unit tests for all strategies

---

#### Task 1.5: Implement Fallback Virtual Provider
**Priority:** P0 - Blocking
**Estimated Effort:** 4 hours

**File:** `pkg/providers/virtual/fallback/provider.go`

```go
package fallback

import (
    "context"
    "fmt"

    "github.com/anthropics/ai-provider-kit/pkg/types"
)

// FallbackProvider tries providers in order until one succeeds
type FallbackProvider struct {
    name      string
    providers []types.Provider
    config    *Config
}

type Config struct {
    ProviderNames []string `yaml:"providers"`
    MaxRetries    int      `yaml:"max_retries"`
}

func NewFallbackProvider(name string, config *Config) *FallbackProvider {
    return &FallbackProvider{
        name:   name,
        config: config,
    }
}

func (f *FallbackProvider) SetProviders(providers []types.Provider) {
    f.providers = providers
}

func (f *FallbackProvider) Name() string              { return f.name }
func (f *FallbackProvider) Type() types.ProviderType { return "fallback" }
func (f *FallbackProvider) Description() string      { return "Tries providers in order until one succeeds" }

func (f *FallbackProvider) GenerateChatCompletion(ctx context.Context, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
    var lastErr error

    for i, provider := range f.providers {
        chatProvider, ok := provider.(types.ChatProvider)
        if !ok {
            continue
        }

        stream, err := chatProvider.GenerateChatCompletion(ctx, opts)
        if err == nil {
            return &fallbackStream{
                inner:         stream,
                providerName:  provider.Name(),
                providerIndex: i,
            }, nil
        }

        lastErr = err
    }

    if lastErr != nil {
        return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
    }
    return nil, fmt.Errorf("no providers available")
}

type fallbackStream struct {
    inner         types.ChatCompletionStream
    providerName  string
    providerIndex int
}

func (s *fallbackStream) Next() (types.ChatCompletionChunk, error) {
    chunk, err := s.inner.Next()
    if chunk.Metadata == nil {
        chunk.Metadata = make(map[string]interface{})
    }
    chunk.Metadata["fallback_provider"] = s.providerName
    chunk.Metadata["fallback_index"] = s.providerIndex
    return chunk, err
}

func (s *fallbackStream) Close() error {
    return s.inner.Close()
}
```

**Acceptance Criteria:**
- [ ] Fallback provider implements types.Provider
- [ ] Tries providers in order
- [ ] Returns first successful response
- [ ] Proper error aggregation
- [ ] Unit tests

---

#### Task 1.6: Implement Load Balance Virtual Provider
**Priority:** P1 - High
**Estimated Effort:** 4 hours

**File:** `pkg/providers/virtual/loadbalance/provider.go`

```go
package loadbalance

import (
    "context"
    "fmt"
    "sync/atomic"

    "github.com/anthropics/ai-provider-kit/pkg/types"
)

// LoadBalanceProvider distributes requests across providers
type LoadBalanceProvider struct {
    name      string
    providers []types.Provider
    config    *Config
    counter   uint64
}

type Config struct {
    Strategy      Strategy `yaml:"strategy"`
    ProviderNames []string `yaml:"providers"`
}

type Strategy string

const (
    StrategyRoundRobin Strategy = "round_robin"
    StrategyRandom     Strategy = "random"
    StrategyWeighted   Strategy = "weighted"
)

func NewLoadBalanceProvider(name string, config *Config) *LoadBalanceProvider {
    return &LoadBalanceProvider{
        name:   name,
        config: config,
    }
}

func (lb *LoadBalanceProvider) SetProviders(providers []types.Provider) {
    lb.providers = providers
}

func (lb *LoadBalanceProvider) Name() string              { return lb.name }
func (lb *LoadBalanceProvider) Type() types.ProviderType { return "loadbalance" }
func (lb *LoadBalanceProvider) Description() string      { return "Distributes requests across providers" }

func (lb *LoadBalanceProvider) GenerateChatCompletion(ctx context.Context, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
    if len(lb.providers) == 0 {
        return nil, fmt.Errorf("no providers configured")
    }

    provider := lb.selectProvider()

    chatProvider, ok := provider.(types.ChatProvider)
    if !ok {
        return nil, fmt.Errorf("selected provider does not support chat")
    }

    return chatProvider.GenerateChatCompletion(ctx, opts)
}

func (lb *LoadBalanceProvider) selectProvider() types.Provider {
    switch lb.config.Strategy {
    case StrategyRandom:
        return lb.providers[randomInt(len(lb.providers))]
    default: // Round robin
        idx := atomic.AddUint64(&lb.counter, 1) - 1
        return lb.providers[idx%uint64(len(lb.providers))]
    }
}

func randomInt(max int) int {
    // Simple random selection
    return int(time.Now().UnixNano() % int64(max))
}
```

**Acceptance Criteria:**
- [ ] Load balance provider implements types.Provider
- [ ] Round-robin strategy
- [ ] Random strategy
- [ ] Thread-safe counter
- [ ] Unit tests

---

#### Task 1.7: Create Extension Framework Interface
**Priority:** P0 - Blocking
**Estimated Effort:** 6 hours

**File:** `pkg/backend/extensions/interface.go`

```go
package extensions

import (
    "context"
    "net/http"

    "github.com/anthropics/ai-provider-kit/pkg/backendtypes"
    "github.com/anthropics/ai-provider-kit/pkg/types"
)

// Extension defines the interface for backend extensions
type Extension interface {
    // Metadata
    Name() string
    Version() string
    Description() string
    Dependencies() []string

    // Lifecycle
    Initialize(config map[string]interface{}) error
    Shutdown(ctx context.Context) error

    // Route registration (optional)
    RegisterRoutes(registrar RouteRegistrar) error

    // Request/Response hooks (optional - return nil to skip)
    BeforeGenerate(ctx context.Context, req *backendtypes.GenerateRequest) error
    AfterGenerate(ctx context.Context, req *backendtypes.GenerateRequest, resp *backendtypes.GenerateResponse) error

    // Provider hooks (optional)
    OnProviderError(ctx context.Context, provider types.Provider, err error) error
    OnProviderSelected(ctx context.Context, provider types.Provider) error
}

// RouteRegistrar allows extensions to register custom routes
type RouteRegistrar interface {
    Handle(pattern string, handler http.Handler)
    HandleFunc(pattern string, handler http.HandlerFunc)
}

// ExtensionRegistry manages extension lifecycle
type ExtensionRegistry interface {
    Register(ext Extension) error
    Get(name string) (Extension, bool)
    List() []Extension
    Initialize(configs map[string]backendtypes.ExtensionConfig) error
    Shutdown(ctx context.Context) error
}

// BaseExtension provides default implementations for optional methods
type BaseExtension struct{}

func (b *BaseExtension) Dependencies() []string { return nil }
func (b *BaseExtension) RegisterRoutes(r RouteRegistrar) error { return nil }
func (b *BaseExtension) BeforeGenerate(ctx context.Context, req *backendtypes.GenerateRequest) error { return nil }
func (b *BaseExtension) AfterGenerate(ctx context.Context, req *backendtypes.GenerateRequest, resp *backendtypes.GenerateResponse) error { return nil }
func (b *BaseExtension) OnProviderError(ctx context.Context, provider types.Provider, err error) error { return nil }
func (b *BaseExtension) OnProviderSelected(ctx context.Context, provider types.Provider) error { return nil }
func (b *BaseExtension) Shutdown(ctx context.Context) error { return nil }
```

**File:** `pkg/backend/extensions/registry.go`

```go
package extensions

import (
    "context"
    "fmt"
    "sync"

    "github.com/anthropics/ai-provider-kit/pkg/backendtypes"
)

type registry struct {
    mu         sync.RWMutex
    extensions map[string]Extension
    order      []string
}

func NewRegistry() ExtensionRegistry {
    return &registry{
        extensions: make(map[string]Extension),
        order:      make([]string, 0),
    }
}

func (r *registry) Register(ext Extension) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    name := ext.Name()
    if _, exists := r.extensions[name]; exists {
        return fmt.Errorf("extension %s already registered", name)
    }

    r.extensions[name] = ext
    r.order = append(r.order, name)
    return nil
}

func (r *registry) Get(name string) (Extension, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    ext, ok := r.extensions[name]
    return ext, ok
}

func (r *registry) List() []Extension {
    r.mu.RLock()
    defer r.mu.RUnlock()

    result := make([]Extension, 0, len(r.extensions))
    for _, name := range r.order {
        result = append(result, r.extensions[name])
    }
    return result
}

func (r *registry) Initialize(configs map[string]backendtypes.ExtensionConfig) error {
    r.mu.RLock()
    defer r.mu.RUnlock()

    sorted, err := r.topologicalSort()
    if err != nil {
        return fmt.Errorf("dependency resolution failed: %w", err)
    }

    for _, name := range sorted {
        ext := r.extensions[name]
        cfg, ok := configs[name]
        if !ok || !cfg.Enabled {
            continue
        }
        if err := ext.Initialize(cfg.Config); err != nil {
            return fmt.Errorf("failed to initialize extension %s: %w", name, err)
        }
    }
    return nil
}

func (r *registry) Shutdown(ctx context.Context) error {
    r.mu.RLock()
    defer r.mu.RUnlock()

    for i := len(r.order) - 1; i >= 0; i-- {
        name := r.order[i]
        if err := r.extensions[name].Shutdown(ctx); err != nil {
            return fmt.Errorf("failed to shutdown extension %s: %w", name, err)
        }
    }
    return nil
}

func (r *registry) topologicalSort() ([]string, error) {
    // Topological sort implementation for dependency ordering
    visited := make(map[string]bool)
    result := make([]string, 0, len(r.extensions))

    var visit func(name string) error
    visit = func(name string) error {
        if visited[name] {
            return nil
        }
        visited[name] = true

        ext, ok := r.extensions[name]
        if !ok {
            return nil
        }

        for _, dep := range ext.Dependencies() {
            if err := visit(dep); err != nil {
                return err
            }
        }

        result = append(result, name)
        return nil
    }

    for _, name := range r.order {
        if err := visit(name); err != nil {
            return nil, err
        }
    }

    return result, nil
}
```

**Acceptance Criteria:**
- [ ] Extension interface defined with all lifecycle methods
- [ ] BaseExtension provides sensible defaults
- [ ] Registry handles registration and lifecycle
- [ ] Topological sort for dependency ordering
- [ ] Unit tests for registry operations

---

#### Task 1.8: Create Core Middleware
**Priority:** P0 - Blocking
**Estimated Effort:** 4 hours

**Files to create:**
- [ ] `pkg/backend/middleware/requestid.go` - Request ID generation
- [ ] `pkg/backend/middleware/cors.go` - CORS handling
- [ ] `pkg/backend/middleware/logging.go` - Request logging
- [ ] `pkg/backend/middleware/recovery.go` - Panic recovery
- [ ] `pkg/backend/middleware/auth.go` - Authentication

**Acceptance Criteria:**
- [ ] All middleware implemented and tested
- [ ] Middleware follows standard http.Handler pattern
- [ ] Configuration-driven behavior
- [ ] Unit tests for each middleware

---

### VELOCITYCODE TASKS (Phase 1)

#### Task 1.9: Create Extension Adapter Layer
**Priority:** P1 - High
**Estimated Effort:** 4 hours

**File:** `internal/extensions/adapter.go`

Create adapter types bridging VelocityCode's current implementation with the new extension framework.

**Acceptance Criteria:**
- [ ] Adapter interface defined
- [ ] Compatibility with ai-provider-kit extensions verified
- [ ] Migration path documented

---

#### Task 1.10: Audit Current Handler Dependencies
**Priority:** P1 - High
**Estimated Effort:** 6 hours

Document all handler dependencies for migration planning.

**Output:** `internal/handlers/MIGRATION_AUDIT.md`

**Acceptance Criteria:**
- [ ] All handler dependencies documented
- [ ] Migration strategy for each handler defined
- [ ] Risk assessment completed

---

### CORTEX TASKS (Phase 1)

#### Task 1.11: Create Cortex Project Structure
**Priority:** P1 - High
**Estimated Effort:** 2 hours

```bash
mkdir -p cortex/{cmd/cortex,internal/{extensions,config}}
mkdir -p cortex/internal/extensions/{openai_api,anthropic_api,apikeys,routing,usage}
```

**Files to create:**
- [ ] `cortex/go.mod`
- [ ] `cortex/cmd/cortex/main.go`
- [ ] `cortex/internal/config/config.go`

**Acceptance Criteria:**
- [ ] Project structure created
- [ ] go.mod with ai-provider-kit dependency
- [ ] Basic main.go that starts server

---

#### Task 1.12: Define Cortex API Key Configuration
**Priority:** P1 - High
**Estimated Effort:** 4 hours

**File:** `cortex/internal/config/apikeys.go`

```go
package config

// APIKeyConfig defines a client API key with permissions
type APIKeyConfig struct {
    Key              string            `yaml:"key"`
    Name             string            `yaml:"name"`
    Description      string            `yaml:"description,omitempty"`

    // Access control
    AllowedProviders []string          `yaml:"allowed_providers"` // ["openai", "anthropic", "*"]
    AllowedModels    []string          `yaml:"allowed_models"`    // ["gpt-4o", "claude-*", "*"]
    DeniedModels     []string          `yaml:"denied_models"`     // ["o1-*"]

    // Rate limiting
    RateLimitRPM     int               `yaml:"rate_limit_rpm"`
    RateLimitTPM     int               `yaml:"rate_limit_tpm,omitempty"`

    // Usage tracking
    TrackUsage       bool              `yaml:"track_usage"`

    // Metadata
    Metadata         map[string]string `yaml:"metadata,omitempty"`
}

// CortexConfig extends BackendConfig with Cortex-specific fields
type CortexConfig struct {
    backendtypes.BackendConfig `yaml:",inline"`

    // Client API keys
    APIKeys []APIKeyConfig `yaml:"api_keys"`

    // Endpoint configuration
    Endpoints EndpointsConfig `yaml:"endpoints"`

    // Model aliases
    Aliases map[string]string `yaml:"aliases,omitempty"`

    // Usage tracking
    Usage UsageConfig `yaml:"usage"`
}

type EndpointsConfig struct {
    OpenAI    EndpointConfig `yaml:"openai"`
    Anthropic EndpointConfig `yaml:"anthropic"`
}

type EndpointConfig struct {
    Enabled    bool   `yaml:"enabled"`
    PathPrefix string `yaml:"path_prefix"`
}

type UsageConfig struct {
    Enabled bool   `yaml:"enabled"`
    Storage string `yaml:"storage"` // "sqlite", "postgres", "memory"
    Path    string `yaml:"path,omitempty"`
}
```

**Acceptance Criteria:**
- [ ] API key configuration types defined
- [ ] Provider/model permission patterns (wildcards)
- [ ] Rate limiting configuration
- [ ] Usage tracking configuration
- [ ] Unit tests for pattern matching

---

## Phase 2: Core Backend Implementation (Week 3-4)

### AI-PROVIDER-KIT TASKS

#### Task 2.1: Implement Base Handler Utilities
**Priority:** P0 - Blocking
**Estimated Effort:** 4 hours

**File:** `pkg/backend/handlers/base.go`

**Acceptance Criteria:**
- [ ] SendSuccess, SendError, SendCreated methods
- [ ] ParseJSON for request parsing
- [ ] Consistent response formatting
- [ ] Unit tests

---

#### Task 2.2: Implement Provider Management Handler
**Priority:** P0 - Blocking
**Estimated Effort:** 8 hours

**File:** `pkg/backend/handlers/providers.go`

**Acceptance Criteria:**
- [ ] List, Get, Update providers
- [ ] Provider health checking
- [ ] Provider testing endpoint
- [ ] Virtual provider support
- [ ] Unit tests

---

#### Task 2.3: Implement Generation Handler
**Priority:** P0 - Blocking
**Estimated Effort:** 8 hours

**File:** `pkg/backend/handlers/generate.go`

**Acceptance Criteria:**
- [ ] Generate endpoint with extension hooks
- [ ] Provider selection (including virtual providers)
- [ ] Error handling with extension notification
- [ ] Unit tests

---

#### Task 2.4: Implement Health and Status Handlers
**Priority:** P1 - High
**Estimated Effort:** 4 hours

**File:** `pkg/backend/handlers/health.go`

**Acceptance Criteria:**
- [ ] Status endpoint
- [ ] Health endpoint with provider health
- [ ] Version information
- [ ] Unit tests

---

#### Task 2.5: Implement Backend Server
**Priority:** P0 - Blocking
**Estimated Effort:** 8 hours

**File:** `pkg/backend/server.go`

**Acceptance Criteria:**
- [ ] Server initialization with config
- [ ] Provider initialization (including virtual providers)
- [ ] Extension initialization
- [ ] Middleware application
- [ ] Graceful shutdown
- [ ] Integration tests

---

### VELOCITYCODE TASKS (Phase 2)

#### Task 2.6: Create Validation Extension
**Priority:** P1 - High
**Estimated Effort:** 8 hours

**File:** `internal/extensions/validation/extension.go`

**Acceptance Criteria:**
- [ ] Validation extension implements Extension interface
- [ ] Language-specific validators (Go, Python, JS, TS)
- [ ] Auto-fix capability
- [ ] Configuration support
- [ ] Unit tests

---

#### Task 2.7: Create Best-of-N Extension
**Priority:** P1 - High
**Estimated Effort:** 12 hours

**File:** `internal/extensions/bestofn/extension.go`

**Note:** Best-of-N remains in VelocityCode only due to code-generation-specific synthesis logic.

**Acceptance Criteria:**
- [ ] Best-of-N extension implements Extension interface
- [ ] Multi-candidate generation logic
- [ ] Code synthesis engine integration
- [ ] Quality validation
- [ ] Configuration support
- [ ] Unit tests

---

#### Task 2.8: Create Dynamic Provider Extension
**Priority:** P1 - High
**Estimated Effort:** 8 hours

**File:** `internal/extensions/dynamic/extension.go`

**Acceptance Criteria:**
- [ ] Provider aliasing
- [ ] Runtime provider activation/deactivation
- [ ] Model overrides
- [ ] Configuration support
- [ ] Unit tests

---

### CORTEX TASKS (Phase 2)

#### Task 2.9: Create OpenAI-Compatible API Extension
**Priority:** P0 - Blocking
**Estimated Effort:** 10 hours

**File:** `cortex/internal/extensions/openai_api/extension.go`

```go
package openai_api

import (
    "context"
    "encoding/json"
    "net/http"

    "github.com/anthropics/ai-provider-kit/pkg/backend/extensions"
    "github.com/anthropics/ai-provider-kit/pkg/backendtypes"
    "github.com/anthropics/ai-provider-kit/pkg/types"
)

type OpenAICompatExtension struct {
    extensions.BaseExtension
    translator *Translator
    providers  map[string]types.Provider
}

func (e *OpenAICompatExtension) Name() string        { return "openai_compat" }
func (e *OpenAICompatExtension) Version() string    { return "1.0.0" }
func (e *OpenAICompatExtension) Description() string { return "OpenAI-compatible API endpoints" }

func (e *OpenAICompatExtension) RegisterRoutes(r extensions.RouteRegistrar) error {
    r.HandleFunc("POST /v1/chat/completions", e.ChatCompletions)
    r.HandleFunc("GET /v1/models", e.ListModels)
    r.HandleFunc("GET /v1/models/{model}", e.GetModel)
    return nil
}

// ChatCompletions handles POST /v1/chat/completions
func (e *OpenAICompatExtension) ChatCompletions(w http.ResponseWriter, r *http.Request) {
    var req OpenAIChatRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        e.sendError(w, "invalid_request_error", err.Error(), http.StatusBadRequest)
        return
    }

    // Translate to StandardRequest
    standardReq := e.translator.ToStandard(&req)

    // Route to provider
    provider, err := e.routeToProvider(req.Model)
    if err != nil {
        e.sendError(w, "model_not_found", err.Error(), http.StatusNotFound)
        return
    }

    // Generate
    chatProvider := provider.(types.ChatProvider)
    stream, err := chatProvider.GenerateChatCompletion(r.Context(), standardReq)
    if err != nil {
        e.sendError(w, "api_error", err.Error(), http.StatusInternalServerError)
        return
    }
    defer stream.Close()

    if req.Stream {
        e.streamResponse(w, stream, req.Model)
    } else {
        e.sendResponse(w, stream, req.Model)
    }
}

// ListModels handles GET /v1/models
func (e *OpenAICompatExtension) ListModels(w http.ResponseWriter, r *http.Request) {
    models := make([]OpenAIModel, 0)

    for name, provider := range e.providers {
        if mp, ok := provider.(types.ModelProvider); ok {
            for _, model := range mp.GetModels() {
                models = append(models, OpenAIModel{
                    ID:      model,
                    Object:  "model",
                    OwnedBy: name,
                })
            }
        }
    }

    json.NewEncoder(w).Encode(OpenAIModelList{
        Object: "list",
        Data:   models,
    })
}
```

**File:** `cortex/internal/extensions/openai_api/types.go`

```go
package openai_api

// OpenAI API types for request/response compatibility

type OpenAIChatRequest struct {
    Model       string           `json:"model"`
    Messages    []OpenAIMessage  `json:"messages"`
    MaxTokens   int              `json:"max_tokens,omitempty"`
    Temperature float64          `json:"temperature,omitempty"`
    Stream      bool             `json:"stream,omitempty"`
    Tools       []OpenAITool     `json:"tools,omitempty"`
    ToolChoice  interface{}      `json:"tool_choice,omitempty"`
}

type OpenAIMessage struct {
    Role       string `json:"role"`
    Content    string `json:"content"`
    Name       string `json:"name,omitempty"`
    ToolCallID string `json:"tool_call_id,omitempty"`
}

type OpenAIChatResponse struct {
    ID      string         `json:"id"`
    Object  string         `json:"object"`
    Created int64          `json:"created"`
    Model   string         `json:"model"`
    Choices []OpenAIChoice `json:"choices"`
    Usage   OpenAIUsage    `json:"usage"`
}

type OpenAIChoice struct {
    Index        int           `json:"index"`
    Message      OpenAIMessage `json:"message"`
    FinishReason string        `json:"finish_reason"`
}

type OpenAIUsage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

type OpenAIModel struct {
    ID      string `json:"id"`
    Object  string `json:"object"`
    OwnedBy string `json:"owned_by"`
}

type OpenAIModelList struct {
    Object string        `json:"object"`
    Data   []OpenAIModel `json:"data"`
}

type OpenAIError struct {
    Error OpenAIErrorDetail `json:"error"`
}

type OpenAIErrorDetail struct {
    Message string `json:"message"`
    Type    string `json:"type"`
    Code    string `json:"code,omitempty"`
}
```

**Acceptance Criteria:**
- [ ] OpenAI chat completions endpoint
- [ ] OpenAI models endpoint
- [ ] Request translation to StandardRequest
- [ ] Response translation to OpenAI format
- [ ] Streaming support (SSE in OpenAI format)
- [ ] Unit tests

---

#### Task 2.10: Create Anthropic-Compatible API Extension
**Priority:** P0 - Blocking
**Estimated Effort:** 10 hours

**File:** `cortex/internal/extensions/anthropic_api/extension.go`

Similar structure to OpenAI extension but with Anthropic API format:
- `POST /anthropic/v1/messages`
- `GET /anthropic/v1/models`

**Acceptance Criteria:**
- [ ] Anthropic messages endpoint
- [ ] Anthropic models endpoint
- [ ] Request translation to StandardRequest
- [ ] Response translation to Anthropic format
- [ ] Streaming support (SSE in Anthropic format)
- [ ] Unit tests

---

#### Task 2.11: Create API Keys Extension
**Priority:** P0 - Blocking
**Estimated Effort:** 8 hours

**File:** `cortex/internal/extensions/apikeys/extension.go`

```go
package apikeys

import (
    "context"
    "fmt"
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/anthropics/ai-provider-kit/pkg/backend/extensions"
    "github.com/anthropics/ai-provider-kit/pkg/backendtypes"
    "cortex/internal/config"
)

type APIKeysExtension struct {
    extensions.BaseExtension
    keys       map[string]*config.APIKeyConfig
    rateLimits map[string]*RateLimiter
    mu         sync.RWMutex
}

type RateLimiter struct {
    rpm       int
    requests  int64
    window    time.Time
    mu        sync.Mutex
}

func (e *APIKeysExtension) Name() string { return "apikeys" }

func (e *APIKeysExtension) Initialize(cfg map[string]interface{}) error {
    // Load API keys from config
    // Initialize rate limiters
    return nil
}

// BeforeGenerate validates API key and checks permissions
func (e *APIKeysExtension) BeforeGenerate(ctx context.Context, req *backendtypes.GenerateRequest) error {
    apiKey := e.extractAPIKey(ctx)
    if apiKey == "" {
        return fmt.Errorf("missing API key")
    }

    keyConfig, ok := e.keys[apiKey]
    if !ok {
        return fmt.Errorf("invalid API key")
    }

    // Check provider permission
    if !e.isProviderAllowed(keyConfig, req.Provider) {
        return fmt.Errorf("provider %s not allowed for this API key", req.Provider)
    }

    // Check model permission
    if !e.isModelAllowed(keyConfig, req.Model) {
        return fmt.Errorf("model %s not allowed for this API key", req.Model)
    }

    // Check rate limit
    if !e.checkRateLimit(apiKey, keyConfig) {
        return fmt.Errorf("rate limit exceeded")
    }

    // Store API key name in context for usage tracking
    req.Metadata["api_key_name"] = keyConfig.Name

    return nil
}

func (e *APIKeysExtension) isProviderAllowed(key *config.APIKeyConfig, provider string) bool {
    for _, allowed := range key.AllowedProviders {
        if allowed == "*" || allowed == provider {
            return true
        }
    }
    return false
}

func (e *APIKeysExtension) isModelAllowed(key *config.APIKeyConfig, model string) bool {
    // Check denied first
    for _, denied := range key.DeniedModels {
        if e.matchPattern(denied, model) {
            return false
        }
    }

    // Check allowed
    for _, allowed := range key.AllowedModels {
        if e.matchPattern(allowed, model) {
            return true
        }
    }

    return false
}

func (e *APIKeysExtension) matchPattern(pattern, value string) bool {
    if pattern == "*" {
        return true
    }
    if strings.HasSuffix(pattern, "*") {
        prefix := strings.TrimSuffix(pattern, "*")
        return strings.HasPrefix(value, prefix)
    }
    return pattern == value
}

func (e *APIKeysExtension) checkRateLimit(apiKey string, keyConfig *config.APIKeyConfig) bool {
    e.mu.Lock()
    limiter, ok := e.rateLimits[apiKey]
    if !ok {
        limiter = &RateLimiter{
            rpm:    keyConfig.RateLimitRPM,
            window: time.Now(),
        }
        e.rateLimits[apiKey] = limiter
    }
    e.mu.Unlock()

    limiter.mu.Lock()
    defer limiter.mu.Unlock()

    now := time.Now()
    if now.Sub(limiter.window) > time.Minute {
        limiter.requests = 0
        limiter.window = now
    }

    if limiter.requests >= int64(limiter.rpm) {
        return false
    }

    limiter.requests++
    return true
}
```

**Acceptance Criteria:**
- [ ] API key validation
- [ ] Provider permission checking with wildcards
- [ ] Model permission checking with wildcards
- [ ] Rate limiting per API key
- [ ] API key name passed to usage tracking
- [ ] Unit tests

---

#### Task 2.12: Create Usage Tracking Extension
**Priority:** P1 - High
**Estimated Effort:** 8 hours

**File:** `cortex/internal/extensions/usage/extension.go`

```go
package usage

import (
    "context"
    "database/sql"
    "time"

    "github.com/anthropics/ai-provider-kit/pkg/backend/extensions"
    "github.com/anthropics/ai-provider-kit/pkg/backendtypes"
    _ "github.com/mattn/go-sqlite3"
)

type UsageExtension struct {
    extensions.BaseExtension
    db     *sql.DB
    config *Config
}

type Config struct {
    Enabled bool   `mapstructure:"enabled"`
    Storage string `mapstructure:"storage"`
    Path    string `mapstructure:"path"`
}

type UsageRecord struct {
    RequestID     string    `json:"request_id"`
    Timestamp     time.Time `json:"timestamp"`
    APIKeyName    string    `json:"api_key_name"`
    Model         string    `json:"model"`
    Provider      string    `json:"provider"`
    Endpoint      string    `json:"endpoint"`
    InputTokens   int       `json:"input_tokens"`
    OutputTokens  int       `json:"output_tokens"`
    TotalTokens   int       `json:"total_tokens"`
    LatencyMS     int64     `json:"latency_ms"`
    Status        string    `json:"status"`
    ErrorCode     string    `json:"error_code,omitempty"`
}

func (e *UsageExtension) Name() string { return "usage" }

func (e *UsageExtension) Initialize(cfg map[string]interface{}) error {
    // Parse config
    // Initialize database
    // Create schema if needed
    return e.initDB()
}

func (e *UsageExtension) initDB() error {
    var err error
    e.db, err = sql.Open("sqlite3", e.config.Path)
    if err != nil {
        return err
    }

    schema := `
    CREATE TABLE IF NOT EXISTS usage (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        request_id TEXT NOT NULL,
        timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
        api_key_name TEXT NOT NULL,
        model TEXT NOT NULL,
        provider TEXT NOT NULL,
        endpoint TEXT NOT NULL,
        input_tokens INTEGER,
        output_tokens INTEGER,
        total_tokens INTEGER,
        latency_ms INTEGER,
        status TEXT NOT NULL,
        error_code TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_usage_api_key ON usage(api_key_name);
    CREATE INDEX IF NOT EXISTS idx_usage_timestamp ON usage(timestamp);
    CREATE INDEX IF NOT EXISTS idx_usage_model ON usage(model);
    `

    _, err = e.db.Exec(schema)
    return err
}

// AfterGenerate records usage
func (e *UsageExtension) AfterGenerate(ctx context.Context, req *backendtypes.GenerateRequest, resp *backendtypes.GenerateResponse) error {
    if !e.config.Enabled {
        return nil
    }

    apiKeyName, _ := req.Metadata["api_key_name"].(string)
    startTime, _ := req.Metadata["start_time"].(time.Time)

    record := &UsageRecord{
        RequestID:   resp.Metadata["request_id"].(string),
        Timestamp:   time.Now(),
        APIKeyName:  apiKeyName,
        Model:       resp.Model,
        Provider:    resp.Provider,
        Endpoint:    req.Metadata["endpoint"].(string),
        LatencyMS:   time.Since(startTime).Milliseconds(),
        Status:      "success",
    }

    if resp.Usage != nil {
        record.InputTokens = resp.Usage.PromptTokens
        record.OutputTokens = resp.Usage.CompletionTokens
        record.TotalTokens = resp.Usage.TotalTokens
    }

    return e.insertRecord(record)
}

func (e *UsageExtension) insertRecord(record *UsageRecord) error {
    _, err := e.db.Exec(`
        INSERT INTO usage (request_id, api_key_name, model, provider, endpoint,
                          input_tokens, output_tokens, total_tokens, latency_ms, status, error_code)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, record.RequestID, record.APIKeyName, record.Model, record.Provider, record.Endpoint,
       record.InputTokens, record.OutputTokens, record.TotalTokens, record.LatencyMS,
       record.Status, record.ErrorCode)
    return err
}

// RegisterRoutes adds usage query endpoints
func (e *UsageExtension) RegisterRoutes(r extensions.RouteRegistrar) error {
    r.HandleFunc("GET /usage", e.GetUsage)
    r.HandleFunc("GET /usage/summary", e.GetUsageSummary)
    return nil
}

func (e *UsageExtension) Shutdown(ctx context.Context) error {
    if e.db != nil {
        return e.db.Close()
    }
    return nil
}
```

**Acceptance Criteria:**
- [ ] Usage tracking per request
- [ ] SQLite storage
- [ ] Usage query endpoints
- [ ] API key usage aggregation
- [ ] Unit tests

---

## Phase 3: Integration (Week 5-6)

### AI-PROVIDER-KIT TASKS

#### Task 3.1: Add OAuth Handler
**Priority:** P1 - High
**Estimated Effort:** 8 hours

**Acceptance Criteria:**
- [ ] OAuth initiation endpoint
- [ ] OAuth callback endpoint
- [ ] Token refresh endpoint
- [ ] Integration with oauthmanager

---

#### Task 3.2: Add Metrics Handler
**Priority:** P1 - High
**Estimated Effort:** 6 hours

**Acceptance Criteria:**
- [ ] Provider metrics endpoint
- [ ] System metrics endpoint
- [ ] Racing performance metrics endpoint

---

#### Task 3.3: Add Streaming Support
**Priority:** P1 - High
**Estimated Effort:** 8 hours

**Acceptance Criteria:**
- [ ] SSE endpoint for streaming generation
- [ ] Proper connection handling
- [ ] Extension hooks for streaming

---

#### Task 3.4: Register Virtual Providers in Factory
**Priority:** P0 - Blocking
**Estimated Effort:** 4 hours

Update factory to register virtual providers:

```go
func RegisterDefaultProviders(f ProviderFactory) {
    // Real providers
    f.RegisterProvider("openai", openai.NewProvider)
    f.RegisterProvider("anthropic", anthropic.NewProvider)
    // ... other providers

    // Virtual providers
    f.RegisterProvider("racing", racing.NewRacingProvider)
    f.RegisterProvider("fallback", fallback.NewFallbackProvider)
    f.RegisterProvider("loadbalance", loadbalance.NewLoadBalanceProvider)
}
```

**Acceptance Criteria:**
- [ ] Virtual providers registered in factory
- [ ] Virtual provider configuration parsing
- [ ] Provider dependency resolution (racing depends on other providers)
- [ ] Unit tests

---

### VELOCITYCODE TASKS (Phase 3)

#### Task 3.5: Refactor Server to Use ai-provider-kit Backend
**Priority:** P0 - Blocking
**Estimated Effort:** 16 hours

**Acceptance Criteria:**
- [ ] VelocityCode uses ai-provider-kit Server
- [ ] Extensions properly registered
- [ ] Virtual providers (racing, fallback) configured
- [ ] All existing functionality preserved
- [ ] Frontend serving maintained

---

#### Task 3.6: Migrate Handlers to Extensions
**Priority:** P0 - Blocking
**Estimated Effort:** 12 hours

**Acceptance Criteria:**
- [ ] All VC-specific logic in extensions
- [ ] Core handlers from ai-provider-kit used
- [ ] Feature parity with current implementation

---

#### Task 3.7: Integration Testing
**Priority:** P0 - Blocking
**Estimated Effort:** 8 hours

**Acceptance Criteria:**
- [ ] All endpoints tested
- [ ] Extension hooks tested
- [ ] Virtual provider integration tested
- [ ] Error scenarios tested

---

### CORTEX TASKS (Phase 3)

#### Task 3.8: Create Routing Extension
**Priority:** P1 - High
**Estimated Effort:** 6 hours

**File:** `cortex/internal/extensions/routing/extension.go`

Handle model → provider routing and aliases:

**Acceptance Criteria:**
- [ ] Model to provider mapping
- [ ] Model alias resolution
- [ ] Auto-detection of provider from model name
- [ ] Integration with virtual providers
- [ ] Unit tests

---

#### Task 3.9: Cortex Server Integration
**Priority:** P0 - Blocking
**Estimated Effort:** 8 hours

**File:** `cortex/cmd/cortex/main.go`

```go
package main

import (
    "log"
    "os"

    "github.com/anthropics/ai-provider-kit/pkg/backend"
    "cortex/internal/config"
    "cortex/internal/extensions/openai_api"
    "cortex/internal/extensions/anthropic_api"
    "cortex/internal/extensions/apikeys"
    "cortex/internal/extensions/routing"
    "cortex/internal/extensions/usage"
)

func main() {
    cfg, err := config.Load("cortex.yaml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    server, err := backend.NewServer(
        &cfg.BackendConfig,
        backend.WithExtension(apikeys.New(cfg.APIKeys)),
        backend.WithExtension(routing.New(cfg.Aliases)),
        backend.WithExtension(openai_api.New()),
        backend.WithExtension(anthropic_api.New()),
        backend.WithExtension(usage.New(cfg.Usage)),
    )
    if err != nil {
        log.Fatalf("Failed to create server: %v", err)
    }

    log.Printf("Starting Cortex on %s:%d", cfg.Server.Host, cfg.Server.Port)
    if err := server.Start(); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

**Acceptance Criteria:**
- [ ] Cortex starts with all extensions
- [ ] OpenAI endpoint works
- [ ] Anthropic endpoint works
- [ ] API key validation works
- [ ] Usage tracking works
- [ ] Racing provider works through proxy

---

#### Task 3.10: Cortex Integration Testing
**Priority:** P0 - Blocking
**Estimated Effort:** 8 hours

**Acceptance Criteria:**
- [ ] OpenAI SDK compatibility tested
- [ ] Anthropic SDK compatibility tested
- [ ] API key permissions tested
- [ ] Rate limiting tested
- [ ] Usage tracking verified
- [ ] Racing through proxy tested

---

## Phase 4: Cleanup and Documentation (Week 7-8)

### AI-PROVIDER-KIT TASKS

#### Task 4.1: Documentation
**Priority:** P1 - High
**Estimated Effort:** 8 hours

- [ ] Package documentation
- [ ] API documentation
- [ ] Extension development guide
- [ ] Virtual provider configuration guide
- [ ] Migration guide

---

#### Task 4.2: Performance Optimization
**Priority:** P2 - Medium
**Estimated Effort:** 8 hours

- [ ] Profile and optimize hot paths
- [ ] Racing provider optimization
- [ ] Connection pooling

---

### VELOCITYCODE TASKS (Phase 4)

#### Task 4.3: Remove Deprecated Code
**Priority:** P1 - High
**Estimated Effort:** 8 hours

- [ ] Remove migrated handlers
- [ ] Remove duplicate middleware
- [ ] Remove duplicate types

---

#### Task 4.4: Update Configuration
**Priority:** P1 - High
**Estimated Effort:** 4 hours

- [ ] Config migration documented
- [ ] Virtual provider configuration added
- [ ] Extension configs structured

---

#### Task 4.5: Final Testing
**Priority:** P0 - Blocking
**Estimated Effort:** 8 hours

- [ ] Full regression testing
- [ ] Performance comparison
- [ ] Feature parity verification

---

### CORTEX TASKS (Phase 4)

#### Task 4.6: Cortex Documentation
**Priority:** P1 - High
**Estimated Effort:** 6 hours

- [ ] API documentation (OpenAI-compat, Anthropic-compat)
- [ ] Configuration guide
- [ ] API key management guide
- [ ] Usage tracking guide

---

#### Task 4.7: Cortex Final Testing
**Priority:** P0 - Blocking
**Estimated Effort:** 6 hours

- [ ] End-to-end testing with real SDKs
- [ ] Load testing
- [ ] Error handling verification

---

## Parallel Execution Summary

### Week 1-2: Foundation
| AI-Provider-Kit | VelocityCode | Cortex |
|----------------|--------------|--------|
| Task 1.1: Package structure | Task 1.9: Extension adapter | Task 1.11: Project structure |
| Task 1.2: Config types | Task 1.10: Handler audit | Task 1.12: API key config |
| Task 1.3: Request/Response types | (blocked) | (blocked) |
| Task 1.4: Racing provider | (blocked) | (blocked) |
| Task 1.5: Fallback provider | (blocked) | (blocked) |
| Task 1.6: LoadBalance provider | (blocked) | (blocked) |
| Task 1.7: Extension framework | (blocked) | (blocked) |
| Task 1.8: Middleware | (blocked) | (blocked) |

### Week 3-4: Core Implementation
| AI-Provider-Kit | VelocityCode | Cortex |
|----------------|--------------|--------|
| Task 2.1: Base handler | Task 2.6: Validation ext | Task 2.9: OpenAI API ext |
| Task 2.2: Providers handler | Task 2.7: Best-of-N ext | Task 2.10: Anthropic API ext |
| Task 2.3: Generate handler | Task 2.8: Dynamic ext | Task 2.11: API keys ext |
| Task 2.4: Health handler | (parallel) | Task 2.12: Usage ext |
| Task 2.5: Server | (parallel) | (parallel) |

### Week 5-6: Integration
| AI-Provider-Kit | VelocityCode | Cortex |
|----------------|--------------|--------|
| Task 3.1: OAuth handler | Task 3.5: Server refactor | Task 3.8: Routing ext |
| Task 3.2: Metrics handler | Task 3.6: Handler migration | Task 3.9: Server integration |
| Task 3.3: Streaming | Task 3.7: Integration testing | Task 3.10: Integration testing |
| Task 3.4: Virtual provider factory | (parallel) | (parallel) |

### Week 7-8: Cleanup
| AI-Provider-Kit | VelocityCode | Cortex |
|----------------|--------------|--------|
| Task 4.1: Documentation | Task 4.3: Remove deprecated | Task 4.6: Documentation |
| Task 4.2: Optimization | Task 4.4: Update config | Task 4.7: Final testing |
| (complete) | Task 4.5: Final testing | (complete) |

---

## Configuration Examples

### ai-provider-kit with Virtual Providers
```yaml
providers:
  openai:
    type: openai
    api_key: ${OPENAI_API_KEY}

  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}

  # Racing virtual provider
  racing-fast:
    type: racing
    config:
      timeout_ms: 5000
      grace_period_ms: 500
      strategy: first_wins
      providers: ["openai", "anthropic"]

  # Fallback virtual provider
  fallback-reliable:
    type: fallback
    config:
      providers: ["openai", "anthropic", "gemini"]
```

### VelocityCode Configuration
```yaml
server:
  port: 8080

providers:
  openai:
    type: openai
    api_key: ${OPENAI_API_KEY}

  racing:
    type: racing
    config:
      providers: ["openai", "anthropic"]
      strategy: first_wins

extensions:
  validation:
    enabled: true
    config:
      auto_fix: true
      languages: ["go", "python", "javascript"]

  best_of_n:
    enabled: true
    config:
      num_candidates: 3
      synthesis_method: code_quality
```

### Cortex Configuration
```yaml
server:
  port: 8090

endpoints:
  openai:
    enabled: true
    path_prefix: "/v1"
  anthropic:
    enabled: true
    path_prefix: "/anthropic/v1"

providers:
  openai:
    type: openai
    api_key: ${OPENAI_API_KEY}

  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}

  racing:
    type: racing
    config:
      providers: ["openai", "anthropic"]

api_keys:
  - key: "sk-cortex-dev"
    name: "development"
    allowed_providers: ["*"]
    allowed_models: ["gpt-4o-mini", "claude-haiku-*"]
    rate_limit_rpm: 60
    track_usage: true

  - key: "sk-cortex-prod"
    name: "production"
    allowed_providers: ["*"]
    allowed_models: ["*"]
    denied_models: ["o1-*"]
    rate_limit_rpm: 1000
    track_usage: true

aliases:
  fast: gpt-4o-mini
  smart: gpt-4o
  code: claude-sonnet-4-20250514

usage:
  enabled: true
  storage: sqlite
  path: ./cortex_usage.db
```

---

## Success Metrics

### Code Quality
- [ ] Test coverage > 80% for new code
- [ ] No increase in cyclomatic complexity
- [ ] All linting checks pass

### Performance
- [ ] Response time within 5% of current implementation
- [ ] Racing provider adds < 10ms overhead
- [ ] Memory usage within 10% of current

### Functionality
- [ ] All VelocityCode features work
- [ ] Cortex passes OpenAI SDK tests
- [ ] Cortex passes Anthropic SDK tests
- [ ] Racing works across all three projects

### Maintainability
- [ ] Reduced code duplication (target: 2000+ lines removed from VC)
- [ ] Clear separation of concerns
- [ ] Well-documented extension points
- [ ] Racing logic shared (not duplicated)
