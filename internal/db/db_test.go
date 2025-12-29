package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir)
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

	db, err := Open(projectDir)
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

	db, err := Open(tmpDir)
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

	db, err := Open(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Run migrations
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Verify schema version
	version, err := db.GetSchemaVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, version)
}

func TestMigrate_CreatesFilesTable(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir)
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

	db, err := Open(tmpDir)
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

	db, err := Open(tmpDir)
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

	db, err := Open(tmpDir)
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

	db, err := Open(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Run migrations multiple times
	for i := 0; i < 3; i++ {
		err = db.Migrate(ctx)
		require.NoError(t, err, "migration %d should succeed", i+1)
	}

	// Verify schema version is still 1
	version, err := db.GetSchemaVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, version)
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir)
	require.NoError(t, err)

	err = db.Close()
	require.NoError(t, err)

	// Verify connection is closed by attempting a query
	_, err = db.Exec(context.Background(), "SELECT 1")
	assert.Error(t, err, "query should fail after close")
}

func TestTableExists(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir)
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
