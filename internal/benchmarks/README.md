# Pommel Benchmarks

This directory contains performance benchmarks for Pommel's critical paths. These benchmarks help ensure the system meets its performance requirements for AI agent usage.

## Running Benchmarks

### Run All Benchmarks

```bash
go test -tags=benchmark -bench=. ./internal/benchmarks/...
```

### Run Specific Benchmark

```bash
# Vector search benchmarks
go test -tags=benchmark -bench=BenchmarkSearch ./internal/benchmarks/...

# Chunker benchmarks
go test -tags=benchmark -bench=BenchmarkChunker ./internal/benchmarks/...

# Database insert benchmarks
go test -tags=benchmark -bench=BenchmarkDB ./internal/benchmarks/...

# Cache benchmarks
go test -tags=benchmark -bench=BenchmarkEmbedder ./internal/benchmarks/...

# Pipeline benchmarks
go test -tags=benchmark -bench=BenchmarkPipeline ./internal/benchmarks/...
```

### Detailed Output

```bash
# Include memory allocation statistics
go test -tags=benchmark -bench=. -benchmem ./internal/benchmarks/...

# Run each benchmark multiple times for statistical accuracy
go test -tags=benchmark -bench=. -count=5 ./internal/benchmarks/...

# Set minimum benchmark duration
go test -tags=benchmark -bench=. -benchtime=5s ./internal/benchmarks/...
```

### Save Benchmark Results

```bash
# Save results to a file for comparison
go test -tags=benchmark -bench=. ./internal/benchmarks/... > benchmark_results.txt

# Compare with previous run (requires benchstat tool)
go install golang.org/x/perf/cmd/benchstat@latest
benchstat old.txt new.txt
```

## Benchmark Categories

### 1. Vector Search Performance

| Benchmark | Description | Target |
|-----------|-------------|--------|
| `BenchmarkSearch_VectorSearch` | Search 1000 chunks, return 10 results | < 100ms |
| `BenchmarkSearch_VectorSearchWithFilter` | Filtered search with path/level constraints | < 100ms |

These benchmarks measure the core search functionality that AI agents use to find relevant code.

### 2. Chunker Performance

| Benchmark | Description | Target |
|-----------|-------------|--------|
| `BenchmarkChunker_Python` | Chunk a 500-line Python file | < 50ms |
| `BenchmarkChunker_PythonLarge` | Chunk a 2000-line Python file | < 200ms |
| `BenchmarkChunker_JavaScript` | Chunk a 500-line JavaScript file | < 50ms |
| `BenchmarkChunker_TypeScript` | Chunk a 500-line TypeScript file | < 50ms |

Chunking performance affects how quickly file changes are indexed.

### 3. Database Insert Performance

| Benchmark | Description | Target |
|-----------|-------------|--------|
| `BenchmarkDB_InsertChunk` | Insert a single chunk | < 10ms |
| `BenchmarkDB_InsertEmbedding` | Insert a single embedding | < 10ms |
| `BenchmarkDB_BulkInsert` | Insert 100 chunks with embeddings | < 500ms |

Database write performance determines indexing throughput.

### 4. Embedder Cache Performance

| Benchmark | Description | Target |
|-----------|-------------|--------|
| `BenchmarkEmbedder_Cache` | Cache hit lookup | < 1ms |
| `BenchmarkEmbedder_CacheMiss` | Cache miss with embedding generation | < 10ms |
| `BenchmarkEmbedder_BatchCache` | Batch embedding with partial cache hits | N/A |

The embedding cache reduces redundant model calls for unchanged code.

### 5. End-to-End Pipeline

| Benchmark | Description | Target |
|-----------|-------------|--------|
| `BenchmarkPipeline_ChunkAndEmbed` | Chunk + embed a 500-line file | < 100ms |
| `BenchmarkPipeline_FullIndex` | Complete indexing including DB writes | < 200ms |

These measure the complete indexing flow from file to searchable chunks.

## Performance Targets

The performance targets are designed to ensure Pommel remains responsive for AI coding agents:

| Operation | Target | Rationale |
|-----------|--------|-----------|
| **Search** | < 100ms | Quick enough to not interrupt agent workflow |
| **Chunking** | < 50ms per 500 lines | Fast enough for real-time indexing on save |
| **Single insert** | < 10ms | Minimal overhead per operation |
| **Bulk insert (100)** | < 500ms | Efficient batch processing |
| **Cache hit** | < 1ms | Near-instant for cached content |
| **Full index (1 file)** | < 200ms | Acceptable for file-at-a-time indexing |

## Baseline Results

Expected baseline performance on a modern laptop (M1/M2 Mac or recent Intel):

```
BenchmarkSearch_VectorSearch-8              1000       15.2 ms/op
BenchmarkSearch_VectorSearchWithFilter-8     500       25.8 ms/op
BenchmarkChunker_Python-8                   2000        8.5 ms/op
BenchmarkChunker_PythonLarge-8               500       32.1 ms/op
BenchmarkChunker_JavaScript-8               2000        9.2 ms/op
BenchmarkChunker_TypeScript-8               2000        9.8 ms/op
BenchmarkDB_InsertChunk-8                  10000        0.15 ms/op
BenchmarkDB_InsertEmbedding-8               5000        0.28 ms/op
BenchmarkDB_BulkInsert-8                     100       45.3 ms/op
BenchmarkEmbedder_Cache-8                1000000        0.0012 ms/op
BenchmarkEmbedder_CacheMiss-8              50000        0.025 ms/op
BenchmarkPipeline_ChunkAndEmbed-8           1000       12.5 ms/op
BenchmarkPipeline_FullIndex-8                200       58.7 ms/op
```

Note: These are example values. Actual results will vary based on hardware.

## Writing New Benchmarks

When adding new benchmarks:

1. Use the `//go:build benchmark` build tag
2. Use helper functions for common setup (see `setupTestDB`, `generateRandomEmbedding`, etc.)
3. Call `b.ResetTimer()` after setup to exclude setup time from measurements
4. Document the target performance in the function comment
5. Update this README with the new benchmark

Example:

```go
// BenchmarkNewOperation measures new operation performance.
// Target: < 50ms
func BenchmarkNewOperation(b *testing.B) {
    // Setup
    db := setupTestDB(b)

    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        // Operation to benchmark
    }
}
```

## Continuous Integration

To include benchmarks in CI pipelines, add a step that runs:

```bash
go test -tags=benchmark -bench=. -benchtime=1s ./internal/benchmarks/... | tee benchmark_results.txt
```

Consider failing the build if performance regresses significantly by comparing against baseline values.
