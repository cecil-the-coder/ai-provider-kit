// Package telemetry provides telemetry utilities for AI provider tracking and monitoring.
// It includes user agent generation with SDK version, OS, and architecture information.
package telemetry

import (
	"fmt"
	"runtime"
	"sync"
)

const (
	// SDKName is the name of the SDK
	SDKName = "ai-provider-kit"
	// SDKVersion is the version of the SDK
	// This should be updated with each release
	SDKVersion = "0.1.0"
)

var (
	// cached user agent string
	cachedUserAgent string
	// mutex to protect cache initialization
	userAgentOnce sync.Once
)

// GetUserAgent returns the User-Agent string for HTTP requests.
// Format: 'ai-provider-kit/VERSION (go VERSION; OS; ARCH)'
// The value is computed once and cached for efficiency.
//
// Example outputs:
//   - "ai-provider-kit/0.1.0 (go1.24.0; linux; amd64)"
//   - "ai-provider-kit/0.1.0 (go1.24.0; darwin; arm64)"
//   - "ai-provider-kit/0.1.0 (go1.24.0; windows; amd64)"
func GetUserAgent() string {
	userAgentOnce.Do(func() {
		cachedUserAgent = buildUserAgent()
	})
	return cachedUserAgent
}

// buildUserAgent constructs the User-Agent string with SDK version, Go version, OS, and architecture.
func buildUserAgent() string {
	goVersion := runtime.Version()    // e.g., "go1.24.0"
	goos := runtime.GOOS              // e.g., "linux", "darwin", "windows"
	goarch := runtime.GOARCH          // e.g., "amd64", "arm64"

	return fmt.Sprintf("%s/%s (%s; %s; %s)", SDKName, SDKVersion, goVersion, goos, goarch)
}

// ResetCache resets the cached User-Agent string.
// This is primarily useful for testing purposes.
func ResetCache() {
	userAgentOnce = sync.Once{}
	cachedUserAgent = ""
}

// GetSDKVersion returns the current SDK version.
func GetSDKVersion() string {
	return SDKVersion
}

// GetSDKName returns the SDK name.
func GetSDKName() string {
	return SDKName
}
