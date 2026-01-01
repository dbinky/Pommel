package chunker

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/models"
)

// Chunker interface that all chunkers implement
type Chunker interface {
	Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error)
	Language() Language
}

// ChunkerRegistry routes files to appropriate chunkers based on language
type ChunkerRegistry struct {
	parser          *Parser
	chunkers        map[Language]Chunker
	extensionToLang map[string]Language // maps file extensions to languages for O(1) lookup
	fallback        Chunker
}

// NewChunkerRegistry creates a new ChunkerRegistry with all supported language chunkers.
// This uses the config-driven approach, loading chunkers from YAML configuration files
// in the languages/ directory.
func NewChunkerRegistry() (*ChunkerRegistry, error) {
	// Get languages directory relative to this file
	languagesDir := getEmbeddedLanguagesDir()

	// Use config-driven registry
	return NewRegistryFromConfig(languagesDir)
}

// getEmbeddedLanguagesDir returns the path to the languages/ directory.
// It checks locations in this order:
//  1. Platform-specific installed location (via config.LanguagesDir)
//  2. Source-relative path (for development)
//  3. Current directory fallback
func getEmbeddedLanguagesDir() string {
	// First, try the installed location (e.g., ~/.local/share/pommel/languages)
	if installedDir, err := config.LanguagesDir(); err == nil {
		if _, err := os.Stat(installedDir); err == nil {
			return installedDir
		}
	}

	// Fall back to source-relative path (for development)
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		sourceDir := filepath.Join(filepath.Dir(filename), "..", "..", "languages")
		if _, err := os.Stat(sourceDir); err == nil {
			return sourceDir
		}
	}

	// Last resort fallback
	return "languages"
}

// NewRegistryFromConfig creates a registry by loading language configs from a directory.
// This is the config-driven constructor that uses GenericChunker for all languages
// defined in YAML configuration files. It enables declarative language support.
func NewRegistryFromConfig(configDir string) (*ChunkerRegistry, error) {
	parser, err := NewParser()
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	reg := &ChunkerRegistry{
		parser:          parser,
		chunkers:        make(map[Language]Chunker),
		extensionToLang: make(map[string]Language),
		fallback:        NewFallbackChunker(),
	}

	if err := reg.loadConfigs(configDir); err != nil {
		return nil, err
	}

	return reg, nil
}

// loadConfigs loads all language configs from a directory and registers chunkers.
// If some configs fail to load, it logs warnings but continues with successfully loaded ones.
func (r *ChunkerRegistry) loadConfigs(configDir string) error {
	configs, errors := LoadAllLanguageConfigs(configDir)

	// Log warnings for any configs that failed to load
	for _, err := range errors {
		log.Printf("WARNING: %v", err)
	}

	// If no configs were loaded successfully, return an error
	if len(configs) == 0 {
		if len(errors) > 0 {
			return fmt.Errorf("failed to load any language configs: %v", errors[0])
		}
		return fmt.Errorf("no language configs found in %s", configDir)
	}

	// Register each successfully loaded config
	for _, config := range configs {
		if err := r.registerFromConfig(config); err != nil {
			log.Printf("WARNING: failed to register language %s: %v", config.Language, err)
		}
	}

	return nil
}

// registerFromConfig creates and registers a GenericChunker from a LanguageConfig.
// It also builds the extension-to-language mapping for O(1) extension lookup.
func (r *ChunkerRegistry) registerFromConfig(config *LanguageConfig) error {
	// Verify the grammar is supported
	if !IsGrammarSupported(config.TreeSitter.Grammar) {
		return fmt.Errorf("unsupported grammar: %s", config.TreeSitter.Grammar)
	}

	// Create GenericChunker for this language
	chunker, err := NewGenericChunker(r.parser, config)
	if err != nil {
		return fmt.Errorf("failed to create chunker for %s: %w", config.Language, err)
	}

	// Determine the Language key - use the grammar name for parser compatibility
	lang := Language(config.TreeSitter.Grammar)

	// Register the chunker
	r.chunkers[lang] = chunker

	// Map all extensions to this language
	for _, ext := range config.Extensions {
		normalizedExt := strings.ToLower(ext)
		r.extensionToLang[normalizedExt] = lang
	}

	return nil
}

// GetChunkerForExtension returns the chunker for a file extension and whether one was found.
// The extension should include the leading dot (e.g., ".go", ".py").
// Extension lookup is case-insensitive.
func (r *ChunkerRegistry) GetChunkerForExtension(ext string) (Chunker, bool) {
	normalizedExt := strings.ToLower(ext)
	lang, ok := r.extensionToLang[normalizedExt]
	if !ok {
		return r.fallback, false
	}
	chunker, ok := r.chunkers[lang]
	if !ok {
		return r.fallback, false
	}
	return chunker, true
}

// GetLanguageForExtension returns the language for a file extension.
// The extension should include the leading dot (e.g., ".go", ".py").
// Extension lookup is case-insensitive.
func (r *ChunkerRegistry) GetLanguageForExtension(ext string) (Language, bool) {
	normalizedExt := strings.ToLower(ext)
	lang, ok := r.extensionToLang[normalizedExt]
	return lang, ok
}

// Chunk processes a source file and returns its chunks using the appropriate chunker
func (r *ChunkerRegistry) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
	if file == nil {
		return nil, fmt.Errorf("file is required")
	}

	// Try to find chunker by file extension first (config-driven approach)
	ext := strings.ToLower(filepath.Ext(file.Path))
	if lang, ok := r.extensionToLang[ext]; ok {
		if chunker, found := r.chunkers[lang]; found {
			// Set language to the config's language name for this extension
			file.Language = string(lang)
			return chunker.Chunk(ctx, file)
		}
	}

	// Fall back to detecting language (legacy approach) for backward compatibility
	lang := DetectLanguage(file.Path)
	if chunker, ok := r.chunkers[lang]; ok {
		file.Language = string(lang)
		return chunker.Chunk(ctx, file)
	}

	// Use fallback for unsupported languages
	file.Language = string(lang)
	return r.fallback.Chunk(ctx, file)
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
