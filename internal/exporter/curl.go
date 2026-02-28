package exporter

import (
	"fmt"
	"os"
	"strings"
)

type CurlExporter struct{}

func (e *CurlExporter) FileExtension() string {
	return ".sh"
}

func (e *CurlExporter) Export(req ExportRequest) error {
	var script strings.Builder

	script.WriteString("#!/bin/bash\n\n")
	fmt.Fprintf(&script, "# Generated curl commands for %s\n", req.BaseURL)
	fmt.Fprintf(&script, "BASE_URL=\"%s\"\n", req.BaseURL)

	if req.AuthType != "" {
		script.WriteString("\n# Set credentials via environment variables before running\n")
		switch req.AuthType {
		case "bearer":
			script.WriteString("# export AUTH_TOKEN=your_token\n")
		case "apikey":
			script.WriteString("# export API_KEY_VALUE=your_key\n")
		case "basic":
			script.WriteString("# export AUTH_USER=your_username\n")
			script.WriteString("# export AUTH_PASS=your_password\n")
		}
	}
	script.WriteString("\n")

	for i, test := range req.Tests {
		if i > 0 {
			script.WriteString("\n")
		}

		fmt.Fprintf(&script, "# Test %d: %s %s\n", i+1, test.Method, test.Endpoint)
		script.WriteString(e.buildCurlCommand(test, req))
		script.WriteString("\n")
	}

	if err := os.WriteFile(req.FilePath, []byte(script.String()), 0755); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (e *CurlExporter) buildCurlCommand(test TestData, req ExportRequest) string {
	var parts []string

	parts = append(parts, "curl")
	parts = append(parts, fmt.Sprintf("-X %s", test.Method))

	for key, value := range test.Headers {
		parts = append(parts, fmt.Sprintf("-H '%s: %s'", key, value))
	}

	if test.Body != nil {
		if bodyStr, ok := test.Body.(string); ok && bodyStr != "" {
			parts = append(parts, "-H 'Content-Type: application/json'")
		}
	}

	if test.RequiresAuth && req.AuthType != "" {
		switch req.AuthType {
		case "bearer":
			parts = append(parts, "-H 'Authorization: Bearer ${AUTH_TOKEN}'")
		case "apikey":
			if keyName, ok := req.AuthData["key_name"]; ok {
				parts = append(parts, fmt.Sprintf("-H '%s: ${API_KEY_VALUE}'", keyName))
			}
		case "basic":
			parts = append(parts, "-u '${AUTH_USER}:${AUTH_PASS}'")
		}
	}

	if test.Body != nil {
		if bodyStr, ok := test.Body.(string); ok {
			escapedBody := strings.ReplaceAll(bodyStr, "'", "'\\''")
			parts = append(parts, fmt.Sprintf("-d '%s'", escapedBody))
		}
	}

	parts = append(parts, "\"${BASE_URL}"+test.Endpoint+"\"")

	return strings.Join(parts, " \\\n  ")
}
