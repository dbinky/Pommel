package rerank

import (
	"context"
	"time"
)

// Reranker scores and reorders search candidates
type Reranker interface {
	// Rerank scores candidates and returns them sorted by relevance
	Rerank(ctx context.Context, query string, candidates []Candidate) ([]RankedCandidate, error)

	// Name returns the reranker type for logging/debugging
	Name() string

	// Available returns true if this reranker can be used
	Available(ctx context.Context) bool
}

// Candidate represents a search result candidate for re-ranking
type Candidate struct {
	ChunkID   string    // Unique identifier for the chunk
	Content   string    // Chunk content
	Name      string    // Function/class name
	FilePath  string    // Path to the file
	ChunkType string    // "function", "class", "file", etc.
	BaseScore float64   // Score from hybrid search
	ModTime   time.Time // Last modification time
}

// RankedCandidate is a candidate with final scoring information
type RankedCandidate struct {
	Candidate
	FinalScore    float64            // Combined final score
	RerankerScore float64            // Score from reranker alone
	SignalScores  map[string]float64 // Individual signal contributions
}

// FallbackReranker tries primary reranker, falls back to secondary on failure
type FallbackReranker struct {
	primary   Reranker
	secondary Reranker
	timeout   time.Duration
}

// NewFallbackReranker creates a reranker that falls back to secondary on primary failure
func NewFallbackReranker(primary, secondary Reranker, timeout time.Duration) *FallbackReranker {
	return &FallbackReranker{
		primary:   primary,
		secondary: secondary,
		timeout:   timeout,
	}
}

// Rerank tries primary, falls back to secondary on failure
func (r *FallbackReranker) Rerank(ctx context.Context, query string, candidates []Candidate) ([]RankedCandidate, error) {
	// Check if primary is available
	if !r.primary.Available(ctx) {
		return r.secondary.Rerank(ctx, query, candidates)
	}

	// Create context with timeout for primary
	ctxWithTimeout, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Try primary
	results, err := r.primary.Rerank(ctxWithTimeout, query, candidates)
	if err != nil {
		// Fall back to secondary
		return r.secondary.Rerank(ctx, query, candidates)
	}

	return results, nil
}

// Name returns a combined name indicating both rerankers
func (r *FallbackReranker) Name() string {
	return r.primary.Name() + "->" + r.secondary.Name()
}

// Available returns true if at least one reranker is available
func (r *FallbackReranker) Available(ctx context.Context) bool {
	return r.primary.Available(ctx) || r.secondary.Available(ctx)
}
