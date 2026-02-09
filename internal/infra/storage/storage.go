package storage

import (
	"encoding/json"
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/core/analyzer"
	"os"
	"path/filepath"
)

const storageDir = ".octrafic"
const analysisFile = "analysis.json"

func Store(analysis *analyzer.Analysis) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	storagePath := filepath.Join(homeDir, storageDir)
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	filePath := filepath.Join(storagePath, analysisFile)

	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal analysis: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write analysis file: %w", err)
	}

	return nil
}

func Load() (*analyzer.Analysis, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	filePath := filepath.Join(homeDir, storageDir, analysisFile)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read analysis file: %w", err)
	}

	var analysis analyzer.Analysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		return nil, fmt.Errorf("failed to unmarshal analysis: %w", err)
	}

	return &analysis, nil
}
