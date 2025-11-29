package extensions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetExtensionConfig_NoMetadata tests behavior when metadata is nil
func TestGetExtensionConfig_NoMetadata(t *testing.T) {
	config, enabled := GetExtensionConfig(nil, "test-extension")

	assert.Nil(t, config, "Config should be nil when metadata is nil")
	assert.True(t, enabled, "Extension should be enabled by default when no metadata")
}

// TestGetExtensionConfig_NoExtensionConfig tests behavior when extension_config key is missing
func TestGetExtensionConfig_NoExtensionConfig(t *testing.T) {
	metadata := map[string]interface{}{
		"user_id":    "user123",
		"request_id": "req456",
	}

	config, enabled := GetExtensionConfig(metadata, "test-extension")

	assert.Nil(t, config, "Config should be nil when extension_config key is missing")
	assert.True(t, enabled, "Extension should be enabled by default when extension_config is missing")
}

// TestGetExtensionConfig_InvalidExtensionConfigType tests behavior when extension_config has wrong type
func TestGetExtensionConfig_InvalidExtensionConfigType(t *testing.T) {
	metadata := map[string]interface{}{
		ExtensionConfigKey: "invalid-type", // Should be map[string]interface{}
	}

	config, enabled := GetExtensionConfig(metadata, "test-extension")

	assert.Nil(t, config, "Config should be nil when extension_config has invalid type")
	assert.True(t, enabled, "Extension should be enabled by default when extension_config has invalid type")
}

// TestGetExtensionConfig_NoConfigForExtension tests behavior when specific extension has no config
func TestGetExtensionConfig_NoConfigForExtension(t *testing.T) {
	metadata := map[string]interface{}{
		ExtensionConfigKey: map[string]interface{}{
			"other-extension": map[string]interface{}{"key": "value"},
		},
	}

	config, enabled := GetExtensionConfig(metadata, "test-extension")

	assert.Nil(t, config, "Config should be nil when extension has no config")
	assert.True(t, enabled, "Extension should be enabled by default when no config for specific extension")
}

// TestGetExtensionConfig_InvalidExtensionConfigTypeForSpecificExtension tests behavior when extension config has wrong type
func TestGetExtensionConfig_InvalidExtensionConfigTypeForSpecificExtension(t *testing.T) {
	metadata := map[string]interface{}{
		ExtensionConfigKey: map[string]interface{}{
			"test-extension": "invalid-type", // Should be map[string]interface{}
		},
	}

	config, enabled := GetExtensionConfig(metadata, "test-extension")

	assert.Nil(t, config, "Config should be nil when extension config has invalid type")
	assert.True(t, enabled, "Extension should be enabled by default when extension config has invalid type")
}

// TestGetExtensionConfig_ExtensionExplicitlyDisabled tests explicitly disabled extension
func TestGetExtensionConfig_ExtensionExplicitlyDisabled(t *testing.T) {
	metadata := map[string]interface{}{
		ExtensionConfigKey: map[string]interface{}{
			"caching": map[string]interface{}{
				"enabled": false,
			},
		},
	}

	config, enabled := GetExtensionConfig(metadata, "caching")

	assert.NotNil(t, config, "Config should not be nil when extension config exists")
	assert.False(t, enabled, "Extension should be disabled when explicitly set to false")
	assert.Equal(t, false, config["enabled"])
}

// TestGetExtensionConfig_ExtensionExplicitlyEnabled tests explicitly enabled extension
func TestGetExtensionConfig_ExtensionExplicitlyEnabled(t *testing.T) {
	metadata := map[string]interface{}{
		ExtensionConfigKey: map[string]interface{}{
			"logging": map[string]interface{}{
				"enabled": true,
				"level":   "debug",
			},
		},
	}

	config, enabled := GetExtensionConfig(metadata, "logging")

	assert.NotNil(t, config, "Config should not be nil when extension config exists")
	assert.True(t, enabled, "Extension should be enabled when explicitly set to true")
	assert.Equal(t, true, config["enabled"])
	assert.Equal(t, "debug", config["level"])
}

// TestGetExtensionConfig_ExtensionWithConfigNoEnabledField tests extension with config but no enabled field
func TestGetExtensionConfig_ExtensionWithConfigNoEnabledField(t *testing.T) {
	metadata := map[string]interface{}{
		ExtensionConfigKey: map[string]interface{}{
			"metrics": map[string]interface{}{
				"interval": 60,
				"port":     9090,
			},
		},
	}

	config, enabled := GetExtensionConfig(metadata, "metrics")

	assert.NotNil(t, config, "Config should not be nil when extension config exists")
	assert.True(t, enabled, "Extension should be enabled by default when enabled field is missing")
	assert.Equal(t, 60, config["interval"])
	assert.Equal(t, 9090, config["port"])
}

// TestGetExtensionConfig_EnabledFieldWrongType tests when enabled field has wrong type
func TestGetExtensionConfig_EnabledFieldWrongType(t *testing.T) {
	metadata := map[string]interface{}{
		ExtensionConfigKey: map[string]interface{}{
			"test-extension": map[string]interface{}{
				"enabled": "yes", // Should be bool
				"key":     "value",
			},
		},
	}

	config, enabled := GetExtensionConfig(metadata, "test-extension")

	assert.NotNil(t, config, "Config should not be nil when extension config exists")
	assert.True(t, enabled, "Extension should be enabled by default when enabled field has wrong type")
	assert.Equal(t, "value", config["key"])
}

// TestGetExtensionConfig_ComplexMetadata tests realistic metadata with multiple extensions
func TestGetExtensionConfig_ComplexMetadata(t *testing.T) {
	metadata := map[string]interface{}{
		"user_id":    "user123",
		"request_id": "req456",
		ExtensionConfigKey: map[string]interface{}{
			"caching": map[string]interface{}{
				"enabled": false,
			},
			"logging": map[string]interface{}{
				"enabled": true,
				"level":   "debug",
			},
			"metrics": map[string]interface{}{
				"interval": 60,
			},
		},
	}

	// Test caching (disabled)
	cachingConfig, cachingEnabled := GetExtensionConfig(metadata, "caching")
	assert.NotNil(t, cachingConfig)
	assert.False(t, cachingEnabled)

	// Test logging (enabled with config)
	loggingConfig, loggingEnabled := GetExtensionConfig(metadata, "logging")
	assert.NotNil(t, loggingConfig)
	assert.True(t, loggingEnabled)
	assert.Equal(t, "debug", loggingConfig["level"])

	// Test metrics (enabled implicitly)
	metricsConfig, metricsEnabled := GetExtensionConfig(metadata, "metrics")
	assert.NotNil(t, metricsConfig)
	assert.True(t, metricsEnabled)
	assert.Equal(t, 60, metricsConfig["interval"])

	// Test non-existent extension (enabled by default)
	unknownConfig, unknownEnabled := GetExtensionConfig(metadata, "unknown")
	assert.Nil(t, unknownConfig)
	assert.True(t, unknownEnabled)
}

// TestIsExtensionEnabled_DefaultBehavior tests IsExtensionEnabled default behavior
func TestIsExtensionEnabled_DefaultBehavior(t *testing.T) {
	t.Run("nil metadata", func(t *testing.T) {
		enabled := IsExtensionEnabled(nil, "test-extension")
		assert.True(t, enabled, "Extension should be enabled by default with nil metadata")
	})

	t.Run("empty metadata", func(t *testing.T) {
		enabled := IsExtensionEnabled(map[string]interface{}{}, "test-extension")
		assert.True(t, enabled, "Extension should be enabled by default with empty metadata")
	})

	t.Run("metadata without extension_config", func(t *testing.T) {
		metadata := map[string]interface{}{
			"user_id": "user123",
		}
		enabled := IsExtensionEnabled(metadata, "test-extension")
		assert.True(t, enabled, "Extension should be enabled by default without extension_config")
	})

	t.Run("extension_config without specific extension", func(t *testing.T) {
		metadata := map[string]interface{}{
			ExtensionConfigKey: map[string]interface{}{
				"other-extension": map[string]interface{}{"key": "value"},
			},
		}
		enabled := IsExtensionEnabled(metadata, "test-extension")
		assert.True(t, enabled, "Extension should be enabled by default when not in config")
	})
}

// TestIsExtensionEnabled_ExplicitlyDisabled tests IsExtensionEnabled when disabled
func TestIsExtensionEnabled_ExplicitlyDisabled(t *testing.T) {
	metadata := map[string]interface{}{
		ExtensionConfigKey: map[string]interface{}{
			"caching": map[string]interface{}{
				"enabled": false,
			},
		},
	}

	enabled := IsExtensionEnabled(metadata, "caching")
	assert.False(t, enabled, "Extension should be disabled when explicitly set to false")
}

// TestIsExtensionEnabled_ExplicitlyEnabled tests IsExtensionEnabled when enabled
func TestIsExtensionEnabled_ExplicitlyEnabled(t *testing.T) {
	metadata := map[string]interface{}{
		ExtensionConfigKey: map[string]interface{}{
			"logging": map[string]interface{}{
				"enabled": true,
				"level":   "debug",
			},
		},
	}

	enabled := IsExtensionEnabled(metadata, "logging")
	assert.True(t, enabled, "Extension should be enabled when explicitly set to true")
}

// TestIsExtensionEnabled_ImplicitlyEnabled tests IsExtensionEnabled when enabled implicitly
func TestIsExtensionEnabled_ImplicitlyEnabled(t *testing.T) {
	metadata := map[string]interface{}{
		ExtensionConfigKey: map[string]interface{}{
			"metrics": map[string]interface{}{
				"interval": 60,
				// No "enabled" field, should default to true
			},
		},
	}

	enabled := IsExtensionEnabled(metadata, "metrics")
	assert.True(t, enabled, "Extension should be enabled when enabled field is missing")
}

// TestExtensionConfigKey_Constant tests the ExtensionConfigKey constant
func TestExtensionConfigKey_Constant(t *testing.T) {
	assert.Equal(t, "extension_config", ExtensionConfigKey, "ExtensionConfigKey should be 'extension_config'")
}

// TestGetExtensionConfig_RealWorldScenarios tests realistic usage scenarios
func TestGetExtensionConfig_RealWorldScenarios(t *testing.T) {
	t.Run("disable caching for specific request", func(t *testing.T) {
		metadata := map[string]interface{}{
			ExtensionConfigKey: map[string]interface{}{
				"caching": map[string]interface{}{
					"enabled": false,
				},
			},
		}

		if !IsExtensionEnabled(metadata, "caching") {
			// Skip caching logic
			t.Log("Caching disabled, skipping cache lookup")
		}

		assert.False(t, IsExtensionEnabled(metadata, "caching"))
	})

	t.Run("custom logging level for request", func(t *testing.T) {
		metadata := map[string]interface{}{
			ExtensionConfigKey: map[string]interface{}{
				"logging": map[string]interface{}{
					"enabled": true,
					"level":   "debug",
				},
			},
		}

		config, enabled := GetExtensionConfig(metadata, "logging")
		if enabled {
			level := config["level"].(string)
			t.Logf("Using custom logging level: %s", level)
			assert.Equal(t, "debug", level)
		}

		assert.True(t, enabled)
	})

	t.Run("custom metrics interval", func(t *testing.T) {
		metadata := map[string]interface{}{
			ExtensionConfigKey: map[string]interface{}{
				"metrics": map[string]interface{}{
					"interval": 30,
					"enabled":  true,
				},
			},
		}

		config, enabled := GetExtensionConfig(metadata, "metrics")
		if enabled {
			interval := config["interval"].(int)
			t.Logf("Using custom metrics interval: %d", interval)
			assert.Equal(t, 30, interval)
		}

		assert.True(t, enabled)
	})

	t.Run("security extension always enabled", func(t *testing.T) {
		// Even if someone tries to disable a security extension, it should ignore the request
		metadata := map[string]interface{}{
			ExtensionConfigKey: map[string]interface{}{
				"authentication": map[string]interface{}{
					"enabled": false, // Attempt to disable
				},
			},
		}

		// Security-critical extensions should ignore the enabled flag
		config, enabled := GetExtensionConfig(metadata, "authentication")

		// In a real implementation, security extensions would check but ignore the disable request
		// For this test, we just verify that we can detect the attempt
		assert.NotNil(t, config)
		assert.False(t, enabled) // This is what GetExtensionConfig returns

		// But in real code, the extension would do:
		// if extensionName == "authentication" {
		//     // Ignore the enabled flag, always run
		//     enabled = true
		// }
		t.Log("Security extensions should ignore disable requests in their implementation")
	})
}

// TestGetExtensionConfig_EdgeCases tests edge cases
func TestGetExtensionConfig_EdgeCases(t *testing.T) {
	t.Run("empty extension name", func(t *testing.T) {
		metadata := map[string]interface{}{
			ExtensionConfigKey: map[string]interface{}{
				"": map[string]interface{}{
					"enabled": false,
				},
			},
		}

		config, enabled := GetExtensionConfig(metadata, "")
		assert.NotNil(t, config)
		assert.False(t, enabled)
	})

	t.Run("extension name with special characters", func(t *testing.T) {
		metadata := map[string]interface{}{
			ExtensionConfigKey: map[string]interface{}{
				"my-ext.v2": map[string]interface{}{
					"enabled": true,
				},
			},
		}

		config, enabled := GetExtensionConfig(metadata, "my-ext.v2")
		assert.NotNil(t, config)
		assert.True(t, enabled)
	})

	t.Run("nested config values", func(t *testing.T) {
		metadata := map[string]interface{}{
			ExtensionConfigKey: map[string]interface{}{
				"complex": map[string]interface{}{
					"enabled": true,
					"database": map[string]interface{}{
						"host": "localhost",
						"port": 5432,
					},
					"features": []string{"feature1", "feature2"},
				},
			},
		}

		config, enabled := GetExtensionConfig(metadata, "complex")
		assert.NotNil(t, config)
		assert.True(t, enabled)

		database, ok := config["database"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "localhost", database["host"])

		features, ok := config["features"].([]string)
		assert.True(t, ok)
		assert.Len(t, features, 2)
	})
}
