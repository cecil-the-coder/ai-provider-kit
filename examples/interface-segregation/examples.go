// Package examples provides code examples demonstrating how to use the
// AI Provider Kit's segregated provider interfaces following the Interface
// Segregation Principle (ISP).
//
// NOTE: This file contains educational examples. The FlexibleMockProvider
// is a mock implementation for demonstration purposes only and should NOT
// be used in production.
package examples

import (
	"context"
	"fmt"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// This file demonstrates how to use the segregated provider interfaces
// for different use cases, following the Interface Segregation Principle.

// Re-export commonly used types for convenience
type (
	// Provider interfaces from pkg/types
	ModelProvider       = types.ModelProvider
	HealthCheckProvider = types.HealthCheckProvider
	ChatProvider        = types.ChatProvider
	ToolCallingProvider = types.ToolCallingProvider
	CoreProvider        = types.CoreProvider
	Provider            = types.Provider

	// Common types from pkg/types
	ProviderType     = types.ProviderType
	ProviderConfig   = types.ProviderConfig
	AuthConfig       = types.AuthConfig
	Model            = types.Model
	GenerateOptions  = types.GenerateOptions
	ChatCompletionStream = types.ChatCompletionStream
	ChatCompletionChunk  = types.ChatCompletionChunk
	ToolFormat       = types.ToolFormat
	ProviderMetrics  = types.ProviderMetrics
	ProviderInfo     = types.ProviderInfo
	HealthStatus     = types.HealthStatus
	Pricing          = types.Pricing
)

// ModelDiscoveryService only needs to discover models, so it depends only on ModelProvider
type ModelDiscoveryService struct {
	providers []ModelProvider
}

func NewModelDiscoveryService() *ModelDiscoveryService {
	return &ModelDiscoveryService{}
}

func (s *ModelDiscoveryService) AddProvider(provider ModelProvider) {
	s.providers = append(s.providers, provider)
}

func (s *ModelDiscoveryService) GetAllModels(ctx context.Context) (map[string][]Model, error) {
	result := make(map[string][]Model)

	for _, provider := range s.providers {
		models, err := provider.GetModels(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get models from provider: %w", err)
		}
		result[provider.GetDefaultModel()] = models
	}

	return result, nil
}

// HealthMonitoringService only needs to check health, so it depends only on HealthCheckProvider
type HealthMonitoringService struct {
	providers []HealthCheckProvider
}

func NewHealthMonitoringService() *HealthMonitoringService {
	return &HealthMonitoringService{}
}

func (s *HealthMonitoringService) AddProvider(provider HealthCheckProvider) {
	s.providers = append(s.providers, provider)
}

func (s *HealthMonitoringService) CheckAllHealth(ctx context.Context) map[string]error {
	result := make(map[string]error)

	for i, provider := range s.providers {
		providerName := fmt.Sprintf("provider-%d", i)
		err := provider.HealthCheck(ctx)
		result[providerName] = err
	}

	return result
}

// ChatService only needs to generate completions, so it depends only on ChatProvider
type ChatService struct {
	provider ChatProvider
}

func NewChatService(provider ChatProvider) *ChatService {
	return &ChatService{provider: provider}
}

func (s *ChatService) GenerateResponse(ctx context.Context, options GenerateOptions) (ChatCompletionStream, error) {
	stream, err := s.provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to generate response: %w", err)
	}

	return stream, nil
}

// ToolExecutionService only needs to call tools, so it depends only on ToolCallingProvider
type ToolExecutionService struct {
	provider ToolCallingProvider
}

func NewToolExecutionService(provider ToolCallingProvider) *ToolExecutionService {
	return &ToolExecutionService{provider: provider}
}

func (s *ToolExecutionService) ExecuteTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	if !s.provider.SupportsToolCalling() {
		return nil, fmt.Errorf("provider does not support tool calling")
	}

	result, err := s.provider.InvokeServerTool(ctx, toolName, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tool %s: %w", toolName, err)
	}

	return result, nil
}

// ProviderInfoService only needs basic provider info, so it depends only on CoreProvider
type ProviderInfoService struct {
	providers []CoreProvider
}

func NewProviderInfoService() *ProviderInfoService {
	return &ProviderInfoService{}
}

func (s *ProviderInfoService) AddProvider(provider CoreProvider) {
	s.providers = append(s.providers, provider)
}

func (s *ProviderInfoService) ListProviders() []ProviderInfo {
	infos := make([]ProviderInfo, 0, len(s.providers))

	for _, provider := range s.providers {
		// Note: This would need to be adapted to only use CoreProvider methods
		// in a real implementation, as ProviderInfo requires more than just name/type/description
		infos = append(infos, ProviderInfo{
			Name: provider.Name(),
			Type: provider.Type(),
		})
	}

	return infos
}

// MultiPurposeService demonstrates how to compose multiple interfaces
type MultiPurposeService struct {
	provider Provider // Uses the full interface for comprehensive functionality
}

func NewMultiPurposeService(provider Provider) *MultiPurposeService {
	return &MultiPurposeService{provider: provider}
}

func (s *MultiPurposeService) GetProviderInfo() string {
	return fmt.Sprintf("%s (%s): %s", s.provider.Name(), s.provider.Type(), s.provider.Description())
}

func (s *MultiPurposeService) SupportsTools() bool {
	return s.provider.SupportsToolCalling()
}

func (s *MultiPurposeService) GetHealth(ctx context.Context) error {
	return s.provider.HealthCheck(ctx)
}

// FlexibleProviderFactory demonstrates how to create providers with different interface requirements.
//
// IMPORTANT: This is an example implementation for educational purposes only.
// It creates mock providers that return simulated data and should NOT be used in production.
// For production use, integrate with the actual provider factory implementations.
type FlexibleProviderFactory struct{}

// CreateModelProvider creates a provider that implements only the ModelProvider interface.
// This allows clients to depend only on model discovery functionality.
func (f *FlexibleProviderFactory) CreateModelProvider(providerType types.ProviderType, config types.ProviderConfig) (types.ModelProvider, error) {
	// Validate provider type
	if !f.isProviderTypeSupported(providerType) {
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}

	// Create a mock provider that implements ModelProvider interface
	return &FlexibleMockProvider{
		name:         config.Name,
		providerType: providerType,
		description:  config.Description,
	}, nil
}

// CreateChatProvider creates a provider that implements only the ChatProvider interface.
// This allows clients to depend only on chat completion functionality.
func (f *FlexibleProviderFactory) CreateChatProvider(providerType types.ProviderType, config types.ProviderConfig) (types.ChatProvider, error) {
	// Validate provider type
	if !f.isProviderTypeSupported(providerType) {
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}

	// Create a mock provider that implements ChatProvider interface
	return &FlexibleMockProvider{
		name:         config.Name,
		providerType: providerType,
		description:  config.Description,
	}, nil
}

// CreateHealthCheckProvider creates a provider that implements only the HealthCheckProvider interface.
// This allows clients to depend only on health checking functionality.
func (f *FlexibleProviderFactory) CreateHealthCheckProvider(providerType types.ProviderType, config types.ProviderConfig) (types.HealthCheckProvider, error) {
	// Validate provider type
	if !f.isProviderTypeSupported(providerType) {
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}

	// Create a mock provider that implements HealthCheckProvider interface
	return &FlexibleMockProvider{
		name:         config.Name,
		providerType: providerType,
		description:  config.Description,
	}, nil
}

// isProviderTypeSupported checks if the provider type is supported by this example factory
func (f *FlexibleProviderFactory) isProviderTypeSupported(providerType types.ProviderType) bool {
	supportedTypes := []types.ProviderType{
		types.ProviderTypeOpenAI,
		types.ProviderTypeAnthropic,
		types.ProviderTypeGemini,
		types.ProviderTypeQwen,
		types.ProviderTypeCerebras,
		types.ProviderTypeOpenRouter,
	}

	for _, supported := range supportedTypes {
		if supported == providerType {
			return true
		}
	}
	return false
}

// FlexibleMockProvider is a mock implementation that satisfies all provider interfaces
// for demonstration purposes in the FlexibleProviderFactory example.
//
// IMPORTANT: This is an example implementation that returns simulated data.
// Do NOT use this in production code.
type FlexibleMockProvider struct {
	name         string
	providerType types.ProviderType
	description  string
}

// Name returns the mock provider name
func (p *FlexibleMockProvider) Name() string {
	if p.name == "" {
		return string(p.providerType) + "-mock"
	}
	return p.name
}

func (p *FlexibleMockProvider) Type() types.ProviderType {
	return p.providerType
}

func (p *FlexibleMockProvider) Description() string {
	if p.description == "" {
		return "Mock provider for " + string(p.providerType)
	}
	return p.description
}

// GetModels returns mock models for the provider
func (p *FlexibleMockProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	providerStr := string(p.providerType)
	return []types.Model{
		{
			ID:                   providerStr + "-mock-model-1",
			Name:                 providerStr + " Mock Model 1",
			Provider:             p.providerType,
			Description:          "Mock model for " + providerStr,
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
		{
			ID:                   providerStr + "-mock-model-2",
			Name:                 providerStr + " Mock Model 2",
			Provider:             p.providerType,
			Description:          "Large mock model for " + providerStr,
			MaxTokens:            8192,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "completion", "analysis"},
			Pricing: types.Pricing{
				InputTokenPrice:  0.002,
				OutputTokenPrice: 0.004,
				Unit:             "token",
			},
		},
	}, nil
}

func (p *FlexibleMockProvider) GetDefaultModel() string {
	return string(p.providerType) + "-mock-default"
}

// Authenticate authenticates the mock provider (always succeeds)
func (p *FlexibleMockProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	// Mock implementation - always succeeds
	return nil
}

func (p *FlexibleMockProvider) IsAuthenticated() bool {
	// Mock implementation - always authenticated
	return true
}

func (p *FlexibleMockProvider) Logout(ctx context.Context) error {
	// Mock implementation - always succeeds
	return nil
}

// Configure configures the mock provider with the given config
func (p *FlexibleMockProvider) Configure(config types.ProviderConfig) error {
	// Mock implementation - store the config
	p.name = config.Name
	p.description = config.Description
	return nil
}

func (p *FlexibleMockProvider) GetConfig() types.ProviderConfig {
	return types.ProviderConfig{
		Type:        p.providerType,
		Name:        p.name,
		Description: p.description,
	}
}

// GenerateChatCompletion generates a mock chat completion stream
func (p *FlexibleMockProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	// Return a mock stream
	return &FlexibleMockStream{
		providerType: p.providerType,
		completed:    false,
	}, nil
}

// SupportsToolCalling returns true for mock provider
func (p *FlexibleMockProvider) SupportsToolCalling() bool {
	return true
}

func (p *FlexibleMockProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

func (p *FlexibleMockProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	// Mock implementation - return a simulated tool response
	return map[string]interface{}{
		"tool":   toolName,
		"result": fmt.Sprintf("Mock result from %s provider", p.providerType),
		"params": params,
	}, nil
}

// SupportsStreaming indicates if the provider supports streaming responses
func (p *FlexibleMockProvider) SupportsStreaming() bool {
	return true
}

// SupportsResponsesAPI indicates if the provider supports the responses API
func (p *FlexibleMockProvider) SupportsResponsesAPI() bool {
	return false
}

// HealthCheck performs a health check on the provider
func (p *FlexibleMockProvider) HealthCheck(ctx context.Context) error {
	// Mock implementation - always healthy
	return nil
}

func (p *FlexibleMockProvider) GetMetrics() types.ProviderMetrics {
	return types.ProviderMetrics{
		RequestCount:    0,
		SuccessCount:    0,
		ErrorCount:      0,
		TotalLatency:    0,
		AverageLatency:  0,
		LastRequestTime: time.Time{},
		LastSuccessTime: time.Time{},
		LastErrorTime:   time.Time{},
		LastError:       "",
		TokensUsed:      0,
		HealthStatus: types.HealthStatus{
			Healthy:      true,
			LastChecked:  time.Now(),
			Message:      "Mock provider is healthy",
			ResponseTime: 0,
			StatusCode:   200,
		},
	}
}

// FlexibleMockStream implements ChatCompletionStream for mock responses
type FlexibleMockStream struct {
	providerType types.ProviderType
	completed    bool
}

func (m *FlexibleMockStream) Next() (types.ChatCompletionChunk, error) {
	if m.completed {
		return types.ChatCompletionChunk{}, nil // End of stream
	}

	m.completed = true
	return types.ChatCompletionChunk{
		ID:      "mock-chunk-" + string(m.providerType),
		Object:  "chat.completion.chunk",
		Created: 0, // Mock timestamp
		Model:   string(m.providerType) + "-mock-model",
		Choices: []types.ChatChoice{
			{
				Index: 0,
				Delta: types.ChatMessage{
					Role:    "assistant",
					Content: fmt.Sprintf("This is a mock response from the %s provider.", m.providerType),
				},
				FinishReason: "stop",
			},
		},
		Usage: types.Usage{
			PromptTokens:     10,
			CompletionTokens: 15,
			TotalTokens:      25,
		},
		Content: fmt.Sprintf("This is a mock response from the %s provider.", m.providerType),
		Done:    true,
	}, nil
}

func (m *FlexibleMockStream) Close() error {
	return nil
}
