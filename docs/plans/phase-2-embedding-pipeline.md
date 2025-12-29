# Phase 2: Embedding Pipeline

**Phase Goal:** Integrate with Ollama to generate embeddings and store them in sqlite-vec, establishing the core vector search infrastructure.

**Prerequisites:** Phase 1 complete (database, config, CLI skeleton)

**Estimated Tasks:** 12 tasks across 4 areas

---

## Table of Contents

1. [Overview](#overview)
2. [Success Criteria](#success-criteria)
3. [Task 2.1: Ollama Client](#task-21-ollama-client)
4. [Task 2.2: Embedding Interface](#task-22-embedding-interface)
5. [Task 2.3: Query Cache](#task-23-query-cache)
6. [Task 2.4: Vector Storage](#task-24-vector-storage)
7. [Dependencies](#dependencies)
8. [Testing Strategy](#testing-strategy)
9. [Risks and Mitigations](#risks-and-mitigations)

---

## Overview

Phase 2 builds the embedding pipeline - the core capability that transforms text into vectors and stores them for similarity search. By the end of this phase:

- Pommel can connect to Ollama and check its health
- Text can be converted to 768-dimensional vectors
- Vectors can be stored in sqlite-vec
- Vectors can be retrieved via similarity search
- Query embeddings are cached to reduce latency

This phase does NOT include file watching or chunking - it focuses purely on the embedding and storage infrastructure.

---

## Success Criteria

| Criterion | Validation |
|-----------|------------|
| Ollama connection | Health check passes when Ollama is running |
| Embedding generation | Text converts to 768-dim float32 vector |
| Batch embedding | Multiple texts embed in single call |
| Vector insert | Embeddings store in sqlite-vec |
| Vector search | Similar texts return high similarity scores |
| Query cache | Second query for same text is faster |

---

## Task 2.1: Ollama Client

### 2.1.1 Create Ollama HTTP Client

**Description:** Implement an HTTP client that communicates with the Ollama API.

**Ollama API Details:**
- Default endpoint: `http://localhost:11434`
- Embeddings endpoint: `POST /api/embeddings`
- Health check: `GET /` (returns 200 if running)

**Steps:**
1. Create `internal/embedder/ollama.go`
2. Implement HTTP client with configurable base URL
3. Add timeout and retry logic

**File Content (internal/embedder/ollama.go):**
```go
package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultOllamaURL     = "http://localhost:11434"
	defaultTimeout       = 30 * time.Second
	embeddingsEndpoint   = "/api/embeddings"
)

// OllamaClient communicates with the Ollama API
type OllamaClient struct {
	baseURL    string
	httpClient *http.Client
	model      string
}

// OllamaConfig contains configuration for the Ollama client
type OllamaConfig struct {
	BaseURL string
	Model   string
	Timeout time.Duration
}

// embeddingRequest is the request body for /api/embeddings
type embeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// embeddingResponse is the response from /api/embeddings
type embeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(cfg OllamaConfig) *OllamaClient {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultOllamaURL
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return &OllamaClient{
		baseURL: baseURL,
		model:   cfg.Model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Health checks if Ollama is running and responsive
func (c *OllamaClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	return nil
}

// ModelLoaded checks if the specified model is available
func (c *OllamaClient) ModelLoaded(ctx context.Context) (bool, error) {
	// Try to generate a tiny embedding - if model isn't loaded, it will fail
	_, err := c.EmbedSingle(ctx, "test")
	if err != nil {
		return false, nil
	}
	return true, nil
}

// EmbedSingle generates an embedding for a single text
func (c *OllamaClient) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	reqBody := embeddingRequest{
		Model:  c.model,
		Prompt: text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + embeddingsEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Embedding, nil
}

// Embed generates embeddings for multiple texts
// Note: Ollama doesn't support batch embeddings natively, so we call sequentially
// Future optimization: parallelize with goroutines
func (c *OllamaClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))

	for i, text := range texts {
		embedding, err := c.EmbedSingle(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		results[i] = embedding
	}

	return results, nil
}

// ModelName returns the configured model name
func (c *OllamaClient) ModelName() string {
	return c.model
}

// Dimensions returns the embedding dimensions (768 for Jina v2)
func (c *OllamaClient) Dimensions() int {
	return 768
}

// BaseURL returns the configured base URL
func (c *OllamaClient) BaseURL() string {
	return c.baseURL
}
```

**Acceptance Criteria:**
- Client connects to Ollama successfully
- Health check returns nil when Ollama is running
- Health check returns error when Ollama is not running
- Model check correctly identifies if model is loaded

---

### 2.1.2 Add Parallel Batch Embedding

**Description:** Optimize batch embedding with concurrent requests.

**Steps:**
1. Add parallel embedding with configurable concurrency
2. Add rate limiting to avoid overwhelming Ollama

**File Content (internal/embedder/ollama_batch.go):**
```go
package embedder

import (
	"context"
	"sync"
)

// BatchConfig configures batch embedding behavior
type BatchConfig struct {
	Concurrency int // Max concurrent embedding requests
	BatchSize   int // Max texts per batch
}

// DefaultBatchConfig returns sensible defaults
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		Concurrency: 4,
		BatchSize:   32,
	}
}

// EmbedBatch generates embeddings with controlled concurrency
func (c *OllamaClient) EmbedBatch(ctx context.Context, texts []string, cfg BatchConfig) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	results := make([][]float32, len(texts))
	errors := make([]error, len(texts))

	// Semaphore for concurrency control
	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup

	for i, text := range texts {
		wg.Add(1)
		go func(idx int, t string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errors[idx] = ctx.Err()
				return
			}

			embedding, err := c.EmbedSingle(ctx, t)
			if err != nil {
				errors[idx] = err
				return
			}
			results[idx] = embedding
		}(i, text)
	}

	wg.Wait()

	// Check for errors
	for i, err := range errors {
		if err != nil {
			return nil, fmt.Errorf("embedding failed for text %d: %w", i, err)
		}
	}

	return results, nil
}

// EmbedBatched splits large inputs into batches and processes them
func (c *OllamaClient) EmbedBatched(ctx context.Context, texts []string, cfg BatchConfig) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	var allResults [][]float32

	for i := 0; i < len(texts); i += cfg.BatchSize {
		end := i + cfg.BatchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		results, err := c.EmbedBatch(ctx, batch, cfg)
		if err != nil {
			return nil, fmt.Errorf("batch %d failed: %w", i/cfg.BatchSize, err)
		}

		allResults = append(allResults, results...)
	}

	return allResults, nil
}
```

**Acceptance Criteria:**
- Batch embedding processes texts concurrently
- Concurrency is limited to configured value
- Large batches are split appropriately
- Context cancellation stops processing

---

### 2.1.3 Write Ollama Client Tests

**Description:** Test Ollama client with mock HTTP server.

**Steps:**
1. Create `internal/embedder/ollama_test.go`
2. Use httptest for mock server
3. Test success and error cases

**File Content (internal/embedder/ollama_test.go):**
```go
package embedder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaClient_Health(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
	})

	err := client.Health(context.Background())
	assert.NoError(t, err)
}

func TestOllamaClient_Health_NotRunning(t *testing.T) {
	client := NewOllamaClient(OllamaConfig{
		BaseURL: "http://localhost:99999", // Invalid port
		Model:   "test-model",
		Timeout: 100 * time.Millisecond,
	})

	err := client.Health(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not reachable")
}

func TestOllamaClient_EmbedSingle(t *testing.T) {
	expectedEmbedding := make([]float32, 768)
	for i := range expectedEmbedding {
		expectedEmbedding[i] = float32(i) * 0.001
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/embeddings", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req embeddingRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "test-model", req.Model)
		assert.Equal(t, "hello world", req.Prompt)

		json.NewEncoder(w).Encode(embeddingResponse{
			Embedding: expectedEmbedding,
		})
	}))
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
	})

	embedding, err := client.EmbedSingle(context.Background(), "hello world")
	require.NoError(t, err)
	assert.Len(t, embedding, 768)
	assert.Equal(t, expectedEmbedding, embedding)
}

func TestOllamaClient_EmbedSingle_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("model not found"))
	}))
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "nonexistent",
	})

	_, err := client.EmbedSingle(context.Background(), "hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestOllamaClient_Embed_Multiple(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		embedding := make([]float32, 768)
		json.NewEncoder(w).Encode(embeddingResponse{Embedding: embedding})
	}))
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
	})

	texts := []string{"one", "two", "three"}
	embeddings, err := client.Embed(context.Background(), texts)
	require.NoError(t, err)
	assert.Len(t, embeddings, 3)
	assert.Equal(t, 3, callCount)
}

func TestOllamaClient_EmbedBatch_Concurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)
		embedding := make([]float32, 768)
		json.NewEncoder(w).Encode(embeddingResponse{Embedding: embedding})
	}))
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{
		BaseURL: server.URL,
		Model:   "test-model",
	})

	texts := make([]string, 10)
	for i := range texts {
		texts[i] = fmt.Sprintf("text %d", i)
	}

	start := time.Now()
	embeddings, err := client.EmbedBatch(context.Background(), texts, BatchConfig{
		Concurrency: 4,
		BatchSize:   10,
	})
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Len(t, embeddings, 10)
	// With 4 concurrency, 10 requests at 10ms each should take ~30ms, not 100ms
	assert.Less(t, elapsed, 80*time.Millisecond)
}

func TestOllamaClient_Dimensions(t *testing.T) {
	client := NewOllamaClient(OllamaConfig{Model: "test"})
	assert.Equal(t, 768, client.Dimensions())
}

func TestOllamaClient_ModelName(t *testing.T) {
	client := NewOllamaClient(OllamaConfig{Model: "my-model"})
	assert.Equal(t, "my-model", client.ModelName())
}
```

**Acceptance Criteria:**
- All tests pass
- Mock server simulates Ollama responses
- Error cases are tested
- Concurrent embedding is verified

---

## Task 2.2: Embedding Interface

### 2.2.1 Define Embedder Interface

**Description:** Create an interface that abstracts embedding generation.

**Steps:**
1. Create `internal/embedder/embedder.go` with interface definition
2. Ensure OllamaClient implements the interface

**File Content (internal/embedder/embedder.go):**
```go
package embedder

import "context"

// Embedder generates vector embeddings from text
type Embedder interface {
	// EmbedSingle generates an embedding for a single text
	EmbedSingle(ctx context.Context, text string) ([]float32, error)

	// Embed generates embeddings for multiple texts
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// Health checks if the embedding service is available
	Health(ctx context.Context) error

	// ModelName returns the name of the embedding model
	ModelName() string

	// Dimensions returns the embedding vector dimensions
	Dimensions() int
}

// Ensure OllamaClient implements Embedder
var _ Embedder = (*OllamaClient)(nil)
```

**Acceptance Criteria:**
- Interface is defined with all required methods
- OllamaClient satisfies the interface
- Compile-time check ensures implementation

---

### 2.2.2 Create Mock Embedder for Testing

**Description:** Create a mock embedder for unit tests that don't need real Ollama.

**Steps:**
1. Create `internal/embedder/mock.go`

**File Content (internal/embedder/mock.go):**
```go
package embedder

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// MockEmbedder generates deterministic embeddings for testing
type MockEmbedder struct {
	dimensions int
	healthy    bool
	modelName  string
}

// NewMockEmbedder creates a new mock embedder
func NewMockEmbedder() *MockEmbedder {
	return &MockEmbedder{
		dimensions: 768,
		healthy:    true,
		modelName:  "mock-embedder",
	}
}

// SetHealthy controls whether Health() returns success
func (m *MockEmbedder) SetHealthy(healthy bool) {
	m.healthy = healthy
}

// EmbedSingle generates a deterministic embedding based on input text
func (m *MockEmbedder) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	return m.generateDeterministic(text), nil
}

// Embed generates embeddings for multiple texts
func (m *MockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		results[i] = m.generateDeterministic(text)
	}
	return results, nil
}

// Health returns nil if healthy, error otherwise
func (m *MockEmbedder) Health(ctx context.Context) error {
	if !m.healthy {
		return fmt.Errorf("mock embedder is unhealthy")
	}
	return nil
}

// ModelName returns the mock model name
func (m *MockEmbedder) ModelName() string {
	return m.modelName
}

// Dimensions returns the embedding dimensions
func (m *MockEmbedder) Dimensions() int {
	return m.dimensions
}

// generateDeterministic creates a reproducible embedding from text
// Similar texts will have similar embeddings (first few dimensions based on length)
func (m *MockEmbedder) generateDeterministic(text string) []float32 {
	embedding := make([]float32, m.dimensions)

	// Use SHA256 to generate pseudo-random but deterministic values
	hash := sha256.Sum256([]byte(text))

	for i := 0; i < m.dimensions; i++ {
		// Use hash bytes to generate float values
		idx := i % 32
		val := float64(hash[idx]) / 255.0

		// Add some variation based on position
		offset := float64(i) / float64(m.dimensions)
		embedding[i] = float32(val*0.5 + offset*0.5)
	}

	// Normalize the vector
	var norm float64
	for _, v := range embedding {
		norm += float64(v * v)
	}
	norm = math.Sqrt(norm)

	for i := range embedding {
		embedding[i] = float32(float64(embedding[i]) / norm)
	}

	return embedding
}

// Ensure MockEmbedder implements Embedder
var _ Embedder = (*MockEmbedder)(nil)
```

**Acceptance Criteria:**
- Mock embedder implements Embedder interface
- Same text always produces same embedding
- Health can be toggled for testing error cases

---

## Task 2.3: Query Cache

### 2.3.1 Implement LRU Cache

**Description:** Create an LRU cache for query embeddings to avoid redundant Ollama calls.

**Steps:**
1. Create `internal/embedder/cache.go`
2. Implement thread-safe LRU cache
3. Add cache hit/miss metrics

**File Content (internal/embedder/cache.go):**
```go
package embedder

import (
	"container/list"
	"context"
	"sync"
)

// CachedEmbedder wraps an Embedder with an LRU cache
type CachedEmbedder struct {
	embedder Embedder
	cache    *lruCache
	metrics  CacheMetrics
	mu       sync.RWMutex
}

// CacheMetrics tracks cache performance
type CacheMetrics struct {
	Hits   int64
	Misses int64
}

// lruCache is a simple LRU cache for embeddings
type lruCache struct {
	capacity int
	items    map[string]*list.Element
	order    *list.List
	mu       sync.Mutex
}

type cacheEntry struct {
	key       string
	embedding []float32
}

// NewCachedEmbedder creates an embedder with caching
func NewCachedEmbedder(embedder Embedder, capacity int) *CachedEmbedder {
	return &CachedEmbedder{
		embedder: embedder,
		cache:    newLRUCache(capacity),
	}
}

func newLRUCache(capacity int) *lruCache {
	return &lruCache{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

// Get retrieves an embedding from cache
func (c *lruCache) Get(key string) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		return elem.Value.(*cacheEntry).embedding, true
	}
	return nil, false
}

// Put adds an embedding to cache
func (c *lruCache) Put(key string, embedding []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If key exists, update and move to front
	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		elem.Value.(*cacheEntry).embedding = embedding
		return
	}

	// Evict if at capacity
	if c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.items, oldest.Value.(*cacheEntry).key)
		}
	}

	// Add new entry
	entry := &cacheEntry{key: key, embedding: embedding}
	elem := c.order.PushFront(entry)
	c.items[key] = elem
}

// Size returns current cache size
func (c *lruCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Clear empties the cache
func (c *lruCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.order.Init()
}

// EmbedSingle checks cache first, then calls underlying embedder
func (c *CachedEmbedder) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	// Check cache
	if embedding, ok := c.cache.Get(text); ok {
		c.mu.Lock()
		c.metrics.Hits++
		c.mu.Unlock()
		return embedding, nil
	}

	// Cache miss - get from embedder
	c.mu.Lock()
	c.metrics.Misses++
	c.mu.Unlock()

	embedding, err := c.embedder.EmbedSingle(ctx, text)
	if err != nil {
		return nil, err
	}

	// Store in cache
	c.cache.Put(text, embedding)
	return embedding, nil
}

// Embed generates embeddings, using cache where possible
func (c *CachedEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	var uncached []int // Indices of texts not in cache

	// Check cache for each text
	for i, text := range texts {
		if embedding, ok := c.cache.Get(text); ok {
			results[i] = embedding
			c.mu.Lock()
			c.metrics.Hits++
			c.mu.Unlock()
		} else {
			uncached = append(uncached, i)
			c.mu.Lock()
			c.metrics.Misses++
			c.mu.Unlock()
		}
	}

	// If all were cached, return early
	if len(uncached) == 0 {
		return results, nil
	}

	// Get embeddings for uncached texts
	uncachedTexts := make([]string, len(uncached))
	for i, idx := range uncached {
		uncachedTexts[i] = texts[idx]
	}

	embeddings, err := c.embedder.Embed(ctx, uncachedTexts)
	if err != nil {
		return nil, err
	}

	// Store results and update cache
	for i, idx := range uncached {
		results[idx] = embeddings[i]
		c.cache.Put(texts[idx], embeddings[i])
	}

	return results, nil
}

// Health delegates to underlying embedder
func (c *CachedEmbedder) Health(ctx context.Context) error {
	return c.embedder.Health(ctx)
}

// ModelName delegates to underlying embedder
func (c *CachedEmbedder) ModelName() string {
	return c.embedder.ModelName()
}

// Dimensions delegates to underlying embedder
func (c *CachedEmbedder) Dimensions() int {
	return c.embedder.Dimensions()
}

// Metrics returns cache performance metrics
func (c *CachedEmbedder) Metrics() CacheMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metrics
}

// CacheSize returns current number of cached embeddings
func (c *CachedEmbedder) CacheSize() int {
	return c.cache.Size()
}

// ClearCache empties the cache
func (c *CachedEmbedder) ClearCache() {
	c.cache.Clear()
}

// Ensure CachedEmbedder implements Embedder
var _ Embedder = (*CachedEmbedder)(nil)
```

**Acceptance Criteria:**
- Cache returns same embedding for repeated queries
- LRU eviction works when capacity is reached
- Metrics track hits and misses
- Thread-safe for concurrent access

---

### 2.3.2 Write Cache Tests

**Description:** Test cache behavior thoroughly.

**Steps:**
1. Create `internal/embedder/cache_test.go`

**File Content (internal/embedder/cache_test.go):**
```go
package embedder

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedEmbedder_CacheHit(t *testing.T) {
	mock := NewMockEmbedder()
	cached := NewCachedEmbedder(mock, 100)

	ctx := context.Background()

	// First call - cache miss
	emb1, err := cached.EmbedSingle(ctx, "hello")
	require.NoError(t, err)

	// Second call - cache hit
	emb2, err := cached.EmbedSingle(ctx, "hello")
	require.NoError(t, err)

	assert.Equal(t, emb1, emb2)

	metrics := cached.Metrics()
	assert.Equal(t, int64(1), metrics.Hits)
	assert.Equal(t, int64(1), metrics.Misses)
}

func TestCachedEmbedder_LRUEviction(t *testing.T) {
	mock := NewMockEmbedder()
	cached := NewCachedEmbedder(mock, 3) // Tiny cache

	ctx := context.Background()

	// Fill cache
	cached.EmbedSingle(ctx, "one")
	cached.EmbedSingle(ctx, "two")
	cached.EmbedSingle(ctx, "three")

	assert.Equal(t, 3, cached.CacheSize())

	// Add one more - should evict "one"
	cached.EmbedSingle(ctx, "four")

	assert.Equal(t, 3, cached.CacheSize())

	// "one" should be a miss now
	cached.EmbedSingle(ctx, "one")
	metrics := cached.Metrics()
	assert.Equal(t, int64(0), metrics.Hits) // No hits yet
	assert.Equal(t, int64(5), metrics.Misses) // All misses
}

func TestCachedEmbedder_LRUAccess(t *testing.T) {
	mock := NewMockEmbedder()
	cached := NewCachedEmbedder(mock, 3)

	ctx := context.Background()

	// Fill cache
	cached.EmbedSingle(ctx, "one")
	cached.EmbedSingle(ctx, "two")
	cached.EmbedSingle(ctx, "three")

	// Access "one" to make it recently used
	cached.EmbedSingle(ctx, "one")

	// Add new item - should evict "two" (oldest)
	cached.EmbedSingle(ctx, "four")

	// "one" should still be cached
	cached.EmbedSingle(ctx, "one")

	metrics := cached.Metrics()
	assert.Equal(t, int64(2), metrics.Hits) // "one" hit twice
}

func TestCachedEmbedder_EmbedMultiple(t *testing.T) {
	mock := NewMockEmbedder()
	cached := NewCachedEmbedder(mock, 100)

	ctx := context.Background()

	// Pre-cache one
	cached.EmbedSingle(ctx, "one")

	// Request batch including cached and uncached
	texts := []string{"one", "two", "three"}
	embeddings, err := cached.Embed(ctx, texts)
	require.NoError(t, err)
	assert.Len(t, embeddings, 3)

	metrics := cached.Metrics()
	assert.Equal(t, int64(1), metrics.Hits)   // "one" was cached
	assert.Equal(t, int64(3), metrics.Misses) // Initial + "two" + "three"
}

func TestCachedEmbedder_Concurrent(t *testing.T) {
	mock := NewMockEmbedder()
	cached := NewCachedEmbedder(mock, 100)

	ctx := context.Background()
	var wg sync.WaitGroup

	// Concurrent access
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			text := fmt.Sprintf("text-%d", n%10) // 10 unique texts
			_, err := cached.EmbedSingle(ctx, text)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Should have cached 10 unique texts
	assert.Equal(t, 10, cached.CacheSize())
}

func TestCachedEmbedder_Clear(t *testing.T) {
	mock := NewMockEmbedder()
	cached := NewCachedEmbedder(mock, 100)

	ctx := context.Background()
	cached.EmbedSingle(ctx, "one")
	cached.EmbedSingle(ctx, "two")

	assert.Equal(t, 2, cached.CacheSize())

	cached.ClearCache()

	assert.Equal(t, 0, cached.CacheSize())
}

func TestLRUCache_Capacity(t *testing.T) {
	cache := newLRUCache(2)

	cache.Put("a", []float32{1.0})
	cache.Put("b", []float32{2.0})
	cache.Put("c", []float32{3.0}) // Should evict "a"

	_, ok := cache.Get("a")
	assert.False(t, ok)

	_, ok = cache.Get("b")
	assert.True(t, ok)

	_, ok = cache.Get("c")
	assert.True(t, ok)
}
```

**Acceptance Criteria:**
- All tests pass
- LRU eviction is verified
- Concurrent access is safe
- Batch embedding uses cache

---

## Task 2.4: Vector Storage

### 2.4.1 Implement Vector Insert

**Description:** Store embeddings in sqlite-vec.

**Steps:**
1. Create `internal/db/vectors.go`
2. Implement insert with proper serialization

**File Content (internal/db/vectors.go):**
```go
package db

import (
	"context"
	"fmt"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/ncruces"
)

// InsertEmbedding stores an embedding for a chunk
func (db *DB) InsertEmbedding(ctx context.Context, chunkID string, embedding []float32) error {
	serialized, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return fmt.Errorf("failed to serialize embedding: %w", err)
	}

	_, err = db.Exec(ctx,
		"INSERT OR REPLACE INTO chunk_embeddings (chunk_id, embedding) VALUES (?, ?)",
		chunkID, serialized)
	if err != nil {
		return fmt.Errorf("failed to insert embedding: %w", err)
	}

	return nil
}

// InsertEmbeddings stores multiple embeddings in a transaction
func (db *DB) InsertEmbeddings(ctx context.Context, chunkIDs []string, embeddings [][]float32) error {
	if len(chunkIDs) != len(embeddings) {
		return fmt.Errorf("chunk IDs and embeddings count mismatch: %d vs %d", len(chunkIDs), len(embeddings))
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		"INSERT OR REPLACE INTO chunk_embeddings (chunk_id, embedding) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for i, chunkID := range chunkIDs {
		serialized, err := sqlite_vec.SerializeFloat32(embeddings[i])
		if err != nil {
			return fmt.Errorf("failed to serialize embedding %d: %w", i, err)
		}

		_, err = stmt.ExecContext(ctx, chunkID, serialized)
		if err != nil {
			return fmt.Errorf("failed to insert embedding %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteEmbedding removes an embedding by chunk ID
func (db *DB) DeleteEmbedding(ctx context.Context, chunkID string) error {
	_, err := db.Exec(ctx, "DELETE FROM chunk_embeddings WHERE chunk_id = ?", chunkID)
	return err
}

// DeleteEmbeddingsByChunkIDs removes embeddings for multiple chunks
func (db *DB) DeleteEmbeddingsByChunkIDs(ctx context.Context, chunkIDs []string) error {
	if len(chunkIDs) == 0 {
		return nil
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "DELETE FROM chunk_embeddings WHERE chunk_id = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, chunkID := range chunkIDs {
		if _, err := stmt.ExecContext(ctx, chunkID); err != nil {
			return fmt.Errorf("failed to delete embedding %s: %w", chunkID, err)
		}
	}

	return tx.Commit()
}

// EmbeddingCount returns the number of stored embeddings
func (db *DB) EmbeddingCount(ctx context.Context) (int, error) {
	var count int
	err := db.QueryRow(ctx, "SELECT COUNT(*) FROM chunk_embeddings").Scan(&count)
	return count, err
}
```

**Acceptance Criteria:**
- Single embedding inserts successfully
- Batch insert uses transaction
- Delete removes embeddings
- Count returns correct number

---

### 2.4.2 Implement Vector Search

**Description:** Query sqlite-vec for similar vectors.

**Steps:**
1. Add search methods to `internal/db/vectors.go`

**Additional Content for internal/db/vectors.go:**
```go
// VectorSearchResult represents a search result
type VectorSearchResult struct {
	ChunkID  string
	Distance float32
}

// SearchSimilar finds chunks with embeddings similar to the query
func (db *DB) SearchSimilar(ctx context.Context, queryEmbedding []float32, limit int) ([]VectorSearchResult, error) {
	serialized, err := sqlite_vec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize query embedding: %w", err)
	}

	rows, err := db.Query(ctx, `
		SELECT chunk_id, distance
		FROM chunk_embeddings
		WHERE embedding MATCH ?
		ORDER BY distance
		LIMIT ?
	`, serialized, limit)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var results []VectorSearchResult
	for rows.Next() {
		var r VectorSearchResult
		if err := rows.Scan(&r.ChunkID, &r.Distance); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return results, nil
}

// SearchSimilarFiltered finds similar chunks with additional filters
func (db *DB) SearchSimilarFiltered(ctx context.Context, queryEmbedding []float32, limit int, chunkIDs []string) ([]VectorSearchResult, error) {
	if len(chunkIDs) == 0 {
		return db.SearchSimilar(ctx, queryEmbedding, limit)
	}

	serialized, err := sqlite_vec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize query embedding: %w", err)
	}

	// Build IN clause for filtering
	placeholders := make([]string, len(chunkIDs))
	args := make([]any, len(chunkIDs)+2)
	args[0] = serialized
	for i, id := range chunkIDs {
		placeholders[i] = "?"
		args[i+1] = id
	}
	args[len(args)-1] = limit

	query := fmt.Sprintf(`
		SELECT chunk_id, distance
		FROM chunk_embeddings
		WHERE embedding MATCH ?
		AND chunk_id IN (%s)
		ORDER BY distance
		LIMIT ?
	`, strings.Join(placeholders, ","))

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("filtered search query failed: %w", err)
	}
	defer rows.Close()

	var results []VectorSearchResult
	for rows.Next() {
		var r VectorSearchResult
		if err := rows.Scan(&r.ChunkID, &r.Distance); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, r)
	}

	return results, nil
}
```

**Acceptance Criteria:**
- Search returns results ordered by distance
- Limit is respected
- Filtered search works with chunk ID list
- Empty results return empty slice (not nil)

---

### 2.4.3 Write Vector Storage Tests

**Description:** Test vector insert and search operations.

**Steps:**
1. Create `internal/db/vectors_test.go`

**File Content (internal/db/vectors_test.go):**
```go
package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *DB {
	tmpDir := t.TempDir()
	db, err := Open(tmpDir)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background()))
	t.Cleanup(func() { db.Close() })
	return db
}

func TestInsertEmbedding(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = float32(i) * 0.001
	}

	err := db.InsertEmbedding(ctx, "chunk-1", embedding)
	require.NoError(t, err)

	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestInsertEmbeddings_Batch(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	ids := []string{"chunk-1", "chunk-2", "chunk-3"}
	embeddings := make([][]float32, 3)
	for i := range embeddings {
		embeddings[i] = make([]float32, 768)
		for j := range embeddings[i] {
			embeddings[i][j] = float32(i*768+j) * 0.0001
		}
	}

	err := db.InsertEmbeddings(ctx, ids, embeddings)
	require.NoError(t, err)

	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestInsertEmbedding_Replace(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	emb1 := make([]float32, 768)
	emb2 := make([]float32, 768)
	emb2[0] = 1.0

	err := db.InsertEmbedding(ctx, "chunk-1", emb1)
	require.NoError(t, err)

	err = db.InsertEmbedding(ctx, "chunk-1", emb2)
	require.NoError(t, err)

	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count) // Still just one
}

func TestDeleteEmbedding(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	embedding := make([]float32, 768)
	err := db.InsertEmbedding(ctx, "chunk-1", embedding)
	require.NoError(t, err)

	err = db.DeleteEmbedding(ctx, "chunk-1")
	require.NoError(t, err)

	count, err := db.EmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSearchSimilar(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert some embeddings
	base := make([]float32, 768)
	for i := range base {
		base[i] = 0.5
	}

	similar := make([]float32, 768)
	for i := range similar {
		similar[i] = 0.51 // Very similar
	}

	different := make([]float32, 768)
	for i := range different {
		different[i] = -0.5 // Very different
	}

	err := db.InsertEmbedding(ctx, "similar", similar)
	require.NoError(t, err)
	err = db.InsertEmbedding(ctx, "different", different)
	require.NoError(t, err)

	// Search with base embedding
	results, err := db.SearchSimilar(ctx, base, 10)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Similar should be first (smaller distance)
	assert.Equal(t, "similar", results[0].ChunkID)
	assert.Equal(t, "different", results[1].ChunkID)
	assert.Less(t, results[0].Distance, results[1].Distance)
}

func TestSearchSimilar_Limit(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert 10 embeddings
	for i := 0; i < 10; i++ {
		emb := make([]float32, 768)
		emb[0] = float32(i)
		err := db.InsertEmbedding(ctx, fmt.Sprintf("chunk-%d", i), emb)
		require.NoError(t, err)
	}

	query := make([]float32, 768)
	results, err := db.SearchSimilar(ctx, query, 5)
	require.NoError(t, err)
	assert.Len(t, results, 5)
}

func TestSearchSimilar_Empty(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	query := make([]float32, 768)
	results, err := db.SearchSimilar(ctx, query, 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}
```

**Acceptance Criteria:**
- All tests pass
- Search returns similar vectors first
- Batch operations work correctly
- Edge cases handled (empty, replace)

---

## Dependencies

### Go Modules Required (additional to Phase 1)

```
# No new dependencies - all covered in Phase 1
# Ollama client uses standard library net/http
# sqlite-vec bindings already included
```

---

## Testing Strategy

### Unit Tests

- Mock HTTP server for Ollama tests
- Mock embedder for cache tests
- Real sqlite-vec for vector storage tests

### Integration Tests

- Test with real Ollama (if available)
- End-to-end: text → embedding → store → search → retrieve

### Manual Testing

1. Start Ollama: `ollama serve`
2. Pull model: `ollama pull unclemusclez/jina-embeddings-v2-base-code`
3. Run integration tests against real Ollama

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Ollama API changes | Low | Medium | Pin to known Ollama version, abstract behind interface |
| Embedding dimensions mismatch | Low | High | Validate dimensions on storage, fail fast |
| sqlite-vec serialization issues | Low | Medium | Test with various embedding values |
| Cache memory usage | Medium | Low | Limit cache size, monitor in production |

---

## Checklist

Before marking Phase 2 complete:

- [ ] Ollama client connects and checks health
- [ ] Single text embedding works
- [ ] Batch embedding with concurrency works
- [ ] Query cache reduces embedding calls
- [ ] Embeddings store in sqlite-vec
- [ ] Vector search returns ordered results
- [ ] All tests pass
- [ ] Code is documented
