package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *TestUIModel) updateFileSuggestions() {
	// Determine directory to look in and prefix to filter by
	var dir, prefix string
	searchText := m.fileFilterText

	// If text ends with separator, we are looking inside that directory with empty prefix
	if strings.HasSuffix(searchText, string(os.PathSeparator)) {
		dir = searchText
		prefix = ""
	} else {
		// Otherwise split into dir and prefix
		dir = filepath.Dir(searchText)
		prefix = filepath.Base(searchText)

		// filepath.Dir returns "." for "foo", which is correct
		// filepath.Dir returns "." for ".", which is correct

		// If input was empty string, Dir is "." and Base is ".", but we want prefix ""
		if searchText == "" {
			dir = "."
			prefix = ""
		}
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		m.filteredFiles = []string{}
		return
	}

	var suggestions []string
	searchPrefix := strings.ToLower(prefix)

	for _, f := range files {
		name := f.Name()
		if strings.HasPrefix(strings.ToLower(name), searchPrefix) {
			if f.IsDir() {
				name += string(os.PathSeparator)
			}
			suggestions = append(suggestions, name)
		}
	}
	sort.Strings(suggestions)
	m.filteredFiles = suggestions
	m.selectedFileIndex = 0
}

func (m *TestUIModel) renderFilePicker() string {
	if len(m.filteredFiles) == 0 {
		return ""
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Theme.Primary).
		Padding(0, 1). // Less padding than ModelSelector to keep it compact
		Width(60)

	titleStyle := lipgloss.NewStyle().
		Foreground(Theme.Primary).
		Bold(true)

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Select File"))
	b.WriteString("\n\n")

	// File list
	maxVisible := 5
	start := 0
	end := len(m.filteredFiles)

	if len(m.filteredFiles) > maxVisible {
		start = m.selectedFileIndex - maxVisible/2
		if start < 0 {
			start = 0
		}
		end = start + maxVisible
		if end > len(m.filteredFiles) {
			end = len(m.filteredFiles)
			start = end - maxVisible
			if start < 0 {
				start = 0
			}
		}
	}

	for i := start; i < end; i++ {
		file := m.filteredFiles[i]
		if i == m.selectedFileIndex {
			prefix := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render("▶")
			fileLine := prefix + " " + lipgloss.NewStyle().Foreground(Theme.Text).Bold(true).Render(file)
			b.WriteString(fileLine + "\n")
		} else {
			fileLine := "  " + lipgloss.NewStyle().Foreground(Theme.TextMuted).Render(file)
			b.WriteString(fileLine + "\n")
		}
	}

	// Help text
	b.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(Theme.TextSubtle)
	b.WriteString(helpStyle.Render("Type to search • ↑/↓ select • Enter confirm • ESC cancel"))

	// We return just the rendered string. The positioning is handled by the caller (Update/View).
	// However, since we are overlaying, we might want to ensure it has a specific width/height or context.
	return borderStyle.Render(b.String())
}

func handleFilePickerState(m *TestUIModel, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyEsc:
		m.showFilePicker = false
		m.fileFilterText = ""
		return m, nil, true
	case tea.KeyUp:
		if m.selectedFileIndex > 0 {
			m.selectedFileIndex--
		}
		return m, nil, true
	case tea.KeyDown:
		if m.selectedFileIndex < len(m.filteredFiles)-1 {
			m.selectedFileIndex++
		}
		return m, nil, true
	case tea.KeyEnter, tea.KeyTab:
		if len(m.filteredFiles) > 0 {
			selectedFile := m.filteredFiles[m.selectedFileIndex]

			// If it's a directory, append to filter and keep searching
			if strings.HasSuffix(selectedFile, string(os.PathSeparator)) {
				// We need to append the selected part to the current directory context
				var dir string
				if strings.HasSuffix(m.fileFilterText, string(os.PathSeparator)) {
					dir = m.fileFilterText
				} else {
					dir = filepath.Dir(m.fileFilterText)
					if dir == "." && !strings.Contains(m.fileFilterText, string(os.PathSeparator)) {
						dir = ""
					}
				}

				// Construct new path.
				// If dir was "", Join("", "foo/") -> "foo/"
				// If dir was "foo", Join("foo", "bar/") -> "foo/bar/"
				newPath := filepath.Join(dir, selectedFile)
				// filepath.Join calls Clean, which strips trailing slash. We need to add it back.
				if strings.HasSuffix(selectedFile, string(os.PathSeparator)) && !strings.HasSuffix(newPath, string(os.PathSeparator)) {
					newPath += string(os.PathSeparator)
				}

				m.fileFilterText = newPath
				m.updateFileSuggestions()

				// Also update textarea to match the directory selection so user sees it
				// This is tricky because we need to replace the partial path with selected path
				val := m.textarea.Value()
				lastAt := strings.LastIndex(val, "@")
				if lastAt != -1 {
					// m.fileFilterText is the full relative path, e.g. "subdir/"
					// We want to replace everything after @ with this path
					// And KEEP the @
					newVal := val[:lastAt] + "@" + newPath
					m.textarea.SetValue(newVal)
					m.textarea.CursorEnd()
				}

				return m, nil, true
			}

			// It's a file, complete and close
			// val := m.textarea.Value()
			// Find last occurrence of @
			val := m.textarea.Value()
			lastAt := strings.LastIndex(val, "@")

			if lastAt != -1 {
				// We need to resolve the full path relative to what was typed?
				// m.fileFilterText contained the path typed so far (e.g. "subdir/f")
				// selectedFile is just "file.txt" (Base name).
				// We want to replace "@subdir/f" with "subdir/file.txt "

				dir := filepath.Dir(m.fileFilterText)
				if dir == "." && !strings.Contains(m.fileFilterText, string(os.PathSeparator)) {
					dir = ""
				}

				fullPath := filepath.Join(dir, selectedFile)

				// valid replacement
				// val[:lastAt] excludes @. We want to keep it.
				newVal := val[:lastAt] + "@" + fullPath + " "
				m.textarea.SetValue(newVal)
				m.textarea.CursorEnd()
			}
			m.showFilePicker = false
			m.fileFilterText = ""
		}
		return m, nil, true
	case tea.KeyRunes:
		m.fileFilterText += msg.String()
		m.updateFileSuggestions()
		return m, nil, false
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.fileFilterText) > 0 {
			m.fileFilterText = m.fileFilterText[:len(m.fileFilterText)-1]
			m.updateFileSuggestions()
		} else {
			m.showFilePicker = false
		}
		return m, nil, false
	}
	return m, nil, false
}

// expandFileContent replaces @filepath tokens with:
// 1. Absolute path for display (UI history) - Highlighted
// 2. Absolute path for storage (DB/Agent Context) - Clean text
// The actual expansion to file content happens dynamically in sendChatMessage
func (m *TestUIModel) expandFileContent(input string) (string, string) {
	words := strings.Split(input, " ")
	displayWords := make([]string, len(words))
	storageWords := make([]string, len(words))
	copy(displayWords, words)
	copy(storageWords, words)

	for i, word := range words {
		if strings.HasPrefix(word, "@") {
			path := word[1:] // remove @

			// Check if file exists
			info, err := os.Stat(path)
			if err == nil && !info.IsDir() {
				absPath, err := filepath.Abs(path)
				if err == nil {
					styledPath := lipgloss.NewStyle().Foreground(Theme.Primary).Bold(true).Render("@" + absPath)
					displayWords[i] = styledPath

					storageWords[i] = "@" + absPath
				}
			}
		}
	}

	return strings.Join(displayWords, " "), strings.Join(storageWords, " ")
}
