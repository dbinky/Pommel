package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pommel-dev/pommel/internal/chunker"
	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/pommel-dev/pommel/internal/models"
)

// IndexStats contains statistics about the indexer state
type IndexStats struct {
	TotalFiles     int64
	TotalChunks    int64
	LastIndexedAt  time.Time
	PendingFiles   int64
	IndexingActive bool
}

// Indexer manages the indexing of source files
type Indexer struct {
	projectRoot string
	config      *config.Config
	db          *db.DB
	embedder    embedder.Embedder
	chunker     *chunker.ChunkerRegistry
	logger      *slog.Logger
	stats       IndexStats
	statsMu     sync.RWMutex
	indexing    atomic.Bool
}

// NewIndexer creates a new Indexer instance
func NewIndexer(projectRoot string, cfg *config.Config, database *db.DB, emb embedder.Embedder, logger *slog.Logger) (*Indexer, error) {
	registry, err := chunker.NewChunkerRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to create chunker registry: %w", err)
	}

	indexer := &Indexer{
		projectRoot: projectRoot,
		config:      cfg,
		db:          database,
		embedder:    emb,
		chunker:     registry,
		logger:      logger,
		stats:       IndexStats{},
	}

	// Load initial counts from database (without updating LastIndexedAt)
	ctx := context.Background()
	indexer.loadStats(ctx)

	return indexer, nil
}

// IndexFile indexes a single file
func (i *Indexer) IndexFile(ctx context.Context, path string) error {
	// Check context early
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Set indexing active
	i.indexing.Store(true)
	defer i.indexing.Store(false)

	// Check if file matches patterns
	relPath, err := filepath.Rel(i.projectRoot, path)
	if err != nil {
		relPath = path
	}
	if !i.MatchesPatterns(relPath) {
		i.logger.Debug("skipping file - does not match patterns", "path", path)
		return nil
	}

	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Check file size limit
	if i.config.Watcher.MaxFileSize > 0 && info.Size() > i.config.Watcher.MaxFileSize {
		i.logger.Debug("skipping file - exceeds max size", "path", path, "size", info.Size(), "maxSize", i.config.Watcher.MaxFileSize)
		return nil
	}

	// Read file content (with retry for locked files on Windows)
	content, err := ReadFileWithRetry(path, 3)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Skip empty files
	if len(content) == 0 {
		i.logger.Debug("skipping file - empty", "path", path)
		return nil
	}

	// Check context again before chunking
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Generate content hash
	hash := sha256.Sum256(content)
	contentHash := hex.EncodeToString(hash[:])

	// Create source file for chunking
	sourceFile := &models.SourceFile{
		Path:         path,
		Content:      content,
		LastModified: info.ModTime(),
	}

	// Chunk the file
	result, err := i.chunker.Chunk(ctx, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to chunk file: %w", err)
	}

	// Handle chunking errors (but continue with available chunks)
	for _, chunkErr := range result.Errors {
		i.logger.Warn("chunking error", "path", path, "error", chunkErr)
	}

	// Skip if no chunks
	if len(result.Chunks) == 0 {
		i.logger.Debug("skipping file - no chunks", "path", path)
		return nil
	}

	// Delete existing chunks and embeddings for this file
	if err := i.deleteFileData(ctx, path); err != nil {
		return fmt.Errorf("failed to delete existing data: %w", err)
	}

	// Insert file record
	fileID, err := i.db.InsertFile(ctx, path, contentHash, result.File.Language, info.Size(), info.ModTime())
	if err != nil {
		return fmt.Errorf("failed to insert file: %w", err)
	}

	// Check context before processing chunks
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Prepare chunks for insertion and embedding
	chunkIDs := make([]string, len(result.Chunks))
	chunkContents := make([]string, len(result.Chunks))

	for idx, chunk := range result.Chunks {
		// Set hashes if not already set
		if chunk.ID == "" {
			chunk.SetHashes()
		}

		// Insert chunk
		if err := i.db.InsertChunk(ctx, chunk, fileID); err != nil {
			return fmt.Errorf("failed to insert chunk: %w", err)
		}

		chunkIDs[idx] = chunk.ID
		chunkContents[idx] = chunk.Content
	}

	// Check context before embedding
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Generate embeddings
	embeddings, err := i.embedder.Embed(ctx, chunkContents)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Insert embeddings
	if err := i.db.InsertEmbeddings(ctx, chunkIDs, embeddings); err != nil {
		return fmt.Errorf("failed to insert embeddings: %w", err)
	}

	// Update stats
	i.updateStats(ctx)

	return nil
}

// DeleteFile removes a file and its chunks/embeddings from the index
func (i *Indexer) DeleteFile(ctx context.Context, path string) error {
	if err := i.deleteFileData(ctx, path); err != nil {
		return err
	}
	// Update stats after deletion
	i.updateStats(ctx)
	return nil
}

// deleteFileData removes all data associated with a file path
func (i *Indexer) deleteFileData(ctx context.Context, path string) error {
	// Get chunk IDs for this file
	chunkIDs, err := i.db.GetChunkIDsByFile(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to get chunk IDs: %w", err)
	}

	// Delete embeddings
	if len(chunkIDs) > 0 {
		if err := i.db.DeleteEmbeddingsByChunkIDs(ctx, chunkIDs); err != nil {
			return fmt.Errorf("failed to delete embeddings: %w", err)
		}
	}

	// Delete chunks
	if err := i.db.DeleteChunksByFile(ctx, path); err != nil {
		return fmt.Errorf("failed to delete chunks: %w", err)
	}

	// Delete file record
	if err := i.db.DeleteFileByPath(ctx, path); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// ReindexAll clears all data and re-indexes all matching files
func (i *Indexer) ReindexAll(ctx context.Context) error {
	// Check context early
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Set indexing active
	i.indexing.Store(true)
	defer i.indexing.Store(false)

	// Clear all data
	if err := i.db.ClearAll(ctx); err != nil {
		return fmt.Errorf("failed to clear database: %w", err)
	}

	// Reset stats
	i.statsMu.Lock()
	i.stats = IndexStats{IndexingActive: true}
	i.statsMu.Unlock()

	// Walk the project directory and index matching files
	var filesIndexed int64
	var totalChunks int64

	// Get stats update interval from config (default to 10 if not set)
	statsUpdateInterval := i.config.Daemon.StatsUpdateInterval
	if statsUpdateInterval <= 0 {
		statsUpdateInterval = 10
	}

	err := filepath.Walk(i.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			i.logger.Warn("error accessing path", "path", path, "error", err)
			return nil // Continue walking
		}

		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if info.IsDir() {
			// Skip .pommel directory
			if info.Name() == ".pommel" {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path for pattern matching
		relPath, err := filepath.Rel(i.projectRoot, path)
		if err != nil {
			relPath = path
		}

		// Check if file matches patterns
		if !i.MatchesPatterns(relPath) {
			return nil
		}

		// Index the file (without recursive indexing flag since we're doing full reindex)
		if err := i.indexFileInternal(ctx, path, info); err != nil {
			i.logger.Warn("failed to index file", "path", path, "error", err)
			return nil // Continue with other files
		}

		filesIndexed++

		// Periodically update stats during indexing so status shows progress
		if filesIndexed%int64(statsUpdateInterval) == 0 {
			if fc, err := i.db.FileCount(ctx); err == nil {
				if cc, err := i.db.ChunkCount(ctx); err == nil {
					i.statsMu.Lock()
					i.stats.TotalFiles = fc
					i.stats.TotalChunks = cc
					i.statsMu.Unlock()
				}
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	// Get final counts from database
	fileCount, err := i.db.FileCount(ctx)
	if err != nil {
		i.logger.Warn("failed to get file count", "error", err)
		fileCount = filesIndexed
	}

	chunkCount, err := i.db.ChunkCount(ctx)
	if err != nil {
		i.logger.Warn("failed to get chunk count", "error", err)
		chunkCount = totalChunks
	}

	// Update stats
	i.statsMu.Lock()
	i.stats.TotalFiles = fileCount
	i.stats.TotalChunks = chunkCount
	i.stats.LastIndexedAt = time.Now()
	i.stats.IndexingActive = false
	i.statsMu.Unlock()

	return nil
}

// indexFileInternal is the internal implementation of IndexFile for use during ReindexAll
func (i *Indexer) indexFileInternal(ctx context.Context, path string, info os.FileInfo) error {
	// Check file size limit
	if i.config.Watcher.MaxFileSize > 0 && info.Size() > i.config.Watcher.MaxFileSize {
		i.logger.Debug("skipping file - exceeds max size", "path", path, "size", info.Size())
		return nil
	}

	// Read file content (with retry for locked files on Windows)
	content, err := ReadFileWithRetry(path, 3)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Skip empty files
	if len(content) == 0 {
		return nil
	}

	// Generate content hash
	hash := sha256.Sum256(content)
	contentHash := hex.EncodeToString(hash[:])

	// Create source file for chunking
	sourceFile := &models.SourceFile{
		Path:         path,
		Content:      content,
		LastModified: info.ModTime(),
	}

	// Chunk the file
	result, err := i.chunker.Chunk(ctx, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to chunk file: %w", err)
	}

	// Skip if no chunks
	if len(result.Chunks) == 0 {
		return nil
	}

	// Insert file record
	fileID, err := i.db.InsertFile(ctx, path, contentHash, result.File.Language, info.Size(), info.ModTime())
	if err != nil {
		return fmt.Errorf("failed to insert file: %w", err)
	}

	// Prepare chunks for insertion and embedding
	chunkIDs := make([]string, len(result.Chunks))
	chunkContents := make([]string, len(result.Chunks))

	for idx, chunk := range result.Chunks {
		// Set hashes if not already set
		if chunk.ID == "" {
			chunk.SetHashes()
		}

		// Insert chunk
		if err := i.db.InsertChunk(ctx, chunk, fileID); err != nil {
			return fmt.Errorf("failed to insert chunk: %w", err)
		}

		chunkIDs[idx] = chunk.ID
		chunkContents[idx] = chunk.Content
	}

	// Generate embeddings
	embeddings, err := i.embedder.Embed(ctx, chunkContents)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Insert embeddings
	if err := i.db.InsertEmbeddings(ctx, chunkIDs, embeddings); err != nil {
		return fmt.Errorf("failed to insert embeddings: %w", err)
	}

	return nil
}

// Stats returns the current indexer statistics
func (i *Indexer) Stats() IndexStats {
	i.statsMu.RLock()
	defer i.statsMu.RUnlock()

	stats := i.stats
	stats.IndexingActive = i.indexing.Load()
	return stats
}

// loadStats loads counts from the database without updating LastIndexedAt
func (i *Indexer) loadStats(ctx context.Context) {
	fileCount, err := i.db.FileCount(ctx)
	if err != nil {
		i.logger.Warn("failed to get file count", "error", err)
	}

	chunkCount, err := i.db.ChunkCount(ctx)
	if err != nil {
		i.logger.Warn("failed to get chunk count", "error", err)
	}

	i.statsMu.Lock()
	i.stats.TotalFiles = fileCount
	i.stats.TotalChunks = chunkCount
	i.statsMu.Unlock()
}

// updateStats refreshes statistics from the database and updates LastIndexedAt
func (i *Indexer) updateStats(ctx context.Context) {
	fileCount, err := i.db.FileCount(ctx)
	if err != nil {
		i.logger.Warn("failed to get file count", "error", err)
	}

	chunkCount, err := i.db.ChunkCount(ctx)
	if err != nil {
		i.logger.Warn("failed to get chunk count", "error", err)
	}

	i.statsMu.Lock()
	i.stats.TotalFiles = fileCount
	i.stats.TotalChunks = chunkCount
	i.stats.LastIndexedAt = time.Now()
	i.statsMu.Unlock()
}

// MatchesPatterns checks if a file path matches the include patterns and doesn't match exclude patterns
func (i *Indexer) MatchesPatterns(path string) bool {
	// Normalize path separators
	path = filepath.ToSlash(path)

	// Check exclude patterns first
	for _, pattern := range i.config.ExcludePatterns {
		if matchGlob(pattern, path) {
			return false
		}
	}

	// Check include patterns
	for _, pattern := range i.config.IncludePatterns {
		if matchGlob(pattern, path) {
			return true
		}
	}

	return false
}

// matchGlob checks if a path matches a glob pattern
// Supports ** for any path segment and * for wildcards within a segment
func matchGlob(pattern, path string) bool {
	// Normalize pattern
	pattern = filepath.ToSlash(pattern)

	// Handle **/ prefix patterns (match anywhere in path)
	if strings.HasPrefix(pattern, "**/") {
		suffix := pattern[3:]
		// Check if it matches from the start
		if matchSimpleGlob(suffix, path) {
			return true
		}
		// Check each path segment
		parts := strings.Split(path, "/")
		for i := range parts {
			subPath := strings.Join(parts[i:], "/")
			if matchSimpleGlob(suffix, subPath) {
				return true
			}
		}
		return false
	}

	// Handle **/ anywhere in pattern
	if strings.Contains(pattern, "**/") {
		parts := strings.Split(pattern, "**/")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]

			// If there's a prefix, the path must start with it
			if prefix != "" {
				if !strings.HasPrefix(path, prefix) {
					return false
				}
				path = path[len(prefix):]
			}

			// The path must contain something matching the suffix
			pathParts := strings.Split(path, "/")
			for i := range pathParts {
				subPath := strings.Join(pathParts[i:], "/")
				if matchSimpleGlob(suffix, subPath) {
					return true
				}
			}
			return false
		}
	}

	return matchSimpleGlob(pattern, path)
}

// matchSimpleGlob matches a pattern with only * wildcards (no **)
func matchSimpleGlob(pattern, path string) bool {
	// Handle trailing /** pattern
	if strings.HasSuffix(pattern, "/**") {
		prefix := pattern[:len(pattern)-3]
		return strings.HasPrefix(path, prefix+"/") || path == prefix
	}

	// Simple * wildcards only match within the same path segment
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i, pp := range patternParts {
		if !matchWildcard(pp, pathParts[i]) {
			return false
		}
	}
	return true
}

// matchWildcard matches a single pattern segment with * wildcards
func matchWildcard(pattern, str string) bool {
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		return pattern == str
	}

	// Split by * and match each part
	parts := strings.Split(pattern, "*")
	pos := 0

	for i, part := range parts {
		if part == "" {
			continue
		}

		idx := strings.Index(str[pos:], part)
		if idx == -1 {
			return false
		}

		// First part must match at the start
		if i == 0 && idx != 0 {
			return false
		}

		pos += idx + len(part)
	}

	// Last part must match at the end
	if parts[len(parts)-1] != "" {
		return strings.HasSuffix(str, parts[len(parts)-1])
	}

	return true
}
