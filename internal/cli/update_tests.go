package cli

import (
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/agents"
	"github.com/Octrafic/octrafic-cli/internal/core/auth"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// handleGenerateTestPlanResult handles the result from GenerateTestPlan backend call
func handleGenerateTestPlanResult(m *TestUIModel, msg generateTestPlanResultMsg) (tea.Model, tea.Cmd) {
	// Handle GenerateTestPlan result from backend

	// Convert backend tests to a simpler format for tool_result
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

	// Activate agent widget and show progress
	if len(testCases) > 0 {
		// Extract unique endpoints for display
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

		// Show regular message
		bullet := lipgloss.NewStyle().Foreground(Theme.Primary).Render("➔")
		m.addMessage(fmt.Sprintf("%s Generated %d test cases", bullet, len(testCases)))
		details := fmt.Sprintf("    Testing: %s", strings.Join(endpoints, ", "))
		m.addMessage(m.subtleStyle.Render(details))
	} else {
		m.addMessage(m.subtleStyle.Render("⚠️  No tests generated"))
		m.updateViewport()
	}

	// Send tool_result with test cases back to agent
	// Agent will then call ExecuteTestGroup with these tests
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
		m.conversationHistory = append(m.conversationHistory, agent.ChatMessage{
			Role:             "user",
			FunctionResponse: funcResp,
		})
		m.currentTestToolID = "" // Clear the tool_use_id

		// Send FunctionResponse to agent so it can call ExecuteTestGroup
		return m, tea.Batch(
			animationTick(),
			m.sendChatMessage(""),
		)
	}

	return m, nil
}

// handleStartTestGroup starts running a group of tests incrementally
func handleStartTestGroup(m *TestUIModel, msg startTestGroupMsg) (tea.Model, tea.Cmd) {
	// Start running a group of tests incrementally
	m.pendingTests = msg.tests
	m.currentTestGroupLabel = msg.label
	m.currentTestToolName = msg.toolName
	// CRITICAL: Only set currentTestToolID if not already set (preserve GenerateTestPlan's tool_use_id)
	if msg.toolID != "" {
		m.currentTestToolID = msg.toolID
	}
	m.testGroupCompletedCount = 0
	m.totalTestsInProgress = len(msg.tests)
	m.testGroupResults = make([]map[string]any, 0, len(msg.tests))
	m.agentState = StateRunningTests

	// Don't add new "Agent:" label - continue with current agent message
	// Just add the test group label as a separator
	m.addMessage("")
	m.addMessage(m.subtleStyle.Render(msg.label))
	m.updateViewport()

	// Start running first test
	return m, runNextTest()
}

// handleRunNextTest executes the next test in the queue
func handleRunNextTest(m *TestUIModel, _ runNextTestMsg) (tea.Model, tea.Cmd) {
	// Execute the next test in the queue
	if len(m.pendingTests) == 0 {
		// All tests done - add FunctionResponse and return to idle
		m.addMessage("")

		// Only add FunctionResponse if this was from a Claude tool_use (has ID)
		// If tests were triggered by UI (user selected tests), don't send FunctionResponse
		hadToolID := m.currentTestToolID != ""      // Check before cleanup
		completedCount := m.testGroupCompletedCount // Save before cleanup

		if hadToolID {
			// Add FunctionResponse to conversation history
			funcResp := &agent.FunctionResponseData{
				ID:   m.currentTestToolID, // tool_use_id from original tool call
				Name: m.currentTestToolName,
				Response: map[string]any{
					"count":   m.testGroupCompletedCount,
					"results": m.testGroupResults,
				},
			}
			m.conversationHistory = append(m.conversationHistory, agent.ChatMessage{
				Role:             "user",
				FunctionResponse: funcResp,
			})
		}

		// Clean up
		m.pendingTests = nil
		m.currentTestGroupLabel = ""
		m.testGroupCompletedCount = 0
		m.totalTestsInProgress = 0
		m.testGroupResults = nil
		m.currentTestToolName = ""
		m.currentTestToolID = ""
		m.agentState = StateProcessing // Keep spinner visible until agent responds
		m.updateViewport()

		// Continue conversation with agent
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

	// Get next test from queue
	testMap := m.pendingTests[0]
	m.pendingTests = m.pendingTests[1:]

	// Execute the test (this is a blocking operation, so we do it here)
	method, _ := testMap["method"].(string)
	endpoint, _ := testMap["endpoint"].(string)
	requiresAuth := false
	if ra, ok := testMap["requires_auth"].(bool); ok {
		requiresAuth = ra
	}

	// Get headers and body
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

	// Choose auth provider based on requires_auth flag
	originalAuth := m.authProvider
	if !requiresAuth {
		m.testExecutor.UpdateAuthProvider(&auth.NoAuth{})
	}

	// Execute test
	result, err := m.testExecutor.ExecuteTest(method, endpoint, headers, body)

	// Restore original auth
	if !requiresAuth {
		m.testExecutor.UpdateAuthProvider(originalAuth)
	}

	// Display result immediately with indentation
	methodStyle, ok := m.methodStyles[method]
	if !ok {
		methodStyle = lipgloss.NewStyle().Foreground(Theme.TextSubtle)
	}
	methodFormatted := methodStyle.Render(method)

	// Build auth indicator - only show if auth is required
	authIndicator := ""
	if requiresAuth {
		authIndicator = " " + lipgloss.NewStyle().Foreground(Theme.Warning).Render("• Auth")
	}

	if err != nil {
		m.addMessage(fmt.Sprintf("  ✗ %s %s%s", methodFormatted, endpoint, authIndicator))
		m.addMessage(m.subtleStyle.Render(fmt.Sprintf("    Error: %s", err.Error())))

		// Add to results for FunctionResponse
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

		// Add to results for FunctionResponse
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

	// Schedule next test
	return m, runNextTest()
}
