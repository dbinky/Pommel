package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/daemon"
	"github.com/pommel-dev/pommel/internal/search"
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

// NOTE: StatusResponse, DaemonStatus, IndexStatus, DependenciesStatus,
// SearchRequest, SearchResponse, SearchResult, ParentInfo, ReindexResponse,
// and ErrorResponse types are defined in types.go

// ConfigResponse represents the config endpoint response
type ConfigResponse struct {
	Config *config.Config `json:"config"`
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
		WriteBadRequest(w, ErrInvalidJSON.WithDetails(err.Error()))
		return
	}

	// Trim whitespace from query before validation
	req.Query = strings.TrimSpace(req.Query)

	if req.Query == "" {
		WriteBadRequest(w, ErrQueryEmpty)
		return
	}

	response, err := h.searcher.Search(r.Context(), req)
	if err != nil {
		WriteInternalError(w, ErrSearchFailed.WithDetails(err.Error()))
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

// =============================================================================
// Search Service Adapter
// =============================================================================

// SearchServiceAdapter adapts search.Service to implement the Searcher interface.
type SearchServiceAdapter struct {
	service *search.Service
}

// NewSearchServiceAdapter creates a new adapter for the given search service.
func NewSearchServiceAdapter(service *search.Service) *SearchServiceAdapter {
	return &SearchServiceAdapter{service: service}
}

// Search implements the Searcher interface by delegating to the search service.
func (a *SearchServiceAdapter) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	// Convert SearchRequest to search.Query
	query := search.Query{
		Text:       req.Query,
		Limit:      req.Limit,
		Levels:     req.Levels,
		PathPrefix: req.PathPrefix,
	}

	// Call search service
	resp, err := a.service.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	// Convert search.Response to SearchResponse
	results := make([]SearchResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		result := SearchResult{
			ID:            r.Chunk.ID,
			File:          r.Chunk.FilePath,
			StartLine:     r.Chunk.StartLine,
			EndLine:       r.Chunk.EndLine,
			Level:         string(r.Chunk.Level),
			Language:      r.Chunk.Language,
			Name:          r.Chunk.Name,
			Score:         r.Score,
			Content:       r.Chunk.Content,
			MatchedSplits: r.MatchedSplits,
		}

		// Convert parent info if present
		if r.Parent != nil {
			result.Parent = &ParentInfo{
				ID:    r.Parent.ID,
				Name:  r.Parent.Name,
				Level: r.Parent.Level,
			}
		}

		results = append(results, result)
	}

	return &SearchResponse{
		Query:        resp.Query,
		Results:      results,
		TotalResults: resp.TotalResults,
		SearchTimeMs: resp.SearchTimeMs,
	}, nil
}
