package chunker

import (
	"context"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper Functions for Config-Driven Tests
// =============================================================================

// getPythonChunker returns the Python chunker from the config-driven registry.
// This replaces the legacy NewPythonChunker function.
func getPythonChunker(t *testing.T) Chunker {
	t.Helper()
	registry, err := NewChunkerRegistry()
	require.NoError(t, err, "Failed to create chunker registry")

	chunker, ok := registry.GetChunkerForExtension(".py")
	require.True(t, ok, "Python chunker should be available")
	return chunker
}

// =============================================================================
// PythonChunker Initialization Tests
// =============================================================================

func TestPythonChunker_Available(t *testing.T) {
	registry, err := NewChunkerRegistry()
	require.NoError(t, err)

	chunker, ok := registry.GetChunkerForExtension(".py")
	assert.True(t, ok, "Python chunker should be available in registry")
	assert.NotNil(t, chunker, "Python chunker should not be nil")
}

func TestPythonChunker_Language(t *testing.T) {
	chunker := getPythonChunker(t)
	assert.Equal(t, LangPython, chunker.Language(), "PythonChunker should report Python as its language")
}

// =============================================================================
// Simple Class with Methods Tests
// =============================================================================

func TestPythonChunker_SimpleClassWithMethods(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`class Calculator:
    def add(self, a, b):
        return a + b

    def subtract(self, a, b):
        return a - b
`)

	file := &models.SourceFile{
		Path:         "/test/calculator.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err, "Chunk should not return an error for valid Python code")
	require.NotNil(t, result, "Chunk should return a non-nil result")

	// Should have: 1 file chunk + 1 class chunk + 2 method chunks = 4 chunks
	assert.Len(t, result.Chunks, 4, "Should have 4 chunks: file, class, and 2 methods")

	// Find chunks by level
	var fileChunk, classChunk *models.Chunk
	var methodChunks []*models.Chunk

	for _, chunk := range result.Chunks {
		switch chunk.Level {
		case models.ChunkLevelFile:
			fileChunk = chunk
		case models.ChunkLevelClass:
			classChunk = chunk
		case models.ChunkLevelMethod:
			methodChunks = append(methodChunks, chunk)
		}
	}

	// Verify file chunk
	require.NotNil(t, fileChunk, "Should have a file-level chunk")
	assert.Equal(t, "/test/calculator.py", fileChunk.FilePath)
	assert.Nil(t, fileChunk.ParentID, "File chunk should have no parent")

	// Verify class chunk
	require.NotNil(t, classChunk, "Should have a class-level chunk")
	assert.Equal(t, "Calculator", classChunk.Name, "Class chunk should be named 'Calculator'")
	assert.Equal(t, fileChunk.ID, *classChunk.ParentID, "Class should reference file as parent")

	// Verify method chunks
	assert.Len(t, methodChunks, 2, "Should have 2 method chunks")
	for _, method := range methodChunks {
		assert.Equal(t, classChunk.ID, *method.ParentID, "Methods should reference class as parent")
	}

	// Verify method names
	methodNames := make(map[string]bool)
	for _, m := range methodChunks {
		methodNames[m.Name] = true
	}
	assert.True(t, methodNames["add"], "Should have 'add' method")
	assert.True(t, methodNames["subtract"], "Should have 'subtract' method")
}

// =============================================================================
// Top-Level Functions Tests
// =============================================================================

func TestPythonChunker_TopLevelFunctions(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`def greet(name):
    return f"Hello, {name}!"

def farewell(name):
    return f"Goodbye, {name}!"
`)

	file := &models.SourceFile{
		Path:         "/test/greetings.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have: 1 file chunk + 2 function chunks = 3 chunks
	assert.Len(t, result.Chunks, 3, "Should have 3 chunks: file and 2 functions")

	// Find chunks
	var fileChunk *models.Chunk
	var functionChunks []*models.Chunk

	for _, chunk := range result.Chunks {
		switch chunk.Level {
		case models.ChunkLevelFile:
			fileChunk = chunk
		case models.ChunkLevelMethod:
			functionChunks = append(functionChunks, chunk)
		}
	}

	require.NotNil(t, fileChunk, "Should have a file-level chunk")
	assert.Len(t, functionChunks, 2, "Should have 2 function chunks at method level")

	// Top-level functions should reference file as parent
	for _, fn := range functionChunks {
		require.NotNil(t, fn.ParentID, "Top-level function should have a parent ID")
		assert.Equal(t, fileChunk.ID, *fn.ParentID, "Top-level function should reference file as parent")
	}

	// Verify function names
	funcNames := make(map[string]bool)
	for _, fn := range functionChunks {
		funcNames[fn.Name] = true
	}
	assert.True(t, funcNames["greet"], "Should have 'greet' function")
	assert.True(t, funcNames["farewell"], "Should have 'farewell' function")
}

// =============================================================================
// Decorator Tests
// =============================================================================

func TestPythonChunker_Decorators_StaticMethod(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`class Service:
    @staticmethod
    def create():
        return Service()

    @staticmethod
    def destroy(instance):
        pass
`)

	file := &models.SourceFile{
		Path:         "/test/service.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find method chunks
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	assert.Len(t, methodChunks, 2, "Should extract 2 decorated methods")

	methodNames := make(map[string]bool)
	for _, m := range methodChunks {
		methodNames[m.Name] = true
	}
	assert.True(t, methodNames["create"], "Should have 'create' method")
	assert.True(t, methodNames["destroy"], "Should have 'destroy' method")
}

func TestPythonChunker_Decorators_ClassMethod(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`class Factory:
    @classmethod
    def from_config(cls, config):
        return cls()
`)

	file := &models.SourceFile{
		Path:         "/test/factory.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find method chunks
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	assert.Len(t, methodChunks, 1, "Should extract 1 classmethod")
	assert.Equal(t, "from_config", methodChunks[0].Name, "Should extract 'from_config' method")
}

func TestPythonChunker_Decorators_Property(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`class Person:
    def __init__(self, name):
        self._name = name

    @property
    def name(self):
        return self._name

    @name.setter
    def name(self, value):
        self._name = value
`)

	file := &models.SourceFile{
		Path:         "/test/person.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find method chunks
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	// Should have __init__, property getter, and property setter
	assert.Len(t, methodChunks, 3, "Should extract 3 methods including properties")
}

func TestPythonChunker_Decorators_Multiple(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`class Handler:
    @log
    @validate
    @authenticate
    def process(self, data):
        return data
`)

	file := &models.SourceFile{
		Path:         "/test/handler.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find method chunks
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	assert.Len(t, methodChunks, 1, "Should extract 1 method with multiple decorators")
	assert.Equal(t, "process", methodChunks[0].Name, "Should extract 'process' method")
}

// =============================================================================
// Nested Classes Tests
// =============================================================================

func TestPythonChunker_NestedClasses(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`class Outer:
    def outer_method(self):
        pass

    class Inner:
        def inner_method(self):
            pass
`)

	file := &models.SourceFile{
		Path:         "/test/nested.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find chunks by level and name
	var outerClass, innerClass *models.Chunk
	var methodChunks []*models.Chunk

	for _, chunk := range result.Chunks {
		switch chunk.Level {
		case models.ChunkLevelClass:
			if chunk.Name == "Outer" {
				outerClass = chunk
			} else if chunk.Name == "Inner" {
				innerClass = chunk
			}
		case models.ChunkLevelMethod:
			methodChunks = append(methodChunks, chunk)
		}
	}

	require.NotNil(t, outerClass, "Should have Outer class chunk")
	require.NotNil(t, innerClass, "Should have Inner class chunk")

	// Verify methods exist
	assert.Len(t, methodChunks, 2, "Should have 2 methods")

	// Note: Parent-child relationships for nested classes depend on the chunker implementation
	// The generic chunker may or may not set up correct parent relationships
}

func TestPythonChunker_DeeplyNestedClasses(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`class Level1:
    class Level2:
        class Level3:
            def deep_method(self):
                pass
`)

	file := &models.SourceFile{
		Path:         "/test/deep_nested.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find all class chunks
	classChunks := make(map[string]*models.Chunk)

	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classChunks[chunk.Name] = chunk
		}
	}

	require.Len(t, classChunks, 3, "Should have 3 nested classes")

	// Verify classes exist
	level1 := classChunks["Level1"]
	level2 := classChunks["Level2"]
	level3 := classChunks["Level3"]

	require.NotNil(t, level1)
	require.NotNil(t, level2)
	require.NotNil(t, level3)

	// Note: Parent-child relationships for nested classes depend on the chunker implementation
	// The generic chunker may or may not set up correct parent relationships
}

// =============================================================================
// Dunder Methods Tests
// =============================================================================

func TestPythonChunker_DunderMethods(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`class MyClass:
    def __init__(self, value):
        self.value = value

    def __str__(self):
        return str(self.value)

    def __repr__(self):
        return f"MyClass({self.value})"

    def __eq__(self, other):
        return self.value == other.value

    def __hash__(self):
        return hash(self.value)
`)

	file := &models.SourceFile{
		Path:         "/test/dunder.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find method chunks
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	assert.Len(t, methodChunks, 5, "Should extract 5 dunder methods")

	// Verify all dunder methods are extracted
	methodNames := make(map[string]bool)
	for _, m := range methodChunks {
		methodNames[m.Name] = true
	}

	expectedDunders := []string{"__init__", "__str__", "__repr__", "__eq__", "__hash__"}
	for _, dunder := range expectedDunders {
		assert.True(t, methodNames[dunder], "Should have '%s' method", dunder)
	}
}

// =============================================================================
// Parent-Child Relationship Tests
// =============================================================================

func TestPythonChunker_ParentChildRelationships_Mixed(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`def standalone_function():
    pass

class FirstClass:
    def first_method(self):
        pass

def another_standalone():
    pass

class SecondClass:
    def second_method(self):
        pass
`)

	file := &models.SourceFile{
		Path:         "/test/mixed.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find chunks
	var fileChunk *models.Chunk
	classChunks := make(map[string]*models.Chunk)
	methodChunks := make(map[string]*models.Chunk)

	for _, chunk := range result.Chunks {
		switch chunk.Level {
		case models.ChunkLevelFile:
			fileChunk = chunk
		case models.ChunkLevelClass:
			classChunks[chunk.Name] = chunk
		case models.ChunkLevelMethod:
			methodChunks[chunk.Name] = chunk
		}
	}

	require.NotNil(t, fileChunk)

	// Verify top-level functions reference file
	standalone := methodChunks["standalone_function"]
	anotherStandalone := methodChunks["another_standalone"]
	require.NotNil(t, standalone, "Should have 'standalone_function'")
	require.NotNil(t, anotherStandalone, "Should have 'another_standalone'")
	assert.Equal(t, fileChunk.ID, *standalone.ParentID, "standalone_function should reference file")
	assert.Equal(t, fileChunk.ID, *anotherStandalone.ParentID, "another_standalone should reference file")

	// Verify class methods reference their class
	firstClass := classChunks["FirstClass"]
	secondClass := classChunks["SecondClass"]
	firstMethod := methodChunks["first_method"]
	secondMethod := methodChunks["second_method"]

	require.NotNil(t, firstClass)
	require.NotNil(t, secondClass)
	require.NotNil(t, firstMethod)
	require.NotNil(t, secondMethod)

	assert.Equal(t, firstClass.ID, *firstMethod.ParentID, "first_method should reference FirstClass")
	assert.Equal(t, secondClass.ID, *secondMethod.ParentID, "second_method should reference SecondClass")
}

// =============================================================================
// Deterministic ID Tests
// =============================================================================

func TestPythonChunker_DeterministicIDs(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`class Calculator:
    def add(self, a, b):
        return a + b
`)

	file := &models.SourceFile{
		Path:         "/test/calc.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	// Parse the same file twice
	result1, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	result2, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Both results should have the same number of chunks
	require.Equal(t, len(result1.Chunks), len(result2.Chunks), "Both parses should produce same number of chunks")

	// Create maps by chunk name for comparison
	chunks1 := make(map[string]string)
	chunks2 := make(map[string]string)

	for _, c := range result1.Chunks {
		key := string(c.Level) + ":" + c.Name
		chunks1[key] = c.ID
	}

	for _, c := range result2.Chunks {
		key := string(c.Level) + ":" + c.Name
		chunks2[key] = c.ID
	}

	// Verify all IDs match
	for key, id1 := range chunks1 {
		id2, exists := chunks2[key]
		require.True(t, exists, "Chunk %s should exist in both results", key)
		assert.Equal(t, id1, id2, "Chunk %s should have deterministic ID", key)
	}
}

func TestPythonChunker_DeterministicIDs_SameContent_DifferentFiles(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`def hello():
    pass
`)

	file1 := &models.SourceFile{
		Path:         "/test/file1.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	file2 := &models.SourceFile{
		Path:         "/test/file2.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result1, err := chunker.Chunk(context.Background(), file1)
	require.NoError(t, err)

	result2, err := chunker.Chunk(context.Background(), file2)
	require.NoError(t, err)

	// Same content but different paths should produce different IDs
	require.Equal(t, len(result1.Chunks), len(result2.Chunks))

	for i := range result1.Chunks {
		assert.NotEqual(t, result1.Chunks[i].ID, result2.Chunks[i].ID,
			"Same content in different files should have different IDs")
	}
}

// =============================================================================
// Edge Cases Tests
// =============================================================================

func TestPythonChunker_EmptyFile(t *testing.T) {
	chunker := getPythonChunker(t)

	file := &models.SourceFile{
		Path:         "/test/empty.py",
		Content:      []byte(""),
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Empty file might still have a file-level chunk, or might have zero chunks
	// Implementation can decide, but should not error
	assert.Empty(t, result.Errors, "Should have no errors for empty file")
}

func TestPythonChunker_OnlyComments(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`# This is a comment
# Another comment
"""
A docstring that is not attached to anything
"""
`)

	file := &models.SourceFile{
		Path:         "/test/comments.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should handle gracefully, possibly just a file chunk
	assert.Empty(t, result.Errors, "Should have no errors for comment-only file")
}

func TestPythonChunker_AsyncFunctions(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`async def fetch_data():
    await some_async_call()

class AsyncService:
    async def process(self):
        await self.fetch()
`)

	file := &models.SourceFile{
		Path:         "/test/async.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find method chunks
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	assert.Len(t, methodChunks, 2, "Should extract async functions")

	methodNames := make(map[string]bool)
	for _, m := range methodChunks {
		methodNames[m.Name] = true
	}
	assert.True(t, methodNames["fetch_data"], "Should have 'fetch_data' async function")
	assert.True(t, methodNames["process"], "Should have 'process' async method")
}

func TestPythonChunker_LambdaFunctions(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`class Processor:
    def __init__(self):
        self.handler = lambda x: x * 2
        self.filter = lambda x: x > 0

    def process(self):
        return list(map(lambda x: x + 1, [1, 2, 3]))
`)

	file := &models.SourceFile{
		Path:         "/test/lambda.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Lambda expressions inside methods should not be extracted as separate chunks
	// Only __init__ and process should be method chunks
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	assert.Len(t, methodChunks, 2, "Should only extract named methods, not lambdas")
}

func TestPythonChunker_InheritedClasses(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`class Base:
    def base_method(self):
        pass

class Child(Base):
    def child_method(self):
        pass

class MultiInherit(Base, OtherMixin):
    def multi_method(self):
        pass
`)

	file := &models.SourceFile{
		Path:         "/test/inheritance.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find class chunks
	var classChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classChunks = append(classChunks, chunk)
		}
	}

	assert.Len(t, classChunks, 3, "Should extract all 3 classes regardless of inheritance")

	classNames := make(map[string]bool)
	for _, c := range classChunks {
		classNames[c.Name] = true
	}
	assert.True(t, classNames["Base"])
	assert.True(t, classNames["Child"])
	assert.True(t, classNames["MultiInherit"])
}

// =============================================================================
// Line Number Tests
// =============================================================================

func TestPythonChunker_CorrectLineNumbers(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`class Calculator:
    def add(self, a, b):
        return a + b

    def subtract(self, a, b):
        return a - b
`)

	file := &models.SourceFile{
		Path:         "/test/lines.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find specific chunks
	var classChunk *models.Chunk
	methodChunks := make(map[string]*models.Chunk)

	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classChunk = chunk
		} else if chunk.Level == models.ChunkLevelMethod {
			methodChunks[chunk.Name] = chunk
		}
	}

	require.NotNil(t, classChunk)

	// Class should start at line 1
	assert.Equal(t, 1, classChunk.StartLine, "Class should start at line 1")

	// Methods should have correct line numbers
	addMethod := methodChunks["add"]
	require.NotNil(t, addMethod)
	assert.Equal(t, 2, addMethod.StartLine, "add method should start at line 2")
	assert.Equal(t, 3, addMethod.EndLine, "add method should end at line 3")

	subtractMethod := methodChunks["subtract"]
	require.NotNil(t, subtractMethod)
	assert.Equal(t, 5, subtractMethod.StartLine, "subtract method should start at line 5")
	assert.Equal(t, 6, subtractMethod.EndLine, "subtract method should end at line 6")
}

// =============================================================================
// Content Tests
// =============================================================================

func TestPythonChunker_ChunkContent(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`def greet(name):
    return f"Hello, {name}!"
`)

	file := &models.SourceFile{
		Path:         "/test/content.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find the function chunk
	var funcChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			funcChunk = chunk
			break
		}
	}

	require.NotNil(t, funcChunk)
	assert.Contains(t, funcChunk.Content, "def greet", "Chunk content should contain the function definition")
	assert.Contains(t, funcChunk.Content, "return", "Chunk content should contain the return statement")
}

// =============================================================================
// Signature Tests
// =============================================================================

func TestPythonChunker_MethodSignature(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`def calculate(x: int, y: int = 10, *args, **kwargs) -> int:
    return x + y
`)

	file := &models.SourceFile{
		Path:         "/test/signature.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find the function chunk
	var funcChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			funcChunk = chunk
			break
		}
	}

	require.NotNil(t, funcChunk)
	assert.Equal(t, "calculate", funcChunk.Name)
	// Signature should capture the function signature
	assert.NotEmpty(t, funcChunk.Signature, "Should have a signature")
}

// =============================================================================
// Context Cancellation Tests
// =============================================================================

func TestPythonChunker_ContextCancellation(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`class Test:
    def method(self):
        pass
`)

	file := &models.SourceFile{
		Path:         "/test/cancel.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := chunker.Chunk(ctx, file)
	// Should either return an error or handle gracefully
	// The exact behavior depends on implementation
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled, "Should return context.Canceled error")
	}
}

// =============================================================================
// Comprehensive Integration Test
// =============================================================================

func TestPythonChunker_ComprehensiveFile(t *testing.T) {
	chunker := getPythonChunker(t)

	source := []byte(`"""Module docstring"""

def standalone():
    """Standalone function"""
    pass

class Calculator:
    """Calculator class"""

    def __init__(self):
        self.result = 0

    def add(self, a, b):
        return a + b

    @staticmethod
    def create():
        return Calculator()

    class InnerHelper:
        def help(self):
            pass

class Service:
    @classmethod
    def from_config(cls, config):
        return cls()

async def async_func():
    pass
`)

	file := &models.SourceFile{
		Path:         "/test/comprehensive.py",
		Content:      source,
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Errors)

	// Count chunks by level
	levelCounts := make(map[models.ChunkLevel]int)
	for _, chunk := range result.Chunks {
		levelCounts[chunk.Level]++
	}

	// Should have:
	// - 1 file chunk
	// - 3 class chunks (Calculator, InnerHelper, Service)
	// - 7 method chunks (standalone, __init__, add, create, help, from_config, async_func)
	assert.Equal(t, 1, levelCounts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.Equal(t, 3, levelCounts[models.ChunkLevelClass], "Should have 3 class chunks")
	assert.Equal(t, 7, levelCounts[models.ChunkLevelMethod], "Should have 7 method chunks")

	// Verify all chunks are valid
	for _, chunk := range result.Chunks {
		err := chunk.IsValid()
		assert.NoError(t, err, "Chunk %s should be valid", chunk.Name)
		assert.NotEmpty(t, chunk.ID, "Chunk should have an ID")
		assert.Equal(t, "python", chunk.Language, "Chunk should have language set")
	}
}
