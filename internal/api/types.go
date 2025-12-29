package api

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
