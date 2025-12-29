package db

import (
	"context"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Tests for file operations
// =============================================================================

func TestGetFileIDByPath(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Non-existent file should return 0
	id, err := db.GetFileIDByPath(ctx, "nonexistent.go")
	require.NoError(t, err)
	assert.Equal(t, int64(0), id)

	// Insert a file
	fileID, err := db.InsertFile(ctx, "src/main.go", "hash123", "go", 1000, time.Now())
	require.NoError(t, err)
	assert.Greater(t, fileID, int64(0))

	// Now GetFileIDByPath should return the ID
	retrievedID, err := db.GetFileIDByPath(ctx, "src/main.go")
	require.NoError(t, err)
	assert.Equal(t, fileID, retrievedID)
}

func TestFileCount(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Initially no files
	count, err := db.FileCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Insert files
	_, err = db.InsertFile(ctx, "src/a.go", "hash1", "go", 100, time.Now())
	require.NoError(t, err)

	count, err = db.FileCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	_, err = db.InsertFile(ctx, "src/b.go", "hash2", "go", 200, time.Now())
	require.NoError(t, err)

	count, err = db.FileCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestDeleteFileByPath(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert a file
	_, err := db.InsertFile(ctx, "src/delete_me.go", "hash", "go", 100, time.Now())
	require.NoError(t, err)

	// Verify it exists
	count, err := db.FileCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Delete the file
	err = db.DeleteFileByPath(ctx, "src/delete_me.go")
	require.NoError(t, err)

	// Verify it's gone
	count, err = db.FileCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Deleting non-existent file should not error
	err = db.DeleteFileByPath(ctx, "nonexistent.go")
	require.NoError(t, err)
}

// =============================================================================
// Tests for chunk operations
// =============================================================================

func TestChunkCount(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Initially no chunks
	count, err := db.ChunkCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Insert a file first
	fileID, err := db.InsertFile(ctx, "src/main.go", "hash", "go", 1000, time.Now())
	require.NoError(t, err)

	// Insert chunks
	chunk1 := &models.Chunk{
		FilePath:  "src/main.go",
		Level:     models.ChunkLevelMethod,
		Name:      "foo",
		Content:   "func foo() {}",
		StartLine: 1,
		EndLine:   1,
	}
	chunk1.SetHashes()

	err = db.InsertChunk(ctx, chunk1, fileID)
	require.NoError(t, err)

	count, err = db.ChunkCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	chunk2 := &models.Chunk{
		FilePath:  "src/main.go",
		Level:     models.ChunkLevelMethod,
		Name:      "bar",
		Content:   "func bar() {}",
		StartLine: 3,
		EndLine:   3,
	}
	chunk2.SetHashes()

	err = db.InsertChunk(ctx, chunk2, fileID)
	require.NoError(t, err)

	count, err = db.ChunkCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestDeleteChunksByFileID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert a file and some chunks
	fileID, err := db.InsertFile(ctx, "src/main.go", "hash", "go", 1000, time.Now())
	require.NoError(t, err)

	chunk1 := &models.Chunk{
		FilePath:  "src/main.go",
		Level:     models.ChunkLevelMethod,
		Name:      "foo",
		Content:   "func foo() {}",
		StartLine: 1,
		EndLine:   1,
	}
	chunk1.SetHashes()
	err = db.InsertChunk(ctx, chunk1, fileID)
	require.NoError(t, err)

	chunk2 := &models.Chunk{
		FilePath:  "src/main.go",
		Level:     models.ChunkLevelMethod,
		Name:      "bar",
		Content:   "func bar() {}",
		StartLine: 3,
		EndLine:   3,
	}
	chunk2.SetHashes()
	err = db.InsertChunk(ctx, chunk2, fileID)
	require.NoError(t, err)

	// Verify chunks exist
	count, err := db.ChunkCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Delete chunks by file ID
	err = db.DeleteChunksByFileID(ctx, fileID)
	require.NoError(t, err)

	// Verify chunks are gone
	count, err = db.ChunkCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Deleting chunks for non-existent file ID should not error
	err = db.DeleteChunksByFileID(ctx, 999999)
	require.NoError(t, err)
}

func TestDeleteChunksByFile(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert a file and some chunks
	fileID, err := db.InsertFile(ctx, "src/delete_chunks.go", "hash", "go", 1000, time.Now())
	require.NoError(t, err)

	chunk := &models.Chunk{
		FilePath:  "src/delete_chunks.go",
		Level:     models.ChunkLevelFile,
		Name:      "delete_chunks.go",
		Content:   "package main",
		StartLine: 1,
		EndLine:   1,
	}
	chunk.SetHashes()
	err = db.InsertChunk(ctx, chunk, fileID)
	require.NoError(t, err)

	// Verify chunk exists
	count, err := db.ChunkCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Delete chunks by file path
	err = db.DeleteChunksByFile(ctx, "src/delete_chunks.go")
	require.NoError(t, err)

	// Verify chunk is gone
	count, err = db.ChunkCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Deleting chunks for non-existent path should not error
	err = db.DeleteChunksByFile(ctx, "nonexistent.go")
	require.NoError(t, err)
}

func TestGetChunkIDsByFile(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Non-existent file should return empty slice
	ids, err := db.GetChunkIDsByFile(ctx, "nonexistent.go")
	require.NoError(t, err)
	assert.Empty(t, ids)

	// Insert a file and chunks
	fileID, err := db.InsertFile(ctx, "src/ids_test.go", "hash", "go", 1000, time.Now())
	require.NoError(t, err)

	chunk1 := &models.Chunk{
		FilePath:  "src/ids_test.go",
		Level:     models.ChunkLevelMethod,
		Name:      "foo",
		Content:   "func foo() {}",
		StartLine: 1,
		EndLine:   1,
	}
	chunk1.SetHashes()
	err = db.InsertChunk(ctx, chunk1, fileID)
	require.NoError(t, err)

	chunk2 := &models.Chunk{
		FilePath:  "src/ids_test.go",
		Level:     models.ChunkLevelMethod,
		Name:      "bar",
		Content:   "func bar() {}",
		StartLine: 3,
		EndLine:   3,
	}
	chunk2.SetHashes()
	err = db.InsertChunk(ctx, chunk2, fileID)
	require.NoError(t, err)

	// Get chunk IDs
	ids, err = db.GetChunkIDsByFile(ctx, "src/ids_test.go")
	require.NoError(t, err)
	assert.Len(t, ids, 2)
	assert.Contains(t, ids, chunk1.ID)
	assert.Contains(t, ids, chunk2.ID)
}

func TestGetChunkIDsByFileID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Non-existent file ID should return empty slice
	ids, err := db.GetChunkIDsByFileID(ctx, 999999)
	require.NoError(t, err)
	assert.Empty(t, ids)

	// Insert a file and chunks
	fileID, err := db.InsertFile(ctx, "src/ids_by_id_test.go", "hash", "go", 1000, time.Now())
	require.NoError(t, err)

	chunk := &models.Chunk{
		FilePath:  "src/ids_by_id_test.go",
		Level:     models.ChunkLevelFile,
		Name:      "ids_by_id_test.go",
		Content:   "package main",
		StartLine: 1,
		EndLine:   1,
	}
	chunk.SetHashes()
	err = db.InsertChunk(ctx, chunk, fileID)
	require.NoError(t, err)

	// Get chunk IDs by file ID
	ids, err = db.GetChunkIDsByFileID(ctx, fileID)
	require.NoError(t, err)
	assert.Len(t, ids, 1)
	assert.Equal(t, chunk.ID, ids[0])
}

func TestGetChunkByID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Non-existent chunk should return nil
	chunk, err := db.GetChunkByID(ctx, "nonexistent-id")
	require.NoError(t, err)
	assert.Nil(t, chunk)

	// Insert a file and chunk
	fileID, err := db.InsertFile(ctx, "src/get_chunk.go", "hash", "go", 1000, time.Now())
	require.NoError(t, err)

	original := &models.Chunk{
		FilePath:  "src/get_chunk.go",
		Level:     models.ChunkLevelMethod,
		Name:      "testFunc",
		Content:   "func testFunc() { return 42 }",
		StartLine: 5,
		EndLine:   7,
	}
	original.SetHashes()
	err = db.InsertChunk(ctx, original, fileID)
	require.NoError(t, err)

	// Retrieve the chunk
	retrieved, err := db.GetChunkByID(ctx, original.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, original.ID, retrieved.ID)
	assert.Equal(t, "src/get_chunk.go", retrieved.FilePath)
	assert.Equal(t, models.ChunkLevel("method"), retrieved.Level)
	assert.Equal(t, "testFunc", retrieved.Name)
	assert.Equal(t, original.Content, retrieved.Content)
	assert.Equal(t, 5, retrieved.StartLine)
	assert.Equal(t, 7, retrieved.EndLine)
}

func TestGetChunksByIDs(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Empty IDs should return empty slice
	chunks, err := db.GetChunksByIDs(ctx, []string{})
	require.NoError(t, err)
	assert.Empty(t, chunks)

	// Non-existent IDs should return empty slice
	chunks, err = db.GetChunksByIDs(ctx, []string{"nonexistent-1", "nonexistent-2"})
	require.NoError(t, err)
	assert.Empty(t, chunks)

	// Insert a file and chunks
	fileID, err := db.InsertFile(ctx, "src/get_chunks.go", "hash", "go", 1000, time.Now())
	require.NoError(t, err)

	chunk1 := &models.Chunk{
		FilePath:  "src/get_chunks.go",
		Level:     models.ChunkLevelMethod,
		Name:      "alpha",
		Content:   "func alpha() {}",
		StartLine: 1,
		EndLine:   1,
	}
	chunk1.SetHashes()
	err = db.InsertChunk(ctx, chunk1, fileID)
	require.NoError(t, err)

	chunk2 := &models.Chunk{
		FilePath:  "src/get_chunks.go",
		Level:     models.ChunkLevelMethod,
		Name:      "beta",
		Content:   "func beta() {}",
		StartLine: 3,
		EndLine:   3,
	}
	chunk2.SetHashes()
	err = db.InsertChunk(ctx, chunk2, fileID)
	require.NoError(t, err)

	chunk3 := &models.Chunk{
		FilePath:  "src/get_chunks.go",
		Level:     models.ChunkLevelMethod,
		Name:      "gamma",
		Content:   "func gamma() {}",
		StartLine: 5,
		EndLine:   5,
	}
	chunk3.SetHashes()
	err = db.InsertChunk(ctx, chunk3, fileID)
	require.NoError(t, err)

	// Get specific chunks
	chunks, err = db.GetChunksByIDs(ctx, []string{chunk1.ID, chunk3.ID})
	require.NoError(t, err)
	assert.Len(t, chunks, 2)

	chunkNames := make(map[string]bool)
	for _, c := range chunks {
		chunkNames[c.Name] = true
	}
	assert.True(t, chunkNames["alpha"])
	assert.True(t, chunkNames["gamma"])
	assert.False(t, chunkNames["beta"])
}

func TestGetChunkByID_WithParentID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert a file
	fileID, err := db.InsertFile(ctx, "src/parent_test.go", "hash", "go", 1000, time.Now())
	require.NoError(t, err)

	// Insert parent chunk (class)
	parent := &models.Chunk{
		FilePath:  "src/parent_test.go",
		Level:     models.ChunkLevelClass,
		Name:      "MyClass",
		Content:   "type MyClass struct {}",
		StartLine: 1,
		EndLine:   10,
	}
	parent.SetHashes()
	err = db.InsertChunk(ctx, parent, fileID)
	require.NoError(t, err)

	// Insert child chunk (method with parent)
	child := &models.Chunk{
		FilePath:  "src/parent_test.go",
		Level:     models.ChunkLevelMethod,
		Name:      "MyMethod",
		Content:   "func (m *MyClass) MyMethod() {}",
		StartLine: 5,
		EndLine:   7,
		ParentID:  &parent.ID,
	}
	child.SetHashes()
	err = db.InsertChunk(ctx, child, fileID)
	require.NoError(t, err)

	// Retrieve child and verify parent ID is set
	retrieved, err := db.GetChunkByID(ctx, child.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	require.NotNil(t, retrieved.ParentID)
	assert.Equal(t, parent.ID, *retrieved.ParentID)
}

// =============================================================================
// Tests for ClearAll
// =============================================================================

func TestClearAll(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert files, chunks, and embeddings
	fileID, err := db.InsertFile(ctx, "src/clear_test.go", "hash", "go", 1000, time.Now())
	require.NoError(t, err)

	chunk := &models.Chunk{
		FilePath:  "src/clear_test.go",
		Level:     models.ChunkLevelFile,
		Name:      "clear_test.go",
		Content:   "package main",
		StartLine: 1,
		EndLine:   1,
	}
	chunk.SetHashes()
	err = db.InsertChunk(ctx, chunk, fileID)
	require.NoError(t, err)

	embedding := makeEmbedding(0.5)
	err = db.InsertEmbedding(ctx, chunk.ID, embedding)
	require.NoError(t, err)

	// Verify data exists
	fileCount, err := db.FileCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), fileCount)

	chunkCount, err := db.ChunkCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), chunkCount)

	embeddingCount, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, embeddingCount)

	// Clear all data
	err = db.ClearAll(ctx)
	require.NoError(t, err)

	// Verify all data is cleared
	fileCount, err = db.FileCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), fileCount)

	chunkCount, err = db.ChunkCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), chunkCount)

	embeddingCount, err = db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, embeddingCount)
}

func TestClearAll_EmptyDatabase(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Clearing an empty database should not error
	err := db.ClearAll(ctx)
	require.NoError(t, err)

	// Verify counts are still zero
	fileCount, err := db.FileCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), fileCount)
}
