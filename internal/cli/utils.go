package cli

import (
	"fmt"
	"strings"

	"github.com/Octrafic/octrafic-cli/internal/agents"
	"github.com/Octrafic/octrafic-cli/internal/infra/storage"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

func renderUserLabel() string {
	return lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render(">")
}

func filterCommands(input string) []Command {
	var filtered []Command
	lowerInput := strings.ToLower(input)

	for _, cmd := range availableCommands {
		if strings.HasPrefix(strings.ToLower(cmd.Name), lowerInput) {
			filtered = append(filtered, cmd)
		}
	}

	return filtered
}

// renderHeader creates and returns header lines (logo + info)
func (m *TestUIModel) renderHeader() []string {
	subtleColor := Theme.TextMuted
	valueColor := Theme.Cyan

	logo := strings.Split(Logo, "\n")

	styledLogo := make([]string, len(logo))
	for i, line := range logo {
		var styledLine strings.Builder
		for _, char := range line {
			if char == '░' {
				styledLine.WriteString(lipgloss.NewStyle().Foreground(Theme.TextSubtle).Render(string(char)))
			} else {
				color := Theme.LogoGradient[i%len(Theme.LogoGradient)]
				styledLine.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(string(char)))
			}
		}
		styledLogo[i] = styledLine.String()
	}

	infoLine := lipgloss.NewStyle().Foreground(subtleColor).Render("Testing: ") +
		lipgloss.NewStyle().Foreground(valueColor).Render(m.baseURL)

	var lines []string
	lines = append(lines, "")
	lines = append(lines, styledLogo...)
	lines = append(lines, "")
	lines = append(lines, infoLine)
	lines = append(lines, lipgloss.NewStyle().Foreground(subtleColor).Render("──────────────────────────────────────────────────────────────────────"))
	lines = append(lines, "")

	return lines
}

func (m *TestUIModel) recreateHeader() tea.Cmd {
	// Note: This is called after /clear command to show welcome message again
	m.messages = []string{}

	// Add header
	for _, line := range m.renderHeader() {
		m.addMessage(line)
	}

	// Add welcome message
	m.addMessage("Hi! I can help you test your API. You can ask me questions or tell me to run tests.")
	m.addMessage("")
	m.addMessage("")
	m.lastMessageRole = "assistant"

	return nil
}

func (m *TestUIModel) shouldAskForConfirmation(toolName string) bool {
	// Tools that are safe and don't need confirmation
	// ExecuteTestGroup is safe - user already approved the plan via checkboxes
	safeTools := map[string]bool{
		"GenerateTestPlan": true, // Planning is safe, doesn't execute anything
		"ExecuteTestGroup": true, // Plan was already approved via checkboxes
		"GenerateReport":   true, // Generating a report is safe
	}

	return !safeTools[toolName]
}

func (m *TestUIModel) addMessage(msg string) tea.Cmd {
	m.messages = append(m.messages, msg)
	m.updateViewport()
	return nil
}

func (m *TestUIModel) updateViewport() {
	wrappedMessages := make([]string, len(m.messages))
	maxWidth := m.viewport.Width
	if maxWidth <= 0 {
		maxWidth = 80 // default width
	}

	for i, msg := range m.messages {
		wrappedMessages[i] = wordwrap.String(msg, maxWidth)
	}

	content := strings.Join(wrappedMessages, "\n")
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func renderMarkdown(content string) string {
	// Normalize line endings (GitHub API returns \r\n)
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	lines := strings.Split(content, "\n")
	var rendered []string

	boldStyle := lipgloss.NewStyle().Bold(true)
	codeStyle := lipgloss.NewStyle().Foreground(Theme.Success).Background(Theme.BgCode)
	headerStyle := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true)

	for _, line := range lines {
		result := line
		isHeader := false

		// Headers - remove hashtags and style
		if strings.HasPrefix(line, "###### ") {
			result = headerStyle.Render(strings.TrimPrefix(line, "###### "))
			isHeader = true
		} else if strings.HasPrefix(line, "##### ") {
			result = headerStyle.Render(strings.TrimPrefix(line, "##### "))
			isHeader = true
		} else if strings.HasPrefix(line, "#### ") {
			result = headerStyle.Render(strings.TrimPrefix(line, "#### "))
			isHeader = true
		} else if strings.HasPrefix(line, "### ") {
			result = headerStyle.Render(strings.TrimPrefix(line, "### "))
			isHeader = true
		} else if strings.HasPrefix(line, "## ") {
			result = headerStyle.Render(strings.TrimPrefix(line, "## "))
			isHeader = true
		} else if strings.HasPrefix(line, "# ") {
			result = headerStyle.Render(strings.TrimPrefix(line, "# "))
			isHeader = true
		}

		// Skip further processing for headers
		if isHeader {
			rendered = append(rendered, result)
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(result), "- ") || strings.HasPrefix(strings.TrimSpace(result), "* ") {
			indent := len(result) - len(strings.TrimSpace(result))
			result = strings.Repeat(" ", indent) + "• " + strings.TrimSpace(result)[2:]
		}

		for {
			start := strings.Index(result, "**")
			if start == -1 {
				break
			}
			end := strings.Index(result[start+2:], "**")
			if end == -1 {
				break
			}
			end += start + 2
			text := result[start+2 : end]
			result = result[:start] + boldStyle.Render(text) + result[end+2:]
		}

		for {
			start := strings.Index(result, "`")
			if start == -1 {
				break
			}
			end := strings.Index(result[start+1:], "`")
			if end == -1 {
				break
			}
			end += start + 1
			text := result[start+1 : end]
			result = result[:start] + codeStyle.Render(text) + result[end+1:]
		}

		rendered = append(rendered, result)
	}

	return strings.Join(rendered, "\n")
}

// showToolMessage displays a tool message with consistent formatting
// Always adds empty line before, bullet point with title, and optional details
func (m *TestUIModel) showToolMessage(title string, details string) {
	m.addMessage("")
	bullet := lipgloss.NewStyle().Foreground(Theme.Primary).Render("➔")
	m.addMessage(fmt.Sprintf("%s %s", bullet, title))
	if details != "" {
		m.addMessage(m.subtleStyle.Render("    " + details))
	}
}

func FormatToolResult(title string, details []string) []string {
	var messages []string

	bullet := lipgloss.NewStyle().Foreground(Theme.Primary).Render("➔")
	messages = append(messages, fmt.Sprintf("%s %s", bullet, title))

	subtleStyle := lipgloss.NewStyle().Foreground(Theme.Gray)
	for _, detail := range details {
		indented := fmt.Sprintf("    %s", detail)
		messages = append(messages, subtleStyle.Render(indented))
	}

	return messages
}

// saveMessageToConversation saves a message to the current conversation database
func (m *TestUIModel) saveMessageToConversation(messageType, content string, metadata map[string]interface{}) {
	// Skip if no conversation or temporary project
	if m.conversationID == "" || m.currentProject == nil || m.currentProject.IsTemporary {
		return
	}

	_ = storage.SaveMessage(m.currentProject.ID, m.conversationID, messageType, content, metadata)
}

func (m *TestUIModel) saveChatMessageToConversation(msg agent.ChatMessage) {
	if m.conversationID == "" || m.currentProject == nil || m.currentProject.IsTemporary {
		return
	}

	if msg.FunctionResponse != nil {
		toolContent := fmt.Sprintf("Tool: %s", msg.FunctionResponse.Name)
		metadata := make(map[string]interface{})
		metadata["tool_name"] = msg.FunctionResponse.Name
		metadata["tool_id"] = msg.FunctionResponse.ID
		if msg.FunctionResponse.Response != nil {
			metadata["tool_output"] = msg.FunctionResponse.Response
		}
		_ = storage.SaveMessage(m.currentProject.ID, m.conversationID, "tool", toolContent, metadata)
		return
	}

	metadata := make(map[string]interface{})
	if msg.ReasoningContent != "" {
		metadata["reasoning"] = msg.ReasoningContent
	}
	if len(msg.FunctionCalls) > 0 {
		metadata["tool_calls"] = msg.FunctionCalls
	}
	if msg.InputTokens > 0 {
		metadata["input_tokens"] = msg.InputTokens
	}
	if msg.OutputTokens > 0 {
		metadata["output_tokens"] = msg.OutputTokens
	}
	_ = storage.SaveMessage(m.currentProject.ID, m.conversationID, msg.Role, msg.Content, metadata)
}

// saveUserMessage wraps addMessage and saves user message to conversation
func (m *TestUIModel) saveUserMessage(content string) {
	m.addMessage("")
	m.addMessage(renderUserLabel() + " " + content)
	m.addMessage("")

	m.saveMessageToConversation("user", content, nil)
}

// saveAssistantMessage saves assistant message to conversation
func (m *TestUIModel) saveAssistantMessage(content string) {
	m.saveMessageToConversation("assistant", content, nil)
}

// loadConversationHistory loads and replays conversation history
func (m *TestUIModel) loadConversationHistory() error {
	if m.conversationID == "" || !m.isLoadedConversation || m.currentProject == nil {
		return nil
	}

	messages, err := storage.GetMessages(m.currentProject.ID, m.conversationID)
	if err != nil {
		return err
	}

	// Clear welcome messages first
	m.messages = []string{}

	// Add header (same as new conversation)
	for _, line := range m.renderHeader() {
		m.addMessage(line)
	}

	// Replay messages
	for i, msg := range messages {
		var chatMsg agent.ChatMessage

		// Check if next message is a tool (for spacing)
		nextIsToolMessage := false
		if i+1 < len(messages) && messages[i+1].Type == "tool" {
			nextIsToolMessage = true
		}

		// Handle different message types
		switch msg.Type {
		case "tool":
			chatMsg = agent.ChatMessage{
				Role: "user",
			}

			if msg.Metadata != nil {
				chatMsg.FunctionResponse = &agent.FunctionResponseData{
					ID:   getString(msg.Metadata, "tool_id"),
					Name: getString(msg.Metadata, "tool_name"),
				}
				if output, ok := msg.Metadata["tool_output"].(map[string]interface{}); ok {
					chatMsg.FunctionResponse.Response = output
				}
			}

			m.conversationHistory = append(m.conversationHistory, chatMsg)

			toolName := getString(msg.Metadata, "tool_name")
			if toolName != "" {
				displayName := ""
				switch toolName {
				case "get_endpoints_details":
					displayName = "Getting endpoint details"
				case "GenerateTestPlan":
					displayName = "Generated test cases"
				case "ExecuteTestGroup":
					displayName = "Executing tests"
				case "GenerateReport":
					displayName = "Generating PDF report"
				case "ExecuteTest":
					displayName = "Executing test"
				default:
					displayName = fmt.Sprintf("Tool: %s", toolName)
				}

				details := ""
				if output, ok := msg.Metadata["tool_output"].(map[string]interface{}); ok {
					if count, ok := output["count"].(float64); ok {
						details = fmt.Sprintf("Completed %d tests", int(count))
					}
				}

				m.showToolMessage(displayName, details)
			}
			continue

		case "user":
			if msg.Content == "" {
				continue
			}

			chatMsg = agent.ChatMessage{
				Role:    "user",
				Content: msg.Content,
			}

			userMessage := lipgloss.NewStyle().
				Foreground(Theme.TextMuted).
				Render("> ") + msg.Content
			m.addMessage("")
			m.addMessage(userMessage)

		case "assistant":
			chatMsg = agent.ChatMessage{
				Role:    "assistant",
				Content: msg.Content,
			}

			if msg.Metadata != nil {
				if reasoning, ok := msg.Metadata["reasoning"].(string); ok {
					chatMsg.ReasoningContent = reasoning
				}

				if toolCalls, ok := msg.Metadata["tool_calls"].([]interface{}); ok {
					for _, tc := range toolCalls {
						if tcMap, ok := tc.(map[string]interface{}); ok {
							toolCall := agent.ToolCall{
								ID:   getString(tcMap, "id"),
								Name: getString(tcMap, "name"),
							}
							if args, ok := tcMap["arguments"].(map[string]interface{}); ok {
								toolCall.Arguments = args
							}
							chatMsg.FunctionCalls = append(chatMsg.FunctionCalls, toolCall)
						}
					}
				}

				if toolResults, ok := msg.Metadata["tool_results"].(map[string]interface{}); ok {
					chatMsg.FunctionResponse = &agent.FunctionResponseData{
						ID:   getString(toolResults, "id"),
						Name: getString(toolResults, "name"),
					}
					if response, ok := toolResults["response"].(map[string]interface{}); ok {
						chatMsg.FunctionResponse.Response = response
					}
				}

				// Load tokens from metadata
				if inputTokens, ok := msg.Metadata["input_tokens"].(float64); ok {
					chatMsg.InputTokens = int64(inputTokens)
					m.inputTokens += int64(inputTokens)
				}
				if outputTokens, ok := msg.Metadata["output_tokens"].(float64); ok {
					chatMsg.OutputTokens = int64(outputTokens)
					m.outputTokens += int64(outputTokens)
				}
			}

			rendered := renderMarkdown(msg.Content)
			m.addMessage("")
			m.addMessage(rendered)
			if !nextIsToolMessage {
				m.addMessage("")
			}
			m.lastMessageRole = "assistant"

		default:
			continue
		}

		m.conversationHistory = append(m.conversationHistory, chatMsg)
	}

	return nil
}

// getString safely gets string from map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
