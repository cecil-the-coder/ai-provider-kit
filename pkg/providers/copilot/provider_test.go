package copilot

import (
	"context"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewCopilotProvider(t *testing.T) {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeCopilot,
		Name:         "test-copilot",
		DefaultModel: "gpt-4o",
		ProviderConfig: map[string]interface{}{
			"account_type": "individual",
		},
	}

	provider := NewCopilotProvider(config)

	if provider == nil {
		t.Fatal("expected provider to be created")
	}

	if provider.Name() != "Copilot" {
		t.Errorf("expected name 'Copilot', got '%s'", provider.Name())
	}

	if provider.Type() != types.ProviderTypeCopilot {
		t.Errorf("expected type '%s', got '%s'", types.ProviderTypeCopilot, provider.Type())
	}
}

func TestCopilotProvider_GetDefaultModel(t *testing.T) {
	tests := []struct {
		name          string
		config        types.ProviderConfig
		expectedModel string
	}{
		{
			name: "default model",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
			},
			expectedModel: copilotDefaultModel,
		},
		{
			name: "custom model",
			config: types.ProviderConfig{
				Type:         types.ProviderTypeCopilot,
				DefaultModel: "gpt-4o-mini",
			},
			expectedModel: "gpt-4o-mini",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCopilotProvider(tt.config)
			model := provider.GetDefaultModel()
			if model != tt.expectedModel {
				t.Errorf("expected model '%s', got '%s'", tt.expectedModel, model)
			}
		})
	}
}

func TestCopilotProvider_GetBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		accountType string
		expectedURL string
	}{
		{
			name:        "individual account",
			accountType: AccountTypeIndividual,
			expectedURL: CopilotBaseURL,
		},
		{
			name:        "business account",
			accountType: AccountTypeBusiness,
			expectedURL: CopilotBusinessBaseURL,
		},
		{
			name:        "enterprise account",
			accountType: AccountTypeEnterprise,
			expectedURL: CopilotEnterpriseBaseURL,
		},
		{
			name:        "unknown account defaults to individual",
			accountType: "unknown",
			expectedURL: CopilotBaseURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
				ProviderConfig: map[string]interface{}{
					"account_type": tt.accountType,
				},
			}
			provider := NewCopilotProvider(config)
			url := provider.GetBaseURL()
			if url != tt.expectedURL {
				t.Errorf("expected URL '%s', got '%s'", tt.expectedURL, url)
			}
		})
	}
}

func TestCopilotProvider_Supports(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	}
	provider := NewCopilotProvider(config)

	if !provider.SupportsToolCalling() {
		t.Error("expected provider to support tool calling")
	}

	if !provider.SupportsStreaming() {
		t.Error("expected provider to support streaming")
	}

	if provider.SupportsResponsesAPI() {
		t.Error("expected provider to not support Responses API")
	}

	if provider.GetToolFormat() != types.ToolFormatOpenAI {
		t.Errorf("expected tool format '%s', got '%s'", types.ToolFormatOpenAI, provider.GetToolFormat())
	}
}

func TestCopilotProvider_IsAuthenticated(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	}
	provider := NewCopilotProvider(config)

	// Initially not authenticated
	if provider.IsAuthenticated() {
		t.Error("expected provider to not be authenticated initially")
	}

	// After setting GitHub token (but no Copilot token)
	provider.tokenMutex.Lock()
	provider.githubToken = "test-github-token"
	provider.tokenMutex.Unlock()

	if provider.IsAuthenticated() {
		t.Error("expected provider to not be authenticated with only GitHub token")
	}

	// After setting Copilot token
	provider.tokenMutex.Lock()
	provider.copilotToken = "test-copilot-token"
	provider.copilotTokenExpiry = time.Now().Add(24 * time.Hour) // Set expiry 24 hours from now
	provider.tokenMutex.Unlock()

	if !provider.IsAuthenticated() {
		t.Error("expected provider to be authenticated with Copilot token")
	}
}

func TestConvertTools(t *testing.T) {
	inputTools := []types.Tool{
		{
			Name:        "test_function",
			Description: "A test function",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"param1": map[string]string{
						"type":        "string",
						"description": "A parameter",
					},
				},
			},
		},
	}

	converted := ConvertTools(inputTools)

	if len(converted) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(converted))
	}

	if converted[0].Type != "function" {
		t.Errorf("expected type 'function', got '%s'", converted[0].Type)
	}

	if converted[0].Function.Name != "test_function" {
		t.Errorf("expected name 'test_function', got '%s'", converted[0].Function.Name)
	}
}

func TestConvertToolChoice(t *testing.T) {
	tests := []struct {
		name          string
		toolChoice    *types.ToolChoice
		expectedValue interface{}
	}{
		{
			name:          "nil returns auto",
			toolChoice:    nil,
			expectedValue: "auto",
		},
		{
			name: "none",
			toolChoice: &types.ToolChoice{Mode: types.ToolChoiceNone},
			expectedValue: "none",
		},
		{
			name: "auto",
			toolChoice: &types.ToolChoice{Mode: types.ToolChoiceAuto},
			expectedValue: "auto",
		},
		{
			name: "required",
			toolChoice: &types.ToolChoice{Mode: types.ToolChoiceRequired},
			expectedValue: "required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToolChoice(tt.toolChoice)
			if result != tt.expectedValue {
				t.Errorf("expected %v, got %v", tt.expectedValue, result)
			}
		})
	}
}

func TestCreateCopilotProvider(t *testing.T) {
	tests := []struct {
		name        string
		config      types.ProviderConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
			},
			expectError: false,
		},
		{
			name: "invalid type",
			config: types.ProviderConfig{
				Type: types.ProviderTypeOpenAI,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := CreateCopilotProvider(tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if provider != nil {
					t.Error("expected nil provider on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if provider == nil {
					t.Error("expected provider, got nil")
				}
			}
		})
	}
}

func TestCopilotProvider_Configure(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
		ProviderConfig: map[string]interface{}{
			"display_name": "Test Copilot",
			"account_type": "business",
		},
	}

	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	err := provider.Configure(config)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if provider.displayName != "Test Copilot" {
		t.Errorf("expected display name 'Test Copilot', got '%s'", provider.displayName)
	}

	if provider.config.AccountType != "business" {
		t.Errorf("expected account type 'business', got '%s'", provider.config.AccountType)
	}
}

func TestToolCallBuilder(t *testing.T) {
	builder := NewToolCallBuilder()

	builder.AddToolCall("call-1", "test_func", `{"arg":"value"}`)

	toolCalls := builder.Build()

	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].ID != "call-1" {
		t.Errorf("expected ID 'call-1', got '%s'", toolCalls[0].ID)
	}

	if toolCalls[0].Function.Name != "test_func" {
		t.Errorf("expected name 'test_func', got '%s'", toolCalls[0].Function.Name)
	}
}

func TestValidateToolDefinition(t *testing.T) {
	tests := []struct {
		name        string
		tool        types.Tool
		expectError bool
	}{
		{
			name: "valid tool",
			tool: types.Tool{
				Name: "test",
				InputSchema: map[string]interface{}{
					"type": "object",
				},
			},
			expectError: false,
		},
		{
			name: "missing name",
			tool: types.Tool{
				Name: "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolDefinition(tt.tool)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCopilotProvider_GetModels(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	// GetModels should return fallback models when not authenticated
	ctx := context.Background()
	models, err := provider.GetModels(ctx)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(models) == 0 {
		t.Error("expected at least one model")
	}

	// Check for expected models
	modelMap := make(map[string]types.Model)
	for _, m := range models {
		modelMap[m.ID] = m
	}

	if _, ok := modelMap["gpt-4o"]; !ok {
		t.Error("expected gpt-4o model")
	}

	if _, ok := modelMap["gpt-4o-mini"]; !ok {
		t.Error("expected gpt-4o-mini model")
	}
}
