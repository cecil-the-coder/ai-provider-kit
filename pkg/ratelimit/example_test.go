package ratelimit_test

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
)

// ExampleOpenAIParser demonstrates parsing OpenAI rate limit headers
func ExampleOpenAIParser() {
	// Simulate OpenAI response headers
	headers := http.Header{
		"X-Ratelimit-Limit-Requests":     []string{"60"},
		"X-Ratelimit-Remaining-Requests": []string{"58"},
		"X-Ratelimit-Reset-Requests":     []string{"6m0s"},
		"X-Ratelimit-Limit-Tokens":       []string{"90000"},
		"X-Ratelimit-Remaining-Tokens":   []string{"85000"},
		"X-Ratelimit-Reset-Tokens":       []string{"1m30s"},
		"X-Request-Id":                   []string{"req_abc123"},
	}

	// Create parser and parse headers
	parser := ratelimit.NewOpenAIParser()
	info, err := parser.Parse(headers, "gpt-4")
	if err != nil {
		fmt.Printf("Error parsing headers: %v\n", err)
		return
	}

	// Display parsed information
	fmt.Printf("Provider: %s\n", info.Provider)
	fmt.Printf("Model: %s\n", info.Model)
	fmt.Printf("Requests: %d / %d remaining\n", info.RequestsRemaining, info.RequestsLimit)
	fmt.Printf("Tokens: %d / %d remaining\n", info.TokensRemaining, info.TokensLimit)
	fmt.Printf("Request ID: %s\n", info.RequestID)
	fmt.Printf("Requests reset in: %v\n", time.Until(info.RequestsReset).Round(time.Second))
	fmt.Printf("Tokens reset in: %v\n", time.Until(info.TokensReset).Round(time.Second))

	// Output:
	// Provider: openai
	// Model: gpt-4
	// Requests: 58 / 60 remaining
	// Tokens: 85000 / 90000 remaining
	// Request ID: req_abc123
	// Requests reset in: 6m0s
	// Tokens reset in: 1m30s
}

// ExampleOpenAIParser_withTracker demonstrates using the parser with a rate limit tracker
func ExampleOpenAIParser_withTracker() {
	// Create a tracker to manage rate limits
	tracker := ratelimit.NewTracker()

	// Simulate parsing response headers
	headers := http.Header{
		"X-Ratelimit-Limit-Requests":     []string{"60"},
		"X-Ratelimit-Remaining-Requests": []string{"5"},
		"X-Ratelimit-Reset-Requests":     []string{"30s"},
		"X-Ratelimit-Limit-Tokens":       []string{"90000"},
		"X-Ratelimit-Remaining-Tokens":   []string{"1000"},
		"X-Ratelimit-Reset-Tokens":       []string{"30s"},
	}

	parser := ratelimit.NewOpenAIParser()
	info, _ := parser.Parse(headers, "gpt-4")

	// Update tracker with parsed info
	tracker.Update(info)

	// Check if we can make a request
	if tracker.CanMakeRequest("gpt-4", 500) {
		fmt.Println("Request allowed")
	} else {
		waitTime := tracker.GetWaitTime("gpt-4")
		fmt.Printf("Rate limited. Retry after: %v\n", waitTime.Round(time.Second))
	}

	// Check if we should throttle (99% threshold)
	if tracker.ShouldThrottle("gpt-4", 0.99) {
		fmt.Println("Approaching rate limits - consider throttling")
	}

	// Output:
	// Request allowed
}
