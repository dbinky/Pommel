package embedder

import (
	"container/list"
	"context"
	"sync"
)

// lruCache is a thread-safe LRU cache for embeddings.
type lruCache struct {
	capacity int
	items    map[string]*list.Element
	order    *list.List
	mu       sync.Mutex
}

// cacheEntry holds a key and embedding pair in the LRU cache.
type cacheEntry struct {
	key       string
	embedding []float32
}

// newLRUCache creates a new LRU cache with the specified capacity.
func newLRUCache(capacity int) *lruCache {
	return &lruCache{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

// Get retrieves an embedding from the cache. Returns nil if not found.
// Accessing an entry moves it to the front (most recently used).
func (c *lruCache) Get(key string) []float32 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		// Move to front (most recently used)
		c.order.MoveToFront(elem)
		return elem.Value.(*cacheEntry).embedding
	}
	return nil
}

// Put adds or updates an entry in the cache.
// If the cache is at capacity, the least recently used entry is evicted.
func (c *lruCache) Put(key string, embedding []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If key already exists, update it and move to front
	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		elem.Value.(*cacheEntry).embedding = embedding
		return
	}

	// Evict oldest entry if at capacity
	if c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.items, oldest.Value.(*cacheEntry).key)
		}
	}

	// Add new entry at front
	entry := &cacheEntry{key: key, embedding: embedding}
	elem := c.order.PushFront(entry)
	c.items[key] = elem
}

// Size returns the current number of entries in the cache.
func (c *lruCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Clear removes all entries from the cache.
func (c *lruCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.order.Init()
}

// CachedEmbedder wraps an Embedder with an LRU cache to avoid redundant
// embedding computations for previously seen text.
type CachedEmbedder struct {
	embedder Embedder
	cache    *lruCache
	metrics  CacheMetrics
	mu       sync.RWMutex
}

// CacheMetrics provides statistics about cache performance.
type CacheMetrics struct {
	Hits   int64
	Misses int64
}

// Compile-time check that CachedEmbedder implements Embedder
var _ Embedder = (*CachedEmbedder)(nil)

// NewCachedEmbedder creates a new CachedEmbedder wrapping the provided embedder
// with a cache of the specified capacity.
func NewCachedEmbedder(embedder Embedder, capacity int) *CachedEmbedder {
	return &CachedEmbedder{
		embedder: embedder,
		cache:    newLRUCache(capacity),
		metrics:  CacheMetrics{},
	}
}

// EmbedSingle generates an embedding for a single text, using the cache if available.
func (c *CachedEmbedder) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	// Check cache first
	if embedding := c.cache.Get(text); embedding != nil {
		c.mu.Lock()
		c.metrics.Hits++
		c.mu.Unlock()
		return embedding, nil
	}

	// Cache miss - call underlying embedder
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

// Embed generates embeddings for multiple texts, using the cache where available.
func (c *CachedEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	var uncachedTexts []string
	var uncachedIndices []int

	// Check cache for each text
	for i, text := range texts {
		if embedding := c.cache.Get(text); embedding != nil {
			c.mu.Lock()
			c.metrics.Hits++
			c.mu.Unlock()
			results[i] = embedding
		} else {
			c.mu.Lock()
			c.metrics.Misses++
			c.mu.Unlock()
			uncachedTexts = append(uncachedTexts, text)
			uncachedIndices = append(uncachedIndices, i)
		}
	}

	// If all texts were cached, return immediately
	if len(uncachedTexts) == 0 {
		return results, nil
	}

	// Embed only the uncached texts
	embeddings, err := c.embedder.Embed(ctx, uncachedTexts)
	if err != nil {
		return nil, err
	}

	// Store new embeddings in cache and results
	for i, embedding := range embeddings {
		idx := uncachedIndices[i]
		text := uncachedTexts[i]
		results[idx] = embedding
		c.cache.Put(text, embedding)
	}

	return results, nil
}

// Health delegates to the underlying embedder.
func (c *CachedEmbedder) Health(ctx context.Context) error {
	return c.embedder.Health(ctx)
}

// ModelName delegates to the underlying embedder.
func (c *CachedEmbedder) ModelName() string {
	return c.embedder.ModelName()
}

// Dimensions delegates to the underlying embedder.
func (c *CachedEmbedder) Dimensions() int {
	return c.embedder.Dimensions()
}

// Metrics returns cache hit/miss statistics.
func (c *CachedEmbedder) Metrics() CacheMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CacheMetrics{
		Hits:   c.metrics.Hits,
		Misses: c.metrics.Misses,
	}
}

// CacheSize returns the current number of entries in the cache.
func (c *CachedEmbedder) CacheSize() int {
	return c.cache.Size()
}

// ClearCache removes all entries from the cache.
func (c *CachedEmbedder) ClearCache() {
	c.cache.Clear()
}
