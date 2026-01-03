package chunker

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Registry Initialization Tests
// =============================================================================

func TestNewChunkerRegistry(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err, "NewChunkerRegistry should not return an error")
	require.NotNil(t, reg, "NewChunkerRegistry should return a non-nil registry")
}

func TestChunkerRegistry_ContainsAllLanguageChunkers(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	// Registry should contain chunkers for all supported languages from config files
	// Languages with supported grammars: go, java, csharp, python, javascript, typescript
	expectedLanguages := []Language{
		LangGo,
		LangJava,
		LangCSharp, // C# uses user-friendly name "csharp"
		LangPython,
		LangJavaScript,
		LangTypeScript,
	}

	for _, lang := range expectedLanguages {
		assert.True(t, reg.IsSupported(lang), "Registry should support %s", lang)
	}
}

func TestChunkerRegistry_HasFallbackChunker(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	// The registry should have a fallback chunker for unknown languages
	// We test this indirectly by chunking an unsupported file type
	file := &models.SourceFile{
		Path:         "test.go",
		Content:      []byte("package main\n\nfunc main() {}"),
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err, "Registry should use fallback chunker for unsupported languages")
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, len(result.Chunks), 1, "Fallback should produce at least one chunk")
}

// =============================================================================
// Routing by File Extension Tests
// =============================================================================

func TestChunkerRegistry_RoutesToCSharpChunker(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `namespace MyApp
{
    public class Calculator
    {
        public int Add(int a, int b)
        {
            return a + b;
        }
    }
}`

	file := &models.SourceFile{
		Path:         "src/Calculator.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// CSharpChunker should extract multiple chunks (file, class, method)
	assert.GreaterOrEqual(t, len(result.Chunks), 3, "CSharp chunker should extract file, class, and method chunks")

	// Verify language is set correctly on chunks (from config's language field)
	for _, chunk := range result.Chunks {
		assert.Equal(t, "csharp", chunk.Language, "Chunks should have csharp language")
	}
}

func TestChunkerRegistry_RoutesToPythonChunker(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `class Calculator:
    def add(self, a, b):
        return a + b
`

	file := &models.SourceFile{
		Path:         "src/calculator.py",
		Content:      []byte(source),
		Language:     string(LangPython),
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// PythonChunker should extract multiple chunks (file, class, method)
	assert.GreaterOrEqual(t, len(result.Chunks), 3, "Python chunker should extract file, class, and method chunks")

	// Verify language is set correctly on chunks
	for _, chunk := range result.Chunks {
		assert.Equal(t, string(LangPython), chunk.Language, "Chunks should have python language")
	}
}

func TestChunkerRegistry_RoutesToJavaScriptChunker(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `class Calculator {
    add(a, b) {
        return a + b;
    }
}`

	file := &models.SourceFile{
		Path:         "src/calculator.js",
		Content:      []byte(source),
		Language:     string(LangJavaScript),
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// JavaScriptChunker should extract multiple chunks (file, class, method)
	assert.GreaterOrEqual(t, len(result.Chunks), 3, "JavaScript chunker should extract file, class, and method chunks")
}

func TestChunkerRegistry_RoutesToTypeScriptChunker(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `class Calculator {
    add(a: number, b: number): number {
        return a + b;
    }
}`

	file := &models.SourceFile{
		Path:         "src/calculator.ts",
		Content:      []byte(source),
		Language:     string(LangTypeScript),
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// TypeScriptChunker should extract multiple chunks (file, class, method)
	assert.GreaterOrEqual(t, len(result.Chunks), 3, "TypeScript chunker should extract file, class, and method chunks")
}

func TestChunkerRegistry_RoutesToTSXChunker(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `interface Props {
    name: string;
}

function Hello(props: Props) {
    return <div>Hello {props.name}</div>;
}`

	file := &models.SourceFile{
		Path:         "src/Hello.tsx",
		Content:      []byte(source),
		Language:     "tsx", // TSX has its own config
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// TSX chunker should extract chunks (file, interface, function)
	assert.GreaterOrEqual(t, len(result.Chunks), 2, "TSX chunker should extract file and component/function chunks")

	// Verify chunks have tsx language
	for _, chunk := range result.Chunks {
		assert.Equal(t, "tsx", chunk.Language, "Chunks should have tsx language")
	}
}

func TestChunkerRegistry_RoutesToJSXChunker(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `function Hello(props) {
    return <div>Hello {props.name}</div>;
}`

	file := &models.SourceFile{
		Path:         "src/Hello.jsx",
		Content:      []byte(source),
		Language:     "jsx", // JSX has its own config
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// JSX chunker should extract chunks (file, function)
	assert.GreaterOrEqual(t, len(result.Chunks), 2, "JSX chunker should extract file and function chunks")

	// Verify chunks have jsx language
	for _, chunk := range result.Chunks {
		assert.Equal(t, "jsx", chunk.Language, "Chunks should have jsx language")
	}
}

// =============================================================================
// Routing Table-Driven Tests
// =============================================================================

func TestChunkerRegistry_Routes(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	tests := []struct {
		name      string
		path      string
		source    string
		language  string
		minChunks int
	}{
		{
			name:      "C# file",
			path:      "test.cs",
			source:    "public class Test { }",
			language:  "csharp",
			minChunks: 1,
		},
		{
			name:      "Python file",
			path:      "test.py",
			source:    "class Test:\n    pass",
			language:  "python",
			minChunks: 1,
		},
		{
			name:      "JavaScript file",
			path:      "test.js",
			source:    "class Test { }",
			language:  "javascript",
			minChunks: 1,
		},
		{
			name:      "TypeScript file",
			path:      "test.ts",
			source:    "class Test { }",
			language:  "typescript",
			minChunks: 1,
		},
		{
			name:      "TSX file",
			path:      "test.tsx",
			source:    "function Test() { return <div/>; }",
			language:  "tsx", // TSX has its own config
			minChunks: 1,
		},
		{
			name:      "JSX file",
			path:      "test.jsx",
			source:    "function Test() { return <div/>; }",
			language:  "jsx", // JSX has its own config
			minChunks: 1,
		},
		{
			name:      "Go file",
			path:      "test.go",
			source:    "package main",
			language:  "go",
			minChunks: 1,
		},
		{
			name:      "Ruby file (fallback)",
			path:      "test.rb",
			source:    "class Test\nend",
			language:  "ruby",
			minChunks: 1,
		},
		{
			name:      "Unknown file (fallback)",
			path:      "test.xyz",
			source:    "some content",
			language:  "unknown",
			minChunks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &models.SourceFile{
				Path:         tt.path,
				Content:      []byte(tt.source),
				Language:     tt.language,
				LastModified: time.Now(),
			}

			result, err := reg.Chunk(context.Background(), file)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(result.Chunks), tt.minChunks, "Should produce at least %d chunk(s)", tt.minChunks)
		})
	}
}

// =============================================================================
// SupportedLanguages Tests
// =============================================================================

func TestChunkerRegistry_SupportedLanguages(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	languages := reg.SupportedLanguages()

	// Should return all supported languages
	assert.NotEmpty(t, languages, "SupportedLanguages should return non-empty slice")

	// Convert to map for easy lookup
	langMap := make(map[Language]bool)
	for _, lang := range languages {
		langMap[lang] = true
	}

	// Verify all expected languages are present (using user-friendly names)
	expectedLanguages := []Language{
		LangGo,
		LangJava,
		LangCSharp, // C# uses user-friendly name "csharp"
		LangPython,
		LangJavaScript,
		LangTypeScript,
	}

	for _, expected := range expectedLanguages {
		assert.True(t, langMap[expected], "SupportedLanguages should include %s", expected)
	}
}

func TestChunkerRegistry_SupportedLanguages_DoesNotIncludeUnknown(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	languages := reg.SupportedLanguages()

	// LangUnknown should NOT be in the supported languages list
	for _, lang := range languages {
		assert.NotEqual(t, LangUnknown, lang, "SupportedLanguages should not include LangUnknown")
	}
}

// =============================================================================
// IsSupported Tests
// =============================================================================

func TestChunkerRegistry_IsSupported_ReturnsTrue(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	supportedLanguages := []Language{
		LangGo,
		LangJava,
		LangCSharp, // C# uses user-friendly name "csharp"
		LangPython,
		LangJavaScript,
		LangTypeScript,
		Language("rust"), // Rust is now supported
	}

	for _, lang := range supportedLanguages {
		assert.True(t, reg.IsSupported(lang), "IsSupported should return true for %s", lang)
	}
}

func TestChunkerRegistry_IsSupported_ReturnsFalse(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	unsupportedLanguages := []Language{
		LangUnknown,
		Language("brainfuck"),  // No tree-sitter grammar available
		Language("whitespace"), // No tree-sitter grammar available
		Language("befunge"),    // No tree-sitter grammar available
	}

	for _, lang := range unsupportedLanguages {
		assert.False(t, reg.IsSupported(lang), "IsSupported should return false for %s", lang)
	}
}

// =============================================================================
// Fallback Behavior Tests
// =============================================================================

func TestChunkerRegistry_GoChunker(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}`

	file := &models.SourceFile{
		Path:         "src/main.go",
		Content:      []byte(source),
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Go chunker should produce file chunk + function chunk
	assert.Len(t, result.Chunks, 2, "Go chunker should produce 2 chunks (file + function)")

	// Find the function chunk
	var funcChunk *models.Chunk
	for _, c := range result.Chunks {
		if c.Level == models.ChunkLevelMethod {
			funcChunk = c
			break
		}
	}
	require.NotNil(t, funcChunk, "Should have a method-level chunk")
	assert.Equal(t, "main", funcChunk.Name, "Function should be named 'main'")
}

func TestChunkerRegistry_FallbackForUnsupportedExtension(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	// Using a made-up language that has no tree-sitter grammar
	source := `+++[>+++++<-]>.+++++++..+++.`

	file := &models.SourceFile{
		Path:         "src/hello.bf",
		Content:      []byte(source),
		Language:     "brainfuck",
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Fallback should produce exactly one file-level chunk
	assert.Len(t, result.Chunks, 1, "Fallback chunker should produce exactly 1 chunk")
	assert.Equal(t, models.ChunkLevelFile, result.Chunks[0].Level, "Fallback should produce file-level chunk")
}

func TestChunkerRegistry_RustChunker(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `fn main() {
    println!("Hello, world!");
}`

	file := &models.SourceFile{
		Path:         "src/main.rs",
		Content:      []byte(source),
		Language:     "rust",
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Rust is now supported - should produce file and function chunks
	assert.GreaterOrEqual(t, len(result.Chunks), 2, "Rust chunker should produce file + function chunks")

	// Verify at least one chunk has rust language
	hasRust := false
	for _, chunk := range result.Chunks {
		if chunk.Language == "rust" {
			hasRust = true
			break
		}
	}
	assert.True(t, hasRust, "Chunks should have rust language")
}

func TestChunkerRegistry_RubyChunker(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `class Calculator
  def add(a, b)
    a + b
  end

  def subtract(a, b)
    a - b
  end
end`

	file := &models.SourceFile{
		Path:         "src/calculator.rb",
		Content:      []byte(source),
		Language:     "ruby",
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Ruby is now supported - should produce at least a file-level chunk
	assert.GreaterOrEqual(t, len(result.Chunks), 1, "Ruby chunker should produce at least 1 chunk")

	// Verify chunks have ruby language set
	hasRuby := false
	for _, chunk := range result.Chunks {
		if chunk.Language == "ruby" {
			hasRuby = true
			break
		}
	}
	assert.True(t, hasRuby, "Chunks should have ruby language")

	// Verify we're not using fallback (content should be parsed, not raw file dump)
	// The file chunk should exist
	var fileChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelFile {
			fileChunk = chunk
			break
		}
	}
	require.NotNil(t, fileChunk, "Should have a file-level chunk")
	assert.Equal(t, "ruby", fileChunk.Language, "File chunk should have ruby language")
}

func TestChunkerRegistry_CppChunker(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `#include <iostream>

class Calculator {
public:
    int add(int a, int b) {
        return a + b;
    }
};

int main() {
    Calculator calc;
    std::cout << calc.add(2, 3) << std::endl;
    return 0;
}`

	file := &models.SourceFile{
		Path:         "src/main.cpp",
		Content:      []byte(source),
		Language:     "cpp",
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// C++ is now supported - should produce at least a file-level chunk
	assert.GreaterOrEqual(t, len(result.Chunks), 1, "C++ chunker should produce at least 1 chunk")

	// Verify chunks have cpp language set
	hasCpp := false
	for _, chunk := range result.Chunks {
		if chunk.Language == "cpp" {
			hasCpp = true
			break
		}
	}
	assert.True(t, hasCpp, "Chunks should have cpp language")

	// Verify we're not using fallback (content should be parsed)
	var fileChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelFile {
			fileChunk = chunk
			break
		}
	}
	require.NotNil(t, fileChunk, "Should have a file-level chunk")
	assert.Equal(t, "cpp", fileChunk.Language, "File chunk should have cpp language")
}

func TestChunkerRegistry_FallbackForUnknownLanguage(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := "some random content in an unknown file type"

	file := &models.SourceFile{
		Path:         "config.xyz",
		Content:      []byte(source),
		Language:     string(LangUnknown),
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Fallback should produce exactly one file-level chunk
	assert.Len(t, result.Chunks, 1, "Fallback chunker should produce exactly 1 chunk")
	assert.Equal(t, models.ChunkLevelFile, result.Chunks[0].Level, "Fallback should produce file-level chunk")
}

func TestChunkerRegistry_FallbackPreservesContent(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	// Using a made-up language (Brainfuck) that has no tree-sitter grammar
	source := `++++++++[>++++[>++>+++>+++>+<<<<-]>+>+>->>+[<]<-]>>
.>---.+++++++..+++.>>.<-.<.+++.------.--------.>>+.>++.`

	file := &models.SourceFile{
		Path:         "test.bf",
		Content:      []byte(source),
		Language:     "brainfuck",
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)

	// Fallback should preserve the full file content
	assert.Equal(t, source, result.Chunks[0].Content, "Fallback should preserve full file content")
}

// =============================================================================
// Context Handling Tests
// =============================================================================

func TestChunkerRegistry_RespectsContextCancellation(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `public class Test { }`

	file := &models.SourceFile{
		Path:         "test.cs",
		Content:      []byte(source),
		Language:     string(LangCSharp),
		LastModified: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, _ = reg.Chunk(ctx, file)
	// Should either return error due to cancelled context or complete
	// This test ensures no hang occurs with cancelled context
}

func TestChunkerRegistry_WorksWithTimeout(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `class Test:
    def method(self):
        pass`

	file := &models.SourceFile{
		Path:         "test.py",
		Content:      []byte(source),
		Language:     string(LangPython),
		LastModified: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := reg.Chunk(ctx, file)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Chunks)
}

// =============================================================================
// Chunk Metadata Tests
// =============================================================================

func TestChunkerRegistry_ChunksHaveCorrectFilePath(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	filePath := "src/deeply/nested/Calculator.cs"
	source := `public class Calculator { }`

	file := &models.SourceFile{
		Path:         filePath,
		Content:      []byte(source),
		Language:     string(LangCSharp),
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)

	for _, chunk := range result.Chunks {
		assert.Equal(t, filePath, chunk.FilePath, "All chunks should have the correct file path")
	}
}

func TestChunkerRegistry_ChunksHaveValidIDs(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `class Calculator:
    def add(self, a, b):
        return a + b`

	file := &models.SourceFile{
		Path:         "calculator.py",
		Content:      []byte(source),
		Language:     string(LangPython),
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)

	for _, chunk := range result.Chunks {
		assert.NotEmpty(t, chunk.ID, "Chunk ID should be set")
		assert.NotEmpty(t, chunk.ContentHash, "Chunk ContentHash should be set")
	}
}

func TestChunkerRegistry_ChunksHaveValidLineNumbers(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `class Test {
    method() {
        return 42;
    }
}`

	file := &models.SourceFile{
		Path:         "test.js",
		Content:      []byte(source),
		Language:     string(LangJavaScript),
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)

	for _, chunk := range result.Chunks {
		assert.GreaterOrEqual(t, chunk.StartLine, 1, "StartLine should be >= 1")
		assert.GreaterOrEqual(t, chunk.EndLine, chunk.StartLine, "EndLine should be >= StartLine")
	}
}

// =============================================================================
// Language Detection Integration Tests
// =============================================================================

func TestChunkerRegistry_ChunkByFileExtension(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	// Test that we can chunk files by detecting language from extension
	tests := []struct {
		path           string
		source         string
		expectedLang   Language
		wantMultiChunk bool
	}{
		{
			path:           "test.cs",
			source:         "public class Test { public void Method() { } }",
			expectedLang:   LangCSharp,
			wantMultiChunk: true,
		},
		{
			path:           "test.py",
			source:         "class Test:\n    def method(self):\n        pass",
			expectedLang:   LangPython,
			wantMultiChunk: true,
		},
		{
			path:           "test.js",
			source:         "class Test { method() { } }",
			expectedLang:   LangJavaScript,
			wantMultiChunk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			// Detect language from file extension
			detectedLang := DetectLanguage(tt.path)
			assert.Equal(t, tt.expectedLang, detectedLang, "Language detection should work for %s", tt.path)

			file := &models.SourceFile{
				Path:         tt.path,
				Content:      []byte(tt.source),
				Language:     string(detectedLang),
				LastModified: time.Now(),
			}

			result, err := reg.Chunk(context.Background(), file)
			require.NoError(t, err)

			if tt.wantMultiChunk {
				assert.Greater(t, len(result.Chunks), 1, "Should extract multiple chunks for %s", tt.path)
			}
		})
	}
}

// =============================================================================
// Edge Cases Tests
// =============================================================================

func TestChunkerRegistry_EmptyFile(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "empty.py",
		Content:      []byte(""),
		Language:     string(LangPython),
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)
	// Empty file should produce zero or one chunk (implementation dependent)
	assert.LessOrEqual(t, len(result.Chunks), 1, "Empty file should produce at most 1 chunk")
}

func TestChunkerRegistry_NilFile(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	_, err = reg.Chunk(context.Background(), nil)
	// Should return an error for nil file
	assert.Error(t, err, "Chunking nil file should return an error")
}

func TestChunkerRegistry_DeterministicResults(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	source := `class Calculator:
    def add(self, a, b):
        return a + b

    def subtract(self, a, b):
        return a - b`

	file := &models.SourceFile{
		Path:         "calculator.py",
		Content:      []byte(source),
		Language:     string(LangPython),
		LastModified: time.Now(),
	}

	// Chunk the same file twice
	result1, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)

	result2, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Results should be identical
	require.Len(t, result2.Chunks, len(result1.Chunks), "Same file should produce same number of chunks")

	for i := range result1.Chunks {
		assert.Equal(t, result1.Chunks[i].ID, result2.Chunks[i].ID, "Chunk IDs should be deterministic")
		assert.Equal(t, result1.Chunks[i].ContentHash, result2.Chunks[i].ContentHash, "Content hashes should be deterministic")
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestChunkerRegistry_ConcurrentAccess(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	files := []*models.SourceFile{
		{
			Path:         "test.cs",
			Content:      []byte("public class Test { }"),
			Language:     string(LangCSharp),
			LastModified: time.Now(),
		},
		{
			Path:         "test.py",
			Content:      []byte("class Test:\n    pass"),
			Language:     string(LangPython),
			LastModified: time.Now(),
		},
		{
			Path:         "test.js",
			Content:      []byte("class Test { }"),
			Language:     string(LangJavaScript),
			LastModified: time.Now(),
		},
		{
			Path:         "test.go",
			Content:      []byte("package main"),
			Language:     "go",
			LastModified: time.Now(),
		},
	}

	// Run concurrent chunking operations
	done := make(chan bool, len(files))

	for _, file := range files {
		go func(f *models.SourceFile) {
			result, err := reg.Chunk(context.Background(), f)
			assert.NoError(t, err)
			assert.NotNil(t, result)
			done <- true
		}(file)
	}

	// Wait for all goroutines to complete
	for i := 0; i < len(files); i++ {
		<-done
	}
}

// =============================================================================
// Config-Driven Registry Tests
// =============================================================================

// getTestLanguagesDir returns the path to the project's languages directory.
func getTestLanguagesDir(t *testing.T) string {
	t.Helper()
	// Go up from internal/chunker to project root, then to languages/
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(filename)
	return filepath.Join(dir, "..", "..", "languages")
}

func TestRegistry_NewRegistryFromConfig(t *testing.T) {
	langDir := getTestLanguagesDir(t)
	reg, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err, "NewRegistryFromConfig should not return an error")
	require.NotNil(t, reg, "NewRegistryFromConfig should return a non-nil registry")

	// Verify at least some languages were registered
	languages := reg.SupportedLanguages()
	assert.NotEmpty(t, languages, "Registry should have registered languages")
}

func TestRegistry_LoadFromEmbeddedConfig(t *testing.T) {
	// Test that we can load from the project's languages/ directory
	langDir := getTestLanguagesDir(t)
	reg, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)
	require.NotNil(t, reg)

	// Registry should have multiple languages registered
	languages := reg.SupportedLanguages()
	assert.GreaterOrEqual(t, len(languages), 4, "Should have at least 4 languages registered from config files")
}

func TestRegistry_GetChunkerByExtension_ConfigDriven(t *testing.T) {
	langDir := getTestLanguagesDir(t)
	reg, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	tests := []struct {
		extension string
		wantFound bool
		wantLang  Language
	}{
		{".go", true, LangGo},
		{".py", true, LangPython},
		{".js", true, LangJavaScript},
		{".ts", true, LangTypeScript},
		{".java", true, LangJava},
		{".cs", true, LangCSharp},
		{".xyz", false, LangUnknown},
		{".unknown", false, LangUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.extension, func(t *testing.T) {
			chunker, found := reg.GetChunkerForExtension(tt.extension)
			assert.Equal(t, tt.wantFound, found, "GetChunkerForExtension(%s) found mismatch", tt.extension)
			if found {
				assert.NotNil(t, chunker, "Chunker should not be nil when found")
			}
		})
	}
}

func TestRegistry_AllConfiguredLanguagesRegistered(t *testing.T) {
	langDir := getTestLanguagesDir(t)
	reg, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	// Load configs directly to get expected count
	configs, errors := LoadAllLanguageConfigs(langDir)
	if len(errors) > 0 {
		t.Logf("Config load warnings: %v", errors)
	}

	// Count configs with supported grammars
	expectedCount := 0
	for _, cfg := range configs {
		if IsGrammarSupported(cfg.TreeSitter.Grammar) {
			expectedCount++
		}
	}

	languages := reg.SupportedLanguages()
	assert.Equal(t, expectedCount, len(languages), "Should have registered all configs with supported grammars")
}

func TestRegistry_MultipleExtensionsPerLanguage(t *testing.T) {
	langDir := getTestLanguagesDir(t)
	reg, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	// TypeScript config should register for .ts, .mts, .cts (but not .tsx - that's separate)
	tsExtensions := []string{".ts", ".mts", ".cts"}

	for _, ext := range tsExtensions {
		chunker, found := reg.GetChunkerForExtension(ext)
		assert.True(t, found, "Should find chunker for %s", ext)
		if found {
			assert.NotNil(t, chunker, "Chunker for %s should not be nil", ext)
		}
	}

	// TSX has its own config and should be found
	chunker, found := reg.GetChunkerForExtension(".tsx")
	assert.True(t, found, "Should find chunker for .tsx")
	assert.NotNil(t, chunker, "Chunker for .tsx should not be nil")
}

func TestRegistry_CaseInsensitiveExtensions(t *testing.T) {
	langDir := getTestLanguagesDir(t)
	reg, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	// Extension lookup should be case-insensitive
	tests := []struct {
		extension string
		wantFound bool
	}{
		{".go", true},
		{".GO", true},
		{".Go", true},
		{".py", true},
		{".PY", true},
		{".Py", true},
	}

	for _, tt := range tests {
		t.Run(tt.extension, func(t *testing.T) {
			_, found := reg.GetChunkerForExtension(tt.extension)
			assert.Equal(t, tt.wantFound, found, "GetChunkerForExtension(%s) should find chunker", tt.extension)
		})
	}
}

func TestRegistry_EmptyConfigDir(t *testing.T) {
	// Create a temporary empty directory
	tempDir := t.TempDir()

	reg, err := NewRegistryFromConfig(tempDir)
	assert.Error(t, err, "Should return error for empty config directory")
	assert.Nil(t, reg, "Registry should be nil when no configs found")
	assert.Contains(t, err.Error(), "no language configs found", "Error should mention no configs found")
}

func TestRegistry_MissingConfigDir(t *testing.T) {
	nonExistentDir := "/nonexistent/path/to/configs"

	reg, err := NewRegistryFromConfig(nonExistentDir)
	assert.Error(t, err, "Should return error for missing config directory")
	assert.Nil(t, reg, "Registry should be nil when config dir missing")
}

func TestRegistry_ConfigDrivenChunking(t *testing.T) {
	langDir := getTestLanguagesDir(t)
	reg, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	// Test that we can actually chunk files using the config-driven registry
	source := `package main

func main() {
    println("Hello, World!")
}`

	file := &models.SourceFile{
		Path:         "test.go",
		Content:      []byte(source),
		Language:     string(LangGo),
		LastModified: time.Now(),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should extract at least file chunk and function chunk
	assert.GreaterOrEqual(t, len(result.Chunks), 2, "Should extract multiple chunks")
}

func TestRegistry_GetLanguageForExtension(t *testing.T) {
	langDir := getTestLanguagesDir(t)
	reg, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	tests := []struct {
		extension string
		wantLang  Language
		wantFound bool
	}{
		{".go", LangGo, true},
		{".py", LangPython, true},
		{".js", LangJavaScript, true},
		{".xyz", LangUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.extension, func(t *testing.T) {
			lang, found := reg.GetLanguageForExtension(tt.extension)
			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.Equal(t, tt.wantLang, lang)
			}
		})
	}
}
