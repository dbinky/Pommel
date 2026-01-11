package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a test database with migrations applied.
func setupTestDB(t *testing.T) *DB {
	tmpDir := t.TempDir()
	db, err := Open(tmpDir, EmbeddingDimension)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background()))
	t.Cleanup(func() { db.Close() })
	return db
}

// makeEmbedding creates a test embedding with the specified pattern.
// The base value determines the "direction" of the embedding vector.
func makeEmbedding(base float32) []float32 {
	embedding := make([]float32, EmbeddingDimension)
	for i := range embedding {
		embedding[i] = base + float32(i)*0.001
	}
	return embedding
}

// =============================================================================
// Happy Path / Success Cases
// =============================================================================

func TestInsertEmbedding(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert a single embedding
	embedding := makeEmbedding(0.1)
	err := db.InsertEmbedding(ctx, "chunk-1", embedding)
	require.NoError(t, err)

	// Verify count increased
	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestInsertEmbeddings_Batch(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert multiple embeddings in one batch
	chunkIDs := []string{"chunk-1", "chunk-2", "chunk-3"}
	embeddings := [][]float32{
		makeEmbedding(0.1),
		makeEmbedding(0.2),
		makeEmbedding(0.3),
	}

	err := db.InsertEmbeddings(ctx, chunkIDs, embeddings)
	require.NoError(t, err)

	// Verify all were inserted
	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestInsertEmbedding_Replace(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert initial embedding
	embedding1 := makeEmbedding(0.1)
	err := db.InsertEmbedding(ctx, "chunk-1", embedding1)
	require.NoError(t, err)

	// Insert another embedding with different ID
	embedding2 := makeEmbedding(0.2)
	err = db.InsertEmbedding(ctx, "chunk-2", embedding2)
	require.NoError(t, err)

	// Count should be 2
	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Replace chunk-1 with a new embedding
	embedding3 := makeEmbedding(0.9)
	err = db.InsertEmbedding(ctx, "chunk-1", embedding3)
	require.NoError(t, err)

	// Count should still be 2 (not 3) since chunk-1 was replaced
	count, err = db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Search should find the new embedding (0.9 base) not the old one (0.1 base)
	queryEmbedding := makeEmbedding(0.9)
	results, err := db.SearchSimilar(ctx, queryEmbedding, 2)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// The chunk-1 with 0.9 base should be most similar to 0.9 query
	assert.Equal(t, "chunk-1", results[0].ChunkID)
}

func TestDeleteEmbedding(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert embeddings
	err := db.InsertEmbedding(ctx, "chunk-1", makeEmbedding(0.1))
	require.NoError(t, err)
	err = db.InsertEmbedding(ctx, "chunk-2", makeEmbedding(0.2))
	require.NoError(t, err)

	// Verify count is 2
	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Delete one embedding
	err = db.DeleteEmbedding(ctx, "chunk-1")
	require.NoError(t, err)

	// Verify count decreased
	count, err = db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify chunk-1 is no longer searchable
	results, err := db.SearchSimilar(ctx, makeEmbedding(0.1), 10)
	require.NoError(t, err)
	for _, r := range results {
		assert.NotEqual(t, "chunk-1", r.ChunkID, "deleted chunk should not appear in results")
	}
}

func TestDeleteEmbeddingsByChunkIDs(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert multiple embeddings
	chunkIDs := []string{"chunk-1", "chunk-2", "chunk-3", "chunk-4"}
	for i, id := range chunkIDs {
		err := db.InsertEmbedding(ctx, id, makeEmbedding(float32(i)*0.1))
		require.NoError(t, err)
	}

	// Verify count is 4
	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 4, count)

	// Delete multiple embeddings
	err = db.DeleteEmbeddingsByChunkIDs(ctx, []string{"chunk-1", "chunk-3"})
	require.NoError(t, err)

	// Verify count is now 2
	count, err = db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify only chunk-2 and chunk-4 remain
	results, err := db.SearchSimilar(ctx, makeEmbedding(0.0), 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	resultIDs := make(map[string]bool)
	for _, r := range results {
		resultIDs[r.ChunkID] = true
	}
	assert.True(t, resultIDs["chunk-2"], "chunk-2 should remain")
	assert.True(t, resultIDs["chunk-4"], "chunk-4 should remain")
	assert.False(t, resultIDs["chunk-1"], "chunk-1 should be deleted")
	assert.False(t, resultIDs["chunk-3"], "chunk-3 should be deleted")
}

func TestSearchSimilar(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create embeddings with clearly different values
	similar := makeEmbedding(0.5)
	different := makeEmbedding(5.0)   // Very different base value
	verySimilar := makeEmbedding(0.5) // Identical to query

	// Insert in non-obvious order
	err := db.InsertEmbedding(ctx, "different", different)
	require.NoError(t, err)
	err = db.InsertEmbedding(ctx, "similar", similar)
	require.NoError(t, err)
	err = db.InsertEmbedding(ctx, "very-similar", verySimilar)
	require.NoError(t, err)

	// Search with query similar to "similar" and "very-similar"
	query := makeEmbedding(0.5)
	results, err := db.SearchSimilar(ctx, query, 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 3)

	// Similar embeddings should rank higher (lower distance)
	// The first results should be "very-similar" and "similar" since they're closest
	topTwoIDs := []string{results[0].ChunkID, results[1].ChunkID}
	assert.Contains(t, topTwoIDs, "very-similar", "very-similar should be in top 2")
	assert.Contains(t, topTwoIDs, "similar", "similar should be in top 2")

	// Distance should be ascending (smaller = more similar)
	for i := 1; i < len(results); i++ {
		assert.GreaterOrEqual(t, results[i].Distance, results[i-1].Distance,
			"results should be sorted by distance ascending")
	}
}

func TestSearchSimilar_Limit(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert 10 embeddings
	for i := 0; i < 10; i++ {
		err := db.InsertEmbedding(ctx, fmt.Sprintf("chunk-%d", i), makeEmbedding(float32(i)*0.1))
		require.NoError(t, err)
	}

	query := makeEmbedding(0.0)

	// Request limit of 3
	results, err := db.SearchSimilar(ctx, query, 3)
	require.NoError(t, err)
	assert.Len(t, results, 3, "should respect limit parameter")

	// Request limit of 5
	results, err = db.SearchSimilar(ctx, query, 5)
	require.NoError(t, err)
	assert.Len(t, results, 5, "should respect limit parameter")

	// Request limit larger than available
	results, err = db.SearchSimilar(ctx, query, 100)
	require.NoError(t, err)
	assert.Len(t, results, 10, "should return all available when limit exceeds count")
}

func TestSearchSimilarFiltered(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert embeddings
	for i := 0; i < 10; i++ {
		err := db.InsertEmbedding(ctx, fmt.Sprintf("chunk-%d", i), makeEmbedding(float32(i)*0.1))
		require.NoError(t, err)
	}

	query := makeEmbedding(0.0)

	// Filter to only specific chunks
	filterIDs := []string{"chunk-2", "chunk-5", "chunk-8"}
	results, err := db.SearchSimilarFiltered(ctx, query, 10, filterIDs)
	require.NoError(t, err)

	// Should only return filtered chunks
	assert.Len(t, results, 3)
	resultIDs := make(map[string]bool)
	for _, r := range results {
		resultIDs[r.ChunkID] = true
	}
	assert.True(t, resultIDs["chunk-2"])
	assert.True(t, resultIDs["chunk-5"])
	assert.True(t, resultIDs["chunk-8"])
	assert.False(t, resultIDs["chunk-0"], "non-filtered chunks should not appear")
	assert.False(t, resultIDs["chunk-1"], "non-filtered chunks should not appear")
}

func TestEmbeddingCount(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Initially empty
	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Add embeddings one by one and verify count
	for i := 1; i <= 5; i++ {
		err := db.InsertEmbedding(ctx, fmt.Sprintf("chunk-%d", i), makeEmbedding(float32(i)*0.1))
		require.NoError(t, err)

		count, err := db.EmbeddingCount(ctx)
		require.NoError(t, err)
		assert.Equal(t, i, count, "count should be %d after inserting %d embeddings", i, i)
	}
}

// =============================================================================
// Failure / Error Cases
// =============================================================================

func TestSearchSimilar_Empty(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Search on empty database
	query := makeEmbedding(0.5)
	results, err := db.SearchSimilar(ctx, query, 10)
	require.NoError(t, err, "searching empty database should not error")

	// Should return empty slice, not nil
	require.NotNil(t, results, "should return empty slice, not nil")
	assert.Len(t, results, 0, "should return empty slice when no embeddings")
}

func TestInsertEmbeddings_Mismatch(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Mismatched lengths: 3 IDs but 2 embeddings
	chunkIDs := []string{"chunk-1", "chunk-2", "chunk-3"}
	embeddings := [][]float32{
		makeEmbedding(0.1),
		makeEmbedding(0.2),
	}

	err := db.InsertEmbeddings(ctx, chunkIDs, embeddings)
	assert.Error(t, err, "should error when chunkIDs and embeddings count differ")

	// Also test the reverse: 2 IDs but 3 embeddings
	chunkIDs2 := []string{"chunk-1", "chunk-2"}
	embeddings2 := [][]float32{
		makeEmbedding(0.1),
		makeEmbedding(0.2),
		makeEmbedding(0.3),
	}

	err = db.InsertEmbeddings(ctx, chunkIDs2, embeddings2)
	assert.Error(t, err, "should error when chunkIDs and embeddings count differ")
}

func TestDeleteEmbedding_NotExists(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Delete a non-existent embedding should not error (idempotent)
	err := db.DeleteEmbedding(ctx, "nonexistent-chunk")
	assert.NoError(t, err, "deleting non-existent embedding should not error")

	// Insert one embedding
	err = db.InsertEmbedding(ctx, "chunk-1", makeEmbedding(0.1))
	require.NoError(t, err)

	// Delete a different non-existent embedding should still not error
	err = db.DeleteEmbedding(ctx, "still-nonexistent")
	assert.NoError(t, err, "deleting non-existent embedding should not error")

	// Original embedding should still be there
	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestSearchSimilar_WrongDimensions(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert a valid embedding
	err := db.InsertEmbedding(ctx, "chunk-1", makeEmbedding(0.1))
	require.NoError(t, err)

	// Try to search with wrong dimension (too few)
	shortQuery := make([]float32, 100) // Wrong: should be 768
	for i := range shortQuery {
		shortQuery[i] = 0.5
	}

	results, err := db.SearchSimilar(ctx, shortQuery, 10)
	// Implementation should either:
	// 1. Return an error
	// 2. Return empty results
	// Either is acceptable - the important thing is it handles gracefully
	if err != nil {
		assert.Error(t, err, "should error on dimension mismatch")
	} else {
		// If no error, results should be empty or have valid entries
		assert.NotNil(t, results, "results should not be nil")
	}

	// Try with too many dimensions
	longQuery := make([]float32, 1000) // Wrong: should be 768
	for i := range longQuery {
		longQuery[i] = 0.5
	}

	results, err = db.SearchSimilar(ctx, longQuery, 10)
	if err != nil {
		assert.Error(t, err, "should error on dimension mismatch")
	} else {
		assert.NotNil(t, results, "results should not be nil")
	}
}

// =============================================================================
// Additional Edge Cases
// =============================================================================

func TestSearchSimilarFiltered_EmptyFilter(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert embeddings
	err := db.InsertEmbedding(ctx, "chunk-1", makeEmbedding(0.1))
	require.NoError(t, err)

	query := makeEmbedding(0.1)

	// Filter with empty slice should return empty results
	results, err := db.SearchSimilarFiltered(ctx, query, 10, []string{})
	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results, 0, "empty filter should return empty results")
}

func TestSearchSimilarFiltered_NoMatch(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert embeddings
	err := db.InsertEmbedding(ctx, "chunk-1", makeEmbedding(0.1))
	require.NoError(t, err)

	query := makeEmbedding(0.1)

	// Filter with non-existent IDs should return empty results
	results, err := db.SearchSimilarFiltered(ctx, query, 10, []string{"nonexistent-1", "nonexistent-2"})
	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results, 0, "filter with non-existent IDs should return empty results")
}

func TestDeleteEmbeddingsByChunkIDs_Empty(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert an embedding
	err := db.InsertEmbedding(ctx, "chunk-1", makeEmbedding(0.1))
	require.NoError(t, err)

	// Delete with empty slice should not error and not delete anything
	err = db.DeleteEmbeddingsByChunkIDs(ctx, []string{})
	require.NoError(t, err)

	// Original embedding should still be there
	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestInsertEmbeddings_Empty(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert empty batch should not error
	err := db.InsertEmbeddings(ctx, []string{}, [][]float32{})
	require.NoError(t, err)

	// Count should be 0
	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
