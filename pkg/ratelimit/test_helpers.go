package ratelimit

import "time"

// Test helper functions for creating test Info structs with proper initialization
// These helpers work around Go struct literal limitations with embedded structs

// MakeTestInfo creates a test Info struct with proper initialization
func MakeTestInfo(provider, model string, setters ...func(*Info)) *Info {
	i := &Info{
		BaseInfo: BaseInfo{
			Provider: provider,
			Model:    model,
		},
	}
	for _, s := range setters {
		s(i)
	}
	return i
}

// WithRequests sets request-related rate limit fields
func WithRequests(limit, remaining int, reset string) func(*Info) {
	return func(i *Info) {
		i.RequestsLimit = limit
		i.RequestsRemaining = remaining
		if reset != "" {
			i.RequestsReset = MustParseTime(reset)
		}
	}
}

// WithTokens sets token-related rate limit fields
func WithTokens(limit, remaining int, reset string) func(*Info) {
	return func(i *Info) {
		i.TokensLimit = limit
		i.TokensRemaining = remaining
		if reset != "" {
			i.TokensReset = MustParseTime(reset)
		}
	}
}

// WithInputTokens sets Anthropic input token fields
func WithInputTokens(limit, remaining int, reset string) func(*Info) {
	return func(i *Info) {
		i.InputTokensLimit = limit
		i.InputTokensRemaining = remaining
		if reset != "" {
			i.InputTokensReset = MustParseTime(reset)
		}
	}
}

// WithOutputTokens sets Anthropic output token fields
func WithOutputTokens(limit, remaining int, reset string) func(*Info) {
	return func(i *Info) {
		i.OutputTokensLimit = limit
		i.OutputTokensRemaining = remaining
		if reset != "" {
			i.OutputTokensReset = MustParseTime(reset)
		}
	}
}

// WithDailyRequests sets Cerebras daily request fields
func WithDailyRequests(limit, remaining int, reset string) func(*Info) {
	return func(i *Info) {
		i.DailyRequestsLimit = limit
		i.DailyRequestsRemaining = remaining
		if reset != "" {
			i.DailyRequestsReset = MustParseTime(reset)
		}
	}
}

// WithCredits sets OpenRouter credit fields
func WithCredits(limit, remaining float64) func(*Info) {
	return func(i *Info) {
		i.CreditsLimit = limit
		i.CreditsRemaining = remaining
	}
}

// WithFreeTier sets OpenRouter free tier flag
func WithFreeTier(freeTier bool) func(*Info) {
	return func(i *Info) {
		i.IsFreeTier = freeTier
	}
}

// WithRequestID sets the request ID
func WithRequestID(id string) func(*Info) {
	return func(i *Info) {
		i.RequestID = id
	}
}

// WithRetryAfter sets the retry-after duration
func WithRetryAfter(d time.Duration) func(*Info) {
	return func(i *Info) {
		i.RetryAfter = d
	}
}

// WithCustomData sets custom data map
func WithCustomData(data map[string]interface{}) func(*Info) {
	return func(i *Info) {
		i.CustomData = data
	}
}

// MustParseTime parses an RFC3339 timestamp or panics
func MustParseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
