package embedder

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Interface Compliance
// ============================================================================

// TestMockEmbedder_ImplementsEmbedder verifies that MockEmbedder implements
// the Embedder interface at compile time.
func TestMockEmbedder_ImplementsEmbedder(t *testing.T) {
	// This test verifies at compile time that MockEmbedder implements Embedder.
	// The var _ Embedder = (*MockEmbedder)(nil) in mock.go does the actual check,
	// but we include a runtime assertion for clarity.
	var embedder Embedder = &MockEmbedder{}
	assert.NotNil(t, embedder, "MockEmbedder should implement Embedder interface")
}

// ============================================================================
// Happy Path / Success Cases
// ============================================================================

// TestMockEmbedder_EmbedSingle_Deterministic verifies that the same input
// always produces the same output embedding. This is essential for testing
// code that depends on consistent embeddings.
func TestMockEmbedder_EmbedSingle_Deterministic(t *testing.T) {
	mock := NewMockEmbedder()
	require.NotNil(t, mock, "NewMockEmbedder should return non-nil instance")

	ctx := context.Background()
	text := "func main() { fmt.Println(\"hello\") }"

	// Generate embedding twice for same input
	embedding1, err1 := mock.EmbedSingle(ctx, text)
	embedding2, err2 := mock.EmbedSingle(ctx, text)

	require.NoError(t, err1, "First EmbedSingle should not return error")
	require.NoError(t, err2, "Second EmbedSingle should not return error")
	require.NotNil(t, embedding1, "First embedding should not be nil")
	require.NotNil(t, embedding2, "Second embedding should not be nil")

	// Embeddings should be identical
	assert.Equal(t, embedding1, embedding2,
		"Same input should always produce identical embeddings")
}

// TestMockEmbedder_EmbedSingle_DifferentInputs verifies that different inputs
// produce different output embeddings.
func TestMockEmbedder_EmbedSingle_DifferentInputs(t *testing.T) {
	mock := NewMockEmbedder()
	require.NotNil(t, mock, "NewMockEmbedder should return non-nil instance")

	ctx := context.Background()

	embedding1, err1 := mock.EmbedSingle(ctx, "func hello() {}")
	embedding2, err2 := mock.EmbedSingle(ctx, "func world() {}")

	require.NoError(t, err1, "First EmbedSingle should not return error")
	require.NoError(t, err2, "Second EmbedSingle should not return error")
	require.NotNil(t, embedding1, "First embedding should not be nil")
	require.NotNil(t, embedding2, "Second embedding should not be nil")

	// Embeddings should be different for different inputs
	assert.NotEqual(t, embedding1, embedding2,
		"Different inputs should produce different embeddings")
}

// TestMockEmbedder_Embed_Multiple verifies that the Embed method correctly
// handles multiple texts and returns the right number of embeddings.
func TestMockEmbedder_Embed_Multiple(t *testing.T) {
	mock := NewMockEmbedder()
	require.NotNil(t, mock, "NewMockEmbedder should return non-nil instance")

	ctx := context.Background()
	texts := []string{
		"func test1() {}",
		"func test2() {}",
		"func test3() {}",
	}

	embeddings, err := mock.Embed(ctx, texts)

	require.NoError(t, err, "Embed should not return error")
	require.NotNil(t, embeddings, "Embeddings should not be nil")
	assert.Len(t, embeddings, len(texts),
		"Should return one embedding for each input text")

	// Each embedding should have correct dimensions
	for i, emb := range embeddings {
		assert.Len(t, emb, 768,
			"Embedding %d should have 768 dimensions", i)
	}
}

// TestMockEmbedder_Dimensions verifies that Dimensions returns 768,
// matching the Jina Code embeddings model.
func TestMockEmbedder_Dimensions(t *testing.T) {
	mock := NewMockEmbedder()
	require.NotNil(t, mock, "NewMockEmbedder should return non-nil instance")

	dims := mock.Dimensions()
	assert.Equal(t, 768, dims, "Dimensions should return 768")
}

// TestMockEmbedder_ModelName verifies that ModelName returns "mock-embedder".
func TestMockEmbedder_ModelName(t *testing.T) {
	mock := NewMockEmbedder()
	require.NotNil(t, mock, "NewMockEmbedder should return non-nil instance")

	name := mock.ModelName()
	assert.Equal(t, "mock-embedder", name,
		"ModelName should return 'mock-embedder'")
}

// TestMockEmbedder_Health_Default verifies that Health returns nil (healthy)
// by default when no errors are configured.
func TestMockEmbedder_Health_Default(t *testing.T) {
	mock := NewMockEmbedder()
	require.NotNil(t, mock, "NewMockEmbedder should return non-nil instance")

	err := mock.Health(context.Background())
	assert.NoError(t, err, "Health should return nil when healthy (default state)")
}

// ============================================================================
// Failure / Error Cases
// ============================================================================

// TestMockEmbedder_Health_Unhealthy verifies that Health returns an error
// when SetHealthy(false) has been called.
func TestMockEmbedder_Health_Unhealthy(t *testing.T) {
	mock := NewMockEmbedder()
	require.NotNil(t, mock, "NewMockEmbedder should return non-nil instance")

	// Set mock to unhealthy state
	mock.SetHealthy(false)

	err := mock.Health(context.Background())
	assert.Error(t, err, "Health should return error when SetHealthy(false)")
	assert.Contains(t, err.Error(), "unhealthy",
		"Error message should indicate unhealthy state")
}

// TestMockEmbedder_Embed_Empty verifies that Embed returns an empty slice
// (not nil) when given an empty input slice.
func TestMockEmbedder_Embed_Empty(t *testing.T) {
	mock := NewMockEmbedder()
	require.NotNil(t, mock, "NewMockEmbedder should return non-nil instance")

	ctx := context.Background()
	embeddings, err := mock.Embed(ctx, []string{})

	require.NoError(t, err, "Embed should not return error for empty input")
	require.NotNil(t, embeddings, "Embeddings should not be nil for empty input")
	assert.Empty(t, embeddings, "Embeddings should be empty slice for empty input")
}

// ============================================================================
// Edge Cases
// ============================================================================

// TestMockEmbedder_EmbedSingle_Normalized verifies that output vectors are
// normalized (have magnitude approximately equal to 1).
func TestMockEmbedder_EmbedSingle_Normalized(t *testing.T) {
	mock := NewMockEmbedder()
	require.NotNil(t, mock, "NewMockEmbedder should return non-nil instance")

	ctx := context.Background()
	embedding, err := mock.EmbedSingle(ctx, "func test() {}")

	require.NoError(t, err, "EmbedSingle should not return error")
	require.NotNil(t, embedding, "Embedding should not be nil")
	require.Len(t, embedding, 768, "Embedding should have 768 dimensions")

	// Calculate magnitude (L2 norm) of the vector
	var sumSquares float64
	for _, v := range embedding {
		sumSquares += float64(v) * float64(v)
	}
	magnitude := math.Sqrt(sumSquares)

	// Magnitude should be approximately 1.0 (with some tolerance for float precision)
	assert.InDelta(t, 1.0, magnitude, 0.001,
		"Embedding vector should be normalized (magnitude ~1.0), got %f", magnitude)
}

// TestMockEmbedder_EmbedSingle_EmptyString verifies that an empty string
// input still produces a valid embedding.
func TestMockEmbedder_EmbedSingle_EmptyString(t *testing.T) {
	mock := NewMockEmbedder()
	require.NotNil(t, mock, "NewMockEmbedder should return non-nil instance")

	ctx := context.Background()
	embedding, err := mock.EmbedSingle(ctx, "")

	require.NoError(t, err, "EmbedSingle should not return error for empty string")
	require.NotNil(t, embedding, "Embedding should not be nil for empty string")
	assert.Len(t, embedding, 768,
		"Empty string should still produce 768-dimensional embedding")
}
