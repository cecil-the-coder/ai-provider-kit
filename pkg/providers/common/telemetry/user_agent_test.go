// Package telemetry provides telemetry utilities for AI provider tracking and monitoring.
package telemetry

import (
	"runtime"
	"strings"
	"sync"
	"testing"
)

// TestGetUserAgent verifies the user agent string format
func TestGetUserAgent(t *testing.T) {
	// Reset cache before test
	ResetCache()

	userAgent := GetUserAgent()

	// Verify format: "ai-provider-kit/VERSION (goVERSION; OS; ARCH)"
	if !strings.HasPrefix(userAgent, "ai-provider-kit/") {
		t.Errorf("User-Agent should start with 'ai-provider-kit/', got: %s", userAgent)
	}

	// Verify contains version
	if !strings.Contains(userAgent, SDKVersion) {
		t.Errorf("User-Agent should contain SDK version %s, got: %s", SDKVersion, userAgent)
	}

	// Verify contains Go version
	goVersion := runtime.Version()
	if !strings.Contains(userAgent, goVersion) {
		t.Errorf("User-Agent should contain Go version %s, got: %s", goVersion, userAgent)
	}

	// Verify contains OS
	goos := runtime.GOOS
	if !strings.Contains(userAgent, goos) {
		t.Errorf("User-Agent should contain OS %s, got: %s", goos, userAgent)
	}

	// Verify contains architecture
	goarch := runtime.GOARCH
	if !strings.Contains(userAgent, goarch) {
		t.Errorf("User-Agent should contain architecture %s, got: %s", goarch, userAgent)
	}

	// Verify format with parentheses
	if !strings.Contains(userAgent, "(") || !strings.Contains(userAgent, ")") {
		t.Errorf("User-Agent should contain parentheses for system info, got: %s", userAgent)
	}

	// Verify semicolons separate components
	partsInParens := strings.Split(strings.Split(userAgent, "(")[1], ")")[0]
	components := strings.Split(partsInParens, ";")
	if len(components) != 3 {
		t.Errorf("User-Agent should have 3 components in parentheses (go version; os; arch), got %d: %s", len(components), userAgent)
	}
}

// TestGetUserAgentCaching verifies that the user agent is cached
func TestGetUserAgentCaching(t *testing.T) {
	// Reset cache before test
	ResetCache()

	// Get user agent twice
	ua1 := GetUserAgent()
	ua2 := GetUserAgent()

	// Should return the same string (cached)
	if ua1 != ua2 {
		t.Errorf("User-Agent should be cached and return same value, got different values: %s vs %s", ua1, ua2)
	}

	// Verify they're the same pointer (true caching)
	if &ua1 == &ua2 {
		t.Log("User-Agent values are cached (same reference)")
	}
}

// TestGetUserAgentConcurrent verifies thread-safety of user agent generation
func TestGetUserAgentConcurrent(t *testing.T) {
	// Reset cache before test
	ResetCache()

	const goroutines = 100
	results := make(chan string, goroutines)
	var wg sync.WaitGroup

	// Launch multiple goroutines to get user agent simultaneously
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- GetUserAgent()
		}()
	}

	wg.Wait()
	close(results)

	// Collect all results
	var userAgents []string
	for ua := range results {
		userAgents = append(userAgents, ua)
	}

	// All should be identical
	if len(userAgents) != goroutines {
		t.Errorf("Expected %d results, got %d", goroutines, len(userAgents))
	}

	firstUA := userAgents[0]
	for i, ua := range userAgents {
		if ua != firstUA {
			t.Errorf("Result %d differs from first result: %s vs %s", i, ua, firstUA)
		}
	}
}

// TestResetCache verifies that cache can be reset
func TestResetCache(t *testing.T) {
	// Get initial user agent
	ua1 := GetUserAgent()

	// Reset cache
	ResetCache()

	// Get user agent again
	ua2 := GetUserAgent()

	// Should still be equal (same build environment)
	if ua1 != ua2 {
		t.Errorf("User-Agent should be identical after reset in same environment: %s vs %s", ua1, ua2)
	}
}

// TestGetSDKVersion verifies SDK version getter
func TestGetSDKVersion(t *testing.T) {
	version := GetSDKVersion()
	if version == "" {
		t.Error("SDK version should not be empty")
	}

	if version != SDKVersion {
		t.Errorf("GetSDKVersion() should return SDKVersion constant, got %s, want %s", version, SDKVersion)
	}
}

// TestGetSDKName verifies SDK name getter
func TestGetSDKName(t *testing.T) {
	name := GetSDKName()
	if name != SDKName {
		t.Errorf("GetSDKName() should return SDKName constant, got %s, want %s", name, SDKName)
	}

	if name != "ai-provider-kit" {
		t.Errorf("SDK name should be 'ai-provider-kit', got %s", name)
	}
}

// TestBuildUserAgent verifies the internal build function
func TestBuildUserAgent(t *testing.T) {
	ua := buildUserAgent()

	// Should match expected format
	expectedPrefix := "ai-provider-kit/" + SDKVersion + " ("
	if !strings.HasPrefix(ua, expectedPrefix) {
		t.Errorf("User-Agent should start with %s, got: %s", expectedPrefix, ua)
	}

	// Should end with closing parenthesis
	if !strings.HasSuffix(ua, ")") {
		t.Errorf("User-Agent should end with ')', got: %s", ua)
	}
}

// TestUserAgentFormat verifies exact format compliance
func TestUserAgentFormat(t *testing.T) {
	ResetCache()
	ua := GetUserAgent()

	// Split by parentheses
	parts := strings.Split(ua, "(")
	if len(parts) != 2 {
		t.Fatalf("User-Agent should have exactly one '(', got: %s", ua)
	}

	// First part: "ai-provider-kit/VERSION "
	sdkPart := strings.TrimSpace(parts[0])
	if !strings.HasPrefix(sdkPart, "ai-provider-kit/") {
		t.Errorf("SDK part should start with 'ai-provider-kit/', got: %s", sdkPart)
	}

	// Second part: "goVERSION; OS; ARCH)"
	systemPart := strings.TrimSuffix(parts[1], ")")
	systemComponents := strings.Split(systemPart, ";")
	if len(systemComponents) != 3 {
		t.Errorf("System part should have 3 components, got %d: %s", len(systemComponents), systemPart)
	}

	// Verify each component
	goVersionPart := strings.TrimSpace(systemComponents[0])
	if !strings.HasPrefix(goVersionPart, "go") {
		t.Errorf("Go version should start with 'go', got: %s", goVersionPart)
	}

	osPart := strings.TrimSpace(systemComponents[1])
	validOS := []string{"linux", "darwin", "windows", "freebsd", "openbsd", "netbsd", "dragonfly", "solaris", "plan9", "android", "ios"}
	osValid := false
	for _, validOSName := range validOS {
		if osPart == validOSName {
			osValid = true
			break
		}
	}
	if !osValid {
		t.Logf("Warning: OS '%s' not in common list, but may be valid", osPart)
	}

	archPart := strings.TrimSpace(systemComponents[2])
	validArch := []string{"amd64", "386", "arm", "arm64", "ppc64", "ppc64le", "mips", "mipsle", "mips64", "mips64le", "s390x", "riscv64", "wasm"}
	archValid := false
	for _, validArchName := range validArch {
		if archPart == validArchName {
			archValid = true
			break
		}
	}
	if !archValid {
		t.Logf("Warning: Architecture '%s' not in common list, but may be valid", archPart)
	}
}

// TestUserAgentExamples documents expected output examples
func TestUserAgentExamples(t *testing.T) {
	ResetCache()
	ua := GetUserAgent()

	t.Logf("Generated User-Agent: %s", ua)

	// Verify it matches documented examples pattern
	// Examples from documentation:
	// - "ai-provider-kit/0.1.0 (go1.24.0; linux; amd64)"
	// - "ai-provider-kit/0.1.0 (go1.24.0; darwin; arm64)"
	// - "ai-provider-kit/0.1.0 (go1.24.0; windows; amd64)"

	expectedPattern := SDKName + "/" + SDKVersion + " (" + runtime.Version() + "; " + runtime.GOOS + "; " + runtime.GOARCH + ")"
	if ua != expectedPattern {
		t.Errorf("User-Agent doesn't match expected pattern.\nGot:      %s\nExpected: %s", ua, expectedPattern)
	}
}

// BenchmarkGetUserAgent benchmarks the cached user agent retrieval
func BenchmarkGetUserAgent(b *testing.B) {
	ResetCache()
	// First call to initialize cache
	_ = GetUserAgent()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetUserAgent()
	}
}

// BenchmarkBuildUserAgent benchmarks the user agent construction
func BenchmarkBuildUserAgent(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = buildUserAgent()
	}
}
