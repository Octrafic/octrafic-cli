package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	agent "github.com/Octrafic/octrafic-cli/internal/agents"
	"github.com/Octrafic/octrafic-cli/internal/core/tester"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// handleGenerateTestPlanResult processes the generated test plan from the agent.
func handleGenerateTestPlanResult(m *TestUIModel, msg generateTestPlanResultMsg) (tea.Model, tea.Cmd) {
	testCases := make([]map[string]any, 0, len(msg.backendTests))
	m.tests = make([]Test, 0, len(msg.backendTests))
	for i, bt := range msg.backendTests {
		m.tests = append(m.tests, Test{
			ID:          i + 1,
			Method:      bt.TestCase.Method,
			Endpoint:    bt.TestCase.Endpoint,
			Description: fmt.Sprintf("%s %s", bt.TestCase.Method, bt.TestCase.Endpoint),
			Status:      "pending",
			Selected:    true,
			BackendTest: &bt.TestCase,
		})

		testCases = append(testCases, map[string]any{
			"method":          bt.TestCase.Method,
			"endpoint":        bt.TestCase.Endpoint,
			"headers":         bt.TestCase.Headers,
			"body":            bt.TestCase.Body,
			"requires_auth":   bt.TestCase.RequiresAuth,
			"description":     bt.TestCase.Description,
			"expected_status": bt.TestCase.ExpectedStatus,
			"extract":         bt.TestCase.Extract,
			"assertions":      bt.TestCase.Assertions,
		})
	}

	if len(testCases) > 0 {
		endpointMap := make(map[string]bool)
		for _, tc := range testCases {
			method, _ := tc["method"].(string)
			endpoint, _ := tc["endpoint"].(string)
			endpointMap[fmt.Sprintf("%s %s", method, endpoint)] = true
		}
		var endpoints []string
		for ep := range endpointMap {
			endpoints = append(endpoints, ep)
		}

		details := fmt.Sprintf("Testing: %s", strings.Join(endpoints, ", "))
		m.showToolMessage(fmt.Sprintf("Generated %d test cases", len(testCases)), details)
	} else {
		m.addMessage(m.subtleStyle.Render("⚠️  No tests generated"))
		m.updateViewport()
	}

	if m.currentTestToolID != "" {
		m.agentState = StateProcessing

		toolID := m.currentTestToolID
		funcResp := &agent.FunctionResponseData{
			ID:   toolID,
			Name: agent.ToolGenerateTestPlan,
			Response: map[string]any{
				"status":     "tests_generated",
				"test_count": len(testCases),
				"test_cases": testCases,
			},
		}
		chatMsg := agent.ChatMessage{
			Role:             "user",
			FunctionResponse: funcResp,
		}
		m.conversationHistory = append(m.conversationHistory, chatMsg)
		m.saveChatMessageToConversation(chatMsg)

		m.currentTestToolID = ""

		return m, tea.Batch(
			animationTick(),
			m.sendChatMessage(""),
		)
	}

	return m, nil
}

// handleStartTestGroup initiates execution of a test group.
func handleStartTestGroup(m *TestUIModel, msg startTestGroupMsg) (tea.Model, tea.Cmd) {
	m.pendingTests = msg.tests
	m.currentTestGroupLabel = msg.label
	m.currentTestToolName = msg.toolName

	if msg.toolID != "" {
		m.currentTestToolID = msg.toolID
	}
	m.testGroupCompletedCount = 0
	m.totalTestsInProgress = len(msg.tests)
	m.testGroupResults = make([]map[string]any, 0, len(msg.tests))
	m.testVars = make(map[string]string)
	m.agentState = StateRunningTests

	m.addMessage("")
	m.addMessage(m.subtleStyle.Render(msg.label))
	m.updateViewport()

	return m, runNextTest()
}

// applyVars substitutes {{var}} placeholders with values from m.testVars.
func (m *TestUIModel) applyVars(s string) string {
	for k, v := range m.testVars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}

// extractVars runs extract rules against a response body, storing results in m.testVars.
func (m *TestUIModel) extractVars(body string, extracts []map[string]any) {
	if len(extracts) == 0 || strings.TrimSpace(body) == "" {
		return
	}
	var parsed any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return
	}
	for _, e := range extracts {
		field, _ := e["field"].(string)
		as, _ := e["as"].(string)
		if field == "" || as == "" {
			continue
		}
		val, ok := tester.ResolvePath(parsed, field)
		if !ok || val == nil {
			continue
		}
		switch v := val.(type) {
		case string:
			m.testVars[as] = v
		case float64:
			m.testVars[as] = fmt.Sprintf("%g", v)
		default:
			b, err := json.Marshal(v)
			if err == nil {
				m.testVars[as] = string(b)
			}
		}
	}
}

// handleRunNextTest executes the next test in the queue.
func handleRunNextTest(m *TestUIModel, _ runNextTestMsg) (tea.Model, tea.Cmd) {
	if len(m.pendingTests) == 0 {
		m.addMessage("")

		hadToolID := m.currentTestToolID != ""
		completedCount := m.testGroupCompletedCount

		if hadToolID {
			funcResp := &agent.FunctionResponseData{
				ID:   m.currentTestToolID,
				Name: m.currentTestToolName,
				Response: map[string]any{
					"count":   m.testGroupCompletedCount,
					"results": m.testGroupResults,
				},
			}
			chatMsg := agent.ChatMessage{
				Role:             "user",
				FunctionResponse: funcResp,
			}
			m.conversationHistory = append(m.conversationHistory, chatMsg)
			m.saveChatMessageToConversation(chatMsg)
		}

		m.pendingTests = nil
		m.currentTestGroupLabel = ""
		m.testGroupCompletedCount = 0
		m.totalTestsInProgress = 0
		m.testGroupResults = nil
		m.currentTestToolName = ""
		m.currentTestToolID = ""
		m.testVars = nil
		m.agentState = StateProcessing
		m.updateViewport()

		if hadToolID {
			return m, m.sendChatMessage("")
		}
		summary := fmt.Sprintf("Tests completed. %d tests executed. Would you like me to analyze the results or run more tests?",
			completedCount)
		return m, m.sendChatMessage(summary)
	}

	testMap := m.pendingTests[0]
	m.pendingTests = m.pendingTests[1:]

	method, _ := testMap["method"].(string)
	endpoint, _ := testMap["endpoint"].(string)

	if m.testVars != nil {
		endpoint = m.applyVars(endpoint)
	}

	requiresAuth := false
	if ra, ok := testMap["requires_auth"].(bool); ok {
		requiresAuth = ra
	}

	expectedStatus := 0
	if es, ok := testMap["expected_status"].(float64); ok {
		expectedStatus = int(es)
	} else if es, ok := testMap["expected_status"].(int); ok {
		expectedStatus = es
	}
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	headers := make(map[string]string)
	if h, ok := testMap["headers"].(map[string]any); ok {
		for k, v := range h {
			if vs, ok := v.(string); ok {
				headers[k] = vs
			}
		}
	}

	var body any
	if b, ok := testMap["body"]; ok {
		if bs, ok := b.(string); ok && m.testVars != nil {
			body = m.applyVars(bs)
		} else {
			body = b
		}
	}

	result, err := m.testExecutor.ExecuteTest(method, endpoint, headers, body, requiresAuth)

	methodStyle, ok := m.methodStyles[method]
	if !ok {
		methodStyle = lipgloss.NewStyle().Foreground(Theme.TextSubtle)
	}
	methodFormatted := methodStyle.Render(method)

	authIndicator := ""
	if requiresAuth {
		authIndicator = " " + lipgloss.NewStyle().Foreground(Theme.Warning).Render("• Auth")
	}

	if err != nil {
		m.addMessage(fmt.Sprintf("  %s %s %s%s", m.errorStyle.Render("✗"), methodFormatted, endpoint, authIndicator))
		m.addMessage(m.subtleStyle.Render(fmt.Sprintf("    Error: %s", friendlyError(err))))
		if m.isHeadless {
			m.headlessExitCode = 1
		}

		m.testGroupResults = append(m.testGroupResults, map[string]any{
			"method":          method,
			"endpoint":        endpoint,
			"error":           err.Error(),
			"requires_auth":   requiresAuth,
			"expected_status": expectedStatus,
			"passed":          false,
		})
	} else {
		if extracts := toMapsSlice(testMap["extract"]); len(extracts) > 0 {
			m.extractVars(result.ResponseBody, extracts)
		}

		passed := result.StatusCode == expectedStatus
		schemaErrors := m.validateResponseSchema(method, endpoint, result.StatusCode, result.ResponseBody)
		schemaValid := len(schemaErrors) == 0

		assertionFailures := tester.RunAssertions(result.ResponseBody, toMapsSlice(testMap["assertions"]))
		assertionsPassed := len(assertionFailures) == 0

		statusIcon := "✓"
		statusStyle := m.successStyle
		if !passed {
			statusIcon = "✗"
			statusStyle = m.errorStyle
			if m.isHeadless {
				m.headlessExitCode = 1
			}
		} else if !schemaValid || !assertionsPassed {
			statusIcon = "⚠"
			statusStyle = lipgloss.NewStyle().Foreground(Theme.Warning)
			if m.isHeadless {
				m.headlessExitCode = 1
			}
		}

		statusMsg := fmt.Sprintf("    Status: %d", result.StatusCode)
		if !passed {
			statusMsg += fmt.Sprintf(" (expected %d)", expectedStatus)
		}
		statusMsg += fmt.Sprintf(" | Duration: %dms", result.Duration.Milliseconds())

		m.addMessage(fmt.Sprintf("  %s %s %s%s", statusStyle.Render(statusIcon), methodFormatted, endpoint, authIndicator))
		m.addMessage(m.subtleStyle.Render(statusMsg))

		if !schemaValid {
			schemaStyle := lipgloss.NewStyle().Foreground(Theme.Warning)
			m.addMessage(schemaStyle.Render("    Schema mismatch:"))
			for _, se := range schemaErrors {
				m.addMessage(schemaStyle.Render("      · " + se))
			}
		}

		if !assertionsPassed {
			warnStyle := lipgloss.NewStyle().Foreground(Theme.Warning)
			m.addMessage(warnStyle.Render("    Assertion failures:"))
			for _, af := range assertionFailures {
				m.addMessage(warnStyle.Render("      · " + af))
			}
		}

		m.testGroupResults = append(m.testGroupResults, map[string]any{
			"method":             method,
			"endpoint":           endpoint,
			"status_code":        result.StatusCode,
			"expected_status":    expectedStatus,
			"response_body":      result.ResponseBody,
			"duration_ms":        result.Duration.Milliseconds(),
			"requires_auth":      requiresAuth,
			"passed":             passed,
			"schema_valid":       schemaValid,
			"schema_errors":      schemaErrors,
			"assertions_passed":  assertionsPassed,
			"assertion_failures": assertionFailures,
		})
	}
	m.testGroupCompletedCount++
	m.updateViewport()

	return m, runNextTest()
}

func extractsToAny(extracts []agent.Extract) []any {
	if len(extracts) == 0 {
		return nil
	}
	out := make([]any, len(extracts))
	for i, e := range extracts {
		out[i] = map[string]any{"field": e.Field, "as": e.As}
	}
	return out
}

func assertionsToAny(assertions []agent.Assertion) []any {
	if len(assertions) == 0 {
		return nil
	}
	out := make([]any, len(assertions))
	for i, a := range assertions {
		out[i] = map[string]any{"field": a.Field, "op": a.Op, "value": a.Value}
	}
	return out
}

func toMapsSlice(v any) []map[string]any {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
