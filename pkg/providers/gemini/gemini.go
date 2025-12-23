// Package gemini provides a Google Gemini AI provider implementation.
// It includes support for chat completions, streaming, tool calling, and OAuth authentication.
//
// The provider has been split into multiple files for better organization:
// - provider.go: Core provider implementation and interface methods
// - config.go: Configuration and initialization
// - chat.go: Chat completion logic
// - gemini_streaming.go: Streaming response handling
// - gemini_tools.go: Tool calling conversion
// - gemini_multimodal.go: Message transformation
// - types.go: Type definitions
// - backend.go: Backend router for Gemini API vs Vertex AI
package gemini

// The GeminiProvider implementation is now split across multiple files.
// The main struct definition and interface implementations are in provider.go.
// See individual files for specific functionality.
