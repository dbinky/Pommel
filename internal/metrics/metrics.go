package metrics

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/pommel-dev/pommel/internal/api"
)

// SearchMetrics contains metrics for a search operation
type SearchMetrics struct {
	ResultCount  int   // Number of results returned
	TotalTokens  int   // Estimated total tokens in results
	TotalLines   int   // Total lines across all results
	SearchTimeMs int64 // Search time in milliseconds
}

// BaselineEstimate contains estimated metrics for a grep-based approach
type BaselineEstimate struct {
	TotalLines      int // Total lines that would need to be read
	EstimatedTokens int // Estimated tokens for all those lines
	FileCount       int // Number of files involved
}

// ContextSavings contains context window savings calculation
type ContextSavings struct {
	TokensSaved  int     // Tokens saved vs baseline
	PercentSaved float64 // Percentage of context saved
}

// tokensPerLine is the estimated tokens per line of code
// Based on ~40 chars per line average
const tokensPerLine = 10

// EstimateTokens estimates the number of tokens in a string
// Uses a simple heuristic: count words and add extra for punctuation
func EstimateTokens(content string) int {
	if content == "" {
		return 0
	}

	// Count words
	words := 0
	inWord := false
	for _, r := range content {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			if !inWord {
				words++
				inWord = true
			}
		} else {
			inWord = false
		}
	}

	// Add tokens for punctuation (roughly 30% extra for code)
	punctCount := 0
	for _, r := range content {
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			punctCount++
		}
	}

	// Each punctuation/symbol is roughly 0.5 tokens on average
	punctTokens := punctCount / 2

	return words + punctTokens
}

// FromSearchResults creates metrics from search results
func FromSearchResults(results []api.SearchResult, searchTime time.Duration) *SearchMetrics {
	metrics := &SearchMetrics{
		ResultCount:  len(results),
		SearchTimeMs: searchTime.Milliseconds(),
	}

	for _, r := range results {
		metrics.TotalTokens += EstimateTokens(r.Content)
		metrics.TotalLines += (r.EndLine - r.StartLine + 1)
	}

	return metrics
}

// EstimateGrepBaseline estimates what searching with grep would require
func EstimateGrepBaseline(fileCount, avgLinesPerFile int) *BaselineEstimate {
	totalLines := fileCount * avgLinesPerFile

	return &BaselineEstimate{
		TotalLines:      totalLines,
		EstimatedTokens: totalLines * tokensPerLine,
		FileCount:       fileCount,
	}
}

// CalculateSavings calculates context window savings
func CalculateSavings(pommelTokens, baselineTokens int) ContextSavings {
	if baselineTokens == 0 {
		return ContextSavings{TokensSaved: 0, PercentSaved: 0}
	}

	saved := baselineTokens - pommelTokens
	if saved < 0 {
		return ContextSavings{TokensSaved: 0, PercentSaved: 0}
	}

	percent := float64(saved) / float64(baselineTokens) * 100

	return ContextSavings{
		TokensSaved:  saved,
		PercentSaved: percent,
	}
}

// FormatMetrics formats search metrics for display
func FormatMetrics(m *SearchMetrics) string {
	var sb strings.Builder

	sb.WriteString("Search Metrics:\n")
	sb.WriteString(fmt.Sprintf("  Results: %d\n", m.ResultCount))
	sb.WriteString(fmt.Sprintf("  Total Lines: %d\n", m.TotalLines))
	sb.WriteString(fmt.Sprintf("  Estimated Tokens: %d\n", m.TotalTokens))
	sb.WriteString(fmt.Sprintf("  Search Time: %dms\n", m.SearchTimeMs))

	return sb.String()
}

// FormatMetricsWithComparison formats metrics with baseline comparison
func FormatMetricsWithComparison(m *SearchMetrics, baseline *BaselineEstimate, savings ContextSavings) string {
	var sb strings.Builder

	sb.WriteString("Search Metrics:\n")
	sb.WriteString(fmt.Sprintf("  Results: %d\n", m.ResultCount))
	sb.WriteString(fmt.Sprintf("  Pommel Tokens: %d\n", m.TotalTokens))
	sb.WriteString(fmt.Sprintf("  Search Time: %dms\n", m.SearchTimeMs))
	sb.WriteString("\n")
	sb.WriteString("Baseline Comparison:\n")
	sb.WriteString(fmt.Sprintf("  Grep would search: %d files, %d lines\n", baseline.FileCount, baseline.TotalLines))
	sb.WriteString(fmt.Sprintf("  Grep tokens (est.): %d\n", baseline.EstimatedTokens))
	sb.WriteString("\n")
	sb.WriteString("Context Savings:\n")
	sb.WriteString(fmt.Sprintf("  Tokens Saved: %d (%.1f%%)\n", savings.TokensSaved, savings.PercentSaved))

	return sb.String()
}

// FormatMetricsSummary returns a compact one-line summary
func FormatMetricsSummary(m *SearchMetrics, savings *ContextSavings) string {
	if savings != nil && savings.PercentSaved > 0 {
		return fmt.Sprintf("%d results, %d tokens (%.0f%% context saved), %dms",
			m.ResultCount, m.TotalTokens, savings.PercentSaved, m.SearchTimeMs)
	}
	return fmt.Sprintf("%d results, %d tokens, %dms",
		m.ResultCount, m.TotalTokens, m.SearchTimeMs)
}
