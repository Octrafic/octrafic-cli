package scanner

import (
	"fmt"
	"path/filepath"

	"github.com/Octrafic/octrafic-cli/internal/agents"
	"github.com/Octrafic/octrafic-cli/internal/infra/logger"
	"github.com/Octrafic/octrafic-cli/internal/llm/common"
)

// Scanner orchestrates the codebase scan
type Scanner struct {
	baseAgent *agent.BaseAgent
	dir       string
	outFile   string
	matcher   *IgnoreMatcher
}

// NewScanner creates a new Scanner instance
func NewScanner(provider common.Provider, dir, outFile string) (*Scanner, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for dir: %w", err)
	}

	matcher, err := NewIgnoreMatcher(absDir)
	if err != nil {
		logger.Warn("Failed to load .gitignore", logger.Err(err))
	}

	return &Scanner{
		baseAgent: agent.NewBaseAgent(provider),
		dir:       absDir,
		outFile:   outFile,
		matcher:   matcher,
	}, nil
}

// RunScan starts the OOPS Pipeline scanning process
func (s *Scanner) RunScan(progressCallback func(string)) error {
	// Stage 1: Framework Detection
	framework, err := s.detectFramework(progressCallback)
	if err != nil {
		return fmt.Errorf("stage 1 (detectFramework) failed: %w", err)
	}

	// Stage 2: Routing Discovery
	routingFiles, err := s.findRoutingFiles(framework, progressCallback)
	if err != nil {
		return fmt.Errorf("stage 2 (findRoutingFiles) failed: %w", err)
	}

	if len(routingFiles) == 0 {
		return fmt.Errorf("no routing files found for framework %s", framework.Framework)
	}

	// Stage 3 & 4: Parallel Endpoint Extraction
	endpoints := s.extractEndpoints(routingFiles, progressCallback)
	if len(endpoints) == 0 {
		return fmt.Errorf("no endpoints could be extracted from the identified routing files")
	}

	// Stage 5: Merge and YAML Generation
	err = s.generateSpec(framework, endpoints, progressCallback)
	if err != nil {
		return fmt.Errorf("stage 5 (generateSpec) failed: %w", err)
	}

	return nil
}
