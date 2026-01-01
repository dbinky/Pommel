package rerank

import (
	"testing"
	"time"
)

// ============================================================================
// 28.2 Heuristic Scoring Signal Tests
// ============================================================================

// Name Match Signal Tests

func TestNameMatchSignal_ExactMatch(t *testing.T) {
	score := NameMatchSignal("parseConfig", []string{"parseconfig"})
	if score <= 0 {
		t.Errorf("Expected positive boost for exact match, got %f", score)
	}
}

func TestNameMatchSignal_PartialMatch(t *testing.T) {
	score := NameMatchSignal("parseConfig", []string{"parse"})
	if score <= 0 {
		t.Errorf("Expected positive boost for partial match, got %f", score)
	}
}

func TestNameMatchSignal_NoMatch(t *testing.T) {
	score := NameMatchSignal("parseConfig", []string{"database"})
	if score != 0 {
		t.Errorf("Expected 0 for no match, got %f", score)
	}
}

func TestNameMatchSignal_CaseInsensitive(t *testing.T) {
	score := NameMatchSignal("parseConfig", []string{"PARSE"})
	if score <= 0 {
		t.Errorf("Expected positive boost for case-insensitive match, got %f", score)
	}
}

func TestNameMatchSignal_MultipleTerms(t *testing.T) {
	singleMatch := NameMatchSignal("parseConfig", []string{"parse"})
	multiMatch := NameMatchSignal("parseConfig", []string{"parse", "config"})
	if multiMatch <= singleMatch {
		t.Errorf("Expected higher boost for multiple matches: single=%f, multi=%f", singleMatch, multiMatch)
	}
}

func TestNameMatchSignal_EmptyName(t *testing.T) {
	score := NameMatchSignal("", []string{"parse"})
	if score != 0 {
		t.Errorf("Expected 0 for empty name, got %f", score)
	}
}

func TestNameMatchSignal_EmptyTerms(t *testing.T) {
	score := NameMatchSignal("parseConfig", []string{})
	if score != 0 {
		t.Errorf("Expected 0 for empty terms, got %f", score)
	}
}

// Exact Phrase Signal Tests

func TestExactPhraseSignal_Found(t *testing.T) {
	content := "This function handles user authentication flow"
	score := ExactPhraseSignal(content, "authentication flow")
	if score <= 0 {
		t.Errorf("Expected positive boost for exact phrase, got %f", score)
	}
}

func TestExactPhraseSignal_NotFound(t *testing.T) {
	content := "This function handles user authentication flow"
	score := ExactPhraseSignal(content, "database connection")
	if score != 0 {
		t.Errorf("Expected 0 for phrase not found, got %f", score)
	}
}

func TestExactPhraseSignal_CaseInsensitive(t *testing.T) {
	content := "This function handles user AUTHENTICATION FLOW"
	score := ExactPhraseSignal(content, "authentication flow")
	if score <= 0 {
		t.Errorf("Expected positive boost for case-insensitive phrase, got %f", score)
	}
}

func TestExactPhraseSignal_PartialNotCounted(t *testing.T) {
	content := "authentication only"
	score := ExactPhraseSignal(content, "authentication flow")
	if score != 0 {
		t.Errorf("Expected 0 for partial phrase match, got %f", score)
	}
}

func TestExactPhraseSignal_EmptyQuery(t *testing.T) {
	content := "some content"
	score := ExactPhraseSignal(content, "")
	if score != 0 {
		t.Errorf("Expected 0 for empty query, got %f", score)
	}
}

// Path Match Signal Tests

func TestPathMatchSignal_DirectoryMatch(t *testing.T) {
	score := PathMatchSignal("internal/auth/handler.go", []string{"auth"})
	if score <= 0 {
		t.Errorf("Expected positive boost for directory match, got %f", score)
	}
}

func TestPathMatchSignal_FileNameMatch(t *testing.T) {
	score := PathMatchSignal("internal/auth/handler.go", []string{"handler"})
	if score <= 0 {
		t.Errorf("Expected positive boost for filename match, got %f", score)
	}
}

func TestPathMatchSignal_NoMatch(t *testing.T) {
	score := PathMatchSignal("internal/auth/handler.go", []string{"database"})
	if score != 0 {
		t.Errorf("Expected 0 for no match, got %f", score)
	}
}

func TestPathMatchSignal_MultipleSegments(t *testing.T) {
	singleMatch := PathMatchSignal("internal/auth/handler.go", []string{"auth"})
	multiMatch := PathMatchSignal("internal/auth/handler.go", []string{"auth", "handler"})
	if multiMatch <= singleMatch {
		t.Errorf("Expected higher boost for multiple matches: single=%f, multi=%f", singleMatch, multiMatch)
	}
}

// Test File Penalty Tests

func TestTestFilePenalty_TestFile(t *testing.T) {
	score := TestFilePenalty("internal/auth/handler_test.go")
	if score >= 0 {
		t.Errorf("Expected negative penalty for test file, got %f", score)
	}
}

func TestTestFilePenalty_TestDirectory(t *testing.T) {
	tests := []string{
		"test/handler.go",
		"tests/handler.go",
		"internal/test/handler.go",
	}
	for _, path := range tests {
		score := TestFilePenalty(path)
		if score >= 0 {
			t.Errorf("Expected negative penalty for test directory %s, got %f", path, score)
		}
	}
}

func TestTestFilePenalty_SpecFile(t *testing.T) {
	score := TestFilePenalty("src/handler.spec.ts")
	if score >= 0 {
		t.Errorf("Expected negative penalty for spec file, got %f", score)
	}
}

func TestTestFilePenalty_NotTestFile(t *testing.T) {
	score := TestFilePenalty("internal/auth/handler.go")
	if score != 0 {
		t.Errorf("Expected 0 for production file, got %f", score)
	}
}

func TestTestFilePenalty_MockFile(t *testing.T) {
	tests := []string{
		"internal/mock_client.go",
		"internal/client_mock.go",
	}
	for _, path := range tests {
		score := TestFilePenalty(path)
		if score >= 0 {
			t.Errorf("Expected negative penalty for mock file %s, got %f", path, score)
		}
	}
}

// Recency Boost Tests

func TestRecencyBoost_VeryRecent(t *testing.T) {
	now := time.Now()
	modTime := now.Add(-1 * time.Hour)
	score := RecencyBoost(modTime, now)
	if score <= 0 {
		t.Errorf("Expected positive boost for very recent file, got %f", score)
	}
}

func TestRecencyBoost_LastWeek(t *testing.T) {
	now := time.Now()
	modTime := now.Add(-7 * 24 * time.Hour)
	score := RecencyBoost(modTime, now)
	if score < 0 {
		t.Errorf("Expected non-negative boost for last week file, got %f", score)
	}
}

func TestRecencyBoost_OldFile(t *testing.T) {
	now := time.Now()
	modTime := now.Add(-60 * 24 * time.Hour)
	score := RecencyBoost(modTime, now)
	if score != 0 {
		t.Errorf("Expected 0 for old file, got %f", score)
	}
}

func TestRecencyBoost_FutureTime(t *testing.T) {
	now := time.Now()
	modTime := now.Add(24 * time.Hour)
	score := RecencyBoost(modTime, now)
	if score != 0 {
		t.Errorf("Expected 0 for future time, got %f", score)
	}
}

func TestRecencyBoost_ZeroTime(t *testing.T) {
	now := time.Now()
	var modTime time.Time
	score := RecencyBoost(modTime, now)
	if score != 0 {
		t.Errorf("Expected 0 for zero time, got %f", score)
	}
}

// Chunk Type Signal Tests

func TestChunkTypeSignal_FunctionForVerb(t *testing.T) {
	// Verb-like queries should prefer functions
	score := ChunkTypeSignal("function", "handle")
	if score < 0 {
		t.Errorf("Expected non-negative score for function with verb query, got %f", score)
	}
}

func TestChunkTypeSignal_ClassForNoun(t *testing.T) {
	// Noun-like queries should prefer classes
	score := ChunkTypeSignal("class", "handler")
	if score < 0 {
		t.Errorf("Expected non-negative score for class with noun query, got %f", score)
	}
}

func TestChunkTypeSignal_Neutral(t *testing.T) {
	// Ambiguous queries should be neutral
	score := ChunkTypeSignal("function", "process")
	// Just verify it doesn't error and returns something reasonable
	if score < -0.2 || score > 0.2 {
		t.Errorf("Expected score in range [-0.2, 0.2], got %f", score)
	}
}
