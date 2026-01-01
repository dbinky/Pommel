package search

import (
	"regexp"
	"strings"
	"unicode"
)

// ProcessedQuery contains the preprocessed components of a search query.
type ProcessedQuery struct {
	Original   string   // Original query text
	Terms      []string // Extracted significant terms (lowercase, stopwords removed)
	FTSQuery   string   // Formatted query for FTS5
	HasPhrases bool     // Whether the query contains quoted phrases
	Phrases    []string // Extracted quoted phrases
}

// Common stopwords to exclude from search terms
var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"but": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "with": true, "by": true, "is": true,
	"it": true, "this": true, "that": true, "be": true, "as": true,
	"are": true, "was": true, "were": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "must": true, "shall": true, "can": true,
	"i": true, "you": true, "he": true, "she": true, "we": true, "they": true,
	"me": true, "him": true, "her": true, "us": true, "them": true,
	"my": true, "your": true, "his": true, "its": true, "our": true, "their": true,
	"what": true, "which": true, "who": true, "when": true, "where": true, "how": true,
	"all": true, "each": true, "every": true, "both": true, "few": true,
	"more": true, "most": true, "other": true, "some": true, "such": true,
	"no": true, "not": true, "only": true, "same": true, "so": true,
	"than": true, "too": true, "very": true, "just": true, "also": true,
	"now": true, "here": true, "there": true, "then": true,
}

// phraseRegex matches quoted phrases
var phraseRegex = regexp.MustCompile(`"([^"]+)"`)

// wordRegex matches word characters (letters, digits, underscores, unicode letters)
var wordRegex = regexp.MustCompile(`[\p{L}\p{N}_]+`)

// PreprocessQuery extracts search terms and phrases from a query string.
func PreprocessQuery(query string) ProcessedQuery {
	result := ProcessedQuery{
		Original: query,
		Terms:    []string{},
		Phrases:  []string{},
	}

	if strings.TrimSpace(query) == "" {
		return result
	}

	// Extract quoted phrases first
	phraseMatches := phraseRegex.FindAllStringSubmatch(query, -1)
	for _, match := range phraseMatches {
		if len(match) >= 2 {
			phrase := strings.TrimSpace(match[1])
			if phrase != "" {
				result.Phrases = append(result.Phrases, phrase)
				result.HasPhrases = true
			}
		}
	}

	// Remove quoted phrases from query for term extraction
	queryWithoutPhrases := phraseRegex.ReplaceAllString(query, " ")

	// Extract words
	words := wordRegex.FindAllString(queryWithoutPhrases, -1)
	for _, word := range words {
		normalized := strings.ToLower(word)
		// Skip stopwords and very short words (single char unless it's a number)
		if stopwords[normalized] {
			continue
		}
		if len(normalized) == 1 && !unicode.IsDigit(rune(normalized[0])) {
			continue
		}
		result.Terms = append(result.Terms, normalized)
	}

	// Build FTS query
	result.FTSQuery = buildFTSQuery(result.Terms, result.Phrases)

	return result
}

// buildFTSQuery creates an FTS5-compatible query string.
func buildFTSQuery(terms []string, phrases []string) string {
	parts := make([]string, 0, len(terms)+len(phrases))

	// Add phrases (quoted)
	for _, phrase := range phrases {
		parts = append(parts, `"`+phrase+`"`)
	}

	// Add terms
	parts = append(parts, terms...)

	if len(parts) == 0 {
		return ""
	}

	// Join with OR for broad matching
	return strings.Join(parts, " OR ")
}
