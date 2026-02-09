package auth

import (
	"fmt"
	"net/http"
	"strings"
)

// APIKeyAuth represents API Key authentication
type APIKeyAuth struct {
	Key      string `json:"key"`      // The key name (e.g., "X-API-Key")
	Value    string `json:"value"`    // The key value
	Location string `json:"location"` // "header" or "query"
}

// NewAPIKeyAuth creates a new API Key authentication provider
func NewAPIKeyAuth(key, value, location string) *APIKeyAuth {
	if location == "" {
		location = "header" // default to header
	}
	return &APIKeyAuth{
		Key:      key,
		Value:    value,
		Location: location,
	}
}

// Apply adds the API key to the request
func (a *APIKeyAuth) Apply(req *http.Request) error {
	if err := a.Validate(); err != nil {
		return err
	}

	switch strings.ToLower(a.Location) {
	case "header":
		req.Header.Set(a.Key, a.Value)
	case "query":
		q := req.URL.Query()
		q.Set(a.Key, a.Value)
		req.URL.RawQuery = q.Encode()
	default:
		return fmt.Errorf("invalid location: %s (must be 'header' or 'query')", a.Location)
	}

	return nil
}

// Type returns the authentication type
func (a *APIKeyAuth) Type() string {
	return "apikey"
}

// Validate checks if the configuration is valid
func (a *APIKeyAuth) Validate() error {
	if strings.TrimSpace(a.Key) == "" {
		return fmt.Errorf("API key name cannot be empty")
	}
	if strings.TrimSpace(a.Value) == "" {
		return fmt.Errorf("API key value cannot be empty")
	}
	location := strings.ToLower(a.Location)
	if location != "header" && location != "query" {
		return fmt.Errorf("location must be 'header' or 'query', got: %s", a.Location)
	}
	return nil
}

// Redact returns a copy with the value redacted
func (a *APIKeyAuth) Redact() AuthProvider {
	return &APIKeyAuth{
		Key:      a.Key,
		Value:    RedactString(a.Value),
		Location: a.Location,
	}
}

// String returns a human-readable representation
func (a *APIKeyAuth) String() string {
	return fmt.Sprintf("API Key %s in %s (%s)", a.Key, a.Location, RedactString(a.Value))
}
