package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/pommel-dev/pommel/internal/embedder"
)

// SearchRequest represents a search query request
type SearchRequest struct {
	Query      string   `json:"query"`
	Limit      int      `json:"limit,omitempty"`
	Levels     []string `json:"levels,omitempty"`
	PathPrefix string   `json:"path_prefix,omitempty"`
}

// SearchResponse represents the search results response
type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Query   string         `json:"query"`
	Limit   int            `json:"limit"`
}

// SearchResult represents a single search result
type SearchResult struct {
	ChunkID   string  `json:"chunk_id"`
	FilePath  string  `json:"file_path"`
	Content   string  `json:"content"`
	Level     string  `json:"level"`
	Score     float64 `json:"score"`
	StartLine int     `json:"start_line,omitempty"`
	EndLine   int     `json:"end_line,omitempty"`
}

// Daemon orchestrates the Pommel daemon, coordinating file watching,
// indexing, and API services.
type Daemon struct {
	projectRoot string
	config      *config.Config
	logger      *slog.Logger
	db          *db.DB
	embedder    embedder.Embedder
	indexer     *Indexer
	watcher     *Watcher
	server      *http.Server
	state       *StateManager
}

// New creates a new Daemon instance with all components initialized.
func New(projectRoot string, cfg *config.Config, logger *slog.Logger) (*Daemon, error) {
	// Validate projectRoot exists
	info, err := os.Stat(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("project root does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project root is not a directory: %s", projectRoot)
	}

	// Validate config is not nil
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Use a no-op logger if none provided
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	// Open database
	database, err := db.Open(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Run migrations
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create embedder (use MockEmbedder for now)
	baseEmbedder := embedder.NewMockEmbedder()

	// Create cached embedder
	cacheSize := cfg.Embedding.CacheSize
	if cacheSize <= 0 {
		cacheSize = 1000
	}
	cachedEmb := embedder.NewCachedEmbedder(baseEmbedder, cacheSize)

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

	// Create state manager
	state := NewStateManager(projectRoot)

	return &Daemon{
		projectRoot: projectRoot,
		config:      cfg,
		logger:      logger,
		db:          database,
		embedder:    cachedEmb,
		indexer:     indexer,
		watcher:     watcher,
		state:       state,
	}, nil
}

// Run starts the daemon and blocks until shutdown.
func (d *Daemon) Run(ctx context.Context) error {
	// Check if already running
	if running, pid := d.state.IsRunning(); running {
		return fmt.Errorf("daemon already running with PID %d", pid)
	}

	// Write PID file
	if err := d.state.WritePID(os.Getpid()); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Create a cancellable context for shutdown coordination
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start watcher
	if err := d.watcher.Start(runCtx); err != nil {
		d.cleanup()
		return fmt.Errorf("failed to start watcher: %w", err)
	}

	// Start API server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", d.handleHealth)
	mux.HandleFunc("/status", d.handleStatus)
	mux.HandleFunc("/search", d.handleSearch)
	mux.HandleFunc("/reindex", d.handleReindex)
	mux.HandleFunc("/config", d.handleConfig)

	addr := d.config.Daemon.Address()
	if d.config.Daemon.Port == 0 {
		// Use a random port for testing
		addr = fmt.Sprintf("%s:0", d.config.Daemon.Host)
	}

	d.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		d.logger.Info("starting API server", "addr", d.server.Addr)
		if err := d.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()

	// Process file events in goroutine
	go d.processFileEvents(runCtx)

	// Do initial index if database empty
	go d.initialIndexIfEmpty(runCtx)

	// Wait for shutdown signal or context cancel
	select {
	case <-ctx.Done():
		d.logger.Info("context cancelled, shutting down")
	case sig := <-sigCh:
		d.logger.Info("received signal, shutting down", "signal", sig)
	case err := <-serverErrCh:
		if err != nil {
			d.logger.Error("server error", "error", err)
			d.cleanup()
			return err
		}
	}

	// Trigger shutdown
	cancel()

	// Graceful shutdown
	return d.shutdown()
}

// HTTP Handlers

func (d *Daemon) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
	})
}

func (d *Daemon) handleStatus(w http.ResponseWriter, r *http.Request) {
	stats := d.indexer.Stats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"daemon": map[string]interface{}{
			"running": true,
			"pid":     os.Getpid(),
		},
		"index": map[string]interface{}{
			"total_files":     stats.TotalFiles,
			"total_chunks":    stats.TotalChunks,
			"indexing_active": stats.IndexingActive,
		},
	})
}

func (d *Daemon) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	resp, err := d.Search(r.Context(), req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (d *Daemon) handleReindex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	go func() {
		ctx := context.Background()
		_ = d.indexer.ReindexAll(ctx)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Reindexing started in background",
	})
}

func (d *Daemon) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"config": d.config,
	})
}

// processFileEvents handles file events from the watcher
func (d *Daemon) processFileEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-d.watcher.Events():
			d.handleFileEvent(ctx, event)
		case err := <-d.watcher.Errors():
			d.logger.Warn("watcher error", "error", err)
		}
	}
}

// handleFileEvent processes a single file event
func (d *Daemon) handleFileEvent(ctx context.Context, event FileEvent) {
	d.logger.Debug("processing file event", "path", event.Path, "op", event.Op)

	switch event.Op {
	case OpCreate, OpModify:
		if err := d.indexer.IndexFile(ctx, event.Path); err != nil {
			d.logger.Warn("failed to index file", "path", event.Path, "error", err)
		}
	case OpDelete, OpRename:
		if err := d.indexer.DeleteFile(ctx, event.Path); err != nil {
			d.logger.Warn("failed to delete file from index", "path", event.Path, "error", err)
		}
	}
}

// initialIndexIfEmpty runs initial indexing if the database is empty
func (d *Daemon) initialIndexIfEmpty(ctx context.Context) {
	fileCount, err := d.db.FileCount(ctx)
	if err != nil {
		d.logger.Warn("failed to get file count", "error", err)
		return
	}

	if fileCount == 0 {
		d.logger.Info("database empty, running initial index")
		if err := d.indexer.ReindexAll(ctx); err != nil {
			d.logger.Warn("initial indexing failed", "error", err)
		} else {
			d.logger.Info("initial indexing complete")
		}
	} else {
		d.logger.Info("database has data, skipping initial index", "files", fileCount)
	}
}

// shutdown performs graceful shutdown of all components
func (d *Daemon) shutdown() error {
	d.logger.Info("shutting down daemon")

	// Shutdown API server
	if d.server != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := d.server.Shutdown(shutdownCtx); err != nil {
			d.logger.Warn("server shutdown error", "error", err)
		}
	}

	// Stop watcher
	if d.watcher != nil {
		if err := d.watcher.Stop(); err != nil {
			d.logger.Warn("watcher stop error", "error", err)
		}
	}

	// Close database
	if d.db != nil {
		if err := d.db.Close(); err != nil {
			d.logger.Warn("database close error", "error", err)
		}
	}

	// Remove PID file
	if err := d.state.RemovePID(); err != nil {
		d.logger.Warn("failed to remove PID file", "error", err)
	}

	return nil
}

// cleanup is called on error before Run returns
func (d *Daemon) cleanup() {
	if d.watcher != nil {
		d.watcher.Stop()
	}
	if d.db != nil {
		d.db.Close()
	}
	d.state.RemovePID()
}

// Search performs a semantic search and returns matching chunks
func (d *Daemon) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	// Set defaults
	limit := req.Limit
	if limit <= 0 {
		limit = d.config.Search.DefaultLimit
	}

	// Get query embedding
	queryEmbedding, err := d.embedder.EmbedSingle(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Search for similar chunks
	vectorResults, err := d.db.SearchSimilar(ctx, queryEmbedding, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// Get chunk details
	chunkIDs := make([]string, len(vectorResults))
	distanceMap := make(map[string]float32)
	for i, vr := range vectorResults {
		chunkIDs[i] = vr.ChunkID
		distanceMap[vr.ChunkID] = vr.Distance
	}

	chunks, err := d.db.GetChunksByIDs(ctx, chunkIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunks: %w", err)
	}

	// Build response
	results := make([]SearchResult, 0, len(chunks))
	for _, chunk := range chunks {
		// Filter by levels if specified
		if len(req.Levels) > 0 {
			match := false
			for _, level := range req.Levels {
				if string(chunk.Level) == level {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		// Filter by path prefix if specified
		if req.PathPrefix != "" {
			if len(chunk.FilePath) < len(req.PathPrefix) || chunk.FilePath[:len(req.PathPrefix)] != req.PathPrefix {
				continue
			}
		}

		// Convert distance to score (lower distance = higher score)
		// Distance is typically between 0 and 2 for cosine distance
		distance := distanceMap[chunk.ID]
		score := 1.0 - float64(distance)/2.0
		if score < 0 {
			score = 0
		}

		results = append(results, SearchResult{
			ChunkID:   chunk.ID,
			FilePath:  chunk.FilePath,
			Content:   chunk.Content,
			Level:     string(chunk.Level),
			Score:     score,
			StartLine: chunk.StartLine,
			EndLine:   chunk.EndLine,
		})
	}

	return &SearchResponse{
		Results: results,
		Query:   req.Query,
		Limit:   limit,
	}, nil
}
