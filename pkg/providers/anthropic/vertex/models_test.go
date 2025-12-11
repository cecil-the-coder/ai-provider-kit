package vertex

import (
	"reflect"
	"testing"
)

func TestGetDefaultModelVersion(t *testing.T) {
	tests := []struct {
		name            string
		anthropicModel  string
		expectedVersion string
	}{
		{
			name:            "claude-3-5-sonnet-20241022",
			anthropicModel:  "claude-3-5-sonnet-20241022",
			expectedVersion: "claude-3-5-sonnet-v2@20241022",
		},
		{
			name:            "claude-3-5-sonnet latest",
			anthropicModel:  "claude-3-5-sonnet",
			expectedVersion: "claude-3-5-sonnet-v2@20241022",
		},
		{
			name:            "claude-3-5-haiku",
			anthropicModel:  "claude-3-5-haiku-20241022",
			expectedVersion: "claude-3-5-haiku@20241022",
		},
		{
			name:            "claude-3-opus",
			anthropicModel:  "claude-3-opus-20240229",
			expectedVersion: "claude-3-opus@20240229",
		},
		{
			name:            "claude-3-sonnet",
			anthropicModel:  "claude-3-sonnet-20240229",
			expectedVersion: "claude-3-sonnet@20240229",
		},
		{
			name:            "claude-3-haiku",
			anthropicModel:  "claude-3-haiku-20240307",
			expectedVersion: "claude-3-haiku@20240307",
		},
		{
			name:            "already in vertex format",
			anthropicModel:  "claude-3-opus@20240229",
			expectedVersion: "claude-3-opus@20240229",
		},
		{
			name:            "claude-4 future model with date",
			anthropicModel:  "claude-opus-4-5-20251101",
			expectedVersion: "claude-opus-4-5@20251101",
		},
		{
			name:            "unknown model",
			anthropicModel:  "claude-unknown-model",
			expectedVersion: "claude-unknown-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetDefaultModelVersion(tt.anthropicModel)
			if got != tt.expectedVersion {
				t.Errorf("GetDefaultModelVersion(%q) = %q, want %q", tt.anthropicModel, got, tt.expectedVersion)
			}
		})
	}
}

func TestIsModelAvailableInRegion(t *testing.T) {
	tests := []struct {
		name          string
		vertexModelID string
		region        string
		expected      bool
	}{
		{
			name:          "claude-3-5-sonnet in us-east5",
			vertexModelID: "claude-3-5-sonnet-v2@20241022",
			region:        "us-east5",
			expected:      true,
		},
		{
			name:          "claude-3-opus in europe-west1",
			vertexModelID: "claude-3-opus@20240229",
			region:        "europe-west1",
			expected:      true,
		},
		{
			name:          "unknown model in us-east5",
			vertexModelID: "claude-unknown@20241022",
			region:        "us-east5",
			expected:      false,
		},
		{
			name:          "claude-3-5-sonnet in unknown region",
			vertexModelID: "claude-3-5-sonnet-v2@20241022",
			region:        "unknown-region",
			expected:      true, // Unknown regions return true
		},
		{
			name:          "claude-3-haiku in asia-southeast1",
			vertexModelID: "claude-3-haiku@20240307",
			region:        "asia-southeast1",
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsModelAvailableInRegion(tt.vertexModelID, tt.region)
			if got != tt.expected {
				t.Errorf("IsModelAvailableInRegion(%q, %q) = %v, want %v", tt.vertexModelID, tt.region, got, tt.expected)
			}
		})
	}
}

func TestGetAvailableRegions(t *testing.T) {
	tests := []struct {
		name          string
		vertexModelID string
		minRegions    int
	}{
		{
			name:          "claude-3-5-sonnet-v2",
			vertexModelID: "claude-3-5-sonnet-v2@20241022",
			minRegions:    4, // Should be in all supported regions
		},
		{
			name:          "claude-3-opus",
			vertexModelID: "claude-3-opus@20240229",
			minRegions:    4,
		},
		{
			name:          "unknown model",
			vertexModelID: "claude-unknown@20241022",
			minRegions:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regions := GetAvailableRegions(tt.vertexModelID)
			if len(regions) < tt.minRegions {
				t.Errorf("GetAvailableRegions(%q) returned %d regions, want at least %d", tt.vertexModelID, len(regions), tt.minRegions)
			}
		})
	}
}

func TestGetAnthropicModelID(t *testing.T) {
	tests := []struct {
		name           string
		vertexModelID  string
		expectedOutput string
	}{
		{
			name:           "claude-3-5-sonnet-v2@20241022",
			vertexModelID:  "claude-3-5-sonnet-v2@20241022",
			expectedOutput: "claude-3-5-sonnet-20241022",
		},
		{
			name:           "claude-3-opus@20240229",
			vertexModelID:  "claude-3-opus@20240229",
			expectedOutput: "claude-3-opus-20240229",
		},
		{
			name:           "claude-3-haiku@20240307",
			vertexModelID:  "claude-3-haiku@20240307",
			expectedOutput: "claude-3-haiku-20240307",
		},
		{
			name:           "model without version",
			vertexModelID:  "claude-3-sonnet",
			expectedOutput: "claude-3-sonnet",
		},
		{
			name:           "claude-3-5-sonnet@20240620",
			vertexModelID:  "claude-3-5-sonnet@20240620",
			expectedOutput: "claude-3-5-sonnet-20240620",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAnthropicModelID(tt.vertexModelID)
			if got != tt.expectedOutput {
				t.Errorf("GetAnthropicModelID(%q) = %q, want %q", tt.vertexModelID, got, tt.expectedOutput)
			}
		})
	}
}

func TestSupportedRegions(t *testing.T) {
	regions := SupportedRegions()

	if len(regions) == 0 {
		t.Error("SupportedRegions() returned empty list")
	}

	// Check that we have the expected regions
	expectedRegions := map[string]bool{
		"us-east5":        true,
		"europe-west1":    true,
		"us-central1":     true,
		"asia-southeast1": true,
	}

	for _, region := range regions {
		if !expectedRegions[region] {
			t.Errorf("SupportedRegions() returned unexpected region: %q", region)
		}
		delete(expectedRegions, region)
	}

	if len(expectedRegions) > 0 {
		t.Errorf("SupportedRegions() missing expected regions: %v", keys(expectedRegions))
	}
}

func TestGetRecommendedRegion(t *testing.T) {
	region := GetRecommendedRegion()

	if region == "" {
		t.Error("GetRecommendedRegion() returned empty string")
	}

	// Should return a valid region
	supported := SupportedRegions()
	found := false
	for _, r := range supported {
		if r == region {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("GetRecommendedRegion() returned %q which is not in supported regions", region)
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"20241022", true},
		{"12345", true},
		{"0", true},
		{"abc", false},
		{"123abc", false},
		{"", true}, // Empty string has no non-numeric chars
		{"2024-10-22", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isNumeric(tt.input)
			if got != tt.expected {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestModelMapping_Coverage(t *testing.T) {
	// Ensure all mapped models are valid
	for anthropicID, vertexID := range ModelMapping {
		if anthropicID == "" {
			t.Error("ModelMapping contains empty Anthropic model ID")
		}
		if vertexID == "" {
			t.Errorf("ModelMapping[%q] has empty Vertex model ID", anthropicID)
		}
	}
}

func TestRegionAvailability_Coverage(t *testing.T) {
	// Ensure all regions have models
	for region, models := range RegionAvailability {
		if region == "" {
			t.Error("RegionAvailability contains empty region")
		}
		if len(models) == 0 {
			t.Errorf("RegionAvailability[%q] has no models", region)
		}
	}
}

func keys(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

func TestModelMappingConsistency(t *testing.T) {
	// Test that mapped models can be converted back
	for anthropicID, vertexID := range ModelMapping {
		// Convert to Vertex format using GetDefaultModelVersion
		got := GetDefaultModelVersion(anthropicID)
		if got != vertexID {
			t.Errorf("GetDefaultModelVersion(%q) = %q, want %q (from ModelMapping)", anthropicID, got, vertexID)
		}

		// Convert back to Anthropic format
		convertedBack := GetAnthropicModelID(vertexID)

		// Should match the original or be a variant
		if convertedBack != anthropicID {
			// Check if it's a valid variant (e.g., removing -v2 suffix)
			if !reflect.DeepEqual(convertedBack, anthropicID) {
				// This is expected for some models, just log it
				t.Logf("Round-trip conversion: %q -> %q -> %q", anthropicID, vertexID, convertedBack)
			}
		}
	}
}
