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

type ModelSelector struct {
	models           []string
	filteredModels   []string
	selectedModel    int
	modelSearchInput textinput.Model
	isLoading        bool
	errorMsg         string
	provider         string
	width            int
	height           int
}

type ModelsFetchedMsg struct {
	Models   []string
	Error    string
	Provider string
}

type OpenRouterModel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ContextLength int    `json:"context_length"`
	Pricing       struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
}

func NewModelSelector() *ModelSelector {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search models..."
	searchInput.CharLimit = 100
	searchInput.Width = 50
	searchInput.Focus()

	return &ModelSelector{
		models:           []string{},
		filteredModels:   []string{},
		modelSearchInput: searchInput,
		width:            80,
		height:           24,
	}
}

func (m *ModelSelector) SetModels(models []string) {
	m.models = models
	m.filteredModels = models
	m.isLoading = false
	if len(models) > 0 {
		m.selectedModel = 0
	}
}

func (m *ModelSelector) SetError(err string) {
	m.errorMsg = err
	m.isLoading = false
}

func (m *ModelSelector) SetProvider(provider string) {
	m.provider = provider
}

func (m *ModelSelector) GetSelectedModel() string {
	if m.selectedModel < len(m.filteredModels) {
		return m.filteredModels[m.selectedModel]
	}
	return ""
}

func (m *ModelSelector) filterModels() {
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
	}
}

func FetchModelsForProvider(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		if cfg.Provider == "" {
			return ModelsFetchedMsg{Error: "No provider configured"}
		}

		var models []string
		var err error

		switch cfg.Provider {
		case "anthropic":
			if cfg.APIKey == "" {
				return ModelsFetchedMsg{Error: "No API key configured for Anthropic", Provider: cfg.Provider}
			}
			models, err = fetchAnthropicModelsList(cfg.APIKey)
		case "openrouter":
			if cfg.APIKey == "" {
				return ModelsFetchedMsg{Error: "No API key configured for OpenRouter", Provider: cfg.Provider}
			}
			models, err = fetchOpenRouterModelsList(cfg.APIKey)
		case "openai":
			if cfg.APIKey == "" {
				return ModelsFetchedMsg{Error: "No API key configured for OpenAI", Provider: cfg.Provider}
			}
			models, err = fetchOpenAIModelsList(cfg.APIKey)
		case "ollama", "llamacpp":
			baseURL := cfg.BaseURL
			if baseURL == "" {
				if cfg.Provider == "ollama" {
					baseURL = "http://localhost:11434"
				} else {
					baseURL = "http://localhost:8080"
				}
			}
			models, err = fetchLocalModelsList(baseURL)
		case "custom":
			if cfg.BaseURL == "" {
				return ModelsFetchedMsg{Error: "No base URL configured for custom provider", Provider: cfg.Provider}
			}
			models, err = fetchCustomModelsList(cfg.BaseURL, cfg.APIKey)
		default:
			return ModelsFetchedMsg{Error: "Unknown provider: " + cfg.Provider, Provider: cfg.Provider}
		}

		if err != nil {
			return ModelsFetchedMsg{Error: err.Error(), Provider: cfg.Provider}
		}

		return ModelsFetchedMsg{Models: models, Provider: cfg.Provider}
	}
}

func fetchAnthropicModelsList(apiKey string) ([]string, error) {
	if !strings.HasPrefix(apiKey, "sk-ant-") {
		return nil, fmt.Errorf("API key doesn't look like an Anthropic key")
	}

	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	page, err := client.Models.List(context.TODO(), anthropic.ModelListParams{})
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	var models []string
	for _, model := range page.Data {
		modelID := string(model.ID)
		if !strings.Contains(modelID, "/") {
			models = append(models, modelID)
		}
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("no models returned from Anthropic API")
	}

	return models, nil
}

func fetchOpenRouterModelsList(apiKey string) ([]string, error) {
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

	popularPrefixes := []string{
		"anthropic/",
		"openai/",
		"google/",
		"meta-llama/",
		"mistralai/",
		"x-ai/",
	}

	getPriority := func(id string) int {
		for i, prefix := range popularPrefixes {
			if strings.HasPrefix(id, prefix) {
				return i
			}
		}
		return len(popularPrefixes)
	}

	models := response.Data
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

func fetchOpenAIModelsList(apiKey string) ([]string, error) {
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
		if strings.HasPrefix(model.ID, "gpt-") || strings.HasPrefix(model.ID, "o1-") || strings.HasPrefix(model.ID, "o3-") {
			modelIDs = append(modelIDs, model.ID)
		}
	}

	if len(modelIDs) == 0 {
		return nil, fmt.Errorf("no chat models found")
	}

	return modelIDs, nil
}

func fetchCustomModelsList(baseURL, apiKey string) ([]string, error) {
	url := strings.TrimSuffix(baseURL, "/") + "/v1/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if apiKey != "" {
		req.Header.Add("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not connect to server at %s: %w", baseURL, err)
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
		return nil, fmt.Errorf("no models found on server")
	}

	return modelIDs, nil
}

func fetchLocalModelsList(serverURL string) ([]string, error) {
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
		return nil, fmt.Errorf("no models found on server")
	}

	return modelIDs, nil
}

func (m *ModelSelector) HandleKey(msg tea.KeyMsg) (bool, string) {
	switch msg.String() {
	case "up", "k":
		if m.selectedModel > 0 {
			m.selectedModel--
		}
		return false, ""
	case "down", "j":
		if m.selectedModel < len(m.filteredModels)-1 {
			m.selectedModel++
		}
		return false, ""
	case "enter":
		if len(m.filteredModels) > 0 && m.selectedModel < len(m.filteredModels) {
			return true, m.filteredModels[m.selectedModel]
		}
		return false, ""
	case "esc":
		return true, ""
	default:
		m.modelSearchInput, _ = m.modelSearchInput.Update(msg)
		m.filterModels()
		return false, ""
	}
}

func (m *ModelSelector) Render() string {
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Theme.Primary).
		Padding(1, 2).
		Width(60)

	titleStyle := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true)

	var b strings.Builder

	b.WriteString(titleStyle.Render("Select AI Model"))
	b.WriteString("\n\n")

	if m.provider != "" {
		providerLabel := lipgloss.NewStyle().
			Foreground(Theme.TextMuted).
			Render("Provider: ") +
			lipgloss.NewStyle().
				Foreground(Theme.Primary).
				Render(providerDisplayName(m.provider))
		b.WriteString(providerLabel)
		b.WriteString("\n\n")
	}

	if m.isLoading {
		spinner := lipgloss.NewStyle().Foreground(Theme.Primary).Render("⠋")
		b.WriteString(spinner + " " + lipgloss.NewStyle().Foreground(Theme.TextMuted).Render("Loading models..."))
	} else if m.errorMsg != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(Theme.Error).Render("✗ " + m.errorMsg))
	} else {
		searchLabel := lipgloss.NewStyle().Foreground(Theme.TextMuted).Render("Search: ")
		b.WriteString(searchLabel + m.modelSearchInput.View())
		b.WriteString("\n")

		countText := lipgloss.NewStyle().
			Foreground(Theme.TextSubtle).
			Render(fmt.Sprintf("(%d/%d models)", len(m.filteredModels), len(m.models)))
		b.WriteString(countText)
		b.WriteString("\n\n")

		var modelItems []string
		maxVisible := 8
		start := 0
		end := len(m.filteredModels)

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

		b.WriteString(strings.Join(modelItems, "\n"))
	}

	b.WriteString("\n\n")
	helpStyle := lipgloss.NewStyle().Foreground(Theme.TextSubtle)
	b.WriteString(helpStyle.Render("Type to search • ↑/↓ to select • Enter to confirm • ESC to cancel"))

	return borderStyle.Render(b.String())
}
