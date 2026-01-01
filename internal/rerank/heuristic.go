package rerank

import (
	"context"
	"sort"
	"strings"
	"time"
)

// HeuristicReranker re-ranks candidates using code-aware heuristic signals
type HeuristicReranker struct{}

// NewHeuristicReranker creates a new heuristic reranker
func NewHeuristicReranker() *HeuristicReranker {
	return &HeuristicReranker{}
}

// Rerank scores candidates using heuristic signals and returns them sorted
func (r *HeuristicReranker) Rerank(ctx context.Context, query string, candidates []Candidate) ([]RankedCandidate, error) {
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if len(candidates) == 0 {
		return []RankedCandidate{}, nil
	}

	// Extract query terms
	queryTerms := extractQueryTerms(query)
	now := time.Now()

	// Score each candidate
	results := make([]RankedCandidate, len(candidates))
	for i, c := range candidates {
		signals := make(map[string]float64)

		// Apply all signals
		nameScore := NameMatchSignal(c.Name, queryTerms)
		signals["name_match"] = nameScore

		phraseScore := ExactPhraseSignal(c.Content, query)
		signals["exact_phrase"] = phraseScore

		pathScore := PathMatchSignal(c.FilePath, queryTerms)
		signals["path_match"] = pathScore

		testPenalty := TestFilePenalty(c.FilePath)
		signals["test_penalty"] = testPenalty

		recencyScore := RecencyBoost(c.ModTime, now)
		signals["recency"] = recencyScore

		typeScore := ChunkTypeSignal(c.ChunkType, query)
		signals["chunk_type"] = typeScore

		// Calculate total signal contribution
		rerankerScore := nameScore + phraseScore + pathScore + testPenalty + recencyScore + typeScore

		// Combine with base score
		// Base score is weighted higher (0.7), reranker signals add adjustment
		finalScore := c.BaseScore*0.7 + c.BaseScore*0.3*(1+rerankerScore)

		results[i] = RankedCandidate{
			Candidate:     c,
			FinalScore:    finalScore,
			RerankerScore: rerankerScore,
			SignalScores:  signals,
		}
	}

	// Sort by final score descending, then by ChunkID for stability
	sort.Slice(results, func(i, j int) bool {
		if results[i].FinalScore != results[j].FinalScore {
			return results[i].FinalScore > results[j].FinalScore
		}
		return results[i].ChunkID < results[j].ChunkID
	})

	return results, nil
}

// Name returns the reranker identifier
func (r *HeuristicReranker) Name() string {
	return "heuristic"
}

// Available returns true - heuristic reranker is always available
func (r *HeuristicReranker) Available(ctx context.Context) bool {
	return true
}

// extractQueryTerms splits a query into individual terms for matching
func extractQueryTerms(query string) []string {
	// Simple tokenization - split on whitespace and remove empty strings
	words := strings.Fields(query)
	terms := make([]string, 0, len(words))

	for _, word := range words {
		// Clean up punctuation
		word = strings.Trim(word, ".,;:!?\"'()[]{}|")
		if len(word) > 1 { // Skip single characters
			terms = append(terms, strings.ToLower(word))
		}
	}

	return terms
}
