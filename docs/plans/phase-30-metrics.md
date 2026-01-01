# Phase 30: Metrics & Benchmarking

**Parent Design:** [2026-01-01-improved-search-design.md](./2026-01-01-improved-search-design.md)
**Methodology:** Strict TDD (Red-Green-Refactor)
**Branch:** `dev-improved-results`
**Depends On:** Phase 29 (Enhanced Output)

## Overview

Implement metrics and benchmarking to measure context savings compared to traditional grep/glob approaches. The `--metrics` flag runs a baseline grep comparison and calculates savings percentages, providing concrete evidence of Pommel's value for AI agents.

## Goals

1. Measure Pommel result size (chars, tokens, files)
2. Run baseline grep to measure what traditional search returns
3. Calculate context, token, and time savings percentages
4. Add `--metrics` CLI flag
5. Display metrics in both human-readable and JSON formats

## File Changes

### New Files
- `internal/metrics/metrics.go` - Core metrics types and calculation
- `internal/metrics/metrics_test.go` - Metrics calculation tests
- `internal/metrics/baseline.go` - Grep baseline measurement
- `internal/metrics/baseline_test.go` - Baseline measurement tests
- `internal/metrics/tokenizer.go` - Token estimation
- `internal/metrics/tokenizer_test.go` - Tokenizer tests

### Modified Files
- `internal/api/types.go` - Add SearchMetrics type
- `internal/cli/search.go` - Add `--metrics` flag
- `internal/cli/search_test.go` - Test metrics output
- `internal/daemon/handlers.go` - Handle metrics request
- `internal/output/formatter.go` - Format metrics output

## Implementation Tasks

### 30.1 Token Estimation

**File:** `internal/metrics/tokenizer.go`

```go
// EstimateTokens estimates the number of tokens in text
// Uses simple heuristic: ~4 characters per token for code
func EstimateTokens(text string) int

// EstimateTokensFromChars estimates tokens from character count
func EstimateTokensFromChars(chars int) int

// TokenRatio is the approximate chars-per-token for code
const TokenRatio = 4
```

**Tests (30.1):**

| Test | Type | Description |
|------|------|-------------|
| `TestEstimateTokens_EmptyString` | Edge case | Returns 0 for empty string |
| `TestEstimateTokens_SingleWord` | Happy path | "hello" → 1-2 tokens |
| `TestEstimateTokens_ShortCode` | Happy path | Short function → reasonable estimate |
| `TestEstimateTokens_LongCode` | Success case | 1000 chars → ~250 tokens |
| `TestEstimateTokens_Whitespace` | Edge case | Whitespace-heavy text handled |
| `TestEstimateTokens_SpecialChars` | Edge case | Symbols counted appropriately |
| `TestEstimateTokensFromChars_Zero` | Edge case | 0 chars → 0 tokens |
| `TestEstimateTokensFromChars_RoundingUp` | Success case | 3 chars → 1 token (not 0) |
| `TestEstimateTokensFromChars_Large` | Success case | 100000 chars → 25000 tokens |

---

### 30.2 Pommel Metrics Calculation

**File:** `internal/metrics/metrics.go`

```go
type PommelMetrics struct {
    Chars    int   // Total characters in results
    Tokens   int   // Estimated tokens
    Files    int   // Unique files
    TimeMs   int64 // Search time in milliseconds
}

// CalculatePommelMetrics measures Pommel search results
func CalculatePommelMetrics(results []api.SearchResult, searchTimeMs int64) *PommelMetrics

// CountUniqueFiles counts unique files in results
func CountUniqueFiles(results []api.SearchResult) int

// SumContentLength sums content length across results
func SumContentLength(results []api.SearchResult) int
```

**Tests (30.2):**

| Test | Type | Description |
|------|------|-------------|
| `TestCalculatePommelMetrics_EmptyResults` | Edge case | Returns zero metrics |
| `TestCalculatePommelMetrics_SingleResult` | Happy path | Counts single result |
| `TestCalculatePommelMetrics_MultipleResults` | Happy path | Sums all results |
| `TestCalculatePommelMetrics_DuplicateFiles` | Success case | Unique file count correct |
| `TestCalculatePommelMetrics_TokenEstimate` | Success case | Tokens = chars/4 approx |
| `TestCalculatePommelMetrics_SearchTime` | Success case | Time preserved |
| `TestCountUniqueFiles_AllSame` | Edge case | Same file → 1 |
| `TestCountUniqueFiles_AllDifferent` | Success case | N files → N |
| `TestSumContentLength_EmptyContent` | Edge case | Empty content → 0 |
| `TestSumContentLength_NilResults` | Edge case | Nil slice → 0 |

---

### 30.3 Baseline Grep Measurement

**File:** `internal/metrics/baseline.go`

```go
type BaselineMetrics struct {
    Chars    int   // Total characters in matched files
    Tokens   int   // Estimated tokens
    Files    int   // Number of files grep matched
    TimeMs   int64 // Time to run grep
}

// MeasureBaseline runs grep with query terms and measures results
func MeasureBaseline(ctx context.Context, projectRoot string, query string) (*BaselineMetrics, error)

// ExtractSearchTerms extracts significant terms from query for grep
func ExtractSearchTerms(query string) []string

// GrepFiles runs grep -l to find files containing terms
func GrepFiles(ctx context.Context, root string, terms []string) ([]string, error)

// SumFileSizes calculates total size of files
func SumFileSizes(files []string) (int, error)
```

**Tests (30.3):**

| Test | Type | Description |
|------|------|-------------|
| `TestExtractSearchTerms_SingleTerm` | Happy path | "database" → ["database"] |
| `TestExtractSearchTerms_MultipleTerms` | Happy path | "database connection" → ["database", "connection"] |
| `TestExtractSearchTerms_StopwordRemoval` | Success case | "the database" → ["database"] |
| `TestExtractSearchTerms_QuotedPhrase` | Success case | `"exact phrase"` → ["exact phrase"] |
| `TestExtractSearchTerms_Empty` | Edge case | Empty → empty |
| `TestExtractSearchTerms_AllStopwords` | Edge case | "the a an" → [] |
| `TestGrepFiles_SingleTerm` | Happy path | Finds files with term |
| `TestGrepFiles_MultipleTerms` | Happy path | Finds files with any term |
| `TestGrepFiles_NoMatches` | Success case | Returns empty for no matches |
| `TestGrepFiles_InvalidRoot` | Failure case | Returns error for bad path |
| `TestGrepFiles_ContextCancellation` | Failure case | Stops on cancelled context |
| `TestGrepFiles_Timeout` | Failure case | Respects timeout |
| `TestGrepFiles_ExcludesHiddenDirs` | Success case | Skips .git, .pommel, etc. |
| `TestGrepFiles_ExcludesBinaryFiles` | Success case | Skips binary files |
| `TestSumFileSizes_EmptyList` | Edge case | Returns 0 |
| `TestSumFileSizes_SingleFile` | Happy path | Returns file size |
| `TestSumFileSizes_MultipleFiles` | Happy path | Sums all sizes |
| `TestSumFileSizes_NonexistentFile` | Failure case | Skips missing files |
| `TestSumFileSizes_Directory` | Edge case | Skips directories |
| `TestMeasureBaseline_Success` | Happy path | Returns all metrics |
| `TestMeasureBaseline_NoMatches` | Success case | Returns zero chars/tokens |
| `TestMeasureBaseline_MeasuresTime` | Success case | Time measured correctly |
| `TestMeasureBaseline_ContextCancellation` | Failure case | Returns error |

---

### 30.4 Savings Calculation

**File:** `internal/metrics/metrics.go`

```go
type SearchMetrics struct {
    // Pommel results
    PommelChars   int   `json:"pommel_chars"`
    PommelTokens  int   `json:"pommel_tokens"`
    PommelFiles   int   `json:"pommel_files"`
    PommelTimeMs  int64 `json:"pommel_time_ms"`

    // Baseline (grep) results
    BaselineChars   int   `json:"baseline_chars"`
    BaselineTokens  int   `json:"baseline_tokens"`
    BaselineFiles   int   `json:"baseline_files"`
    BaselineTimeMs  int64 `json:"baseline_time_ms"`

    // Savings
    ContextSavingsPct        float64 `json:"context_savings_pct"`
    TimeSavingsPct           float64 `json:"time_savings_pct"`
    EstimatedTokenSavingsPct float64 `json:"estimated_token_savings_pct"`
}

// CalculateSavings computes savings percentages
func CalculateSavings(pommel *PommelMetrics, baseline *BaselineMetrics) *SearchMetrics

// savingsPct calculates (1 - a/b) * 100, handling division by zero
func savingsPct(a, b int) float64
```

**Tests (30.4):**

| Test | Type | Description |
|------|------|-------------|
| `TestCalculateSavings_TypicalCase` | Happy path | 2k vs 50k → ~96% savings |
| `TestCalculateSavings_NoSavings` | Success case | Same size → 0% savings |
| `TestCalculateSavings_NegativeSavings` | Edge case | Pommel larger → negative % |
| `TestCalculateSavings_ZeroBaseline` | Edge case | Baseline 0 → 0% (not infinity) |
| `TestCalculateSavings_ZeroPommel` | Success case | Pommel 0 → 100% savings |
| `TestCalculateSavings_TimeSavings` | Success case | Time savings calculated |
| `TestCalculateSavings_TokenSavings` | Success case | Token savings calculated |
| `TestCalculateSavings_PreservesRawMetrics` | Success case | Original values preserved |
| `TestSavingsPct_ZeroDenominator` | Edge case | Returns 0 not NaN |
| `TestSavingsPct_NegativeResult` | Edge case | Can be negative |
| `TestSavingsPct_100Percent` | Edge case | a=0, b>0 → 100% |

---

### 30.5 CLI Flag

**File:** `internal/cli/search.go`

```go
// Add flag
searchCmd.Flags().Bool("metrics", false, "Show context savings compared to grep")

// Usage in search handler
if metrics {
    // Run baseline measurement
    baseline, err := metrics.MeasureBaseline(ctx, projectRoot, query)
    // Calculate savings
    searchMetrics := metrics.CalculateSavings(pommelMetrics, baseline)
    // Include in response
}
```

**Tests (30.5):**

| Test | Type | Description |
|------|------|-------------|
| `TestSearchCmd_MetricsFlag` | Happy path | --metrics shows savings |
| `TestSearchCmd_MetricsWithJSON` | Success case | JSON includes metrics |
| `TestSearchCmd_MetricsWithVerbose` | Success case | Combines with --verbose |
| `TestSearchCmd_MetricsShowsSavings` | Success case | Displays % savings |
| `TestSearchCmd_MetricsShowsComparison` | Success case | Shows Pommel vs baseline |
| `TestSearchCmd_DefaultNoMetrics` | Success case | Without flag, no baseline run |
| `TestSearchCmd_MetricsWithLimit` | Success case | Metrics for limited results |

---

### 30.6 API Updates

**File:** `internal/api/types.go`

```go
type SearchResponse struct {
    Results       []SearchResult `json:"results"`
    Query         string         `json:"query"`
    TotalResults  int            `json:"total_results"`
    SearchTimeMs  int64          `json:"search_time_ms"`
    HybridEnabled bool           `json:"hybrid_enabled"`
    RerankEnabled bool           `json:"rerank_enabled"`
    Metrics       *SearchMetrics `json:"metrics,omitempty"`
}
```

**Tests (30.6):**

| Test | Type | Description |
|------|------|-------------|
| `TestSearchResponse_MetricsOmittedWhenNil` | Success case | No metrics by default |
| `TestSearchResponse_MetricsIncluded` | Success case | Metrics present when requested |
| `TestSearchResponse_MetricsJSONFormat` | Success case | Valid JSON serialization |

---

### 30.7 Output Formatting

**File:** `internal/output/formatter.go`

```go
// FormatMetrics formats metrics for display
func (f *Formatter) FormatMetrics(metrics *SearchMetrics) string

// Sample output:
// 📊 Metrics:
//    Pommel:   2,450 chars (~612 tokens) from 3 files in 145ms
//    Baseline: 48,200 chars (~12,050 tokens) from 23 files in 890ms
//    Savings:  94.9% less context, 94.9% fewer tokens, 83.7% faster
```

**Tests (30.7):**

| Test | Type | Description |
|------|------|-------------|
| `TestFormatMetrics_TypicalOutput` | Happy path | Formatted correctly |
| `TestFormatMetrics_NilMetrics` | Edge case | Returns empty string |
| `TestFormatMetrics_NumberFormatting` | Success case | Numbers comma-separated |
| `TestFormatMetrics_PercentageFormatting` | Success case | One decimal place |
| `TestFormatMetrics_NegativeSavings` | Edge case | Shows "increase" not "savings" |
| `TestFormatMetrics_JSONFormat` | Success case | JSON includes all fields |
| `TestFormatMetrics_ZeroSavings` | Edge case | Shows 0.0% correctly |
| `TestFormatMetrics_100PercentSavings` | Edge case | Shows 100.0% correctly |

---

### 30.8 Daemon Handler Updates

**File:** `internal/daemon/handlers.go`

```go
func (d *Daemon) handleSearch(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...

    // Check if metrics requested
    includeMetrics := r.URL.Query().Get("metrics") == "true"

    if includeMetrics {
        // Calculate Pommel metrics
        pommelMetrics := metrics.CalculatePommelMetrics(results, elapsed.Milliseconds())

        // Run baseline measurement
        baseline, err := metrics.MeasureBaseline(ctx, d.projectRoot, query)
        if err != nil {
            log.Warn("failed to measure baseline", "error", err)
        } else {
            // Calculate savings
            response.Metrics = metrics.CalculateSavings(pommelMetrics, baseline)
        }
    }

    // ... return response ...
}
```

**Tests (30.8):**

| Test | Type | Description |
|------|------|-------------|
| `TestHandleSearch_MetricsParam` | Happy path | metrics=true includes metrics |
| `TestHandleSearch_NoMetricsParam` | Success case | Default no metrics |
| `TestHandleSearch_MetricsBaselineFails` | Failure case | Continues without metrics |
| `TestHandleSearch_MetricsAddedLatency` | Success case | Documents added latency |

---

## Sample Output

### Human-Readable
```
$ pm search "database connection" --metrics

Results for "database connection":

1. internal/db/connection.go:45-89 [function: NewConnection]
   Score: 0.847

2. internal/db/pool.go:12-56 [function: NewPool]
   Score: 0.756

Found 2 results in 142ms (hybrid: on, rerank: on)

📊 Metrics:
   Pommel:   2,450 chars (~612 tokens) from 2 files in 142ms
   Baseline: 48,200 chars (~12,050 tokens) from 23 files in 890ms
   Savings:  94.9% less context, 94.9% fewer tokens, 84.0% faster
```

### JSON
```json
{
  "results": [...],
  "query": "database connection",
  "total_results": 2,
  "search_time_ms": 142,
  "hybrid_enabled": true,
  "rerank_enabled": true,
  "metrics": {
    "pommel_chars": 2450,
    "pommel_tokens": 612,
    "pommel_files": 2,
    "pommel_time_ms": 142,
    "baseline_chars": 48200,
    "baseline_tokens": 12050,
    "baseline_files": 23,
    "baseline_time_ms": 890,
    "context_savings_pct": 94.9,
    "time_savings_pct": 84.0,
    "estimated_token_savings_pct": 94.9
  }
}
```

---

## Integration Tests

| Test | Description |
|------|-------------|
| `TestMetrics_EndToEnd` | Full search with metrics through CLI |
| `TestMetrics_APIEndToEnd` | API returns metrics when requested |
| `TestMetrics_AccurateBaseline` | Baseline matches actual grep results |
| `TestMetrics_ReasonableSavings` | Savings are realistic for test corpus |
| `TestMetrics_Performance` | Metrics add < 2s latency |

---

## Benchmarking Suite

For testing across repositories, create test script:

```bash
#!/bin/bash
# benchmark.sh - Test Pommel across repos

QUERIES=(
    "authentication"
    "database connection"
    "error handling"
    "parse config"
    "HTTP request"
)

for query in "${QUERIES[@]}"; do
    echo "Query: $query"
    pm search "$query" --metrics --json | jq '.metrics'
    echo ""
done
```

---

## Test Execution Order

1. Tokenizer tests (30.1) - Basic utility
2. Pommel metrics tests (30.2) - Result measurement
3. Baseline tests (30.3) - Grep measurement
4. Savings calculation tests (30.4) - Core math
5. CLI flag tests (30.5) - User interface
6. API tests (30.6) - Response format
7. Formatter tests (30.7) - Output display
8. Handler tests (30.8) - API integration
9. Integration tests - End-to-end

## Success Criteria

- [ ] All tests pass
- [ ] Metrics calculated accurately
- [ ] Baseline grep runs correctly
- [ ] Savings percentages make sense
- [ ] `--metrics` flag works
- [ ] JSON output includes metrics
- [ ] Human-readable output is clear
- [ ] Metrics add < 2s latency (acceptable for opt-in feature)

## Dependencies

- Phase 29 (Enhanced Output) - Output formatting infrastructure

## Dependents

- None (this is the final phase)
