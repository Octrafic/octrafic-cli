package tester

import (
	"testing"
)

func TestValidateSchema_Valid(t *testing.T) {
	schema := map[string]any{
		"type":     "object",
		"required": []any{"id", "email"},
		"properties": map[string]any{
			"id":    map[string]any{"type": "string"},
			"email": map[string]any{"type": "string"},
			"age":   map[string]any{"type": "integer"},
		},
	}

	errs := ValidateSchema(`{"id":"abc","email":"x@y.com","age":30}`, schema)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateSchema_MissingRequired(t *testing.T) {
	schema := map[string]any{
		"type":     "object",
		"required": []any{"id", "email"},
		"properties": map[string]any{
			"id":    map[string]any{"type": "string"},
			"email": map[string]any{"type": "string"},
		},
	}

	errs := ValidateSchema(`{"id":"abc"}`, schema)
	if len(errs) != 1 {
		t.Errorf("expected 1 error (missing email), got: %v", errs)
	}
}

func TestValidateSchema_WrongType(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	}

	errs := ValidateSchema(`{"id":42}`, schema)
	if len(errs) != 1 {
		t.Errorf("expected 1 type error, got: %v", errs)
	}
}

func TestValidateSchema_NullNonNullable(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
	}

	errs := ValidateSchema(`{"id":null}`, schema)
	if len(errs) != 1 {
		t.Errorf("expected null error, got: %v", errs)
	}
}

func TestValidateSchema_Nullable(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string", "nullable": true},
		},
	}

	errs := ValidateSchema(`{"id":null}`, schema)
	if len(errs) != 0 {
		t.Errorf("expected no errors for nullable field, got: %v", errs)
	}
}

func TestValidateSchema_Array(t *testing.T) {
	schema := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "integer"},
			},
		},
	}

	errs := ValidateSchema(`[{"id":1},{"id":2}]`, schema)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}

	errs = ValidateSchema(`[{"id":"not-int"}]`, schema)
	if len(errs) == 0 {
		t.Error("expected type error in array item")
	}
}

func TestValidateSchema_InvalidJSON(t *testing.T) {
	schema := map[string]any{"type": "object"}
	errs := ValidateSchema(`not json`, schema)
	if len(errs) != 1 {
		t.Errorf("expected JSON parse error, got: %v", errs)
	}
}

func TestValidateSchema_EmptyBody(t *testing.T) {
	schema := map[string]any{"type": "object"}
	errs := ValidateSchema("", schema)
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty body, got: %v", errs)
	}
}
