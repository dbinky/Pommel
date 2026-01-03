package chunker

//go:generate go run ./generate/main.go

import (
	"context"
	"fmt"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
)

// Language represents a programming language supported by the parser.
type Language string

const (
	LangGo         Language = "go"
	LangJava       Language = "java"
	LangCSharp     Language = "csharp"
	LangPython     Language = "python"
	LangJavaScript Language = "javascript"
	LangTypeScript Language = "typescript"
	LangTSX        Language = "tsx"
	LangJSX        Language = "jsx"
	LangUnknown    Language = "unknown"
)

// Parser wraps tree-sitter functionality for parsing multiple languages.
type Parser struct {
	parsers map[string]*sitter.Parser
	mu      sync.Mutex
}

// NewParser initializes parsers for all supported languages and returns a new Parser instance.
// Parsers are created dynamically from the generated language registry.
func NewParser() (*Parser, error) {
	parsers := make(map[string]*sitter.Parser)

	// Create a parser for each language, looking up its grammar
	for _, langName := range supportedLanguages {
		grammarName := GetGrammarForLanguage(langName)
		getLanguage, ok := grammarRegistry[grammarName]
		if !ok {
			// This shouldn't happen if configs are validated correctly
			continue
		}
		parser := sitter.NewParser()
		parser.SetLanguage(getLanguage())
		parsers[langName] = parser
	}

	return &Parser{
		parsers: parsers,
	}, nil
}

// Parse parses the given source code using the appropriate language parser.
func (p *Parser) Parse(ctx context.Context, lang Language, source []byte) (*sitter.Tree, error) {
	return p.ParseByName(ctx, string(lang), source)
}

// ParseByName parses the given source code using the parser for the named language.
func (p *Parser) ParseByName(ctx context.Context, langName string, source []byte) (*sitter.Tree, error) {
	parser, ok := p.parsers[langName]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", langName)
	}

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Lock to ensure thread safety as tree-sitter parsers are not thread-safe
	p.mu.Lock()
	defer p.mu.Unlock()

	tree, err := parser.ParseCtx(ctx, nil, source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", langName, err)
	}

	return tree, nil
}

// IsSupported returns true if the given language is supported by the parser.
func (p *Parser) IsSupported(lang Language) bool {
	_, ok := p.parsers[string(lang)]
	return ok
}

// IsSupportedByName returns true if the given language name is supported by the parser.
func (p *Parser) IsSupportedByName(langName string) bool {
	_, ok := p.parsers[langName]
	return ok
}

// DetectLanguage detects the programming language based on the file extension.
// This is a convenience wrapper that returns the Language type.
// For the string-based language name, use DetectLanguageByExtension directly.
func DetectLanguage(filename string) Language {
	langName := DetectLanguageByExtension(filename)
	return Language(langName)
}

// SupportedGrammars returns a list of all grammar names supported by the parser.
// Deprecated: Use SupportedLanguages() instead.
func SupportedGrammars() []string {
	return SupportedLanguages()
}
