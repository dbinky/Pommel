# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Working Directory Constraints

Only edit files within this project structure. The sole exception is when installing or configuring third-party dependencies used by the project (e.g., Chroma vector database, Ollama for embedding models).

## Code Quality Principles

- **SOLID principles** - Single responsibility, open/closed, Liskov substitution, interface segregation, dependency inversion
- **DRY with domain sensitivity** - Eliminate duplication, but don't over-abstract across domain boundaries
- **Performance-critical** - This system will be called frequently by AI agents; optimize for low latency on search operations
- **Fast indexing** - File changes must be detected and re-indexed as quickly as possible; minimize time between file save and searchability

## Project Overview

Pommel is a local-first semantic code search system designed to reduce context window consumption for AI coding agents. It maintains an always-current vector database of code embeddings, enabling targeted semantic searches instead of reading numerous files into context.

**Status:** Planning phase (v0.1.0-draft) - no implementation code yet

## Architecture

```
AI Agent / Developer
        │
        ▼
    Pommel CLI (pm)  ──────► search, status, config
        │
        ▼
    Pommel Daemon (pommeld)
    • File watcher (debounced)
    • Multi-level chunker
    • Embedding generator
        │
        ▼
    Chroma Vector DB (local)
        ▲
        │
    Jina Code Embeddings (local model, 768-dim)
```

**Two data flows:**
1. **Indexing:** File changes → Daemon → Chunker → Embedder → Vector DB
2. **Search:** Query → CLI → Embedder → Vector DB → Ranked results

## Planned CLI Commands

```bash
pm init                    # Initialize Pommel in project
pm search <query> --json   # Semantic search (agent-optimized)
pm status                  # Show daemon status
pm start / pm stop         # Daemon control
pm reindex                 # Force full reindex
pm config                  # View/modify configuration
```

All commands support `--json` for structured output, `--limit`, `--level`, and `--path` flags.

## Key Design Decisions

- **Implementation language:** Go
- **Target platforms:** macOS, Linux
- **Vector database:** Chroma (local)
- **Embedding model:** Jina Code Embeddings (jinaai/jina-embeddings-v2-base-code)
- **Chunking levels:** file, class/module, method/function, block, line group
- **Initial language support:** C#, Go, Python, JavaScript, TypeScript, Java, Rust

## Project Structure (Planned)

```
project/
├── .pommel/
│   ├── config.yaml    # Project configuration
│   ├── state.json     # Daemon state
│   └── chroma/        # Vector database files
├── .pommelignore      # Patterns to exclude (like .gitignore)
└── source files...
```

## Related System

Pommel is designed to complement **Beads** (task/issue tracking):
- Pommel: "Where is this logic implemented?" (code search)
- Beads: "What should I work on next?" (task memory)
