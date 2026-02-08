package cli

import (
	"github.com/Octrafic/octrafic-cli/internal/core/auth"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// WizardType represents the type of wizard currently active
type WizardType string

const (
	WizardNone    WizardType = ""
	WizardAuth    WizardType = "auth"
	WizardProfile WizardType = "profile"
)

// WizardStep represents the current step in a wizard
type WizardStep int

const (
	StepSelectType WizardStep = iota // Selecting auth type
	StepFillForm                     // Filling out form fields
	StepConfirm                      // Confirming action
)

// WizardState holds the state of an active wizard
type WizardState struct {
	Type          WizardType
	Step          WizardStep
	SelectedType  string   // Selected auth type: "bearer", "apikey", "basic"
	MenuItems     []string // Menu options for selection
	SelectedIndex int      // Currently selected menu item
	FormFields    []FormField
	FocusedField  int // Currently focused form field
}

// FormField represents a single input field in a form
type FormField struct {
	Name        string
	Label       string
	Value       string
	Placeholder string
	IsPassword  bool
	IsRadio     bool
	RadioIndex  int      // For radio button groups
	Options     []string // Options for radio buttons
}

// NewAuthWizard creates a new authentication wizard
func NewAuthWizard() *WizardState {
	return &WizardState{
		Type:          WizardAuth,
		Step:          StepSelectType,
		MenuItems:     []string{"Bearer Token", "API Key", "Basic Auth", "None (clear auth)"},
		SelectedIndex: 0,
	}
}

// CreateAuthFormFields creates form fields based on selected auth type
func CreateAuthFormFields(authType string) []FormField {
	switch authType {
	case "bearer":
		return []FormField{
			{
				Name:        "token",
				Label:       "Bearer Token:",
				Placeholder: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			},
			{
				Name:        "profile_name",
				Label:       "Save as profile (optional):",
				Placeholder: "production",
			},
		}

	case "apikey":
		return []FormField{
			{
				Name:        "key",
				Label:       "Key name (e.g., X-API-Key):",
				Placeholder: "X-API-Key",
			},
			{
				Name:        "value",
				Label:       "Key value:",
				Placeholder: "your-api-key-here",
			},
			{
				Name:       "location",
				Label:      "Location:",
				IsRadio:    true,
				RadioIndex: 0,
				Options:    []string{"header", "query"},
			},
			{
				Name:        "profile_name",
				Label:       "Save as profile (optional):",
				Placeholder: "staging-apikey",
			},
		}

	case "basic":
		return []FormField{
			{
				Name:        "username",
				Label:       "Username:",
				Placeholder: "admin",
			},
			{
				Name:        "password",
				Label:       "Password:",
				Placeholder: "••••••••",
				IsPassword:  true,
			},
			{
				Name:        "profile_name",
				Label:       "Save as profile (optional):",
				Placeholder: "dev-basic",
			},
		}

	default:
		return []FormField{}
	}
}

// BuildAuthProviderFromForm creates an AuthProvider from form fields
func BuildAuthProviderFromForm(authType string, fields []FormField) (auth.AuthProvider, string, error) {
	fieldMap := make(map[string]string)
	for _, field := range fields {
		if field.IsRadio {
			if field.RadioIndex < len(field.Options) {
				fieldMap[field.Name] = field.Options[field.RadioIndex]
			}
		} else {
			fieldMap[field.Name] = field.Value
		}
	}

	profileName := fieldMap["profile_name"]

	switch authType {
	case "bearer":
		return auth.NewBearerAuth(fieldMap["token"]), profileName, nil

	case "apikey":
		location := fieldMap["location"]
		if location == "" {
			location = "header"
		}
		return auth.NewAPIKeyAuth(fieldMap["key"], fieldMap["value"], location), profileName, nil

	case "basic":
		return auth.NewBasicAuth(fieldMap["username"], fieldMap["password"]), profileName, nil

	case "none":
		return &auth.NoAuth{}, "", nil

	default:
		return nil, "", nil
	}
}

// handleWizardKeys handles keyboard input in wizard mode
func handleWizardKeys(m TestUIModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.wizardState == nil {
		m.agentState = StateIdle
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEsc:
		// Cancel wizard
		m.wizardState = nil
		m.agentState = StateIdle
		m.addMessage(m.subtleStyle.Render("Wizard cancelled"))
		m.addMessage("")
		return m, nil

	case tea.KeyUp:
		switch m.wizardState.Step {
		case StepSelectType:
			// Navigate menu up
			m.wizardState.SelectedIndex--
			if m.wizardState.SelectedIndex < 0 {
				m.wizardState.SelectedIndex = len(m.wizardState.MenuItems) - 1
			}
		case StepFillForm:
			// Navigate form fields up
			m.wizardState.FocusedField--
			if m.wizardState.FocusedField < 0 {
				m.wizardState.FocusedField = len(m.wizardState.FormFields) - 1
			}
		}
		return m, nil

	case tea.KeyDown:
		switch m.wizardState.Step {
		case StepSelectType:
			// Navigate menu down
			m.wizardState.SelectedIndex++
			if m.wizardState.SelectedIndex >= len(m.wizardState.MenuItems) {
				m.wizardState.SelectedIndex = 0
			}
		case StepFillForm:
			// Navigate form fields down
			m.wizardState.FocusedField++
			if m.wizardState.FocusedField >= len(m.wizardState.FormFields) {
				m.wizardState.FocusedField = 0
			}
		}
		return m, nil

	case tea.KeyTab:
		if m.wizardState.Step == StepFillForm {
			// Tab to next field
			m.wizardState.FocusedField++
			if m.wizardState.FocusedField >= len(m.wizardState.FormFields) {
				m.wizardState.FocusedField = 0
			}
		}
		return m, nil

	case tea.KeyLeft, tea.KeyRight:
		if m.wizardState.Step == StepFillForm {
			// Handle radio button navigation
			field := &m.wizardState.FormFields[m.wizardState.FocusedField]
			if field.IsRadio && len(field.Options) > 0 {
				if msg.Type == tea.KeyLeft {
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
		}
		return m, nil

	case tea.KeyEnter:
		switch m.wizardState.Step {
		case StepSelectType:
			// User selected an auth type
			selectedItem := m.wizardState.MenuItems[m.wizardState.SelectedIndex]

			// Map menu item to auth type
			var authType string
			switch selectedItem {
			case "Bearer Token":
				authType = "bearer"
			case "API Key":
				authType = "apikey"
			case "Basic Auth":
				authType = "basic"
			case "None (clear auth)":
				authType = "none"
			}

			m.wizardState.SelectedType = authType

			if authType == "none" {
				// Clear auth immediately
				m.authProvider = &auth.NoAuth{}
				m.testExecutor.UpdateAuthProvider(m.authProvider)
				m.wizardState = nil
				m.agentState = StateIdle
				m.addMessage(m.successStyle.Render("✓ Authentication cleared"))
				m.addMessage("")
				return m, nil
			}

			// Move to form step
			m.wizardState.Step = StepFillForm
			m.wizardState.FormFields = CreateAuthFormFields(authType)
			m.wizardState.FocusedField = 0
			return m, nil

		case StepFillForm:
			// Check if user is on a regular field (not radio) and wants to submit
			field := &m.wizardState.FormFields[m.wizardState.FocusedField]
			if !field.IsRadio {
				// Check if this is the last field - if yes, try to submit
				if m.wizardState.FocusedField == len(m.wizardState.FormFields)-1 {
					return submitAuthForm(m)
				}
				// Otherwise move to next field
				m.wizardState.FocusedField++
				if m.wizardState.FocusedField >= len(m.wizardState.FormFields) {
					m.wizardState.FocusedField = 0
				}
			}
		}
		return m, nil

	case tea.KeyCtrlS:
		// Ctrl+S to submit form
		if m.wizardState.Step == StepFillForm {
			return submitAuthForm(m)
		}
		return m, nil

	case tea.KeyBackspace:
		if m.wizardState.Step == StepFillForm {
			field := &m.wizardState.FormFields[m.wizardState.FocusedField]
			if !field.IsRadio && len(field.Value) > 0 {
				field.Value = field.Value[:len(field.Value)-1]
			}
		}
		return m, nil

	case tea.KeyRunes:
		if m.wizardState.Step == StepFillForm {
			field := &m.wizardState.FormFields[m.wizardState.FocusedField]
			if !field.IsRadio {
				field.Value += string(msg.Runes)
			}
		}
		return m, nil
	}

	return m, nil
}

// submitAuthForm validates and applies the auth configuration
func submitAuthForm(m TestUIModel) (tea.Model, tea.Cmd) {
	authProvider, profileName, err := BuildAuthProviderFromForm(
		m.wizardState.SelectedType,
		m.wizardState.FormFields,
	)

	if err != nil {
		m.addMessage(m.errorStyle.Render("Error: " + err.Error()))
		m.addMessage("")
		return m, nil
	}

	// Validate auth provider
	if err := authProvider.Validate(); err != nil {
		m.addMessage(m.errorStyle.Render("Validation failed: " + err.Error()))
		m.addMessage("")
		return m, nil
	}

	// Apply auth
	m.authProvider = authProvider
	m.testExecutor.UpdateAuthProvider(authProvider)

	// Save as profile if name provided
	if strings.TrimSpace(profileName) != "" {
		m.addMessage(m.successStyle.Render(fmt.Sprintf("✓ Authentication configured and saved as '%s'", profileName)))
	} else {
		if stringer, ok := authProvider.(fmt.Stringer); ok {
			m.addMessage(m.successStyle.Render("✓ " + stringer.String()))
		} else {
			m.addMessage(m.successStyle.Render(fmt.Sprintf("✓ %s authentication configured", authProvider.Type())))
		}
	}
	m.addMessage("")

	// Exit wizard
	m.wizardState = nil
	m.agentState = StateIdle

	return m, nil
}

// RenderWizard renders the current wizard state
func (m TestUIModel) RenderWizard() string {
	if m.wizardState == nil {
		return ""
	}

	switch m.wizardState.Type {
	case WizardAuth:
		return m.renderAuthWizard()
	default:
		return ""
	}
}

// renderAuthWizard renders the authentication wizard
func (m TestUIModel) renderAuthWizard() string {
	if m.wizardState.Step == StepSelectType {
		return m.renderAuthTypeSelector()
	} else if m.wizardState.Step == StepFillForm {
		return m.renderAuthForm()
	}
	return ""
}

// renderAuthTypeSelector renders the auth type selection menu
func (m TestUIModel) renderAuthTypeSelector() string {
	var b strings.Builder

	// Border style
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Theme.Primary).
		Padding(1, 2).
		Width(50)

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true)

	b.WriteString(titleStyle.Render("Select Authentication Type"))
	b.WriteString("\n\n")

	// Menu items
	for i, item := range m.wizardState.MenuItems {
		if i == m.wizardState.SelectedIndex {
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

	b.WriteString(helpStyle.Render("[↑↓ to select, Enter to confirm, Esc to cancel]"))

	return borderStyle.Render(b.String())
}

// renderAuthForm renders the auth configuration form
func (m TestUIModel) renderAuthForm() string {
	var b strings.Builder

	// Border style
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Theme.Primary).
		Padding(1, 2).
		Width(60)

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true)

	var title string
	switch m.wizardState.SelectedType {
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
	for i, field := range m.wizardState.FormFields {
		isFocused := i == m.wizardState.FocusedField

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
	b.WriteString(helpStyle.Render("[Ctrl+S or Enter on last field to submit, Esc to cancel]"))

	return borderStyle.Render(b.String())
}
