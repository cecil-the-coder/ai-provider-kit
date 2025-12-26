// Package gemini provides type definitions for Google Gemini AI provider.
// It includes request/response structures, streaming types, and function calling definitions.
package gemini

import (
	"fmt"
	"os"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// BackendType represents the type of backend being used (Gemini API, Vertex AI, or Code Assist)
type BackendType string

const (
	// BackendGeminiAPI uses the standard Gemini API (generativelanguage.googleapis.com)
	BackendGeminiAPI BackendType = "gemini-api"
	// BackendVertexAI uses the Vertex AI API (PROJECT-LOCATION.aiplatform.googleapis.com)
	BackendVertexAI BackendType = "vertex-ai"
	// BackendCodeAssist uses the Code Assist API (cloudcode-pa.googleapis.com) with OAuth
	BackendCodeAssist BackendType = "code-assist"
)

// AuthMethod defines how API keys are sent to the Gemini API
type AuthMethod string

const (
	// AuthMethodURLParam uses the URL query parameter ?key= (default, backward compatible)
	AuthMethodURLParam AuthMethod = "url_param"
	// AuthMethodHeader uses the X-Goog-Api-Key header
	AuthMethodHeader AuthMethod = "header"
)

// ClientConfig represents configuration for the Gemini client with backend selection
type ClientConfig struct {
	// Backend type - either BackendGeminiAPI or BackendVertexAI
	Backend BackendType `json:"backend,omitempty"`

	// For Vertex AI backend - GCP project ID
	Project string `json:"project,omitempty"`

	// For Vertex AI backend - GCP region (e.g., "us-central1", "europe-west4")
	Location string `json:"location,omitempty"`

	// API Key for Gemini API backend
	APIKey string `json:"api_key,omitempty"`

	// Service account JSON for Vertex AI backend
	ServiceAccountJSON string `json:"service_account_json,omitempty"`

	// Base URL override (optional)
	BaseURL string `json:"base_url,omitempty"`

	// AuthMethod defines how API keys are sent (url_param or header)
	// Defaults to url_param for backward compatibility
	AuthMethod AuthMethod `json:"auth_method,omitempty"`

	// ProjectID is the Google Cloud project ID for Code Assist API
	// Only used when backend is BackendCodeAssist
	ProjectID string `json:"project_id,omitempty"`
}

// Validate checks if the ClientConfig is valid for the selected backend
func (c *ClientConfig) Validate() error {
	switch c.Backend {
	case BackendGeminiAPI:
		if c.APIKey == "" {
			return types.NewInvalidRequestError(types.ProviderTypeGemini, "API key is required for Gemini API backend").
				WithOperation("validate_options")
		}
	case BackendVertexAI:
		if c.Project == "" {
			return types.NewInvalidRequestError(types.ProviderTypeGemini, "project ID is required for Vertex AI backend").
				WithOperation("validate_options")
		}
		if c.Location == "" {
			return types.NewInvalidRequestError(types.ProviderTypeGemini, "location is required for Vertex AI backend").
				WithOperation("validate_options")
		}
	default:
		return types.NewInvalidRequestError(types.ProviderTypeGemini, fmt.Sprintf("unknown backend: %v", c.Backend)).
			WithOperation("validate_options")
	}
	return nil
}

// GetBaseURL returns the appropriate base URL for the backend
func (c *ClientConfig) GetBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}

	switch c.Backend {
	case BackendGeminiAPI:
		return "https://generativelanguage.googleapis.com/v1beta"
	case BackendVertexAI:
		return fmt.Sprintf("https://%s-aiplatform.googleapis.com", c.Location)
	case BackendCodeAssist:
		return "https://cloudcode-pa.googleapis.com/v1internal"
	default:
		return ""
	}
}

// GetEndpoint returns the full endpoint URL for a given model
func (c *ClientConfig) GetEndpoint(model string, streaming bool) string {
	baseURL := c.GetBaseURL()

	switch c.Backend {
	case BackendGeminiAPI:
		if streaming {
			return fmt.Sprintf("%s/models/%s:streamGenerateContent", baseURL, model)
		}
		return fmt.Sprintf("%s/models/%s:generateContent", baseURL, model)
	case BackendVertexAI:
		if streaming {
			return fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/google/models/%s:streamGenerateContent",
				baseURL, c.Project, c.Location, model)
		}
		return fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
			baseURL, c.Project, c.Location, model)
	case BackendCodeAssist:
		// Code Assist API uses same endpoint format as Gemini API
		if streaming {
			return fmt.Sprintf("%s/models/%s:streamGenerateContent", baseURL, model)
		}
		return fmt.Sprintf("%s/models/%s:generateContent", baseURL, model)
	default:
		return ""
	}
}

// DetectBackendFromEnv detects the backend from environment variables
func DetectBackendFromEnv() BackendType {
	// Check GOOGLE_GENAI_USE_VERTEXAI environment variable
	useVertexAI := os.Getenv("GOOGLE_GENAI_USE_VERTEXAI")
	if useVertexAI == "true" || useVertexAI == "1" {
		return BackendVertexAI
	}
	return BackendGeminiAPI
}

// NewClientConfigFromEnv creates a ClientConfig from environment variables
func NewClientConfigFromEnv() (*ClientConfig, error) {
	backend := DetectBackendFromEnv()

	config := &ClientConfig{
		Backend: backend,
	}

	switch backend {
	case BackendGeminiAPI:
		config.APIKey = os.Getenv("GOOGLE_API_KEY")
		if config.APIKey == "" {
			config.APIKey = os.Getenv("GEMINI_API_KEY")
		}
	case BackendVertexAI:
		config.Project = os.Getenv("GOOGLE_CLOUD_PROJECT")
		if config.Project == "" {
			config.Project = os.Getenv("GCP_PROJECT")
		}
		config.Location = os.Getenv("GOOGLE_CLOUD_LOCATION")
		if config.Location == "" {
			config.Location = os.Getenv("GCP_LOCATION")
		}
		// Default location if not specified
		if config.Location == "" {
			config.Location = "us-central1"
		}
	}

	if err := config.Validate(); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "invalid config from environment").
			WithOperation("validate_options").
			WithOriginalErr(err)
	}

	return config, nil
}

// Request/Response types for Gemini API

// GenerateContentRequest represents a request to generate content
type GenerateContentRequest struct {
	Contents         []Content         `json:"contents"`
	GenerationConfig *GenerationConfig `json:"generationConfig,omitempty"`
	Tools            []GeminiTool      `json:"tools,omitempty"`
	SafetySettings   []SafetySetting   `json:"safetySettings,omitempty"`
}

// Content represents message content
type Content struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

// Part represents a part of content
type Part struct {
	Text             string              `json:"text,omitempty"`
	InlineData       *InlineData         `json:"inlineData,omitempty"`
	FileData         *FileData           `json:"fileData,omitempty"`
	FunctionCall     *GeminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *FunctionResponse   `json:"functionResponse,omitempty"`
}

// InlineData represents inline media data (base64)
type InlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// FileData represents file data (URL/GCS URI)
type FileData struct {
	MimeType string `json:"mimeType"`
	FileURI  string `json:"fileUri"`
}

// FunctionResponse represents a response to a function call
type FunctionResponse struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

// GenerationConfig represents generation configuration
type GenerationConfig struct {
	Temperature      float64                `json:"temperature,omitempty"`
	TopP             float64                `json:"topP,omitempty"`
	TopK             int                    `json:"topK,omitempty"`
	MaxOutputTokens  int                    `json:"maxOutputTokens,omitempty"`
	ResponseMimeType string                 `json:"responseMimeType,omitempty"` // For structured outputs
	ResponseSchema   map[string]interface{} `json:"responseSchema,omitempty"`   // For structured outputs JSON schema
}

// GenerateContentResponse represents a response from generate content
type GenerateContentResponse struct {
	Candidates    []Candidate    `json:"candidates"`
	UsageMetadata *UsageMetadata `json:"usageMetadata,omitempty"`
}

// Candidate represents a response candidate
type Candidate struct {
	Content       Content        `json:"content"`
	FinishReason  string         `json:"finishReason,omitempty"`
	SafetyRatings []SafetyRating `json:"safetyRatings,omitempty"`
}

// UsageMetadata represents usage metadata
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// Safety Settings Types

// HarmCategory represents a harm category for safety filtering
type HarmCategory string

const (
	// HarmCategoryUnspecified is an unspecified harm category
	HarmCategoryUnspecified HarmCategory = "HARM_CATEGORY_UNSPECIFIED"
	// HarmCategoryHarassment includes negative or harmful comments targeting identity and/or protected attributes
	HarmCategoryHarassment HarmCategory = "HARM_CATEGORY_HARASSMENT"
	// HarmCategoryHateSpeech includes content that is rude, disrespectful, or profane
	HarmCategoryHateSpeech HarmCategory = "HARM_CATEGORY_HATE_SPEECH"
	// HarmCategorySexuallyExplicit includes references to sexual acts or other lewd content
	HarmCategorySexuallyExplicit HarmCategory = "HARM_CATEGORY_SEXUALLY_EXPLICIT"
	// HarmCategoryDangerousContent promotes, facilitates, or encourages harmful acts
	HarmCategoryDangerousContent HarmCategory = "HARM_CATEGORY_DANGEROUS_CONTENT"
	// HarmCategoryCivicIntegrity includes election-related queries (PaLM 2 legacy)
	HarmCategoryCivicIntegrity HarmCategory = "HARM_CATEGORY_CIVIC_INTEGRITY"
	// HarmCategoryDerogatory includes negative or harmful comments targeting identity and/or protected attributes (legacy)
	HarmCategoryDerogatory HarmCategory = "HARM_CATEGORY_DEROGATORY"
	// HarmCategoryToxicity includes content that is rude, disrespectful, or profane (legacy)
	HarmCategoryToxicity HarmCategory = "HARM_CATEGORY_TOXICITY"
	// HarmCategoryViolence includes describes scenarios depicting violence or bloody/gory content (legacy)
	HarmCategoryViolence HarmCategory = "HARM_CATEGORY_VIOLENCE"
	// HarmCategorySexual includes references to sexual acts or other lewd content (legacy)
	HarmCategorySexual HarmCategory = "HARM_CATEGORY_SEXUAL"
	// HarmCategoryMedical includes content promoting medical advice or procedures (legacy)
	HarmCategoryMedical HarmCategory = "HARM_CATEGORY_MEDICAL"
	// HarmCategoryDangerous promotes or enables access to harmful goods, services, and activities (legacy)
	HarmCategoryDangerous HarmCategory = "HARM_CATEGORY_DANGEROUS"
)

// HarmBlockThreshold represents the threshold for blocking content
type HarmBlockThreshold string

const (
	// HarmBlockThresholdUnspecified uses the default threshold
	HarmBlockThresholdUnspecified HarmBlockThreshold = "HARM_BLOCK_THRESHOLD_UNSPECIFIED"
	// HarmBlockThresholdBlockLowAndAbove blocks when probability or severity is LOW, MEDIUM, or HIGH
	HarmBlockThresholdBlockLowAndAbove HarmBlockThreshold = "BLOCK_LOW_AND_ABOVE"
	// HarmBlockThresholdBlockMediumAndAbove blocks when probability or severity is MEDIUM or HIGH
	HarmBlockThresholdBlockMediumAndAbove HarmBlockThreshold = "BLOCK_MEDIUM_AND_ABOVE"
	// HarmBlockThresholdBlockOnlyHigh blocks when probability or severity is HIGH
	HarmBlockThresholdBlockOnlyHigh HarmBlockThreshold = "BLOCK_ONLY_HIGH"
	// HarmBlockThresholdBlockNone allows all content regardless of probability (returns safety scores)
	HarmBlockThresholdBlockNone HarmBlockThreshold = "BLOCK_NONE"
	// HarmBlockThresholdOff disables automated response blocking and no metadata is returned
	HarmBlockThresholdOff HarmBlockThreshold = "OFF"
)

// HarmProbability represents the probability that content is harmful
type HarmProbability string

const (
	// HarmProbabilityUnspecified means probability is unspecified
	HarmProbabilityUnspecified HarmProbability = "HARM_PROBABILITY_UNSPECIFIED"
	// HarmProbabilityNegligible means content has negligible probability of being unsafe
	HarmProbabilityNegligible HarmProbability = "NEGLIGIBLE"
	// HarmProbabilityLow means content has low probability of being unsafe
	HarmProbabilityLow HarmProbability = "LOW"
	// HarmProbabilityMedium means content has medium probability of being unsafe
	HarmProbabilityMedium HarmProbability = "MEDIUM"
	// HarmProbabilityHigh means content has high probability of being unsafe
	HarmProbabilityHigh HarmProbability = "HIGH"
)

// HarmSeverity represents the severity of harm in content
type HarmSeverity string

const (
	// HarmSeverityUnspecified means severity is unspecified
	HarmSeverityUnspecified HarmSeverity = "HARM_SEVERITY_UNSPECIFIED"
	// HarmSeverityNegligible means content has negligible severity
	HarmSeverityNegligible HarmSeverity = "HARM_SEVERITY_NEGLIGIBLE"
	// HarmSeverityLow means content has low severity
	HarmSeverityLow HarmSeverity = "HARM_SEVERITY_LOW"
	// HarmSeverityMedium means content has medium severity
	HarmSeverityMedium HarmSeverity = "HARM_SEVERITY_MEDIUM"
	// HarmSeverityHigh means content has high severity
	HarmSeverityHigh HarmSeverity = "HARM_SEVERITY_HIGH"
)

// SafetySetting represents a safety setting for a specific harm category
type SafetySetting struct {
	Category  HarmCategory       `json:"category"`
	Threshold HarmBlockThreshold `json:"threshold"`
}

// SafetyRating represents the safety rating for a specific harm category in a response
type SafetyRating struct {
	Category         HarmCategory    `json:"category"`
	Probability      HarmProbability `json:"probability"`
	Severity         HarmSeverity    `json:"severity,omitempty"`
	ProbabilityScore float64         `json:"probabilityScore,omitempty"`
	SeverityScore    float64         `json:"severityScore,omitempty"`
	Blocked          bool            `json:"blocked,omitempty"`
}

// Finish Reason Constants for Gemini API responses
// These indicate why token generation stopped
const (
	// FinishReasonUnspecified is the default value (unused)
	FinishReasonUnspecified = "FINISH_REASON_UNSPECIFIED"

	// FinishReasonStop indicates natural stop point or provided stop sequence
	FinishReasonStop = "STOP"

	// FinishReasonMaxTokens indicates the maximum number of tokens was reached
	FinishReasonMaxTokens = "MAX_TOKENS"

	// FinishReasonSafety indicates content was flagged for safety reasons
	FinishReasonSafety = "SAFETY"

	// FinishReasonRecitation indicates content was flagged for recitation reasons
	FinishReasonRecitation = "RECITATION"

	// FinishReasonLanguage indicates content was flagged for using an unsupported language
	FinishReasonLanguage = "LANGUAGE"

	// FinishReasonOther indicates unknown reason
	FinishReasonOther = "OTHER"

	// FinishReasonBlocklist indicates content contains forbidden terms
	FinishReasonBlocklist = "BLOCKLIST"

	// FinishReasonProhibitedContent indicates potentially prohibited content
	FinishReasonProhibitedContent = "PROHIBITED_CONTENT"

	// FinishReasonSPII indicates content potentially contains Sensitive Personally Identifiable Information
	FinishReasonSPII = "SPII"

	// FinishReasonSpii is an alias for FinishReasonSPII for backwards compatibility
	FinishReasonSpii = FinishReasonSPII

	// FinishReasonMalformedFunctionCall indicates the function call generated by the model is invalid
	FinishReasonMalformedFunctionCall = "MALFORMED_FUNCTION_CALL"

	// FinishReasonImageSafety indicates generated images contain safety violations
	FinishReasonImageSafety = "IMAGE_SAFETY"

	// FinishReasonUnexpectedToolCall indicates model generated a tool call but no tools were enabled
	FinishReasonUnexpectedToolCall = "UNEXPECTED_TOOL_CALL"

	// FinishReasonTooManyToolCalls indicates model called too many tools consecutively
	FinishReasonTooManyToolCalls = "TOO_MANY_TOOL_CALLS"

	// FinishReasonImageProhibitedContent indicates generated images have prohibited content
	FinishReasonImageProhibitedContent = "IMAGE_PROHIBITED_CONTENT"

	// FinishReasonImageOther indicates image generation stopped for other miscellaneous issue
	FinishReasonImageOther = "IMAGE_OTHER"

	// FinishReasonNoImage indicates model was expected to generate an image but none was generated
	FinishReasonNoImage = "NO_IMAGE"
)

// CloudCode API types

// CloudCodeRequestWrapper wraps requests for CloudCode API
type CloudCodeRequestWrapper struct {
	Model        string                 `json:"model"`
	Project      string                 `json:"project,omitempty"`
	UserPromptID string                 `json:"user_prompt_id,omitempty"`
	Request      GenerateContentRequest `json:"request"`
}

// CloudCodeResponseWrapper wraps responses from CloudCode API
type CloudCodeResponseWrapper struct {
	Response GenerateContentResponse `json:"response"`
}

// CodeAssistRequest is the wrapped format for Code Assist API
type CodeAssistRequest struct {
	Model        string                 `json:"model"`
	Project      string                 `json:"project"`
	UserPromptID string                 `json:"user_prompt_id"`
	Request      map[string]interface{} `json:"request"`
}

// CodeAssistRequestMetadata wraps the request metadata for Code Assist API
type CodeAssistRequestMetadata struct {
	Model        string `json:"model"`
	Project      string `json:"project"`
	UserPromptID string `json:"user_prompt_id"`
}

// Onboarding types for CloudCode API

// ClientMetadata represents client metadata for onboarding
type ClientMetadata struct {
	IDEType       string `json:"ideType,omitempty"`
	IDEVersion    string `json:"ideVersion,omitempty"`
	PluginVersion string `json:"pluginVersion,omitempty"`
	Platform      string `json:"platform,omitempty"`
	UpdateChannel string `json:"updateChannel,omitempty"`
	DuetProject   string `json:"duetProject,omitempty"`
	PluginType    string `json:"pluginType,omitempty"`
	IDEName       string `json:"ideName,omitempty"`
}

// LoadCodeAssistRequest represents a request to load code assist
type LoadCodeAssistRequest struct {
	CloudaicompanionProject *string        `json:"cloudaicompanionProject,omitempty"`
	Metadata                ClientMetadata `json:"metadata"`
}

// GeminiUserTier represents a user tier for Gemini
type GeminiUserTier struct {
	ID                                 string         `json:"id"`
	Name                               string         `json:"name"`
	Description                        string         `json:"description"`
	UserDefinedCloudaicompanionProject *bool          `json:"userDefinedCloudaicompanionProject,omitempty"`
	IsDefault                          *bool          `json:"isDefault,omitempty"`
	PrivacyNotice                      *PrivacyNotice `json:"privacyNotice,omitempty"`
	HasAcceptedTos                     *bool          `json:"hasAcceptedTos,omitempty"`
	HasOnboardedPreviously             *bool          `json:"hasOnboardedPreviously,omitempty"`
}

// LoadCodeAssistResponse represents a response from load code assist
type LoadCodeAssistResponse struct {
	CurrentTier             *GeminiUserTier  `json:"currentTier,omitempty"`
	AllowedTiers            []GeminiUserTier `json:"allowedTiers,omitempty"`
	IneligibleTiers         []IneligibleTier `json:"ineligibleTiers,omitempty"`
	CloudaicompanionProject *string          `json:"cloudaicompanionProject,omitempty"`
}

// PrivacyNotice represents a privacy notice
type PrivacyNotice struct {
	ShowNotice bool    `json:"showNotice"`
	NoticeText *string `json:"noticeText,omitempty"`
}

// IneligibleTier represents an ineligible tier
type IneligibleTier struct {
	ReasonCode    string `json:"reasonCode"`
	ReasonMessage string `json:"reasonMessage"`
	TierID        string `json:"tierId"`
	TierName      string `json:"tierName"`
}

// OnboardUserRequest represents a request to onboard a user
type OnboardUserRequest struct {
	TierID                  *string         `json:"tierId,omitempty"`
	CloudaicompanionProject *string         `json:"cloudaicompanionProject,omitempty"`
	Metadata                *ClientMetadata `json:"metadata,omitempty"`
}

// OnboardUserResponse represents a response from user onboarding
type OnboardUserResponse struct {
	CloudaicompanionProject *CloudaicompanionProject `json:"cloudaicompanionProject,omitempty"`
}

// LongRunningOperationResponse represents a long-running operation response
type LongRunningOperationResponse struct {
	Name     string               `json:"name"`
	Done     bool                 `json:"done"`
	Response *OnboardUserResponse `json:"response,omitempty"`
}

// CloudaicompanionProject represents a Cloud AI companion project
type CloudaicompanionProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Constants for user tiers
const (
	UserTierIDFree     = "free-tier"
	UserTierIDLegacy   = "legacy-tier"
	UserTierIDStandard = "standard-tier"
)

// Quota API routes for Code Assist
const (
	getQuotaRoute        = ":getQuota"
	getQuotaUsageRoute   = ":getQuotaUsage"
	getQuotaHistoryRoute = ":getQuotaHistory"
)

// Constants for client metadata
const (
	IDETypeUnspecified  = "IDE_UNSPECIFIED"
	PlatformUnspecified = "PLATFORM_UNSPECIFIED"
	PluginTypeGemini    = "GEMINI"
)

// ProjectIDRequiredError is returned when a project ID is needed but not provided
type ProjectIDRequiredError struct{}

// Error returns the error message
func (e *ProjectIDRequiredError) Error() string {
	return "This account requires setting the GOOGLE_CLOUD_PROJECT env var. Please set GOOGLE_CLOUD_PROJECT before calling setup."
}

// IsProjectIDRequired checks if an error is a ProjectIDRequiredError
func IsProjectIDRequired(err error) bool {
	_, ok := err.(*ProjectIDRequiredError)
	return ok
}

// Helper function to create boolean pointers
func boolPtr(b bool) *bool { return &b }

// GeminiStreamResponse represents a streaming response chunk
type GeminiStreamResponse struct {
	Candidates    []Candidate    `json:"candidates,omitempty"`
	UsageMetadata *UsageMetadata `json:"usageMetadata,omitempty"`
}

// GeminiModelsResponse represents the response from /v1beta/models endpoint
type GeminiModelsResponse struct {
	Models        []GeminiModelData `json:"models"`
	NextPageToken string            `json:"nextPageToken,omitempty"`
}

// GeminiModelData represents a model in the models list
type GeminiModelData struct {
	Name                       string   `json:"name"`
	BaseModelID                string   `json:"baseModelId"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	InputTokenLimit            int      `json:"inputTokenLimit"`
	OutputTokenLimit           int      `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

// Gemini Function Calling Types (Google's specific format)

// GeminiTool represents a tool available to the model
type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"function_declarations"`
}

// GeminiFunctionDeclaration represents a function declaration in Gemini format
type GeminiFunctionDeclaration struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Parameters  GeminiSchema `json:"parameters"`
}

// GeminiSchema represents the schema for function parameters
type GeminiSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]GeminiProperty `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}

// GeminiProperty represents a property in the schema
type GeminiProperty struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// GeminiFunctionCall represents a function call from the model
type GeminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// GetFinishReasonMessage returns a human-readable error message for a finish reason
func GetFinishReasonMessage(finishReason string) string {
	switch finishReason {
	case FinishReasonStop:
		return "response completed successfully"
	case FinishReasonMaxTokens:
		return "maximum token limit reached"
	case FinishReasonSafety:
		return "content was flagged for safety concerns"
	case FinishReasonRecitation:
		return "content was flagged for potential recitation of copyrighted material"
	case FinishReasonLanguage:
		return "content was flagged for using an unsupported language"
	case FinishReasonBlocklist:
		return "content contains forbidden terms from blocklist"
	case FinishReasonProhibitedContent:
		return "content was blocked for potentially containing prohibited content"
	case FinishReasonSPII:
		return "content was blocked for potentially containing Sensitive Personally Identifiable Information (SPII)"
	case FinishReasonMalformedFunctionCall:
		return "function call generated by the model is invalid (consider using function calling mode ANY for constrained decoding)"
	case FinishReasonImageSafety:
		return "generated images contain safety violations"
	case FinishReasonUnexpectedToolCall:
		return "model generated a tool call but no tools were enabled in the request"
	case FinishReasonTooManyToolCalls:
		return "model called too many tools consecutively"
	case FinishReasonImageProhibitedContent:
		return "generated images contain prohibited content"
	case FinishReasonImageOther:
		return "image generation stopped due to miscellaneous issue"
	case FinishReasonNoImage:
		return "model was expected to generate an image but none was generated"
	case FinishReasonOther:
		return "generation stopped for unknown reason"
	case FinishReasonUnspecified, "":
		return "finish reason unspecified"
	default:
		return "generation stopped with reason: " + finishReason
	}
}

// IsErrorFinishReason returns true if the finish reason indicates an error condition
func IsErrorFinishReason(finishReason string) bool {
	switch finishReason {
	case string(FinishReasonSafety),
		string(FinishReasonRecitation),
		string(FinishReasonLanguage),
		string(FinishReasonBlocklist),
		string(FinishReasonProhibitedContent),
		string(FinishReasonSpii),
		string(FinishReasonMalformedFunctionCall),
		string(FinishReasonImageSafety),
		string(FinishReasonUnexpectedToolCall),
		string(FinishReasonTooManyToolCalls),
		string(FinishReasonImageProhibitedContent),
		string(FinishReasonImageOther),
		string(FinishReasonNoImage):
		return true
	default:
		return false
	}
}

// Safety Settings Helper Functions

// IsValidHarmCategory checks if a harm category is valid
func IsValidHarmCategory(category HarmCategory) bool {
	switch category {
	case HarmCategoryUnspecified,
		HarmCategoryHarassment,
		HarmCategoryHateSpeech,
		HarmCategorySexuallyExplicit,
		HarmCategoryDangerousContent,
		HarmCategoryCivicIntegrity,
		HarmCategoryDerogatory,
		HarmCategoryToxicity,
		HarmCategoryViolence,
		HarmCategorySexual,
		HarmCategoryMedical,
		HarmCategoryDangerous:
		return true
	default:
		return false
	}
}

// IsValidHarmBlockThreshold checks if a harm block threshold is valid
func IsValidHarmBlockThreshold(threshold HarmBlockThreshold) bool {
	switch threshold {
	case HarmBlockThresholdUnspecified,
		HarmBlockThresholdBlockLowAndAbove,
		HarmBlockThresholdBlockMediumAndAbove,
		HarmBlockThresholdBlockOnlyHigh,
		HarmBlockThresholdBlockNone,
		HarmBlockThresholdOff:
		return true
	default:
		return false
	}
}

// Validate validates a single safety setting
func (s *SafetySetting) Validate() error {
	if !IsValidHarmCategory(s.Category) {
		return types.NewInvalidRequestError(types.ProviderTypeGemini, fmt.Sprintf("invalid harm category: %s", s.Category)).
			WithOperation("validate_options")
	}
	if !IsValidHarmBlockThreshold(s.Threshold) {
		return types.NewInvalidRequestError(types.ProviderTypeGemini, fmt.Sprintf("invalid harm block threshold: %s", s.Threshold)).
			WithOperation("validate_options")
	}
	return nil
}

// ValidateSafetySettings validates a slice of safety settings
func ValidateSafetySettings(settings []SafetySetting) error {
	seen := make(map[HarmCategory]bool)
	for i, setting := range settings {
		if err := setting.Validate(); err != nil {
			return types.NewInvalidRequestError(types.ProviderTypeGemini, fmt.Sprintf("safety setting %d invalid", i)).
				WithOperation("validate_options").
				WithOriginalErr(err)
		}
		if seen[setting.Category] {
			return types.NewInvalidRequestError(types.ProviderTypeGemini, fmt.Sprintf("duplicate harm category in safety settings: %s", setting.Category)).
				WithOperation("validate_options")
		}
		seen[setting.Category] = true
	}
	return nil
}

// NewSafetySetting creates a new safety setting with validation
func NewSafetySetting(category HarmCategory, threshold HarmBlockThreshold) (*SafetySetting, error) {
	setting := &SafetySetting{
		Category:  category,
		Threshold: threshold,
	}
	if err := setting.Validate(); err != nil {
		return nil, err
	}
	return setting, nil
}

// DefaultSafetySettings returns default safety settings for common harm categories
// Default is BLOCK_MEDIUM_AND_ABOVE for all core categories
func DefaultSafetySettings() []SafetySetting {
	return []SafetySetting{
		{Category: HarmCategoryHarassment, Threshold: HarmBlockThresholdBlockMediumAndAbove},
		{Category: HarmCategoryHateSpeech, Threshold: HarmBlockThresholdBlockMediumAndAbove},
		{Category: HarmCategorySexuallyExplicit, Threshold: HarmBlockThresholdBlockMediumAndAbove},
		{Category: HarmCategoryDangerousContent, Threshold: HarmBlockThresholdBlockMediumAndAbove},
	}
}

// PermissiveSafetySettings returns safety settings that block only high-probability content
func PermissiveSafetySettings() []SafetySetting {
	return []SafetySetting{
		{Category: HarmCategoryHarassment, Threshold: HarmBlockThresholdBlockOnlyHigh},
		{Category: HarmCategoryHateSpeech, Threshold: HarmBlockThresholdBlockOnlyHigh},
		{Category: HarmCategorySexuallyExplicit, Threshold: HarmBlockThresholdBlockOnlyHigh},
		{Category: HarmCategoryDangerousContent, Threshold: HarmBlockThresholdBlockOnlyHigh},
	}
}

// StrictSafetySettings returns safety settings that block low and above probability content
func StrictSafetySettings() []SafetySetting {
	return []SafetySetting{
		{Category: HarmCategoryHarassment, Threshold: HarmBlockThresholdBlockLowAndAbove},
		{Category: HarmCategoryHateSpeech, Threshold: HarmBlockThresholdBlockLowAndAbove},
		{Category: HarmCategorySexuallyExplicit, Threshold: HarmBlockThresholdBlockLowAndAbove},
		{Category: HarmCategoryDangerousContent, Threshold: HarmBlockThresholdBlockLowAndAbove},
	}
}

// NoSafetySettings returns safety settings that don't block any content but return safety scores
func NoSafetySettings() []SafetySetting {
	return []SafetySetting{
		{Category: HarmCategoryHarassment, Threshold: HarmBlockThresholdBlockNone},
		{Category: HarmCategoryHateSpeech, Threshold: HarmBlockThresholdBlockNone},
		{Category: HarmCategorySexuallyExplicit, Threshold: HarmBlockThresholdBlockNone},
		{Category: HarmCategoryDangerousContent, Threshold: HarmBlockThresholdBlockNone},
	}
}

// IsSafetyBlocked checks if a response was blocked due to safety concerns
func (c *Candidate) IsSafetyBlocked() bool {
	if c.FinishReason == "SAFETY" {
		return true
	}
	for _, rating := range c.SafetyRatings {
		if rating.Blocked {
			return true
		}
	}
	return false
}

// GetSafetyRating returns the safety rating for a specific harm category
func (c *Candidate) GetSafetyRating(category HarmCategory) *SafetyRating {
	for i := range c.SafetyRatings {
		if c.SafetyRatings[i].Category == category {
			return &c.SafetyRatings[i]
		}
	}
	return nil
}

// ============================================================================
// Quota/Usage API Types for Code Assist
// ============================================================================

// GetQuotaRequest represents a request to get quota information
type GetQuotaRequest struct {
	Model   string `json:"model,omitempty"`   // Optional: filter by model
	Project string `json:"project,omitempty"` // Optional: filter by project
}

// QuotaLimit represents a quota limit for a specific type
type QuotaLimit struct {
	// Type is the type of quota
	Type string `json:"type"`

	// Limit is the maximum allowed value
	Limit int64 `json:"limit"`

	// Usage is the current usage
	Usage int64 `json:"usage"`

	// Remaining is the remaining quota (Limit - Usage)
	Remaining int64 `json:"remaining"`

	// Period is the time period for this quota (e.g., "day", "minute")
	Period string `json:"period,omitempty"`

	// ResetTime is when this quota will reset (RFC3339 timestamp)
	ResetTime string `json:"resetTime,omitempty"`
}

// GetQuotaResponse represents a response from the getQuota endpoint
type GetQuotaResponse struct {
	// Project is the Google Cloud project ID
	Project string `json:"project"`

	// Tier is the user's current tier
	Tier string `json:"tier"`

	// Quotas is a list of quota limits
	Quotas []QuotaLimit `json:"quotas,omitempty"`

	// CustomData contains provider-specific quota information
	CustomData map[string]interface{} `json:"customData,omitempty"`
}

// GetQuotaUsageRequest represents a request to get quota usage history
type GetQuotaUsageRequest struct {
	StartTime string `json:"startTime,omitempty"` // RFC3339 timestamp
	EndTime   string `json:"endTime,omitempty"`   // RFC3339 timestamp
	Model     string `json:"model,omitempty"`     // Optional: filter by model
}

// UsageRecord represents a single usage record
type UsageRecord struct {
	// Timestamp is when the usage occurred (RFC3339 timestamp)
	Timestamp string `json:"timestamp"`

	// Model is the model used
	Model string `json:"model"`

	// InputTokens is the number of input tokens used
	InputTokens int64 `json:"inputTokens,omitempty"`

	// OutputTokens is the number of output tokens used
	OutputTokens int64 `json:"outputTokens,omitempty"`

	// TotalTokens is the total number of tokens used
	TotalTokens int64 `json:"totalTokens,omitempty"`

	// RequestCount is the number of requests (1 for each record)
	RequestCount int64 `json:"requestCount"`

	// Operation is the type of operation (e.g., "generateContent")
	Operation string `json:"operation,omitempty"`
}

// GetQuotaUsageResponse represents a response from the getQuotaUsage endpoint
type GetQuotaUsageResponse struct {
	// Records is a list of usage records
	Records []UsageRecord `json:"records,omitempty"`

	// Summary aggregates the usage across all records
	Summary UsageSummary `json:"summary,omitempty"`

	// NextPageToken is a token for pagination
	NextPageToken string `json:"nextPageToken,omitempty"`
}

// UsageSummary represents aggregated usage statistics
type UsageSummary struct {
	// TotalInputTokens is the total input tokens used
	TotalInputTokens int64 `json:"totalInputTokens"`

	// TotalOutputTokens is the total output tokens used
	TotalOutputTokens int64 `json:"totalOutputTokens"`

	// TotalTokens is the total tokens used
	TotalTokens int64 `json:"totalTokens"`

	// TotalRequests is the total number of requests
	TotalRequests int64 `json:"totalRequests"`

	// StartTime is the start of the summary period (RFC3339 timestamp)
	StartTime string `json:"startTime"`

	// EndTime is the end of the summary period (RFC3339 timestamp)
	EndTime string `json:"endTime"`
}

// GetQuotaHistoryRequest represents a request to get quota usage history
type GetQuotaHistoryRequest struct {
	StartTime string `json:"startTime,omitempty"` // RFC3339 timestamp
	EndTime   string `json:"endTime,omitempty"`   // RFC3339 timestamp
	Model     string `json:"model,omitempty"`     // Optional: filter by model
	Limit     int32  `json:"limit,omitempty"`     // Maximum number of records to return
}

// GetQuotaHistoryResponse represents a response from the getQuotaHistory endpoint
type GetQuotaHistoryResponse struct {
	// Records is a list of usage records ordered by timestamp
	Records []UsageRecord `json:"records,omitempty"`

	// Summary aggregates the usage across all records
	Summary UsageSummary `json:"summary"`

	// NextPageToken is a token for pagination
	NextPageToken string `json:"nextPageToken,omitempty"`
}
