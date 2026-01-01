# Phase 28: Re-ranker

**Parent Design:** [2026-01-01-improved-search-design.md](./2026-01-01-improved-search-design.md)
**Methodology:** Strict TDD (Red-Green-Refactor)
**Branch:** `dev-improved-results`
**Depends On:** Phase 26 (FTS5 Infrastructure), Phase 27 (Hybrid Search)

## Overview

Implement a two-tier re-ranking system that scores search candidates more precisely. The primary re-ranker uses an Ollama cross-encoder model that sees query and document together. When Ollama is unavailable, a heuristic fallback provides reasonable re-ranking using code-aware signals.

## Goals

1. Implement heuristic re-ranker with code-aware signals
2. Implement Ollama cross-encoder re-ranker
3. Automatic fallback from Ollama to heuristics
4. Configuration options for re-ranking
5. Add `--no-rerank` CLI flag to disable re-ranking

## File Changes

### New Files
- `internal/rerank/reranker.go` - Reranker interface
- `internal/rerank/reranker_test.go` - Interface tests
- `internal/rerank/heuristic.go` - Heuristic re-ranker
- `internal/rerank/heuristic_test.go` - Heuristic tests
- `internal/rerank/ollama.go` - Ollama cross-encoder re-ranker
- `internal/rerank/ollama_test.go` - Ollama re-ranker tests
- `internal/rerank/signals.go` - Scoring signal functions
- `internal/rerank/signals_test.go` - Signal function tests

### Modified Files
- `internal/search/search.go` - Integrate re-ranker
- `internal/search/search_test.go` - Update search tests
- `internal/config/config.go` - Add re-ranker config
- `internal/cli/search.go` - Add `--no-rerank` flag
- `internal/cli/search_test.go` - Test new flag

## Implementation Tasks

### 28.1 Reranker Interface

**File:** `internal/rerank/reranker.go`

```go
// Reranker scores and reorders search candidates
type Reranker interface {
    // Rerank scores candidates and returns them sorted by relevance
    Rerank(ctx context.Context, query string, candidates []Candidate) ([]RankedCandidate, error)

    // Name returns the reranker type for logging/debugging
    Name() string

    // Available returns true if this reranker can be used
    Available(ctx context.Context) bool
}

type Candidate struct {
    ChunkID   string
    Content   string
    Name      string    // Function/class name
    FilePath  string
    ChunkType string    // "function", "class", "file", etc.
    BaseScore float64   // Score from hybrid search
    ModTime   time.Time // Last modification time
}

type RankedCandidate struct {
    Candidate
    FinalScore     float64            // Combined final score
    RerankerScore  float64            // Score from reranker alone
    SignalScores   map[string]float64 // Individual signal contributions
}
```

**Tests (28.1):**

| Test | Type | Description |
|------|------|-------------|
| `TestReranker_InterfaceCompliance` | Happy path | Both implementations satisfy interface |
| `TestCandidate_RequiredFields` | Success case | ChunkID and Content required |
| `TestRankedCandidate_PreservesCandidate` | Success case | Original fields preserved |
| `TestRankedCandidate_SignalScoresOptional` | Edge case | SignalScores can be nil |

---

### 28.2 Heuristic Scoring Signals

**File:** `internal/rerank/signals.go`

```go
// Signal functions return a score boost/penalty in range [-0.2, 0.2]

// NameMatchSignal boosts results where name contains query terms
func NameMatchSignal(name string, queryTerms []string) float64

// ExactPhraseSignal boosts results containing exact query phrase
func ExactPhraseSignal(content string, query string) float64

// PathMatchSignal boosts results where file path contains query terms
func PathMatchSignal(filePath string, queryTerms []string) float64

// TestFilePenalty reduces score for test files
func TestFilePenalty(filePath string) float64

// RecencyBoost slightly boosts recently modified files
func RecencyBoost(modTime time.Time, now time.Time) float64

// ChunkTypeSignal adjusts based on chunk type relevance
func ChunkTypeSignal(chunkType string, query string) float64
```

**Tests (28.2):**

| Test | Type | Description |
|------|------|-------------|
| `TestNameMatchSignal_ExactMatch` | Happy path | "parseConfig" in name → max boost |
| `TestNameMatchSignal_PartialMatch` | Success case | "parse" matches "parseConfig" |
| `TestNameMatchSignal_NoMatch` | Success case | Returns 0 for no match |
| `TestNameMatchSignal_CaseInsensitive` | Success case | "PARSE" matches "parse" |
| `TestNameMatchSignal_MultipleTerms` | Success case | More term matches → higher boost |
| `TestNameMatchSignal_EmptyName` | Edge case | Returns 0 for empty name |
| `TestNameMatchSignal_EmptyTerms` | Edge case | Returns 0 for empty terms |
| `TestExactPhraseSignal_Found` | Happy path | Exact phrase → boost |
| `TestExactPhraseSignal_NotFound` | Success case | Returns 0 when not found |
| `TestExactPhraseSignal_CaseInsensitive` | Success case | Case-insensitive match |
| `TestExactPhraseSignal_PartialNotCounted` | Edge case | Partial phrase → 0 |
| `TestExactPhraseSignal_EmptyQuery` | Edge case | Returns 0 for empty query |
| `TestPathMatchSignal_DirectoryMatch` | Happy path | "auth" matches "auth/handler.go" |
| `TestPathMatchSignal_FileNameMatch` | Success case | "handler" matches "handler.go" |
| `TestPathMatchSignal_NoMatch` | Success case | Returns 0 for no match |
| `TestPathMatchSignal_MultipleSegments` | Success case | Multiple matches → higher boost |
| `TestTestFilePenalty_TestFile` | Happy path | "_test.go" → penalty |
| `TestTestFilePenalty_TestDirectory` | Success case | "test/" or "tests/" → penalty |
| `TestTestFilePenalty_SpecFile` | Success case | ".spec.ts" → penalty |
| `TestTestFilePenalty_NotTestFile` | Success case | Returns 0 for production code |
| `TestTestFilePenalty_MockFile` | Edge case | "mock_" or "_mock" → smaller penalty |
| `TestRecencyBoost_VeryRecent` | Happy path | Modified today → max boost |
| `TestRecencyBoost_LastWeek` | Success case | Modified 7 days ago → medium boost |
| `TestRecencyBoost_OldFile` | Success case | Modified 30+ days ago → no boost |
| `TestRecencyBoost_FutureTime` | Edge case | Future time → no boost |
| `TestRecencyBoost_ZeroTime` | Edge case | Zero time → no boost |
| `TestChunkTypeSignal_FunctionForVerb` | Success case | "handle" query → function boost |
| `TestChunkTypeSignal_ClassForNoun` | Success case | "handler" query → class boost |
| `TestChunkTypeSignal_Neutral` | Success case | Ambiguous query → no adjustment |

---

### 28.3 Heuristic Re-ranker

**File:** `internal/rerank/heuristic.go`

```go
type HeuristicReranker struct {
    signals []SignalFunc
}

func NewHeuristicReranker() *HeuristicReranker

func (r *HeuristicReranker) Rerank(ctx context.Context, query string, candidates []Candidate) ([]RankedCandidate, error)

func (r *HeuristicReranker) Name() string // Returns "heuristic"

func (r *HeuristicReranker) Available(ctx context.Context) bool // Always true
```

**Tests (28.3):**

| Test | Type | Description |
|------|------|-------------|
| `TestHeuristicReranker_Name` | Happy path | Returns "heuristic" |
| `TestHeuristicReranker_Available` | Happy path | Always returns true |
| `TestHeuristicReranker_EmptyCandidates` | Edge case | Returns empty slice |
| `TestHeuristicReranker_SingleCandidate` | Happy path | Returns single result scored |
| `TestHeuristicReranker_MultipleCandidates` | Happy path | Returns sorted by score |
| `TestHeuristicReranker_NameMatchBoostsRanking` | Success case | Name match moves result up |
| `TestHeuristicReranker_ExactPhraseBoostsRanking` | Success case | Exact phrase moves result up |
| `TestHeuristicReranker_TestFileDemoted` | Success case | Test file moves down |
| `TestHeuristicReranker_CombinedSignals` | Success case | Multiple signals combine correctly |
| `TestHeuristicReranker_PreservesBaseScore` | Success case | BaseScore influences final |
| `TestHeuristicReranker_SignalScoresPopulated` | Success case | SignalScores map filled |
| `TestHeuristicReranker_ContextRespected` | Failure case | Cancelled context returns error |
| `TestHeuristicReranker_StableOrdering` | Edge case | Equal scores → deterministic order |
| `TestHeuristicReranker_NilSignals` | Edge case | Missing optional fields handled |
| `TestHeuristicReranker_VeryLongContent` | Edge case | 100KB content handled efficiently |

---

### 28.4 Ollama Cross-Encoder Re-ranker

**File:** `internal/rerank/ollama.go`

```go
type OllamaReranker struct {
    client  *ollama.Client
    model   string
    timeout time.Duration
}

func NewOllamaReranker(client *ollama.Client, model string, timeout time.Duration) *OllamaReranker

func (r *OllamaReranker) Rerank(ctx context.Context, query string, candidates []Candidate) ([]RankedCandidate, error)

func (r *OllamaReranker) Name() string // Returns "ollama"

func (r *OllamaReranker) Available(ctx context.Context) bool // Checks Ollama connectivity
```

**Tests (28.4):**

| Test | Type | Description |
|------|------|-------------|
| `TestOllamaReranker_Name` | Happy path | Returns "ollama" |
| `TestOllamaReranker_Available_Running` | Happy path | Returns true when Ollama responds |
| `TestOllamaReranker_Available_NotRunning` | Failure case | Returns false when Ollama down |
| `TestOllamaReranker_Available_Timeout` | Failure case | Returns false on timeout |
| `TestOllamaReranker_Rerank_Success` | Happy path | Returns scored and sorted results |
| `TestOllamaReranker_Rerank_EmptyCandidates` | Edge case | Returns empty slice |
| `TestOllamaReranker_Rerank_SingleCandidate` | Success case | Single candidate scored |
| `TestOllamaReranker_Rerank_ModelNotFound` | Failure case | Returns error for missing model |
| `TestOllamaReranker_Rerank_OllamaError` | Failure case | Returns error on API failure |
| `TestOllamaReranker_Rerank_Timeout` | Failure case | Returns error on timeout |
| `TestOllamaReranker_Rerank_ContextCancelled` | Failure case | Returns error on cancellation |
| `TestOllamaReranker_Rerank_LongContent` | Success case | Truncates very long content |
| `TestOllamaReranker_Rerank_BatchProcessing` | Success case | Processes candidates in batches |
| `TestOllamaReranker_ScoreNormalization` | Success case | Scores normalized to 0-1 range |

---

### 28.5 Fallback Logic

**File:** `internal/rerank/reranker.go`

```go
// FallbackReranker tries primary reranker, falls back to secondary
type FallbackReranker struct {
    primary   Reranker
    secondary Reranker
    timeout   time.Duration
}

func NewFallbackReranker(primary, secondary Reranker, timeout time.Duration) *FallbackReranker

func (r *FallbackReranker) Rerank(ctx context.Context, query string, candidates []Candidate) ([]RankedCandidate, error)

func (r *FallbackReranker) Name() string // Returns "primary->secondary" format
```

**Tests (28.5):**

| Test | Type | Description |
|------|------|-------------|
| `TestFallbackReranker_UsesPrimaryWhenAvailable` | Happy path | Primary used when available |
| `TestFallbackReranker_FallsBackWhenUnavailable` | Happy path | Secondary used when primary unavailable |
| `TestFallbackReranker_FallsBackOnPrimaryError` | Failure case | Secondary used on primary error |
| `TestFallbackReranker_FallsBackOnPrimaryTimeout` | Failure case | Secondary used on timeout |
| `TestFallbackReranker_ReturnsErrorWhenBothFail` | Failure case | Error when both fail |
| `TestFallbackReranker_Name` | Success case | Returns combined name |
| `TestFallbackReranker_LogsFallback` | Success case | Logs when falling back |
| `TestFallbackReranker_ContextPropagated` | Success case | Context passed to both |

---

### 28.6 Configuration Options

**File:** `internal/config/config.go`

```go
type RerankerConfig struct {
    Enabled    bool          `yaml:"enabled"`    // Default: true
    Model      string        `yaml:"model"`      // Ollama model name
    Timeout    time.Duration `yaml:"timeout"`    // Default: 2s
    Fallback   string        `yaml:"fallback"`   // "heuristic" or "none"
    Candidates int           `yaml:"candidates"` // How many to rerank (default: 20)
}
```

**Tests (28.6):**

| Test | Type | Description |
|------|------|-------------|
| `TestRerankerConfig_DefaultsEnabled` | Happy path | Enabled by default |
| `TestRerankerConfig_DefaultTimeout` | Happy path | 2s default timeout |
| `TestRerankerConfig_DefaultCandidates` | Happy path | 20 candidates default |
| `TestRerankerConfig_LoadFromYAML` | Success case | Parses YAML correctly |
| `TestRerankerConfig_DisabledInConfig` | Success case | enabled: false respected |
| `TestRerankerConfig_CustomTimeout` | Success case | Custom timeout applied |
| `TestRerankerConfig_InvalidTimeout` | Edge case | Negative timeout uses default |
| `TestRerankerConfig_ZeroCandidates` | Edge case | Zero uses default |
| `TestRerankerConfig_FallbackNone` | Success case | No fallback when "none" |

---

### 28.7 CLI Flag

**File:** `internal/cli/search.go`

```go
// Add flag
searchCmd.Flags().Bool("no-rerank", false, "Disable re-ranking stage")

// Usage in search handler
if noRerank {
    opts.RerankEnabled = false
}
```

**Tests (28.7):**

| Test | Type | Description |
|------|------|-------------|
| `TestSearchCmd_NoRerankFlag` | Happy path | --no-rerank disables re-ranking |
| `TestSearchCmd_DefaultRerankEnabled` | Success case | Without flag, rerank is enabled |
| `TestSearchCmd_CombinedNoHybridNoRerank` | Success case | Both flags work together |

---

### 28.8 Search Integration

**File:** `internal/search/search.go`

```go
func (s *Searcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
    // 1. Hybrid search
    candidates := s.HybridSearch(ctx, query, opts)

    // 2. Re-rank if enabled
    if opts.RerankEnabled {
        reranker := s.getReranker()
        candidates = reranker.Rerank(ctx, query, candidates)
    }

    // 3. Return top results
    return candidates[:opts.Limit], nil
}
```

**Tests (28.8):**

| Test | Type | Description |
|------|------|-------------|
| `TestSearch_WithReranking` | Happy path | Results are reranked |
| `TestSearch_WithoutReranking` | Success case | Rerank disabled skips stage |
| `TestSearch_RerankImprovesBadResult` | Quality | Low-ranked semantic match boosted |
| `TestSearch_RerankDemotesTestFile` | Quality | Test file demoted by heuristics |
| `TestSearch_RerankPreservesOrder` | Success case | Good results stay at top |
| `TestSearch_RerankFallbackWorks` | Failure case | Heuristics used when Ollama down |

---

## Integration Tests

| Test | Description |
|------|-------------|
| `TestReranker_EndToEnd` | Full search with re-ranking through API |
| `TestReranker_OllamaFallback_EndToEnd` | Fallback activates when Ollama stopped |
| `TestReranker_ConfigChange_EndToEnd` | Config change takes effect |
| `TestReranker_PerformanceAcceptable` | Re-ranking adds < 200ms for 20 candidates |

---

## Test Execution Order

1. Signal function tests (28.2) - Building blocks
2. Interface tests (28.1) - Contract definition
3. Heuristic reranker tests (28.3) - Always-available implementation
4. Ollama reranker tests (28.4) - Primary implementation
5. Fallback logic tests (28.5) - Combining rerankers
6. Config tests (28.6) - Configuration
7. CLI tests (28.7) - User interface
8. Integration tests (28.8) - End-to-end

## Success Criteria

- [ ] All tests pass
- [ ] Re-ranker enabled by default
- [ ] Heuristic re-ranker always available
- [ ] Ollama re-ranker works when Ollama running
- [ ] Automatic fallback when Ollama unavailable
- [ ] `--no-rerank` flag disables re-ranking
- [ ] Re-ranking improves result quality
- [ ] Re-ranking adds < 200ms latency for 20 candidates

## Dependencies

- Phase 26 (FTS5 Infrastructure)
- Phase 27 (Hybrid Search)

## Dependents

- Phase 29 (Enhanced Output) - Displays reranker scores
