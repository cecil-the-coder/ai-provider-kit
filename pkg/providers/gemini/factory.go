package gemini

import (
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// RegisterGeminiFactory registers the Gemini provider with the factory
func RegisterGeminiFactory(factory types.ProviderFactory) {
	factory.RegisterProvider(types.ProviderTypeGemini, func(config types.ProviderConfig) types.Provider {
		return NewGeminiProvider(config)
	})
}

// CreateGeminiProvider creates a new Gemini provider instance
func CreateGeminiProvider(config types.ProviderConfig) (types.Provider, error) {
	if config.Type != types.ProviderTypeGemini {
		return nil, fmt.Errorf("invalid provider type for Gemini: %s", config.Type)
	}
	return NewGeminiProvider(config), nil
}
