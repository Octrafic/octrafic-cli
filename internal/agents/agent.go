package agent

import (
	"encoding/json"
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/config"
	"github.com/Octrafic/octrafic-cli/internal/infra/logger"
	"github.com/Octrafic/octrafic-cli/internal/llm"
	"github.com/Octrafic/octrafic-cli/internal/llm/common"
	"os"
	"strings"
)

const (
	// Spec processing constants
	MaxIterations = 10 // Maximum iterations for spec processing
)

type Agent struct {
	baseAgent *BaseAgent
	baseURL   string
}

type TestStatus string

const (
	StatusPending TestStatus = "pending"
	StatusRunning TestStatus = "running"
	StatusPassed  TestStatus = "passed"
	StatusFailed  TestStatus = "failed"
)

type Test struct {
	TestCase TestCase   `json:"test_case"`
	Status   TestStatus `json:"status"`
	Analysis string     `json:"analysis,omitempty"`
}

func extractJSONFromMarkdown(response string) string {
	if strings.Contains(response, "```json") {
		start := strings.Index(response, "```json")
		if start != -1 {
			start += 7 // Length of "```json"
			end := strings.Index(response[start:], "```")
			if end != -1 {
				return strings.TrimSpace(response[start : start+end])
			}
		}
	} else if strings.Contains(response, "```") {
		start := strings.Index(response, "```")
		if start != -1 {
			start += 3 // Length of "```"
			end := strings.Index(response[start:], "```")
			if end != -1 {
				return strings.TrimSpace(response[start : start+end])
			}
		}
	}
	return strings.TrimSpace(response)
}

func NewAgent(baseURL string) (*Agent, error) {
	// Try loading config from file first (onboarding users)
	cfg, err := config.Load()
	if err == nil && cfg.Onboarded && (cfg.APIKey != "" || config.IsLocalProvider(cfg.Provider)) {
		// Use config from file
		logger.Info("Using LLM config from onboarding",
			logger.String("provider", cfg.Provider),
			logger.String("model", cfg.Model))

		providerConfig := common.ProviderConfig{
			Provider: cfg.Provider,
			APIKey:   cfg.APIKey,
			BaseURL:  cfg.BaseURL,
			Model:    cfg.Model,
		}

		llmProvider, err := llm.CreateProvider(providerConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider: %w", err)
		}

		return &Agent{
			baseAgent: NewBaseAgent(llmProvider),
			baseURL:   baseURL,
		}, nil
	}

	// Fallback to environment variables with OCTRAFIC_ prefix
	provider := config.GetEnv("PROVIDER")
	if provider == "" {
		provider = "claude" // Default to Claude
	}

	apiKey := config.GetEnv("API_KEY")
	if apiKey == "" {
		// Legacy fallback for backwards compatibility
		if provider == "openai" || provider == "openrouter" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		} else {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
	}

	providerConfig := common.ProviderConfig{
		Provider: provider,
		APIKey:   apiKey,
		BaseURL:  config.GetEnv("BASE_URL"),
		Model:    config.GetEnv("MODEL"),
	}

	// Create provider
	llmProvider, err := llm.CreateProvider(providerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	logger.Info("Using LLM provider", logger.String("provider", provider))
	return &Agent{
		baseAgent: NewBaseAgent(llmProvider),
		baseURL:   baseURL,
	}, nil
}

func (a *Agent) GenerateTestPlan(what, focus string) ([]Test, int64, error) {
	prompt := BuildTestPlanPrompt(what, focus)

	systemPrompt := `# Purpose
Generate API test cases from user request.

# Constraints
- Maximum 10 test cases per request
- Each test must include: method, endpoint, headers (optional), body (optional), requires_auth
- Always use provided endpoint details for accurate testing

# Available Context
HTTP methods: GET, POST, PUT, DELETE, PATCH
Authentication: Bearer token (JWT), Basic auth, API Key header
Test focus options: "happy path", "authentication", "error handling", "all aspects"

# Output Format
Return a JSON object with tests array. Each test:
{
  "method": "GET",
  "endpoint": "/users",
  "body": null,
  "requires_auth": true
}

Return pure JSON only - no markdown, no comments.`
	messages := []ChatMessage{
		{Role: "user", Content: prompt},
	}

	response, err := a.baseAgent.Chat(systemPrompt, nil, messages, false)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to generate test plan: %w", err)
	}

	jsonResponse := extractJSONFromMarkdown(response.Message)

	var testPlan TestPlan
	if err := json.Unmarshal([]byte(jsonResponse), &testPlan); err != nil {
		logger.Error("Failed to parse JSON",
			logger.Err(err),
			logger.String("raw_response", response.Message))
		return nil, 0, fmt.Errorf("failed to parse test plan: %w", err)
	}

	tests := make([]Test, len(testPlan.Tests))
	for i, tc := range testPlan.Tests {
		tests[i] = Test{
			TestCase: tc,
			Status:   StatusPending,
		}
	}

	return tests, response.TokensUsed, nil
}

func (a *Agent) ProcessSpecification(rawContent string, baseURL string) ([]APIEndpoint, error) {
	prompt := fmt.Sprintf(`Role: API analyst
Goal: Extract ALL endpoints from specification in one pass

Base URL: %s

FULL SPECIFICATION:
%s

# Required Fields
- method (GET, POST, PUT, DELETE, PATCH)
- path ("/users", "/api/health")
- description (brief, clear)
- requires_auth (boolean)
- auth_type ("bearer" | "apikey" | "basic" | "none")

# Auth Detection

Priority 1 - OpenAPI security field:
- Present + non-empty → requires_auth=true
- Empty array [] → requires_auth=false
- Missing → use fallback

Priority 2 - Fallback:
TRUE: POST/PUT/DELETE/PATCH, /users /auth /admin /account /api-keys
FALSE: Public GET, /health /status /ping /version

auth_type: bearer (JWT) | apikey (header) | basic (HTTP) | none (public)

# Output Format

JSONL - one JSON object per line:
{"method":"GET","path":"/users","description":"List users","requires_auth":true,"auth_type":"bearer"}
{"method":"POST","path":"/users","description":"Create user","requires_auth":true,"auth_type":"bearer"}

Requirements:
- One object per line
- No array brackets
- No markdown, comments, or whitespace-only lines
- No duplicate (method, path) pairs
- Pure JSON objects only`,
		baseURL,
		rawContent,
	)

	systemPrompt := `# Purpose
Extract ALL API endpoints from specification in a single pass.

# Constraints
- Process entire specification at once
- No duplicate (method, path) pairs
- Auth detection priority: security field > path/method heuristics

# Available Context
HTTP methods: GET, POST, PUT, DELETE, PATCH
Auth types: "bearer" (JWT), "apikey" (X-API-Key), "basic" (HTTP Basic), "none" (public)
Required fields per endpoint: method, path, description, requires_auth, auth_type

Auth detection rules:
- Security field present + non-empty → requires_auth=true
- Security field empty array [] → requires_auth=false
- Fallback: POST/PUT/DELETE/PATCH, /users /auth /admin /account → auth required
- Fallback: GET /health /status /ping /version → public

# Output Format
JSONL - one JSON object per line, no array brackets:
{"method":"GET","path":"/users","description":"List users","requires_auth":true,"auth_type":"bearer"}
{"method":"POST","path":"/users","description":"Create user","requires_auth":true,"auth_type":"bearer"}

Requirements:
- One object per line
- No markdown, no comments
- Pure JSON objects only`
	messages := []ChatMessage{
		{Role: "user", Content: prompt},
	}

	chatResponse, err := a.baseAgent.Chat(systemPrompt, nil, messages, false)
	if err != nil {
		return nil, fmt.Errorf("failed to process specification: %w", err)
	}

	var endpoints []APIEndpoint
	lines := strings.Split(strings.TrimSpace(chatResponse.Message), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var endpoint APIEndpoint
		if err := json.Unmarshal([]byte(line), &endpoint); err != nil {
			if strings.Contains(line, "```") {
				continue
			}
			logger.Warn("Failed to parse endpoint line",
				logger.String("line", line),
				logger.Err(err))
			continue
		}
		endpoints = append(endpoints, endpoint)
	}

	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints found in response")
	}

	return endpoints, nil
}
