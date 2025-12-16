package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestCreateBatch tests the CreateBatch method
func TestCreateBatch(t *testing.T) {
	tests := []struct {
		name           string
		requests       []BatchRequest
		mockResponse   string
		mockStatusCode int
		expectError    bool
		errorContains  string
	}{
		{
			name: "successful batch creation",
			requests: []BatchRequest{
				{
					CustomID: "request-1",
					Params: BatchRequestParams{
						Model:     "claude-sonnet-4-5",
						MaxTokens: 1024,
						Messages: []AnthropicMessage{
							{Role: "user", Content: "Hello, world"},
						},
					},
				},
			},
			mockResponse: `{
				"id": "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d",
				"type": "message_batch",
				"processing_status": "in_progress",
				"request_counts": {
					"processing": 1,
					"succeeded": 0,
					"errored": 0,
					"canceled": 0,
					"expired": 0
				},
				"created_at": "2024-09-24T18:37:24.100435Z",
				"expires_at": "2024-09-25T18:37:24.100435Z"
			}`,
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "empty batch request",
			requests:       []BatchRequest{},
			mockStatusCode: http.StatusOK,
			expectError:    true,
			errorContains:  "at least one request",
		},
		{
			name: "API error response",
			requests: []BatchRequest{
				{
					CustomID: "request-1",
					Params: BatchRequestParams{
						Model:     "claude-sonnet-4-5",
						MaxTokens: 1024,
						Messages: []AnthropicMessage{
							{Role: "user", Content: "Hello"},
						},
					},
				},
			},
			mockResponse: `{
				"type": "error",
				"error": {
					"type": "invalid_request_error",
					"message": "Invalid request"
				}
			}`,
			mockStatusCode: http.StatusBadRequest,
			expectError:    true,
			errorContains:  "Invalid request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				if r.Method != "POST" {
					t.Errorf("Expected POST request, got %s", r.Method)
				}
				if !strings.HasSuffix(r.URL.Path, "/v1/messages/batches") {
					t.Errorf("Expected /v1/messages/batches path, got %s", r.URL.Path)
				}

				// Verify headers
				if r.Header.Get("x-api-key") == "" && r.Header.Get("Authorization") == "" {
					t.Error("Expected authentication header")
				}

				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			// Create provider with mock server
			provider := NewAnthropicProvider(types.ProviderConfig{
				BaseURL: server.URL,
				APIKey:  "test-key",
			})

			// Call CreateBatch
			ctx := context.Background()
			batch, err := provider.CreateBatch(ctx, tt.requests)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if batch == nil {
					t.Error("Expected batch but got nil")
				} else {
					if batch.ID != "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d" {
						t.Errorf("Expected batch ID msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d, got %s", batch.ID)
					}
					if batch.ProcessingStatus != "in_progress" {
						t.Errorf("Expected processing status in_progress, got %s", batch.ProcessingStatus)
					}
				}
			}
		})
	}
}

// TestGetBatch tests the GetBatch method
func TestGetBatch(t *testing.T) {
	tests := []struct {
		name           string
		batchID        string
		mockResponse   string
		mockStatusCode int
		expectError    bool
		errorContains  string
	}{
		{
			name:    "successful batch retrieval",
			batchID: "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d",
			mockResponse: `{
				"id": "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d",
				"type": "message_batch",
				"processing_status": "ended",
				"request_counts": {
					"processing": 0,
					"succeeded": 2,
					"errored": 0,
					"canceled": 0,
					"expired": 0
				},
				"ended_at": "2024-09-24T19:37:24.100435Z",
				"created_at": "2024-09-24T18:37:24.100435Z",
				"expires_at": "2024-09-25T18:37:24.100435Z",
				"results_url": "https://api.anthropic.com/v1/messages/batches/msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d/results"
			}`,
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:          "empty batch ID",
			batchID:       "",
			expectError:   true,
			errorContains: "batch ID is required",
		},
		{
			name:    "batch not found",
			batchID: "msgbatch_invalid",
			mockResponse: `{
				"type": "error",
				"error": {
					"type": "not_found_error",
					"message": "Batch not found"
				}
			}`,
			mockStatusCode: http.StatusNotFound,
			expectError:    true,
			errorContains:  "Batch not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("Expected GET request, got %s", r.Method)
				}

				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			// Create provider with mock server
			provider := NewAnthropicProvider(types.ProviderConfig{
				BaseURL: server.URL,
				APIKey:  "test-key",
			})

			// Call GetBatch
			ctx := context.Background()
			batch, err := provider.GetBatch(ctx, tt.batchID)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if batch == nil {
					t.Error("Expected batch but got nil")
				} else {
					if batch.ProcessingStatus != "ended" {
						t.Errorf("Expected processing status ended, got %s", batch.ProcessingStatus)
					}
					if batch.RequestCounts.Succeeded != 2 {
						t.Errorf("Expected 2 succeeded requests, got %d", batch.RequestCounts.Succeeded)
					}
					if batch.ResultsURL == nil {
						t.Error("Expected results URL but got nil")
					}
				}
			}
		})
	}
}

// TestListBatches tests the ListBatches method
func TestListBatches(t *testing.T) {
	tests := []struct {
		name           string
		limit          int
		beforeID       string
		afterID        string
		mockResponse   string
		mockStatusCode int
		expectError    bool
		expectedCount  int
	}{
		{
			name:  "successful batch listing",
			limit: 20,
			mockResponse: `{
				"data": [
					{
						"id": "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d",
						"type": "message_batch",
						"processing_status": "ended",
						"request_counts": {
							"processing": 0,
							"succeeded": 2,
							"errored": 0,
							"canceled": 0,
							"expired": 0
						},
						"created_at": "2024-09-24T18:37:24.100435Z",
						"expires_at": "2024-09-25T18:37:24.100435Z"
					},
					{
						"id": "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8e",
						"type": "message_batch",
						"processing_status": "in_progress",
						"request_counts": {
							"processing": 5,
							"succeeded": 0,
							"errored": 0,
							"canceled": 0,
							"expired": 0
						},
						"created_at": "2024-09-24T19:37:24.100435Z",
						"expires_at": "2024-09-25T19:37:24.100435Z"
					}
				],
				"has_more": false,
				"first_id": "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d",
				"last_id": "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8e"
			}`,
			mockStatusCode: http.StatusOK,
			expectError:    false,
			expectedCount:  2,
		},
		{
			name:     "batch listing with pagination",
			limit:    10,
			afterID:  "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d",
			mockResponse: `{
				"data": [
					{
						"id": "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8e",
						"type": "message_batch",
						"processing_status": "in_progress",
						"request_counts": {
							"processing": 5,
							"succeeded": 0,
							"errored": 0,
							"canceled": 0,
							"expired": 0
						},
						"created_at": "2024-09-24T19:37:24.100435Z",
						"expires_at": "2024-09-25T19:37:24.100435Z"
					}
				],
				"has_more": true,
				"first_id": "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8e",
				"last_id": "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8e"
			}`,
			mockStatusCode: http.StatusOK,
			expectError:    false,
			expectedCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("Expected GET request, got %s", r.Method)
				}

				// Verify query parameters
				query := r.URL.Query()
				if tt.limit > 0 {
					limit := query.Get("limit")
					if limit == "" {
						t.Error("Expected limit parameter")
					}
				}
				if tt.afterID != "" {
					afterID := query.Get("after_id")
					if afterID != tt.afterID {
						t.Errorf("Expected after_id %s, got %s", tt.afterID, afterID)
					}
				}

				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			// Create provider with mock server
			provider := NewAnthropicProvider(types.ProviderConfig{
				BaseURL: server.URL,
				APIKey:  "test-key",
			})

			// Call ListBatches
			ctx := context.Background()
			response, err := provider.ListBatches(ctx, tt.limit, tt.beforeID, tt.afterID)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if response == nil {
					t.Error("Expected response but got nil")
				} else {
					if len(response.Data) != tt.expectedCount {
						t.Errorf("Expected %d batches, got %d", tt.expectedCount, len(response.Data))
					}
				}
			}
		})
	}
}

// TestCancelBatch tests the CancelBatch method
func TestCancelBatch(t *testing.T) {
	tests := []struct {
		name           string
		batchID        string
		mockResponse   string
		mockStatusCode int
		expectError    bool
		errorContains  string
	}{
		{
			name:    "successful batch cancellation",
			batchID: "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d",
			mockResponse: `{
				"id": "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d",
				"type": "message_batch",
				"processing_status": "canceling",
				"request_counts": {
					"processing": 2,
					"succeeded": 0,
					"errored": 0,
					"canceled": 0,
					"expired": 0
				},
				"created_at": "2024-09-24T18:37:24.100435Z",
				"expires_at": "2024-09-25T18:37:24.100435Z",
				"cancel_initiated_at": "2024-09-24T18:39:03.114875Z"
			}`,
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:          "empty batch ID",
			batchID:       "",
			expectError:   true,
			errorContains: "batch ID is required",
		},
		{
			name:    "batch already ended",
			batchID: "msgbatch_ended",
			mockResponse: `{
				"type": "error",
				"error": {
					"type": "invalid_request_error",
					"message": "Batch has already ended"
				}
			}`,
			mockStatusCode: http.StatusBadRequest,
			expectError:    true,
			errorContains:  "already ended",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("Expected POST request, got %s", r.Method)
				}
				if !strings.Contains(r.URL.Path, "/cancel") {
					t.Errorf("Expected /cancel path, got %s", r.URL.Path)
				}

				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			// Create provider with mock server
			provider := NewAnthropicProvider(types.ProviderConfig{
				BaseURL: server.URL,
				APIKey:  "test-key",
			})

			// Call CancelBatch
			ctx := context.Background()
			batch, err := provider.CancelBatch(ctx, tt.batchID)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if batch == nil {
					t.Error("Expected batch but got nil")
				} else {
					if batch.ProcessingStatus != "canceling" {
						t.Errorf("Expected processing status canceling, got %s", batch.ProcessingStatus)
					}
					if batch.CancelInitiatedAt == nil {
						t.Error("Expected cancel_initiated_at to be set")
					}
				}
			}
		})
	}
}

// TestStreamBatchResults tests the StreamBatchResults method
func TestStreamBatchResults(t *testing.T) {
	tests := []struct {
		name           string
		batchID        string
		mockResponse   string
		mockStatusCode int
		expectError    bool
		errorContains  string
		expectedCount  int
	}{
		{
			name:    "successful result streaming",
			batchID: "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d",
			mockResponse: `{"custom_id":"request-1","result":{"type":"succeeded","message":{"id":"msg_01FqfsLoHwgeFbguDgpz48m7","type":"message","role":"assistant","model":"claude-sonnet-4-5-20250929","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}}}
{"custom_id":"request-2","result":{"type":"succeeded","message":{"id":"msg_01FqfsLoHwgeFbguDgpz48m8","type":"message","role":"assistant","model":"claude-sonnet-4-5-20250929","content":[{"type":"text","text":"Hi again!"}],"stop_reason":"end_turn","usage":{"input_tokens":11,"output_tokens":6}}}}`,
			mockStatusCode: http.StatusOK,
			expectError:    false,
			expectedCount:  2,
		},
		{
			name:          "empty batch ID",
			batchID:       "",
			expectError:   true,
			errorContains: "batch ID is required",
		},
		{
			name:    "batch results not ready",
			batchID: "msgbatch_processing",
			mockResponse: `{
				"type": "error",
				"error": {
					"type": "invalid_request_error",
					"message": "Batch is still processing"
				}
			}`,
			mockStatusCode: http.StatusBadRequest,
			expectError:    true,
			errorContains:  "still processing",
		},
		{
			name:    "results with errors",
			batchID: "msgbatch_with_errors",
			mockResponse: `{"custom_id":"request-1","result":{"type":"succeeded","message":{"id":"msg_01","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"Success"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}}}
{"custom_id":"request-2","result":{"type":"errored","error":{"type":"invalid_request","message":"Invalid parameters"}}}
{"custom_id":"request-3","result":{"type":"expired"}}`,
			mockStatusCode: http.StatusOK,
			expectError:    false,
			expectedCount:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("Expected GET request, got %s", r.Method)
				}
				if !strings.Contains(r.URL.Path, "/results") {
					t.Errorf("Expected /results path, got %s", r.URL.Path)
				}

				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			// Create provider with mock server
			provider := NewAnthropicProvider(types.ProviderConfig{
				BaseURL: server.URL,
				APIKey:  "test-key",
			})

			// Call StreamBatchResults
			ctx := context.Background()
			resultCount := 0
			err := provider.StreamBatchResults(ctx, tt.batchID, func(result BatchResult) error {
				resultCount++

				// Verify result structure
				if result.CustomID == "" {
					t.Error("Expected custom_id to be set")
				}
				if result.Result.Type == "" {
					t.Error("Expected result type to be set")
				}

				// Verify result types
				switch result.Result.Type {
				case "succeeded":
					if result.Result.Message == nil {
						t.Error("Expected message for succeeded result")
					}
				case "errored":
					if result.Result.Error == nil {
						t.Error("Expected error for errored result")
					}
				case "expired", "canceled":
					// These don't have additional data
				default:
					t.Errorf("Unexpected result type: %s", result.Result.Type)
				}

				return nil
			})

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if resultCount != tt.expectedCount {
					t.Errorf("Expected %d results, got %d", tt.expectedCount, resultCount)
				}
			}
		})
	}
}

// TestBatchResultParsing tests parsing of different batch result types
func TestBatchResultParsing(t *testing.T) {
	tests := []struct {
		name       string
		jsonLine   string
		expectType string
		hasMessage bool
		hasError   bool
	}{
		{
			name:       "succeeded result",
			jsonLine:   `{"custom_id":"test-1","result":{"type":"succeeded","message":{"id":"msg_01","type":"message","role":"assistant","content":[{"type":"text","text":"Hello"}],"model":"claude-sonnet-4-5","usage":{"input_tokens":10,"output_tokens":5}}}}`,
			expectType: "succeeded",
			hasMessage: true,
			hasError:   false,
		},
		{
			name:       "errored result",
			jsonLine:   `{"custom_id":"test-2","result":{"type":"errored","error":{"type":"invalid_request","message":"Bad request"}}}`,
			expectType: "errored",
			hasMessage: false,
			hasError:   true,
		},
		{
			name:       "expired result",
			jsonLine:   `{"custom_id":"test-3","result":{"type":"expired"}}`,
			expectType: "expired",
			hasMessage: false,
			hasError:   false,
		},
		{
			name:       "canceled result",
			jsonLine:   `{"custom_id":"test-4","result":{"type":"canceled"}}`,
			expectType: "canceled",
			hasMessage: false,
			hasError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result BatchResult
			err := json.Unmarshal([]byte(tt.jsonLine), &result)
			if err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			if result.Result.Type != tt.expectType {
				t.Errorf("Expected type %s, got %s", tt.expectType, result.Result.Type)
			}

			if tt.hasMessage && result.Result.Message == nil {
				t.Error("Expected message to be present")
			}
			if !tt.hasMessage && result.Result.Message != nil {
				t.Error("Expected message to be nil")
			}

			if tt.hasError && result.Result.Error == nil {
				t.Error("Expected error to be present")
			}
			if !tt.hasError && result.Result.Error != nil {
				t.Error("Expected error to be nil")
			}
		})
	}
}

// TestBatchRequestValidation tests validation of batch requests
func TestBatchRequestValidation(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		APIKey: "test-key",
	})

	ctx := context.Background()

	// Test empty request list
	_, err := provider.CreateBatch(ctx, []BatchRequest{})
	if err == nil {
		t.Error("Expected error for empty request list")
	}

	// Test too many requests (simulated - would need to generate 100,001 requests in real test)
	tooManyRequests := make([]BatchRequest, 100001)
	for i := 0; i < 100001; i++ {
		tooManyRequests[i] = BatchRequest{
			CustomID: fmt.Sprintf("request-%d", i),
			Params: BatchRequestParams{
				Model:     "claude-sonnet-4-5",
				MaxTokens: 1024,
				Messages: []AnthropicMessage{
					{Role: "user", Content: "test"},
				},
			},
		}
	}
	_, err = provider.CreateBatch(ctx, tooManyRequests)
	if err == nil {
		t.Error("Expected error for too many requests")
	}
	if !strings.Contains(err.Error(), "100,000") {
		t.Errorf("Expected error about 100,000 limit, got: %v", err)
	}
}

// TestBatchTimestampParsing tests parsing of timestamps in batch responses
func TestBatchTimestampParsing(t *testing.T) {
	jsonData := `{
		"id": "msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d",
		"type": "message_batch",
		"processing_status": "ended",
		"request_counts": {
			"processing": 0,
			"succeeded": 2,
			"errored": 0,
			"canceled": 0,
			"expired": 0
		},
		"ended_at": "2024-09-24T19:37:24.100435Z",
		"created_at": "2024-09-24T18:37:24.100435Z",
		"expires_at": "2024-09-25T18:37:24.100435Z",
		"cancel_initiated_at": null,
		"results_url": "https://api.anthropic.com/v1/messages/batches/msgbatch_01HkcTjaV5uDC8jWR4ZsDV8d/results"
	}`

	var batch MessageBatch
	err := json.Unmarshal([]byte(jsonData), &batch)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify timestamps
	if batch.CreatedAt.IsZero() {
		t.Error("Expected created_at to be parsed")
	}
	if batch.ExpiresAt.IsZero() {
		t.Error("Expected expires_at to be parsed")
	}
	if batch.EndedAt == nil {
		t.Error("Expected ended_at to be parsed")
	}
	if batch.CancelInitiatedAt != nil {
		t.Error("Expected cancel_initiated_at to be nil")
	}

	// Verify time difference (expires_at should be 24 hours after created_at)
	expectedExpiry := batch.CreatedAt.Add(24 * time.Hour)
	if !batch.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("Expected expires_at to be 24 hours after created_at")
	}
}
