package extensions

import (
	"context"
	"reflect"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ExtensionMeta defines the core metadata required for all extensions.
// All extensions must implement this interface at minimum.
type ExtensionMeta interface {
	Name() string
	Version() string
	Description() string
}

// Initializable defines extensions that need initialization and cleanup.
// Implement this interface if your extension needs setup/teardown.
type Initializable interface {
	Initialize(config map[string]interface{}) error
	Shutdown(ctx context.Context) error
}

// BeforeGenerateHook defines extensions that need to intercept requests before generation.
// Implement this to modify or validate requests before they're sent to providers.
type BeforeGenerateHook interface {
	BeforeGenerate(ctx context.Context, req *GenerateRequest) error
}

// AfterGenerateHook defines extensions that need to process responses after generation.
// Implement this to modify, log, or analyze responses from providers.
type AfterGenerateHook interface {
	AfterGenerate(ctx context.Context, req *GenerateRequest, resp *GenerateResponse) error
}

// ProviderErrorHandler defines extensions that handle provider errors.
// Implement this to add custom error handling, logging, or retry logic.
type ProviderErrorHandler interface {
	OnProviderError(ctx context.Context, provider types.Provider, err error) error
}

// ProviderSelectionHook defines extensions that react to provider selection.
// Implement this to log, validate, or modify behavior based on selected provider.
type ProviderSelectionHook interface {
	OnProviderSelected(ctx context.Context, provider types.Provider) error
}

// RouteProvider defines extensions that provide HTTP routes.
// Implement this to add custom API endpoints to the backend.
type RouteProvider interface {
	RegisterRoutes(registrar RouteRegistrar) error
}

// DependencyDeclarer defines extensions that depend on other extensions.
// Implement this to ensure your extension initializes after its dependencies.
type DependencyDeclarer interface {
	Dependencies() []string
}

// PriorityProvider defines extensions that specify execution priority.
// Implement this to control the order in which your extension's hooks are called.
// Lower priority values execute first.
type PriorityProvider interface {
	Priority() int
}

// GetCapabilities returns a list of capability names that the given extension implements.
// This is useful for debugging and introspection.
func GetCapabilities(ext interface{}) []string {
	if ext == nil {
		return nil
	}

	capabilities := make([]string, 0)

	// Check core interface
	if _, ok := ext.(ExtensionMeta); ok {
		capabilities = append(capabilities, "ExtensionMeta")
	}

	// Check optional capability interfaces
	if _, ok := ext.(Initializable); ok {
		capabilities = append(capabilities, "Initializable")
	}

	if _, ok := ext.(BeforeGenerateHook); ok {
		capabilities = append(capabilities, "BeforeGenerateHook")
	}

	if _, ok := ext.(AfterGenerateHook); ok {
		capabilities = append(capabilities, "AfterGenerateHook")
	}

	if _, ok := ext.(ProviderErrorHandler); ok {
		capabilities = append(capabilities, "ProviderErrorHandler")
	}

	if _, ok := ext.(ProviderSelectionHook); ok {
		capabilities = append(capabilities, "ProviderSelectionHook")
	}

	if _, ok := ext.(RouteProvider); ok {
		capabilities = append(capabilities, "RouteProvider")
	}

	if _, ok := ext.(DependencyDeclarer); ok {
		capabilities = append(capabilities, "DependencyDeclarer")
	}

	if _, ok := ext.(PriorityProvider); ok {
		capabilities = append(capabilities, "PriorityProvider")
	}

	// Check for the monolithic Extension interface (backward compatibility)
	if _, ok := ext.(Extension); ok {
		capabilities = append(capabilities, "Extension")
	}

	return capabilities
}

// HasCapability checks if an extension implements a specific capability.
// The capability name should match the interface name (e.g., "BeforeGenerateHook").
func HasCapability(ext interface{}, capability string) bool {
	if ext == nil {
		return false
	}

	switch capability {
	case "ExtensionMeta":
		_, ok := ext.(ExtensionMeta)
		return ok
	case "Initializable":
		_, ok := ext.(Initializable)
		return ok
	case "BeforeGenerateHook":
		_, ok := ext.(BeforeGenerateHook)
		return ok
	case "AfterGenerateHook":
		_, ok := ext.(AfterGenerateHook)
		return ok
	case "ProviderErrorHandler":
		_, ok := ext.(ProviderErrorHandler)
		return ok
	case "ProviderSelectionHook":
		_, ok := ext.(ProviderSelectionHook)
		return ok
	case "RouteProvider":
		_, ok := ext.(RouteProvider)
		return ok
	case "DependencyDeclarer":
		_, ok := ext.(DependencyDeclarer)
		return ok
	case "PriorityProvider":
		_, ok := ext.(PriorityProvider)
		return ok
	case "Extension":
		_, ok := ext.(Extension)
		return ok
	default:
		return false
	}
}

// GetExtensionType returns the concrete type name of an extension.
// This is useful for debugging and logging.
func GetExtensionType(ext interface{}) string {
	if ext == nil {
		return "<nil>"
	}
	return reflect.TypeOf(ext).String()
}

// GetPriority returns the priority of an extension.
// It checks for PriorityProvider capability first, then falls back to Extension interface.
// If neither is implemented, returns the default priority (PriorityTransform = 500).
func GetPriority(ext interface{}) int {
	if ext == nil {
		return PriorityTransform
	}

	// Check for PriorityProvider capability (new style)
	if provider, ok := ext.(PriorityProvider); ok {
		return provider.Priority()
	}

	// Check for Extension interface (old style, for backward compatibility)
	if extension, ok := ext.(Extension); ok {
		return extension.Priority()
	}

	// Default priority
	return PriorityTransform
}
