package parser

import (
	"encoding/json"
	"testing"
)

func TestIsHTTPMethod(t *testing.T) {
	valid := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, m := range valid {
		if !isHTTPMethod(m) {
			t.Errorf("expected %q to be a valid HTTP method", m)
		}
	}

	invalid := []string{"get", "CONNECT", "TRACE", "", "FOO"}
	for _, m := range invalid {
		if isHTTPMethod(m) {
			t.Errorf("expected %q to not be a valid HTTP method", m)
		}
	}
}

func TestIsHTTPMethodLowercase(t *testing.T) {
	valid := []string{"get", "post", "put", "delete", "patch", "options", "head", "trace"}
	for _, m := range valid {
		if !isHTTPMethodLowercase(m) {
			t.Errorf("expected %q to be valid", m)
		}
	}

	invalid := []string{"connect", "", "foo", "parameters", "servers"}
	for _, m := range invalid {
		if isHTTPMethodLowercase(m) {
			t.Errorf("expected %q to not be valid", m)
		}
	}
}

func TestExtractPathFromURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://api.example.com/users/123", "/users/123"},
		{"http://localhost:8080/api/v1/items", "/api/v1/items"},
		{"{{base_url}}/users", "/users"},
		{"/users", "/users"},
		{"users", "/users"},
		{"https://api.example.com/", "/"},
		{"https://api.example.com", "/"},
	}

	for _, tt := range tests {
		got := extractPathFromURL(tt.input)
		if got != tt.expected {
			t.Errorf("extractPathFromURL(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExtractTagsFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"/users", []string{"users"}},
		{"/users/{id}", []string{"users"}},
		{"/users/{id}/orders", []string{"users", "orders"}},
		{"/api/v1/{id}", []string{"api"}},
		{"/", []string{}},
	}

	for _, tt := range tests {
		got := extractTagsFromPath(tt.path)
		if len(got) != len(tt.expected) {
			t.Errorf("extractTagsFromPath(%q) = %v, want %v", tt.path, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("extractTagsFromPath(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.expected[i])
			}
		}
	}
}

func TestExtractRequestBody(t *testing.T) {
	input := map[string]any{
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{
					"properties": map[string]any{
						"name":  map[string]any{"type": "string"},
						"email": map[string]any{"type": "string"},
						"age":   map[string]any{"type": "integer"},
					},
				},
			},
		},
	}

	result := extractRequestBody(input)
	if result["name"] != "string" {
		t.Errorf("expected name=string, got %v", result["name"])
	}
	if result["email"] != "string" {
		t.Errorf("expected email=string, got %v", result["email"])
	}
	if result["age"] != "integer" {
		t.Errorf("expected age=integer, got %v", result["age"])
	}

	// Empty input
	empty := extractRequestBody(map[string]any{})
	if len(empty) != 0 {
		t.Errorf("expected empty result, got %v", empty)
	}
}

func TestParseMarkdown(t *testing.T) {
	content := `# API Documentation

## GET /users
List all users

## POST /users
Create a new user

### DELETE /users/{id}
Delete a specific user
`

	spec, err := parseMarkdown(content)
	if err != nil {
		t.Fatalf("parseMarkdown failed: %v", err)
	}

	if spec.Format != "markdown" {
		t.Errorf("expected format 'markdown', got %q", spec.Format)
	}

	if len(spec.Endpoints) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(spec.Endpoints))
	}

	expected := []struct {
		method string
		path   string
	}{
		{"GET", "/users"},
		{"POST", "/users"},
		{"DELETE", "/users/{id}"},
	}

	for i, e := range expected {
		if spec.Endpoints[i].Method != e.method {
			t.Errorf("endpoint[%d] method = %q, want %q", i, spec.Endpoints[i].Method, e.method)
		}
		if spec.Endpoints[i].Path != e.path {
			t.Errorf("endpoint[%d] path = %q, want %q", i, spec.Endpoints[i].Path, e.path)
		}
	}
}

func TestParseMarkdownEmpty(t *testing.T) {
	spec, err := parseMarkdown("")
	if err != nil {
		t.Fatalf("parseMarkdown failed: %v", err)
	}
	if len(spec.Endpoints) != 0 {
		t.Errorf("expected 0 endpoints, got %d", len(spec.Endpoints))
	}
}

func TestParseOpenAPI(t *testing.T) {
	content := `{
		"openapi": "3.0.0",
		"paths": {
			"/users": {
				"get": {
					"summary": "List users",
					"description": "Get all users"
				},
				"post": {
					"summary": "Create user"
				}
			},
			"/users/{id}": {
				"delete": {
					"description": "Delete a user"
				}
			}
		}
	}`

	spec, err := parseOpenAPI([]byte(content))
	if err != nil {
		t.Fatalf("parseOpenAPI failed: %v", err)
	}

	if spec.Format != "openapi" {
		t.Errorf("expected format 'openapi', got %q", spec.Format)
	}
	if spec.Version != "3.0.0" {
		t.Errorf("expected version '3.0.0', got %q", spec.Version)
	}
	if len(spec.Endpoints) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(spec.Endpoints))
	}
}

func TestParseOpenAPIYAML(t *testing.T) {
	content := `
openapi: "3.1.0"
paths:
  /items:
    get:
      summary: List items
`
	spec, err := parseOpenAPI([]byte(content))
	if err != nil {
		t.Fatalf("parseOpenAPI YAML failed: %v", err)
	}

	if spec.Version != "3.1.0" {
		t.Errorf("expected version '3.1.0', got %q", spec.Version)
	}
	if len(spec.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(spec.Endpoints))
	}
}

func TestParsePostman(t *testing.T) {
	collection := map[string]any{
		"info": map[string]any{
			"name":        "Test API",
			"_postman_id": "abc-123",
			"schema":      "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		"item": []any{
			map[string]any{
				"name": "Get Users",
				"request": map[string]any{
					"method": "GET",
					"url":    "https://api.example.com/users",
					"header": []any{},
				},
			},
			map[string]any{
				"name": "Create User",
				"request": map[string]any{
					"method": "POST",
					"url":    "https://api.example.com/users",
					"header": []any{
						map[string]any{"key": "Authorization", "value": "Bearer token123"},
					},
					"body": map[string]any{
						"mode": "raw",
						"raw":  `{"name": "John"}`,
					},
				},
			},
		},
	}

	content, _ := json.Marshal(collection)
	spec, err := parsePostman(content)
	if err != nil {
		t.Fatalf("parsePostman failed: %v", err)
	}

	if spec.Format != "postman" {
		t.Errorf("expected format 'postman', got %q", spec.Format)
	}
	if spec.Version != "2.1" {
		t.Errorf("expected version '2.1', got %q", spec.Version)
	}
	if len(spec.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(spec.Endpoints))
	}

	// Check auth detection
	if spec.Endpoints[1].RequiresAuth != true {
		t.Error("expected endpoint with Authorization header to require auth")
	}
	if spec.Endpoints[1].AuthType != "bearer" {
		t.Errorf("expected auth type 'bearer', got %q", spec.Endpoints[1].AuthType)
	}

	// Check request body
	if spec.Endpoints[1].RequestBody != `{"name": "John"}` {
		t.Errorf("unexpected request body: %q", spec.Endpoints[1].RequestBody)
	}
}

func TestParsePostmanNestedFolders(t *testing.T) {
	collection := map[string]any{
		"info": map[string]any{
			"name":   "Nested API",
			"schema": "https://schema.getpostman.com/json/collection/v2.0.0/collection.json",
		},
		"item": []any{
			map[string]any{
				"name": "Users Folder",
				"item": []any{
					map[string]any{
						"name": "Get User",
						"request": map[string]any{
							"method": "GET",
							"url":    "https://api.example.com/users/1",
							"header": []any{},
						},
					},
				},
			},
		},
	}

	content, _ := json.Marshal(collection)
	spec, err := parsePostman(content)
	if err != nil {
		t.Fatalf("parsePostman failed: %v", err)
	}

	if len(spec.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint from nested folder, got %d", len(spec.Endpoints))
	}
	if spec.Endpoints[0].Path != "/users/1" {
		t.Errorf("expected path '/users/1', got %q", spec.Endpoints[0].Path)
	}
}

func TestExtractPostmanURL(t *testing.T) {
	// String URL
	got := extractPostmanURL("https://api.example.com/users")
	if got != "/users" {
		t.Errorf("string URL: expected '/users', got %q", got)
	}

	// Object URL with raw
	got = extractPostmanURL(map[string]any{
		"raw": "https://api.example.com/items",
	})
	if got != "/items" {
		t.Errorf("object URL with raw: expected '/items', got %q", got)
	}

	// Object URL with path array
	got = extractPostmanURL(map[string]any{
		"path": []any{"api", "v1", "users"},
	})
	if got != "/api/v1/users" {
		t.Errorf("object URL with path array: expected '/api/v1/users', got %q", got)
	}

	// Nil/unknown type
	got = extractPostmanURL(42)
	if got != "/" {
		t.Errorf("unknown type: expected '/', got %q", got)
	}
}

func TestParseGraphQL(t *testing.T) {
	content := `
type Query {
  users: [User]
  user(id: ID!): User
}

type Mutation {
  createUser(name: String!, email: String!): User
  deleteUser(id: ID!): Boolean
}

type User {
  id: ID
  name: String
  email: String
}
`

	spec, err := parseGraphQL(content)
	if err != nil {
		t.Fatalf("parseGraphQL failed: %v", err)
	}

	if spec.Format != "graphql" {
		t.Errorf("expected format 'graphql', got %q", spec.Format)
	}

	if len(spec.Endpoints) != 4 {
		t.Fatalf("expected 4 endpoints, got %d", len(spec.Endpoints))
	}

	// Queries should be GET
	queryCount := 0
	mutationCount := 0
	for _, ep := range spec.Endpoints {
		if ep.Method == "GET" {
			queryCount++
		}
		if ep.Method == "POST" {
			mutationCount++
		}
	}
	if queryCount != 2 {
		t.Errorf("expected 2 GET (query) endpoints, got %d", queryCount)
	}
	if mutationCount != 2 {
		t.Errorf("expected 2 POST (mutation) endpoints, got %d", mutationCount)
	}
}

func TestParseGraphQLField(t *testing.T) {
	// Simple field
	ep := parseGraphQLField("users: [User]", "Query")
	if ep == nil {
		t.Fatal("expected non-nil endpoint")
	}
	if ep.Method != "GET" {
		t.Errorf("expected GET for Query, got %q", ep.Method)
	}
	if ep.Path != "/graphql/users" {
		t.Errorf("expected path '/graphql/users', got %q", ep.Path)
	}

	// Field with args
	ep = parseGraphQLField("createUser(name: String!, email: String): User", "Mutation")
	if ep == nil {
		t.Fatal("expected non-nil endpoint")
	}
	if ep.Method != "POST" {
		t.Errorf("expected POST for Mutation, got %q", ep.Method)
	}
	if len(ep.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(ep.Parameters))
	}
	if !ep.Parameters[0].Required {
		t.Error("expected name parameter to be required (has !)")
	}
	if ep.Parameters[1].Required {
		t.Error("expected email parameter to not be required")
	}

	// Field with comment
	ep = parseGraphQLField("users: [User] # Get all users", "Query")
	if ep == nil {
		t.Fatal("expected non-nil endpoint")
	}
	if ep.Description != "Get all users" {
		t.Errorf("expected description 'Get all users', got %q", ep.Description)
	}

	// Empty field
	ep = parseGraphQLField(": String", "Query")
	if ep != nil {
		t.Error("expected nil for field with no name")
	}
}

func TestIsHTTPURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"https://api.example.com/openapi.json", true},
		{"http://localhost:8080/swagger.json", true},
		{"HTTPS://api.example.com/spec", true},
		{"/local/path/to/file.json", false},
		{"./relative/path.yaml", false},
		{"file.json", false},
		{"", false},
		{"ftp://files.example.com/spec.json", false},
	}

	for _, tt := range tests {
		got := isHTTPURL(tt.input)
		if got != tt.expected {
			t.Errorf("isHTTPURL(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestDetectFormatFromContent(t *testing.T) {
	tests := []struct {
		content  string
		expected string
	}{
		{`{"openapi": "3.0.0"}`, ".json"},
		{`  {"data": "value"}`, ".json"},
		{`[{"id": 1}]`, ".json"},
		{"openapi: 3.0.0\npaths: {}", ".yaml"},
		{"  openapi: 3.0.0", ".yaml"},
		{"plain text content", ".yaml"},
	}

	for _, tt := range tests {
		got := detectFormatFromContent([]byte(tt.content))
		if got != tt.expected {
			t.Errorf("detectFormatFromContent(%q) = %q, want %q", tt.content, got, tt.expected)
		}
	}
}
