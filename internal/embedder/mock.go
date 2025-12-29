package embedder

import (
	"context"
	"crypto/sha256"
	"errors"
	"math"
)

// MockEmbedder is a test implementation of the Embedder interface that generates
// deterministic embeddings based on input text. It uses SHA256 hashing to ensure
// that the same input always produces the same output embedding.
type MockEmbedder struct {
	dimensions int
	healthy    bool
	modelName  string
}

// Compile-time check that MockEmbedder implements Embedder
var _ Embedder = (*MockEmbedder)(nil)

// NewMockEmbedder creates a new MockEmbedder with default settings:
// - 768 dimensions (matching Jina Code embeddings)
// - healthy=true
// - modelName="mock-embedder"
func NewMockEmbedder() *MockEmbedder {
	return &MockEmbedder{
		dimensions: 768,
		healthy:    true,
		modelName:  "mock-embedder",
	}
}

// SetHealthy toggles the health state of the mock embedder.
// When set to false, Health() will return an error.
func (m *MockEmbedder) SetHealthy(healthy bool) {
	m.healthy = healthy
}

// EmbedSingle generates a deterministic embedding for a single text input.
// The same input text will always produce the same output embedding.
func (m *MockEmbedder) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	return m.generateDeterministic(text), nil
}

// Embed generates deterministic embeddings for multiple text inputs.
// Each input text produces a consistent embedding.
func (m *MockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embeddings[i] = m.generateDeterministic(text)
	}
	return embeddings, nil
}

// Health returns nil if the embedder is healthy, or an error if not.
// Use SetHealthy(false) to simulate an unhealthy state.
func (m *MockEmbedder) Health(ctx context.Context) error {
	if !m.healthy {
		return errors.New("mock embedder is unhealthy")
	}
	return nil
}

// ModelName returns the name of the mock model.
func (m *MockEmbedder) ModelName() string {
	return m.modelName
}

// Dimensions returns the embedding dimension count.
func (m *MockEmbedder) Dimensions() int {
	return m.dimensions
}

// generateDeterministic creates a deterministic embedding from the input text.
// Uses SHA256 to generate pseudo-random but consistent values, then normalizes
// the vector to have unit magnitude.
func (m *MockEmbedder) generateDeterministic(text string) []float32 {
	embedding := make([]float32, m.dimensions)

	// Use SHA256 to generate pseudo-random but deterministic values
	hash := sha256.Sum256([]byte(text))

	for i := 0; i < m.dimensions; i++ {
		idx := i % 32
		val := float64(hash[idx]) / 255.0
		offset := float64(i) / float64(m.dimensions)
		embedding[i] = float32(val*0.5 + offset*0.5)
	}

	// Normalize the vector (magnitude = 1)
	var norm float64
	for _, v := range embedding {
		norm += float64(v * v)
	}
	norm = math.Sqrt(norm)
	for i := range embedding {
		embedding[i] = float32(float64(embedding[i]) / norm)
	}

	return embedding
}
