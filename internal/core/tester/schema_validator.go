package tester

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateSchema validates a JSON response body against a JSON schema object.
// Returns a slice of human-readable validation errors (empty if valid).
func ValidateSchema(body string, schema map[string]any) []string {
	if schema == nil || strings.TrimSpace(body) == "" {
		return nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return []string{fmt.Sprintf("response is not valid JSON: %v", err)}
	}

	var errors []string
	validateValue(parsed, schema, "", &errors)
	return errors
}

func validateValue(value any, schema map[string]any, path string, errors *[]string) {
	schemaType, _ := schema["type"].(string)

	if value == nil {
		nullable, _ := schema["nullable"].(bool)
		if typeArr, ok := schema["type"].([]any); ok {
			for _, t := range typeArr {
				if s, ok := t.(string); ok && s == "null" {
					return
				}
			}
		}
		if nullable || schemaType == "" {
			return
		}
		*errors = append(*errors, fmt.Sprintf("%s: is null but not nullable", fieldPath(path)))
		return
	}

	switch schemaType {
	case "object":
		validateObject(value, schema, path, errors)
	case "array":
		validateArray(value, schema, path, errors)
	case "string":
		if _, ok := value.(string); !ok {
			*errors = append(*errors, fieldError(path, "string", value))
		}
	case "number":
		switch value.(type) {
		case float64, int, int64:
		default:
			*errors = append(*errors, fieldError(path, "number", value))
		}
	case "integer":
		if f, ok := value.(float64); ok {
			if f != float64(int64(f)) {
				*errors = append(*errors, fmt.Sprintf("%s: expected integer, got float", fieldPath(path)))
			}
		} else if _, ok := value.(int); !ok {
			*errors = append(*errors, fieldError(path, "integer", value))
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			*errors = append(*errors, fieldError(path, "boolean", value))
		}
	case "":
		if _, hasProps := schema["properties"]; hasProps {
			validateObject(value, schema, path, errors)
		}
		if _, hasItems := schema["items"]; hasItems {
			validateArray(value, schema, path, errors)
		}
	}
}

func validateObject(value any, schema map[string]any, path string, errors *[]string) {
	obj, ok := value.(map[string]any)
	if !ok {
		*errors = append(*errors, fieldError(path, "object", value))
		return
	}

	if required, ok := schema["required"].([]any); ok {
		for _, req := range required {
			if fieldName, ok := req.(string); ok {
				if _, exists := obj[fieldName]; !exists {
					*errors = append(*errors, fmt.Sprintf("%s: required field is missing", childPath(path, fieldName)))
				}
			}
		}
	}

	if properties, ok := schema["properties"].(map[string]any); ok {
		for fieldName, propSchema := range properties {
			propMap, ok := propSchema.(map[string]any)
			if !ok {
				continue
			}
			fieldValue, exists := obj[fieldName]
			if !exists {
				continue
			}
			validateValue(fieldValue, propMap, childPath(path, fieldName), errors)
		}
	}
}

func validateArray(value any, schema map[string]any, path string, errors *[]string) {
	arr, ok := value.([]any)
	if !ok {
		*errors = append(*errors, fieldError(path, "array", value))
		return
	}

	items, ok := schema["items"].(map[string]any)
	if !ok {
		return
	}

	for i, item := range arr {
		validateValue(item, items, fmt.Sprintf("%s[%d]", fieldPath(path), i), errors)
		if len(*errors) >= 10 {
			break
		}
	}
}

func childPath(parent, field string) string {
	if parent == "" {
		return field
	}
	return parent + "." + field
}

func fieldPath(path string) string {
	if path == "" {
		return "response"
	}
	return path
}

func fieldError(path, expected string, actual any) string {
	return fmt.Sprintf("%s: expected %s, got %T", fieldPath(path), expected, actual)
}
