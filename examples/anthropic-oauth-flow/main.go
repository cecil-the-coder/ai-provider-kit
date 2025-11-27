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

	"gopkg.in/yaml.v3"
)

// Anthropic OAuth Constants
const (
	AnthropicClientID    = "9d1c250a-e61b-44d9-88ed-5944d1962f5e" // Public client ID (MAX plan)
	AnthropicAuthURL     = "https://claude.ai/oauth/authorize"
	AnthropicTokenURL    = "https://console.anthropic.com/v1/oauth/token"
	AnthropicRedirectURI = "https://console.anthropic.com/oauth/code/callback"
	AnthropicScope       = "org:create_api_key user:profile user:inference"
	AnthropicAPIBaseURL  = "https://api.anthropic.com"
	ServerPort           = "8765"
	LocalRedirectURI     = "http://localhost:8765/callback"
)

// TokenResponse represents the token response from Anthropic
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

// State holds the OAuth flow state
type State struct {
	codeVerifier string
	oauthState   string
	authCode     chan string
	errChan      chan error
}

func main() {
	fmt.Println("=======================================================================")
	fmt.Println("Anthropic OAuth Authentication (Authorization Code Flow)")
	fmt.Println("=======================================================================")
	fmt.Println()
	fmt.Println("This tool will guide you through authenticating with Anthropic using OAuth.")
	fmt.Println("After successful authentication, your credentials will be saved to:")
	fmt.Println("  ~/.mcp-code-api/config.yaml")
	fmt.Println()
	fmt.Println(colorYellow("NOTE: ") + "Anthropic OAuth is primarily designed for Claude Code.")
	fmt.Println("These credentials may have limited API access compared to regular API keys.")
	fmt.Println()

	ctx := context.Background()

	// Generate PKCE verifier and challenge
	codeVerifier := generateCodeVerifier()
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Generate state for CSRF protection
	oauthState := generateCodeVerifier() // Reuse the same random generation

	// Create state holder
	state := &State{
		codeVerifier: codeVerifier,
		oauthState:   oauthState,
		authCode:     make(chan string, 1),
		errChan:      make(chan error, 1),
	}

	// Step 1: Start local callback server
	fmt.Println("Step 1: Starting local callback server...")
	server := startCallbackServer(state)
	defer server.Close()
	fmt.Println(colorGreen("✓") + " Callback server started on " + LocalRedirectURI)
	fmt.Println()

	// Step 2: Build authorization URL
	fmt.Println("Step 2: Building authorization URL...")
	authURL := buildAuthURL(codeChallenge, oauthState)
	fmt.Println(colorGreen("✓") + " Authorization URL ready!")
	fmt.Println()

	// Step 3: Open browser for authorization
	fmt.Println("Step 3: Please authenticate in your browser:")
	fmt.Println()
	fmt.Println(colorBold("  Authorization URL: ") + colorCyan(authURL))
	fmt.Println()
	fmt.Println("Opening browser automatically...")
	if err := openBrowser(authURL); err != nil {
		fmt.Println(colorYellow("(Could not open browser automatically. Please visit the URL above manually)"))
	}
	fmt.Println()

	// Step 4: Wait for authorization code
	fmt.Println("Step 4: Waiting for authorization...")
	fmt.Println("  (Complete the authentication in your browser)")
	fmt.Println()

	var authCode string
	select {
	case authCode = <-state.authCode:
		fmt.Println(colorGreen("✓") + " Authorization code received!")
	case err := <-state.errChan:
		fmt.Printf(colorRed("✗")+" Authorization failed: %v\n", err)
		os.Exit(1)
	case <-time.After(5 * time.Minute):
		fmt.Println(colorRed("✗") + " Authorization timeout - please restart the process")
		os.Exit(1)
	}
	fmt.Println()

	// Step 5: Exchange code for token
	fmt.Println("Step 5: Exchanging code for access token...")
	tokenResp, err := exchangeCodeForToken(ctx, authCode, codeVerifier, oauthState)
	if err != nil {
		fmt.Printf(colorRed("✗")+" Token exchange failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(colorGreen("✓") + " Access token obtained!")
	fmt.Println()

	// Step 6: Save tokens to config
	fmt.Println("Step 6: Saving tokens to config...")
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if err := saveToConfig(tokenResp.AccessToken, tokenResp.RefreshToken, expiresAt); err != nil {
		fmt.Printf(colorRed("✗")+" Error saving to config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(colorGreen("✓") + " Tokens saved to ~/.mcp-code-api/config.yaml")
	fmt.Println()

	// Step 7: Test the token
	fmt.Println("Step 7: Testing token...")
	if err := testToken(tokenResp.AccessToken); err != nil {
		fmt.Printf(colorYellow("⚠")+" Warning: Token test failed: %v\n", err)
		fmt.Println("  Note: This is expected - Anthropic OAuth tokens are restricted to Claude Code.")
		fmt.Println("  The token was saved successfully and will work with Claude Code.")
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
	fmt.Println("You can now use the Anthropic provider with OAuth authentication!")
	fmt.Println()
	fmt.Println(colorYellow("Important: ") + "These OAuth credentials are designed for Claude Code.")
	fmt.Println("For general API access, consider using an API key from console.anthropic.com")
	fmt.Println()
}

// buildAuthURL builds the authorization URL with PKCE
func buildAuthURL(codeChallenge, oauthState string) string {
	params := fmt.Sprintf(
		"client_id=%s&response_type=code&redirect_uri=%s&scope=%s&code_challenge=%s&code_challenge_method=S256&state=%s",
		AnthropicClientID,
		LocalRedirectURI,
		strings.ReplaceAll(AnthropicScope, " ", "+"),
		codeChallenge,
		oauthState,
	)
	return AnthropicAuthURL + "?" + params
}

// startCallbackServer starts the local HTTP server to receive the OAuth callback
func startCallbackServer(state *State) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Extract authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			errorMsg := r.URL.Query().Get("error")
			errorDesc := r.URL.Query().Get("error_description")
			if errorMsg != "" {
				state.errChan <- fmt.Errorf("%s: %s", errorMsg, errorDesc)
				http.Error(w, "Authorization failed: "+errorMsg, http.StatusBadRequest)
				return
			}
			state.errChan <- fmt.Errorf("no authorization code received")
			http.Error(w, "No authorization code received", http.StatusBadRequest)
			return
		}

		// Verify state parameter for CSRF protection
		returnedState := r.URL.Query().Get("state")
		if returnedState != state.oauthState {
			state.errChan <- fmt.Errorf("state mismatch: possible CSRF attack")
			http.Error(w, "State mismatch: possible CSRF attack", http.StatusBadRequest)
			return
		}

		// Send code to main goroutine
		state.authCode <- code

		// Display success page
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<html>
			<head><title>Authentication Successful</title></head>
			<body style="font-family: Arial, sans-serif; text-align: center; padding: 50px;">
				<h1 style="color: #28a745;">✓ Authentication Successful!</h1>
				<p>You have successfully authenticated with Anthropic.</p>
				<p>You can close this window and return to the terminal.</p>
			</body>
			</html>
		`)
	})

	server := &http.Server{
		Addr:    ":" + ServerPort,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			state.errChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	return server
}

// exchangeCodeForToken exchanges the authorization code for an access token
func exchangeCodeForToken(ctx context.Context, code, codeVerifier, oauthState string) (*TokenResponse, error) {
	requestBody := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"state":         oauthState,
		"client_id":     AnthropicClientID,
		"redirect_uri":  LocalRedirectURI,
		"code_verifier": codeVerifier,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", AnthropicTokenURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

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
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &tokenResp, nil
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

	// Get or create anthropic provider config
	var anthropicConfig map[string]interface{}
	if existing, ok := config.Providers["anthropic"].(map[string]interface{}); ok {
		anthropicConfig = existing
	} else {
		anthropicConfig = make(map[string]interface{})
		config.Providers["anthropic"] = anthropicConfig
	}

	// Create or update oauth_credentials array
	credentialSet := map[string]interface{}{
		"id":            "default",
		"client_id":     AnthropicClientID,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_at":    expiresAt.Format(time.RFC3339),
		"scopes":        strings.Split(AnthropicScope, " "),
	}

	// Check if oauth_credentials exists and update it
	if existingCreds, ok := anthropicConfig["oauth_credentials"].([]interface{}); ok && len(existingCreds) > 0 {
		// Update the first credential set
		existingCreds[0] = credentialSet
		anthropicConfig["oauth_credentials"] = existingCreds
	} else {
		// Create new oauth_credentials array
		anthropicConfig["oauth_credentials"] = []interface{}{credentialSet}
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
	// Try to list models
	req, err := http.NewRequest("GET", AnthropicAPIBaseURL+"/v1/models?limit=10", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

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

func colorRed(s string) string {
	return "\033[31m" + s + "\033[0m"
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
