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

// getGoChunker returns the Go chunker from the config-driven registry.
// This replaces the legacy NewGoChunker function.
func getGoChunker(t *testing.T) Chunker {
	t.Helper()
	registry, err := NewChunkerRegistry()
	require.NoError(t, err, "Failed to create chunker registry")

	chunker, ok := registry.GetChunkerForExtension(".go")
	require.True(t, ok, "Go chunker should be available")
	return chunker
}

// =============================================================================
// GoChunker Initialization Tests
// =============================================================================

func TestGoChunker_Available(t *testing.T) {
	registry, err := NewChunkerRegistry()
	require.NoError(t, err)

	chunker, ok := registry.GetChunkerForExtension(".go")
	assert.True(t, ok, "Go chunker should be available in registry")
	assert.NotNil(t, chunker, "Go chunker should not be nil")
}

func TestGoChunker_Language(t *testing.T) {
	chunker := getGoChunker(t)
	assert.Equal(t, LangGo, chunker.Language(), "GoChunker should report Go as its language")
}

// =============================================================================
// Empty File Tests
// =============================================================================

func TestGoChunker_EmptyFile(t *testing.T) {
	chunker := getGoChunker(t)

	file := &models.SourceFile{
		Path:         "/test/empty.go",
		Content:      []byte(""),
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Empty file should not error
	assert.Empty(t, result.Errors, "Should have no errors for empty file")
}

// =============================================================================
// Simple Function Tests
// =============================================================================

func TestGoChunker_SimpleFunction(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package main

func hello() {
	println("Hello, World!")
}

func greet(name string) string {
	return "Hello, " + name
}
`)

	file := &models.SourceFile{
		Path:         "/test/functions.go",
		Content:      source,
		Language:     "go",
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
	assert.True(t, funcNames["hello"], "Should have 'hello' function")
	assert.True(t, funcNames["greet"], "Should have 'greet' function")
}

// =============================================================================
// Method with Receiver Tests
// =============================================================================

func TestGoChunker_MethodWithReceiver(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package main

type User struct {
	Name string
	Age  int
}

func (u User) GetName() string {
	return u.Name
}

func (u *User) SetName(name string) {
	u.Name = name
}
`)

	file := &models.SourceFile{
		Path:         "/test/methods.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have: 1 file chunk + 1 struct chunk + 2 method chunks = 4 chunks
	assert.Len(t, result.Chunks, 4, "Should have 4 chunks: file, struct, and 2 methods")

	// Find chunks by level
	var fileChunk, structChunk *models.Chunk
	var methodChunks []*models.Chunk

	for _, chunk := range result.Chunks {
		switch chunk.Level {
		case models.ChunkLevelFile:
			fileChunk = chunk
		case models.ChunkLevelClass:
			structChunk = chunk
		case models.ChunkLevelMethod:
			methodChunks = append(methodChunks, chunk)
		}
	}

	// Verify file chunk
	require.NotNil(t, fileChunk, "Should have a file-level chunk")
	assert.Equal(t, "/test/methods.go", fileChunk.FilePath)

	// Verify struct chunk
	require.NotNil(t, structChunk, "Should have a class-level chunk for struct")
	assert.Equal(t, "User", structChunk.Name, "Struct chunk should be named 'User'")
	assert.Equal(t, fileChunk.ID, *structChunk.ParentID, "Struct should reference file as parent")

	// Verify method chunks
	assert.Len(t, methodChunks, 2, "Should have 2 method chunks")

	// Methods should reference file as parent (Go methods can be in different files)
	for _, method := range methodChunks {
		require.NotNil(t, method.ParentID, "Method should have a parent ID")
		assert.Equal(t, fileChunk.ID, *method.ParentID, "Method should reference file as parent")
	}

	// Verify method names
	methodNames := make(map[string]bool)
	for _, m := range methodChunks {
		methodNames[m.Name] = true
	}
	assert.True(t, methodNames["GetName"], "Should have 'GetName' method")
	assert.True(t, methodNames["SetName"], "Should have 'SetName' method")
}

// =============================================================================
// Struct Type Tests
// =============================================================================

func TestGoChunker_StructType(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package models

type Person struct {
	FirstName string
	LastName  string
	Age       int
	Email     string
}

type Address struct {
	Street  string
	City    string
	Country string
}
`)

	file := &models.SourceFile{
		Path:         "/test/structs.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have: 1 file chunk + 2 struct chunks = 3 chunks
	assert.Len(t, result.Chunks, 3, "Should have 3 chunks: file and 2 structs")

	// Find struct chunks
	var structChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			structChunks = append(structChunks, chunk)
		}
	}

	assert.Len(t, structChunks, 2, "Should have 2 struct chunks at class level")

	// Verify struct names
	structNames := make(map[string]bool)
	for _, s := range structChunks {
		structNames[s.Name] = true
	}
	assert.True(t, structNames["Person"], "Should have 'Person' struct")
	assert.True(t, structNames["Address"], "Should have 'Address' struct")
}

// =============================================================================
// Interface Type Tests
// =============================================================================

func TestGoChunker_InterfaceType(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package io

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}

type ReadWriter interface {
	Reader
	Writer
}
`)

	file := &models.SourceFile{
		Path:         "/test/interfaces.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have: 1 file chunk + 3 interface chunks = 4 chunks
	assert.Len(t, result.Chunks, 4, "Should have 4 chunks: file and 3 interfaces")

	// Find interface chunks
	var interfaceChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			interfaceChunks = append(interfaceChunks, chunk)
		}
	}

	assert.Len(t, interfaceChunks, 3, "Should have 3 interface chunks at class level")

	// Verify interface names
	interfaceNames := make(map[string]bool)
	for _, i := range interfaceChunks {
		interfaceNames[i.Name] = true
	}
	assert.True(t, interfaceNames["Reader"], "Should have 'Reader' interface")
	assert.True(t, interfaceNames["Writer"], "Should have 'Writer' interface")
	assert.True(t, interfaceNames["ReadWriter"], "Should have 'ReadWriter' interface")
}

// =============================================================================
// Multiple Constructs Tests
// =============================================================================

func TestGoChunker_MultipleConstructs(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package service

type Service interface {
	Start() error
	Stop() error
}

type Config struct {
	Host string
	Port int
}

type Server struct {
	config Config
}

func NewServer(cfg Config) *Server {
	return &Server{config: cfg}
}

func (s *Server) Start() error {
	return nil
}

func (s *Server) Stop() error {
	return nil
}

func helper() {
	// internal helper
}
`)

	file := &models.SourceFile{
		Path:         "/test/service.go",
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

	// Should have:
	// - 1 file chunk
	// - 3 class chunks (Service interface, Config struct, Server struct)
	// - 4 method chunks (NewServer, Start, Stop, helper)
	assert.Equal(t, 1, levelCounts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.Equal(t, 3, levelCounts[models.ChunkLevelClass], "Should have 3 class-level chunks")
	assert.Equal(t, 4, levelCounts[models.ChunkLevelMethod], "Should have 4 method-level chunks")
}

// =============================================================================
// Type Alias Tests
// =============================================================================

func TestGoChunker_TypeAlias(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package types

type ID = string

type UserID string

type Handler func(w Writer, r Reader)
`)

	file := &models.SourceFile{
		Path:         "/test/aliases.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Type aliases and custom types should be extracted
	// The exact level (class or not) depends on implementation choice
	// At minimum, should not error and should have file chunk
	require.NotEmpty(t, result.Chunks, "Should have at least a file chunk")

	var fileChunk *models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelFile {
			fileChunk = chunk
			break
		}
	}
	require.NotNil(t, fileChunk, "Should have a file chunk")
}

// =============================================================================
// Generic Types Tests (Go 1.18+)
// =============================================================================

func TestGoChunker_GenericTypes(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package collections

type Stack[T any] struct {
	items []T
}

func (s *Stack[T]) Push(item T) {
	s.items = append(s.items, item)
}

func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	item := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return item, true
}

type Pair[K comparable, V any] struct {
	Key   K
	Value V
}

func Map[T, U any](items []T, fn func(T) U) []U {
	result := make([]U, len(items))
	for i, item := range items {
		result[i] = fn(item)
	}
	return result
}
`)

	file := &models.SourceFile{
		Path:         "/test/generics.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find chunks
	var structChunks []*models.Chunk
	var funcChunks []*models.Chunk

	for _, chunk := range result.Chunks {
		switch chunk.Level {
		case models.ChunkLevelClass:
			structChunks = append(structChunks, chunk)
		case models.ChunkLevelMethod:
			funcChunks = append(funcChunks, chunk)
		}
	}

	// Should extract generic struct types
	assert.Len(t, structChunks, 2, "Should have 2 generic struct chunks (Stack, Pair)")

	structNames := make(map[string]bool)
	for _, s := range structChunks {
		structNames[s.Name] = true
	}
	assert.True(t, structNames["Stack"], "Should have 'Stack' generic struct")
	assert.True(t, structNames["Pair"], "Should have 'Pair' generic struct")

	// Should extract generic functions and methods
	assert.Len(t, funcChunks, 3, "Should have 3 function/method chunks (Push, Pop, Map)")

	funcNames := make(map[string]bool)
	for _, f := range funcChunks {
		funcNames[f.Name] = true
	}
	assert.True(t, funcNames["Push"], "Should have 'Push' method")
	assert.True(t, funcNames["Pop"], "Should have 'Pop' method")
	assert.True(t, funcNames["Map"], "Should have 'Map' generic function")
}

// =============================================================================
// Line Number Tests
// =============================================================================

func TestGoChunker_CorrectLineNumbers(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package main

type Calculator struct {
	result int
}

func (c *Calculator) Add(a, b int) int {
	return a + b
}

func (c *Calculator) Subtract(a, b int) int {
	return a - b
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

	// Find specific chunks
	var structChunk *models.Chunk
	methodChunks := make(map[string]*models.Chunk)

	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			structChunk = chunk
		} else if chunk.Level == models.ChunkLevelMethod {
			methodChunks[chunk.Name] = chunk
		}
	}

	require.NotNil(t, structChunk)

	// Struct should start at line 3
	assert.Equal(t, 3, structChunk.StartLine, "Struct should start at line 3")
	assert.Equal(t, 5, structChunk.EndLine, "Struct should end at line 5")

	// Methods should have correct line numbers
	addMethod := methodChunks["Add"]
	require.NotNil(t, addMethod)
	assert.Equal(t, 7, addMethod.StartLine, "Add method should start at line 7")
	assert.Equal(t, 9, addMethod.EndLine, "Add method should end at line 9")

	subtractMethod := methodChunks["Subtract"]
	require.NotNil(t, subtractMethod)
	assert.Equal(t, 11, subtractMethod.StartLine, "Subtract method should start at line 11")
	assert.Equal(t, 13, subtractMethod.EndLine, "Subtract method should end at line 13")
}

// =============================================================================
// Content Tests
// =============================================================================

func TestGoChunker_ChunkContent(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package main

func greet(name string) string {
	return "Hello, " + name + "!"
}
`)

	file := &models.SourceFile{
		Path:         "/test/content.go",
		Content:      source,
		Language:     "go",
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
	assert.Contains(t, funcChunk.Content, "func greet", "Chunk content should contain the function definition")
	assert.Contains(t, funcChunk.Content, "return", "Chunk content should contain the return statement")
}

// =============================================================================
// Signature Tests
// =============================================================================

func TestGoChunker_FunctionSignature(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package main

func calculate(x, y int, opts ...Option) (int, error) {
	return x + y, nil
}
`)

	file := &models.SourceFile{
		Path:         "/test/signature.go",
		Content:      source,
		Language:     "go",
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
	assert.NotEmpty(t, funcChunk.Signature, "Should have a signature")
	assert.Contains(t, funcChunk.Signature, "func calculate", "Signature should contain function name")
}

// =============================================================================
// Deterministic ID Tests
// =============================================================================

func TestGoChunker_DeterministicIDs(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package main

type Calculator struct{}

func (c *Calculator) Add(a, b int) int {
	return a + b
}
`)

	file := &models.SourceFile{
		Path:         "/test/calc.go",
		Content:      source,
		Language:     "go",
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

func TestGoChunker_ContextCancellation(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package main

type Test struct{}

func (t *Test) Method() {}
`)

	file := &models.SourceFile{
		Path:         "/test/cancel.go",
		Content:      source,
		Language:     "go",
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
// Edge Cases Tests
// =============================================================================

func TestGoChunker_OnlyPackageDeclaration(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package main
`)

	file := &models.SourceFile{
		Path:         "/test/package_only.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should handle gracefully with just a file chunk
	assert.Empty(t, result.Errors, "Should have no errors")
}

func TestGoChunker_WithImports(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println(os.Args)
}
`)

	file := &models.SourceFile{
		Path:         "/test/imports.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Imports should not create separate chunks, just file + function
	var funcChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			funcChunks = append(funcChunks, chunk)
		}
	}

	assert.Len(t, funcChunks, 1, "Should have 1 function chunk (main)")
	assert.Equal(t, "main", funcChunks[0].Name)
}

func TestGoChunker_InitFunction(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package main

var globalVar int

func init() {
	globalVar = 42
}

func init() {
	globalVar += 1
}

func main() {
	println(globalVar)
}
`)

	file := &models.SourceFile{
		Path:         "/test/init.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should extract multiple init functions and main
	var funcChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelMethod {
			funcChunks = append(funcChunks, chunk)
		}
	}

	// Go allows multiple init() functions
	assert.GreaterOrEqual(t, len(funcChunks), 2, "Should have at least 2 function chunks")
}

func TestGoChunker_ExportedVsUnexported(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package mypackage

type PublicStruct struct {
	PublicField  string
	privateField int
}

type privateStruct struct {
	field string
}

func PublicFunc() {}

func privateFunc() {}

func (p *PublicStruct) PublicMethod() {}

func (p *PublicStruct) privateMethod() {}
`)

	file := &models.SourceFile{
		Path:         "/test/visibility.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should extract both exported and unexported constructs
	var structChunks []*models.Chunk
	var funcChunks []*models.Chunk

	for _, chunk := range result.Chunks {
		switch chunk.Level {
		case models.ChunkLevelClass:
			structChunks = append(structChunks, chunk)
		case models.ChunkLevelMethod:
			funcChunks = append(funcChunks, chunk)
		}
	}

	assert.Len(t, structChunks, 2, "Should have 2 struct chunks (public and private)")
	assert.Len(t, funcChunks, 4, "Should have 4 function/method chunks")

	// Verify names
	structNames := make(map[string]bool)
	for _, s := range structChunks {
		structNames[s.Name] = true
	}
	assert.True(t, structNames["PublicStruct"], "Should have 'PublicStruct'")
	assert.True(t, structNames["privateStruct"], "Should have 'privateStruct'")
}

func TestGoChunker_EmbeddedStructs(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`package main

type Base struct {
	ID int
}

type Derived struct {
	Base
	Name string
}
`)

	file := &models.SourceFile{
		Path:         "/test/embedded.go",
		Content:      source,
		Language:     "go",
		LastModified: time.Now(),
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should extract both structs
	var structChunks []*models.Chunk
	for _, chunk := range result.Chunks {
		if chunk.Level == models.ChunkLevelClass {
			structChunks = append(structChunks, chunk)
		}
	}

	assert.Len(t, structChunks, 2, "Should have 2 struct chunks")

	structNames := make(map[string]bool)
	for _, s := range structChunks {
		structNames[s.Name] = true
	}
	assert.True(t, structNames["Base"], "Should have 'Base' struct")
	assert.True(t, structNames["Derived"], "Should have 'Derived' struct")
}

// =============================================================================
// Comprehensive Integration Test
// =============================================================================

func TestGoChunker_ComprehensiveFile(t *testing.T) {
	chunker := getGoChunker(t)

	source := []byte(`// Package example provides an example for testing.
package example

import (
	"fmt"
	"io"
)

// Reader is an interface for reading data.
type Reader interface {
	Read(p []byte) (n int, err error)
}

// Writer is an interface for writing data.
type Writer interface {
	Write(p []byte) (n int, err error)
}

// Buffer holds data.
type Buffer struct {
	data []byte
}

// NewBuffer creates a new buffer.
func NewBuffer() *Buffer {
	return &Buffer{}
}

// Read implements io.Reader.
func (b *Buffer) Read(p []byte) (n int, err error) {
	copy(p, b.data)
	return len(b.data), io.EOF
}

// Write implements io.Writer.
func (b *Buffer) Write(p []byte) (n int, err error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

// String returns the buffer contents as a string.
func (b *Buffer) String() string {
	return string(b.data)
}

func helper() {
	fmt.Println("helper")
}
`)

	file := &models.SourceFile{
		Path:         "/test/comprehensive.go",
		Content:      source,
		Language:     "go",
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
	// - 3 class chunks (Reader interface, Writer interface, Buffer struct)
	// - 5 method chunks (NewBuffer, Read, Write, String, helper)
	assert.Equal(t, 1, levelCounts[models.ChunkLevelFile], "Should have 1 file chunk")
	assert.Equal(t, 3, levelCounts[models.ChunkLevelClass], "Should have 3 class chunks")
	assert.Equal(t, 5, levelCounts[models.ChunkLevelMethod], "Should have 5 method chunks")

	// Verify all chunks are valid
	for _, chunk := range result.Chunks {
		err := chunk.IsValid()
		assert.NoError(t, err, "Chunk %s should be valid", chunk.Name)
		assert.NotEmpty(t, chunk.ID, "Chunk should have an ID")
		assert.Equal(t, "go", chunk.Language, "Chunk should have language set")
	}
}
