package tester

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// ResolvePath extracts a value from a parsed JSON object using dot notation.
// Supports: "id", "user.name", "items.0.price"
func ResolvePath(data any, path string) (any, bool) {
	if path == "" {
		return data, true
	}
	parts := strings.SplitN(path, ".", 2)
	key := parts[0]

	switch v := data.(type) {
	case map[string]any:
		val, ok := v[key]
		if !ok {
			return nil, false
		}
		if len(parts) == 1 {
			return val, true
		}
		return ResolvePath(val, parts[1])
	case []any:
		idx, err := strconv.Atoi(key)
		if err != nil || idx < 0 || idx >= len(v) {
			return nil, false
		}
		if len(parts) == 1 {
			return v[idx], true
		}
		return ResolvePath(v[idx], parts[1])
	}
	return nil, false
}

// RunAssertions evaluates a list of assertions against a JSON response body.
// Returns a list of failure messages (empty = all passed).
func RunAssertions(body string, assertions []map[string]any) []string {
	if len(assertions) == 0 || strings.TrimSpace(body) == "" {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return []string{fmt.Sprintf("cannot parse response for assertions: %v", err)}
	}

	var failures []string
	for _, a := range assertions {
		field, _ := a["field"].(string)
		op, _ := a["op"].(string)
		expected := a["value"]

		val, exists := ResolvePath(parsed, field)

		switch op {
		case "exists":
			if !exists || val == nil {
				failures = append(failures, fmt.Sprintf("field %q: expected to exist", field))
			}
		case "not_exists":
			if exists && val != nil {
				failures = append(failures, fmt.Sprintf("field %q: expected to not exist", field))
			}
		case "eq":
			if !exists {
				failures = append(failures, fmt.Sprintf("field %q: expected %v but field missing", field, expected))
			} else if !equal(val, expected) {
				failures = append(failures, fmt.Sprintf("field %q: expected %v, got %v", field, expected, val))
			}
		case "neq":
			if exists && equal(val, expected) {
				failures = append(failures, fmt.Sprintf("field %q: expected not %v", field, expected))
			}
		case "contains":
			if !exists {
				failures = append(failures, fmt.Sprintf("field %q: expected to contain %v but field missing", field, expected))
			} else {
				s, ok1 := val.(string)
				sub, ok2 := expected.(string)
				if !ok1 || !ok2 {
					failures = append(failures, fmt.Sprintf("field %q: 'contains' requires string values", field))
				} else if !strings.Contains(s, sub) {
					failures = append(failures, fmt.Sprintf("field %q: %q does not contain %q", field, s, sub))
				}
			}
		case "gt", "gte", "lt", "lte":
			if !exists {
				failures = append(failures, fmt.Sprintf("field %q: expected numeric comparison but field missing", field))
			} else {
				got, errG := toFloat(val)
				exp, errE := toFloat(expected)
				if errG != nil || errE != nil {
					failures = append(failures, fmt.Sprintf("field %q: '%s' requires numeric values", field, op))
				} else {
					ok := false
					switch op {
					case "gt":
						ok = got > exp
					case "gte":
						ok = got >= exp
					case "lt":
						ok = got < exp
					case "lte":
						ok = got <= exp
					}
					if !ok {
						failures = append(failures, fmt.Sprintf("field %q: %v %s %v failed", field, got, op, exp))
					}
				}
			}
		default:
			failures = append(failures, fmt.Sprintf("field %q: unknown operator %q", field, op))
		}
	}
	return failures
}

func equal(a, b any) bool {
	af, errA := toFloat(a)
	bf, errB := toFloat(b)
	if errA == nil && errB == nil {
		return af == bf
	}
	return reflect.DeepEqual(a, b)
}

func toFloat(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case json.Number:
		return n.Float64()
	case string:
		return strconv.ParseFloat(n, 64)
	}
	return 0, fmt.Errorf("not a number")
}
