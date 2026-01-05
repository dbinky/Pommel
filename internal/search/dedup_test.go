package search

import (
	"testing"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeduplicateSplitResults_EmptySlice(t *testing.T) {
	results := DeduplicateSplitResults([]Result{})
	assert.Empty(t, results)
}

func TestDeduplicateSplitResults_SingleResult(t *testing.T) {
	results := []Result{
		{
			Chunk: &models.Chunk{ID: "chunk-1"},
			Score: 0.8,
		},
	}

	deduplicated := DeduplicateSplitResults(results)

	require.Len(t, deduplicated, 1)
	assert.Equal(t, float32(0.8), deduplicated[0].Score)
	assert.Equal(t, 0, deduplicated[0].MatchedSplits) // Not a split
}

func TestDeduplicateSplitResults_SingleSplitChunk(t *testing.T) {
	results := []Result{
		{
			Chunk: &models.Chunk{
				ID:            "split-1",
				ParentChunkID: "parent-1",
			},
			Score: 0.8,
		},
	}

	deduplicated := DeduplicateSplitResults(results)

	require.Len(t, deduplicated, 1)
	assert.Equal(t, float32(0.8), deduplicated[0].Score)
	assert.Equal(t, 1, deduplicated[0].MatchedSplits) // Marked as 1 split
}

func TestDeduplicateSplitResults_NoSplits(t *testing.T) {
	// Multiple results but none are splits
	results := []Result{
		{Chunk: &models.Chunk{ID: "chunk-1"}, Score: 0.9},
		{Chunk: &models.Chunk{ID: "chunk-2"}, Score: 0.8},
		{Chunk: &models.Chunk{ID: "chunk-3"}, Score: 0.7},
	}

	deduplicated := DeduplicateSplitResults(results)

	require.Len(t, deduplicated, 3)
	// Should be sorted by score descending
	assert.Equal(t, float32(0.9), deduplicated[0].Score)
	assert.Equal(t, float32(0.8), deduplicated[1].Score)
	assert.Equal(t, float32(0.7), deduplicated[2].Score)
}

func TestDeduplicateSplitResults_TwoSplitsFromSameParent(t *testing.T) {
	results := []Result{
		{
			Chunk: &models.Chunk{
				ID:            "split-1",
				ParentChunkID: "parent-1",
			},
			Score: 0.8,
		},
		{
			Chunk: &models.Chunk{
				ID:            "split-2",
				ParentChunkID: "parent-1",
			},
			Score: 0.7,
		},
	}

	deduplicated := DeduplicateSplitResults(results)

	require.Len(t, deduplicated, 1)
	assert.Equal(t, "split-1", deduplicated[0].Chunk.ID) // Best score kept
	assert.Equal(t, 2, deduplicated[0].MatchedSplits)
	// Score should be boosted: 0.8 * 1.1 = 0.88
	assert.InDelta(t, 0.88, float64(deduplicated[0].Score), 0.01)
}

func TestDeduplicateSplitResults_ThreeSplitsFromSameParent(t *testing.T) {
	results := []Result{
		{
			Chunk: &models.Chunk{
				ID:            "split-1",
				ParentChunkID: "parent-1",
			},
			Score: 0.7,
		},
		{
			Chunk: &models.Chunk{
				ID:            "split-2",
				ParentChunkID: "parent-1",
			},
			Score: 0.9,
		},
		{
			Chunk: &models.Chunk{
				ID:            "split-3",
				ParentChunkID: "parent-1",
			},
			Score: 0.6,
		},
	}

	deduplicated := DeduplicateSplitResults(results)

	require.Len(t, deduplicated, 1)
	assert.Equal(t, "split-2", deduplicated[0].Chunk.ID) // Best score kept
	assert.Equal(t, 3, deduplicated[0].MatchedSplits)
	// Score should be boosted: 0.9 * 1.2 = 1.08, but capped at 1.0
	assert.Equal(t, float32(1.0), deduplicated[0].Score)
}

func TestDeduplicateSplitResults_MixedSplitsAndNonSplits(t *testing.T) {
	results := []Result{
		// Non-split chunk
		{
			Chunk: &models.Chunk{ID: "regular-1"},
			Score: 0.85,
		},
		// Two splits from parent-1
		{
			Chunk: &models.Chunk{
				ID:            "split-1a",
				ParentChunkID: "parent-1",
			},
			Score: 0.75,
		},
		{
			Chunk: &models.Chunk{
				ID:            "split-1b",
				ParentChunkID: "parent-1",
			},
			Score: 0.70,
		},
		// Another non-split
		{
			Chunk: &models.Chunk{ID: "regular-2"},
			Score: 0.60,
		},
	}

	deduplicated := DeduplicateSplitResults(results)

	require.Len(t, deduplicated, 3) // 2 regular + 1 deduplicated split group

	// Should be sorted by score
	// After boost, split group has: 0.75 * 1.1 = 0.825
	assert.Equal(t, float32(0.85), deduplicated[0].Score)  // regular-1
	assert.InDelta(t, 0.825, float64(deduplicated[1].Score), 0.01) // split group boosted
	assert.Equal(t, float32(0.60), deduplicated[2].Score)  // regular-2

	// Check matched splits count
	assert.Equal(t, 0, deduplicated[0].MatchedSplits) // Not a split
	assert.Equal(t, 2, deduplicated[1].MatchedSplits) // 2 splits merged
	assert.Equal(t, 0, deduplicated[2].MatchedSplits) // Not a split
}

func TestDeduplicateSplitResults_MultipleSplitGroups(t *testing.T) {
	results := []Result{
		// Splits from parent-1
		{
			Chunk: &models.Chunk{
				ID:            "split-1a",
				ParentChunkID: "parent-1",
			},
			Score: 0.8,
		},
		{
			Chunk: &models.Chunk{
				ID:            "split-1b",
				ParentChunkID: "parent-1",
			},
			Score: 0.7,
		},
		// Splits from parent-2
		{
			Chunk: &models.Chunk{
				ID:            "split-2a",
				ParentChunkID: "parent-2",
			},
			Score: 0.6,
		},
		{
			Chunk: &models.Chunk{
				ID:            "split-2b",
				ParentChunkID: "parent-2",
			},
			Score: 0.65,
		},
	}

	deduplicated := DeduplicateSplitResults(results)

	require.Len(t, deduplicated, 2) // 2 split groups

	// Sort order should have parent-1 first (higher boosted score)
	assert.Equal(t, "split-1a", deduplicated[0].Chunk.ID)
	assert.Equal(t, 2, deduplicated[0].MatchedSplits)

	assert.Equal(t, "split-2b", deduplicated[1].Chunk.ID)
	assert.Equal(t, 2, deduplicated[1].MatchedSplits)
}

func TestBoostScore_SingleMatch(t *testing.T) {
	score := boostScore(0.8, 1)
	assert.Equal(t, float32(0.8), score) // No boost for single match
}

func TestBoostScore_TwoMatches(t *testing.T) {
	score := boostScore(0.8, 2)
	// 0.8 * 1.1 = 0.88
	assert.InDelta(t, 0.88, float64(score), 0.01)
}

func TestBoostScore_ThreeMatches(t *testing.T) {
	score := boostScore(0.8, 3)
	// 0.8 * 1.2 = 0.96
	assert.InDelta(t, 0.96, float64(score), 0.01)
}

func TestBoostScore_CappedAt1(t *testing.T) {
	// High score + many matches should cap at 1.0
	score := boostScore(0.9, 5)
	// 0.9 * 1.4 = 1.26 -> capped at 1.0
	assert.Equal(t, float32(1.0), score)
}

func TestBoostScore_MaxBoostCap(t *testing.T) {
	// Many matches should use MaxSplitBoost cap
	score := boostScore(0.5, 10)
	// With 10 matches: 1 + 9*0.1 = 1.9, but capped at MaxSplitBoost (1.5)
	// 0.5 * 1.5 = 0.75
	assert.InDelta(t, 0.75, float64(score), 0.01)
}

func TestBoostScore_ZeroMatches(t *testing.T) {
	// Edge case: 0 matches (shouldn't happen, but handle gracefully)
	score := boostScore(0.8, 0)
	assert.Equal(t, float32(0.8), score)
}

func TestBoostScore_NegativeMatches(t *testing.T) {
	// Edge case: negative matches (shouldn't happen)
	score := boostScore(0.8, -1)
	assert.Equal(t, float32(0.8), score)
}

func TestDeduplicateSplitResults_PreservesParentInfo(t *testing.T) {
	parentInfo := &ParentInfo{
		ID:    "class-1",
		Name:  "Calculator",
		Level: "class",
	}

	results := []Result{
		{
			Chunk: &models.Chunk{
				ID:            "split-1",
				ParentChunkID: "parent-1",
			},
			Score:  0.8,
			Parent: parentInfo,
		},
		{
			Chunk: &models.Chunk{
				ID:            "split-2",
				ParentChunkID: "parent-1",
			},
			Score:  0.7,
			Parent: parentInfo,
		},
	}

	deduplicated := DeduplicateSplitResults(results)

	require.Len(t, deduplicated, 1)
	assert.NotNil(t, deduplicated[0].Parent)
	assert.Equal(t, "class-1", deduplicated[0].Parent.ID)
	assert.Equal(t, "Calculator", deduplicated[0].Parent.Name)
}

func TestDeduplicateSplitResults_ResultOrderingAfterBoost(t *testing.T) {
	// Test that re-sorting works correctly when boost changes ordering
	results := []Result{
		// Non-split with medium score
		{
			Chunk: &models.Chunk{ID: "regular-1"},
			Score: 0.85,
		},
		// Splits that individually score lower but will boost higher
		{
			Chunk: &models.Chunk{
				ID:            "split-1a",
				ParentChunkID: "parent-1",
			},
			Score: 0.80,
		},
		{
			Chunk: &models.Chunk{
				ID:            "split-1b",
				ParentChunkID: "parent-1",
			},
			Score: 0.78,
		},
		{
			Chunk: &models.Chunk{
				ID:            "split-1c",
				ParentChunkID: "parent-1",
			},
			Score: 0.75,
		},
	}

	deduplicated := DeduplicateSplitResults(results)

	require.Len(t, deduplicated, 2)

	// After boost: 0.80 * 1.2 = 0.96 which is > 0.85
	// So the split group should now be first
	assert.Equal(t, "split-1a", deduplicated[0].Chunk.ID)
	assert.InDelta(t, 0.96, float64(deduplicated[0].Score), 0.01)

	assert.Equal(t, "regular-1", deduplicated[1].Chunk.ID)
	assert.Equal(t, float32(0.85), deduplicated[1].Score)
}
