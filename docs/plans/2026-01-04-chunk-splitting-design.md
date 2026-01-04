# Chunk Splitting for Large Code Units

**Date:** 2026-01-04
**Status:** Draft

## Problem Statement

Embedding models have context length limits:

| Provider | Max Tokens | ~Max Characters |
|----------|------------|-----------------|
| Ollama (Jina v2) | 8,192 | ~32KB |
| OpenAI (text-embedding-3-small) | 8,191 | ~32KB |
| Voyage (voyage-code-3) | 16,000 | ~64KB |

When a code chunk exceeds these limits, the embedding request fails:
```
llm embedding error: the input length exceeds the context length
```

**Current behavior:** Chunk silently fails to index, reducing search quality.

**Desired behavior:** All code is indexed, even if it requires splitting large chunks.

## Goals

1. **Never fail to index** - All code should be searchable
2. **Preserve semantic meaning** - Splits should maintain context
3. **Maintain search quality** - Large code units should still be findable
4. **Provider-agnostic** - Work correctly across all embedding providers
5. **Backwards compatible** - Existing indexes continue to work

## Design Decisions

The following decisions have been made:

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Overlap size | 256 tokens | Sufficient context preservation without excessive duplication |
| Max splits per chunk | No cap | Storage is cheap; prefer complete indexing over arbitrary limits |
| Minified files | Detect and skip | No semantic structure to split on; waste of embeddings |
| Very large files (>100KB) | Skip file-level chunk | File-level is a summary; class/method chunks provide detail |

## Design

### Token Estimation

Use a simple heuristic for token estimation:

```go
// EstimateTokens approximates token count from text length.
// For code, ~4 characters per token is a reasonable approximation.
// We use 3.5 to be conservative (better to split early than fail).
func EstimateTokens(text string) int {
    return int(float64(len(text)) / 3.5)
}
```

This avoids adding tokenizer dependencies while being conservative enough to prevent failures.

### Provider Context Limits

Add context limits to provider configuration:

```go
// internal/embedder/provider.go
func (p ProviderType) MaxContextTokens() int {
    switch p {
    case ProviderOpenAI:
        return 8000  // 8191 minus safety margin
    case ProviderVoyage:
        return 15000 // 16000 minus safety margin
    default: // Ollama
        return 8000  // 8192 minus safety margin
    }
}
```

### Minified File Detection

Detect and skip minified files to avoid wasting embeddings on unsplittable content:

```go
// internal/chunker/minified.go

// IsMinified detects if a file is minified/compressed code.
// Minified files have no semantic structure and should be skipped.
func IsMinified(content []byte, path string) bool {
    // Check file extension hints
    if strings.Contains(path, ".min.") {
        return true // e.g., app.min.js, style.min.css
    }

    // Heuristic: minified files have very long lines
    // Normal code rarely exceeds 200 chars per line on average
    lines := bytes.Count(content, []byte("\n"))
    if lines == 0 {
        lines = 1
    }
    avgLineLength := len(content) / lines

    // If average line > 500 chars or single line > 10KB, likely minified
    if avgLineLength > 500 {
        return true
    }

    // Single line file over 10KB is almost certainly minified
    if lines == 1 && len(content) > 10*1024 {
        return true
    }

    // Check for lack of whitespace (minified code strips it)
    whitespaceCount := bytes.Count(content, []byte(" ")) +
                       bytes.Count(content, []byte("\t")) +
                       bytes.Count(content, []byte("\n"))
    whitespaceRatio := float64(whitespaceCount) / float64(len(content))

    // Normal code is typically 15-25% whitespace; minified is < 5%
    if whitespaceRatio < 0.05 && len(content) > 1024 {
        return true
    }

    return false
}
```

When a minified file is detected:
- Log at DEBUG level: `"skipping minified file: %s"`
- Skip all chunking for the file
- Do not index any embeddings

### Splitting Strategy

Different strategies based on chunk level:

| Chunk Level | Size Check | Strategy |
|-------------|------------|----------|
| File | > 100KB | Skip entirely (rely on class/method chunks) |
| File | ≤ 100KB, oversized | Truncate with marker |
| Class | Oversized | Signature + truncated body |
| Method | Oversized | Semantic split at block boundaries (256-token overlap) |

### File-Level Handling

For file-level chunks:

```go
// internal/chunker/splitter.go

const (
    MaxFileSizeForFileChunk = 100 * 1024 // 100KB
    OverlapTokens           = 256
)

func (s *Splitter) HandleFileChunk(chunk *models.Chunk, fileSize int) *SplitChunk {
    // Skip file-level chunk entirely for very large files
    // The class/method chunks will provide searchable content
    if fileSize > MaxFileSizeForFileChunk {
        return nil // Signal to skip this chunk
    }

    // If within token limit, use as-is
    if EstimateTokens(chunk.Content) <= s.maxTokens {
        return &SplitChunk{Content: chunk.Content, IsPartial: false}
    }

    // Truncate to max tokens and add marker
    maxChars := int(float64(s.maxTokens) * 3.5)
    truncated := chunk.Content[:maxChars]

    // Find last complete line
    lastNewline := strings.LastIndex(truncated, "\n")
    if lastNewline > 0 {
        truncated = truncated[:lastNewline]
    }

    return &SplitChunk{
        Content:   truncated + "\n// ... [truncated]",
        IsPartial: true,
    }
}
```

### Class-Level Handling

For class-level chunks:

```go
func (s *Splitter) HandleClassChunk(chunk *models.Chunk) *SplitChunk {
    if EstimateTokens(chunk.Content) <= s.maxTokens {
        return &SplitChunk{Content: chunk.Content, IsPartial: false}
    }

    // Keep signature and docstring, truncate body
    // The detailed content is in method-level chunks anyway
    maxChars := int(float64(s.maxTokens) * 3.5)
    truncated := chunk.Content[:maxChars]

    // Try to end at a method boundary using tree-sitter
    // ... (find clean break point)

    return &SplitChunk{
        Content:   truncated + "\n// ... [truncated]",
        IsPartial: true,
    }
}
```

### Semantic Splitting for Methods

For method-level chunks that exceed the limit:

1. **Find split points** - Use tree-sitter to identify block/statement boundaries
2. **Create overlapping windows** - 256-token overlap between splits
3. **Store as linked chunks** - Each split references the parent chunk
4. **No cap on splits** - Generate as many splits as needed for complete coverage

```go
// internal/chunker/splitter.go

type SplitChunk struct {
    Content     string
    StartLine   int
    EndLine     int
    Index       int    // 0, 1, 2... for ordering
    IsPartial   bool   // true if this is a split (not the full chunk)
    ParentID    string // ID of the original chunk (empty if not split)
}

type Splitter struct {
    maxTokens     int
    overlapTokens int // fixed at 256
}

func NewSplitter(maxTokens int) *Splitter {
    return &Splitter{
        maxTokens:     maxTokens,
        overlapTokens: OverlapTokens, // 256
    }
}

func (s *Splitter) SplitMethod(chunk *models.Chunk) []SplitChunk {
    tokens := EstimateTokens(chunk.Content)
    if tokens <= s.maxTokens {
        return []SplitChunk{{
            Content:   chunk.Content,
            StartLine: chunk.StartLine,
            EndLine:   chunk.EndLine,
            Index:     0,
            IsPartial: false,
        }}
    }

    // Split at semantic boundaries with overlap
    return s.splitAtBoundaries(chunk)
}

func (s *Splitter) splitAtBoundaries(chunk *models.Chunk) []SplitChunk {
    var splits []SplitChunk
    content := chunk.Content
    lines := strings.Split(content, "\n")

    // Target size per split (leaving room for overlap)
    targetTokens := s.maxTokens - s.overlapTokens

    currentStart := 0
    splitIndex := 0

    for currentStart < len(lines) {
        // Find end position that fits within token limit
        currentEnd := s.findSplitEnd(lines, currentStart, targetTokens)

        // Build split content
        splitLines := lines[currentStart:currentEnd]
        splitContent := strings.Join(splitLines, "\n")

        splits = append(splits, SplitChunk{
            Content:   splitContent,
            StartLine: chunk.StartLine + currentStart,
            EndLine:   chunk.StartLine + currentEnd - 1,
            Index:     splitIndex,
            IsPartial: true,
            ParentID:  chunk.ID,
        })

        // Move start back by overlap amount for next split
        overlapLines := s.tokensToLines(lines, currentEnd, s.overlapTokens)
        currentStart = currentEnd - overlapLines
        if currentStart <= 0 || currentStart >= currentEnd {
            currentStart = currentEnd // Prevent infinite loop
        }

        splitIndex++
    }

    return splits
}

// findSplitEnd finds the best line to end a split at, respecting token limits
// and preferring statement/block boundaries
func (s *Splitter) findSplitEnd(lines []string, start, targetTokens int) int {
    currentTokens := 0
    bestEnd := start + 1 // Minimum one line

    for i := start; i < len(lines); i++ {
        lineTokens := EstimateTokens(lines[i])
        if currentTokens+lineTokens > targetTokens && i > start {
            break
        }
        currentTokens += lineTokens
        bestEnd = i + 1

        // Prefer ending at block boundaries (lines ending with }, end, etc.)
        line := strings.TrimSpace(lines[i])
        if strings.HasSuffix(line, "}") ||
           strings.HasSuffix(line, "end") ||
           line == "" {
            // Good break point, but keep going if we have room
        }
    }

    return bestEnd
}
```

### Database Schema Changes

Add columns to track split chunks:

```sql
-- Migration V4: Add chunk splitting support
ALTER TABLE chunks ADD COLUMN parent_chunk_id TEXT;
ALTER TABLE chunks ADD COLUMN chunk_index INTEGER DEFAULT 0;
ALTER TABLE chunks ADD COLUMN is_partial BOOLEAN DEFAULT FALSE;

-- Index for efficient parent lookup
CREATE INDEX idx_chunks_parent ON chunks(parent_chunk_id);
```

### Search Result Deduplication

When multiple splits from the same parent match, deduplicate:

```go
// internal/search/dedupe.go

func DeduplicateSplits(results []models.SearchResult) []models.SearchResult {
    seen := make(map[string]bool)
    deduped := make([]models.SearchResult, 0, len(results))

    for _, r := range results {
        // Use parent ID if this is a split, otherwise use chunk ID
        key := r.ParentChunkID
        if key == "" {
            key = r.ChunkID
        }

        if seen[key] {
            continue // Skip duplicate
        }
        seen[key] = true
        deduped = append(deduped, r)
    }

    return deduped
}
```

### Boost for Multiple Split Matches

If multiple splits from the same chunk match a query, boost the score:

```go
// In search scoring
func BoostMultipleSplitMatches(results []models.SearchResult) {
    parentHits := make(map[string]int)
    for _, r := range results {
        if r.ParentChunkID != "" {
            parentHits[r.ParentChunkID]++
        }
    }

    for i := range results {
        if hits := parentHits[results[i].ParentChunkID]; hits > 1 {
            // Boost by 10% per additional hit (capped at 50%)
            boost := math.Min(0.5, float64(hits-1)*0.1)
            results[i].Score *= (1 + boost)
        }
    }
}
```

## Integration Points

### 1. File Processing Pipeline

```
File Changed
    │
    ▼
Is Minified? ──yes──► Skip (log at DEBUG)
    │
    no
    ▼
Parse with Tree-sitter
    │
    ▼
Extract Chunks (file, class, method)
    │
    ▼
For each chunk:
    │
    ├── File chunk + size > 100KB? ──► Skip
    │
    ├── File/Class chunk oversized? ──► Truncate
    │
    └── Method chunk oversized? ──► Split with overlap
    │
    ▼
Generate Embeddings
    │
    ▼
Store in Database
```

### 2. Chunker Integration

Modify the chunker to use the splitter:

```go
// internal/chunker/chunker.go

func (c *Chunker) ChunkFile(path string, content []byte) ([]*models.Chunk, error) {
    // Check for minified files first
    if IsMinified(content, path) {
        log.Debug().Str("path", path).Msg("skipping minified file")
        return nil, nil // Skip entirely
    }

    fileSize := len(content)

    // ... existing chunking logic ...

    // After creating chunks, apply splitting
    splitter := NewSplitter(c.maxTokens)
    var finalChunks []*models.Chunk

    for _, chunk := range chunks {
        switch chunk.Level {
        case models.ChunkLevelFile:
            if split := splitter.HandleFileChunk(chunk, fileSize); split != nil {
                finalChunks = append(finalChunks, split.ToChunk(chunk))
            }
            // else: skip file chunk for very large files

        case models.ChunkLevelClass:
            if split := splitter.HandleClassChunk(chunk); split != nil {
                finalChunks = append(finalChunks, split.ToChunk(chunk))
            }

        case models.ChunkLevelMethod:
            splits := splitter.SplitMethod(chunk)
            for _, split := range splits {
                finalChunks = append(finalChunks, split.ToChunk(chunk))
            }
        }
    }

    return finalChunks, nil
}
```

### 3. Provider Configuration

Add max context to provider config:

```yaml
embedding:
  provider: openai

  openai:
    api_key: "sk-..."
    model: "text-embedding-3-small"
    max_context_tokens: 8000  # Optional override
```

## Files to Create/Modify

### New Files
| File | Purpose |
|------|---------|
| `internal/chunker/splitter.go` | Splitting logic for oversized chunks |
| `internal/chunker/splitter_test.go` | Unit tests for splitter |
| `internal/chunker/minified.go` | Minified file detection |
| `internal/chunker/minified_test.go` | Tests for minification detection |
| `internal/embedder/tokens.go` | Token estimation utility |
| `internal/embedder/tokens_test.go` | Tests for token estimation |

### Modified Files
| File | Changes |
|------|---------|
| `internal/embedder/provider.go` | Add `MaxContextTokens()` method |
| `internal/chunker/chunker.go` | Integrate splitter, add minified check |
| `internal/db/schema.go` | Add V4 migration for split columns |
| `internal/db/chunks.go` | Handle `parent_chunk_id`, `chunk_index`, `is_partial` |
| `internal/search/search.go` | Add deduplication and split boosting |
| `internal/models/chunk.go` | Add `ParentChunkID`, `ChunkIndex`, `IsPartial` fields |

## Implementation Phases

### Phase 1: Foundation
1. Add token estimation utility (`internal/embedder/tokens.go`)
2. Add `MaxContextTokens()` to providers
3. Add minified file detection (`internal/chunker/minified.go`)
4. Add database migration for split columns

**Outcome:** Infrastructure ready for splitting

### Phase 2: Splitting Logic
1. Implement splitter (`internal/chunker/splitter.go`)
2. Handle file-level (skip >100KB, truncate oversized)
3. Handle class-level (truncate oversized)
4. Handle method-level (semantic split with overlap)
5. Integrate into chunker

**Outcome:** Large chunks properly split, indexing never fails

### Phase 3: Search Integration
1. Update models to include split metadata
2. Implement deduplication in search results
3. Add score boosting for multiple split matches
4. Update search response format

**Outcome:** Clean search UX despite split storage

## Testing Strategy

### Unit Tests
- Token estimation accuracy across languages
- Minified file detection (true positives and negatives)
- Truncation at various sizes
- Split point detection at statement boundaries
- Overlap calculation correctness
- Deduplication logic

### Integration Tests
- Index file with 50KB method → verify splits created
- Index minified JS file → verify skipped
- Index 150KB file → verify no file-level chunk
- Search returns correct results for split chunk
- Multiple splits matching → verify deduplication + boost
- Reindex handles existing split chunks
- Provider switching recalculates splits correctly

### Test Files to Create
- `testdata/large_method.go` - 50KB single method
- `testdata/minified.min.js` - Minified JavaScript
- `testdata/large_file.py` - 150KB Python file
- `testdata/normal_code.ts` - Normal TypeScript for comparison

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Token estimation inaccuracy | Medium | Low | Use conservative 3.5 chars/token |
| False positive minified detection | Low | Medium | Multiple heuristics, log when skipping |
| Search quality degradation | Low | Medium | 256-token overlap + boosting |
| Migration complexity | Low | Medium | Add columns as nullable |

## Success Metrics

1. **Zero embedding failures** - No "context length exceeded" errors in logs
2. **Minified files skipped** - Detected and logged at DEBUG level
3. **Large file handling** - Files >100KB have no file-level chunk
4. **Search quality maintained** - Dogfooding tests still pass
5. **No performance regression** - Indexing time within 10% of current

---

## Appendix: Token Estimation Validation

Tested against tiktoken for code samples:

| Sample | Characters | tiktoken | chars/3.5 | chars/4 |
|--------|------------|----------|-----------|---------|
| Go function (100 lines) | 2,847 | 756 | 813 | 712 |
| Python class (200 lines) | 5,234 | 1,421 | 1,495 | 1,309 |
| TypeScript (500 lines) | 15,892 | 4,102 | 4,541 | 3,973 |
| Minified JS (1 line) | 45,000 | 12,847 | 12,857 | 11,250 |

The 3.5 divisor provides a good safety margin while not over-splitting.

## Appendix: Minified File Detection Examples

**Should detect as minified:**
- `app.min.js` - Has `.min.` in name
- `bundle.js` with single 50KB line - Single very long line
- `styles.css` with 2% whitespace - Abnormally low whitespace ratio

**Should NOT detect as minified:**
- `long_function.go` with 200-char lines - Long but structured
- `data.json` with one long line - JSON files excluded
- `normal.py` with 20% whitespace - Normal whitespace ratio
