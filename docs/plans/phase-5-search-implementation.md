# Phase 5: Search Implementation

**Phase Goal:** Build the complete search functionality from query to ranked results with parent context.

**Prerequisites:** Phase 1-4 complete (database, embeddings, chunking, daemon)

**Estimated Tasks:** 10 tasks across 4 areas

---

## Table of Contents

1. [Overview](#overview)
2. [Success Criteria](#success-criteria)
3. [Task 5.1: Search Query Processing](#task-51-search-query-processing)
4. [Task 5.2: Vector Search](#task-52-vector-search)
5. [Task 5.3: Result Enrichment](#task-53-result-enrichment)
6. [Task 5.4: Search API Integration](#task-54-search-api-integration)
7. [Testing Strategy](#testing-strategy)

---

## Overview

Phase 5 completes the search pipeline:
1. Receive query text from CLI
2. Generate embedding for query
3. Search sqlite-vec for similar chunks
4. Filter by level and path
5. Enrich results with parent context
6. Return ranked results with scores

---

## Success Criteria

| Criterion | Validation |
|-----------|------------|
| Query embedding | Text converts to vector |
| Vector search | Returns similar chunks ordered by distance |
| Level filtering | --level method only returns methods |
| Path filtering | --path src/** only returns src files |
| Parent context | Method results include parent class info |
| Score calculation | Results include similarity score 0-1 |

---

## Task 5.1: Search Query Processing

### 5.1.1 Implement Search Service

**File Content (internal/search/search.go):**
```go
package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/pommel-dev/pommel/internal/models"
)

// Query represents a search query
type Query struct {
	Text       string
	Limit      int
	Levels     []string  // Filter: file, class, method
	PathPrefix string    // Filter: only files under this path
}

// Result represents a single search result
type Result struct {
	Chunk      *models.Chunk `json:"chunk"`
	Score      float32       `json:"score"`      // Similarity score 0-1
	Parent     *ParentInfo   `json:"parent,omitempty"`
}

// ParentInfo contains information about the parent chunk
type ParentInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level string `json:"level"`
}

// Response contains the complete search response
type Response struct {
	Query        string    `json:"query"`
	Results      []Result  `json:"results"`
	TotalResults int       `json:"total_results"`
	SearchTimeMs int64     `json:"search_time_ms"`
}

// Service handles search operations
type Service struct {
	db       *db.DB
	embedder embedder.Embedder
}

// NewService creates a new search service
func NewService(database *db.DB, emb embedder.Embedder) *Service {
	return &Service{
		db:       database,
		embedder: emb,
	}
}

// Search performs a semantic search
func (s *Service) Search(ctx context.Context, query Query) (*Response, error) {
	start := time.Now()

	// Validate query
	if query.Text == "" {
		return nil, fmt.Errorf("query text is required")
	}
	if query.Limit <= 0 {
		query.Limit = 10
	}

	// Generate embedding for query
	embedding, err := s.embedder.EmbedSingle(ctx, query.Text)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Build search options
	opts := db.SearchOptions{
		Embedding:  embedding,
		Limit:      query.Limit * 2, // Get extra for filtering
		Levels:     query.Levels,
		PathPrefix: query.PathPrefix,
	}

	// Execute vector search
	vectorResults, err := s.db.SearchChunks(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Convert to results with enrichment
	results := make([]Result, 0, len(vectorResults))
	for _, vr := range vectorResults {
		// Get chunk details
		chunk, err := s.db.GetChunk(ctx, vr.ChunkID)
		if err != nil {
			continue // Skip if chunk not found
		}

		// Calculate similarity score (convert distance to similarity)
		score := distanceToSimilarity(vr.Distance)

		result := Result{
			Chunk: chunk,
			Score: score,
		}

		// Get parent info if applicable
		if chunk.ParentID != nil && *chunk.ParentID != "" {
			parent, err := s.db.GetChunk(ctx, *chunk.ParentID)
			if err == nil {
				result.Parent = &ParentInfo{
					ID:    parent.ID,
					Name:  parent.Name,
					Level: string(parent.Level),
				}
			}
		}

		results = append(results, result)

		// Stop at limit
		if len(results) >= query.Limit {
			break
		}
	}

	return &Response{
		Query:        query.Text,
		Results:      results,
		TotalResults: len(vectorResults),
		SearchTimeMs: time.Since(start).Milliseconds(),
	}, nil
}

// distanceToSimilarity converts L2 distance to similarity score
// Lower distance = higher similarity
func distanceToSimilarity(distance float32) float32 {
	// Using inverse with scaling
	// This gives ~0.9 for distance 0.1, ~0.5 for distance 1.0
	if distance <= 0 {
		return 1.0
	}
	return 1.0 / (1.0 + distance)
}
```

---

### 5.1.2 Add Search Options to Database

**File Content (internal/db/search.go):**
```go
package db

import (
	"context"
	"fmt"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/ncruces"
	"github.com/pommel-dev/pommel/internal/models"
)

// SearchOptions contains search parameters
type SearchOptions struct {
	Embedding  []float32
	Limit      int
	Levels     []string // Filter by chunk level
	PathPrefix string   // Filter by file path prefix
}

// VectorResult represents a vector search result
type VectorResult struct {
	ChunkID  string
	Distance float32
}

// SearchChunks performs a vector similarity search with filtering
func (db *DB) SearchChunks(ctx context.Context, opts SearchOptions) ([]VectorResult, error) {
	serialized, err := sqlite_vec.SerializeFloat32(opts.Embedding)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize embedding: %w", err)
	}

	// Build query with filters
	query := `
		SELECT ce.chunk_id, ce.distance
		FROM chunk_embeddings ce
		JOIN chunks c ON c.id = ce.chunk_id
		WHERE ce.embedding MATCH ?
	`
	args := []any{serialized}

	// Add level filter
	if len(opts.Levels) > 0 {
		placeholders := make([]string, len(opts.Levels))
		for i, level := range opts.Levels {
			placeholders[i] = "?"
			args = append(args, level)
		}
		query += fmt.Sprintf(" AND c.level IN (%s)", strings.Join(placeholders, ","))
	}

	// Add path prefix filter
	if opts.PathPrefix != "" {
		query += " AND c.file_path LIKE ?"
		args = append(args, opts.PathPrefix+"%")
	}

	// Order and limit
	query += " ORDER BY ce.distance LIMIT ?"
	args = append(args, opts.Limit)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var results []VectorResult
	for rows.Next() {
		var r VectorResult
		if err := rows.Scan(&r.ChunkID, &r.Distance); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// GetChunk retrieves a chunk by ID
func (db *DB) GetChunk(ctx context.Context, id string) (*models.Chunk, error) {
	row := db.QueryRow(ctx, `
		SELECT id, file_path, start_line, end_line, level, language,
		       content, parent_id, name, signature, content_hash, last_modified
		FROM chunks WHERE id = ?
	`, id)

	var chunk models.Chunk
	var parentID *string
	var lastMod string

	err := row.Scan(
		&chunk.ID, &chunk.FilePath, &chunk.StartLine, &chunk.EndLine,
		&chunk.Level, &chunk.Language, &chunk.Content, &parentID,
		&chunk.Name, &chunk.Signature, &chunk.ContentHash, &lastMod,
	)
	if err != nil {
		return nil, err
	}

	chunk.ParentID = parentID
	chunk.LastModified, _ = time.Parse(time.RFC3339, lastMod)

	return &chunk, nil
}

// GetChunkIDsByFile returns all chunk IDs for a file
func (db *DB) GetChunkIDsByFile(ctx context.Context, filePath string) ([]string, error) {
	rows, err := db.Query(ctx, "SELECT id FROM chunks WHERE file_path = ?", filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// InsertChunk inserts a chunk into the database
func (db *DB) InsertChunk(ctx context.Context, chunk *models.Chunk) error {
	_, err := db.Exec(ctx, `
		INSERT OR REPLACE INTO chunks
		(id, file_path, start_line, end_line, level, language, content,
		 parent_id, name, signature, content_hash, last_modified, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
	`,
		chunk.ID, chunk.FilePath, chunk.StartLine, chunk.EndLine,
		chunk.Level, chunk.Language, chunk.Content, chunk.ParentID,
		chunk.Name, chunk.Signature, chunk.ContentHash,
		chunk.LastModified.Format(time.RFC3339),
	)
	return err
}

// DeleteChunksByFile deletes all chunks for a file
func (db *DB) DeleteChunksByFile(ctx context.Context, filePath string) error {
	_, err := db.Exec(ctx, "DELETE FROM chunks WHERE file_path = ?", filePath)
	return err
}

// ClearAll removes all chunks and embeddings
func (db *DB) ClearAll(ctx context.Context) error {
	if _, err := db.Exec(ctx, "DELETE FROM chunk_embeddings"); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, "DELETE FROM chunks"); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, "DELETE FROM files"); err != nil {
		return err
	}
	return nil
}
```

---

## Task 5.2: Search Tests

### 5.2.1 Write Search Service Tests

**File Content (internal/search/search_test.go):**
```go
package search

import (
	"context"
	"testing"

	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestSearch(t *testing.T) (*Service, *db.DB) {
	tmpDir := t.TempDir()
	database, err := db.Open(tmpDir)
	require.NoError(t, err)
	require.NoError(t, database.Migrate(context.Background()))
	t.Cleanup(func() { database.Close() })

	mock := embedder.NewMockEmbedder()
	service := NewService(database, mock)

	return service, database
}

func TestSearch_Basic(t *testing.T) {
	service, database := setupTestSearch(t)
	ctx := context.Background()

	// Insert test chunks
	chunks := []*models.Chunk{
		{
			ID:       "chunk-1",
			FilePath: "auth.py",
			Level:    models.ChunkLevelMethod,
			Name:     "authenticate",
			Content:  "def authenticate(user, password): return True",
		},
		{
			ID:       "chunk-2",
			FilePath: "math.py",
			Level:    models.ChunkLevelMethod,
			Name:     "calculate",
			Content:  "def calculate(a, b): return a + b",
		},
	}

	mock := embedder.NewMockEmbedder()
	for _, chunk := range chunks {
		chunk.SetHashes()
		require.NoError(t, database.InsertChunk(ctx, chunk))

		emb, _ := mock.EmbedSingle(ctx, chunk.Content)
		require.NoError(t, database.InsertEmbedding(ctx, chunk.ID, emb))
	}

	// Search for authentication
	resp, err := service.Search(ctx, Query{
		Text:  "user authentication",
		Limit: 10,
	})
	require.NoError(t, err)

	assert.Equal(t, "user authentication", resp.Query)
	assert.NotEmpty(t, resp.Results)
	assert.Greater(t, resp.SearchTimeMs, int64(0))
}

func TestSearch_LevelFilter(t *testing.T) {
	service, database := setupTestSearch(t)
	ctx := context.Background()

	// Insert chunks at different levels
	chunks := []*models.Chunk{
		{ID: "file-1", FilePath: "app.py", Level: models.ChunkLevelFile, Content: "app code"},
		{ID: "class-1", FilePath: "app.py", Level: models.ChunkLevelClass, Content: "class App"},
		{ID: "method-1", FilePath: "app.py", Level: models.ChunkLevelMethod, Content: "def run"},
	}

	mock := embedder.NewMockEmbedder()
	for _, chunk := range chunks {
		chunk.SetHashes()
		database.InsertChunk(ctx, chunk)
		emb, _ := mock.EmbedSingle(ctx, chunk.Content)
		database.InsertEmbedding(ctx, chunk.ID, emb)
	}

	// Search with method filter
	resp, err := service.Search(ctx, Query{
		Text:   "app",
		Levels: []string{"method"},
		Limit:  10,
	})
	require.NoError(t, err)

	// Should only get method
	for _, r := range resp.Results {
		assert.Equal(t, models.ChunkLevelMethod, r.Chunk.Level)
	}
}

func TestSearch_PathFilter(t *testing.T) {
	service, database := setupTestSearch(t)
	ctx := context.Background()

	chunks := []*models.Chunk{
		{ID: "src-1", FilePath: "src/auth.py", Level: models.ChunkLevelMethod, Content: "auth"},
		{ID: "test-1", FilePath: "tests/test_auth.py", Level: models.ChunkLevelMethod, Content: "auth test"},
	}

	mock := embedder.NewMockEmbedder()
	for _, chunk := range chunks {
		chunk.SetHashes()
		database.InsertChunk(ctx, chunk)
		emb, _ := mock.EmbedSingle(ctx, chunk.Content)
		database.InsertEmbedding(ctx, chunk.ID, emb)
	}

	// Search with path filter
	resp, err := service.Search(ctx, Query{
		Text:       "auth",
		PathPrefix: "src/",
		Limit:      10,
	})
	require.NoError(t, err)

	// All results should be under src/
	for _, r := range resp.Results {
		assert.True(t, strings.HasPrefix(r.Chunk.FilePath, "src/"))
	}
}

func TestSearch_ParentContext(t *testing.T) {
	service, database := setupTestSearch(t)
	ctx := context.Background()

	classChunk := &models.Chunk{
		ID:       "class-1",
		FilePath: "service.py",
		Level:    models.ChunkLevelClass,
		Name:     "AuthService",
		Content:  "class AuthService",
	}
	classChunk.SetHashes()
	database.InsertChunk(ctx, classChunk)

	methodChunk := &models.Chunk{
		ID:       "method-1",
		FilePath: "service.py",
		Level:    models.ChunkLevelMethod,
		Name:     "login",
		Content:  "def login(self)",
		ParentID: &classChunk.ID,
	}
	methodChunk.SetHashes()
	database.InsertChunk(ctx, methodChunk)

	mock := embedder.NewMockEmbedder()
	emb, _ := mock.EmbedSingle(ctx, methodChunk.Content)
	database.InsertEmbedding(ctx, methodChunk.ID, emb)

	// Search for method
	resp, err := service.Search(ctx, Query{
		Text:  "login",
		Limit: 10,
	})
	require.NoError(t, err)

	// Should have parent info
	require.NotEmpty(t, resp.Results)
	result := resp.Results[0]
	require.NotNil(t, result.Parent)
	assert.Equal(t, "AuthService", result.Parent.Name)
	assert.Equal(t, "class", result.Parent.Level)
}

func TestDistanceToSimilarity(t *testing.T) {
	tests := []struct {
		distance float32
		minSim   float32
		maxSim   float32
	}{
		{0.0, 0.99, 1.01},     // Perfect match
		{0.1, 0.85, 0.95},     // Very close
		{1.0, 0.45, 0.55},     // Moderate
		{10.0, 0.05, 0.15},    // Far
	}

	for _, tt := range tests {
		sim := distanceToSimilarity(tt.distance)
		assert.GreaterOrEqual(t, sim, tt.minSim)
		assert.LessOrEqual(t, sim, tt.maxSim)
	}
}
```

---

## Task 5.3: Search API Integration

### 5.3.1 Update Daemon to Use Search Service

**Update to internal/daemon/daemon.go:**
```go
// Add search service to Daemon struct
type Daemon struct {
	// ... existing fields
	search *search.Service
}

// In New(), create search service
func New(projectRoot string, cfg *config.Config, logger *slog.Logger) (*Daemon, error) {
	// ... existing code

	// Create search service
	searchSvc := search.NewService(database, cachedEmb)

	return &Daemon{
		// ... existing fields
		search: searchSvc,
	}, nil
}

// Implement Searcher interface
func (d *Daemon) Search(ctx context.Context, req api.SearchRequest) (*api.SearchResponse, error) {
	query := search.Query{
		Text:       req.Query,
		Limit:      req.Limit,
		Levels:     req.Levels,
		PathPrefix: req.PathPrefix,
	}

	result, err := d.search.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	// Convert to API response
	apiResults := make([]api.SearchResult, len(result.Results))
	for i, r := range result.Results {
		apiResults[i] = api.SearchResult{
			ID:        r.Chunk.ID,
			File:      r.Chunk.FilePath,
			StartLine: r.Chunk.StartLine,
			EndLine:   r.Chunk.EndLine,
			Level:     string(r.Chunk.Level),
			Language:  r.Chunk.Language,
			Name:      r.Chunk.Name,
			Score:     r.Score,
			Content:   r.Chunk.Content,
		}
		if r.Parent != nil {
			apiResults[i].Parent = &api.ParentInfo{
				ID:   r.Parent.ID,
				Name: r.Parent.Name,
				Level: r.Parent.Level,
			}
		}
	}

	return &api.SearchResponse{
		Query:        result.Query,
		Results:      apiResults,
		TotalResults: result.TotalResults,
		SearchTimeMs: result.SearchTimeMs,
	}, nil
}
```

---

### 5.3.2 Update API Types

**File Content (internal/api/types.go):**
```go
package api

// SearchRequest is the POST /search request body
type SearchRequest struct {
	Query      string   `json:"query"`
	Limit      int      `json:"limit,omitempty"`
	Levels     []string `json:"levels,omitempty"`
	PathPrefix string   `json:"path_prefix,omitempty"`
}

// SearchResponse is the POST /search response
type SearchResponse struct {
	Query        string         `json:"query"`
	Results      []SearchResult `json:"results"`
	TotalResults int            `json:"total_results"`
	SearchTimeMs int64          `json:"search_time_ms"`
}

// SearchResult represents a single search result
type SearchResult struct {
	ID        string      `json:"id"`
	File      string      `json:"file"`
	StartLine int         `json:"start_line"`
	EndLine   int         `json:"end_line"`
	Level     string      `json:"level"`
	Language  string      `json:"language"`
	Name      string      `json:"name"`
	Score     float32     `json:"score"`
	Content   string      `json:"content"`
	Parent    *ParentInfo `json:"parent,omitempty"`
}

// ParentInfo contains parent chunk information
type ParentInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level string `json:"level"`
}
```

---

## Testing Strategy

### Integration Test

```go
func TestSearchEndToEnd(t *testing.T) {
	// 1. Create test project with source files
	// 2. Initialize daemon
	// 3. Wait for indexing
	// 4. Send search request
	// 5. Verify results contain expected code
}
```

### Performance Test

```go
func BenchmarkSearch(b *testing.B) {
	// Setup with 1000 indexed chunks
	// Measure search latency
	// Target: <100ms for typical queries
}
```

---

## Checklist

Before marking Phase 5 complete:

- [ ] Query text converts to embedding
- [ ] Vector search returns ranked results
- [ ] Level filtering works (method, class, file)
- [ ] Path filtering works
- [ ] Parent context included in results
- [ ] Similarity scores are 0-1 range
- [ ] Search API endpoint works
- [ ] All tests pass
