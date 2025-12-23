package copilot

import (
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// RegisterCopilotFactory registers the Copilot provider with the factory
func RegisterCopilotFactory(factory types.ProviderFactory) {
	factory.RegisterProvider(types.ProviderTypeCopilot, func(config types.ProviderConfig) types.Provider {
		return NewCopilotProvider(config)
	})
}

// CreateCopilotProvider creates a new Copilot provider instance
func CreateCopilotProvider(config types.ProviderConfig) (types.Provider, error) {
	if config.Type != types.ProviderTypeCopilot {
		return nil, types.NewInvalidRequestError(types.ProviderTypeCopilot, fmt.Sprintf("invalid provider type for Copilot: %s", config.Type)).
			WithOperation("validate_options")
	}
	return NewCopilotProvider(config), nil
}
