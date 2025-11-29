package extensions

// ExtensionConfigKey is the metadata key for per-request extension configuration.
// Request metadata should contain this key with a map of extension names to their configs.
const ExtensionConfigKey = "extension_config"

// GetExtensionConfig extracts configuration for a specific extension from request metadata.
// It returns the extension's configuration map and a boolean indicating if the extension is enabled.
//
// The function looks for configuration in metadata[ExtensionConfigKey][extensionName].
// If no configuration is found, the extension is considered enabled by default (enabled=true).
// If configuration exists with {"enabled": false}, the extension is disabled (enabled=false).
// If configuration exists with other settings, the extension is enabled with those settings.
//
// Example metadata structure:
//
//	{
//	  "extension_config": {
//	    "caching": {"enabled": false},
//	    "logging": {"enabled": true, "level": "debug"},
//	    "metrics": {"interval": 60}  // enabled=true implicitly
//	  }
//	}
//
// Returns:
//   - config: The extension's configuration map (may be nil if no config found)
//   - enabled: Whether the extension is enabled for this request
func GetExtensionConfig(metadata map[string]interface{}, extensionName string) (config map[string]interface{}, enabled bool) {
	// Default: extension is enabled
	enabled = true

	// Check if metadata exists
	if metadata == nil {
		return nil, enabled
	}

	// Get the extension_config map from metadata
	extensionConfigRaw, ok := metadata[ExtensionConfigKey]
	if !ok {
		// No extension config in metadata, use defaults
		return nil, enabled
	}

	// Type assert to map[string]interface{}
	extensionConfigMap, ok := extensionConfigRaw.(map[string]interface{})
	if !ok {
		// Invalid type for extension_config, use defaults
		return nil, enabled
	}

	// Get this specific extension's config
	extConfigRaw, ok := extensionConfigMap[extensionName]
	if !ok {
		// No config for this extension, use defaults
		return nil, enabled
	}

	// Type assert to map[string]interface{}
	extConfig, ok := extConfigRaw.(map[string]interface{})
	if !ok {
		// Invalid type for extension config, use defaults
		return nil, enabled
	}

	// Check if explicitly disabled
	if enabledVal, ok := extConfig["enabled"]; ok {
		if enabledBool, ok := enabledVal.(bool); ok {
			enabled = enabledBool
		}
	}

	return extConfig, enabled
}

// IsExtensionEnabled checks if an extension is enabled for this request.
// It's a convenience wrapper around GetExtensionConfig that only returns the enabled status.
//
// Default behavior: extensions are enabled unless explicitly disabled in metadata.
//
// Example usage:
//
//	if !IsExtensionEnabled(req.Metadata, "caching") {
//	    // Skip caching logic
//	    return nil
//	}
//
// Security Note: Security-critical extensions (e.g., authentication, authorization)
// should ignore disable requests and always execute their logic regardless of this setting.
func IsExtensionEnabled(metadata map[string]interface{}, extensionName string) bool {
	_, enabled := GetExtensionConfig(metadata, extensionName)
	return enabled
}
