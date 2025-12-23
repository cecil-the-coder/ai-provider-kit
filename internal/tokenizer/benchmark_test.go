//go:build rusttokenizer
// +build rusttokenizer

package tokenizer

import (
	"fmt"
	"sync"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/utils"
)

// Test texts of varying sizes
var benchmarkTexts = struct {
	short   string
	medium  string
	long    string
	code    string
	unicode string
}{
	short:   "Hello, world!",
	medium:  "This is a medium-length text that contains several sentences. It's designed to test tokenization performance on a typical paragraph of text that might appear in a chat message or document.",
	long:    generateLongText(1000),
	code:    generateCodeSample(),
	unicode: "Unicode test: 你好世界 🌍 Ñoño café 🚀 日本語 한국어 العربية עברית",
}

// Test models
var benchmarkModels = []string{
	"gpt-4",
	"gpt-4o",
	"claude-3-opus",
	"claude-3.5-sonnet",
}

func generateLongText(sentences int) string {
	words := []string{"The", "quick", "brown", "fox", "jumps", "over", "the", "lazy", "dog", "and", "runs", "through", "the", "meadow"}
	text := ""
	for i := 0; i < sentences; i++ {
		for j := 0; j < 15; j++ {
			text += words[(i+j)%len(words)] + " "
		}
	}
	return text
}

func generateCodeSample() string {
	return `package main

import (
	"fmt"
	"net/http"
	"time"
)

type Request struct {
	ID        string    'json:"id"'
	Timestamp time.Time 'json:"timestamp"'
	Data      []byte    'json:"data"'
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		fmt.Printf("Request processed in %v\n", duration)
	}()

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Process the request...
	fmt.Fprintf(w, "Processed request %s", req.ID)
}

func main() {
	http.HandleFunc("/api/process", handleRequest)
	http.ListenAndServe(":8080", nil)
}`
}

// Benchmark individual implementations
func BenchmarkRustCounter_ShortText(b *testing.B) {
	rc := getRustCounter()
	if rc == nil || !rc.IsAvailable() {
		b.Skip("Rust tokenizer not available")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rc.CountTokens(benchmarkTexts.short, "gpt-4")
	}
}

func BenchmarkRustCounter_MediumText(b *testing.B) {
	rc := getRustCounter()
	if rc == nil || !rc.IsAvailable() {
		b.Skip("Rust tokenizer not available")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rc.CountTokens(benchmarkTexts.medium, "gpt-4")
	}
}

func BenchmarkRustCounter_LongText(b *testing.B) {
	rc := getRustCounter()
	if rc == nil || !rc.IsAvailable() {
		b.Skip("Rust tokenizer not available")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rc.CountTokens(benchmarkTexts.long, "gpt-4")
	}
}

func BenchmarkRustCounter_Code(b *testing.B) {
	rc := getRustCounter()
	if rc == nil || !rc.IsAvailable() {
		b.Skip("Rust tokenizer not available")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rc.CountTokens(benchmarkTexts.code, "gpt-4")
	}
}

func BenchmarkRustCounter_Unicode(b *testing.B) {
	rc := getRustCounter()
	if rc == nil || !rc.IsAvailable() {
		b.Skip("Rust tokenizer not available")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rc.CountTokens(benchmarkTexts.unicode, "gpt-4")
	}
}

func BenchmarkRustCounter_Batch(b *testing.B) {
	rc := getRustCounter()
	if rc == nil || !rc.IsAvailable() {
		b.Skip("Rust tokenizer not available")
	}
	texts := []string{
		benchmarkTexts.short,
		benchmarkTexts.medium,
		benchmarkTexts.code,
		benchmarkTexts.unicode,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rc.CountBatch(texts, "gpt-4")
	}
}

func BenchmarkGoCounter_ShortText(b *testing.B) {
	ResetGlobalCounter()
	gc := ForceGoCounter()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gc.CountTokens(benchmarkTexts.short, "gpt-4")
	}
}

func BenchmarkGoCounter_MediumText(b *testing.B) {
	ResetGlobalCounter()
	gc := ForceGoCounter()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gc.CountTokens(benchmarkTexts.medium, "gpt-4")
	}
}

func BenchmarkGoCounter_LongText(b *testing.B) {
	ResetGlobalCounter()
	gc := ForceGoCounter()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gc.CountTokens(benchmarkTexts.long, "gpt-4")
	}
}

func BenchmarkGoCounter_Code(b *testing.B) {
	ResetGlobalCounter()
	gc := ForceGoCounter()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gc.CountTokens(benchmarkTexts.code, "gpt-4")
	}
}

func BenchmarkGoCounter_Batch(b *testing.B) {
	ResetGlobalCounter()
	gc := ForceGoCounter()
	texts := []string{
		benchmarkTexts.short,
		benchmarkTexts.medium,
		benchmarkTexts.code,
		benchmarkTexts.unicode,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gc.CountBatch(texts, "gpt-4")
	}
}

// Benchmark the utils interface (what users actually call)
func BenchmarkUtils_CountTokens_Rust(b *testing.B) {
	if !IsRustAvailable() {
		b.Skip("Rust tokenizer not available")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = utils.CountTokens(benchmarkTexts.long, "gpt-4")
	}
}

func BenchmarkUtils_CountTokens_Go(b *testing.B) {
	ResetGlobalCounter()
	ForceGoCounter()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = utils.CountTokens(benchmarkTexts.long, "gpt-4")
	}
}

func BenchmarkUtils_CountTokensFromMessages_Rust(b *testing.B) {
	if !IsRustAvailable() {
		b.Skip("Rust tokenizer not available")
	}
	messages := generateTestMessages(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = utils.CountTokensFromMessages(messages, "gpt-4")
	}
}

func BenchmarkUtils_CountTokensFromMessages_Go(b *testing.B) {
	ResetGlobalCounter()
	ForceGoCounter()
	messages := generateTestMessages(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = utils.CountTokensFromMessages(messages, "gpt-4")
	}
}

// Parallel benchmarks
func BenchmarkRustCounter_Parallel(b *testing.B) {
	rc := getRustCounter()
	if rc == nil || !rc.IsAvailable() {
		b.Skip("Rust tokenizer not available")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = rc.CountTokens(benchmarkTexts.long, "gpt-4")
		}
	})
}

func BenchmarkGoCounter_Parallel(b *testing.B) {
	ResetGlobalCounter()
	gc := ForceGoCounter()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = gc.CountTokens(benchmarkTexts.long, "gpt-4")
		}
	})
}

func generateTestMessages(count int) []types.ChatMessage {
	messages := make([]types.ChatMessage, count)
	for i := 0; i < count; i++ {
		messages[i] = types.ChatMessage{
			Role:    "user",
			Content: fmt.Sprintf("Message %d: %s", i, benchmarkTexts.medium),
		}
	}
	return messages
}

// Performance comparison test
func TestPerformanceComparison(t *testing.T) {
	if !IsRustAvailable() {
		t.Skip("Rust tokenizer not available - run with -tags=rusttokenizer")
	}

	// Test texts
	texts := map[string]string{
		"short":  benchmarkTexts.short,
		"medium": benchmarkTexts.medium,
		"long":   benchmarkTexts.long,
		"code":   benchmarkTexts.code,
	}

	// Test models
	models := []string{"gpt-4", "gpt-4o", "claude-3-opus"}

	results := make(map[string]map[string]struct {
		rustCount int
		goCount   int
		match     bool
	})

	ResetGlobalCounter()
	rustCounter := ForceRustCounter()
	ResetGlobalCounter()
	goCounter := ForceGoCounter()

	for name, text := range texts {
		results[name] = make(map[string]struct {
			rustCount int
			goCount   int
			match     bool
		})

		for _, model := range models {
			rustCount, _ := rustCounter.CountTokens(text, model)
			goCount, _ := goCounter.CountTokens(text, model)

			match := rustCount == goCount
			results[name][model] = struct {
				rustCount int
				goCount   int
				match     bool
			}{
				rustCount: rustCount,
				goCount:   goCount,
				match:     match,
			}

			if !match {
				t.Errorf("Token count mismatch for %s/%s: Rust=%d, Go=%d",
					name, model, rustCount, goCount)
			}
		}
	}

	// Print results
	t.Log("Performance Comparison Results:")
	for name, modelResults := range results {
		t.Logf("\n%s:", name)
		for model, result := range modelResults {
			t.Logf("  %s: Rust=%d, Go=%d, Match=%v",
				model, result.rustCount, result.goCount, result.match)
		}
	}
}

// Benchmark that measures actual speedup
func BenchmarkSpeedup(b *testing.B) {
	if !IsRustAvailable() {
		b.Skip("Rust tokenizer not available - run with -tags=rusttokenizer")
	}

	texts := []string{
		benchmarkTexts.short,
		benchmarkTexts.medium,
		benchmarkTexts.long,
		benchmarkTexts.code,
	}

	b.Run("Rust", func(b *testing.B) {
		ResetGlobalCounter()
		ForceRustCounter()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, text := range texts {
				_, _ = CountTokens(text, "gpt-4")
			}
		}
	})

	b.Run("Go", func(b *testing.B) {
		ResetGlobalCounter()
		ForceGoCounter()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, text := range texts {
				_, _ = CountTokens(text, "gpt-4")
			}
		}
	})
}

// Test concurrent usage
func TestConcurrentUsage(t *testing.T) {
	if !IsRustAvailable() {
		t.Skip("Rust tokenizer not available - run with -tags=rusttokenizer")
	}

	const numGoroutines = 100
	const numIterations = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				count, err := CountTokens(benchmarkTexts.medium, "gpt-4")
				if err != nil {
					errors <- fmt.Errorf("goroutine %d: %w", id, err)
					return
				}
				if count <= 0 {
					errors <- fmt.Errorf("goroutine %d: invalid count %d", id, count)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Fatal(err)
	}
}
