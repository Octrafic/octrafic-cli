# AGENTS.md — Octrafic CLI

## Project Overview

Octrafic is an open-source CLI tool for automated API testing and reporting, written in Go. Users describe what they want to test in natural language, and an LLM agent generates test plans, executes them against live endpoints, and exports results in multiple formats (Postman, Pytest, Curl). The CLI features an interactive TUI built with BubbleTea, supports multiple LLM providers (Claude, OpenAI, Ollama, Llama.cpp), and can run in headless mode for CI/CD pipelines.

**Stack:** Go 1.26 · BubbleTea (TUI) · LLM providers (Claude/OpenAI/Ollama) · GoReleaser · golangci-lint v2

---

## Architecture

```
cmd/octrafic/          → Entry point (main.go, root command)
internal/
├── agents/            → LLM agent orchestration (tool definitions, chat protocol, test planning)
├── cli/               → TUI layer (BubbleTea models, views, update loops, handlers)
│   ├── tui.go         → Main TestUIModel struct and View()
│   ├── update.go      → Core Update() loop and tool call dispatch
│   ├── handlers.go    → Tool execution handlers, sendChatMessage()
│   ├── update_tests.go→ Test-specific message handlers
│   ├── theme.go       → Color theme definitions
│   └── wizard.go      → Onboarding wizard flow
├── config/            → App configuration and provider settings
├── core/
│   ├── analyzer/      → OpenAPI spec analysis
│   ├── auth/          → Authentication strategies (Bearer, API Key, Basic)
│   ├── converter/     → JSON extraction and data conversion
│   ├── parser/        → Spec file parsing (OpenAPI, Postman, GraphQL, Markdown)
│   ├── reporter/      → PDF report generation
│   └── tester/        → HTTP test execution engine
├── exporter/          → Test export formats (Postman JSON, Pytest, Curl)
├── infra/
│   ├── logger/        → Structured logging
│   └── storage/       → Project and endpoint persistence
├── llm/               → LLM provider abstraction
│   ├── factory.go     → Provider factory (creates Claude/OpenAI/Ollama clients)
│   ├── claude/        → Anthropic Claude client implementation
│   ├── openai/        → OpenAI-compatible client implementation
│   └── common/        → Shared LLM types
├── runner/            → CLI command runner
├── ui/textarea/       → Custom textarea widget
└── updater/           → Self-update mechanism
```

**Data flow:** User input → `cli/update.go` (BubbleTea Update loop) → `agents/` (LLM orchestration) → `llm/` (provider API call) → tool calls dispatched via `cli/handlers.go` → `core/` modules execute → results rendered in `cli/tui.go`

**Key entry points:**
- `cmd/octrafic/main.go` — CLI bootstrap
- `internal/cli/tui.go` — TUI model definition (`TestUIModel`)
- `internal/cli/update.go` — Main message dispatch (`handleProcessToolCalls`)
- `internal/agents/chat.go` — Tool definitions and system prompt

---

## Commands

```bash
# Development
GOTOOLCHAIN=auto go build -o octrafic-cli ./cmd/octrafic   # Build binary
./octrafic-cli test -s spec.json -u https://api.example.com # Run with spec

# Full local CI (lint + tests + build)
./check.sh

# Individual checks
golangci-lint run           # Lint (requires golangci-lint v2)
go test -v ./...            # Unit tests
go vet ./...                # Go vet

# Headless mode (CI/CD)
./octrafic-cli test -s spec.json -u https://api.example.com --auto --prompt "your instructions"
```

> **Important:** Always set `GOTOOLCHAIN=auto` when building locally if your Go version is older than 1.26. The `check.sh` script handles this automatically.

---

## Code Style

### Naming
- **Packages:** lowercase, single word (`agents`, `exporter`, `storage`)
- **Files:** `snake_case.go` — e.g. `update_tests.go`, `file_picker.go`
- **Types:** `PascalCase` — e.g. `TestUIModel`, `ChatResponse`, `ExportRequest`
- **Functions:** `PascalCase` for exported, `camelCase` for unexported
- **Constants:** `PascalCase` — e.g. `StateIdle`, `ModeAutoExecute`
- **BubbleTea messages:** `camelCase` ending with `Msg` — e.g. `processToolCallsMsg`, `toolResultMsg`

### Formatting
- `gofmt` is enforced via golangci-lint
- Use `fmt.Fprintf(&builder, ...)` instead of `builder.WriteString(fmt.Sprintf(...))`
- Handle all errors explicitly — never use `_` for error returns unless the function genuinely cannot fail
- Group imports: stdlib → external packages → internal packages

### Forbidden
- ❌ `panic()` in production code
- ❌ Global mutable state
- ❌ `init()` functions for anything beyond registering exporters
- ❌ `WriteString(fmt.Sprintf(...))` — use `fmt.Fprintf` instead (enforced by staticcheck QF1012)

---

## Git Workflow

### Branches
```
main                              # Protected, production-ready
feature/short-description         # New features
fix/short-description             # Bug fixes
chore/short-description           # Maintenance, refactoring, tooling
```

### Commits — Conventional Commits
```
feat: add support for GraphQL specs
fix: resolve headless mode exit hang
chore: update golangci-lint to v2
docs: improve authentication guide
refactor: extract test runner into separate package
test: add parser edge case coverage
```

### Process
1. Branch from `main` — never commit directly to `main`
2. Run `./check.sh` before pushing — all lint, tests, and build must pass
3. Push branch → open Pull Request → CI validates on GitHub Actions
4. Merge via GitHub PR (squash or merge commit — no rebase required)

---

## Agent Boundaries

### ✅ Do freely
- Read any file in the repository
- Edit Go source code, markdown, and config files
- Run `./check.sh`, `go build`, `go test`, `golangci-lint run`
- Create new branches from `main`
- Create commits with conventional commit messages
- Run the CLI binary in headless mode for testing

### ⚠️ Ask first
- Pushing to remote (`git push`) — always confirm branch name
- Creating Pull Requests (`gh pr create`)
- Modifying `go.mod` or `go.sum` (dependency changes)
- Changing the CI pipeline (`.github/workflows/`)
- Modifying the `.goreleaser.yaml` release config
- Deleting files or branches

### ❌ Never do
- Push directly to `main`
- Run `go install` for system-wide tools without confirming
- Modify files outside the project directory
- Store secrets, API keys, or tokens in code
- Skip running `./check.sh` before creating a commit on a PR branch
