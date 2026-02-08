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
```bash
octrafic -u https://api.example.com -s spec.json \
  --auth bearer --token "your-token-here"
```

### API Key
```bash
octrafic -u https://api.example.com -s spec.json \
  --auth apikey --key X-API-Key --value "your-key-here"
```

### Basic Auth
```bash
octrafic -u https://api.example.com -s spec.json \
  --auth basic --user admin --pass secret123
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

## Priority System

When multiple auth sources are available:
1. **CLI flags** (highest priority)
2. **Saved project auth**
3. **No auth** (default)

Example:
```bash
# Project has saved apikey auth
octrafic -n "API" --auth bearer --token "xyz"  # Uses bearer (CLI override)
octrafic -n "API"                               # Uses saved apikey
```

## Related

- [Project Management](./project-management.md) - Managing projects
- [Getting Started](../getting-started/quick-start.md) - Quick start guide
