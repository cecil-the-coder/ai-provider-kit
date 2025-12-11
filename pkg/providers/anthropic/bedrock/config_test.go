package bedrock

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBedrockConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *BedrockConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &BedrockConfig{
				Region:          "us-east-1",
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
			wantErr: false,
		},
		{
			name: "missing region",
			config: &BedrockConfig{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
			wantErr: true,
			errMsg:  "region is required",
		},
		{
			name: "missing access key",
			config: &BedrockConfig{
				Region:          "us-east-1",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
			wantErr: true,
			errMsg:  "access_key_id is required",
		},
		{
			name: "missing secret key",
			config: &BedrockConfig{
				Region:      "us-east-1",
				AccessKeyID: "AKIAIOSFODNN7EXAMPLE",
			},
			wantErr: true,
			errMsg:  "secret_access_key is required",
		},
		{
			name: "valid with session token",
			config: &BedrockConfig{
				Region:          "us-west-2",
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				SessionToken:    "AQoDYXdzEJr...",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBedrockConfig_GetEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		config   *BedrockConfig
		expected string
	}{
		{
			name: "default endpoint for us-east-1",
			config: &BedrockConfig{
				Region: "us-east-1",
			},
			expected: "bedrock-runtime.us-east-1.amazonaws.com",
		},
		{
			name: "default endpoint for eu-west-1",
			config: &BedrockConfig{
				Region: "eu-west-1",
			},
			expected: "bedrock-runtime.eu-west-1.amazonaws.com",
		},
		{
			name: "custom endpoint override",
			config: &BedrockConfig{
				Region:   "us-east-1",
				Endpoint: "custom-bedrock.example.com",
			},
			expected: "custom-bedrock.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint := tt.config.GetEndpoint()
			assert.Equal(t, tt.expected, endpoint)
		})
	}
}

func TestNewConfig(t *testing.T) {
	config := NewConfig("us-west-2", "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")

	assert.Equal(t, "us-west-2", config.Region)
	assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", config.AccessKeyID)
	assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", config.SecretAccessKey)
	assert.Empty(t, config.SessionToken)
	assert.Empty(t, config.Endpoint)
}

func TestBedrockConfig_WithMethods(t *testing.T) {
	config := NewConfig("us-east-1", "key", "secret")

	// Test WithSessionToken
	config = config.WithSessionToken("token123")
	assert.Equal(t, "token123", config.SessionToken)

	// Test WithEndpoint
	config = config.WithEndpoint("custom.endpoint.com")
	assert.Equal(t, "custom.endpoint.com", config.Endpoint)

	// Test WithModelMappings
	mappings := map[string]string{
		"claude-3-opus": "anthropic.claude-3-opus-custom-v1:0",
	}
	config = config.WithModelMappings(mappings)
	assert.Equal(t, mappings, config.ModelMappings)

	// Test WithDebug
	config = config.WithDebug(true)
	assert.True(t, config.Debug)
}

func TestNewConfigFromEnv(t *testing.T) {
	// Save original env vars
	origRegion := os.Getenv("AWS_REGION")
	origAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	origSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	origSessionToken := os.Getenv("AWS_SESSION_TOKEN")
	origDebug := os.Getenv("BEDROCK_DEBUG")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("AWS_REGION", origRegion)
		_ = os.Setenv("AWS_ACCESS_KEY_ID", origAccessKey)
		_ = os.Setenv("AWS_SECRET_ACCESS_KEY", origSecretKey)
		_ = os.Setenv("AWS_SESSION_TOKEN", origSessionToken)
		_ = os.Setenv("BEDROCK_DEBUG", origDebug)
	}()

	t.Run("valid environment variables", func(t *testing.T) {
		_ = os.Setenv("AWS_REGION", "ap-southeast-1")
		_ = os.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
		_ = os.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
		_ = os.Setenv("AWS_SESSION_TOKEN", "session-token")
		_ = os.Setenv("BEDROCK_DEBUG", "true")

		config, err := NewConfigFromEnv()
		require.NoError(t, err)

		assert.Equal(t, "ap-southeast-1", config.Region)
		assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", config.AccessKeyID)
		assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", config.SecretAccessKey)
		assert.Equal(t, "session-token", config.SessionToken)
		assert.True(t, config.Debug)
	})

	t.Run("fallback to AWS_DEFAULT_REGION", func(t *testing.T) {
		_ = os.Unsetenv("AWS_REGION")
		_ = os.Setenv("AWS_DEFAULT_REGION", "eu-central-1")
		_ = os.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
		_ = os.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")

		config, err := NewConfigFromEnv()
		require.NoError(t, err)
		assert.Equal(t, "eu-central-1", config.Region)
	})

	t.Run("missing required env vars", func(t *testing.T) {
		_ = os.Unsetenv("AWS_REGION")
		_ = os.Unsetenv("AWS_DEFAULT_REGION")
		_ = os.Unsetenv("AWS_ACCESS_KEY_ID")
		_ = os.Unsetenv("AWS_SECRET_ACCESS_KEY")

		_, err := NewConfigFromEnv()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create config from environment")
	})

	t.Run("debug false by default", func(t *testing.T) {
		_ = os.Setenv("AWS_REGION", "us-east-1")
		_ = os.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
		_ = os.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
		_ = os.Unsetenv("BEDROCK_DEBUG")

		config, err := NewConfigFromEnv()
		require.NoError(t, err)
		assert.False(t, config.Debug)
	})
}

func TestBedrockConfig_Clone(t *testing.T) {
	original := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		SessionToken:    "token",
		Endpoint:        "custom.endpoint.com",
		Debug:           true,
		ModelMappings: map[string]string{
			"model1": "bedrock-model1",
			"model2": "bedrock-model2",
		},
	}

	clone := original.Clone()

	// Verify clone has same values
	assert.Equal(t, original.Region, clone.Region)
	assert.Equal(t, original.AccessKeyID, clone.AccessKeyID)
	assert.Equal(t, original.SecretAccessKey, clone.SecretAccessKey)
	assert.Equal(t, original.SessionToken, clone.SessionToken)
	assert.Equal(t, original.Endpoint, clone.Endpoint)
	assert.Equal(t, original.Debug, clone.Debug)
	assert.Equal(t, original.ModelMappings, clone.ModelMappings)

	// Verify it's a deep copy (modify clone shouldn't affect original)
	clone.Region = "us-west-2"
	clone.ModelMappings["model3"] = "bedrock-model3"

	assert.Equal(t, "us-east-1", original.Region)
	assert.NotContains(t, original.ModelMappings, "model3")
}
