package api

import "time"

// =============================================================================
// Search API Types
// =============================================================================

// SearchRequest represents a search query request
type SearchRequest struct {
	Query         string             `json:"query"`
	Limit         int                `json:"limit,omitempty"`
	Levels        []string           `json:"levels,omitempty"`
	PathPrefix    string             `json:"path_prefix,omitempty"`
	Scope         SearchScopeRequest `json:"scope,omitempty"`
	HybridEnabled *bool              `json:"hybrid_enabled,omitempty"` // nil = use config default, true/false = explicit
	RerankEnabled *bool              `json:"rerank_enabled,omitempty"` // nil = use config default, true/false = explicit
}

// SearchScopeRequest specifies the search scope in the request
type SearchScopeRequest struct {
	Mode  string `json:"mode"`            // "all", "path", "subproject", "auto"
	Value string `json:"value,omitempty"` // path prefix or subproject ID
}

// SearchResponse represents the search results response
type SearchResponse struct {
	Query         string               `json:"query"`
	Results       []SearchResult       `json:"results"`
	TotalResults  int                  `json:"total_results"`
	SearchTimeMs  int64                `json:"search_time_ms"`
	Scope         *SearchScopeResponse `json:"scope,omitempty"`
	HybridEnabled bool                 `json:"hybrid_enabled"`
	RerankEnabled bool                 `json:"rerank_enabled"`
}

// SearchScopeResponse provides scope information in the response
type SearchScopeResponse struct {
	Mode         string  `json:"mode"`
	Subproject   *string `json:"subproject,omitempty"`
	ResolvedPath *string `json:"resolved_path,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	ID            string        `json:"id"`
	File          string        `json:"file"`
	StartLine     int           `json:"start_line"`
	EndLine       int           `json:"end_line"`
	Level         string        `json:"level"`
	Language      string        `json:"language"`
	Name          string        `json:"name"`
	Score         float32       `json:"score"`
	Content       string        `json:"content"`
	Parent        *ParentInfo   `json:"parent,omitempty"`
	MatchSource   string        `json:"match_source,omitempty"`   // "vector", "keyword", or "both"
	ScoreDetails  *ScoreDetails `json:"score_details,omitempty"`  // Detailed score breakdown
	MatchReasons  []string      `json:"match_reasons,omitempty"`  // Human-readable match reasons
	MatchedSplits int           `json:"matched_splits,omitempty"` // Number of chunk splits that matched (for boosted results)
}

// ScoreDetails contains detailed score breakdown for a search result
type ScoreDetails struct {
	VectorScore   float64            `json:"vector_score,omitempty"`
	KeywordScore  float64            `json:"keyword_score,omitempty"`
	RRFScore      float64            `json:"rrf_score,omitempty"`
	RerankerScore float64            `json:"reranker_score,omitempty"`
	SignalScores  map[string]float64 `json:"signal_scores,omitempty"`
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

	// Progress tracking (only populated when indexing is active)
	Progress *IndexProgress `json:"progress,omitempty"`
}

// IndexProgress contains progress information for ongoing indexing
type IndexProgress struct {
	FilesToProcess  int64     `json:"files_to_process"`
	FilesProcessed  int64     `json:"files_processed"`
	PercentComplete float64   `json:"percent_complete"`
	IndexingStarted time.Time `json:"indexing_started"`
	ETASeconds      float64   `json:"eta_seconds,omitempty"` // Estimated seconds remaining
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
// Subprojects API Types
// =============================================================================

// SubprojectsResponse represents the list of sub-projects
type SubprojectsResponse struct {
	Subprojects []SubprojectInfo `json:"subprojects"`
	Total       int              `json:"total"`
}

// SubprojectInfo contains information about a sub-project
type SubprojectInfo struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Name       string `json:"name,omitempty"`
	MarkerFile string `json:"marker_file"`
	Language   string `json:"language,omitempty"`
}

// =============================================================================
// Error Types
// =============================================================================

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Error string `json:"error"`
}
