# Pommel

Local-first semantic code search for AI coding agents.

[![CI](https://github.com/dbinky/Pommel/actions/workflows/ci.yml/badge.svg)](https://github.com/dbinky/Pommel/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/dbinky/Pommel)](https://go.dev/)
[![License](https://img.shields.io/github/license/dbinky/Pommel)](LICENSE)

**v0.6.0** - Now with 33 language support via YAML-driven configuration!

Pommel maintains a vector database of your code, enabling fast semantic search without loading files into context. Designed to complement AI coding assistants by providing targeted code discovery.

## Features

- **Hybrid search** - Combines semantic vector search with keyword search (FTS5) using Reciprocal Rank Fusion for best-of-both-worlds results.
- **Intelligent re-ranking** - Heuristic signals boost results based on name matches, exact phrases, file paths, recency, and code structure.
- **Semantic code search** - Find code by meaning, not just keywords. Search for "rate limiting logic" and find relevant implementations regardless of naming conventions.
- **Always-fresh file watching** - Automatic file system monitoring keeps your index synchronized with code changes. No manual reindexing required.
- **Multi-level chunks** - Search at file, class/module, or method/function granularity for precise results.
- **Low latency local embeddings** - All processing happens locally via Ollama with Jina Code Embeddings v2 (768-dim vectors).
- **Context savings metrics** - See how much context window you're saving compared to grep-based approaches with `--metrics`.
- **JSON output for agents** - All commands support `--json` flag for structured output, optimized for AI agent consumption.

## Installation

### Quick Install (Recommended)

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.ps1 | iex
```

This will:
- Download pre-built binaries (or build from source on Unix)
- Install `pm` and `pommeld` to your PATH
- Install Ollama if not present
- Pull the embedding model (~300MB)

### Prerequisites

**Ollama** is required for generating embeddings. The install scripts handle this automatically, but you can install manually:

```bash
# macOS
brew install ollama

# Linux
curl -fsSL https://ollama.com/install.sh | sh
```

```powershell
# Windows (winget)
winget install Ollama.Ollama
```

### Manual Install

Download binaries from [releases](https://github.com/dbinky/Pommel/releases):

| Platform | Architecture | CLI | Daemon |
|----------|--------------|-----|--------|
| macOS | Intel | pm-darwin-amd64 | pommeld-darwin-amd64 |
| macOS | Apple Silicon | pm-darwin-arm64 | pommeld-darwin-arm64 |
| Linux | x64 | pm-linux-amd64 | pommeld-linux-amd64 |
| Windows | x64 | pm-windows-amd64.exe | pommeld-windows-amd64.exe |

Then pull the embedding model:
```bash
ollama pull unclemusclez/jina-embeddings-v2-base-code
```

### Building from Source

```bash
# Clone and build
git clone https://github.com/dbinky/Pommel.git
cd Pommel
make build

# Install to PATH (Unix)
cp bin/pm bin/pommeld ~/.local/bin/
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

Hybrid search across the codebase. Combines semantic vector search with keyword matching, then re-ranks results using code-aware heuristics.

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

# Verbose output with match reasons and score breakdown
pm search "rate limiting" --verbose

# Show context savings metrics
pm search "database queries" --metrics

# Disable hybrid search (vector-only)
pm search "config parsing" --no-hybrid

# Disable re-ranking stage
pm search "utility functions" --no-rerank
```

**Options:**

| Flag | Short | Description |
|------|-------|-------------|
| `--limit` | `-n` | Maximum number of results (default: 10) |
| `--level` | `-l` | Chunk level filter: `file`, `class`, `method` |
| `--path` | `-p` | Path prefix filter |
| `--json` | `-j` | Output as JSON (agent-friendly) |
| `--verbose` | `-v` | Show detailed match reasons and score breakdown |
| `--metrics` | | Show context savings vs grep baseline |
| `--no-hybrid` | | Disable hybrid search (vector-only mode) |
| `--no-rerank` | | Disable re-ranking stage |

**Example JSON Output:**

```json
{
  "query": "user authentication",
  "results": [
    {
      "id": "chunk-abc123",
      "file": "src/auth/middleware.py",
      "start_line": 15,
      "end_line": 45,
      "level": "class",
      "language": "python",
      "name": "AuthMiddleware",
      "score": 0.89,
      "content": "class AuthMiddleware:\n    ...",
      "match_source": "both",
      "match_reasons": ["semantic similarity", "keyword match via BM25", "contains 'auth' in name"],
      "score_details": {
        "vector_score": 0.85,
        "keyword_score": 0.72,
        "rrf_score": 0.89
      },
      "parent": {
        "id": "chunk-parent123",
        "name": "auth.middleware",
        "level": "file"
      }
    }
  ],
  "total_results": 1,
  "search_time_ms": 42,
  "hybrid_enabled": true,
  "rerank_enabled": true
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
pm config                              # Show current configuration
pm config get embedding.ollama_url     # Get specific setting
pm config set watcher.debounce_ms 1000 # Update setting
pm config set daemon.port 7421         # Change daemon port
pm config set search.default_levels method,class,file  # Set search levels
```

## Configuration

Configuration is stored in `.pommel/config.yaml`:

```yaml
version: 1

# Chunk levels to generate
chunk_levels:
  - method
  - class
  - file

# File patterns to include
include_patterns:
  - "**/*.cs"
  - "**/*.py"
  - "**/*.js"
  - "**/*.ts"
  - "**/*.jsx"
  - "**/*.tsx"

# File patterns to exclude
exclude_patterns:
  - "**/node_modules/**"
  - "**/bin/**"
  - "**/obj/**"
  - "**/__pycache__/**"
  - "**/.git/**"
  - "**/.pommel/**"

# File watcher settings
watcher:
  debounce_ms: 500           # Debounce delay for file changes
  max_file_size: 1048576     # Skip files larger than this (bytes)

# Daemon settings
daemon:
  host: "127.0.0.1"
  port: 7420
  log_level: "info"

# Embedding settings
embedding:
  model: "unclemusclez/jina-embeddings-v2-base-code"
  ollama_url: "http://localhost:11434"
  batch_size: 32
  cache_size: 1000

# Search defaults
search:
  default_limit: 10
  default_levels:
    - method
    - class

# Hybrid search settings (v0.5.0+)
hybrid_search:
  enabled: true              # Enable hybrid vector + keyword search
  rrf_k: 60                  # RRF constant (higher = more weight to lower ranks)
  vector_weight: 1.0         # Weight for vector search results
  keyword_weight: 1.0        # Weight for keyword search results

# Re-ranker settings (v0.5.0+)
reranker:
  enabled: true              # Enable heuristic re-ranking
  model: "heuristic"         # Re-ranking model (currently only "heuristic")
  timeout_ms: 100            # Timeout for re-ranking
  candidates: 50             # Number of candidates to re-rank
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

Pommel is designed specifically for AI coding agents. It provides ~18x token savings compared to traditional exploration.

### When to Use Pommel vs Explorer/Grep

**Use `pm search` FIRST for:**
- Finding specific implementations ("where is X implemented")
- Quick code lookups when you know what you're looking for
- Iterative exploration (multiple related searches)
- Cost/time-sensitive tasks

**Fall back to Explorer/Grep when:**
- Verifying something does NOT exist (Pommel may return false positives)
- Understanding architecture or code flow relationships
- Need full context around matches (not just snippets)
- Searching for exact string literals

**Decision rule:** Start with `pm search`. If results seem off-topic or you need to confirm absence, use Explorer.

### Use Case Reference

| Use Case                         | Recommended Tool          |
|----------------------------------|---------------------------|
| Quick code lookup                | Pommel                    |
| Understanding architecture       | Explorer                  |
| Finding specific implementations | Pommel                    |
| Verifying if feature exists      | Explorer                  |
| Iterative exploration            | Pommel                    |
| Comprehensive documentation      | Explorer                  |
| Cost-sensitive workflows         | Pommel (18x fewer tokens) |
| Time-sensitive tasks             | Pommel (1000x+ faster)    |

### CLAUDE.md Integration

Run `pm init --claude` to automatically add Pommel instructions to your project's `CLAUDE.md`. Or add manually:

```markdown
## Code Search

This project uses Pommel for semantic code search.

\`\`\`bash
# Find code related to a concept
pm search "rate limiting logic" --json --limit 5

# Find implementations of a pattern
pm search "retry with exponential backoff" --level method --json

# Search within a specific area
pm search "validation" --path "src/Api/" --json
\`\`\`

**Tip:** Low scores (< 0.5) suggest weak matches - use Explorer to confirm.
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

Pommel supports 33 programming languages with AST-aware chunking via Tree-sitter:

| Language | Extensions | Chunk Levels |
|----------|------------|--------------|
| Bash | `.sh`, `.bash`, `.bashrc`, `.zsh`, `.zshrc` | file, function |
| C | `.c`, `.h` | file, struct, function |
| C++ | `.cpp`, `.cc`, `.cxx`, `.hpp`, `.hh`, `.hxx`, `.h++` | file, class/struct, function/method |
| C# | `.cs` | file, class/struct/interface, method/property |
| CSS | `.css` | file, rule_set/media/keyframes |
| CUE | `.cue` | file, struct, field |
| Dockerfile | `.dockerfile` | file, instruction |
| Elixir | `.ex`, `.exs` | file, module, function |
| Elm | `.elm` | file, module/type, function |
| Go | `.go` | file, struct/interface, function/method |
| Groovy | `.groovy`, `.gradle` | file, class/interface, method |
| HCL | `.hcl`, `.tf`, `.tfvars` | file, block, attribute |
| HTML | `.html`, `.htm` | file, element |
| Java | `.java` | file, class/interface/enum, method |
| JavaScript | `.js`, `.mjs`, `.cjs` | file, class, function |
| JSX | `.jsx` | file, class, function |
| Kotlin | `.kt`, `.kts` | file, class/object, function |
| Lua | `.lua` | file, function |
| Markdown | `.md`, `.markdown`, `.mdx` | file, section/heading |
| OCaml | `.ml`, `.mli` | file, module/class, function |
| PHP | `.php`, `.php3`, `.php4`, `.php5`, `.phps`, `.phtml` | file, class/trait, method/function |
| Protocol Buffers | `.proto` | file, message/enum/service, field/rpc |
| Python | `.py`, `.pyi`, `.pyw` | file, class, method/function |
| Ruby | `.rb`, `.rake`, `.gemspec` | file, class/module, method |
| Rust | `.rs` | file, struct/enum/trait/impl, function |
| Scala | `.scala`, `.sc` | file, class/object/trait, function |
| SQL | `.sql` | file, table/view, function/procedure |
| Svelte | `.svelte` | file, script/style, element |
| Swift | `.swift` | file, class/struct/protocol, function |
| TOML | `.toml` | file, table |
| TSX | `.tsx` | file, class/interface, function |
| TypeScript | `.ts`, `.mts`, `.cts` | file, class/interface, function |
| YAML | `.yaml`, `.yml` | file, mapping |

Other file types are indexed at file-level only (fallback chunking).

**macOS Build Note:** Building YAML support requires C++ headers. Set `CGO_CXXFLAGS="-I$(xcrun --show-sdk-path)/usr/include/c++/v1"` if you encounter C++ header errors.

## Platform Notes

### Windows

- Pommel runs natively on Windows (no WSL required)
- PowerShell 5.1+ required for install script
- Ollama installed via winget (Windows 10 1709+)
- Data stored in project `.pommel/` directory
- Daemon runs as background process (not Windows Service)
- See [Windows Troubleshooting](docs/troubleshooting-windows.md) for Windows-specific issues

### macOS / Linux

- Standard Unix process management
- Install script uses curl + bash
- Daemon managed via PID files

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

# Remove the database (will be rebuilt)
rm .pommel/index.db

# Restart and reindex
pm start
pm reindex
```

## Using Pommel with Multiple Repositories

Pommel supports running across multiple unrelated repositories simultaneously. Each repository gets its own daemon instance with an automatically-assigned port.

### Setup for Multiple Repos

```bash
# Initialize Pommel in each repository
cd ~/repos/project-a
pm init --auto --start

cd ~/repos/project-b
pm init --auto --start

cd ~/repos/project-c
pm init --auto --start
```

Each project's daemon runs independently on its own port (calculated from a hash of the project path). The CLI automatically connects to the correct daemon based on your current directory.

### Managing Multiple Daemons

```bash
# Check status of current project's daemon
pm status

# Each project has independent state
cd ~/repos/project-a && pm status  # Shows project-a's index
cd ~/repos/project-b && pm status  # Shows project-b's index
```

### How It Works

- **Port assignment**: Each project gets a unique port based on a hash of its absolute path
- **Independent indexes**: Each `.pommel/` directory contains its own `index.db`
- **No conflicts**: Daemons don't interfere with each other
- **Auto-discovery**: The CLI finds the daemon by reading `.pommel/pommel.pid`

### Monorepo Support

For monorepos with multiple sub-projects (detected via markers like `package.json`, `go.mod`, etc.):

```bash
# Initialize with monorepo detection
cd ~/repos/monorepo
pm init --auto --monorepo --start

# Search defaults to current sub-project
cd ~/repos/monorepo/packages/frontend
pm search "component state"

# Search across entire monorepo
pm search "shared utilities" --all

# List detected sub-projects
pm subprojects
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
    ├── File watcher (debounced)
    ├── Multi-level chunker (Tree-sitter)
    ├── Embedding generator (Ollama)
    └── Search Pipeline:
        ├── Vector search (sqlite-vec)
        ├── Keyword search (FTS5)
        ├── RRF merge (k=60)
        └── Heuristic re-ranker
        |
        v
    SQLite Database
    ├── sqlite-vec (vector embeddings)
    └── FTS5 (full-text index)
        ^
        |
    Jina Code Embeddings v2
    (768-dim, via Ollama)
```

## How Search Works

Pommel uses a multi-stage search pipeline for optimal result quality:

### 1. Hybrid Retrieval
- **Vector Search**: Finds semantically similar code using embedding similarity
- **Keyword Search**: Finds exact keyword matches using SQLite FTS5 with BM25 scoring
- Results are merged using Reciprocal Rank Fusion (RRF) with k=60

### 2. Re-ranking
Heuristic signals boost results based on:
- **Name match**: Query terms appearing in function/class names
- **Exact phrase**: Complete query phrase found in content
- **Path match**: Query terms in file path
- **Recency**: Recently modified files get a small boost
- **Test penalty**: Test files ranked slightly lower (configurable)

### 3. Result Enrichment
Each result includes:
- `match_source`: Whether it matched via "vector", "keyword", or "both"
- `match_reasons`: Human-readable explanations of why it matched
- `score_details`: Breakdown of vector, keyword, and RRF scores

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

## Token Savings Benchmark

Real-world comparison of Pommel semantic search vs traditional code exploration (grep/glob/file reading). Tested on Pommel's own codebase (136 files, 2,111 chunks).

### Methodology

- **10 search queries**: 7 expected to find matches, 3 expected to find nothing
- **Pommel**: Single `pm search` call per query
- **Explorer Agent**: Autonomous agent using grep, glob, and file reads to find relevant code

### Results

| Query | Pommel Tokens | Explorer Tokens | Savings |
|-------|---------------|-----------------|---------|
| **Expected Matches** |
| hybrid search implementation | 157 | ~18,000 | 99.1% |
| file watcher debouncing | 1,471 | ~8,500 | 82.7% |
| tree-sitter code chunking | 227 | ~15,000 | 98.5% |
| vector similarity search | 789 | ~12,000 | 93.4% |
| CLI command parsing | 275 | ~14,000 | 98.0% |
| embedding generation with ollama | 545 | ~10,000 | 94.6% |
| database schema migrations | 169 | ~11,000 | 98.5% |
| **Expected Non-Matches** |
| credit card payment processing | 342 | ~6,500 | 94.7% |
| stripe integration webhooks | 342 | ~5,000 | 93.2% |
| kubernetes deployment config | 57 | ~7,000 | 99.2% |

### Summary

```
┌─────────────────────────────────────────────────────────────┐
│  POMMEL (10 searches)          EXPLORER AGENTS (10 searches)│
│  ═══════════════════           ═════════════════════════════│
│  Total tokens:    4,374        Total tokens:    ~107,000    │
│  Avg per search:    437        Avg per search:   ~10,700    │
│  Avg time:        ~14ms        Avg time:        ~30-60s     │
├─────────────────────────────────────────────────────────────┤
│  💰 TOKENS SAVED:     102,626 (95.9% reduction)             │
│  ⚡ SPEED:            ~2000-4000x faster                     │
│  📊 EFFICIENCY:       24x fewer tokens per search           │
└─────────────────────────────────────────────────────────────┘
```

### Key Insight

Even for **non-matches** (code that doesn't exist), explorer agents consume 5,000-7,000 tokens just to exhaustively search and conclude "nothing found." Pommel returns a low-confidence result in <100 tokens with a score of 0.40-0.49, allowing agents to quickly recognize weak matches and move on.

## License

MIT
