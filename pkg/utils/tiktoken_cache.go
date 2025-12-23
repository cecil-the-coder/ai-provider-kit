package utils

import (
	"crypto/sha256"
	"strings"
	"sync"

	"github.com/pkoukk/tiktoken-go"
	lru "github.com/hashicorp/golang-lru/v2"
)

// TiktokenCache provides cached tiktoken-based token counting
type TiktokenCache struct {
	cache    *lru.Cache[string, int]
	encoders map[string]*tiktoken.Tiktoken
	mu       sync.RWMutex
}

var globalTiktokenCache *TiktokenCache
var tiktokenOnce sync.Once

// GetTiktokenCache returns the singleton TiktokenCache instance
func GetTiktokenCache() *TiktokenCache {
	tiktokenOnce.Do(func() {
		cache, _ := lru.New[string, int](1000)
		globalTiktokenCache = &TiktokenCache{
			cache:    cache,
			encoders: make(map[string]*tiktoken.Tiktoken),
		}
	})
	return globalTiktokenCache
}

// GetEncoder returns a cached tiktoken encoder for the model
func (tc *TiktokenCache) GetEncoder(model string) (*tiktoken.Tiktoken, error) {
	// Map model to encoding
	encoding := tc.getEncodingForModel(model)

	tc.mu.RLock()
	if enc, ok := tc.encoders[encoding]; ok {
		tc.mu.RUnlock()
		return enc, nil
	}
	tc.mu.RUnlock()

	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Double-check after acquiring write lock
	if enc, ok := tc.encoders[encoding]; ok {
		return enc, nil
	}

	enc, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		return nil, err
	}

	tc.encoders[encoding] = enc
	return enc, nil
}

// CountTokens returns token count with caching for a single text
func (tc *TiktokenCache) CountTokens(text, model string) (int, error) {
	if text == "" {
		return 0, nil
	}

	enc, err := tc.GetEncoder(model)
	if err != nil {
		return 0, err
	}

	// Create cache key from hash of text + model
	hash := sha256.Sum256([]byte(text + model))
	key := string(hash[:16])

	tc.mu.RLock()
	if count, ok := tc.cache.Get(key); ok {
		tc.mu.RUnlock()
		return count, nil
	}
	tc.mu.RUnlock()

	count := len(enc.Encode(text, nil, nil))

	tc.mu.Lock()
	tc.cache.Add(key, count)
	tc.mu.Unlock()

	return count, nil
}

// CountMessagesTokens returns token count for multiple messages with parallel encoding
func (tc *TiktokenCache) CountMessagesTokens(messages []string, model string) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}

	enc, err := tc.GetEncoder(model)
	if err != nil {
		return 0, err
	}

	var wg sync.WaitGroup
	results := make(chan int, len(messages))
	errors := make(chan error, len(messages))

	for _, msg := range messages {
		if msg == "" {
			results <- 0
			continue
		}

		wg.Add(1)
		go func(text string) {
			defer wg.Done()

			// Create cache key from hash of text + model
			hash := sha256.Sum256([]byte(text + model))
			key := string(hash[:16])

			// Try cache first
			tc.mu.RLock()
			if count, ok := tc.cache.Get(key); ok {
				tc.mu.RUnlock()
				results <- count
				return
			}
			tc.mu.RUnlock()

			// Encode the text
			count := len(enc.Encode(text, nil, nil))

			// Add to cache
			tc.mu.Lock()
			tc.cache.Add(key, count)
			tc.mu.Unlock()

			results <- count
		}(msg)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	total := 0
	for count := range results {
		total += count
	}

	// Check for errors (though we don't expect any in current implementation)
	for range errors {
		// Consume any errors
	}

	return total, nil
}

// getEncodingForModel maps model names to tiktoken encodings
func (tc *TiktokenCache) getEncodingForModel(model string) string {
	model = strings.ToLower(model)

	// Claude models use cl100k_base (similar to GPT-4)
	if contains(model, "claude") || contains(model, "anthropic") {
		return "cl100k_base"
	}

	// GPT-4o models use o200k_base
	if contains(model, "gpt-4o") {
		return "o200k_base"
	}

	// GPT-4 and GPT-3.5-turbo models use cl100k_base
	if contains(model, "gpt-4") || contains(model, "gpt-3.5") || contains(model, "gpt-35") {
		return "cl100k_base"
	}

	// GPT-3 and earlier models use r50k_base
	if contains(model, "gpt-3") {
		return "r50k_base"
	}

	// Codex models use p50k_base
	if contains(model, "code") || contains(model, "codex") || contains(model, "davinci") || contains(model, "curie") || contains(model, "babbage") || contains(model, "ada") {
		return "p50k_base"
	}

	// Embedding models
	if contains(model, "embedding") {
		return "cl100k_base"
	}

	// Default to cl100k_base (most common)
	return "cl100k_base"
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
