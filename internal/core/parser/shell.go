package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ParseShellScript reads a shell script and extracts curl commands as Endpoints
func ParseShellScript(path string) (*Specification, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read shell script: %w", err)
	}

	return parseShellContent(content)
}

func parseShellContent(content []byte) (*Specification, error) {
	spec := &Specification{
		Format:     "sh",
		RawContent: string(content),
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	var currentCommand string
	inCommand := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "curl ") {
			inCommand = true
			currentCommand = line
		} else if inCommand {
			currentCommand += " " + line
		}

		if inCommand && !strings.HasSuffix(line, "\\") {
			inCommand = false
			currentCommand = strings.ReplaceAll(currentCommand, "\\", "")
			
			endpoint := parseCurlCommand(currentCommand)
			if endpoint != nil {
				spec.Endpoints = append(spec.Endpoints, *endpoint)
			}
			currentCommand = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading shell script: %w", err)
	}

	return spec, nil
}

func parseCurlCommand(cmd string) *Endpoint {
	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return nil
	}

	endpoint := &Endpoint{
		Method: "GET", // Default
	}

	var rawURL string
	var bodyBuilder strings.Builder

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		
		switch part {
		case "-X", "--request":
			if i+1 < len(parts) {
				endpoint.Method = strings.ToUpper(strings.Trim(parts[i+1], "'\""))
				i++
			}
		case "-d", "--data", "--data-raw", "--data-binary":
			if i+1 < len(parts) {
				bodyContent := strings.Trim(parts[i+1], "'\"")
				if bodyBuilder.Len() > 0 {
					bodyBuilder.WriteString("&")
				}
				bodyBuilder.WriteString(bodyContent)
				if endpoint.Method == "GET" {
					endpoint.Method = "POST"
				}
				i++
			}
		default:
			if !strings.HasPrefix(part, "-") && rawURL == "" {
				rawURL = strings.Trim(part, "'\"")
			}
		}
	}

	if rawURL != "" {
		if parsedURL, err := url.Parse(rawURL); err == nil {
			endpoint.Path = parsedURL.Path
			if parsedURL.RawQuery != "" {
				endpoint.Path += "?" + parsedURL.RawQuery
			}
			if endpoint.Path == "" {
				endpoint.Path = "/"
			}
		} else {
			endpoint.Path = rawURL
		}
		endpoint.Description = fmt.Sprintf("Curl command to %s", endpoint.Path)
	}

	endpoint.RequestBody = bodyBuilder.String()

	if rawURL == "" || endpoint.Path == "" {
		return nil
	}

	return endpoint
}
