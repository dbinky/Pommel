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
// Test Helpers
// =============================================================================

// getProjectRoot returns the absolute path to the project root directory.
func getProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	// internal/chunker/generic_test.go -> project root
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

// loadGenericTestConfig loads a language config from the project's languages directory.
// This is a helper specifically for generic chunker tests.
func loadGenericTestConfig(t *testing.T, lang string) *LanguageConfig {
	t.Helper()
	root := getProjectRoot()
	configPath := filepath.Join(root, "languages", lang+".yaml")
	config, err := LoadLanguageConfig(configPath)
	require.NoError(t, err, "Failed to load %s.yaml config", lang)
	return config
}

// =============================================================================
// GenericChunker Initialization Tests
// =============================================================================

func TestNewGenericChunker(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)
	assert.NotNil(t, chunker, "NewGenericChunker should return a non-nil chunker")
}

func TestNewGenericChunker_NilConfig(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	chunker, err := NewGenericChunker(parser, nil)
	assert.Error(t, err, "Should return error for nil config")
	assert.Nil(t, chunker, "Chunker should be nil when config is nil")
	assert.Contains(t, err.Error(), "config is required")
}

func TestNewGenericChunker_NilParser(t *testing.T) {
	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(nil, config)
	assert.Error(t, err, "Should return error for nil parser")
	assert.Nil(t, chunker, "Chunker should be nil when parser is nil")
	assert.Contains(t, err.Error(), "parser is required")
}

func TestGenericChunker_Language(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	assert.Equal(t, Language("go"), chunker.Language(), "GenericChunker should report the correct language")
}

// =============================================================================
// Go Language Tests
// =============================================================================

func TestGenericChunker_ChunksGoFile(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`package main

type Calculator struct {
	result int
}

func (c *Calculator) Add(a, b int) int {
	return a + b
}

func helper() {
	println("helper")
}
`)

	file := &models.SourceFile{
		Path:         "/test/calculator.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Count chunks by level
	levelCounts := make(map[models.ChunkLevel]int)
	for _, chunk := range result.Chunks {
		levelCounts[chunk.Level]++
	}

	// Should have: 1 file chunk, 1 class (struct) chunk, 2 method chunks
	assert.Equal(t, 1, levelCounts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.Equal(t, 1, levelCounts[models.ChunkLevelClass], "Should have 1 class-level chunk")
	assert.Equal(t, 2, levelCounts[models.ChunkLevelMethod], "Should have 2 method-level chunks")

	// Verify chunk names
	names := make(map[string]bool)
	for _, chunk := range result.Chunks {
		if chunk.Level != models.ChunkLevelFile {
			names[chunk.Name] = true
		}
	}
	assert.True(t, names["Calculator"], "Should have 'Calculator' struct")
	assert.True(t, names["Add"], "Should have 'Add' method")
	assert.True(t, names["helper"], "Should have 'helper' function")
}

// =============================================================================
// Python Language Tests
// =============================================================================

func TestGenericChunker_ChunksPythonFile(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "python")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`class Calculator:
    def __init__(self):
        self.result = 0

    def add(self, a, b):
        return a + b

def helper():
    print("helper")
`)

	file := &models.SourceFile{
		Path:         "/test/calculator.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Count chunks by level
	levelCounts := make(map[models.ChunkLevel]int)
	for _, chunk := range result.Chunks {
		levelCounts[chunk.Level]++
	}

	// Should have: 1 file chunk, 1 class chunk, 3 method chunks (__init__, add, helper)
	assert.Equal(t, 1, levelCounts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.Equal(t, 1, levelCounts[models.ChunkLevelClass], "Should have 1 class chunk")
	assert.Equal(t, 3, levelCounts[models.ChunkLevelMethod], "Should have 3 method-level chunks")

	// Verify chunk names
	names := make(map[string]bool)
	for _, chunk := range result.Chunks {
		if chunk.Level != models.ChunkLevelFile {
			names[chunk.Name] = true
		}
	}
	assert.True(t, names["Calculator"], "Should have 'Calculator' class")
	assert.True(t, names["__init__"], "Should have '__init__' method")
	assert.True(t, names["add"], "Should have 'add' method")
	assert.True(t, names["helper"], "Should have 'helper' function")
}

// =============================================================================
// Java Language Tests
// =============================================================================

func TestGenericChunker_ChunksJavaFile(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "java")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`package com.example;

public class Calculator {
    private int result;

    public Calculator() {
        this.result = 0;
    }

    public int add(int a, int b) {
        return a + b;
    }
}
`)

	file := &models.SourceFile{
		Path:         "/test/Calculator.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Count chunks by level
	levelCounts := make(map[models.ChunkLevel]int)
	for _, chunk := range result.Chunks {
		levelCounts[chunk.Level]++
	}

	// Should have: 1 file chunk, 1 class chunk (Calculator), 2 method chunks (constructor, add)
	assert.Equal(t, 1, levelCounts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.Equal(t, 1, levelCounts[models.ChunkLevelClass], "Should have 1 class-level chunk")
	assert.Equal(t, 2, levelCounts[models.ChunkLevelMethod], "Should have 2 method-level chunks")

	// Verify chunk names
	names := make(map[string]bool)
	for _, chunk := range result.Chunks {
		if chunk.Level != models.ChunkLevelFile {
			names[chunk.Name] = true
		}
	}
	assert.True(t, names["Calculator"], "Should have 'Calculator' class")
	assert.True(t, names["add"], "Should have 'add' method")
}

// =============================================================================
// Class Detection Tests
// =============================================================================

func TestGenericChunker_FindsClasses(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "java")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`public class User {
    String name;
}

interface Repository {
    void save();
}

enum Status {
    ACTIVE, INACTIVE
}

record Person(String name, int age) {}
`)

	file := &models.SourceFile{
		Path:         "/test/Types.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find class-level chunks
	var classChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classChunks = append(classChunks, chunk)
		}
	}

	// Should find class, interface, enum, and record
	assert.GreaterOrEqual(t, len(classChunks), 4, "Should find at least 4 class-level constructs")

	classNames := make(map[string]bool)
	for _, c := range classChunks {
		classNames[c.Name] = true
	}
	assert.True(t, classNames["User"], "Should find 'User' class")
	assert.True(t, classNames["Repository"], "Should find 'Repository' interface")
	assert.True(t, classNames["Status"], "Should find 'Status' enum")
	assert.True(t, classNames["Person"], "Should find 'Person' record")
}

// =============================================================================
// Method Detection Tests
// =============================================================================

func TestGenericChunker_FindsMethods(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`package main

func standalone() {}

func withArgs(x int, y string) bool {
	return true
}

func withReturn() (int, error) {
	return 0, nil
}

type Service struct{}

func (s *Service) Handle() {}
`)

	file := &models.SourceFile{
		Path:         "/test/methods.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find method-level chunks
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	assert.Len(t, methodChunks, 4, "Should find 4 functions/methods")

	methodNames := make(map[string]bool)
	for _, m := range methodChunks {
		methodNames[m.Name] = true
	}
	assert.True(t, methodNames["standalone"], "Should find 'standalone' function")
	assert.True(t, methodNames["withArgs"], "Should find 'withArgs' function")
	assert.True(t, methodNames["withReturn"], "Should find 'withReturn' function")
	assert.True(t, methodNames["Handle"], "Should find 'Handle' method")
}

// =============================================================================
// Nested Class Tests
// =============================================================================

func TestGenericChunker_FindsNestedClasses(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "java")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`public class Outer {
    private int value;

    public class Inner {
        public void innerMethod() {}
    }

    public static class StaticNested {
        public void nestedMethod() {}
    }
}
`)

	file := &models.SourceFile{
		Path:         "/test/Nested.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find class-level chunks
	var classChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classChunks = append(classChunks, chunk)
		}
	}

	assert.Len(t, classChunks, 3, "Should find 3 class-level constructs (Outer, Inner, StaticNested)")

	classNames := make(map[string]bool)
	for _, c := range classChunks {
		classNames[c.Name] = true
	}
	assert.True(t, classNames["Outer"], "Should find 'Outer' class")
	assert.True(t, classNames["Inner"], "Should find 'Inner' class")
	assert.True(t, classNames["StaticNested"], "Should find 'StaticNested' class")
}

// =============================================================================
// Name Extraction Tests
// =============================================================================

func TestGenericChunker_ExtractsName(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "python")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`class MySpecialClass:
    pass

def my_special_function():
    pass
`)

	file := &models.SourceFile{
		Path:         "/test/names.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find chunks by name
	names := make(map[string]bool)
	for _, chunk := range result.Chunks {
		if chunk.Level != models.ChunkLevelFile {
			names[chunk.Name] = true
		}
	}

	assert.True(t, names["MySpecialClass"], "Should extract 'MySpecialClass' name")
	assert.True(t, names["my_special_function"], "Should extract 'my_special_function' name")
}

// =============================================================================
// File Chunk Tests
// =============================================================================

func TestGenericChunker_FileChunkAlwaysCreated(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	testCases := []struct {
		name    string
		content string
	}{
		{"empty package", "package main\n"},
		{"with function", "package main\n\nfunc main() {}\n"},
		{"with struct", "package main\n\ntype S struct{}\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			file := &models.SourceFile{
				Path:         "/test/file.go",
				Content:      []byte(tc.content),
				Language:     "go",
				LastModified: time.Now(),
			}

			result, err := chunker.Chunk(context.Background(), file)
			require.NoError(t, err)

			// Find file chunk
			var fileChunk *models.Chunk
			for _, chunk := range result.Chunks {
				if chunk.Level == models.ChunkLevelFile {
					fileChunk = chunk
					break
				}
			}

			require.NotNil(t, fileChunk, "File chunk should always be created")
			assert.Equal(t, file.Path, fileChunk.FilePath)
			assert.Equal(t, file.Path, fileChunk.Name)
			assert.Equal(t, 1, fileChunk.StartLine)
		})
	}
}

// =============================================================================
// Parent-Child Relationship Tests
// =============================================================================

func TestGenericChunker_ParentChildRelationships(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "python")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`class Parent:
    def method(self):
        pass

def top_level():
    pass
`)

	file := &models.SourceFile{
		Path:         "/test/parent_child.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find chunks
	var fileChunk, classChunk, methodChunk, topLevelFunc *models.Chunk
	for _, chunk := range result.Chunks {
		switch {
		case chunk.Level == models.ChunkLevelFile:
			fileChunk = chunk
		case chunk.Level == models.ChunkLevelClass && chunk.Name == "Parent":
			classChunk = chunk
		case chunk.Level == models.ChunkLevelMethod && chunk.Name == "method":
			methodChunk = chunk
		case chunk.Level == models.ChunkLevelMethod && chunk.Name == "top_level":
			topLevelFunc = chunk
		}
	}

	require.NotNil(t, fileChunk)
	require.NotNil(t, classChunk)
	require.NotNil(t, methodChunk)
	require.NotNil(t, topLevelFunc)

	// File chunk should have no parent
	assert.Nil(t, fileChunk.ParentID, "File chunk should have no parent")

	// Class should reference file as parent
	require.NotNil(t, classChunk.ParentID)
	assert.Equal(t, fileChunk.ID, *classChunk.ParentID, "Class should reference file as parent")

	// Method inside class should reference class as parent
	require.NotNil(t, methodChunk.ParentID)
	assert.Equal(t, classChunk.ID, *methodChunk.ParentID, "Method should reference class as parent")

	// Top-level function should reference file as parent
	require.NotNil(t, topLevelFunc.ParentID)
	assert.Equal(t, fileChunk.ID, *topLevelFunc.ParentID, "Top-level function should reference file as parent")
}

// =============================================================================
// Line Number Tests
// =============================================================================

func TestGenericChunker_LineNumbers(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`package main

type Example struct {
	field int
}

func (e *Example) Method() {
	println("hello")
}
`)

	file := &models.SourceFile{
		Path:         "/test/lines.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find chunks
	var structChunk, methodChunk *models.Chunk
	for _, chunk := range result.Chunks {
		switch {
		case chunk.Level == models.ChunkLevelClass:
			structChunk = chunk
		case chunk.Level == models.ChunkLevelMethod:
			methodChunk = chunk
		}
	}

	require.NotNil(t, structChunk)
	require.NotNil(t, methodChunk)

	// Struct should be lines 3-5
	assert.Equal(t, 3, structChunk.StartLine, "Struct should start at line 3")
	assert.Equal(t, 5, structChunk.EndLine, "Struct should end at line 5")

	// Method should be lines 7-9
	assert.Equal(t, 7, methodChunk.StartLine, "Method should start at line 7")
	assert.Equal(t, 9, methodChunk.EndLine, "Method should end at line 9")
}

// =============================================================================
// Parse Error Tests
// =============================================================================

func TestGenericChunker_ParseError(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	// Malformed Go code - still parseable by tree-sitter but may have ERROR nodes
	// Tree-sitter is error-tolerant, so we test with something that can still be processed
	source := []byte(`package main

func incomplete( {
	// missing closing paren
}
`)

	file := &models.SourceFile{
		Path:         "/test/broken.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	// Tree-sitter is error-tolerant, so this shouldn't return an error
	// It will just produce chunks with what it can parse
	result, err := chunker.Chunk(context.Background(), file)
	// May or may not error depending on implementation
	if err == nil {
		require.NotNil(t, result)
		// Should at least have a file chunk
		var hasFileChunk bool
		for _, chunk := range result.Chunks {
			if chunk.Level == models.ChunkLevelFile {
				hasFileChunk = true
				break
			}
		}
		assert.True(t, hasFileChunk, "Should have file chunk even with parse errors")
	}
}

// =============================================================================
// Empty File Tests
// =============================================================================

func TestGenericChunker_EmptyFile(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "/test/empty.go",
		Content:      []byte(""),
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Empty file should have no chunks (or just handle gracefully)
	assert.Empty(t, result.Errors, "Should have no errors for empty file")
}

// =============================================================================
// No Matching Nodes Tests
// =============================================================================

func TestGenericChunker_NoMatchingNodes(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	// Go file with only package declaration and imports - no functions or types
	source := []byte(`package main

import (
	"fmt"
)

var globalVar = "hello"
`)

	file := &models.SourceFile{
		Path:         "/test/no_matches.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have exactly 1 file chunk
	assert.Len(t, result.Chunks, 1, "Should have only file chunk when no matching nodes")
	assert.Equal(t, models.ChunkLevelFile, result.Chunks[0].Level)
}

// =============================================================================
// Unicode Content Tests
// =============================================================================

func TestGenericChunker_UnicodeContent(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "python")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`# -*- coding: utf-8 -*-

class Greeting:
    """Greets users in multiple languages."""

    def greet(self, name):
        return f"Hello {name}! Bonjour! Hola!"
`)

	file := &models.SourceFile{
		Path:         "/test/unicode.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should handle unicode without issues
	var classChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classChunk = chunk
			break
		}
	}

	require.NotNil(t, classChunk)
	assert.Equal(t, "Greeting", classChunk.Name)
	assert.Contains(t, classChunk.Content, "Greeting")
}

func TestGenericChunker_UnicodeIdentifiers(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "python")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	// Python supports unicode identifiers
	source := []byte(`def calcul_somme(nombre):
    return nombre + 1
`)

	file := &models.SourceFile{
		Path:         "/test/unicode_ident.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find function chunk
	var funcChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			funcChunk = chunk
			break
		}
	}

	require.NotNil(t, funcChunk)
	assert.Equal(t, "calcul_somme", funcChunk.Name)
}

// =============================================================================
// Context Cancellation Tests
// =============================================================================

func TestGenericChunker_ContextCancellation(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`package main

func test() {}
`)

	file := &models.SourceFile{
		Path:         "/test/cancel.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = chunker.Chunk(ctx, file)
	// Should return context.Canceled error
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled, "Should return context.Canceled error")
	}
}

func TestGenericChunker_ContextTimeout(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`package main

func test() {}
`)

	file := &models.SourceFile{
		Path:         "/test/timeout.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	// Create context that's already timed out
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	// Small sleep to ensure timeout
	time.Sleep(time.Millisecond)

	_, err = chunker.Chunk(ctx, file)
	// Should return context.DeadlineExceeded error
	if err != nil {
		assert.ErrorIs(t, err, context.DeadlineExceeded, "Should return context.DeadlineExceeded error")
	}
}

// =============================================================================
// Deterministic ID Tests
// =============================================================================

func TestGenericChunker_DeterministicIDs(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`package main

type Example struct{}

func (e *Example) Method() {}
`)

	file := &models.SourceFile{
		Path:         "/test/deterministic.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	// Parse twice
	result1, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	result2, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should have same number of chunks
	require.Equal(t, len(result1.Chunks), len(result2.Chunks))

	// Build maps of IDs by name for comparison
	ids1 := make(map[string]string)
	ids2 := make(map[string]string)

	for _, c := range result1.Chunks {
		key := string(c.Level) + ":" + c.Name
		ids1[key] = c.ID
	}

	for _, c := range result2.Chunks {
		key := string(c.Level) + ":" + c.Name
		ids2[key] = c.ID
	}

	// All IDs should match
	for key, id1 := range ids1 {
		id2, exists := ids2[key]
		require.True(t, exists, "Chunk %s should exist in both results", key)
		assert.Equal(t, id1, id2, "Chunk %s should have deterministic ID", key)
	}
}

// =============================================================================
// Content Hash Tests
// =============================================================================

func TestGenericChunker_ContentHashes(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`package main

func example() {}
`)

	file := &models.SourceFile{
		Path:         "/test/hashes.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// All chunks should have content hashes
	for _, chunk := range result.Chunks {
		assert.NotEmpty(t, chunk.ContentHash, "Chunk %s should have a content hash", chunk.Name)
		assert.NotEmpty(t, chunk.ID, "Chunk %s should have an ID", chunk.Name)
	}
}

// =============================================================================
// Signature Tests
// =============================================================================

func TestGenericChunker_Signatures(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "go")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	source := []byte(`package main

func calculate(x, y int) (int, error) {
	return x + y, nil
}
`)

	file := &models.SourceFile{
		Path:         "/test/sig.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find function chunk
	var funcChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			funcChunk = chunk
			break
		}
	}

	require.NotNil(t, funcChunk)
	assert.NotEmpty(t, funcChunk.Signature, "Function should have a signature")
	assert.Contains(t, funcChunk.Signature, "func calculate", "Signature should contain function name")
}

// =============================================================================
// Multiple Languages Compatibility Tests
// =============================================================================

func TestGenericChunker_MultipleLanguages(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	testCases := []struct {
		lang     string
		source   string
		expected struct {
			classes int
			methods int
		}
	}{
		{
			lang: "go",
			source: `package main

type S struct{}

func f() {}
`,
			expected: struct {
				classes int
				methods int
			}{1, 1},
		},
		{
			lang: "python",
			source: `class C:
    def m(self):
        pass
`,
			expected: struct {
				classes int
				methods int
			}{1, 1},
		},
		{
			lang: "java",
			source: `public class C {
    public void m() {}
}
`,
			expected: struct {
				classes int
				methods int
			}{1, 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.lang, func(t *testing.T) {
			config := loadGenericTestConfig(t, tc.lang)
			chunker, err := NewGenericChunker(parser, config)
			require.NoError(t, err)

			file := &models.SourceFile{
				Path:         "/test/file." + tc.lang,
				Content:      []byte(tc.source),
				Language:     tc.lang,
				LastModified: time.Now(),
			}

			result, err := chunker.Chunk(context.Background(), file)
			require.NoError(t, err)

			// Count chunks
			var classes, methods int
			for _, chunk := range result.Chunks {
				switch chunk.Level {
				case models.ChunkLevelClass:
					classes++
				case models.ChunkLevelMethod:
					methods++
				}
			}

			assert.Equal(t, tc.expected.classes, classes, "Should have expected number of classes")
			assert.Equal(t, tc.expected.methods, methods, "Should have expected number of methods")
		})
	}
}

// =============================================================================
// isClassNode and isMethodNode Tests
// =============================================================================

func TestGenericChunker_isClassNode(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "java")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	// Test class node types
	assert.True(t, chunker.isClassNode("class_declaration"), "class_declaration should be a class node")
	assert.True(t, chunker.isClassNode("interface_declaration"), "interface_declaration should be a class node")
	assert.True(t, chunker.isClassNode("enum_declaration"), "enum_declaration should be a class node")
	assert.False(t, chunker.isClassNode("method_declaration"), "method_declaration should not be a class node")
	assert.False(t, chunker.isClassNode("unknown_type"), "unknown_type should not be a class node")
}

func TestGenericChunker_isMethodNode(t *testing.T) {
	parser, err := NewParser()
	require.NoError(t, err)

	config := loadGenericTestConfig(t, "java")
	chunker, err := NewGenericChunker(parser, config)
	require.NoError(t, err)

	// Test method node types
	assert.True(t, chunker.isMethodNode("method_declaration"), "method_declaration should be a method node")
	assert.True(t, chunker.isMethodNode("constructor_declaration"), "constructor_declaration should be a method node")
	assert.False(t, chunker.isMethodNode("class_declaration"), "class_declaration should not be a method node")
	assert.False(t, chunker.isMethodNode("unknown_type"), "unknown_type should not be a method node")
}
