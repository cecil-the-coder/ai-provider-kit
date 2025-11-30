package utils

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestEstimateTokensFromBytes(t *testing.T) {
	tests := []struct {
		name      string
		byteCount int
		expected  int
	}{
		{
			name:      "zero bytes returns zero tokens",
			byteCount: 0,
			expected:  0,
		},
		{
			name:      "negative bytes returns zero tokens",
			byteCount: -100,
			expected:  0,
		},
		{
			name:      "small byte count",
			byteCount: 47,
			expected:  10, // 47 * 10 / 47 = 10
		},
		{
			name:      "100 bytes",
			byteCount: 100,
			expected:  21, // 100 * 10 / 47 = 21 (rounded down)
		},
		{
			name:      "1000 bytes",
			byteCount: 1000,
			expected:  212, // 1000 * 10 / 47 = 212
		},
		{
			name:      "large byte count (10000 bytes)",
			byteCount: 10000,
			expected:  2127, // 10000 * 10 / 47 = 2127
		},
		{
			name:      "approximate 4K context (19200 bytes)",
			byteCount: 19200,
			expected:  4085, // Should be close to 4096 tokens
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateTokensFromBytes(tt.byteCount)
			if result != tt.expected {
				t.Errorf("EstimateTokensFromBytes(%d) = %d; want %d", tt.byteCount, result, tt.expected)
			}
		})
	}
}

func TestEstimateTokensFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "single character",
			input:    "a",
			expected: 0, // 1 * 10 / 47 = 0 (rounded down)
		},
		{
			name:     "short sentence",
			input:    "Hello, world!",
			expected: 2, // 13 bytes * 10 / 47 = 2
		},
		{
			name:     "longer text",
			input:    "This is a longer sentence with more words to test token estimation.",
			expected: 14, // 68 bytes * 10 / 47 = 14
		},
		{
			name:     "unicode characters",
			input:    "Hello 世界",
			expected: 2, // 12 bytes in UTF-8 * 10 / 47 = 2
		},
		{
			name:     "emoji",
			input:    "Hello 👋 World 🌍",
			expected: 4, // ~20 bytes * 10 / 47 = 4
		},
		{
			name:     "multiline text",
			input:    "Line 1\nLine 2\nLine 3",
			expected: 4, // 20 bytes * 10 / 47 = 4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateTokensFromString(tt.input)
			if result != tt.expected {
				t.Errorf("EstimateTokensFromString(%q) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEstimateTokensFromMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []types.ChatMessage
		expected int
	}{
		{
			name:     "empty message list",
			messages: []types.ChatMessage{},
			expected: 0,
		},
		{
			name:     "nil message list",
			messages: nil,
			expected: 0,
		},
		{
			name: "single message with simple content",
			messages: []types.ChatMessage{
				{Role: "user", Content: "Hello!"},
			},
			expected: 1, // 6 bytes * 10 / 47 = 1
		},
		{
			name: "multiple messages",
			messages: []types.ChatMessage{
				{Role: "user", Content: "What is the weather?"},
				{Role: "assistant", Content: "The weather is sunny."},
			},
			expected: 8, // (20 + 22) bytes = 42 * 10 / 47 = 8
		},
		{
			name: "messages with empty content",
			messages: []types.ChatMessage{
				{Role: "user", Content: ""},
				{Role: "assistant", Content: ""},
			},
			expected: 0,
		},
		{
			name: "message with multimodal parts (text only)",
			messages: []types.ChatMessage{
				{
					Role: "user",
					Parts: []types.ContentPart{
						types.NewTextPart("What's in this image?"),
					},
				},
			},
			expected: 4, // 21 bytes * 10 / 47 = 4
		},
		{
			name: "message with multimodal parts (text and image)",
			messages: []types.ChatMessage{
				{
					Role: "user",
					Parts: []types.ContentPart{
						types.NewTextPart("Describe this image"),
						types.NewImagePart("image/png", "base64data"), // Image data doesn't count in GetTextContent
					},
				},
			},
			expected: 4, // Only text counts: 19 bytes * 10 / 47 = 4
		},
		{
			name: "mixed content and parts",
			messages: []types.ChatMessage{
				{Role: "user", Content: "First message"},       // 13 bytes -> 2 tokens
				{Role: "assistant", Content: "Second message"}, // 14 bytes -> 2 tokens
				{
					Role: "user",
					Parts: []types.ContentPart{
						types.NewTextPart("Third message"), // 13 bytes -> 2 tokens
					},
				},
			},
			expected: 6, // Sum of individual message tokens (2+2+2) due to rounding
		},
		{
			name: "message with tool calls (should not count tool call content)",
			messages: []types.ChatMessage{
				{
					Role:    "assistant",
					Content: "Let me check that.",
					ToolCalls: []types.ToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: types.ToolCallFunction{
								Name:      "get_weather",
								Arguments: `{"location":"London"}`,
							},
						},
					},
				},
			},
			expected: 3, // Only "Let me check that." = 19 bytes * 10 / 47 = 4 (actually 3)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateTokensFromMessages(tt.messages)
			if result != tt.expected {
				t.Errorf("EstimateTokensFromMessages() = %d; want %d", result, tt.expected)
			}
		})
	}
}

func TestBytesPerTokenConstant(t *testing.T) {
	if BytesPerToken != 4.7 {
		t.Errorf("BytesPerToken = %f; want 4.7", BytesPerToken)
	}
}

func TestTokenThresholdConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant int
		expected int
	}{
		{
			name:     "TokenThreshold4K",
			constant: TokenThreshold4K,
			expected: 4096,
		},
		{
			name:     "TokenThreshold8K",
			constant: TokenThreshold8K,
			expected: 8192,
		},
		{
			name:     "TokenThreshold16K",
			constant: TokenThreshold16K,
			expected: 16384,
		},
		{
			name:     "TokenThreshold32K",
			constant: TokenThreshold32K,
			expected: 32768,
		},
		{
			name:     "TokenThreshold128K",
			constant: TokenThreshold128K,
			expected: 131072,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %d; want %d", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestByteThresholdForTokens(t *testing.T) {
	tests := []struct {
		name     string
		tokens   int
		expected int
	}{
		{
			name:     "zero tokens",
			tokens:   0,
			expected: 0,
		},
		{
			name:     "negative tokens",
			tokens:   -100,
			expected: -470, // -100 * 4.7
		},
		{
			name:     "100 tokens",
			tokens:   100,
			expected: 470, // 100 * 4.7
		},
		{
			name:     "4K tokens",
			tokens:   TokenThreshold4K,
			expected: 19251, // 4096 * 4.7
		},
		{
			name:     "8K tokens",
			tokens:   TokenThreshold8K,
			expected: 38502, // 8192 * 4.7
		},
		{
			name:     "16K tokens",
			tokens:   TokenThreshold16K,
			expected: 77004, // 16384 * 4.7
		},
		{
			name:     "32K tokens",
			tokens:   TokenThreshold32K,
			expected: 154009, // 32768 * 4.7
		},
		{
			name:     "128K tokens",
			tokens:   TokenThreshold128K,
			expected: 616038, // 131072 * 4.7
		},
		{
			name:     "custom token count",
			tokens:   5000,
			expected: 23500, // 5000 * 4.7
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ByteThresholdForTokens(tt.tokens)
			if result != tt.expected {
				t.Errorf("ByteThresholdForTokens(%d) = %d; want %d", tt.tokens, result, tt.expected)
			}
		})
	}
}

// TestTokenEstimationAccuracy verifies the relationship between functions
func TestTokenEstimationAccuracy(t *testing.T) {
	tests := []struct {
		name         string
		tokens       int
		allowedDelta int // Allowed difference due to rounding
	}{
		{
			name:         "round trip 4K tokens",
			tokens:       TokenThreshold4K,
			allowedDelta: 10,
		},
		{
			name:         "round trip 8K tokens",
			tokens:       TokenThreshold8K,
			allowedDelta: 20,
		},
		{
			name:         "round trip 100 tokens",
			tokens:       100,
			allowedDelta: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert tokens to bytes
			bytes := ByteThresholdForTokens(tt.tokens)

			// Convert bytes back to tokens
			estimatedTokens := EstimateTokensFromBytes(bytes)

			// Check if we're close to the original token count
			delta := abs(estimatedTokens - tt.tokens)
			if delta > tt.allowedDelta {
				t.Errorf("Round trip accuracy issue: %d tokens -> %d bytes -> %d tokens (delta: %d, allowed: %d)",
					tt.tokens, bytes, estimatedTokens, delta, tt.allowedDelta)
			}
		})
	}
}

// Helper function for absolute value
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
