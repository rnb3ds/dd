package internal

import (
	"strings"
	"testing"
)

func TestSanitizeControlChars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"normal", "hello world", "hello world"},
		{"newline", "hello\nworld", "hello\nworld"},
		{"tab", "hello\tworld", "hello\tworld"},
		{"carriage return", "hello\rworld", "hello\rworld"},
		{"null byte", "hello\x00world", "helloworld"},
		{"del char", "hello\x7fworld", "helloworld"},
		{"control char", "hello\x01world", "hello\\x01world"},
		{"multiple control", "\x00\x01\x02", "\\x01\\x02"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeControlChars(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeControlChars(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeANSIEscape(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		shouldNotContain string
	}{
		{"CSI color red", "\x1b[31mhello\x1b[0m", "\x1b[31m"},
		{"CSI bold", "\x1b[1mbold\x1b[0m", "\x1b[1m"},
		{"CSI cursor", "\x1b[2J", "\x1b[2J"},
		{"OSC title", "\x1b]0;title\x07", "\x1b]0;"},
		{"mixed", "normal\x1b[31mred\x1b[0mnormal", "\x1b[31m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeControlChars(tt.input)
			if strings.Contains(result, tt.shouldNotContain) {
				t.Errorf("Result should not contain %q, got: %q", tt.shouldNotContain, result)
			}
		})
	}
}
