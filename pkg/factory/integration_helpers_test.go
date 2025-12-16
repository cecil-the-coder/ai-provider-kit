package factory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// AdvancedMockProvider is a sophisticated mock for integration testing
type AdvancedMockProvider struct {
	name         string
	providerType types.ProviderType
	config       types.ProviderConfig
	metrics      types.ProviderMetrics
	mutex        sync.RWMutex
}

func NewAdvancedMockProvider(name string, providerType types.ProviderType, config types.ProviderConfig) *AdvancedMockProvider {
	return &AdvancedMockProvider{
		name:         name,
		providerType: providerType,
		config:       config,
		metrics: types.ProviderMetrics{
			RequestCount:    0,
			SuccessCount:    0,
			ErrorCount:      0,
			TotalLatency:    0,
			AverageLatency:  0,
			LastRequestTime: time.Now(),
			TokensUsed:      0,
			HealthStatus: types.HealthStatus{
				Healthy:      true,
				LastChecked:  time.Now(),
				Message:      "Mock provider is healthy",
				ResponseTime: 10.0,
				StatusCode:   200,
			},
		},
	}
}

func (p *AdvancedMockProvider) Name() string {
	return p.name
}

func (p *AdvancedMockProvider) Type() types.ProviderType {
	return p.providerType
}

func (p *AdvancedMockProvider) Description() string {
	return fmt.Sprintf("Advanced mock provider: %s (%s)", p.name, p.providerType)
}

func (p *AdvancedMockProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	p.updateMetrics("models", true, 0)

	return []types.Model{
		{
			ID:                   fmt.Sprintf("%s-model-1", p.providerType),
			Name:                 fmt.Sprintf("%s Default Model", p.providerType),
			Provider:             p.providerType,
			Description:          "Default model for mock provider",
			MaxTokens:            4096,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "completion"},
			Pricing: types.Pricing{
				InputTokenPrice:  0.001,
				OutputTokenPrice: 0.002,
				Unit:             "token",
			},
		},
	}, nil
}

func (p *AdvancedMockProvider) GetDefaultModel() string {
	return fmt.Sprintf("%s-default", p.providerType)
}

func (p *AdvancedMockProvider) SupportsToolCalling() bool {
	return true
}

func (p *AdvancedMockProvider) SupportsStreaming() bool {
	return true
}

func (p *AdvancedMockProvider) SupportsResponsesAPI() bool {
	return false
}

func (p *AdvancedMockProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

func (p *AdvancedMockProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Simulate authentication logic
	if authConfig.APIKey == "" {
		return fmt.Errorf("API key required for authentication")
	}

	p.config.APIKey = authConfig.APIKey
	p.config.BaseURL = authConfig.BaseURL
	p.config.DefaultModel = authConfig.DefaultModel

	return nil
}

func (p *AdvancedMockProvider) IsAuthenticated() bool {
	return p.config.APIKey != ""
}

func (p *AdvancedMockProvider) Logout(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.config.APIKey = ""
	return nil
}

func (p *AdvancedMockProvider) Configure(config types.ProviderConfig) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if config.Type != p.providerType {
		return fmt.Errorf("invalid provider type: expected %s, got %s", p.providerType, config.Type)
	}

	p.config = config
	return nil
}

func (p *AdvancedMockProvider) GetConfig() types.ProviderConfig {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.config
}

func (p *AdvancedMockProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		p.updateMetrics("completion", true, latency)
	}()

	// Simulate processing
	responseContent := fmt.Sprintf("Mock response from %s provider for: %s",
		p.providerType, options.Prompt)

	if len(options.Messages) > 0 {
		for _, msg := range options.Messages {
			if msg.Role == "user" {
				responseContent = fmt.Sprintf("Mock response to: %s", msg.Content)
				break
			}
		}
	}

	// Create mock stream
	chunks := []types.ChatCompletionChunk{
		{
			ID:      fmt.Sprintf("chunk-%d", time.Now().Unix()),
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   p.GetDefaultModel(),
			Choices: []types.ChatChoice{
				{
					Index: 0,
					Delta: types.ChatMessage{
						Role:    "assistant",
						Content: responseContent,
					},
					FinishReason: "stop",
				},
			},
			Usage: types.Usage{
				PromptTokens:     10,
				CompletionTokens: len(responseContent) / 4, // Rough estimate
				TotalTokens:      10 + len(responseContent)/4,
			},
			Content: responseContent,
			Done:    true,
		},
	}

	return &AdvancedMockStream{chunks: chunks}, nil
}

func (p *AdvancedMockProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	p.updateMetrics("tool", true, 0)

	return map[string]interface{}{
		"tool":   toolName,
		"params": params,
		"result": fmt.Sprintf("Mock result from %s provider", p.providerType),
	}, nil
}

func (p *AdvancedMockProvider) HealthCheck(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.metrics.HealthStatus.LastChecked = time.Now()
	p.metrics.HealthStatus.ResponseTime = 5.0 + float64(time.Now().UnixNano()%1000)/100 // Simulate variance

	if p.config.APIKey == "" {
		p.metrics.HealthStatus.Healthy = false
		p.metrics.HealthStatus.Message = "Not authenticated"
		p.metrics.HealthStatus.StatusCode = 401
		return fmt.Errorf("provider not authenticated")
	}

	p.metrics.HealthStatus.Healthy = true
	p.metrics.HealthStatus.Message = "Provider is healthy"
	p.metrics.HealthStatus.StatusCode = 200

	return nil
}

func (p *AdvancedMockProvider) GetMetrics() types.ProviderMetrics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.metrics
}

func (p *AdvancedMockProvider) updateMetrics(operation string, success bool, latency time.Duration) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	now := time.Now()
	p.metrics.RequestCount++
	p.metrics.TotalLatency += latency
	p.metrics.AverageLatency = p.metrics.TotalLatency / time.Duration(p.metrics.RequestCount)
	p.metrics.LastRequestTime = now

	if success {
		p.metrics.SuccessCount++
		p.metrics.LastSuccessTime = now
	} else {
		p.metrics.ErrorCount++
		p.metrics.LastErrorTime = now
		p.metrics.LastError = fmt.Sprintf("Error in %s operation", operation)
	}

	// Simulate token usage
	p.metrics.TokensUsed += 10 // Base tokens per request
}

// AdvancedMockStream implements ChatCompletionStream with more features
type AdvancedMockStream struct {
	chunks []types.ChatCompletionChunk
	index  int
}

func (s *AdvancedMockStream) Next() (types.ChatCompletionChunk, error) {
	if s.index >= len(s.chunks) {
		return types.ChatCompletionChunk{}, nil
	}

	chunk := s.chunks[s.index]
	s.index++
	return chunk, nil
}

func (s *AdvancedMockStream) Close() error {
	s.index = 0
	return nil
}

// registerMockProvidersForIntegrationTests registers mock versions of all default providers
// to replace real API calls in integration tests
func registerMockProvidersForIntegrationTests(factory *DefaultProviderFactory) {
	// Register mock providers for all default types
	providerTypes := []types.ProviderType{
		types.ProviderTypeOpenAI,
		types.ProviderTypeAnthropic,
		types.ProviderTypeGemini,
		types.ProviderTypeQwen,
		types.ProviderTypeCerebras,
		types.ProviderTypeOpenRouter,
		types.ProviderTypeLMStudio,
		types.ProviderTypeLlamaCpp,
		types.ProviderTypeOllama,
	}

	for _, providerType := range providerTypes {
		factory.RegisterProvider(providerType, func(config types.ProviderConfig) types.Provider {
			return NewAdvancedMockProvider(config.Name, providerType, config)
		})
	}
}
