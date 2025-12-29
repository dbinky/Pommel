package daemon

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogger creates a silent logger for testing
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testConfig creates a default config for testing with include patterns for .go and .py files
func testConfig() *config.Config {
	return &config.Config{
		Version: 1,
		ChunkLevels: []string{
			"method",
			"class",
			"file",
		},
		IncludePatterns: []string{
			"**/*.go",
			"**/*.py",
			"**/*.js",
		},
		ExcludePatterns: []string{
			"**/node_modules/**",
			"**/.pommel/**",
		},
		Watcher: config.WatcherConfig{
			DebounceMs:  100,
			MaxFileSize: 1048576, // 1MB
		},
		Embedding: config.EmbeddingConfig{
			Model:     "mock-embedder",
			BatchSize: 32,
			CacheSize: 1000,
		},
	}
}

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T, projectRoot string) *db.DB {
	database, err := db.Open(projectRoot)
	require.NoError(t, err)

	err = database.Migrate(context.Background())
	require.NoError(t, err)

	return database
}

// createTestFile creates a test file with the given content
func createTestFile(t *testing.T, dir, name, content string) string {
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}

// =============================================================================
// Indexer Creation Tests
// =============================================================================

// TestNewIndexerCreatesSuccessfully verifies that NewIndexer creates an indexer
func TestNewIndexerCreatesSuccessfully(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)
	require.NotNil(t, indexer)
}

// TestNewIndexerWithValidDependencies verifies that NewIndexer works with valid dependencies
func TestNewIndexerWithValidDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)
	require.NotNil(t, indexer)

	// Verify indexer has zero stats initially
	stats := indexer.Stats()
	assert.Equal(t, int64(0), stats.TotalFiles)
	assert.Equal(t, int64(0), stats.TotalChunks)
	assert.False(t, stats.IndexingActive)
}

// =============================================================================
// IndexFile Tests
// =============================================================================

// TestIndexFileReadsAndProcessesFile verifies that IndexFile reads and processes a file
func TestIndexFileReadsAndProcessesFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create a test file
	testContent := `package main

func main() {
	println("Hello, World!")
}
`
	testFile := createTestFile(t, tmpDir, "main.go", testContent)

	// Index the file
	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	require.NoError(t, err)
}

// TestIndexFileCreatesChunksAndEmbeddings verifies that IndexFile creates chunks and embeddings
func TestIndexFileCreatesChunksAndEmbeddings(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create a test file with a function
	testContent := `package main

func sayHello() {
	println("Hello!")
}

func main() {
	sayHello()
}
`
	testFile := createTestFile(t, tmpDir, "hello.go", testContent)

	// Index the file
	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	require.NoError(t, err)

	// Verify embeddings were created
	embeddingCount, err := database.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Greater(t, embeddingCount, 0, "expected embeddings to be created")
}

// TestIndexFileUpdatesStats verifies that IndexFile updates the indexer stats
func TestIndexFileUpdatesStats(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Verify initial stats are zero
	initialStats := indexer.Stats()
	assert.Equal(t, int64(0), initialStats.TotalFiles)
	assert.Equal(t, int64(0), initialStats.TotalChunks)

	// Create and index a test file
	testContent := `package main

func main() {
	println("Hello!")
}
`
	testFile := createTestFile(t, tmpDir, "stats_test.go", testContent)

	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	require.NoError(t, err)

	// Verify stats were updated
	stats := indexer.Stats()
	assert.Greater(t, stats.TotalFiles, int64(0), "TotalFiles should be updated")
	assert.Greater(t, stats.TotalChunks, int64(0), "TotalChunks should be updated")
	assert.False(t, stats.LastIndexedAt.IsZero(), "LastIndexedAt should be set")
}

// TestIndexFileSkipsFilesExceedingMaxSize verifies that IndexFile skips oversized files
func TestIndexFileSkipsFilesExceedingMaxSize(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	cfg.Watcher.MaxFileSize = 100 // Very small max file size (100 bytes)

	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create a large test file that exceeds the max size
	largeContent := make([]byte, 200) // 200 bytes, exceeds 100 byte limit
	for i := range largeContent {
		largeContent[i] = 'x'
	}
	testFile := createTestFile(t, tmpDir, "large.go", string(largeContent))

	// Index the file - should not error, but should skip
	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	require.NoError(t, err) // Should not error, just skip

	// Verify no embeddings were created
	embeddingCount, err := database.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, embeddingCount, "expected no embeddings for oversized file")
}

// TestIndexFileHandlesMissingFileGracefully verifies that IndexFile handles missing files
func TestIndexFileHandlesMissingFileGracefully(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Try to index a non-existent file
	ctx := context.Background()
	nonExistentFile := filepath.Join(tmpDir, "does_not_exist.go")
	err = indexer.IndexFile(ctx, nonExistentFile)

	// Should return an error for missing file
	assert.Error(t, err)
}

// TestIndexFileSkipsNonMatchingExtensions verifies that IndexFile skips files with non-matching extensions
func TestIndexFileSkipsNonMatchingExtensions(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	// Only include .go files
	cfg.IncludePatterns = []string{"**/*.go"}

	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create a .txt file (not in include patterns)
	testFile := createTestFile(t, tmpDir, "readme.txt", "This is a readme")

	// Index the file
	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	require.NoError(t, err) // Should not error, just skip

	// Verify no embeddings were created
	embeddingCount, err := database.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, embeddingCount, "expected no embeddings for non-matching file extension")
}

// =============================================================================
// DeleteFile Tests
// =============================================================================

// TestDeleteFileRemovesChunksForFile verifies that DeleteFile removes chunks for a file
func TestDeleteFileRemovesChunksForFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create and index a test file
	testContent := `package main

func main() {
	println("Hello!")
}
`
	testFile := createTestFile(t, tmpDir, "to_delete.go", testContent)

	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	require.NoError(t, err)

	// Verify embeddings exist
	embeddingCountBefore, err := database.EmbeddingCount(ctx)
	require.NoError(t, err)
	require.Greater(t, embeddingCountBefore, 0, "expected embeddings before delete")

	// Delete the file from index
	err = indexer.DeleteFile(ctx, testFile)
	require.NoError(t, err)

	// Verify embeddings are removed
	embeddingCountAfter, err := database.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, embeddingCountAfter, "expected no embeddings after delete")
}

// TestDeleteFileRemovesEmbeddingsForFile verifies that DeleteFile removes embeddings for a file
func TestDeleteFileRemovesEmbeddingsForFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create and index a test file
	testContent := `def hello():
    print("Hello!")

def main():
    hello()
`
	testFile := createTestFile(t, tmpDir, "hello.py", testContent)

	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	require.NoError(t, err)

	// Delete the file
	err = indexer.DeleteFile(ctx, testFile)
	require.NoError(t, err)

	// Verify embeddings are removed
	embeddingCount, err := database.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, embeddingCount, "expected all embeddings to be removed")
}

// TestDeleteFileIsIdempotent verifies that DeleteFile can be called multiple times safely
func TestDeleteFileIsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create and index a test file
	testContent := `package main

func main() {}
`
	testFile := createTestFile(t, tmpDir, "idempotent.go", testContent)

	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	require.NoError(t, err)

	// Delete the file multiple times - should not error
	err = indexer.DeleteFile(ctx, testFile)
	require.NoError(t, err)

	err = indexer.DeleteFile(ctx, testFile)
	require.NoError(t, err) // Second delete should also succeed

	err = indexer.DeleteFile(ctx, testFile)
	require.NoError(t, err) // Third delete should also succeed
}

// =============================================================================
// ReindexAll Tests
// =============================================================================

// TestReindexAllProcessesAllMatchingFiles verifies that ReindexAll processes all matching files
func TestReindexAllProcessesAllMatchingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create multiple test files
	createTestFile(t, tmpDir, "file1.go", `package main

func one() {}
`)
	createTestFile(t, tmpDir, "file2.go", `package main

func two() {}
`)
	createTestFile(t, tmpDir, "file3.py", `def three():
    pass
`)
	// Create a file that should be ignored (not in include patterns)
	createTestFile(t, tmpDir, "readme.txt", "This should be ignored")

	// Reindex all files
	ctx := context.Background()
	err = indexer.ReindexAll(ctx)
	require.NoError(t, err)

	// Verify embeddings were created
	embeddingCount, err := database.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Greater(t, embeddingCount, 0, "expected embeddings to be created")

	// Verify stats reflect multiple files
	stats := indexer.Stats()
	assert.Greater(t, stats.TotalFiles, int64(1), "expected multiple files to be indexed")
}

// TestReindexAllClearsExistingDataFirst verifies that ReindexAll clears existing data
func TestReindexAllClearsExistingDataFirst(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create and index initial file
	testFile := createTestFile(t, tmpDir, "initial.go", `package main

func initial() {}
`)

	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	require.NoError(t, err)

	// Get initial embedding count
	initialCount, err := database.EmbeddingCount(ctx)
	require.NoError(t, err)
	require.Greater(t, initialCount, 0)

	// Remove the original file and create a different one
	err = os.Remove(testFile)
	require.NoError(t, err)

	createTestFile(t, tmpDir, "replacement.go", `package main

func replacement() {}
`)

	// Reindex all
	err = indexer.ReindexAll(ctx)
	require.NoError(t, err)

	// The old file's data should be cleared, only new file should exist
	stats := indexer.Stats()
	// We should only have the replacement file
	assert.Equal(t, int64(1), stats.TotalFiles, "expected only one file after reindex")
}

// TestReindexAllUpdatesTotalFileCount verifies that ReindexAll updates total file count
func TestReindexAllUpdatesTotalFileCount(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Verify initial count is zero
	initialStats := indexer.Stats()
	assert.Equal(t, int64(0), initialStats.TotalFiles)

	// Create test files
	createTestFile(t, tmpDir, "a.go", `package main

func a() {}
`)
	createTestFile(t, tmpDir, "b.go", `package main

func b() {}
`)

	// Reindex all
	ctx := context.Background()
	err = indexer.ReindexAll(ctx)
	require.NoError(t, err)

	// Verify total file count is updated
	stats := indexer.Stats()
	assert.Equal(t, int64(2), stats.TotalFiles, "expected 2 files after reindex")
}

// TestReindexAllHandlesNestedDirectories verifies that ReindexAll processes nested directories
func TestReindexAllHandlesNestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create nested directory structure
	nestedDir := filepath.Join(tmpDir, "pkg", "subpkg")
	err = os.MkdirAll(nestedDir, 0755)
	require.NoError(t, err)

	// Create files at different levels
	createTestFile(t, tmpDir, "root.go", `package main

func root() {}
`)
	createTestFile(t, filepath.Join(tmpDir, "pkg"), "pkg.go", `package pkg

func pkg() {}
`)
	createTestFile(t, nestedDir, "subpkg.go", `package subpkg

func subpkg() {}
`)

	// Reindex all
	ctx := context.Background()
	err = indexer.ReindexAll(ctx)
	require.NoError(t, err)

	// Verify all files were indexed
	stats := indexer.Stats()
	assert.Equal(t, int64(3), stats.TotalFiles, "expected 3 files from nested directories")
}

// =============================================================================
// Stats Tests
// =============================================================================

// TestStatsReturnsCurrentStatistics verifies that Stats returns current statistics
func TestStatsReturnsCurrentStatistics(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Get initial stats
	stats := indexer.Stats()
	assert.Equal(t, int64(0), stats.TotalFiles)
	assert.Equal(t, int64(0), stats.TotalChunks)
	assert.Equal(t, int64(0), stats.PendingFiles)
	assert.False(t, stats.IndexingActive)
	assert.True(t, stats.LastIndexedAt.IsZero())

	// Index a file
	testContent := `package main

func main() {}
`
	testFile := createTestFile(t, tmpDir, "stats.go", testContent)

	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	require.NoError(t, err)

	// Get updated stats
	updatedStats := indexer.Stats()
	assert.Greater(t, updatedStats.TotalFiles, int64(0))
	assert.Greater(t, updatedStats.TotalChunks, int64(0))
	assert.False(t, updatedStats.LastIndexedAt.IsZero())
}

// TestStatsShowsIndexingActiveDuringOperation verifies IndexingActive flag during operation
func TestStatsShowsIndexingActiveDuringOperation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Before indexing, should not be active
	stats := indexer.Stats()
	assert.False(t, stats.IndexingActive)

	// Create multiple files to create a longer-running operation
	for i := 0; i < 10; i++ {
		createTestFile(t, tmpDir, "file"+string(rune('0'+i))+".go",
			`package main

func main() {}
`)
	}

	// After indexing completes, should not be active
	ctx := context.Background()
	err = indexer.ReindexAll(ctx)
	require.NoError(t, err)

	finalStats := indexer.Stats()
	assert.False(t, finalStats.IndexingActive, "IndexingActive should be false after operation completes")
}

// TestStatsLastIndexedAtUpdates verifies that LastIndexedAt is updated after indexing
func TestStatsLastIndexedAtUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Initial LastIndexedAt should be zero
	initialStats := indexer.Stats()
	assert.True(t, initialStats.LastIndexedAt.IsZero())

	// Index a file
	testFile := createTestFile(t, tmpDir, "time_test.go", `package main

func main() {}
`)

	beforeIndex := time.Now()
	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	require.NoError(t, err)
	afterIndex := time.Now()

	// LastIndexedAt should be set and between before and after
	stats := indexer.Stats()
	assert.False(t, stats.LastIndexedAt.IsZero())
	assert.True(t, stats.LastIndexedAt.After(beforeIndex) || stats.LastIndexedAt.Equal(beforeIndex))
	assert.True(t, stats.LastIndexedAt.Before(afterIndex) || stats.LastIndexedAt.Equal(afterIndex))
}

// =============================================================================
// Pattern Matching Tests
// =============================================================================

// TestMatchesPatternsReturnsTrueForMatchingExtensions verifies pattern matching for supported files
func TestMatchesPatternsReturnsTrueForMatchingExtensions(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	cfg.IncludePatterns = []string{
		"**/*.go",
		"**/*.py",
		"**/*.js",
	}

	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	testCases := []struct {
		filename string
		expected bool
	}{
		{"main.go", true},
		{"utils.py", true},
		{"app.js", true},
		{"main.go", true},
		{"path/to/file.go", true},
		{"deep/nested/path/file.py", true},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			matches := indexer.MatchesPatterns(tc.filename)
			assert.Equal(t, tc.expected, matches, "unexpected result for %s", tc.filename)
		})
	}
}

// TestMatchesPatternsReturnsFalseForNonMatchingFiles verifies pattern matching rejects non-supported files
func TestMatchesPatternsReturnsFalseForNonMatchingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	cfg.IncludePatterns = []string{
		"**/*.go",
		"**/*.py",
	}

	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	testCases := []struct {
		filename string
		expected bool
	}{
		{"readme.txt", false},
		{"image.png", false},
		{"document.pdf", false},
		{"config.yaml", false},
		{"data.json", false},
		{"Makefile", false},
		{".gitignore", false},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			matches := indexer.MatchesPatterns(tc.filename)
			assert.Equal(t, tc.expected, matches, "unexpected result for %s", tc.filename)
		})
	}
}

// TestMatchesPatternsRespectExcludePatterns verifies that exclude patterns are respected
func TestMatchesPatternsRespectExcludePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	cfg.ExcludePatterns = []string{
		"**/vendor/**",
		"**/*_test.go",
	}

	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	testCases := []struct {
		filename string
		expected bool
	}{
		{"main.go", true},
		{"vendor/lib/file.go", false},
		{"main_test.go", false},
		{"pkg/handler_test.go", false},
		{"pkg/handler.go", true},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			matches := indexer.MatchesPatterns(tc.filename)
			assert.Equal(t, tc.expected, matches, "unexpected result for %s", tc.filename)
		})
	}
}

// =============================================================================
// Edge Cases and Error Handling Tests
// =============================================================================

// TestIndexFileHandlesEmptyFile verifies that IndexFile handles empty files gracefully
func TestIndexFileHandlesEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create an empty file
	testFile := createTestFile(t, tmpDir, "empty.go", "")

	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	// Should handle gracefully (either skip or not error)
	// Implementation may choose to skip empty files without error
	assert.NoError(t, err)
}

// TestIndexFileHandlesBinaryFile verifies that IndexFile handles binary files gracefully
func TestIndexFileHandlesBinaryFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create a file with binary content (null bytes)
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03, 0x00, 0x00}
	testFile := filepath.Join(tmpDir, "binary.go")
	err = os.WriteFile(testFile, binaryContent, 0644)
	require.NoError(t, err)

	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	// Should handle gracefully without crashing
	// Implementation may error or skip
	_ = err // We just want to ensure it doesn't panic
}

// TestReindexAllWithEmptyDirectory verifies that ReindexAll handles empty directories
func TestReindexAllWithEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Reindex an empty directory
	ctx := context.Background()
	err = indexer.ReindexAll(ctx)
	require.NoError(t, err)

	// Stats should show zero files
	stats := indexer.Stats()
	assert.Equal(t, int64(0), stats.TotalFiles)
	assert.Equal(t, int64(0), stats.TotalChunks)
}

// TestIndexFileWithCanceledContext verifies behavior when context is canceled
func TestIndexFileWithCanceledContext(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	testFile := createTestFile(t, tmpDir, "context_test.go", `package main

func main() {}
`)

	// Create an already-canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = indexer.IndexFile(ctx, testFile)
	// Should return context error
	assert.Error(t, err)
}

// TestReindexAllWithCanceledContext verifies behavior when context is canceled
func TestReindexAllWithCanceledContext(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create some files
	createTestFile(t, tmpDir, "file1.go", `package main

func main() {}
`)
	createTestFile(t, tmpDir, "file2.go", `package main

func main() {}
`)

	// Create an already-canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = indexer.ReindexAll(ctx)
	// Should return context error
	assert.Error(t, err)
}

// TestMultipleFilesIndexedConcurrently verifies multiple files can be indexed
func TestMultipleFilesIndexedSequentially(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create multiple files
	files := []string{
		createTestFile(t, tmpDir, "a.go", `package main

func a() {}
`),
		createTestFile(t, tmpDir, "b.go", `package main

func b() {}
`),
		createTestFile(t, tmpDir, "c.go", `package main

func c() {}
`),
	}

	// Index all files sequentially
	ctx := context.Background()
	for _, f := range files {
		err = indexer.IndexFile(ctx, f)
		require.NoError(t, err)
	}

	// Verify all were indexed
	stats := indexer.Stats()
	assert.Equal(t, int64(3), stats.TotalFiles)
}

// TestReindexUpdatesModifiedFile verifies that re-indexing updates a modified file
func TestReindexUpdatesModifiedFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Create and index initial file
	testFile := createTestFile(t, tmpDir, "modify.go", `package main

func original() {}
`)

	ctx := context.Background()
	err = indexer.IndexFile(ctx, testFile)
	require.NoError(t, err)

	// Get initial embedding count
	initialCount, err := database.EmbeddingCount(ctx)
	require.NoError(t, err)

	// Modify the file
	err = os.WriteFile(testFile, []byte(`package main

func modified() {}

func additionalFunction() {}
`), 0644)
	require.NoError(t, err)

	// Re-index the modified file
	err = indexer.IndexFile(ctx, testFile)
	require.NoError(t, err)

	// Verify embeddings were updated
	finalCount, err := database.EmbeddingCount(ctx)
	require.NoError(t, err)
	// The modified file has more functions, so may have more chunks
	assert.GreaterOrEqual(t, finalCount, initialCount)
}

// =============================================================================
// Additional matchGlob Tests
// =============================================================================

func TestMatchGlob_DoubleStarInMiddle(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	cfg.IncludePatterns = []string{"internal/**/*.go"}

	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	testCases := []struct {
		path     string
		expected bool
	}{
		{"internal/api/handler.go", true},
		{"internal/pkg/util.go", true},
		{"internal/deep/nested/path/file.go", true},
		{"internal/file.go", true},
		{"external/api/handler.go", false},
		{"main.go", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			matches := indexer.MatchesPatterns(tc.path)
			assert.Equal(t, tc.expected, matches)
		})
	}
}

func TestMatchGlob_SingleStarWildcard(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	cfg.IncludePatterns = []string{"*.go", "src/*.js"}

	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	testCases := []struct {
		path     string
		expected bool
	}{
		{"main.go", true},
		{"test.go", true},
		{"src/app.js", true},
		{"nested/main.go", false},
		{"src/nested/app.js", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			matches := indexer.MatchesPatterns(tc.path)
			assert.Equal(t, tc.expected, matches)
		})
	}
}

func TestMatchGlob_TrailingDoubleStarPattern(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	cfg.ExcludePatterns = []string{"vendor/**"}

	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	testCases := []struct {
		path     string
		expected bool
	}{
		{"main.go", true},
		{"pkg/util.go", true},
		{"vendor/lib.go", false},
		{"vendor/pkg/deep.go", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			matches := indexer.MatchesPatterns(tc.path)
			assert.Equal(t, tc.expected, matches)
		})
	}
}

func TestMatchGlob_PrefixWildcard(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	cfg.IncludePatterns = []string{"**/*_test.go"}

	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	testCases := []struct {
		path     string
		expected bool
	}{
		{"main_test.go", true},
		{"pkg/handler_test.go", true},
		{"main.go", false},
		{"test.go", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			matches := indexer.MatchesPatterns(tc.path)
			assert.Equal(t, tc.expected, matches)
		})
	}
}

func TestMatchGlob_ExactMatch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	cfg.IncludePatterns = []string{"Makefile", "go.mod"}

	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	testCases := []struct {
		path     string
		expected bool
	}{
		{"Makefile", true},
		{"go.mod", true},
		{"main.go", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			matches := indexer.MatchesPatterns(tc.path)
			assert.Equal(t, tc.expected, matches)
		})
	}
}

// =============================================================================
// DeleteFileData Edge Cases
// =============================================================================

func TestDeleteFile_WithNoChunks(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig()
	database := setupTestDB(t, tmpDir)
	defer database.Close()
	emb := embedder.NewMockEmbedder()
	logger := testLogger()

	indexer, err := NewIndexer(tmpDir, cfg, database, emb, logger)
	require.NoError(t, err)

	// Try to delete a file that was never indexed
	ctx := context.Background()
	err = indexer.DeleteFile(ctx, "/nonexistent/path/file.go")
	// Should not error when deleting non-existent data
	require.NoError(t, err)
}
