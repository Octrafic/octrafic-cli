package cli

import (
	"encoding/json"
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/infra/storage"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FormatInfo contains detected format information
type FormatInfo struct {
	Name            string // Human readable name
	NativeSupport   bool   // Can be parsed without conversion
	NeedsConversion bool   // Needs LLM conversion
	Version         string // Format version if detected
}

// detectSpecFormat analyzes file content to determine the API specification format
func detectSpecFormat(filePath string) (*FormatInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	text := string(content)
	textLower := strings.ToLower(text)

	// Check by content first (more reliable than extension)

	// Try JSON parsing for structured formats
	if ext == ".json" || strings.HasPrefix(strings.TrimSpace(text), "{") {
		var data map[string]interface{}
		if err := json.Unmarshal(content, &data); err == nil {
			return detectJSONFormat(data), nil
		}
	}

	// YAML/JSON with openapi or swagger key
	if strings.Contains(textLower, "openapi:") || strings.Contains(textLower, `"openapi"`) {
		version := extractVersion(text, "openapi")
		return &FormatInfo{Name: "OpenAPI", Version: version, NativeSupport: true}, nil
	}
	if strings.Contains(textLower, "swagger:") || strings.Contains(textLower, `"swagger"`) {
		version := extractVersion(text, "swagger")
		return &FormatInfo{Name: "Swagger", Version: version, NativeSupport: true}, nil
	}

	// AsyncAPI
	if strings.Contains(textLower, "asyncapi:") || strings.Contains(textLower, `"asyncapi"`) {
		return &FormatInfo{Name: "AsyncAPI", NeedsConversion: true}, nil
	}

	// RAML
	if strings.HasPrefix(strings.TrimSpace(text), "#%RAML") {
		return &FormatInfo{Name: "RAML", NeedsConversion: true}, nil
	}

	// API Blueprint
	if strings.HasPrefix(strings.TrimSpace(text), "FORMAT:") ||
		(ext == ".apib") ||
		(strings.Contains(text, "# Group") && strings.Contains(text, "## ")) {
		return &FormatInfo{Name: "API Blueprint", NeedsConversion: true}, nil
	}

	// GraphQL (native support)
	if ext == ".graphql" || ext == ".gql" ||
		strings.Contains(text, "type Query") ||
		strings.Contains(text, "type Mutation") {
		return &FormatInfo{Name: "GraphQL Schema", NativeSupport: true}, nil
	}

	// Protocol Buffers
	if ext == ".proto" || strings.Contains(text, "syntax = \"proto") {
		return &FormatInfo{Name: "Protocol Buffers", NeedsConversion: true}, nil
	}

	// WSDL
	if ext == ".wsdl" || strings.Contains(textLower, "<wsdl:") || strings.Contains(textLower, "<definitions") {
		return &FormatInfo{Name: "WSDL (SOAP)", NeedsConversion: true}, nil
	}

	// HTTP/REST files
	if ext == ".http" || ext == ".rest" || ext == ".hurl" {
		return &FormatInfo{Name: "HTTP File", NeedsConversion: true}, nil
	}

	// Markdown - check if it has structured API format or is just description
	if ext == ".md" || ext == ".markdown" {
		if hasStructuredAPIFormat(text) {
			return &FormatInfo{Name: "Markdown API Docs", NativeSupport: true}, nil
		}
		return &FormatInfo{Name: "API Description (Markdown)", NeedsConversion: true}, nil
	}

	// Plain text - API description
	if ext == ".txt" {
		return &FormatInfo{Name: "API Description (Text)", NeedsConversion: true}, nil
	}

	// Fallback by extension for YAML/JSON
	if ext == ".yaml" || ext == ".yml" || ext == ".json" {
		return &FormatInfo{Name: "Unknown JSON/YAML", NeedsConversion: true}, nil
	}

	// Any other file - treat as API description
	return &FormatInfo{Name: "API Description", NeedsConversion: true}, nil
}

// hasStructuredAPIFormat checks if markdown has structured API endpoint format
// like "## GET /users" or "### POST /api/items"
func hasStructuredAPIFormat(content string) bool {
	lines := strings.Split(content, "\n")
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Check for markdown headers with HTTP methods
		if strings.HasPrefix(line, "#") {
			for _, method := range methods {
				if strings.Contains(line, method+" /") || strings.Contains(line, method+" `") {
					return true
				}
			}
		}
	}
	return false
}

// detectJSONFormat detects format from parsed JSON
func detectJSONFormat(data map[string]interface{}) *FormatInfo {
	// OpenAPI 3.x
	if v, ok := data["openapi"].(string); ok {
		return &FormatInfo{Name: "OpenAPI", Version: v, NativeSupport: true}
	}

	// Swagger 2.0
	if v, ok := data["swagger"].(string); ok {
		return &FormatInfo{Name: "Swagger", Version: v, NativeSupport: true}
	}

	// AsyncAPI
	if _, ok := data["asyncapi"]; ok {
		return &FormatInfo{Name: "AsyncAPI", NeedsConversion: true}
	}

	// Postman Collection (native support)
	if info, ok := data["info"].(map[string]interface{}); ok {
		if _, hasPostmanId := info["_postman_id"]; hasPostmanId {
			return &FormatInfo{Name: "Postman Collection", NativeSupport: true}
		}
		if schema, ok := info["schema"].(string); ok && strings.Contains(schema, "postman") {
			return &FormatInfo{Name: "Postman Collection", NativeSupport: true}
		}
	}

	// Insomnia Export
	if t, ok := data["_type"].(string); ok && t == "export" {
		if _, hasResources := data["resources"]; hasResources {
			return &FormatInfo{Name: "Insomnia Export", NeedsConversion: true}
		}
	}

	// HAR
	if log, ok := data["log"].(map[string]interface{}); ok {
		if _, hasEntries := log["entries"]; hasEntries {
			return &FormatInfo{Name: "HAR (HTTP Archive)", NeedsConversion: true}
		}
	}

	// Unknown JSON
	return &FormatInfo{Name: "Unknown JSON", NeedsConversion: true}
}

// extractVersion extracts version string from content
func extractVersion(content, key string) string {
	// Simple search for version number after the key
	idx := strings.Index(strings.ToLower(content), key)
	if idx == -1 {
		return ""
	}

	rest := content[idx+len(key):]
	// Skip whitespace, :, and quotes to find version number
	for i, c := range rest {
		if c >= '0' && c <= '9' {
			end := i
			for end < len(rest) && (rest[end] == '.' || (rest[end] >= '0' && rest[end] <= '9')) {
				end++
			}
			if end > i {
				return rest[i:end]
			}
		}
		if i > 15 {
			break
		}
	}
	return ""
}

// ProjectCreatorStep represents the current step in project creation
type ProjectCreatorStep int

const (
	ProjectStepURL            ProjectCreatorStep = iota // API URL input
	ProjectStepSpecPath                                 // Specification file path input
	ProjectStepFormatDetected                           // Show detected format
	ProjectStepName                                     // Project name input
	ProjectStepAuthPrompt                               // Ask if user wants to configure auth
	ProjectStepAuthType                                 // Select auth type
	ProjectStepAuthDetails                              // Enter auth details
	ProjectStepConfirm                                  // Confirming creation
)

// ProjectCreatorModel represents the project creation wizard
type ProjectCreatorModel struct {
	step            ProjectCreatorStep
	input           textinput.Model
	validationError string
	confirmed       bool
	cancelled       bool
	width           int
	height          int

	// Result fields
	url      string
	specPath string
	name     string

	// Format detection
	formatInfo *FormatInfo

	// Auth configuration
	configureAuth    bool
	authType         string   // "bearer", "apikey", "basic", "none"
	authMenuItems    []string // Menu options for auth type selection
	authMenuIndex    int      // Selected menu item index
	authFields       []FormField
	authFocusedField int
}

// NewProjectCreatorModel creates a new project creation wizard
func NewProjectCreatorModel() ProjectCreatorModel {
	ti := textinput.New()
	ti.Placeholder = "https://api.example.com"
	ti.CharLimit = 200
	ti.Width = 50
	ti.Focus()

	return ProjectCreatorModel{
		step:   ProjectStepURL,
		input:  ti,
		width:  80,
		height: 24,
	}
}

func (m ProjectCreatorModel) Init() tea.Cmd {
	return nil
}

func (m ProjectCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Handle special keys first
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit

		case "esc":
			// Go back to previous step or cancel
			switch m.step {
			case ProjectStepURL:
				m.cancelled = true
				return m, tea.Quit
			case ProjectStepSpecPath:
				m.step = ProjectStepURL
				m.input.SetValue(m.url)
				m.input.Placeholder = "https://api.example.com"
				m.validationError = ""
				m.input.Focus()
			case ProjectStepFormatDetected:
				m.step = ProjectStepSpecPath
				m.input.SetValue(m.specPath)
				m.input.Placeholder = "./spec.yaml, ./api.raml, ./schema.graphql..."
				m.validationError = ""
				m.formatInfo = nil
				m.input.Focus()
			case ProjectStepName:
				m.step = ProjectStepFormatDetected
				m.validationError = ""
			case ProjectStepConfirm:
				m.step = ProjectStepName
				m.input.SetValue(m.name)
				m.input.Placeholder = "my-api-project"
				m.validationError = ""
				m.input.Focus()
			}
			return m, nil

		case "enter":
			switch m.step {
			case ProjectStepURL:
				m.url = strings.TrimSpace(m.input.Value())
				if m.url == "" {
					m.validationError = "API URL is required"
					return m, nil
				}
				m.step = ProjectStepSpecPath
				m.input.SetValue("")
				m.input.Placeholder = "./spec.yaml, ./api.raml, ./schema.graphql..."
				m.validationError = ""
				m.input.Focus()

			case ProjectStepSpecPath:
				m.specPath = strings.TrimSpace(m.input.Value())
				if m.specPath == "" {
					m.validationError = "Specification file path is required"
					return m, nil
				}
				// Validate spec path exists
				if err := storage.ValidateSpecPath(m.specPath); err != nil {
					m.validationError = fmt.Sprintf("Invalid specification file: %s", err.Error())
					return m, nil
				}
				// Detect format
				formatInfo, err := detectSpecFormat(m.specPath)
				if err != nil {
					m.validationError = fmt.Sprintf("Failed to analyze file: %s", err.Error())
					return m, nil
				}
				m.formatInfo = formatInfo
				m.step = ProjectStepFormatDetected
				m.input.Blur()
				return m, nil

			case ProjectStepFormatDetected:
				// User confirmed format, continue to name
				m.step = ProjectStepName
				m.input.SetValue("")
				m.input.Placeholder = "my-api-project"
				m.validationError = ""
				m.input.Focus()

			case ProjectStepName:
				m.name = strings.TrimSpace(m.input.Value())
				if m.name == "" {
					m.validationError = "Project name is required"
					return m, nil
				}
				// Move to auth prompt step
				m.step = ProjectStepAuthPrompt
				m.input.Blur()
				m.validationError = ""

			case ProjectStepAuthPrompt:
				// Handle y/n for auth configuration
				return m, nil

			case ProjectStepAuthType:
				// User selected auth type from menu
				selectedItem := m.authMenuItems[m.authMenuIndex]
				switch selectedItem {
				case "Bearer Token":
					m.authType = "bearer"
				case "API Key":
					m.authType = "apikey"
				case "Basic Auth":
					m.authType = "basic"
				case "None":
					m.authType = "none"
				}

				if m.authType == "none" {
					// Skip auth details, go straight to confirm
					m.step = ProjectStepConfirm
					return m, nil
				}

				// Move to details step
				m.authFields = CreateAuthFormFields(m.authType)
				// Remove profile_name field - not needed for project creation
				for i, field := range m.authFields {
					if field.Name == "profile_name" {
						m.authFields = append(m.authFields[:i], m.authFields[i+1:]...)
						break
					}
				}
				m.authFocusedField = 0
				m.step = ProjectStepAuthDetails

			case ProjectStepAuthDetails:
				// Submit auth details - validate and continue
				if !validateAuthFields(m.authType, m.authFields) {
					m.validationError = "Please fill all required fields"
					return m, nil
				}
				m.step = ProjectStepConfirm

			case ProjectStepConfirm:
				m.confirmed = true
				return m, tea.Quit
			}
			return m, nil

		// Handle auth prompt yes/no
		case "y", "Y":
			if m.step == ProjectStepAuthPrompt {
				m.configureAuth = true
				m.authMenuItems = []string{"Bearer Token", "API Key", "Basic Auth", "None"}
				m.authMenuIndex = 0
				m.step = ProjectStepAuthType
				return m, nil
			}

		case "n", "N":
			if m.step == ProjectStepAuthPrompt {
				m.configureAuth = false
				m.authType = "none"
				m.step = ProjectStepConfirm
				return m, nil
			}

		// Handle auth menu navigation
		case "up":
			switch m.step {
			case ProjectStepAuthType:
				m.authMenuIndex--
				if m.authMenuIndex < 0 {
					m.authMenuIndex = len(m.authMenuItems) - 1
				}
				return m, nil
			case ProjectStepAuthDetails:
				m.authFocusedField--
				if m.authFocusedField < 0 {
					m.authFocusedField = len(m.authFields) - 1
				}
				return m, nil
			}

		case "down":
			switch m.step {
			case ProjectStepAuthType:
				m.authMenuIndex++
				if m.authMenuIndex >= len(m.authMenuItems) {
					m.authMenuIndex = 0
				}
				return m, nil
			case ProjectStepAuthDetails:
				m.authFocusedField++
				if m.authFocusedField >= len(m.authFields) {
					m.authFocusedField = 0
				}
				return m, nil
			}

		case "tab":
			if m.step == ProjectStepAuthDetails {
				m.authFocusedField++
				if m.authFocusedField >= len(m.authFields) {
					m.authFocusedField = 0
				}
				return m, nil
			}

		case "left", "right":
			if m.step == ProjectStepAuthDetails {
				// Handle radio button navigation
				field := &m.authFields[m.authFocusedField]
				if field.IsRadio && len(field.Options) > 0 {
					if msg.String() == "left" {
						field.RadioIndex--
						if field.RadioIndex < 0 {
							field.RadioIndex = len(field.Options) - 1
						}
					} else {
						field.RadioIndex++
						if field.RadioIndex >= len(field.Options) {
							field.RadioIndex = 0
						}
					}
				}
				return m, nil
			}

		case "backspace":
			if m.step == ProjectStepAuthDetails {
				field := &m.authFields[m.authFocusedField]
				if !field.IsRadio && len(field.Value) > 0 {
					field.Value = field.Value[:len(field.Value)-1]
				}
				return m, nil
			}

		// Handle runes (typing) for auth details
		default:
			if m.step == ProjectStepAuthDetails {
				if len(msg.Runes) > 0 {
					field := &m.authFields[m.authFocusedField]
					if !field.IsRadio {
						field.Value += string(msg.Runes)
					}
					return m, nil
				}
			}
		}

		// Let textinput handle other keys (typing, backspace, etc.)
		if m.step != ProjectStepConfirm && m.step != ProjectStepFormatDetected &&
			m.step != ProjectStepAuthPrompt && m.step != ProjectStepAuthType && m.step != ProjectStepAuthDetails {
			m.input, cmd = m.input.Update(msg)
			m.validationError = ""
			return m, cmd
		}
	}

	return m, nil
}

func (m ProjectCreatorModel) View() string {
	switch m.step {
	case ProjectStepURL:
		return m.renderURLStep()
	case ProjectStepSpecPath:
		return m.renderSpecPathStep()
	case ProjectStepFormatDetected:
		return m.renderFormatDetected()
	case ProjectStepName:
		return m.renderNameStep()
	case ProjectStepAuthPrompt:
		return m.renderAuthPrompt()
	case ProjectStepAuthType:
		return m.renderAuthTypeMenu()
	case ProjectStepAuthDetails:
		return m.renderAuthDetailsForm()
	case ProjectStepConfirm:
		return m.renderConfirmation()
	default:
		return ""
	}
}

func (m ProjectCreatorModel) renderURLStep() string {
	title := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Render("Create New Project")

	subtitle := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("Enter the API base URL")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		title,
		"",
		subtitle,
		"",
		"",
		m.input.View(),
	)

	// Validation error
	if m.validationError != "" {
		errorLine := lipgloss.NewStyle().Foreground(Theme.Error).Render(m.validationError)
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", errorLine)
	}

	help := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("Enter to continue • Esc to cancel")

	content = lipgloss.JoinVertical(lipgloss.Left, content, "", "", help)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m ProjectCreatorModel) renderSpecPathStep() string {
	title := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Render("Specification File")

	subtitle := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("OpenAPI, Swagger, RAML, GraphQL, Protobuf, HAR, HTTP files...")

	hint := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("Non-OpenAPI formats will be converted using LLM")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		title,
		"",
		subtitle,
		hint,
		"",
		"",
		m.input.View(),
	)

	// Validation error
	if m.validationError != "" {
		errorLine := lipgloss.NewStyle().Foreground(Theme.Error).Render(m.validationError)
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", errorLine)
	}

	help := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("Enter to continue • Esc to go back")

	content = lipgloss.JoinVertical(lipgloss.Left, content, "", "", help)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m ProjectCreatorModel) renderNameStep() string {
	title := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Render("Project Name")

	subtitle := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("Choose a name for this project")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		title,
		"",
		subtitle,
		"",
		"",
		m.input.View(),
	)

	// Validation error
	if m.validationError != "" {
		errorLine := lipgloss.NewStyle().Foreground(Theme.Error).Render(m.validationError)
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", errorLine)
	}

	help := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("Enter to continue • Esc to go back")

	content = lipgloss.JoinVertical(lipgloss.Left, content, "", "", help)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m ProjectCreatorModel) renderFormatDetected() string {
	var title string
	var formatName string
	var statusText string
	var statusColor lipgloss.Color

	if m.formatInfo != nil {
		formatName = m.formatInfo.Name
		if m.formatInfo.Version != "" {
			formatName += " " + m.formatInfo.Version
		}

		if m.formatInfo.NativeSupport {
			title = "Format Detected"
			statusText = "Native support - no conversion needed"
			statusColor = Theme.Success
		} else {
			title = "Format Detected - Conversion Required"
			statusText = "Will be converted to OpenAPI using LLM"
			statusColor = Theme.Warning
		}
	} else {
		formatName = "Unknown"
		title = "Format Detection"
		statusText = "Could not detect format"
		statusColor = Theme.Error
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true).
		Render(title)

	formatLabel := lipgloss.NewStyle().
		Foreground(Theme.TextMuted).
		Render("Detected: ") +
		lipgloss.NewStyle().
			Foreground(Theme.Primary).
			Bold(true).
			Render(formatName)

	statusLine := lipgloss.NewStyle().
		Foreground(statusColor).
		Render(statusText)

	var warning string
	if m.formatInfo != nil && m.formatInfo.NeedsConversion {
		warning = lipgloss.NewStyle().
			Foreground(Theme.TextMuted).
			Render("Conversion may take a moment and requires API credits.")
	}

	help := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("Enter to continue • Esc to go back")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		titleStyle,
		"",
		formatLabel,
		"",
		statusLine,
	)

	if warning != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", warning)
	}

	content = lipgloss.JoinVertical(lipgloss.Left, content, "", "", help)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m ProjectCreatorModel) renderConfirmation() string {
	title := lipgloss.NewStyle().
		Foreground(Theme.Success).
		Bold(true).
		Render("Confirm Project Creation")

	labelStyle := lipgloss.NewStyle().Foreground(Theme.TextMuted)
	valueStyle := lipgloss.NewStyle().Foreground(Theme.Text)

	urlLine := labelStyle.Render("API URL: ") + valueStyle.Render(m.url)
	specLine := labelStyle.Render("Specification: ") + valueStyle.Render(m.specPath)
	nameLine := labelStyle.Render("Project Name: ") + valueStyle.Render(m.name)

	var conversionLine string
	if m.formatInfo != nil && m.formatInfo.NeedsConversion {
		conversionLine = labelStyle.Render("Conversion: ") +
			lipgloss.NewStyle().Foreground(Theme.Warning).Render(m.formatInfo.Name+" → OpenAPI (LLM)")
	}

	help := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Render("Enter to confirm • Esc to go back")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		title,
		"",
		"",
		urlLine,
		"",
		specLine,
	)

	if m.formatInfo != nil && m.formatInfo.NeedsConversion {
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", conversionLine)
	}

	content = lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		"",
		nameLine,
		"",
		"",
		help,
	)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

// IsCancelled returns true if the user cancelled the wizard
func (m ProjectCreatorModel) IsCancelled() bool {
	return m.cancelled
}

// IsConfirmed returns true if the user confirmed project creation
func (m ProjectCreatorModel) IsConfirmed() bool {
	return m.confirmed
}

// GetProjectData returns the project data entered by the user
func (m ProjectCreatorModel) GetProjectData() (url, specPath, name string) {
	return m.url, m.specPath, m.name
}

// NeedsConversion returns true if the spec file needs LLM conversion
func (m ProjectCreatorModel) NeedsConversion() bool {
	return m.formatInfo != nil && m.formatInfo.NeedsConversion
}

// GetDetectedFormat returns the detected format name for conversion
func (m ProjectCreatorModel) GetDetectedFormat() string {
	if m.formatInfo != nil {
		return m.formatInfo.Name
	}
	return ""
}

// GetAuthConfig returns the auth configuration (type and fields map)
func (m ProjectCreatorModel) GetAuthConfig() (authType string, authData map[string]string) {
	if !m.configureAuth || m.authType == "none" || m.authType == "" {
		return "none", nil
	}

	authData = make(map[string]string)
	for _, field := range m.authFields {
		if field.IsRadio {
			if field.RadioIndex < len(field.Options) {
				authData[field.Name] = field.Options[field.RadioIndex]
			}
		} else {
			authData[field.Name] = field.Value
		}
	}

	return m.authType, authData
}

// validateAuthFields checks if all required auth fields are filled
func validateAuthFields(authType string, fields []FormField) bool {
	for _, field := range fields {
		if field.Name == "profile_name" {
			continue // Optional
		}
		if !field.IsRadio && strings.TrimSpace(field.Value) == "" {
			return false
		}
	}
	return true
}

// renderAuthPrompt renders the yes/no prompt for auth configuration
func (m ProjectCreatorModel) renderAuthPrompt() string {
	title := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Render("Configure Authentication?")

	question := lipgloss.NewStyle().
		Foreground(Theme.Text).
		Render("\nDo you want to configure authentication for API requests?")

	hint := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Italic(true).
		Render("\n\nAuth will be saved automatically with the project.\n[Y/n] (Esc to go back)")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		title,
		question,
		hint,
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Theme.Primary).
		Padding(1, 2).
		Width(60).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderAuthTypeMenu renders the auth type selection menu
func (m ProjectCreatorModel) renderAuthTypeMenu() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true).
		Render("Select Authentication Type")

	b.WriteString(title)
	b.WriteString("\n\n")

	// Menu items
	for i, item := range m.authMenuItems {
		if i == m.authMenuIndex {
			// Selected item
			selectedStyle := lipgloss.NewStyle().
				Foreground(Theme.Primary).
				Bold(true)
			b.WriteString(selectedStyle.Render("> " + item))
		} else {
			// Unselected item
			subtleStyle := lipgloss.NewStyle().
				Foreground(Theme.TextMuted)
			b.WriteString(subtleStyle.Render("  " + item))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Italic(true)

	b.WriteString(helpStyle.Render("[↑↓ to select, Enter to confirm, Esc to go back]"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Theme.Primary).
		Padding(1, 2).
		Width(60).
		Render(b.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderAuthDetailsForm renders the auth configuration form
func (m ProjectCreatorModel) renderAuthDetailsForm() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true)

	var title string
	switch m.authType {
	case "bearer":
		title = "Bearer Token Authentication"
	case "apikey":
		title = "API Key Authentication"
	case "basic":
		title = "Basic Authentication"
	}

	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Render form fields
	for i, field := range m.authFields {
		isFocused := i == m.authFocusedField

		// Label
		labelStyle := lipgloss.NewStyle().Foreground(Theme.Text)
		if isFocused {
			labelStyle = labelStyle.Bold(true).Foreground(Theme.Primary)
		}
		b.WriteString(labelStyle.Render(field.Label))
		b.WriteString("\n")

		if field.IsRadio {
			// Radio buttons
			radioStyle := lipgloss.NewStyle().Foreground(Theme.TextMuted)
			radioFocusedStyle := lipgloss.NewStyle().Foreground(Theme.Primary)

			for j, option := range field.Options {
				var marker string
				if j == field.RadioIndex {
					marker = "(•)"
				} else {
					marker = "( )"
				}

				style := radioStyle
				if isFocused {
					style = radioFocusedStyle
				}

				b.WriteString(style.Render(fmt.Sprintf("  %s %s", marker, option)))
				b.WriteString("\n")
			}
		} else {
			// Text input
			inputStyle := lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(Theme.BorderSubtle).
				Padding(0, 1).
				Width(50)

			if isFocused {
				inputStyle = inputStyle.BorderForeground(Theme.Primary)
			}

			// Display value or placeholder
			displayValue := field.Value
			if displayValue == "" && field.Placeholder != "" {
				placeholderStyle := lipgloss.NewStyle().Foreground(Theme.TextSubtle)
				displayValue = placeholderStyle.Render(field.Placeholder)
			} else if field.IsPassword && len(displayValue) > 0 {
				// Hide password
				displayValue = strings.Repeat("•", len(displayValue))
			}

			// Add cursor if focused
			if isFocused {
				displayValue += lipgloss.NewStyle().Foreground(Theme.Primary).Render("│")
			}

			b.WriteString(inputStyle.Render(displayValue))
		}

		b.WriteString("\n\n")
	}

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(Theme.TextSubtle).
		Italic(true)

	b.WriteString(helpStyle.Render("[↑↓/Tab to switch fields, ←→ for radio buttons]"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("[Enter to submit, Esc to go back]"))

	// Validation error if any
	if m.validationError != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(Theme.Error).
			Bold(true)
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render("❌ " + m.validationError))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Theme.Primary).
		Padding(1, 2).
		Width(65).
		Render(b.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
