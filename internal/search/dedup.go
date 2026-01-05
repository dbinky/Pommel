package search

import (
	"sort"
)

// SplitBoostFactor is the bonus multiplier for each additional split that matches.
// A score boost is applied: baseScore * (1 + (numSplits-1) * SplitBoostFactor)
// With 0.1 factor: 2 splits = 1.1x, 3 splits = 1.2x, 4 splits = 1.3x, etc.
const SplitBoostFactor = 0.1

// MaxSplitBoost is the maximum score boost multiplier from split matches.
// This prevents unbounded score inflation from many split matches.
const MaxSplitBoost = 1.5

// DeduplicateSplitResults consolidates multiple split chunks from the same
// original chunk into a single result with a boosted score.
//
// When a large chunk is split into multiple parts during indexing, multiple
// parts may match a search query. This function:
// 1. Groups results by their ParentChunkID (the original chunk they came from)
// 2. Keeps the best-scoring split for each group
// 3. Boosts the score based on how many splits matched (more matches = higher relevance)
// 4. Records how many splits matched in the result
//
// Non-split chunks (where ParentChunkID is empty) pass through unchanged.
func DeduplicateSplitResults(results []Result) []Result {
	if len(results) == 0 {
		return results
	}

	// Handle single result specially
	if len(results) == 1 {
		if results[0].Chunk.ParentChunkID != "" {
			results[0].MatchedSplits = 1
		}
		return results
	}

	// Group results by ParentChunkID
	// Chunks without ParentChunkID (not splits) use their own ID as the key
	groups := make(map[string][]Result)
	for _, r := range results {
		key := r.Chunk.ParentChunkID
		if key == "" {
			// Not a split chunk - use chunk's own ID
			key = r.Chunk.ID
		}
		groups[key] = append(groups[key], r)
	}

	// Process each group
	deduplicated := make([]Result, 0, len(groups))
	for _, group := range groups {
		if len(group) == 1 {
			// Single result - no deduplication needed
			result := group[0]
			// But mark as having 1 matched split if it's a split chunk
			if result.Chunk.ParentChunkID != "" {
				result.MatchedSplits = 1
			}
			deduplicated = append(deduplicated, result)
		} else {
			// Multiple splits matched - deduplicate and boost
			best := selectBestAndBoost(group)
			deduplicated = append(deduplicated, best)
		}
	}

	// Sort by score descending (boosted scores may change ordering)
	sort.Slice(deduplicated, func(i, j int) bool {
		return deduplicated[i].Score > deduplicated[j].Score
	})

	return deduplicated
}

// selectBestAndBoost selects the best-scoring result from a group of split
// chunks and applies a score boost based on how many splits matched.
func selectBestAndBoost(group []Result) Result {
	// Find the best-scoring result
	best := group[0]
	for i := 1; i < len(group); i++ {
		if group[i].Score > best.Score {
			best = group[i]
		}
	}

	// Apply score boost for multiple matches
	numMatches := len(group)
	best.Score = boostScore(best.Score, numMatches)
	best.MatchedSplits = numMatches

	return best
}

// boostScore applies a score boost based on the number of split matches.
// More matches suggest higher relevance since multiple parts of the code
// are semantically related to the query.
//
// Formula: score * min(MaxSplitBoost, 1 + (numMatches-1) * SplitBoostFactor)
// This ensures the score stays <= 1.0 even with boosting.
func boostScore(score float32, numMatches int) float32 {
	if numMatches <= 1 {
		return score
	}

	// Calculate boost multiplier
	boostMultiplier := 1.0 + float64(numMatches-1)*SplitBoostFactor
	if boostMultiplier > MaxSplitBoost {
		boostMultiplier = MaxSplitBoost
	}

	// Apply boost
	boosted := float32(float64(score) * boostMultiplier)

	// Cap at 1.0 (max similarity score)
	if boosted > 1.0 {
		boosted = 1.0
	}

	return boosted
}
