package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Octrafic/octrafic-cli/internal/cli"
	"github.com/Octrafic/octrafic-cli/internal/core/analyzer"
	"github.com/Octrafic/octrafic-cli/internal/core/parser"
	"github.com/Octrafic/octrafic-cli/internal/infra/storage"
	"github.com/Octrafic/octrafic-cli/internal/runner"
	"github.com/spf13/cobra"
)

var (
	testPath    string
	testPrompt  string
	testEnvFile string
	testAuto    bool
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run API tests automatically without the interactive UI",
	Run: func(cmd *cobra.Command, args []string) {
		
		if testPath == "" && testPrompt == "" {
			fmt.Println("Error: You must provide a file path (--path) or a prompt (--prompt)")
			os.Exit(1)
		}

		// Initialize required dependencies like auth, project
		authProvider := buildAuthFromFlags()
		if err := authProvider.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid authentication configuration: %v\n", err)
			os.Exit(1)
		}

		if apiURL == "" {
			fmt.Fprintf(os.Stderr, "Error: API URL is required (-u, --url)\n")
			os.Exit(1)
		}

		if testPrompt != "" {
			if specFile == "" {
				fmt.Fprintf(os.Stderr, "Error: Specification file is required (-s, --spec) for prompt execution\n")
				os.Exit(1)
			}
			if err := storage.ValidateSpecPath(specFile); err != nil {
				fmt.Fprintf(os.Stderr, "Error: Specification file not found or invalid: %v\n", err)
				os.Exit(1)
			}

			projectID := generateUUID()
			project, _, err := storage.CreateOrUpdateProject(projectID, projectName, apiURL, specFile, "", true)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to process specification: %v\n", err)
				os.Exit(1)
			}

			specContent, err := parser.ParseSpecification(specFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to parse specification: %v\n", err)
				os.Exit(1)
			}

			analysis, err := analyzer.AnalyzeAPI(apiURL, specContent)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to analyze API: %v\n", err)
				os.Exit(1)
			}

			// Start headless execution with prompt
			exitCode := cli.StartHeadless(apiURL, analysis, project, authProvider, version, testPrompt, testAuto)
			os.Exit(exitCode)
		}

		if testPath != "" {
			var specContent *parser.Specification
			var parseErr error
			
			// Simple logic to select parser if no spec explicit
			if strings.HasSuffix(testPath, ".sh") {
				specContent, parseErr = parser.ParseShellScript(testPath)
			} else {
				// Assumes Postman or other JSON content for now
				rawContent, fileErr := os.ReadFile(testPath)
				if fileErr != nil {
					fmt.Printf("Error reading test file: %v\n", fileErr)
					os.Exit(1)
				}
				// Format detection handles Postman internally
				specContent, parseErr = parser.ParseSpecification(testPath)
				if parseErr != nil {
					// Fallback attempt to force postman if detection failed
					if strings.Contains(string(rawContent), "postman") {
						// Note: parsePostman is not exported from parser package in Go.
						// The main ParseSpecification should handle it if _postman_id or schema matches.
						fmt.Printf("Error: File not recognized as valid API Specification or Postman Collection.\n")
					}
				}
			}

			if parseErr != nil {
				fmt.Printf("Error parsing test file: %v\n", parseErr)
				os.Exit(1)
			}

			if specContent == nil || len(specContent.Endpoints) == 0 {
				fmt.Printf("No tests parsed from file.\n")
				os.Exit(1)
			}

			opts := runner.Options{
				BaseURL:      apiURL,
				AuthProvider: authProvider,
				FailFast:     false,
			}

			os.Exit(runner.RunTests(specContent, opts))
		}
	},
}

func printTestHelp(cmd *cobra.Command) {
	fmt.Printf("Run API tests automatically without the interactive UI\n\n")
	fmt.Printf("Usage:\n  %s\n\n", cmd.UseLine())

	fmt.Printf("Test execution:\n")
	printFlag(cmd, "path", "p", "Path to the test file to execute (Postman collection, sh script, pytest file)")
	printFlag(cmd, "prompt", "", "Instruct the LLM to generate and run specific tests")

	fmt.Printf("\nCore Flags:\n")
	printFlag(cmd, "url", "u", "Base URL of the API to test")
	printFlag(cmd, "spec", "s", "Path to API specification file (OpenAPI/Swagger)")

	fmt.Printf("\nAuthentication:\n")
	printFlag(cmd, "auth", "", "Authentication type: none|bearer|apikey|basic")
	printFlag(cmd, "token", "", "Bearer token value")
	printFlag(cmd, "key", "", "API key header name (e.g., X-API-Key)")
	printFlag(cmd, "value", "", "API key value")
	printFlag(cmd, "user", "", "Username for basic authentication")
	printFlag(cmd, "pass", "", "Password for basic authentication")

	fmt.Printf("\nEnvironment:\n")
	printFlag(cmd, "env", "e", "Path to .env file for environment variables")

	fmt.Printf("\nAdvanced:\n")
	printFlag(cmd, "auto", "a", "Run in Auto-Execute mode without manual confirmation (default true for test cmd)")
	printFlag(cmd, "help", "h", "Show this help message")

	fmt.Printf("\nLearn more: https://github.com/Octrafic/octrafic-cli\n")
}

func init() {
	testCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		printTestHelp(cmd)
	})
	testCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		printTestHelp(cmd)
		return nil
	})

	rootCmd.AddCommand(testCmd)
	testCmd.Flags().StringVarP(&testPath, "path", "p", "", "Path to the test file to execute (Postman collection, sh script, pytest file)")
	testCmd.Flags().StringVar(&testPrompt, "prompt", "", "Instruct the LLM to generate and run specific tests")
	testCmd.Flags().StringVarP(&testEnvFile, "env", "e", "", "Path to .env file for environment variables")
	testCmd.Flags().BoolVarP(&testAuto, "auto", "a", true, "Run in Auto-Execute mode without manual confirmation (default true for test cmd)")

	// Inherit core and auth flags for the test command so they are directly accessible
	testCmd.Flags().StringVarP(&apiURL, "url", "u", "", "Base URL of the API to test")
	testCmd.Flags().StringVarP(&specFile, "spec", "s", "", "Path to API specification file (OpenAPI/Swagger)")
	testCmd.Flags().StringVar(&authType, "auth", "none", "Authentication type: none|bearer|apikey|basic")
	testCmd.Flags().StringVar(&authToken, "token", "", "Bearer token value")
	testCmd.Flags().StringVar(&authKey, "key", "", "API key header name (e.g., X-API-Key)")
	testCmd.Flags().StringVar(&authValue, "value", "", "API key value")
	testCmd.Flags().StringVar(&authUser, "user", "", "Username for basic authentication")
	testCmd.Flags().StringVar(&authPass, "pass", "", "Password for basic authentication")
	testCmd.Flags().StringVar(&debugFilePath, "debug-file", "", "Path to debug log file (enables detailed logging)")
}
