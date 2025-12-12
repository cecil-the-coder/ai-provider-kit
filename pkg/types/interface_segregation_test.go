package types

import (
	"context"
	"fmt"
	"testing"
)

// MockProvider implements all interfaces for testing
type MockProvider struct {
	name         string
	providerType ProviderType
	description  string
	models       []Model
	isHealthy    bool
}

func (m *MockProvider) Name() string                                                  { return m.name }
func (m *MockProvider) Type() ProviderType                                            { return m.providerType }
func (m *MockProvider) Description() string                                           { return m.description }
func (m *MockProvider) GetModels(ctx context.Context) ([]Model, error)                { return m.models, nil }
func (m *MockProvider) GetDefaultModel() string                                       { return "default-model" }
func (m *MockProvider) Authenticate(ctx context.Context, authConfig AuthConfig) error { return nil }
func (m *MockProvider) IsAuthenticated() bool                                         { return true }
func (m *MockProvider) Logout(ctx context.Context) error                              { return nil }
func (m *MockProvider) Configure(config ProviderConfig) error                         { return nil }
func (m *MockProvider) GetConfig() ProviderConfig                                     { return ProviderConfig{} }
func (m *MockProvider) GenerateChatCompletion(ctx context.Context, options GenerateOptions) (ChatCompletionStream, error) {
	return &MockStream{}, nil
}
func (m *MockProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return "tool result", nil
}
func (m *MockProvider) SupportsToolCalling() bool  { return true }
func (m *MockProvider) SupportsStreaming() bool    { return true }
func (m *MockProvider) SupportsResponsesAPI() bool { return false }
func (m *MockProvider) GetToolFormat() ToolFormat  { return ToolFormatOpenAI }
func (m *MockProvider) HealthCheck(ctx context.Context) error {
	if !m.isHealthy {
		return fmt.Errorf("provider is unhealthy")
	}
	return nil
}
func (m *MockProvider) GetMetrics() ProviderMetrics { return ProviderMetrics{} }

type MockStream struct{}

func (m *MockStream) Next() (ChatCompletionChunk, error) {
	return ChatCompletionChunk{}, nil
}
func (m *MockStream) Close() error { return nil }

func TestInterfaceSegregation(t *testing.T) {
	mock := createMockProvider()
	ctx := context.Background()

	// Test individual interfaces
	testCoreProvider(t, mock)
	testModelProvider(t, mock, ctx)
	testAuthenticatedProvider(t, mock, ctx)
	testConfigurableProvider(t, mock)
	testChatProvider(t, mock, ctx)
	testToolCallingProvider(t, mock, ctx)
	testCapabilityProvider(t, mock)
	testHealthCheckProvider(t, mock, ctx)
	testFullProvider(t, mock, ctx)
}

// createMockProvider creates a mock provider for testing
func createMockProvider() *MockProvider {
	return &MockProvider{
		name:         "test-provider",
		providerType: ProviderTypeOpenAI,
		description:  "Test provider for interface segregation",
		models: []Model{
			{ID: "model-1", Name: "Model 1"},
			{ID: "model-2", Name: "Model 2"},
		},
		isHealthy: true,
	}
}

// testCoreProvider tests the CoreProvider interface
func testCoreProvider(t *testing.T, mock *MockProvider) {
	t.Run("CoreProvider", func(t *testing.T) {
		var core CoreProvider = mock
		if got := core.Name(); got != "test-provider" {
			t.Errorf("Name() = %q, want %q", got, "test-provider")
		}
		if got := core.Type(); got != ProviderTypeOpenAI {
			t.Errorf("Type() = %v, want %v", got, ProviderTypeOpenAI)
		}
		if got := core.Description(); got != "Test provider for interface segregation" {
			t.Errorf("Description() = %q, want %q", got, "Test provider for interface segregation")
		}
	})
}

// testModelProvider tests the ModelProvider interface
func testModelProvider(t *testing.T, mock *MockProvider, ctx context.Context) {
	t.Run("ModelProvider", func(t *testing.T) {
		var modelProvider ModelProvider = mock
		models, err := modelProvider.GetModels(ctx)
		if err != nil {
			t.Fatalf("GetModels() error = %v", err)
		}
		if len(models) != 2 {
			t.Errorf("GetModels() returned %d models, want 2", len(models))
		}
		if got := modelProvider.GetDefaultModel(); got != "default-model" {
			t.Errorf("GetDefaultModel() = %q, want %q", got, "default-model")
		}
	})
}

// testAuthenticatedProvider tests the AuthenticatedProvider interface
func testAuthenticatedProvider(t *testing.T, mock *MockProvider, ctx context.Context) {
	t.Run("AuthenticatedProvider", func(t *testing.T) {
		var authProvider AuthenticatedProvider = mock
		if err := authProvider.Authenticate(ctx, AuthConfig{}); err != nil {
			t.Errorf("Authenticate() error = %v", err)
		}
		if !authProvider.IsAuthenticated() {
			t.Error("IsAuthenticated() = false, want true")
		}
		if err := authProvider.Logout(ctx); err != nil {
			t.Errorf("Logout() error = %v", err)
		}
	})
}

// testConfigurableProvider tests the ConfigurableProvider interface
func testConfigurableProvider(t *testing.T, mock *MockProvider) {
	t.Run("ConfigurableProvider", func(t *testing.T) {
		var configProvider ConfigurableProvider = mock
		if err := configProvider.Configure(ProviderConfig{}); err != nil {
			t.Errorf("Configure() error = %v", err)
		}
		// Just test that GetConfig() doesn't panic
		_ = configProvider.GetConfig()
	})
}

// testChatProvider tests the ChatProvider interface
func testChatProvider(t *testing.T, mock *MockProvider, ctx context.Context) {
	t.Run("ChatProvider", func(t *testing.T) {
		var chatProvider ChatProvider = mock
		stream, err := chatProvider.GenerateChatCompletion(ctx, GenerateOptions{})
		if err != nil {
			t.Fatalf("GenerateChatCompletion() error = %v", err)
		}
		defer func() {
			_ = stream.Close()
		}()

		_, err = stream.Next()
		if err != nil {
			t.Errorf("Next() error = %v", err)
		}
	})
}

// testToolCallingProvider tests the ToolCallingProvider interface
func testToolCallingProvider(t *testing.T, mock *MockProvider, ctx context.Context) {
	t.Run("ToolCallingProvider", func(t *testing.T) {
		var toolProvider ToolCallingProvider = mock
		if !toolProvider.SupportsToolCalling() {
			t.Error("SupportsToolCalling() = false, want true")
		}
		if got := toolProvider.GetToolFormat(); got != ToolFormatOpenAI {
			t.Errorf("GetToolFormat() = %v, want %v", got, ToolFormatOpenAI)
		}

		result, err := toolProvider.InvokeServerTool(ctx, "test-tool", map[string]interface{}{"param": "value"})
		if err != nil {
			t.Errorf("InvokeServerTool() error = %v", err)
		}
		if result != "tool result" {
			t.Errorf("InvokeServerTool() = %v, want %v", result, "tool result")
		}
	})
}

// testCapabilityProvider tests the CapabilityProvider interface
func testCapabilityProvider(t *testing.T, mock *MockProvider) {
	t.Run("CapabilityProvider", func(t *testing.T) {
		var capabilityProvider CapabilityProvider = mock
		if !capabilityProvider.SupportsStreaming() {
			t.Error("SupportsStreaming() = false, want true")
		}
		if capabilityProvider.SupportsResponsesAPI() {
			t.Error("SupportsResponsesAPI() = true, want false")
		}
	})
}

// testHealthCheckProvider tests the HealthCheckProvider interface
func testHealthCheckProvider(t *testing.T, mock *MockProvider, ctx context.Context) {
	t.Run("HealthCheckProvider", func(t *testing.T) {
		var healthProvider HealthCheckProvider = mock
		if err := healthProvider.HealthCheck(ctx); err != nil {
			t.Errorf("HealthCheck() error = %v", err)
		}
		// Just test that GetMetrics() doesn't panic
		_ = healthProvider.GetMetrics()
	})
}

// testFullProvider tests the full Provider interface (composition)
func testFullProvider(t *testing.T, mock *MockProvider, ctx context.Context) {
	t.Run("FullProvider", func(t *testing.T) {
		var provider Provider = mock
		// Should be able to call any method from any interface
		if provider.Name() != "test-provider" {
			t.Errorf("Provider.Name() = %q, want %q", provider.Name(), "test-provider")
		}
		if !provider.SupportsToolCalling() {
			t.Error("Provider.SupportsToolCalling() = false, want true")
		}
		if err := provider.HealthCheck(ctx); err != nil {
			t.Errorf("Provider.HealthCheck() error = %v", err)
		}
	})
}

func TestModelDiscoveryService(t *testing.T) {
	service := NewModelDiscoveryService()

	mock := &MockProvider{
		name:         "test-provider",
		providerType: ProviderTypeOpenAI,
		description:  "Test provider",
		models: []Model{
			{ID: "model-1", Name: "Model 1"},
		},
	}

	service.AddProvider(mock)

	ctx := context.Background()
	models, err := service.GetAllModels(ctx)
	if err != nil {
		t.Fatalf("GetAllModels() error = %v", err)
	}

	if len(models) != 1 {
		t.Errorf("GetAllModels() returned %d provider models, want 1", len(models))
	}
}

func TestHealthMonitoringService(t *testing.T) {
	service := NewHealthMonitoringService()

	healthyMock := &MockProvider{
		name:         "healthy-provider",
		providerType: ProviderTypeOpenAI,
		description:  "Healthy provider",
		isHealthy:    true,
	}

	unhealthyMock := &MockProvider{
		name:         "unhealthy-provider",
		providerType: ProviderTypeAnthropic,
		description:  "Unhealthy provider",
		isHealthy:    false,
	}

	service.AddProvider(healthyMock)
	service.AddProvider(unhealthyMock)

	ctx := context.Background()
	results := service.CheckAllHealth(ctx)

	if len(results) != 2 {
		t.Errorf("CheckAllHealth() returned %d results, want 2", len(results))
	}

	if results["provider-0"] != nil {
		t.Errorf("Healthy provider returned error: %v", results["provider-0"])
	}

	if results["provider-1"] == nil {
		t.Error("Unhealthy provider should have returned an error")
	}
}
