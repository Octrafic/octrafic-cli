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
	fmt.Fprintf(&script, "BASE_URL=\"%s\"\n\n", req.BaseURL)

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

	parts = append(parts, "-H 'Content-Type: application/json'")

	for key, value := range test.Headers {
		parts = append(parts, fmt.Sprintf("-H '%s: %s'", key, value))
	}

	if test.RequiresAuth && req.AuthType != "" {
		switch req.AuthType {
		case "bearer":
			if token, ok := req.AuthData["token"]; ok {
				parts = append(parts, fmt.Sprintf("-H 'Authorization: Bearer %s'", token))
			}
		case "apikey":
			if keyName, ok := req.AuthData["key_name"]; ok {
				if keyValue, ok := req.AuthData["key_value"]; ok {
					parts = append(parts, fmt.Sprintf("-H '%s: %s'", keyName, keyValue))
				}
			}
		case "basic":
			if username, ok := req.AuthData["username"]; ok {
				if password, ok := req.AuthData["password"]; ok {
					parts = append(parts, fmt.Sprintf("-u '%s:%s'", username, password))
				}
			}
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
