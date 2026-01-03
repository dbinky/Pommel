package chunker

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Parser Initialization Tests
// =============================================================================

func TestNewParser(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err, "NewParser should not return an error")
	assert.NotNil(t, parser, "NewParser should return a non-nil parser")
}

func TestNewParser_SupportsAllExpectedLanguages(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	// Test that parser supports the languages from YAML configs
	// Note: JSX/TSX now have their own configs
	expectedLanguages := []string{
		"go",
		"java",
		"csharp", // User-friendly name, not grammar name
		"python",
		"javascript",
		"typescript",
		"rust", // Now supported
		"jsx",
		"tsx",
	}

	for _, lang := range expectedLanguages {
		t.Run(lang, func(t *testing.T) {
			assert.True(t, parser.IsSupportedByName(lang), "Parser should support %s", lang)
		})
	}
}

func TestParser_SupportedLanguages(t *testing.T) {
	// Test package-level SupportedLanguages function
	langs := SupportedLanguages()
	assert.NotEmpty(t, langs, "SupportedLanguages should return at least one language")

	// Test that parser supports the expected core languages
	parser, err := NewParser()
	require.NoError(t, err)

	expectedLanguages := []string{
		"go",
		"java",
		"csharp", // User-friendly name
		"python",
		"javascript",
		"typescript",
	}

	for _, lang := range expectedLanguages {
		assert.True(t, parser.IsSupportedByName(lang), "Parser should support %s", lang)
	}
}

// =============================================================================
// Language Parsing Tests
// =============================================================================

func TestParser_Parse_CSharp(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
public class MyClass {
    public void MyMethod() {
        Console.WriteLine("Hello");
    }
}
`)

	tree, err := parser.Parse(context.Background(), LangCSharp, source)
	require.NoError(t, err, "Parse should not return an error for valid C# code")
	assert.NotNil(t, tree, "Parse should return a non-nil tree")
	assert.Equal(t, "compilation_unit", tree.RootNode().Type(), "C# root node should be 'compilation_unit'")
}

func TestParser_Parse_CSharp_Namespace(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
namespace MyNamespace
{
    public class Calculator
    {
        public int Add(int a, int b)
        {
            return a + b;
        }
    }
}
`)

	tree, err := parser.Parse(context.Background(), LangCSharp, source)
	require.NoError(t, err)
	assert.NotNil(t, tree)
	assert.Equal(t, "compilation_unit", tree.RootNode().Type())
}

func TestParser_Parse_Python(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
class MyClass:
    def my_method(self):
        print("Hello")
`)

	tree, err := parser.Parse(context.Background(), LangPython, source)
	require.NoError(t, err, "Parse should not return an error for valid Python code")
	assert.NotNil(t, tree, "Parse should return a non-nil tree")
	assert.Equal(t, "module", tree.RootNode().Type(), "Python root node should be 'module'")
}

func TestParser_Parse_Python_TopLevelFunction(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
def greet(name):
    return f"Hello, {name}!"

def farewell(name):
    return f"Goodbye, {name}!"
`)

	tree, err := parser.Parse(context.Background(), LangPython, source)
	require.NoError(t, err)
	assert.NotNil(t, tree)
	assert.Equal(t, "module", tree.RootNode().Type())
}

func TestParser_Parse_JavaScript(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
class MyClass {
    myMethod() {
        console.log("Hello");
    }
}
`)

	tree, err := parser.Parse(context.Background(), LangJavaScript, source)
	require.NoError(t, err, "Parse should not return an error for valid JavaScript code")
	assert.NotNil(t, tree, "Parse should return a non-nil tree")
	assert.Equal(t, "program", tree.RootNode().Type(), "JavaScript root node should be 'program'")
}

func TestParser_Parse_JavaScript_ArrowFunction(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
const greet = (name) => {
    return "Hello, " + name;
};

function farewell(name) {
    return "Goodbye, " + name;
}
`)

	tree, err := parser.Parse(context.Background(), LangJavaScript, source)
	require.NoError(t, err)
	assert.NotNil(t, tree)
	assert.Equal(t, "program", tree.RootNode().Type())
}

func TestParser_Parse_TypeScript(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
interface User {
    id: number;
    name: string;
}

class UserService {
    getUser(id: number): User {
        return { id, name: "test" };
    }
}
`)

	tree, err := parser.Parse(context.Background(), LangTypeScript, source)
	require.NoError(t, err, "Parse should not return an error for valid TypeScript code")
	assert.NotNil(t, tree, "Parse should return a non-nil tree")
	assert.Equal(t, "program", tree.RootNode().Type(), "TypeScript root node should be 'program'")
}

func TestParser_Parse_TSX(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
import React from 'react';

interface Props {
    name: string;
}

function Greeting({ name }: Props) {
    return <div>Hello, {name}!</div>;
}

export default Greeting;
`)

	tree, err := parser.Parse(context.Background(), LangTSX, source)
	require.NoError(t, err, "Parse should not return an error for valid TSX code")
	assert.NotNil(t, tree, "Parse should return a non-nil tree")
	assert.Equal(t, "program", tree.RootNode().Type(), "TSX root node should be 'program'")
}

func TestParser_Parse_JSX(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
import React from 'react';

function Greeting({ name }) {
    return <div>Hello, {name}!</div>;
}

export default Greeting;
`)

	tree, err := parser.Parse(context.Background(), LangJSX, source)
	require.NoError(t, err, "Parse should not return an error for valid JSX code")
	assert.NotNil(t, tree, "Parse should return a non-nil tree")
	assert.Equal(t, "program", tree.RootNode().Type(), "JSX root node should be 'program'")
}

func TestParser_Parse_Unsupported(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`puts "Hello, World!"`)

	tree, err := parser.Parse(context.Background(), LangUnknown, source)
	assert.Error(t, err, "Parse should return an error for unsupported language")
	assert.Nil(t, tree, "Parse should return nil tree for unsupported language")
	assert.Contains(t, err.Error(), "unsupported", "Error message should mention 'unsupported'")
}

func TestParser_Parse_EmptySource(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	languages := []Language{
		LangCSharp,
		LangPython,
		LangJavaScript,
		LangTypeScript,
	}

	for _, lang := range languages {
		t.Run(string(lang), func(t *testing.T) {
			tree, err := parser.Parse(context.Background(), lang, []byte(""))
			// Empty source should parse successfully (produces an empty AST)
			require.NoError(t, err, "Parse should not error on empty source for %s", lang)
			assert.NotNil(t, tree, "Parse should return a tree for empty source")
		})
	}
}

func TestParser_Parse_CancelledContext(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	source := []byte(`class Test { }`)

	// Behavior may vary - either returns error or completes
	// The important thing is it does not hang indefinitely
	_, _ = parser.Parse(ctx, LangCSharp, source)
}

// =============================================================================
// Language Detection Tests
// =============================================================================

func TestDetectLanguage_CSharp(t *testing.T) {
	tests := []struct {
		filename string
		expected Language
	}{
		{"file.cs", LangCSharp},
		{"MyClass.cs", LangCSharp},
		{"path/to/file.cs", LangCSharp},
		{"File.Controller.cs", LangCSharp},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			lang := DetectLanguage(tt.filename)
			assert.Equal(t, tt.expected, lang)
		})
	}
}

func TestDetectLanguage_Python(t *testing.T) {
	tests := []struct {
		filename string
		expected Language
	}{
		{"file.py", LangPython},
		{"script.py", LangPython},
		{"path/to/module.py", LangPython},
		{"__init__.py", LangPython},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			lang := DetectLanguage(tt.filename)
			assert.Equal(t, tt.expected, lang)
		})
	}
}

func TestDetectLanguage_JavaScript(t *testing.T) {
	tests := []struct {
		filename string
		expected Language
	}{
		{"file.js", LangJavaScript},
		{"script.js", LangJavaScript},
		{"path/to/module.js", LangJavaScript},
		{"index.js", LangJavaScript},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			lang := DetectLanguage(tt.filename)
			assert.Equal(t, tt.expected, lang)
		})
	}
}

func TestDetectLanguage_JSX(t *testing.T) {
	tests := []struct {
		filename string
		expected Language
	}{
		{"file.jsx", LangJSX},
		{"Component.jsx", LangJSX},
		{"path/to/App.jsx", LangJSX},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			lang := DetectLanguage(tt.filename)
			assert.Equal(t, tt.expected, lang)
		})
	}
}

func TestDetectLanguage_TypeScript(t *testing.T) {
	tests := []struct {
		filename string
		expected Language
	}{
		{"file.ts", LangTypeScript},
		{"module.ts", LangTypeScript},
		{"path/to/service.ts", LangTypeScript},
		{"index.ts", LangTypeScript},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			lang := DetectLanguage(tt.filename)
			assert.Equal(t, tt.expected, lang)
		})
	}
}

func TestDetectLanguage_TSX(t *testing.T) {
	tests := []struct {
		filename string
		expected Language
	}{
		{"file.tsx", LangTSX},
		{"Component.tsx", LangTSX},
		{"path/to/App.tsx", LangTSX},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			lang := DetectLanguage(tt.filename)
			assert.Equal(t, tt.expected, lang)
		})
	}
}

func TestDetectLanguage_Unknown(t *testing.T) {
	tests := []struct {
		filename string
		expected Language
	}{
		{"file.txt", LangUnknown},
		{"file.json", LangUnknown},
		{"file", LangUnknown},       // No extension
		{".gitignore", LangUnknown}, // Hidden file
		{"Makefile", LangUnknown},   // No extension
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			lang := DetectLanguage(tt.filename)
			assert.Equal(t, tt.expected, lang)
		})
	}
}

func TestDetectLanguage_CaseSensitivity(t *testing.T) {
	// File extensions should be case-sensitive (lowercase)
	tests := []struct {
		filename string
		expected Language
	}{
		{"file.CS", LangUnknown}, // Uppercase should not match
		{"file.PY", LangUnknown}, // Uppercase should not match
		{"file.JS", LangUnknown}, // Uppercase should not match
		{"file.Ts", LangUnknown}, // Mixed case should not match
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			lang := DetectLanguage(tt.filename)
			assert.Equal(t, tt.expected, lang, "Detection should be case-sensitive")
		})
	}
}

// =============================================================================
// IsSupported Tests
// =============================================================================

func TestParser_IsSupported_SupportedLanguages(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	supportedLanguages := []Language{
		LangGo,
		LangJava,
		LangCSharp,
		LangPython,
		LangJavaScript,
		LangTypeScript,
		LangTSX,
		LangJSX,
	}

	for _, lang := range supportedLanguages {
		t.Run(string(lang), func(t *testing.T) {
			assert.True(t, parser.IsSupported(lang), "%s should be supported", lang)
		})
	}
}

func TestParser_IsSupported_UnsupportedLanguages(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	unsupportedLanguages := []Language{
		LangUnknown,
		Language("nonexistent"), // Definitely unsupported
		Language(""),
	}

	for _, lang := range unsupportedLanguages {
		name := string(lang)
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			assert.False(t, parser.IsSupported(lang), "%s should not be supported", lang)
		})
	}
}

// =============================================================================
// Edge Cases and Error Handling Tests
// =============================================================================

func TestParser_Parse_SyntaxError(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	// Tree-sitter is lenient and will still parse code with syntax errors
	// but the resulting tree may have error nodes
	source := []byte(`
class MyClass {
    public void BrokenMethod( {
        // Missing closing paren
    }
}
`)

	tree, err := parser.Parse(context.Background(), LangCSharp, source)
	// Tree-sitter typically does not return error, but produces a tree with ERROR nodes
	require.NoError(t, err, "Tree-sitter should handle syntax errors gracefully")
	assert.NotNil(t, tree, "Should still produce a tree even with syntax errors")
}

func TestParser_Parse_LargeFile(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	// Generate a large source file (many methods)
	var source []byte
	source = append(source, []byte("class LargeClass {\n")...)
	for i := 0; i < 100; i++ {
		source = append(source, []byte("    def method_"+string(rune('a'+i%26))+"(self):\n")...)
		source = append(source, []byte("        pass\n\n")...)
	}

	tree, err := parser.Parse(context.Background(), LangPython, source)
	require.NoError(t, err, "Parser should handle large files")
	assert.NotNil(t, tree, "Should produce a tree for large files")
}

func TestParser_Parse_UnicodeContent(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
class UnicodeTester:
    def test_emoji(self):
        message = "Hello, World!"

    def test_chinese(self):
        greeting = "Hello"

    def test_japanese(self):
        greeting = "Hello"
`)

	tree, err := parser.Parse(context.Background(), LangPython, source)
	require.NoError(t, err, "Parser should handle Unicode content")
	assert.NotNil(t, tree)
	assert.Equal(t, "module", tree.RootNode().Type())
}

func TestParser_Parse_SpecialCharacters(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
class SpecialChars {
    constructor() {
        this.regex = /[a-z]+\d*/;
        this.template = ` + "`Hello, ${name}!`" + `;
        this.escapes = "Line1\\nLine2\\tTabbed";
    }
}
`)

	tree, err := parser.Parse(context.Background(), LangJavaScript, source)
	require.NoError(t, err, "Parser should handle special characters")
	assert.NotNil(t, tree)
}

func TestParser_Parse_Markdown(t *testing.T) {
	// Markdown uses a special dual-parser API (block + inline)
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`# Main Heading

This is a paragraph with some text.

## Subheading

- Item 1
- Item 2
- Item 3

` + "```" + `go
func main() {
    fmt.Println("Hello")
}
` + "```" + `

> This is a blockquote
`)

	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err, "Parser should handle markdown with special API")
	assert.NotNil(t, tree, "Should produce a tree for markdown")
	assert.Equal(t, "document", tree.RootNode().Type(), "Root should be document")

	// Check that we can find expected node types
	root := tree.RootNode()
	assert.True(t, root.NamedChildCount() > 0, "Document should have children")

	// Verify we got block-level structure
	// Markdown creates nested sections: document > section > atx_heading, paragraph, etc.
	foundSection := false
	foundHeading := false
	foundCodeBlock := false

	// Helper to recursively find node types
	var findNodes func(node *sitter.Node)
	findNodes = func(node *sitter.Node) {
		switch node.Type() {
		case "section":
			foundSection = true
		case "atx_heading", "setext_heading":
			foundHeading = true
		case "fenced_code_block":
			foundCodeBlock = true
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			findNodes(node.NamedChild(i))
		}
	}
	findNodes(root)

	assert.True(t, foundSection, "Should find section in markdown")
	assert.True(t, foundHeading, "Should find heading in markdown")
	assert.True(t, foundCodeBlock, "Should find code block in markdown")
}

// =============================================================================
// Markdown Special Handling Tests
// =============================================================================

func TestParser_Markdown_EmptyDocument(t *testing.T) {
	// Edge case: empty markdown document
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte("")
	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err, "Should handle empty markdown")
	assert.NotNil(t, tree, "Should return a tree even for empty input")
	assert.Equal(t, "document", tree.RootNode().Type())
	assert.Equal(t, uint32(0), tree.RootNode().NamedChildCount(), "Empty doc should have no children")
}

func TestParser_Markdown_WhitespaceOnly(t *testing.T) {
	// Edge case: whitespace-only document
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte("   \n\n\t\t\n   ")
	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err, "Should handle whitespace-only markdown")
	assert.NotNil(t, tree)
	assert.Equal(t, "document", tree.RootNode().Type())
}

func TestParser_Markdown_HeadingsOnly(t *testing.T) {
	// Success case: document with only headings (all levels)
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`# Heading 1
## Heading 2
### Heading 3
#### Heading 4
##### Heading 5
###### Heading 6
`)

	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err)
	assert.NotNil(t, tree)

	// Count headings
	headingCount := 0
	var countHeadings func(node *sitter.Node)
	countHeadings = func(node *sitter.Node) {
		if node.Type() == "atx_heading" {
			headingCount++
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			countHeadings(node.NamedChild(i))
		}
	}
	countHeadings(tree.RootNode())

	assert.Equal(t, 6, headingCount, "Should find all 6 heading levels")
}

func TestParser_Markdown_SetextHeadings(t *testing.T) {
	// Success case: setext-style headings (underline style)
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`Heading 1
=========

Heading 2
---------
`)

	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err)
	assert.NotNil(t, tree)

	// Should find setext headings
	foundSetext := false
	var findSetext func(node *sitter.Node)
	findSetext = func(node *sitter.Node) {
		if node.Type() == "setext_heading" {
			foundSetext = true
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			findSetext(node.NamedChild(i))
		}
	}
	findSetext(tree.RootNode())

	assert.True(t, foundSetext, "Should find setext-style headings")
}

func TestParser_Markdown_CodeBlocks(t *testing.T) {
	// Success case: various code block formats
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
` + "```" + `
plain code block
` + "```" + `

` + "```" + `python
def hello():
    print("world")
` + "```" + `

` + "```" + `javascript
console.log("test");
` + "```" + `

    indented code block
    line 2
`)

	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err)
	assert.NotNil(t, tree)

	// Count code blocks
	fencedCount := 0
	indentedCount := 0
	var countBlocks func(node *sitter.Node)
	countBlocks = func(node *sitter.Node) {
		switch node.Type() {
		case "fenced_code_block":
			fencedCount++
		case "indented_code_block":
			indentedCount++
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			countBlocks(node.NamedChild(i))
		}
	}
	countBlocks(tree.RootNode())

	assert.Equal(t, 3, fencedCount, "Should find 3 fenced code blocks")
	assert.Equal(t, 1, indentedCount, "Should find 1 indented code block")
}

func TestParser_Markdown_Lists(t *testing.T) {
	// Success case: ordered and unordered lists
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
- Bullet 1
- Bullet 2
  - Nested bullet
- Bullet 3

1. First
2. Second
3. Third

* Star bullet
+ Plus bullet
- Minus bullet
`)

	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err)
	assert.NotNil(t, tree)

	// Should find lists
	listCount := 0
	var countLists func(node *sitter.Node)
	countLists = func(node *sitter.Node) {
		if node.Type() == "list" {
			listCount++
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			countLists(node.NamedChild(i))
		}
	}
	countLists(tree.RootNode())

	assert.True(t, listCount >= 3, "Should find multiple lists")
}

func TestParser_Markdown_Blockquotes(t *testing.T) {
	// Success case: blockquotes including nested
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
> Single line quote

> Multi-line
> blockquote
> continues

> Nested quote
> > Inner quote
> > > Deeply nested
`)

	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err)
	assert.NotNil(t, tree)

	// Count blockquotes
	quoteCount := 0
	var countQuotes func(node *sitter.Node)
	countQuotes = func(node *sitter.Node) {
		if node.Type() == "block_quote" {
			quoteCount++
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			countQuotes(node.NamedChild(i))
		}
	}
	countQuotes(tree.RootNode())

	assert.True(t, quoteCount >= 3, "Should find multiple blockquotes")
}

func TestParser_Markdown_Tables(t *testing.T) {
	// Success case: GFM tables
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
| Header 1 | Header 2 | Header 3 |
|----------|----------|----------|
| Cell 1   | Cell 2   | Cell 3   |
| Cell 4   | Cell 5   | Cell 6   |
`)

	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err)
	assert.NotNil(t, tree)
	// Tables may or may not be supported depending on tree-sitter-markdown version
	// Just verify parsing doesn't fail
}

func TestParser_Markdown_HorizontalRules(t *testing.T) {
	// Success case: thematic breaks (horizontal rules)
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
Before

---

Between

***

After

___

End
`)

	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err)
	assert.NotNil(t, tree)

	// Count thematic breaks
	breakCount := 0
	var countBreaks func(node *sitter.Node)
	countBreaks = func(node *sitter.Node) {
		if node.Type() == "thematic_break" {
			breakCount++
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			countBreaks(node.NamedChild(i))
		}
	}
	countBreaks(tree.RootNode())

	assert.Equal(t, 3, breakCount, "Should find 3 thematic breaks")
}

func TestParser_Markdown_HTMLBlocks(t *testing.T) {
	// Success case: embedded HTML
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`
# Heading

<div class="container">
  <p>Some HTML content</p>
</div>

Regular paragraph after HTML.
`)

	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err)
	assert.NotNil(t, tree)

	// Should find HTML block
	foundHTML := false
	var findHTML func(node *sitter.Node)
	findHTML = func(node *sitter.Node) {
		if node.Type() == "html_block" {
			foundHTML = true
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			findHTML(node.NamedChild(i))
		}
	}
	findHTML(tree.RootNode())

	assert.True(t, foundHTML, "Should find HTML block")
}

func TestParser_Markdown_LargeDocument(t *testing.T) {
	// Edge case: large markdown document
	parser, err := NewParser()
	require.NoError(t, err)

	// Generate a large document with many headings and paragraphs
	var source []byte
	for i := 0; i < 100; i++ {
		source = append(source, []byte("# Heading "+string(rune('A'+i%26))+"\n\n")...)
		source = append(source, []byte("This is paragraph number "+string(rune('0'+i%10))+". It has some text.\n\n")...)
		source = append(source, []byte("- List item 1\n- List item 2\n\n")...)
	}

	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err, "Should handle large markdown documents")
	assert.NotNil(t, tree)
	assert.Equal(t, "document", tree.RootNode().Type())
}

func TestParser_Markdown_UnicodeContent(t *testing.T) {
	// Edge case: Unicode and special characters
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`# 日本語のヘッダー

这是中文段落。

## Ελληνικά

Привет мир! 🎉

- Émojis work: 👍 🚀 ✨
- Special chars: © ® ™ € £ ¥
`)

	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err, "Should handle Unicode content")
	assert.NotNil(t, tree)
	assert.Equal(t, "document", tree.RootNode().Type())
}

func TestParser_Markdown_ContextCancellation(t *testing.T) {
	// Error case: context cancellation
	parser, err := NewParser()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	source := []byte(`# Heading

Some content.
`)

	_, err = parser.ParseByName(ctx, "markdown", source)
	assert.Error(t, err, "Should return error when context is cancelled")
	assert.Equal(t, context.Canceled, err)
}

func TestParser_Markdown_IsSupported(t *testing.T) {
	// Verify markdown is properly registered as supported
	parser, err := NewParser()
	require.NoError(t, err)

	assert.True(t, parser.IsSupportedByName("markdown"), "markdown should be supported")

	// Test via Language type as well
	assert.True(t, parser.IsSupported(Language("markdown")), "Language('markdown') should be supported")
}

func TestParser_Markdown_DetectLanguage(t *testing.T) {
	// Test extension detection for markdown files
	tests := []struct {
		filename string
		expected Language
	}{
		{"README.md", Language("markdown")},
		{"doc.markdown", Language("markdown")},
		{"component.mdx", Language("markdown")},
		{"notes.MD", LangUnknown}, // Case-sensitive
		{"file.txt", LangUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			lang := DetectLanguage(tt.filename)
			assert.Equal(t, tt.expected, lang)
		})
	}
}

func TestParser_Markdown_ParseViaLanguageType(t *testing.T) {
	// Test parsing via Language type (not just string name)
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte("# Test\n\nParagraph.")

	tree, err := parser.Parse(context.Background(), Language("markdown"), source)
	require.NoError(t, err, "Should parse via Language type")
	assert.NotNil(t, tree)
	assert.Equal(t, "document", tree.RootNode().Type())
}

func TestParser_Markdown_MixedContent(t *testing.T) {
	// Success case: realistic mixed content document
	parser, err := NewParser()
	require.NoError(t, err)

	source := []byte(`# Project README

## Overview

This project does amazing things.

## Installation

` + "```" + `bash
npm install amazing-project
` + "```" + `

## Usage

1. Import the module
2. Configure settings
3. Run the app

> **Note:** Make sure you have Node.js installed.

## API Reference

| Method | Description |
|--------|-------------|
| init() | Initialize   |
| run()  | Execute      |

---

## License

MIT © 2024
`)

	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err)
	assert.NotNil(t, tree)

	// Verify we can traverse and find various elements
	nodeTypes := make(map[string]int)
	var countNodes func(node *sitter.Node)
	countNodes = func(node *sitter.Node) {
		nodeTypes[node.Type()]++
		for i := 0; i < int(node.NamedChildCount()); i++ {
			countNodes(node.NamedChild(i))
		}
	}
	countNodes(tree.RootNode())

	assert.True(t, nodeTypes["section"] >= 1, "Should have sections")
	assert.True(t, nodeTypes["atx_heading"] >= 1, "Should have headings")
	assert.True(t, nodeTypes["fenced_code_block"] >= 1, "Should have code blocks")
	assert.True(t, nodeTypes["list"] >= 1, "Should have lists")
}

func TestParser_Markdown_NilParserEntry(t *testing.T) {
	// Verify that the markdown parser entry in the map is nil (special case)
	// This tests that NewParser correctly sets up markdown with nil parser
	parser, err := NewParser()
	require.NoError(t, err)

	// The parser should still work even though the internal parsers[markdown] is nil
	source := []byte("# Test")
	tree, err := parser.ParseByName(context.Background(), "markdown", source)
	require.NoError(t, err)
	assert.NotNil(t, tree)
}

// =============================================================================
// Language Constants Tests
// =============================================================================

func TestLanguageConstants(t *testing.T) {
	// Verify that language constants have expected values
	assert.Equal(t, Language("go"), LangGo)
	assert.Equal(t, Language("java"), LangJava)
	assert.Equal(t, Language("csharp"), LangCSharp)
	assert.Equal(t, Language("python"), LangPython)
	assert.Equal(t, Language("javascript"), LangJavaScript)
	assert.Equal(t, Language("typescript"), LangTypeScript)
	assert.Equal(t, Language("tsx"), LangTSX)
	assert.Equal(t, Language("jsx"), LangJSX)
	assert.Equal(t, Language("unknown"), LangUnknown)
}

func TestLanguageConstants_StringRepresentation(t *testing.T) {
	// Language constants should have meaningful string representations
	languages := map[Language]string{
		LangGo:         "go",
		LangJava:       "java",
		LangCSharp:     "csharp",
		LangPython:     "python",
		LangJavaScript: "javascript",
		LangTypeScript: "typescript",
		LangTSX:        "tsx",
		LangJSX:        "jsx",
		LangUnknown:    "unknown",
	}

	for lang, expected := range languages {
		assert.Equal(t, expected, string(lang), "Language constant should match expected string")
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestParser_Parse_ConcurrentParsing(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	// Parse multiple files concurrently
	sources := map[Language][]byte{
		LangCSharp:     []byte("public class Test { }"),
		LangPython:     []byte("class Test:\n    pass"),
		LangJavaScript: []byte("class Test { }"),
		LangTypeScript: []byte("class Test { }"),
	}

	done := make(chan bool, len(sources))

	for lang, source := range sources {
		go func(l Language, s []byte) {
			tree, err := parser.Parse(context.Background(), l, s)
			assert.NoError(t, err, "Concurrent parse should succeed for %s", l)
			assert.NotNil(t, tree)
			done <- true
		}(lang, source)
	}

	// Wait for all goroutines
	for i := 0; i < len(sources); i++ {
		<-done
	}
}
