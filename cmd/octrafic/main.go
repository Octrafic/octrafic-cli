package main

import (
	"crypto/rand"
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/cli"
	internalConfig "github.com/Octrafic/octrafic-cli/internal/config"
	"github.com/Octrafic/octrafic-cli/internal/core/analyzer"
	"github.com/Octrafic/octrafic-cli/internal/core/auth"
	"github.com/Octrafic/octrafic-cli/internal/core/converter"
	"github.com/Octrafic/octrafic-cli/internal/core/parser"
	"github.com/Octrafic/octrafic-cli/internal/infra/logger"
	"github.com/Octrafic/octrafic-cli/internal/infra/storage"
	"github.com/Octrafic/octrafic-cli/internal/updater"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

const (
	authTypeEnvVar  = "OCTRAFIC_AUTH_TYPE"
	authTokenEnvVar = "OCTRAFIC_AUTH_TOKEN"
	authKeyEnvVar   = "OCTRAFIC_AUTH_KEY"
	authValueEnvVar = "OCTRAFIC_AUTH_VALUE"
	authUserEnvVar  = "OCTRAFIC_AUTH_USER"
	authPassEnvVar  = "OCTRAFIC_AUTH_PASS"
)

var (
	version = "dev"
)

var (
	apiURL      string
	specFile    string
	projectName string

	authType  string
	authToken string
	authKey   string
	authValue string
	authUser  string
	authPass  string

	clearAuth bool

	debugFilePath string

	forceOnboarding bool
	updateFlag      bool

	resumeConversationID string
)

var rootCmd = &cobra.Command{
	Use:   "octrafic",
	Short: "AI-powered CLI tool for API testing and exploration",
	Long: `AI-powered CLI tool for API testing and exploration.
Chat naturally with your APIs - no scripts, no configuration files, just conversation.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	Run: func(cmd *cobra.Command, args []string) {
		// Check for unknown arguments (subcommands)
		if len(args) > 0 {
			printCustomHelp(cmd)
			os.Exit(1)
		}

		// Handle --update flag
		if updateFlag {
			if err := updater.PerformUpdate(version); err != nil {
				fmt.Printf("Update failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("\n✓ Update complete!")
			return
		}

		if forceOnboarding {
			completed := runOnboarding()
			if !completed {
				os.Exit(0)
			}
			fmt.Println("\nOnboarding complete! Run 'octrafic' to start using the tool.")
			return
		}

		// Handle --resume <uuid> flag
		if resumeConversationID != "" {
			handleResumeByUUID(resumeConversationID)
			return
		}

		hasURL := apiURL != ""
		hasSpec := specFile != ""
		hasName := projectName != ""

		// If no flags provided, show fullscreen project+conversation selector
		if !hasURL && !hasSpec && !hasName {
			handleFullscreenSelector()
			return
		}

		// If only project name provided, show conversation selector for that project
		if hasName && !hasURL && !hasSpec {
			handleProjectConversationSelector(projectName)
			return
		}
		if !hasURL {
			fmt.Fprintf(os.Stderr, "Error: API URL is required (-u, --url)\n")
			os.Exit(1)
		}

		if !hasSpec {
			fmt.Fprintf(os.Stderr, "Error: Specification file is required (-s, --spec)\n")
			os.Exit(1)
		}

		authProvider := buildAuthFromFlags()

		if err := authProvider.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid authentication configuration: %v\n", err)
			os.Exit(1)
		}

		if err := storage.ValidateSpecPath(specFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Specification file not found or invalid: %v\n", err)
			os.Exit(1)
		}

		isTemporary := !hasName

		var projectID string
		var project *storage.Project

		if hasName {
			existingProject, err := storage.FindProjectByName(projectName)
			if err == nil {
				needsUpdate := false

				if apiURL != existingProject.BaseURL {
					needsUpdate = true
				}

				if specFile != existingProject.SpecPath {
					needsUpdate = true
				} else if specFile != "" {
					newHash, err := storage.ComputeFileHash(specFile)
					if err == nil && newHash != existingProject.SpecHash {
						needsUpdate = true
					}
				}

				if needsUpdate {
					// Something changed - confirm update
					fmt.Printf("⚠️  Project '%s' has changes.\n", projectName)
					if apiURL != existingProject.BaseURL {
						fmt.Printf("   URL: %s → %s\n", existingProject.BaseURL, apiURL)
					}
					if specFile != "" {
						newHash, _ := storage.ComputeFileHash(specFile)
						if newHash != existingProject.SpecHash {
							fmt.Printf("   Spec: %s (modified)\n", existingProject.SpecPath)
						} else if specFile != existingProject.SpecPath {
							fmt.Printf("   Spec: %s → %s\n", existingProject.SpecPath, specFile)
						}
					}
					fmt.Printf("\nUpdate project? (y/N): ")
					var response string
					_, _ = fmt.Scanln(&response)
					if response != "y" && response != "Y" {
						fmt.Println("Update cancelled")
						os.Exit(0)
					}
				} else {
					fmt.Printf("✓ Project '%s' is up to date (no changes detected)\n", projectName)
				}
				projectID = existingProject.ID
			} else {
				conflict, err := storage.CheckNameConflict(projectName, "")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: Failed to check for project name conflicts: %v\n", err)
					os.Exit(1)
				}
				if conflict != nil {
					fmt.Fprintf(os.Stderr, "Error: Project with name '%s' already exists\n", projectName)
					os.Exit(1)
				}
				projectID = generateUUID()
			}
		} else {
			projectID = generateUUID()
		}
		project, _, err := storage.CreateOrUpdateProject(projectID, projectName, apiURL, specFile, "", isTemporary)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to process specification: %v\n", err)
			os.Exit(1)
		}

		// Auto-save auth with named projects
		if hasName && authType != "none" && authType != "" {
			project.AuthConfig = createAuthConfig()
			if err := storage.SaveProject(project); err != nil {
				fmt.Printf("Warning: failed to save authentication: %v\n", err)
			} else {
				fmt.Println("✓ Authentication saved with project")
			}
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

		cli.StartWithProject(apiURL, analysis, project, authProvider, version)
	},
}

// Custom help function with grouped flags
func printCustomHelp(cmd *cobra.Command) {
	fmt.Printf("AI-powered CLI tool for API testing and exploration\n\n")
	fmt.Printf("Usage:\n  %s\n\n", cmd.UseLine())

	fmt.Printf("Core Flags:\n")
	printFlag(cmd, "url", "u", "Base URL of the API to test")
	printFlag(cmd, "spec", "s", "Path to API specification file (OpenAPI/Swagger)")
	printFlag(cmd, "name", "n", "Project name for saving/loading configuration")
	printFlag(cmd, "resume", "r", "Resume a previous conversation")
	printFlag(cmd, "conversation", "", "Specific conversation ID to resume")

	fmt.Printf("\nAuthentication:\n")
	printFlag(cmd, "auth", "", "Authentication type: none|bearer|apikey|basic")
	printFlag(cmd, "token", "", "Bearer token value")
	printFlag(cmd, "key", "", "API key header name (e.g., X-API-Key)")
	printFlag(cmd, "value", "", "API key value")
	printFlag(cmd, "user", "", "Username for basic authentication")
	printFlag(cmd, "pass", "", "Password for basic authentication")
	printFlag(cmd, "clear-auth", "", "Remove saved authentication from project")

	fmt.Printf("\nAdvanced:\n")
	printFlag(cmd, "debug-file", "", "Path to debug log file (enables detailed logging)")
	printFlag(cmd, "onboarding", "", "Force run onboarding wizard")
	printFlag(cmd, "update", "", "Update octrafic to the latest version")

	fmt.Printf("\nOther:\n")
	printFlag(cmd, "help", "h", "Show this help message")
	printFlag(cmd, "version", "v", "Show the version number")

	fmt.Printf("\nLearn more: https://github.com/Octrafic/octrafic-cli\n")
	fmt.Printf("Report issues: https://github.com/Octrafic/octrafic-cli/issues\n")
}

func printFlag(cmd *cobra.Command, name, shorthand, description string) {
	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		return
	}

	var flagName string
	if shorthand != "" {
		flagName = fmt.Sprintf("-%s, --%s", shorthand, name)
	} else {
		flagName = fmt.Sprintf("    --%s", name)
	}
	fmt.Printf("  %-22s %s\n", flagName, wrapText(description, 55))
}

func wrapText(text string, maxWidth int) string {
	if len(text) <= maxWidth {
		return text
	}

	words := strings.Fields(text)
	var result strings.Builder
	lineLen := 0

	for i, word := range words {
		if lineLen > 0 && lineLen+len(word)+1 > maxWidth {
			result.WriteString("\n" + strings.Repeat(" ", 24))
			lineLen = 24
		} else if lineLen > 0 {
			result.WriteString(" ")
			lineLen++
		}
		result.WriteString(word)
		lineLen += len(word)
		if i < len(words)-1 {
			lineLen++
		}
	}

	return result.String()
}

func init() {
	// Override the default help and usage functions
	cobra.AddTemplateFunc("help", printCustomHelp)
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		printCustomHelp(cmd)
	})
	rootCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		printCustomHelp(cmd)
		return nil
	})
	// Set version template
	rootCmd.SetVersionTemplate("octrafic {{.Version}}\n")

	// Add version flag manually
	rootCmd.Flags().BoolP("version", "v", false, "Show the version number")

	rootCmd.Flags().StringVarP(&apiURL, "url", "u", "", "Base URL of the API to test")
	rootCmd.Flags().StringVarP(&specFile, "spec", "s", "", "Path to API specification file (OpenAPI/Swagger)")
	rootCmd.Flags().StringVarP(&projectName, "name", "n", "", "Project name for saving/loading configuration")

	rootCmd.Flags().StringVar(&authType, "auth", "none", "Authentication type: none|bearer|apikey|basic")
	rootCmd.Flags().StringVar(&authToken, "token", "", "Bearer token value")
	rootCmd.Flags().StringVar(&authKey, "key", "", "API key header name (e.g., X-API-Key)")
	rootCmd.Flags().StringVar(&authValue, "value", "", "API key value")
	rootCmd.Flags().StringVar(&authUser, "user", "", "Username for basic authentication")
	rootCmd.Flags().StringVar(&authPass, "pass", "", "Password for basic authentication")
	rootCmd.Flags().BoolVar(&clearAuth, "clear-auth", false, "Remove saved authentication from project")

	rootCmd.Flags().StringVar(&debugFilePath, "debug-file", "", "Path to debug log file (enables detailed logging)")
	rootCmd.Flags().BoolVar(&forceOnboarding, "onboarding", false, "Force run onboarding wizard")
	rootCmd.Flags().BoolVar(&updateFlag, "update", false, "Update octrafic to the latest version")

	rootCmd.Flags().StringVarP(&resumeConversationID, "resume", "r", "", "Resume a specific conversation by UUID")

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		initLogger()
		// Handle --version flag
		if cmd.Flags().Changed("version") {
			fmt.Printf("octrafic version %s\n", version)
			os.Exit(0)
		}
	}
}

func buildAuthFromEnvironments() auth.AuthProvider {
	authType := os.Getenv(authTypeEnvVar)

	switch authType {
	case "bearer":
		authToken := os.Getenv(authTokenEnvVar)
		if authToken == "" {
			fmt.Fprintf(os.Stderr, "Error: %s is required when using %s bearer\n", authTokenEnvVar, authTypeEnvVar)
			os.Exit(1)
		}
		return auth.NewBearerAuth(authToken)
	case "apikey":
		authKey := os.Getenv(authKeyEnvVar)
		authValue := os.Getenv(authValueEnvVar)

		if authKey == "" || authValue == "" {
			fmt.Fprintf(os.Stderr, "Error: %s and %s are required when using %s apikey\n", authKeyEnvVar, authValueEnvVar, authTypeEnvVar)
			os.Exit(1)
		}
		return auth.NewAPIKeyAuth(authKey, authValue, "header")
	case "basic":
		authUser := os.Getenv(authUserEnvVar)
		authPass := os.Getenv(authPassEnvVar)

		if authUser == "" || authPass == "" {
			fmt.Fprintf(os.Stderr, "Error: %s and %s are required when using %s basic\n", authUserEnvVar, authPassEnvVar, authTypeEnvVar)
			os.Exit(1)
		}
		return auth.NewBasicAuth(authUser, authPass)
	case "none":
		return &auth.NoAuth{}
	default:
		fmt.Fprintf(os.Stderr, "Error: Invalid %s: %s\n", authTypeEnvVar, authType)
		os.Exit(1)
		return nil
	}
}

func buildAuthFromFlags() auth.AuthProvider {
	switch authType {
	case "bearer":
		if authToken == "" {
			fmt.Fprintf(os.Stderr, "Error: --token is required when using --auth bearer\n")
			os.Exit(1)
		}
		return auth.NewBearerAuth(authToken)

	case "apikey":
		if authKey == "" || authValue == "" {
			fmt.Fprintf(os.Stderr, "Error: --key and --value are required when using --auth apikey\n")
			os.Exit(1)
		}
		return auth.NewAPIKeyAuth(authKey, authValue, "header")

	case "basic":
		if authUser == "" || authPass == "" {
			fmt.Fprintf(os.Stderr, "Error: --user and --pass are required when using --auth basic\n")
			os.Exit(1)
		}
		return auth.NewBasicAuth(authUser, authPass)

	case "none":
		return &auth.NoAuth{}

	default:
		fmt.Fprintf(os.Stderr, "Error: Invalid auth type: %s\n", authType)
		os.Exit(1)
		return nil
	}
}

func buildAuthFromProject(project *storage.Project) auth.AuthProvider {
	if project.AuthConfig == nil {
		return &auth.NoAuth{}
	}

	switch project.AuthConfig.Type {
	case "bearer":
		return auth.NewBearerAuth(project.AuthConfig.Token)
	case "apikey":
		return auth.NewAPIKeyAuth(project.AuthConfig.KeyName, project.AuthConfig.KeyValue, "header")
	case "basic":
		return auth.NewBasicAuth(project.AuthConfig.Username, project.AuthConfig.Password)
	default:
		return &auth.NoAuth{}
	}
}

func createAuthConfig() *storage.AuthConfig {
	if authType == "none" || authType == "" {
		return nil
	}

	config := &storage.AuthConfig{
		Type: authType,
	}

	switch authType {
	case "bearer":
		config.Token = authToken
	case "apikey":
		config.KeyName = authKey
		config.KeyValue = authValue
		config.Location = "header"
	case "basic":
		config.Username = authUser
		config.Password = authPass
	}

	return config
}

func generateUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to generate UUID: %v\n", err)
		os.Exit(1)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func loadAndStartProject(project *storage.Project) {
	project.LastAccessedAt = time.Now()
	if err := storage.SaveProject(project); err != nil {
		fmt.Printf("Warning: failed to update last accessed time: %v\n", err)
	}

	var authProvider auth.AuthProvider
	if authType != "" && authType != "none" {
		authProvider = buildAuthFromFlags()
	} else if authEnv, exists := os.LookupEnv(authTypeEnvVar); exists && authEnv != "" {
		authProvider = buildAuthFromEnvironments()
	} else if project.HasAuth() {
		authProvider = buildAuthFromProject(project)
		fmt.Printf("✓ Using saved authentication (%s)\n", project.AuthConfig.Type)
	} else {
		authProvider = &auth.NoAuth{}
	}

	var analysis *analyzer.Analysis

	if storage.HasEndpoints(project.ID, project.IsTemporary) {
		fmt.Printf("✓ Using cached endpoints\n")
		analysis = &analyzer.Analysis{
			BaseURL:      project.BaseURL,
			Timestamp:    time.Now(),
			EndpointInfo: make(map[string]analyzer.EndpointAnalysis),
		}
	} else {
		if err := storage.ValidateSpecPath(project.SpecPath); err != nil {
			fmt.Printf("⚠️  Warning: %v\n", err)
			fmt.Printf("Please provide a new path to specification file: ")
			var newPath string
			_, _ = fmt.Scanln(&newPath)
			if err := storage.ValidateSpecPath(newPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error: Spec path validation failed: %v\n", err)
				os.Exit(1)
			}
			project.SpecPath = newPath
			if err := storage.SaveProject(project); err != nil {
				fmt.Printf("Warning: Failed to save updated spec path: %v\n", err)
			}
		}

		specContent, err := parser.ParseSpecification(project.SpecPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to parse specification: %v\n", err)
			os.Exit(1)
		}

		analysis, err = analyzer.AnalyzeAPI(project.BaseURL, specContent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to analyze API: %v\n", err)
			os.Exit(1)
		}
	}

	cli.StartWithProject(project.BaseURL, analysis, project, authProvider, version)
}

// handleFullscreenSelector shows fullscreen project+conversation selector
func handleFullscreenSelector() {
	for {
		projects, err := storage.ListNamedProjects()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to load projects: %v\n", err)
			os.Exit(1)
		}

		selectorModel := cli.NewResumeSelectorModel(projects, version)
		p := tea.NewProgram(selectorModel, tea.WithAltScreen())

		finalModel, err := p.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to run project selector: %v\n", err)
			os.Exit(1)
		}

		result, ok := finalModel.(cli.ResumeSelectorModel)
		if !ok || result.IsCancelled() {
			os.Exit(0)
		}

		if result.ShouldCreateNewProject() {
			promptNewProjectInteractive()
			continue
		}

		project := result.GetSelectedProject()
		if project == nil {
			os.Exit(0)
		}

		if result.ShouldCreateNew() {
			loadAndStartProject(project)
			return
		}

		conversation := result.GetSelectedConversation()
		if conversation != nil {
			loadAndStartConversation(project, conversation)
			return
		}
	}
}

func promptNewProjectInteractive() bool {
	creatorModel := cli.NewProjectCreatorModel()
	p := tea.NewProgram(creatorModel, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return false
	}

	result, ok := finalModel.(cli.ProjectCreatorModel)
	if !ok || result.IsCancelled() || !result.IsConfirmed() {
		return false
	}

	url, specPath, name := result.GetProjectData()
	if result.NeedsConversion() {
		convertedPath, err := converter.ConvertToOpenAPI(specPath, result.GetDetectedFormat())
		if err != nil {
			return false
		}
		specPath = convertedPath
	}

	projectID := generateUUID()
	project, _, err := storage.CreateOrUpdateProject(projectID, name, url, specPath, "", false)
	if err != nil {
		return false
	}

	// Handle auth
	authType, authData := result.GetAuthConfig()
	if authType != "none" && authData != nil {
		project.AuthConfig = &storage.AuthConfig{
			Type:     authType,
			Token:    authData["token"],
			KeyName:  authData["key"],
			KeyValue: authData["value"],
			Location: authData["location"],
			Username: authData["username"],
			Password: authData["password"],
		}
		_ = storage.SaveProject(project)
	}

	return true
}

// handleProjectConversationSelector shows conversation selector for specific project
func handleProjectConversationSelector(projectName string) {
	project, err := storage.FindProjectByName(projectName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Project '%s' not found\n", projectName)
		os.Exit(1)
	}

	convListModel := cli.NewConversationListModel(project)
	p := tea.NewProgram(convListModel, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to run conversation selector: %v\n", err)
		os.Exit(1)
	}

	result, ok := finalModel.(cli.ConversationListModel)
	if !ok || result.IsCancelled() {
		os.Exit(0)
	}

	if result.ShouldCreateNew() {
		loadAndStartProject(project)
		return
	}

	conversation := result.GetSelectedConversation()
	if conversation != nil {
		loadAndStartConversation(project, conversation)
	}
}

// handleResumeByUUID resumes specific conversation by UUID
func handleResumeByUUID(conversationID string) {
	// Need to find which project this conversation belongs to
	projects, err := storage.ListNamedProjects()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to load projects: %v\n", err)
		os.Exit(1)
	}

	var foundProject *storage.Project
	var foundConversation *storage.Conversation

	for _, project := range projects {
		conv, err := storage.LoadConversation(project.ID, conversationID)
		if err == nil && conv != nil {
			foundProject = project
			foundConversation = conv
			break
		}
	}

	if foundProject == nil || foundConversation == nil {
		fmt.Fprintf(os.Stderr, "Error: Conversation with ID '%s' not found\n", conversationID)
		os.Exit(1)
	}

	loadAndStartConversation(foundProject, foundConversation)
}

func loadAndStartConversation(project *storage.Project, conversation *storage.Conversation) {
	// Update last accessed time
	project.LastAccessedAt = time.Now()
	if err := storage.SaveProject(project); err != nil {
		fmt.Printf("Warning: failed to update last accessed time: %v\n", err)
	}

	// Build auth provider
	var authProvider auth.AuthProvider
	if authType != "" && authType != "none" {
		authProvider = buildAuthFromFlags()
	} else if authEnv, exists := os.LookupEnv(authTypeEnvVar); exists && authEnv != "" {
		authProvider = buildAuthFromEnvironments()
	} else if project.HasAuth() {
		authProvider = buildAuthFromProject(project)
		fmt.Printf("✓ Using saved authentication (%s)\n", project.AuthConfig.Type)
	} else {
		authProvider = &auth.NoAuth{}
	}

	// Load analysis
	var analysis *analyzer.Analysis

	if storage.HasEndpoints(project.ID, project.IsTemporary) {
		fmt.Printf("✓ Using cached endpoints\n")
		analysis = &analyzer.Analysis{
			BaseURL:      project.BaseURL,
			Timestamp:    time.Now(),
			EndpointInfo: make(map[string]analyzer.EndpointAnalysis),
		}
	} else {
		if err := storage.ValidateSpecPath(project.SpecPath); err != nil {
			fmt.Printf("⚠️  Warning: %v\n", err)
			fmt.Printf("Please provide a new path to specification file: ")
			var newPath string
			_, _ = fmt.Scanln(&newPath)
			if err := storage.ValidateSpecPath(newPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error: Spec path validation failed: %v\n", err)
				os.Exit(1)
			}
			project.SpecPath = newPath
			if err := storage.SaveProject(project); err != nil {
				fmt.Printf("Warning: Failed to save updated spec path: %v\n", err)
			}
		}

		specContent, err := parser.ParseSpecification(project.SpecPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to parse specification: %v\n", err)
			os.Exit(1)
		}

		analysis, err = analyzer.AnalyzeAPI(project.BaseURL, specContent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to analyze API: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("✓ Loading conversation: %s\n", conversation.Title)

	cli.StartWithConversation(project.BaseURL, analysis, project, authProvider, version, conversation.ID)
}

func main() {
	_ = godotenv.Load()
	if isFirstLaunch, err := internalConfig.IsFirstLaunch(); err == nil && isFirstLaunch {
		completed := runOnboarding()
		if !completed {
			os.Exit(0)
		}
	}

	if version != "dev" {
		checkForUpdate(version)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if debugFilePath != "" {
		logger.Close()
	}
}

func checkForUpdate(currentVersion string) {
	cfg, err := internalConfig.Load()
	if err != nil {
		return
	}

	if !cfg.ShouldCheckForUpdate() {
		return
	}

	info, err := updater.CheckLatestVersion(currentVersion)
	if err != nil {
		return
	}

	cfg.LastUpdateCheck = time.Now()
	cfg.LatestVersion = info.LatestVersion
	_ = cfg.Save()
}

func runOnboarding() bool {
	onboardingModel := cli.NewOnboardingModel()
	p := tea.NewProgram(onboardingModel)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to run onboarding: %v\n", err)
		os.Exit(1)
	}

	result, ok := finalModel.(cli.OnboardingModel)
	if !ok {
		return false
	}

	if result.WasCompleted() {
		fmt.Println("\n✓ Configuration saved!")
		return true
	}

	return false
}

func initLogger() {
	if debugFilePath != "" {
		if err := logger.Init(true, debugFilePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("Octrafic starting", logger.String("log_file", debugFilePath), logger.Bool("debug", true))
	}
}
