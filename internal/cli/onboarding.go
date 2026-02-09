package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Octrafic/octrafic-cli/internal/config"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// OnboardingState tracks the onboarding progress
type OnboardingState int

const (
	OnboardingWelcome OnboardingState = iota
	OnboardingProvider
	OnboardingAPIKey
	OnboardingServerURL
	OnboardingSelectModel
	OnboardingComplete
)

// OnboardingModel handles the onboarding flow
type OnboardingModel struct {
	state            OnboardingState
	provider         string
	selectedProvider int // 0 = anthropic, 1 = openrouter, 2 = openai, 3 = ollama, 4 = llamacpp
	apiKey           string
	apiKeyInput      textinput.Model
	serverURL        string
	serverURLInput   textinput.Model
	models           []string
	filteredModels   []string // Filtered list based on search
	selectedModel    int
	modelSearchInput textinput.Model
	errorMsg         string
	isTestingKey     bool
	width            int
	height           int
	completed        bool // true if user finished onboarding successfully
}

// OnboardingMsg signals state transitions
type OnboardingMsg struct {
	NextState OnboardingState
}

// KeyTestResult signals the result of API key testing
type KeyTestResult struct {
	Success  bool
	Models   []string
	Error    string
	Provider string // Which provider was actually tested
}

// NewOnboardingModel creates the initial onboarding model
func NewOnboardingModel() OnboardingModel {
	// API key input
	ti := textinput.New()
	ti.Placeholder = "sk-ant-..."
	ti.CharLimit = 200
	ti.Width = 50
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'

	// Server URL input (for local providers)
	serverURLInput := textinput.New()
	serverURLInput.Placeholder = "http://localhost:11434"
	serverURLInput.CharLimit = 200
	serverURLInput.Width = 50

	// Model search input
	searchInput := textinput.New()
	searchInput.Placeholder = "Search models..."
	searchInput.CharLimit = 100
	searchInput.Width = 50

	return OnboardingModel{
		state:            OnboardingWelcome,
		width:            80,
		height:           24,
		models:           []string{},
		filteredModels:   []string{},
		apiKeyInput:      ti,
		serverURLInput:   serverURLInput,
		modelSearchInput: searchInput,
		selectedProvider: 0, // Default to Anthropic
	}
}

// Init initializes the onboarding model
func (m OnboardingModel) Init() tea.Cmd {
	return nil
}

// WasCompleted returns true if the user completed onboarding
func (m OnboardingModel) WasCompleted() bool {
	return m.completed
}

// filterModels filters the model list based on search query
func (m *OnboardingModel) filterModels() {
	query := strings.ToLower(m.modelSearchInput.Value())

	if query == "" {
		// No filter - show all models
		m.filteredModels = m.models
	} else {
		// Filter models case-insensitively
		m.filteredModels = []string{}
		for _, model := range m.models {
			if strings.Contains(strings.ToLower(model), query) {
				m.filteredModels = append(m.filteredModels, model)
			}
		}
	}

	// Reset selection if out of bounds
	if m.selectedModel >= len(m.filteredModels) {
		m.selectedModel = 0
		if len(m.filteredModels) > 0 {
			m.selectedModel = 0
		}
	}
}

// Update handles messages during onboarding
func (m OnboardingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Special handling for API key state - let textinput handle typing first
		if m.state == OnboardingAPIKey {
			// Handle special keys first
			switch msg.String() {
			case "enter":
				if len(m.apiKeyInput.Value()) > 0 {
					m.apiKey = m.apiKeyInput.Value()
					m.errorMsg = ""
					m.isTestingKey = true
					return m, m.testAPIKey()
				}
				return m, nil
			case "esc":
				// Clear input and go back
				m.apiKeyInput.SetValue("")
				m.errorMsg = ""
				m.state = OnboardingProvider
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				// Let textinput handle normal typing
				m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
				return m, cmd
			}
		}

		// Special handling for server URL state (local providers)
		if m.state == OnboardingServerURL {
			switch msg.String() {
			case "enter":
				url := m.serverURLInput.Value()
				if url == "" {
					// Use placeholder as default
					if m.provider == "ollama" {
						url = "http://localhost:11434"
					} else {
						url = "http://localhost:8080"
					}
				}
				m.serverURL = url
				m.errorMsg = ""
				m.isTestingKey = true
				return m, m.testServerConnection()
			case "esc":
				m.serverURLInput.SetValue("")
				m.errorMsg = ""
				m.state = OnboardingProvider
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				m.serverURLInput, cmd = m.serverURLInput.Update(msg)
				return m, cmd
			}
		}

		// Special handling for model selection state - let textinput handle typing
		if m.state == OnboardingSelectModel {
			// Handle special keys first
			switch msg.String() {
			case "up", "k":
				if m.selectedModel > 0 {
					m.selectedModel--
				}
				return m, nil
			case "down", "j":
				if m.selectedModel < len(m.filteredModels)-1 {
					m.selectedModel++
				}
				return m, nil
			case "enter":
				if len(m.filteredModels) > 0 && m.selectedModel < len(m.filteredModels) {
					m.state = OnboardingComplete
					m.completed = true
					return m, m.saveConfig()
				}
				return m, nil
			case "esc":
				m.modelSearchInput.SetValue("")
				m.errorMsg = ""
				if config.IsLocalProvider(m.provider) {
					m.state = OnboardingServerURL
					m.serverURLInput.Focus()
				} else {
					m.apiKeyInput.SetValue("")
					m.state = OnboardingAPIKey
				}
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				// Let textinput handle normal typing for search
				m.modelSearchInput, cmd = m.modelSearchInput.Update(msg)
				m.filterModels()
				return m, cmd
			}
		}

		// For other states, use handleKeyPress
		return m.handleKeyPress(msg)

	case OnboardingMsg:
		m.state = msg.NextState

	case KeyTestResult:
		m.isTestingKey = false
		if msg.Success {
			m.models = msg.Models
			m.filteredModels = msg.Models // Initially show all
			m.state = OnboardingSelectModel
			m.modelSearchInput.Focus()
			if len(m.models) > 0 {
				m.selectedModel = 0
			}
		} else {
			// Show error with provider info for debugging
			m.errorMsg = fmt.Sprintf("%s (provider: %s)", msg.Error, msg.Provider)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, cmd
}

func (m *OnboardingModel) handleKeyPress(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case OnboardingWelcome:
		switch keyMsg.String() {
		case "ctrl+c":
			return m, tea.Quit
		default:
			m.state = OnboardingProvider
		}

	case OnboardingProvider:
		switch keyMsg.String() {
		case "up", "k":
			if m.selectedProvider > 0 {
				m.selectedProvider--
			}
		case "down", "j":
			if m.selectedProvider < 4 { // 0=anthropic, 1=openrouter, 2=openai, 3=ollama, 4=llamacpp
				m.selectedProvider++
			}
		case "enter":
			switch m.selectedProvider {
			case 0:
				m.provider = "anthropic"
				m.state = OnboardingAPIKey
				m.apiKeyInput.Focus()
			case 1:
				m.provider = "openrouter"
				m.state = OnboardingAPIKey
				m.apiKeyInput.Focus()
			case 2:
				m.provider = "openai"
				m.state = OnboardingAPIKey
				m.apiKeyInput.Focus()
			case 3:
				m.provider = "ollama"
				m.serverURLInput.SetValue("http://localhost:11434")
				m.state = OnboardingServerURL
				m.serverURLInput.Focus()
			case 4:
				m.provider = "llamacpp"
				m.serverURLInput.SetValue("http://localhost:8080")
				m.state = OnboardingServerURL
				m.serverURLInput.Focus()
			}
		case "esc":
			m.state = OnboardingWelcome
		case "ctrl+c":
			return m, tea.Quit
		}

	case OnboardingComplete:
		return m, tea.Quit
	}

	return m, nil
}

func (m *OnboardingModel) testAPIKey() tea.Cmd {
	// Capture provider and apiKey in local variables to avoid closure issues
	provider := m.provider
	apiKey := m.apiKey

	return func() tea.Msg {
		var models []string
		var err error

		// DEBUG: Check what provider value we have
		if provider == "" {
			return KeyTestResult{
				Success:  false,
				Error:    "Provider is empty string - this is a bug!",
				Provider: "(empty)",
			}
		}

		switch provider {
		case "anthropic":
			models, err = fetchAnthropicModels(apiKey)
		case "openrouter":
			models, err = fetchOpenRouterModels(apiKey)
		case "openai":
			models, err = fetchOpenAIModels(apiKey)
		default:
			return KeyTestResult{
				Success:  false,
				Error:    fmt.Sprintf("Unknown provider: %s", provider),
				Provider: provider,
			}
		}

		if err != nil {
			return KeyTestResult{
				Success:  false,
				Error:    err.Error(),
				Provider: provider,
			}
		}

		// DEBUG: Add provider info to success message
		return KeyTestResult{
			Success:  true,
			Models:   models,
			Provider: fmt.Sprintf("%s (%d models)", provider, len(models)),
		}
	}
}

func (m *OnboardingModel) testServerConnection() tea.Cmd {
	provider := m.provider
	serverURL := m.serverURL

	return func() tea.Msg {
		models, err := fetchLocalModels(serverURL)
		if err != nil {
			return KeyTestResult{
				Success:  false,
				Error:    err.Error(),
				Provider: provider,
			}
		}

		return KeyTestResult{
			Success:  true,
			Models:   models,
			Provider: fmt.Sprintf("%s (%d models)", provider, len(models)),
		}
	}
}

func (m *OnboardingModel) saveConfig() tea.Cmd {
	return func() tea.Msg {
		cfg := config.Config{
			Provider:  m.provider,
			APIKey:    m.apiKey,
			BaseURL:   m.serverURL,
			Model:     m.filteredModels[m.selectedModel],
			Onboarded: true,
		}

		if err := cfg.Save(); err != nil {
			return tea.Quit()
		}

		return tea.Quit()
	}
}

// View renders the onboarding UI
func (m OnboardingModel) View() string {
	switch m.state {
	case OnboardingWelcome:
		return m.renderWelcome()
	case OnboardingProvider:
		return m.renderProvider()
	case OnboardingAPIKey:
		return m.renderAPIKey()
	case OnboardingServerURL:
		return m.renderServerURL()
	case OnboardingSelectModel:
		return m.renderModel()
	case OnboardingComplete:
		return m.renderComplete()
	}
	return ""
}

func (m OnboardingModel) renderWelcome() string {
	// Style the logo with gradient colors
	logoLines := strings.Split(Logo, "\n")
	styledLogo := make([]string, len(logoLines))
	for i, line := range logoLines {
		var styledLine strings.Builder
		for _, char := range line {
			if char == '░' {
				styledLine.WriteString(lipgloss.NewStyle().Foreground(Theme.TextSubtle).Render(string(char)))
			} else {
				color := Theme.LogoGradient[i%len(Theme.LogoGradient)]
				styledLine.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(string(char)))
			}
		}
		styledLogo[i] = styledLine.String()
	}

	logoBlock := strings.Join(styledLogo, "\n")

	subtitle := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("API Testing CLI powered by AI")

	pressKey := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Render("Press any key to continue...")

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		logoBlock,
		"",
		subtitle,
		"",
		"",
		pressKey,
	)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m OnboardingModel) renderProvider() string {
	title := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Render("Welcome to Octrafic!")

	subtitle := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("Let's configure your AI provider")

	providers := []string{"Anthropic", "OpenRouter", "OpenAI", "Ollama (local)", "llama.cpp (local)"}
	var providerItems []string

	for i, provider := range providers {
		if i == m.selectedProvider {
			prefix := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render("▶")
			providerLine := prefix + " " + lipgloss.NewStyle().Foreground(Theme.Text).Bold(true).Render(provider)
			providerItems = append(providerItems, providerLine)
		} else {
			providerLine := "  " + lipgloss.NewStyle().Foreground(Theme.TextMuted).Render(provider)
			providerItems = append(providerItems, providerLine)
		}
	}

	providerList := strings.Join(providerItems, "\n")

	help := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("↑/↓ to select • Enter to continue • ESC to go back")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		title,
		"",
		subtitle,
		"",
		"",
		providerList,
		"",
		"",
		help,
	)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func providerDisplayName(provider string) string {
	switch provider {
	case "anthropic":
		return "Anthropic"
	case "openrouter":
		return "OpenRouter"
	case "openai":
		return "OpenAI"
	case "ollama":
		return "Ollama"
	case "llamacpp":
		return "llama.cpp"
	default:
		return provider
	}
}

func (m OnboardingModel) renderAPIKey() string {
	providerDisplay := providerDisplayName(m.provider)

	title := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Render("Enter your API Key")

	providerLabel := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("Provider: ") +
		lipgloss.NewStyle().
			Foreground(Theme.Primary).
			Bold(true).
			Render(providerDisplay)

	input := m.renderMaskedKey()

	var statusLine string
	if m.isTestingKey {
		spinner := lipgloss.NewStyle().Foreground(Theme.Primary).Render("⠋")
		statusLine = spinner + " " + lipgloss.NewStyle().Foreground(Theme.TextMuted).Render("Testing API key...")
	} else if m.errorMsg != "" {
		statusLine = lipgloss.NewStyle().Foreground(Theme.Error).Render("✗ " + m.errorMsg)
	}

	help := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("Enter to test • ESC to go back")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		title,
		"",
		providerLabel,
		"",
		"",
		input,
	)

	if statusLine != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", statusLine)
	}

	content = lipgloss.JoinVertical(lipgloss.Left, content, "", "", help)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m OnboardingModel) renderModel() string {
	if len(m.models) == 0 {
		spinner := lipgloss.NewStyle().Foreground(Theme.Primary).Render("⠋")
		loading := lipgloss.JoinVertical(
			lipgloss.Left,
			"",
			spinner+" "+lipgloss.NewStyle().Foreground(Theme.TextMuted).Render("Loading models..."),
		)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, loading)
	}

	providerDisplay := providerDisplayName(m.provider)

	title := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Render("Select your AI Model")

	providerLabel := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("Provider: ") +
		lipgloss.NewStyle().
			Foreground(Theme.Primary).
			Render(providerDisplay)

	// Show search box
	searchLabel := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("Search: ")
	searchBox := searchLabel + m.modelSearchInput.View()

	// Show count
	countText := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render(fmt.Sprintf("(%d/%d models)", len(m.filteredModels), len(m.models)))

	// Build model list from filtered models
	var modelItems []string
	maxVisible := 8 // Maximum visible items
	start := 0
	end := len(m.filteredModels)

	// Calculate visible range (scrolling if needed)
	if len(m.filteredModels) > maxVisible {
		start = m.selectedModel - maxVisible/2
		if start < 0 {
			start = 0
		}
		end = start + maxVisible
		if end > len(m.filteredModels) {
			end = len(m.filteredModels)
			start = end - maxVisible
			if start < 0 {
				start = 0
			}
		}
	}

	if len(m.filteredModels) == 0 {
		modelItems = append(modelItems, lipgloss.NewStyle().Foreground(Theme.TextSubtle).Render("  No models found"))
	} else {
		for i := start; i < end; i++ {
			model := m.filteredModels[i]
			if i == m.selectedModel {
				prefix := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render("▶")
				modelLine := prefix + " " + lipgloss.NewStyle().Foreground(Theme.Text).Bold(true).Render(model)
				modelItems = append(modelItems, modelLine)
			} else {
				modelLine := "  " + lipgloss.NewStyle().Foreground(Theme.TextMuted).Render(model)
				modelItems = append(modelItems, modelLine)
			}
		}
	}

	modelList := strings.Join(modelItems, "\n")

	help := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("Type to search • ↑/↓ to select • Enter to confirm • ESC to go back")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		title,
		providerLabel,
		"",
		searchBox,
		countText,
		"",
		modelList,
		"",
		"",
		help,
	)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m OnboardingModel) renderComplete() string {
	checkmark := lipgloss.NewStyle().
		Foreground(Theme.Success).
		Bold(true).
		Render("✓ Configuration Complete!")

	message := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("You can now start using Octrafic")

	pressKey := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Render("Press any key to continue...")

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		checkmark,
		"",
		message,
		"",
		"",
		pressKey,
	)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m OnboardingModel) renderMaskedKey() string {
	// textinput already handles masking with EchoMode
	return m.apiKeyInput.View()
}

func (m OnboardingModel) renderServerURL() string {
	providerDisplay := providerDisplayName(m.provider)

	title := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Render("Enter Server URL")

	providerLabel := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("Provider: ") +
		lipgloss.NewStyle().
			Foreground(Theme.Primary).
			Bold(true).
			Render(providerDisplay)

	input := m.serverURLInput.View()

	var statusLine string
	if m.isTestingKey {
		spinner := lipgloss.NewStyle().Foreground(Theme.Primary).Render("⠋")
		statusLine = spinner + " " + lipgloss.NewStyle().Foreground(Theme.TextMuted).Render("Testing connection...")
	} else if m.errorMsg != "" {
		statusLine = lipgloss.NewStyle().Foreground(Theme.Error).Render("✗ " + m.errorMsg)
	}

	help := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("Enter to test connection • ESC to go back")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		title,
		"",
		providerLabel,
		"",
		"",
		input,
	)

	if statusLine != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", statusLine)
	}

	content = lipgloss.JoinVertical(lipgloss.Left, content, "", "", help)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

// fetchLocalModels fetches available models from a local OpenAI-compatible server (Ollama/llama.cpp)
func fetchLocalModels(serverURL string) ([]string, error) {
	url := strings.TrimSuffix(serverURL, "/") + "/v1/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not connect to server at %s: %w", serverURL, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("server returned status %d: %s", res.StatusCode, string(body))
	}

	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var modelIDs []string
	for _, model := range response.Data {
		modelIDs = append(modelIDs, model.ID)
	}

	if len(modelIDs) == 0 {
		return nil, fmt.Errorf("no models found on server - make sure a model is loaded")
	}

	return modelIDs, nil
}

// fetchAnthropicModels fetches available models from Anthropic API
func fetchAnthropicModels(apiKey string) ([]string, error) {
	// Validate that this looks like an Anthropic key
	if !strings.HasPrefix(apiKey, "sk-ant-") {
		return nil, fmt.Errorf("API key doesn't look like an Anthropic key (should start with 'sk-ant-')")
	}

	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	// Try to fetch models - this will validate the API key
	page, err := client.Models.List(context.TODO(), anthropic.ModelListParams{})
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	var models []string
	for _, model := range page.Data {
		modelID := string(model.ID)

		// Anthropic models should NOT have provider prefixes like "anthropic/"
		if strings.Contains(modelID, "/") {
			return nil, fmt.Errorf("unexpected model format '%s' - looks like OpenRouter data, check your API key", modelID)
		}

		models = append(models, modelID)
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("no models returned from Anthropic API")
	}

	// Anthropic typically has <20 models, not hundreds
	if len(models) > 50 {
		return nil, fmt.Errorf("got %d models from Anthropic API - expected ~10-20, this looks wrong", len(models))
	}

	return models, nil
}

// OpenRouterModel represents a model from OpenRouter API
type OpenRouterModel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ContextLength int    `json:"context_length"`
	Pricing       struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
}

// fetchOpenRouterModels fetches available models from OpenRouter API
func fetchOpenRouterModels(apiKey string) ([]string, error) {
	url := "https://openrouter.ai/api/v1/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+apiKey)
	req.Header.Add("HTTP-Referer", "https://octrafic.com")
	req.Header.Add("X-Title", "Octrafic")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("API returned status %d: %s", res.StatusCode, string(body))
	}

	var response struct {
		Data []OpenRouterModel `json:"data"`
	}

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Sort models by popularity heuristic:
	// 1. Popular providers first (anthropic, openai, google, meta)
	// 2. Then by context length (longer = more capable)
	// 3. Then alphabetically
	models := response.Data

	// Define popular prefixes for priority sorting
	popularPrefixes := []string{
		"anthropic/",
		"openai/",
		"google/",
		"meta-llama/",
		"mistralai/",
		"x-ai/",
	}

	// Helper function to get priority score
	getPriority := func(id string) int {
		for i, prefix := range popularPrefixes {
			if strings.HasPrefix(id, prefix) {
				return i
			}
		}
		return len(popularPrefixes) // Lower priority for others
	}

	// Sort: priority first, then context length descending, then alphabetically
	for i := 0; i < len(models)-1; i++ {
		for j := i + 1; j < len(models); j++ {
			iPriority := getPriority(models[i].ID)
			jPriority := getPriority(models[j].ID)

			shouldSwap := false
			if iPriority != jPriority {
				shouldSwap = iPriority > jPriority
			} else if models[i].ContextLength != models[j].ContextLength {
				shouldSwap = models[i].ContextLength < models[j].ContextLength
			} else {
				shouldSwap = models[i].ID > models[j].ID
			}

			if shouldSwap {
				models[i], models[j] = models[j], models[i]
			}
		}
	}

	var modelIDs []string
	for _, model := range models {
		modelIDs = append(modelIDs, model.ID)
	}

	return modelIDs, nil
}

// fetchOpenAIModels fetches available models from OpenAI API
func fetchOpenAIModels(apiKey string) ([]string, error) {
	url := "https://api.openai.com/v1/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+apiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("API returned status %d: %s", res.StatusCode, string(body))
	}

	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var modelIDs []string
	for _, model := range response.Data {
		// Filter for chat models
		if strings.HasPrefix(model.ID, "gpt-") || strings.HasPrefix(model.ID, "o1-") || strings.HasPrefix(model.ID, "o3-") {
			modelIDs = append(modelIDs, model.ID)
		}
	}

	if len(modelIDs) == 0 {
		return nil, fmt.Errorf("no chat models found in OpenAI account")
	}

	return modelIDs, nil
}
