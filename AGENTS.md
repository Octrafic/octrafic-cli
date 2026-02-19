# AGENTS.md - Agent Coding Guidelines

This file provides guidelines for agentic coding agents working in this repository.

## Project Overview

Octrafic is a Go CLI application (Go 1.26+) that provides an interactive TUI for API testing using AI agents. It uses:
- **spf13/cobra** for CLI commands
- **charmbracelet/bubbletea** for TUI
- **anthropics/anthropic-sdk-go** for LLM integration
- **go.uber.org/zap** for structured logging

## Build, Lint, and Test Commands

### Build
```bash
go build -o octrafic ./cmd/octrafic
```

### Run Application
```bash
go run ./cmd/octrafic
```

### Run All Tests
```bash
go test ./...
```

### Run Single Test
```bash
go test -v ./internal/config        # Run tests in config package
go test -v -run TestShouldCheckForUpdate ./internal/config  # Run specific test
```

### Lint
```bash
golangci-lint run ./...
```

The project uses golangci-lint v2.9.0 with these enabled linters:
- errcheck
- govet
- ineffassign
- staticcheck
- unused

And formatter:
- gofmt

### Format Code
```bash
gofmt -w .
```

---

## Code Style Guidelines

### General Principles
- Follow standard Go conventions (Effective Go, Go Code Review Comments)
- Write clear, self-documenting code
- Keep functions focused and small
- Handle errors explicitly with helpful messages

### Naming Conventions
- **Files**: Use lowercase with underscores (e.g., `config.go`, `auth_test.go`)
- **Types/Interfaces**: PascalCase (e.g., `Config`, `Agent`)
- **Functions/Variables**: camelCase (e.g., `loadConfig`, `userInput`)
- **Constants**: PascalCase or SCREAMING_SNAKE_CASE for exported (e.g., `MaxRetries`)
- **Private functions**: camelCase starting with lowercase (e.g., `handleKey`)
- **Interfaces**: Add `er` suffix for interfaces (e.g., `Reader`, `Writer`)

### Package Structure
```
internal/
├── agents/       # Agent implementations
├── cli/          # CLI commands and TUI
├── config/       # Configuration management
├── core/         # Business logic (analyzer, auth, converter, parser, reporter, tester)
├── infra/        # Infrastructure (logger, storage)
├── llm/          # LLM provider implementations
└── updater/      # Auto-update functionality
```

### Import Organization
Group imports in this order with blank lines between groups:
1. Standard library
2. External packages (third-party)
3. Internal packages (project-local)

```go
import (
    "encoding/json"
    "fmt"
    "time"

    "github.com/anthropics/anthropic-sdk-go"
    "github.com/charmbracelet/bubbletea"

    "github.com/Octrafic/octrafic-cli/internal/agents"
    "github.com/Octrafic/octrafic-cli/internal/config"
)
```

### Error Handling
- Always handle errors explicitly
- Return meaningful error messages
- Use wrapped errors with `fmt.Errorf("...: %w", err)` for context
- Check errors where they occur, don't ignore with `_`

```go
// Good
if err != nil {
    return nil, fmt.Errorf("failed to load config: %w", err)
}

// Avoid
data, _ := os.ReadFile(path)  // Never ignore errors
```

### Type Definitions
- Use explicit struct types rather than generic maps where possible
- Use interfaces to define abstractions
- Document exported types with comments

```go
// Config holds the application configuration
type Config struct {
    Provider string `json:"provider"`
    APIKey   string `json:"api_key,omitempty"`
}
```

### Testing
- Tests should be in `*_test.go` files in the same package
- Use table-driven tests for multiple test cases
- Name test functions: `Test<Function>_<Scenario>`
- Use descriptive test names

```go
func TestIsLocalProvider(t *testing.T) {
    tests := []struct {
        provider string
        expected bool
    }{
        {"ollama", true},
        {"claude", false},
    }

    for _, tt := range tests {
        got := IsLocalProvider(tt.provider)
        if got != tt.expected {
            t.Errorf("IsLocalProvider(%q) = %v, want %v", tt.provider, got, tt.expected)
        }
    }
}
```

### Logging
- Use the project's logging infrastructure in `internal/infra/logger`
- Use structured logging with zap
- Include relevant context in log messages

```go
logger.Debug("Processing request", logger.String("id", requestID))
logger.Error("Failed to connect", logger.String("error", err.Error()))
```

### TUI Development
- Use Bubble Tea patterns (Model, Update, View)
- Handle tea.Msg with type switches
- Return tea.Model and tea.Cmd from Update
- Use tea.Batch for multiple commands

### Commits
Use conventional commits:
- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation
- `refactor:` code refactoring
- `test:` adding tests
- `chore:` maintenance

Example: `feat: add model selector to TUI`

---

## Key Files to Know

- `cmd/octrafic/main.go` - Entry point
- `internal/cli/tui.go` - Main TUI model and update loop
- `internal/config/config.go` - Configuration handling
- `internal/agents/` - Agent implementations
- `internal/infra/storage/` - Data persistence

---

## Common Tasks

### Running the CLI
```bash
go run ./cmd/octrafic [command] [flags]
```

### Adding a New Command
1. Add command to `internal/cli/` or create new file
2. Register in main.go with cobra
3. Add tests

### Adding a New LLM Provider
1. Implement provider interface in `internal/llm/`
2. Add factory registration in `internal/llm/factory.go`

---

## Dependencies

Go 1.26+ is required. Dependencies are managed via go modules:
```bash
go mod download
```
