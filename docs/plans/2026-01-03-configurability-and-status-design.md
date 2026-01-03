# Configurability and Status Improvements Design

**Date:** 2026-01-03
**Branch:** `dev-configurability-and-status-improvement`
**Status:** Approved

---

## Overview

Two improvements to Pommel:

1. **Universal Language Support** - Support all 31 tree-sitter languages without requiring Go code changes to add new languages
2. **Status Improvements** - Show indexing progress with % complete, current files/chunks, and ETA

---

## Feature 1: Universal Language Support

### Problem

Currently adding a new language requires:
- Editing `internal/chunker/treesitter.go` (grammar registry, parser init, language detection)
- Editing test files for coverage
- Recompiling

YAML configs exist for 13 languages, but only 8 work due to hardcoded Go code.

### Solution

**Code generation from YAML configs as the single source of truth.**

- `go generate` reads all `languages/*.yaml` files
- Produces `treesitter_generated.go` with grammar registry, extension mapping, detection
- YAML configs embedded in binary via `//go:embed` for runtime chunk mappings
- Users get pre-compiled binary with all 31 languages

### User Experience

- **Install:** Download binary → all 31 languages work immediately
- **No code generation or compilation by end users**
- **Adding language #32:** We create YAML, run `go generate`, release new binary

### New Chunk Level

Add `ChunkLevelSection` for document-centric formats:

| Level | Code meaning | Document meaning |
|-------|--------------|------------------|
| `file` | Entire file | Entire document |
| `class` | Class/struct | Top-level grouping |
| `section` | (new) | Heading + content |
| `method` | Function | Code block / key-value |

### Languages Supported (31 total)

**Code:** Bash, C, C++, C#, Cue, Dart, Dockerfile, Elixir, Elm, Go, Groovy, HCL, Java, JavaScript, Kotlin, Lua, OCaml, PHP, Protocol Buffers, Python, Ruby, Rust, Scala, Solidity, SQL, Svelte, Swift, TypeScript/TSX

**Documents:** CSS, HTML, Markdown, TOML, YAML

### YAML Config Schema

```yaml
# Required fields
language: go                    # Unique identifier
display_name: Go                # Human-readable name
extensions:                     # File extensions (lowercase, with dot)
  - .go

# Tree-sitter configuration
tree_sitter:
  grammar: go                   # Maps to grammarRegistry key

# Chunk mappings - which AST nodes become which chunk levels
chunk_mappings:
  class:
    - type_spec
    - struct_type
  method:
    - function_declaration
    - method_declaration
  section: []                   # For document formats
  block:
    - if_statement

# Metadata extraction
extraction:
  name_field: name
  doc_comments:
    - comment
  doc_comment_position: preceding_siblings
```

### Code Generator Design

**Location:** `internal/chunker/generate.go`

**Trigger:** `//go:generate go run generate.go`

**Generates:** `treesitter_generated.go` containing:
- `grammarRegistry` map (grammar name → tree-sitter language getter)
- `extensionToLanguage` map (extension → language name)
- `DetectLanguage()` function

**Grammar-to-import mapping:** Static map in generator of grammar names to Go import paths (e.g., `"go"` → `"github.com/smacker/go-tree-sitter/golang"`).

### Files Changed

| File | Change |
|------|--------|
| `internal/models/chunk.go` | Add `ChunkLevelSection` |
| `internal/chunker/generate.go` | New code generator |
| `internal/chunker/treesitter_generated.go` | Generated output |
| `internal/chunker/treesitter.go` | Simplified, imports generated code |
| `languages/*.yaml` | 31 config files |

### Binary Size Impact

- Each grammar: ~1-3MB compiled
- Current: ~15-20MB
- With all 31: ~50-80MB
- **Verdict:** Acceptable for a developer tool

---

## Feature 2: Status Improvements

### Problem

Current `pm status` only shows:
- Daemon running state
- Total files/chunks
- "Indexing in progress..." or "Ready"

No visibility into progress during indexing.

### Solution

Track and display:
- **% complete** - `FilesProcessed / FilesToProcess`
- **Current counts** - Files and chunks as they update
- **ETA** - Minutes:seconds remaining

### Target Output (During Indexing)

```
Pommel Status
=============

Daemon:
  Running:  true
  PID:      16993
  Uptime:   5m 32s

Index:
  Files:    47 / 115 (40.9%)
  Chunks:   892
  Status:   Indexing...
  ETA:      2:15 remaining

Dependencies:
  Database: OK
  Embedder: OK
```

### Target Output (When Idle)

```
Index:
  Files:    115
  Chunks:   1805
  Status:   Ready
```

### IndexStats Changes

```go
type IndexStats struct {
    // Existing
    TotalFiles     int64
    TotalChunks    int64
    LastIndexedAt  time.Time
    IndexingActive bool

    // New fields
    FilesToProcess   int64     // Discovered during scan
    FilesProcessed   int64     // Completed so far
    IndexingStarted  time.Time // When current operation began
}
```

### ETA Calculation

```go
elapsed := time.Since(stats.IndexingStarted)
rate := float64(stats.FilesProcessed) / elapsed.Seconds()
remaining := stats.FilesToProcess - stats.FilesProcessed
etaSeconds := float64(remaining) / rate
```

### API Response Changes

```go
type IndexStatus struct {
    TotalFiles      int64     `json:"total_files"`
    TotalChunks     int64     `json:"total_chunks"`
    LastIndexedAt   time.Time `json:"last_indexed_at"`
    IndexingActive  bool      `json:"indexing_active"`

    // New fields (only present when indexing)
    FilesToProcess  int64   `json:"files_to_process,omitempty"`
    FilesProcessed  int64   `json:"files_processed,omitempty"`
    PercentComplete float64 `json:"percent_complete,omitempty"`
    ETASeconds      float64 `json:"eta_seconds,omitempty"`
}
```

### CLI Display Logic

- Show `X / Y (Z%)` format only when `IndexingActive == true`
- Show ETA only when `FilesProcessed > 0` (otherwise "calculating...")
- Format ETA as `M:SS` or `H:MM:SS` for longer operations
- Transition to "Ready" display when complete

### Files Changed

| File | Change |
|------|--------|
| `internal/daemon/indexer.go` | Track progress in `ReindexAll` |
| `internal/api/types.go` | Add fields to `IndexStatus` |
| `internal/api/handlers.go` | Populate new fields |
| `internal/cli/status.go` | Display progress and ETA |

---

## TDD Test Plan

### Code Generator Tests (`internal/chunker/generate_test.go`)

**Happy Path:**
- `TestGenerator_SingleLanguage` - Generates valid Go code from one YAML
- `TestGenerator_AllLanguages` - Generates complete registry from all 31 YAMLs
- `TestGenerator_OutputCompiles` - Generated code passes `go build`

**Success Scenarios:**
- `TestGenerator_MultipleExtensions` - Language with `.yaml`, `.yml`
- `TestGenerator_GrammarAliases` - Grammar name differs from language
- `TestGenerator_EmptyChunkMappings` - Valid for some formats

**Failure Scenarios:**
- `TestGenerator_MissingLanguageField` - YAML without `language:` → error
- `TestGenerator_MissingGrammar` - Unknown grammar → error
- `TestGenerator_DuplicateLanguage` - Same name twice → error
- `TestGenerator_DuplicateExtension` - Same extension twice → error

**Error Scenarios:**
- `TestGenerator_InvalidYAML` - Malformed syntax → parse error
- `TestGenerator_EmptyExtensions` - No extensions → error
- `TestGenerator_NoYAMLFiles` - Empty directory → error

**Edge Cases:**
- `TestGenerator_CaseSensitiveExtensions` - `.Go` vs `.go`
- `TestGenerator_SpecialCharInLanguage` - Name with `_` or `-`

### Runtime Language Tests (`internal/chunker/treesitter_test.go`)

**Happy Path:**
- `TestDetectLanguage_CommonExtensions` - `.go`, `.py`, `.js` → correct
- `TestDetectLanguage_DocumentFormats` - `.md`, `.yaml` → correct
- `TestGetLanguageGrammar_AllSupported` - All 31 grammars load
- `TestChunker_ParsesAllLanguages` - Sample file per language works

**Success Scenarios:**
- `TestChunker_MarkdownSections` - Headings become section chunks
- `TestChunker_YAMLTopLevelKeys` - Top-level mappings become chunks
- `TestChunker_CodeBlocksInMarkdown` - Fenced code blocks → method level

**Failure Scenarios:**
- `TestDetectLanguage_UnknownExtension` - `.xyz` → "unknown"
- `TestChunker_BinaryFile` - Graceful skip, no panic
- `TestChunker_InvalidSyntax` - Partial results + errors

**Edge Cases:**
- `TestDetectLanguage_UppercaseExtension` - `.GO` → unknown
- `TestDetectLanguage_NoExtension` - `Makefile` → correct
- `TestChunker_UnicodeContent` - Unicode preserved
- `TestChunker_MixedLineEndings` - Line numbers correct

### Indexer Stats Tests (`internal/daemon/indexer_test.go`)

**Happy Path:**
- `TestIndexer_StatsWhileIdle` - Returns totals when not indexing
- `TestIndexer_StatsWhileIndexing` - Returns progress during reindex
- `TestIndexer_ProgressUpdatesPerFile` - `FilesProcessed` increments
- `TestIndexer_ETACalculation` - ETA decreases as files complete

**Success Scenarios:**
- `TestIndexer_DiscoveryPhaseCountsFiles` - `FilesToProcess` set first
- `TestIndexer_ProgressResetOnNewReindex` - Fresh start resets fields
- `TestIndexer_StatsAfterCompletion` - Progress fields cleared

**Failure Scenarios:**
- `TestIndexer_StatsAfterPartialFailure` - Reflects successful only
- `TestIndexer_StatsAfterCancellation` - Shows partial progress

**Edge Cases:**
- `TestIndexer_ETAWithZeroProcessed` - Shows "calculating..."
- `TestIndexer_ProgressWith1File` - 0% then 100%
- `TestIndexer_ConcurrentStatsReads` - No race condition

### Status CLI Tests (`internal/cli/status_test.go`)

**Happy Path:**
- `TestStatusCmd_DisplaysProgress` - Shows `47 / 115 (40.9%)`
- `TestStatusCmd_DisplaysETA` - Shows `2:15 remaining`
- `TestStatusCmd_DisplaysReady` - Shows `Ready` when idle
- `TestStatusCmd_JSONOutput` - Valid JSON with progress

**Success Scenarios:**
- `TestStatusCmd_ETAMinutesSeconds` - Formats as `M:SS`
- `TestStatusCmd_ETAHoursMinutes` - Formats as `H:MM:SS`
- `TestStatusCmd_HidesProgressWhenIdle` - No "X / Y" when done

**Failure Scenarios:**
- `TestStatusCmd_DaemonNotRunning` - Clear error message
- `TestStatusCmd_ConnectionRefused` - Helpful message

**Edge Cases:**
- `TestStatusCmd_VeryLargeNumbers` - Formatted with commas
- `TestStatusCmd_ETACalculating` - Shows calculating message
- `TestStatusCmd_100Percent` - Transitions to Ready

---

## Implementation Order

1. Add `ChunkLevelSection` to models
2. Create code generator
3. Create all 31 YAML configs
4. Update indexer for progress tracking
5. Update API types and handlers
6. Update CLI status display
7. Integration testing

---

## Success Criteria

- [ ] All 31 languages parse without errors
- [ ] YAML/Markdown produce meaningful section chunks
- [ ] `pm status` shows % complete during reindex
- [ ] ETA is reasonably accurate (within 20%)
- [ ] All TDD tests pass
- [ ] No manual Go code changes needed to add language #32
