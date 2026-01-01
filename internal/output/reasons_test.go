package output

import (
	"testing"

	"github.com/pommel-dev/pommel/internal/api"
)

// ============================================================================
// 29.2 Match Reason Generation Tests
// ============================================================================

func TestGenerateMatchReasons_SemanticOnly(t *testing.T) {
	result := &api.SearchResult{
		MatchSource: "vector",
	}
	reasons := GenerateMatchReasons(result, "test query", nil)

	if len(reasons) == 0 {
		t.Fatal("Expected at least one reason")
	}

	hasSemanticReason := false
	for _, r := range reasons {
		if r == "semantic similarity" {
			hasSemanticReason = true
			break
		}
	}
	if !hasSemanticReason {
		t.Errorf("Expected 'semantic similarity' reason, got %v", reasons)
	}
}

func TestGenerateMatchReasons_KeywordOnly(t *testing.T) {
	result := &api.SearchResult{
		MatchSource: "keyword",
	}
	reasons := GenerateMatchReasons(result, "test query", nil)

	hasKeywordReason := false
	for _, r := range reasons {
		if r == "keyword match via BM25" {
			hasKeywordReason = true
			break
		}
	}
	if !hasKeywordReason {
		t.Errorf("Expected 'keyword match via BM25' reason, got %v", reasons)
	}
}

func TestGenerateMatchReasons_Both(t *testing.T) {
	result := &api.SearchResult{
		MatchSource: "both",
	}
	reasons := GenerateMatchReasons(result, "test query", nil)

	hasSemantic := false
	hasKeyword := false
	for _, r := range reasons {
		if r == "semantic similarity" {
			hasSemantic = true
		}
		if r == "keyword match via BM25" {
			hasKeyword = true
		}
	}
	if !hasSemantic || !hasKeyword {
		t.Errorf("Expected both semantic and keyword reasons, got %v", reasons)
	}
}

func TestGenerateMatchReasons_NameMatch(t *testing.T) {
	result := &api.SearchResult{
		Name:        "parseConfig",
		MatchSource: "vector",
	}
	reasons := GenerateMatchReasons(result, "parse config", nil)

	hasNameReason := false
	for _, r := range reasons {
		if containsAny(r, "name") {
			hasNameReason = true
			break
		}
	}
	if !hasNameReason {
		t.Errorf("Expected name match reason, got %v", reasons)
	}
}

func TestGenerateMatchReasons_PathMatch(t *testing.T) {
	result := &api.SearchResult{
		File:        "internal/config/parser.go",
		MatchSource: "vector",
	}
	reasons := GenerateMatchReasons(result, "config", nil)

	hasPathReason := false
	for _, r := range reasons {
		if containsAny(r, "path") {
			hasPathReason = true
			break
		}
	}
	if !hasPathReason {
		t.Errorf("Expected path match reason, got %v", reasons)
	}
}

func TestGenerateMatchReasons_ExactPhrase(t *testing.T) {
	result := &api.SearchResult{
		Content:     "This handles the authentication flow",
		MatchSource: "vector",
	}
	reasons := GenerateMatchReasons(result, "authentication flow", nil)

	hasExactPhraseReason := false
	for _, r := range reasons {
		if containsAny(r, "exact phrase") {
			hasExactPhraseReason = true
			break
		}
	}
	if !hasExactPhraseReason {
		t.Errorf("Expected exact phrase reason, got %v", reasons)
	}
}

func TestGenerateMatchReasons_NoMatch(t *testing.T) {
	result := &api.SearchResult{
		Content: "unrelated content",
	}
	reasons := GenerateMatchReasons(result, "something else", nil)

	// Should still have at least semantic similarity as default
	if len(reasons) == 0 {
		t.Error("Expected at least one default reason")
	}
}

func TestGenerateMatchReasons_EmptyQuery(t *testing.T) {
	result := &api.SearchResult{
		MatchSource: "vector",
	}
	reasons := GenerateMatchReasons(result, "", nil)

	if len(reasons) != 0 {
		t.Errorf("Expected no reasons for empty query, got %v", reasons)
	}
}

func TestGenerateMatchReasons_CaseInsensitive(t *testing.T) {
	result := &api.SearchResult{
		Name:        "ParseConfig",
		MatchSource: "vector",
	}
	reasons := GenerateMatchReasons(result, "PARSE", nil)

	hasNameReason := false
	for _, r := range reasons {
		if containsAny(r, "name") {
			hasNameReason = true
			break
		}
	}
	if !hasNameReason {
		t.Errorf("Expected case-insensitive name match, got %v", reasons)
	}
}

func TestGenerateMatchReasons_Deduplication(t *testing.T) {
	result := &api.SearchResult{
		MatchSource: "vector",
	}
	reasons := GenerateMatchReasons(result, "test", nil)

	// Check for duplicates
	seen := make(map[string]bool)
	for _, r := range reasons {
		if seen[r] {
			t.Errorf("Found duplicate reason: %s", r)
		}
		seen[r] = true
	}
}

func TestGenerateMatchReasons_MaxReasons(t *testing.T) {
	result := &api.SearchResult{
		Name:        "testMethod",
		File:        "test/test.go",
		Content:     "test code here",
		MatchSource: "both",
	}
	details := &api.ScoreDetails{
		SignalScores: map[string]float64{
			"name_match":   0.1,
			"path_match":   0.1,
			"exact_phrase": 0.1,
			"recency":      0.1,
			"chunk_type":   0.1,
		},
	}
	reasons := GenerateMatchReasons(result, "test", details)

	if len(reasons) > MaxReasons {
		t.Errorf("Expected at most %d reasons, got %d", MaxReasons, len(reasons))
	}
}

func TestGenerateMatchReasons_SignalBased(t *testing.T) {
	result := &api.SearchResult{
		MatchSource: "vector",
	}
	details := &api.ScoreDetails{
		SignalScores: map[string]float64{
			"recency": 0.1,
		},
	}
	reasons := GenerateMatchReasons(result, "test", details)

	hasRecencyReason := false
	for _, r := range reasons {
		if containsAny(r, "recent") {
			hasRecencyReason = true
			break
		}
	}
	if !hasRecencyReason {
		t.Errorf("Expected recency-based reason, got %v", reasons)
	}
}

// Helper function
func containsAny(s string, substrs ...string) bool {
	sLower := stringToLower(s)
	for _, sub := range substrs {
		if stringContains(sLower, stringToLower(sub)) {
			return true
		}
	}
	return false
}

func stringToLower(s string) string {
	return string(append([]byte{}, []byte(s)...))
}

func stringContains(s, substr string) bool {
	return len(substr) <= len(s) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
