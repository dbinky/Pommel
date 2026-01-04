package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDB_SetMetadata(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	err := db.SetMetadata(ctx, "provider", "openai")
	require.NoError(t, err)

	value, err := db.GetMetadata(ctx, "provider")
	require.NoError(t, err)
	assert.Equal(t, "openai", value)
}

func TestDB_GetMetadata_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	value, err := db.GetMetadata(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, value)
}

func TestDB_SetMetadata_Update(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	require.NoError(t, db.SetMetadata(ctx, "provider", "ollama"))
	require.NoError(t, db.SetMetadata(ctx, "provider", "openai"))

	value, err := db.GetMetadata(ctx, "provider")
	require.NoError(t, err)
	assert.Equal(t, "openai", value)
}

func TestDB_DeleteMetadata(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	require.NoError(t, db.SetMetadata(ctx, "provider", "openai"))
	require.NoError(t, db.DeleteMetadata(ctx, "provider"))

	value, err := db.GetMetadata(ctx, "provider")
	require.NoError(t, err)
	assert.Empty(t, value)
}

func TestDB_GetProviderInfo(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	require.NoError(t, db.SetMetadata(ctx, "provider", "openai"))
	require.NoError(t, db.SetMetadata(ctx, "model", "text-embedding-3-small"))
	require.NoError(t, db.SetMetadata(ctx, "dimensions", "1536"))

	info, err := db.GetProviderInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, "openai", info.Provider)
	assert.Equal(t, "text-embedding-3-small", info.Model)
	assert.Equal(t, 1536, info.Dimensions)
}

func TestDB_GetProviderInfo_NotSet(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	info, err := db.GetProviderInfo(ctx)
	require.NoError(t, err)
	assert.Empty(t, info.Provider)
	assert.Empty(t, info.Model)
	assert.Zero(t, info.Dimensions)
}

func TestDB_SetProviderInfo(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	info := ProviderInfo{
		Provider:   "voyage",
		Model:      "voyage-code-3",
		Dimensions: 1024,
	}
	require.NoError(t, db.SetProviderInfo(ctx, info))

	retrieved, err := db.GetProviderInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, info.Provider, retrieved.Provider)
	assert.Equal(t, info.Model, retrieved.Model)
	assert.Equal(t, info.Dimensions, retrieved.Dimensions)
}

func TestDB_ProviderChanged(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// No provider set yet - should return false
	changed, err := db.ProviderChanged(ctx, "openai", "text-embedding-3-small")
	require.NoError(t, err)
	assert.False(t, changed)

	// Set provider
	require.NoError(t, db.SetMetadata(ctx, "provider", "openai"))
	require.NoError(t, db.SetMetadata(ctx, "model", "text-embedding-3-small"))

	// Same provider - should return false
	changed, err = db.ProviderChanged(ctx, "openai", "text-embedding-3-small")
	require.NoError(t, err)
	assert.False(t, changed)

	// Different provider - should return true
	changed, err = db.ProviderChanged(ctx, "voyage", "voyage-code-3")
	require.NoError(t, err)
	assert.True(t, changed)

	// Same provider, different model - should return true
	changed, err = db.ProviderChanged(ctx, "openai", "text-embedding-3-large")
	require.NoError(t, err)
	assert.True(t, changed)
}
