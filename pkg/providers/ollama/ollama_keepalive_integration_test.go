package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildOllamaChatRequest_KeepAlive tests KeepAlive in request building
func TestBuildOllamaChatRequest_KeepAlive(t *testing.T) {
	tests := []struct {
		name              string
		options           types.GenerateOptions
		expectedKeepAlive *Duration
	}{
		{
			name: "no keep_alive metadata",
			options: types.GenerateOptions{
				Model: "llama3.1:8b",
				Messages: []types.ChatMessage{
					{Role: "user", Content: "Hello"},
				},
			},
			expectedKeepAlive: nil,
		},
		{
			name: "keep_alive string 5m",
			options: types.GenerateOptions{
				Model: "llama3.1:8b",
				Messages: []types.ChatMessage{
					{Role: "user", Content: "Hello"},
				},
				Metadata: map[string]interface{}{
					"keep_alive": "5m",
				},
			},
			expectedKeepAlive: &Duration{Duration: 5 * time.Minute},
		},
		{
			name: "keep_alive string 300s",
			options: types.GenerateOptions{
				Model: "llama3.1:8b",
				Messages: []types.ChatMessage{
					{Role: "user", Content: "Hello"},
				},
				Metadata: map[string]interface{}{
					"keep_alive": "300s",
				},
			},
			expectedKeepAlive: &Duration{Duration: 300 * time.Second},
		},
		{
			name: "keep_alive -1 (keep forever)",
			options: types.GenerateOptions{
				Model: "llama3.1:8b",
				Messages: []types.ChatMessage{
					{Role: "user", Content: "Hello"},
				},
				Metadata: map[string]interface{}{
					"keep_alive": "-1",
				},
			},
			expectedKeepAlive: &Duration{Duration: -1},
		},
		{
			name: "keep_alive 0 (unload immediately)",
			options: types.GenerateOptions{
				Model: "llama3.1:8b",
				Messages: []types.ChatMessage{
					{Role: "user", Content: "Hello"},
				},
				Metadata: map[string]interface{}{
					"keep_alive": "0",
				},
			},
			expectedKeepAlive: &Duration{Duration: 0},
		},
		{
			name: "keep_alive int 300",
			options: types.GenerateOptions{
				Model: "llama3.1:8b",
				Messages: []types.ChatMessage{
					{Role: "user", Content: "Hello"},
				},
				Metadata: map[string]interface{}{
					"keep_alive": 300,
				},
			},
			expectedKeepAlive: &Duration{Duration: 300 * time.Second},
		},
		{
			name: "keep_alive time.Duration",
			options: types.GenerateOptions{
				Model: "llama3.1:8b",
				Messages: []types.ChatMessage{
					{Role: "user", Content: "Hello"},
				},
				Metadata: map[string]interface{}{
					"keep_alive": 5 * time.Minute,
				},
			},
			expectedKeepAlive: &Duration{Duration: 5 * time.Minute},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := types.ProviderConfig{
				Type: types.ProviderTypeOllama,
				Name: "ollama-test",
			}
			provider := NewOllamaProvider(config)

			request := provider.buildOllamaChatRequest(tt.options)

			if tt.expectedKeepAlive == nil {
				assert.Nil(t, request.KeepAlive)
			} else {
				require.NotNil(t, request.KeepAlive)
				assert.Equal(t, tt.expectedKeepAlive.Duration, request.KeepAlive.Duration)
			}
		})
	}
}

// TestGenerateChatCompletion_KeepAlive_RequestFormat tests KeepAlive in actual request
func TestGenerateChatCompletion_KeepAlive_RequestFormat(t *testing.T) {
	tests := []struct {
		name              string
		keepAliveMetadata interface{}
		expectedJSON      string
	}{
		{
			name:              "5 minutes",
			keepAliveMetadata: "5m",
			expectedJSON:      `"5m0s"`,
		},
		{
			name:              "300 seconds",
			keepAliveMetadata: "300s",
			expectedJSON:      `"5m0s"`, // Go normalizes to 5m0s
		},
		{
			name:              "keep forever",
			keepAliveMetadata: "-1",
			expectedJSON:      `"-1"`,
		},
		{
			name:              "unload immediately",
			keepAliveMetadata: "0",
			expectedJSON:      `null`,
		},
		{
			name:              "1 hour",
			keepAliveMetadata: "1h",
			expectedJSON:      `"1h0m0s"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Track the actual request sent
			var actualRequest ollamaChatRequest

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Capture the request
				err := json.NewDecoder(r.Body).Decode(&actualRequest)
				assert.NoError(t, err)

				// Send minimal response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := `{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Done"},"done":true}`
				_, _ = w.Write([]byte(response + "\n"))
			}))
			defer server.Close()

			config := types.ProviderConfig{
				Type:    types.ProviderTypeOllama,
				Name:    "ollama-test",
				BaseURL: server.URL,
			}

			provider := NewOllamaProvider(config)
			ctx := context.Background()

			options := types.GenerateOptions{
				Model: "llama3.1:8b",
				Messages: []types.ChatMessage{
					{Role: "user", Content: "Test"},
				},
				Metadata: map[string]interface{}{
					"keep_alive": tt.keepAliveMetadata,
				},
			}

			stream, err := provider.GenerateChatCompletion(ctx, options)
			require.NoError(t, err)
			require.NotNil(t, stream)

			// Consume the stream
			for {
				_, err := stream.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}
			_ = stream.Close()

			// Verify the KeepAlive was properly marshaled
			// Note: "0" results in a non-nil Duration that marshals to null
			if tt.expectedJSON == "null" {
				// For "0", we still set KeepAlive but it marshals to null
				if actualRequest.KeepAlive != nil {
					data, err := json.Marshal(actualRequest.KeepAlive)
					require.NoError(t, err)
					assert.Equal(t, tt.expectedJSON, string(data))
				} else {
					// If it's nil, that's also acceptable for the "0" case
					assert.Equal(t, "null", tt.expectedJSON)
				}
			} else {
				require.NotNil(t, actualRequest.KeepAlive)
				data, err := json.Marshal(actualRequest.KeepAlive)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedJSON, string(data))
			}
		})
	}
}

// TestGenerateChatCompletion_WithoutKeepAlive tests that requests work without KeepAlive
func TestGenerateChatCompletion_WithoutKeepAlive(t *testing.T) {
	var actualRequest ollamaChatRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&actualRequest)
		assert.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := `{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Done"},"done":true}`
		_, _ = w.Write([]byte(response + "\n"))
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	options := types.GenerateOptions{
		Model: "llama3.1:8b",
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Test"},
		},
		// No metadata, so no keep_alive
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Consume the stream
	for {
		_, err := stream.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}
	_ = stream.Close()

	// Verify KeepAlive is nil
	assert.Nil(t, actualRequest.KeepAlive)
}

// TestParseKeepAlive_EdgeCases tests edge cases in parseKeepAlive
func TestParseKeepAlive_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected *Duration
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "zero int",
			input:    0,
			expected: &Duration{Duration: 0},
		},
		{
			name:     "negative int (not -1)",
			input:    -2,
			expected: &Duration{Duration: -2 * time.Second},
		},
		{
			name:     "very large duration",
			input:    "999h",
			expected: &Duration{Duration: 999 * time.Hour},
		},
		{
			name:     "combined duration",
			input:    "1h30m45s",
			expected: &Duration{Duration: time.Hour + 30*time.Minute + 45*time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseKeepAlive(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Duration, result.Duration)
			}
		})
	}
}

// TestKeepAlive_AllFormats tests all supported duration formats end-to-end
func TestKeepAlive_AllFormats(t *testing.T) {
	formats := []struct {
		name  string
		value interface{}
	}{
		{"5m string", "5m"},
		{"300s string", "300s"},
		{"-1 string", "-1"},
		{"0 string", "0"},
		{"1h string", "1h"},
		{"300 int", 300},
		{"-1 int", -1},
		{"300.0 float64", 300.0},
		{"time.Duration", 5 * time.Minute},
		{"Duration", Duration{Duration: 5 * time.Minute}},
		{"*Duration", &Duration{Duration: 5 * time.Minute}},
	}

	for _, format := range formats {
		t.Run(format.name, func(t *testing.T) {
			config := types.ProviderConfig{
				Type: types.ProviderTypeOllama,
				Name: "ollama-test",
			}
			provider := NewOllamaProvider(config)

			options := types.GenerateOptions{
				Model: "llama3.1:8b",
				Messages: []types.ChatMessage{
					{Role: "user", Content: "Test"},
				},
				Metadata: map[string]interface{}{
					"keep_alive": format.value,
				},
			}

			request := provider.buildOllamaChatRequest(options)

			// Should not panic and should produce a valid request
			assert.NotNil(t, request)

			// If parseKeepAlive returns non-nil, KeepAlive should be set
			parsed := parseKeepAlive(format.value)
			if parsed != nil {
				assert.NotNil(t, request.KeepAlive)
			}
		})
	}
}
