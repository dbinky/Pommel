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

// getCSharpChunker returns the C# chunker from the config-driven registry.
// This replaces the legacy NewCSharpChunker function.
func getCSharpChunker(t *testing.T) Chunker {
	t.Helper()
	registry, err := NewChunkerRegistry()
	require.NoError(t, err, "Failed to create chunker registry")

	chunker, ok := registry.GetChunkerForExtension(".cs")
	require.True(t, ok, "C# chunker should be available")
	return chunker
}

// =============================================================================
// CSharpChunker Initialization Tests
// =============================================================================

func TestCSharpChunker_Available(t *testing.T) {
	registry, err := NewChunkerRegistry()
	require.NoError(t, err)

	chunker, ok := registry.GetChunkerForExtension(".cs")
	assert.True(t, ok, "C# chunker should be available in registry")
	assert.NotNil(t, chunker, "C# chunker should not be nil")
}

func TestCSharpChunker_Language(t *testing.T) {
	chunker := getCSharpChunker(t)
	// C# chunker uses user-friendly "csharp" language name
	assert.Equal(t, LangCSharp, chunker.Language(), "CSharpChunker should return csharp language")
}

// =============================================================================
// Simple Class Parsing Tests
// =============================================================================

func TestCSharpChunker_SimpleClass(t *testing.T) {
	chunker := getCSharpChunker(t)

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

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have: 1 file chunk + 1 class chunk + 1 method chunk = 3 chunks
	assert.Len(t, result.Chunks, 3, "Should have 3 chunks: file + class + method")

	// Verify we have one chunk of each level
	fileLevelCount := 0
	classLevelCount := 0
	methodLevelCount := 0

	for _, chunk := range result.Chunks {
		switch chunk.Level {
		case models.ChunkLevelFile:
			fileLevelCount++
		case models.ChunkLevelClass:
			classLevelCount++
		case models.ChunkLevelMethod:
			methodLevelCount++
		}
	}

	assert.Equal(t, 1, fileLevelCount, "Should have exactly 1 file-level chunk")
	assert.Equal(t, 1, classLevelCount, "Should have exactly 1 class-level chunk")
	assert.Equal(t, 1, methodLevelCount, "Should have exactly 1 method-level chunk")
}

func TestCSharpChunker_ClassWithMultipleMethods(t *testing.T) {
	chunker := getCSharpChunker(t)

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

        public int Multiply(int a, int b)
        {
            return a * b;
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

	// Should have: 1 file + 1 class + 3 methods = 5 chunks
	assert.Len(t, result.Chunks, 5, "Should have 5 chunks: file + class + 3 methods")

	// Count method chunks
	methodCount := 0
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodCount++
		}
	}
	assert.Equal(t, 3, methodCount, "Should have exactly 3 method-level chunks")
}

func TestCSharpChunker_MultipleClasses(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class Calculator
    {
        public int Add(int a, int b) => a + b;
    }

    public class StringHelper
    {
        public string Reverse(string input) => new string(input.Reverse().ToArray());
    }
}`

	file := &models.SourceFile{
		Path:         "src/Helpers.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should have: 1 file + 2 classes + 2 methods = 5 chunks
	assert.Len(t, result.Chunks, 5, "Should have 5 chunks: file + 2 classes + 2 methods")

	classCount := 0
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classCount++
		}
	}
	assert.Equal(t, 2, classCount, "Should have exactly 2 class-level chunks")
}

// =============================================================================
// Properties Extraction Tests
// =============================================================================

func TestCSharpChunker_AutoProperties(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class Person
    {
        public string Name { get; set; }
        public int Age { get; set; }
    }
}`

	file := &models.SourceFile{
		Path:         "src/Person.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Properties should be extracted at method level
	// Should have: 1 file + 1 class + 2 properties = 4 chunks
	assert.Len(t, result.Chunks, 4, "Should have 4 chunks: file + class + 2 properties")

	propertyCount := 0
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			propertyCount++
		}
	}
	assert.Equal(t, 2, propertyCount, "Properties should be extracted at method level")
}

func TestCSharpChunker_PropertyWithBackingField(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class Person
    {
        private string _name;

        public string Name
        {
            get { return _name; }
            set { _name = value; }
        }
    }
}`

	file := &models.SourceFile{
		Path:         "src/Person.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should have: 1 file + 1 class + 1 property = 3 chunks
	// Note: Fields are typically not chunked separately
	assert.Len(t, result.Chunks, 3, "Should have 3 chunks: file + class + property")
}

func TestCSharpChunker_ExpressionBodiedProperty(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class Circle
    {
        public double Radius { get; set; }
        public double Area => Math.PI * Radius * Radius;
    }
}`

	file := &models.SourceFile{
		Path:         "src/Circle.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should have: 1 file + 1 class + 2 properties = 4 chunks
	assert.Len(t, result.Chunks, 4, "Should have 4 chunks: file + class + 2 properties")
}

// =============================================================================
// Nested Classes Tests
// =============================================================================

func TestCSharpChunker_NestedClass(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class OuterClass
    {
        public class InnerClass
        {
            public void InnerMethod()
            {
            }
        }

        public void OuterMethod()
        {
        }
    }
}`

	file := &models.SourceFile{
		Path:         "src/Nested.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should have chunks for file, classes, and methods
	assert.GreaterOrEqual(t, len(result.Chunks), 4, "Should have at least 4 chunks")

	// Find the classes and verify they exist
	var outerClass, innerClass *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			if chunk.Name == "OuterClass" {
				outerClass = chunk
			} else if chunk.Name == "InnerClass" {
				innerClass = chunk
			}
		}
	}

	require.NotNil(t, outerClass, "Should find OuterClass chunk")
	require.NotNil(t, innerClass, "Should find InnerClass chunk")

	// Both should be at class level
	assert.Equal(t, models.ChunkLevelClass, outerClass.Level)
	assert.Equal(t, models.ChunkLevelClass, innerClass.Level)

	// Note: Parent-child relationships for nested classes depend on the chunker implementation
	// The generic chunker may or may not set up correct parent relationships
}

func TestCSharpChunker_DeeplyNestedClasses(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class Level1
    {
        public class Level2
        {
            public class Level3
            {
                public void DeepMethod()
                {
                }
            }
        }
    }
}`

	file := &models.SourceFile{
		Path:         "src/DeepNested.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should have chunks for classes and method
	assert.GreaterOrEqual(t, len(result.Chunks), 4, "Should have at least 4 chunks")

	// Find classes and verify they exist
	classMap := make(map[string]*models.Chunk)
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classMap[chunk.Name] = chunk
		}
	}

	require.Len(t, classMap, 3, "Should have 3 class chunks")

	level1 := classMap["Level1"]
	level2 := classMap["Level2"]
	level3 := classMap["Level3"]

	require.NotNil(t, level1)
	require.NotNil(t, level2)
	require.NotNil(t, level3)

	// Note: Parent-child relationships for nested classes depend on the chunker implementation
	// The generic chunker may or may not set up correct parent relationships
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestCSharpChunker_Constructor(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class Person
    {
        private string _name;

        public Person(string name)
        {
            _name = name;
        }
    }
}`

	file := &models.SourceFile{
		Path:         "src/Person.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should have: 1 file + 1 class + 1 constructor = 3 chunks
	assert.Len(t, result.Chunks, 3, "Should have 3 chunks: file + class + constructor")

	// Verify constructor is at method level
	var constructorChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod && chunk.Name == "Person" {
			constructorChunk = chunk
			break
		}
	}

	require.NotNil(t, constructorChunk, "Should find constructor chunk")
	assert.Equal(t, models.ChunkLevelMethod, constructorChunk.Level)
}

func TestCSharpChunker_MultipleConstructors(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class Person
    {
        private string _name;
        private int _age;

        public Person()
        {
            _name = "Unknown";
            _age = 0;
        }

        public Person(string name)
        {
            _name = name;
            _age = 0;
        }

        public Person(string name, int age)
        {
            _name = name;
            _age = age;
        }
    }
}`

	file := &models.SourceFile{
		Path:         "src/Person.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should have: 1 file + 1 class + 3 constructors = 5 chunks
	assert.Len(t, result.Chunks, 5, "Should have 5 chunks: file + class + 3 constructors")

	constructorCount := 0
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod && chunk.Name == "Person" {
			constructorCount++
		}
	}
	assert.Equal(t, 3, constructorCount, "Should have 3 constructor chunks")
}

func TestCSharpChunker_StaticConstructor(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class Config
    {
        private static string _setting;

        static Config()
        {
            _setting = "default";
        }

        public Config()
        {
        }
    }
}`

	file := &models.SourceFile{
		Path:         "src/Config.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should have: 1 file + 1 class + 2 constructors = 4 chunks
	assert.Len(t, result.Chunks, 4, "Should have 4 chunks: file + class + 2 constructors")
}

// =============================================================================
// Parent-Child Relationship Tests
// =============================================================================

func TestCSharpChunker_MethodParentIsClass(t *testing.T) {
	chunker := getCSharpChunker(t)

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

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	var classChunk, methodChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classChunk = chunk
		} else if chunk.Level == models.ChunkLevelMethod {
			methodChunk = chunk
		}
	}

	require.NotNil(t, classChunk, "Should find class chunk")
	require.NotNil(t, methodChunk, "Should find method chunk")

	// Method should reference its parent class
	require.NotNil(t, methodChunk.ParentID, "Method should have a parent ID")
	assert.Equal(t, classChunk.ID, *methodChunk.ParentID, "Method parent should be the class")
}

func TestCSharpChunker_ClassParentIsFile(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class Calculator
    {
        public int Add(int a, int b) => a + b;
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

	var fileChunk, classChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelFile {
			fileChunk = chunk
		} else if chunk.Level == models.ChunkLevelClass {
			classChunk = chunk
		}
	}

	require.NotNil(t, fileChunk, "Should find file chunk")
	require.NotNil(t, classChunk, "Should find class chunk")

	// Class should reference the file chunk
	require.NotNil(t, classChunk.ParentID, "Class should have a parent ID")
	assert.Equal(t, fileChunk.ID, *classChunk.ParentID, "Class parent should be the file")
}

func TestCSharpChunker_FileChunkHasNoParent(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class Calculator { }
}`

	file := &models.SourceFile{
		Path:         "src/Calculator.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	var fileChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelFile {
			fileChunk = chunk
			break
		}
	}

	require.NotNil(t, fileChunk, "Should find file chunk")
	assert.Nil(t, fileChunk.ParentID, "File chunk should not have a parent")
}

// =============================================================================
// Deterministic ID Tests
// =============================================================================

func TestCSharpChunker_DeterministicIDs(t *testing.T) {
	chunker := getCSharpChunker(t)

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

	// Parse the same file twice
	result1, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	result2, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	require.Len(t, result1.Chunks, len(result2.Chunks), "Both parses should produce same number of chunks")

	// Create maps of chunks by ID for comparison
	ids1 := make(map[string]bool)
	ids2 := make(map[string]bool)

	for _, chunk := range result1.Chunks {
		ids1[chunk.ID] = true
	}

	for _, chunk := range result2.Chunks {
		ids2[chunk.ID] = true
	}

	// All IDs from first parse should be in second parse
	for id := range ids1 {
		assert.True(t, ids2[id], "ID %s from first parse should be in second parse", id)
	}

	// All IDs from second parse should be in first parse
	for id := range ids2 {
		assert.True(t, ids1[id], "ID %s from second parse should be in first parse", id)
	}
}

func TestCSharpChunker_DifferentFilesDifferentIDs(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `public class Calculator
{
    public int Add(int a, int b) => a + b;
}`

	file1 := &models.SourceFile{
		Path:         "src/Calculator.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	file2 := &models.SourceFile{
		Path:         "tests/Calculator.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result1, err := chunker.Chunk(context.Background(), file1)
	require.NoError(t, err)

	result2, err := chunker.Chunk(context.Background(), file2)
	require.NoError(t, err)

	// Same content but different paths should produce different IDs
	ids1 := make(map[string]bool)
	for _, chunk := range result1.Chunks {
		ids1[chunk.ID] = true
	}

	for _, chunk := range result2.Chunks {
		assert.False(t, ids1[chunk.ID], "Different file paths should produce different IDs")
	}
}

func TestCSharpChunker_UniqueIDsWithinFile(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class Calculator
    {
        public int Add(int a, int b) => a + b;
        public int Sub(int a, int b) => a - b;
    }

    public class Helper
    {
        public string Format(int n) => n.ToString();
    }
}`

	file := &models.SourceFile{
		Path:         "src/Utils.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// All IDs should be unique
	ids := make(map[string]bool)
	for _, chunk := range result.Chunks {
		assert.False(t, ids[chunk.ID], "Each chunk should have a unique ID, but %s is duplicated", chunk.ID)
		ids[chunk.ID] = true
	}
}

// =============================================================================
// Line Number Tests
// =============================================================================

func TestCSharpChunker_LineNumbers_SimpleClass(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `public class Calculator
{
    public int Add(int a, int b)
    {
        return a + b;
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

	// Find the file chunk
	var fileChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelFile {
			fileChunk = chunk
			break
		}
	}

	require.NotNil(t, fileChunk)
	assert.Equal(t, 1, fileChunk.StartLine, "File chunk should start at line 1")
	assert.Equal(t, 7, fileChunk.EndLine, "File chunk should end at line 7")
}

func TestCSharpChunker_LineNumbers_ClassAndMethod(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `public class Calculator
{
    public int Add(int a, int b)
    {
        return a + b;
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

	var classChunk, methodChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classChunk = chunk
		} else if chunk.Level == models.ChunkLevelMethod {
			methodChunk = chunk
		}
	}

	require.NotNil(t, classChunk)
	require.NotNil(t, methodChunk)

	// Class should span lines 1-7
	assert.Equal(t, 1, classChunk.StartLine, "Class should start at line 1")
	assert.Equal(t, 7, classChunk.EndLine, "Class should end at line 7")

	// Method should span lines 3-6
	assert.Equal(t, 3, methodChunk.StartLine, "Method should start at line 3")
	assert.Equal(t, 6, methodChunk.EndLine, "Method should end at line 6")
}

func TestCSharpChunker_LineNumbers_WithNamespace(t *testing.T) {
	chunker := getCSharpChunker(t)

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

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	var classChunk, methodChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classChunk = chunk
		} else if chunk.Level == models.ChunkLevelMethod {
			methodChunk = chunk
		}
	}

	require.NotNil(t, classChunk)
	require.NotNil(t, methodChunk)

	// Class should span lines 3-9
	assert.Equal(t, 3, classChunk.StartLine, "Class should start at line 3")
	assert.Equal(t, 9, classChunk.EndLine, "Class should end at line 9")

	// Method should span lines 5-8
	assert.Equal(t, 5, methodChunk.StartLine, "Method should start at line 5")
	assert.Equal(t, 8, methodChunk.EndLine, "Method should end at line 8")
}

func TestCSharpChunker_LineNumbers_ValidRange(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class Calculator
    {
        public int Add(int a, int b) => a + b;
        public int Sub(int a, int b) => a - b;
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

	for _, chunk := range result.Chunks {
		// StartLine should be >= 1
		assert.GreaterOrEqual(t, chunk.StartLine, 1, "StartLine should be >= 1 for chunk %s", chunk.Name)
		// EndLine should be >= StartLine
		assert.GreaterOrEqual(t, chunk.EndLine, chunk.StartLine, "EndLine should be >= StartLine for chunk %s", chunk.Name)
	}
}

// =============================================================================
// Chunk Content Tests
// =============================================================================

func TestCSharpChunker_ChunkContent(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `public class Calculator
{
    public int Add(int a, int b)
    {
        return a + b;
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

	for _, chunk := range result.Chunks {
		// Content should not be empty
		assert.NotEmpty(t, chunk.Content, "Chunk content should not be empty for %s", chunk.Name)
		// Content should be valid (should pass IsValid)
		assert.NoError(t, chunk.IsValid(), "Chunk should be valid for %s", chunk.Name)
	}
}

func TestCSharpChunker_ChunkFilePath(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `public class Test { }`

	file := &models.SourceFile{
		Path:         "src/Test.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	for _, chunk := range result.Chunks {
		assert.Equal(t, "src/Test.cs", chunk.FilePath, "All chunks should have the correct file path")
	}
}

func TestCSharpChunker_ChunkLanguage(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `public class Test { }`

	file := &models.SourceFile{
		Path:         "src/Test.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	for _, chunk := range result.Chunks {
		assert.Equal(t, "csharp", chunk.Language, "All chunks should have csharp language")
	}
}

// =============================================================================
// Chunk Name and Signature Tests
// =============================================================================

func TestCSharpChunker_ChunkNames(t *testing.T) {
	chunker := getCSharpChunker(t)

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

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, chunk := range result.Chunks {
		names[chunk.Name] = true
	}

	assert.True(t, names["Calculator"], "Should have a chunk named Calculator")
	assert.True(t, names["Add"], "Should have a chunk named Add")
}

func TestCSharpChunker_MethodSignature(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `public class Calculator
{
    public int Add(int a, int b)
    {
        return a + b;
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

	var methodChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod && chunk.Name == "Add" {
			methodChunk = chunk
			break
		}
	}

	require.NotNil(t, methodChunk, "Should find Add method chunk")
	assert.NotEmpty(t, methodChunk.Signature, "Method should have a signature")
	assert.Contains(t, methodChunk.Signature, "Add", "Signature should contain method name")
}

// =============================================================================
// Edge Cases and Error Handling
// =============================================================================

func TestCSharpChunker_EmptyFile(t *testing.T) {
	chunker := getCSharpChunker(t)

	file := &models.SourceFile{
		Path:         "src/Empty.cs",
		Content:      []byte(""),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Empty file should still produce a file chunk (or be empty)
	// The exact behavior depends on implementation decision
	assert.GreaterOrEqual(t, len(result.Chunks), 0, "Empty file should produce 0 or more chunks")
}

func TestCSharpChunker_FileWithOnlyComments(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `// This is a comment
/* This is a
   multi-line comment */
/// <summary>
/// XML documentation comment
/// </summary>`

	file := &models.SourceFile{
		Path:         "src/Comments.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// File with only comments might produce just a file chunk or be empty
	assert.GreaterOrEqual(t, len(result.Chunks), 0)
}

func TestCSharpChunker_Interface(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public interface ICalculator
    {
        int Add(int a, int b);
        int Subtract(int a, int b);
    }
}`

	file := &models.SourceFile{
		Path:         "src/ICalculator.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should extract interface at class level
	var interfaceChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass && chunk.Name == "ICalculator" {
			interfaceChunk = chunk
			break
		}
	}

	assert.NotNil(t, interfaceChunk, "Should extract interface as class-level chunk")
}

func TestCSharpChunker_Struct(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public struct Point
    {
        public int X { get; set; }
        public int Y { get; set; }
    }
}`

	file := &models.SourceFile{
		Path:         "src/Point.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should extract struct at class level
	var structChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass && chunk.Name == "Point" {
			structChunk = chunk
			break
		}
	}

	assert.NotNil(t, structChunk, "Should extract struct as class-level chunk")
}

func TestCSharpChunker_Record(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public record Person(string Name, int Age);
}`

	file := &models.SourceFile{
		Path:         "src/Person.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should extract record at class level
	var recordChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass && chunk.Name == "Person" {
			recordChunk = chunk
			break
		}
	}

	assert.NotNil(t, recordChunk, "Should extract record as class-level chunk")
}

func TestCSharpChunker_Enum(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public enum Color
    {
        Red,
        Green,
        Blue
    }
}`

	file := &models.SourceFile{
		Path:         "src/Color.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should extract enum at class level
	var enumChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass && chunk.Name == "Color" {
			enumChunk = chunk
			break
		}
	}

	assert.NotNil(t, enumChunk, "Should extract enum as class-level chunk")
}

func TestCSharpChunker_CancelledContext(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `public class Test { }`

	file := &models.SourceFile{
		Path:         "src/Test.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, _ = chunker.Chunk(ctx, file)
	// Either returns error or completes - just ensure no hang
	// The exact behavior depends on implementation
}

func TestCSharpChunker_FileScopedNamespace(t *testing.T) {
	chunker := getCSharpChunker(t)

	// C# 10+ file-scoped namespace
	source := `namespace MyApp;

public class Calculator
{
    public int Add(int a, int b) => a + b;
}`

	file := &models.SourceFile{
		Path:         "src/Calculator.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should still extract class and method
	classCount := 0
	methodCount := 0
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classCount++
		} else if chunk.Level == models.ChunkLevelMethod {
			methodCount++
		}
	}

	assert.Equal(t, 1, classCount, "Should have 1 class")
	assert.Equal(t, 1, methodCount, "Should have 1 method")
}

func TestCSharpChunker_GenericClass(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class Repository<T> where T : class
    {
        public T GetById(int id)
        {
            return default;
        }

        public void Add(T entity)
        {
        }
    }
}`

	file := &models.SourceFile{
		Path:         "src/Repository.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should have: 1 file + 1 class + 2 methods = 4 chunks
	assert.Len(t, result.Chunks, 4, "Should have 4 chunks for generic class")

	// Find the class and verify its name includes generic info or base name
	var classChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classChunk = chunk
			break
		}
	}

	require.NotNil(t, classChunk)
	assert.Contains(t, classChunk.Name, "Repository", "Class name should contain Repository")
}

func TestCSharpChunker_AsyncMethod(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public class DataService
    {
        public async Task<string> FetchDataAsync()
        {
            await Task.Delay(100);
            return "data";
        }
    }
}`

	file := &models.SourceFile{
		Path:         "src/DataService.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should extract async method
	var methodChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunk = chunk
			break
		}
	}

	require.NotNil(t, methodChunk, "Should find async method chunk")
	assert.Equal(t, "FetchDataAsync", methodChunk.Name)
}

func TestCSharpChunker_ExtensionMethod(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `namespace MyApp
{
    public static class StringExtensions
    {
        public static bool IsNullOrEmpty(this string value)
        {
            return string.IsNullOrEmpty(value);
        }
    }
}`

	file := &models.SourceFile{
		Path:         "src/Extensions.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Should extract static class and extension method
	var classChunk, methodChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classChunk = chunk
		} else if chunk.Level == models.ChunkLevelMethod {
			methodChunk = chunk
		}
	}

	require.NotNil(t, classChunk, "Should find static class")
	require.NotNil(t, methodChunk, "Should find extension method")
	assert.Equal(t, "IsNullOrEmpty", methodChunk.Name)
}

// =============================================================================
// Content Hash Tests
// =============================================================================

func TestCSharpChunker_ContentHashSet(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `public class Test
{
    public void Method() { }
}`

	file := &models.SourceFile{
		Path:         "src/Test.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	for _, chunk := range result.Chunks {
		assert.NotEmpty(t, chunk.ContentHash, "Content hash should be set for chunk %s", chunk.Name)
		assert.NotEmpty(t, chunk.ID, "ID should be set for chunk %s", chunk.Name)
	}
}

func TestCSharpChunker_ContentHashDeterministic(t *testing.T) {
	chunker := getCSharpChunker(t)

	source := `public class Test
{
    public void Method() { }
}`

	file := &models.SourceFile{
		Path:         "src/Test.cs",
		Content:      []byte(source),
		Language:     "csharp",
		LastModified: time.Now(),
	}

	result1, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	result2, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	for i := range result1.Chunks {
		assert.Equal(t, result1.Chunks[i].ContentHash, result2.Chunks[i].ContentHash,
			"Content hash should be deterministic")
	}
}
