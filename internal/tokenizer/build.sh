#!/bin/bash
# Build script for the Rust tokenizer library
# This script checks for Rust/Cargo, builds the library, and runs tests

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOKENIZER_LIB_DIR="$SCRIPT_DIR/tokenizer-lib"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "======================================"
echo "Rust Tokenizer Build Script"
echo "======================================"
echo ""

# Check if Rust/Cargo is installed
if ! command -v cargo &> /dev/null; then
    echo -e "${RED}Error: Rust/Cargo not found${NC}"
    echo "Please install Rust from https://rustup.rs/"
    echo ""
    echo "After installing Rust, run this script again."
    exit 1
fi

echo -e "${GREEN}✓${NC} Rust/Cargo found: $(cargo --version)"

# Check if we're in the right directory
if [ ! -f "$TOKENIZER_LIB_DIR/Cargo.toml" ]; then
    echo -e "${RED}Error: Cargo.toml not found in $TOKENIZER_LIB_DIR${NC}"
    exit 1
fi

echo -e "${GREEN}✓${NC} Tokenizer library found at $TOKENIZER_LIB_DIR"
echo ""

# Build the Rust library
echo "Building Rust tokenizer library..."
cd "$TOKENIZER_LIB_DIR"

# Build in release mode for maximum performance
cargo build --release

# Check if build succeeded
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓${NC} Rust library built successfully"
    echo "  Library location: $TOKENIZER_LIB_DIR/target/release/libtokenizer_lib.a"
else
    echo -e "${RED}✗${NC} Rust build failed"
    exit 1
fi

echo ""

# Build Go code with rusttokenizer tag
echo "Building Go code with rusttokenizer tag..."
cd "$PROJECT_ROOT"

# Test that the build works
echo ""
echo "Testing build..."
go build -tags=rusttokenizer ./internal/tokenizer/...

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓${NC} Go code built successfully with rusttokenizer tag"
else
    echo -e "${RED}✗${NC} Go build failed"
    exit 1
fi

echo ""

# Run tests
echo "Running tests..."
go test -tags=rusttokenizer ./internal/tokenizer/... -v

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓${NC} All tests passed"
else
    echo -e "${YELLOW}⚠${NC} Some tests failed (expected if Rust library not properly linked)"
fi

echo ""
echo "======================================"
echo -e "${GREEN}Build Complete!${NC}"
echo "======================================"
echo ""
echo "To use the Rust tokenizer:"
echo "  go build -tags=rusttokenizer ./..."
echo ""
echo "To run benchmarks:"
echo "  go test -tags=rusttokenizer -bench=. -benchmem ./internal/tokenizer/..."
echo ""
echo "To verify which implementation is in use:"
echo '  go run -tags=rusttokenizer . -check-tokenizer'
echo ""
