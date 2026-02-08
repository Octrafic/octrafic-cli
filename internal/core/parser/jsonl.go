package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// JSONLEndpoint represents a single API endpoint in JSONL format
type JSONLEndpoint struct {
	Method       string                 `json:"method"`
	Path         string                 `json:"path"`
	Summary      string                 `json:"summary"`
	Description  string                 `json:"description,omitempty"`
	Parameters   []string               `json:"parameters,omitempty"`
	RequestBody  map[string]any `json:"request_body,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	RequiresAuth bool                   `json:"requires_auth"`
	AuthType     string                 `json:"auth_type"` // "bearer", "basic", "apikey", "none"
}

// IsJSONLFormat checks if the file is already in JSONL format
func IsJSONLFormat(specPath string) (bool, error) {
	// Check file extension first
	if strings.HasSuffix(strings.ToLower(specPath), ".jsonl") {
		return true, nil
	}

	// Read first few lines to check format
	file, err := os.Open(specPath)
	if err != nil {
		return false, err
	}
	defer func() { _ = file.Close() }()

	decoder := json.NewDecoder(file)

	// Try to decode first line as JSONLEndpoint
	var firstLine JSONLEndpoint
	if err := decoder.Decode(&firstLine); err != nil {
		return false, nil // Not JSONL format
	}

	// Check if it has required JSONL fields
	if firstLine.Method != "" && firstLine.Path != "" {
		return true, nil
	}

	return false, nil
}

// ConvertOpenAPIToJSONL converts OpenAPI 3.0 or Swagger 2.0 spec to JSONL format
// If the file is already in JSONL format, it copies it to the output path
func ConvertOpenAPIToJSONL(specPath, outputPath string) error {
	// Check if already JSONL format
	isJSONL, err := IsJSONLFormat(specPath)
	if err != nil {
		return fmt.Errorf("failed to check file format: %w", err)
	}

	if isJSONL {
		// File is already JSONL, just copy it
		input, err := os.ReadFile(specPath)
		if err != nil {
			return fmt.Errorf("failed to read JSONL file: %w", err)
		}

		if err := os.WriteFile(outputPath, input, 0644); err != nil {
			return fmt.Errorf("failed to copy JSONL file: %w", err)
		}

		return nil
	}
	// Read spec file
	specData, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("failed to read spec: %w", err)
	}

	// Parse JSON
	var spec map[string]any
	if err := json.Unmarshal(specData, &spec); err != nil {
		return fmt.Errorf("failed to parse spec JSON: %w", err)
	}

	// Get paths
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		return fmt.Errorf("spec missing 'paths' field")
	}

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	encoder := json.NewEncoder(outFile)

	// Iterate through paths
	for path, pathData := range paths {
		pathObj, ok := pathData.(map[string]any)
		if !ok {
			continue
		}

		// Extract tags from path (users, products, orders, etc.)
		tags := extractTagsFromPath(path)

		// Iterate through methods
		for method, methodData := range pathObj {
			// Skip non-method fields (like parameters, servers, etc.)
			if !isHTTPMethodLowercase(method) {
				continue
			}

			methodObj, ok := methodData.(map[string]any)
			if !ok {
				continue
			}

			endpoint := JSONLEndpoint{
				Method: strings.ToUpper(method),
				Path:   path,
				Tags:   tags,
			}

			// Extract summary
			if summary, ok := methodObj["summary"].(string); ok {
				endpoint.Summary = summary
			}

			// Extract description
			if description, ok := methodObj["description"].(string); ok {
				endpoint.Description = description
			}

			// Extract parameters
			if params, ok := methodObj["parameters"].([]any); ok {
				for _, p := range params {
					if paramObj, ok := p.(map[string]any); ok {
						if name, ok := paramObj["name"].(string); ok {
							endpoint.Parameters = append(endpoint.Parameters, name)
						}
					}
				}
			}

			// Extract request body (OpenAPI 3.0)
			if requestBody, ok := methodObj["requestBody"].(map[string]any); ok {
				endpoint.RequestBody = extractRequestBody(requestBody)
			}

			// Add method-specific tags
			switch method {
			case "get":
				endpoint.Tags = append(endpoint.Tags, "read")
			case "post":
				endpoint.Tags = append(endpoint.Tags, "create")
			case "put", "patch":
				endpoint.Tags = append(endpoint.Tags, "update")
			case "delete":
				endpoint.Tags = append(endpoint.Tags, "delete")
			}

			// Write as single line
			if err := encoder.Encode(endpoint); err != nil {
				return fmt.Errorf("failed to write endpoint: %w", err)
			}
		}
	}

	return nil
}

// isHTTPMethodLowercase checks if string is a valid HTTP method (lowercase)
func isHTTPMethodLowercase(s string) bool {
	methods := []string{"get", "post", "put", "delete", "patch", "options", "head", "trace"}
	return slices.Contains(methods, strings.ToLower(s))
}

// extractTagsFromPath extracts category tags from endpoint path
func extractTagsFromPath(path string) []string {
	tags := []string{}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	
	if len(parts) > 0 && parts[0] != "" {
		// First segment is usually the category
		tags = append(tags, parts[0])
		
		// If it's a nested resource, add that too
		if len(parts) > 2 && !strings.HasPrefix(parts[2], "{") {
			tags = append(tags, parts[2])
		}
	}
	
	return tags
}

// extractRequestBody extracts request body schema fields
func extractRequestBody(requestBody map[string]any) map[string]any {
	result := make(map[string]any)
	
	// Navigate: requestBody -> content -> application/json -> schema -> properties
	if content, ok := requestBody["content"].(map[string]any); ok {
		if appJSON, ok := content["application/json"].(map[string]any); ok {
			if schema, ok := appJSON["schema"].(map[string]any); ok {
				if properties, ok := schema["properties"].(map[string]any); ok {
					// Extract field names and types
					for fieldName, fieldData := range properties {
						if fieldObj, ok := fieldData.(map[string]any); ok {
							if fieldType, ok := fieldObj["type"].(string); ok {
								result[fieldName] = fieldType
							}
						}
					}
				}
			}
		}
	}
	
	return result
}

// LoadJSONLEndpoints loads endpoints from JSONL file
func LoadJSONLEndpoints(jsonlPath string) ([]Endpoint, error) {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSONL file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var endpoints []Endpoint
	decoder := json.NewDecoder(file)

	for {
		var jsonlEndpoint JSONLEndpoint
		if err := decoder.Decode(&jsonlEndpoint); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to decode JSONL line: %w", err)
		}

		// Convert to Endpoint format
		endpoints = append(endpoints, Endpoint{
			Method:      jsonlEndpoint.Method,
			Path:        jsonlEndpoint.Path,
			Description: jsonlEndpoint.Summary,
		})
	}

	return endpoints, nil
}
