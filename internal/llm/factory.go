package llm

import (
	"fmt"
	"strings"

	"github.com/Octrafic/octrafic-cli/internal/llm/claude"
	"github.com/Octrafic/octrafic-cli/internal/llm/common"
	"github.com/Octrafic/octrafic-cli/internal/llm/openai"
)

// CreateProvider creates a provider based on the config
func CreateProvider(config common.ProviderConfig) (common.Provider, error) {
	switch config.Provider {
	case "claude", "anthropic":
		return claude.NewClaudeProvider(config)
	case "openai", "openrouter":
		return openai.NewOpenAIProvider(config)
	case "ollama":
		if config.BaseURL == "" {
			config.BaseURL = "http://localhost:11434/v1"
		} else {
			config.BaseURL = ensureV1Suffix(config.BaseURL)
		}
		return openai.NewOpenAIProvider(config)
	case "llamacpp":
		if config.BaseURL == "" {
			config.BaseURL = "http://localhost:8080/v1"
		} else {
			config.BaseURL = ensureV1Suffix(config.BaseURL)
		}
		return openai.NewOpenAIProvider(config)
	case "custom":
		config.BaseURL = ensureV1Suffix(config.BaseURL)
		return openai.NewOpenAIProvider(config)
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: claude, anthropic, openai, openrouter, ollama, llamacpp, custom)", config.Provider)
	}
}

func ensureV1Suffix(baseURL string) string {
	trimmed := strings.TrimSuffix(baseURL, "/")
	if strings.HasSuffix(trimmed, "/v1") {
		return trimmed
	}
	return trimmed + "/v1"
}
