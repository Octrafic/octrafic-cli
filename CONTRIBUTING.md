# Contributing to Octrafic

Thanks for your interest in contributing!

## Ways to Contribute

- **Bug Reports** - Open an issue with clear reproduction steps and environment details
- **Feature Requests** - Describe the problem it solves and example use cases
- **Code Contributions** - See development setup below
- **Documentation** - Fix typos, add examples, improve clarity

## Development Setup

**Prerequisites:** Go 1.26+

```bash
# Clone and build
git clone https://github.com/Octrafic/octrafic-cli.git
cd octrafic-cli
go mod download
GOTOOLCHAIN=auto go build -o octrafic-cli ./cmd/octrafic

# Run full local CI (lint + tests + build)
./check.sh

# Or run checks individually
go test ./...
golangci-lint run
```

## Coding Guidelines

- Follow standard Go conventions
- Use `gofmt` and `golangci-lint` (v2)
- Write clear, self-documenting code
- Add tests for new functionality
- Handle errors explicitly with helpful messages

## Pull Request Process

1. Create a branch: `git checkout -b feat/your-feature`
2. Make changes and verify: `./check.sh` (runs lint, tests, and build)
3. Commit with clear messages: `feat: add support for X`
4. Push and open PR with description

## Commit Format

Use conventional commits:
- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation
- `refactor:` code refactoring
- `test:` adding tests

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
