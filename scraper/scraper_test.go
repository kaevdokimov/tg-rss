package scraper

import (
	"strings"
	"testing"
)

func TestRemoveNullBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "string without null bytes",
			input:    "normal string",
			expected: "normal string",
		},
		{
			name:     "string with null bytes",
			input:    "string\x00with\x00null\x00bytes",
			expected: "stringwithnullbytes",
		},
		{
			name:     "only null bytes",
			input:    "\x00\x00\x00",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "null bytes at beginning and end",
			input:    "\x00start\x00middle\x00end\x00",
			expected: "startmiddleend",
		},
		{
			name:     "HTML content with null bytes",
			input:    "<p>Hello\x00world</p>\x00<script>alert('test')</script>",
			expected: "<p>Helloworld</p><script>alert('test')</script>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeNullBytes(tt.input)
			if result != tt.expected {
				t.Errorf("removeNullBytes(%q) = %q, expected %q", tt.input, result, tt.expected)
			}

			// Проверяем, что в результате нет null байтов
			if strings.Contains(result, "\x00") {
				t.Errorf("Result still contains null bytes: %q", result)
			}
		})
	}
}
