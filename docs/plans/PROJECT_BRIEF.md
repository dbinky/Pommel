# Pommel: Semantic Code Search for AI Coding Agents

## Project Brief — C4 Context Level Document

**Version:** 0.5.2
**Status:** Implemented
**Target Platforms:** macOS, Linux, Windows
**Initial Target Agent:** Claude Code

---

## Executive Summary

Pommel is a local-first semantic code search system designed to reduce context window consumption for AI coding agents. By maintaining an always-current vector database of code embeddings, Pommel enables agents to perform targeted semantic searches rather than reading numerous files into context when searching for relevant code.

The name "Pommel" evokes the counterweight at the end of a sword's handle—a small component that provides balance and control. Similarly, Pommel provides balance to agentic coding workflows by giving agents precise, weighted access to codebases without the overhead of loading entire file trees.

---

## Problem Statement

### The Context Window Problem

AI coding agents like Claude Code operate within fixed context windows. When an agent needs to understand how authentication works, find related implementations, or locate code that handles a specific concern, it typically must:

1. **Grep/search for text patterns** — Limited to exact or regex matches; misses semantically related code
2. **Read multiple files into context** — Each file consumes precious context window space
3. **Traverse directory structures** — Agents often read files speculatively, hoping to find relevance

This approach has compounding costs:

- **Context exhaustion** — Large operations fill the context window, forcing session restarts
- **Latency** — Reading and processing many files takes time
- **Imprecision** — Text search cannot find "code that handles user sessions" unless those exact words appear
- **Missed connections** — Semantically related code in differently-named files is invisible to grep

### The Opportunity

Modern embedding models can capture the semantic meaning of code. A method named `ValidateJwtToken` and another named `CheckAuthHeader` can be understood as related even though they share no common terms. By pre-computing embeddings for code chunks and storing them in a vector database, we can offer agents:

- **Semantic search** — "Find code that handles rate limiting" returns relevant results regardless of naming
- **Precise context** — Return only the specific chunks that match, not entire files
- **Always current** — File system watching keeps embeddings synchronized with code changes
- **Multiple granularities** — Search at line, block, method, class, or file level as appropriate

---

## System Context

### C4 Context Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                   DEVELOPER WORKSTATION                         │
│                                                                                 │
│  ┌──────────────────┐                                                           │
│  │                  │                                                           │
│  │   AI Coding      │──── Uses CLI to search ────┐                              │
│  │   Agent          │                            │                              │
│  │   (Claude Code)  │◄── Returns ranked chunks ──┤                              │
│  │                  │                            │                              │
│  └──────────────────┘                            ▼                              │
│                                         ┌──────────────────┐                    │
│                                         │                  │                    │
│  ┌──────────────────┐                   │   Pommel CLI     │                    │
│  │                  │                   │   (pm)           │                    │
│  │   Developer      │──── Uses CLI ────►│                  │                    │
│  │   (Human)        │     to search     │   • search       │                    │
│  │                  │     or manage     │   • status       │                    │
│  └──────────────────┘                   │   • config       │                    │
│                                         └────────┬─────────┘                    │
│                                                  │                              │
│                                                  │ Queries                      │
│                                                  ▼                              │
│  ┌──────────────────┐    File Events    ┌──────────────────┐                    │
│  │                  │ ─────────────────►│                  │                    │
│  │   OS File        │                   │   Pommel Daemon  │                    │
│  │   System         │                   │   (pommeld)      │                    │
│  │                  │                   │                  │                    │
│  │   • Project      │◄─ Reads files ────│   • File watcher │                    │
│  │     source code  │   for chunking    │   • Chunker      │                    │
│  │   • .pommel/     │                   │   • Embedder     │                    │
│  │     config       │                   │   • DB writer    │                    │
│  │                  │                   │                  │                    │
│  └──────────────────┘                   └────────┬─────────┘                    │
│                                                  │                              │
│                                                  │ Stores/retrieves             │
│                                                  │ embeddings                   │
│                                                  ▼                              │
│                                         ┌──────────────────┐                    │
│                                         │                  │                    │
│                                         │   sqlite-vec     │                    │
│                                         │   Vector DB      │                    │
│                                         │   (index.db)     │                    │
│                                         │                  │                    │
│                                         │   • Embeddings   │                    │
│                                         │   • Metadata     │                    │
│                                         │   • Single file  │                    │
│                                         └──────────────────┘                    │
│                                                                                 │
│                                                  ▲                              │
│                                                  │                              │
│                                         ┌────────┴─────────┐                    │
│                                         │                  │                    │
│                                         │   Ollama         │                    │
│                                         │   + Jina Code v2 │                    │
│                                         │   Embeddings     │                    │
│                                         │                  │                    │
│                                         └──────────────────┘                    │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Data Flow Diagram

```
                                    INDEXING FLOW
                                    ═════════════

     ┌─────────┐      ┌─────────┐      ┌─────────┐      ┌─────────┐      ┌─────────┐
     │  File   │      │ Daemon  │      │ Chunker │      │ Ollama  │      │sqlite-  │
     │ System  │      │(watch)  │      │         │      │(embed)  │      │  vec    │
     └────┬────┘      └────┬────┘      └────┬────┘      └────┬────┘      └────┬────┘
          │                │                │                │                │
          │  file changed  │                │                │                │
          │───────────────►│                │                │                │
          │                │                │                │                │
          │                │  read file     │                │                │
          │◄───────────────│                │                │                │
          │                │                │                │                │
          │   contents     │                │                │                │
          │───────────────►│                │                │                │
          │                │                │                │                │
          │                │  chunk(file)   │                │                │
          │                │───────────────►│                │                │
          │                │                │                │                │
          │                │   chunks[]     │                │                │
          │                │◄───────────────│                │                │
          │                │                │                │                │
          │                │  embed(chunks) │                │                │
          │                │────────────────────────────────►│                │
          │                │                │                │                │
          │                │   vectors[]    │                │                │
          │                │◄────────────────────────────────│                │
          │                │                │                │                │
          │                │  store(vectors, metadata)       │                │
          │                │─────────────────────────────────────────────────►│
          │                │                │                │                │
          │                │                │                │   indexed      │
          │                │◄─────────────────────────────────────────────────│
          │                │                │                │                │


                                    SEARCH FLOW
                                    ═══════════

     ┌─────────┐      ┌─────────┐      ┌─────────┐      ┌─────────┐      ┌─────────┐
     │  Agent  │      │   CLI   │      │ Daemon  │      │ Ollama  │      │sqlite-  │
     │         │      │  (pm)   │      │ (REST)  │      │(embed)  │      │  vec    │
     └────┬────┘      └────┬────┘      └────┬────┘      └────┬────┘      └────┬────┘
          │                │                │                │                │
          │ pm search      │                │                │                │
          │ "auth logic"   │                │                │                │
          │───────────────►│                │                │                │
          │                │                │                │                │
          │                │  POST /search  │                │                │
          │                │───────────────►│                │                │
          │                │                │                │                │
          │                │                │ embed(query)   │                │
          │                │                │───────────────►│                │
          │                │                │                │                │
          │                │                │  query_vector  │                │
          │                │                │◄───────────────│                │
          │                │                │                │                │
          │                │                │ vec_search()   │                │
          │                │                │───────────────────────────────►│
          │                │                │                │                │
          │                │                │   results[] (ids, scores, meta)│
          │                │                │◄───────────────────────────────│
          │                │                │                │                │
          │                │  JSON results  │                │                │
          │                │◄───────────────│                │                │
          │                │                │                │                │
          │  JSON results  │                │                │                │
          │◄───────────────│                │                │                │
          │                │                │                │                │
```

---

## Actors and Systems

### Primary Actors

| Actor | Type | Description |
|-------|------|-------------|
| **AI Coding Agent** | External System | Claude Code (initially) or other agentic coding tools that can execute CLI commands and consume JSON output |
| **Developer** | Human User | The human developer who initializes Pommel, configures settings, and may use the CLI directly for exploration |

### System Components

| Component | Type | Responsibility |
|-----------|------|----------------|
| **Pommel Daemon (pommeld)** | Background Service | Watches file system for changes, chunks modified files, generates embeddings, updates vector database, and serves REST API for search queries |
| **Pommel CLI (pm)** | Command Line Interface | Provides search and management commands for both human users and AI agents; communicates with daemon via REST API |
| **sqlite-vec** | Embedded Database | SQLite extension for vector search; single-file database storing embeddings and metadata |
| **Ollama** | External System (Local) | Local model runtime hosting Jina Code Embeddings v2 for generating semantic vectors |
| **OS File System** | External System | Source of truth for project files; provides file watching events |

---

## Component Responsibilities

### Pommel Daemon (pommeld)

The daemon is the heart of the system, responsible for maintaining synchronization between the codebase and the vector database.

**Core Responsibilities:**

1. **File System Watching**
   - Monitor project directory for file create, modify, delete, and rename events
   - Respect `.pommelignore` patterns (similar to `.gitignore`)
   - Debounce rapid successive changes to the same file
   - Handle batch changes gracefully (e.g., git checkout, large refactors)

2. **Intelligent Chunking**
   - Parse source files into semantic chunks at multiple granularities
   - Maintain chunk hierarchy metadata (which method belongs to which class, etc.)
   - Track chunk boundaries (file path, start line, end line)
   - Preserve enough context in each chunk for semantic understanding

3. **Embedding Generation**
   - Interface with Ollama running Jina Code Embeddings v2
   - Batch embedding requests for efficiency
   - Handle model availability and errors gracefully
   - Cache query embeddings to avoid redundant Ollama calls

4. **Database Management**
   - Store embeddings with rich metadata in sqlite-vec
   - Handle incremental updates (add, update, delete chunks)
   - Maintain referential integrity when files are renamed or moved
   - Use WAL mode for concurrent read/write access

5. **REST API**
   - Serve search queries from CLI via localhost HTTP
   - Provide status and health endpoints
   - Enable future caching and query optimization

6. **Health and Status**
   - Expose status information via REST API
   - Track indexing progress and lag
   - Log errors and warnings appropriately

### Pommel CLI (pm)

The CLI is the interface for both human developers and AI agents.

**Core Commands:**

| Command | Purpose | Primary User |
|---------|---------|--------------|
| `pm init` | Initialize Pommel in a project directory | Developer |
| `pm search <query>` | Semantic search across the codebase | Agent, Developer |
| `pm status` | Show daemon status, index health, stats | Developer |
| `pm start` | Start the daemon for current project | Developer |
| `pm stop` | Stop the daemon | Developer |
| `pm reindex` | Force full reindex of project | Developer |
| `pm config` | View or modify configuration | Developer |

**Agent-Optimized Design Principles:**

- All commands support `--json` flag for structured output
- Search results include file paths, line ranges, relevance scores, and chunk content
- Error messages are clear and actionable
- Exit codes are meaningful and documented
- No interactive prompts; all input via flags and arguments

### sqlite-vec Vector Database

**Role in System:**

- Persistent storage for embeddings and metadata in a single `.db` file
- Fast vector similarity search via SQLite extension
- Project-local isolation (each project has its own database)
- Pure Go integration via WASM bindings (no CGO required)
- WAL mode enables concurrent reads during writes

**Data Model:**

Each stored chunk includes:
- **ID**: Deterministic hash of file path + chunk boundaries
- **Embedding**: 768-dimensional vector from Jina model
- **Metadata**:
  - `file_path`: Relative path from project root
  - `start_line`: Beginning line number (1-indexed)
  - `end_line`: Ending line number (inclusive)
  - `chunk_level`: Granularity (method, class, file)
  - `language`: Detected or configured language
  - `parent_chunk_id`: Reference to containing chunk (for hierarchy)
  - `last_modified`: Timestamp of source file at indexing time
  - `content_hash`: Hash of chunk content for change detection

### Ollama + Jina Code Embeddings

**Role in System:**

- Ollama provides the local model runtime
- Jina Code Embeddings v2 (`unclemusclez/jina-embeddings-v2-base-code`) generates 768-dimensional semantic vectors
- Used for both indexing (chunked code) and querying (search terms)
- Single `ollama pull` command for model installation

**Model Characteristics:**
- Trained specifically on code and documentation (150M+ code Q&A pairs)
- 8K token context window (sufficient for large methods/classes)
- Supports 30+ programming languages
- Same model used for indexing and querying (critical for consistency)

---

## Chunking Strategy

Effective chunking is critical to search quality. Pommel employs a multi-level chunking strategy using Tree-sitter for accurate AST parsing.

### Chunk Levels (v0.1)

| Level | Description | Use Case |
|-------|-------------|----------|
| **File** | Entire file as single chunk | High-level "what is this file about" queries |
| **Class/Module** | Class, struct, interface, or module definition | "Find classes that handle X" |
| **Method/Function** | Individual method or function with signature and body | Most common search granularity |

**Deferred to future versions:** Block-level and line-group chunking for fine-grained searches.

### Chunking Principles

1. **Overlap for Context**
   - Chunks include contextual preamble (e.g., class name for a method)
   - Prevents orphaned chunks that lack semantic meaning

2. **Hierarchy Preservation**
   - Each chunk knows its parent chunk ID
   - Enables drill-down and roll-up queries
   - Required - agents need to navigate from method to containing class

3. **Language Awareness**
   - Tree-sitter provides accurate AST parsing per language
   - Supported: Go, C#, Python, JavaScript, TypeScript, JSX, TSX
   - Fallback: File-level chunking for unsupported languages
   - Future versions: Java, Rust, C, C++

4. **Metadata Richness**
   - Each chunk carries sufficient metadata to locate and contextualize
   - Agents can request "show me the class this method belongs to"

---

## Configuration

Pommel uses a project-local configuration approach.

### Project Structure

```
my-project/
├── .pommel/
│   ├── config.yaml        # Project configuration
│   ├── state.json         # Daemon state (PID, last indexed, etc.)
│   ├── index.db           # sqlite-vec database (single file)
│   └── pommel.log         # Daemon log file
├── .pommelignore          # Patterns to exclude from indexing
└── (project source files)
```

### Configuration Options

```yaml
# .pommel/config.yaml

# Chunk levels to generate (more levels = more storage, richer search)
chunk_levels:
  - method
  - class
  - file

# File patterns to include (default: supported language extensions)
include_patterns:
  - "**/*.cs"
  - "**/*.py"
  - "**/*.js"
  - "**/*.ts"
  - "**/*.jsx"
  - "**/*.tsx"

# Additional ignore patterns (beyond .pommelignore)
exclude_patterns:
  - "**/node_modules/**"
  - "**/bin/**"
  - "**/obj/**"
  - "**/__pycache__/**"

# Debounce delay for file watcher (milliseconds)
debounce_ms: 500

# Maximum file size to index (bytes)
max_file_size: 1048576  # 1MB

# Daemon configuration
daemon:
  port: 7420  # Default REST API port
  host: "127.0.0.1"

# Embedding model configuration (via Ollama)
embedding:
  model: "unclemusclez/jina-embeddings-v2-base-code"
  batch_size: 32

# Search defaults
search:
  default_limit: 10
  default_levels:
    - method
    - class
```

---

## CLI Interface Specification

### Search Command

The primary command for AI agents.

```bash
# Basic semantic search
pm search "authentication middleware"

# With options
pm search "database connection pooling" \
  --limit 20 \
  --level method,class \
  --path "src/Services/**" \
  --json

# Output (--json)
{
  "query": "database connection pooling",
  "results": [
    {
      "id": "a1b2c3d4",
      "file": "src/Services/DbConnectionPool.cs",
      "start_line": 45,
      "end_line": 92,
      "level": "method",
      "score": 0.89,
      "content": "public async Task<DbConnection> GetConnectionAsync()...",
      "parent_id": "e5f6g7h8",
      "parent_name": "DbConnectionPool"
    },
    ...
  ],
  "total_results": 15,
  "search_time_ms": 42
}
```

### Status Command

For health checks and debugging.

```bash
pm status --json

# Output
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

### Init Command

Project initialization.

```bash
# Initialize with defaults
pm init

# Initialize with specific languages
pm init --languages cs,go

# Initialize and start daemon
pm init --start

# Initialize and add Claude configuration
pm init --claude
```

---

## Integration with AI Agents

### Claude Code Integration

Pommel is designed to be added to a project's `CLAUDE.md` configuration when `pm init` is called with the `--claude` option:

```markdown
### Code Search

This project uses Pommel for semantic code search. Before reading multiple
files to find relevant code, use the `pm` CLI:

\`\`\`bash
# Find code related to a concept
pm search "rate limiting logic" --json

# Find implementations of a pattern
pm search "retry with exponential backoff" --level method --json

# Search within a specific area
pm search "validation" --path "src/Api/**" --json
\`\`\`

Use Pommel search results to identify specific files and line ranges,
then read only those targeted sections into context.
```

### Workflow Example

**Without Pommel:**
```
Agent: I need to understand how authentication works in this project.
       Let me read the src/Auth/ directory... (reads 15 files, 2000 lines)
       Now let me check src/Middleware/... (reads 8 more files)
       And src/Services/... (reads 12 more files)
       
       [Context window significantly consumed]
```

**With Pommel:**
```
Agent: I need to understand how authentication works in this project.
       
       $ pm search "authentication flow" --level class,method --json
       
       [Receives 10 targeted results with file:line references and snippets]
       
       These 3 results look most relevant. Let me read just those sections.
       
       [Context window minimally impacted]
```

---

## v0.2.0 Scope

### Implemented Features

- macOS and Linux support (bash environments)
- Go implementation for daemon and CLI
- Ollama + Jina Code Embeddings v2 (local model)
- sqlite-vec vector database (single-file, embedded)
- Multi-level chunking: method/function, class/module, file
- **Language support (full AST-aware chunking):** Go, C#, Python, JavaScript, TypeScript, JSX, TSX
- Tree-sitter for AST parsing
- File system watching with debounce
- CLI ↔ Daemon communication via localhost REST API
- `.pommelignore` support
- JSON output for all commands
- Project-local configuration and database
- **Multi-project support**: Each project gets its own daemon on a unique port (hash-based)
- **Monorepo support**: Auto-detection of sub-projects, scoped search
- Chunk hierarchy with parent references
- Auto-setup: detect dependencies, prompt to install, auto-start services
- `pm init --claude` to inject usage instructions into CLAUDE.md
- `pm init --auto` for automatic language detection

### Out of Scope (Future Consideration)

- Windows support
- Remote/cloud database options
- IDE plugins
- Web UI
- Embedding model selection (locked to Jina Code v2 for now)
- Additional languages: Java, Rust, C, C++
- Block-level and line-group chunking
- Cross-repository search (searching multiple unrelated repos in one query)
- Real-time streaming results
- Natural language to code generation
- Diff-aware incremental re-chunking (full file re-chunk on change)

### Implementation Guidelines
- Write SOLID code built for high testability (via DI for mocking)
- All systems must have a robust test suite that consists of happy path, failing, and error conditions
- Everything must run locally without using the internet

---

## Success Criteria

Pommel will be considered successful when:

1. **Reduction in Context Usage**
   - Agents using Pommel read 50%+ fewer lines into context for exploratory tasks
   
2. **Search Quality**
   - Semantic searches return relevant results in top 5 for 80%+ of queries
   - Agents can find code without knowing exact names or patterns

3. **Freshness**
   - Index updates complete within 2 seconds of file save for typical files
   - No manual reindex required during normal development

4. **Reliability**
   - Daemon runs stably for multi-day sessions
   - Graceful handling of edge cases (large files, binary files, rapid changes)

5. **Usability**
   - `pm init && pm start` gets a project indexed and searchable
   - Agents can use Pommel with minimal instruction in AGENTS.md

---

## Appendix A: Glossary

| Term | Definition |
|------|------------|
| **Chunk** | A discrete piece of code extracted for embedding; may be a method, class, or other unit |
| **Embedding** | A fixed-length vector representation of text that captures semantic meaning |
| **Vector Database** | A database optimized for storing and searching high-dimensional vectors |
| **Semantic Search** | Search based on meaning rather than exact text matching |
| **Context Window** | The maximum amount of text an AI model can consider in a single interaction |
| **Daemon** | A background process that runs continuously |
| **Debounce** | Combining multiple rapid events into a single action after a delay |

---

## Appendix B: Related Projects and Prior Art

| Project | Relationship to Pommel |
|---------|----------------------|
| **Beads** | Complementary; task memory vs. code memory |
| **Sourcegraph** | Similar goals, but cloud-hosted and enterprise-focused |
| **GitHub Copilot** | Uses embeddings internally; not exposed as searchable index |
| **Cursor** | IDE with semantic features; Pommel is agent-first and IDE-agnostic |
| **Greptile** | API-based code search; Pommel is local-first |

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 0.1.0-draft | 2025-01-XX | Ryan + Claude | Initial draft |
| 0.1.0-refined | 2025-12-28 | Ryan + Claude | Design refinement: sqlite-vec replaces Chroma, Ollama for embeddings, REST API for CLI↔Daemon, Tree-sitter for parsing, scoped to 4 languages and 3 chunk levels |
| 0.2.0 | 2025-12-29 | Ryan + Claude | Implementation complete: Added Go language support, multi-project support with hash-based ports, monorepo/sub-project detection, updated status to Implemented |

---

*This document is intended for use with Claude Code and the Beads task management system for implementation planning.*
