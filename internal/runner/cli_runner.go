package runner

import (
	"fmt"

	"github.com/Octrafic/octrafic-cli/internal/core/auth"
	"github.com/Octrafic/octrafic-cli/internal/core/parser"
	"github.com/Octrafic/octrafic-cli/internal/core/tester"
)

// Options configuring the headless test runner
type Options struct {
	BaseURL      string
	AuthProvider auth.AuthProvider
	FailFast     bool
}

// RunTests Headlessly executes tests inside the specification
func RunTests(spec *parser.Specification, opts Options) int {
	if len(spec.Endpoints) == 0 {
		fmt.Println("No tests found to execute in the provided spec.")
		return 0
	}

	fmt.Printf("Starting execution of %d tests in %s spec...\n\n", len(spec.Endpoints), spec.Format)

	executor := tester.NewExecutor(opts.BaseURL, opts.AuthProvider)
	failed := 0

	for i, endpoint := range spec.Endpoints {
		fmt.Printf("[%d/%d] Running %s %s... ", i+1, len(spec.Endpoints), endpoint.Method, endpoint.Path)

		headers := make(map[string]string)
		var body any

		if endpoint.RequestBody != "" {
			body = endpoint.RequestBody
		}

		result, err := executor.ExecuteTest(endpoint.Method, endpoint.Path, headers, body, endpoint.RequiresAuth)
		if err != nil {
			fmt.Printf("FAILED ❌\n")
			fmt.Printf("      Error: %v\n", err)
			failed++
			if opts.FailFast {
				break
			}
			continue
		}

		// Very basic status code assertion if we had assertions, but for now
		// assume 2xx is pass.
		if result.StatusCode >= 200 && result.StatusCode < 400 {
			fmt.Printf("OK (%dms) ✅\n", result.Duration.Milliseconds())
		} else {
			fmt.Printf("FAILED (Status %d) ❌\n", result.StatusCode)
			fmt.Printf("      Response: %s\n", result.ResponseBody)
			failed++
			if opts.FailFast {
				break
			}
		}
	}

	fmt.Println("\n=============================================")
	fmt.Printf("Summary: %d executed, %d passed, %d failed\n", len(spec.Endpoints), len(spec.Endpoints)-failed, failed)

	if failed > 0 {
		return 1
	}
	return 0
}
