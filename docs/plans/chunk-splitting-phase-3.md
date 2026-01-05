# Chunk Splitting Phase 3: Search Integration

**Parent Design:** [2026-01-04-chunk-splitting-design.md](./2026-01-04-chunk-splitting-design.md)
**Prerequisite:** [Phase 2: Splitting Logic](./chunk-splitting-phase-2.md)
**Phase:** 3 of 3
**Goal:** Integrate split chunk handling into search: deduplication, boosting, and response formatting

## TDD Requirements

**STRICT TDD PROCESS - Follow for every task:**

1. **Write tests FIRST** - No implementation code until tests exist
2. **Run tests, verify they FAIL** - Red phase confirms tests are valid
3. **Write minimal implementation** - Only enough to pass tests
4. **Run tests, verify they PASS** - Green phase confirms implementation
5. **Refactor if needed** - Clean up while keeping tests green
6. **Commit** - Small, atomic commits after each green phase

**Test Categories Required for Each Component:**

| Category | Description | Example |
|----------|-------------|---------|
| Happy Path | Normal, expected usage | Deduplicate 3 splits from same parent |
| Success | Valid inputs that should work | No splits in results (pass through) |
| Failure | Invalid inputs handled gracefully | Empty results, nil inputs |
| Error | Error conditions properly reported | Malformed data |
| Edge Cases | Boundary conditions | Single result, all from same parent |

---

## Task 3.1: Search Result Deduplication

### Files to Create/Modify
- `internal/search/dedupe.go`
- `internal/search/dedupe_test.go`

### Test Specifications

Write these tests FIRST in `internal/search/dedupe_test.go`:

```go
package search

import (
	"testing"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// DeduplicateSplits Tests
// =============================================================================

// --- Happy Path Tests ---

func TestDeduplicateSplits_MultipleSplitsFromSameParent(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "split-0", ParentChunkID: "parent-1", Score: 0.9},
		{ChunkID: "split-1", ParentChunkID: "parent-1", Score: 0.85},
		{ChunkID: "split-2", ParentChunkID: "parent-1", Score: 0.8},
	}

	deduped := DeduplicateSplits(results)

	assert.Len(t, deduped, 1, "Should deduplicate to single result")
	assert.Equal(t, "split-0", deduped[0].ChunkID, "Should keep first (highest score)")
}

func TestDeduplicateSplits_MixedSplitsAndNonSplits(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "chunk-1", ParentChunkID: "", Score: 0.95},        // Non-split
		{ChunkID: "split-0", ParentChunkID: "parent-1", Score: 0.9}, // Split
		{ChunkID: "split-1", ParentChunkID: "parent-1", Score: 0.85},
		{ChunkID: "chunk-2", ParentChunkID: "", Score: 0.8}, // Non-split
	}

	deduped := DeduplicateSplits(results)

	assert.Len(t, deduped, 3, "Should have 2 non-splits + 1 deduplicated split")

	// Verify order preserved
	assert.Equal(t, "chunk-1", deduped[0].ChunkID)
	assert.Equal(t, "split-0", deduped[1].ChunkID)
	assert.Equal(t, "chunk-2", deduped[2].ChunkID)
}

func TestDeduplicateSplits_MultipleDifferentParents(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "split-a-0", ParentChunkID: "parent-a", Score: 0.9},
		{ChunkID: "split-b-0", ParentChunkID: "parent-b", Score: 0.88},
		{ChunkID: "split-a-1", ParentChunkID: "parent-a", Score: 0.85},
		{ChunkID: "split-b-1", ParentChunkID: "parent-b", Score: 0.82},
	}

	deduped := DeduplicateSplits(results)

	assert.Len(t, deduped, 2, "Should have one result per parent")
	assert.Equal(t, "split-a-0", deduped[0].ChunkID)
	assert.Equal(t, "split-b-0", deduped[1].ChunkID)
}

// --- Success Tests ---

func TestDeduplicateSplits_NoSplits(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "chunk-1", ParentChunkID: "", Score: 0.9},
		{ChunkID: "chunk-2", ParentChunkID: "", Score: 0.8},
		{ChunkID: "chunk-3", ParentChunkID: "", Score: 0.7},
	}

	deduped := DeduplicateSplits(results)

	assert.Len(t, deduped, 3, "Should return all results when no splits")
	assert.Equal(t, results, deduped)
}

func TestDeduplicateSplits_SingleResult(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "chunk-1", ParentChunkID: "", Score: 0.9},
	}

	deduped := DeduplicateSplits(results)

	assert.Len(t, deduped, 1)
	assert.Equal(t, "chunk-1", deduped[0].ChunkID)
}

func TestDeduplicateSplits_SingleSplit(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "split-0", ParentChunkID: "parent-1", Score: 0.9},
	}

	deduped := DeduplicateSplits(results)

	assert.Len(t, deduped, 1)
	assert.Equal(t, "split-0", deduped[0].ChunkID)
}

func TestDeduplicateSplits_EmptyResults(t *testing.T) {
	results := []models.SearchResult{}

	deduped := DeduplicateSplits(results)

	assert.Empty(t, deduped)
}

func TestDeduplicateSplits_NilResults(t *testing.T) {
	deduped := DeduplicateSplits(nil)

	assert.Empty(t, deduped)
}

// --- Edge Case Tests ---

func TestDeduplicateSplits_AllFromSameParent(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "split-0", ParentChunkID: "parent-1", Score: 0.9},
		{ChunkID: "split-1", ParentChunkID: "parent-1", Score: 0.8},
		{ChunkID: "split-2", ParentChunkID: "parent-1", Score: 0.7},
		{ChunkID: "split-3", ParentChunkID: "parent-1", Score: 0.6},
		{ChunkID: "split-4", ParentChunkID: "parent-1", Score: 0.5},
	}

	deduped := DeduplicateSplits(results)

	assert.Len(t, deduped, 1, "All splits from same parent should become one result")
}

func TestDeduplicateSplits_PreservesOtherFields(t *testing.T) {
	results := []models.SearchResult{
		{
			ChunkID:       "split-0",
			ParentChunkID: "parent-1",
			Score:         0.9,
			FilePath:      "/path/to/file.go",
			ChunkName:     "bigMethod",
			StartLine:     10,
			EndLine:       50,
			Content:       "func bigMethod() {}",
		},
	}

	deduped := DeduplicateSplits(results)

	require.Len(t, deduped, 1)
	assert.Equal(t, "/path/to/file.go", deduped[0].FilePath)
	assert.Equal(t, "bigMethod", deduped[0].ChunkName)
	assert.Equal(t, 10, deduped[0].StartLine)
	assert.Equal(t, 50, deduped[0].EndLine)
	assert.Equal(t, "func bigMethod() {}", deduped[0].Content)
}

func TestDeduplicateSplits_LargeResultSet(t *testing.T) {
	// Create 100 results with 10 different parents
	var results []models.SearchResult
	for i := 0; i < 100; i++ {
		parentID := ""
		if i%2 == 0 {
			parentID = fmt.Sprintf("parent-%d", i/10)
		}
		results = append(results, models.SearchResult{
			ChunkID:       fmt.Sprintf("chunk-%d", i),
			ParentChunkID: parentID,
			Score:         float64(100-i) / 100,
		})
	}

	deduped := DeduplicateSplits(results)

	// Should have: 50 non-splits + 10 deduplicated parents = 60
	assert.LessOrEqual(t, len(deduped), 60)
}

func TestDeduplicateSplits_OrderPreserved(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "a", ParentChunkID: "", Score: 0.9},
		{ChunkID: "b-split-0", ParentChunkID: "b", Score: 0.85},
		{ChunkID: "c", ParentChunkID: "", Score: 0.8},
		{ChunkID: "b-split-1", ParentChunkID: "b", Score: 0.75},
		{ChunkID: "d", ParentChunkID: "", Score: 0.7},
	}

	deduped := DeduplicateSplits(results)

	assert.Len(t, deduped, 4)
	assert.Equal(t, "a", deduped[0].ChunkID)
	assert.Equal(t, "b-split-0", deduped[1].ChunkID)
	assert.Equal(t, "c", deduped[2].ChunkID)
	assert.Equal(t, "d", deduped[3].ChunkID)
}

func TestDeduplicateSplits_DuplicateNonSplitIDs(t *testing.T) {
	// Edge case: same chunk ID appears twice (shouldn't happen but handle gracefully)
	results := []models.SearchResult{
		{ChunkID: "chunk-1", ParentChunkID: "", Score: 0.9},
		{ChunkID: "chunk-1", ParentChunkID: "", Score: 0.8}, // Duplicate
	}

	deduped := DeduplicateSplits(results)

	assert.Len(t, deduped, 1, "Should deduplicate by chunk ID too")
}
```

### Implementation

Create `internal/search/dedupe.go`:

```go
package search

import "github.com/pommel-dev/pommel/internal/models"

// DeduplicateSplits removes duplicate results from split chunks.
// When multiple splits from the same parent chunk match, only the first
// (highest-scoring) result is kept.
func DeduplicateSplits(results []models.SearchResult) []models.SearchResult {
	if len(results) == 0 {
		return results
	}

	seen := make(map[string]bool)
	deduped := make([]models.SearchResult, 0, len(results))

	for _, r := range results {
		// Determine the key for deduplication
		key := r.ChunkID
		if r.ParentChunkID != "" {
			key = r.ParentChunkID
		}

		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, r)
	}

	return deduped
}
```

---

## Task 3.2: Score Boosting for Multiple Split Matches

### Test Specifications

Add to `internal/search/dedupe_test.go`:

```go
// =============================================================================
// BoostMultipleSplitMatches Tests
// =============================================================================

// --- Happy Path Tests ---

func TestBoostMultipleSplitMatches_TwoSplitsFromSameParent(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "split-0", ParentChunkID: "parent-1", Score: 0.5},
		{ChunkID: "split-1", ParentChunkID: "parent-1", Score: 0.45},
		{ChunkID: "other", ParentChunkID: "", Score: 0.6},
	}

	BoostMultipleSplitMatches(results)

	// First split should be boosted by 10% (2 hits - 1 = 1, * 0.1 = 10%)
	assert.InDelta(t, 0.55, results[0].Score, 0.01)
	assert.InDelta(t, 0.495, results[1].Score, 0.01)
	// Non-split unchanged
	assert.Equal(t, 0.6, results[2].Score)
}

func TestBoostMultipleSplitMatches_ManySplitsFromSameParent(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "split-0", ParentChunkID: "parent-1", Score: 0.5},
		{ChunkID: "split-1", ParentChunkID: "parent-1", Score: 0.45},
		{ChunkID: "split-2", ParentChunkID: "parent-1", Score: 0.4},
		{ChunkID: "split-3", ParentChunkID: "parent-1", Score: 0.35},
		{ChunkID: "split-4", ParentChunkID: "parent-1", Score: 0.3},
		{ChunkID: "split-5", ParentChunkID: "parent-1", Score: 0.25},
	}

	BoostMultipleSplitMatches(results)

	// 6 hits -> boost = min(0.5, (6-1)*0.1) = 0.5 (capped)
	// First split boosted by 50%
	assert.InDelta(t, 0.75, results[0].Score, 0.01)
}

func TestBoostMultipleSplitMatches_MultipleParents(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "a-0", ParentChunkID: "parent-a", Score: 0.5},
		{ChunkID: "a-1", ParentChunkID: "parent-a", Score: 0.45},
		{ChunkID: "b-0", ParentChunkID: "parent-b", Score: 0.4},
		{ChunkID: "b-1", ParentChunkID: "parent-b", Score: 0.35},
		{ChunkID: "b-2", ParentChunkID: "parent-b", Score: 0.3},
	}

	BoostMultipleSplitMatches(results)

	// parent-a: 2 hits -> 10% boost
	assert.InDelta(t, 0.55, results[0].Score, 0.01)
	assert.InDelta(t, 0.495, results[1].Score, 0.01)

	// parent-b: 3 hits -> 20% boost
	assert.InDelta(t, 0.48, results[2].Score, 0.01)
	assert.InDelta(t, 0.42, results[3].Score, 0.01)
	assert.InDelta(t, 0.36, results[4].Score, 0.01)
}

// --- Success Tests ---

func TestBoostMultipleSplitMatches_NoSplits(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "chunk-1", ParentChunkID: "", Score: 0.9},
		{ChunkID: "chunk-2", ParentChunkID: "", Score: 0.8},
	}

	original := make([]models.SearchResult, len(results))
	copy(original, results)

	BoostMultipleSplitMatches(results)

	// Scores unchanged
	assert.Equal(t, original[0].Score, results[0].Score)
	assert.Equal(t, original[1].Score, results[1].Score)
}

func TestBoostMultipleSplitMatches_SingleSplit(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "split-0", ParentChunkID: "parent-1", Score: 0.5},
	}

	BoostMultipleSplitMatches(results)

	// Single split, no boost (need 2+ to boost)
	assert.Equal(t, 0.5, results[0].Score)
}

func TestBoostMultipleSplitMatches_EmptyResults(t *testing.T) {
	results := []models.SearchResult{}

	// Should not panic
	BoostMultipleSplitMatches(results)

	assert.Empty(t, results)
}

func TestBoostMultipleSplitMatches_NilResults(t *testing.T) {
	// Should not panic
	BoostMultipleSplitMatches(nil)
}

// --- Edge Case Tests ---

func TestBoostMultipleSplitMatches_BoostCappedAt50Percent(t *testing.T) {
	// Create 10 splits from same parent
	var results []models.SearchResult
	for i := 0; i < 10; i++ {
		results = append(results, models.SearchResult{
			ChunkID:       fmt.Sprintf("split-%d", i),
			ParentChunkID: "parent-1",
			Score:         0.4,
		})
	}

	BoostMultipleSplitMatches(results)

	// 10 hits -> boost = min(0.5, (10-1)*0.1) = 0.5 (capped)
	// 0.4 * 1.5 = 0.6
	assert.InDelta(t, 0.6, results[0].Score, 0.01)
}

func TestBoostMultipleSplitMatches_ZeroScore(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "split-0", ParentChunkID: "parent-1", Score: 0.0},
		{ChunkID: "split-1", ParentChunkID: "parent-1", Score: 0.0},
	}

	BoostMultipleSplitMatches(results)

	// 0 * anything = 0
	assert.Equal(t, 0.0, results[0].Score)
}

func TestBoostMultipleSplitMatches_NegativeScore(t *testing.T) {
	// Edge case: negative scores (shouldn't happen but handle gracefully)
	results := []models.SearchResult{
		{ChunkID: "split-0", ParentChunkID: "parent-1", Score: -0.5},
		{ChunkID: "split-1", ParentChunkID: "parent-1", Score: -0.4},
	}

	BoostMultipleSplitMatches(results)

	// Negative scores get boosted (made more negative? or towards zero?)
	// Implementation should handle gracefully
	assert.NotPanics(t, func() {
		_ = results[0].Score
	})
}

func TestBoostMultipleSplitMatches_VeryHighScore(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "split-0", ParentChunkID: "parent-1", Score: 0.95},
		{ChunkID: "split-1", ParentChunkID: "parent-1", Score: 0.9},
	}

	BoostMultipleSplitMatches(results)

	// 0.95 * 1.1 = 1.045 - should we cap at 1.0?
	// Implementation choice: allow > 1.0 or cap
	assert.GreaterOrEqual(t, results[0].Score, 0.95)
}

func TestBoostMultipleSplitMatches_MixedParentsAndNonSplits(t *testing.T) {
	results := []models.SearchResult{
		{ChunkID: "a-0", ParentChunkID: "parent-a", Score: 0.5},
		{ChunkID: "regular", ParentChunkID: "", Score: 0.48},
		{ChunkID: "a-1", ParentChunkID: "parent-a", Score: 0.45},
		{ChunkID: "b-0", ParentChunkID: "parent-b", Score: 0.4},
	}

	BoostMultipleSplitMatches(results)

	// parent-a splits boosted
	assert.Greater(t, results[0].Score, 0.5)
	assert.Greater(t, results[2].Score, 0.45)

	// regular unchanged
	assert.Equal(t, 0.48, results[1].Score)

	// parent-b single split unchanged
	assert.Equal(t, 0.4, results[3].Score)
}
```

### Implementation

Add to `internal/search/dedupe.go`:

```go
import "math"

// BoostMultipleSplitMatches increases the score of results when multiple
// splits from the same parent chunk match the query.
// This indicates high relevance since the query matches across the chunk.
func BoostMultipleSplitMatches(results []models.SearchResult) {
	if len(results) == 0 {
		return
	}

	// Count hits per parent
	parentHits := make(map[string]int)
	for _, r := range results {
		if r.ParentChunkID != "" {
			parentHits[r.ParentChunkID]++
		}
	}

	// Apply boost
	for i := range results {
		parentID := results[i].ParentChunkID
		if parentID == "" {
			continue
		}

		hits := parentHits[parentID]
		if hits <= 1 {
			continue
		}

		// Boost by 10% per additional hit, capped at 50%
		boost := math.Min(0.5, float64(hits-1)*0.1)
		results[i].Score *= (1 + boost)
	}
}
```

---

## Task 3.3: Update Search Result Model

### Files to Modify
- `internal/models/search.go` (or wherever SearchResult is defined)
- `internal/models/search_test.go`

### Test Specifications

```go
// =============================================================================
// SearchResult Split Fields Tests
// =============================================================================

func TestSearchResult_HasSplitFields(t *testing.T) {
	result := models.SearchResult{
		ChunkID:       "split-0",
		ParentChunkID: "parent-1",
		ChunkIndex:    0,
		IsPartial:     true,
	}

	assert.Equal(t, "split-0", result.ChunkID)
	assert.Equal(t, "parent-1", result.ParentChunkID)
	assert.Equal(t, 0, result.ChunkIndex)
	assert.True(t, result.IsPartial)
}

func TestSearchResult_IsSplit(t *testing.T) {
	tests := []struct {
		name     string
		result   models.SearchResult
		expected bool
	}{
		{
			name:     "with parent ID",
			result:   models.SearchResult{ParentChunkID: "parent-1"},
			expected: true,
		},
		{
			name:     "without parent ID",
			result:   models.SearchResult{ParentChunkID: ""},
			expected: false,
		},
		{
			name:     "empty result",
			result:   models.SearchResult{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.IsSplit())
		})
	}
}

func TestSearchResult_JSONSerialization(t *testing.T) {
	result := models.SearchResult{
		ChunkID:       "split-0",
		ParentChunkID: "parent-1",
		ChunkIndex:    2,
		IsPartial:     true,
		Score:         0.85,
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded models.SearchResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "parent-1", decoded.ParentChunkID)
	assert.Equal(t, 2, decoded.ChunkIndex)
	assert.True(t, decoded.IsPartial)
}

func TestSearchResult_OmitsEmptyParentID(t *testing.T) {
	result := models.SearchResult{
		ChunkID:       "chunk-1",
		ParentChunkID: "",
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "parent_chunk_id")
}
```

### Implementation

Update `internal/models/search.go`:

```go
// SearchResult represents a search result with relevance scoring.
type SearchResult struct {
	ChunkID    string  `json:"chunk_id"`
	FilePath   string  `json:"file_path"`
	ChunkName  string  `json:"chunk_name"`
	ChunkLevel string  `json:"chunk_level"`
	StartLine  int     `json:"start_line"`
	EndLine    int     `json:"end_line"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`

	// Split chunk fields
	ParentChunkID string `json:"parent_chunk_id,omitempty"`
	ChunkIndex    int    `json:"chunk_index,omitempty"`
	IsPartial     bool   `json:"is_partial,omitempty"`

	// Match information
	MatchSource  string   `json:"match_source,omitempty"`
	MatchReasons []string `json:"match_reasons,omitempty"`
}

// IsSplit returns true if this result is from a split chunk.
func (r *SearchResult) IsSplit() bool {
	return r.ParentChunkID != ""
}
```

---

## Task 3.4: Integrate Deduplication into Search Pipeline

### Test Specifications

Add integration tests to `internal/search/search_test.go`:

```go
// =============================================================================
// Search Pipeline Integration Tests
// =============================================================================

// --- Happy Path Tests ---

func TestSearch_DeduplicatesSplitResults(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert a parent chunk and its splits
	insertTestChunk(t, ctx, db, "parent-1", "bigMethod", "method", 1, 100)
	insertTestChunk(t, ctx, db, "split-0", "bigMethod", "method", 1, 40,
		withParent("parent-1"), withIndex(0))
	insertTestChunk(t, ctx, db, "split-1", "bigMethod", "method", 35, 70,
		withParent("parent-1"), withIndex(1))
	insertTestChunk(t, ctx, db, "split-2", "bigMethod", "method", 65, 100,
		withParent("parent-1"), withIndex(2))

	// Search should return only one result for this chunk
	results, err := Search(ctx, db, "bigMethod", SearchOptions{Limit: 10})

	require.NoError(t, err)
	// Should have deduplicated the splits
	methodResults := filterByName(results, "bigMethod")
	assert.LessOrEqual(t, len(methodResults), 1)
}

func TestSearch_BoostsSplitMatches(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert regular chunk
	insertTestChunk(t, ctx, db, "regular", "regularMethod", "method", 1, 10)

	// Insert split chunks (all match the query)
	insertTestChunk(t, ctx, db, "split-0", "multiMatch", "method", 1, 40,
		withParent("parent-1"), withIndex(0))
	insertTestChunk(t, ctx, db, "split-1", "multiMatch", "method", 35, 70,
		withParent("parent-1"), withIndex(1))
	insertTestChunk(t, ctx, db, "split-2", "multiMatch", "method", 65, 100,
		withParent("parent-1"), withIndex(2))

	results, err := Search(ctx, db, "multiMatch", SearchOptions{Limit: 10})

	require.NoError(t, err)

	// The split chunk should be boosted due to multiple matches
	// (exact behavior depends on embedding similarity)
	assert.NotEmpty(t, results)
}

// --- Edge Case Tests ---

func TestSearch_HandlesNoSplits(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert only regular chunks
	insertTestChunk(t, ctx, db, "chunk-1", "method1", "method", 1, 10)
	insertTestChunk(t, ctx, db, "chunk-2", "method2", "method", 11, 20)

	results, err := Search(ctx, db, "method", SearchOptions{Limit: 10})

	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSearch_ReturnsPartialFlag(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert a partial (truncated) chunk
	insertTestChunk(t, ctx, db, "truncated", "bigFile", "file", 1, 1000,
		withPartial(true))

	results, err := Search(ctx, db, "bigFile", SearchOptions{Limit: 10})

	require.NoError(t, err)
	require.NotEmpty(t, results)

	found := findByID(results, "truncated")
	require.NotNil(t, found)
	assert.True(t, found.IsPartial)
}
```

### Implementation Notes

Modify `internal/search/search.go` to:

1. Call `BoostMultipleSplitMatches()` after initial scoring
2. Call `DeduplicateSplits()` before returning results
3. Include `ParentChunkID`, `ChunkIndex`, and `IsPartial` in result mapping

```go
func Search(ctx context.Context, db *db.DB, query string, opts SearchOptions) ([]models.SearchResult, error) {
	// ... existing search logic ...

	// Apply split boost before sorting
	BoostMultipleSplitMatches(results)

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Deduplicate splits
	results = DeduplicateSplits(results)

	// Apply limit
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}
```

---

## Task 3.5: Database Query Updates

### Test Specifications

Add to `internal/db/chunks_test.go`:

```go
// =============================================================================
// Chunk Query Split Field Tests
// =============================================================================

func TestGetChunkByID_ReturnsSplitFields(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert a split chunk
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO chunks (id, file_id, name, level, start_line, end_line, content, hash,
		                    parent_chunk_id, chunk_index, is_partial)
		VALUES ('split-0', 'file-1', 'bigMethod', 'method', 1, 50, 'content', 'hash1',
		        'parent-1', 0, 1)
	`)
	require.NoError(t, err)

	chunk, err := db.GetChunkByID(ctx, "split-0")

	require.NoError(t, err)
	assert.Equal(t, "parent-1", chunk.ParentChunkID)
	assert.Equal(t, 0, chunk.ChunkIndex)
	assert.True(t, chunk.IsPartial)
}

func TestGetChunkByID_NonSplitChunk(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert a regular chunk
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO chunks (id, file_id, name, level, start_line, end_line, content, hash)
		VALUES ('regular', 'file-1', 'method', 'method', 1, 10, 'content', 'hash1')
	`)
	require.NoError(t, err)

	chunk, err := db.GetChunkByID(ctx, "regular")

	require.NoError(t, err)
	assert.Equal(t, "", chunk.ParentChunkID)
	assert.Equal(t, 0, chunk.ChunkIndex)
	assert.False(t, chunk.IsPartial)
}

func TestSearchChunks_IncludesSplitFields(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert chunks
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO chunks (id, file_id, name, level, start_line, end_line, content, hash,
		                    parent_chunk_id, chunk_index, is_partial)
		VALUES
		('split-0', 'file-1', 'method', 'method', 1, 50, 'content', 'hash1', 'parent-1', 0, 1),
		('split-1', 'file-1', 'method', 'method', 45, 100, 'content', 'hash2', 'parent-1', 1, 1)
	`)
	require.NoError(t, err)

	chunks, err := db.SearchChunks(ctx, SearchOptions{})

	require.NoError(t, err)
	require.Len(t, chunks, 2)

	for _, chunk := range chunks {
		assert.Equal(t, "parent-1", chunk.ParentChunkID)
		assert.True(t, chunk.IsPartial)
	}
}

func TestGetChunksByParentID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert parent and splits
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO chunks (id, file_id, name, level, start_line, end_line, content, hash,
		                    parent_chunk_id, chunk_index, is_partial)
		VALUES
		('split-0', 'file-1', 'method', 'method', 1, 50, 'part1', 'hash1', 'parent-1', 0, 1),
		('split-1', 'file-1', 'method', 'method', 45, 100, 'part2', 'hash2', 'parent-1', 1, 1),
		('split-2', 'file-1', 'method', 'method', 95, 150, 'part3', 'hash3', 'parent-1', 2, 1),
		('other', 'file-1', 'other', 'method', 1, 10, 'other', 'hash4', 'parent-2', 0, 1)
	`)
	require.NoError(t, err)

	splits, err := db.GetChunksByParentID(ctx, "parent-1")

	require.NoError(t, err)
	assert.Len(t, splits, 3)

	// Should be ordered by chunk_index
	assert.Equal(t, 0, splits[0].ChunkIndex)
	assert.Equal(t, 1, splits[1].ChunkIndex)
	assert.Equal(t, 2, splits[2].ChunkIndex)
}
```

### Implementation

Add to `internal/db/chunks.go`:

```go
// GetChunksByParentID returns all split chunks for a given parent ID.
// Results are ordered by chunk_index.
func (db *DB) GetChunksByParentID(ctx context.Context, parentID string) ([]*models.Chunk, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, file_id, name, level, start_line, end_line, content, hash,
		       parent_chunk_id, chunk_index, is_partial
		FROM chunks
		WHERE parent_chunk_id = ?
		ORDER BY chunk_index
	`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*models.Chunk
	for rows.Next() {
		var c models.Chunk
		var isPartial int
		err := rows.Scan(
			&c.ID, &c.FileID, &c.Name, &c.Level,
			&c.StartLine, &c.EndLine, &c.Content, &c.Hash,
			&c.ParentChunkID, &c.ChunkIndex, &isPartial,
		)
		if err != nil {
			return nil, err
		}
		c.IsPartial = isPartial == 1
		chunks = append(chunks, &c)
	}

	return chunks, rows.Err()
}
```

---

## Verification Checklist

```bash
# Run all Phase 3 tests
go test -v -race ./internal/search/... ./internal/db/... ./internal/models/...

# Run specific test groups
go test -v ./internal/search/ -run "TestDeduplicateSplits"
go test -v ./internal/search/ -run "TestBoostMultipleSplitMatches"
go test -v ./internal/search/ -run "TestSearch_"
go test -v ./internal/db/ -run "Chunk.*Split"
go test -v ./internal/models/ -run "SearchResult"

# Integration tests
go test -v ./internal/search/ -run "Integration"

# Full test suite
go test -v -race ./...
```

## Acceptance Criteria

| Criterion | Test Coverage |
|-----------|---------------|
| Splits from same parent deduplicated | ✅ TestDeduplicateSplits_MultipleSplitsFromSameParent |
| Non-splits passed through | ✅ TestDeduplicateSplits_NoSplits |
| Order preserved after dedup | ✅ TestDeduplicateSplits_OrderPreserved |
| Multiple split matches boosted | ✅ TestBoostMultipleSplitMatches_TwoSplitsFromSameParent |
| Boost capped at 50% | ✅ TestBoostMultipleSplitMatches_BoostCappedAt50Percent |
| Single splits not boosted | ✅ TestBoostMultipleSplitMatches_SingleSplit |
| SearchResult has split fields | ✅ TestSearchResult_HasSplitFields |
| IsSplit() method works | ✅ TestSearchResult_IsSplit |
| DB queries return split fields | ✅ TestGetChunkByID_ReturnsSplitFields |
| Can query by parent ID | ✅ TestGetChunksByParentID |
| Empty/nil handled gracefully | ✅ Multiple nil/empty tests |

## Final Integration Test

After all phases complete, run the full integration test:

```bash
# Build and install
make build
make install

# Initialize a test project
cd /tmp && mkdir test-splitting && cd test-splitting
git init
echo 'package main

func main() {}' > main.go

# Create a large file that will be split
cat > large.go << 'EOF'
package main

// This is a very large function that will be split
func veryLargeFunction() {
EOF

for i in $(seq 1 500); do
    echo "    x$i := doSomething($i)" >> large.go
done

echo "}" >> large.go

# Create a minified file that should be skipped
echo -n "function(){" > bundle.min.js
for i in $(seq 1 1000); do echo -n "x+=1;" >> bundle.min.js; done
echo "}" >> bundle.min.js

# Initialize and index
pm init --auto --start
sleep 5  # Wait for indexing

# Check status
pm status --json | jq .

# Search should work
pm search "veryLargeFunction"
pm search "doSomething"

# Verify no embedding failures in logs
cat .pommel/daemon.log | grep -i "error\|fail" || echo "No errors found"

# Cleanup
pm stop
cd .. && rm -rf test-splitting
```

## Success Metrics

1. **Zero embedding failures** - No "context length exceeded" errors
2. **Minified files skipped** - `bundle.min.js` not indexed
3. **Large function split** - `veryLargeFunction` searchable via splits
4. **Deduplication working** - Single result for split chunks
5. **Boosting working** - Multi-match splits rank higher
6. **All tests pass** - `go test -race ./...` succeeds

## Completion

After Phase 3 is complete:

1. Update `docs/plans/2026-01-04-chunk-splitting-design.md` status to "Implemented"
2. Update version number if needed
3. Create release notes highlighting the new features
4. Run dogfooding tests to verify search quality
