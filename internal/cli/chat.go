package cli

import (
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/core/analyzer"
	"strings"

	"github.com/Octrafic/octrafic-cli/internal/ui/textarea"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

const gap = "\n\n"

// keyMap defines keybindings for the chat interface
type keyMap struct {
	ToggleMode key.Binding
}

// ShortHelp returns keybindings to be shown in mini help view
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.ToggleMode}
}

// FullHelp returns all keybindings
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.ToggleMode},
	}
}

var keys = keyMap{
	ToggleMode: key.NewBinding(
		key.WithKeys("ctrl+t", "alt+m"), // Ctrl+T and Alt+M as alternatives
		key.WithHelp("ctrl+t", "toggle mode"),
	),
}

// Model represents the state of our chat-based interactive application
type ChatModel struct {
	analysis       *analyzer.Analysis
	baseURL        string
	viewport       viewport.Model
	textarea       textarea.Model
	messages       []string
	currentMode    string // "ask" or "test"
	keys           keyMap
	senderStyle    lipgloss.Style
	systemStyle    lipgloss.Style
	errorStyle     lipgloss.Style
	modeStyle      lipgloss.Style
	agentModeStyle lipgloss.Style
	askModeStyle   lipgloss.Style
	width          int
	height         int
}

// NewChatModel creates a new chat-based interactive model
func NewChatModel(baseURL string, analysis *analyzer.Analysis) *ChatModel {
	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 500
	ta.SetWidth(50)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.MaxVisualHeight = 10
	ta.KeyMap.InsertNewline.SetEnabled(false)

	// Initialize viewport with mouse wheel support
	vp := viewport.New(50, 10)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3 // Scroll 3 lines per wheel tick
	vp.SetContent(`Welcome to API Tester!

Type your questions or commands and press Enter.
Press ` + keys.ToggleMode.Help().Key + ` to toggle between ASK and AGENT modes.

Available commands:
  endpoints  - List all API endpoints
  insights   - Show AI-generated insights
  help       - Show this help message
  clear      - Clear screen
  exit       - Exit interactive mode`)

	// Initialize styles
	senderStyle := lipgloss.NewStyle().Foreground(Theme.PrimaryDark)            // Blue-500
	systemStyle := lipgloss.NewStyle().Foreground(Theme.Primary)                // Blue-400
	errorStyle := lipgloss.NewStyle().Foreground(Theme.Error)                   // Red-500
	modeStyle := lipgloss.NewStyle().Foreground(Theme.PrimaryStrong).Bold(true) // Blue-600

	// Mode-specific colors
	agentModeStyle := lipgloss.NewStyle().Foreground(Theme.Success).Bold(true)   // Emerald-500 (green)
	askModeStyle := lipgloss.NewStyle().Foreground(Theme.PrimaryDark).Bold(true) // Blue-500 (blue)

	return &ChatModel{
		analysis:       analysis,
		baseURL:        baseURL,
		viewport:       vp,
		textarea:       ta,
		messages:       []string{},
		currentMode:    "agent", // Agent mode is now default
		keys:           keys,
		senderStyle:    senderStyle,
		systemStyle:    systemStyle,
		errorStyle:     errorStyle,
		modeStyle:      modeStyle,
		agentModeStyle: agentModeStyle,
		askModeStyle:   askModeStyle,
	}
}

// Init initializes the model
func (m ChatModel) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages and updates the model
func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.textarea.SetWidth(msg.Width)
		m.viewport.Height = msg.Height - m.textarea.Height() - lipgloss.Height(gap)

		if len(m.messages) > 0 {
			m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(strings.Join(m.messages, "\n")))
		}
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		// Handle key bindings
		if key.Matches(msg, m.keys.ToggleMode) {
			m.toggleMode()
			m.updateViewport()
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.textarea.Value() != "" {
				m.handleInput(m.textarea.Value())
				m.textarea.Reset()
				m.updateViewport()
			}
		}
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

// View renders the UI
func (m ChatModel) View() string {
	modeIndicator := m.askModeStyle.Render("ASK")
	if m.currentMode == "agent" {
		modeIndicator = m.agentModeStyle.Render("AGENT")
	}

	return fmt.Sprintf(
		"%s%s%s\n%s %s",
		m.viewport.View(),
		gap,
		m.textarea.View(),
		modeIndicator,
		lipgloss.NewStyle().Foreground(Theme.TextSubtle).Render(fmt.Sprintf("%s to toggle", m.keys.ToggleMode.Help().Key)),
	)
}

// toggleMode switches between Ask and Agent modes
func (m *ChatModel) toggleMode() {
	if m.currentMode == "agent" {
		m.currentMode = "ask"
		m.addSystemMessage("Switched to ASK mode")
	} else {
		m.currentMode = "agent"
		m.addSystemMessage("Switched to AGENT mode")
	}
}

// handleInput processes user input
func (m *ChatModel) handleInput(input string) {
	input = strings.TrimSpace(input)
	m.addUserMessage(input)

	switch input {
	case "exit", "quit":
		m.addSystemMessage("Goodbye!")
		// This will trigger tea.Quit in the next update
	case "help":
		m.showHelp()
	case "endpoints":
		m.listEndpoints()
	case "insights":
		m.showInsights()
	case "clear":
		m.clearMessages()
	default:
		m.handleUserPrompt(input)
	}
}

// addUserMessage adds a user message to the chat
func (m *ChatModel) addUserMessage(message string) {
	modePrefix := "ASK "
	modeColor := m.askModeStyle
	if m.currentMode == "agent" {
		modePrefix = "AGENT "
		modeColor = m.agentModeStyle
	}
	m.messages = append(m.messages, modeColor.Render(modePrefix+"You:")+" "+message)
}

// addSystemMessage adds a system message to the chat
func (m *ChatModel) addSystemMessage(message string) {
	m.messages = append(m.messages, m.systemStyle.Render("System:")+" "+message)
}

// updateViewport updates the viewport content
func (m *ChatModel) updateViewport() {
	if len(m.messages) > 0 {
		// Wrap each message to fit viewport width
		wrappedMessages := make([]string, len(m.messages))
		maxWidth := m.viewport.Width
		if maxWidth <= 0 {
			maxWidth = 80 // default width
		}

		for i, msg := range m.messages {
			// Wrap long lines to viewport width
			wrappedMessages[i] = wordwrap.String(msg, maxWidth)
		}

		content := strings.Join(wrappedMessages, "\n")
		m.viewport.SetContent(content)
		m.viewport.GotoBottom()
	}
}

// showHelp displays available commands
func (m *ChatModel) showHelp() {
	modeHelp := "ASK Mode: Ask questions about the API"
	if m.currentMode == "agent" {
		modeHelp = "AGENT Mode: Generate and run API tests"
	}

	helpMessage := fmt.Sprintf(`%s

Available commands:
  endpoints  - List all API endpoints
  insights   - Show AI-generated insights
  clear      - Clear screen
  help       - Show this help message
  exit       - Exit interactive mode

Mode switching:
  %s  - Toggle between ASK and AGENT modes`, modeHelp, m.keys.ToggleMode.Help().Key)

	m.addSystemMessage(helpMessage)
}

// listEndpoints displays available endpoints
func (m *ChatModel) listEndpoints() {
	if len(m.analysis.EndpointInfo) == 0 {
		m.addSystemMessage("No endpoints found.")
		return
	}

	var endpointList strings.Builder
	endpointList.WriteString("Available endpoints:")

	for key, info := range m.analysis.EndpointInfo {
		endpointList.WriteString(fmt.Sprintf("\n  %s\n", key))
		endpointList.WriteString(fmt.Sprintf("    Purpose: %s\n", info.Purpose))
		if len(info.TestScenarios) > 0 {
			endpointList.WriteString(fmt.Sprintf("    Test scenarios: %d\n", len(info.TestScenarios)))
		}
	}

	m.addSystemMessage(endpointList.String())
}

// showInsights displays AI insights
func (m *ChatModel) showInsights() {
	if len(m.analysis.Insights) == 0 {
		m.addSystemMessage("No insights available.")
		return
	}

	var insightsList strings.Builder
	insightsList.WriteString("AI Insights:")

	for i, insight := range m.analysis.Insights {
		insightsList.WriteString(fmt.Sprintf("\n  %d. %s", i+1, insight))
	}

	m.addSystemMessage(insightsList.String())
}

// clearMessages clears the message history
func (m *ChatModel) clearMessages() {
	m.messages = []string{}
	m.viewport.SetContent("Message history cleared.")
}

// handleUserPrompt handles general user questions
func (m *ChatModel) handleUserPrompt(prompt string) {
	if m.currentMode == "agent" {
		m.handleAgentPrompt(prompt)
	} else {
		m.handleAskPrompt(prompt)
	}
}

// handleAskPrompt handles questions in ASK mode
func (m *ChatModel) handleAskPrompt(prompt string) {
	m.addSystemMessage("AI: [MOCKUP MODE]")

	lowerPrompt := strings.ToLower(prompt)

	if strings.Contains(lowerPrompt, "test") || strings.Contains(lowerPrompt, "how") {
		m.addSystemMessage("Analyzing your question and providing detailed testing guidance.")
		m.addSystemMessage(fmt.Sprintf("Question: '%s'", prompt))
		m.addSystemMessage("Suggested testing approach:\n1. Start with authentication endpoints\n2. Test happy path scenarios first\n3. Then test edge cases and error handling")
	} else if strings.Contains(lowerPrompt, "endpoint") {
		m.addSystemMessage("Providing information about specific endpoints.")
		m.addSystemMessage(fmt.Sprintf("Found %d endpoints in the specification.", len(m.analysis.EndpointInfo)))
	} else {
		m.addSystemMessage("Analyzing your question using the API specification context.")
		m.addSystemMessage(fmt.Sprintf("Context includes: %d endpoints, analyzed at %s",
			len(m.analysis.EndpointInfo),
			m.analysis.Timestamp.Format("15:04:05")))
	}

	m.addSystemMessage("[Production: This would send your question + API analysis to OpenAI]")
}

// handleAgentPrompt handles commands in AGENT mode
func (m *ChatModel) handleAgentPrompt(prompt string) {
	m.addSystemMessage("AGENT: [MOCKUP MODE]")

	lowerPrompt := strings.ToLower(prompt)

	if strings.Contains(lowerPrompt, "generate") || strings.Contains(lowerPrompt, "create") {
		m.addSystemMessage("Generating test cases for the API.")
		m.addSystemMessage(fmt.Sprintf("Test generation request: '%s'", prompt))
		m.addSystemMessage("Generated test scenarios:\n1. Authentication flow tests\n2. Endpoint validation tests\n3. Error handling tests\n4. Performance benchmark tests")
	} else if strings.Contains(lowerPrompt, "run") || strings.Contains(lowerPrompt, "execute") {
		m.addSystemMessage("Executing API tests.")
		m.addSystemMessage(fmt.Sprintf("Test execution command: '%s'", prompt))
		m.addSystemMessage("Test execution results:\n✓ Authentication tests: PASSED\n✓ Endpoint validation: PASSED\n⚠ Error handling: 2 warnings\n✓ Performance tests: PASSED")
	} else {
		m.addSystemMessage("Analyzing test requirements.")
		m.addSystemMessage(fmt.Sprintf("Test analysis: '%s'", prompt))
		m.addSystemMessage(fmt.Sprintf("Context includes: %d endpoints for testing",
			len(m.analysis.EndpointInfo)))
	}

	m.addSystemMessage("[Production: This would generate and execute actual API tests]")
}
