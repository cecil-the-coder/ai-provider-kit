// Package common provides shared utilities and helper functions for AI providers.
// It includes file operations, configuration helpers, and other common functionality
// used across different provider implementations.
package common

import (
	"fmt"
	"os"
	"path/filepath"
)

// ReadFileContent reads the content of a file and returns it as a string.
// This is a shared utility used by providers to read context files and existing content.
// Returns the file content as a string and any error encountered.
func ReadFileContent(filename string) (string, error) {
	// Validate filename to prevent path traversal
	if filename == "" {
		return "", fmt.Errorf("filename cannot be empty")
	}
	// Clean the path to prevent directory traversal
	cleanFilename := filepath.Clean(filename)

	data, err := os.ReadFile(cleanFilename)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FilterContextFiles filters out the output file from context files to avoid duplication.
// This prevents the same file from being included both as context and as existing content.
//
// Parameters:
//   - contextFiles: List of context file paths
//   - outputFile: Path to the output file to exclude from context
//
// Returns:
//   - Filtered list of context files excluding the output file
func FilterContextFiles(contextFiles []string, outputFile string) []string {
	if outputFile == "" {
		return contextFiles
	}

	var filtered []string
	for _, file := range contextFiles {
		contextAbs := filepath.Clean(file)
		outputAbs := filepath.Clean(outputFile)

		if contextAbs != outputAbs {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

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
