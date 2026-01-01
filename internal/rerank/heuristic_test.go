package rerank

import (
	"context"
	"testing"
	"time"
)

// ============================================================================
// 28.3 Heuristic Re-ranker Tests
// ============================================================================

func TestHeuristicReranker_Name(t *testing.T) {
	r := NewHeuristicReranker()
	if r.Name() != "heuristic" {
		t.Errorf("Expected name 'heuristic', got '%s'", r.Name())
	}
}

func TestHeuristicReranker_Available(t *testing.T) {
	r := NewHeuristicReranker()
	if !r.Available(context.Background()) {
		t.Error("Heuristic reranker should always be available")
	}
}

func TestHeuristicReranker_EmptyCandidates(t *testing.T) {
	r := NewHeuristicReranker()
	results, err := r.Rerank(context.Background(), "test query", []Candidate{})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected empty results, got %d", len(results))
	}
}

func TestHeuristicReranker_SingleCandidate(t *testing.T) {
	r := NewHeuristicReranker()
	candidates := []Candidate{
		{
			ChunkID:   "chunk-1",
			Content:   "func parseConfig() {}",
			Name:      "parseConfig",
			FilePath:  "config.go",
			ChunkType: "function",
			BaseScore: 0.8,
		},
	}

	results, err := r.Rerank(context.Background(), "parseConfig", candidates)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].ChunkID != "chunk-1" {
		t.Error("Wrong result returned")
	}
	if results[0].FinalScore == 0 {
		t.Error("FinalScore should be set")
	}
}

func TestHeuristicReranker_MultipleCandidates(t *testing.T) {
	r := NewHeuristicReranker()
	candidates := []Candidate{
		{ChunkID: "chunk-1", Content: "low relevance", BaseScore: 0.5},
		{ChunkID: "chunk-2", Content: "medium relevance", BaseScore: 0.7},
		{ChunkID: "chunk-3", Content: "high relevance", BaseScore: 0.9},
	}

	results, err := r.Rerank(context.Background(), "test", candidates)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// Should be sorted by score descending
	for i := 1; i < len(results); i++ {
		if results[i].FinalScore > results[i-1].FinalScore {
			t.Errorf("Results not sorted descending at position %d", i)
		}
	}
}

func TestHeuristicReranker_NameMatchBoostsRanking(t *testing.T) {
	r := NewHeuristicReranker()
	candidates := []Candidate{
		{ChunkID: "no-match", Content: "some other code", Name: "otherFunc", BaseScore: 0.8},
		{ChunkID: "match", Content: "config parsing", Name: "parseConfig", BaseScore: 0.79}, // Very close base score
	}

	results, err := r.Rerank(context.Background(), "parseConfig", candidates)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// The one with name match should be boosted despite slightly lower base score
	if results[0].ChunkID != "match" {
		t.Error("Name match should boost result to top")
	}
}

func TestHeuristicReranker_ExactPhraseBoostsRanking(t *testing.T) {
	r := NewHeuristicReranker()
	candidates := []Candidate{
		{ChunkID: "no-phrase", Content: "handles authentication", BaseScore: 0.8},
		{ChunkID: "has-phrase", Content: "authentication flow handler", BaseScore: 0.79}, // Very close base score
	}

	results, err := r.Rerank(context.Background(), "authentication flow", candidates)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// The one with exact phrase should be boosted
	if results[0].ChunkID != "has-phrase" {
		t.Error("Exact phrase should boost result to top")
	}
}

func TestHeuristicReranker_TestFileDemoted(t *testing.T) {
	r := NewHeuristicReranker()
	candidates := []Candidate{
		{ChunkID: "test-file", Content: "test code", FilePath: "handler_test.go", BaseScore: 0.81},
		{ChunkID: "prod-file", Content: "prod code", FilePath: "handler.go", BaseScore: 0.80},
	}

	results, err := r.Rerank(context.Background(), "handler", candidates)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Production file should rank higher despite lower base score (test file penalty)
	if results[0].ChunkID != "prod-file" {
		t.Error("Test file should be demoted")
	}
}

func TestHeuristicReranker_CombinedSignals(t *testing.T) {
	r := NewHeuristicReranker()
	candidates := []Candidate{
		{
			ChunkID:   "all-signals",
			Content:   "parse config from file",
			Name:      "parseConfig",
			FilePath:  "config/parser.go",
			ChunkType: "function",
			BaseScore: 0.7,
			ModTime:   time.Now().Add(-1 * time.Hour),
		},
		{
			ChunkID:   "no-signals",
			Content:   "random code",
			Name:      "doSomething",
			FilePath:  "other/stuff.go",
			ChunkType: "function",
			BaseScore: 0.8,
		},
	}

	results, err := r.Rerank(context.Background(), "parse config", candidates)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// The one with multiple matching signals should win
	if results[0].ChunkID != "all-signals" {
		t.Error("Candidate with multiple matching signals should rank higher")
	}
}

func TestHeuristicReranker_PreservesBaseScore(t *testing.T) {
	r := NewHeuristicReranker()
	candidates := []Candidate{
		{ChunkID: "chunk-1", Content: "test", BaseScore: 0.9},
	}

	results, err := r.Rerank(context.Background(), "test", candidates)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// BaseScore should influence FinalScore
	if results[0].Candidate.BaseScore != 0.9 {
		t.Error("BaseScore should be preserved in Candidate")
	}
	if results[0].FinalScore == 0 {
		t.Error("FinalScore should incorporate BaseScore")
	}
}

func TestHeuristicReranker_SignalScoresPopulated(t *testing.T) {
	r := NewHeuristicReranker()
	candidates := []Candidate{
		{
			ChunkID:   "chunk-1",
			Content:   "test content",
			Name:      "testFunc",
			FilePath:  "test.go",
			ChunkType: "function",
			BaseScore: 0.8,
		},
	}

	results, err := r.Rerank(context.Background(), "test", candidates)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if results[0].SignalScores == nil {
		t.Fatal("SignalScores should be populated")
	}

	expectedKeys := []string{"name_match", "exact_phrase", "path_match", "test_penalty", "recency", "chunk_type"}
	for _, key := range expectedKeys {
		if _, ok := results[0].SignalScores[key]; !ok {
			t.Errorf("Missing signal key: %s", key)
		}
	}
}

func TestHeuristicReranker_ContextRespected(t *testing.T) {
	r := NewHeuristicReranker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := r.Rerank(ctx, "test", []Candidate{{ChunkID: "chunk-1"}})

	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestHeuristicReranker_StableOrdering(t *testing.T) {
	r := NewHeuristicReranker()
	candidates := []Candidate{
		{ChunkID: "chunk-b", Content: "same", BaseScore: 0.8},
		{ChunkID: "chunk-a", Content: "same", BaseScore: 0.8},
	}

	// Run multiple times
	var firstOrder []string
	for i := 0; i < 5; i++ {
		results, _ := r.Rerank(context.Background(), "test", candidates)
		order := make([]string, len(results))
		for j, r := range results {
			order[j] = r.ChunkID
		}
		if i == 0 {
			firstOrder = order
		} else {
			for j, id := range order {
				if id != firstOrder[j] {
					t.Errorf("Ordering not stable at iteration %d", i)
					break
				}
			}
		}
	}
}

func TestHeuristicReranker_VeryLongContent(t *testing.T) {
	r := NewHeuristicReranker()

	// Create a 100KB content string
	longContent := make([]byte, 100*1024)
	for i := range longContent {
		longContent[i] = 'x'
	}

	candidates := []Candidate{
		{ChunkID: "long", Content: string(longContent), BaseScore: 0.8},
	}

	start := time.Now()
	results, err := r.Rerank(context.Background(), "test", candidates)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Error("Should handle long content")
	}
	if elapsed > time.Second {
		t.Errorf("Took too long for long content: %v", elapsed)
	}
}

func TestExtractQueryTerms(t *testing.T) {
	tests := []struct {
		query    string
		expected int
	}{
		{"parse config", 2},
		{"parse", 1},
		{"", 0},
		{"a b c", 0}, // Single chars filtered
		{"parse-config", 1},
	}

	for _, tt := range tests {
		terms := extractQueryTerms(tt.query)
		if len(terms) != tt.expected {
			t.Errorf("extractQueryTerms(%q) = %d terms, want %d", tt.query, len(terms), tt.expected)
		}
	}
}
