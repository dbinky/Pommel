# Pommel

Local-first semantic code search for AI coding agents.

<!-- Badges placeholder -->
<!-- ![Build Status](https://img.shields.io/github/actions/workflow/status/owner/pommel/ci.yml?branch=main) -->
<!-- ![License](https://img.shields.io/github/license/owner/pommel) -->
<!-- ![Go Version](https://img.shields.io/github/go-mod/go-version/owner/pommel) -->

Pommel maintains a vector database of your code, enabling fast semantic search without loading files into context. Designed to complement AI coding assistants by providing targeted code discovery.

## Features

- **Semantic code search** - Find code by meaning, not just keywords. Search for "rate limiting logic" and find relevant implementations regardless of naming conventions.
- **Always-fresh file watching** - Automatic file system monitoring keeps your index synchronized with code changes. No manual reindexing required.
- **Multi-level chunks** - Search at file, class/module, or method/function granularity for precise results.
- **Low latency local embeddings** - All processing happens locally via Ollama with Jina Code Embeddings v2 (768-dim vectors).
- **JSON output for agents** - All commands support `--json` flag for structured output, optimized for AI agent consumption.

## Installation

### Prerequisites

**Ollama** - Local LLM runtime for generating embeddings:

```bash
# macOS
brew install ollama

# Linux
curl -fsSL https://ollama.com/install.sh | sh
```

**Embedding Model** - Pull the Jina code embeddings model:

```bash
ollama pull unclemusclez/jina-embeddings-v2-base-code
```

**Go 1.21+** - Required for building from source:

```bash
# macOS
brew install go

# Linux (Ubuntu/Debian)
sudo apt install golang-go
```

### Install Pommel

```bash
# From source
go install github.com/pommel-dev/pommel/cmd/pm@latest
go install github.com/pommel-dev/pommel/cmd/pommeld@latest

# Or build locally
git clone https://github.com/pommel-dev/pommel.git
cd pommel
make build
```

## Quick Start

```bash
# Navigate to your project
cd your-project

# Initialize Pommel
pm init

# Start the daemon (begins indexing automatically)
pm start

# Search for code semantically
pm search "user authentication"

# Check indexing status
pm status
```

## CLI Commands

### `pm init`

Initialize Pommel in the current directory. Creates `.pommel/` directory with configuration files.

```bash
pm init                    # Initialize with defaults
pm init --auto             # Auto-detect languages and configure
pm init --claude           # Also add usage instructions to CLAUDE.md
pm init --start            # Initialize and start daemon immediately
```

### `pm start` / `pm stop`

Control the Pommel daemon for the current project.

```bash
pm start                   # Start daemon in background
pm start --foreground      # Start in foreground (for debugging)
pm stop                    # Stop the running daemon
```

### `pm search <query>`

Semantic search across the codebase. Returns ranked results based on semantic similarity.

```bash
# Basic search
pm search "authentication middleware"

# Limit results
pm search "database connection" --limit 20

# Filter by chunk level
pm search "error handling" --level method
pm search "service classes" --level class

# Filter by path
pm search "api handler" --path src/api/

# JSON output (for agents)
pm search "user validation" --json --limit 5
```

**Options:**

| Flag | Short | Description |
|------|-------|-------------|
| `--limit` | `-n` | Maximum number of results (default: 10) |
| `--level` | `-l` | Chunk level filter: `file`, `class`, `method` |
| `--path` | `-p` | Path prefix filter |
| `--json` | `-j` | Output as JSON (agent-friendly) |

**Example JSON Output:**

```json
{
  "query": "user authentication",
  "results": [
    {
      "path": "src/auth/middleware.py",
      "name": "AuthMiddleware",
      "level": "class",
      "content": "class AuthMiddleware:\n    ...",
      "start_line": 15,
      "end_line": 45,
      "parent": "auth.middleware",
      "score": 0.89
    }
  ],
  "took_ms": 42
}
```

### `pm status`

Show daemon status and indexing statistics.

```bash
pm status                  # Human-readable output
pm status --json           # JSON output
```

**Example Output:**

```json
{
  "daemon": {
    "running": true,
    "pid": 12345,
    "uptime_seconds": 3600
  },
  "index": {
    "total_files": 342,
    "total_chunks": 4521,
    "last_indexed": "2025-01-15T10:30:00Z",
    "pending_changes": 0
  },
  "health": {
    "status": "healthy",
    "embedding_model": "loaded",
    "database": "connected"
  }
}
```

### `pm reindex`

Force a full re-index of the project. Useful after major refactors or if the index becomes corrupted.

```bash
pm reindex                 # Reindex all files
pm reindex --path src/     # Reindex specific path only
```

### `pm config`

View or modify project configuration.

```bash
pm config                  # Show current configuration
pm config get ollama_url   # Get specific setting
pm config set debounce_ms 1000  # Update setting
```

## Configuration

Configuration is stored in `.pommel/config.yaml`:

```yaml
# Ollama settings
ollama_url: http://localhost:11434
model: unclemusclez/jina-embeddings-v2-base-code

# Indexing settings
debounce_ms: 500           # Debounce delay for file watcher
max_file_size_kb: 1024     # Skip files larger than this

# Chunk levels to generate
chunk_levels:
  - file
  - class
  - method

# Languages to index (empty = auto-detect from files)
languages:
  - csharp
  - python
  - javascript
  - typescript

# Search defaults
search:
  default_limit: 10
  default_levels:
    - method
    - class

# Daemon settings
daemon:
  port: 7420
  host: "127.0.0.1"
```

## Ignoring Files

Create `.pommelignore` in your project root using gitignore syntax:

```gitignore
# Dependencies
node_modules/
vendor/
.venv/
packages/

# Build outputs
dist/
build/
bin/
obj/
*.min.js
*.min.css

# Generated files
*.generated.cs
*.g.cs
__pycache__/

# IDE and editor files
.idea/
.vscode/
*.swp

# Test fixtures
**/testdata/
**/fixtures/
```

Pommel also respects your existing `.gitignore` by default.

## AI Agent Integration

Pommel is designed specifically for AI coding agents. Add to your `CLAUDE.md` (or equivalent agent instructions):

```markdown
## Code Search

This project uses Pommel for semantic code search. Before reading multiple
files to find relevant code, use the `pm` CLI:

\`\`\`bash
# Find code related to a concept
pm search "rate limiting logic" --json --limit 5

# Find implementations of a pattern
pm search "retry with exponential backoff" --level method --json

# Search within a specific area
pm search "validation" --path "src/Api/" --json
\`\`\`

Use Pommel search results to identify specific files and line ranges,
then read only those targeted sections into context.
```

### Workflow Comparison

**Without Pommel:**
```
Agent needs to understand authentication...
  -> Reads src/Auth/ (15 files, 2000 lines)
  -> Reads src/Middleware/ (8 files)
  -> Reads src/Services/ (12 files)
  -> Context window significantly consumed
```

**With Pommel:**
```
Agent needs to understand authentication...
  -> pm search "authentication flow" --json --limit 5
  -> Receives 5 targeted results with file:line references
  -> Reads only the 3 most relevant sections
  -> Context window minimally impacted
```

## Supported Languages

| Language | Extensions | Chunk Levels |
|----------|------------|--------------|
| C# | `.cs` | file, class, method, property |
| Python | `.py` | file, class, method/function |
| JavaScript | `.js`, `.jsx`, `.mjs` | file, class, function |
| TypeScript | `.ts`, `.tsx` | file, class/interface, function |

Additional language support (Go, Java, Rust) planned for future releases.

## Troubleshooting

### Cannot connect to Ollama

**Symptom:** Error message about Ollama connection failure.

**Solution:**
```bash
# Check if Ollama is running
ollama list

# Start Ollama if needed
ollama serve
```

### Model not found

**Symptom:** Error about embedding model not being installed.

**Solution:**
```bash
# Install the embedding model
ollama pull unclemusclez/jina-embeddings-v2-base-code

# Verify it's installed
ollama list
```

### Daemon not starting

**Symptom:** `pm start` fails or daemon exits immediately.

**Solution:**
```bash
# Check for existing daemon process
pm status

# Check daemon logs
cat .pommel/daemon.log

# Try running in foreground to see errors
pm start --foreground
```

### Slow initial indexing

**Symptom:** First-time indexing takes very long.

**Explanation:** Initial indexing requires generating embeddings for all files, which can take 2-5 minutes for ~1000 files. Subsequent updates are incremental and much faster (< 500ms per file).

**Tips:**
- Add large generated directories to `.pommelignore`
- Exclude test fixtures and vendor dependencies
- Run initial indexing when you can let it complete

### Search returns no results

**Symptom:** Searches return empty results even for known code.

**Solution:**
```bash
# Check if daemon is running and indexing complete
pm status --json

# Wait for pending_changes to reach 0
# Then retry your search

# If needed, force a reindex
pm reindex
```

### Database corruption

**Symptom:** Errors about database or corrupted index.

**Solution:**
```bash
# Stop the daemon
pm stop

# Remove the index (will be rebuilt)
rm .pommel/index.db

# Restart and reindex
pm start
pm reindex
```

## Performance

| Operation | Typical Time |
|-----------|--------------|
| Initial indexing (1000 files) | 2-5 minutes |
| File change re-index | < 500ms |
| Search query (10 results) | < 100ms |
| Daemon memory usage | ~50-100 MB |

## Architecture

```
AI Agent / Developer
        |
        v
    Pommel CLI (pm)  ------> search, status, config
        |
        v
    Pommel Daemon (pommeld)
    - File watcher (debounced)
    - Multi-level chunker (Tree-sitter)
    - Embedding generator (Ollama)
        |
        v
    sqlite-vec (local vector DB)
        ^
        |
    Jina Code Embeddings v2
    (768-dim, via Ollama)
```

## Development

### Dogfooding

Pommel includes a dogfooding script that tests the system on its own codebase. This validates search quality and performance with real Go code.

```bash
# Run dogfooding tests (requires Ollama running)
./scripts/dogfood.sh

# Output results as JSON
./scripts/dogfood.sh --json

# Save results to file
./scripts/dogfood.sh --results-file results.json --json

# Keep .pommel directory after run (for debugging)
./scripts/dogfood.sh --skip-cleanup
```

**Prerequisites:**
- Ollama running locally (`ollama serve`)
- Embedding model installed (`ollama pull unclemusclez/jina-embeddings-v2-base-code`)

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | All tests passed |
| 1 | Build failed |
| 2 | Ollama not available (skipped gracefully) |
| 3 | Daemon failed to start |
| 4 | Search tests failed |

The script cleans up the `.pommel` directory after each run unless `--skip-cleanup` is specified. Results are documented in `docs/dogfood-results.md`.

## License

MIT
