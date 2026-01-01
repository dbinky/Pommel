package search

import (
	"sort"
)

// DefaultRRFK is the standard RRF constant (k=60 is commonly used)
const DefaultRRFK = 60

// RankedResult represents a search result with its rank from a single source.
type RankedResult struct {
	ChunkID string  // Unique identifier for the chunk
	Score   float64 // Original score from the search (similarity or BM25)
	Rank    int     // 0-indexed rank in the result list
}

// MergedResult represents a result after RRF fusion of multiple ranked lists.
type MergedResult struct {
	ChunkID      string  // Unique identifier for the chunk
	RRFScore     float64 // Combined RRF score
	VectorScore  float64 // Original vector similarity score (0 if not in vector results)
	KeywordScore float64 // Original keyword/FTS score (0 if not in keyword results)
	VectorRank   int     // Rank in vector results (-1 if not present)
	KeywordRank  int     // Rank in keyword results (-1 if not present)
}

// MatchSource returns a string indicating which search sources matched this result.
func (m MergedResult) MatchSource() string {
	hasVector := m.VectorRank >= 0
	hasKeyword := m.KeywordRank >= 0

	if hasVector && hasKeyword {
		return "both"
	}
	if hasVector {
		return "vector"
	}
	if hasKeyword {
		return "keyword"
	}
	return "none"
}

// RRFMerge combines two ranked result lists using Reciprocal Rank Fusion.
// The formula is: RRF(d) = sum(1 / (k + rank(d)))
// where k is a constant (typically 60) and rank is 0-indexed.
func RRFMerge(vectorResults, keywordResults []RankedResult, k int, limit int) []MergedResult {
	if limit <= 0 {
		return []MergedResult{}
	}

	// Map to accumulate scores by chunk ID
	scoreMap := make(map[string]*MergedResult)

	// Process vector results
	for _, r := range vectorResults {
		if _, exists := scoreMap[r.ChunkID]; !exists {
			scoreMap[r.ChunkID] = &MergedResult{
				ChunkID:     r.ChunkID,
				VectorRank:  -1,
				KeywordRank: -1,
			}
		}
		m := scoreMap[r.ChunkID]
		m.VectorScore = r.Score
		m.VectorRank = r.Rank
		m.RRFScore += 1.0 / float64(k+r.Rank+1)
	}

	// Process keyword results
	for _, r := range keywordResults {
		if _, exists := scoreMap[r.ChunkID]; !exists {
			scoreMap[r.ChunkID] = &MergedResult{
				ChunkID:     r.ChunkID,
				VectorRank:  -1,
				KeywordRank: -1,
			}
		}
		m := scoreMap[r.ChunkID]
		m.KeywordScore = r.Score
		m.KeywordRank = r.Rank
		m.RRFScore += 1.0 / float64(k+r.Rank+1)
	}

	// Convert map to slice
	results := make([]MergedResult, 0, len(scoreMap))
	for _, m := range scoreMap {
		results = append(results, *m)
	}

	// Sort by RRF score descending, then by ChunkID for stable ordering
	sort.Slice(results, func(i, j int) bool {
		if results[i].RRFScore != results[j].RRFScore {
			return results[i].RRFScore > results[j].RRFScore
		}
		// Stable secondary sort by ChunkID
		return results[i].ChunkID < results[j].ChunkID
	})

	// Apply limit
	if len(results) > limit {
		results = results[:limit]
	}

	return results
}
