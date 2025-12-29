package chunker

import (
	"context"
	"fmt"
	"strings"

	"github.com/pommel-dev/pommel/internal/models"
)

// FallbackChunker is a simple chunker that creates a single file-level chunk
// for any file. It is used when no language-specific chunker is available.
type FallbackChunker struct{}

// NewFallbackChunker creates a new FallbackChunker instance.
func NewFallbackChunker() *FallbackChunker {
	return &FallbackChunker{}
}

// Chunk creates a single file-level chunk from the given source file.
func (c *FallbackChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
	// Check context
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Handle nil file
	if file == nil {
		return nil, fmt.Errorf("file is required")
	}

	// Count lines
	content := string(file.Content)
	var endLine int
	if content == "" {
		endLine = 1
	} else {
		lines := strings.Split(content, "\n")
		endLine = len(lines)
		if endLine == 0 {
			endLine = 1
		}
	}

	chunk := &models.Chunk{
		FilePath:     file.Path,
		StartLine:    1,
		EndLine:      endLine,
		Level:        models.ChunkLevelFile,
		Language:     file.Language,
		Content:      content,
		Name:         file.Path,
		LastModified: file.LastModified,
	}
	chunk.SetHashes()

	return &models.ChunkResult{
		File:   file,
		Chunks: []*models.Chunk{chunk},
	}, nil
}

// Language returns LangUnknown as this chunker handles all unknown/unsupported languages.
func (c *FallbackChunker) Language() Language {
	return LangUnknown
}
