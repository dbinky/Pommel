package chunker

import (
	"context"
	"fmt"

	"github.com/pommel-dev/pommel/internal/models"
)

// Chunker interface that all chunkers implement
type Chunker interface {
	Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error)
	Language() Language
}

// ChunkerRegistry routes files to appropriate chunkers based on language
type ChunkerRegistry struct {
	parser   *Parser
	chunkers map[Language]Chunker
	fallback Chunker
}

// NewChunkerRegistry creates a new ChunkerRegistry with all supported language chunkers
func NewChunkerRegistry() (*ChunkerRegistry, error) {
	parser, err := NewParser()
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	reg := &ChunkerRegistry{
		parser:   parser,
		chunkers: make(map[Language]Chunker),
		fallback: NewFallbackChunker(),
	}

	// Register language-specific chunkers
	reg.chunkers[LangGo] = NewGoChunker(parser)
	reg.chunkers[LangJava] = NewJavaChunker(parser)
	reg.chunkers[LangCSharp] = NewCSharpChunker(parser)
	reg.chunkers[LangPython] = NewPythonChunker(parser)
	reg.chunkers[LangJavaScript] = NewJavaScriptChunker(parser, LangJavaScript)
	reg.chunkers[LangJSX] = NewJavaScriptChunker(parser, LangJSX)
	reg.chunkers[LangTypeScript] = NewJavaScriptChunker(parser, LangTypeScript)
	reg.chunkers[LangTSX] = NewJavaScriptChunker(parser, LangTSX)

	return reg, nil
}

// Chunk processes a source file and returns its chunks using the appropriate chunker
func (r *ChunkerRegistry) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
	if file == nil {
		return nil, fmt.Errorf("file is required")
	}

	lang := DetectLanguage(file.Path)

	chunker, ok := r.chunkers[lang]
	if !ok {
		// Use fallback for unsupported languages
		file.Language = string(lang)
		return r.fallback.Chunk(ctx, file)
	}

	file.Language = string(lang)
	return chunker.Chunk(ctx, file)
}

// SupportedLanguages returns a list of all languages with registered chunkers
func (r *ChunkerRegistry) SupportedLanguages() []Language {
	languages := make([]Language, 0, len(r.chunkers))
	for lang := range r.chunkers {
		languages = append(languages, lang)
	}
	return languages
}

// IsSupported returns true if there is a registered chunker for the given language
func (r *ChunkerRegistry) IsSupported(lang Language) bool {
	_, ok := r.chunkers[lang]
	return ok
}
