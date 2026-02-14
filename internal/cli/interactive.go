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

func Start(baseURL string, specPath string, analysis *analyzer.Analysis, authProvider auth.AuthProvider, version string) {
	model := NewTestUIModel(baseURL, specPath, analysis, authProvider, version)

	p := tea.NewProgram(model, tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		logger.Error("Error running interactive mode", logger.Err(err))
		os.Exit(1)
	}
}

func StartWithProject(baseURL string, analysis *analyzer.Analysis, project *storage.Project, authProvider auth.AuthProvider, version string) {
	specPath := project.SpecPath

	model := NewTestUIModel(baseURL, specPath, analysis, authProvider, version)

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
func StartWithConversation(baseURL string, analysis *analyzer.Analysis, project *storage.Project, authProvider auth.AuthProvider, version string, conversationID string) {
	specPath := project.SpecPath

	model := NewTestUIModel(baseURL, specPath, analysis, authProvider, version)

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
