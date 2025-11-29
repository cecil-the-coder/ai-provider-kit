package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileContent(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		setup       func() string
		expectError bool
		expected    string
	}{
		{
			name: "read valid file",
			setup: func() string {
				path := filepath.Join(tmpDir, "test.txt")
				content := "Hello, World!"
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			expectError: false,
			expected:    "Hello, World!",
		},
		{
			name: "empty filename",
			setup: func() string {
				return ""
			},
			expectError: true,
		},
		{
			name: "non-existent file",
			setup: func() string {
				return filepath.Join(tmpDir, "nonexistent.txt")
			},
			expectError: true,
		},
		{
			name: "read empty file",
			setup: func() string {
				path := filepath.Join(tmpDir, "empty.txt")
				if err := os.WriteFile(path, []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			expectError: false,
			expected:    "",
		},
		{
			name: "read file with special characters",
			setup: func() string {
				path := filepath.Join(tmpDir, "special.txt")
				content := "特殊字符\nемoji 🚀\ttabs"
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			expectError: false,
			expected:    "特殊字符\nемoji 🚀\ttabs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := tt.setup()
			content, err := ReadFileContent(filename)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if content != tt.expected {
					t.Errorf("got %q, expected %q", content, tt.expected)
				}
			}
		})
	}
}

func TestFilterContextFiles(t *testing.T) {
	tests := []struct {
		name         string
		contextFiles []string
		outputFile   string
		expected     []string
	}{
		{
			name:         "empty output file",
			contextFiles: []string{"file1.txt", "file2.txt"},
			outputFile:   "",
			expected:     []string{"file1.txt", "file2.txt"},
		},
		{
			name:         "filter out output file",
			contextFiles: []string{"file1.txt", "file2.txt", "output.txt"},
			outputFile:   "output.txt",
			expected:     []string{"file1.txt", "file2.txt"},
		},
		{
			name:         "output file not in context",
			contextFiles: []string{"file1.txt", "file2.txt"},
			outputFile:   "output.txt",
			expected:     []string{"file1.txt", "file2.txt"},
		},
		{
			name:         "empty context files",
			contextFiles: []string{},
			outputFile:   "output.txt",
			expected:     nil,
		},
		{
			name:         "filter with absolute paths",
			contextFiles: []string{"/tmp/file1.txt", "/tmp/file2.txt", "/tmp/output.txt"},
			outputFile:   "/tmp/output.txt",
			expected:     []string{"/tmp/file1.txt", "/tmp/file2.txt"},
		},
		{
			name:         "filter with relative paths",
			contextFiles: []string{"./file1.txt", "./file2.txt", "./output.txt"},
			outputFile:   "output.txt",
			expected:     []string{"./file1.txt", "./file2.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterContextFiles(tt.contextFiles, tt.outputFile)

			if len(result) != len(tt.expected) {
				t.Errorf("got %d files, expected %d", len(result), len(tt.expected))
				return
			}

			for i, file := range result {
				if file != tt.expected[i] {
					t.Errorf("at index %d: got %q, expected %q", i, file, tt.expected[i])
				}
			}
		})
	}
}

func TestReadConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		setup       func() string
		expectError bool
		expected    []byte
	}{
		{
			name: "read valid config",
			setup: func() string {
				path := filepath.Join(tmpDir, "config.json")
				content := []byte(`{"key": "value"}`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			expectError: false,
			expected:    []byte(`{"key": "value"}`),
		},
		{
			name: "empty config path",
			setup: func() string {
				return ""
			},
			expectError: true,
		},
		{
			name: "non-existent config",
			setup: func() string {
				return filepath.Join(tmpDir, "nonexistent.json")
			},
			expectError: true,
		},
		{
			name: "read YAML config",
			setup: func() string {
				path := filepath.Join(tmpDir, "config.yaml")
				content := []byte("key: value\n")
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			expectError: false,
			expected:    []byte("key: value\n"),
		},
		{
			name: "read empty config",
			setup: func() string {
				path := filepath.Join(tmpDir, "empty.json")
				if err := os.WriteFile(path, []byte{}, 0644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			expectError: false,
			expected:    []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := tt.setup()
			content, err := ReadConfigFile(configPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if string(content) != string(tt.expected) {
					t.Errorf("got %q, expected %q", string(content), string(tt.expected))
				}
			}
		})
	}
}
