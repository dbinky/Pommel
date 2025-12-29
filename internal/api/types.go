package api

import "time"

// =============================================================================
// Search API Types
// =============================================================================

// SearchRequest represents a search query request
type SearchRequest struct {
	Query      string   `json:"query"`
	Limit      int      `json:"limit,omitempty"`
	Levels     []string `json:"levels,omitempty"`
	PathPrefix string   `json:"path_prefix,omitempty"`
}

// SearchResponse represents the search results response
type SearchResponse struct {
	Query        string         `json:"query"`
	Results      []SearchResult `json:"results"`
	TotalResults int            `json:"total_results"`
	SearchTimeMs int64          `json:"search_time_ms"`
}

// SearchResult represents a single search result
type SearchResult struct {
	ID        string      `json:"id"`
	File      string      `json:"file"`
	StartLine int         `json:"start_line"`
	EndLine   int         `json:"end_line"`
	Level     string      `json:"level"`
	Language  string      `json:"language"`
	Name      string      `json:"name"`
	Score     float32     `json:"score"`
	Content   string      `json:"content"`
	Parent    *ParentInfo `json:"parent,omitempty"`
}

// ParentInfo provides information about a parent code element
type ParentInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level string `json:"level"`
}

// =============================================================================
// Status API Types
// =============================================================================

// StatusResponse represents the daemon and index status response
type StatusResponse struct {
	Daemon       *DaemonStatus       `json:"daemon"`
	Index        *IndexStatus        `json:"index"`
	Dependencies *DependenciesStatus `json:"dependencies"`
}

// DaemonStatus contains information about the daemon process
type DaemonStatus struct {
	Running       bool    `json:"running"`
	PID           int     `json:"pid"`
	UptimeSeconds float64 `json:"uptime_seconds"`
}

// IndexStatus contains information about the code index
type IndexStatus struct {
	TotalFiles     int64     `json:"total_files"`
	TotalChunks    int64     `json:"total_chunks"`
	LastIndexedAt  time.Time `json:"last_indexed_at,omitempty"`
	IndexingActive bool      `json:"indexing_active"`
	PendingChanges int       `json:"pending_changes"`
}

// DependenciesStatus contains information about external dependencies
type DependenciesStatus struct {
	Database bool `json:"database"`
	Embedder bool `json:"embedder"`
}

// =============================================================================
// Reindex API Types
// =============================================================================

// ReindexRequest represents a request to reindex the codebase
type ReindexRequest struct {
	Force bool   `json:"force"`
	Path  string `json:"path,omitempty"`
}

// ReindexResponse represents the reindex operation response
type ReindexResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// =============================================================================
// Error Types
// =============================================================================

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Error string `json:"error"`
}
