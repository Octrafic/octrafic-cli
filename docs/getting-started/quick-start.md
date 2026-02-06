# Quick Start

This tutorial will help you get started with Octrafic CLI as quickly as possible.

## Step 1: Install Octrafic

Choose the installation method for your operating system:

### Linux/macOS

```bash
curl -fsSL https://octrafic.com/install.sh | sh
```

### Windows

```powershell
iex (irm https://octrafic.com/install.ps1)
```

### Using Go

```bash
go install github.com/yourusername/octrafic@latest
```

## Step 2: First launch setup

When you start Octrafic for the first time, you'll go through a quick setup:

1. **Choose your AI provider**
   - Anthropic Claude (recommended)
   - OpenRouter (access to multiple models)

2. **Enter your API key**
   - Get Claude API key: [console.anthropic.com](https://console.anthropic.com)
   - Get OpenRouter API key: [openrouter.ai/keys](https://openrouter.ai/keys)

3. **Select a model**
   - Choose from available models for your provider

Your configuration is saved locally and you won't need to set it up again.

## Step 3: Start your first session

There are three ways to start Octrafic:

### Quick test (temporary session)

For one-time testing without saving:

```bash
octrafic -u https://jsonplaceholder.typicode.com -s api-spec.json
```

This creates a temporary session that won't be saved.

### Named project (saved for later)

To save a project for future use:

```bash
octrafic -u https://jsonplaceholder.typicode.com -s api-spec.json -n "My API Project"
```

The project is saved and you can easily return to it later:

```bash
octrafic -n "My API Project"
```

### Browse saved projects

To see all your saved projects:

```bash
octrafic
```

This opens an interactive list where you can:
- Navigate with arrow keys or `j`/`k`
- Search projects by pressing `/`
- Select a project with `Enter`
- Quit with `q`

## Step 4: Ask your first question

Now you can interact with your API using natural language. Try asking:

```
what endpoints does this API have?
```

Octrafic will analyze the OpenAPI spec and show you all available endpoints.

You can also ask more specific questions:

```
show me the users endpoint
what are the required fields for creating a post?
how do I authenticate requests?
```

## Step 5: Make your first API call

Test an endpoint directly:

```
get the first user
```

Octrafic will:
- Find the appropriate endpoint
- Show you the request details
- Execute the call
- Display the response

## Step 6: Explore and analyze

Ask Octrafic to analyze your API:

```
analyze the API structure
```

Or get specific information:

```
what authentication methods are supported?
what are the response formats?
are there any rate limits?
```

## Step 7: Generate test cases

Automatically generate test cases from your spec:

```
generate tests for the users endpoint
```

Octrafic will create comprehensive test scenarios covering:
- Happy path
- Edge cases
- Error handling
- Validation

## Essential commands

### Starting Octrafic

| Command | What it does | Example |
|---------|--------------|---------|
| `octrafic -u <url> -s <spec>` | Quick test (temporary) | `octrafic -u https://api.example.com -s spec.yaml` |
| `octrafic -u <url> -s <spec> -n "Name"` | Create named project | `octrafic -u https://api.example.com -s spec.yaml -n "Production API"` |
| `octrafic -n "Name"` | Load named project | `octrafic -n "Production API"` |
| `octrafic` | Browse saved projects | `octrafic` |

### Authentication flags

| Flag | What it does | Example |
|------|--------------|---------|
| `--auth bearer --token <token>` | Bearer authentication | `--auth bearer --token abc123` |
| `--auth apikey --key <name> --value <val>` | API key authentication | `--auth apikey --key X-API-Key --value abc123` |
| `--auth basic --user <u> --pass <p>` | Basic authentication | `--auth basic --user admin --pass secret` |
| `--save-auth` | Save auth with project (plain text) | `--save-auth` |
| `--clear-auth` | Remove saved auth from project | `--clear-auth` |

### In-session commands

| Command | What it does | Example |
|---------|--------------|---------|
| `/help` | Show available commands | `/help` |
| `exit` or `Ctrl+C` | Exit Octrafic | `exit` |

## Common workflows

### Testing a new API once

```bash
# Quick test without saving
octrafic -u https://test-api.com -s spec.json
```

### Working with a production API

```bash
# Save for future use
octrafic -u https://api.example.com -s prod-spec.yaml -n "Production API"

# Return to it later from anywhere
octrafic -n "Production API"
```

### Managing multiple APIs

```bash
# Save different APIs
octrafic -u https://api1.com -s spec1.json -n "Stripe API"
octrafic -u https://api2.com -s spec2.json -n "GitHub API"
octrafic -u https://api3.com -s spec3.json -n "Client API"

# Browse and select
octrafic  # Opens interactive list with search
```

### Updating a project

```bash
# Update with new spec or URL
octrafic -u https://api.example.com -s new-spec.yaml -n "Production API"

# You'll be prompted to confirm the update
```

### Saving authentication

```bash
# Save project with authentication
octrafic -u https://api.example.com -s spec.yaml -n "API" \
  --auth apikey --key X-API-Key --value secret123 --save-auth

# Warning will be shown about plain text storage

# Load project - auth is used automatically
octrafic -n "API"
# ✓ Using saved authentication (apikey)

# Override saved auth with different credentials
octrafic -n "API" --auth bearer --token newtoken

# Clear saved authentication
octrafic -n "API" --clear-auth
```

## Next steps

- Explore [Authentication Methods](../guides/authentication.md)
- Read about [Project Management](../guides/project-management.md)
