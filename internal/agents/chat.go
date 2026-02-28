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

// getMainAgentTools returns the tools for the main agent.
// Delegates to the centralized tool registry.
func getMainAgentTools() []common.Tool {
	return GetToolDefinitions()
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
