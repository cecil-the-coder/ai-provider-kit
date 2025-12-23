package common

import (
	"os"
	"path/filepath"
	"testing"
)

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
				if err := os.WriteFile(path, content, 0600); err != nil {
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
				if err := os.WriteFile(path, content, 0600); err != nil {
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
				if err := os.WriteFile(path, []byte{}, 0600); err != nil {
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
