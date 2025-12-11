package vertex

import (
	"strings"
)

// ModelMapping defines the mapping between Anthropic model IDs and Vertex AI model identifiers
var ModelMapping = map[string]string{
	// Claude 3.5 Sonnet
	"claude-3-5-sonnet-20241022": "claude-3-5-sonnet-v2@20241022",
	"claude-3-5-sonnet-20240620": "claude-3-5-sonnet@20240620",
	"claude-3-5-sonnet":          "claude-3-5-sonnet-v2@20241022", // Latest version

	// Claude 3.5 Haiku
	"claude-3-5-haiku-20241022": "claude-3-5-haiku@20241022",
	"claude-3-5-haiku":          "claude-3-5-haiku@20241022",

	// Claude 3 Opus
	"claude-3-opus-20240229": "claude-3-opus@20240229",
	"claude-3-opus":          "claude-3-opus@20240229",

	// Claude 3 Sonnet
	"claude-3-sonnet-20240229": "claude-3-sonnet@20240229",
	"claude-3-sonnet":          "claude-3-sonnet@20240229",

	// Claude 3 Haiku
	"claude-3-haiku-20240307": "claude-3-haiku@20240307",
	"claude-3-haiku":          "claude-3-haiku@20240307",
}

// RegionAvailability defines which models are available in which regions
// Based on Google Cloud Vertex AI Claude model availability
var RegionAvailability = map[string][]string{
	"us-east5": {
		"claude-3-5-sonnet-v2@20241022",
		"claude-3-5-sonnet@20240620",
		"claude-3-5-haiku@20241022",
		"claude-3-opus@20240229",
		"claude-3-sonnet@20240229",
		"claude-3-haiku@20240307",
	},
	"europe-west1": {
		"claude-3-5-sonnet-v2@20241022",
		"claude-3-5-sonnet@20240620",
		"claude-3-5-haiku@20241022",
		"claude-3-opus@20240229",
		"claude-3-sonnet@20240229",
		"claude-3-haiku@20240307",
	},
	"us-central1": {
		"claude-3-5-sonnet-v2@20241022",
		"claude-3-5-sonnet@20240620",
		"claude-3-5-haiku@20241022",
		"claude-3-opus@20240229",
		"claude-3-sonnet@20240229",
		"claude-3-haiku@20240307",
	},
	"asia-southeast1": {
		"claude-3-5-sonnet-v2@20241022",
		"claude-3-5-sonnet@20240620",
		"claude-3-5-haiku@20241022",
		"claude-3-opus@20240229",
		"claude-3-sonnet@20240229",
		"claude-3-haiku@20240307",
	},
}

// GetDefaultModelVersion returns the default Vertex AI model version for an Anthropic model ID
func GetDefaultModelVersion(anthropicModelID string) string {
	// Check exact match first
	if version, ok := ModelMapping[anthropicModelID]; ok {
		return version
	}

	// Try to infer from model ID patterns
	// Handle versioned model IDs (e.g., "claude-3-5-sonnet-v2@20241022")
	if strings.Contains(anthropicModelID, "@") {
		return anthropicModelID
	}

	// For Claude 4+ models, pass through as-is with @ format if version is detected
	if strings.HasPrefix(anthropicModelID, "claude-") {
		parts := strings.Split(anthropicModelID, "-")
		if len(parts) >= 4 {
			// Extract date if present (YYYYMMDD format at end)
			lastPart := parts[len(parts)-1]
			if len(lastPart) == 8 && isNumeric(lastPart) {
				// Model with date: claude-opus-4-5-20251101 -> claude-opus-4-5@20251101
				modelName := strings.Join(parts[:len(parts)-1], "-")
				return modelName + "@" + lastPart
			}
		}
	}

	// Fallback: return as-is
	return anthropicModelID
}

// IsModelAvailableInRegion checks if a model is available in a specific region
func IsModelAvailableInRegion(vertexModelID, region string) bool {
	availableModels, ok := RegionAvailability[region]
	if !ok {
		// Unknown region, assume available (will fail at runtime if not)
		return true
	}

	for _, model := range availableModels {
		if model == vertexModelID {
			return true
		}
	}

	return false
}

// GetAvailableRegions returns a list of regions where a model is available
func GetAvailableRegions(vertexModelID string) []string {
	var regions []string
	for region, models := range RegionAvailability {
		for _, model := range models {
			if model == vertexModelID {
				regions = append(regions, region)
				break
			}
		}
	}
	return regions
}

// GetAnthropicModelID converts a Vertex AI model identifier back to Anthropic format
// Example: "claude-3-5-sonnet@20240620" -> "claude-3-5-sonnet-20240620"
func GetAnthropicModelID(vertexModelID string) string {
	// Split on @ to separate model name and version
	parts := strings.Split(vertexModelID, "@")
	if len(parts) == 2 {
		// Remove -v2, -v3 etc suffixes if present
		modelName := strings.TrimSuffix(parts[0], "-v2")
		modelName = strings.TrimSuffix(modelName, "-v3")
		return modelName + "-" + parts[1]
	}

	// Return as-is if not in expected format
	return vertexModelID
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// SupportedRegions returns a list of all supported regions
func SupportedRegions() []string {
	regions := make([]string, 0, len(RegionAvailability))
	for region := range RegionAvailability {
		regions = append(regions, region)
	}
	return regions
}

// GetRecommendedRegion returns a recommended region based on simple heuristics
// This is a basic implementation that could be enhanced with latency-based selection
func GetRecommendedRegion() string {
	// Default to us-east5 as it typically has the most stable availability
	return "us-east5"
}
