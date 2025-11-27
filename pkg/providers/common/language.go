package common

import (
	"path/filepath"
	"strings"
)

// DetectLanguage determines the programming language based on file extension or special filenames.
// This is a shared utility used across all providers to ensure consistent language detection.
// Returns the language name as a string, defaulting to "text" if the language cannot be determined.
func DetectLanguage(filename string) string {
	if filename == "" {
		return "text"
	}

	// Check for special filenames without extensions
	if language := detectByFilename(filename); language != "" {
		return language
	}

	// Check file extensions
	return detectByExtension(filename)
}

// detectByFilename checks for special filenames without extensions
func detectByFilename(filename string) string {
	base := strings.ToLower(filepath.Base(filename))
	specialFiles := map[string]string{
		"dockerfile": "dockerfile",
		"makefile":   "makefile",
		"readme":     "markdown",
		"readme.md":  "markdown",
	}

	if language, exists := specialFiles[base]; exists {
		return language
	}
	return ""
}

// detectByExtension checks file extensions and returns the corresponding language
func detectByExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	// Group extensions by language for easier maintenance
	extensionMap := map[string][]string{
		"javascript": {".js", ".jsx"},
		"typescript": {".ts", ".tsx"},
		"cpp":        {".cpp", ".cc", ".cxx"},
		"scss":       {".scss", ".sass"},
		"yaml":       {".yaml", ".yml"},
		"bash":       {".sh", ".bash"},
	}

	// Check grouped extensions first
	for language, extensions := range extensionMap {
		for _, extension := range extensions {
			if ext == extension {
				return language
			}
		}
	}

	// Check single extensions
	singleExtensions := map[string]string{
		".go":    "go",
		".py":    "python",
		".java":  "java",
		".c":     "c",
		".cs":    "csharp",
		".php":   "php",
		".rb":    "ruby",
		".swift": "swift",
		".kt":    "kotlin",
		".rs":    "rust",
		".html":  "html",
		".css":   "css",
		".json":  "json",
		".xml":   "xml",
		".sql":   "sql",
		".ps1":   "powershell",
		".md":    "markdown",
	}

	if language, exists := singleExtensions[ext]; exists {
		return language
	}

	return "text"
}
