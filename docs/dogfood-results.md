# Pommel Dogfooding Results

This document tracks the results of running Pommel on its own codebase. These tests validate that Pommel can effectively index and search Go code.

## Latest Run

**Date:** [YYYY-MM-DD HH:MM:SS UTC]
**Version:** [VERSION or COMMIT]
**Status:** [SUCCESS / FAILURE / SKIPPED]

---

## Indexing Statistics

| Metric | Value | Notes |
|--------|-------|-------|
| Total Files | | Go source files indexed |
| Total Chunks | | All chunk levels combined |
| Indexing Time | | Time from start to completion |
| Average Time/File | | Indexing efficiency metric |
| Memory Usage (Peak) | | Optional: if monitored |

### Chunk Breakdown (if available)

| Level | Count | Percentage |
|-------|-------|------------|
| File | | |
| Class/Type | | |
| Method/Function | | |
| Block | | |

---

## Search Quality Tests

These searches test that Pommel can find relevant code semantically.

### Test 1: Embedding Generation

**Query:** `"embedding generation"`

**Expected Results:**
- `internal/embedder/ollama.go` - OllamaClient, embedding functions
- `internal/embedder/embedder.go` - Embedder interface

**Actual Results:**

| Rank | File | Chunk | Score | Expected? |
|------|------|-------|-------|-----------|
| 1 | | | | |
| 2 | | | | |
| 3 | | | | |

**Status:** [PASS/FAIL]
**Notes:**

---

### Test 2: File Watcher Debounce

**Query:** `"file watcher debounce"`

**Expected Results:**
- `internal/daemon/watcher.go` - Watcher struct, debounceEvent function
- Debounce timer logic

**Actual Results:**

| Rank | File | Chunk | Score | Expected? |
|------|------|-------|-------|-----------|
| 1 | | | | |
| 2 | | | | |
| 3 | | | | |

**Status:** [PASS/FAIL]
**Notes:**

---

### Test 3: Vector Search

**Query:** `"vector search"`

**Expected Results:**
- `internal/search/search.go` - Search service, SearchChunks
- `internal/db/search.go` - Database search operations

**Actual Results:**

| Rank | File | Chunk | Score | Expected? |
|------|------|-------|-------|-----------|
| 1 | | | | |
| 2 | | | | |
| 3 | | | | |

**Status:** [PASS/FAIL]
**Notes:**

---

### Test 4: CLI Command

**Query:** `"CLI command"`

**Expected Results:**
- `internal/cli/root.go` - Cobra root command
- Other CLI command files (search.go, init.go, etc.)

**Actual Results:**

| Rank | File | Chunk | Score | Expected? |
|------|------|-------|-------|-----------|
| 1 | | | | |
| 2 | | | | |
| 3 | | | | |

**Status:** [PASS/FAIL]
**Notes:**

---

### Test 5: Code Chunking

**Query:** `"code chunking"`

**Expected Results:**
- `internal/chunker/chunker.go` - Chunker interface
- `internal/chunker/treesitter.go` - Tree-sitter based chunking

**Actual Results:**

| Rank | File | Chunk | Score | Expected? |
|------|------|-------|-------|-----------|
| 1 | | | | |
| 2 | | | | |
| 3 | | | | |

**Status:** [PASS/FAIL]
**Notes:**

---

### Test 6: Configuration Loading

**Query:** `"configuration loading"`

**Expected Results:**
- `internal/config/loader.go` - Configuration loading
- `internal/config/config.go` - Config struct

**Actual Results:**

| Rank | File | Chunk | Score | Expected? |
|------|------|-------|-------|-----------|
| 1 | | | | |
| 2 | | | | |
| 3 | | | | |

**Status:** [PASS/FAIL]
**Notes:**

---

### Test 7: HTTP API Handler

**Query:** `"HTTP API handler"`

**Expected Results:**
- `internal/api/handlers.go` - HTTP handlers
- `internal/api/router.go` - Router setup

**Actual Results:**

| Rank | File | Chunk | Score | Expected? |
|------|------|-------|-------|-----------|
| 1 | | | | |
| 2 | | | | |
| 3 | | | | |

**Status:** [PASS/FAIL]
**Notes:**

---

### Test 8: Database Schema

**Query:** `"database schema"`

**Expected Results:**
- `internal/db/schema.go` - SQLite schema definitions
- `internal/db/sqlite.go` - Database initialization

**Actual Results:**

| Rank | File | Chunk | Score | Expected? |
|------|------|-------|-------|-----------|
| 1 | | | | |
| 2 | | | | |
| 3 | | | | |

**Status:** [PASS/FAIL]
**Notes:**

---

## Performance Metrics

| Operation | Time (ms) | Target | Status |
|-----------|-----------|--------|--------|
| Single search query | | <100ms | |
| Search (10 results) | | <150ms | |
| Daemon startup | | <2s | |
| File change detection | | <500ms | |

---

## Issues Found

List any issues discovered during dogfooding:

### Issue 1: [Title]

**Severity:** [Critical / High / Medium / Low]
**Category:** [Search Quality / Performance / Stability / Other]

**Description:**


**Steps to Reproduce:**
1.
2.
3.

**Expected Behavior:**


**Actual Behavior:**


**Potential Fix:**


---

## Test Environment

| Component | Version/Details |
|-----------|-----------------|
| macOS Version | |
| Go Version | |
| Ollama Version | |
| Embedding Model | unclemusclez/jina-embeddings-v2-base-code |
| CPU | |
| Memory | |

---

## Historical Results

Track dogfooding results over time to monitor improvements.

| Date | Version | Files | Chunks | Index Time | Tests Passed | Notes |
|------|---------|-------|--------|------------|--------------|-------|
| | | | | | | |

---

## Notes

- This document should be updated after each significant dogfooding run
- Use `scripts/dogfood.sh --results-file docs/dogfood-results.md` to auto-generate basic results
- Manual review of search quality is recommended for nuanced assessment
