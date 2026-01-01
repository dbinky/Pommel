# Phase 7: Polish & Testing

**Phase Goal:** Achieve comprehensive test coverage, polish error handling, create documentation, and validate the system by dogfooding on the Pommel codebase itself.

**Prerequisites:** Phases 1-6 complete (all features implemented)

**Estimated Tasks:** 25 tasks across 6 areas

---

## Table of Contents

1. [Overview](#overview)
2. [Success Criteria](#success-criteria)
3. [Task 7.1: Unit Tests](#task-71-unit-tests)
4. [Task 7.2: Integration Tests](#task-72-integration-tests)
5. [Task 7.3: Dogfooding on Pommel](#task-73-dogfooding-on-pommel)
6. [Task 7.4: Error Handling Polish](#task-74-error-handling-polish)
7. [Task 7.5: Documentation](#task-75-documentation)
8. [Task 7.6: Performance Validation](#task-76-performance-validation)
9. [Dependencies](#dependencies)
10. [Risks and Mitigations](#risks-and-mitigations)

---

## Overview

Phase 7 is the final phase before v0.1 release. By the end of this phase:

- Unit test coverage exceeds 80% for all `internal/` packages
- Integration tests validate full indexing and search flows
- Pommel successfully indexes and searches its own codebase (dogfooding)
- Error messages are clear, actionable, and consistent
- README and CLI help text are complete and accurate
- Search latency meets target (< 100ms for typical queries)

This phase prioritizes **real-world validation** over synthetic testing. Dogfooding on Pommel's own Go codebase proves the system works for actual code search scenarios.

---

## Success Criteria

| Criterion | Validation |
|-----------|------------|
| Unit test coverage > 80% | `go test -cover` shows 80%+ |
| Integration tests pass | All `_integration_test.go` files pass |
| Dogfood test passes | Pommel indexes Pommel, searches find expected results |
| Error messages reviewed | Checklist of common errors verified |
| README complete | Covers install, setup, usage, configuration |
| Search latency < 100ms | Benchmark for 10-result queries |

---

## Task 7.1: Unit Tests

### 7.1.1 Embedder Package Tests

**Description:** Unit tests for the embedding pipeline with Ollama mocking.

**Files:**
- `internal/embedder/ollama_test.go`
- `internal/embedder/cache_test.go`
- `internal/embedder/embedder_test.go`

**Test Cases:**

```go
// internal/embedder/ollama_test.go
package embedder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaClient_GenerateEmbedding_Success(t *testing.T) {
	// Mock Ollama server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/embeddings", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Return mock embedding
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// 768-dimension embedding (truncated for example)
		embedding := make([]float64, 768)
		for i := range embedding {
			embedding[i] = float64(i) / 768.0
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"embedding": embedding,
		})
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "jina-embeddings-v2-base-code")

	embedding, err := client.GenerateEmbedding(context.Background(), "func main() {}")

	require.NoError(t, err)
	assert.Len(t, embedding, 768)
}

func TestOllamaClient_GenerateEmbedding_ConnectionError(t *testing.T) {
	client := NewOllamaClient("http://localhost:99999", "test-model")

	_, err := client.GenerateEmbedding(context.Background(), "test")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection")
}

func TestOllamaClient_GenerateEmbedding_ModelNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "model not found"}`))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "nonexistent-model")

	_, err := client.GenerateEmbedding(context.Background(), "test")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "model")
}

func TestOllamaClient_BatchEmbedding(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		embedding := make([]float64, 768)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"embedding": embedding,
		})
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "test-model")

	texts := []string{"code1", "code2", "code3"}
	embeddings, err := client.BatchEmbedding(context.Background(), texts)

	require.NoError(t, err)
	assert.Len(t, embeddings, 3)
	assert.Equal(t, 3, callCount)
}
```

```go
// internal/embedder/cache_test.go
package embedder

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryCache_GetOrCompute_CacheHit(t *testing.T) {
	cache := NewQueryCache(100, time.Minute)

	// Pre-populate
	expected := make([]float32, 768)
	expected[0] = 1.0
	cache.Set("test query", expected)

	callCount := 0
	compute := func(ctx context.Context, text string) ([]float32, error) {
		callCount++
		return nil, nil
	}

	result, err := cache.GetOrCompute(context.Background(), "test query", compute)

	require.NoError(t, err)
	assert.Equal(t, expected, result)
	assert.Equal(t, 0, callCount, "compute should not be called on cache hit")
}

func TestQueryCache_GetOrCompute_CacheMiss(t *testing.T) {
	cache := NewQueryCache(100, time.Minute)

	expected := make([]float32, 768)
	expected[0] = 2.0

	compute := func(ctx context.Context, text string) ([]float32, error) {
		return expected, nil
	}

	result, err := cache.GetOrCompute(context.Background(), "new query", compute)

	require.NoError(t, err)
	assert.Equal(t, expected, result)

	// Verify it's now cached
	cached, found := cache.Get("new query")
	assert.True(t, found)
	assert.Equal(t, expected, cached)
}

func TestQueryCache_Expiration(t *testing.T) {
	cache := NewQueryCache(100, 50*time.Millisecond)

	cache.Set("expiring", make([]float32, 768))

	_, found := cache.Get("expiring")
	assert.True(t, found)

	time.Sleep(60 * time.Millisecond)

	_, found = cache.Get("expiring")
	assert.False(t, found)
}

func TestQueryCache_LRUEviction(t *testing.T) {
	cache := NewQueryCache(3, time.Minute) // Only 3 entries

	cache.Set("a", make([]float32, 768))
	cache.Set("b", make([]float32, 768))
	cache.Set("c", make([]float32, 768))

	// Access "a" to make it recently used
	cache.Get("a")

	// Add "d" - should evict "b" (least recently used)
	cache.Set("d", make([]float32, 768))

	_, foundA := cache.Get("a")
	_, foundB := cache.Get("b")
	_, foundC := cache.Get("c")
	_, foundD := cache.Get("d")

	assert.True(t, foundA, "a should still exist")
	assert.False(t, foundB, "b should be evicted")
	assert.True(t, foundC, "c should still exist")
	assert.True(t, foundD, "d should exist")
}

func TestQueryCache_Concurrent(t *testing.T) {
	cache := NewQueryCache(100, time.Minute)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i%10)
			cache.Set(key, make([]float32, 768))
			cache.Get(key)
		}(i)
	}
	wg.Wait()

	// No race conditions or panics
}
```

**Acceptance Criteria:**
- All tests pass
- Coverage > 85% for embedder package
- Mock server tests validate HTTP interactions

---

### 7.1.2 Chunker Package Tests

**Description:** Unit tests for all language chunkers with real code samples.

**Files:**
- `internal/chunker/csharp_test.go`
- `internal/chunker/python_test.go`
- `internal/chunker/javascript_test.go`
- `internal/chunker/chunker_test.go`

**Test Cases:**

```go
// internal/chunker/csharp_test.go
package chunker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSharpChunker_ClassesAndMethods(t *testing.T) {
	code := `
namespace MyApp.Services
{
    public class UserService
    {
        private readonly IRepository _repo;

        public UserService(IRepository repo)
        {
            _repo = repo;
        }

        public User GetById(int id)
        {
            return _repo.Find<User>(id);
        }

        public async Task<User> CreateAsync(CreateUserRequest request)
        {
            var user = new User(request.Name, request.Email);
            await _repo.AddAsync(user);
            return user;
        }
    }

    internal class HelperClass
    {
        public static string Format(string input) => input.Trim();
    }
}
`

	chunker := NewCSharpChunker()
	chunks, err := chunker.Chunk("UserService.cs", []byte(code))

	require.NoError(t, err)

	// Should produce: 1 file + 2 classes + 4 methods
	assert.Len(t, chunks, 7)

	// Verify file-level chunk
	fileChunk := findChunkByLevel(chunks, LevelFile)
	require.NotNil(t, fileChunk)
	assert.Equal(t, "UserService.cs", fileChunk.Path)

	// Verify class chunks
	classChunks := findChunksByLevel(chunks, LevelClass)
	assert.Len(t, classChunks, 2)

	userServiceClass := findChunkByName(classChunks, "UserService")
	require.NotNil(t, userServiceClass)
	assert.Equal(t, "MyApp.Services", userServiceClass.ParentName)

	// Verify method chunks
	methodChunks := findChunksByLevel(chunks, LevelMethod)
	assert.Len(t, methodChunks, 4) // constructor + 2 methods + Format

	getByIdMethod := findChunkByName(methodChunks, "GetById")
	require.NotNil(t, getByIdMethod)
	assert.Equal(t, "UserService", getByIdMethod.ParentName)
	assert.Contains(t, getByIdMethod.Content, "return _repo.Find<User>(id)")
}

func TestCSharpChunker_Interfaces(t *testing.T) {
	code := `
public interface IUserService
{
    User GetById(int id);
    Task<User> CreateAsync(CreateUserRequest request);
}
`

	chunker := NewCSharpChunker()
	chunks, err := chunker.Chunk("IUserService.cs", []byte(code))

	require.NoError(t, err)

	// Interface should be at class level
	interfaceChunk := findChunkByName(findChunksByLevel(chunks, LevelClass), "IUserService")
	require.NotNil(t, interfaceChunk)
	assert.Equal(t, "interface", interfaceChunk.Kind)
}

func TestCSharpChunker_NestedClasses(t *testing.T) {
	code := `
public class Outer
{
    public class Inner
    {
        public void DoWork() { }
    }
}
`

	chunker := NewCSharpChunker()
	chunks, err := chunker.Chunk("Nested.cs", []byte(code))

	require.NoError(t, err)

	innerClass := findChunkByName(findChunksByLevel(chunks, LevelClass), "Inner")
	require.NotNil(t, innerClass)
	assert.Equal(t, "Outer", innerClass.ParentName)
}

func TestCSharpChunker_Properties(t *testing.T) {
	code := `
public class Person
{
    public string Name { get; set; }
    public int Age { get; private set; }

    public bool IsAdult => Age >= 18;
}
`

	chunker := NewCSharpChunker()
	chunks, err := chunker.Chunk("Person.cs", []byte(code))

	require.NoError(t, err)

	// Properties should be captured at method level
	methodChunks := findChunksByLevel(chunks, LevelMethod)
	assert.GreaterOrEqual(t, len(methodChunks), 3)
}

// Helper functions
func findChunkByLevel(chunks []Chunk, level ChunkLevel) *Chunk {
	for i := range chunks {
		if chunks[i].Level == level {
			return &chunks[i]
		}
	}
	return nil
}

func findChunksByLevel(chunks []Chunk, level ChunkLevel) []Chunk {
	var result []Chunk
	for _, c := range chunks {
		if c.Level == level {
			result = append(result, c)
		}
	}
	return result
}

func findChunkByName(chunks []Chunk, name string) *Chunk {
	for i := range chunks {
		if chunks[i].Name == name {
			return &chunks[i]
		}
	}
	return nil
}
```

```go
// internal/chunker/python_test.go
package chunker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPythonChunker_ClassesAndMethods(t *testing.T) {
	code := `
class UserService:
    """Service for managing users."""

    def __init__(self, repository):
        self.repository = repository

    def get_by_id(self, user_id: int) -> User:
        """Get user by ID."""
        return self.repository.find(user_id)

    async def create_user(self, name: str, email: str) -> User:
        """Create a new user."""
        user = User(name=name, email=email)
        await self.repository.add(user)
        return user

def helper_function(value):
    """A standalone helper function."""
    return value.strip()
`

	chunker := NewPythonChunker()
	chunks, err := chunker.Chunk("user_service.py", []byte(code))

	require.NoError(t, err)

	// File + 1 class + 3 methods + 1 function
	assert.Len(t, chunks, 6)

	// Verify class chunk
	classChunks := findChunksByLevel(chunks, LevelClass)
	assert.Len(t, classChunks, 1)
	assert.Equal(t, "UserService", classChunks[0].Name)

	// Verify method chunks include docstrings
	getByIdMethod := findChunkByName(findChunksByLevel(chunks, LevelMethod), "get_by_id")
	require.NotNil(t, getByIdMethod)
	assert.Contains(t, getByIdMethod.Content, "Get user by ID")

	// Verify standalone function
	helperFunc := findChunkByName(findChunksByLevel(chunks, LevelMethod), "helper_function")
	require.NotNil(t, helperFunc)
	assert.Empty(t, helperFunc.ParentName) // No parent class
}

func TestPythonChunker_Decorators(t *testing.T) {
	code := `
class MyClass:
    @staticmethod
    def static_method():
        pass

    @classmethod
    def class_method(cls):
        pass

    @property
    def my_property(self):
        return self._value
`

	chunker := NewPythonChunker()
	chunks, err := chunker.Chunk("decorators.py", []byte(code))

	require.NoError(t, err)

	// Verify decorators are included in method content
	staticMethod := findChunkByName(chunks, "static_method")
	require.NotNil(t, staticMethod)
	assert.Contains(t, staticMethod.Content, "@staticmethod")
}

func TestPythonChunker_NestedFunctions(t *testing.T) {
	code := `
def outer_function():
    def inner_function():
        return "inner"
    return inner_function()
`

	chunker := NewPythonChunker()
	chunks, err := chunker.Chunk("nested.py", []byte(code))

	require.NoError(t, err)

	// Inner function should have outer as parent
	innerFunc := findChunkByName(chunks, "inner_function")
	require.NotNil(t, innerFunc)
	assert.Equal(t, "outer_function", innerFunc.ParentName)
}
```

```go
// internal/chunker/javascript_test.go
package chunker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJavaScriptChunker_ES6Class(t *testing.T) {
	code := `
class UserService {
    constructor(repository) {
        this.repository = repository;
    }

    async getById(userId) {
        return await this.repository.find(userId);
    }

    create(name, email) {
        const user = { name, email };
        return this.repository.add(user);
    }
}

export default UserService;
`

	chunker := NewJavaScriptChunker()
	chunks, err := chunker.Chunk("userService.js", []byte(code))

	require.NoError(t, err)

	// File + 1 class + 3 methods
	classChunks := findChunksByLevel(chunks, LevelClass)
	assert.Len(t, classChunks, 1)
	assert.Equal(t, "UserService", classChunks[0].Name)

	methodChunks := findChunksByLevel(chunks, LevelMethod)
	assert.Len(t, methodChunks, 3)
}

func TestJavaScriptChunker_Functions(t *testing.T) {
	code := `
function regularFunction(arg) {
    return arg + 1;
}

const arrowFunction = (arg) => {
    return arg * 2;
};

const shortArrow = arg => arg + 3;

async function asyncFunction() {
    await doSomething();
}

export { regularFunction, arrowFunction };
`

	chunker := NewJavaScriptChunker()
	chunks, err := chunker.Chunk("functions.js", []byte(code))

	require.NoError(t, err)

	methodChunks := findChunksByLevel(chunks, LevelMethod)

	// All function types should be captured
	names := make(map[string]bool)
	for _, c := range methodChunks {
		names[c.Name] = true
	}

	assert.True(t, names["regularFunction"])
	assert.True(t, names["arrowFunction"])
	assert.True(t, names["shortArrow"])
	assert.True(t, names["asyncFunction"])
}

func TestTypeScriptChunker_Interfaces(t *testing.T) {
	code := `
interface User {
    id: number;
    name: string;
    email: string;
}

interface UserService {
    getById(id: number): Promise<User>;
    create(data: CreateUserDTO): Promise<User>;
}

class UserServiceImpl implements UserService {
    async getById(id: number): Promise<User> {
        return { id, name: "Test", email: "test@example.com" };
    }

    async create(data: CreateUserDTO): Promise<User> {
        return { ...data, id: 1 };
    }
}
`

	chunker := NewJavaScriptChunker() // Handles TypeScript too
	chunks, err := chunker.Chunk("userService.ts", []byte(code))

	require.NoError(t, err)

	// Interfaces should be at class level
	classChunks := findChunksByLevel(chunks, LevelClass)
	names := make(map[string]bool)
	for _, c := range classChunks {
		names[c.Name] = true
	}

	assert.True(t, names["User"])
	assert.True(t, names["UserService"])
	assert.True(t, names["UserServiceImpl"])
}

func TestJavaScriptChunker_ModuleExports(t *testing.T) {
	code := `
// CommonJS module
module.exports = {
    doSomething: function() {
        return "result";
    },

    doSomethingElse() {
        return "other";
    }
};
`

	chunker := NewJavaScriptChunker()
	chunks, err := chunker.Chunk("module.js", []byte(code))

	require.NoError(t, err)

	// Module exports functions should be captured
	methodChunks := findChunksByLevel(chunks, LevelMethod)
	assert.GreaterOrEqual(t, len(methodChunks), 2)
}
```

**Acceptance Criteria:**
- All language chunkers tested with realistic code
- Edge cases covered (nested classes, decorators, arrow functions)
- Parent hierarchy validated for all chunk types

---

### 7.1.3 Database Package Tests

**Description:** Unit tests for SQLite and sqlite-vec operations.

**Files:**
- `internal/db/sqlite_test.go`
- `internal/db/chunks_test.go`
- `internal/db/search_test.go`

**Test Cases:**

```go
// internal/db/sqlite_test.go
package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabase_Initialize(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Verify tables exist
	tables, err := db.ListTables()
	require.NoError(t, err)

	assert.Contains(t, tables, "chunks")
	assert.Contains(t, tables, "files")
	assert.Contains(t, tables, "chunk_embeddings")
}

func TestDatabase_VecExtensionLoaded(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Verify sqlite-vec is loaded
	var version string
	err = db.QueryRow("SELECT vec_version()").Scan(&version)
	require.NoError(t, err)
	assert.NotEmpty(t, version)
}

func TestDatabase_Migrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open and close to create initial schema
	db1, err := Open(dbPath)
	require.NoError(t, err)
	db1.Close()

	// Open again - should detect existing schema
	db2, err := Open(dbPath)
	require.NoError(t, err)
	defer db2.Close()

	// Should not error or duplicate tables
}
```

```go
// internal/db/chunks_test.go
package db

import (
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *Database {
	tmpDir := t.TempDir()
	db, err := Open(filepath.Join(tmpDir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestChunks_InsertAndRetrieve(t *testing.T) {
	db := setupTestDB(t)

	chunk := &models.Chunk{
		Path:       "src/main.go",
		Name:       "main",
		Level:      models.LevelMethod,
		Kind:       "function",
		Content:    "func main() { fmt.Println(\"hello\") }",
		StartLine:  1,
		EndLine:    3,
		ParentName: "",
		Language:   "go",
	}

	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = float32(i) / 768.0
	}

	id, err := db.InsertChunk(chunk, embedding)
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))

	// Retrieve by ID
	retrieved, err := db.GetChunk(id)
	require.NoError(t, err)
	assert.Equal(t, chunk.Path, retrieved.Path)
	assert.Equal(t, chunk.Name, retrieved.Name)
	assert.Equal(t, chunk.Content, retrieved.Content)
}

func TestChunks_UpdateFile(t *testing.T) {
	db := setupTestDB(t)

	// Insert initial chunks for a file
	chunk1 := &models.Chunk{
		Path:    "src/service.go",
		Name:    "OldMethod",
		Level:   models.LevelMethod,
		Content: "func OldMethod() {}",
	}

	embedding := make([]float32, 768)
	_, err := db.InsertChunk(chunk1, embedding)
	require.NoError(t, err)

	// Delete all chunks for file
	deleted, err := db.DeleteChunksByPath("src/service.go")
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	// Insert new chunks
	chunk2 := &models.Chunk{
		Path:    "src/service.go",
		Name:    "NewMethod",
		Level:   models.LevelMethod,
		Content: "func NewMethod() {}",
	}
	_, err = db.InsertChunk(chunk2, embedding)
	require.NoError(t, err)

	// Verify only new chunk exists
	chunks, err := db.GetChunksByPath("src/service.go")
	require.NoError(t, err)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "NewMethod", chunks[0].Name)
}

func TestChunks_BulkInsert(t *testing.T) {
	db := setupTestDB(t)

	chunks := make([]*models.Chunk, 100)
	embeddings := make([][]float32, 100)

	for i := 0; i < 100; i++ {
		chunks[i] = &models.Chunk{
			Path:    "src/large.go",
			Name:    fmt.Sprintf("Method%d", i),
			Level:   models.LevelMethod,
			Content: fmt.Sprintf("func Method%d() {}", i),
		}
		embeddings[i] = make([]float32, 768)
	}

	start := time.Now()
	err := db.BulkInsertChunks(chunks, embeddings)
	duration := time.Since(start)

	require.NoError(t, err)
	t.Logf("Bulk insert of 100 chunks took %v", duration)

	// Verify all inserted
	all, err := db.GetChunksByPath("src/large.go")
	require.NoError(t, err)
	assert.Len(t, all, 100)
}
```

```go
// internal/db/search_test.go
package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch_VectorSearch(t *testing.T) {
	db := setupTestDB(t)

	// Insert test chunks with distinct embeddings
	chunks := []struct {
		name      string
		content   string
		embedding []float32
	}{
		{
			name:      "createUser",
			content:   "func createUser(name string) error",
			embedding: createTestEmbedding(1.0, 0.0, 0.0), // "user creation" semantic
		},
		{
			name:      "deleteUser",
			content:   "func deleteUser(id int) error",
			embedding: createTestEmbedding(0.8, 0.1, 0.0), // Similar to createUser
		},
		{
			name:      "sendEmail",
			content:   "func sendEmail(to, subject string) error",
			embedding: createTestEmbedding(0.0, 1.0, 0.0), // "email" semantic
		},
	}

	for _, c := range chunks {
		chunk := &models.Chunk{
			Path:    "src/service.go",
			Name:    c.name,
			Level:   models.LevelMethod,
			Content: c.content,
		}
		_, err := db.InsertChunk(chunk, c.embedding)
		require.NoError(t, err)
	}

	// Search for "user management" (similar to createUser/deleteUser)
	queryEmbedding := createTestEmbedding(0.9, 0.05, 0.0)
	results, err := db.VectorSearch(queryEmbedding, 10, nil)

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 2)

	// User-related functions should rank higher
	assert.Contains(t, []string{"createUser", "deleteUser"}, results[0].Chunk.Name)
}

func TestSearch_FilterByLevel(t *testing.T) {
	db := setupTestDB(t)

	embedding := make([]float32, 768)

	// Insert file-level chunk
	fileChunk := &models.Chunk{
		Path:    "src/main.go",
		Name:    "main.go",
		Level:   models.LevelFile,
		Content: "package main\n\nimport \"fmt\"",
	}
	db.InsertChunk(fileChunk, embedding)

	// Insert method-level chunk
	methodChunk := &models.Chunk{
		Path:    "src/main.go",
		Name:    "main",
		Level:   models.LevelMethod,
		Content: "func main() {}",
	}
	db.InsertChunk(methodChunk, embedding)

	// Search with level filter
	filter := &SearchFilter{
		Levels: []models.ChunkLevel{models.LevelMethod},
	}
	results, err := db.VectorSearch(embedding, 10, filter)

	require.NoError(t, err)
	for _, r := range results {
		assert.Equal(t, models.LevelMethod, r.Chunk.Level)
	}
}

func TestSearch_FilterByPath(t *testing.T) {
	db := setupTestDB(t)

	embedding := make([]float32, 768)

	// Insert chunks in different paths
	db.InsertChunk(&models.Chunk{
		Path:    "src/api/handler.go",
		Name:    "HandleRequest",
		Level:   models.LevelMethod,
		Content: "func HandleRequest() {}",
	}, embedding)

	db.InsertChunk(&models.Chunk{
		Path:    "src/db/repository.go",
		Name:    "Save",
		Level:   models.LevelMethod,
		Content: "func Save() {}",
	}, embedding)

	// Search only in api directory
	filter := &SearchFilter{
		PathPrefix: "src/api/",
	}
	results, err := db.VectorSearch(embedding, 10, filter)

	require.NoError(t, err)
	for _, r := range results {
		assert.True(t, strings.HasPrefix(r.Chunk.Path, "src/api/"))
	}
}

// Helper to create test embeddings with distinct characteristics
func createTestEmbedding(x, y, z float32) []float32 {
	embedding := make([]float32, 768)
	embedding[0] = x
	embedding[1] = y
	embedding[2] = z
	// Fill rest with small values
	for i := 3; i < 768; i++ {
		embedding[i] = 0.01 * float32(i) / 768
	}
	return embedding
}
```

**Acceptance Criteria:**
- Database initialization tested
- CRUD operations verified
- Vector search with filtering works correctly

---

### 7.1.4 Daemon Package Tests

**Description:** Unit tests for watcher, indexer, and daemon orchestration.

**Files:**
- `internal/daemon/watcher_test.go`
- `internal/daemon/indexer_test.go`
- `internal/daemon/daemon_test.go`

**Key Test Cases:**

```go
// internal/daemon/watcher_test.go
package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcher_DetectsFileCreation(t *testing.T) {
	tmpDir := t.TempDir()

	watcher, err := NewWatcher(tmpDir, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go watcher.Start(ctx)

	// Create a file
	filePath := filepath.Join(tmpDir, "test.go")
	err = os.WriteFile(filePath, []byte("package main"), 0644)
	require.NoError(t, err)

	// Wait for event
	select {
	case event := <-watcher.Events():
		assert.Equal(t, filePath, event.Path)
		assert.Equal(t, OpCreate, event.Operation)
	case <-ctx.Done():
		t.Fatal("timeout waiting for file event")
	}
}

func TestWatcher_Debouncing(t *testing.T) {
	tmpDir := t.TempDir()

	config := &WatcherConfig{
		DebounceMs: 100,
	}
	watcher, err := NewWatcher(tmpDir, config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go watcher.Start(ctx)

	filePath := filepath.Join(tmpDir, "test.go")

	// Rapid writes should be debounced
	for i := 0; i < 5; i++ {
		os.WriteFile(filePath, []byte(fmt.Sprintf("version %d", i)), 0644)
		time.Sleep(20 * time.Millisecond)
	}

	// Should only receive one event (debounced)
	eventCount := 0
	timeout := time.After(300 * time.Millisecond)

loop:
	for {
		select {
		case <-watcher.Events():
			eventCount++
		case <-timeout:
			break loop
		}
	}

	assert.Equal(t, 1, eventCount, "rapid writes should be debounced to single event")
}

func TestWatcher_RespectsIgnorePatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .pommelignore
	ignoreFile := filepath.Join(tmpDir, ".pommelignore")
	os.WriteFile(ignoreFile, []byte("*.log\nnode_modules/\n"), 0644)

	watcher, err := NewWatcher(tmpDir, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go watcher.Start(ctx)

	// Create ignored file
	logFile := filepath.Join(tmpDir, "debug.log")
	os.WriteFile(logFile, []byte("log content"), 0644)

	// Create non-ignored file
	goFile := filepath.Join(tmpDir, "main.go")
	os.WriteFile(goFile, []byte("package main"), 0644)

	// Should only receive event for .go file
	select {
	case event := <-watcher.Events():
		assert.Equal(t, goFile, event.Path)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected event for .go file")
	}

	// No event for .log file
	select {
	case event := <-watcher.Events():
		assert.NotEqual(t, logFile, event.Path, "should not receive event for ignored file")
	case <-time.After(200 * time.Millisecond):
		// Expected - no event for ignored file
	}
}
```

**Acceptance Criteria:**
- File system events detected correctly
- Debouncing works as configured
- Ignore patterns respected

---

### 7.1.5 API Package Tests

**Description:** HTTP handler tests with request/response validation.

**Files:**
- `internal/api/handlers_test.go`
- `internal/api/middleware_test.go`

**Key Test Cases:**

```go
// internal/api/handlers_test.go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchHandler_ValidRequest(t *testing.T) {
	mockSearcher := &MockSearcher{
		Results: []SearchResult{
			{
				Chunk: models.Chunk{
					Path:    "src/main.go",
					Name:    "main",
					Content: "func main() {}",
				},
				Score: 0.95,
			},
		},
	}

	handler := NewSearchHandler(mockSearcher)

	reqBody := SearchRequest{
		Query: "entry point function",
		Limit: 10,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp SearchResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Len(t, resp.Results, 1)
	assert.Equal(t, "main", resp.Results[0].Name)
}

func TestSearchHandler_EmptyQuery(t *testing.T) {
	handler := NewSearchHandler(&MockSearcher{})

	reqBody := SearchRequest{
		Query: "",
		Limit: 10,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp ErrorResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.Contains(t, resp.Error, "query")
}

func TestHealthHandler(t *testing.T) {
	handler := NewHealthHandler()

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp HealthResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.Equal(t, "healthy", resp.Status)
}

func TestStatusHandler(t *testing.T) {
	mockState := &MockState{
		FilesIndexed: 100,
		ChunksStored: 500,
		LastIndexed:  time.Now(),
	}

	handler := NewStatusHandler(mockState)

	req := httptest.NewRequest("GET", "/status", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp StatusResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.Equal(t, 100, resp.FilesIndexed)
	assert.Equal(t, 500, resp.ChunksStored)
}

// Mock implementations
type MockSearcher struct {
	Results []SearchResult
	Error   error
}

func (m *MockSearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	return m.Results, m.Error
}

type MockState struct {
	FilesIndexed int
	ChunksStored int
	LastIndexed  time.Time
}

func (m *MockState) GetStatus() *Status {
	return &Status{
		FilesIndexed: m.FilesIndexed,
		ChunksStored: m.ChunksStored,
		LastIndexed:  m.LastIndexed,
	}
}
```

**Acceptance Criteria:**
- All endpoints respond correctly to valid requests
- Error responses follow consistent format
- JSON encoding/decoding validated

---

## Task 7.2: Integration Tests

### 7.2.1 Full Indexing Flow Test

**Description:** End-to-end test that indexes a test project and verifies database state.

**File:** `integration_test.go` (in root, with build tag)

```go
//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/daemon"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullIndexingFlow(t *testing.T) {
	// Skip if Ollama not available
	if !isOllamaAvailable() {
		t.Skip("Ollama not available - skipping integration test")
	}

	// Create test project structure
	projectDir := t.TempDir()
	createTestProject(t, projectDir)

	// Initialize Pommel
	pommelDir := filepath.Join(projectDir, ".pommel")
	err := os.MkdirAll(pommelDir, 0755)
	require.NoError(t, err)

	// Create daemon
	cfg := &daemon.Config{
		ProjectRoot: projectDir,
		PommelDir:   pommelDir,
		OllamaURL:   "http://localhost:11434",
		Model:       "unclemusclez/jina-embeddings-v2-base-code",
	}

	d, err := daemon.New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start daemon and wait for initial indexing
	go d.Start(ctx)

	// Wait for initial indexing to complete
	require.Eventually(t, func() bool {
		status := d.Status()
		return status.State == "ready" && status.FilesIndexed > 0
	}, 2*time.Minute, 1*time.Second, "initial indexing should complete")

	// Verify database state
	database, err := db.Open(filepath.Join(pommelDir, "index.db"))
	require.NoError(t, err)
	defer database.Close()

	// Check files indexed
	status := d.Status()
	assert.Greater(t, status.FilesIndexed, 0)
	assert.Greater(t, status.ChunksStored, status.FilesIndexed) // More chunks than files

	// Verify chunk levels
	chunks, err := database.GetAllChunks()
	require.NoError(t, err)

	levelCounts := make(map[string]int)
	for _, c := range chunks {
		levelCounts[string(c.Level)]++
	}

	assert.Greater(t, levelCounts["file"], 0, "should have file-level chunks")
	assert.Greater(t, levelCounts["method"], 0, "should have method-level chunks")

	// Cleanup
	d.Stop()
}

func createTestProject(t *testing.T, dir string) {
	// Create Python file
	pyDir := filepath.Join(dir, "src", "python")
	os.MkdirAll(pyDir, 0755)
	os.WriteFile(filepath.Join(pyDir, "service.py"), []byte(`
class UserService:
    def get_user(self, user_id: int):
        """Get user by ID."""
        return {"id": user_id, "name": "Test"}

    def create_user(self, name: str):
        """Create a new user."""
        return {"id": 1, "name": name}
`), 0644)

	// Create JavaScript file
	jsDir := filepath.Join(dir, "src", "js")
	os.MkdirAll(jsDir, 0755)
	os.WriteFile(filepath.Join(jsDir, "handler.js"), []byte(`
class ApiHandler {
    async handleRequest(req) {
        const data = await this.parseRequest(req);
        return this.processData(data);
    }

    parseRequest(req) {
        return JSON.parse(req.body);
    }
}

module.exports = ApiHandler;
`), 0644)

	// Create C# file
	csDir := filepath.Join(dir, "src", "csharp")
	os.MkdirAll(csDir, 0755)
	os.WriteFile(filepath.Join(csDir, "Repository.cs"), []byte(`
namespace MyApp.Data
{
    public class UserRepository
    {
        public User FindById(int id)
        {
            return new User { Id = id };
        }

        public void Save(User user)
        {
            // Save to database
        }
    }
}
`), 0644)
}

func isOllamaAvailable() bool {
	resp, err := http.Get("http://localhost:11434/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
```

**Acceptance Criteria:**
- Test creates real project structure
- Daemon indexes all files
- All chunk levels present in database

---

### 7.2.2 Search Flow Test

**Description:** End-to-end search test validating result quality.

```go
//go:build integration

func TestSearchFlow(t *testing.T) {
	if !isOllamaAvailable() {
		t.Skip("Ollama not available")
	}

	// Setup indexed project (reuse from previous test or setup fresh)
	projectDir := t.TempDir()
	createTestProject(t, projectDir)

	// Initialize and index
	d := setupIndexedDaemon(t, projectDir)
	defer d.Stop()

	// Perform semantic search
	client := client.New("http://localhost:7420")

	testCases := []struct {
		query          string
		expectedInTop3 []string // Names that should appear in top 3
	}{
		{
			query:          "get user by identifier",
			expectedInTop3: []string{"get_user", "FindById"},
		},
		{
			query:          "create new user record",
			expectedInTop3: []string{"create_user", "Save"},
		},
		{
			query:          "parse incoming request data",
			expectedInTop3: []string{"parseRequest", "handleRequest"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.query, func(t *testing.T) {
			results, err := client.Search(context.Background(), tc.query, 10)
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(results), 3)

			top3Names := make([]string, 3)
			for i := 0; i < 3 && i < len(results); i++ {
				top3Names[i] = results[i].Name
			}

			// At least one expected result should be in top 3
			foundExpected := false
			for _, expected := range tc.expectedInTop3 {
				for _, name := range top3Names {
					if name == expected {
						foundExpected = true
						break
					}
				}
			}

			assert.True(t, foundExpected,
				"query '%s' should have one of %v in top 3, got %v",
				tc.query, tc.expectedInTop3, top3Names)
		})
	}
}
```

**Acceptance Criteria:**
- Semantic search returns relevant results
- Similar queries find related code
- Ranking places best matches first

---

### 7.2.3 CLI End-to-End Test

**Description:** Test CLI commands work correctly.

```go
//go:build integration

func TestCLICommands(t *testing.T) {
	projectDir := t.TempDir()
	createTestProject(t, projectDir)

	// Test pm init
	t.Run("init", func(t *testing.T) {
		cmd := exec.Command("./pm", "init")
		cmd.Dir = projectDir
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "pm init failed: %s", output)

		// Verify .pommel directory created
		_, err = os.Stat(filepath.Join(projectDir, ".pommel"))
		assert.NoError(t, err)
	})

	// Test pm start
	t.Run("start", func(t *testing.T) {
		cmd := exec.Command("./pm", "start")
		cmd.Dir = projectDir
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "pm start failed: %s", output)

		// Give daemon time to start
		time.Sleep(2 * time.Second)

		// Verify daemon is running
		cmd = exec.Command("./pm", "status", "--json")
		cmd.Dir = projectDir
		output, err = cmd.CombinedOutput()
		require.NoError(t, err)

		var status map[string]interface{}
		json.Unmarshal(output, &status)
		assert.Equal(t, "running", status["status"])
	})

	// Test pm search
	t.Run("search", func(t *testing.T) {
		// Wait for indexing
		time.Sleep(30 * time.Second)

		cmd := exec.Command("./pm", "search", "get user", "--json")
		cmd.Dir = projectDir
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "pm search failed: %s", output)

		var results struct {
			Results []map[string]interface{} `json:"results"`
		}
		json.Unmarshal(output, &results)
		assert.Greater(t, len(results.Results), 0)
	})

	// Test pm stop
	t.Run("stop", func(t *testing.T) {
		cmd := exec.Command("./pm", "stop")
		cmd.Dir = projectDir
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "pm stop failed: %s", output)

		// Verify daemon stopped
		time.Sleep(1 * time.Second)
		cmd = exec.Command("./pm", "status")
		cmd.Dir = projectDir
		output, _ = cmd.CombinedOutput()
		assert.Contains(t, string(output), "not running")
	})
}
```

**Acceptance Criteria:**
- All CLI commands execute successfully
- JSON output parses correctly
- Daemon lifecycle works as expected

---

## Task 7.3: Dogfooding on Pommel

**This is the most critical validation step.** Pommel should be able to index and search its own Go codebase.

### 7.3.1 Index Pommel Codebase

**Description:** Initialize and index the Pommel project itself.

**Steps:**
1. Build Pommel binaries
2. Run `pm init` in Pommel project root
3. Run `pm start` to begin indexing
4. Wait for indexing to complete
5. Verify all Go files are indexed

**Validation Script:**

```bash
#!/bin/bash
# test_dogfood.sh

set -e

echo "Building Pommel..."
make build

echo "Initializing Pommel on itself..."
./pm init

echo "Starting daemon..."
./pm start

echo "Waiting for indexing (60 seconds)..."
sleep 60

echo "Checking status..."
./pm status --json | jq .

echo "Running test searches..."

# Search for specific functionality
echo "Search: 'embedding generation'..."
./pm search "embedding generation" --limit 5 --json | jq '.results[] | {name, path, score}'

echo "Search: 'file watcher debounce'..."
./pm search "file watcher debounce" --limit 5 --json | jq '.results[] | {name, path, score}'

echo "Search: 'vector search sqlite'..."
./pm search "vector search sqlite" --limit 5 --json | jq '.results[] | {name, path, score}'

echo "Search: 'CLI command handler'..."
./pm search "CLI command handler" --limit 5 --json | jq '.results[] | {name, path, score}'

echo "Stopping daemon..."
./pm stop

echo "Dogfood test complete!"
```

**Expected Results:**

| Query | Expected Top Results |
|-------|---------------------|
| "embedding generation" | `OllamaClient.GenerateEmbedding`, `Embedder` interface |
| "file watcher debounce" | `Watcher.debounce`, `WatcherConfig` |
| "vector search sqlite" | `Database.VectorSearch`, `sqlite-vec` related |
| "CLI command handler" | `searchCmd`, `initCmd`, Cobra handlers |

### 7.3.2 Document Dogfood Results

**Description:** Create a test report documenting dogfood results.

**File:** `docs/dogfood-results.md`

```markdown
# Pommel Dogfood Test Results

**Date:** [date of test]
**Version:** v0.1.0
**Test Environment:** [macOS/Linux version, Go version]

## Indexing Statistics

| Metric | Value |
|--------|-------|
| Files indexed | X |
| Total chunks | X |
| File-level chunks | X |
| Class-level chunks | X |
| Method-level chunks | X |
| Indexing time | X seconds |
| Database size | X MB |

## Search Quality Tests

### Test 1: "embedding generation"

**Expected:** Find OllamaClient and embedding-related functions

| Rank | Result | Score | Relevant? |
|------|--------|-------|-----------|
| 1 | ... | ... | ✅/❌ |
| 2 | ... | ... | ✅/❌ |
| 3 | ... | ... | ✅/❌ |

### Test 2: "file watcher debounce"

[same format]

## Performance Metrics

| Operation | Time |
|-----------|------|
| Initial indexing (full project) | X seconds |
| Search query (10 results) | X ms |
| Single file re-index | X ms |

## Issues Found

[Document any issues discovered during dogfooding]

## Recommendations

[Recommendations for improvement based on dogfood testing]
```

**Acceptance Criteria:**
- Pommel successfully indexes its own codebase
- Search finds relevant code for real queries
- Performance meets targets
- Any issues documented for future work

---

## Task 7.4: Error Handling Polish

### 7.4.1 Error Message Audit

**Description:** Review all error messages for clarity and actionability.

**Checklist:**

```go
// Good error messages include:
// 1. What went wrong
// 2. Why it might have happened
// 3. How to fix it

// Bad:
return fmt.Errorf("connection failed")

// Good:
return fmt.Errorf("cannot connect to Ollama at %s: %w. Is Ollama running? Try: ollama serve", url, err)

// Bad:
return fmt.Errorf("model not found")

// Good:
return fmt.Errorf("embedding model %q not found in Ollama. Install with: ollama pull %s", model, model)

// Bad:
return fmt.Errorf("invalid config")

// Good:
return fmt.Errorf("invalid configuration at %s: %w. Check YAML syntax", configPath, err)
```

**Files to Audit:**
- `internal/embedder/ollama.go` - Ollama connection errors
- `internal/setup/detector.go` - Dependency detection errors
- `internal/cli/*.go` - All CLI command errors
- `internal/daemon/daemon.go` - Startup/shutdown errors
- `internal/db/sqlite.go` - Database errors

### 7.4.2 Consistent Error Format

**Description:** Implement consistent error format for API responses.

```go
// internal/api/errors.go
package api

type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

var (
	ErrQueryEmpty = APIError{
		Code:       "QUERY_EMPTY",
		Message:    "Search query cannot be empty",
		Suggestion: "Provide a search query describing what you're looking for",
	}

	ErrDaemonNotRunning = APIError{
		Code:       "DAEMON_NOT_RUNNING",
		Message:    "Pommel daemon is not running",
		Suggestion: "Start the daemon with: pm start",
	}

	ErrOllamaUnavailable = APIError{
		Code:       "OLLAMA_UNAVAILABLE",
		Message:    "Cannot connect to Ollama embedding service",
		Suggestion: "Ensure Ollama is running: ollama serve",
	}

	ErrModelNotInstalled = APIError{
		Code:       "MODEL_NOT_INSTALLED",
		Message:    "Required embedding model is not installed",
		Suggestion: "Install the model: ollama pull unclemusclez/jina-embeddings-v2-base-code",
	}
)
```

**Acceptance Criteria:**
- All errors have actionable messages
- API errors follow consistent JSON format
- CLI errors print user-friendly messages

---

## Task 7.5: Documentation

### 7.5.1 README.md

**File:** `README.md`

```markdown
# Pommel

Local-first semantic code search for AI coding agents.

Pommel maintains a vector database of your code, enabling fast semantic search without loading files into context. Designed to complement AI coding assistants by providing targeted code discovery.

## Features

- **Semantic search** - Find code by meaning, not just keywords
- **Always fresh** - File watcher keeps index up-to-date automatically
- **Multi-level chunks** - Search at file, class, or method granularity
- **Low latency** - Local embeddings via Ollama, SQLite vector storage
- **Agent-friendly** - JSON output for easy integration

## Installation

### Prerequisites

- **Ollama** - Local LLM runtime for embeddings
  ```bash
  # macOS
  brew install ollama

  # Linux
  curl -fsSL https://ollama.com/install.sh | sh
  ```

- **Embedding model**
  ```bash
  ollama pull unclemusclez/jina-embeddings-v2-base-code
  ```

### Install Pommel

```bash
# From source
go install github.com/pommel-dev/pommel/cmd/pm@latest
go install github.com/pommel-dev/pommel/cmd/pommeld@latest

# Or download binary from releases
```

## Quick Start

```bash
# Initialize in your project
cd your-project
pm init

# Start the daemon (indexes automatically)
pm start

# Search for code
pm search "authentication middleware"

# Check status
pm status
```

## CLI Commands

### `pm init`

Initialize Pommel in the current directory. Creates `.pommel/` directory with configuration.

```bash
pm init                    # Interactive setup
pm init --auto             # Auto-detect dependencies and configure
```

### `pm start`

Start the Pommel daemon for the current project.

```bash
pm start                   # Start in background
pm start --foreground      # Start in foreground (for debugging)
```

### `pm stop`

Stop the running daemon.

```bash
pm stop
```

### `pm search <query>`

Semantic search across the codebase.

```bash
pm search "user authentication"
pm search "database connection" --limit 20
pm search "error handling" --level method
pm search "api handler" --path src/api/ --json
```

**Options:**
- `--limit, -n` - Maximum results (default: 10)
- `--level, -l` - Chunk level filter: file, class, method
- `--path, -p` - Path prefix filter
- `--json, -j` - Output as JSON (for agents)

### `pm status`

Show daemon status and indexing statistics.

```bash
pm status                  # Human-readable output
pm status --json           # JSON output
```

### `pm reindex`

Force a full re-index of the project.

```bash
pm reindex                 # Reindex all files
pm reindex --path src/     # Reindex specific path
```

### `pm config`

View or modify configuration.

```bash
pm config                  # Show current config
pm config set debounce_ms 1000
pm config get ollama_url
```

## Configuration

Configuration is stored in `.pommel/config.yaml`:

```yaml
# Ollama settings
ollama_url: http://localhost:11434
model: unclemusclez/jina-embeddings-v2-base-code

# Indexing settings
debounce_ms: 500
max_file_size_kb: 1024

# Languages to index (empty = auto-detect)
languages:
  - python
  - javascript
  - typescript
  - csharp
```

## Ignoring Files

Create `.pommelignore` in your project root (uses gitignore syntax):

```
# Dependencies
node_modules/
vendor/
.venv/

# Build outputs
dist/
build/
*.min.js

# Generated files
*.generated.go
```

Pommel also respects `.gitignore` by default.

## AI Agent Integration

Pommel is designed for AI coding agents. Add to your `CLAUDE.md`:

```markdown
## Code Search

Use `pm search` to find relevant code before reading files:

\`\`\`bash
pm search "your query" --json --limit 5
\`\`\`

This returns semantic matches with file paths, reducing context usage.
```

### JSON Output Format

```json
{
  "results": [
    {
      "path": "src/auth/middleware.py",
      "name": "AuthMiddleware",
      "level": "class",
      "content": "class AuthMiddleware:\n    ...",
      "start_line": 15,
      "end_line": 45,
      "parent": "auth.middleware",
      "score": 0.89
    }
  ],
  "query": "authentication middleware",
  "took_ms": 42
}
```

## Supported Languages

- C# (.cs)
- Python (.py)
- JavaScript (.js, .jsx, .mjs)
- TypeScript (.ts, .tsx)

## Performance

| Operation | Typical Time |
|-----------|--------------|
| Initial indexing (1000 files) | 2-5 minutes |
| File change re-index | < 500ms |
| Search query (10 results) | < 100ms |

## Troubleshooting

### "Cannot connect to Ollama"

```bash
# Check if Ollama is running
ollama list

# Start Ollama if needed
ollama serve
```

### "Model not found"

```bash
# Install the embedding model
ollama pull unclemusclez/jina-embeddings-v2-base-code
```

### Daemon not starting

```bash
# Check for existing daemon
pm status

# Check logs
cat .pommel/daemon.log
```

## License

MIT
```

### 7.5.2 CLI Help Text

**Description:** Ensure all CLI commands have helpful `--help` output.

```go
// Example for search command
var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Semantic search across the codebase",
	Long: `Search for code using natural language queries.

Pommel uses semantic embeddings to find code that matches your intent,
not just keyword matches. Results are ranked by relevance.

Examples:
  pm search "user authentication"
  pm search "database connection handling" --limit 20
  pm search "error handler" --level method --json
  pm search "api endpoint" --path src/api/`,
	Args: cobra.MinimumNArgs(1),
	Run:  runSearch,
}
```

**Acceptance Criteria:**
- README covers installation, usage, configuration
- All CLI commands have descriptive help text
- Examples provided for common workflows

---

## Task 7.6: Performance Validation

### 7.6.1 Benchmarks

**Description:** Create benchmarks for critical paths.

**File:** `benchmarks_test.go`

```go
//go:build benchmark

package benchmarks

import (
	"context"
	"testing"
)

func BenchmarkEmbedding_SingleText(b *testing.B) {
	client := setupOllamaClient(b)
	ctx := context.Background()
	text := "func processUserRequest(req *Request) (*Response, error)"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GenerateEmbedding(ctx, text)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearch_10Results(b *testing.B) {
	db := setupIndexedDatabase(b) // Pre-indexed with 1000 chunks
	ctx := context.Background()
	queryEmbedding := generateQueryEmbedding(b, "user authentication")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.VectorSearch(ctx, queryEmbedding, 10, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkChunker_PythonFile(b *testing.B) {
	chunker := NewPythonChunker()
	code := loadTestFile(b, "testdata/python/large_service.py")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chunker.Chunk("service.py", code)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFullIndexing_100Files(b *testing.B) {
	// Measure time to index 100 files
	projectDir := setupTestProject(b, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indexer := setupIndexer(b, projectDir)
		err := indexer.IndexAll(context.Background())
		if err != nil {
			b.Fatal(err)
		}
		cleanupIndex(b, projectDir)
	}
}
```

### 7.6.2 Performance Targets

**Description:** Validate performance meets targets.

| Operation | Target | Validation |
|-----------|--------|------------|
| Search (10 results) | < 100ms | Benchmark average |
| Single file index | < 500ms | Benchmark with embedding |
| Full index (1000 files) | < 5 min | Integration test timing |
| Query embedding cache hit | < 1ms | Benchmark |

**Acceptance Criteria:**
- All targets met on reference hardware
- Benchmarks documented with baseline results

---

## Dependencies

| Dependency | Reason |
|------------|--------|
| Phase 1-6 complete | All features must be implemented |
| Ollama installed | Required for integration tests |
| Test fixtures created | Needed for unit/integration tests |

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Ollama unavailable in CI | Integration tests fail | Skip integration tests, run locally |
| Flaky file watcher tests | CI failures | Use longer timeouts, add retries |
| Embedding model slow | Tests timeout | Use smaller test fixtures |
| Search quality varies | Dogfood results inconsistent | Document acceptable variance |

---

## Checklist

- [ ] Unit test coverage > 80%
- [ ] Integration tests pass
- [ ] Dogfood on Pommel successful
- [ ] Error messages audited
- [ ] README complete
- [ ] CLI help text complete
- [ ] Benchmarks created
- [ ] Performance targets met
- [ ] All issues from dogfood documented
