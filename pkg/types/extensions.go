package types

import (
	"fmt"
	"sync"
)

// DefaultExtensionRegistry implements ExtensionRegistry
type DefaultExtensionRegistry struct {
	extensions map[ProviderType]CoreProviderExtension
	mutex      sync.RWMutex
}

// NewExtensionRegistry creates a new extension registry
func NewExtensionRegistry() ExtensionRegistry {
	return &DefaultExtensionRegistry{
		extensions: make(map[ProviderType]CoreProviderExtension),
	}
}

// Register registers an extension for a provider type
func (r *DefaultExtensionRegistry) Register(providerType ProviderType, extension CoreProviderExtension) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.extensions[providerType]; exists {
		return fmt.Errorf("extension already registered for provider type: %s", providerType)
	}

	r.extensions[providerType] = extension
	return nil
}

// Get gets an extension for a provider type
func (r *DefaultExtensionRegistry) Get(providerType ProviderType) (CoreProviderExtension, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	extension, exists := r.extensions[providerType]
	if !exists {
		return nil, fmt.Errorf("no extension registered for provider type: %s", providerType)
	}

	return extension, nil
}

// List returns all registered extensions
func (r *DefaultExtensionRegistry) List() map[ProviderType]CoreProviderExtension {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[ProviderType]CoreProviderExtension)
	for providerType, extension := range r.extensions {
		result[providerType] = extension
	}

	return result
}

// Has checks if a provider type has a registered extension
func (r *DefaultExtensionRegistry) Has(providerType ProviderType) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	_, exists := r.extensions[providerType]
	return exists
}

// BaseExtension provides common functionality for provider extensions
type BaseExtension struct {
	name         string
	version      string
	description  string
	capabilities []string
}

// NewBaseExtension creates a new base extension
func NewBaseExtension(name, version, description string, capabilities []string) *BaseExtension {
	return &BaseExtension{
		name:         name,
		version:      version,
		description:  description,
		capabilities: capabilities,
	}
}

// Name returns the extension name
func (e *BaseExtension) Name() string {
	return e.name
}

// Version returns the extension version
func (e *BaseExtension) Version() string {
	return e.version
}

// Description returns the extension description
func (e *BaseExtension) Description() string {
	return e.description
}

// GetCapabilities returns the extension capabilities
func (e *BaseExtension) GetCapabilities() []string {
	result := make([]string, len(e.capabilities))
	copy(result, e.capabilities)
	return result
}

// ValidateOptions performs basic validation common to all providers
func (e *BaseExtension) ValidateOptions(options map[string]interface{}) error {
	// Base validation - can be overridden by specific implementations
	return nil
}

// CoreAPI represents the standardized core API that all providers should implement
type CoreAPI struct {
	registry ExtensionRegistry
}

// NewCoreAPI creates a new core API instance
func NewCoreAPI() *CoreAPI {
	return &CoreAPI{
		registry: NewExtensionRegistry(),
	}
}

// RegisterExtension registers a provider extension
func (api *CoreAPI) RegisterExtension(providerType ProviderType, extension CoreProviderExtension) error {
	return api.registry.Register(providerType, extension)
}

// GetExtension gets a provider extension
func (api *CoreAPI) GetExtension(providerType ProviderType) (CoreProviderExtension, error) {
	return api.registry.Get(providerType)
}

// HasExtension checks if a provider has an extension
func (api *CoreAPI) HasExtension(providerType ProviderType) bool {
	return api.registry.Has(providerType)
}

// ListExtensions lists all registered extensions
func (api *CoreAPI) ListExtensions() map[ProviderType]CoreProviderExtension {
	return api.registry.List()
}

// ConvertToProvider converts a standard request to provider-specific format
func (api *CoreAPI) ConvertToProvider(providerType ProviderType, request StandardRequest) (interface{}, error) {
	extension, err := api.registry.Get(providerType)
	if err != nil {
		// Return the standard request as-is if no extension is registered
		// This allows for basic functionality without extensions
		return request, nil
	}

	return extension.StandardToProvider(request)
}

// ConvertFromProvider converts a provider-specific response to standard format
func (api *CoreAPI) ConvertFromProvider(providerType ProviderType, response interface{}) (*StandardResponse, error) {
	extension, err := api.registry.Get(providerType)
	if err != nil {
		// Try to convert directly if no extension is registered
		if standardResp, ok := response.(*StandardResponse); ok {
			return standardResp, nil
		}
		return nil, fmt.Errorf("no extension registered for provider type %s and response is not in standard format", providerType)
	}

	return extension.ProviderToStandard(response)
}

// ConvertChunkFromProvider converts a provider-specific chunk to standard format
func (api *CoreAPI) ConvertChunkFromProvider(providerType ProviderType, chunk interface{}) (*StandardStreamChunk, error) {
	extension, err := api.registry.Get(providerType)
	if err != nil {
		// Try to convert directly if no extension is registered
		if standardChunk, ok := chunk.(*StandardStreamChunk); ok {
			return standardChunk, nil
		}
		return nil, fmt.Errorf("no extension registered for provider type %s and chunk is not in standard format", providerType)
	}

	return extension.ProviderToStandardChunk(chunk)
}

// ValidateProviderOptions validates provider-specific options
func (api *CoreAPI) ValidateProviderOptions(providerType ProviderType, options map[string]interface{}) error {
	extension, err := api.registry.Get(providerType)
	if err != nil {
		// No validation needed if no extension is registered
		return nil
	}

	return extension.ValidateOptions(options)
}

// GetProviderCapabilities gets the capabilities of a provider
func (api *CoreAPI) GetProviderCapabilities(providerType ProviderType) []string {
	extension, err := api.registry.Get(providerType)
	if err != nil {
		// Return basic capabilities if no extension is registered
		return []string{"chat", "streaming"}
	}

	return extension.GetCapabilities()
}

// Global core API instance
var defaultCoreAPI = NewCoreAPI()

// GetDefaultCoreAPI returns the default core API instance
func GetDefaultCoreAPI() *CoreAPI {
	return defaultCoreAPI
}

// RegisterDefaultExtension registers an extension with the default core API
func RegisterDefaultExtension(providerType ProviderType, extension CoreProviderExtension) error {
	return defaultCoreAPI.RegisterExtension(providerType, extension)
}

// GetDefaultExtension gets an extension from the default core API
func GetDefaultExtension(providerType ProviderType) (CoreProviderExtension, error) {
	return defaultCoreAPI.GetExtension(providerType)
}

// HasDefaultExtension checks if the default core API has an extension
func HasDefaultExtension(providerType ProviderType) bool {
	return defaultCoreAPI.HasExtension(providerType)
}
