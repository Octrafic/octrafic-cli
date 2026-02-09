package cli

import (
	"github.com/Octrafic/octrafic-cli/internal/core/analyzer"
	"github.com/Octrafic/octrafic-cli/internal/core/auth"
	"github.com/Octrafic/octrafic-cli/internal/infra/logger"
	"github.com/Octrafic/octrafic-cli/internal/infra/storage"
	"os"

	"github.com/charmbracelet/bubbletea"
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

	p := tea.NewProgram(model, tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		logger.Error("Error running interactive mode", logger.Err(err))
		os.Exit(1)
	}
}
