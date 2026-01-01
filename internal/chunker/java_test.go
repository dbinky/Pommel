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

// getJavaChunker returns the Java chunker from the config-driven registry.
// This replaces the legacy NewJavaChunker function.
func getJavaChunker(t *testing.T) Chunker {
	t.Helper()
	registry, err := NewChunkerRegistry()
	require.NoError(t, err, "Failed to create chunker registry")

	chunker, ok := registry.GetChunkerForExtension(".java")
	require.True(t, ok, "Java chunker should be available")
	return chunker
}

// =============================================================================
// JavaChunker Initialization Tests
// =============================================================================

func TestJavaChunker_Available(t *testing.T) {
	registry, err := NewChunkerRegistry()
	require.NoError(t, err)

	chunker, ok := registry.GetChunkerForExtension(".java")
	assert.True(t, ok, "Java chunker should be available in registry")
	assert.NotNil(t, chunker, "Java chunker should not be nil")
}

func TestJavaChunker_Language(t *testing.T) {
	chunker := getJavaChunker(t)
	assert.Equal(t, LangJava, chunker.Language(), "JavaChunker should report Java as its language")
}

// =============================================================================
// Empty File Tests
// =============================================================================

func TestJavaChunker_EmptyFile(t *testing.T) {
	chunker := getJavaChunker(t)

	file := &models.SourceFile{
		Path:         "/test/Empty.java",
		Content:      []byte(""),
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Empty file should not error
	assert.Empty(t, result.Errors, "Should have no errors for empty file")
}

// =============================================================================
// Simple Class Tests
// =============================================================================

func TestJavaChunker_SimpleClass(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public class HelloWorld {
    private String message;

    public void sayHello() {
        System.out.println("Hello, World!");
    }

    public String getMessage() {
        return message;
    }
}
`)

	file := &models.SourceFile{
		Path:         "/test/HelloWorld.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have: 1 file chunk + 1 class chunk + 2 method chunks = 4 chunks
	assert.Len(t, result.Chunks, 4, "Should have 4 chunks: file, class, and 2 methods")

	// Find chunks
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

	require.NotNil(t, fileChunk, "Should have a file-level chunk")
	require.NotNil(t, classChunk, "Should have a class-level chunk")
	assert.Equal(t, "HelloWorld", classChunk.Name, "Class should be named 'HelloWorld'")
	assert.Len(t, methodChunks, 2, "Should have 2 method chunks")

	// Methods should reference class as parent
	for _, method := range methodChunks {
		require.NotNil(t, method.ParentID, "Method should have a parent ID")
		assert.Equal(t, classChunk.ID, *method.ParentID, "Method should reference class as parent")
	}

	// Verify method names
	methodNames := make(map[string]bool)
	for _, m := range methodChunks {
		methodNames[m.Name] = true
	}
	assert.True(t, methodNames["sayHello"], "Should have 'sayHello' method")
	assert.True(t, methodNames["getMessage"], "Should have 'getMessage' method")
}

// =============================================================================
// Interface Tests
// =============================================================================

func TestJavaChunker_Interface(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public interface Readable {
    int read(byte[] buffer);
    void close();
}

interface Writable {
    void write(byte[] data);
}
`)

	file := &models.SourceFile{
		Path:         "/test/Readable.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find interface chunks
	var interfaceChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			interfaceChunks = append(interfaceChunks, chunk)
		}
	}

	assert.Len(t, interfaceChunks, 2, "Should have 2 interface chunks at class level")

	// Verify interface names
	interfaceNames := make(map[string]bool)
	for _, i := range interfaceChunks {
		interfaceNames[i.Name] = true
	}
	assert.True(t, interfaceNames["Readable"], "Should have 'Readable' interface")
	assert.True(t, interfaceNames["Writable"], "Should have 'Writable' interface")
}

// =============================================================================
// Enum Tests
// =============================================================================

func TestJavaChunker_Enum(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public enum Color {
    RED, GREEN, BLUE;

    public String toHex() {
        switch (this) {
            case RED: return "#FF0000";
            case GREEN: return "#00FF00";
            case BLUE: return "#0000FF";
            default: return "#000000";
        }
    }
}

enum Status {
    PENDING, ACTIVE, COMPLETED
}
`)

	file := &models.SourceFile{
		Path:         "/test/Color.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find enum chunks
	var enumChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			enumChunks = append(enumChunks, chunk)
		}
	}

	assert.Len(t, enumChunks, 2, "Should have 2 enum chunks at class level")
	// Note: enum methods may or may not be extracted depending on tree-sitter structure
	// The important thing is that enums themselves are extracted as class-level chunks

	// Verify enum names
	enumNames := make(map[string]bool)
	for _, e := range enumChunks {
		enumNames[e.Name] = true
	}
	assert.True(t, enumNames["Color"], "Should have 'Color' enum")
	assert.True(t, enumNames["Status"], "Should have 'Status' enum")
}

// =============================================================================
// Record Tests (Java 14+)
// =============================================================================

func TestJavaChunker_Record(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public record Point(int x, int y) {
    public double distance() {
        return Math.sqrt(x * x + y * y);
    }
}

record Person(String name, int age) {}
`)

	file := &models.SourceFile{
		Path:         "/test/Point.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find record chunks
	var recordChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			recordChunks = append(recordChunks, chunk)
		}
	}

	assert.Len(t, recordChunks, 2, "Should have 2 record chunks at class level")

	// Verify record names
	recordNames := make(map[string]bool)
	for _, r := range recordChunks {
		recordNames[r.Name] = true
	}
	assert.True(t, recordNames["Point"], "Should have 'Point' record")
	assert.True(t, recordNames["Person"], "Should have 'Person' record")
}

// =============================================================================
// Annotation Type Tests
// =============================================================================

func TestJavaChunker_AnnotationType(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public @interface MyAnnotation {
    String value() default "";
    int priority() default 0;
}

@interface Deprecated {
    String since() default "";
}
`)

	file := &models.SourceFile{
		Path:         "/test/MyAnnotation.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find annotation type chunks
	var annotationChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			annotationChunks = append(annotationChunks, chunk)
		}
	}

	assert.Len(t, annotationChunks, 2, "Should have 2 annotation type chunks at class level")

	// Verify annotation names
	annotationNames := make(map[string]bool)
	for _, a := range annotationChunks {
		annotationNames[a.Name] = true
	}
	assert.True(t, annotationNames["MyAnnotation"], "Should have 'MyAnnotation' annotation type")
	assert.True(t, annotationNames["Deprecated"], "Should have 'Deprecated' annotation type")
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestJavaChunker_Constructor(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public class User {
    private String name;
    private int age;

    public User() {
        this.name = "Unknown";
        this.age = 0;
    }

    public User(String name) {
        this.name = name;
        this.age = 0;
    }

    public User(String name, int age) {
        this.name = name;
        this.age = age;
    }

    public String getName() {
        return name;
    }
}
`)

	file := &models.SourceFile{
		Path:         "/test/User.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find method-level chunks (constructors and methods)
	var methodChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	// Should have 3 constructors + 1 method = 4 method-level chunks
	assert.Len(t, methodChunks, 4, "Should have 4 method-level chunks (3 constructors + 1 method)")

	// Verify constructor names (should be class name)
	constructorCount := 0
	for _, m := range methodChunks {
		if m.Name == "User" {
			constructorCount++
		}
	}
	assert.Equal(t, 3, constructorCount, "Should have 3 constructors named 'User'")
}

// =============================================================================
// Multiple Classes Tests
// =============================================================================

func TestJavaChunker_MultipleClasses(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public class Main {
    public static void main(String[] args) {
        System.out.println("Hello");
    }
}

class Helper {
    void help() {}
}

class Utils {
    static void util() {}
}
`)

	file := &models.SourceFile{
		Path:         "/test/Main.java",
		Content:      source,
		Language:     "java",
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

	assert.Len(t, classChunks, 3, "Should have 3 class chunks")

	// Verify class names
	classNames := make(map[string]bool)
	for _, c := range classChunks {
		classNames[c.Name] = true
	}
	assert.True(t, classNames["Main"], "Should have 'Main' class")
	assert.True(t, classNames["Helper"], "Should have 'Helper' class")
	assert.True(t, classNames["Utils"], "Should have 'Utils' class")

	// All classes should have file as parent (flat structure)
	var fileChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelFile {
			fileChunk = chunk
			break
		}
	}
	require.NotNil(t, fileChunk)

	for _, c := range classChunks {
		require.NotNil(t, c.ParentID)
		assert.Equal(t, fileChunk.ID, *c.ParentID, "Class should have file as parent")
	}
}

// =============================================================================
// Nested Class Tests (Flat Structure)
// =============================================================================

func TestJavaChunker_NestedClass(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public class Outer {
    private int value;

    public class Inner {
        public void innerMethod() {}
    }

    public static class StaticNested {
        public void nestedMethod() {}
    }

    public void outerMethod() {}
}
`)

	file := &models.SourceFile{
		Path:         "/test/Outer.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find chunks
	var fileChunk *models.Chunk
	var classChunks []*models.Chunk

	for _, chunk := range result.Chunks {
		switch chunk.Level {
		case models.ChunkLevelFile:
			fileChunk = chunk
		case models.ChunkLevelClass:
			classChunks = append(classChunks, chunk)
		}
	}

	require.NotNil(t, fileChunk)
	assert.Len(t, classChunks, 3, "Should have 3 class chunks (Outer, Inner, StaticNested)")

	// Verify all classes have file as parent (flat structure)
	for _, c := range classChunks {
		require.NotNil(t, c.ParentID)
		assert.Equal(t, fileChunk.ID, *c.ParentID, "All classes should have file as parent (flat structure)")
	}

	// Verify class names
	classNames := make(map[string]bool)
	for _, c := range classChunks {
		classNames[c.Name] = true
	}
	assert.True(t, classNames["Outer"], "Should have 'Outer' class")
	assert.True(t, classNames["Inner"], "Should have 'Inner' class")
	assert.True(t, classNames["StaticNested"], "Should have 'StaticNested' class")
}

// =============================================================================
// Generic Types Tests
// =============================================================================

func TestJavaChunker_Generics(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public class Box<T> {
    private T content;

    public void set(T content) {
        this.content = content;
    }

    public T get() {
        return content;
    }
}

class Pair<K, V> {
    private K key;
    private V value;

    public Pair(K key, V value) {
        this.key = key;
        this.value = value;
    }
}

interface Comparable<T> {
    int compareTo(T other);
}
`)

	file := &models.SourceFile{
		Path:         "/test/Box.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find class-level chunks
	var classChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			classChunks = append(classChunks, chunk)
		}
	}

	assert.Len(t, classChunks, 3, "Should have 3 class-level chunks (Box, Pair, Comparable)")

	// Verify names
	classNames := make(map[string]bool)
	for _, c := range classChunks {
		classNames[c.Name] = true
	}
	assert.True(t, classNames["Box"], "Should have 'Box' generic class")
	assert.True(t, classNames["Pair"], "Should have 'Pair' generic class")
	assert.True(t, classNames["Comparable"], "Should have 'Comparable' generic interface")
}

// =============================================================================
// Line Number Tests
// =============================================================================

func TestJavaChunker_CorrectLineNumbers(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public class Calculator {
    private int result;

    public int add(int a, int b) {
        return a + b;
    }

    public int subtract(int a, int b) {
        return a - b;
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

	// Class should start at line 3
	assert.Equal(t, 3, classChunk.StartLine, "Class should start at line 3")
	assert.Equal(t, 13, classChunk.EndLine, "Class should end at line 13")

	// Methods should have correct line numbers
	addMethod := methodChunks["add"]
	require.NotNil(t, addMethod)
	assert.Equal(t, 6, addMethod.StartLine, "add method should start at line 6")
	assert.Equal(t, 8, addMethod.EndLine, "add method should end at line 8")

	subtractMethod := methodChunks["subtract"]
	require.NotNil(t, subtractMethod)
	assert.Equal(t, 10, subtractMethod.StartLine, "subtract method should start at line 10")
	assert.Equal(t, 12, subtractMethod.EndLine, "subtract method should end at line 12")
}

// =============================================================================
// Content Tests
// =============================================================================

func TestJavaChunker_ChunkContent(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public class Greeter {
    public String greet(String name) {
        return "Hello, " + name + "!";
    }
}
`)

	file := &models.SourceFile{
		Path:         "/test/Greeter.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find the method chunk
	var methodChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunk = chunk
			break
		}
	}

	require.NotNil(t, methodChunk)
	assert.Contains(t, methodChunk.Content, "public String greet", "Chunk content should contain the method definition")
	assert.Contains(t, methodChunk.Content, "return", "Chunk content should contain the return statement")
}

// =============================================================================
// Signature Tests
// =============================================================================

func TestJavaChunker_MethodSignature(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public class Service {
    public <T> List<T> process(Collection<T> items, Predicate<T> filter) throws IOException {
        return items.stream().filter(filter).collect(Collectors.toList());
    }
}
`)

	file := &models.SourceFile{
		Path:         "/test/Service.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	// Find the method chunk
	var methodChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunk = chunk
			break
		}
	}

	require.NotNil(t, methodChunk)
	assert.Equal(t, "process", methodChunk.Name)
	assert.NotEmpty(t, methodChunk.Signature, "Should have a signature")
	assert.Contains(t, methodChunk.Signature, "process", "Signature should contain method name")
}

// =============================================================================
// Deterministic ID Tests
// =============================================================================

func TestJavaChunker_DeterministicIDs(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public class Calculator {
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

// =============================================================================
// Context Cancellation Tests
// =============================================================================

func TestJavaChunker_ContextCancellation(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example;

public class Test {
    public void method() {}
}
`)

	file := &models.SourceFile{
		Path:         "/test/Test.java",
		Content:      source,
		Language:     "java",
		LastModified: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := chunker.Chunk(ctx, file)
	// Should either return an error or handle gracefully
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled, "Should return context.Canceled error")
	}
}

// =============================================================================
// Comprehensive Integration Test
// =============================================================================

func TestJavaChunker_ComprehensiveFile(t *testing.T) {
	chunker := getJavaChunker(t)

	source := []byte(`package com.example.service;

import java.util.List;
import java.util.Optional;

/**
 * Service interface for managing users.
 */
public interface UserService {
    Optional<User> findById(long id);
    List<User> findAll();
    void save(User user);
}

/**
 * User data transfer object.
 */
public record User(long id, String name, String email) {}

/**
 * User status enumeration.
 */
public enum UserStatus {
    ACTIVE, INACTIVE, PENDING;

    public boolean isActive() {
        return this == ACTIVE;
    }
}

/**
 * Default implementation of UserService.
 */
public class UserServiceImpl implements UserService {
    private final List<User> users;

    public UserServiceImpl() {
        this.users = new ArrayList<>();
    }

    public UserServiceImpl(List<User> users) {
        this.users = users;
    }

    @Override
    public Optional<User> findById(long id) {
        return users.stream()
            .filter(u -> u.id() == id)
            .findFirst();
    }

    @Override
    public List<User> findAll() {
        return List.copyOf(users);
    }

    @Override
    public void save(User user) {
        users.add(user);
    }

    private void validate(User user) {
        if (user == null) {
            throw new IllegalArgumentException("User cannot be null");
        }
    }
}
`)

	file := &models.SourceFile{
		Path:         "/test/UserService.java",
		Content:      source,
		Language:     "java",
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
	// - 4 class-level chunks (UserService interface, User record, UserStatus enum, UserServiceImpl class)
	// - 8 method-level chunks (3 interface methods extracted? + isActive + 2 constructors + 4 methods)
	// Note: Interface method declarations may or may not be extracted depending on implementation
	assert.Equal(t, 1, levelCounts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.Equal(t, 4, levelCounts[models.ChunkLevelClass], "Should have 4 class-level chunks")
	assert.GreaterOrEqual(t, levelCounts[models.ChunkLevelMethod], 7, "Should have at least 7 method-level chunks")

	// Verify all chunks are valid
	for _, chunk := range result.Chunks {
		assert.NotEmpty(t, chunk.ID, "Chunk should have an ID")
		assert.Equal(t, "java", chunk.Language, "Chunk should have language set")
	}
}
