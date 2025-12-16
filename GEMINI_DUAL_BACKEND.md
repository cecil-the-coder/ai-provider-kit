# Gemini Dual Backend Support Implementation

## Overview
This implementation enables support for both Gemini API and Vertex AI backends with automatic schema conversion for enterprise GCP integration.

## Features Implemented

### 1. Backend Type System
- **BackendType Enumeration**: Added `BackendGeminiAPI` and `BackendVertexAI` constants
- **ClientConfig Structure**: Enhanced configuration with backend-aware fields
  - Backend selection (Gemini API vs Vertex AI)
  - Project and Location for Vertex AI
  - Service account JSON support
  - Base URL override capability

### 2. Automatic Backend Detection
**BackendDetector** class with detection priority:
1. Explicit backend configuration in `ClientConfig`
2. `GOOGLE_GENAI_USE_VERTEXAI` environment variable
3. Presence of Project/Location in config (implies Vertex AI)
4. Defaults to Gemini API

### 3. Schema Conversion
**SchemaConverter** handles conversion between formats:
- Request conversion (Gemini API ↔ Vertex AI)
- Response conversion with error handling
- Streaming response conversion
- Both backends currently use the same schema format, but the converter provides abstraction for future differences

### 4. Backend Router
**BackendRouter** manages routing logic:
- Automatic backend detection
- Base URL construction for each backend
- Model path formatting
  - Gemini API: `models/{model}`
  - Vertex AI: `projects/{project}/locations/{location}/publishers/google/models/{model}`
- Request URL building with API key support

### 5. Regional Deployment Support
Vertex AI regional deployment features:
- Configurable GCP regions (e.g., "us-central1", "europe-west4")
- Region-specific endpoint URLs
- Default location: "us-central1"

### 6. Vertex AI Authentication
**VertexAIAuthenticator** with multiple credential sources:
- Service account JSON (explicit parameter)
- `GOOGLE_APPLICATION_CREDENTIALS` environment variable (file path)
- `GOOGLE_SERVICE_ACCOUNT_JSON` environment variable (JSON string)
- GCE metadata server fallback
- Automatic token refresh

### 7. Integration with GeminiProvider
Enhanced GeminiProvider with:
- Backend router initialization in constructor and configuration
- Updated API request methods to use backend router
- Streaming support for both backends
- Connectivity tests for both backends

### 8. Configuration Methods
New helper functions in types.go:
- `DetectBackendFromEnv()`: Detect backend from environment
- `NewClientConfigFromEnv()`: Create config from environment variables
- `ClientConfig.Validate()`: Validate backend-specific configuration
- `ClientConfig.GetBaseURL()`: Get backend-appropriate base URL
- `ClientConfig.GetEndpoint()`: Get full endpoint URL for model

## Usage Examples

### Gemini API Backend (Default)
```go
config := types.ProviderConfig{
    Type: types.ProviderTypeGemini,
    ProviderConfig: map[string]interface{}{
        "api_key": "your-api-key",
        "backend": "gemini-api", // optional, this is default
    },
}
provider := gemini.NewGeminiProvider(config)
```

### Vertex AI Backend
```go
config := types.ProviderConfig{
    Type: types.ProviderTypeGemini,
    ProviderConfig: map[string]interface{}{
        "backend":  "vertex-ai",
        "project":  "my-gcp-project",
        "location": "us-central1",
        "service_account_json": serviceAccountJSON, // optional
    },
}
provider := gemini.NewGeminiProvider(config)
```

### Environment Variable Configuration
```bash
# Use Vertex AI backend
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=my-gcp-project
export GOOGLE_CLOUD_LOCATION=us-central1
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
```

### Automatic Detection
```go
// Backend automatically detected from environment and config
clientConfig, err := gemini.NewClientConfigFromEnv()
if err != nil {
    log.Fatal(err)
}
```

## File Structure

### New Files
- `pkg/providers/gemini/backend.go` - Backend detection, routing, and schema conversion
- `pkg/providers/gemini/vertexai_auth.go` - Vertex AI authentication
- `pkg/providers/gemini/backend_test.go` - Comprehensive backend tests

### Modified Files
- `pkg/providers/gemini/types.go` - Added backend types and configuration
- `pkg/providers/gemini/gemini.go` - Integrated backend router
- `pkg/providers/gemini/gemini_streaming.go` - Updated streaming for both backends

## Testing
All tests pass successfully with coverage for:
- Backend type validation
- Backend detection logic
- URL construction for both backends
- Configuration validation
- Schema conversion
- Router functionality

Run tests:
```bash
go test ./pkg/providers/gemini/...
```

## Backend Comparison

| Feature | Gemini API | Vertex AI |
|---------|-----------|-----------|
| Authentication | API Key or OAuth | Service Account or OAuth |
| Base URL | generativelanguage.googleapis.com | {location}-aiplatform.googleapis.com |
| Model Path | models/{model} | projects/{project}/locations/{location}/publishers/google/models/{model} |
| Regional | Single endpoint | Multi-region support |
| Enterprise | Consumer & Enterprise | Enterprise-focused |
| Pricing | Pay-per-use | GCP billing |

## Future Enhancements
- Full JWT assertion implementation for service accounts
- Advanced token caching strategies
- Regional failover support
- Vertex AI-specific features (e.g., private endpoints)
- Enhanced monitoring and metrics per backend

## Notes
- The implementation maintains backward compatibility with existing Gemini API usage
- Both backends share the same request/response schema format currently
- Schema converter provides abstraction for future backend-specific differences
- Service account authentication uses a simplified implementation (production should use official Google Cloud SDK)
