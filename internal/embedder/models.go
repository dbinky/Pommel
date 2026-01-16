package embedder

import (
	"fmt"
	"strings"
)

// ModelInfo contains metadata about an embedding model.
type ModelInfo struct {
	Name        string // Full Ollama model name
	Dimensions  int    // Embedding vector dimensions
	ContextSize int    // Maximum context window in tokens
	Size        string // Human-readable size (e.g., "~300MB")
}

// EmbeddingModels maps short names to model info.
var EmbeddingModels = map[string]ModelInfo{
	"v2": {
		Name:        "unclemusclez/jina-embeddings-v2-base-code",
		Dimensions:  768,
		ContextSize: 8192,
		Size:        "~300MB",
	},
	"v4": {
		Name:        "sellerscrisp/jina-embeddings-v4-text-code-q4",
		Dimensions:  1024,
		ContextSize: 32768,
		Size:        "~8GB",
	},
}

// DefaultModel is the short name of the default embedding model.
const DefaultModel = "v2"

// DefaultDimensions is the fallback for unknown models.
const DefaultDimensions = 768

// DefaultContextSize is the fallback for unknown models.
const DefaultContextSize = 8192

// GetModelInfo returns model info by short name (v2, v4).
func GetModelInfo(shortName string) (*ModelInfo, error) {
	shortName = strings.ToLower(strings.TrimSpace(shortName))
	if shortName == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}
	info, ok := EmbeddingModels[shortName]
	if !ok {
		return nil, fmt.Errorf("unknown model '%s', use v2 or v4", shortName)
	}
	return &info, nil
}

// GetModelByFullName returns model info by full Ollama model name.
// Returns nil if the model is not in our registry (unknown model).
func GetModelByFullName(fullName string) *ModelInfo {
	fullName = strings.TrimSpace(fullName)
	for _, info := range EmbeddingModels {
		if info.Name == fullName {
			return &info
		}
	}
	return nil
}

// GetDimensionsForModel returns dimensions for a model by full name.
// Returns DefaultDimensions (768) for unknown models.
func GetDimensionsForModel(fullName string) int {
	info := GetModelByFullName(fullName)
	if info == nil {
		return DefaultDimensions
	}
	return info.Dimensions
}

// GetContextSizeForModel returns context size for a model by full name.
// Returns DefaultContextSize (8192) for unknown models.
func GetContextSizeForModel(fullName string) int {
	info := GetModelByFullName(fullName)
	if info == nil {
		return DefaultContextSize
	}
	return info.ContextSize
}

// GetShortNameForModel returns the short name (v2, v4) for a full Ollama model name.
// Returns empty string if the model is not in our registry.
func GetShortNameForModel(fullName string) string {
	fullName = strings.TrimSpace(fullName)
	for shortName, info := range EmbeddingModels {
		if info.Name == fullName {
			return shortName
		}
	}
	return ""
}
