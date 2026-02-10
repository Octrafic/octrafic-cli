# Authentication

Octrafic supports multiple authentication methods for testing secured APIs.

## Security & Privacy

**IMPORTANT:** Your API keys, tokens, and passwords are **NEVER sent to the AI backend**.

How it works:
1. You provide credentials locally via CLI flags or saved project config
2. AI analyzes the API specification and conversation context
3. AI instructs when authentication should be added to a request
4. Your local CLI adds the actual credentials to the HTTP request
5. Request is sent directly to your API endpoint

**Privacy Guarantee:**
- AI sees: Authentication type and header names
- AI does NOT see: Actual tokens, passwords, or keys
- Your credentials stay on your machine

## Quick Start

### No Authentication
```bash
octrafic -u https://api.example.com -s spec.json --auth none
```

### Bearer Token
**Using CLI flags:**
```bash
octrafic -u https://api.example.com -s spec.json \
  --auth bearer --token "your-token-here"
```

**Using environment variables:**
```bash
export OCTRAFIC_AUTH_TYPE=bearer
export OCTRAFIC_AUTH_TOKEN=your-token-here
octrafic -u https://api.example.com -s spec.json
```

### API Key
**Using CLI flags:**
```bash
octrafic -u https://api.example.com -s spec.json \
  --auth apikey --key X-API-Key --value "your-key-here"
```

**Using environment variables:**
```bash
export OCTRAFIC_AUTH_TYPE=apikey
export OCTRAFIC_AUTH_KEY=X-API-Key
export OCTRAFIC_AUTH_VALUE=your-key-here
octrafic -u https://api.example.com -s spec.json
```

### Basic Auth
**Using CLI flags:**
```bash
octrafic -u https://api.example.com -s spec.json \
  --auth basic --user admin --pass secret123
```

**Using environment variables:**
```bash
export OCTRAFIC_AUTH_TYPE=basic
export OCTRAFIC_AUTH_USER=admin
export OCTRAFIC_AUTH_PASS=secret123
octrafic -u https://api.example.com -s spec.json
```

## Managing Authentication

### Override Auth
```bash
# Project has saved apikey, use different token temporarily
octrafic -n "My API" --auth bearer --token "different-token"
```

### Clear Saved Auth
```bash
octrafic -n "My API" --clear-auth
# ✓ Authentication cleared from project
```

## Environment Variables

For safer credential management, you can configure authentication via environment variables. This is especially useful for:
- CI/CD pipelines
- Containerized environments
- Avoiding credentials in shell history
- Team workflows with shared configurations

### Available Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `OCTRAFIC_AUTH_TYPE` | Auth type (`bearer`, `apikey`, `basic`, `none`) | `bearer` |
| `OCTRAFIC_AUTH_TOKEN` | Token for bearer authentication | `sk-abc123...` |
| `OCTRAFIC_AUTH_KEY` | Header name for API key authentication | `X-API-Key` |
| `OCTRAFIC_AUTH_VALUE` | Value for API key authentication | `key_abc123...` |
| `OCTRAFIC_AUTH_USER` | Username for basic authentication | `admin` |
| `OCTRAFIC_AUTH_PASS` | Password for basic authentication | `secret123` |

### Example Usage

```bash
# Set up bearer auth for a session
export OCTRAFIC_AUTH_TYPE=bearer
export OCTRAFIC_AUTH_TOKEN=your-token-here

# Now run without flags
octrafic -u https://api.example.com -s spec.json
octrafic -n "My API"  # Works with saved projects too
```

## Priority System

When multiple auth sources are available:
1. **CLI flags** (highest priority)
2. **Environment variables**
3. **Saved project auth**
4. **No auth** (default)

Example:
```bash
# Project has saved apikey auth
export OCTRAFIC_AUTH_TYPE=bearer
export OCTRAFIC_AUTH_TOKEN=env-token

octrafic -n "API" --auth basic --user admin --pass xyz  # Uses basic (CLI override)
octrafic -n "API"                                        # Uses bearer (env var)
unset OCTRAFIC_AUTH_TYPE OCTRAFIC_AUTH_TOKEN
octrafic -n "API"                                        # Uses saved apikey
```

## Related

- [Project Management](./project-management.md) - Managing projects
- [Getting Started](../getting-started/quick-start.md) - Quick start guide
