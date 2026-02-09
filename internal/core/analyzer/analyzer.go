package analyzer

import (
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/core/parser"
	"time"
)

type Analysis struct {
	BaseURL       string                      `json:"base_url"`
	Specification *parser.Specification       `json:"specification"`
	Timestamp     time.Time                   `json:"timestamp"`
	Insights      []string                    `json:"insights"`
	EndpointInfo  map[string]EndpointAnalysis `json:"endpoint_info"`
}

type EndpointAnalysis struct {
	Path           string   `json:"path"`
	Method         string   `json:"method"`
	Purpose        string   `json:"purpose"`
	ExpectedInputs []string `json:"expected_inputs"`
	ExpectedOutput string   `json:"expected_output"`
	TestScenarios  []string `json:"test_scenarios"`
}

func AnalyzeAPI(baseURL string, spec *parser.Specification) (*Analysis, error) {
	analysis := &Analysis{
		BaseURL:       baseURL,
		Specification: spec,
		Timestamp:     time.Now(),
		Insights:      []string{},
		EndpointInfo:  make(map[string]EndpointAnalysis),
	}

	for _, endpoint := range spec.Endpoints {
		key := fmt.Sprintf("%s %s", endpoint.Method, endpoint.Path)

		endpointAnalysis := EndpointAnalysis{
			Path:    endpoint.Path,
			Method:  endpoint.Method,
			Purpose: endpoint.Description,
		}

		analysis.EndpointInfo[key] = endpointAnalysis
	}

	return analysis, nil
}
