package gemini

import (
	"encoding/json"
	"testing"
)

// TestHarmCategories tests all harm category constants
func TestHarmCategories(t *testing.T) {
	categories := []HarmCategory{
		HarmCategoryUnspecified,
		HarmCategoryHarassment,
		HarmCategoryHateSpeech,
		HarmCategorySexuallyExplicit,
		HarmCategoryDangerousContent,
		HarmCategoryCivicIntegrity,
		HarmCategoryDerogatory,
		HarmCategoryToxicity,
		HarmCategoryViolence,
		HarmCategorySexual,
		HarmCategoryMedical,
		HarmCategoryDangerous,
	}

	for _, category := range categories {
		if !IsValidHarmCategory(category) {
			t.Errorf("IsValidHarmCategory(%s) = false, want true", category)
		}
	}

	// Test invalid category
	if IsValidHarmCategory(HarmCategory("INVALID_CATEGORY")) {
		t.Error("IsValidHarmCategory(INVALID_CATEGORY) = true, want false")
	}
}

// TestHarmBlockThresholds tests all harm block threshold constants
func TestHarmBlockThresholds(t *testing.T) {
	thresholds := []HarmBlockThreshold{
		HarmBlockThresholdUnspecified,
		HarmBlockThresholdBlockLowAndAbove,
		HarmBlockThresholdBlockMediumAndAbove,
		HarmBlockThresholdBlockOnlyHigh,
		HarmBlockThresholdBlockNone,
		HarmBlockThresholdOff,
	}

	for _, threshold := range thresholds {
		if !IsValidHarmBlockThreshold(threshold) {
			t.Errorf("IsValidHarmBlockThreshold(%s) = false, want true", threshold)
		}
	}

	// Test invalid threshold
	if IsValidHarmBlockThreshold(HarmBlockThreshold("INVALID_THRESHOLD")) {
		t.Error("IsValidHarmBlockThreshold(INVALID_THRESHOLD) = true, want false")
	}
}

// TestSafetySettingValidation tests safety setting validation
func TestSafetySettingValidation(t *testing.T) {
	tests := []struct {
		name      string
		setting   SafetySetting
		wantError bool
	}{
		{
			name: "valid setting",
			setting: SafetySetting{
				Category:  HarmCategoryHarassment,
				Threshold: HarmBlockThresholdBlockMediumAndAbove,
			},
			wantError: false,
		},
		{
			name: "invalid category",
			setting: SafetySetting{
				Category:  HarmCategory("INVALID"),
				Threshold: HarmBlockThresholdBlockMediumAndAbove,
			},
			wantError: true,
		},
		{
			name: "invalid threshold",
			setting: SafetySetting{
				Category:  HarmCategoryHarassment,
				Threshold: HarmBlockThreshold("INVALID"),
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setting.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("SafetySetting.Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestValidateSafetySettings tests validation of multiple safety settings
func TestValidateSafetySettings(t *testing.T) {
	tests := []struct {
		name      string
		settings  []SafetySetting
		wantError bool
	}{
		{
			name: "valid settings",
			settings: []SafetySetting{
				{Category: HarmCategoryHarassment, Threshold: HarmBlockThresholdBlockMediumAndAbove},
				{Category: HarmCategoryHateSpeech, Threshold: HarmBlockThresholdBlockOnlyHigh},
			},
			wantError: false,
		},
		{
			name: "duplicate category",
			settings: []SafetySetting{
				{Category: HarmCategoryHarassment, Threshold: HarmBlockThresholdBlockMediumAndAbove},
				{Category: HarmCategoryHarassment, Threshold: HarmBlockThresholdBlockOnlyHigh},
			},
			wantError: true,
		},
		{
			name: "invalid category in list",
			settings: []SafetySetting{
				{Category: HarmCategoryHarassment, Threshold: HarmBlockThresholdBlockMediumAndAbove},
				{Category: HarmCategory("INVALID"), Threshold: HarmBlockThresholdBlockOnlyHigh},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSafetySettings(tt.settings)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateSafetySettings() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestNewSafetySetting tests creating a new safety setting
func TestNewSafetySetting(t *testing.T) {
	tests := []struct {
		name      string
		category  HarmCategory
		threshold HarmBlockThreshold
		wantError bool
	}{
		{
			name:      "valid setting",
			category:  HarmCategoryHarassment,
			threshold: HarmBlockThresholdBlockMediumAndAbove,
			wantError: false,
		},
		{
			name:      "invalid category",
			category:  HarmCategory("INVALID"),
			threshold: HarmBlockThresholdBlockMediumAndAbove,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting, err := NewSafetySetting(tt.category, tt.threshold)
			if (err != nil) != tt.wantError {
				t.Errorf("NewSafetySetting() error = %v, wantError %v", err, tt.wantError)
			}
			if err == nil && setting == nil {
				t.Error("NewSafetySetting() returned nil setting with no error")
			}
		})
	}
}

// TestDefaultSafetySettings tests the default safety settings preset
func TestDefaultSafetySettings(t *testing.T) {
	settings := DefaultSafetySettings()

	if len(settings) != 4 {
		t.Errorf("DefaultSafetySettings() returned %d settings, want 4", len(settings))
	}

	// Validate all default settings
	if err := ValidateSafetySettings(settings); err != nil {
		t.Errorf("DefaultSafetySettings() returned invalid settings: %v", err)
	}

	// Check that all use BLOCK_MEDIUM_AND_ABOVE threshold
	for _, setting := range settings {
		if setting.Threshold != HarmBlockThresholdBlockMediumAndAbove {
			t.Errorf("DefaultSafetySettings() setting has threshold %s, want %s",
				setting.Threshold, HarmBlockThresholdBlockMediumAndAbove)
		}
	}
}

// TestPermissiveSafetySettings tests the permissive safety settings preset
func TestPermissiveSafetySettings(t *testing.T) {
	settings := PermissiveSafetySettings()

	if len(settings) != 4 {
		t.Errorf("PermissiveSafetySettings() returned %d settings, want 4", len(settings))
	}

	// Validate all permissive settings
	if err := ValidateSafetySettings(settings); err != nil {
		t.Errorf("PermissiveSafetySettings() returned invalid settings: %v", err)
	}

	// Check that all use BLOCK_ONLY_HIGH threshold
	for _, setting := range settings {
		if setting.Threshold != HarmBlockThresholdBlockOnlyHigh {
			t.Errorf("PermissiveSafetySettings() setting has threshold %s, want %s",
				setting.Threshold, HarmBlockThresholdBlockOnlyHigh)
		}
	}
}

// TestStrictSafetySettings tests the strict safety settings preset
func TestStrictSafetySettings(t *testing.T) {
	settings := StrictSafetySettings()

	if len(settings) != 4 {
		t.Errorf("StrictSafetySettings() returned %d settings, want 4", len(settings))
	}

	// Validate all strict settings
	if err := ValidateSafetySettings(settings); err != nil {
		t.Errorf("StrictSafetySettings() returned invalid settings: %v", err)
	}

	// Check that all use BLOCK_LOW_AND_ABOVE threshold
	for _, setting := range settings {
		if setting.Threshold != HarmBlockThresholdBlockLowAndAbove {
			t.Errorf("StrictSafetySettings() setting has threshold %s, want %s",
				setting.Threshold, HarmBlockThresholdBlockLowAndAbove)
		}
	}
}

// TestNoSafetySettings tests the no-blocking safety settings preset
func TestNoSafetySettings(t *testing.T) {
	settings := NoSafetySettings()

	if len(settings) != 4 {
		t.Errorf("NoSafetySettings() returned %d settings, want 4", len(settings))
	}

	// Validate all no-blocking settings
	if err := ValidateSafetySettings(settings); err != nil {
		t.Errorf("NoSafetySettings() returned invalid settings: %v", err)
	}

	// Check that all use BLOCK_NONE threshold
	for _, setting := range settings {
		if setting.Threshold != HarmBlockThresholdBlockNone {
			t.Errorf("NoSafetySettings() setting has threshold %s, want %s",
				setting.Threshold, HarmBlockThresholdBlockNone)
		}
	}
}

// TestSafetySettingJSON tests JSON marshaling/unmarshaling
func TestSafetySettingJSON(t *testing.T) {
	original := SafetySetting{
		Category:  HarmCategoryHarassment,
		Threshold: HarmBlockThresholdBlockMediumAndAbove,
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal from JSON
	var decoded SafetySetting
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Compare
	if decoded.Category != original.Category {
		t.Errorf("decoded.Category = %s, want %s", decoded.Category, original.Category)
	}
	if decoded.Threshold != original.Threshold {
		t.Errorf("decoded.Threshold = %s, want %s", decoded.Threshold, original.Threshold)
	}
}

// TestSafetyRatingJSON tests JSON unmarshaling of safety ratings
func TestSafetyRatingJSON(t *testing.T) {
	jsonData := `{
		"category": "HARM_CATEGORY_HARASSMENT",
		"probability": "HIGH",
		"severity": "HARM_SEVERITY_MEDIUM",
		"probabilityScore": 0.85,
		"severityScore": 0.65,
		"blocked": true
	}`

	var rating SafetyRating
	if err := json.Unmarshal([]byte(jsonData), &rating); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if rating.Category != HarmCategoryHarassment {
		t.Errorf("rating.Category = %s, want %s", rating.Category, HarmCategoryHarassment)
	}
	if rating.Probability != HarmProbabilityHigh {
		t.Errorf("rating.Probability = %s, want %s", rating.Probability, HarmProbabilityHigh)
	}
	if rating.Severity != HarmSeverityMedium {
		t.Errorf("rating.Severity = %s, want %s", rating.Severity, HarmSeverityMedium)
	}
	if rating.ProbabilityScore != 0.85 {
		t.Errorf("rating.ProbabilityScore = %f, want 0.85", rating.ProbabilityScore)
	}
	if rating.SeverityScore != 0.65 {
		t.Errorf("rating.SeverityScore = %f, want 0.65", rating.SeverityScore)
	}
	if !rating.Blocked {
		t.Error("rating.Blocked = false, want true")
	}
}

// TestCandidateIsSafetyBlocked tests the IsSafetyBlocked method
func TestCandidateIsSafetyBlocked(t *testing.T) {
	tests := []struct {
		name      string
		candidate Candidate
		want      bool
	}{
		{
			name: "blocked by finish reason",
			candidate: Candidate{
				FinishReason: "SAFETY",
			},
			want: true,
		},
		{
			name: "blocked by safety rating",
			candidate: Candidate{
				FinishReason: "STOP",
				SafetyRatings: []SafetyRating{
					{Category: HarmCategoryHarassment, Blocked: true},
				},
			},
			want: true,
		},
		{
			name: "not blocked",
			candidate: Candidate{
				FinishReason: "STOP",
				SafetyRatings: []SafetyRating{
					{Category: HarmCategoryHarassment, Blocked: false},
				},
			},
			want: false,
		},
		{
			name: "no safety ratings",
			candidate: Candidate{
				FinishReason: "STOP",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.candidate.IsSafetyBlocked()
			if got != tt.want {
				t.Errorf("Candidate.IsSafetyBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCandidateGetSafetyRating tests the GetSafetyRating method
func TestCandidateGetSafetyRating(t *testing.T) {
	candidate := Candidate{
		SafetyRatings: []SafetyRating{
			{Category: HarmCategoryHarassment, Probability: HarmProbabilityLow},
			{Category: HarmCategoryHateSpeech, Probability: HarmProbabilityMedium},
		},
	}

	// Test getting existing rating
	rating := candidate.GetSafetyRating(HarmCategoryHarassment)
	if rating == nil {
		t.Fatal("GetSafetyRating(HarmCategoryHarassment) returned nil")
	}
	if rating.Category != HarmCategoryHarassment {
		t.Errorf("rating.Category = %s, want %s", rating.Category, HarmCategoryHarassment)
	}
	if rating.Probability != HarmProbabilityLow {
		t.Errorf("rating.Probability = %s, want %s", rating.Probability, HarmProbabilityLow)
	}

	// Test getting non-existent rating
	rating = candidate.GetSafetyRating(HarmCategoryDangerousContent)
	if rating != nil {
		t.Errorf("GetSafetyRating(HarmCategoryDangerousContent) = %v, want nil", rating)
	}
}

// TestGenerateContentRequestWithSafetySettings tests that safety settings are properly included in requests
func TestGenerateContentRequestWithSafetySettings(t *testing.T) {
	req := GenerateContentRequest{
		Contents: []Content{
			{
				Role: "user",
				Parts: []Part{
					{Text: "Hello"},
				},
			},
		},
		SafetySettings: DefaultSafetySettings(),
	}

	// Marshal to JSON
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal back
	var decoded GenerateContentRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(decoded.SafetySettings) != 4 {
		t.Errorf("decoded request has %d safety settings, want 4", len(decoded.SafetySettings))
	}
}

// TestAllHarmCategoriesUnique tests that all harm category constants are unique
func TestAllHarmCategoriesUnique(t *testing.T) {
	categories := []HarmCategory{
		HarmCategoryUnspecified,
		HarmCategoryHarassment,
		HarmCategoryHateSpeech,
		HarmCategorySexuallyExplicit,
		HarmCategoryDangerousContent,
		HarmCategoryCivicIntegrity,
		HarmCategoryDerogatory,
		HarmCategoryToxicity,
		HarmCategoryViolence,
		HarmCategorySexual,
		HarmCategoryMedical,
		HarmCategoryDangerous,
	}

	seen := make(map[HarmCategory]bool)
	for _, category := range categories {
		if seen[category] {
			t.Errorf("duplicate harm category: %s", category)
		}
		seen[category] = true
	}
}

// TestAllHarmBlockThresholdsUnique tests that all threshold constants are unique
func TestAllHarmBlockThresholdsUnique(t *testing.T) {
	thresholds := []HarmBlockThreshold{
		HarmBlockThresholdUnspecified,
		HarmBlockThresholdBlockLowAndAbove,
		HarmBlockThresholdBlockMediumAndAbove,
		HarmBlockThresholdBlockOnlyHigh,
		HarmBlockThresholdBlockNone,
		HarmBlockThresholdOff,
	}

	seen := make(map[HarmBlockThreshold]bool)
	for _, threshold := range thresholds {
		if seen[threshold] {
			t.Errorf("duplicate harm block threshold: %s", threshold)
		}
		seen[threshold] = true
	}
}
