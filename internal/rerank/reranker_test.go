package rerank

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ============================================================================
// 28.1 Reranker Interface Tests
// ============================================================================

func TestCandidate_RequiredFields(t *testing.T) {
	c := Candidate{
		ChunkID: "test-chunk",
		Content: "some content",
	}
	if c.ChunkID == "" {
		t.Error("ChunkID should be set")
	}
	if c.Content == "" {
		t.Error("Content should be set")
	}
}

func TestRankedCandidate_PreservesCandidate(t *testing.T) {
	c := Candidate{
		ChunkID:   "test-chunk",
		Content:   "some content",
		Name:      "testFunc",
		FilePath:  "test.go",
		ChunkType: "function",
		BaseScore: 0.8,
	}

	rc := RankedCandidate{
		Candidate:     c,
		FinalScore:    0.85,
		RerankerScore: 0.9,
	}

	if rc.ChunkID != c.ChunkID {
		t.Errorf("ChunkID not preserved: got %s, want %s", rc.ChunkID, c.ChunkID)
	}
	if rc.Content != c.Content {
		t.Errorf("Content not preserved")
	}
	if rc.Name != c.Name {
		t.Errorf("Name not preserved")
	}
}

func TestRankedCandidate_SignalScoresOptional(t *testing.T) {
	rc := RankedCandidate{
		Candidate:     Candidate{ChunkID: "test"},
		FinalScore:    0.85,
		RerankerScore: 0.9,
		SignalScores:  nil, // Optional
	}

	if rc.SignalScores != nil {
		t.Error("SignalScores should be nil when not set")
	}
}

// ============================================================================
// 28.5 Fallback Logic Tests
// ============================================================================

type mockReranker struct {
	name      string
	available bool
	results   []RankedCandidate
	err       error
	called    bool
}

func (m *mockReranker) Rerank(ctx context.Context, query string, candidates []Candidate) ([]RankedCandidate, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func (m *mockReranker) Name() string {
	return m.name
}

func (m *mockReranker) Available(ctx context.Context) bool {
	return m.available
}

func TestFallbackReranker_UsesPrimaryWhenAvailable(t *testing.T) {
	primary := &mockReranker{
		name:      "primary",
		available: true,
		results:   []RankedCandidate{{Candidate: Candidate{ChunkID: "from-primary"}}},
	}
	secondary := &mockReranker{
		name:      "secondary",
		available: true,
		results:   []RankedCandidate{{Candidate: Candidate{ChunkID: "from-secondary"}}},
	}

	fallback := NewFallbackReranker(primary, secondary, 2*time.Second)
	results, err := fallback.Rerank(context.Background(), "test", []Candidate{})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !primary.called {
		t.Error("Primary should have been called")
	}
	if secondary.called {
		t.Error("Secondary should not have been called")
	}
	if len(results) > 0 && results[0].ChunkID != "from-primary" {
		t.Error("Should return primary results")
	}
}

func TestFallbackReranker_FallsBackWhenUnavailable(t *testing.T) {
	primary := &mockReranker{
		name:      "primary",
		available: false,
		results:   []RankedCandidate{{Candidate: Candidate{ChunkID: "from-primary"}}},
	}
	secondary := &mockReranker{
		name:      "secondary",
		available: true,
		results:   []RankedCandidate{{Candidate: Candidate{ChunkID: "from-secondary"}}},
	}

	fallback := NewFallbackReranker(primary, secondary, 2*time.Second)
	results, err := fallback.Rerank(context.Background(), "test", []Candidate{})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if primary.called {
		t.Error("Primary should not have been called when unavailable")
	}
	if !secondary.called {
		t.Error("Secondary should have been called")
	}
	if len(results) > 0 && results[0].ChunkID != "from-secondary" {
		t.Error("Should return secondary results")
	}
}

func TestFallbackReranker_FallsBackOnPrimaryError(t *testing.T) {
	primary := &mockReranker{
		name:      "primary",
		available: true,
		err:       errors.New("primary error"),
	}
	secondary := &mockReranker{
		name:      "secondary",
		available: true,
		results:   []RankedCandidate{{Candidate: Candidate{ChunkID: "from-secondary"}}},
	}

	fallback := NewFallbackReranker(primary, secondary, 2*time.Second)
	results, err := fallback.Rerank(context.Background(), "test", []Candidate{})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !primary.called {
		t.Error("Primary should have been called")
	}
	if !secondary.called {
		t.Error("Secondary should have been called after primary error")
	}
	if len(results) > 0 && results[0].ChunkID != "from-secondary" {
		t.Error("Should return secondary results")
	}
}

func TestFallbackReranker_ReturnsErrorWhenBothFail(t *testing.T) {
	primary := &mockReranker{
		name:      "primary",
		available: true,
		err:       errors.New("primary error"),
	}
	secondary := &mockReranker{
		name:      "secondary",
		available: true,
		err:       errors.New("secondary error"),
	}

	fallback := NewFallbackReranker(primary, secondary, 2*time.Second)
	_, err := fallback.Rerank(context.Background(), "test", []Candidate{})

	if err == nil {
		t.Error("Expected error when both fail")
	}
}

func TestFallbackReranker_Name(t *testing.T) {
	primary := &mockReranker{name: "ollama"}
	secondary := &mockReranker{name: "heuristic"}

	fallback := NewFallbackReranker(primary, secondary, 2*time.Second)
	name := fallback.Name()

	if name != "ollama->heuristic" {
		t.Errorf("Expected 'ollama->heuristic', got '%s'", name)
	}
}

func TestFallbackReranker_ContextPropagated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	primary := &mockReranker{
		name:      "primary",
		available: true,
		results:   []RankedCandidate{},
	}
	secondary := &mockReranker{
		name:      "secondary",
		available: true,
		results:   []RankedCandidate{},
	}

	fallback := NewFallbackReranker(primary, secondary, 2*time.Second)
	_, _ = fallback.Rerank(ctx, "test", []Candidate{})

	// The test passes if it doesn't hang - context cancellation is propagated
}
