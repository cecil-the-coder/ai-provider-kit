package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestGetQuotaSuccess tests successful quota retrieval
func TestGetQuotaSuccess(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Verify path (the mock server URL doesn't include /v1internal prefix)
		expectedPath := "/projects/test-project-id:getQuota"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Expected authorization header 'Bearer test-token', got '%s'", auth)
		}

		// Return mock response
		response := GetQuotaResponse{
			Project: "test-project-id",
			Tier:    "free-tier",
			Quotas: []QuotaLimit{
				{
					Type:      "requests",
					Limit:     15,
					Usage:     5,
					Remaining: 10,
					Period:    "minute",
					ResetTime: time.Now().Add(time.Minute).Format(time.RFC3339),
				},
				{
					Type:      "tokens",
					Limit:     1000000,
					Usage:     500000,
					Remaining: 500000,
					Period:    "day",
					ResetTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
				},
			},
			CustomData: map[string]interface{}{
				"user_tier_name": "Free Tier",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock server
	client := &http.Client{}
	codeAssistClient := NewCodeAssistClient(client, "test-token")
	codeAssistClient.baseURL = server.URL

	// Make request
	req := GetQuotaRequest{
		Project: "test-project-id",
		Model:   "gemini-2.5-flash",
	}

	resp, err := codeAssistClient.GetQuota(context.Background(), "test-project-id", req)
	if err != nil {
		t.Fatalf("GetQuota failed: %v", err)
	}

	// Verify response
	if resp.Project != "test-project-id" {
		t.Errorf("Expected project 'test-project-id', got '%s'", resp.Project)
	}

	if resp.Tier != "free-tier" {
		t.Errorf("Expected tier 'free-tier', got '%s'", resp.Tier)
	}

	if len(resp.Quotas) != 2 {
		t.Fatalf("Expected 2 quotas, got %d", len(resp.Quotas))
	}

	// Verify request quota
	requestQuota := resp.Quotas[0]
	if requestQuota.Type != "requests" {
		t.Errorf("Expected quota type 'requests', got '%s'", requestQuota.Type)
	}
	if requestQuota.Limit != 15 {
		t.Errorf("Expected limit 15, got %d", requestQuota.Limit)
	}
	if requestQuota.Remaining != 10 {
		t.Errorf("Expected remaining 10, got %d", requestQuota.Remaining)
	}

	// Verify custom data
	if resp.CustomData["user_tier_name"] != "Free Tier" {
		t.Errorf("Expected custom data 'user_tier_name' to be 'Free Tier', got '%v'", resp.CustomData["user_tier_name"])
	}
}

// TestGetQuotaNotFound tests quota endpoint returning 404
func TestGetQuotaNotFound(t *testing.T) {
	// Create a mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Create client with mock server
	client := &http.Client{}
	codeAssistClient := NewCodeAssistClient(client, "test-token")
	codeAssistClient.baseURL = server.URL

	// Make request
	req := GetQuotaRequest{}

	resp, err := codeAssistClient.GetQuota(context.Background(), "test-project-id", req)
	if err != nil {
		t.Fatalf("GetQuota should not fail on 404: %v", err)
	}

	// Verify empty response
	if resp.Project != "test-project-id" {
		t.Errorf("Expected project 'test-project-id', got '%s'", resp.Project)
	}

	if resp.Tier != "unknown" {
		t.Errorf("Expected tier 'unknown', got '%s'", resp.Tier)
	}

	if len(resp.Quotas) != 0 {
		t.Errorf("Expected empty quotas, got %d", len(resp.Quotas))
	}
}

// TestGetQuotaUsageSuccess tests successful usage retrieval
func TestGetQuotaUsageSuccess(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Return mock response
		response := GetQuotaUsageResponse{
			Records: []UsageRecord{
				{
					Timestamp:    time.Now().Add(-time.Hour).Format(time.RFC3339),
					Model:        "gemini-2.5-flash",
					InputTokens:  1000,
					OutputTokens: 500,
					TotalTokens:  1500,
					RequestCount: 1,
					Operation:    "generateContent",
				},
			},
			Summary: UsageSummary{
				TotalInputTokens:  5000,
				TotalOutputTokens: 2500,
				TotalTokens:       7500,
				TotalRequests:     5,
				StartTime:         time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
				EndTime:           time.Now().Format(time.RFC3339),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock server
	client := &http.Client{}
	codeAssistClient := NewCodeAssistClient(client, "test-token")
	codeAssistClient.baseURL = server.URL

	// Make request
	req := GetQuotaUsageRequest{
		StartTime: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Format(time.RFC3339),
	}

	resp, err := codeAssistClient.GetQuotaUsage(context.Background(), "test-project-id", req)
	if err != nil {
		t.Fatalf("GetQuotaUsage failed: %v", err)
	}

	// Verify response
	if len(resp.Records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(resp.Records))
	}

	record := resp.Records[0]
	if record.InputTokens != 1000 {
		t.Errorf("Expected 1000 input tokens, got %d", record.InputTokens)
	}

	if resp.Summary.TotalTokens != 7500 {
		t.Errorf("Expected 7500 total tokens, got %d", resp.Summary.TotalTokens)
	}
}

// TestGetQuotaHistorySuccess tests successful history retrieval
func TestGetQuotaHistorySuccess(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Return mock response
		response := GetQuotaHistoryResponse{
			Records: []UsageRecord{
				{
					Timestamp:    time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
					Model:        "gemini-2.5-flash",
					InputTokens:  2000,
					OutputTokens: 1000,
					TotalTokens:  3000,
					RequestCount: 1,
					Operation:    "generateContent",
				},
				{
					Timestamp:    time.Now().Add(-time.Hour).Format(time.RFC3339),
					Model:        "gemini-2.5-flash",
					InputTokens:  1500,
					OutputTokens: 750,
					TotalTokens:  2250,
					RequestCount: 1,
					Operation:    "streamGenerateContent",
				},
			},
			Summary: UsageSummary{
				TotalInputTokens:  3500,
				TotalOutputTokens: 1750,
				TotalTokens:       5250,
				TotalRequests:     2,
				StartTime:         time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
				EndTime:           time.Now().Format(time.RFC3339),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock server
	client := &http.Client{}
	codeAssistClient := NewCodeAssistClient(client, "test-token")
	codeAssistClient.baseURL = server.URL

	// Make request
	req := GetQuotaHistoryRequest{
		StartTime: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Format(time.RFC3339),
		Limit:     100,
	}

	resp, err := codeAssistClient.GetQuotaHistory(context.Background(), "test-project-id", req)
	if err != nil {
		t.Fatalf("GetQuotaHistory failed: %v", err)
	}

	// Verify response
	if len(resp.Records) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(resp.Records))
	}

	if resp.Summary.TotalRequests != 2 {
		t.Errorf("Expected 2 total requests, got %d", resp.Summary.TotalRequests)
	}
}

// TestConvertToQuotaInfo tests conversion from GetQuotaResponse to types.QuotaInfo
func TestConvertToQuotaInfo(t *testing.T) {
	client := &http.Client{}
	codeAssistClient := NewCodeAssistClient(client, "test-token")

	resp := &GetQuotaResponse{
		Project: "test-project-id",
		Tier:    "free-tier",
		Quotas: []QuotaLimit{
			{
				Type:      "requests",
				Limit:     15,
				Usage:     5,
				Remaining: 10,
				Period:    "minute",
				ResetTime: time.Now().Add(time.Minute).Format(time.RFC3339),
			},
			{
				Type:      "input_tokens",
				Limit:     1000000,
				Usage:     500000,
				Remaining: 500000,
				Period:    "day",
				ResetTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			},
			{
				Type:      "output_tokens",
				Limit:     500000,
				Usage:     250000,
				Remaining: 250000,
				Period:    "day",
				ResetTime: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			},
		},
		CustomData: map[string]interface{}{
			"user_tier_name": "Free Tier",
		},
	}

	quotaInfo := codeAssistClient.ConvertToQuotaInfo(resp, "gemini-2.5-flash")

	if quotaInfo == nil {
		t.Fatalf("ConvertToQuotaInfo returned nil")
	}

	if quotaInfo.Provider != "gemini" {
		t.Errorf("Expected provider 'gemini', got '%s'", quotaInfo.Provider)
	}

	if quotaInfo.Model != "gemini-2.5-flash" {
		t.Errorf("Expected model 'gemini-2.5-flash', got '%s'", quotaInfo.Model)
	}

	// Check metadata
	if quotaInfo.Metadata["project"] != "test-project-id" {
		t.Errorf("Expected project metadata 'test-project-id', got '%v'", quotaInfo.Metadata["project"])
	}

	if quotaInfo.Metadata["tier"] != "free-tier" {
		t.Errorf("Expected tier metadata 'free-tier', got '%v'", quotaInfo.Metadata["tier"])
	}

	// Check custom usage
	if quotaInfo.CustomUsage["user_tier_name"] != "Free Tier" {
		t.Errorf("Expected custom usage 'user_tier_name' to be 'Free Tier', got '%v'", quotaInfo.CustomUsage["user_tier_name"])
	}

	// Check quotas
	if len(quotaInfo.Quotas) != 3 {
		t.Errorf("Expected 3 quotas, got %d", len(quotaInfo.Quotas))
	}

	// Check request quota
	requestQuota, exists := quotaInfo.Quotas["requests"]
	if !exists {
		t.Error("Request quota not found")
	} else {
		if requestQuota.Limit != 15 {
			t.Errorf("Expected request limit 15, got %d", requestQuota.Limit)
		}
		if requestQuota.Remaining != 10 {
			t.Errorf("Expected request remaining 10, got %d", requestQuota.Remaining)
		}
		if requestQuota.RemainingPercent != 66.66666666666666 { // 10/15 * 100
			t.Errorf("Expected remaining percent 66.67, got %f", requestQuota.RemainingPercent)
		}
	}

	// Check input token quota
	inputQuota, exists := quotaInfo.Quotas["input_tokens"]
	if !exists {
		t.Error("Input tokens quota not found")
	} else if inputQuota.Limit != 1000000 {
		t.Errorf("Expected input token limit 1000000, got %d", inputQuota.Limit)
	}

	// Check output token quota
	outputQuota, exists := quotaInfo.Quotas["output_tokens"]
	if !exists {
		t.Error("Output tokens quota not found")
	} else if outputQuota.Limit != 500000 {
		t.Errorf("Expected output token limit 500000, got %d", outputQuota.Limit)
	}
}

// TestConvertToQuotaHistory tests conversion from GetQuotaHistoryResponse to types.QuotaHistory
func TestConvertToQuotaHistory(t *testing.T) {
	client := &http.Client{}
	codeAssistClient := NewCodeAssistClient(client, "test-token")

	now := time.Now()
	resp := &GetQuotaHistoryResponse{
		Records: []UsageRecord{
			{
				Timestamp:    now.Add(-time.Hour).Format(time.RFC3339),
				Model:        "gemini-2.5-flash",
				InputTokens:  1000,
				OutputTokens: 500,
				TotalTokens:  1500,
				RequestCount: 1,
				Operation:    "generateContent",
			},
		},
		Summary: UsageSummary{
			TotalInputTokens:  1000,
			TotalOutputTokens: 500,
			TotalTokens:       1500,
			TotalRequests:     1,
			StartTime:         now.Add(-24 * time.Hour).Format(time.RFC3339),
			EndTime:           now.Format(time.RFC3339),
		},
	}

	history := codeAssistClient.ConvertToQuotaHistory(resp, "test-project-id", "gemini-2.5-flash")

	if history == nil {
		t.Fatalf("ConvertToQuotaHistory returned nil")
	}

	if history.Provider != "gemini" {
		t.Errorf("Expected provider 'gemini', got '%s'", history.Provider)
	}

	if history.Model != "gemini-2.5-flash" {
		t.Errorf("Expected model 'gemini-2.5-flash', got '%s'", history.Model)
	}

	if len(history.Records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(history.Records))
	}

	if history.TotalUsage["tokens"] != 1500 {
		t.Errorf("Expected total tokens 1500, got %d", history.TotalUsage["tokens"])
	}

	if history.TotalUsage["input_tokens"] != 1000 {
		t.Errorf("Expected total input tokens 1000, got %d", history.TotalUsage["input_tokens"])
	}

	if history.TotalUsage["output_tokens"] != 500 {
		t.Errorf("Expected total output tokens 500, got %d", history.TotalUsage["output_tokens"])
	}
}

// TestSupportsQuotaReporting tests the SupportsQuotaReporting method
func TestSupportsQuotaReporting(t *testing.T) {
	// With token
	clientWithToken := NewCodeAssistClient(&http.Client{}, "test-token")
	if !clientWithToken.SupportsQuotaReporting() {
		t.Error("Expected SupportsQuotaReporting to return true with token")
	}

	// Without token
	clientWithoutToken := NewCodeAssistClient(&http.Client{}, "")
	if clientWithoutToken.SupportsQuotaReporting() {
		t.Error("Expected SupportsQuotaReporting to return false without token")
	}
}

// TestSupportsQuotaHistory tests the SupportsQuotaHistory method
func TestSupportsQuotaHistory(t *testing.T) {
	// With token
	clientWithToken := NewCodeAssistClient(&http.Client{}, "test-token")
	if !clientWithToken.SupportsQuotaHistory() {
		t.Error("Expected SupportsQuotaHistory to return true with token")
	}

	// Without token
	clientWithoutToken := NewCodeAssistClient(&http.Client{}, "")
	if clientWithoutToken.SupportsQuotaHistory() {
		t.Error("Expected SupportsQuotaHistory to return false without token")
	}
}

// TestProviderSupportsQuotaReporting tests the GeminiProvider SupportsQuotaReporting method
func TestProviderSupportsQuotaReporting(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeGemini,
		ProviderConfig: map[string]interface{}{
			"backend":    BackendCodeAssist,
			"project":    "test-project-id",
			"project_id": "test-project-id",
		},
	}

	provider := NewGeminiProvider(config)

	// Not code assist backend
	if provider.SupportsQuotaReporting() {
		t.Error("Expected SupportsQuotaReporting to return false for non-CodeAssist backend by default")
	}

	// The actual setup would require OAuth credentials
	// In practice, this would be true only when OAuth is configured
}

// TestProviderGetQuotaInfoNotSupported tests GetQuotaInfo when not using Code Assist backend
func TestProviderGetQuotaInfoNotSupported(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeGemini,
		ProviderConfig: map[string]interface{}{
			"api_key": "test-key",
		},
	}

	provider := NewGeminiProvider(config)
	provider.Authenticate(context.Background(), types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "test-key",
	})

	_, err := provider.GetQuotaInfo(context.Background(), "gemini-2.5-flash")
	if err == nil {
		t.Error("Expected error when not using Code Assist backend")
	}

	expected := "quota reporting is only supported for Code Assist backend"
	if err != nil && !contains(err.Error(), expected) {
		t.Errorf("Expected error message to contain '%s', got '%s'", expected, err.Error())
	}
}

// TestProviderGetQuotaHistoryNotSupported tests GetQuotaHistory when not using Code Assist backend
func TestProviderGetQuotaHistoryNotSupported(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeGemini,
		ProviderConfig: map[string]interface{}{
			"api_key": "test-key",
		},
	}

	provider := NewGeminiProvider(config)
	provider.Authenticate(context.Background(), types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "test-key",
	})

	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	_, err := provider.GetQuotaHistory(context.Background(), "gemini-2.5-flash", startTime, endTime)
	if err == nil {
		t.Error("Expected error when not using Code Assist backend")
	}

	expected := "quota history is only supported for Code Assist backend"
	if err != nil && !contains(err.Error(), expected) {
		t.Errorf("Expected error message to contain '%s', got '%s'", expected, err.Error())
	}
}

// TestProviderConvertToQuotaInfo tests the provider's convertToQuotaInfo method
func TestProviderConvertToQuotaInfo(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeGemini,
		ProviderConfig: map[string]interface{}{
			"backend":    BackendCodeAssist,
			"project_id": "test-project-id",
		},
	}

	provider := NewGeminiProvider(config)
	provider.projectID = "test-project-id"

	resp := &GetQuotaResponse{
		Project: "test-project-id",
		Tier:    "free-tier",
		Quotas: []QuotaLimit{
			{
				Type:      "requests",
				Limit:     15,
				Usage:     5,
				Remaining: 10,
				Period:    "minute",
				ResetTime: time.Now().Add(time.Minute).Format(time.RFC3339),
			},
		},
	}

	quotaInfo := provider.convertToQuotaInfo(resp, "gemini-2.5-flash")

	if quotaInfo == nil {
		t.Fatalf("convertToQuotaInfo returned nil")
	}

	if quotaInfo.Provider != "gemini" {
		t.Errorf("Expected provider 'gemini', got '%s'", quotaInfo.Provider)
	}

	if quotaInfo.ProviderType != types.ProviderTypeGemini {
		t.Errorf("Expected provider type '%s', got '%s'", types.ProviderTypeGemini, quotaInfo.ProviderType)
	}

	// Check that request quota exists
	requestQuota, exists := quotaInfo.Quotas[types.QuotaTypeRequests]
	if !exists {
		t.Error("Request quota not found")
	} else {
		if requestQuota.Limit != 15 {
			t.Errorf("Expected request limit 15, got %d", requestQuota.Limit)
		}
		if requestQuota.Remaining != 10 {
			t.Errorf("Expected request remaining 10, got %d", requestQuota.Remaining)
		}
	}
}

// TestProviderConvertToQuotaHistory tests the provider's convertToQuotaHistory method
func TestProviderConvertToQuotaHistory(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeGemini,
		ProviderConfig: map[string]interface{}{
			"backend":    BackendCodeAssist,
			"project_id": "test-project-id",
		},
	}

	provider := NewGeminiProvider(config)

	now := time.Now()
	resp := &GetQuotaHistoryResponse{
		Records: []UsageRecord{
			{
				Timestamp:    now.Add(-time.Hour).Format(time.RFC3339),
				Model:        "gemini-2.5-flash",
				InputTokens:  1000,
				OutputTokens: 500,
				TotalTokens:  1500,
				RequestCount: 1,
				Operation:    "generateContent",
			},
		},
		Summary: UsageSummary{
			TotalInputTokens:  1000,
			TotalOutputTokens: 500,
			TotalTokens:       1500,
			TotalRequests:     1,
			StartTime:         now.Add(-24 * time.Hour).Format(time.RFC3339),
			EndTime:           now.Format(time.RFC3339),
		},
	}

	history := provider.convertToQuotaHistory(resp, "test-project-id", "gemini-2.5-flash")

	if history == nil {
		t.Fatalf("convertToQuotaHistory returned nil")
	}

	if history.Provider != "gemini" {
		t.Errorf("Expected provider 'gemini', got '%s'", history.Provider)
	}

	if history.Model != "gemini-2.5-flash" {
		t.Errorf("Expected model 'gemini-2.5-flash', got '%s'", history.Model)
	}

	// Check that the record exists with the correct model
	if len(history.Records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(history.Records))
	}

	record := history.Records[0]
	if record.Model != "gemini-2.5-flash" {
		t.Errorf("Expected record model 'gemini-2.5-flash', got '%s'", record.Model)
	}

	// Check total usage uses types.QuotaType
	if history.TotalUsage[types.QuotaTypeRequests] != 1 {
		t.Errorf("Expected total requests 1, got %d", history.TotalUsage[types.QuotaTypeRequests])
	}

	if history.TotalUsage[types.QuotaTypeTokens] != 1500 {
		t.Errorf("Expected total tokens 1500, got %d", history.TotalUsage[types.QuotaTypeTokens])
	}
}

// TestQuotaTypeMapping tests that quota types are properly mapped
func TestQuotaTypeMapping(t *testing.T) {
	tests := []struct {
		quotationType string
	}{
		{"requests"},
		{"api_requests"},
		{"tokens"},
		{"total_tokens"},
		{"input_tokens"},
		{"output_tokens"},
		{"daily_tokens"},
		{"daily_requests"},
		{"custom_type"},
	}

	client := NewCodeAssistClient(&http.Client{}, "test-token")

	for _, tt := range tests {
		t.Run(tt.quotationType, func(t *testing.T) {
			resp := &GetQuotaResponse{
				Project: "test-project-id",
				Tier:    "free-tier",
				Quotas: []QuotaLimit{
					{
						Type:      tt.quotationType,
						Limit:     100,
						Usage:     50,
						Remaining: 50,
						Period:    "day",
					},
				},
			}

			quotaInfo := client.ConvertToQuotaInfo(resp, "gemini-2.5-flash")

			// Check that a quota was created for the type
			if len(quotaInfo.Quotas) == 0 {
				t.Errorf("Expected at least one quota for type %s", tt.quotationType)
			}
		})
	}
}

// TestQuotaPeriodMapping tests that quota periods are properly mapped
func TestQuotaPeriodMapping(t *testing.T) {
	tests := []struct {
		apiPeriod string
	}{
		{"minute"},
		{"1m"},
		{"hour"},
		{"1h"},
		{"day"},
		{"1d"},
		{"daily"},
		{"week"},
		{"1w"},
		{"month"},
		{"1M"},
		{"custom"},
	}

	client := NewCodeAssistClient(&http.Client{}, "test-token")

	for _, tt := range tests {
		t.Run(tt.apiPeriod, func(t *testing.T) {
			resp := &GetQuotaResponse{
				Project: "test-project-id",
				Tier:    "free-tier",
				Quotas: []QuotaLimit{
					{
						Type:      "requests",
						Limit:     100,
						Usage:     50,
						Remaining: 50,
						Period:    tt.apiPeriod,
					},
				},
			}

			quotaInfo := client.ConvertToQuotaInfo(resp, "gemini-2.5-flash")

			// Find the quota and check it has a valid period
			if len(quotaInfo.Quotas) == 0 {
				t.Error("Expected at least one quota")
			} else {
				// Just verify that the period was parsed successfully
				for _, quota := range quotaInfo.Quotas {
					if quota.Period == "" {
						t.Errorf("Expected non-empty period for API period %s", tt.apiPeriod)
					}
					break // Only one quota in test
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
