package agent

import "github.com/Octrafic/octrafic-cli/internal/llm/common"

// Tool name constants — the single source of truth for all tool names.
// Use these throughout the codebase instead of raw strings to prevent
// typos and make "find all references" work reliably.
const (
	ToolGetEndpointsDetails = "get_endpoints_details"
	ToolGenerateTestPlan    = "GenerateTestPlan"
	ToolExecuteTestGroup    = "ExecuteTestGroup"
	ToolExecuteTest         = "ExecuteTest" // internal, dispatched inside ExecuteTestGroup
	ToolExportTests         = "ExportTests"
	ToolGenerateReport      = "GenerateReport"
	ToolWait                = "wait"
)

// ToolMeta holds the LLM-facing definition together with UI display hints.
type ToolMeta struct {
	// Definition is sent to the LLM provider as a tool schema.
	Definition common.Tool

	// WidgetTitle is the short label shown in the TUI spinner widget
	// while the tool is executing (e.g. "Generating PDF report").
	WidgetTitle string
}

// registry is the ordered list of tools presented to the LLM.
// Order matters — the LLM sees them in this sequence.
var registry = []ToolMeta{
	{
		WidgetTitle: "Getting endpoint details",
		Definition: common.Tool{
			Name:        ToolGetEndpointsDetails,
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
	},
	{
		WidgetTitle: "Generating test plan",
		Definition: common.Tool{
			Name:        ToolGenerateTestPlan,
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
	},
	{
		WidgetTitle: "Executing tests",
		Definition: common.Tool{
			Name:        ToolExecuteTestGroup,
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
	},
	{
		WidgetTitle: "Generating PDF report",
		Definition: common.Tool{
			Name:        ToolGenerateReport,
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
	},
	{
		WidgetTitle: "Exporting tests",
		Definition: common.Tool{
			Name:        ToolExportTests,
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
	},
	{
		WidgetTitle: "Waiting",
		Definition: common.Tool{
			Name:        ToolWait,
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
	},
}

// GetToolDefinitions returns the LLM-facing tool definitions.
func GetToolDefinitions() []common.Tool {
	defs := make([]common.Tool, len(registry))
	for i, tm := range registry {
		defs[i] = tm.Definition
	}
	return defs
}

// GetToolMeta returns the ToolMeta for a given tool name, or nil if not found.
func GetToolMeta(name string) *ToolMeta {
	for i := range registry {
		if registry[i].Definition.Name == name {
			return &registry[i]
		}
	}
	return nil
}
