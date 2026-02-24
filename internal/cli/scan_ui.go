package cli

import (
	"fmt"
	"strings"

	"github.com/Octrafic/octrafic-cli/internal/scanner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ScanProgressMsg is sent by the scanner to update the UI
type ScanProgressMsg struct {
	Message string
}

// ScanDoneMsg is sent when scanning is complete
type ScanDoneMsg struct {
	Error error
}

// ScanUIModel represents the state of the scanning TUI
type ScanUIModel struct {
	scanner      *scanner.Scanner
	status       string
	spinnerFrame int
	err          error
	done         bool
}

// NewScanUIModel creates a new ScanUIModel
func NewScanUIModel(s *scanner.Scanner) ScanUIModel {
	return ScanUIModel{
		scanner:      s,
		status:       "Initializing scan...",
		spinnerFrame: 0,
		done:         false,
	}
}

// Init starts the animation ticker
func (m ScanUIModel) Init() tea.Cmd {
	return animationTick()
}

// Update handles messages from the system and the background scanner
func (m ScanUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	case animationTickMsg:
		m.spinnerFrame++
		if !m.done {
			return m, animationTick()
		}
		return m, nil
	case ScanProgressMsg:
		msgStr := msg.Message
		var formattedMsg string

		if strings.HasPrefix(msgStr, "➔") {
			// Main pipeline stages
			bullet := lipgloss.NewStyle().Foreground(Theme.Primary).Render("➔")
			text := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render(strings.TrimSpace(strings.TrimPrefix(msgStr, "➔")))
			formattedMsg = fmt.Sprintf("  %s %s\n", bullet, text)
		} else if strings.HasPrefix(msgStr, "  ↳") {
			// Sub-steps
			bullet := lipgloss.NewStyle().Foreground(Theme.Cyan).Render("↳")
			text := lipgloss.NewStyle().Foreground(Theme.TextSubtle).Render(strings.TrimSpace(strings.TrimPrefix(msgStr, "  ↳")))
			formattedMsg = fmt.Sprintf("    %s %s\n", bullet, text)
		} else if strings.HasPrefix(msgStr, "    ✓") {
			// Success sub-hits
			bullet := lipgloss.NewStyle().Foreground(Theme.Success).Render("✓")
			text := lipgloss.NewStyle().Foreground(Theme.TextSubtle).Render(strings.TrimSpace(strings.TrimPrefix(msgStr, "    ✓")))
			formattedMsg = fmt.Sprintf("      %s %s\n", bullet, text)
		} else if strings.HasPrefix(msgStr, "✅") {
			// Final success
			formattedMsg = "\n" + lipgloss.NewStyle().Foreground(Theme.Success).Bold(true).Render("✅ "+strings.TrimSpace(strings.TrimPrefix(msgStr, "✅"))) + "\n"
		} else {
			formattedMsg = lipgloss.NewStyle().Foreground(Theme.Text).Render(msgStr) + "\n"
		}

		return m, tea.Printf("%s", strings.TrimRight(formattedMsg, "\n"))
	case ScanDoneMsg:
		m.done = true
		if msg.Error != nil {
			m.err = msg.Error
			errorStyle := lipgloss.NewStyle().Foreground(Theme.Error).Bold(true)
			return m, tea.Sequence(
				tea.Printf("%s", errorStyle.Render("\n❌ Error during scan:\n")+lipgloss.NewStyle().Foreground(Theme.TextMuted).Render(m.err.Error())),
				tea.Quit,
			)
		}

		successStyle := lipgloss.NewStyle().Foreground(Theme.Success).Bold(true)
		return m, tea.Sequence(
			tea.Printf("%s", "\n"+successStyle.Render("✅ Scan complete!")+"\n"+lipgloss.NewStyle().Foreground(Theme.TextSubtle).Render("Generated specification saved output file.")),
			tea.Quit,
		)
	}
	return m, nil
}

// View renders the current state of the scanner UI
func (m ScanUIModel) View() string {
	return ""
}

// StartScannerUI runs the scanner wrapped in a BubbleTea program.
func StartScannerUI(s *scanner.Scanner) error {
	fmt.Println(lipgloss.NewStyle().Margin(1, 2).Render(RenderLogo()))
	fmt.Println()

	m := NewScanUIModel(s)
	p := tea.NewProgram(m)

	go func() {
		err := s.RunScan(func(msg string) {
			p.Send(ScanProgressMsg{Message: msg})
		})
		p.Send(ScanDoneMsg{Error: err})
	}()

	_, err := p.Run()
	return err
}
