//go:build benchmark

package benchmarks

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/chunker"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/pommel-dev/pommel/internal/models"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupTestDB creates a migrated database in a temporary directory.
func setupTestDB(b *testing.B) *db.DB {
	b.Helper()
	tmpDir := b.TempDir()
	database, err := db.Open(tmpDir)
	if err != nil {
		b.Fatalf("failed to open database: %v", err)
	}

	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		database.Close()
		b.Fatalf("failed to migrate database: %v", err)
	}

	b.Cleanup(func() {
		database.Close()
	})

	return database
}

// generateRandomEmbedding creates a random 768-dimensional embedding vector.
func generateRandomEmbedding() []float32 {
	embedding := make([]float32, db.EmbeddingDimension)
	bytes := make([]byte, db.EmbeddingDimension*4)
	rand.Read(bytes)
	for i := 0; i < db.EmbeddingDimension; i++ {
		// Convert 4 bytes to float32-ish value between 0 and 1
		val := float32(bytes[i*4]) / 255.0
		embedding[i] = val
	}
	return embedding
}

// createTestChunk creates a test chunk with the given parameters.
func createTestChunk(filePath string, startLine, endLine int, level models.ChunkLevel, name, content string) *models.Chunk {
	chunk := &models.Chunk{
		FilePath:     filePath,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        level,
		Language:     "python",
		Content:      content,
		Name:         name,
		LastModified: time.Now(),
	}
	chunk.SetHashes()
	return chunk
}

// populateDBWithChunks populates the database with the specified number of chunks
// and their embeddings. Returns the chunk IDs for use in queries.
func populateDBWithChunks(b *testing.B, database *db.DB, numChunks int) []string {
	b.Helper()
	ctx := context.Background()

	chunkIDs := make([]string, numChunks)
	embeddings := make([][]float32, numChunks)

	// Insert files and chunks in batches
	filesPerBatch := 100
	chunksPerFile := (numChunks + filesPerBatch - 1) / filesPerBatch

	chunkIndex := 0
	for fileNum := 0; fileNum < filesPerBatch && chunkIndex < numChunks; fileNum++ {
		filePath := fmt.Sprintf("/test/file_%d.py", fileNum)
		fileID, err := database.InsertFile(ctx, filePath, fmt.Sprintf("hash_%d", fileNum), "python", 1024, time.Now())
		if err != nil {
			b.Fatalf("failed to insert file: %v", err)
		}

		for chunkNum := 0; chunkNum < chunksPerFile && chunkIndex < numChunks; chunkNum++ {
			chunk := createTestChunk(
				filePath,
				chunkNum*10+1,
				chunkNum*10+10,
				models.ChunkLevelMethod,
				fmt.Sprintf("function_%d", chunkIndex),
				fmt.Sprintf("def function_%d():\n    pass", chunkIndex),
			)

			if err := database.InsertChunk(ctx, chunk, fileID); err != nil {
				b.Fatalf("failed to insert chunk: %v", err)
			}

			chunkIDs[chunkIndex] = chunk.ID
			embeddings[chunkIndex] = generateRandomEmbedding()
			chunkIndex++
		}
	}

	// Bulk insert embeddings
	if err := database.InsertEmbeddings(ctx, chunkIDs, embeddings); err != nil {
		b.Fatalf("failed to insert embeddings: %v", err)
	}

	return chunkIDs
}

// generateRealisticPythonFile generates a realistic Python file with classes and methods.
func generateRealisticPythonFile(numLines int) []byte {
	var sb strings.Builder

	// Module docstring
	sb.WriteString(`"""
Module for handling data processing operations.
This module provides utilities for transforming and analyzing data.
"""

import os
import sys
from typing import List, Dict, Optional, Any
from dataclasses import dataclass
from abc import ABC, abstractmethod

`)

	classNum := 0
	methodNum := 0
	currentLine := 12 // Account for imports

	for currentLine < numLines {
		// Add a class
		className := fmt.Sprintf("DataProcessor%d", classNum)
		sb.WriteString(fmt.Sprintf(`
class %s:
    """
    A data processor class that handles various data operations.

    Attributes:
        name: The name of the processor
        config: Configuration dictionary
    """

    def __init__(self, name: str, config: Optional[Dict[str, Any]] = None):
        """Initialize the processor with name and optional config."""
        self.name = name
        self.config = config or {}
        self._cache = {}
        self._initialized = False

`, className))
		currentLine += 20

		// Add methods to the class
		for i := 0; i < 5 && currentLine < numLines; i++ {
			methodName := fmt.Sprintf("process_data_%d", methodNum)
			sb.WriteString(fmt.Sprintf(`    def %s(self, data: List[Any]) -> List[Any]:
        """
        Process the input data and return transformed results.

        Args:
            data: List of data items to process

        Returns:
            Transformed list of data items
        """
        if not data:
            return []

        results = []
        for item in data:
            if isinstance(item, dict):
                processed = self._process_dict(item)
            elif isinstance(item, list):
                processed = self._process_list(item)
            else:
                processed = self._process_primitive(item)
            results.append(processed)

        return results

`, methodName))
			currentLine += 25
			methodNum++
		}

		classNum++
	}

	// Add some standalone functions
	sb.WriteString(`
def main():
    """Main entry point for the module."""
    processor = DataProcessor0("main", {"debug": True})
    data = [1, 2, 3, {"key": "value"}, [4, 5, 6]]
    result = processor.process_data_0(data)
    print(f"Processed {len(result)} items")

if __name__ == "__main__":
    main()
`)

	return []byte(sb.String())
}

// generateRealisticJavaScriptFile generates a realistic JavaScript/TypeScript file.
func generateRealisticJavaScriptFile(numLines int) []byte {
	var sb strings.Builder

	sb.WriteString(`/**
 * Data processing utilities for the application.
 * @module dataProcessor
 */

import { EventEmitter } from 'events';

`)

	classNum := 0
	methodNum := 0
	currentLine := 8

	for currentLine < numLines {
		className := fmt.Sprintf("DataHandler%d", classNum)
		sb.WriteString(fmt.Sprintf(`
/**
 * Handles data processing operations.
 */
class %s extends EventEmitter {
    /**
     * Creates a new DataHandler instance.
     * @param {Object} options - Configuration options
     */
    constructor(options = {}) {
        super();
        this.options = options;
        this.cache = new Map();
        this.initialized = false;
    }

`, className))
		currentLine += 16

		// Add methods
		for i := 0; i < 5 && currentLine < numLines; i++ {
			methodName := fmt.Sprintf("processItem%d", methodNum)
			sb.WriteString(fmt.Sprintf(`    /**
     * Processes a single data item.
     * @param {any} item - The item to process
     * @returns {any} The processed item
     */
    %s(item) {
        if (!item) {
            return null;
        }

        const cacheKey = JSON.stringify(item);
        if (this.cache.has(cacheKey)) {
            return this.cache.get(cacheKey);
        }

        let result;
        if (Array.isArray(item)) {
            result = item.map(x => this.transform(x));
        } else if (typeof item === 'object') {
            result = Object.fromEntries(
                Object.entries(item).map(([k, v]) => [k, this.transform(v)])
            );
        } else {
            result = this.transform(item);
        }

        this.cache.set(cacheKey, result);
        this.emit('processed', result);
        return result;
    }

`, methodName))
			currentLine += 30
			methodNum++
		}

		sb.WriteString(`}

`)
		classNum++
	}

	// Add exports
	sb.WriteString(`
export default DataHandler0;
export { DataHandler0, DataHandler1 };
`)

	return []byte(sb.String())
}

// =============================================================================
// Benchmark: Vector Search Performance
// =============================================================================

// BenchmarkSearch_VectorSearch measures vector search performance with a pre-populated database.
// Target: < 100ms for searching 1000 chunks and returning 10 results.
func BenchmarkSearch_VectorSearch(b *testing.B) {
	database := setupTestDB(b)
	ctx := context.Background()

	// Pre-populate with 1000 chunks
	populateDBWithChunks(b, database, 1000)

	// Generate a query embedding
	queryEmbedding := generateRandomEmbedding()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		results, err := database.SearchSimilar(ctx, queryEmbedding, 10)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
		if len(results) == 0 {
			b.Fatal("expected results, got none")
		}
	}
}

// BenchmarkSearch_VectorSearchWithFilter measures filtered vector search performance.
func BenchmarkSearch_VectorSearchWithFilter(b *testing.B) {
	database := setupTestDB(b)
	ctx := context.Background()

	// Pre-populate with 1000 chunks
	chunkIDs := populateDBWithChunks(b, database, 1000)

	// Generate a query embedding
	queryEmbedding := generateRandomEmbedding()

	// Use a subset of chunk IDs as filter (simulating path/level filtering)
	filterIDs := chunkIDs[:500]

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		results, err := database.SearchSimilarFiltered(ctx, queryEmbedding, 10, filterIDs)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
		if len(results) == 0 {
			b.Fatal("expected results, got none")
		}
	}
}

// =============================================================================
// Benchmark: Chunker Performance
// =============================================================================

// BenchmarkChunker_Python measures Python file chunking performance.
// Target: < 50ms per 500-line file.
func BenchmarkChunker_Python(b *testing.B) {
	registry, err := chunker.NewChunkerRegistry()
	if err != nil {
		b.Fatalf("failed to create chunker registry: %v", err)
	}

	// Generate a realistic 500-line Python file
	content := generateRealisticPythonFile(500)

	sourceFile := &models.SourceFile{
		Path:         "/test/processor.py",
		Content:      content,
		Language:     "python",
		LastModified: time.Now(),
	}

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := registry.Chunk(ctx, sourceFile)
		if err != nil {
			b.Fatalf("chunking failed: %v", err)
		}
		if len(result.Chunks) == 0 {
			b.Fatal("expected chunks, got none")
		}
	}
}

// BenchmarkChunker_PythonLarge measures chunking performance for larger files.
func BenchmarkChunker_PythonLarge(b *testing.B) {
	registry, err := chunker.NewChunkerRegistry()
	if err != nil {
		b.Fatalf("failed to create chunker registry: %v", err)
	}

	// Generate a larger 2000-line Python file
	content := generateRealisticPythonFile(2000)

	sourceFile := &models.SourceFile{
		Path:         "/test/large_processor.py",
		Content:      content,
		Language:     "python",
		LastModified: time.Now(),
	}

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := registry.Chunk(ctx, sourceFile)
		if err != nil {
			b.Fatalf("chunking failed: %v", err)
		}
		if len(result.Chunks) == 0 {
			b.Fatal("expected chunks, got none")
		}
	}
}

// BenchmarkChunker_JavaScript measures JavaScript/TypeScript file chunking performance.
func BenchmarkChunker_JavaScript(b *testing.B) {
	registry, err := chunker.NewChunkerRegistry()
	if err != nil {
		b.Fatalf("failed to create chunker registry: %v", err)
	}

	// Generate a realistic 500-line JavaScript file
	content := generateRealisticJavaScriptFile(500)

	sourceFile := &models.SourceFile{
		Path:         "/test/handler.js",
		Content:      content,
		Language:     "javascript",
		LastModified: time.Now(),
	}

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := registry.Chunk(ctx, sourceFile)
		if err != nil {
			b.Fatalf("chunking failed: %v", err)
		}
		if len(result.Chunks) == 0 {
			b.Fatal("expected chunks, got none")
		}
	}
}

// BenchmarkChunker_TypeScript measures TypeScript file chunking performance.
func BenchmarkChunker_TypeScript(b *testing.B) {
	registry, err := chunker.NewChunkerRegistry()
	if err != nil {
		b.Fatalf("failed to create chunker registry: %v", err)
	}

	content := generateRealisticJavaScriptFile(500)

	sourceFile := &models.SourceFile{
		Path:         "/test/handler.ts",
		Content:      content,
		Language:     "typescript",
		LastModified: time.Now(),
	}

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := registry.Chunk(ctx, sourceFile)
		if err != nil {
			b.Fatalf("chunking failed: %v", err)
		}
		if len(result.Chunks) == 0 {
			b.Fatal("expected chunks, got none")
		}
	}
}

// =============================================================================
// Benchmark: Database Insert Performance
// =============================================================================

// BenchmarkDB_InsertChunk measures single chunk insert performance.
// Target: < 10ms per chunk.
func BenchmarkDB_InsertChunk(b *testing.B) {
	database := setupTestDB(b)
	ctx := context.Background()

	// Pre-create a file to insert chunks into
	fileID, err := database.InsertFile(ctx, "/test/benchmark.py", "hash123", "python", 1024, time.Now())
	if err != nil {
		b.Fatalf("failed to insert file: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		chunk := createTestChunk(
			"/test/benchmark.py",
			i*10+1,
			i*10+10,
			models.ChunkLevelMethod,
			fmt.Sprintf("function_%d", i),
			fmt.Sprintf("def function_%d():\n    pass", i),
		)

		if err := database.InsertChunk(ctx, chunk, fileID); err != nil {
			b.Fatalf("failed to insert chunk: %v", err)
		}
	}
}

// BenchmarkDB_InsertEmbedding measures single embedding insert performance.
func BenchmarkDB_InsertEmbedding(b *testing.B) {
	database := setupTestDB(b)
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		chunkID := fmt.Sprintf("chunk_%d", i)
		embedding := generateRandomEmbedding()

		if err := database.InsertEmbedding(ctx, chunkID, embedding); err != nil {
			b.Fatalf("failed to insert embedding: %v", err)
		}
	}
}

// BenchmarkDB_BulkInsert measures bulk insert performance for 100 chunks.
// Target: < 500ms for 100 chunks with embeddings.
func BenchmarkDB_BulkInsert(b *testing.B) {
	database := setupTestDB(b)
	ctx := context.Background()

	const batchSize = 100

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Create a file for this batch
		filePath := fmt.Sprintf("/test/batch_%d.py", i)
		fileID, err := database.InsertFile(ctx, filePath, fmt.Sprintf("hash_%d", i), "python", 1024, time.Now())
		if err != nil {
			b.Fatalf("failed to insert file: %v", err)
		}

		chunkIDs := make([]string, batchSize)
		embeddings := make([][]float32, batchSize)

		// Insert chunks
		for j := 0; j < batchSize; j++ {
			chunk := createTestChunk(
				filePath,
				j*10+1,
				j*10+10,
				models.ChunkLevelMethod,
				fmt.Sprintf("function_%d_%d", i, j),
				fmt.Sprintf("def function_%d_%d():\n    pass", i, j),
			)

			if err := database.InsertChunk(ctx, chunk, fileID); err != nil {
				b.Fatalf("failed to insert chunk: %v", err)
			}

			chunkIDs[j] = chunk.ID
			embeddings[j] = generateRandomEmbedding()
		}

		// Bulk insert embeddings
		if err := database.InsertEmbeddings(ctx, chunkIDs, embeddings); err != nil {
			b.Fatalf("failed to insert embeddings: %v", err)
		}
	}
}

// =============================================================================
// Benchmark: Embedder Cache Performance
// =============================================================================

// BenchmarkEmbedder_Cache measures cache hit performance.
// Target: < 1ms for cache hits.
func BenchmarkEmbedder_Cache(b *testing.B) {
	// Create a mock embedder
	mockEmb := embedder.NewMockEmbedder()

	// Wrap with cache (capacity of 1000)
	cachedEmb := embedder.NewCachedEmbedder(mockEmb, 1000)

	ctx := context.Background()

	// Pre-populate cache with test texts
	testTexts := make([]string, 100)
	for i := 0; i < 100; i++ {
		testTexts[i] = fmt.Sprintf("def function_%d():\n    return %d", i, i)
		// Warm up cache
		_, err := cachedEmb.EmbedSingle(ctx, testTexts[i])
		if err != nil {
			b.Fatalf("failed to embed: %v", err)
		}
	}

	b.ResetTimer()

	// Benchmark cache hits
	for i := 0; i < b.N; i++ {
		text := testTexts[i%100]
		_, err := cachedEmb.EmbedSingle(ctx, text)
		if err != nil {
			b.Fatalf("failed to get cached embedding: %v", err)
		}
	}

	b.StopTimer()

	// Verify we got cache hits
	metrics := cachedEmb.Metrics()
	if metrics.Hits < int64(b.N) {
		b.Logf("Warning: Expected %d cache hits, got %d (misses: %d)", b.N, metrics.Hits, metrics.Misses)
	}
}

// BenchmarkEmbedder_CacheMiss measures cache miss performance (includes embedding generation).
func BenchmarkEmbedder_CacheMiss(b *testing.B) {
	// Create a mock embedder
	mockEmb := embedder.NewMockEmbedder()

	// Wrap with cache (small capacity to force evictions)
	cachedEmb := embedder.NewCachedEmbedder(mockEmb, 10)

	ctx := context.Background()

	b.ResetTimer()

	// Benchmark cache misses (each text is unique)
	for i := 0; i < b.N; i++ {
		text := fmt.Sprintf("unique_function_%d():\n    return %d", i, i)
		_, err := cachedEmb.EmbedSingle(ctx, text)
		if err != nil {
			b.Fatalf("failed to embed: %v", err)
		}
	}
}

// BenchmarkEmbedder_BatchCache measures batch embedding with partial cache hits.
func BenchmarkEmbedder_BatchCache(b *testing.B) {
	mockEmb := embedder.NewMockEmbedder()
	cachedEmb := embedder.NewCachedEmbedder(mockEmb, 1000)

	ctx := context.Background()

	// Pre-populate cache with half the texts
	cachedTexts := make([]string, 50)
	for i := 0; i < 50; i++ {
		cachedTexts[i] = fmt.Sprintf("cached_function_%d():\n    return %d", i, i)
		_, _ = cachedEmb.EmbedSingle(ctx, cachedTexts[i])
	}

	// Create batch with mix of cached and uncached
	batchTexts := make([]string, 100)
	for i := 0; i < 50; i++ {
		batchTexts[i] = cachedTexts[i]                                                // cached
		batchTexts[50+i] = fmt.Sprintf("new_function_%d():\n    return %d", i+b.N, i) // uncached
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := cachedEmb.Embed(ctx, batchTexts)
		if err != nil {
			b.Fatalf("failed to batch embed: %v", err)
		}
	}
}

// =============================================================================
// Benchmark: End-to-End Indexing Pipeline
// =============================================================================

// BenchmarkPipeline_ChunkAndEmbed measures the chunking + embedding pipeline.
func BenchmarkPipeline_ChunkAndEmbed(b *testing.B) {
	registry, err := chunker.NewChunkerRegistry()
	if err != nil {
		b.Fatalf("failed to create chunker registry: %v", err)
	}

	mockEmb := embedder.NewMockEmbedder()
	cachedEmb := embedder.NewCachedEmbedder(mockEmb, 1000)

	// Generate a realistic 500-line Python file
	content := generateRealisticPythonFile(500)

	sourceFile := &models.SourceFile{
		Path:         "/test/processor.py",
		Content:      content,
		Language:     "python",
		LastModified: time.Now(),
	}

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Chunk the file
		result, err := registry.Chunk(ctx, sourceFile)
		if err != nil {
			b.Fatalf("chunking failed: %v", err)
		}

		// Generate embeddings for each chunk
		for _, chunk := range result.Chunks {
			_, err := cachedEmb.EmbedSingle(ctx, chunk.Content)
			if err != nil {
				b.Fatalf("embedding failed: %v", err)
			}
		}
	}
}

// BenchmarkPipeline_FullIndex measures the complete indexing pipeline including DB writes.
func BenchmarkPipeline_FullIndex(b *testing.B) {
	database := setupTestDB(b)

	registry, err := chunker.NewChunkerRegistry()
	if err != nil {
		b.Fatalf("failed to create chunker registry: %v", err)
	}

	mockEmb := embedder.NewMockEmbedder()
	cachedEmb := embedder.NewCachedEmbedder(mockEmb, 1000)

	// Generate a realistic 500-line Python file
	content := generateRealisticPythonFile(500)

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		filePath := fmt.Sprintf("/test/processor_%d.py", i)

		sourceFile := &models.SourceFile{
			Path:         filePath,
			Content:      content,
			Language:     "python",
			LastModified: time.Now(),
		}

		// Insert file
		fileID, err := database.InsertFile(ctx, filePath, fmt.Sprintf("hash_%d", i), "python", int64(len(content)), time.Now())
		if err != nil {
			b.Fatalf("failed to insert file: %v", err)
		}

		// Chunk the file
		result, err := registry.Chunk(ctx, sourceFile)
		if err != nil {
			b.Fatalf("chunking failed: %v", err)
		}

		chunkIDs := make([]string, len(result.Chunks))
		embeddings := make([][]float32, len(result.Chunks))

		// Insert chunks and generate embeddings
		for j, chunk := range result.Chunks {
			if err := database.InsertChunk(ctx, chunk, fileID); err != nil {
				b.Fatalf("failed to insert chunk: %v", err)
			}

			emb, err := cachedEmb.EmbedSingle(ctx, chunk.Content)
			if err != nil {
				b.Fatalf("embedding failed: %v", err)
			}

			chunkIDs[j] = chunk.ID
			embeddings[j] = emb
		}

		// Bulk insert embeddings
		if err := database.InsertEmbeddings(ctx, chunkIDs, embeddings); err != nil {
			b.Fatalf("failed to insert embeddings: %v", err)
		}
	}
}
