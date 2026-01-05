package embedder

// Token estimation constants
const (
	// CharsPerToken is the estimated average characters per token for code.
	// For code, ~4 characters per token is typical, but we use 3.5 to be
	// conservative (better to split early than fail on context limits).
	CharsPerToken = 3.5

	// ConservativeCharsPerToken is a more conservative estimate for
	// calculating maximum safe content length.
	ConservativeCharsPerToken = 3.2
)

// EstimateTokens approximates token count from text length.
// For code, ~4 characters per token is a reasonable approximation.
// We use 3.5 to be conservative (better to split early than fail).
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return int(float64(len(text)) / CharsPerToken)
}

// EstimateChars converts token count to approximate character count.
func EstimateChars(tokens int) int {
	return int(float64(tokens) * CharsPerToken)
}

// MaxCharsForTokens returns the maximum characters that fit in the given token budget.
// Uses a slightly more conservative multiplier for safety.
func MaxCharsForTokens(tokens int) int {
	return int(float64(tokens) * ConservativeCharsPerToken)
}
