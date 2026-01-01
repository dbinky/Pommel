package rerank

import (
	"strings"
	"time"
)

// Signal functions return a score boost/penalty in range [-0.2, 0.2]

// NameMatchSignal boosts results where name contains query terms
func NameMatchSignal(name string, queryTerms []string) float64 {
	if name == "" || len(queryTerms) == 0 {
		return 0
	}

	nameLower := strings.ToLower(name)
	matches := 0

	for _, term := range queryTerms {
		if strings.Contains(nameLower, strings.ToLower(term)) {
			matches++
		}
	}

	if matches == 0 {
		return 0
	}

	// More matches = higher boost, capped at 0.2
	boost := float64(matches) * 0.1
	if boost > 0.2 {
		boost = 0.2
	}
	return boost
}

// ExactPhraseSignal boosts results containing exact query phrase
func ExactPhraseSignal(content string, query string) float64 {
	if query == "" {
		return 0
	}

	contentLower := strings.ToLower(content)
	queryLower := strings.ToLower(query)

	if strings.Contains(contentLower, queryLower) {
		return 0.15 // Significant boost for exact phrase
	}
	return 0
}

// PathMatchSignal boosts results where file path contains query terms
func PathMatchSignal(filePath string, queryTerms []string) float64 {
	if filePath == "" || len(queryTerms) == 0 {
		return 0
	}

	pathLower := strings.ToLower(filePath)
	matches := 0

	for _, term := range queryTerms {
		if strings.Contains(pathLower, strings.ToLower(term)) {
			matches++
		}
	}

	if matches == 0 {
		return 0
	}

	// More matches = higher boost, capped at 0.15
	boost := float64(matches) * 0.075
	if boost > 0.15 {
		boost = 0.15
	}
	return boost
}

// TestFilePenalty reduces score for test files
func TestFilePenalty(filePath string) float64 {
	pathLower := strings.ToLower(filePath)

	// Check for test file patterns
	testPatterns := []string{
		"_test.go",
		".test.js",
		".test.ts",
		".spec.js",
		".spec.ts",
		".spec.tsx",
		".spec.jsx",
		"_test.py",
		"test_",
	}

	for _, pattern := range testPatterns {
		if strings.Contains(pathLower, pattern) {
			return -0.15 // Test file penalty
		}
	}

	// Check for test directory
	testDirs := []string{"/test/", "/tests/", "\\test\\", "\\tests\\"}
	for _, dir := range testDirs {
		if strings.Contains(pathLower, dir) {
			return -0.15 // Test directory penalty
		}
	}
	// Also check if path starts with test/
	if strings.HasPrefix(pathLower, "test/") || strings.HasPrefix(pathLower, "tests/") {
		return -0.15
	}

	// Check for mock files (smaller penalty)
	mockPatterns := []string{"mock_", "_mock."}
	for _, pattern := range mockPatterns {
		if strings.Contains(pathLower, pattern) {
			return -0.1 // Mock file penalty (smaller)
		}
	}

	return 0
}

// RecencyBoost slightly boosts recently modified files
func RecencyBoost(modTime time.Time, now time.Time) float64 {
	if modTime.IsZero() {
		return 0
	}

	// Check for future time (shouldn't happen but handle it)
	if modTime.After(now) {
		return 0
	}

	daysSince := now.Sub(modTime).Hours() / 24

	// Very recent (< 1 day): max boost
	if daysSince < 1 {
		return 0.1
	}

	// Recent (< 7 days): medium boost
	if daysSince < 7 {
		return 0.05
	}

	// Moderately recent (< 30 days): small boost
	if daysSince < 30 {
		return 0.02
	}

	// Old file: no boost
	return 0
}

// ChunkTypeSignal adjusts based on chunk type relevance to query
func ChunkTypeSignal(chunkType string, query string) float64 {
	queryLower := strings.ToLower(query)
	typeLower := strings.ToLower(chunkType)

	// Verb endings that suggest function-like behavior
	verbSuffixes := []string{"ing", "ed", "ate", "ize", "ify"}

	// Check if query looks like a verb
	isVerbLike := false
	for _, suffix := range verbSuffixes {
		if strings.HasSuffix(queryLower, suffix) {
			isVerbLike = true
			break
		}
	}

	// Common verb prefixes
	verbPrefixes := []string{"get", "set", "do", "is", "has", "can", "make", "create", "update", "delete", "handle", "process", "parse", "load", "save", "send", "receive"}
	for _, prefix := range verbPrefixes {
		if strings.HasPrefix(queryLower, prefix) {
			isVerbLike = true
			break
		}
	}

	// Noun suffixes
	nounSuffixes := []string{"er", "or", "tion", "sion", "ment", "ness", "ity", "ance", "ence"}
	isNounLike := false
	for _, suffix := range nounSuffixes {
		if strings.HasSuffix(queryLower, suffix) {
			isNounLike = true
			break
		}
	}

	// Adjust score based on match between query type and chunk type
	if isVerbLike && (typeLower == "function" || typeLower == "method") {
		return 0.05 // Boost functions for verb-like queries
	}

	if isNounLike && (typeLower == "class" || typeLower == "struct" || typeLower == "type") {
		return 0.05 // Boost classes for noun-like queries
	}

	return 0 // Neutral for ambiguous cases
}
