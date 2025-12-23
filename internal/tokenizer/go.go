//go:build !rusttokenizer
// +build !rusttokenizer

package tokenizer

import (
	"strings"
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

// goCounter implements tokenCounter using the pure Go tiktoken library
type goCounter struct {
	encoders map[string]*tiktoken.Tiktoken
	mu       sync.RWMutex
}

// getGoCounter returns a new goCounter instance
func getGoCounter() tokenCounter {
	return &goCounter{
		encoders: make(map[string]*tiktoken.Tiktoken),
	}
}

// IsAvailable always returns true for the Go implementation
func (gc *goCounter) IsAvailable() bool {
	return true
}

// Name returns the name of this implementation
func (gc *goCounter) Name() string {
	return "go-pure"
}

// getEncoder returns a cached tiktoken encoder for the model
func (gc *goCounter) getEncoder(model string) (*tiktoken.Tiktoken, error) {
	// Map model to encoding
	encoding := gc.getEncodingForModel(model)

	gc.mu.RLock()
	if enc, ok := gc.encoders[encoding]; ok {
		gc.mu.RUnlock()
		return enc, nil
	}
	gc.mu.RUnlock()

	gc.mu.Lock()
	defer gc.mu.Unlock()

	// Double-check after acquiring write lock
	if enc, ok := gc.encoders[encoding]; ok {
		return enc, nil
	}

	enc, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		return nil, err
	}

	gc.encoders[encoding] = enc
	return enc, nil
}

// CountTokens counts tokens using the Go tiktoken implementation
func (gc *goCounter) CountTokens(text, model string) (int, error) {
	if text == "" {
		return 0, nil
	}

	enc, err := gc.getEncoder(model)
	if err != nil {
		return 0, err
	}

	tokens := enc.Encode(text, nil, nil)
	return len(tokens), nil
}

// CountBatch counts tokens for multiple texts
func (gc *goCounter) CountBatch(texts []string, model string) (int, error) {
	if len(texts) == 0 {
		return 0, nil
	}

	enc, err := gc.getEncoder(model)
	if err != nil {
		return 0, err
	}

	total := 0
	for _, text := range texts {
		if text == "" {
			continue
		}
		tokens := enc.Encode(text, nil, nil)
		total += len(tokens)
	}

	return total, nil
}

// getEncodingForModel maps model names to tiktoken encodings
func (gc *goCounter) getEncodingForModel(model string) string {
	model = strings.ToLower(model)

	// Claude models use cl100k_base (similar to GPT-4)
	if strings.Contains(model, "claude") || strings.Contains(model, "anthropic") {
		return "cl100k_base"
	}

	// GPT-4o models use o200k_base
	if strings.Contains(model, "gpt-4o") {
		return "o200k_base"
	}

	// GPT-4 and GPT-3.5-turbo models use cl100k_base
	if strings.Contains(model, "gpt-4") || strings.Contains(model, "gpt-3.5") || strings.Contains(model, "gpt-35") {
		return "cl100k_base"
	}

	// GPT-3 and earlier models use r50k_base
	if strings.Contains(model, "gpt-3") {
		return "r50k_base"
	}

	// Codex models use p50k_base
	if strings.Contains(model, "code") || strings.Contains(model, "codex") ||
		strings.Contains(model, "davinci") || strings.Contains(model, "curie") ||
		strings.Contains(model, "babbage") || strings.Contains(model, "ada") {
		return "p50k_base"
	}

	// Embedding models
	if strings.Contains(model, "embedding") {
		return "cl100k_base"
	}

	// Default to cl100k_base (most common)
	return "cl100k_base"
}

// getRustCounter returns nil since we're not building with the rust tag
func getRustCounter() tokenCounter {
	return nil
}
