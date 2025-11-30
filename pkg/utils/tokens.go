package utils

import "github.com/cecil-the-coder/ai-provider-kit/pkg/types"

// EstimateTokensFromBytes estimates token count from byte length.
// Based on empirical observation: ~4.7 bytes per token on average.
// This is a rough estimate, not exact tokenization.
func EstimateTokensFromBytes(byteCount int) int {
	if byteCount <= 0 {
		return 0
	}
	return (byteCount * 10) / 47 // ~4.7 bytes per token, avoids floating point
}

// EstimateTokensFromString estimates token count from string content.
func EstimateTokensFromString(s string) int {
	return EstimateTokensFromBytes(len(s))
}

// EstimateTokensFromMessages estimates total tokens across all messages.
// Uses GetTextContent() to extract text from both simple and multimodal messages.
func EstimateTokensFromMessages(messages []types.ChatMessage) int {
	total := 0
	for _, msg := range messages {
		total += EstimateTokensFromString(msg.GetTextContent())
	}
	return total
}

// BytesPerToken is the empirically-derived average bytes per token.
// Can be used by consumers for custom calculations.
const BytesPerToken = 4.7

// TokenThreshold represents common context window sizes in tokens.
const (
	TokenThreshold4K   = 4096
	TokenThreshold8K   = 8192
	TokenThreshold16K  = 16384
	TokenThreshold32K  = 32768
	TokenThreshold128K = 131072
)

// ByteThresholdForTokens converts token thresholds to approximate byte sizes.
// Useful for quick content-length based routing decisions.
func ByteThresholdForTokens(tokens int) int {
	return int(float64(tokens) * BytesPerToken)
}
