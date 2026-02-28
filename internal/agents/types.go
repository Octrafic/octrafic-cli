package agent

import (
	"fmt"
)

type APIEndpoint struct {
	Method       string `json:"method"`
	Path         string `json:"path"`
	Description  string `json:"description"`
	RequiresAuth bool   `json:"requires_auth"`
	AuthType     string `json:"auth_type"`
}

type TestPlan struct {
	Tests []TestCase `json:"tests"`
}

type TestCase struct {
	ID             int               `json:"id"`
	Description    string            `json:"description"`
	Method         string            `json:"method"`
	Endpoint       string            `json:"endpoint"`
	Headers        map[string]string `json:"headers,omitempty"`
	Body           interface{}       `json:"body,omitempty"`
	ExpectedStatus int               `json:"expected_status"`
	Reasoning      string            `json:"reasoning"`
	RequiresAuth   bool              `json:"requires_auth"`
}

// BuildTestPlanPrompt generates tests based on detailed endpoint description
// The main agent uses SearchSpec to gather full endpoint details and passes them in 'what'
func BuildTestPlanPrompt(what, focus string) string {
	return fmt.Sprintf(`Role: Precise test case generator
Goal: Create minimal, focused test cases for specified endpoints

ENDPOINTS TO TEST:
%s

TEST FOCUS: %s

CRITICAL RULES:
1. Test ONLY endpoints above - no additions
2. Use Security field to determine requires_auth:
   - Security: [] or empty → false
   - Security: [{"bearer":[]}] → true
3. Use Request Body for expected data
4. Use Responses for expected_status
5. Generate tests per focus level:
   - "happy path" → 1 test (success)
   - "authentication" → 2 tests (with/without auth)
   - "error handling" → 1-2 tests (validation, 404)
   - "all aspects" → 2-3 tests (combined)

# Authentication

requires_auth in endpoint = what spec requires
requires_auth in test = whether to send auth

Test scenarios:
| Endpoint Auth | Test Auth | CLI Behavior | Expected |
|--------------|-----------|--------------|----------|
| true         | true      | Adds auth    | 200-299   |
| true         | false     | No auth      | 401       |
| false        | false     | No auth      | 200-299   |

# Output Format

Pure JSON, no markdown:
{
  "tests": [
    {
      "id": 1,
      "description": "Brief description",
      "method": "GET",
      "endpoint": "/exact/path",
      "expected_status": 404,
      "reasoning": "Why this test matters",
      "requires_auth": true
    }
  ]
}

Requirements:
- No code fences, comments, or extra fields
- Double quotes for keys/strings
- No trailing commas
- Sequential IDs starting from 1`, what, focus)
}
