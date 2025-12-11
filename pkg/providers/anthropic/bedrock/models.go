package bedrock

import (
	"fmt"
	"strings"
)

// DefaultModelMappings provides the default mapping between Anthropic model names
// and AWS Bedrock model IDs. These mappings are based on the official AWS Bedrock
// documentation for Anthropic Claude models.
var DefaultModelMappings = map[string]string{
	// Claude 3.5 Models
	"claude-3-5-sonnet-20241022": "anthropic.claude-3-5-sonnet-20241022-v2:0",
	"claude-3-5-haiku-20241022":  "anthropic.claude-3-5-haiku-20241022-v1:0",
	"claude-3-5-sonnet-20240620": "anthropic.claude-3-5-sonnet-20240620-v1:0",

	// Claude 3 Models
	"claude-3-opus-20240229":   "anthropic.claude-3-opus-20240229-v1:0",
	"claude-3-sonnet-20240229": "anthropic.claude-3-sonnet-20240229-v1:0",
	"claude-3-haiku-20240307":  "anthropic.claude-3-haiku-20240307-v1:0",

	// Claude 2.x Models (Legacy)
	"claude-2.1": "anthropic.claude-v2:1",
	"claude-2.0": "anthropic.claude-v2",

	// Instant Models (Legacy)
	"claude-instant-1.2": "anthropic.claude-instant-v1",

	// Alias mappings for convenience (map short names to full model IDs)
	"claude-3-opus":     "anthropic.claude-3-opus-20240229-v1:0",
	"claude-3-sonnet":   "anthropic.claude-3-sonnet-20240229-v1:0",
	"claude-3-haiku":    "anthropic.claude-3-haiku-20240307-v1:0",
	"claude-3.5-sonnet": "anthropic.claude-3-5-sonnet-20241022-v2:0",
	"claude-3.5-haiku":  "anthropic.claude-3-5-haiku-20241022-v1:0",

	// Claude 4 Models (when available on Bedrock)
	"claude-opus-4-5-20251101":   "anthropic.claude-opus-4-5-20251101-v1:0",
	"claude-opus-4-5":            "anthropic.claude-opus-4-5-20251101-v1:0",
	"claude-opus-4-1-20250805":   "anthropic.claude-opus-4-1-20250805-v1:0",
	"claude-opus-4-1":            "anthropic.claude-opus-4-1-20250805-v1:0",
	"claude-sonnet-4-5-20250929": "anthropic.claude-sonnet-4-5-20250929-v1:0",
	"claude-sonnet-4-5":          "anthropic.claude-sonnet-4-5-20250929-v1:0",
	"claude-sonnet-4-20250514":   "anthropic.claude-sonnet-4-20250514-v1:0",
	"claude-sonnet-4":            "anthropic.claude-sonnet-4-20250514-v1:0",
	"claude-haiku-4-5-20251001":  "anthropic.claude-haiku-4-5-20251001-v1:0",
	"claude-haiku-4-5":           "anthropic.claude-haiku-4-5-20251001-v1:0",
}

// ModelMapper handles conversion between Anthropic and Bedrock model identifiers
type ModelMapper struct {
	mappings map[string]string
}

// NewModelMapper creates a new ModelMapper with default mappings
func NewModelMapper() *ModelMapper {
	// Create a copy of default mappings
	mappings := make(map[string]string, len(DefaultModelMappings))
	for k, v := range DefaultModelMappings {
		mappings[k] = v
	}

	return &ModelMapper{
		mappings: mappings,
	}
}

// NewModelMapperWithCustomMappings creates a ModelMapper with custom mappings
// Custom mappings will override default mappings for matching keys
func NewModelMapperWithCustomMappings(customMappings map[string]string) *ModelMapper {
	mapper := NewModelMapper()
	mapper.AddMappings(customMappings)
	return mapper
}

// AddMappings adds or overrides model mappings
func (m *ModelMapper) AddMappings(mappings map[string]string) {
	for k, v := range mappings {
		m.mappings[k] = v
	}
}

// ToBedrockModelID converts an Anthropic model name to a Bedrock model ID
// Returns the Bedrock model ID and true if found, or the original model name and false if not found
func (m *ModelMapper) ToBedrockModelID(anthropicModel string) (string, bool) {
	// Direct lookup
	if bedrockID, ok := m.mappings[anthropicModel]; ok {
		return bedrockID, true
	}

	// If the model name already looks like a Bedrock ID (starts with "anthropic."), return as-is
	if strings.HasPrefix(anthropicModel, "anthropic.") {
		return anthropicModel, true
	}

	// Try case-insensitive lookup
	lowerModel := strings.ToLower(anthropicModel)
	for k, v := range m.mappings {
		if strings.ToLower(k) == lowerModel {
			return v, true
		}
	}

	// Not found - return original
	return anthropicModel, false
}

// ToAnthropicModelName converts a Bedrock model ID to an Anthropic model name
// Returns the Anthropic model name and true if found, or the original ID and false if not found
func (m *ModelMapper) ToAnthropicModelName(bedrockID string) (string, bool) {
	// Reverse lookup
	for k, v := range m.mappings {
		if v == bedrockID {
			return k, true
		}
	}

	// If it doesn't look like a Bedrock ID, assume it's already an Anthropic name
	if !strings.HasPrefix(bedrockID, "anthropic.") {
		return bedrockID, true
	}

	// Not found - return original
	return bedrockID, false
}

// GetAllMappings returns a copy of all current mappings
func (m *ModelMapper) GetAllMappings() map[string]string {
	copy := make(map[string]string, len(m.mappings))
	for k, v := range m.mappings {
		copy[k] = v
	}
	return copy
}

// IsBedrockModelID checks if a string looks like a Bedrock model ID
func IsBedrockModelID(modelID string) bool {
	return strings.HasPrefix(modelID, "anthropic.")
}

// IsAnthropicModelName checks if a string looks like an Anthropic model name
func IsAnthropicModelName(modelName string) bool {
	return !strings.HasPrefix(modelName, "anthropic.")
}

// ValidateModelMapping checks if a model can be mapped to Bedrock
func (m *ModelMapper) ValidateModelMapping(anthropicModel string) error {
	bedrockID, found := m.ToBedrockModelID(anthropicModel)
	if !found {
		return fmt.Errorf("no Bedrock mapping found for Anthropic model: %s", anthropicModel)
	}

	if bedrockID == anthropicModel {
		return fmt.Errorf("model mapping returned unchanged value, possibly invalid: %s", anthropicModel)
	}

	return nil
}

// GetSupportedModels returns a list of all Anthropic model names that have Bedrock mappings
func (m *ModelMapper) GetSupportedModels() []string {
	models := make([]string, 0, len(m.mappings))
	for k := range m.mappings {
		models = append(models, k)
	}
	return models
}
