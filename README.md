# Octrafic

[![Go Version](https://img.shields.io/github/go-mod/go-version/Octrafic/octrafic-cli)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Open source CLI tool for automated API testing and reporting. Test your APIs by describing what you want in natural language.

![Demo](./assets/demo.gif)

## Features

- **Natural language testing** - describe what you want to test in plain English; the agent generates and executes the test plan
- **Broad spec support** - OpenAPI 3.x, Swagger 2.0, Postman Collections, GraphQL, and Markdown docs
- **Multiple auth methods** - Bearer token, API Key, Basic Auth, or none
- **Export tests** - generate Postman collections, Python pytest files, or Bash curl scripts from any test session
- **PDF reports** - produce professional test reports with a single command (requires `weasyprint`)
- **Multiple LLM providers** - Anthropic Claude, OpenAI, Google Gemini, OpenRouter, Ollama, llama.cpp, or any OpenAI-compatible endpoint
- **Headless / CI mode** - run non-interactively with `octrafic test` for pipeline integration

## Install

**Linux & macOS:**
```bash
curl -fsSL https://octrafic.com/install.sh | bash
```

**macOS (Homebrew):**
```bash
brew install octrafic/tap/octrafic
```

**Windows:**
```powershell
iex (iwr -useb https://octrafic.com/install.ps1)
```

## Quick Start

```bash
octrafic
```

## Documentation

**Getting Started**
- [Introduction](https://docs.octrafic.com/getting-started/introduction)
- [Quick Start](https://docs.octrafic.com/getting-started/quick-start)

**Guides**
- [Chat Features](https://docs.octrafic.com/guides/chat-features)
- [Project Management](https://docs.octrafic.com/guides/project-management)
- [Providers](https://docs.octrafic.com/guides/providers)
- [Authentication](https://docs.octrafic.com/guides/authentication)
- [PDF Reports](https://docs.octrafic.com/guides/reports)
- [Exporting Tests](https://docs.octrafic.com/guides/exports)
- [Headless Mode](https://docs.octrafic.com/guides/headless)

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT - see [LICENSE](LICENSE)
