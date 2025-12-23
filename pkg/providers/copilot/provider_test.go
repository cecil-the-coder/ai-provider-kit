package copilot

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestNewCopilotProvider tests basic provider creation
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

// TestCreateCopilotProvider tests factory creation
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
