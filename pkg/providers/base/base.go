// Package base provides common functionality and utilities for AI providers.
// It includes base implementations, shared components, and standardized patterns
// that can be used by specific provider implementations.
package base

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// BaseProvider provides common functionality for all providers
type BaseProvider struct {
	name    string
	config  types.ProviderConfig
	client  *http.Client
	logger  *log.Logger
	mutex   sync.RWMutex
	metrics types.ProviderMetrics
}

// NewBaseProvider creates a new base provider
func NewBaseProvider(name string, config types.ProviderConfig, client *http.Client, logger *log.Logger) *BaseProvider {
	return &BaseProvider{
		name:   name,
		config: config,
		client: client,
		logger: logger,
		metrics: types.ProviderMetrics{
			RequestCount: 0,
			SuccessCount: 0,
			ErrorCount:   0,
			TokensUsed:   0,
		},
	}
}

func (p *BaseProvider) Name() string {
	return p.name
}

func (p *BaseProvider) Type() types.ProviderType {
	return p.config.Type
}

func (p *BaseProvider) Description() string {
	return "Base provider implementation"
}

// Configure updates the provider configuration
func (p *BaseProvider) Configure(config types.ProviderConfig) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	oldType := p.config.Type
	p.config = config
	if p.logger != nil {
		p.logger.Printf("Provider %s type changed from %s to %s", p.name, oldType, config.Type)
	}
	return nil
}

// UpdateConfig updates provider configuration
func (p *BaseProvider) UpdateConfig(config types.ProviderConfig) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	oldType := p.config.Type
	p.config = config
	if p.logger != nil {
		p.logger.Printf("Provider %s updated from %s to %s", p.name, oldType, config.Type)
	}
}

// GetConfig returns the current provider configuration
func (p *BaseProvider) GetConfig() types.ProviderConfig {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.config
}

func (p *BaseProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	return []types.Model{}, nil
}

func (p *BaseProvider) GetDefaultModel() string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	if p.config.DefaultModel != "" {
		return p.config.DefaultModel
	}
	return "default-model"
}

// Authenticate handles API key authentication
func (p *BaseProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if authConfig.APIKey != "" {
		p.config.APIKey = authConfig.APIKey
		return nil
	}
	return fmt.Errorf("authentication not implemented")
}

func (p *BaseProvider) IsAuthenticated() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.config.APIKey != ""
}

func (p *BaseProvider) Logout(ctx context.Context) error {
	return nil
}

func (p *BaseProvider) SupportsToolCalling() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.config.SupportsToolCalling
}

func (p *BaseProvider) SupportsStreaming() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.config.SupportsStreaming
}

func (p *BaseProvider) SupportsResponsesAPI() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.config.SupportsResponsesAPI
}

// InvokeServerTool invokes a server tool (not implemented in base provider)
func (p *BaseProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, fmt.Errorf("tool invocation not implemented")
}

// GenerateChatCompletion generates a mock chat completion
func (p *BaseProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	return &MockStream{
		chunks: []types.ChatCompletionChunk{
			{Content: "Mock response from " + p.name, Done: true},
		},
	}, nil
}

func (p *BaseProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

func (p *BaseProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (p *BaseProvider) GetMetrics() types.ProviderMetrics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.metrics
}

// IncrementRequestCount increments the request counter
func (p *BaseProvider) IncrementRequestCount() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.metrics.RequestCount++
	p.metrics.LastRequestTime = time.Now()
}

// RecordSuccess records a successful API call
func (p *BaseProvider) RecordSuccess(latency time.Duration, tokensUsed int64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.metrics.SuccessCount++
	p.metrics.TotalLatency += latency
	p.metrics.TokensUsed += tokensUsed
	p.metrics.LastSuccessTime = time.Now()

	// Calculate average latency
	if p.metrics.SuccessCount > 0 {
		p.metrics.AverageLatency = p.metrics.TotalLatency / time.Duration(p.metrics.SuccessCount)
	}
}

// RecordError records a failed API call
func (p *BaseProvider) RecordError(err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.metrics.ErrorCount++
	p.metrics.LastErrorTime = time.Now()
	if err != nil {
		p.metrics.LastError = err.Error()
	}
}

// UpdateHealthStatus updates the health status
func (p *BaseProvider) UpdateHealthStatus(healthy bool, message string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.metrics.HealthStatus.Healthy = healthy
	p.metrics.HealthStatus.Message = message
	p.metrics.HealthStatus.LastChecked = time.Now()
}

// UpdateHealthStatusResponseTime updates the health status response time
func (p *BaseProvider) UpdateHealthStatusResponseTime(responseTime float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.metrics.HealthStatus.ResponseTime = responseTime
}

// LogRequest logs an HTTP request
func (p *BaseProvider) LogRequest(method, url string, headers map[string]string, body interface{}) {
	if p.logger == nil {
		return
	}
	p.logger.Printf("Provider %s - %s %s", p.name, method, url)
	for key, value := range headers {
		p.logger.Printf("  Header: %s: %s", key, value)
	}
	if body != nil {
		p.logger.Printf("  Body: %+v", body)
	}
}

// LogResponse logs detailed response information
func (p *BaseProvider) LogResponse(resp *http.Response, duration time.Duration) {
	if p.logger == nil {
		return
	}
	p.logger.Printf("Provider %s response in %v - Status: %d", p.name, duration, resp.StatusCode)
}

// MockStream implementation
type MockStream struct {
	chunks []types.ChatCompletionChunk
	index  int
}

func (ms *MockStream) Next() (types.ChatCompletionChunk, error) {
	if ms.index >= len(ms.chunks) {
		return types.ChatCompletionChunk{}, nil
	}
	chunk := ms.chunks[ms.index]
	ms.index++
	return chunk, nil
}

func (ms *MockStream) Close() error {
	ms.index = 0
	return nil
}

// BaseProviderStub wraps BaseProvider to implement Provider interface
type BaseProviderStub struct {
	*BaseProvider
}

func NewBaseProviderStub(name string, config types.ProviderConfig, client *http.Client, logger *log.Logger) *BaseProviderStub {
	base := NewBaseProvider(name, config, client, logger)
	return &BaseProviderStub{BaseProvider: base}
}

// Name returns the stub provider name
func (b *BaseProviderStub) Name() string             { return b.name }
func (b *BaseProviderStub) Type() types.ProviderType { return b.config.Type }
func (b *BaseProviderStub) Description() string      { return "Base provider stub" }

// GetModels returns models from the base provider
func (b *BaseProviderStub) GetModels(ctx context.Context) ([]types.Model, error) {
	return b.BaseProvider.GetModels(ctx)
}

func (b *BaseProviderStub) GetDefaultModel() string {
	return b.BaseProvider.GetDefaultModel()
}

func (b *BaseProviderStub) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return b.BaseProvider.Authenticate(ctx, authConfig)
}

func (b *BaseProviderStub) IsAuthenticated() bool {
	return b.BaseProvider.IsAuthenticated()
}

func (b *BaseProviderStub) Logout(ctx context.Context) error {
	return b.BaseProvider.Logout(ctx)
}

func (b *BaseProviderStub) Configure(config types.ProviderConfig) error {
	return b.BaseProvider.Configure(config)
}

func (b *BaseProviderStub) GetConfig() types.ProviderConfig {
	return b.BaseProvider.GetConfig()
}

func (b *BaseProviderStub) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	return b.BaseProvider.GenerateChatCompletion(ctx, options)
}

func (b *BaseProviderStub) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return b.BaseProvider.InvokeServerTool(ctx, toolName, params)
}

func (b *BaseProviderStub) SupportsToolCalling() bool {
	return b.BaseProvider.SupportsToolCalling()
}

func (b *BaseProviderStub) SupportsStreaming() bool {
	return b.BaseProvider.SupportsStreaming()
}

func (b *BaseProviderStub) SupportsResponsesAPI() bool {
	return b.BaseProvider.SupportsResponsesAPI()
}

func (b *BaseProviderStub) GetToolFormat() types.ToolFormat {
	return b.BaseProvider.GetToolFormat()
}

func (b *BaseProviderStub) HealthCheck(ctx context.Context) error {
	return b.BaseProvider.HealthCheck(ctx)
}

func (b *BaseProviderStub) GetMetrics() types.ProviderMetrics {
	return b.BaseProvider.GetMetrics()
}
