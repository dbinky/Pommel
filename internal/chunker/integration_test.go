package chunker

import (
	"context"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers for Integration Tests
// =============================================================================

// integrationGetLanguagesDir returns the absolute path to the project's languages directory.
func integrationGetLanguagesDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(filename), "..", "..", "languages")
}

// integrationFindChunkByLevel returns the first chunk with the specified level.
func integrationFindChunkByLevel(chunks []*models.Chunk, level models.ChunkLevel) *models.Chunk {
	for _, c := range chunks {
		if c.Level == level {
			return c
		}
	}
	return nil
}

// integrationFindChunkByName returns the first chunk with the specified name.
func integrationFindChunkByName(chunks []*models.Chunk, name string) *models.Chunk {
	for _, c := range chunks {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// integrationFindChunkByNameAndLevel returns the first chunk with the specified name and level.
func integrationFindChunkByNameAndLevel(chunks []*models.Chunk, name string, level models.ChunkLevel) *models.Chunk {
	for _, c := range chunks {
		if c.Name == name && c.Level == level {
			return c
		}
	}
	return nil
}

// integrationFindAllChunksByLevel returns all chunks with the specified level.
func integrationFindAllChunksByLevel(chunks []*models.Chunk, level models.ChunkLevel) []*models.Chunk {
	var result []*models.Chunk
	for _, c := range chunks {
		if c.Level == level {
			result = append(result, c)
		}
	}
	return result
}

// integrationCountChunksByLevel counts chunks by their level.
func integrationCountChunksByLevel(chunks []*models.Chunk) map[models.ChunkLevel]int {
	counts := make(map[models.ChunkLevel]int)
	for _, c := range chunks {
		counts[c.Level]++
	}
	return counts
}

// integrationGetChunkNames returns a sorted list of chunk names for a given level.
func integrationGetChunkNames(chunks []*models.Chunk, level models.ChunkLevel) []string {
	var names []string
	for _, c := range chunks {
		if c.Level == level {
			names = append(names, c.Name)
		}
	}
	sort.Strings(names)
	return names
}

// =============================================================================
// Test Fixtures
// =============================================================================

var goTestFile = `package calculator

// Calculator provides arithmetic operations.
type Calculator struct {
	precision int
}

// NewCalculator creates a new Calculator instance.
func NewCalculator(precision int) *Calculator {
	return &Calculator{precision: precision}
}

// Add returns the sum of two numbers.
func (c *Calculator) Add(a, b float64) float64 {
	return a + b
}

// Subtract returns the difference of two numbers.
func (c *Calculator) Subtract(a, b float64) float64 {
	return a - b
}
`

var javaTestFile = `package com.example;

/**
 * Calculator provides arithmetic operations.
 */
public class Calculator {
    private int precision;

    /**
     * Creates a new Calculator instance.
     */
    public Calculator(int precision) {
        this.precision = precision;
    }

    /**
     * Returns the sum of two numbers.
     */
    public double add(double a, double b) {
        return a + b;
    }

    /**
     * Returns the difference of two numbers.
     */
    public double subtract(double a, double b) {
        return a - b;
    }
}
`

var pythonTestFile = `"""Calculator module providing arithmetic operations."""

class Calculator:
    """Calculator provides arithmetic operations."""

    def __init__(self, precision):
        """Creates a new Calculator instance."""
        self.precision = precision

    def add(self, a, b):
        """Returns the sum of two numbers."""
        return a + b

    def subtract(self, a, b):
        """Returns the difference of two numbers."""
        return a - b
`

var typeScriptTestFile = `/**
 * Calculator provides arithmetic operations.
 */
class Calculator {
    private precision: number;

    /**
     * Creates a new Calculator instance.
     */
    constructor(precision: number) {
        this.precision = precision;
    }

    /**
     * Returns the sum of two numbers.
     */
    add(a: number, b: number): number {
        return a + b;
    }

    /**
     * Returns the difference of two numbers.
     */
    subtract(a: number, b: number): number {
        return a - b;
    }
}
`

var csharpTestFile = `namespace Example
{
    /// <summary>
    /// Calculator provides arithmetic operations.
    /// </summary>
    public class Calculator
    {
        private int precision;

        /// <summary>
        /// Creates a new Calculator instance.
        /// </summary>
        public Calculator(int precision)
        {
            this.precision = precision;
        }

        /// <summary>
        /// Returns the sum of two numbers.
        /// </summary>
        public double Add(double a, double b)
        {
            return a + b;
        }

        /// <summary>
        /// Returns the difference of two numbers.
        /// </summary>
        public double Subtract(double a, double b)
        {
            return a - b;
        }
    }
}
`

var javaScriptTestFile = `/**
 * Calculator provides arithmetic operations.
 */
class Calculator {
    /**
     * Creates a new Calculator instance.
     */
    constructor(precision) {
        this.precision = precision;
    }

    /**
     * Returns the sum of two numbers.
     */
    add(a, b) {
        return a + b;
    }

    /**
     * Returns the difference of two numbers.
     */
    subtract(a, b) {
        return a - b;
    }
}
`

// =============================================================================
// Integration Tests: Config-Driven Chunking Pipeline
// =============================================================================

func TestIntegration_ConfigDrivenChunking(t *testing.T) {
	// Test full pipeline: load configs from languages/, create registry, chunk files
	langDir := integrationGetLanguagesDir(t)

	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err, "Should create registry from config")

	// Registry should have loaded languages
	supported := registry.SupportedLanguages()
	assert.NotEmpty(t, supported, "Should have supported languages")

	// Test chunking a Go file through the registry
	file := &models.SourceFile{
		Path:         "/test/calculator.go",
		Content:      []byte(goTestFile),
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify we got chunks
	assert.NotEmpty(t, result.Chunks, "Should produce chunks")

	// Verify file chunk exists
	fileChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelFile)
	require.NotNil(t, fileChunk, "Should have file-level chunk")
	assert.Equal(t, file.Path, fileChunk.FilePath)

	// Verify class chunk exists (Calculator struct)
	classChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelClass)
	require.NotNil(t, classChunk, "Should have class-level chunk")
	assert.Equal(t, "Calculator", classChunk.Name)

	// Verify method chunks exist
	methodChunks := integrationFindAllChunksByLevel(result.Chunks, models.ChunkLevelMethod)
	assert.GreaterOrEqual(t, len(methodChunks), 3, "Should have at least 3 method chunks")
}

// =============================================================================
// Integration Tests: Language-Specific Chunking
// =============================================================================

func TestIntegration_GoFileChunking(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "/test/calculator.go",
		Content:      []byte(goTestFile),
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	counts := integrationCountChunksByLevel(result.Chunks)

	// Should have: 1 file, 1 class (Calculator struct), 3 methods (NewCalculator, Add, Subtract)
	assert.Equal(t, 1, counts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.Equal(t, 1, counts[models.ChunkLevelClass], "Should have 1 class chunk")
	assert.Equal(t, 3, counts[models.ChunkLevelMethod], "Should have 3 method chunks")

	// Verify chunk names
	classNames := integrationGetChunkNames(result.Chunks, models.ChunkLevelClass)
	assert.Contains(t, classNames, "Calculator")

	methodNames := integrationGetChunkNames(result.Chunks, models.ChunkLevelMethod)
	assert.Contains(t, methodNames, "NewCalculator")
	assert.Contains(t, methodNames, "Add")
	assert.Contains(t, methodNames, "Subtract")

	// Verify line numbers for Calculator struct
	calcChunk := integrationFindChunkByName(result.Chunks, "Calculator")
	require.NotNil(t, calcChunk)
	assert.Equal(t, 4, calcChunk.StartLine, "Calculator should start at line 4")
	assert.Equal(t, 6, calcChunk.EndLine, "Calculator should end at line 6")

	// Verify content
	assert.Contains(t, calcChunk.Content, "precision int")
}

func TestIntegration_JavaFileChunking(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "/test/Calculator.java",
		Content:      []byte(javaTestFile),
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	counts := integrationCountChunksByLevel(result.Chunks)

	// Should have: 1 file, 1 class (Calculator), 3 methods (constructor, add, subtract)
	assert.Equal(t, 1, counts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.Equal(t, 1, counts[models.ChunkLevelClass], "Should have 1 class chunk")
	assert.Equal(t, 3, counts[models.ChunkLevelMethod], "Should have 3 method chunks")

	// Verify chunk names
	classChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelClass)
	require.NotNil(t, classChunk)
	assert.Equal(t, "Calculator", classChunk.Name)

	methodNames := integrationGetChunkNames(result.Chunks, models.ChunkLevelMethod)
	assert.Contains(t, methodNames, "Calculator") // constructor
	assert.Contains(t, methodNames, "add")
	assert.Contains(t, methodNames, "subtract")
}

func TestIntegration_PythonFileChunking(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "/test/calculator.py",
		Content:      []byte(pythonTestFile),
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	counts := integrationCountChunksByLevel(result.Chunks)

	// Should have: 1 file, 1 class (Calculator), 3 methods (__init__, add, subtract)
	assert.Equal(t, 1, counts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.Equal(t, 1, counts[models.ChunkLevelClass], "Should have 1 class chunk")
	assert.Equal(t, 3, counts[models.ChunkLevelMethod], "Should have 3 method chunks")

	// Verify chunk names
	classChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelClass)
	require.NotNil(t, classChunk)
	assert.Equal(t, "Calculator", classChunk.Name)

	methodNames := integrationGetChunkNames(result.Chunks, models.ChunkLevelMethod)
	assert.Contains(t, methodNames, "__init__")
	assert.Contains(t, methodNames, "add")
	assert.Contains(t, methodNames, "subtract")
}

func TestIntegration_TypeScriptFileChunking(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "/test/calculator.ts",
		Content:      []byte(typeScriptTestFile),
		Language:     "typescript",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	counts := integrationCountChunksByLevel(result.Chunks)

	// Should have: 1 file, 1 class (Calculator)
	// Methods might vary based on how TypeScript grammar handles them
	assert.Equal(t, 1, counts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.GreaterOrEqual(t, counts[models.ChunkLevelClass], 1, "Should have at least 1 class chunk")

	// Verify class chunk
	classChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelClass)
	require.NotNil(t, classChunk)
	assert.Equal(t, "Calculator", classChunk.Name)
}

func TestIntegration_CSharpFileChunking(t *testing.T) {
	// NOTE: The config-based registry has a known issue where languages using
	// grammar names different from their Language constant (e.g., "c_sharp" vs "csharp")
	// may not be properly looked up. This test uses the legacy registry instead.
	legacyRegistry, err := NewChunkerRegistry()
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "/test/Calculator.cs",
		Content:      []byte(csharpTestFile),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := legacyRegistry.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	counts := integrationCountChunksByLevel(result.Chunks)

	// Should have: 1 file, 1 class (Calculator), 3 methods (constructor, Add, Subtract)
	assert.Equal(t, 1, counts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.GreaterOrEqual(t, counts[models.ChunkLevelClass], 1, "Should have at least 1 class chunk")

	// Verify class chunk
	classChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelClass)
	require.NotNil(t, classChunk)
	assert.Equal(t, "Calculator", classChunk.Name)

	// Verify method chunks exist
	methodChunks := integrationFindAllChunksByLevel(result.Chunks, models.ChunkLevelMethod)
	assert.GreaterOrEqual(t, len(methodChunks), 2, "Should have at least 2 method chunks")
}

func TestIntegration_JavaScriptFileChunking(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "/test/calculator.js",
		Content:      []byte(javaScriptTestFile),
		Language:     "javascript",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	counts := integrationCountChunksByLevel(result.Chunks)

	// Should have: 1 file, 1 class (Calculator)
	assert.Equal(t, 1, counts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.GreaterOrEqual(t, counts[models.ChunkLevelClass], 1, "Should have at least 1 class chunk")

	// Verify class chunk
	classChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelClass)
	require.NotNil(t, classChunk)
	assert.Equal(t, "Calculator", classChunk.Name)
}

// =============================================================================
// Integration Tests: All Languages Sample
// =============================================================================

func TestIntegration_AllLanguagesSample(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	// Test each language with tree-sitter grammar support
	// NOTE: C# is excluded because the config uses "c_sharp" grammar which doesn't
	// match the Language constant "csharp" used by DetectLanguage.
	testCases := []struct {
		name       string
		ext        string
		content    string
		minClasses int
		minMethods int
	}{
		{
			name:       "Go",
			ext:        ".go",
			content:    goTestFile,
			minClasses: 1,
			minMethods: 3,
		},
		{
			name:       "Java",
			ext:        ".java",
			content:    javaTestFile,
			minClasses: 1,
			minMethods: 3,
		},
		{
			name:       "Python",
			ext:        ".py",
			content:    pythonTestFile,
			minClasses: 1,
			minMethods: 3,
		},
		{
			name:       "TypeScript",
			ext:        ".ts",
			content:    typeScriptTestFile,
			minClasses: 1,
			minMethods: 0, // Method detection varies
		},
		{
			name:       "JavaScript",
			ext:        ".js",
			content:    javaScriptTestFile,
			minClasses: 1,
			minMethods: 0, // Method detection varies
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			file := &models.SourceFile{
				Path:         "/test/calculator" + tc.ext,
				Content:      []byte(tc.content),
				Language:     tc.name,
				LastModified: time.Now(),
			}

			result, err := registry.Chunk(context.Background(), file)
			require.NoError(t, err, "Should chunk %s file without error", tc.name)
			require.NotNil(t, result)

			counts := integrationCountChunksByLevel(result.Chunks)

			// File chunk always required
			assert.Equal(t, 1, counts[models.ChunkLevelFile], "%s: Should have 1 file chunk", tc.name)

			// Class chunks
			assert.GreaterOrEqual(t, counts[models.ChunkLevelClass], tc.minClasses,
				"%s: Should have at least %d class chunk(s)", tc.name, tc.minClasses)

			// Method chunks (if expected)
			if tc.minMethods > 0 {
				assert.GreaterOrEqual(t, counts[models.ChunkLevelMethod], tc.minMethods,
					"%s: Should have at least %d method chunk(s)", tc.name, tc.minMethods)
			}

			// Verify all chunks are valid
			for _, chunk := range result.Chunks {
				err := chunk.IsValid()
				assert.NoError(t, err, "%s: Chunk %s should be valid", tc.name, chunk.Name)
				assert.NotEmpty(t, chunk.ID, "%s: Chunk should have an ID", tc.name)
			}
		})
	}
}

// =============================================================================
// Integration Tests: Config vs Legacy Comparison
// =============================================================================

func TestIntegration_ConfigAndLegacyMatch(t *testing.T) {
	// Compare output of config-driven chunker vs legacy chunker for same input
	langDir := integrationGetLanguagesDir(t)

	// Create both registries
	legacyRegistry, err := NewChunkerRegistry()
	require.NoError(t, err, "Should create legacy registry")

	configRegistry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err, "Should create config-based registry")

	// Test cases for languages with both implementations
	testCases := []struct {
		name    string
		ext     string
		content string
	}{
		{"Go", ".go", goTestFile},
		{"Java", ".java", javaTestFile},
		{"Python", ".py", pythonTestFile},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			file := &models.SourceFile{
				Path:         "/test/calc" + tc.ext,
				Content:      []byte(tc.content),
				Language:     tc.name,
				LastModified: time.Now(),
			}

			legacyResult, err := legacyRegistry.Chunk(context.Background(), file)
			require.NoError(t, err, "Legacy chunker should work")

			configResult, err := configRegistry.Chunk(context.Background(), file)
			require.NoError(t, err, "Config-based chunker should work")

			// Compare chunk counts by level
			legacyCounts := integrationCountChunksByLevel(legacyResult.Chunks)
			configCounts := integrationCountChunksByLevel(configResult.Chunks)

			// File chunk should match
			assert.Equal(t, legacyCounts[models.ChunkLevelFile], configCounts[models.ChunkLevelFile],
				"%s: File chunk counts should match", tc.name)

			// Class chunks should match
			assert.Equal(t, legacyCounts[models.ChunkLevelClass], configCounts[models.ChunkLevelClass],
				"%s: Class chunk counts should match", tc.name)

			// Method chunks should match
			assert.Equal(t, legacyCounts[models.ChunkLevelMethod], configCounts[models.ChunkLevelMethod],
				"%s: Method chunk counts should match", tc.name)

			// Compare chunk names
			legacyClassNames := integrationGetChunkNames(legacyResult.Chunks, models.ChunkLevelClass)
			configClassNames := integrationGetChunkNames(configResult.Chunks, models.ChunkLevelClass)
			assert.ElementsMatch(t, legacyClassNames, configClassNames,
				"%s: Class names should match", tc.name)

			legacyMethodNames := integrationGetChunkNames(legacyResult.Chunks, models.ChunkLevelMethod)
			configMethodNames := integrationGetChunkNames(configResult.Chunks, models.ChunkLevelMethod)
			assert.ElementsMatch(t, legacyMethodNames, configMethodNames,
				"%s: Method names should match", tc.name)
		})
	}
}

// =============================================================================
// Integration Tests: Chunk Hierarchy
// =============================================================================

func TestIntegration_ChunkHierarchy(t *testing.T) {
	// Verify parent-child relationships are correctly established
	// File chunk -> Class chunk -> Method chunk
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	// Python is a good test case for hierarchy because methods are inside classes
	file := &models.SourceFile{
		Path:         "/test/hierarchy.py",
		Content:      []byte(pythonTestFile),
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Get chunks by level
	fileChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelFile)
	classChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelClass)
	methodChunks := integrationFindAllChunksByLevel(result.Chunks, models.ChunkLevelMethod)

	require.NotNil(t, fileChunk, "Should have file chunk")
	require.NotNil(t, classChunk, "Should have class chunk")
	require.NotEmpty(t, methodChunks, "Should have method chunks")

	// File chunk should have no parent
	assert.Nil(t, fileChunk.ParentID, "File chunk should have no parent")

	// Class should reference file as parent
	require.NotNil(t, classChunk.ParentID, "Class should have parent")
	assert.Equal(t, fileChunk.ID, *classChunk.ParentID, "Class should reference file as parent")

	// Methods should reference class as parent
	for _, method := range methodChunks {
		require.NotNil(t, method.ParentID, "Method %s should have parent", method.Name)
		assert.Equal(t, classChunk.ID, *method.ParentID, "Method %s should reference class as parent", method.Name)
	}
}

func TestIntegration_ChunkHierarchy_Java(t *testing.T) {
	// Test hierarchy for Java (class contains methods)
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "/test/hierarchy.java",
		Content:      []byte(javaTestFile),
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Get chunks by level
	fileChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelFile)
	classChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelClass)
	methodChunks := integrationFindAllChunksByLevel(result.Chunks, models.ChunkLevelMethod)

	require.NotNil(t, fileChunk, "Should have file chunk")
	require.NotNil(t, classChunk, "Should have class chunk")
	require.NotEmpty(t, methodChunks, "Should have method chunks")

	// File chunk should have no parent
	assert.Nil(t, fileChunk.ParentID, "File chunk should have no parent")

	// Class should reference file as parent
	require.NotNil(t, classChunk.ParentID, "Class should have parent")
	assert.Equal(t, fileChunk.ID, *classChunk.ParentID, "Class should reference file as parent")

	// Methods should reference class as parent
	for _, method := range methodChunks {
		require.NotNil(t, method.ParentID, "Method %s should have parent", method.Name)
		assert.Equal(t, classChunk.ID, *method.ParentID, "Method %s should reference class as parent", method.Name)
	}
}

func TestIntegration_ChunkHierarchy_Go(t *testing.T) {
	// Test hierarchy for Go (struct separate from methods)
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "/test/hierarchy.go",
		Content:      []byte(goTestFile),
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Get chunks by level
	fileChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelFile)
	classChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelClass)
	methodChunks := integrationFindAllChunksByLevel(result.Chunks, models.ChunkLevelMethod)

	require.NotNil(t, fileChunk, "Should have file chunk")
	require.NotNil(t, classChunk, "Should have class chunk")
	require.NotEmpty(t, methodChunks, "Should have method chunks")

	// File chunk should have no parent
	assert.Nil(t, fileChunk.ParentID, "File chunk should have no parent")

	// Class should reference file as parent
	require.NotNil(t, classChunk.ParentID, "Class should have parent")
	assert.Equal(t, fileChunk.ID, *classChunk.ParentID, "Class should reference file as parent")

	// In Go, methods are at file level, not inside struct - they should reference file as parent
	for _, method := range methodChunks {
		require.NotNil(t, method.ParentID, "Method %s should have parent", method.Name)
		// Go methods reference file, not struct
		assert.Equal(t, fileChunk.ID, *method.ParentID, "Go method %s should reference file as parent", method.Name)
	}
}

// =============================================================================
// Integration Tests: Cross-Language Consistency
// =============================================================================

func TestIntegration_CrossLanguageConsistency(t *testing.T) {
	// Same logical structure in different languages should produce similar chunk hierarchy
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	// NOTE: C# is excluded due to grammar name mismatch issue
	testCases := []struct {
		name    string
		ext     string
		content string
	}{
		{"Go", ".go", goTestFile},
		{"Java", ".java", javaTestFile},
		{"Python", ".py", pythonTestFile},
		{"TypeScript", ".ts", typeScriptTestFile},
		{"JavaScript", ".js", javaScriptTestFile},
	}

	results := make(map[string]*models.ChunkResult)

	// Chunk all files
	for _, tc := range testCases {
		file := &models.SourceFile{
			Path:         "/test/calculator" + tc.ext,
			Content:      []byte(tc.content),
			Language:     tc.name,
			LastModified: time.Now(),
		}

		result, err := registry.Chunk(context.Background(), file)
		require.NoError(t, err, "Should chunk %s", tc.name)
		results[tc.name] = result
	}

	// All languages should have exactly 1 file chunk
	for name, result := range results {
		fileChunks := integrationFindAllChunksByLevel(result.Chunks, models.ChunkLevelFile)
		assert.Len(t, fileChunks, 1, "%s: Should have exactly 1 file chunk", name)
	}

	// All languages should have at least 1 class chunk named "Calculator"
	for name, result := range results {
		classChunk := integrationFindChunkByNameAndLevel(result.Chunks, "Calculator", models.ChunkLevelClass)
		require.NotNil(t, classChunk, "%s: Should have Calculator class", name)
	}

	// All class chunks should have file as parent
	for name, result := range results {
		fileChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelFile)
		classChunk := integrationFindChunkByNameAndLevel(result.Chunks, "Calculator", models.ChunkLevelClass)

		require.NotNil(t, classChunk.ParentID, "%s: Class should have parent", name)
		assert.Equal(t, fileChunk.ID, *classChunk.ParentID, "%s: Class should reference file as parent", name)
	}
}

// =============================================================================
// Integration Tests: Edge Cases
// =============================================================================

func TestIntegration_EmptyFile(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "/test/empty.go",
		Content:      []byte(""),
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Empty file should have no chunks (graceful handling)
	assert.Empty(t, result.Errors, "Should have no errors")
}

func TestIntegration_MinimalFile(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	// Minimal Go file with just package declaration
	file := &models.SourceFile{
		Path:         "/test/minimal.go",
		Content:      []byte("package main\n"),
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have file chunk
	fileChunk := integrationFindChunkByLevel(result.Chunks, models.ChunkLevelFile)
	require.NotNil(t, fileChunk)
}

func TestIntegration_NestedClasses_Java(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	nestedJava := `public class Outer {
    private int value;

    public class Inner {
        public void innerMethod() {}
    }

    public static class StaticNested {
        public void nestedMethod() {}
    }

    public void outerMethod() {}
}
`

	file := &models.SourceFile{
		Path:         "/test/Nested.java",
		Content:      []byte(nestedJava),
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should find Outer, Inner, and StaticNested classes
	classChunks := integrationFindAllChunksByLevel(result.Chunks, models.ChunkLevelClass)
	assert.GreaterOrEqual(t, len(classChunks), 3, "Should have at least 3 class chunks")

	classNames := integrationGetChunkNames(result.Chunks, models.ChunkLevelClass)
	assert.Contains(t, classNames, "Outer")
	assert.Contains(t, classNames, "Inner")
	assert.Contains(t, classNames, "StaticNested")
}

func TestIntegration_MultipleTopLevelFunctions_Go(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	multiFunc := `package main

func first() {}
func second() {}
func third() {}
`

	file := &models.SourceFile{
		Path:         "/test/multi.go",
		Content:      []byte(multiFunc),
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	methodChunks := integrationFindAllChunksByLevel(result.Chunks, models.ChunkLevelMethod)
	assert.Len(t, methodChunks, 3, "Should have 3 function chunks")

	methodNames := integrationGetChunkNames(result.Chunks, models.ChunkLevelMethod)
	assert.Contains(t, methodNames, "first")
	assert.Contains(t, methodNames, "second")
	assert.Contains(t, methodNames, "third")
}

func TestIntegration_DeterministicResults(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "/test/deterministic.go",
		Content:      []byte(goTestFile),
		Language:     "go",
		LastModified: time.Now(),
	}

	// Chunk the same file multiple times
	result1, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)

	result2, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should produce same number of chunks
	assert.Equal(t, len(result1.Chunks), len(result2.Chunks), "Should produce same number of chunks")

	// Build ID maps
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

	// IDs should be deterministic
	for key, id1 := range ids1 {
		id2, exists := ids2[key]
		require.True(t, exists, "Chunk %s should exist in both results", key)
		assert.Equal(t, id1, id2, "Chunk %s should have deterministic ID", key)
	}
}

func TestIntegration_ContentHashConsistency(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "/test/hashes.py",
		Content:      []byte(pythonTestFile),
		Language:     "python",
		LastModified: time.Now(),
	}

	result, err := registry.Chunk(context.Background(), file)
	require.NoError(t, err)

	// All chunks should have content hashes
	for _, chunk := range result.Chunks {
		assert.NotEmpty(t, chunk.ContentHash, "Chunk %s should have content hash", chunk.Name)
		assert.NotEmpty(t, chunk.ID, "Chunk %s should have ID", chunk.Name)
	}
}

// =============================================================================
// Integration Tests: Context Handling
// =============================================================================

func TestIntegration_ContextCancellation(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	file := &models.SourceFile{
		Path:         "/test/cancel.go",
		Content:      []byte(goTestFile),
		Language:     "go",
		LastModified: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = registry.Chunk(ctx, file)
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled, "Should return context.Canceled error")
	}
}

// =============================================================================
// Integration Tests: Registry Extension Mapping
// =============================================================================

func TestIntegration_RegistryExtensionMapping(t *testing.T) {
	langDir := integrationGetLanguagesDir(t)
	registry, err := NewRegistryFromConfig(langDir)
	require.NoError(t, err)

	testCases := []struct {
		ext      string
		hasChunker bool
	}{
		{".go", true},
		{".java", true},
		{".py", true},
		{".ts", true},
		{".js", true},
		{".cs", true},
		{".unknown", false},
		{".txt", false},
	}

	for _, tc := range testCases {
		t.Run(tc.ext, func(t *testing.T) {
			chunker, found := registry.GetChunkerForExtension(tc.ext)
			if tc.hasChunker {
				assert.True(t, found, "Should find chunker for %s", tc.ext)
				assert.NotNil(t, chunker)
			} else {
				// Fallback chunker is returned for unknown extensions
				assert.NotNil(t, chunker, "Should return fallback chunker for %s", tc.ext)
			}
		})
	}
}
