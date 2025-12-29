# Phase 4: Daemon Core

**Phase Goal:** Build the long-running daemon with file watching, incremental indexing, and REST API server.

**Prerequisites:** Phase 1-3 complete (database, embeddings, chunking)

**Estimated Tasks:** 16 tasks across 5 areas

---

## Table of Contents

1. [Overview](#overview)
2. [Success Criteria](#success-criteria)
3. [Task 4.1: File Watcher](#task-41-file-watcher)
4. [Task 4.2: Indexer](#task-42-indexer)
5. [Task 4.3: State Management](#task-43-state-management)
6. [Task 4.4: REST API Server](#task-44-rest-api-server)
7. [Task 4.5: Daemon Orchestration](#task-45-daemon-orchestration)
8. [Dependencies](#dependencies)
9. [Testing Strategy](#testing-strategy)

---

## Overview

Phase 4 builds the daemon - the heart of Pommel that runs continuously, watching for file changes and keeping the index fresh. By the end of this phase:

- File changes are detected within 500ms (configurable debounce)
- Changed files are re-chunked and re-embedded automatically
- REST API serves status and health endpoints
- Daemon can be started and stopped gracefully
- State persists across restarts

---

## Success Criteria

| Criterion | Validation |
|-----------|------------|
| File watcher detects changes | Create/modify/delete trigger events |
| Debouncing works | Rapid saves produce single index |
| Indexer updates database | Modified file has updated chunks |
| REST API responds | GET /health returns 200 |
| Graceful shutdown | SIGTERM stops daemon cleanly |
| State persists | PID file and state.json updated |

---

## Task 4.1: File Watcher

### 4.1.1 Implement File Watcher with fsnotify

**File Content (internal/daemon/watcher.go):**
```go
package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pommel-dev/pommel/internal/config"
)

// FileEvent represents a file system event
type FileEvent struct {
	Path      string
	Op        Operation
	Timestamp time.Time
}

// Operation type
type Operation int

const (
	OpCreate Operation = iota
	OpModify
	OpDelete
	OpRename
)

// Watcher monitors file system for changes
type Watcher struct {
	projectRoot string
	config      *config.Config
	fsWatcher   *fsnotify.Watcher
	events      chan FileEvent
	errors      chan error

	// Debouncing
	pending     map[string]*time.Timer
	pendingMu   sync.Mutex

	// Ignore patterns
	ignorer     *Ignorer

	done        chan struct{}
}

// NewWatcher creates a new file watcher
func NewWatcher(projectRoot string, cfg *config.Config) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ignorer, err := NewIgnorer(projectRoot, cfg.ExcludePatterns)
	if err != nil {
		fsWatcher.Close()
		return nil, err
	}

	return &Watcher{
		projectRoot: projectRoot,
		config:      cfg,
		fsWatcher:   fsWatcher,
		events:      make(chan FileEvent, 100),
		errors:      make(chan error, 10),
		pending:     make(map[string]*time.Timer),
		ignorer:     ignorer,
		done:        make(chan struct{}),
	}, nil
}

// Start begins watching the project directory
func (w *Watcher) Start(ctx context.Context) error {
	// Add all directories recursively
	err := filepath.Walk(w.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if w.ignorer.ShouldIgnore(path) {
				return filepath.SkipDir
			}
			return w.fsWatcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Start event processing
	go w.processEvents(ctx)

	return nil
}

func (w *Watcher) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			close(w.done)
			return

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			select {
			case w.errors <- err:
			default:
			}
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	// Skip ignored paths
	if w.ignorer.ShouldIgnore(path) {
		return
	}

	// Skip non-matching patterns
	if !w.matchesIncludePattern(path) {
		// But handle directory creation
		if event.Op&fsnotify.Create != 0 {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				w.fsWatcher.Add(path)
			}
		}
		return
	}

	// Debounce the event
	w.debounce(path, event.Op)
}

func (w *Watcher) debounce(path string, op fsnotify.Op) {
	w.pendingMu.Lock()
	defer w.pendingMu.Unlock()

	// Cancel existing timer
	if timer, ok := w.pending[path]; ok {
		timer.Stop()
	}

	// Create new timer
	debounce := w.config.Watcher.DebounceDuration()
	w.pending[path] = time.AfterFunc(debounce, func() {
		w.pendingMu.Lock()
		delete(w.pending, path)
		w.pendingMu.Unlock()

		// Determine operation
		var fileOp Operation
		switch {
		case op&fsnotify.Remove != 0:
			fileOp = OpDelete
		case op&fsnotify.Create != 0:
			fileOp = OpCreate
		case op&fsnotify.Rename != 0:
			fileOp = OpRename
		default:
			fileOp = OpModify
		}

		// Send event
		select {
		case w.events <- FileEvent{Path: path, Op: fileOp, Timestamp: time.Now()}:
		default:
			// Channel full, skip
		}
	})
}

func (w *Watcher) matchesIncludePattern(path string) bool {
	for _, pattern := range w.config.IncludePatterns {
		// Simple glob matching
		matched, _ := filepath.Match(pattern, filepath.Base(path))
		if matched {
			return true
		}
		// Check extension patterns like **/*.cs
		if strings.HasPrefix(pattern, "**/*") {
			ext := strings.TrimPrefix(pattern, "**/*")
			if strings.HasSuffix(path, ext) {
				return true
			}
		}
	}
	return false
}

// Events returns the channel of file events
func (w *Watcher) Events() <-chan FileEvent {
	return w.events
}

// Errors returns the channel of errors
func (w *Watcher) Errors() <-chan error {
	return w.errors
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	return w.fsWatcher.Close()
}

// Done returns a channel that closes when watcher stops
func (w *Watcher) Done() <-chan struct{} {
	return w.done
}
```

---

### 4.1.2 Implement Ignore Pattern Matcher

**File Content (internal/daemon/ignorer.go):**
```go
package daemon

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Ignorer determines which paths should be ignored
type Ignorer struct {
	projectRoot string
	patterns    []string
}

// NewIgnorer creates a new ignorer with patterns from config and .pommelignore/.gitignore
func NewIgnorer(projectRoot string, configPatterns []string) (*Ignorer, error) {
	patterns := make([]string, 0)

	// Add config patterns
	patterns = append(patterns, configPatterns...)

	// Load .pommelignore
	pommelIgnore := filepath.Join(projectRoot, ".pommelignore")
	if filePatterns, err := loadIgnoreFile(pommelIgnore); err == nil {
		patterns = append(patterns, filePatterns...)
	}

	// Load .gitignore
	gitIgnore := filepath.Join(projectRoot, ".gitignore")
	if filePatterns, err := loadIgnoreFile(gitIgnore); err == nil {
		patterns = append(patterns, filePatterns...)
	}

	// Always ignore .pommel directory
	patterns = append(patterns, ".pommel", ".pommel/**")

	return &Ignorer{
		projectRoot: projectRoot,
		patterns:    patterns,
	}, nil
}

func loadIgnoreFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns, scanner.Err()
}

// ShouldIgnore returns true if the path should be ignored
func (i *Ignorer) ShouldIgnore(path string) bool {
	// Get relative path
	relPath, err := filepath.Rel(i.projectRoot, path)
	if err != nil {
		relPath = path
	}

	for _, pattern := range i.patterns {
		if i.matchPattern(relPath, pattern) {
			return true
		}
	}
	return false
}

func (i *Ignorer) matchPattern(path, pattern string) bool {
	// Handle directory patterns
	if strings.HasSuffix(pattern, "/") {
		pattern = pattern[:len(pattern)-1]
		if strings.Contains(path, pattern+string(filepath.Separator)) {
			return true
		}
		return path == pattern
	}

	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		// Convert to regex-like matching
		pattern = strings.ReplaceAll(pattern, "**", "*")
		parts := strings.Split(pattern, "*")

		idx := 0
		for _, part := range parts {
			if part == "" {
				continue
			}
			newIdx := strings.Index(path[idx:], part)
			if newIdx == -1 {
				return false
			}
			idx += newIdx + len(part)
		}
		return true
	}

	// Simple glob matching
	matched, _ := filepath.Match(pattern, filepath.Base(path))
	if matched {
		return true
	}

	// Try matching full relative path
	matched, _ = filepath.Match(pattern, relPath)
	return matched
}
```

---

### 4.1.3 Write Watcher Tests

**File Content (internal/daemon/watcher_test.go):**
```go
package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcher_DetectsChanges(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.Default()
	cfg.IncludePatterns = []string{"**/*.txt"}
	cfg.Watcher.DebounceMs = 50

	watcher, err := NewWatcher(tmpDir, cfg)
	require.NoError(t, err)
	defer watcher.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = watcher.Start(ctx)
	require.NoError(t, err)

	// Create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("hello"), 0644)
	require.NoError(t, err)

	// Wait for event
	select {
	case event := <-watcher.Events():
		assert.Equal(t, testFile, event.Path)
		assert.Equal(t, OpCreate, event.Op)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestWatcher_Debounces(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.Default()
	cfg.IncludePatterns = []string{"**/*.txt"}
	cfg.Watcher.DebounceMs = 100

	watcher, err := NewWatcher(tmpDir, cfg)
	require.NoError(t, err)
	defer watcher.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = watcher.Start(ctx)
	require.NoError(t, err)

	// Create file
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("1"), 0644)

	// Rapid modifications
	for i := 0; i < 5; i++ {
		os.WriteFile(testFile, []byte(string(rune('0'+i))), 0644)
		time.Sleep(20 * time.Millisecond)
	}

	// Should only get one event after debounce
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

	assert.LessOrEqual(t, eventCount, 2) // Create + final modify
}

func TestWatcher_IgnoresPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.Default()
	cfg.IncludePatterns = []string{"**/*.txt"}
	cfg.ExcludePatterns = []string{"ignored/**"}
	cfg.Watcher.DebounceMs = 50

	// Create ignored directory
	ignoredDir := filepath.Join(tmpDir, "ignored")
	os.MkdirAll(ignoredDir, 0755)

	watcher, err := NewWatcher(tmpDir, cfg)
	require.NoError(t, err)
	defer watcher.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = watcher.Start(ctx)
	require.NoError(t, err)

	// Create file in ignored directory
	ignoredFile := filepath.Join(ignoredDir, "test.txt")
	os.WriteFile(ignoredFile, []byte("ignored"), 0644)

	// Should not get event
	select {
	case <-watcher.Events():
		t.Fatal("should not receive event for ignored file")
	case <-time.After(200 * time.Millisecond):
		// Expected
	}
}
```

---

## Task 4.2: Indexer

### 4.2.1 Implement Indexer

**File Content (internal/daemon/indexer.go):**
```go
package daemon

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pommel-dev/pommel/internal/chunker"
	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/pommel-dev/pommel/internal/models"
)

// IndexStats contains indexing statistics
type IndexStats struct {
	TotalFiles     int64
	TotalChunks    int64
	LastIndexedAt  time.Time
	PendingFiles   int64
	IndexingActive bool
}

// Indexer manages the indexing pipeline
type Indexer struct {
	projectRoot string
	config      *config.Config
	db          *db.DB
	embedder    embedder.Embedder
	chunker     *chunker.ChunkerRegistry
	logger      *slog.Logger

	stats      IndexStats
	statsMu    sync.RWMutex
	indexing   atomic.Bool
}

// NewIndexer creates a new indexer
func NewIndexer(
	projectRoot string,
	cfg *config.Config,
	database *db.DB,
	emb embedder.Embedder,
	logger *slog.Logger,
) (*Indexer, error) {
	chunkerReg, err := chunker.NewChunkerRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to create chunker: %w", err)
	}

	return &Indexer{
		projectRoot: projectRoot,
		config:      cfg,
		db:          database,
		embedder:    emb,
		chunker:     chunkerReg,
		logger:      logger,
	}, nil
}

// IndexFile indexes a single file
func (i *Indexer) IndexFile(ctx context.Context, path string) error {
	i.indexing.Store(true)
	defer i.indexing.Store(false)

	i.logger.Debug("indexing file", "path", path)

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Check file size
	if info.Size() > i.config.Watcher.MaxFileSize {
		i.logger.Warn("file too large, skipping", "path", path, "size", info.Size())
		return nil
	}

	// Create source file
	relPath, _ := filepath.Rel(i.projectRoot, path)
	sourceFile := &models.SourceFile{
		Path:         relPath,
		Content:      content,
		LastModified: info.ModTime(),
	}

	// Chunk the file
	result, err := i.chunker.Chunk(ctx, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to chunk file: %w", err)
	}

	// Delete existing chunks for this file
	if err := i.db.DeleteChunksByFile(ctx, relPath); err != nil {
		return fmt.Errorf("failed to delete existing chunks: %w", err)
	}

	// Generate embeddings
	texts := make([]string, len(result.Chunks))
	for j, chunk := range result.Chunks {
		texts[j] = chunk.Content
	}

	embeddings, err := i.embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Store chunks and embeddings
	for j, chunk := range result.Chunks {
		if err := i.db.InsertChunk(ctx, chunk); err != nil {
			return fmt.Errorf("failed to insert chunk: %w", err)
		}
		if err := i.db.InsertEmbedding(ctx, chunk.ID, embeddings[j]); err != nil {
			return fmt.Errorf("failed to insert embedding: %w", err)
		}
	}

	// Update stats
	i.statsMu.Lock()
	i.stats.TotalChunks += int64(len(result.Chunks))
	i.stats.LastIndexedAt = time.Now()
	i.statsMu.Unlock()

	i.logger.Debug("indexed file", "path", path, "chunks", len(result.Chunks))

	return nil
}

// DeleteFile removes all chunks for a file
func (i *Indexer) DeleteFile(ctx context.Context, path string) error {
	relPath, _ := filepath.Rel(i.projectRoot, path)

	// Get chunk IDs to delete embeddings
	chunkIDs, err := i.db.GetChunkIDsByFile(ctx, relPath)
	if err != nil {
		return fmt.Errorf("failed to get chunk IDs: %w", err)
	}

	// Delete embeddings
	if err := i.db.DeleteEmbeddingsByChunkIDs(ctx, chunkIDs); err != nil {
		return fmt.Errorf("failed to delete embeddings: %w", err)
	}

	// Delete chunks
	if err := i.db.DeleteChunksByFile(ctx, relPath); err != nil {
		return fmt.Errorf("failed to delete chunks: %w", err)
	}

	i.logger.Debug("deleted file from index", "path", path)

	return nil
}

// ReindexAll reindexes all files in the project
func (i *Indexer) ReindexAll(ctx context.Context) error {
	i.logger.Info("starting full reindex")

	// Clear existing data
	if err := i.db.ClearAll(ctx); err != nil {
		return fmt.Errorf("failed to clear database: %w", err)
	}

	// Walk project directory
	var fileCount int64
	err := filepath.Walk(i.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			return nil
		}

		// Check if file matches patterns
		if !i.matchesPatterns(path) {
			return nil
		}

		if err := i.IndexFile(ctx, path); err != nil {
			i.logger.Error("failed to index file", "path", path, "error", err)
			return nil // Continue with other files
		}

		fileCount++
		return nil
	})

	if err != nil {
		return fmt.Errorf("walk failed: %w", err)
	}

	i.statsMu.Lock()
	i.stats.TotalFiles = fileCount
	i.statsMu.Unlock()

	i.logger.Info("full reindex complete", "files", fileCount)

	return nil
}

func (i *Indexer) matchesPatterns(path string) bool {
	for _, pattern := range i.config.IncludePatterns {
		if strings.HasPrefix(pattern, "**/*") {
			ext := strings.TrimPrefix(pattern, "**/*")
			if strings.HasSuffix(path, ext) {
				return true
			}
		}
	}
	return false
}

// Stats returns current indexing statistics
func (i *Indexer) Stats() IndexStats {
	i.statsMu.RLock()
	defer i.statsMu.RUnlock()
	stats := i.stats
	stats.IndexingActive = i.indexing.Load()
	return stats
}
```

---

## Task 4.3: State Management

### 4.3.1 Implement State Manager

**File Content (internal/daemon/state.go):**
```go
package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	StateFile = "state.json"
	PIDFile   = "pommel.pid"
)

// DaemonState represents persistent daemon state
type DaemonState struct {
	Version int `json:"version"`
	Daemon  struct {
		PID       int       `json:"pid"`
		StartedAt time.Time `json:"started_at"`
		Port      int       `json:"port"`
	} `json:"daemon"`
	Index struct {
		LastFullIndex time.Time `json:"last_full_index"`
		TotalFiles    int       `json:"total_files"`
		TotalChunks   int       `json:"total_chunks"`
	} `json:"index"`
}

// StateManager handles daemon state persistence
type StateManager struct {
	pommelDir string
}

// NewStateManager creates a new state manager
func NewStateManager(projectRoot string) *StateManager {
	return &StateManager{
		pommelDir: filepath.Join(projectRoot, ".pommel"),
	}
}

// SaveState writes the current state to disk
func (s *StateManager) SaveState(state *DaemonState) error {
	path := filepath.Join(s.pommelDir, StateFile)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// LoadState reads the state from disk
func (s *StateManager) LoadState() (*DaemonState, error) {
	path := filepath.Join(s.pommelDir, StateFile)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &DaemonState{Version: 1}, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state DaemonState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// WritePID writes the daemon PID to a file
func (s *StateManager) WritePID(pid int) error {
	path := filepath.Join(s.pommelDir, PIDFile)
	return os.WriteFile(path, []byte(fmt.Sprintf("%d", pid)), 0644)
}

// ReadPID reads the daemon PID from file
func (s *StateManager) ReadPID() (int, error) {
	path := filepath.Join(s.pommelDir, PIDFile)

	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return 0, fmt.Errorf("invalid PID file: %w", err)
	}

	return pid, nil
}

// RemovePID removes the PID file
func (s *StateManager) RemovePID() error {
	path := filepath.Join(s.pommelDir, PIDFile)
	return os.Remove(path)
}

// IsRunning checks if a daemon is already running
func (s *StateManager) IsRunning() (bool, int) {
	pid, err := s.ReadPID()
	if err != nil {
		return false, 0
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}

	// On Unix, FindProcess always succeeds. Send signal 0 to check if alive.
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		s.RemovePID() // Clean up stale PID file
		return false, 0
	}

	return true, pid
}
```

---

## Task 4.4: REST API Server

### 4.4.1 Implement API Router

**File Content (internal/api/router.go):**
```go
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server represents the REST API server
type Server struct {
	router  chi.Router
	handler *Handler
	logger  *slog.Logger
}

// NewServer creates a new API server
func NewServer(handler *Handler, logger *slog.Logger) *Server {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(LoggerMiddleware(logger))

	s := &Server{
		router:  r,
		handler: handler,
		logger:  logger,
	}

	s.setupRoutes()

	return s
}

func (s *Server) setupRoutes() {
	s.router.Get("/health", s.handler.Health)
	s.router.Get("/status", s.handler.Status)
	s.router.Post("/search", s.handler.Search)
	s.router.Post("/reindex", s.handler.Reindex)
	s.router.Get("/config", s.handler.GetConfig)
}

// Handler returns the http.Handler
func (s *Server) Handler() http.Handler {
	return s.router
}

// LoggerMiddleware creates a logging middleware
func LoggerMiddleware(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			logger.Debug("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration", time.Since(start),
			)
		})
	}
}
```

---

### 4.4.2 Implement API Handlers

**File Content (internal/api/handlers.go):**
```go
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/daemon"
)

// Handler contains all API handlers
type Handler struct {
	indexer  *daemon.Indexer
	config   *config.Config
	searcher Searcher
	startTime time.Time
}

// Searcher interface for search operations
type Searcher interface {
	Search(ctx context.Context, query SearchRequest) (*SearchResponse, error)
}

// NewHandler creates a new handler
func NewHandler(indexer *daemon.Indexer, cfg *config.Config, searcher Searcher) *Handler {
	return &Handler{
		indexer:   indexer,
		config:    cfg,
		searcher:  searcher,
		startTime: time.Now(),
	}
}

// HealthResponse is the /health response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// Health handles GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
	}
	writeJSON(w, http.StatusOK, resp)
}

// StatusResponse is the /status response
type StatusResponse struct {
	Daemon       DaemonStatus       `json:"daemon"`
	Index        IndexStatus        `json:"index"`
	Dependencies DependencyStatus   `json:"dependencies"`
}

type DaemonStatus struct {
	Running       bool   `json:"running"`
	PID           int    `json:"pid"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	Version       string `json:"version"`
}

type IndexStatus struct {
	TotalFiles     int64     `json:"total_files"`
	TotalChunks    int64     `json:"total_chunks"`
	LastIndexed    time.Time `json:"last_indexed"`
	PendingChanges int       `json:"pending_changes"`
	IndexingActive bool      `json:"indexing_active"`
}

type DependencyStatus struct {
	Ollama   OllamaStatus   `json:"ollama"`
	Database DatabaseStatus `json:"database"`
}

type OllamaStatus struct {
	Status      string `json:"status"`
	ModelLoaded bool   `json:"model_loaded"`
	Model       string `json:"model"`
}

type DatabaseStatus struct {
	Status    string `json:"status"`
	SizeBytes int64  `json:"size_bytes"`
}

// Status handles GET /status
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	stats := h.indexer.Stats()

	resp := StatusResponse{
		Daemon: DaemonStatus{
			Running:       true,
			PID:           os.Getpid(),
			UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
			Version:       "0.1.0",
		},
		Index: IndexStatus{
			TotalFiles:     stats.TotalFiles,
			TotalChunks:    stats.TotalChunks,
			LastIndexed:    stats.LastIndexedAt,
			IndexingActive: stats.IndexingActive,
		},
		Dependencies: DependencyStatus{
			Ollama: OllamaStatus{
				Status: "running",
				Model:  h.config.Embedding.Model,
			},
			Database: DatabaseStatus{
				Status: "connected",
			},
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// SearchRequest is the POST /search request
type SearchRequest struct {
	Query      string   `json:"query"`
	Limit      int      `json:"limit,omitempty"`
	Levels     []string `json:"levels,omitempty"`
	PathPrefix string   `json:"path_prefix,omitempty"`
}

// Search handles POST /search
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "query is required")
		return
	}

	// Apply defaults
	if req.Limit == 0 {
		req.Limit = h.config.Search.DefaultLimit
	}
	if len(req.Levels) == 0 {
		req.Levels = h.config.Search.DefaultLevels
	}

	resp, err := h.searcher.Search(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SEARCH_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// ReindexRequest is the POST /reindex request
type ReindexRequest struct {
	Force bool `json:"force"`
}

// ReindexResponse is the POST /reindex response
type ReindexResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Reindex handles POST /reindex
func (h *Handler) Reindex(w http.ResponseWriter, r *http.Request) {
	// Start reindex in background
	go h.indexer.ReindexAll(r.Context())

	resp := ReindexResponse{
		Status:  "started",
		Message: "Reindex started in background",
	}
	writeJSON(w, http.StatusAccepted, resp)
}

// GetConfig handles GET /config
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.config)
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	resp := ErrorResponse{}
	resp.Error.Code = code
	resp.Error.Message = message
	writeJSON(w, status, resp)
}
```

---

## Task 4.5: Daemon Orchestration

### 4.5.1 Implement Daemon Main Loop

**File Content (internal/daemon/daemon.go):**
```go
package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pommel-dev/pommel/internal/api"
	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
)

// Daemon orchestrates all daemon components
type Daemon struct {
	projectRoot string
	config      *config.Config
	logger      *slog.Logger

	db       *db.DB
	embedder embedder.Embedder
	indexer  *Indexer
	watcher  *Watcher
	server   *http.Server
	state    *StateManager
}

// New creates a new daemon
func New(projectRoot string, cfg *config.Config, logger *slog.Logger) (*Daemon, error) {
	// Open database
	database, err := db.Open(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Run migrations
	if err := database.Migrate(context.Background()); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Create embedder
	emb := embedder.NewOllamaClient(embedder.OllamaConfig{
		Model: cfg.Embedding.Model,
	})

	// Create cached embedder
	cachedEmb := embedder.NewCachedEmbedder(emb, cfg.Embedding.CacheSize)

	// Create indexer
	indexer, err := NewIndexer(projectRoot, cfg, database, cachedEmb, logger)
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to create indexer: %w", err)
	}

	// Create watcher
	watcher, err := NewWatcher(projectRoot, cfg)
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	return &Daemon{
		projectRoot: projectRoot,
		config:      cfg,
		logger:      logger,
		db:          database,
		embedder:    cachedEmb,
		indexer:     indexer,
		watcher:     watcher,
		state:       NewStateManager(projectRoot),
	}, nil
}

// Run starts the daemon and blocks until shutdown
func (d *Daemon) Run(ctx context.Context) error {
	// Check if already running
	if running, pid := d.state.IsRunning(); running {
		return fmt.Errorf("daemon already running with PID %d", pid)
	}

	// Write PID file
	if err := d.state.WritePID(os.Getpid()); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer d.state.RemovePID()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start components
	d.logger.Info("starting daemon", "project", d.projectRoot, "port", d.config.Daemon.Port)

	// Start file watcher
	if err := d.watcher.Start(ctx); err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}

	// Start API server
	handler := api.NewHandler(d.indexer, d.config, d)
	apiServer := api.NewServer(handler, d.logger)

	d.server = &http.Server{
		Addr:    d.config.Daemon.Address(),
		Handler: apiServer.Handler(),
	}

	// Server goroutine
	serverErr := make(chan error, 1)
	go func() {
		d.logger.Info("API server listening", "addr", d.server.Addr)
		if err := d.server.ListenAndServe(); err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// File event processor
	go d.processFileEvents(ctx)

	// Do initial index if database is empty
	go d.initialIndexIfNeeded(ctx)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
	case sig := <-sigCh:
		d.logger.Info("received signal", "signal", sig)
	case err := <-serverErr:
		d.logger.Error("server error", "error", err)
		cancel()
		return err
	}

	// Graceful shutdown
	return d.shutdown()
}

func (d *Daemon) processFileEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-d.watcher.Events():
			d.handleFileEvent(ctx, event)
		case err := <-d.watcher.Errors():
			d.logger.Error("watcher error", "error", err)
		}
	}
}

func (d *Daemon) handleFileEvent(ctx context.Context, event FileEvent) {
	switch event.Op {
	case OpCreate, OpModify:
		if err := d.indexer.IndexFile(ctx, event.Path); err != nil {
			d.logger.Error("failed to index file", "path", event.Path, "error", err)
		}
	case OpDelete:
		if err := d.indexer.DeleteFile(ctx, event.Path); err != nil {
			d.logger.Error("failed to delete file from index", "path", event.Path, "error", err)
		}
	case OpRename:
		// Treat as delete + create
		d.indexer.DeleteFile(ctx, event.Path)
		// The new file will trigger a Create event
	}
}

func (d *Daemon) initialIndexIfNeeded(ctx context.Context) {
	count, err := d.db.EmbeddingCount(ctx)
	if err != nil {
		d.logger.Error("failed to check embedding count", "error", err)
		return
	}

	if count == 0 {
		d.logger.Info("database empty, starting initial index")
		if err := d.indexer.ReindexAll(ctx); err != nil {
			d.logger.Error("initial index failed", "error", err)
		}
	}
}

func (d *Daemon) shutdown() error {
	d.logger.Info("shutting down")

	// Stop HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := d.server.Shutdown(ctx); err != nil {
		d.logger.Error("server shutdown error", "error", err)
	}

	// Stop watcher
	d.watcher.Stop()

	// Close database
	d.db.Close()

	// Save state
	state, _ := d.state.LoadState()
	state.Index.TotalFiles = int(d.indexer.Stats().TotalFiles)
	state.Index.TotalChunks = int(d.indexer.Stats().TotalChunks)
	d.state.SaveState(state)

	d.logger.Info("shutdown complete")
	return nil
}

// Search implements the Searcher interface for the API handler
func (d *Daemon) Search(ctx context.Context, req api.SearchRequest) (*api.SearchResponse, error) {
	// This will be implemented in Phase 5
	return nil, fmt.Errorf("not implemented")
}
```

---

## Dependencies

```
github.com/fsnotify/fsnotify v1.7.0
github.com/go-chi/chi/v5 v5.0.11
```

---

## Testing Strategy

- Unit tests for each component with mocks
- Integration tests with temp directories
- Test file watching with actual file operations
- Test API endpoints with httptest

---

## Checklist

Before marking Phase 4 complete:

- [ ] File watcher detects create/modify/delete
- [ ] Debouncing reduces event noise
- [ ] Ignore patterns work correctly
- [ ] Indexer updates database on file changes
- [ ] REST API serves health/status endpoints
- [ ] State persists across restarts
- [ ] Graceful shutdown works
- [ ] All tests pass
