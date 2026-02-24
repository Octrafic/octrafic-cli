package scanner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Octrafic/octrafic-cli/internal/agents"
	"github.com/Octrafic/octrafic-cli/internal/infra/logger"
)

// EndpointDef represents a single scraped OpenAPI endpoint
type EndpointDef struct {
	Method       string         `json:"method"`
	Path         string         `json:"path"`
	Summary      string         `json:"summary"`
	RequestBody  map[string]any `json:"requestBody,omitempty"`
	ResponseBody map[string]any `json:"responseBody,omitempty"`
}

// FileExtractResult holds the endpoints found in one specific file
type FileExtractResult struct {
	File      string
	Endpoints []EndpointDef
	Error     error
}

// extractEndpoints spawns parallel LLM calls for each routing file to extract specific endpoints.
func (s *Scanner) extractEndpoints(routingFiles []RouterFile, progressCallback func(string)) map[string][]EndpointDef {
	if progressCallback != nil {
		progressCallback("➔ Extracting endpoints...")
	}

	results := make(map[string][]EndpointDef)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Semaphore to limit massive parallel LLM calls (e.g. max 5 concurrent)
	sem := make(chan struct{}, 5)

	start := time.Now()

	for i, routeFile := range routingFiles {
		wg.Add(1)

		go func(idx int, rf RouterFile) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			if progressCallback != nil {
				progressCallback(fmt.Sprintf("  ↳ Scanning %s...", rf.Path))
			}

			fileResults, err := s.extractFromFile(rf.Path)
			if err != nil {
				logger.Warn("Failed to extract endpoints from file", logger.String("file", rf.Path), logger.Err(err))
				return
			}

			if len(fileResults) > 0 {
				mu.Lock()
				results[rf.Path] = fileResults
				mu.Unlock()

				if progressCallback != nil {
					progressCallback(fmt.Sprintf("    ✓ Extracted %d endpoints", len(fileResults)))
				}
			}
		}(i, routeFile)
	}

	wg.Wait()

	if progressCallback != nil {
		total := 0
		for _, eps := range results {
			total += len(eps)
		}
		progressCallback(fmt.Sprintf("    ✓ Found %d total endpoints in %s", total, time.Since(start).Round(time.Millisecond)))
	}

	return results
}

func (s *Scanner) extractFromFile(relPath string) ([]EndpointDef, error) {
	absPath := filepath.Join(s.dir, relPath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %w", err)
	}

	systemPrompt := `You are an expert OpenAPI endpoint extractor.
Analyze the provided source code file and extract ALL API endpoints defined inside it.
Focus heavily on HTTP methods, route paths, request payloads (structs/classes parsed from body), and response structures.

Output your response STRICTLY as a JSON array matching this schema exactly:
[
  {
    "method": "GET",
    "path": "/api/users/{id}",
    "summary": "Fetches a user by ID",
    "requestBody": null,
    "responseBody": {
		"id": "string",
		"name": "string"
	}
  }
]

Do not include markdown blocks, explanations, or any other text. Output ONLY the raw JSON array.
If no explicit endpoints are found in the file, output an empty array: []`

	messages := []agent.ChatMessage{
		{
			Role:    "user",
			Content: fmt.Sprintf("File Path: %s\n\n```\n%s\n```", relPath, string(content)),
		},
	}

	response, err := s.baseAgent.Chat(systemPrompt, nil, messages, false)
	if err != nil {
		return nil, fmt.Errorf("agent chat failed: %w", err)
	}

	cleanJSON := strings.TrimPrefix(strings.TrimSpace(response.Message), "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	if cleanJSON == "" || cleanJSON == "[]" {
		return []EndpointDef{}, nil
	}

	var endpoints []EndpointDef
	if err := json.Unmarshal([]byte(cleanJSON), &endpoints); err != nil {
		return nil, fmt.Errorf("failed to parse extracted JSON: %w\nJSON was: %s", err, cleanJSON)
	}

	return endpoints, nil
}
