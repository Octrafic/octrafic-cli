package cli

import (
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/agents"
	"github.com/Octrafic/octrafic-cli/internal/core/auth"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// handleGenerateTestPlanResult processes the generated test plan from the agent.
func handleGenerateTestPlanResult(m *TestUIModel, msg generateTestPlanResultMsg) (tea.Model, tea.Cmd) {
	testCases := make([]map[string]any, 0, len(msg.backendTests))
	for _, bt := range msg.backendTests {
		testCases = append(testCases, map[string]any{
			"method":        bt.TestCase.Method,
			"endpoint":      bt.TestCase.Endpoint,
			"headers":       bt.TestCase.Headers,
			"body":          bt.TestCase.Body,
			"requires_auth": bt.TestCase.RequiresAuth,
			"description":   bt.TestCase.Description,
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
			Name: "GenerateTestPlan",
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
	m.agentState = StateRunningTests

	m.addMessage("")
	m.addMessage(m.subtleStyle.Render(msg.label))
	m.updateViewport()

	return m, runNextTest()
}

// handleRunNextTest executes the next test in the queue.
func handleRunNextTest(m *TestUIModel, _ runNextTestMsg) (tea.Model, tea.Cmd) {
	if len(m.pendingTests) == 0 {
		m.addMessage("")

		// Only add FunctionResponse if this was from a Claude tool_use (has ID)
		// If tests were triggered by UI (user selected tests), don't send FunctionResponse
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
		m.agentState = StateProcessing
		m.updateViewport()

		if hadToolID {
			// Tests were triggered by Claude tool_use - FunctionResponse was added to history
			// MUST send it to backend now! tool_result must be the next message after tool_use
			return m, m.sendChatMessage("")
		} else {
			// Tests were triggered by UI - send summary message to Claude
			summary := fmt.Sprintf("Tests completed. %d tests executed. Would you like me to analyze the results or run more tests?",
				completedCount)
			return m, m.sendChatMessage(summary)
		}
	}

	testMap := m.pendingTests[0]
	m.pendingTests = m.pendingTests[1:]

	method, _ := testMap["method"].(string)
	endpoint, _ := testMap["endpoint"].(string)
	requiresAuth := false
	if ra, ok := testMap["requires_auth"].(bool); ok {
		requiresAuth = ra
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
		body = b
	}

	originalAuth := m.authProvider
	if !requiresAuth {
		m.testExecutor.UpdateAuthProvider(&auth.NoAuth{})
	}

	result, err := m.testExecutor.ExecuteTest(method, endpoint, headers, body)

	if !requiresAuth {
		m.testExecutor.UpdateAuthProvider(originalAuth)
	}

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
		m.addMessage(fmt.Sprintf("  ✗ %s %s%s", methodFormatted, endpoint, authIndicator))
		m.addMessage(m.subtleStyle.Render(fmt.Sprintf("    Error: %s", err.Error())))

		m.testGroupResults = append(m.testGroupResults, map[string]any{
			"method":        method,
			"endpoint":      endpoint,
			"error":         err.Error(),
			"requires_auth": requiresAuth,
		})
	} else {
		statusIcon := "✓"
		statusStyle := m.successStyle
		if result.StatusCode >= 400 {
			statusIcon = "✗"
			statusStyle = m.errorStyle
		}
		m.addMessage(fmt.Sprintf("  %s %s %s%s", statusStyle.Render(statusIcon), methodFormatted, endpoint, authIndicator))
		m.addMessage(m.subtleStyle.Render(fmt.Sprintf("    Status: %d | Duration: %dms", result.StatusCode, result.Duration.Milliseconds())))

		m.testGroupResults = append(m.testGroupResults, map[string]any{
			"method":        method,
			"endpoint":      endpoint,
			"status_code":   result.StatusCode,
			"response_body": result.ResponseBody,
			"duration_ms":   result.Duration.Milliseconds(),
			"requires_auth": requiresAuth,
		})
	}
	m.testGroupCompletedCount++
	m.updateViewport()

	return m, runNextTest()
}
