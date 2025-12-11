package vertex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestVertexConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *VertexConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid bearer token config",
			config: &VertexConfig{
				ProjectID:   "test-project",
				Region:      "us-east5",
				AuthType:    AuthTypeBearerToken,
				BearerToken: "test-token",
			},
			wantErr: false,
		},
		{
			name: "valid ADC config",
			config: &VertexConfig{
				ProjectID: "test-project",
				Region:    "us-east5",
				AuthType:  AuthTypeApplicationDefault,
			},
			wantErr: false,
		},
		{
			name: "valid service account JSON config",
			config: &VertexConfig{
				ProjectID:          "test-project",
				Region:             "us-east5",
				AuthType:           AuthTypeServiceAccount,
				ServiceAccountJSON: `{"type": "service_account", "project_id": "test"}`,
			},
			wantErr: false,
		},
		{
			name: "missing project ID",
			config: &VertexConfig{
				Region:      "us-east5",
				AuthType:    AuthTypeBearerToken,
				BearerToken: "test-token",
			},
			wantErr: true,
			errMsg:  "project_id is required",
		},
		{
			name: "missing region",
			config: &VertexConfig{
				ProjectID:   "test-project",
				AuthType:    AuthTypeBearerToken,
				BearerToken: "test-token",
			},
			wantErr: true,
			errMsg:  "region is required",
		},
		{
			name: "missing auth type",
			config: &VertexConfig{
				ProjectID: "test-project",
				Region:    "us-east5",
			},
			wantErr: true,
			errMsg:  "auth_type is required",
		},
		{
			name: "invalid auth type",
			config: &VertexConfig{
				ProjectID: "test-project",
				Region:    "us-east5",
				AuthType:  "invalid",
			},
			wantErr: true,
			errMsg:  "invalid auth_type",
		},
		{
			name: "bearer token missing token",
			config: &VertexConfig{
				ProjectID: "test-project",
				Region:    "us-east5",
				AuthType:  AuthTypeBearerToken,
			},
			wantErr: true,
			errMsg:  "bearer_token is required",
		},
		{
			name: "service account missing credentials",
			config: &VertexConfig{
				ProjectID: "test-project",
				Region:    "us-east5",
				AuthType:  AuthTypeServiceAccount,
			},
			wantErr: true,
			errMsg:  "either service_account_file or service_account_json is required",
		},
		{
			name: "service account invalid JSON",
			config: &VertexConfig{
				ProjectID:          "test-project",
				Region:             "us-east5",
				AuthType:           AuthTypeServiceAccount,
				ServiceAccountJSON: "invalid json",
			},
			wantErr: true,
			errMsg:  "service_account_json is not valid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestVertexConfig_GetEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		config   *VertexConfig
		expected string
	}{
		{
			name: "default endpoint",
			config: &VertexConfig{
				Region: "us-east5",
			},
			expected: "https://us-east5-aiplatform.googleapis.com",
		},
		{
			name: "custom endpoint",
			config: &VertexConfig{
				Region:   "us-east5",
				Endpoint: "https://custom.endpoint.com",
			},
			expected: "https://custom.endpoint.com",
		},
		{
			name: "europe region",
			config: &VertexConfig{
				Region: "europe-west1",
			},
			expected: "https://europe-west1-aiplatform.googleapis.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetEndpoint()
			if got != tt.expected {
				t.Errorf("GetEndpoint() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestVertexConfig_GetModelVersion(t *testing.T) {
	tests := []struct {
		name            string
		config          *VertexConfig
		anthropicModel  string
		expectedVersion string
	}{
		{
			name:            "claude-3-5-sonnet with default mapping",
			config:          &VertexConfig{},
			anthropicModel:  "claude-3-5-sonnet-20241022",
			expectedVersion: "claude-3-5-sonnet-v2@20241022",
		},
		{
			name: "custom model mapping",
			config: &VertexConfig{
				ModelVersionMap: map[string]string{
					"claude-3-5-sonnet-20241022": "custom-version@20241022",
				},
			},
			anthropicModel:  "claude-3-5-sonnet-20241022",
			expectedVersion: "custom-version@20241022",
		},
		{
			name:            "claude-3-opus",
			config:          &VertexConfig{},
			anthropicModel:  "claude-3-opus-20240229",
			expectedVersion: "claude-3-opus@20240229",
		},
		{
			name:            "unknown model falls back to default",
			config:          &VertexConfig{},
			anthropicModel:  "claude-unknown-model",
			expectedVersion: "claude-unknown-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetModelVersion(tt.anthropicModel)
			if got != tt.expectedVersion {
				t.Errorf("GetModelVersion() = %v, want %v", got, tt.expectedVersion)
			}
		})
	}
}

func TestNewDefaultConfig(t *testing.T) {
	projectID := "test-project"
	region := "us-east5"

	config := NewDefaultConfig(projectID, region)

	if config.ProjectID != projectID {
		t.Errorf("ProjectID = %v, want %v", config.ProjectID, projectID)
	}
	if config.Region != region {
		t.Errorf("Region = %v, want %v", config.Region, region)
	}
	if config.AuthType != AuthTypeApplicationDefault {
		t.Errorf("AuthType = %v, want %v", config.AuthType, AuthTypeApplicationDefault)
	}
	if config.ModelVersionMap == nil {
		t.Error("ModelVersionMap should be initialized")
	}
}

func TestVertexConfig_FluentInterface(t *testing.T) {
	config := NewDefaultConfig("test-project", "us-east5")

	// Test bearer token
	config.WithBearerToken("test-token")
	if config.AuthType != AuthTypeBearerToken {
		t.Errorf("WithBearerToken() did not set AuthType correctly")
	}
	if config.BearerToken != "test-token" {
		t.Errorf("WithBearerToken() did not set BearerToken correctly")
	}

	// Test service account file
	config.WithServiceAccountFile("/path/to/file.json")
	if config.AuthType != AuthTypeServiceAccount {
		t.Errorf("WithServiceAccountFile() did not set AuthType correctly")
	}
	if config.ServiceAccountFile != "/path/to/file.json" {
		t.Errorf("WithServiceAccountFile() did not set ServiceAccountFile correctly")
	}

	// Test service account JSON
	jsonContent := `{"type": "service_account"}` //nolint:gosec // G101: test data, not real credentials
	config.WithServiceAccountJSON(jsonContent)
	if config.AuthType != AuthTypeServiceAccount {
		t.Errorf("WithServiceAccountJSON() did not set AuthType correctly")
	}
	if config.ServiceAccountJSON != jsonContent {
		t.Errorf("WithServiceAccountJSON() did not set ServiceAccountJSON correctly")
	}

	// Test ADC
	config.WithApplicationDefault()
	if config.AuthType != AuthTypeApplicationDefault {
		t.Errorf("WithApplicationDefault() did not set AuthType correctly")
	}
}

func TestVertexConfig_ServiceAccountFileValidation(t *testing.T) {
	// Create a temporary service account file
	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "valid.json")
	serviceAccountJSON := map[string]interface{}{
		"type":                        "service_account",
		"project_id":                  "test-project",
		"private_key_id":              "key-id",
		"private_key":                 "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----\n",
		"client_email":                "test@test-project.iam.gserviceaccount.com",
		"client_id":                   "123456789",
		"auth_uri":                    "https://accounts.google.com/o/oauth2/auth",
		"token_uri":                   "https://oauth2.googleapis.com/token",
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
	}
	jsonBytes, _ := json.Marshal(serviceAccountJSON)
	if err := os.WriteFile(validFile, jsonBytes, 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		config  *VertexConfig
		wantErr bool
	}{
		{
			name: "valid service account file",
			config: &VertexConfig{
				ProjectID:          "test-project",
				Region:             "us-east5",
				AuthType:           AuthTypeServiceAccount,
				ServiceAccountFile: validFile,
			},
			wantErr: false,
		},
		{
			name: "non-existent service account file",
			config: &VertexConfig{
				ProjectID:          "test-project",
				Region:             "us-east5",
				AuthType:           AuthTypeServiceAccount,
				ServiceAccountFile: "/non/existent/file.json",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) > 0 && len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
