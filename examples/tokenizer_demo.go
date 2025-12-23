// Command tokenizer_demo demonstrates the tokenizer usage and performance
// To build and run:
//   go run -tags=rusttokenizer examples/tokenizer_demo.go -check
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/tokenizer"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/utils"
)

var (
	checkImplementation = flag.Bool("check", false, "Check which implementation is active")
	runBenchmarks       = flag.Bool("bench", false, "Run performance benchmarks")
	model               = flag.String("model", "gpt-4", "Model name for tokenization")
	text                = flag.String("text", "", "Text to tokenize (uses sample text if empty)")
)

func main() {
	flag.Parse()

	if *checkImplementation {
		checkImpl()
		return
	}

	if *runBenchmarks {
		runBenchs()
		return
	}

	// Default demo
	demo()
}

func checkImpl() {
	fmt.Println("Tokenizer Implementation Check")
	fmt.Println("================================")
	fmt.Printf("Implementation: %s\n", tokenizer.GetImplementationName())
	fmt.Printf("Rust Available: %v\n", tokenizer.IsRustAvailable())
}

func demo() {
	fmt.Println("Tokenizer Demo")
	fmt.Println("===============")
	fmt.Printf("Using: %s\n\n", tokenizer.GetImplementationName())

	// Sample texts
	texts := []struct {
		name string
		text string
	}{
		{
			name: "Short text",
			text: "Hello, world!",
		},
		{
			name: "Medium text",
			text: "This is a medium-length text that contains several sentences. " +
				"It demonstrates how tokenization works on typical chat messages.",
		},
		{
			name: "Code snippet",
			text: `func main() {
	fmt.Println("Hello, World!")
}`,
		},
		{
			name: "Unicode text",
			text: "Hello 世界 🌍 日本語 한국어",
		},
	}

	if *text != "" {
		texts = []struct {
			name string
			text string
		}{{
			name: "User provided",
			text: *text,
		}}
	}

	model := *model

	fmt.Printf("Model: %s\n\n", model)

	for _, tt := range texts {
		count, err := utils.CountTokens(tt.text, model)
		if err != nil {
			fmt.Printf("Error tokenizing %s: %v\n", tt.name, err)
			continue
		}

		fmt.Printf("%s:\n", tt.name)
		fmt.Printf("  Text: %q\n", truncate(tt.text, 50))
		fmt.Printf("  Tokens: %d\n", count)
		fmt.Printf("  Estimate: %d (bytes/%.1f)\n",
			utils.EstimateTokensFromString(tt.text),
			utils.BytesPerToken)

		// Show character/token ratio
		if count > 0 {
			fmt.Printf("  Chars per token: %.1f\n", float64(len(tt.text))/float64(count))
		}
		fmt.Println()
	}

	// Demo batch counting
	fmt.Println("Batch Tokenization Demo:")
	fmt.Println("------------------------")

	messages := []types.ChatMessage{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello, how are you?"},
		{Role: "assistant", Content: "I'm doing well, thank you!"},
		{Role: "user", Content: "Can you help me with something?"},
	}

	start := time.Now()
	count, err := utils.CountTokensFromMessages(messages, model)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Messages: %d\n", len(messages))
	fmt.Printf("Total tokens: %d\n", count)
	fmt.Printf("Time: %v\n", elapsed)
	fmt.Printf("Tokens/second: %.0f\n", float64(count)/elapsed.Seconds())
}

func runBenchs() {
	fmt.Println("Performance Benchmarks")
	fmt.Println("======================")
	fmt.Printf("Using: %s\n\n", tokenizer.GetImplementationName())

	// Test texts
	testCases := []struct {
		name string
		text string
	}{
		{
			name: "Short",
			text: "Hello, world!",
		},
		{
			name: "Medium",
			text: generateParagraph(5),
		},
		{
			name: "Long",
			text: generateParagraph(50),
		},
	}

	model := *model
	iterations := 10000

	for _, tc := range testCases {
		// Warm up
		for i := 0; i < 100; i++ {
			utils.CountTokens(tc.text, model)
		}

		// Measure
		start := time.Now()
		for i := 0; i < iterations; i++ {
			utils.CountTokens(tc.text, model)
		}
		elapsed := time.Since(start)

		// Get token count
		count, _ := utils.CountTokens(tc.text, model)

		// Results
		fmt.Printf("%s (%d tokens):\n", tc.name, count)
		fmt.Printf("  Total time: %v\n", elapsed)
		fmt.Printf("  Per call: %v\n", elapsed/time.Duration(iterations))
		fmt.Printf("  Ops/sec: %.0f\n", float64(iterations)/elapsed.Seconds())
		fmt.Printf("  Tokens/sec: %.0f\n", float64(count*iterations)/elapsed.Seconds())
		fmt.Println()
	}

	// Batch benchmark
	fmt.Println("Batch Benchmark:")
	messages := generateTestMessages(100)

	start := time.Now()
	for i := 0; i < 1000; i++ {
		utils.CountTokensFromMessages(messages, model)
	}
	elapsed := time.Since(start)

	fmt.Printf("  100 messages x 1000 iterations\n")
	fmt.Printf("  Total time: %v\n", elapsed)
	fmt.Printf("  Per batch: %v\n", elapsed/1000)
	fmt.Printf("  Batches/sec: %.0f\n", 1000.0/elapsed.Seconds())

	totalTokens, _ := utils.CountTokensFromMessages(messages, model)
	fmt.Printf("  Tokens/sec: %.0f\n", float64(totalTokens*1000)/elapsed.Seconds())
}

func generateParagraph(sentences int) string {
	words := []string{
		"The", "quick", "brown", "fox", "jumps", "over", "the", "lazy", "dog",
		"and", "runs", "through", "the", "meadow", "chasing", "butterflies",
		"while", "the", "sun", "shines", "brightly", "in", "the", "blue", "sky",
	}

	para := ""
	for i := 0; i < sentences; i++ {
		for j := 0; j < 15; j++ {
			para += words[(i+j)%len(words)] + " "
		}
	}
	return para
}

func generateTestMessages(count int) []types.ChatMessage {
	messages := make([]types.ChatMessage, count)
	for i := 0; i < count; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = types.ChatMessage{
			Role:    role,
			Content: fmt.Sprintf("Message %d: %s", i, generateParagraph(3)),
		}
	}
	return messages
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func init() {
	flag.Parse()
}
