package embedder

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// EstimateTokens Tests
// =============================================================================

// --- Happy Path Tests ---

func TestEstimateTokens_TypicalGoFunction(t *testing.T) {
	code := `func calculateSum(a, b int) int {
	result := a + b
	return result
}`
	tokens := EstimateTokens(code)
	// ~60 chars / 3.5 ≈ 17 tokens
	assert.True(t, tokens >= 15 && tokens <= 20, "Expected ~17 tokens, got %d", tokens)
}

func TestEstimateTokens_TypicalPythonClass(t *testing.T) {
	code := `class Calculator:
    def __init__(self):
        self.value = 0

    def add(self, x):
        self.value += x
        return self.value`
	tokens := EstimateTokens(code)
	// ~150 chars / 3.5 ≈ 43 tokens
	assert.True(t, tokens >= 35 && tokens <= 50, "Expected ~43 tokens, got %d", tokens)
}

func TestEstimateTokens_LargeCodeFile(t *testing.T) {
	// Simulate a 32KB file (near context limit)
	largeCode := strings.Repeat("func foo() { return 1 }\n", 1400) // ~32KB
	tokens := EstimateTokens(largeCode)
	// 32000 chars / 3.5 ≈ 9142 tokens
	assert.True(t, tokens >= 8000 && tokens <= 10000, "Expected ~9000 tokens, got %d", tokens)
}

// --- Success Tests ---

func TestEstimateTokens_EmptyString(t *testing.T) {
	assert.Equal(t, 0, EstimateTokens(""))
}

func TestEstimateTokens_SingleCharacter(t *testing.T) {
	// 1 char / 3.5 = 0.28, should round to 0 or 1
	tokens := EstimateTokens("x")
	assert.True(t, tokens >= 0 && tokens <= 1)
}

func TestEstimateTokens_SingleWord(t *testing.T) {
	tokens := EstimateTokens("function")
	// 8 chars / 3.5 ≈ 2 tokens
	assert.True(t, tokens >= 1 && tokens <= 3)
}

func TestEstimateTokens_WhitespaceOnly(t *testing.T) {
	tokens := EstimateTokens("   \t\n  ")
	// Whitespace still counts as characters
	assert.True(t, tokens >= 1 && tokens <= 3)
}

// --- Edge Case Tests ---

func TestEstimateTokens_UnicodeCharacters(t *testing.T) {
	// Unicode: each rune may be multiple bytes
	code := "func 你好() { return \"世界\" }"
	tokens := EstimateTokens(code)
	// Should handle unicode gracefully - uses byte length
	assert.True(t, tokens > 0)
}

func TestEstimateTokens_VeryLongSingleLine(t *testing.T) {
	// 100KB single line (minified-like)
	longLine := strings.Repeat("x", 100*1024)
	tokens := EstimateTokens(longLine)
	// 100KB / 3.5 ≈ 29257 tokens
	assert.True(t, tokens >= 28000 && tokens <= 31000)
}

func TestEstimateTokens_OnlyNewlines(t *testing.T) {
	tokens := EstimateTokens("\n\n\n\n\n")
	assert.True(t, tokens >= 1 && tokens <= 2)
}

func TestEstimateTokens_MixedIndentation(t *testing.T) {
	code := "\t\tfunc foo() {\n\t\t\treturn 1\n\t\t}"
	tokens := EstimateTokens(code)
	assert.True(t, tokens > 0)
}

func TestEstimateTokens_BinaryLikeContent(t *testing.T) {
	// Content with null bytes and control characters
	binary := string([]byte{0x00, 0x01, 0x02, 'h', 'e', 'l', 'l', 'o', 0x00})
	tokens := EstimateTokens(binary)
	assert.True(t, tokens >= 0)
}

// =============================================================================
// EstimateChars Tests
// =============================================================================

// --- Happy Path Tests ---

func TestEstimateChars_TypicalTokenCount(t *testing.T) {
	chars := EstimateChars(1000)
	// 1000 * 3.5 = 3500
	assert.Equal(t, 3500, chars)
}

func TestEstimateChars_ContextLimit(t *testing.T) {
	chars := EstimateChars(8000)
	// 8000 * 3.5 = 28000
	assert.Equal(t, 28000, chars)
}

// --- Success Tests ---

func TestEstimateChars_Zero(t *testing.T) {
	assert.Equal(t, 0, EstimateChars(0))
}

func TestEstimateChars_One(t *testing.T) {
	chars := EstimateChars(1)
	assert.Equal(t, 3, chars) // 1 * 3.5 = 3.5, truncated to 3
}

// --- Edge Case Tests ---

func TestEstimateChars_LargeTokenCount(t *testing.T) {
	chars := EstimateChars(100000)
	assert.Equal(t, 350000, chars)
}

// =============================================================================
// MaxCharsForTokens Tests
// =============================================================================

// --- Happy Path Tests ---

func TestMaxCharsForTokens_TypicalLimit(t *testing.T) {
	chars := MaxCharsForTokens(8000)
	// 8000 * 3.2 = 25600 (conservative)
	assert.Equal(t, 25600, chars)
}

// --- Success Tests ---

func TestMaxCharsForTokens_Zero(t *testing.T) {
	assert.Equal(t, 0, MaxCharsForTokens(0))
}

// --- Edge Case Tests ---

func TestMaxCharsForTokens_IsMoreConservative(t *testing.T) {
	tokens := 1000
	maxChars := MaxCharsForTokens(tokens)
	estimateChars := EstimateChars(tokens)
	// MaxChars should be less than EstimateChars (more conservative)
	assert.Less(t, maxChars, estimateChars)
}

// =============================================================================
// Roundtrip Tests
// =============================================================================

func TestTokenEstimation_Roundtrip(t *testing.T) {
	testCases := []string{
		"func main() {}",
		"class Foo:\n    pass",
		strings.Repeat("x", 10000),
	}

	for _, original := range testCases {
		tokens := EstimateTokens(original)
		chars := EstimateChars(tokens)
		// Roundtrip should be within 20% of original length
		delta := float64(len(original)) * 0.2
		assert.InDelta(t, len(original), chars, delta)
	}
}
