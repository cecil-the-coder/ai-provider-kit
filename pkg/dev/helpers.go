//go:build dev || debug

package dev

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ConfigHelper provides utilities for working with provider configurations
type ConfigHelper struct{}

// NewConfigHelper creates a new config helper
func NewConfigHelper() *ConfigHelper {
	return &ConfigHelper{}
}

// LoadConfigFromFile loads provider configuration from a JSON file
func (ch *ConfigHelper) LoadConfigFromFile(filename string) (map[string]types.ProviderConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var configs map[string]types.ProviderConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return configs, nil
}

// SaveConfigToFile saves provider configuration to a JSON file
func (ch *ConfigHelper) SaveConfigToFile(configs map[string]types.ProviderConfig, filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// CreateTestConfig creates a test configuration for a provider
func (ch *ConfigHelper) CreateTestConfig(providerType types.ProviderType, apiKey string) types.ProviderConfig {
	config := types.ProviderConfig{
		Type:         providerType,
		APIKey:       apiKey,
		BaseURL:      ch.getDefaultBaseURL(providerType),
		DefaultModel: ch.getDefaultModel(providerType),
		ProviderConfig: map[string]interface{}{
			"timeout":      "60s",
			"max_retries":  3,
			"enable_debug": true,
		},
	}

	// Set provider-specific defaults
	switch providerType {
	case types.ProviderTypeOpenAI:
		config.SupportsStreaming = true
		config.SupportsToolCalling = true
		config.SupportsResponsesAPI = false
	case types.ProviderTypeAnthropic:
		config.SupportsStreaming = true
		config.SupportsToolCalling = true
		config.SupportsResponsesAPI = false
	case types.ProviderTypeOpenRouter:
		config.SupportsStreaming = true
		config.SupportsToolCalling = false
		config.SupportsResponsesAPI = false
	case types.ProviderTypeCerebras:
		config.SupportsStreaming = true
		config.SupportsToolCalling = false
		config.SupportsResponsesAPI = false
	case types.ProviderTypeGemini:
		config.SupportsStreaming = true
		config.SupportsToolCalling = true
		config.SupportsResponsesAPI = false
	}

	return config
}

// getDefaultBaseURL returns the default base URL for a provider
func (ch *ConfigHelper) getDefaultBaseURL(providerType types.ProviderType) string {
	switch providerType {
	case types.ProviderTypeOpenAI:
		return "https://api.openai.com"
	case types.ProviderTypeAnthropic:
		return "https://api.anthropic.com"
	case types.ProviderTypeOpenRouter:
		return "https://openrouter.ai"
	case types.ProviderTypeCerebras:
		return "https://api.cerebras.ai"
	case types.ProviderTypeGemini:
		return "https://generativelanguage.googleapis.com"
	default:
		return ""
	}
}

// getDefaultModel returns the default model for a provider
func (ch *ConfigHelper) getDefaultModel(providerType types.ProviderType) string {
	switch providerType {
	case types.ProviderTypeOpenAI:
		return "gpt-3.5-turbo"
	case types.ProviderTypeAnthropic:
		return "claude-sonnet-4-5"
	case types.ProviderTypeOpenRouter:
		return "meta-llama/llama-3.1-70b-instruct:free"
	case types.ProviderTypeCerebras:
		return "llama3.1-70b"
	case types.ProviderTypeGemini:
		return "gemini-1.5-flash"
	default:
		return ""
	}
}

// ValidateConfig validates a provider configuration
func (ch *ConfigHelper) ValidateConfig(config types.ProviderConfig) error {
	if config.Type == "" {
		return fmt.Errorf("provider type is required")
	}

	if config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	// Provider-specific validation
	switch config.Type {
	case types.ProviderTypeOpenAI, types.ProviderTypeOpenRouter, types.ProviderTypeCerebras:
		if !strings.HasPrefix(config.APIKey, "sk-") && !strings.HasPrefix(config.APIKey, "sk-or-") {
			return fmt.Errorf("invalid API key format for %s", config.Type)
		}
	case types.ProviderTypeAnthropic:
		if len(config.APIKey) < 20 {
			return fmt.Errorf("API key appears too short for Anthropic")
		}
	case types.ProviderTypeGemini:
		if len(config.APIKey) < 30 {
			return fmt.Errorf("API key appears too short for Gemini")
		}
	}

	return nil
}

// PromptHelper provides utilities for working with prompts
type PromptHelper struct{}

// NewPromptHelper creates a new prompt helper
func NewPromptHelper() *PromptHelper {
	return &PromptHelper{}
}

// BuildCodeGenerationPrompt builds a code generation prompt
func (ph *PromptHelper) BuildCodeGenerationPrompt(prompt, language string, contextFiles []string, existingContent string) string {
	var parts []string

	// Add context files
	if len(contextFiles) > 0 {
		parts = append(parts, "Context Files:")
		for _, file := range contextFiles {
			if content, err := ph.readFileContent(file); err == nil {
				fileLanguage := ph.detectLanguage(file)
				parts = append(parts, fmt.Sprintf("\nFile: %s\n```%s\n%s\n```", file, fileLanguage, content))
			}
		}
	}

	// Add existing content
	if existingContent != "" {
		parts = append(parts, fmt.Sprintf("Existing file content:\n```%s\n%s\n```", language, existingContent))
	}

	// Add the main prompt
	systemPrompt := fmt.Sprintf("You are an expert programmer. Generate clean, functional code in %s with no explanations or markdown formatting. Include necessary imports and ensure the code is ready to run.", language)
	parts = append(parts, systemPrompt)
	parts = append(parts, fmt.Sprintf("Generate %s code for: %s", language, prompt))

	return strings.Join(parts, "\n\n")
}

// BuildChatPrompt builds a chat completion prompt
func (ph *PromptHelper) BuildChatPrompt(messages []types.ChatMessage, systemPrompt string) string {
	var parts []string

	if systemPrompt != "" {
		parts = append(parts, fmt.Sprintf("System: %s", systemPrompt))
	}

	for _, msg := range messages {
		parts = append(parts, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
	}

	return strings.Join(parts, "\n\n")
}

// readFileContent reads file content (simplified version)
func (ph *PromptHelper) readFileContent(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// detectLanguage detects programming language from file extension
func (ph *PromptHelper) detectLanguage(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".java":
		return "java"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".c":
		return "c"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".sh", ".bash":
		return "bash"
	case ".html":
		return "html"
	case ".css":
		return "css"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".sql":
		return "sql"
	default:
		return "text"
	}
}

// ScenarioHelper provides test scenarios for providers
type ScenarioHelper struct{}

// NewScenarioHelper creates a new scenario helper
func NewScenarioHelper() *ScenarioHelper {
	return &ScenarioHelper{}
}

// CreateBasicScenario creates a basic test scenario
func (sh *ScenarioHelper) CreateBasicScenario() TestScenario {
	return TestScenario{
		Name:        "Basic Test",
		Description: "Basic functionality test",
		Steps: []TestStep{
			{
				Name:        "Initialize Provider",
				Description: "Initialize the provider with test config",
				Action:      "initialize",
				Expected:    "provider initialized successfully",
			},
			{
				Name:        "Generate Code",
				Description: "Generate a simple hello world function",
				Action:      "generate",
				Parameters: map[string]interface{}{
					"prompt":   "Create a hello world function",
					"language": "javascript",
				},
				Expected: "generated code contains function definition",
			},
		},
	}
}

// CreateLoadTestScenario creates a load test scenario
func (sh *ScenarioHelper) CreateLoadTestScenario() TestScenario {
	return TestScenario{
		Name:        "Load Test",
		Description: "Test provider under load",
		Steps: []TestStep{
			{
				Name:        "Initialize Provider",
				Description: "Initialize the provider",
				Action:      "initialize",
			},
			{
				Name:        "Concurrent Requests",
				Description: "Send multiple concurrent requests",
				Action:      "concurrent_generate",
				Parameters: map[string]interface{}{
					"concurrent_requests": 10,
					"prompt":              "Generate a simple function",
					"language":            "python",
				},
				Expected: "all requests completed successfully",
			},
		},
	}
}

// TestScenario represents a test scenario
type TestScenario struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Steps       []TestStep `json:"steps"`
}

// TestStep represents a step in a test scenario
type TestStep struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Action      string                 `json:"action"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Expected    string                 `json:"expected"`
	Timeout     time.Duration          `json:"timeout,omitempty"`
}

// LogHelper provides logging utilities for development
type LogHelper struct {
	LogFile *os.File
}

// NewLogHelper creates a new log helper
func NewLogHelper(logFile string) (*LogHelper, error) {
	if logFile == "" {
		return &LogHelper{}, nil
	}

	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &LogHelper{LogFile: file}, nil
}

// Log logs a message with timestamp
func (lh *LogHelper) Log(level, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] [%s] %s\n", timestamp, strings.ToUpper(level), message)

	if lh.LogFile != nil {
		lh.LogFile.WriteString(logEntry)
	}
	fmt.Print(logEntry)
}

// LogRequest logs an HTTP request
func (lh *LogHelper) LogRequest(method, url string, headers map[string]string, body interface{}) {
	lh.Log("INFO", fmt.Sprintf("REQUEST: %s %s", method, url))
	if len(headers) > 0 {
		for key, value := range headers {
			lh.Log("DEBUG", fmt.Sprintf("  Header: %s: %s", key, value))
		}
	}
	if body != nil {
		if jsonBody, err := json.Marshal(body); err == nil {
			lh.Log("DEBUG", fmt.Sprintf("  Body: %s", string(jsonBody)))
		}
	}
}

// LogResponse logs an HTTP response
func (lh *LogHelper) LogResponse(statusCode int, duration time.Duration, body interface{}) {
	lh.Log("INFO", fmt.Sprintf("RESPONSE: %d (%v)", statusCode, duration))
	if body != nil {
		if jsonBody, err := json.Marshal(body); err == nil {
			lh.Log("DEBUG", fmt.Sprintf("  Body: %s", string(jsonBody)))
		}
	}
}

// Close closes the log helper
func (lh *LogHelper) Close() error {
	if lh.LogFile != nil {
		return lh.LogFile.Close()
	}
	return nil
}

// EnvironmentHelper provides utilities for environment-specific testing
type EnvironmentHelper struct{}

// NewEnvironmentHelper creates a new environment helper
func NewEnvironmentHelper() *EnvironmentHelper {
	return &EnvironmentHelper{}
}

// GetTestAPIKey gets an API key from environment variables
func (eh *EnvironmentHelper) GetTestAPIKey(providerType types.ProviderType) (string, error) {
	envVar := eh.getAPIKeyEnvVar(providerType)
	apiKey := os.Getenv(envVar)
	if apiKey == "" {
		return "", fmt.Errorf("environment variable %s is not set", envVar)
	}
	return apiKey, nil
}

// getAPIKeyEnvVar returns the environment variable name for a provider's API key
func (eh *EnvironmentHelper) getAPIKeyEnvVar(providerType types.ProviderType) string {
	switch providerType {
	case types.ProviderTypeOpenAI:
		return "OPENAI_API_KEY"
	case types.ProviderTypeAnthropic:
		return "ANTHROPIC_API_KEY"
	case types.ProviderTypeOpenRouter:
		return "OPENROUTER_API_KEY"
	case types.ProviderTypeCerebras:
		return "CEREBRAS_API_KEY"
	case types.ProviderTypeGemini:
		return "GEMINI_API_KEY"
	default:
		return fmt.Sprintf("%s_API_KEY", strings.ToUpper(string(providerType)))
	}
}

// IsCI checks if running in CI environment
func (eh *EnvironmentHelper) IsCI() bool {
	return os.Getenv("CI") != "" || os.Getenv("CONTINUOUS_INTEGRATION") != ""
}

// IsTestEnvironment checks if running in test environment
func (eh *EnvironmentHelper) IsTestEnvironment() bool {
	return os.Getenv("GO_ENV") == "test" || strings.HasSuffix(os.Args[0], ".test")
}

// CreateTempDir creates a temporary directory for testing
func (eh *EnvironmentHelper) CreateTempDir() (string, error) {
	return os.MkdirTemp("", "ai-provider-test-*")
}

// CleanupTempDir cleans up a temporary directory
func (eh *EnvironmentHelper) CleanupTempDir(dir string) error {
	return os.RemoveAll(dir)
}

// CreateTestFile creates a test file with content
func (eh *EnvironmentHelper) CreateTestFile(dir, filename, content string) (string, error) {
	filePath := filepath.Join(dir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return "", err
	}
	return filePath, nil
}

// ReadTestFile reads a test file
func (eh *EnvironmentHelper) ReadTestFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// StreamHelper provides utilities for testing streaming responses
type StreamHelper struct{}

// NewStreamHelper creates a new stream helper
func NewStreamHelper() *StreamHelper {
	return &StreamHelper{}
}

// CollectStream reads all chunks from a stream
func (sh *StreamHelper) CollectStream(stream types.ChatCompletionStream) ([]types.ChatCompletionChunk, error) {
	var chunks []types.ChatCompletionChunk

	for {
		chunk, err := stream.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		chunks = append(chunks, chunk)

		if chunk.Done {
			break
		}
	}

	return chunks, nil
}

// CombineStreamContent combines content from all chunks
func (sh *StreamHelper) CombineStreamContent(chunks []types.ChatCompletionChunk) string {
	var content strings.Builder

	for _, chunk := range chunks {
		content.WriteString(chunk.Content)
	}

	return content.String()
}

// ValidateStream validates a streaming response
func (sh *StreamHelper) ValidateStream(chunks []types.ChatCompletionChunk) error {
	if len(chunks) == 0 {
		return fmt.Errorf("no chunks received")
	}

	// Check if the last chunk is marked as done
	lastChunk := chunks[len(chunks)-1]
	if !lastChunk.Done {
		return fmt.Errorf("stream not properly terminated")
	}

	return nil
}
