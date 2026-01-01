# Improved Search Results Design

**Date:** 2026-01-01
**Branch:** `dev-improved-results`
**Version Target:** v0.5.0

## Overview

This design improves Pommel's search relevancy through two complementary strategies:

1. **Hybrid Search** - Combines vector similarity with keyword (BM25) search
2. **Re-ranking** - Second-stage scoring using cross-encoder models

Both features are enabled by default and work together to surface more relevant results.

## Architecture

```
Query
  │
  ▼
┌─────────────────────────────────────────────────┐
│           Stage 1: Hybrid Retrieval             │
│  ┌─────────────────┐   ┌─────────────────────┐  │
│  │  Vector Search  │   │  FTS5 Keyword Search│  │
│  │  (sqlite-vec)   │   │  (SQLite BM25)      │  │
│  └────────┬────────┘   └──────────┬──────────┘  │
│           │                       │             │
│           └───────────┬───────────┘             │
│                       ▼                         │
│              Reciprocal Rank Fusion             │
│              (merge & deduplicate)              │
└───────────────────────┬─────────────────────────┘
                        │
                        ▼ top-N candidates
┌─────────────────────────────────────────────────┐
│            Stage 2: Re-ranking                  │
│  ┌─────────────────────────────────────────┐    │
│  │  Primary: Ollama Cross-Encoder          │    │
│  │  Fallback: Heuristic Scoring            │    │
│  └─────────────────────────────────────────┘    │
└───────────────────────┬─────────────────────────┘
                        │
                        ▼ re-scored & sorted
┌─────────────────────────────────────────────────┐
│            Stage 3: Results                     │
│  • Final ranking by combined score              │
│  • Optional: score breakdown per signal         │
│  • Optional: metrics comparison vs grep         │
└─────────────────────────────────────────────────┘
```

## SQLite FTS5 Schema

### New Table

```sql
CREATE VIRTUAL TABLE chunks_fts USING fts5(
    chunk_id UNINDEXED,    -- Reference to chunks.id
    content,                -- Searchable text (code + docstrings)
    name,                   -- Function/class name for boosting
    file_path,              -- For path-based filtering
    tokenize='porter unicode61'  -- Stemming + unicode support
);
```

### Auto-Migration

On daemon startup, detect and handle missing FTS table:

```go
func (db *Database) EnsureFTSTable(ctx context.Context) error {
    exists, err := db.tableExists("chunks_fts")
    if err != nil {
        return err
    }
    if !exists {
        log.Info("FTS table missing, creating and triggering reindex")
        if err := db.createFTSTable(); err != nil {
            return err
        }
        // Trigger async reindex
        go db.indexer.ReindexAll(context.Background())
    }
    return nil
}
```

### Sync on Indexing

When chunks are inserted/updated/deleted, sync to FTS:

```sql
-- On insert/update
INSERT OR REPLACE INTO chunks_fts(chunk_id, content, name, file_path)
VALUES (?, ?, ?, ?);

-- On delete
DELETE FROM chunks_fts WHERE chunk_id = ?;
```

## Hybrid Retrieval

### Parallel Query Execution

```go
func (s *Search) HybridSearch(ctx context.Context, query string, limit int) ([]Result, error) {
    // Run both searches in parallel
    var vectorResults, keywordResults []Result
    g, ctx := errgroup.WithContext(ctx)

    g.Go(func() error {
        var err error
        vectorResults, err = s.vectorSearch(ctx, query, limit*2)
        return err
    })

    g.Go(func() error {
        var err error
        keywordResults, err = s.keywordSearch(ctx, query, limit*2)
        return err
    })

    if err := g.Wait(); err != nil {
        return nil, err
    }

    // Merge using RRF
    return s.reciprocalRankFusion(vectorResults, keywordResults, limit), nil
}
```

### Reciprocal Rank Fusion (RRF)

Merge strategy that doesn't require score normalization:

```go
const k = 60 // Standard RRF constant

func reciprocalRankFusion(vectorResults, keywordResults []Result, limit int) []Result {
    scores := make(map[string]float64) // chunk_id -> RRF score
    chunks := make(map[string]Result)  // chunk_id -> Result

    for rank, r := range vectorResults {
        scores[r.ChunkID] += 1.0 / float64(k + rank + 1)
        chunks[r.ChunkID] = r
    }

    for rank, r := range keywordResults {
        scores[r.ChunkID] += 1.0 / float64(k + rank + 1)
        if _, exists := chunks[r.ChunkID]; !exists {
            chunks[r.ChunkID] = r
        }
    }

    // Sort by RRF score, return top limit
    return sortByScore(scores, chunks, limit)
}
```

### FTS5 Query

```sql
SELECT chunk_id, bm25(chunks_fts) as score
FROM chunks_fts
WHERE chunks_fts MATCH ?
ORDER BY score
LIMIT ?
```

Query preprocessing extracts significant terms:
- "database connection handling" → `database OR connection OR handling`
- Remove stopwords, apply stemming via FTS5 tokenizer

## Re-ranker Integration

### Primary: Ollama Cross-Encoder

```go
type OllamaReranker struct {
    client *ollama.Client
    model  string // e.g., "cross-encoder/ms-marco-MiniLM-L-6-v2"
}

func (r *OllamaReranker) Rerank(ctx context.Context, query string, candidates []Result) ([]Result, error) {
    // Build prompt for cross-encoder scoring
    for i, candidate := range candidates {
        score, err := r.scoreQueryDocument(ctx, query, candidate.Content)
        if err != nil {
            return nil, err
        }
        candidates[i].RerankerScore = score
    }

    // Sort by reranker score
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].RerankerScore > candidates[j].RerankerScore
    })

    return candidates, nil
}
```

### Fallback: Heuristic Scoring

When Ollama is unavailable or times out:

```go
type HeuristicReranker struct{}

func (r *HeuristicReranker) Rerank(query string, candidates []Result) []Result {
    queryTerms := tokenize(strings.ToLower(query))

    for i, c := range candidates {
        score := c.VectorScore // Start with vector similarity

        // Boost: Name contains query terms
        if nameContainsTerms(c.Name, queryTerms) {
            score += 0.15
        }

        // Boost: Exact phrase match in content
        if strings.Contains(strings.ToLower(c.Content), strings.ToLower(query)) {
            score += 0.10
        }

        // Boost: Path relevance (e.g., query "auth" matches "auth/handler.go")
        if pathContainsTerms(c.FilePath, queryTerms) {
            score += 0.05
        }

        // Penalty: Test files (usually less relevant for implementation queries)
        if isTestFile(c.FilePath) {
            score -= 0.05
        }

        // Boost: Recently modified (fresher code may be more relevant)
        if c.ModifiedWithin(7 * 24 * time.Hour) {
            score += 0.02
        }

        candidates[i].FinalScore = score
    }

    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].FinalScore > candidates[j].FinalScore
    })

    return candidates
}
```

### Reranker Selection

```go
func (s *Search) getReranker() Reranker {
    if s.config.Reranker.Enabled {
        if s.ollamaAvailable() {
            return s.ollamaReranker
        }
        log.Debug("Ollama unavailable, falling back to heuristic reranker")
    }
    return s.heuristicReranker
}
```

## Configuration

### Config File (`config.yaml`)

```yaml
search:
  # Hybrid search settings
  hybrid_search:
    enabled: true           # Use both vector + keyword (default: true)
    vector_weight: 0.7      # Weight for vector results in RRF
    keyword_weight: 0.3     # Weight for keyword results in RRF
    rrf_k: 60               # RRF constant (default: 60)

  # Re-ranker settings
  reranker:
    enabled: true           # Enable re-ranking stage (default: true)
    model: "cross-encoder"  # Ollama model for reranking
    timeout: 2s             # Max time for reranker before fallback
    fallback: heuristic     # What to use if Ollama fails
    candidates: 20          # How many candidates to rerank

  # Result settings
  default_limit: 10         # Default number of results
  min_score: 0.3            # Minimum score threshold
```

### Defaults

Both hybrid search and re-ranker are **enabled by default** for best out-of-box experience. Users can disable via:

```bash
pm config set search.hybrid_search.enabled false
pm config set search.reranker.enabled false
```

## API & CLI Changes

### Search Response

```go
type SearchResult struct {
    ChunkID      string   `json:"chunk_id"`
    File         string   `json:"file"`
    StartLine    int      `json:"start_line"`
    EndLine      int      `json:"end_line"`
    Name         string   `json:"name"`
    ChunkType    string   `json:"chunk_type"`
    Content      string   `json:"content"`
    Score        float64  `json:"score"`          // Final combined score
    ScoreDetails *Details `json:"score_details"`  // Optional breakdown
    MatchReasons []string `json:"match_reasons"`  // Why this matched
}

type Details struct {
    VectorScore   float64 `json:"vector_score"`
    KeywordScore  float64 `json:"keyword_score,omitempty"`
    RRFScore      float64 `json:"rrf_score,omitempty"`
    RerankerScore float64 `json:"reranker_score,omitempty"`
}
```

### CLI Output

```
$ pm search "database connection handling" --verbose

Results for "database connection handling":

1. internal/db/connection.go:45-89 [function: NewConnection]
   Score: 0.847 (vector: 0.72, keyword: 0.65, reranker: +0.12)
   Matched: semantic similarity, "connection" in name, "database" in path

2. internal/db/pool.go:12-56 [function: NewPool]
   Score: 0.756 (vector: 0.68, keyword: 0.58, reranker: +0.08)
   Matched: semantic similarity, "connection" in content
```

### New CLI Flags

```
--verbose, -v      Show score breakdown and match reasons
--no-hybrid        Disable hybrid search (vector only)
--no-rerank        Disable re-ranking stage
--metrics          Show context/time savings vs grep
```

## Metrics & Benchmarking

### What to Measure

| Metric | Description |
|--------|-------------|
| `pommel_chars` | Total characters in Pommel results |
| `pommel_tokens` | Estimated tokens (~chars/4) |
| `pommel_files` | Unique files in results |
| `pommel_time_ms` | Search latency |
| `baseline_chars` | Characters grep/glob would return |
| `baseline_tokens` | Estimated baseline tokens |
| `baseline_files` | Files grep would match |
| `baseline_time_ms` | Time to run grep equivalent |
| `context_savings_pct` | `(1 - pommel_chars/baseline_chars) * 100` |
| `time_savings_pct` | `(1 - pommel_time/baseline_time) * 100` |
| `estimated_token_savings_pct` | `(1 - pommel_tokens/baseline_tokens) * 100` |

### Baseline Calculation

For fair comparison, actually run grep with query terms:

```go
// Extract significant terms from query
terms := extractSearchTerms(query) // e.g., "database connection" → ["database", "connection"]

// Run grep -l to find matching files
matchingFiles := grepFilesContaining(terms, projectRoot)

// Sum file sizes for context estimate
baselineChars := sumFileSizes(matchingFiles)
```

### API Types

```go
type SearchMetrics struct {
    PommelChars             int     `json:"pommel_chars"`
    PommelTokens            int     `json:"pommel_tokens"`
    PommelFiles             int     `json:"pommel_files"`
    PommelTimeMs            int64   `json:"pommel_time_ms"`
    BaselineChars           int     `json:"baseline_chars"`
    BaselineTokens          int     `json:"baseline_tokens"`
    BaselineFiles           int     `json:"baseline_files"`
    BaselineTimeMs          int64   `json:"baseline_time_ms"`
    ContextSavingsPct       float64 `json:"context_savings_pct"`
    TimeSavingsPct          float64 `json:"time_savings_pct"`
    EstimatedTokenSavingsPct float64 `json:"estimated_token_savings_pct"`
}

type SearchResponse struct {
    // ... existing fields
    Metrics *SearchMetrics `json:"metrics,omitempty"`
}
```

### CLI Output

```
$ pm search "database connection handling" --metrics

Query: "database connection handling"
Results: 5 chunks from 3 files (2,450 chars / ~612 tokens)

📊 Metrics:
   Pommel:   2,450 chars (~612 tokens) from 3 files in 145ms
   Baseline: 48,200 chars (~12,050 tokens) from 23 files in 890ms
   Savings:  94.9% less context, 94.9% fewer tokens, 83.7% faster
```

### Caching

Baseline measurement adds latency (~500ms-2s depending on codebase). Options:
- Only compute when `--metrics` flag is used (recommended)
- Cache baseline results for repeated queries
- Offer `--metrics=estimate` for faster approximation using FTS term frequency

## Testing Strategy

All features will be implemented using strict TDD.

### Unit Tests

- FTS5 table creation and population
- RRF score calculation with various rank combinations
- Heuristic re-ranker scoring (name match, phrase match, path match)
- Query preprocessing (stopword removal, term extraction)
- Metrics calculation (savings percentages)
- Baseline grep execution and measurement

### Integration Tests

- Hybrid retrieval returns results from both vector and keyword paths
- Auto-migration detects missing FTS table and triggers reindex
- Re-ranker fallback activates when Ollama unavailable
- Search with `--no-hybrid` uses vector-only path
- Metrics flag produces valid comparison data

### Quality Tests (using known corpus)

- Semantic queries ("authentication logic") should rank semantic matches high
- Exact term queries ("parseConfig") should surface keyword matches
- Mixed queries ("database connection retry") should benefit from both
- Irrelevant queries should score below threshold

### Performance Tests

- Hybrid search latency < 500ms for 10k chunks
- FTS5 query time < 50ms
- Re-ranker adds < 200ms for top-20 candidates
- Metrics calculation adds < 2s (acceptable since opt-in)

## Implementation Phases

### Phase 1: FTS5 Infrastructure
- Create FTS5 table schema
- Implement auto-migration detection
- Sync FTS on chunk insert/update/delete
- Add FTS query function

### Phase 2: Hybrid Search
- Implement parallel vector + keyword search
- Implement RRF merge
- Add `--no-hybrid` flag
- Configuration options

### Phase 3: Re-ranker
- Implement heuristic re-ranker
- Implement Ollama cross-encoder integration
- Fallback logic
- Add `--no-rerank` flag

### Phase 4: Enhanced Output
- Score details in response
- Match reasons
- `--verbose` flag
- Update JSON output

### Phase 5: Metrics
- Implement baseline measurement
- Calculate savings percentages
- Add `--metrics` flag
- Human-readable and JSON output

---

*Generated with Claude Code*
