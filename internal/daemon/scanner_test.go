package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupScannerTestDB(t *testing.T) *db.DB {
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

// =============================================================================
// Scanner Detection Tests
// =============================================================================

func TestScanner_DetectsNewFiles(t *testing.T) {
	database := setupScannerTestDB(t)
	ctx := context.Background()

	// Create temp project structure
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.go"), []byte("package main"), 0644))

	cfg := config.Default()
	cfg.IncludePatterns = []string{"*.go"}

	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	scanner := NewStartupScanner(tmpDir, cfg, database, ignorer)
	result, err := scanner.Scan(ctx)
	require.NoError(t, err)

	// Both files should be added
	assert.Len(t, result.Added, 2)
	assert.Empty(t, result.Modified)
	assert.Empty(t, result.Deleted)
}

func TestScanner_DetectsModifiedFiles(t *testing.T) {
	database := setupScannerTestDB(t)
	ctx := context.Background()

	// Create temp project structure
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "main.go")
	require.NoError(t, os.WriteFile(filePath, []byte("package main"), 0644))

	// Index the file with an old modification time
	oldTime := time.Now().Add(-1 * time.Hour)
	_, err := database.InsertFile(ctx, "main.go", "hash1", "go", 12, oldTime)
	require.NoError(t, err)

	cfg := config.Default()
	cfg.IncludePatterns = []string{"*.go"}

	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	scanner := NewStartupScanner(tmpDir, cfg, database, ignorer)
	result, err := scanner.Scan(ctx)
	require.NoError(t, err)

	// File should be detected as modified (current mtime > old indexed time)
	assert.Len(t, result.Modified, 1)
	assert.Equal(t, "main.go", result.Modified[0])
	assert.Empty(t, result.Added)
	assert.Empty(t, result.Deleted)
}

func TestScanner_DetectsDeletedFiles(t *testing.T) {
	database := setupScannerTestDB(t)
	ctx := context.Background()

	// Create temp project with no files
	tmpDir := t.TempDir()

	// But add a file to the index
	_, err := database.InsertFile(ctx, "deleted.go", "hash1", "go", 12, time.Now())
	require.NoError(t, err)

	cfg := config.Default()
	cfg.IncludePatterns = []string{"*.go"}

	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	scanner := NewStartupScanner(tmpDir, cfg, database, ignorer)
	result, err := scanner.Scan(ctx)
	require.NoError(t, err)

	// File should be detected as deleted
	assert.Len(t, result.Deleted, 1)
	assert.Equal(t, "deleted.go", result.Deleted[0])
	assert.Empty(t, result.Added)
	assert.Empty(t, result.Modified)
}

func TestScanner_MixedChanges(t *testing.T) {
	database := setupScannerTestDB(t)
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create files in filesystem
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "new.go"), []byte("new"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "modified.go"), []byte("modified"), 0644))

	// Index files with old times
	oldTime := time.Now().Add(-1 * time.Hour)
	_, err := database.InsertFile(ctx, "modified.go", "oldhash", "go", 8, oldTime)
	require.NoError(t, err)
	_, err = database.InsertFile(ctx, "deleted.go", "hash", "go", 8, time.Now())
	require.NoError(t, err)

	cfg := config.Default()
	cfg.IncludePatterns = []string{"*.go"}

	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	scanner := NewStartupScanner(tmpDir, cfg, database, ignorer)
	result, err := scanner.Scan(ctx)
	require.NoError(t, err)

	assert.Len(t, result.Added, 1)
	assert.Equal(t, "new.go", result.Added[0])
	assert.Len(t, result.Modified, 1)
	assert.Equal(t, "modified.go", result.Modified[0])
	assert.Len(t, result.Deleted, 1)
	assert.Equal(t, "deleted.go", result.Deleted[0])
}

// =============================================================================
// Scanner Filtering Tests
// =============================================================================

func TestScanner_RespectsIncludePatterns(t *testing.T) {
	database := setupScannerTestDB(t)
	ctx := context.Background()

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("go code"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("markdown"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte("{}"), 0644))

	cfg := config.Default()
	cfg.IncludePatterns = []string{"*.go"}

	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	scanner := NewStartupScanner(tmpDir, cfg, database, ignorer)
	result, err := scanner.Scan(ctx)
	require.NoError(t, err)

	// Only .go files should be detected
	assert.Len(t, result.Added, 1)
	assert.Equal(t, "main.go", result.Added[0])
}

func TestScanner_RespectsIgnorer(t *testing.T) {
	database := setupScannerTestDB(t)
	ctx := context.Background()

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("go code"), 0644))

	// Create vendor directory with go files
	vendorDir := filepath.Join(tmpDir, "vendor")
	require.NoError(t, os.MkdirAll(vendorDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(vendorDir, "lib.go"), []byte("vendor"), 0644))

	cfg := config.Default()
	cfg.IncludePatterns = []string{"*.go"}

	// Ignore vendor directory
	ignorer, err := NewIgnorer(tmpDir, []string{"vendor/"})
	require.NoError(t, err)

	scanner := NewStartupScanner(tmpDir, cfg, database, ignorer)
	result, err := scanner.Scan(ctx)
	require.NoError(t, err)

	// Only main.go should be detected, vendor/lib.go ignored
	assert.Len(t, result.Added, 1)
	assert.Equal(t, "main.go", result.Added[0])
}

func TestScanner_NestedDirectories(t *testing.T) {
	database := setupScannerTestDB(t)
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create nested directory structure
	srcDir := filepath.Join(tmpDir, "src", "pkg")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "handler.go"), []byte("code"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("main"), 0644))

	cfg := config.Default()
	cfg.IncludePatterns = []string{"**/*.go", "*.go"}

	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	scanner := NewStartupScanner(tmpDir, cfg, database, ignorer)
	result, err := scanner.Scan(ctx)
	require.NoError(t, err)

	// Both files should be found
	assert.Len(t, result.Added, 2)
}

func TestScanner_EmptyProject(t *testing.T) {
	database := setupScannerTestDB(t)
	ctx := context.Background()

	tmpDir := t.TempDir()

	cfg := config.Default()
	cfg.IncludePatterns = []string{"*.go"}

	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	scanner := NewStartupScanner(tmpDir, cfg, database, ignorer)
	result, err := scanner.Scan(ctx)
	require.NoError(t, err)

	assert.Empty(t, result.Added)
	assert.Empty(t, result.Modified)
	assert.Empty(t, result.Deleted)
}

func TestScanner_UnchangedFiles(t *testing.T) {
	database := setupScannerTestDB(t)
	ctx := context.Background()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "main.go")
	require.NoError(t, os.WriteFile(filePath, []byte("package main"), 0644))

	// Get file info for exact mtime
	info, err := os.Stat(filePath)
	require.NoError(t, err)

	// Index with same mtime
	_, err = database.InsertFile(ctx, "main.go", "hash", "go", 12, info.ModTime())
	require.NoError(t, err)

	cfg := config.Default()
	cfg.IncludePatterns = []string{"*.go"}

	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	scanner := NewStartupScanner(tmpDir, cfg, database, ignorer)
	result, err := scanner.Scan(ctx)
	require.NoError(t, err)

	// No changes - file exists with same mtime
	assert.Empty(t, result.Added)
	assert.Empty(t, result.Modified)
	assert.Empty(t, result.Deleted)
}
