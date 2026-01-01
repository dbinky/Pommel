package output

import (
	"strings"

	"github.com/pommel-dev/pommel/internal/api"
)

// MaxReasons is the maximum number of reasons to return
const MaxReasons = 5

// GenerateMatchReasons creates human-readable explanations of why a result matched
func GenerateMatchReasons(result *api.SearchResult, query string, details *api.ScoreDetails) []string {
	reasons := []string{}

	if query == "" {
		return reasons
	}

	queryTerms := extractTerms(query)

	// Check match source
	if result.MatchSource == "vector" || result.MatchSource == "both" {
		reasons = append(reasons, "semantic similarity")
	}

	if result.MatchSource == "keyword" || result.MatchSource == "both" {
		reasons = append(reasons, "keyword match via BM25")
	}

	// Check for name matches
	if result.Name != "" {
		for _, term := range queryTerms {
			if strings.Contains(strings.ToLower(result.Name), term) {
				reasons = append(reasons, "contains '"+term+"' in name")
				break // Only add one name reason
			}
		}
	}

	// Check for path matches
	if result.File != "" {
		for _, term := range queryTerms {
			if strings.Contains(strings.ToLower(result.File), term) {
				reasons = append(reasons, "file path contains '"+term+"'")
				break // Only add one path reason
			}
		}
	}

	// Check for exact phrase match
	queryLower := strings.ToLower(query)
	if result.Content != "" && strings.Contains(strings.ToLower(result.Content), queryLower) {
		reasons = append(reasons, "exact phrase match")
	}

	// Use signal scores if available
	if details != nil && details.SignalScores != nil {
		for signal, score := range details.SignalScores {
			if score > 0.05 { // Only significant signals
				reasons = append(reasons, formatSignalReason(signal))
			}
		}
	}

	// Deduplicate and limit
	reasons = deduplicateReasons(reasons)
	if len(reasons) > MaxReasons {
		reasons = reasons[:MaxReasons]
	}

	// Default reason if nothing else
	if len(reasons) == 0 {
		reasons = append(reasons, "semantic similarity")
	}

	return reasons
}

// extractTerms splits a query into lowercase terms
func extractTerms(query string) []string {
	words := strings.Fields(query)
	terms := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()[]{}|")
		if len(w) > 1 {
			terms = append(terms, strings.ToLower(w))
		}
	}
	return terms
}

// formatSignalReason converts a signal name to a human-readable reason
func formatSignalReason(signal string) string {
	switch signal {
	case "name_match":
		return "name relevance boost"
	case "path_match":
		return "path relevance boost"
	case "exact_phrase":
		return "exact phrase boost"
	case "recency":
		return "recently modified"
	case "chunk_type":
		return "chunk type relevance"
	case "test_penalty":
		return "" // Don't show penalty as reason
	default:
		return signal + " signal"
	}
}

// deduplicateReasons removes duplicate reasons
func deduplicateReasons(reasons []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(reasons))
	for _, r := range reasons {
		if r == "" {
			continue
		}
		lower := strings.ToLower(r)
		if !seen[lower] {
			seen[lower] = true
			result = append(result, r)
		}
	}
	return result
}
