package search

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/pommel-dev/pommel/internal/models"
)

// DefaultLimit is the default number of results to return when no limit is specified.
const DefaultLimit = 10

// Query represents a search request.
type Query struct {
	// Text is the search query text (required).
	Text string
	// Limit is the maximum number of results to return (default: 10).
	Limit int
	// Levels filters results to specific chunk levels (e.g., "method", "class", "file").
	Levels []string
	// PathPrefix filters results to chunks whose file path starts with this prefix.
	PathPrefix string
}

// Result represents a single search result.
type Result struct {
	// Chunk is the matching code chunk.
	Chunk *models.Chunk
	// Score is the similarity score (0-1, higher is more similar).
	Score float32
	// Parent contains info about the parent chunk, if any.
	Parent *ParentInfo
}

// ParentInfo contains information about a chunk's parent.
type ParentInfo struct {
	// ID is the parent chunk's ID.
	ID string
	// Name is the parent chunk's name (e.g., class name).
	Name string
	// Level is the parent chunk's level (e.g., "class", "file").
	Level string
}

// Response represents the search response.
type Response struct {
	// Query is the original query text.
	Query string
	// Results contains the matching chunks ordered by relevance.
	Results []Result
	// TotalResults is the count of results returned.
	TotalResults int
	// SearchTimeMs is the search duration in milliseconds.
	SearchTimeMs int64
}

// Service provides semantic code search functionality.
type Service struct {
	db       *db.DB
	embedder embedder.Embedder
}

// NewService creates a new search service.
func NewService(database *db.DB, emb embedder.Embedder) *Service {
	return &Service{
		db:       database,
		embedder: emb,
	}
}

// Search performs a semantic search for code chunks matching the query.
func (s *Service) Search(ctx context.Context, query Query) (*Response, error) {
	start := time.Now()

	// Validate query text
	trimmedText := strings.TrimSpace(query.Text)
	if trimmedText == "" {
		return nil, errors.New("empty query text")
	}

	// Apply default limit if not specified
	limit := query.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}

	// Generate embedding for query
	queryEmbedding, err := s.embedder.EmbedSingle(ctx, trimmedText)
	if err != nil {
		return nil, err
	}

	// Build search options
	searchOpts := db.SearchOptions{
		Embedding:  queryEmbedding,
		Limit:      limit,
		Levels:     query.Levels,
		PathPrefix: query.PathPrefix,
	}

	// Perform vector search
	vectorResults, err := s.db.SearchChunks(ctx, searchOpts)
	if err != nil {
		return nil, err
	}

	// Build results with chunk details
	results := make([]Result, 0, len(vectorResults))
	for _, vr := range vectorResults {
		// Get chunk details
		chunk, err := s.db.GetChunk(ctx, vr.ChunkID)
		if err != nil {
			// Skip chunks that can't be retrieved (shouldn't happen in normal operation)
			continue
		}

		result := Result{
			Chunk: chunk,
			Score: DistanceToSimilarity(vr.Distance),
		}

		// Get parent info if ParentID is set
		if chunk.ParentID != nil {
			parentChunk, err := s.db.GetChunk(ctx, *chunk.ParentID)
			if err == nil && parentChunk != nil {
				result.Parent = &ParentInfo{
					ID:    parentChunk.ID,
					Name:  parentChunk.Name,
					Level: string(parentChunk.Level),
				}
			}
		}

		results = append(results, result)
	}

	elapsed := time.Since(start)
	searchTimeMs := elapsed.Milliseconds()
	// Ensure at least 1ms is reported if there was any elapsed time
	if searchTimeMs == 0 && elapsed > 0 {
		searchTimeMs = 1
	}

	return &Response{
		Query:        query.Text,
		Results:      results,
		TotalResults: len(results),
		SearchTimeMs: searchTimeMs,
	}, nil
}

// DistanceToSimilarity converts a vector distance to a similarity score.
// Distance of 0 means identical vectors (score = 1.0).
// Larger distances result in lower similarity scores.
// Formula: 1.0 / (1.0 + distance)
func DistanceToSimilarity(distance float32) float32 {
	// Handle edge case: distance <= 0 means perfect or near-perfect match
	if distance <= 0 {
		return 1.0
	}
	return 1.0 / (1.0 + distance)
}
