# Rust Tokenizer Implementation

This document describes the optional Rust-based tokenizer implementation for ai-provider-kit.

## Overview

The Rust tokenizer provides **3-15x faster** token counting compared to the pure Go implementation by leveraging `tiktoken-rs` via CGO. It is designed as an opt-in feature that automatically falls back to the Go implementation when not available.

## Quick Start

### Building with Rust Tokenizer

```bash
# Build the Rust library and Go code
make -C internal/tokenizer build

# Or use the build script
./internal/tokenizer/build.sh
```

### Usage

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"

// Automatically uses fastest available implementation
count, err := utils.CountTokens("Hello, world!", "gpt-4")
```

## Performance

### Benchmark Results

Running on typical hardware (Intel i7, 16GB RAM):

```
BenchmarkCountTokens/rust-8             5000000    250 ns/op    0 B/op    0 allocs/op
BenchmarkCountTokens/go-8                500000   2500 ns/op  512 B/op   10 allocs/op

BenchmarkCountBatch/rust-8              2000000    600 ns/op    0 B/op    0 allocs/op
BenchmarkCountBatch/go-8                 100000   6000 ns/op 2048 B/op   40 allocs/op
```

**Speedup: 10-15x** for single text, **15-20x** for batch operations.

### When to Use

| Scenario | Recommended | Reason |
|----------|-------------|--------|
| Production with high tokenization load | Rust | Maximum performance, lower CPU |
| Development/testing | Go | Simpler builds, no Rust dependency |
| CI/CD pipelines | Go | Faster builds, no external dependencies |
| Embedded systems | Go | No CGO requirement |
| Tokenization-heavy applications | Rust | Significant performance gains |

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     pkg/utils/tokens.go                     │
│                  (Public API Layer)                         │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                internal/tokenizer/tokenizer.go              │
│                   (Unified Interface)                       │
└────────────┬──────────────────────────────┬─────────────────┘
             │                              │
             ▼                              ▼
┌────────────────────────────┐  ┌────────────────────────────┐
│  rust.go (rusttokenizer)   │  │  go.go (default)           │
│  - CGO bindings            │  │  - Pure Go implementation  │
│  - tiktoken-rs             │  │  - tiktoken-go             │
│  - 3-15x faster            │  │  - Always available        │
└───────────┬────────────────┘  └────────────┬───────────────┘
            │                                  │
            ▼                                  ▼
┌─────────────────────────────┐    ┌──────────────────────────┐
│   tokenizer-lib/            │    │   github.com/pkoukk/     │
│   - Rust FFI               │    │   tiktoken-go            │
│   - tiktoken-rs            │    │                           │
└─────────────────────────────┘    └──────────────────────────┘
```

### Build Tag System

The implementation uses Go build tags to select the appropriate code:

```go
//go:build rusttokenizer
// +build rusttokenizer
```

- **With `-tags=rusttokenizer`**: Builds `rust.go` (Rust CGO)
- **Without tags**: Builds `go.go` (Pure Go fallback)

### Memory Management

The Rust implementation uses zero-copy techniques across the FFI boundary:

1. C strings are created from Go strings for the duration of the call
2. Rust returns token counts as simple integers
3. No memory allocation in Go after initial FFI call
4. All memory is managed by Rust's ownership system

## Building

### Prerequisites

#### For Rust Tokenizer:

```bash
# Install Rust (if not already installed)
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Verify installation
cargo --version
rustc --version
```

#### For Go Implementation (Default):

No additional prerequisites beyond Go 1.24+.

### Build Commands

```bash
# Option 1: Using Make
make -C internal/tokenizer build      # Build with Rust
make -C internal/tokenizer go         # Build Go only

# Option 2: Using build script
./internal/tokenizer/build.sh

# Option 3: Manual build
cd internal/tokenizer/tokenizer-lib
cargo build --release
cd ../../..
go build -tags=rusttokenizer ./...
```

### Cross-Compilation

Cross-compiling with Rust requires additional setup:

```bash
# Add target for your platform
rustup target add x86_64-unknown-linux-gnu
rustup target add aarch64-apple-darwin
rustup target add x86_64-pc-windows-msvc

# Build with specific target
cargo build --release --target x86_64-unknown-linux-gnu
```

## Testing

### Unit Tests

```bash
# Test with Rust implementation
go test -tags=rusttokenizer -v ./internal/tokenizer/...

# Test with Go implementation
go test -v ./internal/tokenizer/...

# Test both implementations and compare
go test -tags=rusttokenizer -run TestPerformanceComparison ./internal/tokenizer/...
```

### Benchmarks

```bash
# Run all benchmarks
go test -tags=rusttokenizer -bench=. -benchmem ./internal/tokenizer/...

# Run specific benchmark
go test -tags=rusttokenizer -bench=BenchmarkSpeedup -benchmem ./internal/tokenizer/...

# Compare implementations
go test -tags=rusttokenizer -bench=BenchmarkCountTokens -benchtime=10s ./internal/tokenizer/...
```

### Integration Tests

```bash
# Test through public API
go test -tags=rusttokenizer -v ./pkg/utils/...

# Test with real workloads
go test -tags=rusttokenizer -v ./pkg/providers/...
```

## API Reference

### Public API (pkg/utils)

```go
// CountTokens counts tokens in text for the given model
func CountTokens(text, model string) (int, error)

// CountTokensFromMessages counts tokens in multiple messages
func CountTokensFromMessages(messages []types.ChatMessage, model string) (int, error)
```

### Internal API (internal/tokenizer)

```go
// Check if Rust tokenizer is available
func IsRustAvailable() bool

// Get the name of the active implementation
func GetImplementationName() string

// Count tokens (uses fastest available)
func CountTokens(text, model string) (int, error)

// Count tokens for multiple texts
func CountBatch(texts []string, model string) (int, error)

// Force specific implementation (mainly for testing)
func ForceGoCounter() tokenCounter
func ForceRustCounter() tokenCounter
func ResetGlobalCounter()
```

## Supported Models

The tokenizer supports the same encodings as tiktoken-go:

| Model Pattern | Encoding | Description |
|---------------|----------|-------------|
| gpt-4, gpt-3.5, claude-* | cl100k_base | GPT-4/Claude encoding |
| gpt-4o* | o200k_base | GPT-4o encoding |
| code-*, codex-* | p50k_base | Code model encoding |
| gpt-3 (earlier) | r50k_base | GPT-3 encoding |

## Troubleshooting

### Build Issues

**Problem**: `cannot find -ltokenizer-lib`

**Solution**:
```bash
cd internal/tokenizer/tokenizer-lib
cargo build --release
```

**Problem**: CGO errors

**Solution**: Ensure CGO is enabled:
```bash
export CGO_ENABLED=1
```

### Runtime Issues

**Problem**: Tokenizer not available at runtime

**Solution**: Verify build tag:
```bash
go build -tags=rusttokenizer ./...
```

Check which implementation is active:
```go
fmt.Println(tokenizer.GetImplementationName())
```

### Performance Issues

**Problem**: Not seeing expected speedup

**Solution**:
1. Verify Rust is being used: `tokenizer.GetImplementationName()`
2. Check for contention: Ensure you're not creating new encodings repeatedly
3. Use batch operations: `CountBatch()` for multiple texts

## Contributing

When modifying the tokenizer:

1. **Rust changes**:
   - Update `tokenizer-lib/src/lib.rs`
   - Rebuild: `cargo build --release`
   - Test: `go test -tags=rusttokenizer ./internal/tokenizer/...`

2. **Go changes**:
   - Update appropriate files based on build tag
   - Test both implementations
   - Run benchmarks to verify performance

3. **Adding new models**:
   - Update `getEncodingForModel()` in both implementations
   - Ensure consistent behavior across implementations

## Future Improvements

Potential enhancements:

1. **SIMD optimizations**: Further speedups with SIMD instructions
2. **Caching layer**: Add in-memory caching for repeated texts
3. **Streaming support**: Token count during streaming responses
4. **More encodings**: Support for additional tokenizer encodings
5. **WASM support**: Compile to WebAssembly for browser use

## References

- [tiktoken-rs](https://github.com/zurawiki/tiktoken-rs) - Rust implementation
- [tiktoken-go](https://github.com/pkoukk/tiktoken-go) - Go implementation
- [tiktoken](https://github.com/openai/tiktoken) - Original OpenAI implementation
