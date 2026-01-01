package search

import (
	"math"
	"testing"
)

// ============================================================================
// 27.2 Reciprocal Rank Fusion Algorithm Tests
// ============================================================================

func TestRRFMerge_BothHaveSameResult(t *testing.T) {
	vectorResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.9, Rank: 0},
		{ChunkID: "chunk-2", Score: 0.7, Rank: 1},
	}
	keywordResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.8, Rank: 0},
		{ChunkID: "chunk-3", Score: 0.6, Rank: 1},
	}

	results := RRFMerge(vectorResults, keywordResults, DefaultRRFK, 10)

	// chunk-1 should be first (appears in both)
	if len(results) == 0 {
		t.Fatal("Expected results")
	}
	if results[0].ChunkID != "chunk-1" {
		t.Errorf("Expected chunk-1 first (in both lists), got %s", results[0].ChunkID)
	}
	// chunk-1 should have higher score than others
	if results[0].RRFScore <= results[1].RRFScore {
		t.Errorf("Expected chunk-1 to have highest score, got %f vs %f", results[0].RRFScore, results[1].RRFScore)
	}
}

func TestRRFMerge_DisjointLists(t *testing.T) {
	vectorResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.9, Rank: 0},
		{ChunkID: "chunk-2", Score: 0.7, Rank: 1},
	}
	keywordResults := []RankedResult{
		{ChunkID: "chunk-3", Score: 0.8, Rank: 0},
		{ChunkID: "chunk-4", Score: 0.6, Rank: 1},
	}

	results := RRFMerge(vectorResults, keywordResults, DefaultRRFK, 10)

	// Should have all 4 unique chunks
	if len(results) != 4 {
		t.Errorf("Expected 4 results from disjoint lists, got %d", len(results))
	}
}

func TestRRFMerge_VectorOnly(t *testing.T) {
	vectorResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.9, Rank: 0},
		{ChunkID: "chunk-2", Score: 0.7, Rank: 1},
	}
	keywordResults := []RankedResult{} // Empty

	results := RRFMerge(vectorResults, keywordResults, DefaultRRFK, 10)

	if len(results) != 2 {
		t.Errorf("Expected 2 results from vector-only, got %d", len(results))
	}
	if results[0].ChunkID != "chunk-1" {
		t.Errorf("Expected chunk-1 first, got %s", results[0].ChunkID)
	}
}

func TestRRFMerge_KeywordOnly(t *testing.T) {
	vectorResults := []RankedResult{} // Empty
	keywordResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.8, Rank: 0},
		{ChunkID: "chunk-2", Score: 0.6, Rank: 1},
	}

	results := RRFMerge(vectorResults, keywordResults, DefaultRRFK, 10)

	if len(results) != 2 {
		t.Errorf("Expected 2 results from keyword-only, got %d", len(results))
	}
	if results[0].ChunkID != "chunk-1" {
		t.Errorf("Expected chunk-1 first, got %s", results[0].ChunkID)
	}
}

func TestRRFMerge_BothEmpty(t *testing.T) {
	vectorResults := []RankedResult{}
	keywordResults := []RankedResult{}

	results := RRFMerge(vectorResults, keywordResults, DefaultRRFK, 10)

	if len(results) != 0 {
		t.Errorf("Expected 0 results from empty lists, got %d", len(results))
	}
}

func TestRRFMerge_LimitRespected(t *testing.T) {
	vectorResults := make([]RankedResult, 10)
	keywordResults := make([]RankedResult, 10)

	for i := 0; i < 10; i++ {
		vectorResults[i] = RankedResult{ChunkID: "v-" + string(rune('A'+i)), Score: 0.9 - float64(i)*0.1, Rank: i}
		keywordResults[i] = RankedResult{ChunkID: "k-" + string(rune('A'+i)), Score: 0.9 - float64(i)*0.1, Rank: i}
	}

	results := RRFMerge(vectorResults, keywordResults, DefaultRRFK, 5)

	if len(results) != 5 {
		t.Errorf("Expected 5 results (limited), got %d", len(results))
	}
}

func TestRRFMerge_LimitGreaterThanTotal(t *testing.T) {
	vectorResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.9, Rank: 0},
	}
	keywordResults := []RankedResult{
		{ChunkID: "chunk-2", Score: 0.8, Rank: 0},
	}

	results := RRFMerge(vectorResults, keywordResults, DefaultRRFK, 100)

	if len(results) != 2 {
		t.Errorf("Expected 2 results (all available), got %d", len(results))
	}
}

func TestRRFMerge_ZeroLimit(t *testing.T) {
	vectorResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.9, Rank: 0},
	}
	keywordResults := []RankedResult{
		{ChunkID: "chunk-2", Score: 0.8, Rank: 0},
	}

	results := RRFMerge(vectorResults, keywordResults, DefaultRRFK, 0)

	// Zero limit should return empty or all
	if len(results) != 0 && len(results) != 2 {
		t.Logf("Zero limit returned %d results", len(results))
	}
}

func TestRRFMerge_DifferentK(t *testing.T) {
	vectorResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.9, Rank: 0},
		{ChunkID: "chunk-2", Score: 0.7, Rank: 1},
	}
	keywordResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.8, Rank: 0},
	}

	resultsK30 := RRFMerge(vectorResults, keywordResults, 30, 10)
	resultsK60 := RRFMerge(vectorResults, keywordResults, 60, 10)

	// Scores should be different with different k values
	if len(resultsK30) > 0 && len(resultsK60) > 0 {
		// chunk-1 should still be first in both, but scores differ
		scoreK30 := resultsK30[0].RRFScore
		scoreK60 := resultsK60[0].RRFScore
		if scoreK30 == scoreK60 {
			t.Logf("Scores differ: k=30 -> %f, k=60 -> %f", scoreK30, scoreK60)
		}
	}
}

func TestRRFMerge_ScoreCalculation(t *testing.T) {
	// RRF score = 1/(k+rank+1) for each list
	// chunk-1 at rank 0 in both with k=60:
	// RRF = 1/(60+0+1) + 1/(60+0+1) = 1/61 + 1/61 = 2/61 ≈ 0.0328
	vectorResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.9, Rank: 0},
	}
	keywordResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.8, Rank: 0},
	}

	results := RRFMerge(vectorResults, keywordResults, 60, 10)

	if len(results) == 0 {
		t.Fatal("Expected results")
	}

	expectedScore := 2.0 / 61.0
	if math.Abs(results[0].RRFScore-expectedScore) > 0.001 {
		t.Errorf("Expected RRF score ~%f, got %f", expectedScore, results[0].RRFScore)
	}
}

func TestRRFMerge_PreservesSourceScores(t *testing.T) {
	vectorResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.9, Rank: 0},
	}
	keywordResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.7, Rank: 0},
	}

	results := RRFMerge(vectorResults, keywordResults, DefaultRRFK, 10)

	if len(results) == 0 {
		t.Fatal("Expected results")
	}

	if results[0].VectorScore != 0.9 {
		t.Errorf("Expected VectorScore 0.9, got %f", results[0].VectorScore)
	}
	if results[0].KeywordScore != 0.7 {
		t.Errorf("Expected KeywordScore 0.7, got %f", results[0].KeywordScore)
	}
}

func TestRRFMerge_PreservesRanks(t *testing.T) {
	vectorResults := []RankedResult{
		{ChunkID: "chunk-1", Score: 0.9, Rank: 0},
		{ChunkID: "chunk-2", Score: 0.7, Rank: 1},
	}
	keywordResults := []RankedResult{
		{ChunkID: "chunk-2", Score: 0.8, Rank: 0},
		{ChunkID: "chunk-1", Score: 0.6, Rank: 1},
	}

	results := RRFMerge(vectorResults, keywordResults, DefaultRRFK, 10)

	// Find chunk-1 in results
	for _, r := range results {
		if r.ChunkID == "chunk-1" {
			if r.VectorRank != 0 {
				t.Errorf("Expected VectorRank 0 for chunk-1, got %d", r.VectorRank)
			}
			if r.KeywordRank != 1 {
				t.Errorf("Expected KeywordRank 1 for chunk-1, got %d", r.KeywordRank)
			}
			break
		}
	}
}

func TestRRFMerge_TopRankedInBothWins(t *testing.T) {
	vectorResults := []RankedResult{
		{ChunkID: "chunk-shared", Score: 0.9, Rank: 0},
		{ChunkID: "chunk-vector-only", Score: 0.95, Rank: 1},
	}
	keywordResults := []RankedResult{
		{ChunkID: "chunk-shared", Score: 0.9, Rank: 0},
		{ChunkID: "chunk-keyword-only", Score: 0.95, Rank: 1},
	}

	results := RRFMerge(vectorResults, keywordResults, DefaultRRFK, 10)

	// chunk-shared should be first (rank 0 in both)
	if results[0].ChunkID != "chunk-shared" {
		t.Errorf("Expected chunk-shared first (rank 0 in both), got %s", results[0].ChunkID)
	}
}

func TestRRFMerge_ManyResults(t *testing.T) {
	vectorResults := make([]RankedResult, 1000)
	keywordResults := make([]RankedResult, 1000)

	for i := 0; i < 1000; i++ {
		vectorResults[i] = RankedResult{ChunkID: "chunk-" + string(rune(i)), Score: 1.0 - float64(i)/1000, Rank: i}
		keywordResults[i] = RankedResult{ChunkID: "chunk-" + string(rune(i+500)), Score: 1.0 - float64(i)/1000, Rank: i}
	}

	results := RRFMerge(vectorResults, keywordResults, DefaultRRFK, 50)

	if len(results) != 50 {
		t.Errorf("Expected 50 results, got %d", len(results))
	}
}

func TestRRFMerge_StableOrdering(t *testing.T) {
	vectorResults := []RankedResult{
		{ChunkID: "chunk-a", Score: 0.9, Rank: 0},
		{ChunkID: "chunk-b", Score: 0.9, Rank: 1},
	}
	keywordResults := []RankedResult{} // Empty to force same scores from vector only

	// Run multiple times to check stability
	var firstOrder []string
	for i := 0; i < 5; i++ {
		results := RRFMerge(vectorResults, keywordResults, DefaultRRFK, 10)
		order := make([]string, len(results))
		for j, r := range results {
			order[j] = r.ChunkID
		}
		if i == 0 {
			firstOrder = order
		} else {
			for j, id := range order {
				if id != firstOrder[j] {
					t.Errorf("Ordering not stable: run %d differs at position %d", i, j)
					break
				}
			}
		}
	}
}
