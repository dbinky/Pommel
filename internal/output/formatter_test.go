package output

import (
	"strings"
	"testing"

	"github.com/pommel-dev/pommel/internal/api"
)

// ============================================================================
// 29.3 Formatter Tests
// ============================================================================

func TestNewFormatter(t *testing.T) {
	f := NewFormatter(FormatNormal)
	if f.Mode != FormatNormal {
		t.Errorf("Expected FormatNormal, got %v", f.Mode)
	}

	f = NewFormatter(FormatVerbose)
	if f.Mode != FormatVerbose {
		t.Errorf("Expected FormatVerbose, got %v", f.Mode)
	}
}

func TestFormatResult_Normal(t *testing.T) {
	f := NewFormatter(FormatNormal)

	result := &api.SearchResult{
		File:      "internal/cli/search.go",
		StartLine: 42,
		EndLine:   58,
		Level:     "function",
		Name:      "executeSearch",
		Score:     0.8765,
	}

	output := f.FormatResult(result, 0)

	// Should contain key elements
	if !strings.Contains(output, "[1]") {
		t.Error("Expected index [1] in output")
	}
	if !strings.Contains(output, "internal/cli/search.go") {
		t.Error("Expected file path in output")
	}
	if !strings.Contains(output, ":42-58") {
		t.Error("Expected line range in output")
	}
	if !strings.Contains(output, "(function)") {
		t.Error("Expected level in output")
	}
	if !strings.Contains(output, "executeSearch") {
		t.Error("Expected name in output")
	}
	if !strings.Contains(output, "0.876") || !strings.Contains(output, "0.877") {
		// Allow for rounding
		if !strings.Contains(output, "[0.") {
			t.Errorf("Expected score in output, got: %s", output)
		}
	}
}

func TestFormatResult_Normal_NoName(t *testing.T) {
	f := NewFormatter(FormatNormal)

	result := &api.SearchResult{
		File:      "README.md",
		StartLine: 1,
		EndLine:   50,
		Level:     "file",
		Score:     0.5,
	}

	output := f.FormatResult(result, 2)

	if !strings.Contains(output, "[3]") {
		t.Error("Expected index [3] in output")
	}
	if !strings.Contains(output, "(file)") {
		t.Error("Expected level in output")
	}
}

func TestFormatResult_Verbose(t *testing.T) {
	f := NewFormatter(FormatVerbose)

	result := &api.SearchResult{
		File:         "internal/cli/search.go",
		StartLine:    42,
		EndLine:      58,
		Level:        "function",
		Name:         "executeSearch",
		Language:     "go",
		Score:        0.8765,
		MatchSource:  "both",
		Content:      "func executeSearch(cmd *cobra.Command, args []string) error {\n\treturn nil\n}",
		MatchReasons: []string{"semantic similarity", "keyword match via BM25"},
		ScoreDetails: &api.ScoreDetails{
			VectorScore:  0.82,
			KeywordScore: 0.75,
			RRFScore:     0.87,
		},
	}

	output := f.FormatResult(result, 0)

	// Should be multi-line
	lines := strings.Split(output, "\n")
	if len(lines) < 4 {
		t.Errorf("Expected at least 4 lines in verbose output, got %d", len(lines))
	}

	// Should contain detailed info
	if !strings.Contains(output, "Level: function") {
		t.Error("Expected level details")
	}
	if !strings.Contains(output, "Name: executeSearch") {
		t.Error("Expected name details")
	}
	if !strings.Contains(output, "Lang: go") {
		t.Error("Expected language")
	}
	if !strings.Contains(output, "Source: both") {
		t.Error("Expected match source")
	}
	if !strings.Contains(output, "Breakdown:") {
		t.Error("Expected score breakdown")
	}
	if !strings.Contains(output, "Reasons:") {
		t.Error("Expected match reasons")
	}
	if !strings.Contains(output, "Preview:") {
		t.Error("Expected content preview")
	}
}

func TestFormatResult_Verbose_NoOptionalFields(t *testing.T) {
	f := NewFormatter(FormatVerbose)

	result := &api.SearchResult{
		File:      "test.go",
		StartLine: 1,
		EndLine:   10,
		Level:     "file",
		Score:     0.5,
	}

	output := f.FormatResult(result, 0)

	// Should not error with missing optional fields
	if !strings.Contains(output, "Score:") {
		t.Error("Expected basic score info")
	}
	// Should not have breakdown without details
	if strings.Contains(output, "Breakdown:") {
		t.Error("Should not have breakdown without score details")
	}
}

func TestFormatScoreDetails(t *testing.T) {
	f := NewFormatter(FormatVerbose)

	details := &api.ScoreDetails{
		VectorScore:   0.85,
		KeywordScore:  0.72,
		RRFScore:      0.90,
		RerankerScore: 0.88,
		SignalScores: map[string]float64{
			"name_match": 0.15,
			"recency":    0.08,
		},
	}

	output := f.formatScoreDetails(details)

	if !strings.Contains(output, "vector=0.8500") {
		t.Error("Expected vector score")
	}
	if !strings.Contains(output, "keyword=0.7200") {
		t.Error("Expected keyword score")
	}
	if !strings.Contains(output, "rrf=0.9000") {
		t.Error("Expected RRF score")
	}
	if !strings.Contains(output, "rerank=0.8800") {
		t.Error("Expected reranker score")
	}
	if !strings.Contains(output, "Signals:") {
		t.Error("Expected signals section")
	}
	if !strings.Contains(output, "name_match") {
		t.Error("Expected name_match signal")
	}
}

func TestFormatScoreDetails_EmptySignals(t *testing.T) {
	f := NewFormatter(FormatVerbose)

	details := &api.ScoreDetails{
		VectorScore: 0.85,
	}

	output := f.formatScoreDetails(details)

	if strings.Contains(output, "Signals:") {
		t.Error("Should not have signals section when empty")
	}
}

func TestFormatScoreDetails_LowSignalsFiltered(t *testing.T) {
	f := NewFormatter(FormatVerbose)

	details := &api.ScoreDetails{
		SignalScores: map[string]float64{
			"name_match": 0.001, // Below threshold
			"recency":    0.005, // Below threshold
		},
	}

	output := f.formatScoreDetails(details)

	// Should filter out low signals
	if strings.Contains(output, "Signals:") {
		t.Error("Should not show signals below threshold")
	}
}

func TestFormatSummary_Normal(t *testing.T) {
	f := NewFormatter(FormatNormal)

	response := &api.SearchResponse{
		TotalResults:  15,
		SearchTimeMs:  42,
		HybridEnabled: true,
		RerankEnabled: true,
	}

	output := f.FormatSummary(response)

	if !strings.Contains(output, "Found 15 results") {
		t.Errorf("Expected result count, got: %s", output)
	}
	// Normal mode should not include timing or flags
	if strings.Contains(output, "42ms") {
		t.Error("Normal mode should not show timing")
	}
}

func TestFormatSummary_Verbose(t *testing.T) {
	f := NewFormatter(FormatVerbose)

	response := &api.SearchResponse{
		TotalResults:  15,
		SearchTimeMs:  42,
		HybridEnabled: true,
		RerankEnabled: true,
	}

	output := f.FormatSummary(response)

	if !strings.Contains(output, "Found 15 results") {
		t.Error("Expected result count")
	}
	if !strings.Contains(output, "42ms") {
		t.Error("Verbose mode should show timing")
	}
	if !strings.Contains(output, "hybrid") {
		t.Error("Should indicate hybrid search")
	}
	if !strings.Contains(output, "reranked") {
		t.Error("Should indicate reranking")
	}
}

func TestFormatSummary_VerboseNoFlags(t *testing.T) {
	f := NewFormatter(FormatVerbose)

	response := &api.SearchResponse{
		TotalResults:  5,
		SearchTimeMs:  100,
		HybridEnabled: false,
		RerankEnabled: false,
	}

	output := f.FormatSummary(response)

	if strings.Contains(output, "(hybrid") || strings.Contains(output, "(reranked") {
		t.Error("Should not show flags when disabled")
	}
}

func TestTruncateContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		maxLen   int
		expected string
	}{
		{
			name:     "short content",
			content:  "hello world",
			maxLen:   20,
			expected: "hello world",
		},
		{
			name:     "exact length",
			content:  "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "truncated",
			content:  "this is a very long string that needs truncation",
			maxLen:   20,
			expected: "this is a very lo...",
		},
		{
			name:     "newlines collapsed",
			content:  "line1\nline2\nline3",
			maxLen:   50,
			expected: "line1 line2 line3",
		},
		{
			name:     "tabs collapsed",
			content:  "hello\tworld",
			maxLen:   20,
			expected: "hello world",
		},
		{
			name:     "multiple spaces collapsed",
			content:  "hello    world",
			maxLen:   20,
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateContent(tt.content, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatMatchSource(t *testing.T) {
	tests := []struct {
		source   string
		expected string
	}{
		{"vector", "semantic match"},
		{"keyword", "keyword match"},
		{"both", "semantic + keyword"},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			result := FormatMatchSource(tt.source)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatResult_JSON_ReturnsEmpty(t *testing.T) {
	f := NewFormatter(FormatJSON)

	result := &api.SearchResult{
		File:  "test.go",
		Score: 0.5,
	}

	output := f.FormatResult(result, 0)

	// JSON mode returns empty as it's handled at higher level
	if output != "" {
		t.Errorf("Expected empty string for JSON mode, got: %s", output)
	}
}

func TestFormatter_IndexNumbering(t *testing.T) {
	f := NewFormatter(FormatNormal)

	result := &api.SearchResult{
		File:      "test.go",
		StartLine: 1,
		EndLine:   10,
		Level:     "file",
		Score:     0.5,
	}

	// Test that indices are 1-based for display
	for i := 0; i < 5; i++ {
		output := f.FormatResult(result, i)
		expectedIdx := i + 1
		expectedStr := "[" + string(rune('0'+expectedIdx)) + "]"
		if !strings.Contains(output, expectedStr) {
			t.Errorf("Expected index [%d] at position %d, got: %s", expectedIdx, i, output)
		}
	}
}
