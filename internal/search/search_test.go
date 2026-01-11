package search

import (
	"context"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a temporary database for testing with migrations applied.
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()

	database, err := db.Open(tmpDir, db.EmbeddingDimension)
	require.NoError(t, err)

	ctx := context.Background()
	err = database.Migrate(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		database.Close()
	})

	return database
}

// insertTestChunk inserts a chunk into the database and returns it.
func insertTestChunk(t *testing.T, ctx context.Context, database *db.DB, chunk *models.Chunk) {
	t.Helper()

	// Ensure chunk has hashes set
	chunk.SetHashes()

	// Insert file first (path, contentHash, language, size, modifiedAt)
	fileID, err := database.InsertFile(ctx, chunk.FilePath, "hash-"+chunk.FilePath, chunk.Language, 1024, time.Now())
	require.NoError(t, err)

	// Insert the chunk
	err = database.InsertChunk(ctx, chunk, fileID)
	require.NoError(t, err)
}

// insertTestEmbedding inserts an embedding for a chunk.
func insertTestEmbedding(t *testing.T, ctx context.Context, database *db.DB, emb embedder.Embedder, chunkID string, content string) {
	t.Helper()

	embedding, err := emb.EmbedSingle(ctx, content)
	require.NoError(t, err)

	err = database.InsertEmbedding(ctx, chunkID, embedding)
	require.NoError(t, err)
}

func TestSearch_Basic(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	mockEmb := embedder.NewMockEmbedder()

	// Create test chunks
	chunks := []*models.Chunk{
		{
			FilePath:  "/project/auth/login.go",
			StartLine: 10,
			EndLine:   25,
			Level:     models.ChunkLevelMethod,
			Name:      "HandleLogin",
			Content:   "func HandleLogin(w http.ResponseWriter, r *http.Request) { // handle user login }",
			Language:  "go",
		},
		{
			FilePath:  "/project/auth/logout.go",
			StartLine: 5,
			EndLine:   15,
			Level:     models.ChunkLevelMethod,
			Name:      "HandleLogout",
			Content:   "func HandleLogout(w http.ResponseWriter, r *http.Request) { // handle user logout }",
			Language:  "go",
		},
		{
			FilePath:  "/project/db/connection.go",
			StartLine: 1,
			EndLine:   50,
			Level:     models.ChunkLevelFile,
			Name:      "connection.go",
			Content:   "package db\n\nimport \"database/sql\"\n\nfunc Connect() *sql.DB { return nil }",
			Language:  "go",
		},
	}

	// Insert chunks and their embeddings
	for _, chunk := range chunks {
		insertTestChunk(t, ctx, database, chunk)
		insertTestEmbedding(t, ctx, database, mockEmb, chunk.ID, chunk.Content)
	}

	// Create search service
	svc := NewService(database, mockEmb)

	// Search for authentication-related code
	query := Query{
		Text:  "user authentication login",
		Limit: 10,
	}

	response, err := svc.Search(ctx, query)
	require.NoError(t, err)

	// Should return results
	assert.NotEmpty(t, response.Results, "should return search results")
	assert.Equal(t, "user authentication login", response.Query)
	assert.Greater(t, response.SearchTimeMs, int64(0), "search time should be recorded")

	// Results should have chunks with scores
	for _, result := range response.Results {
		assert.NotNil(t, result.Chunk, "result should have chunk")
		assert.NotEmpty(t, result.Chunk.ID, "chunk should have ID")
		assert.GreaterOrEqual(t, result.Score, float32(0), "score should be >= 0")
		assert.LessOrEqual(t, result.Score, float32(1), "score should be <= 1")
	}
}

func TestSearch_LevelFilter(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	mockEmb := embedder.NewMockEmbedder()

	// Create test chunks at different levels
	fileChunk := &models.Chunk{
		FilePath:  "/project/service.go",
		StartLine: 1,
		EndLine:   100,
		Level:     models.ChunkLevelFile,
		Name:      "service.go",
		Content:   "package main\n\ntype Service struct{}\n\nfunc (s *Service) Process() {}",
		Language:  "go",
	}

	classChunk := &models.Chunk{
		FilePath:  "/project/service.go",
		StartLine: 3,
		EndLine:   50,
		Level:     models.ChunkLevelClass,
		Name:      "Service",
		Content:   "type Service struct{}\n\nfunc (s *Service) Process() {}",
		Language:  "go",
	}

	methodChunk := &models.Chunk{
		FilePath:  "/project/service.go",
		StartLine: 5,
		EndLine:   20,
		Level:     models.ChunkLevelMethod,
		Name:      "Process",
		Content:   "func (s *Service) Process() {}",
		Language:  "go",
	}

	// Insert all chunks
	for _, chunk := range []*models.Chunk{fileChunk, classChunk, methodChunk} {
		insertTestChunk(t, ctx, database, chunk)
		insertTestEmbedding(t, ctx, database, mockEmb, chunk.ID, chunk.Content)
	}

	svc := NewService(database, mockEmb)

	// Test filtering by method level only
	query := Query{
		Text:   "service process",
		Limit:  10,
		Levels: []string{"method"},
	}

	response, err := svc.Search(ctx, query)
	require.NoError(t, err)

	// All results should be method level
	for _, result := range response.Results {
		assert.Equal(t, models.ChunkLevelMethod, result.Chunk.Level,
			"all results should be method level when filtered")
	}

	// Test filtering by multiple levels
	query = Query{
		Text:   "service process",
		Limit:  10,
		Levels: []string{"class", "file"},
	}

	response, err = svc.Search(ctx, query)
	require.NoError(t, err)

	// Results should be either class or file level
	for _, result := range response.Results {
		assert.Contains(t, []models.ChunkLevel{models.ChunkLevelClass, models.ChunkLevelFile},
			result.Chunk.Level, "results should be class or file level when filtered")
	}
}

func TestSearch_PathFilter(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	mockEmb := embedder.NewMockEmbedder()

	// Create test chunks in different paths
	chunks := []*models.Chunk{
		{
			FilePath:  "/project/api/handlers/user.go",
			StartLine: 1,
			EndLine:   20,
			Level:     models.ChunkLevelMethod,
			Name:      "GetUser",
			Content:   "func GetUser() {}",
			Language:  "go",
		},
		{
			FilePath:  "/project/api/handlers/order.go",
			StartLine: 1,
			EndLine:   20,
			Level:     models.ChunkLevelMethod,
			Name:      "GetOrder",
			Content:   "func GetOrder() {}",
			Language:  "go",
		},
		{
			FilePath:  "/project/internal/db/queries.go",
			StartLine: 1,
			EndLine:   20,
			Level:     models.ChunkLevelMethod,
			Name:      "GetUserFromDB",
			Content:   "func GetUserFromDB() {}",
			Language:  "go",
		},
	}

	for _, chunk := range chunks {
		insertTestChunk(t, ctx, database, chunk)
		insertTestEmbedding(t, ctx, database, mockEmb, chunk.ID, chunk.Content)
	}

	svc := NewService(database, mockEmb)

	// Filter by path prefix
	query := Query{
		Text:       "get user",
		Limit:      10,
		PathPrefix: "/project/api/",
	}

	response, err := svc.Search(ctx, query)
	require.NoError(t, err)

	// All results should be under the api path
	for _, result := range response.Results {
		assert.True(t, hasPathPrefix(result.Chunk.FilePath, "/project/api/"),
			"result path %s should have prefix /project/api/", result.Chunk.FilePath)
	}
}

func TestSearch_ParentContext(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	mockEmb := embedder.NewMockEmbedder()

	// Create parent (class) chunk
	parentChunk := &models.Chunk{
		FilePath:  "/project/user_service.go",
		StartLine: 1,
		EndLine:   100,
		Level:     models.ChunkLevelClass,
		Name:      "UserService",
		Content:   "type UserService struct{}\nfunc (s *UserService) Create() {}\nfunc (s *UserService) Delete() {}",
		Language:  "go",
	}
	insertTestChunk(t, ctx, database, parentChunk)
	insertTestEmbedding(t, ctx, database, mockEmb, parentChunk.ID, parentChunk.Content)

	// Create child (method) chunk with parent reference
	childChunk := &models.Chunk{
		FilePath:  "/project/user_service.go",
		StartLine: 2,
		EndLine:   10,
		Level:     models.ChunkLevelMethod,
		Name:      "Create",
		Content:   "func (s *UserService) Create() { // creates a user }",
		ParentID:  &parentChunk.ID,
		Language:  "go",
	}
	insertTestChunk(t, ctx, database, childChunk)
	insertTestEmbedding(t, ctx, database, mockEmb, childChunk.ID, childChunk.Content)

	svc := NewService(database, mockEmb)

	// Search for the child method
	query := Query{
		Text:   "create user",
		Limit:  10,
		Levels: []string{"method"},
	}

	response, err := svc.Search(ctx, query)
	require.NoError(t, err)
	require.NotEmpty(t, response.Results)

	// Find the result for our child chunk
	var found *Result
	for _, result := range response.Results {
		if result.Chunk.ID == childChunk.ID {
			found = &result
			break
		}
	}

	require.NotNil(t, found, "should find the child chunk in results")
	require.NotNil(t, found.Parent, "result should include parent info")
	assert.Equal(t, parentChunk.ID, found.Parent.ID)
	assert.Equal(t, "UserService", found.Parent.Name)
	assert.Equal(t, "class", found.Parent.Level)
}

func TestSearch_EmptyQuery(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	mockEmb := embedder.NewMockEmbedder()

	svc := NewService(database, mockEmb)

	// Empty query should return error
	query := Query{
		Text:  "",
		Limit: 10,
	}

	_, err := svc.Search(ctx, query)
	require.Error(t, err, "empty query should return error")
	assert.Contains(t, err.Error(), "empty", "error should mention empty query")

	// Whitespace-only query should also fail
	query = Query{
		Text:  "   \t\n  ",
		Limit: 10,
	}

	_, err = svc.Search(ctx, query)
	require.Error(t, err, "whitespace-only query should return error")
}

func TestSearch_DefaultLimit(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	mockEmb := embedder.NewMockEmbedder()

	// Create more than 10 chunks
	for i := 0; i < 15; i++ {
		chunk := &models.Chunk{
			FilePath:  "/project/file" + string(rune('a'+i)) + ".go",
			StartLine: 1,
			EndLine:   10,
			Level:     models.ChunkLevelFile,
			Name:      "file" + string(rune('a'+i)) + ".go",
			Content:   "package main // file content " + string(rune('a'+i)),
			Language:  "go",
		}
		insertTestChunk(t, ctx, database, chunk)
		insertTestEmbedding(t, ctx, database, mockEmb, chunk.ID, chunk.Content)
	}

	svc := NewService(database, mockEmb)

	// Query with Limit=0 should use default of 10
	query := Query{
		Text:  "main package content",
		Limit: 0,
	}

	response, err := svc.Search(ctx, query)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(response.Results), DefaultLimit,
		"should use default limit when Limit is 0")
}

func TestDistanceToSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		distance float32
		wantMin  float32
		wantMax  float32
	}{
		{
			name:     "zero distance is perfect similarity",
			distance: 0.0,
			wantMin:  0.99,
			wantMax:  1.0,
		},
		{
			name:     "small distance is high similarity",
			distance: 0.1,
			wantMin:  0.8,
			wantMax:  0.95,
		},
		{
			name:     "medium distance is medium similarity",
			distance: 0.5,
			wantMin:  0.4,
			wantMax:  0.7,
		},
		{
			name:     "large distance is low similarity",
			distance: 1.0,
			wantMin:  0.0,
			wantMax:  0.5,
		},
		{
			name:     "very large distance approaches zero",
			distance: 2.0,
			wantMin:  0.0,
			wantMax:  0.35,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := DistanceToSimilarity(tt.distance)
			assert.GreaterOrEqual(t, score, tt.wantMin,
				"score %f should be >= %f for distance %f", score, tt.wantMin, tt.distance)
			assert.LessOrEqual(t, score, tt.wantMax,
				"score %f should be <= %f for distance %f", score, tt.wantMax, tt.distance)
		})
	}
}

func TestSearch_NoResults(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	mockEmb := embedder.NewMockEmbedder()

	// Empty database - no chunks
	svc := NewService(database, mockEmb)

	query := Query{
		Text:  "find something",
		Limit: 10,
	}

	response, err := svc.Search(ctx, query)
	require.NoError(t, err, "search on empty database should not error")
	assert.Empty(t, response.Results, "should return empty results")
	assert.Equal(t, 0, response.TotalResults)
}

func TestSearch_TotalResultsCount(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	mockEmb := embedder.NewMockEmbedder()

	// Create several chunks
	for i := 0; i < 5; i++ {
		chunk := &models.Chunk{
			FilePath:  "/project/file" + string(rune('a'+i)) + ".go",
			StartLine: 1,
			EndLine:   10,
			Level:     models.ChunkLevelMethod,
			Name:      "Method" + string(rune('A'+i)),
			Content:   "func Method() { // implementation }",
			Language:  "go",
		}
		insertTestChunk(t, ctx, database, chunk)
		insertTestEmbedding(t, ctx, database, mockEmb, chunk.ID, chunk.Content)
	}

	svc := NewService(database, mockEmb)

	// Search with limit smaller than total
	query := Query{
		Text:  "method implementation",
		Limit: 3,
	}

	response, err := svc.Search(ctx, query)
	require.NoError(t, err)

	assert.Len(t, response.Results, 3, "should respect limit")
	// TotalResults should reflect how many were returned (or could be total matching)
	assert.GreaterOrEqual(t, response.TotalResults, 3,
		"total results should be at least as many as returned")
}

// hasPathPrefix checks if a path has the given prefix.
func hasPathPrefix(path, prefix string) bool {
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}
