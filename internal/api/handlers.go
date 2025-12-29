package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/daemon"
)

// Handler handles HTTP requests for the Pommel API
type Handler struct {
	indexer   *daemon.Indexer
	config    *config.Config
	searcher  Searcher
	startTime time.Time
}

// Searcher defines the interface for search operations
type Searcher interface {
	Search(ctx context.Context, req SearchRequest) (*SearchResponse, error)
}

// NewHandler creates a new Handler instance
func NewHandler(indexer *daemon.Indexer, cfg *config.Config, searcher Searcher) *Handler {
	return &Handler{
		indexer:   indexer,
		config:    cfg,
		searcher:  searcher,
		startTime: time.Now(),
	}
}

// =============================================================================
// Response Types
// =============================================================================

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// StatusResponse represents the status endpoint response
type StatusResponse struct {
	Daemon       *DaemonStatus       `json:"daemon"`
	Index        *IndexStatus        `json:"index"`
	Dependencies *DependenciesStatus `json:"dependencies"`
}

// DaemonStatus contains daemon runtime information
type DaemonStatus struct {
	Running       bool    `json:"running"`
	PID           int     `json:"pid"`
	UptimeSeconds float64 `json:"uptime_seconds"`
}

// IndexStatus contains index statistics
type IndexStatus struct {
	TotalFiles     int64     `json:"total_files"`
	TotalChunks    int64     `json:"total_chunks"`
	LastIndexedAt  time.Time `json:"last_indexed_at,omitempty"`
	IndexingActive bool      `json:"indexing_active"`
}

// DependenciesStatus contains dependency availability information
type DependenciesStatus struct {
	Database  bool `json:"database"`
	Embedder  bool `json:"embedder"`
}

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

// ReindexResponse represents the reindex endpoint response
type ReindexResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ConfigResponse represents the config endpoint response
type ConfigResponse struct {
	Config *config.Config `json:"config"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// =============================================================================
// Handlers
// =============================================================================

// Health handles GET /health requests
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
	}
	writeJSON(w, http.StatusOK, response)
}

// Status handles GET /status requests
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	stats := h.indexer.Stats()

	response := StatusResponse{
		Daemon: &DaemonStatus{
			Running:       true,
			PID:           os.Getpid(),
			UptimeSeconds: time.Since(h.startTime).Seconds(),
		},
		Index: &IndexStatus{
			TotalFiles:     stats.TotalFiles,
			TotalChunks:    stats.TotalChunks,
			LastIndexedAt:  stats.LastIndexedAt,
			IndexingActive: stats.IndexingActive,
		},
		Dependencies: &DependenciesStatus{
			Database: true,
			Embedder: true,
		},
	}
	writeJSON(w, http.StatusOK, response)
}

// Search handles POST /search requests
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "query is required",
		})
		return
	}

	response, err := h.searcher.Search(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, response)
}

// Reindex handles POST /reindex requests
func (h *Handler) Reindex(w http.ResponseWriter, r *http.Request) {
	// Start reindexing in background
	go func() {
		ctx := context.Background()
		_ = h.indexer.ReindexAll(ctx)
	}()

	response := ReindexResponse{
		Status:  "started",
		Message: "Reindexing started in background",
	}
	writeJSON(w, http.StatusAccepted, response)
}

// Config handles GET /config requests
func (h *Handler) Config(w http.ResponseWriter, r *http.Request) {
	response := ConfigResponse{
		Config: h.config,
	}
	writeJSON(w, http.StatusOK, response)
}

// =============================================================================
// Helpers
// =============================================================================

// writeJSON writes a JSON response with the given status code
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
