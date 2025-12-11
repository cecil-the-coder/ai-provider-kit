// Package vertex provides Google Vertex AI integration for Anthropic Claude models.
// It implements middleware to transform requests between Anthropic API format and Vertex AI format.
package vertex

import (
	"encoding/json"
	"fmt"
	"os"
)

// AuthType represents the type of GCP authentication
type AuthType string

const (
	// AuthTypeBearerToken uses a static bearer token for authentication
	AuthTypeBearerToken AuthType = "bearer_token"
	// AuthTypeServiceAccount uses a service account JSON key file
	AuthTypeServiceAccount AuthType = "service_account"
	// AuthTypeApplicationDefault uses Application Default Credentials (ADC)
	AuthTypeApplicationDefault AuthType = "adc"
)

// VertexConfig holds configuration for Google Vertex AI integration
type VertexConfig struct {
	// ProjectID is the GCP project ID
	ProjectID string `json:"project_id"`

	// Region is the GCP region (e.g., "us-east5", "europe-west1")
	Region string `json:"region"`

	// AuthType specifies the authentication method
	AuthType AuthType `json:"auth_type"`

	// BearerToken is used when AuthType is AuthTypeBearerToken
	BearerToken string `json:"bearer_token,omitempty"`

	// ServiceAccountFile is the path to the service account JSON key file
	// Used when AuthType is AuthTypeServiceAccount
	ServiceAccountFile string `json:"service_account_file,omitempty"`

	// ServiceAccountJSON is the raw JSON content of a service account key
	// Used as an alternative to ServiceAccountFile
	ServiceAccountJSON string `json:"service_account_json,omitempty"`

	// ModelVersionMap allows custom mapping of Anthropic model IDs to Vertex AI versions
	// Example: {"claude-3-5-sonnet-20241022": "claude-3-5-sonnet@20241022"}
	ModelVersionMap map[string]string `json:"model_version_map,omitempty"`

	// Endpoint is the custom Vertex AI endpoint (optional)
	// If not set, will use default: https://{region}-aiplatform.googleapis.com
	Endpoint string `json:"endpoint,omitempty"`
}

// Validate checks if the configuration is valid
func (c *VertexConfig) Validate() error {
	if c.ProjectID == "" {
		return fmt.Errorf("project_id is required")
	}

	if c.Region == "" {
		return fmt.Errorf("region is required")
	}

	// Validate auth configuration based on type
	switch c.AuthType {
	case AuthTypeBearerToken:
		if c.BearerToken == "" {
			return fmt.Errorf("bearer_token is required when auth_type is bearer_token")
		}
	case AuthTypeServiceAccount:
		if c.ServiceAccountFile == "" && c.ServiceAccountJSON == "" {
			return fmt.Errorf("either service_account_file or service_account_json is required when auth_type is service_account")
		}
		if c.ServiceAccountFile != "" {
			// Check if file exists
			if _, err := os.Stat(c.ServiceAccountFile); os.IsNotExist(err) {
				return fmt.Errorf("service_account_file does not exist: %s", c.ServiceAccountFile)
			}
		}
		if c.ServiceAccountJSON != "" {
			// Validate JSON format
			var test map[string]interface{}
			if err := json.Unmarshal([]byte(c.ServiceAccountJSON), &test); err != nil {
				return fmt.Errorf("service_account_json is not valid JSON: %w", err)
			}
		}
	case AuthTypeApplicationDefault:
		// No additional validation needed for ADC
	case "":
		return fmt.Errorf("auth_type is required")
	default:
		return fmt.Errorf("invalid auth_type: %s (must be one of: bearer_token, service_account, adc)", c.AuthType)
	}

	return nil
}

// GetEndpoint returns the Vertex AI endpoint URL
func (c *VertexConfig) GetEndpoint() string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	return fmt.Sprintf("https://%s-aiplatform.googleapis.com", c.Region)
}

// GetModelVersion returns the Vertex AI model version for a given Anthropic model ID
// Returns the mapped version if available, otherwise returns a default mapping
func (c *VertexConfig) GetModelVersion(anthropicModelID string) string {
	// Check custom mapping first
	if c.ModelVersionMap != nil {
		if version, ok := c.ModelVersionMap[anthropicModelID]; ok {
			return version
		}
	}

	// Return default mapping
	return GetDefaultModelVersion(anthropicModelID)
}

// NewDefaultConfig creates a VertexConfig with default values
func NewDefaultConfig(projectID, region string) *VertexConfig {
	return &VertexConfig{
		ProjectID:       projectID,
		Region:          region,
		AuthType:        AuthTypeApplicationDefault,
		ModelVersionMap: make(map[string]string),
	}
}

// WithBearerToken sets bearer token authentication
func (c *VertexConfig) WithBearerToken(token string) *VertexConfig {
	c.AuthType = AuthTypeBearerToken
	c.BearerToken = token
	return c
}

// WithServiceAccountFile sets service account file authentication
func (c *VertexConfig) WithServiceAccountFile(filePath string) *VertexConfig {
	c.AuthType = AuthTypeServiceAccount
	c.ServiceAccountFile = filePath
	return c
}

// WithServiceAccountJSON sets service account JSON authentication
func (c *VertexConfig) WithServiceAccountJSON(jsonContent string) *VertexConfig {
	c.AuthType = AuthTypeServiceAccount
	c.ServiceAccountJSON = jsonContent
	return c
}

// WithApplicationDefault sets ADC authentication
func (c *VertexConfig) WithApplicationDefault() *VertexConfig {
	c.AuthType = AuthTypeApplicationDefault
	return c
}
