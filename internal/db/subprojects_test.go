package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Subproject CRUD Tests
// =============================================================================

func TestInsertSubproject(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	sp := &models.Subproject{
		ID:           "frontend",
		Path:         "packages/frontend",
		Name:         "Frontend App",
		MarkerFile:   "package.json",
		LanguageHint: "typescript",
		AutoDetected: true,
	}
	sp.SetTimestamps()

	err := db.InsertSubproject(ctx, sp)
	require.NoError(t, err)

	// Verify it was inserted
	loaded, err := db.GetSubproject(ctx, "frontend")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, sp.ID, loaded.ID)
	assert.Equal(t, sp.Path, loaded.Path)
	assert.Equal(t, sp.Name, loaded.Name)
	assert.Equal(t, sp.MarkerFile, loaded.MarkerFile)
	assert.Equal(t, sp.LanguageHint, loaded.LanguageHint)
	assert.Equal(t, sp.AutoDetected, loaded.AutoDetected)
}

func TestInsertSubproject_Upsert(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert initial
	sp := &models.Subproject{
		ID:           "backend",
		Path:         "packages/backend",
		Name:         "Backend API",
		MarkerFile:   "go.mod",
		LanguageHint: "go",
		AutoDetected: true,
	}
	sp.SetTimestamps()

	err := db.InsertSubproject(ctx, sp)
	require.NoError(t, err)

	// Update with same ID
	sp.Name = "Backend Service"
	sp.LanguageHint = "golang"
	sp.Touch()

	err = db.InsertSubproject(ctx, sp)
	require.NoError(t, err)

	// Verify update
	loaded, err := db.GetSubproject(ctx, "backend")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, "Backend Service", loaded.Name)
	assert.Equal(t, "golang", loaded.LanguageHint)
}

func TestGetSubproject_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	loaded, err := db.GetSubproject(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestListSubprojects(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert multiple subprojects
	subprojects := []*models.Subproject{
		{ID: "auth", Path: "services/auth", Name: "Auth Service"},
		{ID: "api", Path: "services/api", Name: "API Gateway"},
		{ID: "web", Path: "apps/web", Name: "Web App"},
	}

	for _, sp := range subprojects {
		sp.SetTimestamps()
		err := db.InsertSubproject(ctx, sp)
		require.NoError(t, err)
	}

	// List all
	list, err := db.ListSubprojects(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 3)

	// Should be ordered by path
	assert.Equal(t, "web", list[0].ID)  // apps/web comes first
	assert.Equal(t, "api", list[1].ID)  // services/api
	assert.Equal(t, "auth", list[2].ID) // services/auth
}

func TestListSubprojects_Empty(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	list, err := db.ListSubprojects(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestDeleteSubproject(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	sp := &models.Subproject{
		ID:   "to-delete",
		Path: "packages/to-delete",
	}
	sp.SetTimestamps()

	err := db.InsertSubproject(ctx, sp)
	require.NoError(t, err)

	// Verify it exists
	loaded, err := db.GetSubproject(ctx, "to-delete")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	// Delete it
	err = db.DeleteSubproject(ctx, "to-delete")
	require.NoError(t, err)

	// Verify it's gone
	loaded, err = db.GetSubproject(ctx, "to-delete")
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestDeleteSubproject_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Should not error when deleting non-existent
	err := db.DeleteSubproject(ctx, "nonexistent")
	require.NoError(t, err)
}

// =============================================================================
// Subproject Path Lookup Tests
// =============================================================================

func TestGetSubprojectByPath(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert subprojects with different paths
	subprojects := []*models.Subproject{
		{ID: "root", Path: "."},
		{ID: "frontend", Path: "packages/frontend"},
		{ID: "frontend-admin", Path: "packages/frontend/admin"},
		{ID: "backend", Path: "packages/backend"},
	}

	for _, sp := range subprojects {
		sp.SetTimestamps()
		err := db.InsertSubproject(ctx, sp)
		require.NoError(t, err)
	}

	tests := []struct {
		name       string
		filePath   string
		expectedID string
	}{
		{
			name:       "file in frontend admin (most specific)",
			filePath:   "packages/frontend/admin/Dashboard.tsx",
			expectedID: "frontend-admin",
		},
		{
			name:       "file in frontend but not admin",
			filePath:   "packages/frontend/App.tsx",
			expectedID: "frontend",
		},
		{
			name:       "file in backend",
			filePath:   "packages/backend/main.go",
			expectedID: "backend",
		},
		{
			name:       "file at root",
			filePath:   "README.md",
			expectedID: "root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp, err := db.GetSubprojectByPath(ctx, tt.filePath)
			require.NoError(t, err)
			require.NotNil(t, sp, "expected to find subproject for path %s", tt.filePath)
			assert.Equal(t, tt.expectedID, sp.ID)
		})
	}
}

func TestGetSubprojectByPath_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert a subproject
	sp := &models.Subproject{
		ID:   "frontend",
		Path: "packages/frontend",
	}
	sp.SetTimestamps()
	err := db.InsertSubproject(ctx, sp)
	require.NoError(t, err)

	// Look for a file outside any subproject
	found, err := db.GetSubprojectByPath(ctx, "other/path/file.go")
	require.NoError(t, err)
	assert.Nil(t, found)
}

// =============================================================================
// Subproject Count Tests
// =============================================================================

func TestSubprojectCount(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Initially zero
	count, err := db.SubprojectCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Add some
	for i := 0; i < 5; i++ {
		sp := &models.Subproject{
			ID:   fmt.Sprintf("sp-%d", i),
			Path: fmt.Sprintf("path-%d", i),
		}
		sp.SetTimestamps()
		err := db.InsertSubproject(ctx, sp)
		require.NoError(t, err)
	}

	count, err = db.SubprojectCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)
}
