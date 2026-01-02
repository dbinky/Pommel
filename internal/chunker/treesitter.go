package chunker

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	markdown "github.com/smacker/go-tree-sitter/markdown/tree-sitter-markdown"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// grammarRegistry maps grammar names (from config) to tree-sitter language getters.
// This allows config-driven language loading without hard-coding language types.
// Multiple aliases are supported for compatibility (e.g., "csharp" and "c_sharp").
var grammarRegistry = map[string]func() *sitter.Language{
	"go":         golang.GetLanguage,
	"java":       java.GetLanguage,
	"csharp":     csharp.GetLanguage,
	"c_sharp":    csharp.GetLanguage, // alias for csharp
	"python":     python.GetLanguage,
	"javascript": javascript.GetLanguage,
	"typescript": typescript.GetLanguage,
	"tsx":        tsx.GetLanguage,
	"markdown":   markdown.GetLanguage,
}

// GetLanguageGrammar returns the tree-sitter language for the given grammar name.
// The grammar name should match the tree_sitter.grammar field in language config files.
func GetLanguageGrammar(name string) (*sitter.Language, error) {
	getter, ok := grammarRegistry[name]
	if !ok {
		return nil, fmt.Errorf("unsupported grammar: %s", name)
	}
	return getter(), nil
}

// SupportedGrammars returns a list of all grammar names supported by the parser.
func SupportedGrammars() []string {
	grammars := make([]string, 0, len(grammarRegistry))
	for name := range grammarRegistry {
		grammars = append(grammars, name)
	}
	return grammars
}

// IsGrammarSupported returns true if the given grammar name is supported.
func IsGrammarSupported(name string) bool {
	_, ok := grammarRegistry[name]
	return ok
}

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
	LangMarkdown   Language = "markdown"
	LangUnknown    Language = "unknown"
)

// Parser wraps tree-sitter functionality for parsing multiple languages.
type Parser struct {
	parsers map[Language]*sitter.Parser
	mu      sync.Mutex
}

// NewParser initializes all language parsers and returns a new Parser instance.
func NewParser() (*Parser, error) {
	parsers := make(map[Language]*sitter.Parser)

	// Initialize Go parser
	goParser := sitter.NewParser()
	goParser.SetLanguage(golang.GetLanguage())
	parsers[LangGo] = goParser

	// Initialize Java parser
	javaParser := sitter.NewParser()
	javaParser.SetLanguage(java.GetLanguage())
	parsers[LangJava] = javaParser

	// Initialize C# parser
	csharpParser := sitter.NewParser()
	csharpParser.SetLanguage(csharp.GetLanguage())
	parsers[LangCSharp] = csharpParser
	parsers[Language("c_sharp")] = csharpParser // alias for config compatibility

	// Initialize Python parser
	pythonParser := sitter.NewParser()
	pythonParser.SetLanguage(python.GetLanguage())
	parsers[LangPython] = pythonParser

	// Initialize JavaScript parser
	jsParser := sitter.NewParser()
	jsParser.SetLanguage(javascript.GetLanguage())
	parsers[LangJavaScript] = jsParser

	// Initialize TypeScript parser
	tsParser := sitter.NewParser()
	tsParser.SetLanguage(typescript.GetLanguage())
	parsers[LangTypeScript] = tsParser

	// Initialize TSX parser
	tsxParser := sitter.NewParser()
	tsxParser.SetLanguage(tsx.GetLanguage())
	parsers[LangTSX] = tsxParser

	// Initialize JSX parser (uses JavaScript parser since JS supports JSX)
	jsxParser := sitter.NewParser()
	jsxParser.SetLanguage(javascript.GetLanguage())
	parsers[LangJSX] = jsxParser

	// Initialize Markdown parser
	mdParser := sitter.NewParser()
	mdParser.SetLanguage(markdown.GetLanguage())
	parsers[LangMarkdown] = mdParser

	return &Parser{
		parsers: parsers,
	}, nil
}

// Parse parses the given source code using the appropriate language parser.
func (p *Parser) Parse(ctx context.Context, lang Language, source []byte) (*sitter.Tree, error) {
	parser, ok := p.parsers[lang]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", lang)
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
		return nil, fmt.Errorf("failed to parse %s: %w", lang, err)
	}

	return tree, nil
}

// SupportedLanguages returns a list of all supported languages.
func (p *Parser) SupportedLanguages() []Language {
	languages := make([]Language, 0, len(p.parsers))
	for lang := range p.parsers {
		languages = append(languages, lang)
	}
	return languages
}

// IsSupported returns true if the given language is supported by the parser.
func (p *Parser) IsSupported(lang Language) bool {
	_, ok := p.parsers[lang]
	return ok
}

// DetectLanguage detects the programming language based on the file extension.
// Detection is case-sensitive - only lowercase extensions are recognized.
func DetectLanguage(filename string) Language {
	ext := strings.ToLower(filepath.Ext(filename))

	// Check if the original extension matches the lowercase version
	// This ensures case-sensitivity - only lowercase extensions match
	originalExt := filepath.Ext(filename)
	if ext != originalExt {
		return LangUnknown
	}

	switch ext {
	case ".go":
		return LangGo
	case ".java":
		return LangJava
	case ".cs":
		return LangCSharp
	case ".py":
		return LangPython
	case ".js":
		return LangJavaScript
	case ".jsx":
		return LangJSX
	case ".ts":
		return LangTypeScript
	case ".tsx":
		return LangTSX
	case ".md", ".markdown", ".mdown", ".mkdn":
		return LangMarkdown
	default:
		return LangUnknown
	}
}
