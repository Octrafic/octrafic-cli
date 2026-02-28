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
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type ChatResponse struct {
	Message      string     `json:"message"`
	Reasoning    string     `json:"reasoning,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	TokensUsed   int64      `json:"tokens_used,omitempty"`
	InputTokens  int64      `json:"input_tokens,omitempty"`
	OutputTokens int64      `json:"output_tokens,omitempty"`
}

type ToolCall struct {
	ID               string         `json:"id,omitempty"`
	Name             string         `json:"name"`
	Arguments        map[string]any `json:"arguments"`
	ThoughtSignature string         `json:"thought_signature,omitempty"`
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
								"expected_status": map[string]any{
									"type":        "integer",
									"description": "Expected HTTP status code. Set correctly: 201 for POST creating resources, 204 for DELETE, 400 for bad input, 401 for unauthorized, 404 for not found.",
								},
								"extract": map[string]any{
									"type":        []any{"array", "null"},
									"description": "Extract values from response body for use in later tests. Each item: {\"field\": \"id\", \"as\": \"user_id\"}. Use dot notation for nested fields: \"data.token\", \"items.0.id\".",
									"items": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"field": map[string]any{"type": "string", "description": "Dot-path to field in response JSON"},
											"as":    map[string]any{"type": "string", "description": "Variable name to store value as"},
										},
										"required": []string{"field", "as"},
									},
								},
								"assertions": map[string]any{
									"type":        []any{"array", "null"},
									"description": "Assert specific values in the response body. Each item: {\"field\": \"name\", \"op\": \"eq\", \"value\": \"Alice\"}. Operators: eq, neq, exists, not_exists, contains, gt, gte, lt, lte.",
									"items": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"field": map[string]any{"type": "string", "description": "Dot-path to field in response JSON"},
											"op":    map[string]any{"type": "string", "description": "Operator: eq, neq, exists, not_exists, contains, gt, gte, lt, lte"},
											"value": map[string]any{"description": "Expected value (omit for exists/not_exists)"},
										},
										"required": []string{"field", "op"},
									},
								},
							},
							"required": []string{"method", "endpoint", "headers", "body", "requires_auth", "expected_status"},
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
		{
			Name:        "ExportTests",
			Description: "Export API tests to files in the specified formats. Can export either executed tests or generated test plans (even if they haven't been executed yet). Call this ONLY when the user explicitly requests to save/export. Can export to multiple formats at once.",
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"exports": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]any{
								"format": map[string]any{
									"type":        "string",
									"enum":        []string{"postman", "pytest", "sh"},
									"description": "Export format: 'postman' for Postman Collection v2.1 (JSON), 'pytest' for Python tests (.py), 'sh' for bash script with curl commands",
								},
								"filepath": map[string]any{
									"type":        "string",
									"description": "Output file path (e.g., 'tests.json', 'test_api.py', 'api-tests.sh'). Can be relative or absolute.",
								},
							},
							"required": []string{"format", "filepath"},
						},
						"description": "List of export configurations. Each entry specifies a format and output filepath.",
					},
				},
				"required": []string{"exports"},
			},
		},
		{
			Name:        "wait",
			Description: "Wait for a specified number of seconds before proceeding. Use this when you receive a 429 rate limit response or a Retry-After header to pause before retrying.",
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"seconds": map[string]any{
						"type":        "integer",
						"description": "Number of seconds to wait (1-60)",
						"minimum":     1,
						"maximum":     60,
					},
					"reason": map[string]any{
						"type":        "string",
						"description": "Why you are waiting (e.g., 'Rate limit hit, Retry-After: 10')",
					},
				},
				"required": []string{"seconds"},
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
Response includes per test: status_code, response_body, duration_ms, passed, schema_valid, schema_errors, assertions_passed, assertion_failures.
- passed=false → status code did not match expected
- schema_valid=false → response body does not match the OpenAPI schema (even if passed=true)
- assertions_passed=false → one or more assertions failed
- Always report schema_errors and assertion_failures to the user
- expected_status is REQUIRED — set correctly: 200 GET, 201 POST create, 204 DELETE, 400 bad input, 401 unauthorized, 404 not found

### Chaining tests with extract
Use extract to capture values from a response and reuse them in later tests via {{var_name}}:
{"method":"POST","endpoint":"/users","body":"{\"name\":\"Alice\"}","expected_status":201,"extract":[{"field":"id","as":"user_id"}],...}
{"method":"GET","endpoint":"/users/{{user_id}}","expected_status":200,...}
{"method":"DELETE","endpoint":"/users/{{user_id}}","expected_status":204,...}

### Asserting response values
Use assertions to verify specific fields in the response body:
{"field":"name","op":"eq","value":"Alice"} — exact match
{"field":"id","op":"exists"} — field must be present
{"field":"age","op":"gte","value":18} — numeric comparison
{"field":"token","op":"contains","value":"Bearer"} — substring
Operators: eq, neq, exists, not_exists, contains, gt, gte, lt, lte

## wait
Wait N seconds before proceeding. Use when:
- You receive a 429 status code
- Response headers contain Retry-After
- You want to avoid hitting rate limits between test groups

## GenerateReport
Generate PDF report from test results.
IMPORTANT: Only call this when the user explicitly asks for a report (e.g. "generate report", "save report", "export PDF"). Never call it automatically after tests.

# Behavior
- User mentions endpoint (e.g., "users", "auth") → fetch details, show info OR generate tests
- User says "test X" → fetch details, generate & run tests
- User says "list endpoints" → show list from available endpoints (no tool call)
- User says "generate report" / "save PDF" / "export report" → call GenerateReport
- After 429 response → call wait(seconds=N) where N comes from Retry-After header or default to 5
- requires_auth=true → CLI adds auth header, requires_auth=false → no auth`, baseURL, endpointsInfo)
}

func (a *Agent) ChatStream(messages []ChatMessage, thinkingEnabled bool, callback ReasoningCallback, endpointsList ...string) (*ChatResponse, error) {
	systemPrompt := buildSystemPrompt(a.baseURL, endpointsList...)
	tools := getMainAgentTools()
	return a.baseAgent.ChatStream(systemPrompt, tools, messages, thinkingEnabled, callback)
}
