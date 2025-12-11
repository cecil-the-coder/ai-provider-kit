package bedrock

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModelMapper(t *testing.T) {
	mapper := NewModelMapper()
	assert.NotNil(t, mapper)
	assert.NotNil(t, mapper.mappings)

	// Verify default mappings are loaded
	assert.Greater(t, len(mapper.mappings), 0)
}

func TestNewModelMapperWithCustomMappings(t *testing.T) {
	customMappings := map[string]string{
		"custom-model": "anthropic.custom-model-v1:0",
	}

	mapper := NewModelMapperWithCustomMappings(customMappings)
	assert.NotNil(t, mapper)

	// Verify custom mapping is present
	bedrockID, found := mapper.ToBedrockModelID("custom-model")
	assert.True(t, found)
	assert.Equal(t, "anthropic.custom-model-v1:0", bedrockID)

	// Verify default mappings are still present
	bedrockID, found = mapper.ToBedrockModelID("claude-3-opus-20240229")
	assert.True(t, found)
	assert.Equal(t, "anthropic.claude-3-opus-20240229-v1:0", bedrockID)
}

func TestModelMapper_ToBedrockModelID(t *testing.T) {
	mapper := NewModelMapper()

	tests := []struct {
		name           string
		anthropicModel string
		expectedID     string
		expectedFound  bool
	}{
		{
			name:           "claude-3-opus exact match",
			anthropicModel: "claude-3-opus-20240229",
			expectedID:     "anthropic.claude-3-opus-20240229-v1:0",
			expectedFound:  true,
		},
		{
			name:           "claude-3-sonnet exact match",
			anthropicModel: "claude-3-sonnet-20240229",
			expectedID:     "anthropic.claude-3-sonnet-20240229-v1:0",
			expectedFound:  true,
		},
		{
			name:           "claude-3-haiku exact match",
			anthropicModel: "claude-3-haiku-20240307",
			expectedID:     "anthropic.claude-3-haiku-20240307-v1:0",
			expectedFound:  true,
		},
		{
			name:           "claude-3.5-sonnet",
			anthropicModel: "claude-3-5-sonnet-20241022",
			expectedID:     "anthropic.claude-3-5-sonnet-20241022-v2:0",
			expectedFound:  true,
		},
		{
			name:           "alias: claude-3-opus",
			anthropicModel: "claude-3-opus",
			expectedID:     "anthropic.claude-3-opus-20240229-v1:0",
			expectedFound:  true,
		},
		{
			name:           "already bedrock ID",
			anthropicModel: "anthropic.claude-3-opus-20240229-v1:0",
			expectedID:     "anthropic.claude-3-opus-20240229-v1:0",
			expectedFound:  true,
		},
		{
			name:           "unknown model",
			anthropicModel: "unknown-model",
			expectedID:     "unknown-model",
			expectedFound:  false,
		},
		{
			name:           "case insensitive match",
			anthropicModel: "CLAUDE-3-OPUS-20240229",
			expectedID:     "anthropic.claude-3-opus-20240229-v1:0",
			expectedFound:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bedrockID, found := mapper.ToBedrockModelID(tt.anthropicModel)
			assert.Equal(t, tt.expectedFound, found)
			assert.Equal(t, tt.expectedID, bedrockID)
		})
	}
}

func TestModelMapper_ToAnthropicModelName(t *testing.T) {
	mapper := NewModelMapper()

	tests := []struct {
		name          string
		bedrockID     string
		expectedName  string
		expectedFound bool
	}{
		{
			name:          "reverse lookup claude-3-opus",
			bedrockID:     "anthropic.claude-3-opus-20240229-v1:0",
			expectedName:  "claude-3-opus-20240229",
			expectedFound: true,
		},
		{
			name:          "reverse lookup claude-3-sonnet",
			bedrockID:     "anthropic.claude-3-sonnet-20240229-v1:0",
			expectedName:  "claude-3-sonnet-20240229", // Could also return "claude-3-sonnet" alias
			expectedFound: true,
		},
		{
			name:          "unknown bedrock ID",
			bedrockID:     "anthropic.unknown-model-v1:0",
			expectedName:  "anthropic.unknown-model-v1:0",
			expectedFound: false,
		},
		{
			name:          "anthropic name passed in",
			bedrockID:     "claude-3-opus-20240229",
			expectedName:  "claude-3-opus-20240229",
			expectedFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anthropicName, found := mapper.ToAnthropicModelName(tt.bedrockID)
			assert.Equal(t, tt.expectedFound, found)
			switch {
			case found && !strings.HasPrefix(tt.bedrockID, "anthropic."):
				// If input was already an Anthropic name, should return as-is
				assert.Equal(t, tt.expectedName, anthropicName)
			case found:
				// Verify the returned name maps back to the same Bedrock ID
				bedrockID, _ := mapper.ToBedrockModelID(anthropicName)
				assert.Equal(t, tt.bedrockID, bedrockID)
			default:
				assert.Equal(t, tt.expectedName, anthropicName)
			}
		})
	}
}

func TestModelMapper_AddMappings(t *testing.T) {
	mapper := NewModelMapper()

	customMappings := map[string]string{
		"new-model-1": "anthropic.new-model-1-v1:0",
		"new-model-2": "anthropic.new-model-2-v1:0",
	}

	mapper.AddMappings(customMappings)

	// Verify new mappings are added
	bedrockID, found := mapper.ToBedrockModelID("new-model-1")
	assert.True(t, found)
	assert.Equal(t, "anthropic.new-model-1-v1:0", bedrockID)

	bedrockID, found = mapper.ToBedrockModelID("new-model-2")
	assert.True(t, found)
	assert.Equal(t, "anthropic.new-model-2-v1:0", bedrockID)
}

func TestModelMapper_AddMappings_Override(t *testing.T) {
	mapper := NewModelMapper()

	// Override an existing mapping
	customMappings := map[string]string{
		"claude-3-opus-20240229": "anthropic.claude-3-opus-custom-v2:0",
	}

	mapper.AddMappings(customMappings)

	// Verify the mapping was overridden
	bedrockID, found := mapper.ToBedrockModelID("claude-3-opus-20240229")
	assert.True(t, found)
	assert.Equal(t, "anthropic.claude-3-opus-custom-v2:0", bedrockID)
}

func TestModelMapper_GetAllMappings(t *testing.T) {
	mapper := NewModelMapper()

	mappings := mapper.GetAllMappings()

	// Verify we get a copy
	assert.NotNil(t, mappings)
	assert.Greater(t, len(mappings), 0)

	// Verify it's a copy (modifying shouldn't affect mapper)
	mappings["test-key"] = "test-value"
	bedrockID, found := mapper.ToBedrockModelID("test-key")
	assert.False(t, found)
	assert.Equal(t, "test-key", bedrockID)
}

func TestIsBedrockModelID(t *testing.T) {
	tests := []struct {
		name     string
		modelID  string
		expected bool
	}{
		{
			name:     "valid bedrock ID",
			modelID:  "anthropic.claude-3-opus-20240229-v1:0",
			expected: true,
		},
		{
			name:     "anthropic model name",
			modelID:  "claude-3-opus-20240229",
			expected: false,
		},
		{
			name:     "empty string",
			modelID:  "",
			expected: false,
		},
		{
			name:     "other provider prefix",
			modelID:  "cohere.command-v1",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBedrockModelID(tt.modelID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsAnthropicModelName(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		expected  bool
	}{
		{
			name:      "anthropic model name",
			modelName: "claude-3-opus-20240229",
			expected:  true,
		},
		{
			name:      "bedrock ID",
			modelName: "anthropic.claude-3-opus-20240229-v1:0",
			expected:  false,
		},
		{
			name:      "empty string",
			modelName: "",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAnthropicModelName(tt.modelName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestModelMapper_ValidateModelMapping(t *testing.T) {
	mapper := NewModelMapper()

	tests := []struct {
		name           string
		anthropicModel string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "valid model",
			anthropicModel: "claude-3-opus-20240229",
			wantErr:        false,
		},
		{
			name:           "unknown model",
			anthropicModel: "unknown-model",
			wantErr:        true,
			errContains:    "no Bedrock mapping found",
		},
		{
			name:           "bedrock ID as input",
			anthropicModel: "anthropic.claude-3-opus-20240229-v1:0",
			wantErr:        true,
			errContains:    "mapping returned unchanged value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapper.ValidateModelMapping(tt.anthropicModel)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestModelMapper_GetSupportedModels(t *testing.T) {
	mapper := NewModelMapper()

	models := mapper.GetSupportedModels()

	// Verify we get a list of models
	assert.NotNil(t, models)
	assert.Greater(t, len(models), 0)

	// Verify some expected models are present
	assert.Contains(t, models, "claude-3-opus-20240229")
	assert.Contains(t, models, "claude-3-sonnet-20240229")
	assert.Contains(t, models, "claude-3-haiku-20240307")
}

func TestDefaultModelMappings(t *testing.T) {
	// Test that default mappings contain expected models
	expectedModels := []string{
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
		"claude-3-5-sonnet-20241022",
		"claude-3-5-haiku-20241022",
	}

	for _, model := range expectedModels {
		t.Run(model, func(t *testing.T) {
			bedrockID, exists := DefaultModelMappings[model]
			assert.True(t, exists, "model should exist in default mappings")
			assert.NotEmpty(t, bedrockID, "bedrock ID should not be empty")
			assert.True(t, IsBedrockModelID(bedrockID), "should be a valid bedrock ID")
		})
	}
}
