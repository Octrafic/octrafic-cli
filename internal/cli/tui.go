package cli

import (
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/agents"
	"github.com/Octrafic/octrafic-cli/internal/config"
	"github.com/Octrafic/octrafic-cli/internal/core/analyzer"
	"github.com/Octrafic/octrafic-cli/internal/core/auth"
	"github.com/Octrafic/octrafic-cli/internal/core/tester"
	"github.com/Octrafic/octrafic-cli/internal/infra/storage"
	"github.com/Octrafic/octrafic-cli/internal/updater"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

type AgentState int

const (
	StateIdle AgentState = iota
	StateThinking
	StateUsingTool
	StateProcessing
	StateAskingConfirmation
	StateShowingCommands
	StateShowingTestPlan
	StateRunningTests
	StateWizard
)

type ExecutionMode int

const (
	ModeAsk ExecutionMode = iota
	ModeAutoExecute
)

type Command struct {
	Name        string
	Description string
}

var availableCommands = []Command{
	{Name: "/think", Description: "Toggle thinking mode (Ctrl+T)"},
	{Name: "/clear", Description: "Clear the conversation history"},
	{Name: "/help", Description: "Show help and available commands"},
	{Name: "/logout", Description: "Logout and clear session"},
	{Name: "/exit", Description: "Exit the application"},
	{Name: "/auth", Description: "Open authentication wizard"},
	{Name: "/info", Description: "Show current project info"},
	{Name: "/release-notes", Description: "Show latest release notes"},
}

type Test struct {
	ID          int
	Method      string
	Endpoint    string
	Description string
	Status      string
	Result      string
	BackendTest *agent.TestCase
	Selected    bool
}

type agentResponseMsg struct {
	message   string
	reasoning string
	toolCalls []agent.ToolCall
	err       error
}

type reasoningChunkMsg struct {
	chunk   string
	channel <-chan string
}

type streamReasoningMsg struct {
	channel <-chan string
}

type streamDoneMsg struct{}

type toolResultMsg struct {
	toolName string
	toolID   string // tool_use_id for FunctionResponse
	result   any
	err      error
}

type animationTickMsg time.Time

type processToolCallsMsg struct{}

type clearHintTimeoutMsg struct{}

type backendErrorMsg struct {
	err error
}

type generateTestPlanResultMsg struct {
	what         string
	focus        string
	backendTests []agent.Test
}

type showTestSelectionMsg struct {
	tests    []map[string]any
	toolCall agent.ToolCall
}

// TestUIModel represents the test UI state
type TestUIModel struct {
	analysis       *analyzer.Analysis
	baseURL        string
	specPath       string           // Path to spec file for SearchSpec
	currentProject *storage.Project // Currently active project
	localAgent     *agent.Agent
	testExecutor   *tester.Executor
	authProvider   auth.AuthProvider

	// Agent state
	agentState               AgentState
	executionMode            ExecutionMode
	thinkingEnabled          bool   // Whether to use /think tag for reasoning
	lastMessageRole          string // Track who sent the last message ("user" or "assistant")
	conversationHistory      []agent.ChatMessage
	conversationID           string // Current conversation UUID
	isLoadedConversation     bool   // Whether this is a loaded conversation
	currentToolCall          *agent.ToolCall
	pendingToolCall          *agent.ToolCall
	pendingTestGroupToolCall *agent.ToolCall  // Saved ExecuteTestGroup tool call for test selection
	streamedToolCalls        []agent.ToolCall // Tool calls received from stream, processed when DONE
	streamedAgentMessage     string           // Agent message received from stream, saved to history when DONE
	streamedReasoningChunk   string
	streamedTextChunk        string
	streamedInputTokens      int64 // Tokens for current streaming message
	streamedOutputTokens     int64 // Tokens for current streaming message
	confirmationChoice       int
	cancelStream             chan struct{} // Channel to cancel ongoing stream

	// Command suggestions
	filteredCommands     []Command
	selectedCommandIndex int

	// Wizard state
	wizardState *WizardState

	// Chat viewport
	viewport viewport.Model
	messages []string

	// Input
	textarea      textarea.Model
	lastEscPress  time.Time // Track last ESC press for double-ESC detection
	showClearHint bool      // Show "Press ESC again to clear" hint

	// Command history
	commandHistory []string // List of previous commands
	historyIndex   int      // Current position in history (-1 = not browsing)
	temporaryInput string   // Temporary storage for current input while browsing history

	// Spinner
	spinner spinner.Model

	// Animation frame counter
	animationFrame int

	// Token usage tracking
	inputTokens  int64
	outputTokens int64

	// Tests
	tests                   []Test
	selectedTestIndex       int
	totalTestsInProgress    int
	pendingTests            []map[string]any
	currentTestGroupLabel   string           // Header for test group (e.g., "Testing users api")
	testGroupCompletedCount int              // Number of tests completed in current group
	testGroupResults        []map[string]any // Results from current test group for FunctionResponse
	currentTestToolName     string           // Name of the tool being executed (e.g., "ExecuteTestGroup")
	currentTestToolID       string           // ID of the tool_use for FunctionResponse

	// Version
	currentVersion string
	latestVersion  string

	// Styles
	titleStyle   lipgloss.Style
	methodStyles map[string]lipgloss.Style
	successStyle lipgloss.Style
	errorStyle   lipgloss.Style
	helpStyle    lipgloss.Style
	subtleStyle  lipgloss.Style
	userStyle    lipgloss.Style
	agentStyle   lipgloss.Style
	toolStyle    lipgloss.Style
	borderStyle  lipgloss.Style

	width  int
	height int
}

func NewTestUIModel(baseURL string, specPath string, analysis *analyzer.Analysis, authProvider auth.AuthProvider, version string) *TestUIModel {
	// Textarea
	ta := textarea.New()
	ta.Placeholder = "Chat with the testing agent..."
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.SetHeight(1)
	ta.MaxHeight = 10
	ta.Prompt = ""
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(Theme.TextSubtle).Italic(true)

	// Viewport - will be resized on first WindowSizeMsg
	vp := viewport.New(120, 20)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	// Spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(Theme.Warning)

	// HTTP Method color schemes
	methodStyles := map[string]lipgloss.Style{
		"GET":    lipgloss.NewStyle().Foreground(Theme.Success).Bold(true),     // green
		"POST":   lipgloss.NewStyle().Foreground(Theme.PrimaryDark).Bold(true), // blue
		"PUT":    lipgloss.NewStyle().Foreground(Theme.Warning).Bold(true),     // orange
		"DELETE": lipgloss.NewStyle().Foreground(Theme.Error).Bold(true),       // red
		"PATCH":  lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true),     // light blue
	}

	model := &TestUIModel{
		analysis:            analysis,
		baseURL:             baseURL,
		specPath:            specPath,
		localAgent:          nil, // Will be initialized when needed
		testExecutor:        tester.NewExecutor(baseURL, authProvider),
		authProvider:        authProvider,
		agentState:          StateIdle,
		executionMode:       ModeAsk,
		thinkingEnabled:     true, // Thinking enabled by default
		lastMessageRole:     "",   // Empty = no messages yet, so first message will show label
		conversationHistory: []agent.ChatMessage{},
		viewport:            vp,
		messages:            []string{},
		textarea:            ta,
		spinner:             s,
		animationFrame:      0,
		tests:               []Test{},
		commandHistory:      []string{},
		historyIndex:        -1,
		temporaryInput:      "",
		titleStyle:          lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true),
		methodStyles:        methodStyles,
		successStyle:        lipgloss.NewStyle().Foreground(Theme.Success).Bold(true),
		errorStyle:          lipgloss.NewStyle().Foreground(Theme.Error).Bold(true),
		helpStyle:           lipgloss.NewStyle().Foreground(Theme.TextSubtle),
		subtleStyle:         lipgloss.NewStyle().Foreground(Theme.TextMuted),
		userStyle:           lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true),
		agentStyle:          lipgloss.NewStyle().Foreground(Theme.Primary),
		toolStyle:           lipgloss.NewStyle().Foreground(Theme.Warning),
		borderStyle:         lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Theme.Primary).Padding(0, 1),
	}

	model.currentVersion = version
	if cfg, err := config.Load(); err == nil && cfg.LatestVersion != "" && updater.IsNewer(cfg.LatestVersion, version) {
		model.latestVersion = cfg.LatestVersion
	}

	// Add header (logo + info)
	for _, line := range model.renderHeader() {
		model.addMessage(line)
	}

	// Add welcome message
	model.addMessage("Hi! I can help you test your API. You can ask me questions or tell me to run tests.")
	model.lastMessageRole = "assistant" // Mark that agent sent the welcome message

	return model
}

// Init initializes the model
func (m TestUIModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick, tea.SetWindowTitle("Octrafic"))
}

// renderTestPlanWithCheckboxes renders the interactive test plan with checkboxes
func renderTestPlanWithCheckboxes(m *TestUIModel) string {
	var s strings.Builder

	// Header
	bullet := lipgloss.NewStyle().Foreground(Theme.Primary).Render("➔")
	title := bullet + " Generated Test Plan"
	subtitle := lipgloss.NewStyle().Foreground(Theme.TextMuted).Render(fmt.Sprintf("(%d tests)", len(m.tests)))
	s.WriteString(title + " " + subtitle + "\n\n")

	// List of tests with checkboxes
	for i, test := range m.tests {
		// Checkbox indicator
		checkbox := "[ ]"
		if test.Selected {
			checkbox = lipgloss.NewStyle().Foreground(Theme.Success).Render("[✓]")
		}

		// Method styling
		methodStyle, ok := m.methodStyles[test.Method]
		if !ok {
			methodStyle = lipgloss.NewStyle().Foreground(Theme.TextSubtle)
		}

		// Line indicator for selected test
		indicator := "  "
		if i == m.selectedTestIndex {
			indicator = lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render(">>")
		}

		// Format test line
		method := methodStyle.Render(fmt.Sprintf("%-6s", test.Method))
		endpoint := lipgloss.NewStyle().Foreground(Theme.Text).Render(test.Endpoint)
		description := lipgloss.NewStyle().Foreground(Theme.TextMuted).Render(test.Description)

		s.WriteString(fmt.Sprintf("%s %s %s %s → %s\n", indicator, checkbox, method, endpoint, description))
	}

	// Help text
	s.WriteString("\n")
	helpText := lipgloss.NewStyle().Foreground(Theme.TextSubtle).Render(
		"↑/↓ navigate • Space toggle selection • Enter run selected tests • ESC cancel")
	s.WriteString(helpText)

	return s.String()
}

// View renders the UI
func (m TestUIModel) View() string {
	var s strings.Builder

	s.WriteString(m.viewport.View() + "\n")

	// Show wizard if active
	if m.agentState == StateWizard {
		s.WriteString("\n")
		s.WriteString(m.RenderWizard())
		s.WriteString("\n")
		return s.String()
	}

	// Show test plan with checkboxes
	if m.agentState == StateShowingTestPlan {
		s.WriteString(renderTestPlanWithCheckboxes(&m))
		s.WriteString("\n")
		return s.String()
	}

	// Show confirmation dialog or text input
	if m.agentState == StateAskingConfirmation {
		// Show tool confirmation dialog with details
		toolName := "Unknown"
		if m.pendingToolCall != nil {
			toolName = m.pendingToolCall.Name
		}

		s.WriteString(lipgloss.NewStyle().Foreground(Theme.Warning).Bold(true).Render("Execute tool: "+toolName) + "\n")

		// Show test details if executing a test
		if strings.HasPrefix(toolName, "ExecuteTest") {
			// Find next pending test to show details
			for _, test := range m.tests {
				if test.Status == "pending" {
					methodStyle, ok := m.methodStyles[test.Method]
					if !ok {
						methodStyle = lipgloss.NewStyle().Foreground(Theme.TextSubtle)
					}
					s.WriteString(m.subtleStyle.Render("Request: ") + methodStyle.Render(test.Method) + " " + test.Endpoint + "\n")
					if test.Description != "" {
						s.WriteString(m.subtleStyle.Render("Description: "+test.Description) + "\n")
					}
					break
				}
			}
		}

		s.WriteString("\n")
		yesStyle := lipgloss.NewStyle().Foreground(Theme.Success)
		noStyle := lipgloss.NewStyle().Foreground(Theme.Error)
		skipStyle := lipgloss.NewStyle().Foreground(Theme.TextSubtle)
		descStyle := lipgloss.NewStyle().Foreground(Theme.TextMuted)

		// Vertical list with 3 options and descriptions
		switch m.confirmationChoice {
		case 0:
			s.WriteString(yesStyle.Bold(true).Render("> Yes") + "\n")
			s.WriteString(descStyle.Render("  Execute this request and see the results") + "\n\n")
			s.WriteString(noStyle.Render("  No") + "\n")
			s.WriteString(descStyle.Render("  Cancel and stop all remaining tests") + "\n\n")
			s.WriteString(skipStyle.Render("  Skip") + "\n")
			s.WriteString(descStyle.Render("  Skip this test and continue to the next one") + "\n")
		case 1:
			s.WriteString(yesStyle.Render("  Yes") + "\n")
			s.WriteString(descStyle.Render("  Execute this request and see the results") + "\n\n")
			s.WriteString(noStyle.Bold(true).Render("> No") + "\n")
			s.WriteString(descStyle.Render("  Cancel and stop all remaining tests") + "\n\n")
			s.WriteString(skipStyle.Render("  Skip") + "\n")
			s.WriteString(descStyle.Render("  Skip this test and continue to the next one") + "\n")
		default:
			s.WriteString(yesStyle.Render("  Yes") + "\n")
			s.WriteString(descStyle.Render("  Execute this request and see the results") + "\n\n")
			s.WriteString(noStyle.Render("  No") + "\n")
			s.WriteString(descStyle.Render("  Cancel and stop all remaining tests") + "\n\n")
			s.WriteString(skipStyle.Bold(true).Render("> Skip") + "\n")
			s.WriteString(descStyle.Render("  Skip this test and continue to the next one") + "\n")
		}
		s.WriteString("\n")
		s.WriteString("\n")
		s.WriteString(m.helpStyle.Render("↑/↓ to select • Enter to confirm • ESC to cancel") + "\n")
	} else {
		// Add empty line before status bar
		s.WriteString("\n")

		// Status bar - always shown
		var icon string
		var statusMsg string
		if m.agentState == StateThinking || m.agentState == StateProcessing || m.agentState == StateRunningTests || m.agentState == StateUsingTool {
			icon = m.spinner.View()
			statusMsg = generateGradientText("Working...", m.animationFrame)
		} else if m.textarea.Value() == "" {
			icon = "○"
			statusMsg = "Write a message"
		} else {
			icon = "●"
			statusMsg = "Press enter to send"
		}

		// Token display
		tokensStyle := lipgloss.NewStyle().Foreground(Theme.TextMuted)
		tokenDisplay := tokensStyle.Render(fmt.Sprintf(" • ↑%d ↓%d", m.inputTokens, m.outputTokens))

		// Update indicator
		updateDisplay := ""
		if m.latestVersion != "" {
			updateDisplay = lipgloss.NewStyle().Foreground(Theme.Warning).Render(fmt.Sprintf(" • v%s available", m.latestVersion))
		}

		s.WriteString(icon + " " + statusMsg + tokenDisplay + updateDisplay + "\n")

		// Input AFTER status line (with border)
		s.WriteString(m.borderStyle.Render(m.textarea.View()) + "\n")

		// Command suggestions (if showing)
		if m.agentState == StateShowingCommands && len(m.filteredCommands) > 0 {
			s.WriteString("\n")

			// Styles for command list
			selectedStyle := lipgloss.NewStyle().
				Foreground(Theme.Primary).
				Background(Theme.BgDark).
				Bold(true)
			normalStyle := lipgloss.NewStyle().
				Foreground(Theme.TextMuted)
			descStyle := lipgloss.NewStyle().
				Foreground(Theme.TextSubtle)

			for i, cmd := range m.filteredCommands {
				if i == m.selectedCommandIndex {
					// Selected command
					s.WriteString(selectedStyle.Render("> "+cmd.Name) + " ")
					s.WriteString(descStyle.Render(cmd.Description) + "\n")
				} else {
					// Normal command
					s.WriteString(normalStyle.Render("  "+cmd.Name) + " ")
					s.WriteString(descStyle.Render(cmd.Description) + "\n")
				}
			}

			s.WriteString("\n")
			s.WriteString(lipgloss.NewStyle().Foreground(Theme.TextSubtle).
				Render("↑/↓ navigate • Enter select • ESC cancel") + "\n")
		}

		// Help text BELOW input (only when idle)
		if m.agentState == StateIdle {
			helpText := ""
			// Show ESC clear hint if active
			if m.showClearHint {
				helpText = lipgloss.NewStyle().Foreground(Theme.Warning).Render("Press ESC again to clear input")
			} else {
				if m.thinkingEnabled {
					helpText += lipgloss.NewStyle().Foreground(Theme.Violet).Render("Think") + " • "
				}
				helpText += "Ctrl+T thinking • Ctrl+C to quit"
			}
			// Wrap help text if too long
			if m.width > 0 {
				helpText = wordwrap.String(helpText, m.width-4)
			}
			s.WriteString(m.helpStyle.Render(helpText) + "\n")
		}
	}

	return s.String()
}
