package subproject

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *db.DB {
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
// Manager Sync Tests
// =============================================================================

func TestManager_SyncSubprojects_Add(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// Create temp project structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "services", "api")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module api"), 0644))

	cfg := &config.SubprojectsConfig{
		AutoDetect: true,
	}

	manager := NewManager(database, tmpDir, cfg)
	added, removed, unchanged, err := manager.SyncSubprojects(ctx)
	require.NoError(t, err)

	assert.Equal(t, 1, added)
	assert.Equal(t, 0, removed)
	assert.Equal(t, 0, unchanged)

	// Verify persisted
	subprojects, err := database.ListSubprojects(ctx)
	require.NoError(t, err)
	assert.Len(t, subprojects, 1)
	assert.Equal(t, filepath.Join("services", "api"), subprojects[0].Path)
}

func TestManager_SyncSubprojects_Remove(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// Insert an auto-detected subproject
	sp := &models.Subproject{
		ID:           "old-project",
		Path:         "old/project",
		AutoDetected: true,
	}
	sp.SetTimestamps()
	require.NoError(t, database.InsertSubproject(ctx, sp))

	// Create a new project structure (without the old one)
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "new", "project")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module new"), 0644))

	cfg := &config.SubprojectsConfig{
		AutoDetect: true,
	}

	manager := NewManager(database, tmpDir, cfg)
	added, removed, unchanged, err := manager.SyncSubprojects(ctx)
	require.NoError(t, err)

	assert.Equal(t, 1, added)
	assert.Equal(t, 1, removed)
	assert.Equal(t, 0, unchanged)

	// Verify old is gone, new is present
	subprojects, err := database.ListSubprojects(ctx)
	require.NoError(t, err)
	assert.Len(t, subprojects, 1)
	assert.Equal(t, filepath.Join("new", "project"), subprojects[0].Path)
}

func TestManager_SyncSubprojects_Unchanged(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// Create temp project structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module backend"), 0644))

	cfg := &config.SubprojectsConfig{
		AutoDetect: true,
	}

	manager := NewManager(database, tmpDir, cfg)

	// First sync
	added, _, _, err := manager.SyncSubprojects(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, added)

	// Second sync - should be unchanged
	added, removed, unchanged, err := manager.SyncSubprojects(ctx)
	require.NoError(t, err)

	assert.Equal(t, 0, added)
	assert.Equal(t, 0, removed)
	assert.Equal(t, 1, unchanged)
}

func TestManager_SyncSubprojects_ManualNotRemoved(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// Insert a manually configured subproject
	sp := &models.Subproject{
		ID:           "manual-project",
		Path:         "manual/project",
		AutoDetected: false, // Manual
	}
	sp.SetTimestamps()
	require.NoError(t, database.InsertSubproject(ctx, sp))

	// Create empty project structure (no markers)
	tmpDir := t.TempDir()

	cfg := &config.SubprojectsConfig{
		AutoDetect: true,
	}

	manager := NewManager(database, tmpDir, cfg)
	_, removed, _, err := manager.SyncSubprojects(ctx)
	require.NoError(t, err)

	// Manual project should NOT be removed
	assert.Equal(t, 0, removed)

	subprojects, err := database.ListSubprojects(ctx)
	require.NoError(t, err)
	assert.Len(t, subprojects, 1)
	assert.Equal(t, "manual-project", subprojects[0].ID)
}

func TestManager_SyncSubprojects_AutoDetectDisabled(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// Create temp project structure with markers
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module backend"), 0644))

	cfg := &config.SubprojectsConfig{
		AutoDetect: false, // Disabled
	}

	manager := NewManager(database, tmpDir, cfg)
	added, _, _, err := manager.SyncSubprojects(ctx)
	require.NoError(t, err)

	// Nothing should be detected
	assert.Equal(t, 0, added)

	subprojects, err := database.ListSubprojects(ctx)
	require.NoError(t, err)
	assert.Empty(t, subprojects)
}

// =============================================================================
// Config Override Tests
// =============================================================================

func TestManager_MergeWithConfig_Override(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// Create temp project structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "packages", "frontend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "package.json"), []byte("{}"), 0644))

	cfg := &config.SubprojectsConfig{
		AutoDetect: true,
		Projects: []config.ProjectOverride{
			{
				ID:   "custom-frontend",
				Path: filepath.Join("packages", "frontend"),
				Name: "My Frontend App",
			},
		},
	}

	manager := NewManager(database, tmpDir, cfg)
	_, _, _, err := manager.SyncSubprojects(ctx)
	require.NoError(t, err)

	// Verify custom ID and name were applied
	subprojects, err := database.ListSubprojects(ctx)
	require.NoError(t, err)
	assert.Len(t, subprojects, 1)
	assert.Equal(t, "custom-frontend", subprojects[0].ID)
	assert.Equal(t, "My Frontend App", subprojects[0].Name)
	assert.False(t, subprojects[0].AutoDetected)
}

func TestManager_MergeWithConfig_ManualOnly(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// No actual project structure, just config
	tmpDir := t.TempDir()

	cfg := &config.SubprojectsConfig{
		AutoDetect: false,
		Projects: []config.ProjectOverride{
			{
				ID:   "manual-api",
				Path: "services/api",
				Name: "API Service",
			},
		},
	}

	manager := NewManager(database, tmpDir, cfg)
	added, _, _, err := manager.SyncSubprojects(ctx)
	require.NoError(t, err)

	assert.Equal(t, 1, added)

	subprojects, err := database.ListSubprojects(ctx)
	require.NoError(t, err)
	assert.Len(t, subprojects, 1)
	assert.Equal(t, "manual-api", subprojects[0].ID)
}

// =============================================================================
// Chunk Assignment Tests
// =============================================================================

func TestManager_AssignSubprojectToChunk(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// Create a subproject in the database
	sp := &models.Subproject{
		ID:   "frontend",
		Path: "packages/frontend",
	}
	sp.SetTimestamps()
	require.NoError(t, database.InsertSubproject(ctx, sp))

	// Create manager
	manager := NewManager(database, ".", &config.SubprojectsConfig{})

	// Create a chunk
	chunk := &models.Chunk{
		FilePath:  "packages/frontend/src/App.tsx",
		StartLine: 1,
		EndLine:   10,
		Level:     models.ChunkLevelFile,
		Content:   "test",
	}

	// Assign subproject
	err := manager.AssignSubprojectToChunk(ctx, chunk)
	require.NoError(t, err)

	assert.NotNil(t, chunk.SubprojectID)
	assert.Equal(t, "frontend", *chunk.SubprojectID)
	assert.NotNil(t, chunk.SubprojectPath)
	assert.Equal(t, "packages/frontend", *chunk.SubprojectPath)
}

func TestManager_AssignSubprojectToChunk_NoMatch(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// Create a subproject
	sp := &models.Subproject{
		ID:   "frontend",
		Path: "packages/frontend",
	}
	sp.SetTimestamps()
	require.NoError(t, database.InsertSubproject(ctx, sp))

	manager := NewManager(database, ".", &config.SubprojectsConfig{})

	// Create a chunk outside any subproject
	chunk := &models.Chunk{
		FilePath:  "other/path/file.go",
		StartLine: 1,
		EndLine:   10,
		Level:     models.ChunkLevelFile,
		Content:   "test",
	}

	err := manager.AssignSubprojectToChunk(ctx, chunk)
	require.NoError(t, err)

	// Should remain nil
	assert.Nil(t, chunk.SubprojectID)
	assert.Nil(t, chunk.SubprojectPath)
}

func TestManager_GetSubprojectForPath(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// Create subprojects
	subprojects := []*models.Subproject{
		{ID: "frontend", Path: "packages/frontend"},
		{ID: "backend", Path: "packages/backend"},
	}
	for _, sp := range subprojects {
		sp.SetTimestamps()
		require.NoError(t, database.InsertSubproject(ctx, sp))
	}

	manager := NewManager(database, ".", &config.SubprojectsConfig{})

	// Test frontend file
	sp, err := manager.GetSubprojectForPath(ctx, "packages/frontend/src/App.tsx")
	require.NoError(t, err)
	require.NotNil(t, sp)
	assert.Equal(t, "frontend", sp.ID)

	// Test backend file
	sp, err = manager.GetSubprojectForPath(ctx, "packages/backend/main.go")
	require.NoError(t, err)
	require.NotNil(t, sp)
	assert.Equal(t, "backend", sp.ID)

	// Test no match
	sp, err = manager.GetSubprojectForPath(ctx, "other/file.go")
	require.NoError(t, err)
	assert.Nil(t, sp)
}
