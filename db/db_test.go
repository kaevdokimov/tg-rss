package db

import (
	"testing"
)

func TestCleanUTF8String(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid UTF-8 string",
			input:    "Привет мир",
			expected: "Привет мир",
		},
		{
			name:     "string with null bytes",
			input:    "Hello\x00World\x00Test",
			expected: "HelloWorldTest",
		},
		{
			name:     "string with invalid UTF-8",
			input:    "Hello\x80\x81World", // Invalid UTF-8 bytes
			expected: "Hello�World",        // Should be replaced with replacement char
		},
		{
			name:     "string with both null bytes and invalid UTF-8",
			input:    "Test\x00\x80\x81String\x00",
			expected: "Test�String",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only null bytes",
			input:    "\x00\x00\x00",
			expected: "",
		},
		{
			name:     "HTML content with null bytes",
			input:    "<div>\x00Hello\x00</div>\x00<script>\x80\x81</script>",
			expected: "<div>Hello</div><script>�</script>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanUTF8String(tt.input)
			if result != tt.expected {
				t.Errorf("cleanUTF8String(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
