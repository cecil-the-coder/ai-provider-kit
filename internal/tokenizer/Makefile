# Makefile for building the Rust tokenizer

.PHONY: all build test clean benchmark help rust go

# Default target
all: build

# Build everything (Rust library + Go code)
build: rust
	@echo "Building Go code with rusttokenizer tag..."
	go build -tags=rusttokenizer ./...
	@echo "Build complete!"

# Build only the Rust library
rust:
	@echo "Building Rust tokenizer library..."
	cd tokenizer-lib && cargo build --release
	@echo "Rust library built: tokenizer-lib/target/release/libtokenizer_lib.a"

# Build Go code without Rust (pure Go implementation)
go:
	@echo "Building Go code with pure Go implementation..."
	go build ./...
	@echo "Build complete (pure Go)!"

# Run tests
test: build
	@echo "Running tests with Rust tokenizer..."
	go test -tags=rusttokenizer -v ./internal/tokenizer/...

# Run tests with Go implementation only
test-go:
	@echo "Running tests with pure Go implementation..."
	go test -v ./internal/tokenizer/...

# Run benchmarks
benchmark: build
	@echo "Running benchmarks..."
	go test -tags=rusttokenizer -bench=. -benchmem ./internal/tokenizer/...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	cd tokenizer-lib && cargo clean
	go clean ./...
	@echo "Clean complete!"

# Run the build script
install:
	@echo "Running build script..."
	./internal/tokenizer/build.sh

# Show help
help:
	@echo "Rust Tokenizer Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  all        - Build Rust library and Go code (default)"
	@echo "  build      - Same as 'all'"
	@echo "  rust       - Build only the Rust library"
	@echo "  go         - Build Go code with pure Go implementation"
	@echo "  test       - Run tests with Rust tokenizer"
	@echo "  test-go    - Run tests with pure Go implementation"
	@echo "  benchmark  - Run benchmarks"
	@echo "  clean      - Clean build artifacts"
	@echo "  install    - Run the build script"
	@echo "  help       - Show this help message"
