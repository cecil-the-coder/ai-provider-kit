// Package gemini provides type definitions for Google Gemini AI provider.
// It includes request/response structures, streaming types, and function calling definitions.
package gemini

// Request/Response types for Gemini API

// GenerateContentRequest represents a request to generate content
type GenerateContentRequest struct {
	Contents         []Content         `json:"contents"`
	GenerationConfig *GenerationConfig `json:"generationConfig,omitempty"`
	Tools            []GeminiTool      `json:"tools,omitempty"`
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
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason,omitempty"`
}

// UsageMetadata represents usage metadata
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

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
