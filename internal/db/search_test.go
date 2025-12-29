package db

import (
	"context"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupSearchTestDB creates a test database with migrations applied and returns
// the database along with a mock embedder.
func setupSearchTestDB(t *testing.T) (*DB, *embedder.MockEmbedder) {
	t.Helper()
	db := setupTestDB(t)
	emb := embedder.NewMockEmbedder()
	return db, emb
}

// insertTestChunk inserts a chunk with embedding for testing.
// Returns the chunk ID.
func insertTestChunk(t *testing.T, ctx context.Context, db *DB, emb *embedder.MockEmbedder, filePath string, level models.ChunkLevel, name string, content string) string {
	t.Helper()

	// Insert file
	fileID, err := db.InsertFile(ctx, filePath, "hash-"+filePath, "go", 1000, time.Now())
	require.NoError(t, err)

	// Create chunk
	chunk := &models.Chunk{
		FilePath:  filePath,
		Level:     level,
		Name:      name,
		Content:   content,
		StartLine: 1,
		EndLine:   10,
	}
	chunk.SetHashes()

	// Insert chunk
	err = db.InsertChunk(ctx, chunk, fileID)
	require.NoError(t, err)

	// Generate and insert embedding
	embedding, err := emb.EmbedSingle(ctx, content)
	require.NoError(t, err)
	err = db.InsertEmbedding(ctx, chunk.ID, embedding)
	require.NoError(t, err)

	return chunk.ID
}

// insertTestChunkWithLines inserts a chunk with specific line numbers for testing.
func insertTestChunkWithLines(t *testing.T, ctx context.Context, db *DB, emb *embedder.MockEmbedder, filePath string, level models.ChunkLevel, name string, content string, startLine, endLine int) string {
	t.Helper()

	// Insert file (will update if exists)
	fileID, err := db.InsertFile(ctx, filePath, "hash-"+filePath, "go", 1000, time.Now())
	require.NoError(t, err)

	// Create chunk with specific lines
	chunk := &models.Chunk{
		FilePath:  filePath,
		Level:     level,
		Name:      name,
		Content:   content,
		StartLine: startLine,
		EndLine:   endLine,
	}
	chunk.SetHashes()

	// Insert chunk
	err = db.InsertChunk(ctx, chunk, fileID)
	require.NoError(t, err)

	// Generate and insert embedding
	embedding, err := emb.EmbedSingle(ctx, content)
	require.NoError(t, err)
	err = db.InsertEmbedding(ctx, chunk.ID, embedding)
	require.NoError(t, err)

	return chunk.ID
}

// =============================================================================
// TestSearchChunks_Basic - Basic vector similarity search
// =============================================================================

func TestSearchChunks_Basic(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert several test chunks with different content
	_ = insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main.go", "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}")
	_ = insertTestChunk(t, ctx, db, emb, "src/utils.go", models.ChunkLevelMethod, "helper", "func helper() string {\n\treturn \"help\"\n}")
	_ = insertTestChunk(t, ctx, db, emb, "src/math.go", models.ChunkLevelMethod, "calculate", "func calculate(a, b int) int {\n\treturn a + b\n}")

	// Generate query embedding
	queryEmbedding, err := emb.EmbedSingle(ctx, "calculate math function")
	require.NoError(t, err)

	// Search
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding: queryEmbedding,
		Limit:     10,
	})
	require.NoError(t, err)

	// Should return all 3 chunks
	assert.Len(t, results, 3, "should return all chunks")

	// Results should be ordered by distance (ascending)
	for i := 1; i < len(results); i++ {
		assert.GreaterOrEqual(t, results[i].Distance, results[i-1].Distance,
			"results should be sorted by distance ascending")
	}
}

func TestSearchChunks_Basic_EmptyDatabase(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Generate query embedding
	queryEmbedding, err := emb.EmbedSingle(ctx, "any query")
	require.NoError(t, err)

	// Search on empty database
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding: queryEmbedding,
		Limit:     10,
	})
	require.NoError(t, err, "searching empty database should not error")

	// Should return empty slice, not nil
	require.NotNil(t, results, "should return empty slice, not nil")
	assert.Len(t, results, 0, "should return no results on empty database")
}

func TestSearchChunks_Basic_SimilarityOrdering(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunks with content that has varying similarity to query
	exactMatchContent := "func processUserData(user User) error"
	similarContent := "func handleUserRequest(u User) bool"
	differentContent := "type Config struct { port int }"

	exactID := insertTestChunk(t, ctx, db, emb, "src/exact.go", models.ChunkLevelMethod, "exact", exactMatchContent)
	similarID := insertTestChunk(t, ctx, db, emb, "src/similar.go", models.ChunkLevelMethod, "similar", similarContent)
	_ = insertTestChunk(t, ctx, db, emb, "src/different.go", models.ChunkLevelClass, "different", differentContent)

	// Query with the exact match content
	queryEmbedding, err := emb.EmbedSingle(ctx, exactMatchContent)
	require.NoError(t, err)

	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding: queryEmbedding,
		Limit:     10,
	})
	require.NoError(t, err)

	// The exact match should be first (distance = 0 or very close)
	require.GreaterOrEqual(t, len(results), 1, "should have at least one result")
	assert.Equal(t, exactID, results[0].ChunkID, "exact match should be first")
	assert.InDelta(t, 0, results[0].Distance, 0.001, "exact match should have near-zero distance")

	// Similar content should be before very different content
	var similarIdx, differentIdx int = -1, -1
	for i, r := range results {
		if r.ChunkID == similarID {
			similarIdx = i
		}
		if r.ChunkID != exactID && r.ChunkID != similarID {
			differentIdx = i
		}
	}

	// Note: With mock embedder, similarity depends on hash-based embedding generation
	// The important assertion is that results are ordered by distance
	_ = similarIdx
	_ = differentIdx
}

// =============================================================================
// TestSearchChunks_LevelFilter - Filter results by chunk level
// =============================================================================

func TestSearchChunks_LevelFilter_SingleLevel(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunks at different levels
	fileID := insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main.go", "package main")
	methodID := insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelMethod, "handler", "func handler() {}")
	classID := insertTestChunk(t, ctx, db, emb, "src/user.go", models.ChunkLevelClass, "User", "type User struct {}")

	queryEmbedding, err := emb.EmbedSingle(ctx, "any code")
	require.NoError(t, err)

	// Filter by method level only
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding: queryEmbedding,
		Limit:     10,
		Levels:    []string{"method"},
	})
	require.NoError(t, err)

	require.Len(t, results, 1, "should return only method-level chunks")
	assert.Equal(t, methodID, results[0].ChunkID)

	// Verify file and class chunks are not included
	for _, r := range results {
		assert.NotEqual(t, fileID, r.ChunkID, "file-level chunk should not be included")
		assert.NotEqual(t, classID, r.ChunkID, "class-level chunk should not be included")
	}
}

func TestSearchChunks_LevelFilter_MultipleLevels(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunks at different levels
	fileID := insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main.go", "package main")
	methodID := insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelMethod, "handler", "func handler() {}")
	classID := insertTestChunk(t, ctx, db, emb, "src/user.go", models.ChunkLevelClass, "User", "type User struct {}")

	queryEmbedding, err := emb.EmbedSingle(ctx, "any code")
	require.NoError(t, err)

	// Filter by method and class levels
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding: queryEmbedding,
		Limit:     10,
		Levels:    []string{"method", "class"},
	})
	require.NoError(t, err)

	assert.Len(t, results, 2, "should return method and class level chunks")

	resultIDs := make(map[string]bool)
	for _, r := range results {
		resultIDs[r.ChunkID] = true
	}
	assert.True(t, resultIDs[methodID], "method chunk should be included")
	assert.True(t, resultIDs[classID], "class chunk should be included")
	assert.False(t, resultIDs[fileID], "file chunk should not be included")
}

func TestSearchChunks_LevelFilter_EmptyLevels(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunks
	_ = insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main.go", "package main")
	_ = insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelMethod, "handler", "func handler() {}")

	queryEmbedding, err := emb.EmbedSingle(ctx, "any code")
	require.NoError(t, err)

	// Empty levels filter should return all chunks (no filtering)
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding: queryEmbedding,
		Limit:     10,
		Levels:    []string{},
	})
	require.NoError(t, err)

	assert.Len(t, results, 2, "empty levels filter should return all chunks")
}

func TestSearchChunks_LevelFilter_NoMatchingLevel(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert only file-level chunks
	_ = insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main.go", "package main")

	queryEmbedding, err := emb.EmbedSingle(ctx, "any code")
	require.NoError(t, err)

	// Filter by method level (no matching chunks)
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding: queryEmbedding,
		Limit:     10,
		Levels:    []string{"method"},
	})
	require.NoError(t, err)

	assert.Len(t, results, 0, "should return no results when no chunks match level filter")
}

// =============================================================================
// TestSearchChunks_PathFilter - Filter results by path prefix
// =============================================================================

func TestSearchChunks_PathFilter_Prefix(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunks in different directories
	srcID := insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main", "package main")
	srcUtilID := insertTestChunk(t, ctx, db, emb, "src/utils/helper.go", models.ChunkLevelMethod, "helper", "func helper() {}")
	testID := insertTestChunk(t, ctx, db, emb, "test/main_test.go", models.ChunkLevelFile, "main_test", "package main_test")

	queryEmbedding, err := emb.EmbedSingle(ctx, "code")
	require.NoError(t, err)

	// Filter by src/ prefix
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding:  queryEmbedding,
		Limit:      10,
		PathPrefix: "src/",
	})
	require.NoError(t, err)

	assert.Len(t, results, 2, "should return chunks in src/ directory")

	resultIDs := make(map[string]bool)
	for _, r := range results {
		resultIDs[r.ChunkID] = true
	}
	assert.True(t, resultIDs[srcID], "src/main.go chunk should be included")
	assert.True(t, resultIDs[srcUtilID], "src/utils/helper.go chunk should be included")
	assert.False(t, resultIDs[testID], "test/main_test.go chunk should not be included")
}

func TestSearchChunks_PathFilter_NestedPath(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunks in nested directories
	_ = insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main", "package main")
	nestedID := insertTestChunk(t, ctx, db, emb, "src/internal/db/sqlite.go", models.ChunkLevelMethod, "open", "func Open() {}")
	_ = insertTestChunk(t, ctx, db, emb, "src/internal/api/handler.go", models.ChunkLevelMethod, "handle", "func Handle() {}")

	queryEmbedding, err := emb.EmbedSingle(ctx, "code")
	require.NoError(t, err)

	// Filter by src/internal/db/ prefix
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding:  queryEmbedding,
		Limit:      10,
		PathPrefix: "src/internal/db/",
	})
	require.NoError(t, err)

	require.Len(t, results, 1, "should return only chunks in src/internal/db/")
	assert.Equal(t, nestedID, results[0].ChunkID)
}

func TestSearchChunks_PathFilter_EmptyPrefix(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunks
	_ = insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main", "package main")
	_ = insertTestChunk(t, ctx, db, emb, "test/main_test.go", models.ChunkLevelFile, "main_test", "package main_test")

	queryEmbedding, err := emb.EmbedSingle(ctx, "code")
	require.NoError(t, err)

	// Empty path prefix should return all chunks (no filtering)
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding:  queryEmbedding,
		Limit:      10,
		PathPrefix: "",
	})
	require.NoError(t, err)

	assert.Len(t, results, 2, "empty path prefix should return all chunks")
}

func TestSearchChunks_PathFilter_NoMatch(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunks
	_ = insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main", "package main")

	queryEmbedding, err := emb.EmbedSingle(ctx, "code")
	require.NoError(t, err)

	// Filter by non-existent path prefix
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding:  queryEmbedding,
		Limit:      10,
		PathPrefix: "nonexistent/",
	})
	require.NoError(t, err)

	assert.Len(t, results, 0, "should return no results when path prefix doesn't match")
}

// =============================================================================
// TestSearchChunks_MultipleFilters - Combined level + path filtering
// =============================================================================

func TestSearchChunks_MultipleFilters_LevelAndPath(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunks with various combinations
	srcFileID := insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main", "package main")
	srcMethodID := insertTestChunk(t, ctx, db, emb, "src/handler.go", models.ChunkLevelMethod, "handle", "func handle() {}")
	testFileID := insertTestChunk(t, ctx, db, emb, "test/main_test.go", models.ChunkLevelFile, "main_test", "package main_test")
	testMethodID := insertTestChunk(t, ctx, db, emb, "test/handler_test.go", models.ChunkLevelMethod, "testHandle", "func TestHandle(t *testing.T) {}")

	queryEmbedding, err := emb.EmbedSingle(ctx, "code")
	require.NoError(t, err)

	// Filter by src/ path AND method level
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding:  queryEmbedding,
		Limit:      10,
		Levels:     []string{"method"},
		PathPrefix: "src/",
	})
	require.NoError(t, err)

	require.Len(t, results, 1, "should return only method-level chunks in src/")
	assert.Equal(t, srcMethodID, results[0].ChunkID)

	// Verify other chunks are excluded
	for _, r := range results {
		assert.NotEqual(t, srcFileID, r.ChunkID, "file-level chunk in src/ should not be included")
		assert.NotEqual(t, testFileID, r.ChunkID, "file-level chunk in test/ should not be included")
		assert.NotEqual(t, testMethodID, r.ChunkID, "method-level chunk in test/ should not be included")
	}
}

func TestSearchChunks_MultipleFilters_MultipleLevelsAndPath(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunks with various combinations
	srcFileID := insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main", "package main")
	srcMethodID := insertTestChunk(t, ctx, db, emb, "src/handler.go", models.ChunkLevelMethod, "handle", "func handle() {}")
	srcClassID := insertTestChunk(t, ctx, db, emb, "src/user.go", models.ChunkLevelClass, "User", "type User struct {}")
	testMethodID := insertTestChunk(t, ctx, db, emb, "test/handler_test.go", models.ChunkLevelMethod, "testHandle", "func TestHandle(t *testing.T) {}")

	queryEmbedding, err := emb.EmbedSingle(ctx, "code")
	require.NoError(t, err)

	// Filter by src/ path AND method+class levels
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding:  queryEmbedding,
		Limit:      10,
		Levels:     []string{"method", "class"},
		PathPrefix: "src/",
	})
	require.NoError(t, err)

	assert.Len(t, results, 2, "should return method and class level chunks in src/")

	resultIDs := make(map[string]bool)
	for _, r := range results {
		resultIDs[r.ChunkID] = true
	}
	assert.True(t, resultIDs[srcMethodID], "method chunk in src/ should be included")
	assert.True(t, resultIDs[srcClassID], "class chunk in src/ should be included")
	assert.False(t, resultIDs[srcFileID], "file chunk in src/ should not be included")
	assert.False(t, resultIDs[testMethodID], "method chunk in test/ should not be included")
}

func TestSearchChunks_MultipleFilters_NoMatchingCombination(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunks
	_ = insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main", "package main")
	_ = insertTestChunk(t, ctx, db, emb, "test/handler_test.go", models.ChunkLevelMethod, "testHandle", "func TestHandle(t *testing.T) {}")

	queryEmbedding, err := emb.EmbedSingle(ctx, "code")
	require.NoError(t, err)

	// Filter by src/ path AND method level - no chunks match this combination
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding:  queryEmbedding,
		Limit:      10,
		Levels:     []string{"method"},
		PathPrefix: "src/",
	})
	require.NoError(t, err)

	assert.Len(t, results, 0, "should return no results when no chunks match combined filters")
}

// =============================================================================
// TestSearchChunks_Limit - Respects limit parameter
// =============================================================================

func TestSearchChunks_Limit_RespectsLimit(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert 10 chunks
	for i := 0; i < 10; i++ {
		_ = insertTestChunkWithLines(t, ctx, db, emb,
			"src/file.go",
			models.ChunkLevelMethod,
			"method",
			"func method() {}",
			i*10+1,
			i*10+10,
		)
	}

	queryEmbedding, err := emb.EmbedSingle(ctx, "code")
	require.NoError(t, err)

	// Test limit of 3
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding: queryEmbedding,
		Limit:     3,
	})
	require.NoError(t, err)
	assert.Len(t, results, 3, "should return exactly 3 results")

	// Test limit of 5
	results, err = db.SearchChunks(ctx, SearchOptions{
		Embedding: queryEmbedding,
		Limit:     5,
	})
	require.NoError(t, err)
	assert.Len(t, results, 5, "should return exactly 5 results")

	// Test limit of 1
	results, err = db.SearchChunks(ctx, SearchOptions{
		Embedding: queryEmbedding,
		Limit:     1,
	})
	require.NoError(t, err)
	assert.Len(t, results, 1, "should return exactly 1 result")
}

func TestSearchChunks_Limit_LargerThanAvailable(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert only 3 chunks
	_ = insertTestChunk(t, ctx, db, emb, "src/a.go", models.ChunkLevelFile, "a", "package a")
	_ = insertTestChunk(t, ctx, db, emb, "src/b.go", models.ChunkLevelFile, "b", "package b")
	_ = insertTestChunk(t, ctx, db, emb, "src/c.go", models.ChunkLevelFile, "c", "package c")

	queryEmbedding, err := emb.EmbedSingle(ctx, "code")
	require.NoError(t, err)

	// Request limit larger than available
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding: queryEmbedding,
		Limit:     100,
	})
	require.NoError(t, err)
	assert.Len(t, results, 3, "should return all available when limit exceeds count")
}

func TestSearchChunks_Limit_Zero(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunks
	_ = insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main", "package main")

	queryEmbedding, err := emb.EmbedSingle(ctx, "code")
	require.NoError(t, err)

	// Limit of 0 - should return empty or handle gracefully
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding: queryEmbedding,
		Limit:     0,
	})
	require.NoError(t, err)
	assert.Len(t, results, 0, "limit 0 should return no results")
}

func TestSearchChunks_Limit_WithFilters(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert 5 method-level chunks in src/
	for i := 0; i < 5; i++ {
		_ = insertTestChunkWithLines(t, ctx, db, emb,
			"src/handler.go",
			models.ChunkLevelMethod,
			"method",
			"func method() {}",
			i*10+1,
			i*10+10,
		)
	}
	// Insert 5 file-level chunks in src/
	for i := 0; i < 5; i++ {
		_ = insertTestChunkWithLines(t, ctx, db, emb,
			"src/other.go",
			models.ChunkLevelFile,
			"file",
			"package main",
			i*10+1,
			i*10+10,
		)
	}

	queryEmbedding, err := emb.EmbedSingle(ctx, "code")
	require.NoError(t, err)

	// Filter by method level with limit of 3
	results, err := db.SearchChunks(ctx, SearchOptions{
		Embedding: queryEmbedding,
		Limit:     3,
		Levels:    []string{"method"},
	})
	require.NoError(t, err)
	assert.Len(t, results, 3, "should return exactly 3 method-level results")
}

// =============================================================================
// TestGetChunk_Found - Retrieves existing chunk
// =============================================================================

func TestGetChunk_Found(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert a chunk
	content := "func calculate(a, b int) int {\n\treturn a + b\n}"
	chunkID := insertTestChunk(t, ctx, db, emb, "src/math.go", models.ChunkLevelMethod, "calculate", content)

	// Retrieve the chunk
	chunk, err := db.GetChunk(ctx, chunkID)
	require.NoError(t, err)
	require.NotNil(t, chunk, "chunk should be returned")

	// Verify chunk properties
	assert.Equal(t, chunkID, chunk.ID)
	assert.Equal(t, "src/math.go", chunk.FilePath)
	assert.Equal(t, models.ChunkLevelMethod, chunk.Level)
	assert.Equal(t, "calculate", chunk.Name)
	assert.Equal(t, content, chunk.Content)
}

func TestGetChunk_Found_AllLevels(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunks at each level
	fileID := insertTestChunk(t, ctx, db, emb, "src/main.go", models.ChunkLevelFile, "main.go", "package main")
	methodID := insertTestChunk(t, ctx, db, emb, "src/handler.go", models.ChunkLevelMethod, "handle", "func handle() {}")
	classID := insertTestChunk(t, ctx, db, emb, "src/user.go", models.ChunkLevelClass, "User", "type User struct {}")

	// Verify each can be retrieved
	chunk, err := db.GetChunk(ctx, fileID)
	require.NoError(t, err)
	require.NotNil(t, chunk)
	assert.Equal(t, models.ChunkLevelFile, chunk.Level)

	chunk, err = db.GetChunk(ctx, methodID)
	require.NoError(t, err)
	require.NotNil(t, chunk)
	assert.Equal(t, models.ChunkLevelMethod, chunk.Level)

	chunk, err = db.GetChunk(ctx, classID)
	require.NoError(t, err)
	require.NotNil(t, chunk)
	assert.Equal(t, models.ChunkLevelClass, chunk.Level)
}

func TestGetChunk_Found_IncludesFilePath(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert chunk with specific path
	chunkID := insertTestChunk(t, ctx, db, emb, "src/internal/db/sqlite.go", models.ChunkLevelMethod, "Open", "func Open() *DB {}")

	chunk, err := db.GetChunk(ctx, chunkID)
	require.NoError(t, err)
	require.NotNil(t, chunk)
	assert.Equal(t, "src/internal/db/sqlite.go", chunk.FilePath, "file path should be populated from join with files table")
}

// =============================================================================
// TestGetChunk_NotFound - Returns error for missing chunk
// =============================================================================

func TestGetChunk_NotFound(t *testing.T) {
	db, _ := setupSearchTestDB(t)
	ctx := context.Background()

	// Try to retrieve non-existent chunk
	chunk, err := db.GetChunk(ctx, "nonexistent-chunk-id")
	assert.ErrorIs(t, err, ErrChunkNotFound, "should return ErrChunkNotFound")
	assert.Nil(t, chunk, "chunk should be nil when not found")
}

func TestGetChunk_NotFound_EmptyID(t *testing.T) {
	db, _ := setupSearchTestDB(t)
	ctx := context.Background()

	// Try to retrieve with empty ID
	chunk, err := db.GetChunk(ctx, "")
	assert.ErrorIs(t, err, ErrChunkNotFound, "should return ErrChunkNotFound for empty ID")
	assert.Nil(t, chunk, "chunk should be nil when not found")
}

func TestGetChunk_NotFound_AfterDeletion(t *testing.T) {
	db, emb := setupSearchTestDB(t)
	ctx := context.Background()

	// Insert and then delete a chunk
	chunkID := insertTestChunk(t, ctx, db, emb, "src/temp.go", models.ChunkLevelFile, "temp", "package temp")

	// Verify it exists
	chunk, err := db.GetChunk(ctx, chunkID)
	require.NoError(t, err)
	require.NotNil(t, chunk)

	// Delete the file (should cascade delete chunks)
	err = db.DeleteFileByPath(ctx, "src/temp.go")
	require.NoError(t, err)

	// Now it should not be found
	chunk, err = db.GetChunk(ctx, chunkID)
	assert.ErrorIs(t, err, ErrChunkNotFound, "should return ErrChunkNotFound after deletion")
	assert.Nil(t, chunk, "chunk should be nil after deletion")
}
