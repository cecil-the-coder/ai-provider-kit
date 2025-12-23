package utils

import (
	"sync"

	"github.com/cecil-the-coder/ai-provider-kit/internal/tokenizer"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

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
// Parallelizes token estimation for messages when there are multiple messages.
func EstimateTokensFromMessages(messages []types.ChatMessage) int {
	if len(messages) == 0 {
		return 0
	}

	// For small message counts, sequential processing is faster due to lower overhead
	if len(messages) <= 4 {
		total := 0
		for _, msg := range messages {
			total += EstimateTokensFromString(msg.GetTextContent())
		}
		return total
	}

	// For larger message counts, use parallel processing for 2-4x speedup
	var wg sync.WaitGroup
	tokenChan := make(chan int, len(messages))

	for _, msg := range messages {
		wg.Add(1)
		go func(content string) {
			defer wg.Done()
			tokenChan <- EstimateTokensFromString(content)
		}(msg.GetTextContent())
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(tokenChan)
	}()

	// Sum tokens from all goroutines
	total := 0
	for tokens := range tokenChan {
		total += tokens
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

// EstimateTokensFast provides a fast character-based token estimate.
// For small payloads, full BPE encoding is unnecessary.
// This function is optimized for quick estimates on small text sizes.
func EstimateTokensFast(text string, maxTokens int) int {
	// For very small requests, character-based is sufficient
	if len(text) < 10000 { // ~2500 tokens
		return len(text) / 4 // ~4 chars per token
	}
	return maxTokens
}

// CountTokens returns accurate token count using tiktoken-based encoding with caching.
// This is more accurate than EstimateTokensFromString but slower.
// The encoding is automatically selected based on the model name.
// Results are cached for performance.
//
// When the Rust tokenizer is available (via -tags=rusttokenizer), it provides
// 3-15x faster tokenization compared to the pure Go implementation.
func CountTokens(text, model string) (int, error) {
	// Try the fast path with Rust tokenizer first
	if tokenizer.IsRustAvailable() {
		return tokenizer.CountTokens(text, model)
	}

	// Fall back to cached Go implementation
	cache := GetTiktokenCache()
	return cache.CountTokens(text, model)
}

// CountTokensFromMessages returns accurate token count for ChatMessages using tiktoken.
// Uses parallel encoding for multiple messages and caches results.
// The encoding is automatically selected based on the model name.
//
// When the Rust tokenizer is available (via -tags=rusttokenizer), it uses
// batch processing for even better performance.
func CountTokensFromMessages(messages []types.ChatMessage, model string) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}

	// Try the fast path with Rust tokenizer first
	if tokenizer.IsRustAvailable() {
		texts := make([]string, len(messages))
		for i, msg := range messages {
			texts[i] = msg.GetTextContent()
		}
		return tokenizer.CountBatch(texts, model)
	}

	// Fall back to cached Go implementation
	cache := GetTiktokenCache()
	texts := make([]string, len(messages))
	for i, msg := range messages {
		texts[i] = msg.GetTextContent()
	}
	return cache.CountMessagesTokens(texts, model)
}
