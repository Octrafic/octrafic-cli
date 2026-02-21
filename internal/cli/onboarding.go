package cli

import (
	"fmt"
	"strings"

	"github.com/Octrafic/octrafic-cli/internal/config"

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
	OnboardingCustomAPIKey
	OnboardingSelectModel
	OnboardingManualModel
	OnboardingComplete
)

// OnboardingModel handles the onboarding flow
type OnboardingModel struct {
	state            OnboardingState
	provider         string
	selectedProvider int
	apiKey           string
	apiKeyInput      textinput.Model
	serverURL        string
	serverURLInput   textinput.Model
	models           []string
	filteredModels   []string // Filtered list based on search
	selectedModel    int
	modelSearchInput textinput.Model
	manualModelInput textinput.Model
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
	ti := textinput.New()
	ti.Placeholder = "sk-ant-..."
	ti.CharLimit = 200
	ti.Width = 50
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'

	serverURLInput := textinput.New()
	serverURLInput.Placeholder = "http://localhost:11434"
	serverURLInput.CharLimit = 200
	serverURLInput.Width = 50

	searchInput := textinput.New()
	searchInput.Placeholder = "Search models..."
	searchInput.CharLimit = 100
	searchInput.Width = 50

	manualInput := textinput.New()
	manualInput.Placeholder = "e.g. llama-3.3-70b-versatile"
	manualInput.CharLimit = 200
	manualInput.Width = 50

	return OnboardingModel{
		state:            OnboardingWelcome,
		width:            80,
		height:           24,
		models:           []string{},
		filteredModels:   []string{},
		apiKeyInput:      ti,
		serverURLInput:   serverURLInput,
		modelSearchInput: searchInput,
		manualModelInput: manualInput,
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
		m.filteredModels = m.models
	} else {
		m.filteredModels = []string{}
		for _, model := range m.models {
			if strings.Contains(strings.ToLower(model), query) {
				m.filteredModels = append(m.filteredModels, model)
			}
		}
	}

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
		if m.state == OnboardingAPIKey {
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
				m.apiKeyInput.SetValue("")
				m.errorMsg = ""
				m.state = OnboardingProvider
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
				return m, cmd
			}
		}

		if m.state == OnboardingServerURL {
			switch msg.String() {
			case "enter":
				url := m.serverURLInput.Value()
				if url == "" {
					if m.provider == "ollama" {
						url = "http://localhost:11434"
					} else if m.provider == "llamacpp" {
						url = "http://localhost:8080"
					}
				}
				m.serverURL = url
				m.errorMsg = ""
				if m.provider == "custom" {
					m.apiKeyInput.SetValue("")
					m.state = OnboardingCustomAPIKey
					m.apiKeyInput.Focus()
					return m, nil
				}
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

		if m.state == OnboardingManualModel {
			switch msg.String() {
			case "enter":
				if len(m.manualModelInput.Value()) > 0 {
					m.models = []string{m.manualModelInput.Value()}
					m.filteredModels = m.models
					m.selectedModel = 0
					m.state = OnboardingSelectModel
					m.modelSearchInput.Focus()
					return m, nil
				}
				return m, nil
			case "esc":
				m.manualModelInput.SetValue("")
				m.errorMsg = ""
				m.state = OnboardingCustomAPIKey
				m.apiKeyInput.Focus()
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				m.manualModelInput, cmd = m.manualModelInput.Update(msg)
				return m, cmd
			}
		}

		if m.state == OnboardingCustomAPIKey {
			switch msg.String() {
			case "enter":
				m.apiKey = m.apiKeyInput.Value()
				m.errorMsg = ""
				m.isTestingKey = true
				return m, m.testCustomConnection()
			case "esc":
				m.apiKeyInput.SetValue("")
				m.errorMsg = ""
				m.state = OnboardingServerURL
				m.serverURLInput.Focus()
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
				return m, cmd
			}
		}

		if m.state == OnboardingSelectModel {
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
				} else if m.provider == "custom" {
					m.state = OnboardingCustomAPIKey
					m.apiKeyInput.Focus()
				} else {
					m.apiKeyInput.SetValue("")
					m.state = OnboardingAPIKey
					m.apiKeyInput.Focus()
				}
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				m.modelSearchInput, cmd = m.modelSearchInput.Update(msg)
				m.filterModels()
				return m, cmd
			}
		}

		return m.handleKeyPress(msg)

	case OnboardingMsg:
		m.state = msg.NextState

	case KeyTestResult:
		m.isTestingKey = false
		if msg.Success {
			m.models = msg.Models
			m.filteredModels = msg.Models
			m.state = OnboardingSelectModel
			m.modelSearchInput.Focus()
			if len(m.models) > 0 {
				m.selectedModel = 0
			}
		} else if m.provider == "custom" {
			m.manualModelInput.SetValue("")
			m.errorMsg = fmt.Sprintf("%s (provider: %s)", msg.Error, msg.Provider)
			m.state = OnboardingManualModel
			m.manualModelInput.Focus()
		} else {
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
			if m.selectedProvider < 5 {
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
			case 5:
				m.provider = "custom"
				m.serverURLInput.SetValue("")
				m.serverURLInput.Placeholder = "https://api.groq.com/openai"
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
	provider := m.provider
	apiKey := m.apiKey

	return func() tea.Msg {
		if provider == "" {
			return KeyTestResult{
				Success:  false,
				Error:    "Provider is empty string - this is a bug!",
				Provider: "(empty)",
			}
		}

		cfg := &config.Config{
			Provider: provider,
			APIKey:   apiKey,
		}

		result := <-func() <-chan tea.Msg {
			ch := make(chan tea.Msg, 1)
			go func() {
				cmd := FetchModelsForProvider(cfg)
				ch <- cmd()
			}()
			return ch
		}()

		if msg, ok := result.(ModelsFetchedMsg); ok {
			if msg.Error != "" {
				return KeyTestResult{
					Success:  false,
					Error:    msg.Error,
					Provider: msg.Provider,
				}
			}
			return KeyTestResult{
				Success:  true,
				Models:   msg.Models,
				Provider: msg.Provider,
			}
		}
		return KeyTestResult{
			Success:  false,
			Error:    "Unexpected response type",
			Provider: provider,
		}
	}
}

func (m *OnboardingModel) testServerConnection() tea.Cmd {
	provider := m.provider
	serverURL := m.serverURL

	return func() tea.Msg {
		cfg := &config.Config{
			Provider: provider,
			BaseURL:  serverURL,
		}

		result := <-func() <-chan tea.Msg {
			ch := make(chan tea.Msg, 1)
			go func() {
				cmd := FetchModelsForProvider(cfg)
				ch <- cmd()
			}()
			return ch
		}()

		if msg, ok := result.(ModelsFetchedMsg); ok {
			if msg.Error != "" {
				return KeyTestResult{
					Success:  false,
					Error:    msg.Error,
					Provider: msg.Provider,
				}
			}
			return KeyTestResult{
				Success:  true,
				Models:   msg.Models,
				Provider: msg.Provider,
			}
		}
		return KeyTestResult{
			Success:  false,
			Error:    "Unexpected response type",
			Provider: provider,
		}
	}
}

func (m *OnboardingModel) testCustomConnection() tea.Cmd {
	provider := m.provider
	serverURL := m.serverURL
	apiKey := m.apiKey

	return func() tea.Msg {
		cfg := &config.Config{
			Provider: provider,
			BaseURL:  serverURL,
			APIKey:   apiKey,
		}

		result := <-func() <-chan tea.Msg {
			ch := make(chan tea.Msg, 1)
			go func() {
				cmd := FetchModelsForProvider(cfg)
				ch <- cmd()
			}()
			return ch
		}()

		if msg, ok := result.(ModelsFetchedMsg); ok {
			if msg.Error != "" {
				return KeyTestResult{
					Success:  false,
					Error:    msg.Error,
					Provider: msg.Provider,
				}
			}
			return KeyTestResult{
				Success:  true,
				Models:   msg.Models,
				Provider: msg.Provider,
			}
		}
		return KeyTestResult{
			Success:  false,
			Error:    "Unexpected response type",
			Provider: provider,
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
	case OnboardingCustomAPIKey:
		return m.renderCustomAPIKey()
	case OnboardingManualModel:
		return m.renderManualModel()
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

	providers := []string{"Anthropic", "OpenRouter", "OpenAI", "Ollama (local)", "llama.cpp (local)", "Custom (OpenAI-compatible)"}
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
	case "custom":
		return "Custom (OpenAI-compatible)"
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

func (m OnboardingModel) renderManualModel() string {
	title := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Render("Enter Model Name")

	urlLabel := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("URL: ") +
		lipgloss.NewStyle().
			Foreground(Theme.Primary).
			Render(m.serverURL)

	var errorLine string
	if m.errorMsg != "" {
		errorLine = lipgloss.NewStyle().Foreground(Theme.Error).Render("Could not fetch models: " + m.errorMsg)
	}

	hint := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("Type the model name manually:")

	help := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("Enter to confirm • ESC to go back")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		title,
		"",
		urlLabel,
		"",
	)

	if errorLine != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, errorLine, "")
	}

	content = lipgloss.JoinVertical(lipgloss.Left, content, "", hint, "", m.manualModelInput.View(), "", "", help)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m OnboardingModel) renderCustomAPIKey() string {
	title := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Render("Enter API Key (optional)")

	urlLabel := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("URL: ") +
		lipgloss.NewStyle().
			Foreground(Theme.Primary).
			Render(m.serverURL)

	input := m.renderMaskedKey()

	var statusLine string
	if m.isTestingKey {
		spinner := lipgloss.NewStyle().Foreground(Theme.Primary).Render("⠋")
		statusLine = spinner + " " + lipgloss.NewStyle().Foreground(Theme.TextMuted).Render("Testing connection...")
	} else if m.errorMsg != "" {
		statusLine = lipgloss.NewStyle().Foreground(Theme.Error).Render("✗ " + m.errorMsg)
	}

	help := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("Enter to connect (empty = no auth) • ESC to go back")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		title,
		"",
		urlLabel,
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
