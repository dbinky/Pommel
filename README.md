# Pommel

Semantic code search for AI coding agents.

## What is Pommel?

Pommel is a local-first semantic code search system that reduces context window consumption for AI coding agents. Instead of reading numerous files to find relevant code, agents can perform targeted semantic searches against an always-current vector database.

## How it Works

A background daemon watches your project files, chunks them intelligently, generates embeddings, and stores them in a local vector database. The CLI provides fast semantic search that returns ranked results with file paths, line ranges, and relevance scores.

```bash
pm init                              # Initialize in your project
pm start                             # Start the daemon
pm search "authentication flow" --json   # Semantic search
```

## Status

**Planning phase** - See [docs/plans/PROJECT_BRIEF.md](docs/plans/PROJECT_BRIEF.md) for the full specification.

## Tech Stack

- **Language:** Go
- **Vector DB:** Chroma (local)
- **Embeddings:** Jina Code Embeddings via Ollama
- **Platforms:** macOS, Linux
