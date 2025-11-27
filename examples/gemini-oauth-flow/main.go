package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Google OAuth Constants (from official gemini-cli - safe to embed per Google's OAuth guidelines)
const (
	GoogleClientID     = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"
	GoogleClientSecret = "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"
	GoogleAuthURL      = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL     = "https://oauth2.googleapis.com/token"
	GoogleScope        = "https://www.googleapis.com/auth/cloud-platform"
	CallbackPort       = 8080
	CallbackEndPort    = 8089
	CallbackPath       = "/oauth2callback"
	Timeout            = 5 * time.Minute
)

// PKCEParams holds PKCE parameters
type PKCEParams struct {
	CodeVerifier  string
	CodeChallenge string
	State         string
}

// TokenResponse represents the token response from Google
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

// Config represents the structure of the config.yaml file
type Config struct {
	Providers map[string]interface{} `yaml:"providers"`
}

// CallbackResult represents the result of OAuth callback
type CallbackResult struct {
	Code  string
	State string
	Error string
}

func main() {
	fmt.Println("=======================================================================")
	fmt.Println("Google Gemini OAuth Authentication")
	fmt.Println("=======================================================================")
	fmt.Println()
	fmt.Println("This tool will guide you through authenticating with Google Gemini using OAuth.")
	fmt.Println("After successful authentication, your credentials will be saved to:")
	fmt.Println("  ~/.mcp-code-api/config.yaml")
	fmt.Println()
	fmt.Println(colorBold("Note:") + " This uses the official Gemini CLI OAuth credentials (safe to embed)")
	fmt.Println()

	ctx := context.Background()

	// Step 1: Generate PKCE parameters
	fmt.Println("Step 1: Generating PKCE security parameters...")
	pkceParams, err := generatePKCEParams()
	if err != nil {
		fmt.Printf("Error generating PKCE params: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(colorGreen("✓") + " PKCE protection enabled!")
	fmt.Println()

	// Step 2: Start callback server
	fmt.Println("Step 2: Starting local callback server...")
	callbackServer, redirectURL, err := startCallbackServer()
	if err != nil {
		fmt.Printf("Error starting callback server: %v\n", err)
		os.Exit(1)
	}
	defer callbackServer.Close()
	fmt.Printf(colorGreen("✓")+" Callback server listening on %s\n", redirectURL)
	fmt.Println()

	// Step 3: Build authorization URL
	fmt.Println("Step 3: Building authorization URL...")
	authURL := buildAuthURL(redirectURL, pkceParams)
	fmt.Println(colorGreen("✓") + " Authorization URL ready!")
	fmt.Println()

	// Step 4: Open browser
	fmt.Println("Step 4: Opening browser for authentication...")
	fmt.Println()
	fmt.Println(colorBold("  Authorization URL: ") + colorCyan(authURL))
	fmt.Println()
	fmt.Println("Opening browser automatically...")
	if err := openBrowser(authURL); err != nil {
		fmt.Println(colorYellow("(Could not open browser automatically. Please visit the URL above manually)"))
	}
	fmt.Println()

	// Step 5: Wait for callback
	fmt.Println("Step 5: Waiting for authorization...")
	fmt.Println("  Complete the login in your browser")
	fmt.Println("  The browser window will close automatically when done")
	fmt.Println()

	result, err := waitForCallback(callbackServer, Timeout)
	if err != nil {
		fmt.Printf("Error during authorization: %v\n", err)
		os.Exit(1)
	}

	if result.Error != "" {
		fmt.Printf("OAuth error: %s\n", result.Error)
		os.Exit(1)
	}

	// Validate state
	if result.State != pkceParams.State {
		fmt.Println("Error: State mismatch (possible CSRF attack)")
		os.Exit(1)
	}

	fmt.Println(colorGreen("✓") + " Authorization code received!")
	fmt.Println()

	// Step 6: Exchange code for token
	fmt.Println("Step 6: Exchanging authorization code for access token...")
	tokenResp, err := exchangeCodeForToken(ctx, result.Code, redirectURL, pkceParams.CodeVerifier)
	if err != nil {
		fmt.Printf("Error exchanging code for token: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(colorGreen("✓") + " Access token obtained!")
	fmt.Println()

	// Step 7: Save tokens to config
	fmt.Println("Step 7: Saving tokens to config...")
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if err := saveToConfig(tokenResp.AccessToken, tokenResp.RefreshToken, expiresAt); err != nil {
		fmt.Printf("Error saving to config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(colorGreen("✓") + " Tokens saved to ~/.mcp-code-api/config.yaml")
	fmt.Println()

	// Step 8: Test the token
	fmt.Println("Step 8: Testing token...")
	if err := testToken(tokenResp.AccessToken); err != nil {
		fmt.Printf(colorYellow("⚠")+" Warning: Token test failed: %v\n", err)
		fmt.Println("  (Token was saved but API test failed. This may be normal if you don't have a project set up yet)")
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
	fmt.Println("You can now use the Gemini provider with OAuth authentication!")
	fmt.Println()
	fmt.Println(colorBold("Note: ") + "If you haven't set up a Google Cloud project yet, you may need to:")
	fmt.Println("  1. Visit https://console.cloud.google.com/")
	fmt.Println("  2. Create a new project or select an existing one")
	fmt.Println("  3. Enable the Generative Language API")
	fmt.Println("  4. Add the project_id to your config.yaml")
	fmt.Println()
}

// generatePKCEParams generates PKCE parameters for OAuth security
func generatePKCEParams() (*PKCEParams, error) {
	// Generate 32-byte random code verifier
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	// Generate SHA256 code challenge
	hash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(hash[:])

	// Generate state for CSRF protection (use verifier as state like cerebras-code-mcp)
	state := codeVerifier

	return &PKCEParams{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
		State:         state,
	}, nil
}

// startCallbackServer starts a local HTTP server to receive OAuth callback
func startCallbackServer() (net.Listener, string, error) {
	// Try ports 8080-8089 to avoid conflicts
	for port := CallbackPort; port <= CallbackEndPort; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
		if err == nil {
			redirectURL := fmt.Sprintf("http://localhost:%d%s", port, CallbackPath)
			return listener, redirectURL, nil
		}
	}
	return nil, "", fmt.Errorf("no available port in range %d-%d", CallbackPort, CallbackEndPort)
}

// buildAuthURL builds the OAuth authorization URL
func buildAuthURL(redirectURL string, pkceParams *PKCEParams) string {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", GoogleClientID)
	params.Set("redirect_uri", redirectURL)
	params.Set("scope", GoogleScope)
	params.Set("state", pkceParams.State)
	params.Set("code_challenge", pkceParams.CodeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("access_type", "offline") // Request refresh token
	params.Set("prompt", "consent")      // Force consent to get refresh token

	return GoogleAuthURL + "?" + params.Encode()
}

// waitForCallback waits for the OAuth callback with timeout
func waitForCallback(listener net.Listener, timeout time.Duration) (*CallbackResult, error) {
	resultChan := make(chan CallbackResult, 1)

	// Setup HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc(CallbackPath, func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		oauthError := r.URL.Query().Get("error")

		resultChan <- CallbackResult{
			Code:  code,
			State: state,
			Error: oauthError,
		}

		// Send success page
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
	<title>Authentication Successful</title>
	<style>
		body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
		h1 { color: #4CAF50; }
		p { font-size: 18px; }
	</style>
</head>
<body>
	<h1>✅ Authentication Successful!</h1>
	<p>You can close this window and return to the terminal.</p>
	<script>setTimeout(function() { window.close(); }, 3000);</script>
</body>
</html>`)
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	defer server.Close()

	// Wait for result or timeout
	select {
	case result := <-resultChan:
		return &result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for authorization (5 minutes)")
	}
}

// exchangeCodeForToken exchanges authorization code for access token
func exchangeCodeForToken(ctx context.Context, code, redirectURL, codeVerifier string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURL)
	data.Set("client_id", GoogleClientID)
	data.Set("client_secret", GoogleClientSecret)
	data.Set("code_verifier", codeVerifier) // PKCE verification

	req, err := http.NewRequestWithContext(ctx, "POST", GoogleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}

// saveToConfig saves the OAuth tokens to the config file
func saveToConfig(accessToken, refreshToken string, expiresAt time.Time) error {
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
		config.Providers = make(map[string]interface{})
	} else {
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}
		if config.Providers == nil {
			config.Providers = make(map[string]interface{})
		}
	}

	// Get or create gemini provider config
	var geminiConfig map[string]interface{}
	if existing, ok := config.Providers["gemini"].(map[string]interface{}); ok {
		geminiConfig = existing
	} else {
		geminiConfig = make(map[string]interface{})
		config.Providers["gemini"] = geminiConfig
	}

	// Create or update oauth_credentials array
	credentialSet := map[string]interface{}{
		"id":            "default",
		"client_id":     GoogleClientID,
		"client_secret": GoogleClientSecret,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_at":    expiresAt.Format(time.RFC3339),
		"scopes":        []string{GoogleScope},
	}

	// Check if oauth_credentials exists and update it
	if existingCreds, ok := geminiConfig["oauth_credentials"].([]interface{}); ok && len(existingCreds) > 0 {
		existingCreds[0] = credentialSet
		geminiConfig["oauth_credentials"] = existingCreds
	} else {
		geminiConfig["oauth_credentials"] = []interface{}{credentialSet}
	}

	// Set default model if not present
	if _, ok := geminiConfig["default_model"]; !ok {
		geminiConfig["default_model"] = "gemini-2.5-pro"
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
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename config: %w", err)
	}

	return nil
}

// testToken tests the OAuth token by making a simple API call
func testToken(accessToken string) error {
	req, err := http.NewRequest("GET", "https://oauth2.googleapis.com/tokeninfo?access_token="+accessToken, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

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
