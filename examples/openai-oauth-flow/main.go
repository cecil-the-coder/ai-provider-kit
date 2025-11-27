package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// OpenAI API Constants
const (
	OpenAIAPIBaseURL = "https://api.openai.com/v1"
)

// Config represents the structure of the config.yaml file
type Config struct {
	Providers map[string]interface{} `yaml:"providers"`
}

// OpenAIModelsResponse represents the response from the models API
type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// OpenAIModel represents a model in the OpenAI API
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func main() {
	fmt.Println("=======================================================================")
	fmt.Println("OpenAI API Key Authentication")
	fmt.Println("=======================================================================")
	fmt.Println()
	fmt.Println("IMPORTANT: OpenAI uses API key authentication, NOT OAuth!")
	fmt.Println()
	fmt.Println("Unlike some AI providers, OpenAI does not support OAuth 2.0 for API")
	fmt.Println("access. Instead, you need to use an API key from your OpenAI account.")
	fmt.Println()
	fmt.Println("This tool will help you:")
	fmt.Println("  1. Validate your OpenAI API key")
	fmt.Println("  2. Save it to the config file: ~/.mcp-code-api/config.yaml")
	fmt.Println()
	fmt.Println("=======================================================================")
	fmt.Println()

	// Step 1: Get API key from user
	fmt.Println("Step 1: Enter your OpenAI API Key")
	fmt.Println()
	fmt.Println("To get an API key:")
	fmt.Println("  1. Go to https://platform.openai.com/api-keys")
	fmt.Println("  2. Sign in to your OpenAI account")
	fmt.Println("  3. Click 'Create new secret key'")
	fmt.Println("  4. Copy the key (it starts with 'sk-')")
	fmt.Println()
	fmt.Print("Enter your OpenAI API key: ")

	reader := bufio.NewReader(os.Stdin)
	apiKey, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		os.Exit(1)
	}
	apiKey = strings.TrimSpace(apiKey)

	if apiKey == "" {
		fmt.Println(colorRed("Error:") + " API key cannot be empty")
		os.Exit(1)
	}

	// Basic validation - OpenAI keys typically start with "sk-"
	if !strings.HasPrefix(apiKey, "sk-") {
		fmt.Println(colorYellow("Warning:") + " OpenAI API keys typically start with 'sk-'")
		fmt.Println("Are you sure this is a valid OpenAI API key?")
		fmt.Print("Continue anyway? (y/n): ")

		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(strings.ToLower(confirm))
		if confirm != "y" && confirm != "yes" {
			fmt.Println("Cancelled.")
			os.Exit(0)
		}
	}

	fmt.Println()

	// Step 2: Validate the API key
	fmt.Println("Step 2: Validating API key...")
	ctx := context.Background()
	if err := validateAPIKey(ctx, apiKey); err != nil {
		fmt.Printf(colorRed("✗")+" API key validation failed: %v\n", err)
		fmt.Println()
		fmt.Println("Common reasons for validation failure:")
		fmt.Println("  - The API key is invalid or has been revoked")
		fmt.Println("  - Your OpenAI account has insufficient credits")
		fmt.Println("  - Network connectivity issues")
		fmt.Println()
		fmt.Print("Save the key anyway? (y/n): ")

		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(strings.ToLower(confirm))
		if confirm != "y" && confirm != "yes" {
			fmt.Println("Cancelled.")
			os.Exit(1)
		}
	} else {
		fmt.Println(colorGreen("✓") + " API key is valid!")
	}
	fmt.Println()

	// Step 3: Save to config
	fmt.Println("Step 3: Saving API key to config...")
	if err := saveToConfig(apiKey); err != nil {
		fmt.Printf(colorRed("✗")+" Error saving to config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(colorGreen("✓") + " API key saved to ~/.mcp-code-api/config.yaml")
	fmt.Println()

	// Step 4: Display success information
	fmt.Println("=======================================================================")
	fmt.Println("Configuration Complete")
	fmt.Println("=======================================================================")
	fmt.Println()
	fmt.Printf("  API Key: %s... (first 10 chars)\n", maskToken(apiKey, 10))
	fmt.Println()
	fmt.Println("You can now use the OpenAI provider with API key authentication!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  - Try the demo-client example to test your configuration")
	fmt.Println("  - Visit https://platform.openai.com/usage to monitor your usage")
	fmt.Println("  - Visit https://platform.openai.com/api-keys to manage your keys")
	fmt.Println()
	fmt.Println("Security reminder:")
	fmt.Println("  - Never commit your API key to version control")
	fmt.Println("  - Never share your API key publicly")
	fmt.Println("  - Rotate your keys periodically")
	fmt.Println("  - Set usage limits in your OpenAI dashboard")
	fmt.Println()
}

// validateAPIKey validates the API key by making a test API call
func validateAPIKey(ctx context.Context, apiKey string) error {
	// Try to list models - this is a simple, low-cost API call
	url := OpenAIAPIBaseURL + "/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// Parse the response to show some info
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			var modelsResp OpenAIModelsResponse
			if err := json.Unmarshal(body, &modelsResp); err == nil {
				// Count chat models
				chatModels := 0
				for _, model := range modelsResp.Data {
					if isChatModel(model.ID) {
						chatModels++
					}
				}
				fmt.Printf("  Found %d available models (%d chat models)\n", len(modelsResp.Data), chatModels)
			}
		}
		return nil
	}

	// Read error response
	body, _ := io.ReadAll(resp.Body)

	// Try to parse error
	var errorResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error.Message != "" {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return fmt.Errorf("invalid API key (401): %s", errorResp.Error.Message)
		case http.StatusForbidden:
			return fmt.Errorf("access forbidden (403): %s", errorResp.Error.Message)
		case http.StatusTooManyRequests:
			return fmt.Errorf("rate limited (429): %s", errorResp.Error.Message)
		default:
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, errorResp.Error.Message)
		}
	}

	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
}

// saveToConfig saves the API key to the config file
func saveToConfig(apiKey string) error {
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

	// Get or create openai provider config
	var openaiConfig map[string]interface{}
	if existing, ok := config.Providers["openai"].(map[string]interface{}); ok {
		openaiConfig = existing
	} else {
		openaiConfig = make(map[string]interface{})
		config.Providers["openai"] = openaiConfig
	}

	// Set the API key
	openaiConfig["api_key"] = apiKey

	// Optionally set default model if not already set
	if _, exists := openaiConfig["default_model"]; !exists {
		openaiConfig["default_model"] = "gpt-4o"
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

// isChatModel checks if a model ID is a chat model
func isChatModel(modelID string) bool {
	chatModels := []string{
		"gpt-4", "gpt-4-32k", "gpt-4-0613", "gpt-4-32k-0613",
		"gpt-4-turbo", "gpt-4-turbo-preview",
		"gpt-3.5-turbo", "gpt-3.5-turbo-16k", "gpt-3.5-turbo-0613",
		"gpt-4o", "gpt-4o-mini",
	}
	for _, model := range chatModels {
		if strings.HasPrefix(modelID, model) {
			return true
		}
	}
	return false
}

// Helper functions for formatting

func maskToken(token string, length int) string {
	if len(token) < length {
		return token
	}
	return token[:length]
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
