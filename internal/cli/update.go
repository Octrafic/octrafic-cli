package cli

import (
	"encoding/json"
	"fmt"
	"github.com/Octrafic/octrafic-cli/internal/agents"
	"github.com/Octrafic/octrafic-cli/internal/config"
	"github.com/Octrafic/octrafic-cli/internal/core/auth"
	"github.com/Octrafic/octrafic-cli/internal/infra/logger"
	"github.com/Octrafic/octrafic-cli/internal/infra/storage"
	"github.com/Octrafic/octrafic-cli/internal/updater"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type releaseNotesMsg struct {
	notes string
	url   string
	err   error
}

// Update handles all incoming messages and updates the model state.
func (m TestUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 7
		m.textarea.SetWidth(msg.Width - 4)
		m.updateViewport()

	case streamReasoningMsg:
		if m.lastMessageRole != "assistant" {
			m.lastMessageRole = "assistant"
		}
		return m, waitForReasoning(msg.channel)

	case streamDoneMsg:
		m.agentState = StateIdle
		return m, nil

	case reasoningChunkMsg:
		return handleStreamingMsg(&m, msg)

	case agentResponseMsg:
		m.agentState = StateIdle

		if msg.err != nil {
			m.addMessage("")
			m.addMessage(m.errorStyle.Render("Error - " + msg.err.Error()))
			m.addMessage("")
		} else {
			rendered := renderMarkdown(msg.message)
			m.addMessage(rendered)
			m.addMessage("")

			m.lastMessageRole = "assistant"

			chatMsg := agent.ChatMessage{
				Role:             "assistant",
				Content:          msg.message,
				ReasoningContent: msg.reasoning,
				FunctionCalls:    msg.toolCalls,
			}

			m.conversationHistory = append(m.conversationHistory, chatMsg)
			m.saveChatMessageToConversation(chatMsg)

			if len(msg.toolCalls) > 0 {
				toolCall := msg.toolCalls[0]
				needsConfirmation := m.shouldAskForConfirmation(toolCall.Name)

				if m.executionMode == ModeAutoExecute || !needsConfirmation {
					m.currentToolCall = &toolCall
					m.agentState = StateUsingTool
					m.animationFrame = 0
					m.spinner.Style = lipgloss.NewStyle().Foreground(Theme.PrimaryDark)
					return m, tea.Batch(animationTick(), m.executeTool(toolCall))
				} else {
					m.pendingToolCall = &toolCall
					m.confirmationChoice = 0
					m.agentState = StateAskingConfirmation
					return m, nil
				}
			}
		}

	case toolResultMsg:
		if msg.err != nil {
			m.addMessage(m.errorStyle.Render("Error: " + msg.err.Error()))
			m.addMessage("")
			m.lastMessageRole = "assistant"

			m.conversationHistory = append(m.conversationHistory, agent.ChatMessage{
				Role:    "user",
				Content: fmt.Sprintf("Tool error: %s", msg.err.Error()),
			})

			m.agentState = StateThinking
			return m, m.sendChatMessage("")
		} else {
			nextCmd := m.handleToolResult(msg.toolName, msg.toolID, msg.result)
			if nextCmd != nil {
				m.agentState = StateThinking
				return m, nextCmd
			}
			m.agentState = StateIdle
		}

	case releaseNotesMsg:
		if msg.err != nil {
			m.addMessage(m.errorStyle.Render("Failed to fetch release notes: " + msg.err.Error()))
		} else {
			m.addMessage(renderMarkdown(msg.notes))
			if msg.url != "" {
				m.addMessage("")
				m.addMessage(lipgloss.NewStyle().Foreground(Theme.Cyan).Render(msg.url))
			}
		}
		m.lastMessageRole = "assistant"
		m.updateViewport()
		return m, nil

	case ModelsFetchedMsg:
		if m.modelSelector != nil {
			if msg.Error != "" {
				m.modelSelector.SetError(msg.Error)
			} else {
				m.modelSelector.SetModels(msg.Models)
			}
		}
		return m, nil

	case clearHintTimeoutMsg:
		if m.showClearHint && time.Since(m.lastEscPress) >= 700*time.Millisecond {
			m.showClearHint = false
		}
		return m, nil

	case tea.KeyMsg:
		if newM, cmd, handled := handleGlobalKeyboard(&m, msg); handled {
			return *newM, cmd
		}

		// Handle file picker specific keys first
		if m.showFilePicker {
			if newM, cmd, handled := handleFilePickerState(&m, msg); handled {
				// After handling, verify state (did user delete '@' with backspace?)
				// Actually backspace is handled in handleFilePickerState.
				// But we should check if we should still show it.
				return newM, cmd
			}
		}

		if m.agentState == StateShowingTestPlan {
			return handleTestPlanState(&m, msg)
		}

		if m.agentState == StateAskingConfirmation {
			return handleConfirmationState(&m, msg)
		}

		if m.agentState == StateWizard {
			return handleWizardKeys(m, msg)
		}

		if m.agentState == StateModelSelector {
			return handleModelSelectorState(&m, msg)
		}

		if m.agentState == StateShowingCommands {
			return handleCommandsState(&m, msg)
		}

		if m.agentState == StateIdle {
			switch msg.Type {
			case tea.KeyEnter:
				userInput := m.textarea.Value()
				var displayInput, llmInput string

				// Handle file content expansion if not escaped newline
				if !strings.HasSuffix(userInput, "\\") {
					displayInput, llmInput = m.expandFileContent(userInput)
				} else {
					displayInput = userInput
					llmInput = userInput
				}

				if strings.HasSuffix(userInput, "\\") {
					newValue := strings.TrimSuffix(userInput, "\\") + "\n"
					m.textarea.SetValue(newValue)
					m.textarea.SetCursor(len(newValue))

					lines := strings.Count(m.textarea.Value(), "\n") + 1
					if lines < 1 {
						lines = 1
					} else if lines > 6 {
						lines = 6
					}
					if m.textarea.Height() != lines {
						m.textarea.SetHeight(lines)
					}

					return m, nil
				}
				if strings.TrimSpace(userInput) != "" {
					m.commandHistory = append(m.commandHistory, userInput)
					m.historyIndex = -1
					m.temporaryInput = ""

					m.textarea.SetValue("")
					m.textarea.SetHeight(1)
					m.showClearHint = false

					if newM, cmd, handled := handleSlashCommands(&m, userInput); handled {
						return *newM, cmd
					}

					if newM, cmd, handled := handleAuthCommand(&m, userInput); handled {
						return *newM, cmd
					}

					if m.conversationID != "" && !m.isLoadedConversation && m.currentProject != nil && !m.currentProject.IsTemporary {
						if _, err := storage.LoadConversation(m.currentProject.ID, m.conversationID); err != nil {
							title := userInput
							if len(title) > 100 {
								title = title[:97] + "..."
							}
							_, _ = storage.CreateConversation(m.currentProject.ID, m.conversationID, title)
							m.conversationTitle = title
						}
					}

					if m.isLoadedConversation {
						m.isLoadedConversation = false
					}

					userMessage := lipgloss.NewStyle().
						Foreground(Theme.TextMuted).
						Render("> ") + displayInput
					m.addMessage("")
					m.addMessage(userMessage)

					// Save clean path to storage, keep display version (with colors) in metadata for UI restoration
					meta := map[string]interface{}{
						"display_content": displayInput,
					}
					m.saveMessageToConversation("user", llmInput, meta)
					m.lastMessageRole = "user"

					m.conversationHistory = append(m.conversationHistory, agent.ChatMessage{
						Role:    "user",
						Content: llmInput,
					})

					m.cancelStream = make(chan struct{})
					m.agentState = StateProcessing
					m.animationFrame = 0
					m.spinner.Style = lipgloss.NewStyle().Foreground(Theme.Primary)

					var cmds []tea.Cmd
					cmds = append(cmds, animationTick(), m.sendChatMessage(llmInput))
					if m.conversationTitle != "" {
						cmds = append(cmds, tea.SetWindowTitle(m.conversationTitle+" - Octrafic"))
					}
					return m, tea.Batch(cmds...)
				}
			case tea.KeyUp:
				if len(m.commandHistory) > 0 {
					if m.historyIndex == -1 {
						m.temporaryInput = m.textarea.Value()
						m.historyIndex = len(m.commandHistory) - 1
					} else if m.historyIndex > 0 {
						m.historyIndex--
					}
					if m.historyIndex >= 0 && m.historyIndex < len(m.commandHistory) {
						m.textarea.SetValue(m.commandHistory[m.historyIndex])
						m.textarea.SetCursor(len(m.commandHistory[m.historyIndex]))
					}
				}
				return m, nil
			case tea.KeyDown:
				if len(m.commandHistory) > 0 && m.historyIndex != -1 {
					if m.historyIndex < len(m.commandHistory)-1 {
						m.historyIndex++
						m.textarea.SetValue(m.commandHistory[m.historyIndex])
						m.textarea.SetCursor(len(m.commandHistory[m.historyIndex]))
					} else {
						m.historyIndex = -1
						m.textarea.SetValue(m.temporaryInput)
						m.textarea.SetCursor(len(m.temporaryInput))
					}
				}
				return m, nil
			case tea.KeyPgUp:
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			case tea.KeyPgDown:
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
			m.textarea, cmd = m.textarea.Update(msg)

			// Check if we should close the picker due to cursor movement or text changes
			// executed by the textarea update (e.g. left arrow, deletion)
			if m.showFilePicker {
				val := m.textarea.Value()
				cursor := m.textarea.CursorIndex()

				// Simple check: do we have an '@' left of cursor?
				// Find last '@'
				lastAt := -1
				for i := cursor - 1; i >= 0; i-- {
					if i < len(val) && val[i] == '@' {
						lastAt = i
						break
					}
				}

				if lastAt == -1 {
					// No '@' before cursor, close picker
					m.showFilePicker = false
					m.fileFilterText = ""
				} else {
					// Update filter text to match what's between @ and cursor
					// This keeps it in sync if user moves cursor or types
					// However, we need to be careful not to overwrite if we just did a path completion
					// But for now, sync is good.
					if cursor > lastAt+1 {
						m.fileFilterText = val[lastAt+1 : cursor]
					} else {
						m.fileFilterText = ""
					}
					m.updateFileSuggestions()
				}
			}

			if msg.String() == "@" {
				m.showFilePicker = true
				m.fileFilterText = ""
				m.selectedFileIndex = 0
				m.updateFileSuggestions()
			}

			if msg.Type == tea.KeyRunes || msg.Type == tea.KeyBackspace || msg.Type == tea.KeyDelete {
				m.historyIndex = -1
			}

			m.showClearHint = false

			lines := strings.Count(m.textarea.Value(), "\n") + 1
			if lines < 1 {
				lines = 1
			} else if lines > 6 {
				lines = 6
			}
			if m.textarea.Height() != lines {
				m.textarea.SetHeight(lines)
			}

			input := m.textarea.Value()
			if strings.HasPrefix(input, "/") && len(input) > 0 {
				m.filteredCommands = filterCommands(input)
				if len(m.filteredCommands) > 0 {
					m.agentState = StateShowingCommands
					m.selectedCommandIndex = 0
				}
			}

			return m, cmd
		}

	case backendErrorMsg:
		m.addMessage(m.errorStyle.Render(fmt.Sprintf("❌ Error: %v", msg.err)))
		m.agentState = StateIdle
		m.updateViewport()
		return m, nil

	case generateTestPlanResultMsg:
		return handleGenerateTestPlanResult(&m, msg)

	case showTestSelectionMsg:
		return handleShowTestSelection(&m, msg)

	case processToolCallsMsg:
		return handleProcessToolCalls(&m, msg)

	case animationTickMsg:
		if m.agentState == StateThinking || m.agentState == StateUsingTool || m.agentState == StateProcessing || m.agentState == StateRunningTests {
			m.animationFrame = (m.animationFrame + 1) % 1000
			return m, animationTick()
		}

	case startTestGroupMsg:
		return handleStartTestGroup(&m, msg)

	case runNextTestMsg:
		return handleRunNextTest(&m, msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleStreamingMsg processes streaming chunks from the agent.
func handleStreamingMsg(m *TestUIModel, msg reasoningChunkMsg) (tea.Model, tea.Cmd) {
	msgType := "UNKNOWN"
	if strings.HasPrefix(msg.chunk, "\x00ERROR:") {
		msgType = "ERROR"
	} else if strings.HasPrefix(msg.chunk, "\x00AGENT:") {
		msgType = "AGENT"
	} else if strings.HasPrefix(msg.chunk, "\x00TOKENS:") {
		msgType = "TOKENS"
	} else if strings.HasPrefix(msg.chunk, "\x00DONE:") {
		msgType = "DONE"
	} else if strings.HasPrefix(msg.chunk, "\x00TOOLS:") {
		msgType = "TOOLS"
	} else if strings.HasPrefix(msg.chunk, "\x00THINK:") {
		msgType = "THINK"
	} else if strings.HasPrefix(msg.chunk, "\x00TEXT:") {
		msgType = "TEXT"
	}
	logger.Debug("Received streaming message", logger.String("type", msgType), zap.Int("length", len(msg.chunk)))

	if strings.HasPrefix(msg.chunk, "\x00ERROR:") {
		errMsg := strings.TrimPrefix(msg.chunk, "\x00ERROR:")
		logger.Error("Streaming error", logger.String("error", errMsg))
		m.addMessage(m.errorStyle.Render("Error - " + errMsg))
		m.addMessage("")
		m.agentState = StateIdle
		return m, nil
	} else if strings.HasPrefix(msg.chunk, "\x00AGENT:") {
		agentMsg := strings.TrimPrefix(msg.chunk, "\x00AGENT:")
		m.streamedReasoningChunk = ""

		displayedContent := false
		if m.streamedTextChunk != "" {
			logger.Debug("Displaying accumulated content", zap.Int("length", len(m.streamedTextChunk)))
			if m.streamedAgentMessage == "" {
				m.streamedAgentMessage = m.streamedTextChunk
			}
			m.addMessage("")
			m.addMessage(renderMarkdown(m.streamedTextChunk))
			m.updateViewport()
			m.streamedTextChunk = ""
			displayedContent = true
		}
		if agentMsg != "" && !displayedContent {
			logger.Debug("Displaying agent message", zap.Int("length", len(agentMsg)))
			m.streamedAgentMessage = agentMsg
			m.addMessage("")
			m.addMessage(renderMarkdown(agentMsg))
			m.updateViewport()
		} else if agentMsg != "" && displayedContent {
			m.streamedAgentMessage = agentMsg
		}
		return m, waitForReasoning(msg.channel)
	} else if strings.HasPrefix(msg.chunk, "\x00TOKENS:") {
		tokenData := strings.TrimPrefix(msg.chunk, "\x00TOKENS:")
		var input, output int64
		if _, err := fmt.Sscanf(tokenData, "%d,%d", &input, &output); err == nil {
			m.streamedInputTokens = input
			m.streamedOutputTokens = output
			logger.Debug("Token counts for current message", zap.Int64("input", input), zap.Int64("output", output))
		}
		return m, waitForReasoning(msg.channel)
	} else if strings.HasPrefix(msg.chunk, "\x00DONE:") {
		if m.streamedTextChunk != "" {
			logger.Debug("Displaying remaining content in DONE", zap.Int("length", len(m.streamedTextChunk)))
			m.addMessage("")
			m.addMessage(renderMarkdown(m.streamedTextChunk))
			m.updateViewport()
		}

		finalContent := m.streamedAgentMessage
		if finalContent == "" && m.streamedTextChunk != "" {
			finalContent = m.streamedTextChunk
		}

		m.streamedReasoningChunk = ""
		m.streamedTextChunk = ""

		chatMsg := agent.ChatMessage{
			Role:         "assistant",
			Content:      finalContent,
			InputTokens:  m.streamedInputTokens,
			OutputTokens: m.streamedOutputTokens,
		}
		if len(m.streamedToolCalls) > 0 {
			chatMsg.FunctionCalls = m.streamedToolCalls
		}
		m.conversationHistory = append(m.conversationHistory, chatMsg)
		m.saveChatMessageToConversation(chatMsg)

		m.inputTokens += m.streamedInputTokens
		m.outputTokens += m.streamedOutputTokens

		m.streamedAgentMessage = ""
		m.streamedInputTokens = 0
		m.streamedOutputTokens = 0

		if len(m.streamedToolCalls) > 0 {
			return m, tea.Tick(time.Second*1, func(time.Time) tea.Msg {
				return processToolCallsMsg{}
			})
		}

		if m.agentState == StateShowingTestPlan {
			return m, nil
		}

		m.agentState = StateIdle
		return m, nil
	} else if strings.HasPrefix(msg.chunk, "\x00TOOLS:") {
		toolCallsJSON := strings.TrimPrefix(msg.chunk, "\x00TOOLS:")
		var toolCalls []agent.ToolCall
		if err := json.Unmarshal([]byte(toolCallsJSON), &toolCalls); err == nil {
			m.streamedToolCalls = toolCalls
		}
		return m, waitForReasoning(msg.channel)
	} else if strings.HasPrefix(msg.chunk, "\x00THINK:") {
		chunk := strings.TrimPrefix(msg.chunk, "\x00THINK:")
		m.streamedReasoningChunk += chunk
		return m, waitForReasoning(msg.channel)
	} else if strings.HasPrefix(msg.chunk, "\x00TEXT:") {
		chunk := strings.TrimPrefix(msg.chunk, "\x00TEXT:")
		m.streamedTextChunk += chunk
		return m, waitForReasoning(msg.channel)
	} else if strings.HasPrefix(msg.chunk, "\x00CANCELLED:") {
		if m.streamedTextChunk != "" {
			m.addMessage("")
			m.addMessage(renderMarkdown(m.streamedTextChunk))
			m.updateViewport()
		}

		finalContent := m.streamedTextChunk
		if finalContent == "" {
			finalContent = m.streamedAgentMessage
		}

		if finalContent != "" {
			chatMsg := agent.ChatMessage{
				Role:    "assistant",
				Content: finalContent + " (cancelled)",
			}
			m.saveChatMessageToConversation(chatMsg)
		}

		m.streamedReasoningChunk = ""
		m.streamedTextChunk = ""
		m.streamedAgentMessage = ""
		m.streamedToolCalls = nil

		m.agentState = StateIdle
		return m, nil
	}

	return m, waitForReasoning(msg.channel)
}

// handleGlobalKeyboard processes global keyboard shortcuts like Ctrl+C and Esc.
func handleGlobalKeyboard(m *TestUIModel, msg tea.KeyMsg) (*TestUIModel, tea.Cmd, bool) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit, true
	}

	if msg.Type == tea.KeyEsc {
		if m.agentState != StateIdle {
			if m.agentState == StateProcessing || m.agentState == StateThinking {
				if m.cancelStream != nil {
					close(m.cancelStream)
					m.cancelStream = nil
				}

				if m.streamedTextChunk != "" || m.streamedAgentMessage != "" {
					content := m.streamedTextChunk
					if content == "" {
						content = m.streamedAgentMessage
					}
					m.conversationHistory = append(m.conversationHistory, agent.ChatMessage{
						Role:    "assistant",
						Content: content,
					})
				}

				m.streamedReasoningChunk = ""
				m.streamedTextChunk = ""
				m.streamedAgentMessage = ""
				m.streamedToolCalls = nil
			}

			m.agentState = StateIdle
			m.currentToolCall = nil
			m.pendingToolCall = nil
			m.addMessage("")
			m.addMessage(m.errorStyle.Render("Operation cancelled"))
			m.addMessage("")
			m.lastMessageRole = "assistant"
			m.showClearHint = false
			return m, nil, true
		}

		if m.agentState == StateIdle && m.textarea.Value() != "" {
			now := time.Now()
			if m.showClearHint && now.Sub(m.lastEscPress) < 700*time.Millisecond {
				m.textarea.SetValue("")
				m.textarea.SetHeight(1)
				m.showClearHint = false
				return m, nil, true
			} else {
				m.lastEscPress = now
				m.showClearHint = true
				return m, tea.Tick(700*time.Millisecond, func(time.Time) tea.Msg {
					return clearHintTimeoutMsg{}
				}), true
			}
		}
	}

	return m, nil, false
}

// handleSlashCommands processes commands starting with "/" like /help, /clear, etc.
func handleSlashCommands(m *TestUIModel, userInput string) (*TestUIModel, tea.Cmd, bool) {
	if !strings.HasPrefix(userInput, "/") {
		return m, nil, false
	}

	if strings.HasPrefix(userInput, "/save") {
		m.addMessage("")
		m.addMessage(renderUserLabel() + " " + userInput)
		m.addMessage("")
		m.lastMessageRole = "user"

		if m.currentProject == nil || !m.currentProject.IsTemporary {
			m.addMessage(m.errorStyle.Render("✗ No temporary project to save"))
			m.addMessage(m.subtleStyle.Render("Use /save <name> when working with a temp project"))
			m.lastMessageRole = "assistant"
			return m, nil, true
		}

		parts := strings.Fields(userInput)
		if len(parts) < 2 || parts[1] == "" {
			m.addMessage(m.errorStyle.Render("Usage: /save <project-name>"))
			m.lastMessageRole = "assistant"
			return m, nil, true
		}

		newName := parts[1]

		if conflict, err := storage.CheckNameConflict(newName, m.currentProject.ID); err != nil {
			m.addMessage(m.errorStyle.Render("Failed to check name: " + err.Error()))
			m.lastMessageRole = "assistant"
			return m, nil, true
		} else if conflict != nil {
			m.addMessage(m.errorStyle.Render(fmt.Sprintf("✗ Project name \"%s\" already exists", newName)))
			m.lastMessageRole = "assistant"
			return m, nil, true
		}

		updatedProject, err := storage.ConvertToPermanent(m.currentProject, newName)
		if err != nil {
			m.addMessage(m.errorStyle.Render("✗ Failed to save project: " + err.Error()))
			m.lastMessageRole = "assistant"
			return m, nil, true
		}

		m.currentProject = updatedProject

		if m.conversationID == "" {
			m.conversationID = uuid.New().String()
		}

		if len(m.conversationHistory) > 0 {
			title := newName
			if _, err := storage.CreateConversation(updatedProject.ID, m.conversationID, title); err == nil {
				m.conversationTitle = title
				for _, msg := range m.conversationHistory {
					meta := map[string]interface{}{}
					if msg.ReasoningContent != "" {
						meta["reasoning"] = msg.ReasoningContent
					}
					if len(msg.FunctionCalls) > 0 {
						meta["tool_calls"] = msg.FunctionCalls
					}
					if msg.InputTokens > 0 {
						meta["input_tokens"] = msg.InputTokens
					}
					if msg.OutputTokens > 0 {
						meta["output_tokens"] = msg.OutputTokens
					}
					_ = storage.SaveMessage(updatedProject.ID, m.conversationID, msg.Role, msg.Content, meta)
				}
			}
		}

		m.addMessage(m.successStyle.Render(fmt.Sprintf("✓ Project saved as \"%s\"", newName)))
		m.lastMessageRole = "assistant"
		return m, nil, true
	}

	switch userInput {
	case "/clear":
		m.addMessage("")
		m.addMessage(renderUserLabel() + " " + userInput)
		m.addMessage("")
		m.lastMessageRole = "user"

		m.addMessage(m.successStyle.Render("✓ Conversation cleared"))
		m.lastMessageRole = "assistant"

		m.conversationHistory = []agent.ChatMessage{}
		m.recreateHeader()
		return m, nil, true

	case "/help":
		m.addMessage("")
		m.addMessage(renderUserLabel() + " " + userInput)
		m.addMessage("")
		m.lastMessageRole = "user"

		m.addMessage(m.agentStyle.Render("Available commands:"))
		for _, cmd := range availableCommands {
			m.addMessage(lipgloss.NewStyle().Foreground(Theme.Primary).Render(cmd.Name) + " - " + cmd.Description)
		}
		m.lastMessageRole = "assistant"
		return m, nil, true

	case "/logout":
		m.addMessage("")
		m.addMessage(renderUserLabel() + " " + userInput)
		m.addMessage("")
		m.lastMessageRole = "user"

		if err := storage.ClearSession(); err != nil {
			m.addMessage(m.errorStyle.Render("Failed to logout: " + err.Error()))
		} else {
			m.addMessage(m.successStyle.Render("✓ Logged out successfully"))
			m.addMessage(m.subtleStyle.Render("Restart the CLI to login again"))
		}
		m.lastMessageRole = "assistant"
		return m, nil, true

	case "/exit":
		m.addMessage("")
		m.addMessage(renderUserLabel() + " " + userInput)
		m.addMessage("")
		m.addMessage(m.subtleStyle.Render("Goodbye!"))
		return m, tea.Quit, true

	case "/auth":
		m.addMessage("")
		m.addMessage(renderUserLabel() + " " + userInput)
		m.addMessage("")
		m.lastMessageRole = "user"

		m.addMessage(m.subtleStyle.Render("Opening authentication wizard..."))
		m.lastMessageRole = "assistant"

		m.wizardState = NewAuthWizard()
		m.agentState = StateWizard
		return m, nil, true

	case "/release-notes":
		m.addMessage("")
		m.addMessage(renderUserLabel() + " " + userInput)
		m.addMessage("")
		m.lastMessageRole = "user"

		cmd := func() tea.Msg {
			notes, url, err := updater.FetchReleaseNotes("")
			return releaseNotesMsg{notes: notes, url: url, err: err}
		}
		return m, cmd, true

	case "/models":
		m.addMessage("")
		m.addMessage(renderUserLabel() + " " + userInput)
		m.addMessage("")
		m.lastMessageRole = "user"

		m.modelSelector = NewModelSelector()
		m.modelSelector.isLoading = true
		m.agentState = StateModelSelector

		cfg, err := config.Load()
		if err != nil {
			m.modelSelector.SetError("Failed to load config: " + err.Error())
			return m, nil, true
		}
		m.modelSelector.SetProvider(cfg.Provider)
		return m, FetchModelsForProvider(cfg), true

	case "/info":
		m.addMessage("")
		m.addMessage(renderUserLabel() + " " + userInput)
		m.addMessage("")
		m.lastMessageRole = "user"

		if m.currentProject == nil {
			m.addMessage(m.subtleStyle.Render("No active project"))
			m.lastMessageRole = "assistant"
			return m, nil, true
		}

		m.addMessage(m.agentStyle.Render(fmt.Sprintf("Project: %s", m.currentProject.Name)))
		m.addMessage(fmt.Sprintf("  ID: %s", m.currentProject.ID))
		m.addMessage(fmt.Sprintf("  URL: %s", m.currentProject.BaseURL))
		if m.currentProject.SpecPath != "" {
			m.addMessage(fmt.Sprintf("  Spec: %s", m.currentProject.SpecPath))
			if m.currentProject.SpecHash != "" {
				m.addMessage(m.subtleStyle.Render(fmt.Sprintf("  Hash: %s", m.currentProject.SpecHash[:8]+"...")))
			}
		}
		m.addMessage(m.subtleStyle.Render(fmt.Sprintf("  Created: %s", m.currentProject.CreatedAt.Format("2006-01-02 15:04"))))
		m.lastMessageRole = "assistant"
		return m, nil, true
	}

	return m, nil, false
}

// handleAuthCommand processes auth subcommands like "auth bearer", "auth apikey", etc.
func handleAuthCommand(m *TestUIModel, userInput string) (*TestUIModel, tea.Cmd, bool) {
	if !strings.HasPrefix(userInput, "auth ") {
		return m, nil, false
	}

	m.addMessage("")
	m.addMessage(renderUserLabel() + " " + userInput)
	m.addMessage("")
	m.lastMessageRole = "user"

	parts := strings.Fields(userInput)
	if len(parts) < 2 {
		m.addMessage(m.errorStyle.Render("Usage: auth <command>"))
		m.addMessage(m.subtleStyle.Render("Commands: bearer <token> | apikey <key> <value> | basic <user> <pass> | show | clear"))
		m.lastMessageRole = "assistant"
		return m, nil, true
	}

	subCmd := parts[1]
	switch subCmd {
	case "bearer":
		if len(parts) < 3 {
			m.addMessage(m.errorStyle.Render("Usage: auth bearer <token>"))
			m.lastMessageRole = "assistant"
			return m, nil, true
		}
		m.authProvider = auth.NewBearerAuth(parts[2])
		m.testExecutor.UpdateAuthProvider(m.authProvider)
		m.addMessage(m.successStyle.Render("✓ Bearer authentication configured"))
		m.lastMessageRole = "assistant"
		return m, nil, true

	case "apikey":
		if len(parts) < 4 {
			m.addMessage(m.errorStyle.Render("Usage: auth apikey <key> <value>"))
			m.addMessage(m.subtleStyle.Render("Example: auth apikey X-API-Key your-key-here"))
			m.lastMessageRole = "assistant"
			return m, nil, true
		}
		m.authProvider = auth.NewAPIKeyAuth(parts[2], parts[3], "header")
		m.testExecutor.UpdateAuthProvider(m.authProvider)
		m.addMessage(m.successStyle.Render(fmt.Sprintf("✓ API Key authentication configured (%s)", parts[2])))
		m.lastMessageRole = "assistant"
		return m, nil, true

	case "basic":
		if len(parts) < 4 {
			m.addMessage(m.errorStyle.Render("Usage: auth basic <username> <password>"))
			m.lastMessageRole = "assistant"
			return m, nil, true
		}
		m.authProvider = auth.NewBasicAuth(parts[2], parts[3])
		m.testExecutor.UpdateAuthProvider(m.authProvider)
		m.addMessage(m.successStyle.Render(fmt.Sprintf("✓ Basic authentication configured (%s)", parts[2])))
		m.lastMessageRole = "assistant"
		return m, nil, true

	case "show":
		if m.authProvider == nil {
			m.addMessage(m.subtleStyle.Render("No authentication configured"))
		} else {
			redacted := m.authProvider.Redact()
			if stringer, ok := redacted.(fmt.Stringer); ok {
				m.addMessage(m.subtleStyle.Render("Current auth: " + stringer.String()))
			} else {
				m.addMessage(m.subtleStyle.Render(fmt.Sprintf("Current auth: %s", redacted.Type())))
			}
		}
		m.lastMessageRole = "assistant"
		return m, nil, true

	case "clear":
		m.authProvider = &auth.NoAuth{}
		m.testExecutor.UpdateAuthProvider(m.authProvider)
		m.addMessage(m.successStyle.Render("✓ Authentication cleared"))
		m.lastMessageRole = "assistant"
		return m, nil, true

	default:
		m.addMessage(m.errorStyle.Render(fmt.Sprintf("Unknown auth command: %s", subCmd)))
		m.addMessage(m.subtleStyle.Render("Commands: bearer | apikey | basic | show | clear"))
		m.lastMessageRole = "assistant"
		return m, nil, true
	}
}

// handleTestPlanState processes keyboard input when showing the test plan selection UI.
func handleTestPlanState(m *TestUIModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.selectedTestIndex > 0 {
			m.selectedTestIndex--
		}
		return m, nil
	case tea.KeyDown:
		if m.selectedTestIndex < len(m.tests)-1 {
			m.selectedTestIndex++
		}
		return m, nil
	case tea.KeyRunes:
		if string(msg.Runes) == " " {
			m.tests[m.selectedTestIndex].Selected = !m.tests[m.selectedTestIndex].Selected
			return m, nil
		}
	case tea.KeyEnter:
		m.addMessage("")

		var selectedTests []Test
		for _, test := range m.tests {
			if test.Selected && test.Status == "pending" {
				selectedTests = append(selectedTests, test)
			}
		}

		if len(selectedTests) == 0 {
			m.addMessage("No tests selected for execution.")
			m.addMessage("")
			m.lastMessageRole = "assistant"
			m.agentState = StateIdle
			m.pendingTestGroupToolCall = nil
			return m, nil
		}

		m.lastMessageRole = "assistant"

		tests := make([]map[string]any, 0)
		for _, test := range selectedTests {
			tests = append(tests, map[string]any{
				"method":        test.Method,
				"endpoint":      test.Endpoint,
				"headers":       test.BackendTest.Headers,
				"body":          test.BackendTest.Body,
				"requires_auth": test.BackendTest.RequiresAuth,
			})
		}

		label := "Running tests"
		if len(tests) > 0 {
			label = fmt.Sprintf("Testing %s %s", tests[0]["method"], tests[0]["endpoint"])
			if len(tests) > 1 {
				label = fmt.Sprintf("Testing %d endpoints", len(tests))
			}
		}

		toolID := ""
		toolName := "ExecuteTestGroup"
		if m.pendingTestGroupToolCall != nil {
			toolID = m.pendingTestGroupToolCall.ID
			toolName = m.pendingTestGroupToolCall.Name
		}
		m.pendingTestGroupToolCall = nil

		m.agentState = StateUsingTool
		m.animationFrame = 0
		m.spinner.Style = lipgloss.NewStyle().Foreground(Theme.PrimaryDark)
		return m, tea.Batch(animationTick(), func() tea.Msg {
			return startTestGroupMsg{
				tests:    tests,
				label:    label,
				toolName: toolName,
				toolID:   toolID,
			}
		})
	case tea.KeyEsc:
		m.agentState = StateIdle
		return m, nil
	default:
		return m, nil
	}
	return m, nil
}

// handleConfirmationState processes keyboard input when asking for tool execution confirmation.
func handleConfirmationState(m *TestUIModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		m.confirmationChoice--
		if m.confirmationChoice < 0 {
			m.confirmationChoice = 2
		}
		return m, nil
	case tea.KeyDown:
		m.confirmationChoice++
		if m.confirmationChoice > 2 {
			m.confirmationChoice = 0
		}
		return m, nil
	case tea.KeyEnter:
		switch m.confirmationChoice {
		case 0:
			m.currentToolCall = m.pendingToolCall
			m.pendingToolCall = nil
			m.agentState = StateUsingTool
			m.animationFrame = 0
			m.spinner.Style = lipgloss.NewStyle().Foreground(Theme.PrimaryDark)
			return m, tea.Batch(animationTick(), m.executeTool(*m.currentToolCall))
		case 1:
			m.pendingToolCall = nil
			m.agentState = StateIdle
			m.addMessage("")
			m.addMessage("Tool execution cancelled")
			m.addMessage("")
			m.lastMessageRole = "assistant"
			return m, nil
		default:
			isExecuteTest := m.pendingToolCall != nil && strings.HasPrefix(m.pendingToolCall.Name, "ExecuteTest")
			m.pendingToolCall = nil

			if isExecuteTest {
				for i, test := range m.tests {
					if test.Status == "pending" {
						m.tests[i].Status = "skipped"
						m.addMessage("")
						m.addMessage(m.subtleStyle.Render("Skipped: " + test.Method + " " + test.Endpoint))
						m.addMessage("")
						m.lastMessageRole = "assistant"
						break
					}
				}
				hasPendingTests := false
				for _, test := range m.tests {
					if test.Status == "pending" {
						hasPendingTests = true
						break
					}
				}
				if hasPendingTests {
					toolCall := agent.ToolCall{Name: "ExecuteTest"}
					m.pendingToolCall = &toolCall
					m.confirmationChoice = 0
					m.agentState = StateAskingConfirmation
					return m, nil
				} else {
					m.agentState = StateIdle
				}
			} else {
				m.addMessage("")
				m.addMessage("Tool execution skipped")
				m.addMessage("")
				m.lastMessageRole = "assistant"
				m.agentState = StateIdle
			}
			return m, nil
		}
	}
	return m, nil
}

// handleCommandsState processes keyboard input when showing command autocomplete suggestions.
func handleCommandsState(m *TestUIModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.Type {
	case tea.KeyUp:
		if m.selectedCommandIndex > 0 {
			m.selectedCommandIndex--
		}
		return m, nil
	case tea.KeyDown:
		if m.selectedCommandIndex < len(m.filteredCommands)-1 {
			m.selectedCommandIndex++
		}
		return m, nil
	case tea.KeyEnter:
		if m.selectedCommandIndex < len(m.filteredCommands) {
			selectedCmd := m.filteredCommands[m.selectedCommandIndex]
			m.textarea.SetValue(selectedCmd.Name)
			m.textarea.SetHeight(1)
			m.textarea.SetCursor(len(selectedCmd.Name))
			m.agentState = StateIdle
			m.filteredCommands = nil
			m.selectedCommandIndex = 0
		}
		return m, nil
	case tea.KeyEsc:
		m.agentState = StateIdle
		m.filteredCommands = nil
		m.selectedCommandIndex = 0
		return m, nil
	default:
		m.textarea, cmd = m.textarea.Update(msg)

		m.showClearHint = false

		lines := strings.Count(m.textarea.Value(), "\n") + 1
		if lines < 1 {
			lines = 1
		} else if lines > 6 {
			lines = 6
		}
		if m.textarea.Height() != lines {
			m.textarea.SetHeight(lines)
		}

		input := m.textarea.Value()
		if strings.HasPrefix(input, "/") {
			m.filteredCommands = filterCommands(input)
			m.selectedCommandIndex = 0
			if len(m.filteredCommands) == 0 {
				m.agentState = StateIdle
			}
		} else {
			m.agentState = StateIdle
			m.filteredCommands = nil
			m.selectedCommandIndex = 0
		}
		return m, cmd
	}
}

// showToolWidget displays a tool execution widget with title and details.
// Deprecated: use m.showToolMessage() instead.
func showToolWidget(m *TestUIModel, title string, details string) {
	m.showToolMessage(title, details)
	m.updateViewport()
}

// handleProcessToolCalls executes tool calls received from the agent.
func handleProcessToolCalls(m *TestUIModel, _ processToolCallsMsg) (tea.Model, tea.Cmd) {
	if len(m.streamedToolCalls) > 0 {
		for _, toolCall := range m.streamedToolCalls {
			if toolCall.Name == "get_endpoints_details" {
				m.streamedToolCalls = nil
				m.currentTestToolID = toolCall.ID
				m.currentTestToolName = "get_endpoints_details"
				m.agentState = StateThinking

				if endpointsArg, ok := toolCall.Arguments["endpoints"].([]any); ok {
					var endpointsList []string
					for _, ep := range endpointsArg {
						if epMap, ok := ep.(map[string]any); ok {
							method, _ := epMap["method"].(string)
							path, _ := epMap["path"].(string)
							endpointsList = append(endpointsList, fmt.Sprintf("%s %s", method, path))
						}
					}
					details := strings.Join(endpointsList, ", ")
					showToolWidget(m, "Getting endpoint details", details)
				}

				return m, m.executeTool(toolCall)
			}
		}

		for _, toolCall := range m.streamedToolCalls {
			if toolCall.Name == "GenerateTestPlan" {
				m.streamedToolCalls = nil

				what, ok := toolCall.Arguments["what"].(string)
				if !ok || what == "" {
					m.addMessage(m.subtleStyle.Render("⚠️  GenerateTestPlan missing 'what' parameter"))
					return m, nil
				}

				focus, ok := toolCall.Arguments["focus"].(string)
				if !ok || focus == "" {
					focus = "happy path"
				}

				m.currentTestToolID = toolCall.ID
				m.currentTestToolName = "GenerateTestPlan"

				m.agentState = StateUsingTool
				m.animationFrame = 0
				m.spinner.Style = lipgloss.NewStyle().Foreground(Theme.Primary)
				return m, tea.Batch(
					animationTick(),
					func() tea.Msg {
						if m.localAgent == nil {
							var err error
							m.localAgent, err = agent.NewAgent(m.baseURL)
							if err != nil {
								return backendErrorMsg{err: fmt.Errorf("failed to initialize agent: %w", err)}
							}
						}

						tests, _, err := m.localAgent.GenerateTestPlan(what, focus)
						if err != nil {
							return backendErrorMsg{err: fmt.Errorf("failed to generate test plan: %w", err)}
						}
						return generateTestPlanResultMsg{
							what:         what,
							focus:        focus,
							backendTests: tests,
						}
					},
				)
			}
		}

		for _, toolCall := range m.streamedToolCalls {
			if toolCall.Name == "ExecuteTestGroup" {
				m.streamedToolCalls = nil

				m.currentTestToolID = toolCall.ID
				m.currentTestToolName = "ExecuteTestGroup"

				m.agentState = StateProcessing
				return m, m.executeTool(toolCall)
			}
		}

		for _, toolCall := range m.streamedToolCalls {
			if toolCall.Name == "ExportTests" {
				m.streamedToolCalls = nil

				m.currentTestToolID = toolCall.ID
				m.currentTestToolName = "ExportTests"

				format, _ := toolCall.Arguments["format"].(string)
				formatLabel := map[string]string{
					"postman": "Postman Collection",
					"pytest":  "pytest tests",
					"sh":      "curl script",
				}
				label := formatLabel[format]
				if label == "" {
					label = format
				}

				showToolWidget(m, "Exporting tests", label)
				m.agentState = StateUsingTool
				m.animationFrame = 0
				m.spinner.Style = lipgloss.NewStyle().Foreground(Theme.Primary)
				return m, tea.Batch(animationTick(), m.executeTool(toolCall))
			}
		}

		for _, toolCall := range m.streamedToolCalls {
			if toolCall.Name == "GenerateReport" {
				m.streamedToolCalls = nil

				m.currentTestToolID = toolCall.ID
				m.currentTestToolName = "GenerateReport"

				showToolWidget(m, "Generating PDF report", "")
				m.agentState = StateUsingTool
				m.animationFrame = 0
				m.spinner.Style = lipgloss.NewStyle().Foreground(Theme.Primary)
				return m, tea.Batch(animationTick(), m.executeTool(toolCall))
			}
		}

		m.agentState = StateIdle
	}

	m.agentState = StateIdle
	return m, nil
}

// handleShowTestSelection displays the test selection UI with generated test cases.
func handleShowTestSelection(m *TestUIModel, msg showTestSelectionMsg) (tea.Model, tea.Cmd) {
	m.tests = make([]Test, 0, len(msg.tests))
	for i, testMap := range msg.tests {
		method, _ := testMap["method"].(string)
		endpoint, _ := testMap["endpoint"].(string)

		requiresAuth := false
		if ra, ok := testMap["requires_auth"].(bool); ok {
			requiresAuth = ra
		}

		headers := make(map[string]string)
		if h, ok := testMap["headers"].(map[string]string); ok {
			headers = h
		}

		testCase := &agent.TestCase{
			Method:       method,
			Endpoint:     endpoint,
			Headers:      headers,
			Body:         testMap["body"],
			RequiresAuth: requiresAuth,
		}

		m.tests = append(m.tests, Test{
			ID:          i + 1,
			Method:      method,
			Endpoint:    endpoint,
			Description: fmt.Sprintf("%s %s", method, endpoint),
			Status:      "pending",
			Selected:    true,
			BackendTest: testCase,
		})
	}

	m.pendingTestGroupToolCall = &msg.toolCall

	m.selectedTestIndex = 0
	m.agentState = StateShowingTestPlan

	return m, nil
}

func handleModelSelectorState(m *TestUIModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modelSelector == nil {
		m.agentState = StateIdle
		return m, nil
	}

	done, selectedModel := m.modelSelector.HandleKey(msg)

	if done {
		if selectedModel != "" {
			cfg, err := config.Load()
			if err != nil {
				m.addMessage(m.errorStyle.Render("Failed to load config: " + err.Error()))
			} else {
				cfg.Model = selectedModel
				if err := cfg.Save(); err != nil {
					m.addMessage(m.errorStyle.Render("Failed to save config: " + err.Error()))
				} else {
					m.modelName = selectedModel
					m.addMessage(m.successStyle.Render("✓ Model changed to: " + selectedModel))
				}
			}
			m.addMessage("")
		}
		m.modelSelector = nil
		m.agentState = StateIdle
		m.lastMessageRole = "assistant"
	}

	return m, nil
}
