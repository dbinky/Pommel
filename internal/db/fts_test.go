package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/models"
)

// ============================================================================
// 26.1 FTS5 Table Schema Tests
// ============================================================================

func TestCreateFTSTable_Success(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	err := db.CreateFTSTable(ctx)
	if err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	// Verify table exists
	exists, err := db.FTSTableExists(ctx)
	if err != nil {
		t.Fatalf("FTSTableExists failed: %v", err)
	}
	if !exists {
		t.Error("FTS table should exist after creation")
	}
}

func TestCreateFTSTable_AlreadyExists(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Create table first time
	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("First CreateFTSTable failed: %v", err)
	}

	// Create table second time - should not error
	err := db.CreateFTSTable(ctx)
	if err != nil {
		t.Errorf("Second CreateFTSTable should not error, got: %v", err)
	}
}

func TestFTSTableSchema_HasExpectedColumns(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	// Insert a test entry to verify columns work
	err := db.InsertFTSEntry(ctx, &models.Chunk{
		ID:       "test-chunk-1",
		Content:  "test content",
		Name:     "testFunction",
		FilePath: "/test/path.go",
	})
	if err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	// Query should work
	results, err := db.FTSSearch(ctx, "test", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestFTSTableSchema_PorterTokenizer(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	// Insert content with stemmed words
	err := db.InsertFTSEntry(ctx, &models.Chunk{
		ID:       "test-chunk-1",
		Content:  "running runners ran",
		Name:     "runTest",
		FilePath: "/test/path.go",
	})
	if err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	// Search for base form should match stemmed variations
	results, err := db.FTSSearch(ctx, "run", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("Porter stemmer should match 'run' to 'running', 'runners', 'ran'")
	}
}

func TestFTSTableSchema_UnicodeSupport(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	// Insert content with unicode
	err := db.InsertFTSEntry(ctx, &models.Chunk{
		ID:       "test-chunk-1",
		Content:  "日本語テスト café résumé",
		Name:     "unicodeFunc",
		FilePath: "/test/path.go",
	})
	if err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	// Should be searchable
	results, err := db.FTSSearch(ctx, "café", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("Unicode content should be searchable")
	}
}

// ============================================================================
// 26.2 Auto-Migration Detection Tests
// ============================================================================

func TestFTSTableExists_NonExistent(t *testing.T) {
	// Use a minimal setup without migrations to test non-existent state
	db := setupFTSTestDBNoMigration(t)
	defer db.Close()

	ctx := context.Background()

	// Table shouldn't exist yet (no migration run)
	exists, err := db.FTSTableExists(ctx)
	if err != nil {
		t.Fatalf("FTSTableExists failed: %v", err)
	}
	if exists {
		t.Error("FTS table should not exist before creation")
	}
}

func TestFTSTableExists_AfterCreation(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	exists, err := db.FTSTableExists(ctx)
	if err != nil {
		t.Fatalf("FTSTableExists failed: %v", err)
	}
	if !exists {
		t.Error("FTS table should exist after creation")
	}
}

func TestEnsureFTSTable_CreatesWhenMissing(t *testing.T) {
	// Use minimal setup to test creation from scratch
	db := setupFTSTestDBNoMigration(t)
	defer db.Close()

	ctx := context.Background()

	created, err := db.EnsureFTSTable(ctx)
	if err != nil {
		t.Fatalf("EnsureFTSTable failed: %v", err)
	}
	if !created {
		t.Error("EnsureFTSTable should return true when table is created")
	}

	// Verify table exists
	exists, err := db.FTSTableExists(ctx)
	if err != nil {
		t.Fatalf("FTSTableExists failed: %v", err)
	}
	if !exists {
		t.Error("FTS table should exist after EnsureFTSTable")
	}
}

func TestEnsureFTSTable_NoOpWhenExists(t *testing.T) {
	// Use full migration setup - FTS table already exists from v3 migration
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Table already exists from migration, Ensure should be no-op
	created, err := db.EnsureFTSTable(ctx)
	if err != nil {
		t.Fatalf("EnsureFTSTable failed: %v", err)
	}
	if created {
		t.Error("EnsureFTSTable should return false when table already exists")
	}
}

// ============================================================================
// 26.3 FTS Sync on Insert Tests
// ============================================================================

func TestInsertFTSEntry_Success(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	chunk := &models.Chunk{
		ID:       "chunk-1",
		Content:  "func main() { fmt.Println(\"Hello\") }",
		Name:     "main",
		FilePath: "/test/main.go",
	}

	err := db.InsertFTSEntry(ctx, chunk)
	if err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	// Should be searchable immediately
	results, err := db.FTSSearch(ctx, "main", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if results[0].ChunkID != "chunk-1" {
		t.Errorf("Expected chunk-1, got %s", results[0].ChunkID)
	}
}

func TestInsertFTSEntry_WithName(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	chunk := &models.Chunk{
		ID:       "chunk-1",
		Content:  "some content",
		Name:     "parseConfigFile",
		FilePath: "/test/config.go",
	}

	if err := db.InsertFTSEntry(ctx, chunk); err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	// Search by name should work
	results, err := db.FTSSearch(ctx, "parseConfigFile", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result searching by name, got %d", len(results))
	}
}

func TestInsertFTSEntry_WithoutName(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	chunk := &models.Chunk{
		ID:       "chunk-1",
		Content:  "anonymous content block",
		Name:     "", // Empty name
		FilePath: "/test/file.go",
	}

	err := db.InsertFTSEntry(ctx, chunk)
	if err != nil {
		t.Fatalf("InsertFTSEntry with empty name failed: %v", err)
	}

	// Should still be searchable by content
	results, err := db.FTSSearch(ctx, "anonymous", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestInsertFTSEntry_LongContent(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	// Create 100KB of content
	longContent := ""
	for i := 0; i < 10000; i++ {
		longContent += "word" + string(rune('A'+i%26)) + " "
	}

	chunk := &models.Chunk{
		ID:       "chunk-1",
		Content:  longContent,
		Name:     "longFunction",
		FilePath: "/test/long.go",
	}

	err := db.InsertFTSEntry(ctx, chunk)
	if err != nil {
		t.Fatalf("InsertFTSEntry with long content failed: %v", err)
	}

	results, err := db.FTSSearch(ctx, "wordA", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestInsertFTSEntry_EmptyContent(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	chunk := &models.Chunk{
		ID:       "chunk-1",
		Content:  "",
		Name:     "emptyFunc",
		FilePath: "/test/empty.go",
	}

	// Should not error
	err := db.InsertFTSEntry(ctx, chunk)
	if err != nil {
		t.Fatalf("InsertFTSEntry with empty content failed: %v", err)
	}

	// Should still be searchable by name
	results, err := db.FTSSearch(ctx, "emptyFunc", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result searching by name, got %d", len(results))
	}
}

func TestInsertFTSEntry_SpecialCharacters(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	chunk := &models.Chunk{
		ID:       "chunk-1",
		Content:  "func test() { map[string]int{} }",
		Name:     "test",
		FilePath: "/test/special.go",
	}

	err := db.InsertFTSEntry(ctx, chunk)
	if err != nil {
		t.Fatalf("InsertFTSEntry with special characters failed: %v", err)
	}

	// Should be searchable
	results, err := db.FTSSearch(ctx, "test", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestInsertFTSEntry_NilChunk(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	err := db.InsertFTSEntry(ctx, nil)
	if err == nil {
		t.Error("InsertFTSEntry with nil chunk should return error")
	}
}

func TestInsertFTSEntry_MissingChunkID(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	chunk := &models.Chunk{
		ID:       "", // Empty ID
		Content:  "some content",
		Name:     "test",
		FilePath: "/test/file.go",
	}

	err := db.InsertFTSEntry(ctx, chunk)
	if err == nil {
		t.Error("InsertFTSEntry with empty chunk ID should return error")
	}
}

func TestInsertFTSEntry_NoFTSTable(t *testing.T) {
	// Use minimal setup without migration so FTS table doesn't exist
	db := setupFTSTestDBNoMigration(t)
	defer db.Close()

	ctx := context.Background()

	// Don't create FTS table - it shouldn't exist

	chunk := &models.Chunk{
		ID:       "chunk-1",
		Content:  "some content",
		Name:     "test",
		FilePath: "/test/file.go",
	}

	err := db.InsertFTSEntry(ctx, chunk)
	if err == nil {
		t.Error("InsertFTSEntry without FTS table should return error")
	}
}

func TestInsertFTSEntry_DuplicateChunkID(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	chunk1 := &models.Chunk{
		ID:       "chunk-1",
		Content:  "original content",
		Name:     "original",
		FilePath: "/test/file.go",
	}

	chunk2 := &models.Chunk{
		ID:       "chunk-1", // Same ID
		Content:  "updated content",
		Name:     "updated",
		FilePath: "/test/file.go",
	}

	if err := db.InsertFTSEntry(ctx, chunk1); err != nil {
		t.Fatalf("First InsertFTSEntry failed: %v", err)
	}

	// Second insert should update (using INSERT OR REPLACE behavior)
	err := db.InsertFTSEntry(ctx, chunk2)
	if err != nil {
		t.Fatalf("Second InsertFTSEntry failed: %v", err)
	}

	// Should find updated content
	results, err := db.FTSSearch(ctx, "updated", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	// Should not find original content
	results, err = db.FTSSearch(ctx, "original", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for old content, got %d", len(results))
	}
}

// ============================================================================
// 26.4 FTS Sync on Update Tests
// ============================================================================

func TestUpdateFTSEntry_Success(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	// Insert first
	chunk := &models.Chunk{
		ID:       "chunk-1",
		Content:  "original function content",
		Name:     "originalFunc",
		FilePath: "/test/file.go",
	}
	if err := db.InsertFTSEntry(ctx, chunk); err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	// Update
	chunk.Content = "updated function content"
	chunk.Name = "updatedFunc"
	err := db.UpdateFTSEntry(ctx, chunk)
	if err != nil {
		t.Fatalf("UpdateFTSEntry failed: %v", err)
	}

	// Should find updated content
	results, err := db.FTSSearch(ctx, "updatedFunc", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for updated name, got %d", len(results))
	}
}

func TestUpdateFTSEntry_ContentChange(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	chunk := &models.Chunk{
		ID:       "chunk-1",
		Content:  "database connection handling",
		Name:     "connect",
		FilePath: "/test/db.go",
	}
	if err := db.InsertFTSEntry(ctx, chunk); err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	// Update content completely
	chunk.Content = "http server routing"
	if err := db.UpdateFTSEntry(ctx, chunk); err != nil {
		t.Fatalf("UpdateFTSEntry failed: %v", err)
	}

	// Old terms should not match
	results, err := db.FTSSearch(ctx, "database", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for old content, got %d", len(results))
	}

	// New terms should match
	results, err = db.FTSSearch(ctx, "server", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for new content, got %d", len(results))
	}
}

func TestUpdateFTSEntry_NonExistent(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	chunk := &models.Chunk{
		ID:       "nonexistent-chunk",
		Content:  "some content",
		Name:     "test",
		FilePath: "/test/file.go",
	}

	err := db.UpdateFTSEntry(ctx, chunk)
	if err == nil {
		t.Error("UpdateFTSEntry for non-existent chunk should return error")
	}
}

func TestUpdateFTSEntry_NilChunk(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	err := db.UpdateFTSEntry(ctx, nil)
	if err == nil {
		t.Error("UpdateFTSEntry with nil chunk should return error")
	}
}

// ============================================================================
// 26.5 FTS Sync on Delete Tests
// ============================================================================

func TestDeleteFTSEntry_Success(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	chunk := &models.Chunk{
		ID:       "chunk-1",
		Content:  "deletable content",
		Name:     "deleteMe",
		FilePath: "/test/file.go",
	}
	if err := db.InsertFTSEntry(ctx, chunk); err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	// Verify it's searchable
	results, err := db.FTSSearch(ctx, "deletable", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result before delete, got %d", len(results))
	}

	// Delete
	err = db.DeleteFTSEntry(ctx, "chunk-1")
	if err != nil {
		t.Fatalf("DeleteFTSEntry failed: %v", err)
	}

	// Should no longer be searchable
	results, err = db.FTSSearch(ctx, "deletable", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results after delete, got %d", len(results))
	}
}

func TestDeleteFTSEntry_NonExistent(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	// Delete non-existent should not error
	err := db.DeleteFTSEntry(ctx, "nonexistent-chunk")
	if err != nil {
		t.Errorf("DeleteFTSEntry for non-existent chunk should not error, got: %v", err)
	}
}

func TestDeleteFTSEntry_EmptyChunkID(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	err := db.DeleteFTSEntry(ctx, "")
	if err == nil {
		t.Error("DeleteFTSEntry with empty chunk ID should return error")
	}
}

func TestDeleteFTSEntriesByFile_Success(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	// Insert multiple chunks from same file
	chunks := []*models.Chunk{
		{ID: "chunk-1", Content: "first chunk", Name: "func1", FilePath: "/test/file.go"},
		{ID: "chunk-2", Content: "second chunk", Name: "func2", FilePath: "/test/file.go"},
		{ID: "chunk-3", Content: "other file chunk", Name: "func3", FilePath: "/test/other.go"},
	}
	for _, c := range chunks {
		if err := db.InsertFTSEntry(ctx, c); err != nil {
			t.Fatalf("InsertFTSEntry failed: %v", err)
		}
	}

	// Delete all chunks from /test/file.go
	err := db.DeleteFTSEntriesByFile(ctx, "/test/file.go")
	if err != nil {
		t.Fatalf("DeleteFTSEntriesByFile failed: %v", err)
	}

	// Chunks from file.go should be gone
	results, err := db.FTSSearch(ctx, "first", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results from deleted file, got %d", len(results))
	}

	// Chunks from other.go should remain
	results, err = db.FTSSearch(ctx, "other", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result from other file, got %d", len(results))
	}
}

// ============================================================================
// 26.6 FTS Query Function Tests
// ============================================================================

func TestFTSSearch_SingleTerm(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	chunks := []*models.Chunk{
		{ID: "chunk-1", Content: "database connection pooling", Name: "pool", FilePath: "/db/pool.go"},
		{ID: "chunk-2", Content: "http server routing", Name: "route", FilePath: "/http/router.go"},
		{ID: "chunk-3", Content: "database query builder", Name: "query", FilePath: "/db/query.go"},
	}
	for _, c := range chunks {
		if err := db.InsertFTSEntry(ctx, c); err != nil {
			t.Fatalf("InsertFTSEntry failed: %v", err)
		}
	}

	results, err := db.FTSSearch(ctx, "database", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'database', got %d", len(results))
	}
}

func TestFTSSearch_MultipleTerms(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	chunks := []*models.Chunk{
		{ID: "chunk-1", Content: "database connection pooling", Name: "pool", FilePath: "/db/pool.go"},
		{ID: "chunk-2", Content: "http server connection", Name: "server", FilePath: "/http/server.go"},
		{ID: "chunk-3", Content: "file reader", Name: "read", FilePath: "/io/reader.go"},
	}
	for _, c := range chunks {
		if err := db.InsertFTSEntry(ctx, c); err != nil {
			t.Fatalf("InsertFTSEntry failed: %v", err)
		}
	}

	// Search for both terms - should match chunk with both
	results, err := db.FTSSearch(ctx, "database connection", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	// FTS5 with multiple terms will match documents containing both
	if len(results) == 0 {
		t.Error("Expected at least 1 result for 'database connection'")
	}
}

func TestFTSSearch_NoResults(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	if err := db.InsertFTSEntry(ctx, &models.Chunk{
		ID: "chunk-1", Content: "hello world", Name: "hello", FilePath: "/test.go",
	}); err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	results, err := db.FTSSearch(ctx, "nonexistent", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestFTSSearch_LimitRespected(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	// Insert 10 chunks with same term
	for i := 0; i < 10; i++ {
		chunk := &models.Chunk{
			ID:       fmt.Sprintf("chunk-%d", i),
			Content:  "common term here",
			Name:     fmt.Sprintf("func%d", i),
			FilePath: fmt.Sprintf("/test%d.go", i),
		}
		if err := db.InsertFTSEntry(ctx, chunk); err != nil {
			t.Fatalf("InsertFTSEntry failed: %v", err)
		}
	}

	results, err := db.FTSSearch(ctx, "common", 5)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Expected 5 results (limited), got %d", len(results))
	}
}

func TestFTSSearch_OrderedByRelevance(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	chunks := []*models.Chunk{
		{ID: "chunk-1", Content: "single mention of database", Name: "func1", FilePath: "/a.go"},
		{ID: "chunk-2", Content: "database database database many mentions", Name: "database", FilePath: "/b.go"},
	}
	for _, c := range chunks {
		if err := db.InsertFTSEntry(ctx, c); err != nil {
			t.Fatalf("InsertFTSEntry failed: %v", err)
		}
	}

	results, err := db.FTSSearch(ctx, "database", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// chunk-2 should have better score (more mentions)
	if results[0].ChunkID != "chunk-2" {
		t.Errorf("Expected chunk-2 (more mentions) to be first, got %s", results[0].ChunkID)
	}
}

func TestFTSSearch_CaseInsensitive(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	if err := db.InsertFTSEntry(ctx, &models.Chunk{
		ID: "chunk-1", Content: "Database Connection", Name: "Connect", FilePath: "/test.go",
	}); err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	// Search with different case
	results, err := db.FTSSearch(ctx, "database", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result (case insensitive), got %d", len(results))
	}
}

func TestFTSSearch_PrefixMatch(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	if err := db.InsertFTSEntry(ctx, &models.Chunk{
		ID: "chunk-1", Content: "database connection", Name: "connect", FilePath: "/test.go",
	}); err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	// Prefix search with *
	results, err := db.FTSSearch(ctx, "data*", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for prefix search, got %d", len(results))
	}
}

func TestFTSSearch_EmptyQuery(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	results, err := db.FTSSearch(ctx, "", 10)
	if err != nil {
		t.Fatalf("FTSSearch with empty query should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty query, got %d", len(results))
	}
}

func TestFTSSearch_SpecialCharacters(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	if err := db.InsertFTSEntry(ctx, &models.Chunk{
		ID: "chunk-1", Content: "map[string]int{}", Name: "test", FilePath: "/test.go",
	}); err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	// Query with special chars should be handled safely
	results, err := db.FTSSearch(ctx, "map[string]", 10)
	// This might not find results due to tokenization, but shouldn't error
	if err != nil {
		t.Fatalf("FTSSearch with special chars should not error: %v", err)
	}
	_ = results // Result count depends on tokenization
}

func TestFTSSearch_ZeroLimit(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	if err := db.InsertFTSEntry(ctx, &models.Chunk{
		ID: "chunk-1", Content: "test content", Name: "test", FilePath: "/test.go",
	}); err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	results, err := db.FTSSearch(ctx, "test", 0)
	if err != nil {
		t.Fatalf("FTSSearch with zero limit should not error: %v", err)
	}
	// With zero limit, should return empty or use default
	if len(results) > 0 {
		t.Logf("Zero limit returned %d results (implementation dependent)", len(results))
	}
}

func TestFTSSearch_NegativeLimit(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	if err := db.InsertFTSEntry(ctx, &models.Chunk{
		ID: "chunk-1", Content: "test content", Name: "test", FilePath: "/test.go",
	}); err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	// Negative limit should be handled (either error or use default)
	results, err := db.FTSSearch(ctx, "test", -1)
	if err != nil {
		// Acceptable to error
		t.Logf("Negative limit returned error (acceptable): %v", err)
	} else {
		// Or return empty/default
		t.Logf("Negative limit returned %d results", len(results))
	}
}

func TestFTSSearch_ContextCancellation(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	if err := db.CreateFTSTable(context.Background()); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := db.FTSSearch(ctx, "test", 10)
	if err == nil {
		t.Error("FTSSearch with cancelled context should return error")
	}
}

// ============================================================================
// 26.7 Bulk FTS Population Tests
// ============================================================================

func TestPopulateFTS_EmptyChunks(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	count, err := db.PopulateFTSFromChunks(ctx)
	if err != nil {
		t.Fatalf("PopulateFTSFromChunks failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 populated, got %d", count)
	}
}

func TestPopulateFTS_WithChunks(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Run migrations to create chunks table
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	// Insert file and chunks into main tables
	fileID, err := db.InsertFile(ctx, "/test/file.go", "hash1", "go", 100, time.Now())
	if err != nil {
		t.Fatalf("InsertFile failed: %v", err)
	}

	chunks := []*models.Chunk{
		{ID: "chunk-1", Content: "first chunk content", Name: "func1", Level: "function"},
		{ID: "chunk-2", Content: "second chunk content", Name: "func2", Level: "function"},
	}
	for _, c := range chunks {
		if err := db.InsertChunk(ctx, c, fileID); err != nil {
			t.Fatalf("InsertChunk failed: %v", err)
		}
	}

	// Populate FTS from chunks
	count, err := db.PopulateFTSFromChunks(ctx)
	if err != nil {
		t.Fatalf("PopulateFTSFromChunks failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 populated, got %d", count)
	}

	// Verify searchable
	results, err := db.FTSSearch(ctx, "chunk", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 searchable results, got %d", len(results))
	}
}

func TestPopulateFTS_ClearsExisting(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	ctx := context.Background()

	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	if err := db.CreateFTSTable(ctx); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	// Insert old FTS entry directly
	if err := db.InsertFTSEntry(ctx, &models.Chunk{
		ID: "old-chunk", Content: "old orphan content", Name: "old", FilePath: "/old.go",
	}); err != nil {
		t.Fatalf("InsertFTSEntry failed: %v", err)
	}

	// Insert real chunk
	fileID, err := db.InsertFile(ctx, "/test/file.go", "hash1", "go", 100, time.Now())
	if err != nil {
		t.Fatalf("InsertFile failed: %v", err)
	}
	if err := db.InsertChunk(ctx, &models.Chunk{
		ID: "real-chunk", Content: "real content", Name: "real", Level: "function",
	}, fileID); err != nil {
		t.Fatalf("InsertChunk failed: %v", err)
	}

	// Populate should clear old and add new
	count, err := db.PopulateFTSFromChunks(ctx)
	if err != nil {
		t.Fatalf("PopulateFTSFromChunks failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 populated, got %d", count)
	}

	// Old should be gone
	results, err := db.FTSSearch(ctx, "orphan", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for old content, got %d", len(results))
	}

	// New should exist
	results, err = db.FTSSearch(ctx, "real", 10)
	if err != nil {
		t.Fatalf("FTSSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for new content, got %d", len(results))
	}
}

func TestPopulateFTS_ContextCancellation(t *testing.T) {
	db := setupFTSTestDB(t)
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	if err := db.CreateFTSTable(context.Background()); err != nil {
		t.Fatalf("CreateFTSTable failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := db.PopulateFTSFromChunks(ctx)
	if err == nil {
		t.Error("PopulateFTSFromChunks with cancelled context should return error")
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func setupFTSTestDB(t *testing.T) *DB {
	t.Helper()

	// Create temp directory for test database
	tempDir, err := os.MkdirTemp("", "pommel-fts-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create .pommel directory
	pommelDir := filepath.Join(tempDir, ".pommel")
	if err := os.MkdirAll(pommelDir, 0755); err != nil {
		t.Fatalf("Failed to create .pommel dir: %v", err)
	}

	db, err := Open(tempDir, EmbeddingDimension)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Run base migrations (includes v3 which creates FTS table)
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(tempDir)
	})

	return db
}

// setupFTSTestDBNoMigration creates a test DB without running migrations
// Used for testing FTS table creation and existence checks
func setupFTSTestDBNoMigration(t *testing.T) *DB {
	t.Helper()

	// Create temp directory for test database
	tempDir, err := os.MkdirTemp("", "pommel-fts-test-nomigration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create .pommel directory
	pommelDir := filepath.Join(tempDir, ".pommel")
	if err := os.MkdirAll(pommelDir, 0755); err != nil {
		t.Fatalf("Failed to create .pommel dir: %v", err)
	}

	db, err := Open(tempDir, EmbeddingDimension)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Do NOT run migrations - we want a clean DB to test FTS table creation

	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(tempDir)
	})

	return db
}
