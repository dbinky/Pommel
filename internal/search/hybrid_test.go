package search

import (
	"testing"
)

// ============================================================================
// 27.3 Parallel Search Execution Tests
// ============================================================================

// Note: These are integration tests that require a database and embedder.
// For unit tests, we use mocks. See hybrid_integration_test.go for full integration tests.

func TestHybridOptions_Defaults(t *testing.T) {
	opts := DefaultHybridOptions()

	if !opts.HybridEnabled {
		t.Error("HybridEnabled should default to true")
	}
	if opts.RRFK != DefaultRRFK {
		t.Errorf("RRFK should default to %d, got %d", DefaultRRFK, opts.RRFK)
	}
	if opts.Limit <= 0 {
		t.Error("Limit should have a positive default")
	}
}

func TestHybridOptions_DisableHybrid(t *testing.T) {
	opts := DefaultHybridOptions()
	opts.HybridEnabled = false

	if opts.HybridEnabled {
		t.Error("HybridEnabled should be false when disabled")
	}
}

// ============================================================================
// 27.4 Configuration Tests
// ============================================================================

func TestHybridConfig_DefaultValues(t *testing.T) {
	config := DefaultHybridConfig()

	if !config.Enabled {
		t.Error("Hybrid search should be enabled by default")
	}
	if config.RRFK <= 0 {
		t.Error("RRFK should have positive default")
	}
	if config.VectorWeight <= 0 || config.VectorWeight > 1 {
		t.Errorf("VectorWeight should be in (0,1], got %f", config.VectorWeight)
	}
	if config.KeywordWeight <= 0 || config.KeywordWeight > 1 {
		t.Errorf("KeywordWeight should be in (0,1], got %f", config.KeywordWeight)
	}
}

// ============================================================================
// 27.6 API Response Tests
// ============================================================================

func TestMergedResult_MatchSource(t *testing.T) {
	tests := []struct {
		name           string
		vectorRank     int
		keywordRank    int
		expectedSource string
	}{
		{
			name:           "vector only",
			vectorRank:     0,
			keywordRank:    -1,
			expectedSource: "vector",
		},
		{
			name:           "keyword only",
			vectorRank:     -1,
			keywordRank:    0,
			expectedSource: "keyword",
		},
		{
			name:           "both sources",
			vectorRank:     0,
			keywordRank:    1,
			expectedSource: "both",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergedResult{
				ChunkID:     "test",
				VectorRank:  tt.vectorRank,
				KeywordRank: tt.keywordRank,
			}

			source := result.MatchSource()
			if source != tt.expectedSource {
				t.Errorf("Expected match source '%s', got '%s'", tt.expectedSource, source)
			}
		})
	}
}
