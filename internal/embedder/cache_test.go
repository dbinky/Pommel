package embedder

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test Helpers
// ============================================================================

// TrackingMockEmbedder wraps MockEmbedder to track call counts for testing cache behavior.
type TrackingMockEmbedder struct {
	*MockEmbedder
	embedSingleCalls atomic.Int64
	embedCalls       atomic.Int64
	lastEmbedTexts   []string
	mu               sync.Mutex
}

func NewTrackingMockEmbedder() *TrackingMockEmbedder {
	return &TrackingMockEmbedder{
		MockEmbedder: NewMockEmbedder(),
	}
}

func (t *TrackingMockEmbedder) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	t.embedSingleCalls.Add(1)
	return t.MockEmbedder.EmbedSingle(ctx, text)
}

func (t *TrackingMockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	t.embedCalls.Add(1)
	t.mu.Lock()
	t.lastEmbedTexts = make([]string, len(texts))
	copy(t.lastEmbedTexts, texts)
	t.mu.Unlock()
	return t.MockEmbedder.Embed(ctx, texts)
}

func (t *TrackingMockEmbedder) EmbedSingleCallCount() int64 {
	return t.embedSingleCalls.Load()
}

func (t *TrackingMockEmbedder) EmbedCallCount() int64 {
	return t.embedCalls.Load()
}

func (t *TrackingMockEmbedder) LastEmbedTexts() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]string, len(t.lastEmbedTexts))
	copy(result, t.lastEmbedTexts)
	return result
}

// ============================================================================
// Interface Compliance
// ============================================================================

// TestCachedEmbedder_ImplementsEmbedder verifies that CachedEmbedder implements
// the Embedder interface at compile time.
func TestCachedEmbedder_ImplementsEmbedder(t *testing.T) {
	// This test verifies at compile time that CachedEmbedder implements Embedder.
	// The var _ Embedder = (*CachedEmbedder)(nil) in cache.go does the actual check,
	// but we include a runtime assertion for clarity.
	var embedder Embedder = &CachedEmbedder{}
	assert.NotNil(t, embedder, "CachedEmbedder should implement Embedder interface")
}

// ============================================================================
// Happy Path / Success Cases
// ============================================================================

// TestCachedEmbedder_CacheMiss verifies that the first call for a text goes
// to the underlying embedder (cache miss).
func TestCachedEmbedder_CacheMiss(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	cached := NewCachedEmbedder(underlying, 100)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	ctx := context.Background()
	text := "func hello() { return 42 }"

	embedding, err := cached.EmbedSingle(ctx, text)

	require.NoError(t, err, "EmbedSingle should not return error")
	require.NotNil(t, embedding, "Embedding should not be nil")
	assert.Len(t, embedding, 768, "Embedding should have 768 dimensions")
	assert.Equal(t, int64(1), underlying.EmbedSingleCallCount(),
		"Underlying embedder should be called once on cache miss")
}

// TestCachedEmbedder_CacheHit verifies that the second call for the same text
// returns cached embedding without calling the underlying embedder.
func TestCachedEmbedder_CacheHit(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	cached := NewCachedEmbedder(underlying, 100)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	ctx := context.Background()
	text := "func hello() { return 42 }"

	// First call - cache miss
	embedding1, err1 := cached.EmbedSingle(ctx, text)
	require.NoError(t, err1, "First EmbedSingle should not return error")
	require.NotNil(t, embedding1, "First embedding should not be nil")

	// Second call - should be cache hit
	embedding2, err2 := cached.EmbedSingle(ctx, text)
	require.NoError(t, err2, "Second EmbedSingle should not return error")
	require.NotNil(t, embedding2, "Second embedding should not be nil")

	// Verify embeddings are identical
	assert.Equal(t, embedding1, embedding2,
		"Cached embedding should be identical to original")

	// Underlying embedder should only be called once
	assert.Equal(t, int64(1), underlying.EmbedSingleCallCount(),
		"Underlying embedder should be called only once (second call was cache hit)")
}

// TestCachedEmbedder_EmbedMultiple_AllCached verifies that when all texts are
// already cached, the underlying embedder is not called.
func TestCachedEmbedder_EmbedMultiple_AllCached(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	cached := NewCachedEmbedder(underlying, 100)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	ctx := context.Background()
	texts := []string{
		"func test1() {}",
		"func test2() {}",
		"func test3() {}",
	}

	// First, cache all texts individually
	for _, text := range texts {
		_, err := cached.EmbedSingle(ctx, text)
		require.NoError(t, err, "EmbedSingle should not return error")
	}

	initialCalls := underlying.EmbedSingleCallCount()

	// Now call Embed with all texts - should all be cached
	embeddings, err := cached.Embed(ctx, texts)

	require.NoError(t, err, "Embed should not return error")
	require.Len(t, embeddings, 3, "Should return 3 embeddings")

	// No additional calls to underlying embedder
	assert.Equal(t, int64(0), underlying.EmbedCallCount(),
		"Underlying Embed should not be called when all texts are cached")
	assert.Equal(t, initialCalls, underlying.EmbedSingleCallCount(),
		"Underlying EmbedSingle should not be called when all texts are cached")
}

// TestCachedEmbedder_EmbedMultiple_PartialCache verifies that when some texts are
// cached and some are not, only the uncached texts go to the underlying embedder.
func TestCachedEmbedder_EmbedMultiple_PartialCache(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	cached := NewCachedEmbedder(underlying, 100)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	ctx := context.Background()

	// Pre-cache some texts
	cachedTexts := []string{"func cached1() {}", "func cached2() {}"}
	for _, text := range cachedTexts {
		_, err := cached.EmbedSingle(ctx, text)
		require.NoError(t, err, "EmbedSingle should not return error")
	}

	// Request mix of cached and uncached
	allTexts := []string{
		"func cached1() {}",   // cached
		"func uncached1() {}", // not cached
		"func cached2() {}",   // cached
		"func uncached2() {}", // not cached
	}

	embeddings, err := cached.Embed(ctx, allTexts)

	require.NoError(t, err, "Embed should not return error")
	require.Len(t, embeddings, 4, "Should return 4 embeddings")

	// Only the uncached texts should have been sent to underlying embedder
	lastTexts := underlying.LastEmbedTexts()
	assert.Len(t, lastTexts, 2,
		"Only 2 uncached texts should be sent to underlying embedder")
	assert.Contains(t, lastTexts, "func uncached1() {}",
		"Uncached text 1 should be sent to embedder")
	assert.Contains(t, lastTexts, "func uncached2() {}",
		"Uncached text 2 should be sent to embedder")
}

// TestCachedEmbedder_LRUEviction verifies that when the cache reaches capacity,
// the least recently used entries are evicted.
func TestCachedEmbedder_LRUEviction(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	capacity := 3
	cached := NewCachedEmbedder(underlying, capacity)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	ctx := context.Background()

	// Fill the cache to capacity
	texts := []string{"text1", "text2", "text3"}
	for _, text := range texts {
		_, err := cached.EmbedSingle(ctx, text)
		require.NoError(t, err, "EmbedSingle should not return error")
	}

	assert.Equal(t, capacity, cached.CacheSize(),
		"Cache should be at capacity")

	// Add one more - should evict "text1" (oldest)
	_, err := cached.EmbedSingle(ctx, "text4")
	require.NoError(t, err, "EmbedSingle should not return error")

	assert.Equal(t, capacity, cached.CacheSize(),
		"Cache should still be at capacity after eviction")

	// Reset call count
	callsBefore := underlying.EmbedSingleCallCount()

	// "text1" should be evicted, so accessing it should cause a cache miss
	_, err = cached.EmbedSingle(ctx, "text1")
	require.NoError(t, err, "EmbedSingle should not return error")

	assert.Equal(t, callsBefore+1, underlying.EmbedSingleCallCount(),
		"Accessing evicted entry should cause cache miss")
}

// TestCachedEmbedder_LRUAccess verifies that accessing an entry moves it to
// the front of the LRU list, preventing its eviction.
func TestCachedEmbedder_LRUAccess(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	capacity := 3
	cached := NewCachedEmbedder(underlying, capacity)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	ctx := context.Background()

	// Fill the cache: text1, text2, text3 (text1 is oldest)
	for _, text := range []string{"text1", "text2", "text3"} {
		_, err := cached.EmbedSingle(ctx, text)
		require.NoError(t, err, "EmbedSingle should not return error")
	}

	// Access text1 - moves it to front (most recently used)
	_, err := cached.EmbedSingle(ctx, "text1")
	require.NoError(t, err, "EmbedSingle should not return error")

	// Now text2 is the oldest. Add text4 - should evict text2 (not text1)
	_, err = cached.EmbedSingle(ctx, "text4")
	require.NoError(t, err, "EmbedSingle should not return error")

	callsBefore := underlying.EmbedSingleCallCount()

	// text1 should still be cached (we accessed it, moving it to front)
	_, err = cached.EmbedSingle(ctx, "text1")
	require.NoError(t, err, "EmbedSingle should not return error")
	assert.Equal(t, callsBefore, underlying.EmbedSingleCallCount(),
		"text1 should still be cached after LRU access")

	// text2 should be evicted
	_, err = cached.EmbedSingle(ctx, "text2")
	require.NoError(t, err, "EmbedSingle should not return error")
	assert.Equal(t, callsBefore+1, underlying.EmbedSingleCallCount(),
		"text2 should have been evicted (cache miss)")
}

// TestCachedEmbedder_Metrics_HitsAndMisses verifies that cache hit and miss
// counts are tracked correctly.
func TestCachedEmbedder_Metrics_HitsAndMisses(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	cached := NewCachedEmbedder(underlying, 100)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	ctx := context.Background()

	// Initial metrics should be zero
	metrics := cached.Metrics()
	assert.Equal(t, int64(0), metrics.Hits, "Initial hits should be 0")
	assert.Equal(t, int64(0), metrics.Misses, "Initial misses should be 0")

	// First call - cache miss
	_, _ = cached.EmbedSingle(ctx, "text1")
	metrics = cached.Metrics()
	assert.Equal(t, int64(0), metrics.Hits, "Hits should still be 0")
	assert.Equal(t, int64(1), metrics.Misses, "Misses should be 1")

	// Second call same text - cache hit
	_, _ = cached.EmbedSingle(ctx, "text1")
	metrics = cached.Metrics()
	assert.Equal(t, int64(1), metrics.Hits, "Hits should be 1")
	assert.Equal(t, int64(1), metrics.Misses, "Misses should still be 1")

	// Third call different text - cache miss
	_, _ = cached.EmbedSingle(ctx, "text2")
	metrics = cached.Metrics()
	assert.Equal(t, int64(1), metrics.Hits, "Hits should still be 1")
	assert.Equal(t, int64(2), metrics.Misses, "Misses should be 2")

	// Fourth call same as first - cache hit
	_, _ = cached.EmbedSingle(ctx, "text1")
	metrics = cached.Metrics()
	assert.Equal(t, int64(2), metrics.Hits, "Hits should be 2")
	assert.Equal(t, int64(2), metrics.Misses, "Misses should still be 2")
}

// TestCachedEmbedder_CacheSize verifies that CacheSize returns the correct
// current number of entries in the cache.
func TestCachedEmbedder_CacheSize(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	cached := NewCachedEmbedder(underlying, 100)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	ctx := context.Background()

	// Initially empty
	assert.Equal(t, 0, cached.CacheSize(), "Initial cache size should be 0")

	// Add one entry
	_, _ = cached.EmbedSingle(ctx, "text1")
	assert.Equal(t, 1, cached.CacheSize(), "Cache size should be 1")

	// Add another entry
	_, _ = cached.EmbedSingle(ctx, "text2")
	assert.Equal(t, 2, cached.CacheSize(), "Cache size should be 2")

	// Accessing existing entry should not change size
	_, _ = cached.EmbedSingle(ctx, "text1")
	assert.Equal(t, 2, cached.CacheSize(), "Cache size should still be 2 after cache hit")
}

// TestCachedEmbedder_ClearCache verifies that ClearCache removes all entries
// from the cache.
func TestCachedEmbedder_ClearCache(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	cached := NewCachedEmbedder(underlying, 100)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	ctx := context.Background()

	// Add some entries
	for _, text := range []string{"text1", "text2", "text3"} {
		_, err := cached.EmbedSingle(ctx, text)
		require.NoError(t, err, "EmbedSingle should not return error")
	}
	assert.Equal(t, 3, cached.CacheSize(), "Cache should have 3 entries")

	// Clear the cache
	cached.ClearCache()

	assert.Equal(t, 0, cached.CacheSize(), "Cache should be empty after clear")

	// All entries should now be cache misses
	callsBefore := underlying.EmbedSingleCallCount()
	_, _ = cached.EmbedSingle(ctx, "text1")
	assert.Equal(t, callsBefore+1, underlying.EmbedSingleCallCount(),
		"Previously cached entry should now be a cache miss")
}

// ============================================================================
// Delegation Cases
// ============================================================================

// TestCachedEmbedder_Health_Delegates verifies that Health() delegates to
// the underlying embedder.
func TestCachedEmbedder_Health_Delegates(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	cached := NewCachedEmbedder(underlying, 100)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	ctx := context.Background()

	// When underlying is healthy
	underlying.SetHealthy(true)
	err := cached.Health(ctx)
	assert.NoError(t, err, "Health should return nil when underlying is healthy")

	// When underlying is unhealthy
	underlying.SetHealthy(false)
	err = cached.Health(ctx)
	assert.Error(t, err, "Health should return error when underlying is unhealthy")
	assert.Contains(t, err.Error(), "unhealthy",
		"Error should contain 'unhealthy'")
}

// TestCachedEmbedder_ModelName_Delegates verifies that ModelName() delegates
// to the underlying embedder.
func TestCachedEmbedder_ModelName_Delegates(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	cached := NewCachedEmbedder(underlying, 100)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	modelName := cached.ModelName()
	assert.Equal(t, "mock-embedder", modelName,
		"ModelName should delegate to underlying embedder")
}

// TestCachedEmbedder_Dimensions_Delegates verifies that Dimensions() delegates
// to the underlying embedder.
func TestCachedEmbedder_Dimensions_Delegates(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	cached := NewCachedEmbedder(underlying, 100)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	dims := cached.Dimensions()
	assert.Equal(t, 768, dims,
		"Dimensions should delegate to underlying embedder")
}

// ============================================================================
// Concurrency Cases
// ============================================================================

// TestCachedEmbedder_Concurrent verifies that multiple goroutines can safely
// access the cached embedder concurrently.
func TestCachedEmbedder_Concurrent(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	cached := NewCachedEmbedder(underlying, 100)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	ctx := context.Background()
	numGoroutines := 50
	numIterations := 20
	texts := []string{"text1", "text2", "text3", "text4", "text5"}

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				text := texts[(id+j)%len(texts)]
				embedding, err := cached.EmbedSingle(ctx, text)
				assert.NoError(t, err, "Concurrent EmbedSingle should not error")
				assert.NotNil(t, embedding, "Concurrent embedding should not be nil")
				assert.Len(t, embedding, 768, "Embedding should have 768 dimensions")
			}
		}(i)
	}

	wg.Wait()

	// Verify cache is consistent - same text should produce same embedding
	embedding1, _ := cached.EmbedSingle(ctx, "text1")
	embedding2, _ := cached.EmbedSingle(ctx, "text1")
	assert.Equal(t, embedding1, embedding2,
		"Cached embeddings should be consistent after concurrent access")
}

// TestLRUCache_Concurrent verifies that the underlying LRU cache is thread-safe
// under concurrent read/write operations including evictions.
func TestLRUCache_Concurrent(t *testing.T) {
	underlying := NewTrackingMockEmbedder()
	// Small capacity to trigger evictions
	cached := NewCachedEmbedder(underlying, 10)
	require.NotNil(t, cached, "NewCachedEmbedder should return non-nil instance")

	ctx := context.Background()
	numGoroutines := 100
	numIterations := 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Track any panics or errors
	errors := make(chan error, numGoroutines*numIterations)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errors <- &concurrencyError{msg: "panic in goroutine"}
				}
			}()

			for j := 0; j < numIterations; j++ {
				// Use many different texts to cause evictions
				text := "text" + string(rune('A'+id%26)) + string(rune('0'+j%10))
				embedding, err := cached.EmbedSingle(ctx, text)
				if err != nil {
					errors <- err
					return
				}
				if embedding == nil {
					errors <- &concurrencyError{msg: "nil embedding"}
					return
				}
				if len(embedding) != 768 {
					errors <- &concurrencyError{msg: "wrong embedding dimensions"}
					return
				}

				// Also test concurrent access to metrics and size
				_ = cached.Metrics()
				_ = cached.CacheSize()
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}

	// Cache should not exceed capacity
	assert.LessOrEqual(t, cached.CacheSize(), 10,
		"Cache size should not exceed capacity")
}

type concurrencyError struct {
	msg string
}

func (e *concurrencyError) Error() string {
	return e.msg
}
