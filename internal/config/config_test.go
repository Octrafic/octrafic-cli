package config

import (
	"testing"
	"time"
)

func TestShouldCheckForUpdate(t *testing.T) {
	// Last check was 25 hours ago — should check
	c := &Config{LastUpdateCheck: time.Now().Add(-25 * time.Hour)}
	if !c.ShouldCheckForUpdate() {
		t.Error("expected true when last check was 25h ago")
	}

	// Last check was 1 hour ago — should not check
	c = &Config{LastUpdateCheck: time.Now().Add(-1 * time.Hour)}
	if c.ShouldCheckForUpdate() {
		t.Error("expected false when last check was 1h ago")
	}

	// Never checked (zero value) — should check
	c = &Config{}
	if !c.ShouldCheckForUpdate() {
		t.Error("expected true when never checked")
	}
}

func TestIsLocalProvider(t *testing.T) {
	tests := []struct {
		provider string
		expected bool
	}{
		{"ollama", true},
		{"llamacpp", true},
		{"claude", false},
		{"openai", false},
		{"openrouter", false},
		{"", false},
	}

	for _, tt := range tests {
		got := IsLocalProvider(tt.provider)
		if got != tt.expected {
			t.Errorf("IsLocalProvider(%q) = %v, want %v", tt.provider, got, tt.expected)
		}
	}
}

func TestGetEnvVarName(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"api_key", "OCTRAFIC_API_KEY"},
		{"provider", "OCTRAFIC_PROVIDER"},
		{"base_url", "OCTRAFIC_BASE_URL"},
		{"", "OCTRAFIC_"},
	}

	for _, tt := range tests {
		got := GetEnvVarName(tt.key)
		if got != tt.expected {
			t.Errorf("GetEnvVarName(%q) = %q, want %q", tt.key, got, tt.expected)
		}
	}
}
