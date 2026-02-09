package llm

import (
	"testing"

	"github.com/Octrafic/octrafic-cli/internal/llm/common"
)

func TestEnsureV1Suffix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"http://localhost:11434", "http://localhost:11434/v1"},
		{"http://localhost:11434/", "http://localhost:11434/v1"},
		{"http://localhost:11434/v1", "http://localhost:11434/v1"},
		{"http://localhost:11434/v1/", "http://localhost:11434/v1"},
		{"http://example.com/api", "http://example.com/api/v1"},
	}

	for _, tt := range tests {
		got := ensureV1Suffix(tt.input)
		if got != tt.expected {
			t.Errorf("ensureV1Suffix(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestCreateProviderUnsupported(t *testing.T) {
	_, err := CreateProvider(common.ProviderConfig{Provider: "unsupported"})
	if err == nil {
		t.Error("expected error for unsupported provider")
	}
}

func TestCreateProviderOllamaDefaultURL(t *testing.T) {
	provider, err := CreateProvider(common.ProviderConfig{
		Provider: "ollama",
		APIKey:   "dummy",
		Model:    "llama3",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestCreateProviderLlamacppDefaultURL(t *testing.T) {
	provider, err := CreateProvider(common.ProviderConfig{
		Provider: "llamacpp",
		APIKey:   "dummy",
		Model:    "default",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
}
