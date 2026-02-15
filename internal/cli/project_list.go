package cli

import (
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/infra/storage"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Theme.PrimaryDark).
			MarginTop(1).
			MarginBottom(1)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(Theme.TextMuted)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(Theme.PrimaryStrong).
				Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(Theme.TextSubtle).
			MarginTop(1)

	searchStyle = lipgloss.NewStyle().
			Foreground(Theme.PrimaryDark)
)

// ProjectListModel represents the interactive project list UI
type ProjectListModel struct {
	projects         []*storage.Project
	filteredProjects []*storage.Project
	cursor           int
	searchInput      textinput.Model
	searching        bool
	selected         *storage.Project
	createNew        bool // Set to true when user selects "Create new project"
	err              error
}

func NewProjectListModel(projects []*storage.Project) ProjectListModel {
	ti := textinput.New()
	ti.Placeholder = "Search projects..."
	ti.CharLimit = 50

	// Default cursor to first project (position 1), or "Create new" (position 0) if no projects
	cursor := 1
	if len(projects) == 0 {
		cursor = 0
	}

	return ProjectListModel{
		projects:         projects,
		filteredProjects: projects,
		cursor:           cursor,
		searchInput:      ti,
		searching:        false,
	}
}

func (m ProjectListModel) Init() tea.Cmd {
	return nil
}

func (m ProjectListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If searching, handle search input
		if m.searching {
			switch msg.String() {
			case "esc":
				m.searching = false
				m.searchInput.Blur()
				m.searchInput.SetValue("")
				m.filteredProjects = m.projects
				// Reset cursor to first project (or "Create new" if no projects)
				m.cursor = 1
				if len(m.filteredProjects) == 0 {
					m.cursor = 0
				}
				return m, nil

			case "enter":
				m.searching = false
				m.searchInput.Blur()
				return m, nil

			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.filterProjects()
				// Reset cursor to first project (or "Create new" if no projects)
				m.cursor = 1
				if len(m.filteredProjects) == 0 {
					m.cursor = 0
				}
				return m, cmd
			}
		}

		// Normal navigation
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "/":
			m.searching = true
			m.searchInput.Focus()
			return m, textinput.Blink

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			// Max cursor is len(filteredProjects) - "Create new project" is at 0, projects start at 1
			if m.cursor < len(m.filteredProjects) {
				m.cursor++
			}

		case "enter":
			// Check if "Create new project" is selected (at cursor position 0)
			if m.cursor == 0 {
				m.createNew = true
				return m, tea.Quit
			}
			// Otherwise select the project at cursor-1 (since project 0 is at cursor 1)
			if m.cursor > 0 && m.cursor-1 < len(m.filteredProjects) {
				m.selected = m.filteredProjects[m.cursor-1]
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m *ProjectListModel) filterProjects() {
	query := strings.ToLower(m.searchInput.Value())
	if query == "" {
		m.filteredProjects = m.projects
		return
	}

	filtered := make([]*storage.Project, 0)
	for _, project := range m.projects {
		if strings.Contains(strings.ToLower(project.Name), query) ||
			strings.Contains(strings.ToLower(project.BaseURL), query) {
			filtered = append(filtered, project)
		}
	}
	m.filteredProjects = filtered
}

func (m ProjectListModel) View() string {
	var s strings.Builder

	// ASCII art logo
	logo := strings.Split(Logo, "\n")
	for i, line := range logo {
		// Style the logo with gradient colors - empty blocks (░) get dark color
		var styledLine strings.Builder
		for _, char := range line {
			if char == '░' {
				styledLine.WriteString(lipgloss.NewStyle().Foreground(Theme.TextSubtle).Render(string(char)))
			} else {
				color := Theme.LogoGradient[i%len(Theme.LogoGradient)]
				styledLine.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(string(char)))
			}
		}
		s.WriteString(styledLine.String() + "\n")
	}
	s.WriteString("\n")

	// Title
	s.WriteString(titleStyle.Render("Select a Project"))
	s.WriteString("\n\n")

	// Search bar
	if m.searching {
		s.WriteString(searchStyle.Render("Search: "))
		s.WriteString(m.searchInput.View())
		s.WriteString("\n\n")
	} else {
		s.WriteString(helpStyle.Render("Press '/' to search"))
		s.WriteString("\n\n")
	}

	// "Create new project" at position 0 - always show +, use ▶ when selected
	cursor := "+ "
	if m.cursor == 0 {
		cursor = "▶ "
	}
	createNewLine := fmt.Sprintf("%sCreate new project", cursor)

	if m.cursor == 0 {
		s.WriteString(selectedItemStyle.Render(createNewLine))
	} else {
		s.WriteString(itemStyle.Render(createNewLine))
	}
	s.WriteString("\n\n")

	// Projects list (starting at position 1)
	if len(m.filteredProjects) == 0 && !m.searching {
		s.WriteString(helpStyle.Render("No projects found. Create one to get started!"))
		s.WriteString("\n\n")
	} else if len(m.filteredProjects) > 0 {
		for i, project := range m.filteredProjects {
			cursor := " "
			// Project at index i is displayed at cursor position i+1
			if m.cursor == i+1 {
				cursor = "▶"
			}

			// Format last accessed time
			var timeStr string
			if !project.LastAccessedAt.IsZero() {
				timeStr = formatRelativeTime(project.LastAccessedAt)
			} else {
				timeStr = formatRelativeTime(project.UpdatedAt)
			}

			line := fmt.Sprintf("%s %s\n  %s • Last used: %s",
				cursor,
				project.Name,
				project.BaseURL,
				timeStr,
			)

			if m.cursor == i+1 {
				s.WriteString(selectedItemStyle.Render(line))
			} else {
				s.WriteString(itemStyle.Render(line))
			}
			s.WriteString("\n\n")
		}
	}

	// Help text
	help := "↑/k up • ↓/j down • enter select • / search • q quit"
	if m.searching {
		help = "esc cancel search • enter apply"
	}
	s.WriteString(helpStyle.Render(help))

	return s.String()
}

func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(diff.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

// ShouldCreateNew returns true if the user selected "Create new project"
func (m ProjectListModel) ShouldCreateNew() bool {
	return m.createNew
}

// GetSelectedProject returns the selected project (call after tea.Program exits)
func (m ProjectListModel) GetSelectedProject() *storage.Project {
	return m.selected
}

// GetError returns any error that occurred
func (m ProjectListModel) GetError() error {
	return m.err
}
