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
	SpecPreviewLines = 30 // Number of lines to show in spec preview
	MaxIterations    = 10 // Maximum iterations for spec processing
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

type ProcessSpecResult struct {
	Done      bool
	Endpoints []APIEndpoint
	ToolCalls []ToolCall
	Messages  []ChatMessage
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

	systemPrompt := "Role: Test case generator. Rules: Create minimal tests matching user request. Return pure JSON only - no markdown, no comments."
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

func (a *Agent) ProcessSpecificationIterative(rawContent string, baseURL string, messages []ChatMessage, toolResults []map[string]interface{}) (*ProcessSpecResult, error) {
	if len(messages) == 0 {
		lines := strings.Split(rawContent, "\n")
		previewLines := SpecPreviewLines
		if len(lines) < previewLines {
			previewLines = len(lines)
		}

		preview := strings.Join(lines[:previewLines], "\n") + "\n\n... [CONTENT TRUNCATED - Use SearchSpec to see more] ..."
		totalLines := len(lines)

		prompt := fmt.Sprintf(`Role: API specification analyst
Goal: Extract ALL endpoints with complete metadata

Base URL: %s
Spec: %d lines total, %d lines visible

SPEC PREVIEW:
%s

%s

# Required Fields
Extract for each endpoint:
- method (GET, POST, PUT, DELETE, PATCH)
- path (e.g., "/users", "/api/health")
- description (brief, clear)
- requires_auth (boolean)
- auth_type ("bearer" | "apikey" | "basic" | "none")

# Auth Detection

Priority 1 - Security field:
- Present + non-empty → requires_auth=true
- Empty array [] → requires_auth=false
- Missing → use fallback

Priority 2 - Fallback heuristics:
TRUE: POST/PUT/DELETE, paths: /users /auth /admin /account /api-keys
FALSE: Public GET, /health /status /ping, public docs

auth_type:
- bearer (most common, JWT)
- apikey (X-API-Key header)
- basic (HTTP Basic)
- none (when requires_auth=false)

# Search Strategy

The spec is indexed - use SearchSpec tool:
1. Search HTTP methods: "GET", "POST", "PUT", "DELETE", "PATCH"
2. Search keywords: "users", "auth", "login", "create", "update"
3. Search terms: "api", "endpoint", "path", "route"
4. Continue until no new results

# Output Format

JSON array, sorted by path then method:
[
  {"method":"GET","path":"/health","description":"Health check","requires_auth":false,"auth_type":"none"},
  {"method":"GET","path":"/users","description":"List users","requires_auth":true,"auth_type":"bearer"}
]

START by using SearchSpec tool to find all endpoints!`,
			baseURL,
			totalLines,
			previewLines,
			preview,
			func() string {
				if totalLines > previewLines {
					remaining := totalLines - previewLines
					return fmt.Sprintf("⚠️ Only %d lines visible. %d MORE lines to search!", previewLines, remaining)
				}
				return ""
			}(),
		)

		messages = []ChatMessage{
			{Role: "system", Content: "Role: API specification analyst. Goal: Extract ALL endpoints accurately with complete metadata. Use SearchSpec tool to explore large specifications efficiently."},
			{Role: "user", Content: prompt},
		}
	}

	if len(toolResults) > 0 {
		var toolCallIDs []string
		for i := len(messages) - 1; i >= 0; i-- {
			if len(messages[i].FunctionCalls) > 0 {
				for _, fc := range messages[i].FunctionCalls {
					toolCallIDs = append(toolCallIDs, fc.ID)
				}
				break
			}
		}

		for i, result := range toolResults {
			toolID := "unknown"
			if i < len(toolCallIDs) {
				toolID = toolCallIDs[i]
			}
			messages = append(messages, ChatMessage{
				Role: "user",
				FunctionResponse: &FunctionResponseData{
					ID:       toolID, // tool_use_id from original tool use block
					Name:     "SearchSpec",
					Response: result,
				},
			})
		}
	}

	tools := []common.Tool{
		{
			Name:        "SearchSpec",
			Description: "Search the API specification using full-text search to find endpoints, methods, parameters, or other information. The index contains parsed endpoint data with methods, paths, descriptions, parameters, and more.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query - can be keywords, endpoint paths, HTTP methods, or natural language (e.g., 'users', 'authentication', 'POST /login', 'create user endpoint')",
					},
				},
				"required": []string{"query"},
			},
		},
	}

	systemPrompt := "Role: API specification analyst. Goal: Extract ALL endpoints accurately with complete metadata. Use SearchSpec tool to explore large specifications efficiently."

	chatResponse, err := a.baseAgent.Chat(systemPrompt, tools, messages, false)
	if err != nil {
		return nil, fmt.Errorf("failed to process specification: %w", err)
	}

	if len(chatResponse.ToolCalls) > 0 {
		var toolCalls []ToolCall
		var functionCallsData []ToolCall

		for _, fc := range chatResponse.ToolCalls {
			toolCalls = append(toolCalls, ToolCall{
				ID:        fc.ID,
				Name:      fc.Name,
				Arguments: fc.Arguments,
			})
			functionCallsData = append(functionCallsData, ToolCall{
				ID:        fc.ID,
				Name:      fc.Name,
				Arguments: fc.Arguments,
			})
		}

		messages = append(messages, ChatMessage{
			Role:          "model",
			Content:       chatResponse.Message,
			FunctionCalls: functionCallsData,
		})

		return &ProcessSpecResult{
			Done:      false,
			ToolCalls: toolCalls,
			Messages:  messages,
		}, nil
	}

	jsonResponse := extractJSONFromMarkdown(chatResponse.Message)

	var endpoints []APIEndpoint
	if err := json.Unmarshal([]byte(jsonResponse), &endpoints); err != nil {
		logger.Error("Failed to parse endpoints",
			logger.Err(err),
			logger.String("raw_response", chatResponse.Message))
		return nil, fmt.Errorf("failed to parse endpoints: %w", err)
	}

	return &ProcessSpecResult{
		Done:      true,
		Endpoints: endpoints,
		Messages:  messages,
	}, nil
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

	systemPrompt := "Role: API analyst. Rules: Extract endpoints accurately. Return JSONL (one JSON object per line)."
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
