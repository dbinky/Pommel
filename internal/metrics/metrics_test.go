package metrics

import (
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/api"
)

// ============================================================================
// 30.1 Token Estimation Tests
// ============================================================================

func TestEstimateTokens_Simple(t *testing.T) {
	content := "hello world" // 2 words
	tokens := EstimateTokens(content)

	// Tokens ~= words * 1.3 (roughly)
	if tokens < 2 || tokens > 5 {
		t.Errorf("Expected 2-5 tokens for '%s', got %d", content, tokens)
	}
}

func TestEstimateTokens_CodeContent(t *testing.T) {
	content := `func handleSearch(ctx context.Context, query string) error {
		results, err := db.Search(query)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}
		return nil
	}`

	tokens := EstimateTokens(content)

	// Code tends to have more tokens due to punctuation
	if tokens < 30 || tokens > 80 {
		t.Errorf("Expected 30-80 tokens for code content, got %d", tokens)
	}
}

func TestEstimateTokens_Empty(t *testing.T) {
	tokens := EstimateTokens("")
	if tokens != 0 {
		t.Errorf("Expected 0 tokens for empty string, got %d", tokens)
	}
}

// ============================================================================
// 30.2 SearchMetrics Tests
// ============================================================================

func TestSearchMetrics_FromResults(t *testing.T) {
	results := []api.SearchResult{
		{
			Content:   "func foo() {}",
			StartLine: 1,
			EndLine:   3,
		},
		{
			Content:   "func bar() { return nil }",
			StartLine: 10,
			EndLine:   15,
		},
	}

	metrics := FromSearchResults(results, 25*time.Millisecond)

	if metrics.ResultCount != 2 {
		t.Errorf("Expected 2 results, got %d", metrics.ResultCount)
	}

	if metrics.TotalLines < 6 {
		t.Errorf("Expected at least 6 total lines, got %d", metrics.TotalLines)
	}

	if metrics.SearchTimeMs != 25 {
		t.Errorf("Expected 25ms search time, got %d", metrics.SearchTimeMs)
	}
}

func TestSearchMetrics_TokenCount(t *testing.T) {
	results := []api.SearchResult{
		{Content: "hello world"},
		{Content: "goodbye world"},
	}

	metrics := FromSearchResults(results, 10*time.Millisecond)

	if metrics.TotalTokens < 4 {
		t.Errorf("Expected at least 4 tokens, got %d", metrics.TotalTokens)
	}
}

func TestSearchMetrics_EmptyResults(t *testing.T) {
	metrics := FromSearchResults(nil, 5*time.Millisecond)

	if metrics.ResultCount != 0 {
		t.Errorf("Expected 0 results, got %d", metrics.ResultCount)
	}
	if metrics.TotalTokens != 0 {
		t.Errorf("Expected 0 tokens, got %d", metrics.TotalTokens)
	}
	if metrics.TotalLines != 0 {
		t.Errorf("Expected 0 lines, got %d", metrics.TotalLines)
	}
}

// ============================================================================
// 30.3 Baseline Comparison Tests
// ============================================================================

func TestBaselineEstimate_SmallProject(t *testing.T) {
	baseline := EstimateGrepBaseline(100, 50) // 100 files, 50 lines average

	// 100 files * 50 lines = 5000 lines
	if baseline.TotalLines < 4000 || baseline.TotalLines > 6000 {
		t.Errorf("Expected ~5000 total lines, got %d", baseline.TotalLines)
	}

	// Token estimate should be significant
	if baseline.EstimatedTokens < 10000 {
		t.Errorf("Expected at least 10000 tokens, got %d", baseline.EstimatedTokens)
	}
}

func TestBaselineEstimate_LargeProject(t *testing.T) {
	baseline := EstimateGrepBaseline(1000, 100) // 1000 files, 100 lines average

	// Should be very large
	if baseline.TotalLines < 90000 {
		t.Errorf("Expected at least 90000 total lines, got %d", baseline.TotalLines)
	}
}

func TestBaselineEstimate_NoFiles(t *testing.T) {
	baseline := EstimateGrepBaseline(0, 50)

	if baseline.TotalLines != 0 {
		t.Errorf("Expected 0 lines for 0 files, got %d", baseline.TotalLines)
	}
}

// ============================================================================
// 30.4 Context Savings Calculation Tests
// ============================================================================

func TestCalculateSavings_Basic(t *testing.T) {
	pommelTokens := 500
	baselineTokens := 10000

	savings := CalculateSavings(pommelTokens, baselineTokens)

	// Should save 95%
	if savings.PercentSaved < 90 || savings.PercentSaved > 100 {
		t.Errorf("Expected ~95%% savings, got %.1f%%", savings.PercentSaved)
	}

	if savings.TokensSaved != 9500 {
		t.Errorf("Expected 9500 tokens saved, got %d", savings.TokensSaved)
	}
}

func TestCalculateSavings_NoSavings(t *testing.T) {
	pommelTokens := 10000
	baselineTokens := 10000

	savings := CalculateSavings(pommelTokens, baselineTokens)

	if savings.PercentSaved != 0 {
		t.Errorf("Expected 0%% savings, got %.1f%%", savings.PercentSaved)
	}
}

func TestCalculateSavings_ZeroBaseline(t *testing.T) {
	savings := CalculateSavings(100, 0)

	if savings.PercentSaved != 0 {
		t.Errorf("Expected 0%% savings with zero baseline, got %.1f%%", savings.PercentSaved)
	}
}

func TestCalculateSavings_NegativeSavings(t *testing.T) {
	// If Pommel uses more tokens than baseline (shouldn't happen normally)
	pommelTokens := 15000
	baselineTokens := 10000

	savings := CalculateSavings(pommelTokens, baselineTokens)

	// Should report 0% savings (not negative)
	if savings.PercentSaved != 0 {
		t.Errorf("Expected 0%% when no savings, got %.1f%%", savings.PercentSaved)
	}
}

// ============================================================================
// 30.5 Metrics Formatting Tests
// ============================================================================

func TestFormatMetrics_Basic(t *testing.T) {
	metrics := &SearchMetrics{
		ResultCount:  5,
		TotalTokens:  200,
		TotalLines:   50,
		SearchTimeMs: 15,
	}

	output := FormatMetrics(metrics)

	if output == "" {
		t.Error("Expected non-empty output")
	}

	// Should contain key info
	if !containsString(output, "5") {
		t.Error("Expected result count in output")
	}
	if !containsString(output, "200") {
		t.Error("Expected token count in output")
	}
}

func TestFormatMetricsWithComparison_Basic(t *testing.T) {
	metrics := &SearchMetrics{
		ResultCount:  5,
		TotalTokens:  200,
		TotalLines:   50,
		SearchTimeMs: 15,
	}

	baseline := &BaselineEstimate{
		TotalLines:      5000,
		EstimatedTokens: 10000,
	}

	savings := CalculateSavings(metrics.TotalTokens, baseline.EstimatedTokens)

	output := FormatMetricsWithComparison(metrics, baseline, savings)

	if output == "" {
		t.Error("Expected non-empty output")
	}

	// Should contain savings percentage
	if !containsString(output, "%") {
		t.Error("Expected savings percentage in output")
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstr(s, substr) >= 0
}

func findSubstr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
