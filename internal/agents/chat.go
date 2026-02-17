package agent

import (
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/llm/common"
)

type ChatMessage struct {
	Role             string                `json:"role"`
	Content          string                `json:"content"`
	ReasoningContent string                `json:"reasoning_content,omitempty"`
	FunctionCalls    []ToolCall            `json:"function_calls,omitempty"`
	FunctionResponse *FunctionResponseData `json:"function_response,omitempty"`
	InputTokens      int64                 `json:"input_tokens,omitempty"`
	OutputTokens     int64                 `json:"output_tokens,omitempty"`
}

type FunctionResponseData struct {
	ID       string         `json:"id"` // tool_use_id from original tool use block
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type ChatResponse struct {
	Message      string     `json:"message"`
	Reasoning    string     `json:"reasoning,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	TokensUsed   int64      `json:"tokens_used,omitempty"`   // Total tokens (input + output)
	InputTokens  int64      `json:"input_tokens,omitempty"`  // Input tokens used
	OutputTokens int64      `json:"output_tokens,omitempty"` // Output tokens used
}

type ToolCall struct {
	ID               string         `json:"id,omitempty"`
	Name             string         `json:"name"`
	Arguments        map[string]any `json:"arguments"`
	ThoughtSignature string         `json:"thought_signature,omitempty"` // base64 encoded []byte (not used by Claude)
}

// ReasoningCallback is called for each chunk as it's streamed
// isThought=true for reasoning chunks, false for text chunks
type ReasoningCallback func(chunk string, isThought bool)

// getMainAgentTools returns the tools for the main agent
func getMainAgentTools() []common.Tool {
	return []common.Tool{
		{
			Name:        "get_endpoints_details",
			Description: "Get detailed information about specified endpoints including description, parameters, security, request body, and responses.",
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"endpoints": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]any{
								"path": map[string]any{
									"type":        "string",
									"description": "Endpoint path (e.g., /users)",
								},
								"method": map[string]any{
									"type":        "string",
									"description": "HTTP method (GET, POST, PUT, DELETE, PATCH)",
								},
							},
							"required": []string{"path", "method"},
						},
					},
				},
				"required": []string{"endpoints"},
			},
		},
		{
			Name:        "GenerateTestPlan",
			Description: "Generate test cases for API endpoints. Describe endpoints with all relevant details from get_endpoints_details.",
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"what": map[string]any{
						"type":        "string",
						"description": "Detailed endpoint description including: method, path, what it does, authentication requirements (Security field), request body schema, expected responses, parameters. Be thorough!",
					},
					"focus": map[string]any{
						"type":        "string",
						"description": "Testing focus: 'happy path' (basic success), 'authentication' (with/without auth), 'error handling' (validation, 404, etc), 'all aspects' (comprehensive)",
					},
				},
				"required": []string{"what", "focus"},
			},
		},
		{
			Name:        "ExecuteTestGroup",
			Description: "Execute a group of tests against the API. Tests are run locally by the CLI and results are returned. Call this AFTER GenerateTestPlan to actually run the tests.",
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"tests": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]any{
								"method": map[string]any{
									"type":        "string",
									"description": "HTTP method (GET, POST, PUT, DELETE, etc)",
								},
								"endpoint": map[string]any{
									"type":        "string",
									"description": "API endpoint path (e.g., /api/health)",
								},
								"headers": map[string]any{
									"type":                 []any{"object", "null"},
									"additionalProperties": false,
									"description":          "Optional HTTP headers",
								},
								"body": map[string]any{
									"type":        []any{"string", "null"},
									"description": "Optional request body (JSON string)",
								},
								"requires_auth": map[string]any{
									"type":        "boolean",
									"description": "Whether authentication is required for this test",
								},
							},
							"required": []string{"method", "endpoint", "headers", "body", "requires_auth"},
						},
					},
				},
				"required": []string{"tests"},
			},
		},
		{
			Name:        "GenerateReport",
			Description: "Generate a PDF report from test results. Call this AFTER tests have been executed to create a professional report. Write the report content in Markdown format — it will be converted to a styled PDF. Include: title, summary, test results table (method, endpoint, status, duration), and analysis.",
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"report_content": map[string]any{
						"type":        "string",
						"description": "Full report content in Markdown format. Use headers, tables, lists, and code blocks for a professional layout. Include: report title, test summary (total/passed/failed), detailed results table, and analysis/recommendations.",
					},
					"file_name": map[string]any{
						"type":        "string",
						"description": "Optional output file name for the PDF (e.g., 'api-test-report.pdf'). If not provided, a timestamped name will be used.",
					},
				},
				"required": []string{"report_content"},
			},
		},
	}
}

func (a *Agent) Chat(messages []ChatMessage, thinkingEnabled bool, endpointsList ...string) (*ChatResponse, error) {
	systemPrompt := buildSystemPrompt(a.baseURL, endpointsList...)
	tools := getMainAgentTools()
	return a.baseAgent.Chat(systemPrompt, tools, messages, thinkingEnabled)
}

func buildSystemPrompt(baseURL string, endpointsList ...string) string {
	endpointsInfo := ""
	if len(endpointsList) > 0 && endpointsList[0] != "" {
		endpointsInfo = fmt.Sprintf(`

# Available Endpoints
%s
`, endpointsList[0])
	}

	return fmt.Sprintf(`# Purpose
API testing assistant that helps users explore endpoints, generate tests, and analyze results.

# Constraints
- Be proactive - minimize clarifications
- Use ONE tool per response
- Default to "happy path" tests unless user specifies otherwise

# Available Context
Base URL: %s%s

HTTP methods: GET, POST, PUT, DELETE, PATCH
Authentication: Bearer token (JWT), Basic auth, API Key header
Test focus: "happy path" | "authentication" | "error handling" | "all aspects"

# Available Tools

## get_endpoints_details
Fetch detailed specs (params, auth, schemas).
Use when: need technical details for response/tests or user asks about endpoint behavior.

## GenerateTestPlan
Generate test cases for API endpoints.
Parameters:
- what: endpoint details from get_endpoints_details
- focus: default "happy path", or user's choice

## ExecuteTestGroup
Execute a group of tests against the API. Call AFTER GenerateTestPlan.

## GenerateReport
Generate PDF report from test results. Call AFTER tests are executed.

# Behavior
- User mentions endpoint (e.g., "users", "auth") → fetch details, show info OR generate tests
- User says "test X" → fetch details, generate & run tests
- User says "list endpoints" → show list from available endpoints (no tool call)
- requires_auth=true → CLI adds auth header, requires_auth=false → no auth`, baseURL, endpointsInfo)
}

func (a *Agent) ChatStream(messages []ChatMessage, thinkingEnabled bool, callback ReasoningCallback, endpointsList ...string) (*ChatResponse, error) {
	systemPrompt := buildSystemPrompt(a.baseURL, endpointsList...)
	tools := getMainAgentTools()
	return a.baseAgent.ChatStream(systemPrompt, tools, messages, thinkingEnabled, callback)
}
