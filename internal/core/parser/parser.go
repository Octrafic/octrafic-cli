package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Specification struct {
	Format     string     `json:"format"`
	Version    string     `json:"version,omitempty"`
	Endpoints  []Endpoint `json:"endpoints"`
	RawContent string     `json:"raw_content"`
}

type Endpoint struct {
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	Description  string            `json:"description"`
	Parameters   []Parameter       `json:"parameters,omitempty"`
	RequestBody  string            `json:"request_body,omitempty"`
	Responses    map[string]string `json:"responses,omitempty"`
	RequiresAuth bool              `json:"requires_auth"`
	AuthType     string            `json:"auth_type"` // "bearer", "basic", "apikey", "none"
}

type Parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

func ParseSpecification(path string) (*Specification, error) {
	var content []byte
	var err error
	var isURL bool

	if isURL = isHTTPURL(path); isURL {
		content, err = fetchFromURL(path)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch spec from URL: %w", err)
		}
	} else {
		content, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
	}

	ext := strings.ToLower(filepath.Ext(path))
	if isURL && ext == "" {
		ext = detectFormatFromContent(content)
	}

	if ext == ".json" {
		var data map[string]any
		if err := json.Unmarshal(content, &data); err == nil {
			if info, ok := data["info"].(map[string]any); ok {
				if _, hasPostmanId := info["_postman_id"]; hasPostmanId {
					return parsePostman(content)
				}
				if schema, ok := info["schema"].(string); ok && strings.Contains(schema, "postman") {
					return parsePostman(content)
				}
			}
			if _, ok := data["openapi"]; ok {
				return parseOpenAPI(content)
			}
			if _, ok := data["swagger"]; ok {
				return parseOpenAPI(content)
			}
		}
		return parseOpenAPI(content)
	}

	switch ext {
	case ".md", ".markdown":
		return parseMarkdown(string(content))
	case ".yaml", ".yml":
		return parseOpenAPI(content)
	case ".graphql", ".gql":
		return parseGraphQL(string(content))
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}
}

func parseMarkdown(content string) (*Specification, error) {
	spec := &Specification{
		Format:     "markdown",
		RawContent: content,
		Endpoints:  []Endpoint{},
	}

	lines := strings.Split(content, "\n")
	var currentEndpoint *Endpoint

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				method := strings.ToUpper(strings.Trim(parts[1], "`"))
				if isHTTPMethod(method) {
					if currentEndpoint != nil {
						spec.Endpoints = append(spec.Endpoints, *currentEndpoint)
					}
					currentEndpoint = &Endpoint{
						Method:    method,
						Path:      strings.Trim(parts[2], "`"),
						Responses: make(map[string]string),
					}
				}
			}
		} else if currentEndpoint != nil && line != "" && !strings.HasPrefix(line, "#") {
			if currentEndpoint.Description == "" {
				currentEndpoint.Description = line
			}
		}
	}

	if currentEndpoint != nil {
		spec.Endpoints = append(spec.Endpoints, *currentEndpoint)
	}

	return spec, nil
}

func parseOpenAPI(content []byte) (*Specification, error) {
	var openapi map[string]any

	if err := json.Unmarshal(content, &openapi); err != nil {
		if err := yaml.Unmarshal(content, &openapi); err != nil {
			return nil, fmt.Errorf("failed to parse OpenAPI (tried JSON and YAML): %w", err)
		}
	}

	spec := &Specification{
		Format:     "openapi",
		RawContent: string(content),
		Endpoints:  []Endpoint{},
	}

	if version, ok := openapi["openapi"].(string); ok {
		spec.Version = version
	}

	if paths, ok := openapi["paths"].(map[string]any); ok {
		for path, methods := range paths {
			if methodMap, ok := methods.(map[string]any); ok {
				for method, details := range methodMap {
					endpoint := Endpoint{
						Method:    strings.ToUpper(method),
						Path:      path,
						Responses: make(map[string]string),
					}

					if detailsMap, ok := details.(map[string]any); ok {
						if desc, ok := detailsMap["description"].(string); ok {
							endpoint.Description = desc
						}
						if summary, ok := detailsMap["summary"].(string); ok {
							if endpoint.Description == "" {
								endpoint.Description = summary
							}
						}
					}

					spec.Endpoints = append(spec.Endpoints, endpoint)
				}
			}
		}
	}

	return spec, nil
}

func isHTTPMethod(s string) bool {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	return slices.Contains(methods, s)
}

func parsePostman(content []byte) (*Specification, error) {
	var collection struct {
		Info struct {
			Name   string `json:"name"`
			Schema string `json:"schema"`
		} `json:"info"`
		Item []postmanItem `json:"item"`
	}

	if err := json.Unmarshal(content, &collection); err != nil {
		return nil, fmt.Errorf("failed to parse Postman collection: %w", err)
	}

	spec := &Specification{
		Format:     "postman",
		RawContent: string(content),
		Endpoints:  []Endpoint{},
	}

	if strings.Contains(collection.Info.Schema, "v2.1") {
		spec.Version = "2.1"
	} else if strings.Contains(collection.Info.Schema, "v2.0") {
		spec.Version = "2.0"
	}

	extractPostmanItems(collection.Item, &spec.Endpoints)

	return spec, nil
}

type postmanItem struct {
	Name    string          `json:"name"`
	Request *postmanRequest `json:"request,omitempty"`
	Item    []postmanItem   `json:"item,omitempty"` // Nested folders
}

type postmanRequest struct {
	Method      string          `json:"method"`
	Header      []postmanHeader `json:"header"`
	URL         any             `json:"url"` // Can be string or object
	Body        *postmanBody    `json:"body,omitempty"`
	Description string          `json:"description,omitempty"`
}

type postmanHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type postmanBody struct {
	Mode string `json:"mode"`
	Raw  string `json:"raw,omitempty"`
}

func extractPostmanItems(items []postmanItem, endpoints *[]Endpoint) {
	for _, item := range items {
		if len(item.Item) > 0 {
			extractPostmanItems(item.Item, endpoints)
			continue
		}

		if item.Request != nil {
			endpoint := Endpoint{
				Method:      strings.ToUpper(item.Request.Method),
				Path:        extractPostmanURL(item.Request.URL),
				Description: item.Name,
				Responses:   make(map[string]string),
			}

			if item.Request.Description != "" {
				endpoint.Description = item.Request.Description
			}

			for _, header := range item.Request.Header {
				headerKey := strings.ToLower(header.Key)
				if headerKey == "authorization" {
					endpoint.RequiresAuth = true
					if strings.HasPrefix(strings.ToLower(header.Value), "bearer") {
						endpoint.AuthType = "bearer"
					} else if strings.HasPrefix(strings.ToLower(header.Value), "basic") {
						endpoint.AuthType = "basic"
					}
				}
				if headerKey == "x-api-key" || headerKey == "api-key" {
					endpoint.RequiresAuth = true
					endpoint.AuthType = "apikey"
				}
			}

			if item.Request.Body != nil && item.Request.Body.Raw != "" {
				endpoint.RequestBody = item.Request.Body.Raw
			}

			*endpoints = append(*endpoints, endpoint)
		}
	}
}

func extractPostmanURL(url any) string {
	switch v := url.(type) {
	case string:
		return extractPathFromURL(v)
	case map[string]any:
		// URL object format
		if raw, ok := v["raw"].(string); ok {
			return extractPathFromURL(raw)
		}
		// Build from path array
		if pathArr, ok := v["path"].([]any); ok {
			var parts []string
			for _, p := range pathArr {
				if s, ok := p.(string); ok {
					parts = append(parts, s)
				}
			}
			return "/" + strings.Join(parts, "/")
		}
	}
	return "/"
}

func extractPathFromURL(rawURL string) string {
	if idx := strings.Index(rawURL, "}}"); idx != -1 {
		rawURL = rawURL[idx+2:]
	}
	if strings.HasPrefix(rawURL, "http") {
		parts := strings.SplitN(rawURL, "/", 4)
		if len(parts) >= 4 {
			return "/" + parts[3]
		}
		return "/"
	}
	if !strings.HasPrefix(rawURL, "/") {
		return "/" + rawURL
	}
	return rawURL
}

func parseGraphQL(content string) (*Specification, error) {
	spec := &Specification{
		Format:     "graphql",
		RawContent: content,
		Endpoints:  []Endpoint{},
	}

	lines := strings.Split(content, "\n")
	var currentType string // "Query" or "Mutation"
	var braceDepth int
	var currentField strings.Builder
	var inType bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "type Query") && (strings.Contains(trimmed, "{") || strings.Contains(trimmed, "implements")) {
			currentType = "Query"
			inType = true
			braceDepth = strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			continue
		}
		if strings.HasPrefix(trimmed, "type Mutation") && (strings.Contains(trimmed, "{") || strings.Contains(trimmed, "implements")) {
			currentType = "Mutation"
			inType = true
			braceDepth = strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			continue
		}

		if !inType {
			continue
		}

		for _, c := range trimmed {
			switch c {
			case '{':
				braceDepth++
			case '}':
				braceDepth--
				if braceDepth <= 0 {
					inType = false
					currentType = ""
				}
			}
		}

		if trimmed == "{" || trimmed == "}" {
			continue
		}

		if braceDepth > 0 && currentType != "" && !strings.HasPrefix(trimmed, "#") {
			currentField.WriteString(trimmed)

			fieldStr := currentField.String()
			if strings.Contains(fieldStr, ":") {
				endpoint := parseGraphQLField(fieldStr, currentType)
				if endpoint != nil {
					spec.Endpoints = append(spec.Endpoints, *endpoint)
				}
				currentField.Reset()
			}
		}
	}

	return spec, nil
}

func parseGraphQLField(field string, queryType string) *Endpoint {
	field = strings.TrimSpace(field)

	var name string
	var args string
	var description string

	if idx := strings.Index(field, "#"); idx != -1 {
		description = strings.TrimSpace(field[idx+1:])
		field = strings.TrimSpace(field[:idx])
	}

	if parenIdx := strings.Index(field, "("); parenIdx != -1 {
		name = strings.TrimSpace(field[:parenIdx])
		closeIdx := strings.Index(field, ")")
		if closeIdx != -1 && closeIdx > parenIdx {
			args = field[parenIdx+1 : closeIdx]
		}
	} else if colonIdx := strings.Index(field, ":"); colonIdx != -1 {
		name = strings.TrimSpace(field[:colonIdx])
	}

	if name == "" {
		return nil
	}

	method := "POST"
	path := "/graphql"

	if queryType == "Query" {
		method = "GET"
	}

	endpoint := &Endpoint{
		Method:      method,
		Path:        path + "/" + name,
		Description: fmt.Sprintf("GraphQL %s: %s", queryType, name),
		Responses:   make(map[string]string),
	}

	if description != "" {
		endpoint.Description = description
	}

	if args != "" {
		for _, arg := range strings.Split(args, ",") {
			arg = strings.TrimSpace(arg)
			if arg == "" {
				continue
			}
			parts := strings.SplitN(arg, ":", 2)
			if len(parts) == 2 {
				paramName := strings.TrimSpace(parts[0])
				paramType := strings.TrimSpace(parts[1])
				required := strings.HasSuffix(paramType, "!")
				paramType = strings.TrimSuffix(paramType, "!")

				endpoint.Parameters = append(endpoint.Parameters, Parameter{
					Name:     paramName,
					In:       "body",
					Type:     paramType,
					Required: required,
				})
			}
		}
	}

	return endpoint
}

func isHTTPURL(path string) bool {
	parsed, err := url.Parse(path)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func fetchFromURL(specURL string) ([]byte, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(specURL)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return content, nil
}

func detectFormatFromContent(content []byte) string {
	trimmed := strings.TrimSpace(string(content))
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return ".json"
	}
	return ".yaml"
}
