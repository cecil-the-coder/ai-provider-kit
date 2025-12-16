package ollama

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDuration_MarshalJSON tests JSON marshaling of Duration type
func TestDuration_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		duration Duration
		expected string
	}{
		{
			name:     "zero duration (null)",
			duration: Duration{Duration: 0},
			expected: "null",
		},
		{
			name:     "5 minutes",
			duration: Duration{Duration: 5 * time.Minute},
			expected: `"5m0s"`,
		},
		{
			name:     "300 seconds",
			duration: Duration{Duration: 300 * time.Second},
			expected: `"5m0s"`, // Go normalizes to 5m0s
		},
		{
			name:     "keep forever (-1)",
			duration: Duration{Duration: -1},
			expected: `"-1"`,
		},
		{
			name:     "1 hour",
			duration: Duration{Duration: time.Hour},
			expected: `"1h0m0s"`,
		},
		{
			name:     "30 seconds",
			duration: Duration{Duration: 30 * time.Second},
			expected: `"30s"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.duration)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

// TestDuration_UnmarshalJSON tests JSON unmarshaling of Duration type
func TestDuration_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Duration
		wantErr  bool
	}{
		{
			name:     "5 minutes string",
			input:    `"5m"`,
			expected: Duration{Duration: 5 * time.Minute},
			wantErr:  false,
		},
		{
			name:     "300 seconds string",
			input:    `"300s"`,
			expected: Duration{Duration: 300 * time.Second},
			wantErr:  false,
		},
		{
			name:     "keep forever string",
			input:    `"-1"`,
			expected: Duration{Duration: -1},
			wantErr:  false,
		},
		{
			name:     "null value",
			input:    `null`,
			expected: Duration{Duration: 0},
			wantErr:  false,
		},
		{
			name:     "1 hour string",
			input:    `"1h"`,
			expected: Duration{Duration: time.Hour},
			wantErr:  false,
		},
		{
			name:     "numeric nanoseconds",
			input:    `300000000000`, // 300 seconds in nanoseconds
			expected: Duration{Duration: 300 * time.Second},
			wantErr:  false,
		},
		{
			name:    "invalid format",
			input:   `"invalid"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var duration Duration
			err := json.Unmarshal([]byte(tt.input), &duration)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected.Duration, duration.Duration)
			}
		})
	}
}

// TestDuration_RoundTrip tests marshaling and unmarshaling
func TestDuration_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		duration Duration
	}{
		{
			name:     "5 minutes",
			duration: Duration{Duration: 5 * time.Minute},
		},
		{
			name:     "keep forever",
			duration: Duration{Duration: -1},
		},
		{
			name:     "30 seconds",
			duration: Duration{Duration: 30 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(tt.duration)
			require.NoError(t, err)

			// Unmarshal
			var unmarshaled Duration
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			// Compare
			assert.Equal(t, tt.duration.Duration, unmarshaled.Duration)
		})
	}
}

// TestParseKeepAlive tests the parseKeepAlive helper function
func TestParseKeepAlive(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected *Duration
	}{
		{
			name:     "string 5m",
			input:    "5m",
			expected: &Duration{Duration: 5 * time.Minute},
		},
		{
			name:     "string 300s",
			input:    "300s",
			expected: &Duration{Duration: 300 * time.Second},
		},
		{
			name:     "string -1 (keep forever)",
			input:    "-1",
			expected: &Duration{Duration: -1},
		},
		{
			name:     "string 0 (unload immediately)",
			input:    "0",
			expected: &Duration{Duration: 0},
		},
		{
			name:     "int seconds",
			input:    300,
			expected: &Duration{Duration: 300 * time.Second},
		},
		{
			name:     "int -1 (keep forever)",
			input:    -1,
			expected: &Duration{Duration: -1},
		},
		{
			name:     "int64 seconds",
			input:    int64(300),
			expected: &Duration{Duration: 300 * time.Second},
		},
		{
			name:     "float64 seconds",
			input:    300.0,
			expected: &Duration{Duration: 300 * time.Second},
		},
		{
			name:     "time.Duration",
			input:    5 * time.Minute,
			expected: &Duration{Duration: 5 * time.Minute},
		},
		{
			name:     "Duration type",
			input:    Duration{Duration: 5 * time.Minute},
			expected: &Duration{Duration: 5 * time.Minute},
		},
		{
			name:     "pointer to Duration",
			input:    &Duration{Duration: 5 * time.Minute},
			expected: &Duration{Duration: 5 * time.Minute},
		},
		{
			name:     "invalid string",
			input:    "invalid",
			expected: nil,
		},
		{
			name:     "nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "unsupported type",
			input:    []string{"test"},
			expected: nil,
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

// TestOllamaChatRequest_KeepAlive_JSON tests KeepAlive field in request JSON
func TestOllamaChatRequest_KeepAlive_JSON(t *testing.T) {
	tests := []struct {
		name     string
		request  ollamaChatRequest
		expected string
	}{
		{
			name: "without keep_alive",
			request: ollamaChatRequest{
				Model:  "llama3.1:8b",
				Stream: true,
			},
			expected: `{"model":"llama3.1:8b","messages":null,"stream":true}`,
		},
		{
			name: "with keep_alive 5 minutes",
			request: ollamaChatRequest{
				Model:     "llama3.1:8b",
				Stream:    true,
				KeepAlive: &Duration{Duration: 5 * time.Minute},
			},
			expected: `{"model":"llama3.1:8b","messages":null,"stream":true,"keep_alive":"5m0s"}`,
		},
		{
			name: "with keep_alive -1 (forever)",
			request: ollamaChatRequest{
				Model:     "llama3.1:8b",
				Stream:    true,
				KeepAlive: &Duration{Duration: -1},
			},
			expected: `{"model":"llama3.1:8b","messages":null,"stream":true,"keep_alive":"-1"}`,
		},
		{
			name: "with keep_alive 0 (unload)",
			request: ollamaChatRequest{
				Model:     "llama3.1:8b",
				Stream:    true,
				KeepAlive: &Duration{Duration: 0},
			},
			expected: `{"model":"llama3.1:8b","messages":null,"stream":true,"keep_alive":null}`,
		},
		{
			name: "with keep_alive 30 seconds",
			request: ollamaChatRequest{
				Model:     "llama3.1:8b",
				Stream:    true,
				KeepAlive: &Duration{Duration: 30 * time.Second},
			},
			expected: `{"model":"llama3.1:8b","messages":null,"stream":true,"keep_alive":"30s"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.request)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}

// TestOllamaChatRequest_KeepAlive_Unmarshal tests unmarshaling KeepAlive from JSON
func TestOllamaChatRequest_KeepAlive_Unmarshal(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedKeepLive *Duration
		expectNil        bool
	}{
		{
			name:      "without keep_alive",
			input:     `{"model":"llama3.1:8b","stream":true}`,
			expectNil: true,
		},
		{
			name:             "with keep_alive 5m",
			input:            `{"model":"llama3.1:8b","stream":true,"keep_alive":"5m"}`,
			expectedKeepLive: &Duration{Duration: 5 * time.Minute},
		},
		{
			name:             "with keep_alive -1",
			input:            `{"model":"llama3.1:8b","stream":true,"keep_alive":"-1"}`,
			expectedKeepLive: &Duration{Duration: -1},
		},
		{
			name:      "with keep_alive null",
			input:     `{"model":"llama3.1:8b","stream":true,"keep_alive":null}`,
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request ollamaChatRequest
			err := json.Unmarshal([]byte(tt.input), &request)
			require.NoError(t, err)

			if tt.expectNil {
				assert.Nil(t, request.KeepAlive)
			} else {
				require.NotNil(t, request.KeepAlive)
				assert.Equal(t, tt.expectedKeepLive.Duration, request.KeepAlive.Duration)
			}
		})
	}
}
