package cli

import "testing"

func TestExtractIDFromContainerName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"with ID", "airlock-claude-abc123", "abc123"},
		{"legacy name", "airlock-claude", ""},
		{"short ID", "airlock-claude-a1", "a1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIDFromContainerName(tt.input, "airlock-claude-")
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
