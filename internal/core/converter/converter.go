package converter

import (
	"github.com/Octrafic/octrafic-cli/internal/config"
	"github.com/Octrafic/octrafic-cli/internal/llm"
	"github.com/Octrafic/octrafic-cli/internal/llm/common"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const conversionPrompt = `Convert the following API specification to OpenAPI 3.0 JSON format.

Source format: %s
Source file content:
---
%s
---

Requirements:
1. Output ONLY valid JSON - no markdown, no code blocks, no explanations
2. Follow OpenAPI 3.0.x specification strictly
3. Preserve all endpoints, methods, parameters, and response schemas
4. If some information is missing, make reasonable assumptions
5. Include proper paths, methods, parameters, request bodies, and responses

Output the complete OpenAPI 3.0 JSON specification:`

// ConvertToOpenAPI converts a non-OpenAPI spec file to OpenAPI format using LLM
func ConvertToOpenAPI(specPath string, detectedFormat string) (string, error) {
	// Load app config
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.APIKey == "" {
		return "", fmt.Errorf("no API key configured - run octrafic to complete onboarding")
	}

	// Read source file
	content, err := os.ReadFile(specPath)
	if err != nil {
		return "", fmt.Errorf("failed to read spec file: %w", err)
	}

	// Create LLM provider
	providerConfig := common.ProviderConfig{
		Provider: cfg.Provider,
		APIKey:   cfg.APIKey,
		Model:    cfg.Model,
		Timeout:  120 * time.Second,
	}

	provider, err := llm.CreateProvider(providerConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create LLM provider: %w", err)
	}
	defer func() { _ = provider.Close() }()

	// Prepare conversion prompt
	prompt := fmt.Sprintf(conversionPrompt, detectedFormat, string(content))

	messages := []common.Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// Send to LLM
	response, err := provider.Chat(messages, nil, false)
	if err != nil {
		return "", fmt.Errorf("LLM conversion failed: %w", err)
	}

	// Extract JSON from response
	jsonContent := extractJSON(response.Message)
	if jsonContent == "" {
		return "", fmt.Errorf("LLM did not return valid JSON")
	}

	// Generate output path
	dir := filepath.Dir(specPath)
	baseName := strings.TrimSuffix(filepath.Base(specPath), filepath.Ext(specPath))
	outputPath := filepath.Join(dir, baseName+".openapi.json")

	// Write converted file
	if err := os.WriteFile(outputPath, []byte(jsonContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write converted spec: %w", err)
	}

	return outputPath, nil
}

// extractJSON extracts JSON content from LLM response
func extractJSON(response string) string {
	response = strings.TrimSpace(response)

	// Remove markdown code blocks if present
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	// Find JSON object bounds
	start := strings.Index(response, "{")
	if start == -1 {
		return ""
	}

	// Find matching closing brace
	depth := 0
	end := -1
	for i := start; i < len(response); i++ {
		switch response[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
			}
		}
		if end != -1 {
			break
		}
	}

	if end == -1 {
		return ""
	}

	return response[start:end]
}
