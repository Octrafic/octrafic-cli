package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/Octrafic/octrafic-cli/internal/infra/storage"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProjectWithConversationsModel shows projects with conversation preview below
type ProjectWithConversationsModel struct {
	projects             []*storage.Project
	cursor               int
	selectedProject      *storage.Project
	conversationsPreview []*storage.Conversation
	err                  error
	cancelled            bool
}

// NewProjectWithConversationsModel creates a new project selector with conversation preview
func NewProjectWithConversationsModel(projects []*storage.Project) ProjectWithConversationsModel {
	m := ProjectWithConversationsModel{
		projects: projects,
		cursor:   0,
	}

	// Load conversations for first project
	if len(projects) > 0 {
		convs, _ := storage.ListConversations(projects[0].ID)
		m.conversationsPreview = convs
	}

	return m
}

func (m ProjectWithConversationsModel) Init() tea.Cmd {
	return nil
}

func (m ProjectWithConversationsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				// Load conversations for newly selected project
				if len(m.projects) > m.cursor {
					convs, _ := storage.ListConversations(m.projects[m.cursor].ID)
					m.conversationsPreview = convs
				}
			}

		case "down", "j":
			if m.cursor < len(m.projects)-1 {
				m.cursor++
				// Load conversations for newly selected project
				if len(m.projects) > m.cursor {
					convs, _ := storage.ListConversations(m.projects[m.cursor].ID)
					m.conversationsPreview = convs
				}
			}

		case "enter":
			if len(m.projects) > m.cursor {
				m.selectedProject = m.projects[m.cursor]
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m ProjectWithConversationsModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	var s strings.Builder

	// Title
	title := titleStyle.Render("Select a Project")
	s.WriteString(title)
	s.WriteString("\n\n")

	// Table header
	headerStyle := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true)
	s.WriteString("  ")
	s.WriteString(headerStyle.Render(fmt.Sprintf("%-30s %-42s %s", "NAME", "URL", "LAST USED")))
	s.WriteString("\n")

	// Projects list
	for i, project := range m.projects {
		style := lipgloss.NewStyle().Foreground(Theme.TextMuted)
		prefix := "  "
		if i == m.cursor {
			style = lipgloss.NewStyle().
				Foreground(Theme.PrimaryStrong).
				Background(Theme.BgSelected).
				Bold(true)
			prefix = "❯ "
		}

		timeAgo := formatTimeAgo(project.LastAccessedAt)

		// Format URL (remove protocol for cleaner look)
		url := project.BaseURL
		url = strings.TrimPrefix(url, "https://")
		url = strings.TrimPrefix(url, "http://")

		// Truncate if too long
		name := project.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}
		if len(url) > 42 {
			url = url[:39] + "..."
		}

		// Build table row
		data := fmt.Sprintf("%-30s %-42s %s", name, url, timeAgo)

		s.WriteString(prefix)
		s.WriteString(style.Render(data))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render("───── Conversations ─────"))
	s.WriteString("\n\n")

	// Conversations preview for selected project
	if len(m.conversationsPreview) == 0 {
		s.WriteString(lipgloss.NewStyle().Foreground(Theme.TextSubtle).Italic(true).Render("  No conversations yet"))
		s.WriteString("\n")
	} else {
		// Show max 3 most recent conversations
		maxShow := 3
		if len(m.conversationsPreview) < maxShow {
			maxShow = len(m.conversationsPreview)
		}

		for i := 0; i < maxShow; i++ {
			conv := m.conversationsPreview[i]
			timeAgo := formatTimeAgo(conv.UpdatedAt)
			title := conv.Title
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			line := fmt.Sprintf("  • %s %s", title, lipgloss.NewStyle().Foreground(Theme.TextSubtle).Render("("+timeAgo+")"))
			s.WriteString(lipgloss.NewStyle().Foreground(Theme.TextMuted).Render(line))
			s.WriteString("\n")
		}

		if len(m.conversationsPreview) > maxShow {
			remaining := len(m.conversationsPreview) - maxShow
			s.WriteString(lipgloss.NewStyle().Foreground(Theme.TextSubtle).Italic(true).Render(fmt.Sprintf("  ... and %d more", remaining)))
			s.WriteString("\n")
		}
	}

	s.WriteString("\n")
	help := helpStyle.Render("↑/↓: navigate • enter: select project • esc: cancel")
	s.WriteString(help)

	return s.String()
}

// IsCancelled returns true if user cancelled
func (m ProjectWithConversationsModel) IsCancelled() bool {
	return m.cancelled
}

// GetSelectedProject returns the selected project
func (m ProjectWithConversationsModel) GetSelectedProject() *storage.Project {
	return m.selectedProject
}

// ConversationListModel shows list of conversations for a project
type ConversationListModel struct {
	project       *storage.Project
	conversations []*storage.Conversation
	cursor        int
	selected      *storage.Conversation
	createNew     bool
	cancelled     bool
	err           error
}

// NewConversationListModel creates a new conversation selector
func NewConversationListModel(project *storage.Project) ConversationListModel {
	conversations, err := storage.ListConversations(project.ID)

	cursor := 0
	if len(conversations) > 0 {
		cursor = 1 // Start at first conversation if available
	}

	return ConversationListModel{
		project:       project,
		conversations: conversations,
		cursor:        cursor,
		err:           err,
	}
}

func (m ConversationListModel) Init() tea.Cmd {
	return nil
}

func (m ConversationListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			// +1 for "New conversation" option
			if m.cursor < len(m.conversations) {
				m.cursor++
			}

		case "enter":
			if m.cursor == 0 {
				// Create new conversation
				m.createNew = true
			} else {
				// Select existing conversation
				m.selected = m.conversations[m.cursor-1]
			}
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m ConversationListModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	var s strings.Builder

	// Title with project name
	title := titleStyle.Render(fmt.Sprintf("Select a Conversation - %s", m.project.Name))
	s.WriteString(title)
	s.WriteString("\n\n")

	// "New conversation" option
	cursor := " "
	style := itemStyle
	if m.cursor == 0 {
		cursor = "❯"
		style = selectedItemStyle
	}
	line := fmt.Sprintf("%s + Start new conversation", cursor)
	s.WriteString(style.Render(line))
	s.WriteString("\n\n")

	// Separator
	s.WriteString(lipgloss.NewStyle().Foreground(Theme.TextSubtle).Render("  ───────────────────────"))
	s.WriteString("\n\n")

	// Existing conversations
	if len(m.conversations) == 0 {
		s.WriteString(lipgloss.NewStyle().Foreground(Theme.TextSubtle).Italic(true).Render("  No previous conversations"))
		s.WriteString("\n")
	} else {
		for i, conv := range m.conversations {
			cursor := " "
			style := itemStyle
			if i+1 == m.cursor {
				cursor = "❯"
				style = selectedItemStyle
			}

			title := conv.Title
			if len(title) > 60 {
				title = title[:57] + "..."
			}

			timeAgo := formatTimeAgo(conv.UpdatedAt)
			line := fmt.Sprintf("%s %s", cursor, title)
			s.WriteString(style.Render(line))
			s.WriteString("\n")

			// Show timestamp below title if selected
			if i+1 == m.cursor {
				timeLine := fmt.Sprintf("     Last updated: %s", timeAgo)
				s.WriteString(lipgloss.NewStyle().Foreground(Theme.TextMuted).Render(timeLine))
				s.WriteString("\n")
			}
		}
	}

	s.WriteString("\n")
	help := helpStyle.Render("↑/↓: navigate • enter: select • esc: cancel")
	s.WriteString(help)

	return s.String()
}

// IsCancelled returns true if user cancelled
func (m ConversationListModel) IsCancelled() bool {
	return m.cancelled
}

// ShouldCreateNew returns true if user wants to create a new conversation
func (m ConversationListModel) ShouldCreateNew() bool {
	return m.createNew
}

// GetSelectedConversation returns the selected conversation
func (m ConversationListModel) GetSelectedConversation() *storage.Conversation {
	return m.selected
}

// formatTimeAgo formats time duration as human-readable string
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else if duration < 30*24*time.Hour {
		weeks := int(duration.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	} else if duration < 365*24*time.Hour {
		months := int(duration.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	} else {
		years := int(duration.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}
