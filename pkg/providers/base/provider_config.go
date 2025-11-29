// Package base provides common functionality and utilities for AI providers.
package base

import (
	"log"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"golang.org/x/time/rate"
)

// ProviderInitConfig holds common initialization parameters for providers.
// This struct centralizes all the configuration data needed to initialize
// a provider, eliminating duplicate parameter passing across providers.
type ProviderInitConfig struct {
	// Provider identification
	ProviderType types.ProviderType
	ProviderName string

	// Provider configuration
	Config types.ProviderConfig

	// HTTP client configuration
	HTTPTimeout time.Duration // 0 means use default (10 seconds)

	// Logging
	Logger *log.Logger // nil means use default logger

	// Client-side rate limiting (for providers without rate limit headers)
	EnableClientRateLimiting bool
	ClientRateLimitRPM       int           // Requests per minute (0 means no limit)
	ClientRateLimitBurst     int           // Token bucket burst size
	ClientRateLimitInterval  time.Duration // Rate limit window (default: 1 minute)
}

// ProviderComponents holds initialized components ready for use by providers.
// This struct packages all common provider infrastructure into a single
// container, eliminating the need for each provider to initialize these
// components individually.
type ProviderComponents struct {
	// HTTP client for API requests
	HTTPClient *http.Client

	// Authentication helper
	AuthHelper *common.AuthHelper

	// Configuration helper
	ConfigHelper *common.ConfigHelper

	// Rate limit helper
	RateLimitHelper *common.RateLimitHelper

	// Base provider with metrics, logging, etc.
	BaseProvider *BaseProvider

	// Client-side rate limiter (only set if EnableClientRateLimiting was true)
	ClientSideLimiter *rate.Limiter

	// Extracted configuration values
	BaseURL      string
	DefaultModel string
	Timeout      time.Duration
	MaxTokens    int
	MergedConfig types.ProviderConfig
}
