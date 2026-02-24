package main

import (
	"fmt"
	"os"

	"github.com/Octrafic/octrafic-cli/internal/cli"
	internalConfig "github.com/Octrafic/octrafic-cli/internal/config"
	"github.com/Octrafic/octrafic-cli/internal/llm"
	"github.com/Octrafic/octrafic-cli/internal/llm/common"
	"github.com/Octrafic/octrafic-cli/internal/scanner"
	"github.com/spf13/cobra"
)

var (
	scanPath string
	scanOut  string
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan project directory and automatically generate OpenAPI spec.",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := internalConfig.Load()

		provider := internalConfig.GetEnv("PROVIDER")
		apiKey := internalConfig.GetEnv("API_KEY")
		baseURL := internalConfig.GetEnv("BASE_URL")
		modelName := internalConfig.GetEnv("MODEL")

		if err == nil && cfg.Onboarded && (cfg.APIKey != "" || internalConfig.IsLocalProvider(cfg.Provider)) {
			// Use config from file if available and no override
			if provider == "" {
				provider = cfg.Provider
			}
			if apiKey == "" {
				apiKey = cfg.APIKey
			}
			if baseURL == "" {
				baseURL = cfg.BaseURL
			}
			if modelName == "" {
				modelName = cfg.Model
			}
		}

		if provider == "" {
			provider = "claude"
		}
		if apiKey == "" {
			if provider == "openai" || provider == "openrouter" {
				apiKey = os.Getenv("OPENAI_API_KEY")
			} else {
				apiKey = os.Getenv("ANTHROPIC_API_KEY")
			}
		}

		if (apiKey == "" && !internalConfig.IsLocalProvider(provider)) || modelName == "" {
			fmt.Fprintln(os.Stderr, "Error: missing LLM configuration.")
			fmt.Fprintln(os.Stderr, "Please run 'octrafic' to complete interactive onboarding, or configure via environment variables (e.g., OCTRAFIC_PROVIDER, OCTRAFIC_API_KEY, OCTRAFIC_MODEL).")
			fmt.Fprintln(os.Stderr, "Read more: https://docs.octrafic.com/guides/scanner.html")
			os.Exit(1)
		}

		providerConfig := common.ProviderConfig{
			Provider: provider,
			APIKey:   apiKey,
			BaseURL:  baseURL,
			Model:    modelName,
		}

		llmProvider, err := llm.CreateProvider(providerConfig)
		if err != nil {
			fmt.Printf("Error creating LLM provider: %v\n", err)
			os.Exit(1)
		}

		scannerInst, err := scanner.NewScanner(llmProvider, scanPath, scanOut)
		if err != nil {
			fmt.Printf("Error initializing scanner: %v\n", err)
			os.Exit(1)
		}

		if err := cli.StartScannerUI(scannerInst); err != nil {
			fmt.Printf("Error during scan: %v\n", err)
			os.Exit(1)
		}
	},
}

func printScanHelp(cmd *cobra.Command) {
	fmt.Printf("Scan project directory and automatically generate OpenAPI spec.\n\n")
	fmt.Printf("Usage:\n  %s\n\n", cmd.UseLine())

	fmt.Printf("Scan configuration:\n")
	printFlag(cmd, "path", "p", "Target path to scan (default '.')")
	printFlag(cmd, "out", "o", "Output file for the generated OpenAPI spec")

	fmt.Printf("\nLearn more: https://github.com/Octrafic/octrafic-cli\n")
}

func init() {
	scanCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		printScanHelp(cmd)
	})
	scanCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		printScanHelp(cmd)
		return nil
	})

	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().StringVarP(&scanPath, "path", "p", ".", "Target path to scan")
	scanCmd.Flags().StringVarP(&scanOut, "out", "o", "openapi.spec", "Output file for the generated OpenAPI spec")
}
