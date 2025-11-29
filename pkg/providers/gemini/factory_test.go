package gemini

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestCreateGeminiProvider(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeGemini,
		APIKey: "test-key",
	}

	provider, err := CreateGeminiProvider(config)
	if err != nil {
		t.Fatalf("CreateGeminiProvider returned error: %v", err)
	}

	if provider == nil {
		t.Fatal("CreateGeminiProvider returned nil")
	}

	geminiProvider, ok := provider.(*GeminiProvider)
	if !ok {
		t.Fatal("CreateGeminiProvider did not return a *GeminiProvider")
	}

	if geminiProvider.Type() != types.ProviderTypeGemini {
		t.Errorf("Expected type %s, got %s", types.ProviderTypeGemini, geminiProvider.Type())
	}
}

func TestCreateGeminiProvider_InvalidType(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeAnthropic, // Wrong type
	}

	_, err := CreateGeminiProvider(config)
	if err == nil {
		t.Error("Expected error for invalid provider type")
	}
}
