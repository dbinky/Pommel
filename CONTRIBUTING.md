# Contributing to Pommel

Thank you for your interest in contributing to Pommel! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Branching Strategy](#branching-strategy)
- [Making Changes](#making-changes)
- [Pull Request Process](#pull-request-process)
- [Code Style](#code-style)
- [Testing](#testing)
- [Reporting Issues](#reporting-issues)

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment. Be kind, constructive, and professional in all interactions.

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally
3. Set up your development environment (see below)
4. Create a feature branch from `dev`
5. Make your changes
6. Submit a pull request to `dev`

## Development Setup

### Prerequisites

- Go 1.21 or later
- Ollama (for local embeddings)
- Make

### Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/Pommel.git
cd Pommel

# Add upstream remote
git remote add upstream https://github.com/dbinky/Pommel.git

# Install dependencies
go mod download

# Pull the embedding model
ollama pull unclemusclez/jina-embeddings-v2-base-code

# Build
make build

# Run tests
make test
```

## Branching Strategy

We use a two-branch workflow:

```
feature-branch → dev → main
```

- **`main`** - Stable releases only. Protected branch.
- **`dev`** - Integration branch for ongoing development. Protected branch.
- **Feature branches** - Your working branches, created from `dev`.

### Branch Naming

Use descriptive branch names:

- `feature/semantic-search-improvements`
- `fix/daemon-startup-crash`
- `docs/update-readme`
- `refactor/chunker-performance`

### Workflow

1. **Sync your fork** with upstream:
   ```bash
   git fetch upstream
   git checkout dev
   git merge upstream/dev
   ```

2. **Create a feature branch** from `dev`:
   ```bash
   git checkout -b feature/your-feature dev
   ```

3. **Make your changes** and commit:
   ```bash
   git add .
   git commit -m "feat: add your feature description"
   ```

4. **Push to your fork**:
   ```bash
   git push origin feature/your-feature
   ```

5. **Open a PR** targeting `dev` (not `main`)

## Making Changes

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks

Examples:
```
feat: add support for Ruby language parsing
fix: resolve daemon crash on empty config
docs: update installation instructions for Linux
refactor: simplify embedding cache logic
test: add integration tests for search API
```

### Keep Changes Focused

- One logical change per PR
- Avoid mixing features, fixes, and refactors
- Keep PRs reasonably sized (< 500 lines when possible)

## Pull Request Process

1. **Ensure tests pass** locally:
   ```bash
   make test
   go vet ./...
   ```

2. **Update documentation** if your change affects user-facing behavior

3. **Fill out the PR template** completely

4. **Request review** - A maintainer will review your PR

5. **Address feedback** - Make requested changes and push updates

6. **Merge** - Once approved, a maintainer will merge your PR

### PR Requirements

- All CI checks must pass
- At least one approving review
- No merge conflicts with `dev`
- Commits should be clean (squash if needed)

## Code Style

### Go Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use meaningful variable and function names
- Keep functions focused and reasonably sized
- Add comments for non-obvious logic
- Export only what needs to be public

### Project Conventions

- Error messages should be helpful and actionable
- CLI output should be concise and scannable
- JSON output should be consistent and well-structured
- Performance matters - this tool is called frequently by AI agents

### Formatting

```bash
# Format code
gofmt -w .

# Check for issues
go vet ./...
```

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/chunker/...

# Run with verbose output
go test -v ./...
```

### Writing Tests

- Write tests for new functionality
- Maintain or improve code coverage
- Use table-driven tests where appropriate
- Test edge cases and error conditions

### Integration Testing

For changes that affect the daemon or CLI:

```bash
# Build and test locally
make build
./bin/pm init
./bin/pm start
./bin/pm search "test query"
./bin/pm stop
```

## Reporting Issues

### Bug Reports

When reporting bugs, include:

- Pommel version (`pm --version`)
- Go version (`go version`)
- Operating system
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs or error messages

### Feature Requests

For feature requests, describe:

- The problem you're trying to solve
- Your proposed solution
- Alternative approaches you've considered
- How this benefits other users

### Security Issues

For security vulnerabilities, please email the maintainers directly rather than opening a public issue.

## Questions?

- Check existing issues and discussions
- Open a new discussion for questions
- Join the community chat (if available)

Thank you for contributing to Pommel!
