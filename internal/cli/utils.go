package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

func renderAgentLabel() string {
	text := "Agent:"
	// Theme.Primary (#38BDF8), Theme.Blue (#60A5FA), Theme.Indigo (#818CF8)
	gradientColors := []lipgloss.Color{Theme.Primary, Theme.Blue, Theme.Indigo}

	result := ""
	for i, char := range text {
		colorIndex := (i * len(gradientColors)) / len(text)
		if colorIndex >= len(gradientColors) {
			colorIndex = len(gradientColors) - 1
		}
		style := lipgloss.NewStyle().
			Foreground(gradientColors[colorIndex]).
			Bold(true)
		result += style.Render(string(char))
	}

	return result
}

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

func (m *TestUIModel) recreateHeader() tea.Cmd {
	// Note: This is called after /clear command to show welcome message again
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

	m.messages = []string{}
	m.addMessage("")
	for _, line := range styledLogo {
		m.addMessage(line)
	}
	m.addMessage("")
	m.addMessage(infoLine)
	m.addMessage(lipgloss.NewStyle().Foreground(subtleColor).Render("──────────────────────────────────────────────────────────────────────"))
	m.addMessage("")
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
