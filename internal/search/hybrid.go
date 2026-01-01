package search

import (
	"context"
	"sync"

	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
)

// HybridConfig holds configuration for hybrid search behavior.
type HybridConfig struct {
	Enabled       bool    // Whether hybrid search is enabled
	RRFK          int     // RRF constant k (typically 60)
	VectorWeight  float64 // Weight for vector search (0-1)
	KeywordWeight float64 // Weight for keyword search (0-1)
}

// DefaultHybridConfig returns the default hybrid search configuration.
func DefaultHybridConfig() HybridConfig {
	return HybridConfig{
		Enabled:       true,
		RRFK:          DefaultRRFK,
		VectorWeight:  1.0,
		KeywordWeight: 1.0,
	}
}

// HybridOptions holds options for a single hybrid search request.
type HybridOptions struct {
	HybridEnabled bool // Whether to use hybrid search for this request
	RRFK          int  // RRF constant k
	Limit         int  // Maximum number of results to return
}

// DefaultHybridOptions returns the default hybrid search options.
func DefaultHybridOptions() HybridOptions {
	return HybridOptions{
		HybridEnabled: true,
		RRFK:          DefaultRRFK,
		Limit:         10,
	}
}

// HybridSearcher performs hybrid search combining vector and keyword search.
type HybridSearcher struct {
	db       *db.DB
	embedder embedder.Embedder
	config   HybridConfig
}

// NewHybridSearcher creates a new hybrid searcher.
func NewHybridSearcher(database *db.DB, emb embedder.Embedder, config HybridConfig) *HybridSearcher {
	return &HybridSearcher{
		db:       database,
		embedder: emb,
		config:   config,
	}
}

// searchResults holds results from parallel search operations.
type searchResults struct {
	vectorResults  []RankedResult
	keywordResults []RankedResult
	vectorErr      error
	keywordErr     error
}

// Search performs a hybrid search combining vector similarity and keyword search.
// It runs both searches in parallel and merges results using RRF.
func (h *HybridSearcher) Search(ctx context.Context, query string, opts HybridOptions) ([]MergedResult, error) {
	// Preprocess the query
	processed := PreprocessQuery(query)

	// If hybrid is disabled or no terms, fall back to vector-only search
	if !opts.HybridEnabled || !h.config.Enabled {
		return h.vectorOnlySearch(ctx, query, opts.Limit)
	}

	// Run vector and keyword searches in parallel
	results := h.parallelSearch(ctx, query, processed, opts.Limit*2) // Fetch more for better fusion

	// Handle errors
	if results.vectorErr != nil && results.keywordErr != nil {
		// Both failed, return the vector error as primary
		return nil, results.vectorErr
	}

	// Merge results using RRF
	k := opts.RRFK
	if k <= 0 {
		k = DefaultRRFK
	}

	merged := RRFMerge(results.vectorResults, results.keywordResults, k, opts.Limit)
	return merged, nil
}

// parallelSearch runs vector and keyword searches concurrently.
func (h *HybridSearcher) parallelSearch(ctx context.Context, query string, processed ProcessedQuery, limit int) searchResults {
	var results searchResults
	var wg sync.WaitGroup

	// Vector search goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		vectorResults, err := h.executeVectorSearch(ctx, query, limit)
		results.vectorResults = vectorResults
		results.vectorErr = err
	}()

	// Keyword search goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		keywordResults, err := h.executeKeywordSearch(ctx, processed, limit)
		results.keywordResults = keywordResults
		results.keywordErr = err
	}()

	wg.Wait()
	return results
}

// executeVectorSearch performs vector similarity search.
func (h *HybridSearcher) executeVectorSearch(ctx context.Context, query string, limit int) ([]RankedResult, error) {
	// Generate query embedding
	embedding, err := h.embedder.EmbedSingle(ctx, query)
	if err != nil {
		return nil, err
	}

	// Perform vector search
	chunks, err := h.db.SearchSimilar(ctx, embedding, limit)
	if err != nil {
		return nil, err
	}

	// Convert to ranked results
	// Note: Distance is lower is better, so we convert to similarity (1 - distance)
	results := make([]RankedResult, len(chunks))
	for i, chunk := range chunks {
		results[i] = RankedResult{
			ChunkID: chunk.ChunkID,
			Score:   float64(1.0 - chunk.Distance), // Convert distance to similarity
			Rank:    i,
		}
	}

	return results, nil
}

// executeKeywordSearch performs FTS keyword search.
func (h *HybridSearcher) executeKeywordSearch(ctx context.Context, processed ProcessedQuery, limit int) ([]RankedResult, error) {
	// Skip if no useful query terms
	if processed.FTSQuery == "" {
		return []RankedResult{}, nil
	}

	// Perform FTS search
	ftsResults, err := h.db.FTSSearch(ctx, processed.FTSQuery, limit)
	if err != nil {
		return nil, err
	}

	// Convert to ranked results
	results := make([]RankedResult, len(ftsResults))
	for i, fts := range ftsResults {
		results[i] = RankedResult{
			ChunkID: fts.ChunkID,
			Score:   fts.Score, // BM25 score
			Rank:    i,
		}
	}

	return results, nil
}

// vectorOnlySearch performs a vector-only search (when hybrid is disabled).
func (h *HybridSearcher) vectorOnlySearch(ctx context.Context, query string, limit int) ([]MergedResult, error) {
	vectorResults, err := h.executeVectorSearch(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	// Convert to merged results (vector only)
	results := make([]MergedResult, len(vectorResults))
	for i, v := range vectorResults {
		results[i] = MergedResult{
			ChunkID:      v.ChunkID,
			RRFScore:     v.Score, // Use original score when not fusing
			VectorScore:  v.Score,
			KeywordScore: 0,
			VectorRank:   v.Rank,
			KeywordRank:  -1,
		}
	}

	return results, nil
}

// GetConfig returns the current hybrid search configuration.
func (h *HybridSearcher) GetConfig() HybridConfig {
	return h.config
}
