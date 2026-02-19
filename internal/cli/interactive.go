package cli

import (
	"github.com/Octrafic/octrafic-cli/internal/core/analyzer"
	"github.com/Octrafic/octrafic-cli/internal/core/auth"
	"github.com/Octrafic/octrafic-cli/internal/infra/logger"
	"github.com/Octrafic/octrafic-cli/internal/infra/storage"
	"os"

	"github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func Start(baseURL string, specPath string, analysis *analyzer.Analysis, authProvider auth.AuthProvider, version string, yoloMode bool) {
	model := NewTestUIModel(baseURL, specPath, analysis, authProvider, version, yoloMode, false)

	p := tea.NewProgram(model, tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		logger.Error("Error running interactive mode", logger.Err(err))
		os.Exit(1)
	}
}

func StartWithProject(baseURL string, analysis *analyzer.Analysis, project *storage.Project, authProvider auth.AuthProvider, version string, yoloMode bool) {
	specPath := project.SpecPath

	model := NewTestUIModel(baseURL, specPath, analysis, authProvider, version, yoloMode, false)

	model.currentProject = project

	// Create new conversation for this project (only for named projects)
	if !project.IsTemporary {
		conversationID := uuid.New().String()
		model.conversationID = conversationID
		model.isLoadedConversation = false

		// Conversation will be created on first user message (to get title from first prompt)
	}

	p := tea.NewProgram(model, tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		logger.Error("Error running interactive mode", logger.Err(err))
		os.Exit(1)
	}
}

// StartWithConversation starts the TUI with a loaded conversation
func StartWithConversation(baseURL string, analysis *analyzer.Analysis, project *storage.Project, authProvider auth.AuthProvider, version string, conversationID string, yoloMode bool) {
	specPath := project.SpecPath

	model := NewTestUIModel(baseURL, specPath, analysis, authProvider, version, yoloMode, false)

	model.currentProject = project
	model.conversationID = conversationID
	model.isLoadedConversation = true

	// Load conversation history
	if err := model.loadConversationHistory(); err != nil {
		logger.Error("Failed to load conversation history", logger.Err(err))
		os.Exit(1)
	}

	p := tea.NewProgram(model, tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		logger.Error("Error running interactive mode", logger.Err(err))
		os.Exit(1)
	}
}

// StartHeadless executes a single prompt non-interactively and exits when complete.
// Used for CI/CD environments where no user interaction is available.
func StartHeadless(baseURL string, analysis *analyzer.Analysis, project *storage.Project, authProvider auth.AuthProvider, version string, prompt string, yoloMode bool) {
	specPath := project.SpecPath

	model := NewTestUIModel(baseURL, specPath, analysis, authProvider, version, yoloMode, true)
	model.currentProject = project
	model.textarea.SetValue(prompt)
	model.textarea.SetCursor(len(prompt))
	model.initialPrompt = prompt

	conversationID := uuid.New().String()
	model.conversationID = conversationID
	model.isLoadedConversation = false
	model.agentState = StateIdle

	// Use a closed pipe as stdin to prevent blocking on input reads
	r, w, err := os.Pipe()
	if err != nil {
		logger.Error("Failed to create pipe", logger.Err(err))
		os.Exit(1)
	}
	w.Close()

	p := tea.NewProgram(model, tea.WithInput(r))

	if _, err := p.Run(); err != nil {
		logger.Error("Error running headless mode", logger.Err(err))
		os.Exit(1)
	}
	r.Close()
}
