//go:build rusttokenizer
// +build rusttokenizer

package tokenizer

/*
#cgo LDFLAGS: -L${SRCDIR}/tokenizer-lib/target/release -ltokenizer-lib -lm -ldl
#cgo CFLAGS: -I${SRCDIR}/tokenizer-lib/include
#cgo linux LDFLAGS: -lpthread
#cgo darwin LDFLAGS: -framework Security
#cgo windows LDFLAGS: -ladvapi32

#include <stdlib.h>
#include <stdint.h>

// Rust FFI function declarations
extern int tiktoken_count_tokens(const char *text, const char *model);
extern int tiktoken_count_batch(const char **texts, size_t count, const char *model);
extern int tiktoken_is_available(void);
extern const char *tiktoken_version(void);
extern const char *tiktoken_get_encoding_for_model(const char *model);

// Detailed result structure
typedef struct {
    int count;
    char *error;
} TikTokenResult;

extern TikTokenResult tiktoken_count_tokens_detailed(const char *text, const char *model);
extern void tiktoken_free_string(char *ptr);
extern void tiktoken_free_result(TikTokenResult *result);
*/
import "C"
import (
	"fmt"
	"sync"
	"unsafe"
)

// rustCounter implements tokenCounter using the Rust tiktoken library via CGO
type rustCounter struct {
	available bool
	version   string
	mu        sync.RWMutex
}

// getRustCounter returns a new rustCounter instance if the Rust library is available
func getRustCounter() tokenCounter {
	rc := &rustCounter{}
	if !rc.IsAvailable() {
		return nil
	}
	return rc
}

// IsAvailable checks if the Rust tokenizer library is available
func (rc *rustCounter) IsAvailable() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if rc.available {
		return true
	}

	// Check availability by calling the Rust function
	available := C.tiktoken_is_available() == 1
	rc.available = available

	if available {
		// Get version information
		cVersion := C.tiktoken_version()
		if cVersion != nil {
			rc.version = C.GoString(cVersion)
		}
	}

	return available
}

// Name returns the name of this implementation
func (rc *rustCounter) Name() string {
	return fmt.Sprintf("rust-cgo (%s)", rc.version)
}

// CountTokens counts tokens using the Rust implementation
func (rc *rustCounter) CountTokens(text, model string) (int, error) {
	if !rc.IsAvailable() {
		return 0, fmt.Errorf("rust tokenizer not available")
	}

	if text == "" {
		return 0, nil
	}

	// Convert Go strings to C strings
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	cModel := C.CString(model)
	defer C.free(unsafe.Pointer(cModel))

	// Call the Rust function
	count := C.tiktoken_count_tokens(cText, cModel)

	if count < 0 {
		return 0, fmt.Errorf("token counting failed")
	}

	return int(count), nil
}

// CountBatch counts tokens for multiple texts using the Rust implementation
// This is more efficient than calling CountTokens multiple times
func (rc *rustCounter) CountBatch(texts []string, model string) (int, error) {
	if !rc.IsAvailable() {
		return 0, fmt.Errorf("rust tokenizer not available")
	}

	if len(texts) == 0 {
		return 0, nil
	}

	// Convert Go strings to C strings
	cTexts := make([]*C.char, len(texts))
	for i, text := range texts {
		cTexts[i] = C.CString(text)
		defer C.free(unsafe.Pointer(cTexts[i]))
	}

	cModel := C.CString(model)
	defer C.free(unsafe.Pointer(cModel))

	// Create a slice of pointers to pass to C
	var cTextsPtr **C.char
	if len(cTexts) > 0 {
		cTextsPtr = &cTexts[0]
	}

	// Call the Rust batch function
	count := C.tiktoken_count_batch(
		cTextsPtr,
		C.size_t(len(texts)),
		cModel,
	)

	if count < 0 {
		return 0, fmt.Errorf("batch token counting failed")
	}

	return int(count), nil
}

// GetEncodingForModel returns the encoding name for a given model
func GetEncodingForModel(model string) string {
	if !IsRustAvailable() {
		return ""
	}

	cModel := C.CString(model)
	defer C.free(unsafe.Pointer(cModel))

	cEncoding := C.tiktoken_get_encoding_for_model(cModel)
	if cEncoding == nil {
		return ""
	}

	return C.GoString(cEncoding)
}

// CountTokensDetailed counts tokens with detailed error information
// This is useful for debugging tokenization issues
func CountTokensDetailed(text, model string) (count int, errMsg string, err error) {
	rc := getRustCounter()
	if rc == nil || !rc.IsAvailable() {
		return 0, "", fmt.Errorf("rust tokenizer not available")
	}

	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	cModel := C.CString(model)
	defer C.free(unsafe.Pointer(cModel))

	result := C.tiktoken_count_tokens_detailed(cText, cModel)
	defer C.tiktoken_free_result(&result)

	if result.error != nil {
		errMsg = C.GoString(result.error)
		C.tiktoken_free_string(result.error)
		return int(result.count), errMsg, fmt.Errorf("tokenization error: %s", errMsg)
	}

	return int(result.count), "", nil
}
