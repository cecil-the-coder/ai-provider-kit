# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of AI Provider Kit
- Multi-provider support for OpenAI, Anthropic, Gemini, Qwen, Cerebras, and OpenRouter
- Local model provider support for LM Studio, Ollama, and Llama.cpp
- Factory pattern for dynamic provider creation and configuration
- Comprehensive authentication support (API keys, OAuth 2.0)
- Health monitoring and metrics collection
- Load balancing across multiple API keys
- Extensible architecture for adding new providers
- Complete test coverage with unit and integration tests
- CI/CD pipeline with automated releases
- Docker containerization support

### Features
- Provider factory with thread-safe operations
- Mock provider implementations for testing
- Stream-based chat completions
- Tool calling support
- Model discovery and capabilities
- Usage tracking and metrics
- Configuration management
- Error handling and validation

### Documentation
- Comprehensive API documentation
- Usage examples and quick start guide
- Test coverage reports
- CI/CD pipeline documentation

## [1.0.0] - 2025-11-16

### Added
- Initial stable release
- Core provider factory implementation
- Support for major AI providers
- Complete test suite
- Release automation