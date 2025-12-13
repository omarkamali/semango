# Contributing to Semango

Thank you for your interest in contributing to Semango! This document provides guidelines and information for contributors.

## Code of Conduct

Please be respectful and constructive in all interactions. We're building something together.

## Getting Started

### Prerequisites

- **Go 1.23+** with CGO enabled
- **Node.js 20+** and Yarn for UI development
- **System libraries**: OpenBLAS, FAISS (see build instructions)

### Development Setup

1. **Clone the repository**
   ```bash
   git clone https://github.com/omarkamali/semango.git
   cd semango
   ```

2. **Install Go dependencies**
   ```bash
   go mod download
   ```

3. **Install UI dependencies**
   ```bash
   cd ui && yarn install && cd ..
   ```

4. **Build and run**
   ```bash
   make build
   ./semango --help
   ```

### Development Commands

```bash
# Build everything (UI + Go binary)
make build

# Build Go binary only (faster iteration)
make build-no-ui

# Run tests
make test

# Start UI dev server (hot reload)
make dev-ui

# Clean build artifacts
make clean
```

## How to Contribute

### Reporting Bugs

1. Check existing issues to avoid duplicates
2. Use the bug report template
3. Include:
   - Semango version (`semango version`)
   - OS and architecture
   - Steps to reproduce
   - Expected vs actual behavior
   - Relevant logs

### Suggesting Features

1. Check existing issues/discussions
2. Describe the use case and problem you're solving
3. Propose a solution if you have one

### Pull Requests

1. **Fork and branch**: Create a feature branch from `main`
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make changes**: Follow the coding standards below

3. **Test**: Ensure tests pass
   ```bash
   go test ./...
   ```

4. **Commit**: Use conventional commit messages
   ```
   feat: add support for XLSX files
   fix: handle empty search results gracefully
   docs: update configuration reference
   ```

5. **Push and PR**: Open a pull request against `main`

## Coding Standards

### Go Code

- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Run `gofmt` and `goimports` before committing
- Add tests for new functionality
- Keep functions focused and well-documented
- Use meaningful variable names

### UI Code (React/TypeScript)

- Follow existing patterns in `ui/src/`
- Use TypeScript strict mode
- Run `yarn lint` before committing
- Use Tailwind CSS for styling

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `test:` - Test additions/changes
- `refactor:` - Code refactoring
- `chore:` - Maintenance tasks

## Project Structure

```
semango/
├── cmd/semango/      # CLI entry point
├── internal/         # Private packages
│   ├── api/          # HTTP server and handlers
│   ├── config/       # Configuration loading
│   ├── ingest/       # File crawling and processing
│   ├── pipeline/     # Indexing orchestration
│   ├── search/       # Query execution
│   ├── storage/      # FAISS, Bleve, metadata
│   └── util/         # Shared utilities
├── pkg/              # Public Go SDK (if any)
├── ui/               # React frontend
├── docs/             # Documentation
└── scripts/          # Build and dev scripts
```

## Testing

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Specific package
go test ./internal/search/...
```

### Writing Tests

- Place tests in `*_test.go` files alongside the code
- Use table-driven tests where appropriate
- Mock external dependencies (APIs, filesystems)

## Documentation

- Update README.md for user-facing changes
- Update docs/SEMANGO_GUIDE.md for configuration changes
- Add godoc comments for exported functions

## Release Process

Releases are automated via GitHub Actions when tags are pushed:

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Questions?

- Open a GitHub Discussion for general questions
- Open an Issue for bugs or feature requests
- Email: semango@omarkama.li

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
