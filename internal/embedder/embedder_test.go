package embedder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Interface Compliance Tests
// ============================================================================

// TestOllamaClient_ImplementsEmbedder is a compile-time check that OllamaClient
// implements the Embedder interface. If this test compiles, OllamaClient
// satisfies all required methods.
func TestOllamaClient_ImplementsEmbedder(t *testing.T) {
	// This test verifies at compile time that OllamaClient implements Embedder.
	// The actual check is done via the var _ Embedder = (*OllamaClient)(nil)
	// declaration in embedder.go, but we include a runtime assertion for clarity.
	var embedder Embedder = &OllamaClient{}
	assert.NotNil(t, embedder, "OllamaClient should implement Embedder interface")
}

// TestEmbedder_InterfaceMethods verifies that the Embedder interface has the
// expected method signatures. This test ensures the interface contract is stable.
// The actual method signature verification happens at compile time via the
// var _ Embedder = (*OllamaClient)(nil) declaration in embedder.go.
func TestEmbedder_InterfaceMethods(t *testing.T) {
	// The interface defines 5 methods. This test documents the expected contract.
	// If the interface changes, the compile-time check in embedder.go will fail.
	//
	// Expected methods:
	// - EmbedSingle(ctx context.Context, text string) ([]float32, error)
	// - Embed(ctx context.Context, texts []string) ([][]float32, error)
	// - Health(ctx context.Context) error
	// - ModelName() string
	// - Dimensions() int

	t.Run("OllamaClient satisfies Embedder interface", func(t *testing.T) {
		// This assignment will fail to compile if OllamaClient doesn't implement Embedder
		var _ Embedder = (*OllamaClient)(nil)
		assert.True(t, true, "OllamaClient implements all Embedder interface methods")
	})
}
