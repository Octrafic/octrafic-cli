package scanner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Octrafic/octrafic-cli/internal/agents"
)

// ProjectFramework contains LLM-detected metadata about the project
type ProjectFramework struct {
	Language  string `json:"language"`
	Framework string `json:"framework"`
}

// detectFramework finds dependency files and asks the LLM to identify the framework.
func (s *Scanner) detectFramework(progressCallback func(string)) (*ProjectFramework, error) {
	if progressCallback != nil {
		progressCallback("➔ Analyzing project structure...")
	}

	// We only look at common root-level config files
	configFiles := []string{
		"go.mod", "package.json", "requirements.txt",
		"pyproject.toml", "pom.xml", "build.gradle",
		"composer.json", "Gemfile", "Cargo.toml",
	}

	var foundConfigs []string
	var configContents strings.Builder

	for _, file := range configFiles {
		path := filepath.Join(s.dir, file)
		content, err := os.ReadFile(path)
		if err == nil {
			foundConfigs = append(foundConfigs, file)
			fmt.Fprintf(&configContents, "\n--- %s ---\n", file)
			// Truncate to avoid massive lockfiles just in case, though these are usually small
			if len(content) > 5000 {
				configContents.Write(content[:5000])
				configContents.WriteString("\n... (truncated)")
			} else {
				configContents.Write(content)
			}
		}
	}

	if len(foundConfigs) == 0 {
		return &ProjectFramework{Language: "Unknown", Framework: "Unknown"}, nil
	}

	systemPrompt := `You are a framework analyzer.
Based on the provided dependency files, identify the programming language and primary web/API framework used in the project.
Output your response STRICTLY as a JSON object matching this schema:
{
  "language": "Go|Python|JavaScript|TypeScript|Java|PHP|Ruby|Rust|etc",
  "framework": "Gin|Echo|Express|Django|FastAPI|Spring|etc"
}
Do not include markdown blocks, just the raw JSON string.`

	messages := []agent.ChatMessage{
		{
			Role:    "user",
			Content: fmt.Sprintf("Dependency files found:\n%s", configContents.String()),
		},
	}

	response, err := s.baseAgent.Chat(systemPrompt, nil, messages, false)
	if err != nil {
		return nil, fmt.Errorf("failed to detect framework via LLM: %w", err)
	}

	var framework ProjectFramework
	cleanJSON := strings.TrimPrefix(strings.TrimSpace(response.Message), "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")

	if err := json.Unmarshal([]byte(cleanJSON), &framework); err != nil {
		// Fallback if LLM fails formatting
		return &ProjectFramework{Language: "Unknown", Framework: "Unknown"}, nil
	}

	if progressCallback != nil {
		progressCallback(fmt.Sprintf("  ↳ Detected: %s + %s", framework.Language, framework.Framework))
	}

	return &framework, nil
}
