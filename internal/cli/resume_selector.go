package cli

import (
	"fmt"
	"strings"

	"github.com/Octrafic/octrafic-cli/internal/infra/storage"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type resumeStage int

const (
	stageSelectProject resumeStage = iota
	stageSelectConversation
	stageDone
)

const (
	cardWidth       = 40 // Width for all cards
	createCardWidth = 40 // Width for "create new" cards
	conversationsPerPage = 5 // Conversations visible at once
	projectsPerPage = 3 // Projects visible at once
)

// truncateMiddle truncates text in the middle if too long
func truncateMiddle(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen < 5 {
		return text[:maxLen]
	}
	
	leftLen := (maxLen - 3) / 2
	rightLen := maxLen - 3 - leftLen
	
	return text[:leftLen] + "..." + text[len(text)-rightLen:]
}

// ResumeSelectorModel is a fullscreen form-like UI for selecting project and conversation
type ResumeSelectorModel struct {
	stage resumeStage
	
	// Project selection
	projects         []*storage.Project
	filteredProjects []*storage.Project
	projectCursor    int
	searchInput      textinput.Model
	searching        bool
	
	// Conversation selection
	selectedProject    *storage.Project
	conversations      []*storage.Conversation
	conversationCursor int
	conversationViewport viewport.Model
	viewportReady      bool
	
	// Results
	selectedConversation *storage.Conversation
	createNew            bool
	createNewProject     bool
	cancelled            bool
	
	version string
	width   int
	height  int
}

func NewResumeSelectorModel(projects []*storage.Project, version string) ResumeSelectorModel {
	ti := textinput.New()
	ti.Placeholder = "Search projects..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 40
	
	return ResumeSelectorModel{
		stage:            stageSelectProject,
		projects:         projects,
		filteredProjects: projects,
		projectCursor:    1, // Start at first real project, not "Create new"
		searchInput:      ti,
		searching:        false,
		version:          version,
		width:            80,
		height:           24,
	}
}

func (m ResumeSelectorModel) Init() tea.Cmd {
	return nil
}

func (m ResumeSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Initialize viewport for conversations when we have size
		if !m.viewportReady {
			m.conversationViewport = viewport.New(msg.Width, msg.Height-10)
			m.viewportReady = true
		}
		return m, nil
		
	case tea.KeyMsg:
		switch m.stage {
		case stageSelectProject:
			return m.updateProjectSelection(msg)
		case stageSelectConversation:
			return m.updateConversationSelection(msg)
		}
	}
	
	return m, nil
}

func (m ResumeSelectorModel) updateProjectSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search input
	if m.searching {
		switch msg.String() {
		case "esc":
			m.searching = false
			m.searchInput.Blur()
			m.searchInput.SetValue("")
			m.filteredProjects = m.projects
			m.projectCursor = 1 // Reset to first real project
			return m, nil
		case "enter":
			m.searching = false
			m.searchInput.Blur()
			// Select project if any filtered
			if m.projectCursor == 0 {
				// "Create new project" selected
				m.createNewProject = true
				m.stage = stageDone
				return m, tea.Quit
			}
			
			projectIndex := m.projectCursor - 1
			if projectIndex >= 0 && projectIndex < len(m.filteredProjects) {
				m.selectedProject = m.filteredProjects[projectIndex]
				convs, _ := storage.ListConversations(m.selectedProject.ID)
				m.conversations = convs
				m.conversationCursor = 0 // Start at "New conversation"
				m.stage = stageSelectConversation
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			
			// Filter projects
			query := strings.ToLower(m.searchInput.Value())
			if query == "" {
				m.filteredProjects = m.projects
			} else {
				m.filteredProjects = []*storage.Project{}
				for _, p := range m.projects {
					if strings.Contains(strings.ToLower(p.Name), query) ||
						strings.Contains(strings.ToLower(p.BaseURL), query) {
						m.filteredProjects = append(m.filteredProjects, p)
					}
				}
			}
			
			// Reset cursor to first project (position 1, not 0 which is "Create new")
			if len(m.filteredProjects) > 0 {
				m.projectCursor = 1
			} else {
				m.projectCursor = 0
			}
			
			return m, cmd
		}
	}
	
	// Normal navigation
	switch msg.String() {
	case "ctrl+c":
		m.cancelled = true
		m.stage = stageDone
		return m, tea.Quit
		
	case "esc":
		m.cancelled = true
		m.stage = stageDone
		return m, tea.Quit
		
	case "/":
		m.searching = true
		m.searchInput.Focus()
		return m, nil
		
	case "up", "k":
		if m.projectCursor > 0 {
			m.projectCursor--
		}
		
	case "down", "j":
		// +1 for "Create new project" option at top
		maxCursor := len(m.filteredProjects) // Last valid position is last project
		if m.projectCursor < maxCursor {
			m.projectCursor++
		}
		
	case "enter":
		// Check if "Create new project" is selected (position 0)
		if m.projectCursor == 0 {
			m.createNewProject = true
			m.stage = stageDone
			return m, tea.Quit
		}
		
		// Select project (adjust index by -1 because "Create new" is at 0)
		projectIndex := m.projectCursor - 1
		if projectIndex >= 0 && projectIndex < len(m.filteredProjects) {
			m.selectedProject = m.filteredProjects[projectIndex]
			// Load conversations for selected project
			convs, _ := storage.ListConversations(m.selectedProject.ID)
			m.conversations = convs
			m.conversationCursor = 0 // Start at "New conversation"
			m.stage = stageSelectConversation
		}
	}
	
	return m, nil
}

func (m ResumeSelectorModel) updateConversationSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		// Go back to project selection
		m.stage = stageSelectProject
		m.selectedProject = nil
		
	case "up", "k":
		if m.conversationCursor > 0 {
			m.conversationCursor--
		}
		
	case "down", "j":
		maxCursor := len(m.conversations) // +1 for "New conversation" option
		if m.conversationCursor < maxCursor {
			m.conversationCursor++
		}
		
	case "enter":
		if m.conversationCursor == 0 {
			// Create new conversation
			m.createNew = true
		} else if m.conversationCursor > 0 && m.conversationCursor <= len(m.conversations) {
			m.selectedConversation = m.conversations[m.conversationCursor-1]
		}
		m.stage = stageDone
		return m, tea.Quit
	}
	
	return m, nil
}

func (m ResumeSelectorModel) View() string {
	if m.stage == stageDone {
		return ""
	}
	
	var sections []string
	
	// Header with logo only for project selection
	if m.stage == stageSelectProject {
		sections = append(sections, m.renderHeader())
	}
	
	// Main content
	var content string
	switch m.stage {
	case stageSelectProject:
		content = m.viewProjectSelection()
	case stageSelectConversation:
		content = m.viewConversationSelection()
	}
	sections = append(sections, content)
	
	// Footer
	sections = append(sections, m.renderFooter())
	
	// Combine all sections and center vertically/horizontally
	return m.layoutFullscreen(sections)
}

func (m ResumeSelectorModel) renderHeader() string {
	var s strings.Builder
	
	// Logo only
	s.WriteString(RenderLogo())
	s.WriteString("\n")
	
	return s.String()
}

func (m ResumeSelectorModel) renderFooter() string {
	var helpText string
	
	switch m.stage {
	case stageSelectProject:
		if m.searching {
			helpText = "esc: cancel search • enter: select"
		} else {
			helpText = "↑/↓: navigate • enter: select • /: search • esc: exit"
		}
	case stageSelectConversation:
		helpText = "↑/↓: navigate • enter: select • esc: back"
	}
	
	footerStyle := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Align(lipgloss.Center)
	
	return footerStyle.Render(helpText)
}

func (m ResumeSelectorModel) viewProjectSelection() string {
	var s strings.Builder
	
	// Search bar
	if m.searching {
		searchLabel := lipgloss.NewStyle().Foreground(Theme.PrimaryDark).Render("Search: ")
		s.WriteString(searchLabel)
		s.WriteString(m.searchInput.View())
		s.WriteString("\n\n")
	} else {
		hint := lipgloss.NewStyle().Foreground(Theme.TextSubtle).Italic(true).Render("Press / to search")
		s.WriteString(hint)
		s.WriteString("\n\n")
	}
	
	// "Create new project" card at the top
	s.WriteString(m.renderNewProjectCard(m.projectCursor == 0))
	s.WriteString("\n")
	
	// Projects with scrolling
	if len(m.filteredProjects) == 0 {
		noResults := lipgloss.NewStyle().
			Foreground(Theme.TextSubtle).
			Italic(true).
			Render("No projects found")
		s.WriteString(noResults)
		s.WriteString("\n")
	} else {
		// Calculate visible range
		visibleStart := 0
		visibleEnd := len(m.filteredProjects)
		
		// Adjust for cursor position (cursor-1 because 0 is "Create new")
		actualProjectCursor := m.projectCursor - 1
		if actualProjectCursor >= projectsPerPage {
			visibleStart = actualProjectCursor - projectsPerPage + 1
		}
		if visibleEnd > visibleStart + projectsPerPage {
			visibleEnd = visibleStart + projectsPerPage
		}
		
		// Show indicator if there are more above
		if visibleStart > 0 {
			indicator := lipgloss.NewStyle().
				Foreground(Theme.TextSubtle).
				Render(fmt.Sprintf("  ↑ %d more above...", visibleStart))
			s.WriteString(indicator)
			s.WriteString("\n")
		}
		
		// Show visible projects
		for i := visibleStart; i < visibleEnd; i++ {
			project := m.filteredProjects[i]
			// +1 because "Create new" is at position 0
			isSelected := i+1 == m.projectCursor
			s.WriteString(m.renderProjectCard(project, isSelected))
			s.WriteString("\n")
		}
		
		// Show indicator if there are more below
		if visibleEnd < len(m.filteredProjects) {
			indicator := lipgloss.NewStyle().
				Foreground(Theme.TextSubtle).
				Render(fmt.Sprintf("  ↓ %d more below...", len(m.filteredProjects)-visibleEnd))
			s.WriteString(indicator)
			s.WriteString("\n")
		}
	}
	
	return s.String()
}

func (m ResumeSelectorModel) renderNewProjectCard(selected bool) string {
	textColor := Theme.Text
	
	if selected {
		textColor = Theme.PrimaryStrong
	}
	
	title := lipgloss.NewStyle().
		Foreground(textColor).
		Bold(true).
		Render("+ Create new project")
	
	subtitle := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("Add a new API project")
	
	content := fmt.Sprintf("%s\n%s", title, subtitle)
	
	style := lipgloss.NewStyle().
		Width(createCardWidth).
		Align(lipgloss.Left)
	
	if selected {
		style = style.Background(Theme.BgSelected)
	}
	
	return style.Render(content)
}

func (m ResumeSelectorModel) renderProjectCard(project *storage.Project, selected bool) string {
	nameColor := Theme.Text
	
	if selected {
		nameColor = Theme.PrimaryStrong
	}
	
	// Project name - truncate if needed
	nameText := truncateMiddle(project.Name, cardWidth-20) // Reserve space for time
	
	// Time ago
	timeAgo := formatTimeAgo(project.LastAccessedAt)

	// First line: name on left, time on right
	nameStyle := lipgloss.NewStyle().Foreground(nameColor).Bold(true)
	timeStyle := lipgloss.NewStyle().Foreground(Theme.TextSubtle)
	spacerStyle := lipgloss.NewStyle()

	if selected {
		nameStyle = nameStyle.Background(Theme.BgSelected)
		timeStyle = timeStyle.Background(Theme.BgSelected)
		spacerStyle = spacerStyle.Background(Theme.BgSelected)
	}

	firstLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		nameStyle.Render(nameText),
		spacerStyle.Width(cardWidth-len(nameText)-len(timeAgo)-2).Render(""),
		timeStyle.Render(timeAgo),
	)
	
	// URL
	url := project.BaseURL
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = truncateMiddle(url, cardWidth-4)
	urlStyle := lipgloss.NewStyle().Foreground(Theme.Cyan)
	if selected {
		urlStyle = urlStyle.Background(Theme.BgSelected)
	}
	urlLine := urlStyle.Render(url)
	
	// Combine
	content := fmt.Sprintf("%s\n%s", firstLine, urlLine)
	
	style := lipgloss.NewStyle().
		Width(cardWidth).
		Align(lipgloss.Left)
	
	if selected {
		style = style.Background(Theme.BgSelected)
	}
	
	return style.Render(content)
}

func (m ResumeSelectorModel) viewConversationSelection() string {
	var s strings.Builder
	
	// Title with project name
	title := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Render(fmt.Sprintf("Select Conversation - %s", m.selectedProject.Name))
	s.WriteString(title)
	s.WriteString("\n\n")
	
	// "New conversation" card
	s.WriteString(m.renderNewConversationCard(m.conversationCursor == 0))
	s.WriteString("\n")
	
	// Conversations list with pagination
	if len(m.conversations) == 0 {
		hint := lipgloss.NewStyle().
			Foreground(Theme.TextSubtle).
			Italic(true).
			Render("No previous conversations")
		s.WriteString(hint)
		s.WriteString("\n")
	} else {
		// Calculate visible range (scrolling window)
		visibleStart := 0
		visibleEnd := len(m.conversations)
		
		// If cursor is beyond first page, scroll the window
		if m.conversationCursor > conversationsPerPage {
			visibleStart = m.conversationCursor - conversationsPerPage
		}
		if visibleEnd > visibleStart + conversationsPerPage {
			visibleEnd = visibleStart + conversationsPerPage
		}
		
		// Show indicator if there are more above
		if visibleStart > 0 {
			indicator := lipgloss.NewStyle().
				Foreground(Theme.TextSubtle).
				Render(fmt.Sprintf("  ↑ %d more above...", visibleStart))
			s.WriteString(indicator)
			s.WriteString("\n")
		}
		
		// Show visible conversations
		for i := visibleStart; i < visibleEnd; i++ {
			conv := m.conversations[i]
			isSelected := i+1 == m.conversationCursor
			s.WriteString(m.renderConversationCard(conv, isSelected))
			s.WriteString("\n")
		}
		
		// Show indicator if there are more below
		if visibleEnd < len(m.conversations) {
			indicator := lipgloss.NewStyle().
				Foreground(Theme.TextSubtle).
				Render(fmt.Sprintf("  ↓ %d more below...", len(m.conversations)-visibleEnd))
			s.WriteString(indicator)
			s.WriteString("\n")
		}
	}
	
	return s.String()
}

func (m ResumeSelectorModel) renderNewConversationCard(selected bool) string {
	textColor := Theme.Text
	
	if selected {
		textColor = Theme.PrimaryStrong
	}
	
	title := lipgloss.NewStyle().
		Foreground(textColor).
		Bold(true).
		Render("+ Start new conversation")
	
	subtitle := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("Begin a fresh testing session")
	
	content := fmt.Sprintf("%s\n%s", title, subtitle)
	
	style := lipgloss.NewStyle().
		Width(createCardWidth).
		Align(lipgloss.Left)
	
	if selected {
		style = style.Background(Theme.BgSelected)
	}
	
	return style.Render(content)
}

func (m ResumeSelectorModel) renderConversationCard(conv *storage.Conversation, selected bool) string {
	titleColor := Theme.Text
	
	if selected {
		titleColor = Theme.PrimaryStrong
	}
	
	// Conversation title - truncate if needed
	titleText := truncateMiddle(conv.Title, cardWidth-4)
	titleStyle := lipgloss.NewStyle().Foreground(titleColor).Bold(true)
	if selected {
		titleStyle = titleStyle.Background(Theme.BgSelected)
	}
	titleLine := titleStyle.Render(titleText)

	// Time ago
	timeAgo := formatTimeAgo(conv.UpdatedAt)
	timeLineStyle := lipgloss.NewStyle().Foreground(Theme.TextSubtle)
	if selected {
		timeLineStyle = timeLineStyle.Background(Theme.BgSelected)
	}
	timeLine := timeLineStyle.Render("Last updated: " + timeAgo)
	
	content := fmt.Sprintf("%s\n%s", titleLine, timeLine)
	
	style := lipgloss.NewStyle().
		Width(cardWidth).
		Align(lipgloss.Left)
	
	if selected {
		style = style.Background(Theme.BgSelected)
	}
	
	return style.Render(content)
}

func (m ResumeSelectorModel) layoutFullscreen(sections []string) string {
	var result strings.Builder
	
	// Calculate total content height
	totalLines := 0
	for _, section := range sections {
		totalLines += strings.Count(section, "\n")
	}
	
	// Top padding to center vertically
	topPadding := (m.height - totalLines) / 2
	if topPadding < 0 {
		topPadding = 0
	}
	
	// Add top padding
	for i := 0; i < topPadding; i++ {
		result.WriteString("\n")
	}
	
	// Render each section centered horizontally
	for _, section := range sections {
		lines := strings.Split(section, "\n")
		for _, line := range lines {
			if line == "" {
				result.WriteString("\n")
				continue
			}
			
			// Center horizontally
			lineWidth := lipgloss.Width(line)
			leftPadding := (m.width - lineWidth) / 2
			if leftPadding < 0 {
				leftPadding = 0
			}
			
			result.WriteString(strings.Repeat(" ", leftPadding))
			result.WriteString(line)
			result.WriteString("\n")
		}
	}
	
	return result.String()
}

// IsCancelled returns true if user cancelled
func (m ResumeSelectorModel) IsCancelled() bool {
	return m.cancelled
}

// GetSelectedProject returns the selected project
func (m ResumeSelectorModel) GetSelectedProject() *storage.Project {
	return m.selectedProject
}

// GetSelectedConversation returns the selected conversation
func (m ResumeSelectorModel) GetSelectedConversation() *storage.Conversation {
	return m.selectedConversation
}

// ShouldCreateNew returns true if user wants to create new conversation
func (m ResumeSelectorModel) ShouldCreateNew() bool {
	return m.createNew
}

// ShouldCreateNewProject returns true if user wants to create new project
func (m ResumeSelectorModel) ShouldCreateNewProject() bool {
	return m.createNewProject
}
