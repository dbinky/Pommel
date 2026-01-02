package chunker

import (
	"context"
	"testing"

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

	expectedLanguages := []Language{
		LangGo,
		LangJava,
		LangCSharp,
		LangPython,
		LangJavaScript,
		LangTypeScript,
		LangTSX,
		LangJSX,
		LangMarkdown,
	}

	for _, lang := range expectedLanguages {
		t.Run(string(lang), func(t *testing.T) {
			assert.True(t, parser.IsSupported(lang), "Parser should support %s", lang)
		})
	}
}

func TestParser_SupportedLanguages(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	langs := parser.SupportedLanguages()
	assert.Contains(t, langs, LangGo, "SupportedLanguages should include Go")
	assert.Contains(t, langs, LangJava, "SupportedLanguages should include Java")
	assert.Contains(t, langs, LangCSharp, "SupportedLanguages should include C#")
	assert.Contains(t, langs, LangPython, "SupportedLanguages should include Python")
	assert.Contains(t, langs, LangJavaScript, "SupportedLanguages should include JavaScript")
	assert.Contains(t, langs, LangTypeScript, "SupportedLanguages should include TypeScript")
	assert.Contains(t, langs, LangTSX, "SupportedLanguages should include TSX")
	assert.Contains(t, langs, LangJSX, "SupportedLanguages should include JSX")
	assert.Contains(t, langs, LangMarkdown, "SupportedLanguages should include Markdown")
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

func TestDetectLanguage_Markdown(t *testing.T) {
	tests := []struct {
		filename string
		expected Language
	}{
		{"file.md", LangMarkdown},
		{"README.md", LangMarkdown},
		{"docs/architecture.md", LangMarkdown},
		{"file.markdown", LangMarkdown},
		{"file.mdown", LangMarkdown},
		{"file.mkdn", LangMarkdown},
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
		{"file.rb", LangUnknown},
		{"file.rust", LangUnknown},
		{"file.c", LangUnknown},
		{"file.cpp", LangUnknown},
		{"file.h", LangUnknown},
		{"file.txt", LangUnknown},
		{"file.json", LangUnknown},
		{"file.yaml", LangUnknown},
		{"file", LangUnknown},       // No extension
		{".gitignore", LangUnknown}, // Hidden file
		{"Makefile", LangUnknown},   // No extension
		{"Dockerfile", LangUnknown}, // No extension
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
		Language("rust"),
		Language("ruby"),
		Language("cpp"),
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
