package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"gopkg.in/yaml.v3"
)

// Config represents the complete configuration for the demo client
type Config struct {
	App           AppConfig                 `yaml:"app"`
	Providers     map[string]ProviderConfig `yaml:"providers"`
	TestScenarios []TestScenario            `yaml:"test_scenarios"`
	RateLimits    RateLimitConfig           `yaml:"rate_limits"`
	Retry         RetryConfig               `yaml:"retry"`
	HealthCheck   HealthCheckConfig         `yaml:"health_check"`
}

// AppConfig contains application-level settings
type AppConfig struct {
	LogLevel     string  `yaml:"log_level"`
	TokenStorage string  `yaml:"token_storage"`
	TokenFile    string  `yaml:"token_file"`
	TestPrompt   string  `yaml:"test_prompt"`
	MaxTokens    int     `yaml:"max_tokens"`
	Temperature  float64 `yaml:"temperature"`
}

// ProviderConfig contains provider-specific configuration
type ProviderConfig struct {
	Enabled     bool     `yaml:"enabled"`
	AuthType    string   `yaml:"auth_type"`
	APIKey      string   `yaml:"api_key"`
	APIKeys     []string `yaml:"api_keys"` // For multi-key support
	Model       string   `yaml:"model"`
	BaseURL     string   `yaml:"base_url"`
	Timeout     int      `yaml:"timeout"`
	MaxTokens   int      `yaml:"max_tokens"`
	Temperature float64  `yaml:"temperature"`

	// OAuth fields
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	RedirectURL  string   `yaml:"redirect_url"`
	Scopes       []string `yaml:"scopes"`
	ProjectID    string   `yaml:"project_id"`
	AccessToken  string   `yaml:"access_token"`
	RefreshToken string   `yaml:"refresh_token"`
	TokenExpiry  string   `yaml:"token_expiry"`
}

// TestScenario defines a test case to run
type TestScenario struct {
	Name        string  `yaml:"name"`
	Prompt      string  `yaml:"prompt"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
	Stream      bool    `yaml:"stream"`
	TestTools   bool    `yaml:"test_tools"`
}

// RateLimitConfig defines rate limiting settings
type RateLimitConfig struct {
	RequestsPerMinute  int `yaml:"requests_per_minute"`
	TokensPerMinute    int `yaml:"tokens_per_minute"`
	ConcurrentRequests int `yaml:"concurrent_requests"`
}

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxAttempts     int `yaml:"max_attempts"`
	InitialDelay    int `yaml:"initial_delay"`
	MaxDelay        int `yaml:"max_delay"`
	ExponentialBase int `yaml:"exponential_base"`
}

// HealthCheckConfig defines health check settings
type HealthCheckConfig struct {
	Enabled          bool `yaml:"enabled"`
	Interval         int  `yaml:"interval"`
	Timeout          int  `yaml:"timeout"`
	FailureThreshold int  `yaml:"failure_threshold"`
}

// ConfigManager handles configuration loading and token management
type ConfigManager struct {
	configPath string
	config     *Config
	tokenStore *TokenStore
	mu         sync.RWMutex
}

// TokenStore manages OAuth tokens separately
type TokenStore struct {
	Tokens map[string]*OAuthToken `json:"tokens"`
	mu     sync.RWMutex
	path   string
}

// OAuthToken represents stored OAuth tokens
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configPath string) (*ConfigManager, error) {
	cm := &ConfigManager{
		configPath: configPath,
	}

	// Load main configuration
	if err := cm.LoadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize token store
	tokenPath := cm.config.App.TokenFile
	if tokenPath == "" {
		tokenPath = ".tokens.json"
	}

	cm.tokenStore = &TokenStore{
		Tokens: make(map[string]*OAuthToken),
		path:   tokenPath,
	}

	// Load existing tokens if file exists
	if err := cm.tokenStore.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load tokens: %w", err)
	}

	// Merge stored tokens into config
	cm.mergeTokensIntoConfig()

	return cm, nil
}

// LoadConfig loads the configuration from the YAML file
func (cm *ConfigManager) LoadConfig() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Read the config file
	data, err := ioutil.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	data = cm.expandEnvVars(data)

	// Parse YAML
	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	cm.config = config
	return nil
}

// expandEnvVars replaces ${VAR} with environment variable values
func (cm *ConfigManager) expandEnvVars(data []byte) []byte {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	result := re.ReplaceAllFunc(data, func(match []byte) []byte {
		varName := string(match[2 : len(match)-1])
		value := os.Getenv(varName)
		if value == "" {
			// Keep the placeholder if env var is not set
			return match
		}
		return []byte(value)
	})
	return result
}

// mergeTokensIntoConfig updates the config with stored tokens
func (cm *ConfigManager) mergeTokensIntoConfig() {
	cm.tokenStore.mu.RLock()
	defer cm.tokenStore.mu.RUnlock()

	for provider, token := range cm.tokenStore.Tokens {
		if providerConfig, ok := cm.config.Providers[provider]; ok {
			providerConfig.AccessToken = token.AccessToken
			providerConfig.RefreshToken = token.RefreshToken
			providerConfig.TokenExpiry = token.ExpiresAt.Format(time.RFC3339)
			cm.config.Providers[provider] = providerConfig
		}
	}
}

// GetProviderConfig returns configuration for a specific provider
func (cm *ConfigManager) GetProviderConfig(providerName string) (*ProviderConfig, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	config, ok := cm.config.Providers[providerName]
	if !ok {
		return nil, fmt.Errorf("provider %s not found in config", providerName)
	}

	return &config, nil
}

// CreateTokenRefreshCallback creates a callback function for OAuth token updates
func (cm *ConfigManager) CreateTokenRefreshCallback(providerName string) func(string, string, time.Time) error {
	return func(accessToken, refreshToken string, expiresAt time.Time) error {
		fmt.Printf("[Token Update] Provider %s refreshed tokens\n", providerName)

		// Update token store
		if err := cm.tokenStore.UpdateToken(providerName, accessToken, refreshToken, expiresAt); err != nil {
			return fmt.Errorf("failed to update token store: %w", err)
		}

		// Update config in memory
		cm.mu.Lock()
		if providerConfig, ok := cm.config.Providers[providerName]; ok {
			providerConfig.AccessToken = accessToken
			providerConfig.RefreshToken = refreshToken
			providerConfig.TokenExpiry = expiresAt.Format(time.RFC3339)
			cm.config.Providers[providerName] = providerConfig
		}
		cm.mu.Unlock()

		// Save to disk
		if err := cm.SaveConfig(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("[Token Update] Successfully persisted new tokens for %s\n", providerName)
		return nil
	}
}

// SaveConfig saves the current configuration back to the YAML file
func (cm *ConfigManager) SaveConfig() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Read the original file to preserve formatting and comments
	originalData, err := ioutil.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to read original config: %w", err)
	}

	// For OAuth providers, update only the token fields
	lines := strings.Split(string(originalData), "\n")
	updatedLines := cm.updateTokenFields(lines)

	// Write back
	updatedData := strings.Join(updatedLines, "\n")
	if err := ioutil.WriteFile(cm.configPath, []byte(updatedData), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// updateTokenFields updates only token-related fields in the config
func (cm *ConfigManager) updateTokenFields(lines []string) []string {
	currentProvider := ""
	indentLevel := 0

	for i, line := range lines {
		// Detect provider sections
		if strings.Contains(line, ":") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			parts := strings.Split(line, ":")
			if len(parts) > 0 {
				potentialProvider := strings.TrimSpace(parts[0])
				if _, ok := cm.config.Providers[potentialProvider]; ok {
					currentProvider = potentialProvider
					// Calculate indent level for this provider
					indentLevel = len(line) - len(strings.TrimLeft(line, " "))
				}
			}
		}

		// Update token fields for current provider
		if currentProvider != "" {
			if config, ok := cm.config.Providers[currentProvider]; ok {
				if strings.Contains(line, "access_token:") {
					lines[i] = fmt.Sprintf("%saccess_token: \"%s\"", strings.Repeat(" ", indentLevel+2), config.AccessToken)
				} else if strings.Contains(line, "refresh_token:") {
					lines[i] = fmt.Sprintf("%srefresh_token: \"%s\"", strings.Repeat(" ", indentLevel+2), config.RefreshToken)
				} else if strings.Contains(line, "token_expiry:") {
					lines[i] = fmt.Sprintf("%stoken_expiry: \"%s\"", strings.Repeat(" ", indentLevel+2), config.TokenExpiry)
				}
			}
		}

		// Reset provider when we exit its section
		if currentProvider != "" && strings.TrimSpace(line) != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			if !strings.Contains(line, currentProvider) {
				currentProvider = ""
			}
		}
	}

	return lines
}

// Load reads tokens from disk
func (ts *TokenStore) Load() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	data, err := ioutil.ReadFile(ts.path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, ts)
}

// Save writes tokens to disk
func (ts *TokenStore) Save() error {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	data, err := json.MarshalIndent(ts, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(ts.path, data, 0600) // Secure permissions
}

// UpdateToken updates or adds a token for a provider
func (ts *TokenStore) UpdateToken(provider, accessToken, refreshToken string, expiresAt time.Time) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.Tokens[provider] = &OAuthToken{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		UpdatedAt:    time.Now(),
	}

	return ts.Save()
}

// GetToken retrieves a token for a provider
func (ts *TokenStore) GetToken(provider string) (*OAuthToken, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	token, ok := ts.Tokens[provider]
	return token, ok
}

// ConvertToProviderConfig converts our config to the library's types.ProviderConfig
func (pc *ProviderConfig) ConvertToProviderConfig(providerType types.ProviderType, tokenCallback func(string, string, time.Time) error) types.ProviderConfig {
	config := types.ProviderConfig{
		Type:         providerType,
		Name:         string(providerType),
		BaseURL:      pc.BaseURL,
		DefaultModel: pc.Model,
		Timeout:      time.Duration(pc.Timeout) * time.Second,
		MaxTokens:    pc.MaxTokens,
	}

	// Handle authentication based on type
	if pc.AuthType == "oauth" {
		// Parse token expiry
		var expiresAt time.Time
		if pc.TokenExpiry != "" {
			expiresAt, _ = time.Parse(time.RFC3339, pc.TokenExpiry)
		}

		// Use multi-OAuth format with single credential
		// Wrap the callback to match the multi-OAuth signature (id parameter)
		multiOAuthCallback := func(id, accessToken, refreshToken string, expiresAt time.Time) error {
			return tokenCallback(accessToken, refreshToken, expiresAt)
		}

		credSet := &types.OAuthCredentialSet{
			ID:             "default",
			ClientID:       pc.ClientID,
			ClientSecret:   pc.ClientSecret,
			AccessToken:    pc.AccessToken,
			RefreshToken:   pc.RefreshToken,
			ExpiresAt:      expiresAt,
			Scopes:         pc.Scopes,
			OnTokenRefresh: multiOAuthCallback,
		}
		config.OAuthCredentials = []*types.OAuthCredentialSet{credSet}

		// For Gemini, add project ID via ProviderConfig map
		if providerType == types.ProviderTypeGemini && pc.ProjectID != "" {
			if config.ProviderConfig == nil {
				config.ProviderConfig = make(map[string]interface{})
			}
			config.ProviderConfig["project_id"] = pc.ProjectID
		}
	} else if len(pc.APIKeys) > 0 {
		// Use first API key (library doesn't support multiple keys directly)
		config.APIKey = pc.APIKeys[0]
		// Store all keys in ProviderConfig for providers that support multi-key
		if config.ProviderConfig == nil {
			config.ProviderConfig = make(map[string]interface{})
		}
		config.ProviderConfig["api_keys"] = pc.APIKeys
	} else if pc.APIKey != "" {
		// Single API key
		config.APIKey = pc.APIKey
	}

	// Store temperature in ProviderConfig since it's not a direct field
	if pc.Temperature > 0 {
		if config.ProviderConfig == nil {
			config.ProviderConfig = make(map[string]interface{})
		}
		config.ProviderConfig["temperature"] = pc.Temperature
	}

	return config
}
