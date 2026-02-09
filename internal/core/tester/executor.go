package tester

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/core/auth"
	"io"
	"net/http"
	"strings"
	"time"
)

type TestResult struct {
	StatusCode   int
	ResponseBody string
	Duration     time.Duration
	Error        error
}

type Executor struct {
	baseURL      string
	client       *http.Client
	authProvider auth.AuthProvider
}

func NewExecutor(baseURL string, authProvider auth.AuthProvider) *Executor {
	return &Executor{
		baseURL:      baseURL,
		authProvider: authProvider,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// UpdateAuthProvider updates the authentication provider
func (e *Executor) UpdateAuthProvider(authProvider auth.AuthProvider) {
	e.authProvider = authProvider
}

func (e *Executor) ExecuteTest(method, endpoint string, headers map[string]string, body any) (*TestResult, error) {
	startTime := time.Now()

	// Build full URL
	fullURL := e.baseURL + endpoint
	if !strings.HasPrefix(fullURL, "http://") && !strings.HasPrefix(fullURL, "https://") {
		fullURL = "http://" + fullURL
	}

	// Prepare request body
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return &TestResult{Error: fmt.Errorf("failed to marshal body: %w", err)}, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	// Create request
	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return &TestResult{Error: fmt.Errorf("failed to create request: %w", err)}, err
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Apply authentication
	if e.authProvider != nil {
		if err := e.authProvider.Apply(req); err != nil {
			return &TestResult{Error: fmt.Errorf("failed to apply auth: %w", err)}, err
		}
	}

	// Execute request
	resp, err := e.client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		return &TestResult{
			Duration: duration,
			Error:    fmt.Errorf("request failed: %w", err),
		}, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &TestResult{
			StatusCode: resp.StatusCode,
			Duration:   duration,
			Error:      fmt.Errorf("failed to read response: %w", err),
		}, err
	}

	return &TestResult{
		StatusCode:   resp.StatusCode,
		ResponseBody: string(respBody),
		Duration:     duration,
		Error:        nil,
	}, nil
}
