package scanner

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// OasDocument represents a top-level OpenAPI 3.1 spec
type OasDocument struct {
	OpenAPI string                 `yaml:"openapi"`
	Info    OasInfo                `yaml:"info"`
	Paths   map[string]OasPathItem `yaml:"paths"`
}

type OasInfo struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
}

type OasPathItem map[string]OasOperation // "get", "post", etc. -> OasOperation

type OasOperation struct {
	Summary     string                 `yaml:"summary,omitempty"`
	RequestBody *OasRequestBody        `yaml:"requestBody,omitempty"`
	Responses   map[string]OasResponse `yaml:"responses"`
}

type OasRequestBody struct {
	Content map[string]OasMediaType `yaml:"content"`
}

type OasResponse struct {
	Description string                  `yaml:"description"`
	Content     map[string]OasMediaType `yaml:"content,omitempty"`
}

type OasMediaType struct {
	Schema map[string]any `yaml:"schema"`
}

// generateSpec merges all scraped snippets into a single OpenAPI 3.1.0 document
func (s *Scanner) generateSpec(project *ProjectFramework, allEndpoints map[string][]EndpointDef, progressCallback func(string)) error {
	if progressCallback != nil {
		progressCallback("➔ Generating OpenAPI specification...")
	}

	doc := OasDocument{
		OpenAPI: "3.1.0",
		Info: OasInfo{
			Title:       "Octrafic Auto-Generated API Spec",
			Description: fmt.Sprintf("Automatically extracted via OOPS Architecture (%s / %s)", project.Language, project.Framework),
			Version:     "1.0.0",
		},
		Paths: make(map[string]OasPathItem),
	}

	for _, fileEndpoints := range allEndpoints {
		for _, ep := range fileEndpoints {
			if ep.Path == "" || ep.Method == "" {
				continue
			}

			methodLow := strings.ToLower(ep.Method)
			if _, exists := doc.Paths[ep.Path]; !exists {
				doc.Paths[ep.Path] = make(OasPathItem)
			}

			op := OasOperation{
				Summary: ep.Summary,
				Responses: map[string]OasResponse{
					"200": {
						Description: "Successful operation",
					},
				},
			}

			if len(ep.RequestBody) > 0 {
				op.RequestBody = &OasRequestBody{
					Content: map[string]OasMediaType{
						"application/json": {
							Schema: s.convertToJSONSchema(ep.RequestBody),
						},
					},
				}
			}

			if len(ep.ResponseBody) > 0 {
				op.Responses["200"] = OasResponse{
					Description: "Successful operation",
					Content: map[string]OasMediaType{
						"application/json": {
							Schema: s.convertToJSONSchema(ep.ResponseBody),
						},
					},
				}
			}

			doc.Paths[ep.Path][methodLow] = op
		}
	}

	yamlData, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("failed to marshal OpenAPI spec to YAML: %w", err)
	}

	err = os.WriteFile(s.outFile, yamlData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write spec to file %s: %w", s.outFile, err)
	}

	if progressCallback != nil {
		progressCallback(fmt.Sprintf("\n✅ Successfully generated specification and saved to %s", s.outFile))
	}
	return nil
}

// convertToJSONSchema takes a raw map (representing a struct/object definition)
// and naively wraps it in a simplistic OpenAPI JSON schema wrapper.
func (s *Scanner) convertToJSONSchema(obj map[string]any) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": obj,
	}
}
