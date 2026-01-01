package output

import (
	"fmt"
	"strings"

	"github.com/pommel-dev/pommel/internal/api"
)

// FormatMode specifies the output format
type FormatMode int

const (
	// FormatNormal is the standard compact output
	FormatNormal FormatMode = iota
	// FormatVerbose includes match reasons and score details
	FormatVerbose
	// FormatJSON outputs raw JSON
	FormatJSON
)

// Formatter handles search result output formatting
type Formatter struct {
	Mode       FormatMode
	ShowColors bool
	Query      string
}

// NewFormatter creates a formatter with the specified mode
func NewFormatter(mode FormatMode) *Formatter {
	return &Formatter{
		Mode:       mode,
		ShowColors: false, // Default to no colors for CLI compatibility
	}
}

// FormatResult formats a single search result
func (f *Formatter) FormatResult(result *api.SearchResult, index int) string {
	switch f.Mode {
	case FormatVerbose:
		return f.formatVerbose(result, index)
	case FormatJSON:
		// JSON formatting is handled at a higher level
		return ""
	default:
		return f.formatNormal(result, index)
	}
}

// formatNormal produces compact single-line output
func (f *Formatter) formatNormal(result *api.SearchResult, index int) string {
	// Format: [index] file:startLine-endLine (level) name [score]
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("[%d] ", index+1))
	sb.WriteString(result.File)
	sb.WriteString(fmt.Sprintf(":%d-%d", result.StartLine, result.EndLine))
	sb.WriteString(fmt.Sprintf(" (%s)", result.Level))

	if result.Name != "" {
		sb.WriteString(fmt.Sprintf(" %s", result.Name))
	}

	sb.WriteString(fmt.Sprintf(" [%.3f]", result.Score))

	return sb.String()
}

// formatVerbose produces detailed multi-line output
func (f *Formatter) formatVerbose(result *api.SearchResult, index int) string {
	var sb strings.Builder

	// Header line
	sb.WriteString(fmt.Sprintf("[%d] %s", index+1, result.File))
	sb.WriteString(fmt.Sprintf(":%d-%d", result.StartLine, result.EndLine))
	sb.WriteString("\n")

	// Details section
	sb.WriteString(fmt.Sprintf("    Level: %s", result.Level))
	if result.Name != "" {
		sb.WriteString(fmt.Sprintf(" | Name: %s", result.Name))
	}
	if result.Language != "" {
		sb.WriteString(fmt.Sprintf(" | Lang: %s", result.Language))
	}
	sb.WriteString("\n")

	// Score section
	sb.WriteString(fmt.Sprintf("    Score: %.4f", result.Score))

	if result.MatchSource != "" {
		sb.WriteString(fmt.Sprintf(" | Source: %s", result.MatchSource))
	}
	sb.WriteString("\n")

	// Score details if available
	if result.ScoreDetails != nil {
		sb.WriteString(f.formatScoreDetails(result.ScoreDetails))
	}

	// Match reasons if available
	if len(result.MatchReasons) > 0 {
		sb.WriteString(fmt.Sprintf("    Reasons: %s\n", strings.Join(result.MatchReasons, ", ")))
	}

	// Content preview (truncated)
	if result.Content != "" {
		preview := truncateContent(result.Content, 120)
		sb.WriteString(fmt.Sprintf("    Preview: %s\n", preview))
	}

	return sb.String()
}

// formatScoreDetails formats the detailed score breakdown
func (f *Formatter) formatScoreDetails(details *api.ScoreDetails) string {
	var sb strings.Builder
	var scores []string

	if details.VectorScore > 0 {
		scores = append(scores, fmt.Sprintf("vector=%.4f", details.VectorScore))
	}
	if details.KeywordScore > 0 {
		scores = append(scores, fmt.Sprintf("keyword=%.4f", details.KeywordScore))
	}
	if details.RRFScore > 0 {
		scores = append(scores, fmt.Sprintf("rrf=%.4f", details.RRFScore))
	}
	if details.RerankerScore > 0 {
		scores = append(scores, fmt.Sprintf("rerank=%.4f", details.RerankerScore))
	}

	if len(scores) > 0 {
		sb.WriteString(fmt.Sprintf("    Breakdown: %s\n", strings.Join(scores, " | ")))
	}

	// Signal scores
	if len(details.SignalScores) > 0 {
		var signals []string
		for name, score := range details.SignalScores {
			if score > 0.01 { // Only show significant signals
				signals = append(signals, fmt.Sprintf("%s=%.3f", name, score))
			}
		}
		if len(signals) > 0 {
			sb.WriteString(fmt.Sprintf("    Signals: %s\n", strings.Join(signals, " | ")))
		}
	}

	return sb.String()
}

// FormatSummary formats the search summary line
func (f *Formatter) FormatSummary(response *api.SearchResponse) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Found %d results", response.TotalResults))

	if f.Mode == FormatVerbose {
		sb.WriteString(fmt.Sprintf(" in %dms", response.SearchTimeMs))

		var flags []string
		if response.HybridEnabled {
			flags = append(flags, "hybrid")
		}
		if response.RerankEnabled {
			flags = append(flags, "reranked")
		}
		if len(flags) > 0 {
			sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(flags, ", ")))
		}
	}

	return sb.String()
}

// truncateContent truncates content to maxLen, adding ellipsis if needed
func truncateContent(content string, maxLen int) string {
	// Replace newlines with spaces for single-line preview
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\t", " ")

	// Collapse multiple spaces
	for strings.Contains(content, "  ") {
		content = strings.ReplaceAll(content, "  ", " ")
	}

	content = strings.TrimSpace(content)

	if len(content) <= maxLen {
		return content
	}

	return content[:maxLen-3] + "..."
}

// FormatMatchSource returns a human-readable match source description
func FormatMatchSource(source string) string {
	switch source {
	case "vector":
		return "semantic match"
	case "keyword":
		return "keyword match"
	case "both":
		return "semantic + keyword"
	default:
		return source
	}
}
