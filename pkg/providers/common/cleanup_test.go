package common

import "testing"

func TestCleanCodeResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Clean code without formatting",
			input:    "function test() {}",
			expected: "function test() {}",
		},
		{
			name:     "Code with markdown blocks",
			input:    "```javascript\nfunction test() {}\n```",
			expected: "function test() {}",
		},
		{
			name:     "Code with language identifier",
			input:    "javascript\nfunction test() {}",
			expected: "function test() {}",
		},
		{
			name:     "Code with leading and trailing whitespace",
			input:    "   \nfunction test() {}\n   ",
			expected: "function test() {}",
		},
		{
			name:     "Go code with markdown blocks",
			input:    "```go\nfunc main() { println('Hello') }\n```",
			expected: "func main() { println('Hello') }",
		},
		{
			name:     "Go code with language identifier",
			input:    "go\nfunc main() { println('Hello') }",
			expected: "func main() { println('Hello') }",
		},
		{
			name:     "Code with only whitespace padding",
			input:    "  func main() { println('Hello') }  ",
			expected: "func main() { println('Hello') }",
		},
		{
			name:     "Code with backticks only at start",
			input:    "```\nfunction test() {}",
			expected: "function test() {}",
		},
		{
			name:     "Code with backticks only at end",
			input:    "function test() {}\n```",
			expected: "function test() {}",
		},
		{
			name:     "Multi-line code with language identifier",
			input:    "python\ndef hello():\n    print('world')",
			expected: "def hello():\n    print('world')",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only whitespace",
			input:    "   \n\n   ",
			expected: "",
		},
		{
			name:     "Language identifier that is too long (not removed)",
			input:    "this-is-a-very-long-identifier-more-than-twenty-chars\ncode here",
			expected: "this-is-a-very-long-identifier-more-than-twenty-chars\ncode here",
		},
		{
			name:     "First line with spaces (not a language identifier)",
			input:    "this has spaces\ncode here",
			expected: "this has spaces\ncode here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanCodeResponse(tt.input)
			if result != tt.expected {
				t.Errorf("CleanCodeResponse(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Benchmark to ensure the function is performant
func BenchmarkCleanCodeResponse(b *testing.B) {
	input := "```go\nfunc main() {\n    println(\"Hello, World!\")\n}\n```"
	for i := 0; i < b.N; i++ {
		CleanCodeResponse(input)
	}
}
