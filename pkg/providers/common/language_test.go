package common

import (
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		// Programming languages
		{"test.go", "go"},
		{"app.js", "javascript"},
		{"component.jsx", "javascript"},
		{"script.ts", "typescript"},
		{"component.tsx", "typescript"},
		{"main.py", "python"},
		{"Main.java", "java"},
		{"program.cpp", "cpp"},
		{"program.cc", "cpp"},
		{"program.cxx", "cpp"},
		{"hello.c", "c"},
		{"app.cs", "csharp"},
		{"index.php", "php"},
		{"script.rb", "ruby"},
		{"app.swift", "swift"},
		{"Main.kt", "kotlin"},
		{"lib.rs", "rust"},

		// Web technologies
		{"index.html", "html"},
		{"style.css", "css"},
		{"style.scss", "scss"},
		{"style.sass", "scss"},

		// Data formats
		{"config.json", "json"},
		{"data.xml", "xml"},
		{"settings.yaml", "yaml"},
		{"settings.yml", "yaml"},

		// Databases
		{"query.sql", "sql"},

		// Shell scripts
		{"script.sh", "bash"},
		{"script.bash", "bash"},
		{"script.ps1", "powershell"},

		// Documentation
		{"README.md", "markdown"},
		{"doc.md", "markdown"},

		// Special filenames without extensions
		{"Dockerfile", "dockerfile"},
		{"Makefile", "makefile"},
		{"readme", "markdown"},
		{"README", "markdown"},

		// Edge cases
		{"unknown.xyz", "text"},
		{"", "text"},
		{"noextension", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			actual := DetectLanguage(tt.filename)
			if actual != tt.expected {
				t.Errorf("DetectLanguage(%s) = %s, expected %s", tt.filename, actual, tt.expected)
			}
		})
	}
}

func TestDetectByFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"path/to/Dockerfile", "dockerfile"},
		{"somedir/Makefile", "makefile"},
		{"README", "markdown"},
		{"README.md", "markdown"},
		{"other.txt", ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := DetectLanguage(tt.filename)
			if tt.expected == "" && result == "text" {
				return // Empty expected means default "text"
			}
			if result != tt.expected && tt.expected != "" {
				t.Errorf("got %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestDetectByExtension(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"file.YAML", "yaml"},
		{"FILE.YML", "yaml"},
		{"script.SH", "bash"},
		{"test.GO", "go"},
		{"app.PY", "python"},
	}
	
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := DetectLanguage(tt.filename)
			if result != tt.expected {
				t.Errorf("got %q, expected %q", result, tt.expected)
			}
		})
	}
}
