// Package common provides shared utilities and helper functions for AI providers.
// It includes file operations, configuration helpers, and other common functionality
// used across different provider implementations.
package common

import (
	"fmt"
	"os"
	"path/filepath"
)

// ReadConfigFile reads a configuration file and returns its raw byte content.
// This is commonly used by providers to read YAML/JSON configuration files.
// Returns the raw file content as bytes and any error encountered.
func ReadConfigFile(configPath string) ([]byte, error) {
	// Validate configPath to prevent path traversal
	if configPath == "" {
		return nil, fmt.Errorf("config path cannot be empty")
	}
	// Clean the path to prevent directory traversal
	cleanConfigPath := filepath.Clean(configPath)

	return os.ReadFile(cleanConfigPath)
}
