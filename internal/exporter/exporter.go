package exporter

import (
	"fmt"
	"os"
	"path/filepath"
)

// TestData represents a single test result to export
type TestData struct {
	Method       string
	Endpoint     string
	Headers      map[string]string
	Body         interface{}
	StatusCode   int
	ResponseBody string
	DurationMS   int64
	RequiresAuth bool
	Error        string
}

// ExportRequest contains all data needed for export
type ExportRequest struct {
	BaseURL  string
	Tests    []TestData
	FilePath string
	AuthType string
	AuthData map[string]string
}

// Exporter defines the interface for test exporters
type Exporter interface {
	Export(req ExportRequest) error
	FileExtension() string
}

var exporters = map[string]Exporter{
	"postman": &PostmanExporter{},
	"pytest":  &PytestExporter{},
	"sh":      &CurlExporter{},
}

// Export exports tests to the specified format
func Export(format string, req ExportRequest) error {
	exporter, ok := exporters[format]
	if !ok {
		return fmt.Errorf("unsupported export format: %s", format)
	}

	if err := os.MkdirAll(filepath.Dir(req.FilePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return exporter.Export(req)
}

// ResolveExportPath converts relative paths to absolute paths under ~/Documents/octrafic/tests/
func ResolveExportPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	testsDir := filepath.Join(homeDir, "Documents", "octrafic", "tests")
	return filepath.Join(testsDir, path), nil
}

// SupportedFormats returns list of supported export formats
func SupportedFormats() []string {
	formats := make([]string, 0, len(exporters))
	for format := range exporters {
		formats = append(formats, format)
	}
	return formats
}
