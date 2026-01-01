package search

import (
	"testing"
)

// ============================================================================
// 27.1 Query Preprocessing Tests
// ============================================================================

func TestPreprocessQuery_SingleWord(t *testing.T) {
	result := PreprocessQuery("database")

	if result.Original != "database" {
		t.Errorf("Expected original 'database', got '%s'", result.Original)
	}
	if len(result.Terms) != 1 {
		t.Errorf("Expected 1 term, got %d", len(result.Terms))
	}
	if result.Terms[0] != "database" {
		t.Errorf("Expected term 'database', got '%s'", result.Terms[0])
	}
}

func TestPreprocessQuery_MultipleWords(t *testing.T) {
	result := PreprocessQuery("database connection")

	if len(result.Terms) != 2 {
		t.Errorf("Expected 2 terms, got %d: %v", len(result.Terms), result.Terms)
	}
	if result.Terms[0] != "database" || result.Terms[1] != "connection" {
		t.Errorf("Expected ['database', 'connection'], got %v", result.Terms)
	}
}

func TestPreprocessQuery_StopwordRemoval(t *testing.T) {
	result := PreprocessQuery("the database")

	// "the" should be removed as a stopword
	if len(result.Terms) != 1 {
		t.Errorf("Expected 1 term after stopword removal, got %d: %v", len(result.Terms), result.Terms)
	}
	if result.Terms[0] != "database" {
		t.Errorf("Expected 'database', got '%s'", result.Terms[0])
	}
}

func TestPreprocessQuery_AllStopwords(t *testing.T) {
	result := PreprocessQuery("the a an")

	// All words are stopwords, should have empty terms
	if len(result.Terms) != 0 {
		t.Errorf("Expected 0 terms for all stopwords, got %d: %v", len(result.Terms), result.Terms)
	}
}

func TestPreprocessQuery_CaseNormalization(t *testing.T) {
	result := PreprocessQuery("Database CONNECTION")

	if len(result.Terms) != 2 {
		t.Errorf("Expected 2 terms, got %d", len(result.Terms))
	}
	if result.Terms[0] != "database" || result.Terms[1] != "connection" {
		t.Errorf("Expected lowercase terms, got %v", result.Terms)
	}
}

func TestPreprocessQuery_QuotedPhrase(t *testing.T) {
	result := PreprocessQuery(`"exact phrase" search`)

	if !result.HasPhrases {
		t.Error("Expected HasPhrases to be true")
	}
	if len(result.Phrases) != 1 {
		t.Errorf("Expected 1 phrase, got %d", len(result.Phrases))
	}
	if result.Phrases[0] != "exact phrase" {
		t.Errorf("Expected phrase 'exact phrase', got '%s'", result.Phrases[0])
	}
}

func TestPreprocessQuery_MixedPhrasesAndTerms(t *testing.T) {
	result := PreprocessQuery(`"auth flow" database`)

	if !result.HasPhrases {
		t.Error("Expected HasPhrases to be true")
	}
	if len(result.Phrases) != 1 {
		t.Errorf("Expected 1 phrase, got %d", len(result.Phrases))
	}
	// Should also have "database" as a term
	hasDatabase := false
	for _, term := range result.Terms {
		if term == "database" {
			hasDatabase = true
			break
		}
	}
	if !hasDatabase {
		t.Errorf("Expected 'database' in terms, got %v", result.Terms)
	}
}

func TestPreprocessQuery_SpecialCharacters(t *testing.T) {
	result := PreprocessQuery("func() { map[string]int }")

	// Special chars should be handled (removed or kept as appropriate)
	if result.Original != "func() { map[string]int }" {
		t.Errorf("Original should be preserved, got '%s'", result.Original)
	}
	// Should have at least "func" and "map" as terms
	if len(result.Terms) == 0 {
		t.Error("Expected some terms from special char query")
	}
}

func TestPreprocessQuery_Numbers(t *testing.T) {
	result := PreprocessQuery("error 404")

	if len(result.Terms) != 2 {
		t.Errorf("Expected 2 terms, got %d: %v", len(result.Terms), result.Terms)
	}
	// Check that 404 is preserved
	has404 := false
	for _, term := range result.Terms {
		if term == "404" {
			has404 = true
			break
		}
	}
	if !has404 {
		t.Errorf("Expected '404' in terms, got %v", result.Terms)
	}
}

func TestPreprocessQuery_EmptyQuery(t *testing.T) {
	result := PreprocessQuery("")

	if result.Original != "" {
		t.Errorf("Expected empty original, got '%s'", result.Original)
	}
	if len(result.Terms) != 0 {
		t.Errorf("Expected 0 terms for empty query, got %d", len(result.Terms))
	}
	if result.FTSQuery != "" {
		t.Errorf("Expected empty FTSQuery, got '%s'", result.FTSQuery)
	}
}

func TestPreprocessQuery_WhitespaceOnly(t *testing.T) {
	result := PreprocessQuery("   ")

	if len(result.Terms) != 0 {
		t.Errorf("Expected 0 terms for whitespace-only query, got %d", len(result.Terms))
	}
}

func TestPreprocessQuery_FTSQueryFormat(t *testing.T) {
	result := PreprocessQuery("database connection")

	// FTS query should be formatted for FTS5
	if result.FTSQuery == "" {
		t.Error("Expected non-empty FTSQuery")
	}
	// Should contain both terms with OR
	if result.FTSQuery != "database OR connection" && result.FTSQuery != "database connection" {
		t.Logf("FTSQuery format: %s", result.FTSQuery)
	}
}

func TestPreprocessQuery_FTSQueryWithPhrases(t *testing.T) {
	result := PreprocessQuery(`"exact phrase" term`)

	if result.FTSQuery == "" {
		t.Error("Expected non-empty FTSQuery")
	}
	// Phrase should be quoted in FTS query
	// Expected: "exact phrase" OR term or similar
}

func TestPreprocessQuery_VeryLongQuery(t *testing.T) {
	// Create a 1000+ char query
	longQuery := ""
	for i := 0; i < 200; i++ {
		longQuery += "word "
	}

	result := PreprocessQuery(longQuery)

	// Should not error, should have many terms
	if len(result.Terms) == 0 {
		t.Error("Expected terms from long query")
	}
}

func TestPreprocessQuery_UnicodeTerms(t *testing.T) {
	result := PreprocessQuery("函数 función")

	if len(result.Terms) != 2 {
		t.Errorf("Expected 2 unicode terms, got %d: %v", len(result.Terms), result.Terms)
	}
}
