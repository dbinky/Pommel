# Jina v4 Embedding Support Design

**Date:** 2026-01-15
**Branch:** dev-jina-4
**Version:** 0.8.0

## Overview

Add support for Jina v4 embeddings as an optional upgrade from the default v2 model, selectable at install time. This provides users with better code search quality while keeping the lightweight v2 as the default for most users.

## Models

| Model | Ollama Name | Size | Dimensions | Notes |
|-------|-------------|------|------------|-------|
| v2 (default) | `unclemusclez/jina-embeddings-v2-base-code` | ~300MB | 768 | Lightweight, well-tested |
| v4 (optional) | `sellerscrisp/jina-embeddings-v4-text-code-q4` | ~8GB | 1024 | Better quality, larger |

## User Experience

### Install Script Flow

When Ollama is selected as the provider, prompt for model choice:

```
[2/5] Configure embedding provider

  How would you like to generate embeddings?

  1) Local Ollama    - Free, runs on this machine
  2) Remote Ollama   - Free, connect to Ollama on another machine
  3) OpenAI API      - Paid, no local setup required
  4) Voyage AI       - Paid, optimized for code search

  Choice [1]: 1

  Which embedding model?

  1) Standard    - Jina v2 Code (~300MB, faster, good quality) (Recommended)
  2) Maximum     - Jina v4 Code (~8GB, slower, best quality)

  Choice [1]: _
```

### Per-Project Override

```bash
pm config model v4   # Switch this project to v4 (triggers reindex prompt)
pm config model v2   # Switch back to v2
```

## Configuration Storage

### Global Config (`~/.config/pommel/config.yaml`)

```yaml
embedding:
  provider: ollama
  ollama:
    url: "http://localhost:11434"
    model: "unclemusclez/jina-embeddings-v2-base-code"
```

### Per-Project Override (`.pommel/config.yaml`)

```yaml
embedding:
  ollama:
    model: "sellerscrisp/jina-embeddings-v4-text-code-q4"
```

### Model Metadata (Hardcoded in Go)

```go
var EmbeddingModels = map[string]ModelInfo{
    "v2": {
        Name:       "unclemusclez/jina-embeddings-v2-base-code",
        Dimensions: 768,
        Size:       "~300MB",
    },
    "v4": {
        Name:       "sellerscrisp/jina-embeddings-v4-text-code-q4",
        Dimensions: 1024,
        Size:       "~8GB",
    },
}
```

## Code Changes

### Files to Modify

1. **`scripts/install.sh`** (and `install.ps1`)
   - Add model selection prompt after provider selection
   - Store model choice in global config
   - Pull the selected model

2. **`internal/embedder/ollama.go`**
   - Add `ModelInfo` struct and model registry
   - Update `Dimensions()` to return based on configured model
   - Remove hardcoded `JinaContextSize` constant, make it model-aware

3. **`internal/cli/config.go`**
   - Add `pm config model [v2|v4]` subcommand
   - Warn user that changing model requires reindex
   - Update config file with new model

### No Changes Needed

- `internal/config/config.go` - Already supports `embedding.ollama.model`
- `internal/db/vectors.go` - Already supports dynamic dimensions via `db.Open(path, dimensions)`
- Daemon already reads model from config

## Model Switching Behavior

### When user runs `pm config model v4`

1. Check if daemon is running - if yes, prompt to stop it first
2. Check current model in config vs requested model
3. If different:
   - Warn: "Switching models requires a full reindex. The existing index will be deleted."
   - Prompt: "Continue? (y/N)"
   - If yes: Update config, delete `.pommel/pommel.db`
   - Print: "Model changed to v4. Run `pm start` to reindex with the new model."

### When daemon starts with mismatched dimensions

If the database exists but has different dimensions than the configured model:
- Log error: "Database dimensions (768) don't match configured model (1024). Run `pm reindex --force` or `pm config model v2` to fix."
- Exit with error (don't silently corrupt data)

### Fresh project (`pm init`)

Uses whatever model is in global config. No special handling needed.

## Edge Cases

1. **Remote Ollama with model choice:** Same prompt, but warn that the model must also be available on the remote server

2. **Upgrade from v0.7.3:** Existing installs keep their current model (v2). No automatic migration.

3. **Unknown model in config:** If user manually sets a model not in our registry, fall back to querying Ollama for dimensions or default to 768

4. **Model pull fails:** Same behavior as today - warn and continue, user can pull manually

## Summary

| Aspect | Decision |
|--------|----------|
| Default model | v2 (~300MB, 768 dims) |
| Optional model | v4 (~8GB, 1024 dims) |
| Choice timing | Install script, after Ollama provider selection |
| Storage | Global default + per-project override |
| Model switching | Requires explicit reindex, warns user |
| Dimension mismatch | Hard error, don't corrupt data |
