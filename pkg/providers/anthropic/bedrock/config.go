// Package bedrock provides AWS Bedrock integration middleware for the Anthropic provider.
// It enables routing Anthropic API requests through AWS Bedrock with proper authentication,
// request transformation, and AWS Signature V4 signing.
package bedrock

import (
	"fmt"
	"os"
	"strings"
)

// BedrockConfig holds configuration for AWS Bedrock integration
type BedrockConfig struct {
	// AWS Region for Bedrock API (e.g., "us-east-1", "us-west-2")
	Region string

	// AWS credentials for signing requests
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string // Optional, for temporary credentials

	// Custom endpoint override (optional)
	// If not set, uses: bedrock-runtime.{region}.amazonaws.com
	Endpoint string

	// Model ID mappings (optional overrides)
	// Maps Anthropic model names to Bedrock model IDs
	// If not provided, uses default mappings from models.go
	ModelMappings map[string]string

	// Enable debug logging for AWS requests
	Debug bool
}

// Validate checks if the configuration is valid
func (c *BedrockConfig) Validate() error {
	if c.Region == "" {
		return fmt.Errorf("bedrock: region is required")
	}

	if c.AccessKeyID == "" {
		return fmt.Errorf("bedrock: access_key_id is required")
	}

	if c.SecretAccessKey == "" {
		return fmt.Errorf("bedrock: secret_access_key is required")
	}

	return nil
}

// GetEndpoint returns the Bedrock endpoint URL
func (c *BedrockConfig) GetEndpoint() string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	return fmt.Sprintf("bedrock-runtime.%s.amazonaws.com", c.Region)
}

// NewConfigFromEnv creates a BedrockConfig from environment variables
// It follows the standard AWS credential chain precedence:
// 1. Explicit environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
// 2. Session token if available (AWS_SESSION_TOKEN)
// 3. Region from AWS_REGION or AWS_DEFAULT_REGION
func NewConfigFromEnv() (*BedrockConfig, error) {
	config := &BedrockConfig{
		Region:          getEnvOrDefault("AWS_REGION", getEnvOrDefault("AWS_DEFAULT_REGION", "")),
		AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		SessionToken:    os.Getenv("AWS_SESSION_TOKEN"),
		Debug:           strings.ToLower(os.Getenv("BEDROCK_DEBUG")) == "true",
	}

	// Validate that we have the minimum required configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("failed to create config from environment: %w", err)
	}

	return config, nil
}

// NewConfig creates a BedrockConfig with the provided parameters
func NewConfig(region, accessKeyID, secretAccessKey string) *BedrockConfig {
	return &BedrockConfig{
		Region:          region,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
	}
}

// WithSessionToken adds a session token for temporary credentials
func (c *BedrockConfig) WithSessionToken(token string) *BedrockConfig {
	c.SessionToken = token
	return c
}

// WithEndpoint sets a custom endpoint
func (c *BedrockConfig) WithEndpoint(endpoint string) *BedrockConfig {
	c.Endpoint = endpoint
	return c
}

// WithModelMappings sets custom model ID mappings
func (c *BedrockConfig) WithModelMappings(mappings map[string]string) *BedrockConfig {
	c.ModelMappings = mappings
	return c
}

// WithDebug enables debug logging
func (c *BedrockConfig) WithDebug(debug bool) *BedrockConfig {
	c.Debug = debug
	return c
}

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Clone creates a copy of the config
func (c *BedrockConfig) Clone() *BedrockConfig {
	clone := &BedrockConfig{
		Region:          c.Region,
		AccessKeyID:     c.AccessKeyID,
		SecretAccessKey: c.SecretAccessKey,
		SessionToken:    c.SessionToken,
		Endpoint:        c.Endpoint,
		Debug:           c.Debug,
	}

	if c.ModelMappings != nil {
		clone.ModelMappings = make(map[string]string, len(c.ModelMappings))
		for k, v := range c.ModelMappings {
			clone.ModelMappings[k] = v
		}
	}

	return clone
}
