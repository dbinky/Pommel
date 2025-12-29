# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Pommel is a local-first semantic code search system designed to reduce context window consumption for AI coding agents. It maintains an always-current vector database of code embeddings, enabling targeted semantic searches instead of reading numerous files into context.

**Status:** v0.1.0 - Fully functional with CLI, daemon, and semantic search

## Quick Start

```bash
# Initialize Pommel in a project
pm init --auto --claude --start

# Search the codebase semantically
pm search "database connection handling"
pm search "error handling patterns" --limit 5
pm search "CLI command setup" --json

# Check status
pm status

# Reindex after major changes
pm reindex
```

## Architecture

```
AI Agent / Developer
        │
        ▼
    Pommel CLI (pm)  ──────► search, status, init, start, stop, reindex, config
        │
        ▼
    Pommel Daemon (pommeld)
    ├── File watcher (fsnotify, debounced)
    ├── Tree-sitter chunker (AST-aware)
    └── Embedding generator (Ollama client)
        │
        ▼
    SQLite + sqlite-vec (local vector DB)
        ▲
        │
    Jina Code Embeddings via Ollama (768-dim vectors)
```

**Data flows:**
1. **Indexing:** File changes → Watcher → Chunker → Embedder → Vector DB
2. **Search:** Query → CLI → Daemon → Embedder → Vector DB → Ranked results

## CLI Commands

| Command | Description |
|---------|-------------|
| `pm init` | Initialize Pommel in project (flags: `--auto`, `--claude`, `--start`) |
| `pm search <query>` | Semantic search (flags: `--json`, `--limit`, `--level`, `--path`) |
| `pm status` | Show daemon status and index statistics |
| `pm start` | Start the daemon |
| `pm stop` | Stop the daemon |
| `pm reindex` | Force full reindex of the codebase |
| `pm config` | View/modify configuration |
| `pm version` | Show version information |

All commands support `--json` for structured output and `-p/--project` to specify project root.

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.24+ |
| CLI Framework | Cobra + Viper |
| Vector Database | SQLite + sqlite-vec |
| Embedding Model | jina-embeddings-v2-base-code (via Ollama) |
| Code Parsing | Tree-sitter (Go, Python, TypeScript, JavaScript, Java, C#, Rust, C, C++) |
| File Watching | fsnotify |
| HTTP Server | go-chi |

## Project Structure

```
pommel/
├── cmd/
│   ├── pm/              # CLI entry point
│   └── pommeld/         # Daemon entry point
├── internal/
│   ├── api/             # HTTP API types and handlers
│   ├── chunker/         # Tree-sitter based code chunking
│   ├── cli/             # Cobra command implementations
│   ├── config/          # YAML configuration loading/validation
│   ├── daemon/          # Daemon server, watcher, indexer
│   ├── db/              # SQLite + sqlite-vec database layer
│   ├── embedder/        # Ollama embedding client
│   ├── models/          # Shared data models
│   ├── search/          # Vector similarity search
│   └── setup/           # Dependency detection
├── scripts/
│   └── install.sh       # Installation script
├── docs/                # Documentation and plans
└── .pommel/             # Per-project data (gitignored)
    ├── config.yaml      # Project configuration
    └── pommel.db        # SQLite database with vectors
```

## Development

### Prerequisites

- Go 1.24+
- Ollama with `unclemusclez/jina-embeddings-v2-base-code` model

### Building

```bash
go build -o pm ./cmd/pm
go build -o pommeld ./cmd/pommeld
```

### Testing

```bash
go test ./...                    # Run all tests
go test ./internal/cli/...       # Run CLI tests
go test -v -run TestInitCmd      # Run specific test
```

### Installation

```bash
# Quick install
curl -fsSL https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.sh | bash

# Or manual
go install ./cmd/pm ./cmd/pommeld
ollama pull unclemusclez/jina-embeddings-v2-base-code
```

## Code Quality Principles

- **SOLID principles** - Single responsibility, open/closed, Liskov substitution, interface segregation, dependency inversion
- **DRY with domain sensitivity** - Eliminate duplication, but don't over-abstract across domain boundaries
- **Performance-critical** - Optimize for low latency on search operations (sub-second response times)
- **Fast indexing** - Minimize time between file save and searchability
- **Test-driven** - Write tests first for new features

## Key Packages

### `internal/cli`
Cobra command implementations. Each command has its own file (init.go, search.go, etc.) with a corresponding test file. Commands communicate with the daemon via HTTP client in `client.go`.

### `internal/daemon`
The pommeld server that watches files, indexes changes, and serves search requests. Key components:
- `daemon.go` - HTTP server and request routing
- `watcher.go` - fsnotify file watcher with debouncing
- `indexer.go` - Coordinates chunking and embedding
- `ignorer.go` - .pommelignore pattern matching

### `internal/chunker`
Tree-sitter based code chunking. Parses source files into AST and extracts meaningful chunks (files, classes, functions, blocks). Language-specific grammars in separate files.

### `internal/db`
SQLite database with sqlite-vec extension for vector storage. Handles chunk storage, vector indexing, and similarity search queries.

### `internal/embedder`
Embedding generation via Ollama's local API. Includes caching layer and mock implementation for testing.

## Related System

Pommel complements **Beads** (https://github.com/steveyegge/beads) for task/issue tracking:
- **Pommel:** "Where is this logic implemented?" (semantic code search)
- **Beads:** "What should I work on next?" (task memory and tracking)

## Pommel - Semantic Code Search

This project uses Pommel for semantic code search. Use the following commands to search the codebase efficiently:

### Quick Search Examples
```bash
# Find code by semantic meaning (not just keywords)
pm search "authentication logic"
pm search "error handling patterns"
pm search "database connection setup"

# Search with JSON output for programmatic use
pm search "user validation" --json

# Limit results
pm search "API endpoints" --limit 5

# Search specific chunk levels
pm search "class definitions" --level class
pm search "function implementations" --level function
```

### Available Commands
- `pm search <query>` - Semantic search across the codebase
- `pm status` - Check daemon status and index statistics
- `pm reindex` - Force a full reindex of the codebase

### Tips
- Use natural language queries - Pommel understands semantic meaning
- Keep the daemon running (`pm start`) for always-current search results
- Use `--json` flag when you need structured output for processing
