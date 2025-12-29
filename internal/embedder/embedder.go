package embedder

import "context"

// Embedder generates vector embeddings from text
type Embedder interface {
	EmbedSingle(ctx context.Context, text string) ([]float32, error)
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Health(ctx context.Context) error
	ModelName() string
	Dimensions() int
}

// Compile-time check that OllamaClient implements Embedder
var _ Embedder = (*OllamaClient)(nil)
