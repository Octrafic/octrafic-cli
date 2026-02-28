package cli

import (
	"testing"

	"github.com/Octrafic/octrafic-cli/internal/core/analyzer"
	"github.com/Octrafic/octrafic-cli/internal/core/parser"
)

func TestMatchPath_Exact(t *testing.T) {
	if !matchPath("/users", "/users") {
		t.Error("expected exact match")
	}
}

func TestMatchPath_WithParam(t *testing.T) {
	if !matchPath("/users/{id}", "/users/42") {
		t.Error("expected param segment to match any value")
	}
}

func TestMatchPath_MultipleParams(t *testing.T) {
	if !matchPath("/users/{id}/orders/{orderId}", "/users/1/orders/99") {
		t.Error("expected multiple params to match")
	}
}

func TestMatchPath_DifferentSegmentCount(t *testing.T) {
	if matchPath("/users/{id}", "/users") {
		t.Error("expected no match for different segment count")
	}
}

func TestMatchPath_LiteralMismatch(t *testing.T) {
	if matchPath("/users/{id}", "/products/42") {
		t.Error("expected no match when literal segment differs")
	}
}

func TestMatchPath_CaseInsensitive(t *testing.T) {
	if !matchPath("/Users/{id}", "/users/1") {
		t.Error("expected case-insensitive match on literal segments")
	}
}

func TestValidateResponseSchema_NilAnalysis(t *testing.T) {
	m := &TestUIModel{}
	errs := m.validateResponseSchema("GET", "/users", 200, `{"id":"1"}`)
	if errs != nil {
		t.Errorf("expected nil when analysis is nil, got %v", errs)
	}
}

func TestValidateResponseSchema_NoMatchingEndpoint(t *testing.T) {
	m := &TestUIModel{
		analysis: &analyzer.Analysis{
			Specification: &parser.Specification{
				Endpoints: []parser.Endpoint{
					{Method: "GET", Path: "/users"},
				},
			},
		},
	}
	errs := m.validateResponseSchema("POST", "/other", 200, `{}`)
	if errs != nil {
		t.Errorf("expected nil for unmatched endpoint, got %v", errs)
	}
}

func TestValidateResponseSchema_NoSchemaForStatus(t *testing.T) {
	m := &TestUIModel{
		analysis: &analyzer.Analysis{
			Specification: &parser.Specification{
				Endpoints: []parser.Endpoint{
					{Method: "GET", Path: "/users"},
				},
			},
		},
	}
	errs := m.validateResponseSchema("GET", "/users", 200, `{"id":"1"}`)
	if errs != nil {
		t.Errorf("expected nil when no schema for status code, got %v", errs)
	}
}

func TestValidateResponseSchema_Valid(t *testing.T) {
	m := &TestUIModel{
		analysis: &analyzer.Analysis{
			Specification: &parser.Specification{
				Endpoints: []parser.Endpoint{
					{
						Method: "GET",
						Path:   "/users/{id}",
						ResponseSchemas: map[string]map[string]any{
							"200": {
								"type": "object",
								"properties": map[string]any{
									"id": map[string]any{"type": "string"},
								},
							},
						},
					},
				},
			},
		},
	}
	errs := m.validateResponseSchema("GET", "/users/42", 200, `{"id":"abc"}`)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateResponseSchema_SchemaError(t *testing.T) {
	m := &TestUIModel{
		analysis: &analyzer.Analysis{
			Specification: &parser.Specification{
				Endpoints: []parser.Endpoint{
					{
						Method: "GET",
						Path:   "/users/{id}",
						ResponseSchemas: map[string]map[string]any{
							"200": {
								"type": "object",
								"properties": map[string]any{
									"id": map[string]any{"type": "string"},
								},
							},
						},
					},
				},
			},
		},
	}
	errs := m.validateResponseSchema("GET", "/users/42", 200, `{"id":99}`)
	if len(errs) == 0 {
		t.Error("expected schema validation error for wrong type")
	}
}
