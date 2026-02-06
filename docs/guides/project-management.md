# Project Management

Projects in Octrafic are primarily managed through an interactive TUI wizard. CLI flags available for automation.

## Project Types

### Named Projects
Persistent projects saved to `~/.octrafic/projects/`. Default for TUI wizard.

### Temporary Projects
Stored in `/tmp/octrafic-projects/`. CLI only (omit `-n` flag).

## Creating Projects

### TUI Wizard (Primary Method)
```bash
octrafic
# Select "Create new project"
# Steps: API URL → Spec Path → Project Name → Confirm
# Navigation: Enter (next) • Esc (back) • Ctrl+C (cancel)
```

Wizard validates:
- API URL required
- Spec file exists
- Project name required

### Supported Formats

**Native:** OpenAPI/Swagger, Postman Collection, GraphQL Schema

**All other formats** (RAML, HAR, plain text, markdown, etc.) are automatically converted to OpenAPI using LLM. Wizard detects format by content and asks for confirmation before conversion.

### CLI
```bash
# Named project
octrafic -u https://api.example.com -s spec.json -n "My Project"

# Temporary project
octrafic -u https://api.example.com -s spec.json
```

## Loading Projects

### TUI List
```bash
octrafic
# ▶ Create new project
#   Stripe API
#   GitHub API
#
# / search • ↑↓ or jk navigate • enter select • q quit
```

### CLI
```bash
octrafic -n "Project Name"
```

## Updating Projects

```bash
octrafic -u https://api.example.com -s new-spec.yaml -n "Production API"
# Prompts for confirmation if project exists
```

## Storage

**Named projects:** `~/.octrafic/projects/{project-uuid}/`
**Temporary projects:** `/tmp/octrafic-projects/{project-uuid}/`

Each project directory contains:
- `project.json` - Metadata (URL, spec path, auth, timestamps)
- `endpoints.json` - Cached parsed endpoints
- `spec.hash` - Spec file hash for cache invalidation
