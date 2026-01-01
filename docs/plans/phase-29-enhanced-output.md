# Phase 29: Enhanced Output

**Parent Design:** [2026-01-01-improved-search-design.md](./2026-01-01-improved-search-design.md)
**Methodology:** Strict TDD (Red-Green-Refactor)
**Branch:** `dev-improved-results`
**Depends On:** Phase 27 (Hybrid Search), Phase 28 (Re-ranker)

## Overview

Enhance search output to provide transparency into how results were ranked. Users and AI agents can see score breakdowns, match reasons, and detailed signal contributions. This helps with debugging search quality and understanding why certain results appear.

## Goals

1. Add score breakdown (vector, keyword, reranker scores)
2. Add human-readable match reasons
3. Implement `--verbose` flag for detailed output
4. Update JSON output with new fields
5. Improve human-readable output formatting

## File Changes

### New Files
- `internal/output/formatter.go` - Output formatting logic
- `internal/output/formatter_test.go` - Formatter tests
- `internal/output/reasons.go` - Match reason generation
- `internal/output/reasons_test.go` - Reason generation tests

### Modified Files
- `internal/api/types.go` - Add score details and match reasons
- `internal/api/types_test.go` - Test new type serialization
- `internal/cli/search.go` - Add `--verbose` flag
- `internal/cli/search_test.go` - Test verbose output
- `internal/search/search.go` - Populate new fields
- `internal/daemon/handlers.go` - Include details in response

## Implementation Tasks

### 29.1 API Type Updates

**File:** `internal/api/types.go`

```go
type SearchResult struct {
    ChunkID      string        `json:"chunk_id"`
    File         string        `json:"file"`
    StartLine    int           `json:"start_line"`
    EndLine      int           `json:"end_line"`
    Name         string        `json:"name,omitempty"`
    ChunkType    string        `json:"chunk_type"`
    Content      string        `json:"content"`
    Score        float64       `json:"score"`
    ScoreDetails *ScoreDetails `json:"score_details,omitempty"`
    MatchReasons []string      `json:"match_reasons,omitempty"`
    MatchSource  string        `json:"match_source,omitempty"` // "vector", "keyword", "both"
}

type ScoreDetails struct {
    VectorScore   float64            `json:"vector_score,omitempty"`
    KeywordScore  float64            `json:"keyword_score,omitempty"`
    RRFScore      float64            `json:"rrf_score,omitempty"`
    RerankerScore float64            `json:"reranker_score,omitempty"`
    SignalScores  map[string]float64 `json:"signal_scores,omitempty"`
}

type SearchResponse struct {
    Results       []SearchResult `json:"results"`
    Query         string         `json:"query"`
    TotalResults  int            `json:"total_results"`
    SearchTimeMs  int64          `json:"search_time_ms"`
    HybridEnabled bool           `json:"hybrid_enabled"`
    RerankEnabled bool           `json:"rerank_enabled"`
}
```

**Tests (29.1):**

| Test | Type | Description |
|------|------|-------------|
| `TestSearchResult_JSONSerialization` | Happy path | All fields serialize correctly |
| `TestSearchResult_OmitEmptyScoreDetails` | Success case | Nil ScoreDetails omitted |
| `TestSearchResult_OmitEmptyMatchReasons` | Success case | Empty slice omitted |
| `TestScoreDetails_AllFieldsPresent` | Success case | All score types serialize |
| `TestScoreDetails_PartialFields` | Success case | Missing fields omitted |
| `TestScoreDetails_SignalScoresMap` | Success case | Map serializes as object |
| `TestSearchResponse_HybridAndRerankFlags` | Success case | Boolean flags serialize |
| `TestSearchResult_Deserialization` | Success case | JSON deserializes correctly |
| `TestScoreDetails_ZeroValues` | Edge case | Zero vs omitted handling |

---

### 29.2 Match Reason Generation

**File:** `internal/output/reasons.go`

```go
// GenerateMatchReasons creates human-readable explanations of why a result matched
func GenerateMatchReasons(result *SearchResult, query string, details *ScoreDetails) []string

// Possible reasons:
// - "semantic similarity"
// - "contains 'query term' in name"
// - "contains 'query term' in content"
// - "file path contains 'query term'"
// - "exact phrase match"
// - "keyword match via BM25"
```

**Tests (29.2):**

| Test | Type | Description |
|------|------|-------------|
| `TestGenerateMatchReasons_SemanticOnly` | Happy path | Vector-only result → "semantic similarity" |
| `TestGenerateMatchReasons_KeywordOnly` | Happy path | Keyword-only result → "keyword match" |
| `TestGenerateMatchReasons_Both` | Success case | Both sources → both reasons |
| `TestGenerateMatchReasons_NameMatch` | Success case | Name contains term → name reason |
| `TestGenerateMatchReasons_PathMatch` | Success case | Path contains term → path reason |
| `TestGenerateMatchReasons_ExactPhrase` | Success case | Exact query in content → phrase reason |
| `TestGenerateMatchReasons_MultipleTerms` | Success case | Multiple terms → multiple reasons |
| `TestGenerateMatchReasons_NoMatch` | Edge case | No clear match → "semantic similarity" |
| `TestGenerateMatchReasons_EmptyQuery` | Edge case | Empty query → minimal reasons |
| `TestGenerateMatchReasons_CaseInsensitive` | Success case | Matching is case-insensitive |
| `TestGenerateMatchReasons_Deduplication` | Edge case | No duplicate reasons |
| `TestGenerateMatchReasons_MaxReasons` | Edge case | Limits to reasonable number (e.g., 5) |
| `TestGenerateMatchReasons_SignalBased` | Success case | Uses signal scores for reasons |

---

### 29.3 Output Formatter

**File:** `internal/output/formatter.go`

```go
type Formatter struct {
    verbose bool
    json    bool
}

func NewFormatter(verbose, json bool) *Formatter

// FormatResults formats search results for display
func (f *Formatter) FormatResults(response *SearchResponse) string

// FormatResult formats a single result
func (f *Formatter) FormatResult(result *SearchResult, rank int) string

// FormatScoreDetails formats score breakdown for verbose mode
func (f *Formatter) FormatScoreDetails(details *ScoreDetails) string
```

**Tests (29.3):**

| Test | Type | Description |
|------|------|-------------|
| `TestFormatter_BasicOutput` | Happy path | Normal mode shows file:line, name, score |
| `TestFormatter_VerboseOutput` | Happy path | Verbose shows score breakdown |
| `TestFormatter_JSONOutput` | Happy path | JSON mode outputs valid JSON |
| `TestFormatter_EmptyResults` | Edge case | No results → appropriate message |
| `TestFormatter_SingleResult` | Success case | Single result formatted correctly |
| `TestFormatter_MultipleResults` | Success case | Multiple results numbered |
| `TestFormatter_LongFileName` | Edge case | Long paths handled/truncated |
| `TestFormatter_LongContent` | Edge case | Content preview truncated |
| `TestFormatter_SpecialCharactersInContent` | Edge case | Code with symbols displayed |
| `TestFormatter_ScoreFormatting` | Success case | Scores show 3 decimal places |
| `TestFormatter_VerboseWithNoDetails` | Edge case | Verbose with nil details handled |
| `TestFormatter_MatchReasonsDisplay` | Success case | Reasons shown in verbose mode |
| `TestFormatter_ColorOutput` | Success case | Terminal colors applied (if tty) |
| `TestFormatter_NoColorOutput` | Success case | No colors when piped |
| `TestFormatter_SearchTimeDisplay` | Success case | Shows search time in ms |
| `TestFormatter_HybridIndicator` | Success case | Shows if hybrid was enabled |

---

### 29.4 Verbose Flag

**File:** `internal/cli/search.go`

```go
// Add flag
searchCmd.Flags().BoolP("verbose", "v", false, "Show detailed score breakdown")

// Usage
verbose, _ := cmd.Flags().GetBool("verbose")
formatter := output.NewFormatter(verbose, jsonOutput)
```

**Tests (29.4):**

| Test | Type | Description |
|------|------|-------------|
| `TestSearchCmd_VerboseFlag` | Happy path | --verbose shows score details |
| `TestSearchCmd_VerboseShortFlag` | Happy path | -v works as short form |
| `TestSearchCmd_VerboseWithJSON` | Success case | Verbose + JSON includes details |
| `TestSearchCmd_VerboseShowsReasons` | Success case | Match reasons displayed |
| `TestSearchCmd_VerboseShowsScoreBreakdown` | Success case | Vector/keyword/reranker shown |
| `TestSearchCmd_DefaultNotVerbose` | Success case | Without flag, compact output |
| `TestSearchCmd_VerboseWithLimit` | Success case | Combines with --limit |

---

### 29.5 Search Integration - Populate Details

**File:** `internal/search/search.go`

```go
// After hybrid search and reranking, populate score details
func (s *Searcher) populateScoreDetails(results []SearchResult, hybridResults []HybridResult, rerankResults []RankedCandidate) {
    for i := range results {
        results[i].ScoreDetails = &ScoreDetails{
            VectorScore:   hybridResults[i].VectorScore,
            KeywordScore:  hybridResults[i].KeywordScore,
            RRFScore:      hybridResults[i].RRFScore,
            RerankerScore: rerankResults[i].RerankerScore,
            SignalScores:  rerankResults[i].SignalScores,
        }
    }
}
```

**Tests (29.5):**

| Test | Type | Description |
|------|------|-------------|
| `TestPopulateScoreDetails_AllFields` | Happy path | All scores populated |
| `TestPopulateScoreDetails_HybridOnly` | Success case | No rerank → no reranker score |
| `TestPopulateScoreDetails_VectorOnly` | Success case | No hybrid → no keyword score |
| `TestPopulateScoreDetails_SignalScores` | Success case | Signal map populated |
| `TestPopulateScoreDetails_Mismatch` | Edge case | Result count mismatch handled |

---

### 29.6 Daemon Handler Updates

**File:** `internal/daemon/handlers.go`

```go
func (d *Daemon) handleSearch(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...

    // Include details in response
    response := &api.SearchResponse{
        Results:       results,
        Query:         query,
        TotalResults:  len(results),
        SearchTimeMs:  elapsed.Milliseconds(),
        HybridEnabled: opts.HybridEnabled,
        RerankEnabled: opts.RerankEnabled,
    }

    // Generate match reasons for each result
    for i := range response.Results {
        response.Results[i].MatchReasons = output.GenerateMatchReasons(
            &response.Results[i],
            query,
            response.Results[i].ScoreDetails,
        )
    }

    // ... return response ...
}
```

**Tests (29.6):**

| Test | Type | Description |
|------|------|-------------|
| `TestHandleSearch_IncludesScoreDetails` | Happy path | API returns score details |
| `TestHandleSearch_IncludesMatchReasons` | Happy path | API returns match reasons |
| `TestHandleSearch_IncludesSearchTime` | Success case | Search time in response |
| `TestHandleSearch_IncludesHybridStatus` | Success case | Hybrid enabled flag |
| `TestHandleSearch_IncludesRerankStatus` | Success case | Rerank enabled flag |
| `TestHandleSearch_NoDetailsWhenDisabled` | Success case | Details omitted if not computed |

---

## Sample Output

### Normal Mode
```
$ pm search "database connection"

Results for "database connection":

1. internal/db/connection.go:45-89 [function: NewConnection]
   Score: 0.847

2. internal/db/pool.go:12-56 [function: NewPool]
   Score: 0.756

3. internal/config/database.go:20-45 [struct: DatabaseConfig]
   Score: 0.698

Found 3 results in 142ms (hybrid: on, rerank: on)
```

### Verbose Mode
```
$ pm search "database connection" --verbose

Results for "database connection":

1. internal/db/connection.go:45-89 [function: NewConnection]
   Score: 0.847 (vector: 0.72, keyword: 0.65, reranker: +0.12)
   Matched: semantic similarity, "connection" in name, "database" in path

2. internal/db/pool.go:12-56 [function: NewPool]
   Score: 0.756 (vector: 0.68, keyword: 0.58, reranker: +0.08)
   Matched: semantic similarity, "connection" in content

3. internal/config/database.go:20-45 [struct: DatabaseConfig]
   Score: 0.698 (vector: 0.61, keyword: 0.72, reranker: +0.03)
   Matched: keyword match, "database" in name and path

Found 3 results in 142ms (hybrid: on, rerank: on)
```

### JSON Mode (with verbose)
```json
{
  "results": [
    {
      "chunk_id": "abc123",
      "file": "internal/db/connection.go",
      "start_line": 45,
      "end_line": 89,
      "name": "NewConnection",
      "chunk_type": "function",
      "score": 0.847,
      "score_details": {
        "vector_score": 0.72,
        "keyword_score": 0.65,
        "rrf_score": 0.031,
        "reranker_score": 0.12,
        "signal_scores": {
          "name_match": 0.15,
          "path_match": 0.05
        }
      },
      "match_reasons": [
        "semantic similarity",
        "contains 'connection' in name",
        "file path contains 'database'"
      ],
      "match_source": "both"
    }
  ],
  "query": "database connection",
  "total_results": 3,
  "search_time_ms": 142,
  "hybrid_enabled": true,
  "rerank_enabled": true
}
```

---

## Integration Tests

| Test | Description |
|------|-------------|
| `TestEnhancedOutput_EndToEnd` | Full search with verbose output |
| `TestEnhancedOutput_JSONEndToEnd` | Full search with JSON + verbose |
| `TestEnhancedOutput_APIEndToEnd` | API returns all new fields |
| `TestEnhancedOutput_MatchReasonsAccurate` | Reasons match actual signals |
| `TestEnhancedOutput_ScoresConsistent` | Score breakdown sums correctly |

---

## Test Execution Order

1. API type tests (29.1) - Data structures
2. Match reason tests (29.2) - Reason generation
3. Formatter tests (29.3) - Output formatting
4. CLI flag tests (29.4) - User interface
5. Search integration tests (29.5) - Data population
6. Handler tests (29.6) - API integration
7. Integration tests - End-to-end

## Success Criteria

- [ ] All tests pass
- [ ] Score details available in API response
- [ ] Match reasons generated accurately
- [ ] `--verbose` flag shows detailed output
- [ ] JSON output includes all new fields
- [ ] Human-readable output is clear and concise
- [ ] Verbose mode doesn't break piped output

## Dependencies

- Phase 27 (Hybrid Search) - Vector/keyword scores
- Phase 28 (Re-ranker) - Reranker scores and signals

## Dependents

- Phase 30 (Metrics) - Uses enhanced output format
