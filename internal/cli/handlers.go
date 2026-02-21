package cli

import (
	"encoding/json"
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/agents"
	"github.com/Octrafic/octrafic-cli/internal/core/parser"
	"github.com/Octrafic/octrafic-cli/internal/core/reporter"
	"github.com/Octrafic/octrafic-cli/internal/exporter"
	"github.com/Octrafic/octrafic-cli/internal/infra/logger"
	"github.com/Octrafic/octrafic-cli/internal/infra/storage"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.uber.org/zap"
)

// ToolCallData represents a tool call with its arguments.
type ToolCallData struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Args map[string]any `json:"arguments"`
}

type runNextTestMsg struct{}

func runNextTest() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(100 * time.Millisecond)
		return runNextTestMsg{}
	}
}

type startTestGroupMsg struct {
	tests    []map[string]any
	label    string
	toolName string
	toolID   string
}

// sendChatMessage initiates a streaming chat request with the agent.
func (m *TestUIModel) sendChatMessage(_ string) tea.Cmd {
	if len(m.streamedToolCalls) > 0 {
		return func() tea.Msg {
			return processToolCallsMsg{}
		}
	}

	return func() tea.Msg {
		time.Sleep(100 * time.Millisecond)

		streamChan := make(chan string, 100)
		cancelChan := m.cancelStream

		go func() {
			defer close(streamChan)

			if m.localAgent == nil {
				var err error
				m.localAgent, err = agent.NewAgent(m.baseURL)
				if err != nil {
					streamChan <- "\x00ERROR:Failed to initialize local agent: " + err.Error()
					return
				}
			}

			endpointsList := ""
			if m.currentProject != nil {
				if endpoints, err := m.loadProjectEndpoints(); err == nil && len(endpoints) > 0 {
					endpointsList = storage.GetEndpointsList(endpoints)
				}
			}

			// Run ChatStream in separate goroutine so we can cancel
			type chatResult struct {
				response *agent.ChatResponse
				err      error
			}
			resultChan := make(chan chatResult, 1)

			go func() {
				expandedHistory := make([]agent.ChatMessage, len(m.conversationHistory))
				for i, msg := range m.conversationHistory {
					expandedHistory[i] = msg

					if msg.Role == "user" && strings.Contains(msg.Content, "@") {
						expandedHistory[i].Content = expandContentForLLM(msg.Content)
					}
				}

				resp, err := m.localAgent.ChatStream(expandedHistory, true,
					func(chunk string, isThought bool) {
						select {
						case <-cancelChan:
							return
						default:
						}

						if isThought {
							logger.Debug("Received THINK chunk", logger.String("chunk", chunk[:min(len(chunk), 50)]+"..."))
							select {
							case streamChan <- "\x00THINK:" + chunk:
							case <-cancelChan:
								return
							}
						} else {
							logger.Debug("Received TEXT chunk", logger.String("chunk", chunk[:min(len(chunk), 50)]+"..."))
							select {
							case streamChan <- "\x00TEXT:" + chunk:
							case <-cancelChan:
								return
							}
						}
					}, endpointsList)
				resultChan <- chatResult{response: resp, err: err}
			}()

			select {
			case <-cancelChan:
				logger.Info("ChatStream cancelled by user")
				streamChan <- "\x00CANCELLED:"
				return
			case result := <-resultChan:
				select {
				case <-cancelChan:
					logger.Info("ChatStream cancelled by user")
					streamChan <- "\x00CANCELLED:"
					return
				default:
				}

				if result.err != nil {
					logger.Error("ChatStream failed", logger.Err(result.err))
					streamChan <- "\x00ERROR:" + result.err.Error()
				} else {
					logger.Info("ChatStream completed",
						zap.Int64("input_tokens", result.response.InputTokens),
						zap.Int64("output_tokens", result.response.OutputTokens),
						logger.String("message_preview", result.response.Message[:min(len(result.response.Message), 100)]+"..."))
					streamChan <- "\x00AGENT:" + result.response.Message
					if len(result.response.ToolCalls) > 0 {
						logger.Debug("Tool calls received", zap.Int("count", len(result.response.ToolCalls)))
						toolCallsJSON, _ := json.Marshal(result.response.ToolCalls)
						streamChan <- "\x00TOOLS:" + string(toolCallsJSON)
					}
					tokenData := fmt.Sprintf("%d,%d", result.response.InputTokens, result.response.OutputTokens)
					streamChan <- "\x00TOKENS:" + tokenData
					streamChan <- "\x00DONE:"
				}
			}
		}()

		return streamReasoningMsg{channel: streamChan}
	}
}

// expandContentForLLM takes a string with @/abs/path tokens and replaces them with file content
func expandContentForLLM(input string) string {
	words := strings.Split(input, " ")
	for i, word := range words {
		if strings.HasPrefix(word, "@") {
			path := word[1:]
			info, err := os.Stat(path)
			if err == nil && !info.IsDir() {
				content, err := os.ReadFile(path)
				if err == nil {
					ext := filepath.Ext(path)
					lang := ""
					if len(ext) > 1 {
						lang = ext[1:]
					}
					expanded := fmt.Sprintf("File: %s\n```%s\n%s\n```", path, lang, string(content))
					words[i] = expanded
				}
			}
		}
	}
	return strings.Join(words, " ")
}

// waitForReasoning waits for the next chunk from the streaming channel.
func waitForReasoning(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		chunk, ok := <-ch
		if !ok {
			return streamDoneMsg{}
		}
		return reasoningChunkMsg{chunk: chunk, channel: ch}
	}
}

// executeTool executes a tool call and returns its result.
func (m *TestUIModel) executeTool(toolCall agent.ToolCall) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(300 * time.Millisecond)

		if toolCall.Name == "get_endpoints_details" {
			endpointsArg, ok := toolCall.Arguments["endpoints"]
			if !ok {
				return toolResultMsg{
					toolID:   toolCall.ID,
					toolName: toolCall.Name,
					result:   nil,
					err:      fmt.Errorf("missing required parameter: endpoints"),
				}
			}

			endpointsSlice, ok := endpointsArg.([]any)
			if !ok {
				return toolResultMsg{
					toolID:   toolCall.ID,
					toolName: toolCall.Name,
					result:   nil,
					err:      fmt.Errorf("'endpoints' parameter must be an array"),
				}
			}

			allEndpoints, err := m.loadProjectEndpoints()
			if err != nil {
				return toolResultMsg{
					toolID:   toolCall.ID,
					toolName: toolCall.Name,
					result:   nil,
					err:      fmt.Errorf("failed to load endpoints: %w", err),
				}
			}

			var results []map[string]any
			for _, e := range endpointsSlice {
				if epMap, ok := e.(map[string]any); ok {
					path, _ := epMap["path"].(string)
					method, _ := epMap["method"].(string)

					for _, ep := range allEndpoints {
						if ep.Path == path && ep.Method == method {
							result := map[string]any{
								"method":        ep.Method,
								"path":          ep.Path,
								"description":   ep.Description,
								"requires_auth": ep.RequiresAuth,
								"auth_type":     ep.AuthType,
							}
							if len(ep.Parameters) > 0 {
								result["parameters"] = ep.Parameters
							}
							if ep.RequestBody != "" {
								result["request_body"] = ep.RequestBody
							}
							if len(ep.Responses) > 0 {
								result["responses"] = ep.Responses
							}
							results = append(results, result)
							break
						}
					}
				}
			}

			return toolResultMsg{
				toolID:   toolCall.ID,
				toolName: toolCall.Name,
				result:   map[string]any{"endpoints": results},
				err:      nil,
			}
		}

		if toolCall.Name == "ExecuteTest" {
			method, _ := toolCall.Arguments["method"].(string)
			endpoint, _ := toolCall.Arguments["endpoint"].(string)

			if method == "" || endpoint == "" {
				return toolResultMsg{
					toolID:   toolCall.ID,
					toolName: toolCall.Name,
					result:   nil,
					err:      fmt.Errorf("missing required parameters: method and endpoint"),
				}
			}

			expectedStatus := 200
			if es, ok := toolCall.Arguments["expected_status"].(float64); ok {
				expectedStatus = int(es)
			} else if es, ok := toolCall.Arguments["expected_status"].(int); ok {
				expectedStatus = es
			}

			headers := make(map[string]string)
			if h, ok := toolCall.Arguments["headers"].(map[string]any); ok {
				for k, v := range h {
					if vs, ok := v.(string); ok {
						headers[k] = vs
					}
				}
			}

			var body any
			if b, ok := toolCall.Arguments["body"]; ok {
				body = b
			}

			requiresAuth := false
			if ra, ok := toolCall.Arguments["requires_auth"].(bool); ok {
				requiresAuth = ra
			}

			result, err := m.testExecutor.ExecuteTest(method, endpoint, headers, body, requiresAuth)

			if err != nil {
				return toolResultMsg{
					toolID:   toolCall.ID,
					toolName: toolCall.Name,
					result: map[string]any{
						"method":          method,
						"endpoint":        endpoint,
						"error":           err.Error(),
						"expected_status": expectedStatus,
						"passed":          false,
					},
					err: err,
				}
			}

			passed := result.StatusCode == expectedStatus

			return toolResultMsg{
				toolID:   toolCall.ID,
				toolName: toolCall.Name,
				result: map[string]any{
					"method":          method,
					"endpoint":        endpoint,
					"status_code":     result.StatusCode,
					"expected_status": expectedStatus,
					"response_body":   result.ResponseBody,
					"duration_ms":     result.Duration.Milliseconds(),
					"passed":          passed,
				},
				err: nil,
			}
		}

		if toolCall.Name == "GenerateReport" {
			reportContent, _ := toolCall.Arguments["report_content"].(string)
			if reportContent == "" {
				return toolResultMsg{
					toolID:   toolCall.ID,
					toolName: toolCall.Name,
					err:      fmt.Errorf("missing required parameter: report_content"),
				}
			}

			fileName, _ := toolCall.Arguments["file_name"].(string)

			pdfPath, err := reporter.GeneratePDF(reportContent, fileName)
			if err != nil {
				return toolResultMsg{
					toolID:   toolCall.ID,
					toolName: toolCall.Name,
					result:   nil,
					err:      err,
				}
			}

			return toolResultMsg{
				toolID:   toolCall.ID,
				toolName: toolCall.Name,
				result: map[string]any{
					"status":    "success",
					"file_path": pdfPath,
				},
				err: nil,
			}
		}

		if toolCall.Name == "ExecuteTestGroup" {
			testsArg, ok := toolCall.Arguments["tests"]
			if !ok {
				return toolResultMsg{
					toolID:   toolCall.ID,
					toolName: toolCall.Name,
					err:      fmt.Errorf("missing 'tests' parameter"),
				}
			}

			testsSlice, ok := testsArg.([]any)
			if !ok {
				return toolResultMsg{
					toolID:   toolCall.ID,
					toolName: toolCall.Name,
					err:      fmt.Errorf("'tests' parameter must be an array"),
				}
			}

			tests := make([]map[string]any, 0, len(testsSlice))
			for _, testArg := range testsSlice {
				testMap, ok := testArg.(map[string]any)
				if !ok {
					continue
				}
				tests = append(tests, testMap)
			}

			if len(tests) == 0 {
				return toolResultMsg{
					toolID:   toolCall.ID,
					toolName: toolCall.Name,
					err:      fmt.Errorf("no valid tests to execute"),
				}
			}

			return showTestSelectionMsg{
				tests:    tests,
				toolCall: toolCall,
			}
		}

		if toolCall.Name == "ExportTests" {
			return m.handleExportTests(toolCall)
		}

		return toolResultMsg{
			toolID:   toolCall.ID,
			toolName: toolCall.Name,
			err:      fmt.Errorf("unknown tool: %s", toolCall.Name),
		}
	}
}

func (m *TestUIModel) handleToolResult(toolName string, toolID string, result any) tea.Cmd {
	if toolName == "ExecuteTest" {
		if resultMap, ok := result.(map[string]any); ok {
			method, _ := resultMap["method"].(string)
			endpoint, _ := resultMap["endpoint"].(string)
			statusCode, _ := resultMap["status_code"].(int)
			expectedStatus, _ := resultMap["expected_status"].(int)
			responseBody, _ := resultMap["response_body"].(string)
			durationMs, _ := resultMap["duration_ms"].(int64)

			var passed bool
			if p, ok := resultMap["passed"].(bool); ok {
				passed = p
			} else {
				if expectedStatus == 0 {
					expectedStatus = 200
				}
				passed = statusCode == expectedStatus
			}

			methodStyle, ok := m.methodStyles[method]
			if !ok {
				methodStyle = lipgloss.NewStyle().Foreground(Theme.TextSubtle)
			}
			methodFormatted := methodStyle.Render(method)

			statusStyle := m.successStyle
			statusIcon := "✓"
			if !passed {
				statusStyle = m.errorStyle
				statusIcon = "✗"
				if m.isHeadless {
					m.headlessExitCode = 1
				}
			}

			statusMsg := fmt.Sprintf("   Status: %d", statusCode)
			if !passed && expectedStatus > 0 {
				statusMsg += fmt.Sprintf(" (expected %d)", expectedStatus)
			}
			statusMsg += fmt.Sprintf(" | Duration: %dms", durationMs)

			m.addMessage("")
			m.addMessage(statusStyle.Render(statusIcon) + " " + methodFormatted + " " + endpoint)
			m.addMessage(m.subtleStyle.Render(statusMsg))

			if len(responseBody) > 0 {
				preview := responseBody
				if len(preview) > 200 {
					preview = preview[:200] + "..."
				}
				m.addMessage(m.subtleStyle.Render("   Response: " + preview))
			}

			if toolID != "" {
				chatMsg := agent.ChatMessage{
					Role: "user",
					FunctionResponse: &agent.FunctionResponseData{
						ID:       toolID,
						Name:     "ExecuteTest",
						Response: resultMap,
					},
				}
				m.conversationHistory = append(m.conversationHistory, chatMsg)

				// Save function response to conversation
				m.saveChatMessageToConversation(chatMsg)

				// Send back to agent to continue
				return m.sendChatMessage("")
			}
			return nil // No tool_use, so don't send response back
		}
	}

	if toolName == "ExecuteTestGroup" {
		// Display results from test group
		if resultMap, ok := result.(map[string]any); ok {
			count, _ := resultMap["count"].(int)
			results, _ := resultMap["results"].([]map[string]any)

			failedCount := 0
			passedCount := 0

			// Display each test result
			for _, testResult := range results {
				method, _ := testResult["method"].(string)
				endpoint, _ := testResult["endpoint"].(string)
				statusCode, _ := testResult["status_code"].(int)
				expectedStatus, _ := testResult["expected_status"].(int)
				durationMs, _ := testResult["duration_ms"].(int64)
				requiresAuth := false
				if ra, ok := testResult["requires_auth"].(bool); ok {
					requiresAuth = ra
				}

				var passed bool
				if p, ok := testResult["passed"].(bool); ok {
					passed = p
				} else {
					if expectedStatus == 0 {
						expectedStatus = 200
					}
					passed = statusCode == expectedStatus
				}

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

				statusStyle := m.successStyle
				statusIcon := "✓"
				if !passed {
					statusStyle = m.errorStyle
					statusIcon = "✗"
					failedCount++
				} else {
					passedCount++
				}

				statusMsg := fmt.Sprintf("   Status: %d", statusCode)
				if !passed && expectedStatus > 0 {
					statusMsg += fmt.Sprintf(" (expected %d)", expectedStatus)
				}
				statusMsg += fmt.Sprintf(" | Duration: %dms", durationMs)

				m.addMessage("")
				m.addMessage(statusStyle.Render(statusIcon) + " " + methodFormatted + " " + endpoint + authIndicator)
				m.addMessage(m.subtleStyle.Render(statusMsg))
			}

			m.addMessage("")
			if failedCount > 0 {
				m.addMessage(m.errorStyle.Render(fmt.Sprintf("✗ %d/%d tests failed", failedCount, count)))
				if m.isHeadless {
					m.headlessExitCode = 1
				}
			} else {
				m.addMessage(m.successStyle.Render(fmt.Sprintf("✓ All %d tests passed", count)))
			}

			// Add tool result to conversation history as function response
			// Only if this was from a Claude tool_use (has toolID)
			if toolID != "" {
				chatMsg := agent.ChatMessage{
					Role: "user",
					FunctionResponse: &agent.FunctionResponseData{
						ID:       toolID,
						Name:     "ExecuteTestGroup",
						Response: resultMap,
					},
				}
				m.conversationHistory = append(m.conversationHistory, chatMsg)

				// Save function response to conversation
				m.saveChatMessageToConversation(chatMsg)

				// Send back to agent
				return m.sendChatMessage("")
			}
			return nil // No tool_use, so don't send response back
		}
	}

	if toolName == "get_endpoints_details" {
		// Add tool result to conversation history as function response
		if toolID != "" {
			var resultMap map[string]any
			if r, ok := result.(map[string]any); ok {
				resultMap = make(map[string]any)
				maps.Copy(resultMap, r)
			}

			chatMsg := agent.ChatMessage{
				Role: "user",
				FunctionResponse: &agent.FunctionResponseData{
					ID:       toolID,
					Name:     "get_endpoints_details",
					Response: resultMap,
				},
			}
			m.conversationHistory = append(m.conversationHistory, chatMsg)

			// Save function response to conversation
			m.saveChatMessageToConversation(chatMsg)

			// Send back to agent to continue
			return m.sendChatMessage("")
		}
		return nil // No tool_use, so don't send response back
	}

	if toolName == "GenerateReport" {
		if resultMap, ok := result.(map[string]any); ok {
			filePath, _ := resultMap["file_path"].(string)

			m.addMessage("")
			m.addMessage(m.successStyle.Render("✓ Report generated"))
			m.addMessage(m.subtleStyle.Render("   " + filePath))

			if toolID != "" {
				chatMsg := agent.ChatMessage{
					Role: "user",
					FunctionResponse: &agent.FunctionResponseData{
						ID:       toolID,
						Name:     "GenerateReport",
						Response: resultMap,
					},
				}
				m.conversationHistory = append(m.conversationHistory, chatMsg)

				// Save function response to conversation
				m.saveChatMessageToConversation(chatMsg)

				return m.sendChatMessage("")
			}
			return nil
		}
	}

	if toolName == "ExportTests" {
		if resultMap, ok := result.(map[string]any); ok {
			testCount, _ := resultMap["test_count"].(int)
			exports, _ := resultMap["exports"].([]map[string]any)

			formatLabel := map[string]string{
				"postman": "Postman Collection",
				"pytest":  "pytest tests",
				"sh":      "curl script",
			}

			m.addMessage("")
			m.addMessage(m.successStyle.Render("✓ Tests exported"))
			m.addMessage(m.subtleStyle.Render(fmt.Sprintf("   Tests: %d", testCount)))
			m.addMessage("")

			for _, exportItem := range exports {
				format, _ := exportItem["format"].(string)
				filePath, _ := exportItem["filepath"].(string)

				formatName := formatLabel[format]
				if formatName == "" {
					formatName = format
				}

				m.addMessage(m.subtleStyle.Render(fmt.Sprintf("   • %s: %s", formatName, filePath)))
			}

			if toolID != "" {
				chatMsg := agent.ChatMessage{
					Role: "user",
					FunctionResponse: &agent.FunctionResponseData{
						ID:       toolID,
						Name:     "ExportTests",
						Response: resultMap,
					},
				}
				m.conversationHistory = append(m.conversationHistory, chatMsg)
				m.saveChatMessageToConversation(chatMsg)
				return m.sendChatMessage("")
			}
			return nil
		}
	}

	if toolName == "GenerateTestPlan" {
		// Add tool result to conversation history as function response
		if toolID != "" {
			var resultMap map[string]any
			if r, ok := result.(map[string]any); ok {
				resultMap = make(map[string]any)
				maps.Copy(resultMap, r)
			}

			chatMsg := agent.ChatMessage{
				Role: "user",
				FunctionResponse: &agent.FunctionResponseData{
					ID:       toolID,
					Name:     "GenerateTestPlan",
					Response: resultMap,
				},
			}
			m.conversationHistory = append(m.conversationHistory, chatMsg)

			// Save function response to conversation
			m.saveChatMessageToConversation(chatMsg)

			// Send back to agent to continue
			return m.sendChatMessage("")
		}
		return nil // No tool_use, so don't send response back
	}

	return nil
}

func (m *TestUIModel) loadProjectEndpoints() ([]parser.Endpoint, error) {
	return storage.LoadEndpoints(m.currentProject.ID, m.currentProject.IsTemporary)
}

func (m *TestUIModel) handleExportTests(toolCall agent.ToolCall) tea.Msg {
	exportsArg, ok := toolCall.Arguments["exports"]
	if !ok {
		return toolResultMsg{
			toolID:   toolCall.ID,
			toolName: toolCall.Name,
			err:      fmt.Errorf("missing 'exports' parameter"),
		}
	}

	exportsSlice, ok := exportsArg.([]any)
	if !ok {
		return toolResultMsg{
			toolID:   toolCall.ID,
			toolName: toolCall.Name,
			err:      fmt.Errorf("'exports' must be an array"),
		}
	}

	if len(exportsSlice) == 0 {
		return toolResultMsg{
			toolID:   toolCall.ID,
			toolName: toolCall.Name,
			err:      fmt.Errorf("'exports' array cannot be empty"),
		}
	}

	var tests []exporter.TestData
	if len(m.testGroupResults) > 0 {
		tests = make([]exporter.TestData, 0, len(m.testGroupResults))
		for _, result := range m.testGroupResults {
			method, _ := result["method"].(string)
			endpoint, _ := result["endpoint"].(string)
			statusCode, _ := result["status_code"].(int)
			responseBody, _ := result["response_body"].(string)
			durationMS, _ := result["duration_ms"].(int64)
			requiresAuth, _ := result["requires_auth"].(bool)
			errStr, _ := result["error"].(string)

			testData := exporter.TestData{
				Method:       method,
				Endpoint:     endpoint,
				StatusCode:   statusCode,
				ResponseBody: responseBody,
				DurationMS:   durationMS,
				RequiresAuth: requiresAuth,
				Error:        errStr,
			}
			tests = append(tests, testData)
		}
	} else if len(m.tests) > 0 {
		tests = make([]exporter.TestData, 0, len(m.tests))
		for _, test := range m.tests {
			requiresAuth := false
			var headers map[string]string
			var body interface{}

			if test.BackendTest != nil {
				requiresAuth = test.BackendTest.RequiresAuth
				headers = test.BackendTest.Headers
				body = test.BackendTest.Body
			}
			testData := exporter.TestData{
				Method:       test.Method,
				Endpoint:     test.Endpoint,
				RequiresAuth: requiresAuth,
				Headers:      headers,
				Body:         body,
			}
			tests = append(tests, testData)
		}
	} else {
		return toolResultMsg{
			toolID:   toolCall.ID,
			toolName: toolCall.Name,
			err:      fmt.Errorf("no tests available to export. Generate test plan first using GenerateTestPlan"),
		}
	}

	authType := ""
	authData := make(map[string]string)
	if m.currentProject != nil && m.currentProject.AuthConfig != nil {
		authType = m.currentProject.AuthConfig.Type
		authData["token"] = m.currentProject.AuthConfig.Token
		authData["key_name"] = m.currentProject.AuthConfig.KeyName
		authData["key_value"] = m.currentProject.AuthConfig.KeyValue
		authData["username"] = m.currentProject.AuthConfig.Username
		authData["password"] = m.currentProject.AuthConfig.Password
	}

	var exportResults []map[string]any
	for _, exportItem := range exportsSlice {
		exportMap, ok := exportItem.(map[string]any)
		if !ok {
			continue
		}

		format, _ := exportMap["format"].(string)
		outputPath, _ := exportMap["filepath"].(string)

		if format == "" || outputPath == "" {
			continue
		}

		resolvedPath, err := exporter.ResolveExportPath(outputPath)
		if err != nil {
			return toolResultMsg{
				toolID:   toolCall.ID,
				toolName: toolCall.Name,
				err:      fmt.Errorf("failed to resolve path %s: %w", outputPath, err),
			}
		}

		req := exporter.ExportRequest{
			BaseURL:  m.baseURL,
			Tests:    tests,
			FilePath: resolvedPath,
			AuthType: authType,
			AuthData: authData,
		}

		if err := exporter.Export(format, req); err != nil {
			return toolResultMsg{
				toolID:   toolCall.ID,
				toolName: toolCall.Name,
				err:      fmt.Errorf("export to %s failed: %w", format, err),
			}
		}

		exportResults = append(exportResults, map[string]any{
			"format":   format,
			"filepath": resolvedPath,
		})
	}

	return toolResultMsg{
		toolID:   toolCall.ID,
		toolName: toolCall.Name,
		result: map[string]any{
			"success":    true,
			"exports":    exportResults,
			"test_count": len(tests),
		},
	}
}
