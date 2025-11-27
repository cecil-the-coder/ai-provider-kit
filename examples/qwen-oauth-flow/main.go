package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Qwen OAuth Constants
const (
	QwenClientID      = "f0304373b74a44d2b584a3fb70ca9e56" // Public client ID
	QwenTokenURL      = "https://chat.qwen.ai/api/v1/oauth2/token"
	QwenDeviceAuthURL = "https://chat.qwen.ai/api/v1/oauth2/device/code"
	QwenVerifyURL     = "https://chat.qwen.ai/device"
	QwenAPIBaseURL    = "https://portal.qwen.ai/v1"
	QwenScope         = "model.completion"
	MaxPollAttempts   = 60
	PollIntervalSec   = 5
)

// DeviceCodeResponse represents the device code response from Qwen
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse represents the token response from Qwen
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

// TokenErrorResponse represents an error response during token polling
type TokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Config represents the structure of the config.yaml file
type Config struct {
	Providers map[string]interface{} `yaml:"providers"`
}

func main() {
	fmt.Println("=======================================================================")
	fmt.Println("Qwen OAuth Authentication")
	fmt.Println("=======================================================================")
	fmt.Println()
	fmt.Println("This tool will guide you through authenticating with Qwen using OAuth.")
	fmt.Println("After successful authentication, your credentials will be saved to:")
	fmt.Println("  ~/.mcp-code-api/config.yaml")
	fmt.Println()

	ctx := context.Background()

	// Generate PKCE verifier and challenge
	codeVerifier := generateCodeVerifier()
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Step 1: Request device code
	fmt.Println("Step 1: Requesting device code...")
	deviceCodeResp, err := requestDeviceCode(ctx, codeChallenge)
	if err != nil {
		fmt.Printf("Error requesting device code: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(colorGreen("✓") + " Device code obtained!")
	fmt.Println()

	// Step 2: Display user code and verification URL
	fmt.Println("Step 2: Please authenticate in your browser:")
	fmt.Println()
	fmt.Println(colorBold("  Verification URL: ") + colorCyan(deviceCodeResp.VerificationURI))
	fmt.Println(colorBold("  User Code: ") + colorYellow(deviceCodeResp.UserCode))
	fmt.Println()

	// Determine the verification URL to open
	verifyURL := deviceCodeResp.VerificationURI
	if deviceCodeResp.VerificationURIComplete != "" {
		verifyURL = deviceCodeResp.VerificationURIComplete
	}

	// Try to open browser
	fmt.Println("Opening browser automatically...")
	if err := openBrowser(verifyURL); err != nil {
		fmt.Println(colorYellow("(Could not open browser automatically. Please visit the URL above manually)"))
	}
	fmt.Println()

	// Step 3: Poll for token
	fmt.Println("Step 3: Waiting for authentication...")
	interval := deviceCodeResp.Interval
	if interval == 0 {
		interval = PollIntervalSec
	}

	tokenResp, err := pollForToken(ctx, deviceCodeResp.DeviceCode, codeVerifier, interval)
	if err != nil {
		fmt.Printf("Error during authentication: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(colorGreen("✓") + " Authentication successful!")
	fmt.Println()

	// Step 4: Save tokens to config
	fmt.Println("Step 4: Saving tokens to config...")
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if err := saveToConfig(tokenResp.AccessToken, tokenResp.RefreshToken, expiresAt); err != nil {
		fmt.Printf("Error saving to config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(colorGreen("✓") + " Tokens saved to ~/.mcp-code-api/config.yaml")
	fmt.Println()

	// Step 5: Test the token
	fmt.Println("Step 5: Testing token...")
	if err := testToken(tokenResp.AccessToken); err != nil {
		fmt.Printf(colorYellow("⚠")+" Warning: Token test failed: %v\n", err)
		fmt.Println("  (Token was saved but API test failed. This may be normal if you don't have API access yet)")
	} else {
		fmt.Println(colorGreen("✓") + " Token is valid!")
	}
	fmt.Println()

	// Display success summary
	fmt.Println("=======================================================================")
	fmt.Println("Authentication Complete")
	fmt.Println("=======================================================================")
	fmt.Println()
	fmt.Printf("  Access Token:  %s... (first 10 chars)\n", maskToken(tokenResp.AccessToken, 10))
	fmt.Printf("  Refresh Token: %s... (first 10 chars)\n", maskToken(tokenResp.RefreshToken, 10))
	fmt.Printf("  Expires:       %s (%s)\n", expiresAt.Format("2006-01-02 15:04:05"), formatDuration(time.Until(expiresAt)))
	fmt.Println()
	fmt.Println("You can now use the Qwen provider with OAuth authentication!")
	fmt.Println()
}

// requestDeviceCode requests a device code from Qwen with PKCE support
func requestDeviceCode(ctx context.Context, codeChallenge string) (*DeviceCodeResponse, error) {
	// Build the request body with PKCE parameters
	data := fmt.Sprintf("client_id=%s&scope=%s&code_challenge=%s&code_challenge_method=S256",
		QwenClientID, QwenScope, codeChallenge)

	req, err := http.NewRequestWithContext(ctx, "POST", QwenDeviceAuthURL, strings.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-request-id", uuid.New().String())
	req.Header.Set("User-Agent", "AI-Provider-Kit-OAuth/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var deviceCodeResp DeviceCodeResponse
	if err := json.Unmarshal(body, &deviceCodeResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &deviceCodeResp, nil
}

// pollForToken polls the token endpoint until authentication is complete
func pollForToken(ctx context.Context, deviceCode, codeVerifier string, interval int) (*TokenResponse, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	for attempt := 1; attempt <= MaxPollAttempts; attempt++ {
		fmt.Printf("  ⏳ Polling... (attempt %d/%d)\n", attempt, MaxPollAttempts)

		// Build the request with PKCE verifier
		data := fmt.Sprintf("grant_type=urn:ietf:params:oauth:grant-type:device_code&device_code=%s&client_id=%s&code_verifier=%s",
			deviceCode, QwenClientID, codeVerifier)

		req, err := http.NewRequestWithContext(ctx, "POST", QwenTokenURL, strings.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("x-request-id", uuid.New().String())
		req.Header.Set("User-Agent", "AI-Provider-Kit-OAuth/1.0")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		// Check for success
		if resp.StatusCode == http.StatusOK {
			var tokenResp TokenResponse
			if err := json.Unmarshal(body, &tokenResp); err != nil {
				return nil, fmt.Errorf("failed to parse token response: %w", err)
			}
			return &tokenResp, nil
		}

		// Check for error responses
		var errorResp TokenErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error != "" {
			switch errorResp.Error {
			case "authorization_pending":
				// User hasn't authorized yet, continue polling
			case "slow_down":
				// Increase polling interval
				interval = interval * 2
			case "expired_token":
				return nil, fmt.Errorf("device code expired - please restart the authentication process")
			case "access_denied":
				return nil, fmt.Errorf("user denied the authorization request")
			default:
				return nil, fmt.Errorf("OAuth error: %s - %s", errorResp.Error, errorResp.ErrorDescription)
			}
		}

		// Wait before next poll
		if attempt < MaxPollAttempts {
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}

	return nil, fmt.Errorf("authentication timeout - user did not complete authorization")
}

// saveToConfig saves the OAuth tokens to the config file
func saveToConfig(accessToken, refreshToken string, expiresAt time.Time) error {
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".mcp-code-api")
	configPath := filepath.Join(configDir, "config.yaml")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read existing config or create new one
	var config Config
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read config: %w", err)
		}
		// Config doesn't exist, create new one
		config.Providers = make(map[string]interface{})
	} else {
		// Parse existing config
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}
		if config.Providers == nil {
			config.Providers = make(map[string]interface{})
		}
	}

	// Get or create qwen provider config
	var qwenConfig map[string]interface{}
	if existing, ok := config.Providers["qwen"].(map[string]interface{}); ok {
		qwenConfig = existing
	} else {
		qwenConfig = make(map[string]interface{})
		config.Providers["qwen"] = qwenConfig
	}

	// Create or update oauth_credentials array
	credentialSet := map[string]interface{}{
		"id":            "default",
		"client_id":     QwenClientID,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_at":    expiresAt.Format(time.RFC3339),
		"scopes":        []string{QwenScope},
	}

	// Check if oauth_credentials exists and update it
	if existingCreds, ok := qwenConfig["oauth_credentials"].([]interface{}); ok && len(existingCreds) > 0 {
		// Update the first credential set
		existingCreds[0] = credentialSet
		qwenConfig["oauth_credentials"] = existingCreds
	} else {
		// Create new oauth_credentials array
		qwenConfig["oauth_credentials"] = []interface{}{credentialSet}
	}

	// Marshal config back to YAML
	newData, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to temp file first (atomic write)
	tempPath := configPath + ".tmp"
	if err := os.WriteFile(tempPath, newData, 0600); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	// Rename temp file to actual config
	if err := os.Rename(tempPath, configPath); err != nil {
		os.Remove(tempPath) // Clean up temp file on error
		return fmt.Errorf("failed to rename config: %w", err)
	}

	return nil
}

// testToken tests the OAuth token by making a simple API call
func testToken(accessToken string) error {
	// Make a simple API call to list models
	req, err := http.NewRequest("GET", QwenAPIBaseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
}

// openBrowser attempts to open the default browser with the given URL
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

// Helper functions for formatting

func maskToken(token string, length int) string {
	if len(token) < length {
		return token
	}
	return token[:length]
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

// ANSI color codes for better UX
func colorGreen(s string) string {
	return "\033[32m" + s + "\033[0m"
}

func colorYellow(s string) string {
	return "\033[33m" + s + "\033[0m"
}

func colorCyan(s string) string {
	return "\033[36m" + s + "\033[0m"
}

func colorBold(s string) string {
	return "\033[1m" + s + "\033[0m"
}

// PKCE helper functions

// generateCodeVerifier generates a random code verifier for PKCE
func generateCodeVerifier() string {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("failed to generate random bytes: %v", err))
	}

	// Base64 URL encode without padding
	return base64.RawURLEncoding.EncodeToString(bytes)
}

// generateCodeChallenge generates a code challenge from the verifier using SHA256
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}
