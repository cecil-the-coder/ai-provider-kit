// Package virtual provides composite provider implementations that combine multiple underlying providers.
//
// Virtual providers implement the same Provider interface as concrete providers (OpenAI, Anthropic, etc.)
// but delegate to one or more underlying providers based on different strategies. This allows
// applications to build sophisticated request handling without changing their provider interface.
//
// # Available Virtual Providers
//
// The package includes three virtual provider types, each in its own sub-package:
//
// # Racing Provider (racing/)
//
// The racing provider sends requests to multiple providers concurrently and returns the first
// successful response. This is useful for:
//
//   - Minimizing latency by racing multiple providers
//   - Best-of-N selection when combined with validation
//   - Automatic failover when one provider is slow
//
// # Fallback Provider (fallback/)
//
// The fallback provider tries providers in sequence, falling back to the next on failure.
// This is useful for:
//
//   - High availability with primary/secondary providers
//   - Cost optimization (try cheaper providers first)
//   - Graceful degradation during outages
//
// # Load Balance Provider (loadbalance/)
//
// The load balance provider distributes requests across multiple providers using various
// strategies. This is useful for:
//
//   - Distributing load across provider accounts
//   - Rate limit management across multiple API keys
//   - Geographic distribution of requests
//
// # Usage
//
// Virtual providers are configured through the factory package and can be nested:
//
//	// Racing with fallback backup
//	providers := map[string]types.Provider{
//	    "fast": racing.New(openai, anthropic),
//	    "reliable": fallback.New(openai, anthropic, gemini),
//	}
//
// All virtual providers implement the standard types.Provider interface, making them
// interchangeable with concrete providers throughout the application.
package virtual
