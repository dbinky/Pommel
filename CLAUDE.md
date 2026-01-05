# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Pommel is a local-first semantic code search system designed to reduce context window consumption for AI coding agents. It maintains an always-current vector database of code embeddings, enabling targeted semantic searches instead of reading numerous files into context.

**Status:** v0.7.0 - Multiple embedding providers and intelligent chunk splitting

## Code Search Priority

**IMPORTANT: Use `pm search` BEFORE using Grep/Glob for code exploration.**

When looking for:
- How something is implemented → `pm search "authentication flow"`
- Where a pattern is used → `pm search "error handling"`
- Related code/concepts → `pm search "database connection"`
- Code that does X → `pm search "validate user input"`

Only fall back to Grep/Glob when:
- Searching for an exact string literal (e.g., a specific error message)
- Looking for a specific identifier name you already know
- Pommel daemon is not running

Example workflow:
```bash
# First: semantic search to find relevant code
pm search "retry logic with backoff" --limit 5

# Then: read the specific files/lines returned
# Only use grep if you need exact string matching
```

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
| Language | Go 1.21+ |
| Platform Support | macOS (Intel/ARM), Linux (x64/ARM64), Windows (x64/ARM64) |
| CLI Framework | Cobra + Viper |
| Vector Database | SQLite + sqlite-vec |
| Embedding Model | jina-embeddings-v2-base-code (via Ollama) |
| Code Parsing | Tree-sitter (33 languages - see Supported Languages below) |
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

**Important:** Always include `-tags fts5` to enable FTS5 full-text search support.

```bash
# Using make (recommended - includes all required tags)
make build

# Or manually with required tags
go build -tags fts5 -o pm ./cmd/pm
go build -tags fts5 -o pommeld ./cmd/pommeld
```

### Testing

**Important:** Always include `-tags fts5` to enable FTS5 support in tests.

```bash
# Using make (recommended)
make test

# Or manually with required tags
go test -tags fts5 ./...                    # Run all tests
go test -tags fts5 ./internal/cli/...       # Run CLI tests
go test -tags fts5 -v -run TestInitCmd      # Run specific test
```

### Installation

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.ps1 | iex
```

**Manual (from source):**
```bash
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

## Supported Languages

Pommel provides AST-aware code chunking for 33 programming languages:

**Systems Languages:** C, C++, Rust, Go
**JVM Languages:** Java, Kotlin, Scala, Groovy
**Web Languages:** JavaScript, TypeScript, JSX, TSX, HTML, CSS, Svelte
**Scripting Languages:** Python, Ruby, Lua, Bash, Elixir, PHP
**Functional Languages:** Elm, OCaml
**Mobile Languages:** Swift, Kotlin
**Infrastructure Languages:** HCL (Terraform), Dockerfile, SQL, Protocol Buffers, CUE, TOML, YAML
**Documentation Languages:** Markdown

Language configurations are stored in `languages/*.yaml` and define:
- File extensions to detect
- Tree-sitter grammar mapping
- Chunk type mappings (class, method, block)
- Documentation comment extraction rules

To add a new language, create a YAML config and run `go run internal/chunker/generate/main.go`.

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

## Embedding Provider Configuration

Pommel supports multiple embedding providers. Check configuration status:

```bash
pm status
```

If embeddings are not configured, you'll see:
```
⚠️  No embedding provider configured
   Run 'pm config provider' to set up embeddings.
```

### Quick Setup

```bash
# Interactive setup (recommended)
pm config provider

# Or set directly
pm config provider ollama                          # Local Ollama (default)
pm config provider openai --api-key sk-your-key    # OpenAI API
pm config provider voyage --api-key pa-your-key    # Voyage AI (code-specialized)
pm config provider ollama-remote --url http://host:11434  # Remote Ollama
```

### Provider-Specific Notes

| Provider | Requirements | Notes |
|----------|-------------|-------|
| Local Ollama | `ollama serve` running | Default, free, private |
| Remote Ollama | Accessible URL | Offload to server/NAS |
| OpenAI | Valid API key | `OPENAI_API_KEY` env var supported |
| Voyage AI | Valid API key | `VOYAGE_API_KEY` env var supported |

### Troubleshooting

**"No embedding provider configured"**
```bash
pm config provider   # Run interactive setup
```

**"Cannot connect to Ollama"**
```bash
ollama serve         # Start Ollama if not running
```

**"Invalid API key"**
- Verify key is correct and active
- Check for typos or trailing whitespace
- Ensure billing is enabled (for paid APIs)
