# Providers

Octrafic supports cloud and local AI providers. Choose one during first launch or switch anytime with `octrafic --onboarding`.

## Cloud

| Provider | Get API key |
|----------|-------------|
| **Claude (Anthropic)** | [console.anthropic.com](https://console.anthropic.com) |
| **OpenRouter** | [openrouter.ai/keys](https://openrouter.ai/keys) |
| **OpenAI** | [platform.openai.com](https://platform.openai.com) |

## Local

No API key needed. Just run a model server locally.

| Provider | Default URL | Get started |
|----------|-------------|-------------|
| **Ollama** | `localhost:11434` | [ollama.com](https://ollama.com) |
| **llama.cpp** | `localhost:8080` | [github.com/ggml-org/llama.cpp](https://github.com/ggml-org/llama.cpp) |

Quick start with Ollama:

```bash
ollama pull llama3.1
octrafic --onboarding   # select Ollama
```

## Configuration

Settings are stored in `~/.octrafic/config.json`. To switch provider, run `octrafic --onboarding`.
