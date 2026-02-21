# CLAUDE.md — Claude-Specific Instructions for Octrafic CLI

> This file extends [AGENTS.md](./AGENTS.md) with Claude-specific instructions.

## Model Context

You are assisting with **Octrafic**, a Go CLI for AI-powered API testing. Before making changes, read `AGENTS.md` for full project context.

## Tool Use Preferences

When working with this codebase, use the following tools in order of preference:

1. **Search first** — use `grep` or code search before reading entire files
2. **Read outlines** — understand file structure before diving into implementations
3. **Edit surgically** — make minimal, targeted changes rather than rewriting whole files
4. **Verify immediately** — run `./check.sh` after any code change to catch lint/test issues early

## Code Generation Guidelines

- **BubbleTea patterns:** This project uses the Elm Architecture (Model-Update-View). All state mutations happen in `Update()` handlers. Never modify model state outside of an `Update` handler.
- **Tool call dispatch:** When adding a new agent tool, you must update three locations:
  1. `internal/agents/chat.go` — add the tool definition in `getMainAgentTools()`
  2. `internal/cli/update.go` — add a `case` in the `switch toolCall.Name` block inside `handleProcessToolCalls()`
  3. `internal/cli/handlers.go` — add the tool's execution handler
- **LLM providers:** Both Claude and OpenAI clients live in `internal/llm/`. They share a common interface. If modifying one, check if the other needs the same change.
- **Error handling:** Always wrap errors with context: `fmt.Errorf("failed to X: %w", err)`

## Response Format

- Use Polish when the user writes in Polish, English otherwise
- Keep explanations concise — the user is an experienced developer
- When proposing changes, show the diff or specific lines rather than full file rewrites
- Always mention which files were modified and why

## Testing Strategy

- Run `./check.sh` before every commit — it covers lint, tests, and build
- For TUI changes, test in headless mode: `./octrafic-cli test -s spec.json -u URL --auto --prompt "..."`
- The project currently has tests in: `config`, `core/auth`, `core/converter`, `core/parser`, `infra/storage`, `llm`, `updater`
