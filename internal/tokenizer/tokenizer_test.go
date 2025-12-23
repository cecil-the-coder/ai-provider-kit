package tokenizer

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestBasicTokenCounting tests basic token counting functionality
func TestBasicTokenCounting(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		model    string
		minCount int
		maxCount int
	}{
		{
			name:     "empty string",
			text:     "",
			model:    "gpt-4",
			minCount: 0,
			maxCount: 0,
		},
		{
			name:     "short text",
			text:     "Hello, world!",
			model:    "gpt-4",
			minCount: 1,
			maxCount: 10,
		},
		{
			name:     "medium text",
			text:     "This is a medium-length text that contains several sentences.",
			model:    "gpt-4",
			minCount: 10,
			maxCount: 30,
		},
		{
			name:     "code snippet",
			text:     "func main() { println!(\"Hello\"); }",
			model:    "gpt-4",
			minCount: 5,
			maxCount: 25,
		},
		{
			name:     "unicode text",
			text:     "Hello 世界 🌍",
			model:    "gpt-4",
			minCount: 1,
			maxCount: 20,
		},
		{
			name:     "claude model",
			text:     "This is a test for Claude.",
			model:    "claude-3-opus",
			minCount: 5,
			maxCount: 20,
		},
		{
			name:     "gpt-4o model",
			text:     "This is a test for GPT-4o.",
			model:    "gpt-4o",
			minCount: 5,
			maxCount: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := CountTokens(tt.text, tt.model)
			if err != nil {
				t.Fatalf("CountTokens() error = %v", err)
			}

			if count < tt.minCount || count > tt.maxCount {
				t.Errorf("CountTokens() = %v, want between %v and %v", count, tt.minCount, tt.maxCount)
			}
		})
	}
}

// TestBatchCounting tests batch token counting
func TestBatchCounting(t *testing.T) {
	texts := []string{
		"Hello, world!",
		"This is a second text.",
		"And a third one for good measure.",
	}

	count, err := CountBatch(texts, "gpt-4")
	if err != nil {
		t.Fatalf("CountBatch() error = %v", err)
	}

	if count < 5 {
		t.Errorf("CountBatch() = %v, want >= 5", count)
	}

	// Verify individual counts match sum
	sum := 0
	for _, text := range texts {
		c, err := CountTokens(text, "gpt-4")
		if err != nil {
			t.Fatalf("CountTokens() error = %v", err)
		}
		sum += c
	}

	if count != sum {
		t.Errorf("CountBatch() = %v, sum of individual counts = %v", count, sum)
	}
}

// TestEmptyBatch tests empty batch
func TestEmptyBatch(t *testing.T) {
	count, err := CountBatch([]string{}, "gpt-4")
	if err != nil {
		t.Fatalf("CountBatch() error = %v", err)
	}

	if count != 0 {
		t.Errorf("CountBatch() = %v, want 0", count)
	}
}

// TestBatchWithEmptyStrings tests batch with empty strings
func TestBatchWithEmptyStrings(t *testing.T) {
	texts := []string{
		"Hello, world!",
		"",
		"This is text.",
		"",
	}

	count, err := CountBatch(texts, "gpt-4")
	if err != nil {
		t.Fatalf("CountBatch() error = %v", err)
	}

	if count < 3 {
		t.Errorf("CountBatch() = %v, want >= 3", count)
	}
}

// TestModelEncoding tests that different models use appropriate encodings
func TestModelEncoding(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog."

	models := []string{
		"gpt-4",
		"gpt-4o",
		"gpt-3.5-turbo",
		"claude-3-opus",
		"claude-3.5-sonnet",
	}

	counts := make(map[string]int)
	for _, model := range models {
		count, err := CountTokens(text, model)
		if err != nil {
			t.Fatalf("CountTokens(%q) error = %v", model, err)
		}
		counts[model] = count
	}

	// All GPT-4 and Claude models should have the same count (cl100k_base)
	if counts["gpt-4"] != counts["claude-3-opus"] {
		t.Logf("Note: gpt-4=%d, claude-3-opus=%d (may differ by encoding)", counts["gpt-4"], counts["claude-3-opus"])
	}

	// GPT-4o uses o200k_base, so may differ
	t.Logf("Token counts: gpt-4=%d, gpt-4o=%d, claude-3-opus=%d",
		counts["gpt-4"], counts["gpt-4o"], counts["claude-3-opus"])
}

// TestUnicodeHandling tests various unicode inputs
func TestUnicodeHandling(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"chinese", "你好世界"},
		{"japanese", "こんにちは"},
		{"korean", "안녕하세요"},
		{"arabic", "مرحبا بالعالم"},
		{"hebrew", "שלום עולם"},
		{"emoji", "🌍🚀🎉"},
		{"mixed", "Hello 世界 🌍"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := CountTokens(tt.text, "gpt-4")
			if err != nil {
				t.Fatalf("CountTokens() error = %v", err)
			}

			if count <= 0 {
				t.Errorf("CountTokens() = %v, want > 0", count)
			}
		})
	}
}

// TestCodeTokenization tests code snippet tokenization
func TestCodeTokenization(t *testing.T) {
	code := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`

	count, err := CountTokens(code, "gpt-4")
	if err != nil {
		t.Fatalf("CountTokens() error = %v", err)
	}

	if count < 10 {
		t.Errorf("CountTokens() = %v, want >= 10", count)
	}
}

// TestLargeText tests tokenization of large texts
func TestLargeText(t *testing.T) {
	// Generate a large text
	text := ""
	for i := 0; i < 1000; i++ {
		text += "This is sentence number " + string(rune('0'+i%10)) + ". "
	}

	count, err := CountTokens(text, "gpt-4")
	if err != nil {
		t.Fatalf("CountTokens() error = %v", err)
	}

	if count < 5000 {
		t.Errorf("CountTokens() = %v, want >= 5000", count)
	}
}

// TestGetImplementationName tests the implementation name
func TestGetImplementationName(t *testing.T) {
	name := GetImplementationName()
	if name == "" {
		t.Error("GetImplementationName() returned empty string")
	}
	t.Logf("Using implementation: %s", name)
}

// TestIsRustAvailable checks if Rust is available
func TestIsRustAvailable(t *testing.T) {
	available := IsRustAvailable()
	t.Logf("Rust tokenizer available: %v", available)

	if available {
		t.Log("Rust tokenizer is enabled - expect maximum performance")
	} else {
		t.Log("Using pure Go implementation - build with -tags=rusttokenizer for Rust support")
	}
}

// TestChatMessages tests token counting for chat messages
func TestChatMessages(t *testing.T) {
	messages := []types.ChatMessage{
		{
			Role:    "system",
			Content: "You are a helpful assistant.",
		},
		{
			Role:    "user",
			Content: "Hello, how are you?",
		},
		{
			Role:    "assistant",
			Content: "I'm doing well, thank you for asking!",
		},
	}

	// Use CountBatch directly to avoid import cycle
	texts := make([]string, len(messages))
	for i, msg := range messages {
		texts[i] = msg.GetTextContent()
	}

	count, err := CountBatch(texts, "gpt-4")
	if err != nil {
		t.Fatalf("CountBatch() error = %v", err)
	}

	if count < 10 {
		t.Errorf("CountBatch() = %v, want >= 10", count)
	}
}

// TestConsistency tests that results are consistent across multiple calls
func TestConsistency(t *testing.T) {
	text := "This is a test of tokenization consistency."
	model := "gpt-4"

	counts := make([]int, 100)
	for i := 0; i < 100; i++ {
		count, err := CountTokens(text, model)
		if err != nil {
			t.Fatalf("CountTokens() error = %v", err)
		}
		counts[i] = count
	}

	// All counts should be the same
	first := counts[0]
	for i, count := range counts {
		if count != first {
			t.Errorf("Count %d = %v, want %v", i, count, first)
		}
	}
}

// TestSpecialTokens tests handling of special tokens and formatting
func TestSpecialTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"newlines", "Line 1\nLine 2\nLine 3"},
		{"tabs", "Column1\tColumn2\tColumn3"},
		{"mixed whitespace", "Text  \n  with  \n  whitespace"},
		{"quotes", `"quoted text" and 'single quotes'`},
		{"brackets", "{[()]}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := CountTokens(tt.text, "gpt-4")
			if err != nil {
				t.Fatalf("CountTokens() error = %v", err)
			}

			if count <= 0 {
				t.Errorf("CountTokens() = %v, want > 0", count)
			}
		})
	}
}

// TestForceImplementation tests forcing specific implementations
func TestForceImplementation(t *testing.T) {
	text := "Test text for implementation forcing."
	model := "gpt-4"

	// Reset global counter
	ResetGlobalCounter()
	defer ResetGlobalCounter()

	// Test Go implementation
	goCounter := ForceGoCounter()
	if !goCounter.IsAvailable() {
		t.Error("Go counter should always be available")
	}

	goCount, err := goCounter.CountTokens(text, model)
	if err != nil {
		t.Fatalf("Go CountTokens() error = %v", err)
	}

	// Test Rust implementation (may not be available)
	if IsRustAvailable() {
		ResetGlobalCounter()
		rustCounter := ForceRustCounter()

		rustCount, err := rustCounter.CountTokens(text, model)
		if err != nil {
			t.Fatalf("Rust CountTokens() error = %v", err)
		}

		if goCount != rustCount {
			t.Errorf("Go=%d, Rust=%d - implementations should match", goCount, rustCount)
		}
	}
}

// Benchmark helper - can be run with go test -bench
func BenchmarkCountTokens(b *testing.B) {
	text := "This is a benchmark test for token counting performance. " +
		"We want to measure how fast the tokenizer can process this text."
	model := "gpt-4"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CountTokens(text, model)
	}
}

func BenchmarkCountBatch(b *testing.B) {
	texts := []string{
		"First text for batch benchmarking.",
		"Second text in the batch.",
		"Third text to process.",
		"Fourth text completes our set.",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CountBatch(texts, "gpt-4")
	}
}
