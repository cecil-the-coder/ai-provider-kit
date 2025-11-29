// Package common provides shared utilities and helper functions for AI provider implementations.
package common

import "strings"

// CleanCodeResponse removes markdown code block formatting from responses.
// This utility function is commonly used across providers to clean up
// generated code responses that may include markdown formatting.
func CleanCodeResponse(content string) string {
	// Remove markdown code blocks if present
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")

	// Remove language identifiers (e.g., "go", "python", "javascript")
	// These typically appear on the first line without spaces and are short
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		// If the first line is a single word without spaces and is short (< 20 chars),
		// it's likely a language identifier
		if firstLine != "" && !strings.Contains(firstLine, " ") && len(firstLine) < 20 {
			content = strings.Join(lines[1:], "\n")
		}
	}

	return strings.TrimSpace(content)
}
