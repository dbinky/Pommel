package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	// Verify database file was created
	dbPath := filepath.Join(tmpDir, ".pommel", DatabaseFile)
	assert.Equal(t, dbPath, db.Path())

	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "database file should exist")

	// Verify connection is valid
	assert.NotNil(t, db.Conn())
}

func TestOpen_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "myproject")

	// Don't create the project directory - Open should create .pommel inside it
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	db, err := Open(projectDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	// Verify .pommel directory was created
	pommelDir := filepath.Join(projectDir, ".pommel")
	info, err := os.Stat(pommelDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestSqliteVecAvailable(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	// Verify sqlite-vec is available by checking vec_version()
	var version string
	err = db.QueryRow(context.Background(), "SELECT vec_version()").Scan(&version)
	require.NoError(t, err)
	assert.NotEmpty(t, version, "vec_version() should return a version string")
}

func TestMigrate(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Run migrations
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Verify schema version
	version, err := db.GetSchemaVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, SchemaVersion, version)
}

func TestMigrate_CreatesFilesTable(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Verify files table exists
	exists, err := db.TableExists(ctx, "files")
	require.NoError(t, err)
	assert.True(t, exists, "files table should exist")

	// Test inserting a file record
	_, err = db.Exec(ctx, `
		INSERT INTO files (path, content_hash, size, modified_at, language)
		VALUES (?, ?, ?, datetime('now'), ?)
	`, "/test/file.go", "abc123", 1024, "go")
	require.NoError(t, err)

	// Verify the record was inserted
	var path string
	err = db.QueryRow(ctx, "SELECT path FROM files WHERE path = ?", "/test/file.go").Scan(&path)
	require.NoError(t, err)
	assert.Equal(t, "/test/file.go", path)
}

func TestMigrate_CreatesChunksTable(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Verify chunks table exists
	exists, err := db.TableExists(ctx, "chunks")
	require.NoError(t, err)
	assert.True(t, exists, "chunks table should exist")

	// First insert a file to satisfy foreign key
	_, err = db.Exec(ctx, `
		INSERT INTO files (path, content_hash, size, modified_at)
		VALUES (?, ?, ?, datetime('now'))
	`, "/test/file.go", "abc123", 1024)
	require.NoError(t, err)

	// Get the file ID
	var fileID int64
	err = db.QueryRow(ctx, "SELECT id FROM files WHERE path = ?", "/test/file.go").Scan(&fileID)
	require.NoError(t, err)

	// Test inserting a chunk record
	_, err = db.Exec(ctx, `
		INSERT INTO chunks (id, file_id, level, name, start_line, end_line, content, content_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "chunk-1", fileID, "method", "TestFunction", 10, 20, "func TestFunction() {}", "hash123")
	require.NoError(t, err)

	// Verify the record was inserted
	var level string
	err = db.QueryRow(ctx, "SELECT level FROM chunks WHERE id = ?", "chunk-1").Scan(&level)
	require.NoError(t, err)
	assert.Equal(t, "method", level)
}

func TestMigrate_CreatesChunkEmbeddingsTable(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Verify chunk_embeddings virtual table exists
	// Note: sqlite-vec virtual tables show up in sqlite_master as regular tables
	var count int
	err = db.QueryRow(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name='chunk_embeddings'
	`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "chunk_embeddings table should exist")

	// Test inserting an embedding (768-dimensional vector)
	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = float32(i) * 0.001
	}

	// Serialize the embedding using sqlite-vec's serialization function
	serialized, err := sqlite_vec.SerializeFloat32(embedding)
	require.NoError(t, err)

	// Insert using vec0 format
	_, err = db.Exec(ctx, `
		INSERT INTO chunk_embeddings (chunk_id, embedding)
		VALUES (?, ?)
	`, "chunk-1", serialized)
	require.NoError(t, err)

	// Verify we can query the embedding
	var chunkID string
	err = db.QueryRow(ctx, "SELECT chunk_id FROM chunk_embeddings WHERE chunk_id = ?", "chunk-1").Scan(&chunkID)
	require.NoError(t, err)
	assert.Equal(t, "chunk-1", chunkID)
}

func TestMigrate_CreatesMetadataTable(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Verify metadata table exists
	exists, err := db.TableExists(ctx, "metadata")
	require.NoError(t, err)
	assert.True(t, exists, "metadata table should exist")

	// Test inserting a metadata record
	_, err = db.Exec(ctx, `
		INSERT INTO metadata (key, value)
		VALUES (?, ?)
	`, "embedding_model", "jina-embeddings-v2-base-code")
	require.NoError(t, err)

	// Verify the record was inserted
	var value string
	err = db.QueryRow(ctx, "SELECT value FROM metadata WHERE key = ?", "embedding_model").Scan(&value)
	require.NoError(t, err)
	assert.Equal(t, "jina-embeddings-v2-base-code", value)
}

func TestMigrate_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Run migrations multiple times
	for i := 0; i < 3; i++ {
		err = db.Migrate(ctx)
		require.NoError(t, err, "migration %d should succeed", i+1)
	}

	// Verify schema version is unchanged
	version, err := db.GetSchemaVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, SchemaVersion, version)
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)

	err = db.Close()
	require.NoError(t, err)

	// Verify connection is closed by attempting a query
	_, err = db.Exec(context.Background(), "SELECT 1")
	assert.Error(t, err, "query should fail after close")
}

func TestTableExists(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Before migration, custom tables shouldn't exist
	exists, err := db.TableExists(ctx, "files")
	require.NoError(t, err)
	assert.False(t, exists)

	// After migration, tables should exist
	err = db.Migrate(ctx)
	require.NoError(t, err)

	exists, err = db.TableExists(ctx, "files")
	require.NoError(t, err)
	assert.True(t, exists)

	// Non-existent table
	exists, err = db.TableExists(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestVirtualTableExists(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Before migration, virtual tables shouldn't exist
	exists, err := db.VirtualTableExists(ctx, "chunk_embeddings")
	require.NoError(t, err)
	assert.False(t, exists)

	// After migration, virtual tables should exist
	err = db.Migrate(ctx)
	require.NoError(t, err)

	exists, err = db.VirtualTableExists(ctx, "chunk_embeddings")
	require.NoError(t, err)
	assert.True(t, exists)

	// Non-existent virtual table
	exists, err = db.VirtualTableExists(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)

	// Regular table should not be reported as virtual table
	exists, err = db.VirtualTableExists(ctx, "files")
	require.NoError(t, err)
	assert.False(t, exists)
}

// =============================================================================
// Additional Coverage Tests
// =============================================================================

func TestClearAll_ClearsAllTables(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Insert some data
	fileID, err := db.InsertFile(ctx, "/test/file.go", "hash123", "go", 1000, time.Now())
	require.NoError(t, err)

	chunk := &models.Chunk{
		ID:          "chunk-clear-1",
		Level:       models.ChunkLevelMethod,
		Name:        "TestFunc",
		StartLine:   1,
		EndLine:     10,
		Content:     "func TestFunc() {}",
		ContentHash: "contenthash",
	}
	err = db.InsertChunk(ctx, chunk, fileID)
	require.NoError(t, err)

	// Clear all
	err = db.ClearAll(ctx)
	require.NoError(t, err)

	// Verify tables are empty
	fileCount, err := db.FileCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), fileCount)

	chunkCount, err := db.ChunkCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), chunkCount)
}

func TestInsertFile_ReturnsDifferentIDsForDifferentFiles(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Insert multiple files
	id1, err := db.InsertFile(ctx, "/test/file1.go", "hash1", "go", 100, time.Now())
	require.NoError(t, err)

	id2, err := db.InsertFile(ctx, "/test/file2.go", "hash2", "go", 200, time.Now())
	require.NoError(t, err)

	id3, err := db.InsertFile(ctx, "/test/file3.go", "hash3", "go", 300, time.Now())
	require.NoError(t, err)

	// IDs should all be different
	assert.NotEqual(t, id1, id2)
	assert.NotEqual(t, id2, id3)
	assert.NotEqual(t, id1, id3)
}

func TestGetFileIDByPath_ReturnsZeroForNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Try to get ID for non-existent file - returns 0 and no error
	id, err := db.GetFileIDByPath(ctx, "/nonexistent/path.go")
	require.NoError(t, err)
	assert.Equal(t, int64(0), id)
}

func TestDeleteFileByPath_NoopForNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Delete non-existent file should not error
	err = db.DeleteFileByPath(ctx, "/nonexistent/path.go")
	require.NoError(t, err)
}

func TestDeleteChunksByFile_NoopForNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Delete chunks for non-existent file should not error
	err = db.DeleteChunksByFile(ctx, "/nonexistent/path.go")
	require.NoError(t, err)
}

func TestGetChunkIDsByFile_ReturnsEmptyForNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Get chunk IDs for non-existent file
	ids, err := db.GetChunkIDsByFile(ctx, "/nonexistent/path.go")
	require.NoError(t, err)
	assert.Empty(t, ids)
}

func TestGetChunkByID_ReturnsNilForNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Get non-existent chunk - returns nil chunk but no error
	chunk, err := db.GetChunkByID(ctx, "nonexistent-chunk-id")
	require.NoError(t, err)
	assert.Nil(t, chunk)
}

func TestEmbeddingCount_ReturnsZeroInitially(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestMigrate_IsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Run migrate multiple times - should succeed each time
	err = db.Migrate(ctx)
	require.NoError(t, err)

	err = db.Migrate(ctx)
	require.NoError(t, err)

	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Verify schema version
	version, err := db.GetSchemaVersion(ctx)
	require.NoError(t, err)
	assert.Greater(t, version, 0)
}

func TestInsertFile_WithAllFields(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	modTime := time.Now().Add(-time.Hour)
	fileID, err := db.InsertFile(ctx, "/test/complete.go", "completehash", "go", 12345, modTime)
	require.NoError(t, err)
	assert.Greater(t, fileID, int64(0))

	// Verify file was inserted correctly
	retrievedID, err := db.GetFileIDByPath(ctx, "/test/complete.go")
	require.NoError(t, err)
	assert.Equal(t, fileID, retrievedID)
}

func TestDeleteChunksByFileID_DeletesChunks(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Insert file and chunk
	fileID, err := db.InsertFile(ctx, "/test/deletable.go", "hash123", "go", 1000, time.Now())
	require.NoError(t, err)

	chunk := &models.Chunk{
		ID:          "chunk-delete-test",
		Level:       models.ChunkLevelMethod,
		Name:        "TestFunc",
		StartLine:   1,
		EndLine:     10,
		Content:     "func TestFunc() {}",
		ContentHash: "contenthash",
	}
	err = db.InsertChunk(ctx, chunk, fileID)
	require.NoError(t, err)

	// Verify chunk exists
	chunkCount, err := db.ChunkCount(ctx)
	require.NoError(t, err)
	assert.Greater(t, chunkCount, int64(0))

	// Delete chunks by file ID
	err = db.DeleteChunksByFileID(ctx, fileID)
	require.NoError(t, err)

	// Verify chunks are deleted
	chunkCount, err = db.ChunkCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), chunkCount)
}

func TestGetChunksByIDs_ReturnsMultipleChunks(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Insert file
	fileID, err := db.InsertFile(ctx, "/test/multi.go", "multihash", "go", 1000, time.Now())
	require.NoError(t, err)

	// Insert multiple chunks
	chunkIDs := []string{"chunk-multi-1", "chunk-multi-2", "chunk-multi-3"}
	for i, id := range chunkIDs {
		chunk := &models.Chunk{
			ID:          id,
			Level:       models.ChunkLevelMethod,
			Name:        "Func" + string(rune('A'+i)),
			StartLine:   i * 10,
			EndLine:     (i + 1) * 10,
			Content:     "func content",
			ContentHash: "hash" + id,
		}
		err = db.InsertChunk(ctx, chunk, fileID)
		require.NoError(t, err)
	}

	// Get chunks by IDs
	chunks, err := db.GetChunksByIDs(ctx, chunkIDs)
	require.NoError(t, err)
	assert.Len(t, chunks, 3)
}

func TestGetChunkIDsByFileID_ReturnsChunkIDs(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Insert file
	fileID, err := db.InsertFile(ctx, "/test/withchunks.go", "chunkhash", "go", 1000, time.Now())
	require.NoError(t, err)

	// Insert chunks
	expectedIDs := []string{"chunk-byfile-1", "chunk-byfile-2"}
	for _, id := range expectedIDs {
		chunk := &models.Chunk{
			ID:          id,
			Level:       models.ChunkLevelMethod,
			Name:        "TestFunc",
			StartLine:   1,
			EndLine:     10,
			Content:     "func content",
			ContentHash: "hash" + id,
		}
		err = db.InsertChunk(ctx, chunk, fileID)
		require.NoError(t, err)
	}

	// Get chunk IDs by file ID
	ids, err := db.GetChunkIDsByFileID(ctx, fileID)
	require.NoError(t, err)
	assert.Len(t, ids, 2)
}
