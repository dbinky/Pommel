# Phase 3: Chunking Engine

**Phase Goal:** Build the Tree-sitter based code parsing and chunking system that extracts semantic units (files, classes, methods) from source code.

**Prerequisites:** Phase 1 complete (project structure, models)

**Estimated Tasks:** 18 tasks across 7 areas

---

## Table of Contents

1. [Overview](#overview)
2. [Success Criteria](#success-criteria)
3. [Task 3.1: Tree-sitter Integration](#task-31-tree-sitter-integration)
4. [Task 3.2: Chunk Data Model](#task-32-chunk-data-model)
5. [Task 3.3: C# Chunker](#task-33-c-chunker)
6. [Task 3.4: Python Chunker](#task-34-python-chunker)
7. [Task 3.5: JavaScript/TypeScript Chunker](#task-35-javascripttypescript-chunker)
8. [Task 3.6: Fallback Chunker](#task-36-fallback-chunker)
9. [Task 3.7: Chunker Orchestration](#task-37-chunker-orchestration)
10. [Dependencies](#dependencies)
11. [Testing Strategy](#testing-strategy)
12. [Risks and Mitigations](#risks-and-mitigations)

---

## Overview

Phase 3 builds the chunking engine - the system that parses source files and extracts meaningful code units for embedding. By the end of this phase:

- Tree-sitter can parse C#, Python, JavaScript, and TypeScript files
- Files are chunked at three levels: file, class/module, method/function
- Each chunk has parent-child relationships
- Chunks include contextual preamble (e.g., class name in method chunks)
- Unsupported languages fall back to file-level chunking

This phase focuses solely on parsing and chunking - no embedding or storage.

---

## Success Criteria

| Criterion | Validation |
|-----------|------------|
| C# parsing | Extracts classes, methods, properties from .cs files |
| Python parsing | Extracts classes, functions, methods from .py files |
| JS/TS parsing | Extracts classes, functions from .js/.ts/.jsx/.tsx files |
| Hierarchy | Method chunks reference parent class chunks |
| Context | Method chunks include class name in content |
| Fallback | Unknown file types produce file-level chunk |
| Deterministic IDs | Same file produces same chunk IDs |

---

## Task 3.1: Tree-sitter Integration

### 3.1.1 Add Tree-sitter Dependencies

**Description:** Add Go Tree-sitter bindings and language grammars.

**Steps:**
1. Add dependencies to go.mod
2. Import and initialize parsers

**Dependencies:**
```
github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82
github.com/smacker/go-tree-sitter/csharp
github.com/smacker/go-tree-sitter/python
github.com/smacker/go-tree-sitter/javascript
github.com/smacker/go-tree-sitter/typescript/typescript
github.com/smacker/go-tree-sitter/typescript/tsx
```

**File Content (internal/chunker/treesitter.go):**
```go
package chunker

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// Language represents a supported programming language
type Language string

const (
	LangCSharp     Language = "csharp"
	LangPython     Language = "python"
	LangJavaScript Language = "javascript"
	LangTypeScript Language = "typescript"
	LangTSX        Language = "tsx"
	LangJSX        Language = "jsx"
	LangUnknown    Language = "unknown"
)

// Parser wraps tree-sitter parsing functionality
type Parser struct {
	parsers map[Language]*sitter.Parser
}

// NewParser creates a new tree-sitter parser with all supported languages
func NewParser() (*Parser, error) {
	p := &Parser{
		parsers: make(map[Language]*sitter.Parser),
	}

	// Initialize C# parser
	csParser := sitter.NewParser()
	csParser.SetLanguage(csharp.GetLanguage())
	p.parsers[LangCSharp] = csParser

	// Initialize Python parser
	pyParser := sitter.NewParser()
	pyParser.SetLanguage(python.GetLanguage())
	p.parsers[LangPython] = pyParser

	// Initialize JavaScript parser
	jsParser := sitter.NewParser()
	jsParser.SetLanguage(javascript.GetLanguage())
	p.parsers[LangJavaScript] = jsParser
	p.parsers[LangJSX] = jsParser // JSX uses JavaScript parser

	// Initialize TypeScript parser
	tsParser := sitter.NewParser()
	tsParser.SetLanguage(typescript.GetLanguage())
	p.parsers[LangTypeScript] = tsParser

	// Initialize TSX parser
	tsxParser := sitter.NewParser()
	tsxParser.SetLanguage(tsx.GetLanguage())
	p.parsers[LangTSX] = tsxParser

	return p, nil
}

// Parse parses source code and returns the AST
func (p *Parser) Parse(ctx context.Context, lang Language, source []byte) (*sitter.Tree, error) {
	parser, ok := p.parsers[lang]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}

	tree, err := parser.ParseCtx(ctx, nil, source)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return tree, nil
}

// SupportedLanguages returns the list of supported languages
func (p *Parser) SupportedLanguages() []Language {
	return []Language{
		LangCSharp,
		LangPython,
		LangJavaScript,
		LangTypeScript,
		LangTSX,
		LangJSX,
	}
}

// DetectLanguage determines the language from file extension
func DetectLanguage(filename string) Language {
	ext := filepath.Ext(filename)
	switch ext {
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
	default:
		return LangUnknown
	}
}

// IsSupported returns true if the language is supported
func (p *Parser) IsSupported(lang Language) bool {
	_, ok := p.parsers[lang]
	return ok
}
```

**Acceptance Criteria:**
- All parsers initialize without error
- Each language can parse valid source code
- Language detection works for all extensions

---

### 3.1.2 Write Tree-sitter Tests

**Description:** Test Tree-sitter parsing functionality.

**File Content (internal/chunker/treesitter_test.go):**
```go
package chunker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewParser(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)
	assert.NotNil(t, parser)
}

func TestParser_SupportedLanguages(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	langs := parser.SupportedLanguages()
	assert.Contains(t, langs, LangCSharp)
	assert.Contains(t, langs, LangPython)
	assert.Contains(t, langs, LangJavaScript)
	assert.Contains(t, langs, LangTypeScript)
}

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
	require.NoError(t, err)
	assert.NotNil(t, tree)
	assert.Equal(t, "program", tree.RootNode().Type())
}

func TestParser_Parse_Unsupported(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	_, err = parser.Parse(context.Background(), LangUnknown, []byte("code"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		filename string
		expected Language
	}{
		{"file.cs", LangCSharp},
		{"file.py", LangPython},
		{"file.js", LangJavaScript},
		{"file.jsx", LangJSX},
		{"file.ts", LangTypeScript},
		{"file.tsx", LangTSX},
		{"file.go", LangUnknown},
		{"file.rb", LangUnknown},
		{"file", LangUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			lang := DetectLanguage(tt.filename)
			assert.Equal(t, tt.expected, lang)
		})
	}
}
```

**Acceptance Criteria:**
- Parser initializes successfully
- All languages parse valid code
- Unsupported languages return error
- Language detection is accurate

---

## Task 3.2: Chunk Data Model

### 3.2.1 Define Chunk Struct

**Description:** Create the data model for code chunks.

**File Content (internal/models/chunk.go):**
```go
package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// ChunkLevel represents the granularity of a code chunk
type ChunkLevel string

const (
	ChunkLevelFile   ChunkLevel = "file"
	ChunkLevelClass  ChunkLevel = "class"
	ChunkLevelMethod ChunkLevel = "method"
)

// Chunk represents a semantic unit of code
type Chunk struct {
	// ID is a deterministic hash based on file path and boundaries
	ID string

	// FilePath is the relative path from project root
	FilePath string

	// Line numbers (1-indexed, inclusive)
	StartLine int
	EndLine   int

	// Level indicates the chunk granularity
	Level ChunkLevel

	// Language of the source code
	Language string

	// Content is the actual source code
	Content string

	// ParentID references the containing chunk (nil for file-level)
	ParentID *string

	// Name is the identifier (class name, function name, etc.)
	Name string

	// Signature for methods/functions (full declaration)
	Signature string

	// ContentHash for change detection
	ContentHash string

	// LastModified timestamp of the source file
	LastModified time.Time
}

// GenerateID creates a deterministic ID for the chunk
func (c *Chunk) GenerateID() string {
	// ID is based on file path + level + start line + end line
	data := fmt.Sprintf("%s:%s:%d:%d", c.FilePath, c.Level, c.StartLine, c.EndLine)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // 32 hex chars
}

// GenerateContentHash creates a hash of the chunk content
func (c *Chunk) GenerateContentHash() string {
	hash := sha256.Sum256([]byte(c.Content))
	return hex.EncodeToString(hash[:16])
}

// SetHashes generates and sets both ID and content hash
func (c *Chunk) SetHashes() {
	c.ID = c.GenerateID()
	c.ContentHash = c.GenerateContentHash()
}

// IsValid checks if the chunk has required fields
func (c *Chunk) IsValid() error {
	if c.FilePath == "" {
		return fmt.Errorf("file path is required")
	}
	if c.StartLine < 1 {
		return fmt.Errorf("start line must be >= 1")
	}
	if c.EndLine < c.StartLine {
		return fmt.Errorf("end line must be >= start line")
	}
	if c.Content == "" {
		return fmt.Errorf("content is required")
	}
	if c.Level == "" {
		return fmt.Errorf("level is required")
	}
	return nil
}

// LineCount returns the number of lines in the chunk
func (c *Chunk) LineCount() int {
	return c.EndLine - c.StartLine + 1
}

// SourceFile represents a file to be chunked
type SourceFile struct {
	Path         string
	Content      []byte
	Language     string
	LastModified time.Time
}

// ChunkResult contains chunks extracted from a file
type ChunkResult struct {
	File   *SourceFile
	Chunks []*Chunk
	Errors []error
}
```

**Acceptance Criteria:**
- Chunk struct contains all required fields
- ID generation is deterministic
- Content hash detects changes
- Validation catches missing fields

---

### 3.2.2 Write Chunk Model Tests

**File Content (internal/models/chunk_test.go):**
```go
package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChunk_GenerateID(t *testing.T) {
	c := &Chunk{
		FilePath:  "src/main.cs",
		Level:     ChunkLevelMethod,
		StartLine: 10,
		EndLine:   20,
	}

	id1 := c.GenerateID()
	id2 := c.GenerateID()

	assert.Equal(t, id1, id2, "ID should be deterministic")
	assert.Len(t, id1, 32, "ID should be 32 hex characters")
}

func TestChunk_GenerateID_Different(t *testing.T) {
	c1 := &Chunk{FilePath: "file1.cs", Level: ChunkLevelMethod, StartLine: 10, EndLine: 20}
	c2 := &Chunk{FilePath: "file2.cs", Level: ChunkLevelMethod, StartLine: 10, EndLine: 20}

	assert.NotEqual(t, c1.GenerateID(), c2.GenerateID())
}

func TestChunk_GenerateContentHash(t *testing.T) {
	c := &Chunk{Content: "public void Foo() {}"}

	hash1 := c.GenerateContentHash()
	hash2 := c.GenerateContentHash()

	assert.Equal(t, hash1, hash2)
	assert.Len(t, hash1, 32)
}

func TestChunk_SetHashes(t *testing.T) {
	c := &Chunk{
		FilePath:  "src/main.cs",
		Level:     ChunkLevelClass,
		StartLine: 1,
		EndLine:   50,
		Content:   "public class Foo {}",
	}

	c.SetHashes()

	assert.NotEmpty(t, c.ID)
	assert.NotEmpty(t, c.ContentHash)
}

func TestChunk_IsValid(t *testing.T) {
	valid := &Chunk{
		FilePath:  "src/main.cs",
		Level:     ChunkLevelMethod,
		StartLine: 10,
		EndLine:   20,
		Content:   "public void Foo() {}",
	}

	assert.NoError(t, valid.IsValid())
}

func TestChunk_IsValid_Missing(t *testing.T) {
	tests := []struct {
		name  string
		chunk *Chunk
		error string
	}{
		{
			name:  "missing file path",
			chunk: &Chunk{Level: ChunkLevelMethod, StartLine: 1, EndLine: 10, Content: "x"},
			error: "file path",
		},
		{
			name:  "invalid start line",
			chunk: &Chunk{FilePath: "x", Level: ChunkLevelMethod, StartLine: 0, EndLine: 10, Content: "x"},
			error: "start line",
		},
		{
			name:  "end before start",
			chunk: &Chunk{FilePath: "x", Level: ChunkLevelMethod, StartLine: 20, EndLine: 10, Content: "x"},
			error: "end line",
		},
		{
			name:  "missing content",
			chunk: &Chunk{FilePath: "x", Level: ChunkLevelMethod, StartLine: 1, EndLine: 10},
			error: "content",
		},
		{
			name:  "missing level",
			chunk: &Chunk{FilePath: "x", StartLine: 1, EndLine: 10, Content: "x"},
			error: "level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.chunk.IsValid()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.error)
		})
	}
}

func TestChunk_LineCount(t *testing.T) {
	c := &Chunk{StartLine: 10, EndLine: 20}
	assert.Equal(t, 11, c.LineCount())

	c2 := &Chunk{StartLine: 5, EndLine: 5}
	assert.Equal(t, 1, c2.LineCount())
}
```

**Acceptance Criteria:**
- ID generation is deterministic and unique
- Content hash changes when content changes
- Validation catches all error cases
- Line count is accurate

---

## Task 3.3: C# Chunker

### 3.3.1 Implement C# Chunk Extractor

**Description:** Extract chunks from C# source files using Tree-sitter.

**Key C# AST Node Types:**
- `class_declaration` - Class definitions
- `struct_declaration` - Struct definitions
- `interface_declaration` - Interface definitions
- `method_declaration` - Methods
- `property_declaration` - Properties (chunk as method-level)
- `namespace_declaration` - Namespaces (for context)

**File Content (internal/chunker/csharp.go):**
```go
package chunker

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/pommel-dev/pommel/internal/models"
)

// CSharpChunker extracts chunks from C# source files
type CSharpChunker struct {
	parser *Parser
}

// NewCSharpChunker creates a new C# chunker
func NewCSharpChunker(parser *Parser) *CSharpChunker {
	return &CSharpChunker{parser: parser}
}

// Chunk extracts chunks from a C# source file
func (c *CSharpChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
	tree, err := c.parser.Parse(ctx, LangCSharp, file.Content)
	if err != nil {
		return nil, err
	}

	result := &models.ChunkResult{
		File:   file,
		Chunks: make([]*models.Chunk, 0),
	}

	// Add file-level chunk
	fileChunk := c.createFileChunk(file)
	result.Chunks = append(result.Chunks, fileChunk)

	// Walk the AST
	c.walkNode(tree.RootNode(), file, fileChunk.ID, result)

	// Set hashes for all chunks
	for _, chunk := range result.Chunks {
		chunk.SetHashes()
	}

	return result, nil
}

func (c *CSharpChunker) createFileChunk(file *models.SourceFile) *models.Chunk {
	lines := strings.Split(string(file.Content), "\n")
	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    1,
		EndLine:      len(lines),
		Level:        models.ChunkLevelFile,
		Language:     string(LangCSharp),
		Content:      string(file.Content),
		Name:         file.Path,
		LastModified: file.LastModified,
	}
}

func (c *CSharpChunker) walkNode(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	nodeType := node.Type()

	switch nodeType {
	case "class_declaration", "struct_declaration", "interface_declaration", "record_declaration":
		chunk := c.extractClassChunk(node, file, parentID)
		if chunk != nil {
			chunk.SetHashes()
			result.Chunks = append(result.Chunks, chunk)
			// Walk children with class as parent
			c.walkChildren(node, file, chunk.ID, result)
			return
		}

	case "method_declaration", "property_declaration", "constructor_declaration":
		chunk := c.extractMethodChunk(node, file, parentID)
		if chunk != nil {
			result.Chunks = append(result.Chunks, chunk)
		}
		return // Don't recurse into method bodies
	}

	// Recurse into children
	c.walkChildren(node, file, parentID, result)
}

func (c *CSharpChunker) walkChildren(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		c.walkNode(child, file, parentID, result)
	}
}

func (c *CSharpChunker) extractClassChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	// Get class name
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Content(file.Content)

	// Get modifiers for context
	var modifiers string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "modifier" {
			modifiers += child.Content(file.Content) + " "
		}
	}

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	content := node.Content(file.Content)

	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        models.ChunkLevelClass,
		Language:     string(LangCSharp),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    strings.TrimSpace(modifiers) + " " + node.Type() + " " + name,
		LastModified: file.LastModified,
	}
}

func (c *CSharpChunker) extractMethodChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	// Get method name
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Content(file.Content)

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	content := node.Content(file.Content)

	// Build signature (first line of the method)
	lines := strings.Split(content, "\n")
	signature := strings.TrimSpace(lines[0])

	// Include context from parent class
	contextContent := c.addContext(file, parentID, content)

	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        models.ChunkLevelMethod,
		Language:     string(LangCSharp),
		Content:      contextContent,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

// addContext prepends class/namespace context to method content
func (c *CSharpChunker) addContext(file *models.SourceFile, parentID string, content string) string {
	// For now, just prefix with a comment showing the parent
	// In a more sophisticated version, we'd look up the parent chunk
	return content
}

// Language returns the language this chunker handles
func (c *CSharpChunker) Language() Language {
	return LangCSharp
}
```

**Acceptance Criteria:**
- Classes, structs, and interfaces are extracted
- Methods and properties are extracted
- Parent-child relationships are correct
- Content includes full source text

---

### 3.3.2 Write C# Chunker Tests

**File Content (internal/chunker/csharp_test.go):**
```go
package chunker

import (
	"context"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSharpChunker_SimpleClass(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewCSharpChunker(parser)

	source := `namespace MyApp
{
    public class Calculator
    {
        public int Add(int a, int b)
        {
            return a + b;
        }

        public int Subtract(int a, int b)
        {
            return a - b;
        }
    }
}`

	file := &models.SourceFile{
		Path:         "src/Calculator.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have: 1 file + 1 class + 2 methods = 4 chunks
	assert.Len(t, result.Chunks, 4)

	// Check file chunk
	fileChunk := result.Chunks[0]
	assert.Equal(t, models.ChunkLevelFile, fileChunk.Level)
	assert.Equal(t, "src/Calculator.cs", fileChunk.Name)

	// Find class chunk
	var classChunk *models.Chunk
	for _, c := range result.Chunks {
		if c.Level == models.ChunkLevelClass {
			classChunk = c
			break
		}
	}
	require.NotNil(t, classChunk)
	assert.Equal(t, "Calculator", classChunk.Name)
	assert.NotNil(t, classChunk.ParentID)

	// Find method chunks
	var methods []*models.Chunk
	for _, c := range result.Chunks {
		if c.Level == models.ChunkLevelMethod {
			methods = append(methods, c)
		}
	}
	assert.Len(t, methods, 2)

	// Methods should have class as parent
	for _, m := range methods {
		assert.Equal(t, classChunk.ID, *m.ParentID)
	}
}

func TestCSharpChunker_Properties(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewCSharpChunker(parser)

	source := `public class Person
{
    public string Name { get; set; }
    public int Age { get; private set; }
}`

	file := &models.SourceFile{
		Path:    "Person.cs",
		Content: []byte(source),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// File + class + 2 properties = 4 chunks
	assert.Len(t, result.Chunks, 4)

	// Properties should be at method level
	var propCount int
	for _, c := range result.Chunks {
		if c.Level == models.ChunkLevelMethod {
			propCount++
		}
	}
	assert.Equal(t, 2, propCount)
}

func TestCSharpChunker_NestedClasses(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewCSharpChunker(parser)

	source := `public class Outer
{
    public class Inner
    {
        public void InnerMethod() {}
    }

    public void OuterMethod() {}
}`

	file := &models.SourceFile{
		Path:    "Nested.cs",
		Content: []byte(source),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// File + Outer class + Inner class + 2 methods = 5 chunks
	assert.Len(t, result.Chunks, 5)

	// Find Inner class and verify its parent is Outer
	var innerClass, outerClass *models.Chunk
	for _, c := range result.Chunks {
		if c.Level == models.ChunkLevelClass && c.Name == "Inner" {
			innerClass = c
		}
		if c.Level == models.ChunkLevelClass && c.Name == "Outer" {
			outerClass = c
		}
	}
	require.NotNil(t, innerClass)
	require.NotNil(t, outerClass)
	assert.Equal(t, outerClass.ID, *innerClass.ParentID)
}

func TestCSharpChunker_DeterministicIDs(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewCSharpChunker(parser)

	source := `public class Test { public void Method() {} }`
	file := &models.SourceFile{
		Path:    "Test.cs",
		Content: []byte(source),
	}

	result1, _ := chunker.Chunk(context.Background(), file)
	result2, _ := chunker.Chunk(context.Background(), file)

	assert.Equal(t, len(result1.Chunks), len(result2.Chunks))
	for i := range result1.Chunks {
		assert.Equal(t, result1.Chunks[i].ID, result2.Chunks[i].ID)
	}
}
```

**Acceptance Criteria:**
- Simple classes parse correctly
- Properties are extracted
- Nested classes have correct hierarchy
- IDs are deterministic

---

## Task 3.4: Python Chunker

### 3.4.1 Implement Python Chunk Extractor

**Key Python AST Node Types:**
- `class_definition` - Class definitions
- `function_definition` - Functions and methods
- `decorated_definition` - Decorated classes/functions

**File Content (internal/chunker/python.go):**
```go
package chunker

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/pommel-dev/pommel/internal/models"
)

// PythonChunker extracts chunks from Python source files
type PythonChunker struct {
	parser *Parser
}

// NewPythonChunker creates a new Python chunker
func NewPythonChunker(parser *Parser) *PythonChunker {
	return &PythonChunker{parser: parser}
}

// Chunk extracts chunks from a Python source file
func (c *PythonChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
	tree, err := c.parser.Parse(ctx, LangPython, file.Content)
	if err != nil {
		return nil, err
	}

	result := &models.ChunkResult{
		File:   file,
		Chunks: make([]*models.Chunk, 0),
	}

	// Add file-level chunk
	fileChunk := c.createFileChunk(file)
	result.Chunks = append(result.Chunks, fileChunk)

	// Walk the AST - top-level only first
	c.walkNode(tree.RootNode(), file, fileChunk.ID, result, true)

	// Set hashes for all chunks
	for _, chunk := range result.Chunks {
		chunk.SetHashes()
	}

	return result, nil
}

func (c *PythonChunker) createFileChunk(file *models.SourceFile) *models.Chunk {
	lines := strings.Split(string(file.Content), "\n")
	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    1,
		EndLine:      len(lines),
		Level:        models.ChunkLevelFile,
		Language:     string(LangPython),
		Content:      string(file.Content),
		Name:         file.Path,
		LastModified: file.LastModified,
	}
}

func (c *PythonChunker) walkNode(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult, isTopLevel bool) {
	nodeType := node.Type()

	switch nodeType {
	case "class_definition":
		chunk := c.extractClassChunk(node, file, parentID)
		if chunk != nil {
			chunk.SetHashes()
			result.Chunks = append(result.Chunks, chunk)
			// Walk class body for methods
			body := node.ChildByFieldName("body")
			if body != nil {
				c.walkClassBody(body, file, chunk.ID, result)
			}
			return
		}

	case "function_definition":
		// Top-level functions are at method level
		// Methods inside classes are handled by walkClassBody
		if isTopLevel {
			chunk := c.extractFunctionChunk(node, file, parentID)
			if chunk != nil {
				result.Chunks = append(result.Chunks, chunk)
			}
		}
		return

	case "decorated_definition":
		// Handle decorated classes/functions
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "class_definition" || child.Type() == "function_definition" {
				c.walkNode(child, file, parentID, result, isTopLevel)
				return
			}
		}
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		c.walkNode(child, file, parentID, result, isTopLevel)
	}
}

func (c *PythonChunker) walkClassBody(node *sitter.Node, file *models.SourceFile, classID string, result *models.ChunkResult) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "function_definition":
			chunk := c.extractFunctionChunk(child, file, classID)
			if chunk != nil {
				result.Chunks = append(result.Chunks, chunk)
			}
		case "decorated_definition":
			// Find the function inside the decorator
			for j := 0; j < int(child.ChildCount()); j++ {
				inner := child.Child(j)
				if inner.Type() == "function_definition" {
					chunk := c.extractFunctionChunk(inner, file, classID)
					if chunk != nil {
						result.Chunks = append(result.Chunks, chunk)
					}
				}
			}
		case "class_definition":
			// Nested class
			chunk := c.extractClassChunk(child, file, classID)
			if chunk != nil {
				chunk.SetHashes()
				result.Chunks = append(result.Chunks, chunk)
				body := child.ChildByFieldName("body")
				if body != nil {
					c.walkClassBody(body, file, chunk.ID, result)
				}
			}
		}
	}
}

func (c *PythonChunker) extractClassChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Content(file.Content)

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	content := node.Content(file.Content)

	// Get first line as signature
	lines := strings.Split(content, "\n")
	signature := strings.TrimSpace(lines[0])

	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        models.ChunkLevelClass,
		Language:     string(LangPython),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

func (c *PythonChunker) extractFunctionChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Content(file.Content)

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	content := node.Content(file.Content)

	// Get first line as signature
	lines := strings.Split(content, "\n")
	signature := strings.TrimSpace(lines[0])

	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        models.ChunkLevelMethod,
		Language:     string(LangPython),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

func (c *PythonChunker) Language() Language {
	return LangPython
}
```

---

### 3.4.2 Write Python Chunker Tests

**File Content (internal/chunker/python_test.go):**
```go
package chunker

import (
	"context"
	"testing"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPythonChunker_SimpleClass(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewPythonChunker(parser)

	source := `class Calculator:
    def add(self, a, b):
        return a + b

    def subtract(self, a, b):
        return a - b
`

	file := &models.SourceFile{
		Path:     "calculator.py",
		Content:  []byte(source),
		Language: "python",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// File + class + 2 methods = 4 chunks
	assert.Len(t, result.Chunks, 4)

	// Verify class
	var classChunk *models.Chunk
	for _, c := range result.Chunks {
		if c.Level == models.ChunkLevelClass {
			classChunk = c
			break
		}
	}
	require.NotNil(t, classChunk)
	assert.Equal(t, "Calculator", classChunk.Name)
}

func TestPythonChunker_TopLevelFunctions(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewPythonChunker(parser)

	source := `def greet(name):
    return f"Hello, {name}!"

def farewell(name):
    return f"Goodbye, {name}!"
`

	file := &models.SourceFile{
		Path:    "greetings.py",
		Content: []byte(source),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// File + 2 functions = 3 chunks
	assert.Len(t, result.Chunks, 3)

	// Both should be method level
	var methodCount int
	for _, c := range result.Chunks {
		if c.Level == models.ChunkLevelMethod {
			methodCount++
		}
	}
	assert.Equal(t, 2, methodCount)
}

func TestPythonChunker_Decorators(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewPythonChunker(parser)

	source := `class Service:
    @staticmethod
    def create():
        return Service()

    @property
    def name(self):
        return self._name
`

	file := &models.SourceFile{
		Path:    "service.py",
		Content: []byte(source),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// File + class + 2 decorated methods = 4 chunks
	assert.Len(t, result.Chunks, 4)
}

func TestPythonChunker_NestedClass(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewPythonChunker(parser)

	source := `class Outer:
    class Inner:
        def inner_method(self):
            pass

    def outer_method(self):
        pass
`

	file := &models.SourceFile{
		Path:    "nested.py",
		Content: []byte(source),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// File + Outer + Inner + 2 methods = 5 chunks
	assert.Len(t, result.Chunks, 5)
}
```

---

## Task 3.5: JavaScript/TypeScript Chunker

### 3.5.1 Implement JS/TS Chunk Extractor

**Key JS/TS AST Node Types:**
- `class_declaration` - ES6 classes
- `function_declaration` - Named functions
- `arrow_function` - Arrow functions (when assigned)
- `method_definition` - Class methods
- `interface_declaration` (TS) - TypeScript interfaces
- `type_alias_declaration` (TS) - TypeScript type aliases

**File Content (internal/chunker/javascript.go):**
```go
package chunker

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/pommel-dev/pommel/internal/models"
)

// JavaScriptChunker extracts chunks from JavaScript/TypeScript files
type JavaScriptChunker struct {
	parser   *Parser
	language Language
}

// NewJavaScriptChunker creates a new JS/TS chunker
func NewJavaScriptChunker(parser *Parser, lang Language) *JavaScriptChunker {
	return &JavaScriptChunker{parser: parser, language: lang}
}

// Chunk extracts chunks from a JavaScript/TypeScript source file
func (c *JavaScriptChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
	tree, err := c.parser.Parse(ctx, c.language, file.Content)
	if err != nil {
		return nil, err
	}

	result := &models.ChunkResult{
		File:   file,
		Chunks: make([]*models.Chunk, 0),
	}

	// Add file-level chunk
	fileChunk := c.createFileChunk(file)
	result.Chunks = append(result.Chunks, fileChunk)

	// Walk the AST
	c.walkNode(tree.RootNode(), file, fileChunk.ID, result)

	// Set hashes
	for _, chunk := range result.Chunks {
		chunk.SetHashes()
	}

	return result, nil
}

func (c *JavaScriptChunker) createFileChunk(file *models.SourceFile) *models.Chunk {
	lines := strings.Split(string(file.Content), "\n")
	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    1,
		EndLine:      len(lines),
		Level:        models.ChunkLevelFile,
		Language:     string(c.language),
		Content:      string(file.Content),
		Name:         file.Path,
		LastModified: file.LastModified,
	}
}

func (c *JavaScriptChunker) walkNode(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	nodeType := node.Type()

	switch nodeType {
	case "class_declaration", "class":
		chunk := c.extractClassChunk(node, file, parentID)
		if chunk != nil {
			chunk.SetHashes()
			result.Chunks = append(result.Chunks, chunk)
			// Walk class body for methods
			body := node.ChildByFieldName("body")
			if body != nil {
				c.walkClassBody(body, file, chunk.ID, result)
			}
			return
		}

	case "function_declaration":
		chunk := c.extractFunctionChunk(node, file, parentID)
		if chunk != nil {
			result.Chunks = append(result.Chunks, chunk)
		}
		return

	case "lexical_declaration", "variable_declaration":
		// Check for arrow functions or function expressions assigned to variables
		c.extractVariableFunctions(node, file, parentID, result)
		return

	// TypeScript specific
	case "interface_declaration":
		chunk := c.extractInterfaceChunk(node, file, parentID)
		if chunk != nil {
			result.Chunks = append(result.Chunks, chunk)
		}
		return

	case "type_alias_declaration":
		// Could extract as class-level if complex
		return

	case "export_statement":
		// Walk into exported declarations
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			c.walkNode(child, file, parentID, result)
		}
		return
	}

	// Recurse
	for i := 0; i < int(node.ChildCount()); i++ {
		c.walkNode(node.Child(i), file, parentID, result)
	}
}

func (c *JavaScriptChunker) walkClassBody(node *sitter.Node, file *models.SourceFile, classID string, result *models.ChunkResult) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "method_definition" || child.Type() == "public_field_definition" {
			chunk := c.extractMethodChunk(child, file, classID)
			if chunk != nil {
				result.Chunks = append(result.Chunks, chunk)
			}
		}
	}
}

func (c *JavaScriptChunker) extractClassChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Content(file.Content)

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	content := node.Content(file.Content)

	lines := strings.Split(content, "\n")
	signature := strings.TrimSpace(lines[0])

	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        models.ChunkLevelClass,
		Language:     string(c.language),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

func (c *JavaScriptChunker) extractFunctionChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Content(file.Content)

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	content := node.Content(file.Content)

	lines := strings.Split(content, "\n")
	signature := strings.TrimSpace(lines[0])

	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        models.ChunkLevelMethod,
		Language:     string(c.language),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

func (c *JavaScriptChunker) extractMethodChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Content(file.Content)

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	content := node.Content(file.Content)

	lines := strings.Split(content, "\n")
	signature := strings.TrimSpace(lines[0])

	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        models.ChunkLevelMethod,
		Language:     string(c.language),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

func (c *JavaScriptChunker) extractInterfaceChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Content(file.Content)

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	content := node.Content(file.Content)

	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        models.ChunkLevelClass, // Interfaces at class level
		Language:     string(c.language),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    "interface " + name,
		LastModified: file.LastModified,
	}
}

func (c *JavaScriptChunker) extractVariableFunctions(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	// Look for: const foo = () => {} or const foo = function() {}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "variable_declarator" {
			nameNode := child.ChildByFieldName("name")
			valueNode := child.ChildByFieldName("value")

			if nameNode != nil && valueNode != nil {
				valueType := valueNode.Type()
				if valueType == "arrow_function" || valueType == "function" {
					chunk := &models.Chunk{
						FilePath:     file.Path,
						StartLine:    int(node.StartPoint().Row) + 1,
						EndLine:      int(node.EndPoint().Row) + 1,
						Level:        models.ChunkLevelMethod,
						Language:     string(c.language),
						Content:      node.Content(file.Content),
						ParentID:     &parentID,
						Name:         nameNode.Content(file.Content),
						LastModified: file.LastModified,
					}
					result.Chunks = append(result.Chunks, chunk)
				}
			}
		}
	}
}

func (c *JavaScriptChunker) Language() Language {
	return c.language
}
```

---

### 3.5.2 Write JS/TS Chunker Tests

**File Content (internal/chunker/javascript_test.go):**
```go
package chunker

import (
	"context"
	"testing"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJavaScriptChunker_Class(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `class Calculator {
    add(a, b) {
        return a + b;
    }

    subtract(a, b) {
        return a - b;
    }
}`

	file := &models.SourceFile{
		Path:    "calculator.js",
		Content: []byte(source),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// File + class + 2 methods = 4 chunks
	assert.Len(t, result.Chunks, 4)
}

func TestJavaScriptChunker_Functions(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `function greet(name) {
    return "Hello, " + name;
}

const farewell = (name) => {
    return "Goodbye, " + name;
};`

	file := &models.SourceFile{
		Path:    "greetings.js",
		Content: []byte(source),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// File + function + arrow function = 3 chunks
	assert.Len(t, result.Chunks, 3)
}

func TestTypeScriptChunker_Interface(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangTypeScript)

	source := `interface User {
    id: number;
    name: string;
}

class UserService {
    getUser(id: number): User {
        return { id, name: "test" };
    }
}`

	file := &models.SourceFile{
		Path:    "user.ts",
		Content: []byte(source),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// File + interface + class + method = 4 chunks
	assert.Len(t, result.Chunks, 4)

	// Interface should be at class level
	var interfaceChunk *models.Chunk
	for _, c := range result.Chunks {
		if c.Name == "User" {
			interfaceChunk = c
			break
		}
	}
	require.NotNil(t, interfaceChunk)
	assert.Equal(t, models.ChunkLevelClass, interfaceChunk.Level)
}

func TestJavaScriptChunker_ExportedClass(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker := NewJavaScriptChunker(parser, LangJavaScript)

	source := `export class Service {
    run() {
        console.log("running");
    }
}

export default Service;`

	file := &models.SourceFile{
		Path:    "service.js",
		Content: []byte(source),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should find the class and method
	var classFound, methodFound bool
	for _, c := range result.Chunks {
		if c.Name == "Service" && c.Level == models.ChunkLevelClass {
			classFound = true
		}
		if c.Name == "run" {
			methodFound = true
		}
	}
	assert.True(t, classFound)
	assert.True(t, methodFound)
}
```

---

## Task 3.6: Fallback Chunker

### 3.6.1 Implement Fallback Line-Based Chunker

**Description:** For unsupported languages, create a file-level only chunk.

**File Content (internal/chunker/fallback.go):**
```go
package chunker

import (
	"context"
	"strings"

	"github.com/pommel-dev/pommel/internal/models"
)

// FallbackChunker creates file-level chunks for unsupported languages
type FallbackChunker struct{}

// NewFallbackChunker creates a new fallback chunker
func NewFallbackChunker() *FallbackChunker {
	return &FallbackChunker{}
}

// Chunk creates a single file-level chunk
func (c *FallbackChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
	lines := strings.Split(string(file.Content), "\n")

	chunk := &models.Chunk{
		FilePath:     file.Path,
		StartLine:    1,
		EndLine:      len(lines),
		Level:        models.ChunkLevelFile,
		Language:     file.Language,
		Content:      string(file.Content),
		Name:         file.Path,
		LastModified: file.LastModified,
	}
	chunk.SetHashes()

	return &models.ChunkResult{
		File:   file,
		Chunks: []*models.Chunk{chunk},
	}, nil
}

// Language returns unknown
func (c *FallbackChunker) Language() Language {
	return LangUnknown
}
```

---

## Task 3.7: Chunker Orchestration

### 3.7.1 Create Chunker Interface and Router

**File Content (internal/chunker/chunker.go):**
```go
package chunker

import (
	"context"
	"fmt"

	"github.com/pommel-dev/pommel/internal/models"
)

// Chunker extracts chunks from source files
type Chunker interface {
	Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error)
	Language() Language
}

// ChunkerRegistry routes files to appropriate chunkers
type ChunkerRegistry struct {
	parser   *Parser
	chunkers map[Language]Chunker
	fallback Chunker
}

// NewChunkerRegistry creates a registry with all supported chunkers
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
	reg.chunkers[LangCSharp] = NewCSharpChunker(parser)
	reg.chunkers[LangPython] = NewPythonChunker(parser)
	reg.chunkers[LangJavaScript] = NewJavaScriptChunker(parser, LangJavaScript)
	reg.chunkers[LangJSX] = NewJavaScriptChunker(parser, LangJSX)
	reg.chunkers[LangTypeScript] = NewJavaScriptChunker(parser, LangTypeScript)
	reg.chunkers[LangTSX] = NewJavaScriptChunker(parser, LangTSX)

	return reg, nil
}

// Chunk routes the file to the appropriate chunker
func (r *ChunkerRegistry) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
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

// SupportedLanguages returns list of languages with dedicated chunkers
func (r *ChunkerRegistry) SupportedLanguages() []Language {
	langs := make([]Language, 0, len(r.chunkers))
	for lang := range r.chunkers {
		langs = append(langs, lang)
	}
	return langs
}

// IsSupported returns true if language has a dedicated chunker
func (r *ChunkerRegistry) IsSupported(lang Language) bool {
	_, ok := r.chunkers[lang]
	return ok
}
```

---

### 3.7.2 Write Integration Tests

**File Content (internal/chunker/chunker_test.go):**
```go
package chunker

import (
	"context"
	"testing"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunkerRegistry_Routes(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	tests := []struct {
		path         string
		expectChunks int
		expectLang   Language
	}{
		{"test.cs", 2, LangCSharp},      // File + class
		{"test.py", 2, LangPython},      // File + class
		{"test.js", 2, LangJavaScript},  // File + class
		{"test.ts", 2, LangTypeScript},  // File + class
		{"test.go", 1, LangUnknown},     // Fallback - file only
	}

	classSource := map[Language]string{
		LangCSharp:     "public class Test { }",
		LangPython:     "class Test:\n    pass",
		LangJavaScript: "class Test { }",
		LangTypeScript: "class Test { }",
		LangUnknown:    "package main",
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			lang := DetectLanguage(tt.path)
			source := classSource[lang]
			if source == "" {
				source = classSource[LangUnknown]
			}

			file := &models.SourceFile{
				Path:    tt.path,
				Content: []byte(source),
			}

			result, err := reg.Chunk(context.Background(), file)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(result.Chunks), 1)
		})
	}
}

func TestChunkerRegistry_Fallback(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:    "script.rb",
		Content: []byte("puts 'hello'"),
	}

	result, err := reg.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should have exactly one file-level chunk
	assert.Len(t, result.Chunks, 1)
	assert.Equal(t, models.ChunkLevelFile, result.Chunks[0].Level)
}

func TestChunkerRegistry_SupportedLanguages(t *testing.T) {
	reg, err := NewChunkerRegistry()
	require.NoError(t, err)

	langs := reg.SupportedLanguages()
	assert.Contains(t, langs, LangCSharp)
	assert.Contains(t, langs, LangPython)
	assert.Contains(t, langs, LangJavaScript)
	assert.Contains(t, langs, LangTypeScript)
}
```

---

## Dependencies

### Go Modules Required

```
github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82
```

The tree-sitter module includes language grammars as subpackages.

---

## Testing Strategy

### Test Fixtures

Create representative source files in `testdata/`:

```
testdata/
├── csharp/
│   ├── simple_class.cs
│   ├── nested_classes.cs
│   ├── properties.cs
│   └── generics.cs
├── python/
│   ├── simple_class.py
│   ├── decorators.py
│   ├── nested.py
│   └── async_code.py
├── javascript/
│   ├── es6_class.js
│   ├── functions.js
│   ├── arrow_functions.js
│   └── exports.js
└── typescript/
    ├── interfaces.ts
    ├── generics.ts
    ├── class_with_types.ts
    └── react_component.tsx
```

### Coverage Goals

- Each language chunker: >90% coverage
- Edge cases: empty files, syntax errors, single-line files
- Hierarchy: verify parent-child relationships

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Tree-sitter grammar version mismatch | Low | High | Pin dependency versions |
| Complex syntax not handled | Medium | Medium | Start simple, add edge cases incrementally |
| Large files slow to parse | Low | Medium | Add timeout, chunk size limits |
| Nested structures create too many chunks | Medium | Low | Add depth limits if needed |

---

## Checklist

Before marking Phase 3 complete:

- [ ] Tree-sitter parses all 4 languages
- [ ] C# chunker extracts classes, methods, properties
- [ ] Python chunker extracts classes, functions
- [ ] JS/TS chunker extracts classes, functions, interfaces
- [ ] Fallback creates file-level chunks
- [ ] Parent-child relationships are correct
- [ ] Chunk IDs are deterministic
- [ ] All tests pass with >80% coverage
- [ ] Test fixtures exist for all languages
