# Phase 27: Hybrid Search

**Parent Design:** [2026-01-01-improved-search-design.md](./2026-01-01-improved-search-design.md)
**Methodology:** Strict TDD (Red-Green-Refactor)
**Branch:** `dev-improved-results`
**Depends On:** Phase 26 (FTS5 Infrastructure)

## Overview

Implement hybrid search that combines vector similarity search with FTS5 keyword search, merging results using Reciprocal Rank Fusion (RRF). This improves recall by capturing both semantic similarity and exact keyword matches.

## Goals

1. Execute vector and keyword searches in parallel
2. Merge results using Reciprocal Rank Fusion
3. Deduplicate results from both sources
4. Add configuration options for hybrid search
5. Add `--no-hybrid` CLI flag for vector-only mode

## File Changes

### New Files
- `internal/search/hybrid.go` - Hybrid search implementation
- `internal/search/hybrid_test.go` - Hybrid search tests
- `internal/search/rrf.go` - Reciprocal Rank Fusion algorithm
- `internal/search/rrf_test.go` - RRF tests
- `internal/search/query.go` - Query preprocessing
- `internal/search/query_test.go` - Query preprocessing tests

### Modified Files
- `internal/search/search.go` - Integrate hybrid search
- `internal/search/search_test.go` - Update search tests
- `internal/config/config.go` - Add hybrid search config
- `internal/cli/search.go` - Add `--no-hybrid` flag
- `internal/cli/search_test.go` - Test new flag
- `internal/api/types.go` - Add hybrid search fields to response

## Implementation Tasks

### 27.1 Query Preprocessing

**File:** `internal/search/query.go`

```go
// PreprocessQuery extracts search terms from a natural language query
func PreprocessQuery(query string) ProcessedQuery

type ProcessedQuery struct {
    Original     string   // Original query
    Terms        []string // Extracted significant terms
    FTSQuery     string   // Formatted for FTS5 (term1 OR term2 OR term3)
    HasPhrases   bool     // Contains quoted phrases
    Phrases      []string // Extracted quoted phrases
}

// Common stopwords to exclude
var stopwords = map[string]bool{
    "the": true, "a": true, "an": true, "and": true, "or": true,
    "but": true, "in": true, "on": true, "at": true, "to": true,
    "for": true, "of": true, "with": true, "by": true, "is": true,
    "it": true, "this": true, "that": true, "be": true, "as": true,
}
```

**Tests (27.1):**

| Test | Type | Description |
|------|------|-------------|
| `TestPreprocessQuery_SingleWord` | Happy path | "database" → terms: ["database"] |
| `TestPreprocessQuery_MultipleWords` | Happy path | "database connection" → terms: ["database", "connection"] |
| `TestPreprocessQuery_StopwordRemoval` | Success case | "the database" → terms: ["database"] |
| `TestPreprocessQuery_AllStopwords` | Edge case | "the a an" → terms: [] (empty) |
| `TestPreprocessQuery_CaseNormalization` | Success case | "Database" → terms: ["database"] |
| `TestPreprocessQuery_QuotedPhrase` | Success case | `"exact phrase"` preserved as phrase |
| `TestPreprocessQuery_MixedPhrasesAndTerms` | Success case | `"auth flow" database` → phrase + term |
| `TestPreprocessQuery_SpecialCharacters` | Edge case | Symbols stripped, terms preserved |
| `TestPreprocessQuery_Numbers` | Edge case | "error 404" → terms: ["error", "404"] |
| `TestPreprocessQuery_EmptyQuery` | Edge case | "" → empty ProcessedQuery |
| `TestPreprocessQuery_WhitespaceOnly` | Edge case | "   " → empty ProcessedQuery |
| `TestPreprocessQuery_FTSQueryFormat` | Success case | Generates valid FTS5 query syntax |
| `TestPreprocessQuery_FTSQueryWithPhrases` | Success case | Phrases quoted in FTS query |
| `TestPreprocessQuery_VeryLongQuery` | Edge case | 1000 char query handled |
| `TestPreprocessQuery_UnicodeTerms` | Edge case | Non-ASCII terms preserved |

---

### 27.2 Reciprocal Rank Fusion Algorithm

**File:** `internal/search/rrf.go`

```go
const DefaultRRFK = 60 // Standard RRF constant

// RRFMerge combines two ranked lists using Reciprocal Rank Fusion
func RRFMerge(vectorResults, keywordResults []RankedResult, k int, limit int) []MergedResult

type RankedResult struct {
    ChunkID string
    Score   float64 // Original score from source
    Rank    int     // 0-indexed rank in source list
}

type MergedResult struct {
    ChunkID      string
    RRFScore     float64 // Combined RRF score
    VectorRank   int     // Rank in vector results (-1 if not present)
    KeywordRank  int     // Rank in keyword results (-1 if not present)
    VectorScore  float64 // Original vector score
    KeywordScore float64 // Original keyword score
}
```

**Tests (27.2):**

| Test | Type | Description |
|------|------|-------------|
| `TestRRFMerge_BothHaveSameResult` | Happy path | Same chunk in both lists gets highest score |
| `TestRRFMerge_DisjointLists` | Happy path | No overlap, interleaved by score |
| `TestRRFMerge_VectorOnly` | Success case | Empty keyword list, returns vector results |
| `TestRRFMerge_KeywordOnly` | Success case | Empty vector list, returns keyword results |
| `TestRRFMerge_BothEmpty` | Edge case | Returns empty result |
| `TestRRFMerge_LimitRespected` | Success case | Returns at most `limit` results |
| `TestRRFMerge_LimitGreaterThanTotal` | Edge case | Returns all available |
| `TestRRFMerge_ZeroLimit` | Edge case | Returns empty or all |
| `TestRRFMerge_DifferentK` | Success case | k=30 vs k=60 changes ranking |
| `TestRRFMerge_ScoreCalculation` | Success case | RRF score = 1/(k+rank+1) summed |
| `TestRRFMerge_PreservesSourceScores` | Success case | VectorScore/KeywordScore preserved |
| `TestRRFMerge_PreservesRanks` | Success case | VectorRank/KeywordRank tracked |
| `TestRRFMerge_TopRankedInBothWins` | Success case | Rank 1 in both > Rank 1 in one |
| `TestRRFMerge_ManyResults` | Success case | 1000 results from each list |
| `TestRRFMerge_DuplicateChunkIDs` | Edge case | Same chunk multiple times in one list |
| `TestRRFMerge_StableOrdering` | Edge case | Same scores produce deterministic order |

---

### 27.3 Parallel Search Execution

**File:** `internal/search/hybrid.go`

```go
// HybridSearch executes vector and keyword searches in parallel
func (s *Searcher) HybridSearch(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)

type SearchOptions struct {
    Limit         int
    HybridEnabled bool    // Default: true
    RRFK          int     // Default: 60
    VectorWeight  float64 // For weighted RRF (future)
    KeywordWeight float64 // For weighted RRF (future)
}
```

**Tests (27.3):**

| Test | Type | Description |
|------|------|-------------|
| `TestHybridSearch_ReturnsResults` | Happy path | Query returns relevant chunks |
| `TestHybridSearch_CombinesVectorAndKeyword` | Success case | Results from both sources present |
| `TestHybridSearch_VectorOnlyWhenDisabled` | Success case | HybridEnabled=false uses vector only |
| `TestHybridSearch_ParallelExecution` | Success case | Both searches run concurrently (timing) |
| `TestHybridSearch_VectorFailure` | Failure case | Returns keyword results if vector fails |
| `TestHybridSearch_KeywordFailure` | Failure case | Returns vector results if keyword fails |
| `TestHybridSearch_BothFail` | Failure case | Returns error when both fail |
| `TestHybridSearch_ContextCancellation` | Failure case | Cancels both searches |
| `TestHybridSearch_ContextTimeout` | Failure case | Respects timeout |
| `TestHybridSearch_EmptyQuery` | Edge case | Returns empty or error |
| `TestHybridSearch_NoFTSTable` | Failure case | Falls back to vector-only gracefully |
| `TestHybridSearch_LimitApplied` | Success case | Final results respect limit |
| `TestHybridSearch_SemanticQueryBenefitsFromVector` | Quality | "authentication flow" finds semantic matches |
| `TestHybridSearch_ExactTermBenefitsFromKeyword` | Quality | "parseConfig" finds exact matches |
| `TestHybridSearch_MixedQueryBenefitsFromBoth` | Quality | Benefits from hybrid approach |

---

### 27.4 Configuration Options

**File:** `internal/config/config.go`

```go
type SearchConfig struct {
    HybridSearch HybridSearchConfig `yaml:"hybrid_search"`
    // ... existing fields
}

type HybridSearchConfig struct {
    Enabled       bool    `yaml:"enabled"`        // Default: true
    VectorWeight  float64 `yaml:"vector_weight"`  // Default: 0.7
    KeywordWeight float64 `yaml:"keyword_weight"` // Default: 0.3
    RRFK          int     `yaml:"rrf_k"`          // Default: 60
}
```

**Tests (27.4):**

| Test | Type | Description |
|------|------|-------------|
| `TestHybridConfig_DefaultsEnabled` | Happy path | Default config has hybrid enabled |
| `TestHybridConfig_LoadFromYAML` | Success case | Parses YAML config correctly |
| `TestHybridConfig_DisabledInConfig` | Success case | enabled: false respected |
| `TestHybridConfig_CustomRRFK` | Success case | Custom k value applied |
| `TestHybridConfig_InvalidRRFK` | Edge case | Negative k uses default |
| `TestHybridConfig_ZeroRRFK` | Edge case | Zero k uses default |
| `TestHybridConfig_WeightsOutOfRange` | Edge case | Weights clamped to 0-1 |
| `TestHybridConfig_MissingSection` | Edge case | Missing section uses defaults |

---

### 27.5 CLI Flag

**File:** `internal/cli/search.go`

```go
// Add flag
searchCmd.Flags().Bool("no-hybrid", false, "Disable hybrid search (vector only)")

// Usage in search handler
if noHybrid {
    opts.HybridEnabled = false
}
```

**Tests (27.5):**

| Test | Type | Description |
|------|------|-------------|
| `TestSearchCmd_NoHybridFlag` | Happy path | --no-hybrid disables hybrid search |
| `TestSearchCmd_DefaultHybridEnabled` | Success case | Without flag, hybrid is enabled |
| `TestSearchCmd_NoHybridShortForm` | Edge case | No short form (intentional) |
| `TestSearchCmd_HybridWithOtherFlags` | Success case | Combines with --limit, --json |

---

### 27.6 API Response Updates

**File:** `internal/api/types.go`

```go
type SearchResult struct {
    // ... existing fields
    MatchSource string `json:"match_source,omitempty"` // "vector", "keyword", or "both"
}

type SearchResponse struct {
    // ... existing fields
    HybridEnabled bool `json:"hybrid_enabled"`
}
```

**Tests (27.6):**

| Test | Type | Description |
|------|------|-------------|
| `TestSearchResponse_MatchSourceVector` | Success case | Vector-only result shows "vector" |
| `TestSearchResponse_MatchSourceKeyword` | Success case | Keyword-only result shows "keyword" |
| `TestSearchResponse_MatchSourceBoth` | Success case | Dual-match shows "both" |
| `TestSearchResponse_HybridEnabledField` | Success case | Response indicates hybrid status |
| `TestSearchResponse_JSONFormat` | Success case | JSON includes new fields |

---

## Integration Tests

| Test | Description |
|------|-------------|
| `TestHybridSearch_EndToEnd` | Full search through API with hybrid enabled |
| `TestHybridSearch_VectorOnlyEndToEnd` | Full search with --no-hybrid |
| `TestHybridSearch_AfterReindex` | Hybrid works after reindex populates FTS |
| `TestHybridSearch_NewChunksSearchable` | Newly indexed chunks appear in both paths |
| `TestHybridSearch_DeletedChunksRemoved` | Deleted chunks removed from both indexes |

---

## Test Execution Order

1. Query preprocessing tests (27.1) - Input handling
2. RRF algorithm tests (27.2) - Core algorithm
3. Parallel execution tests (27.3) - Integration
4. Config tests (27.4) - Configuration
5. CLI tests (27.5) - User interface
6. API tests (27.6) - Response format
7. Integration tests - End-to-end

## Success Criteria

- [ ] All tests pass
- [ ] Hybrid search enabled by default
- [ ] Vector and keyword searches execute in parallel
- [ ] RRF correctly merges and deduplicates results
- [ ] `--no-hybrid` flag disables keyword search
- [ ] Configuration options work correctly
- [ ] Graceful fallback when one search fails
- [ ] Search latency < 500ms for typical queries

## Dependencies

- Phase 26 (FTS5 Infrastructure) - FTSSearch function

## Dependents

- Phase 28 (Re-ranker) - Re-ranks hybrid results
- Phase 29 (Enhanced Output) - Score details include vector/keyword breakdown
