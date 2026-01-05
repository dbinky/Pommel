# Chunk Splitting Phase 1: Foundation

**Parent Design:** [2026-01-04-chunk-splitting-design.md](./2026-01-04-chunk-splitting-design.md)
**Phase:** 1 of 3
**Goal:** Build infrastructure for token estimation, provider limits, minified detection, and database schema

## TDD Requirements

**STRICT TDD PROCESS - Follow for every task:**

1. **Write tests FIRST** - No implementation code until tests exist
2. **Run tests, verify they FAIL** - Red phase confirms tests are valid
3. **Write minimal implementation** - Only enough to pass tests
4. **Run tests, verify they PASS** - Green phase confirms implementation
5. **Refactor if needed** - Clean up while keeping tests green
6. **Commit** - Small, atomic commits after each green phase

**Test Categories Required for Each Component:**

| Category | Description | Example |
|----------|-------------|---------|
| Happy Path | Normal, expected usage | Estimate tokens for 1000-char code |
| Success | Valid inputs that should work | Empty string returns 0 tokens |
| Failure | Invalid inputs handled gracefully | Negative values, nil inputs |
| Error | Error conditions properly reported | File read errors, parse failures |
| Edge Cases | Boundary conditions | Max int, empty input, unicode |

---

## Task 1.1: Token Estimation Utility

### Files to Create
- `internal/embedder/tokens.go`
- `internal/embedder/tokens_test.go`

### Test Specifications

Write these tests FIRST in `internal/embedder/tokens_test.go`:

```go
package embedder

import (
	"strings"
	"testing"
	"unicode/utf8"

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
```

### Implementation

After tests are written and failing, implement in `internal/embedder/tokens.go`:

```go
package embedder

// EstimateTokens approximates token count from text length.
// For code, ~4 characters per token is a reasonable approximation.
// We use 3.5 to be conservative (better to split early than fail).
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return int(float64(len(text)) / 3.5)
}

// EstimateChars converts token count to approximate character count.
func EstimateChars(tokens int) int {
	return int(float64(tokens) * 3.5)
}

// MaxCharsForTokens returns the maximum characters that fit in the given token budget.
// Uses a slightly more conservative multiplier for safety.
func MaxCharsForTokens(tokens int) int {
	return int(float64(tokens) * 3.2)
}
```

### Verification Commands

```bash
# 1. Write tests first
# 2. Run tests - should FAIL
go test -v ./internal/embedder/ -run TestEstimateTokens
go test -v ./internal/embedder/ -run TestEstimateChars
go test -v ./internal/embedder/ -run TestMaxCharsForTokens

# 3. Implement code
# 4. Run tests - should PASS
go test -v ./internal/embedder/ -run "Token"

# 5. Run with race detection
go test -race ./internal/embedder/ -run "Token"
```

---

## Task 1.2: Provider Context Limits

### Files to Modify
- `internal/embedder/provider.go`
- `internal/embedder/provider_test.go`

### Test Specifications

Add these tests FIRST to `internal/embedder/provider_test.go`:

```go
// =============================================================================
// MaxContextTokens Tests
// =============================================================================

// --- Happy Path Tests ---

func TestProviderType_MaxContextTokens_Ollama(t *testing.T) {
	assert.Equal(t, 8000, ProviderOllama.MaxContextTokens())
}

func TestProviderType_MaxContextTokens_OllamaRemote(t *testing.T) {
	assert.Equal(t, 8000, ProviderOllamaRemote.MaxContextTokens())
}

func TestProviderType_MaxContextTokens_OpenAI(t *testing.T) {
	assert.Equal(t, 8000, ProviderOpenAI.MaxContextTokens())
}

func TestProviderType_MaxContextTokens_Voyage(t *testing.T) {
	// Voyage has larger context
	assert.Equal(t, 15000, ProviderVoyage.MaxContextTokens())
}

// --- Edge Case Tests ---

func TestProviderType_MaxContextTokens_UnknownProvider(t *testing.T) {
	// Unknown provider should return conservative default
	unknown := ProviderType("unknown-provider")
	assert.Equal(t, 8000, unknown.MaxContextTokens())
}

func TestProviderType_MaxContextTokens_EmptyString(t *testing.T) {
	empty := ProviderType("")
	assert.Equal(t, 8000, empty.MaxContextTokens())
}

// --- Consistency Tests ---

func TestProviderType_MaxContextTokens_AllProvidersHaveLimits(t *testing.T) {
	providers := []ProviderType{
		ProviderOllama,
		ProviderOllamaRemote,
		ProviderOpenAI,
		ProviderVoyage,
	}

	for _, p := range providers {
		t.Run(string(p), func(t *testing.T) {
			limit := p.MaxContextTokens()
			assert.Greater(t, limit, 0, "Provider %s should have positive context limit", p)
			assert.LessOrEqual(t, limit, 20000, "Provider %s limit seems too high", p)
		})
	}
}

func TestProviderType_MaxContextTokens_HasSafetyMargin(t *testing.T) {
	// Verify limits have safety margin (not exact API limits)
	// OpenAI: 8191 actual, we use 8000
	// Voyage: 16000 actual, we use 15000
	assert.Less(t, ProviderOpenAI.MaxContextTokens(), 8191)
	assert.Less(t, ProviderVoyage.MaxContextTokens(), 16000)
}
```

### Implementation

Add to `internal/embedder/provider.go`:

```go
// MaxContextTokens returns the maximum context window size in tokens for this provider.
// Returns a conservative limit with safety margin to prevent failures.
func (p ProviderType) MaxContextTokens() int {
	switch p {
	case ProviderOpenAI:
		return 8000 // text-embedding-3-small: 8191 minus safety margin
	case ProviderVoyage:
		return 15000 // voyage-code-3: 16000 minus safety margin
	default: // ProviderOllama, ProviderOllamaRemote, unknown
		return 8000 // Jina v2: 8192 minus safety margin
	}
}
```

---

## Task 1.3: Minified File Detection

### Files to Create
- `internal/chunker/minified.go`
- `internal/chunker/minified_test.go`

### Test Specifications

Write these tests FIRST in `internal/chunker/minified_test.go`:

```go
package chunker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// IsMinified - Filename Detection Tests
// =============================================================================

// --- Happy Path Tests ---

func TestIsMinified_MinJsFilename(t *testing.T) {
	content := []byte("var x=1;")
	assert.True(t, IsMinified(content, "app.min.js"))
}

func TestIsMinified_MinCssFilename(t *testing.T) {
	content := []byte("body{margin:0}")
	assert.True(t, IsMinified(content, "styles.min.css"))
}

func TestIsMinified_NormalJsFilename(t *testing.T) {
	content := []byte("function hello() {\n    console.log('hi');\n}\n")
	assert.False(t, IsMinified(content, "app.js"))
}

// --- Success Tests ---

func TestIsMinified_MinInPath(t *testing.T) {
	content := []byte("var x=1;")
	assert.True(t, IsMinified(content, "dist/js/vendor.min.js"))
}

func TestIsMinified_MinMapFile(t *testing.T) {
	content := []byte("{}")
	assert.True(t, IsMinified(content, "app.min.js.map"))
}

// --- Edge Case Tests ---

func TestIsMinified_MinimumInName(t *testing.T) {
	// "minimum" contains "min" but not ".min."
	content := []byte("function minimum() {}\n")
	assert.False(t, IsMinified(content, "minimum.js"))
}

func TestIsMinified_AdminFile(t *testing.T) {
	// "admin" contains "min" but not ".min."
	content := []byte("function adminPanel() {}\n")
	assert.False(t, IsMinified(content, "admin.js"))
}

func TestIsMinified_CaseSensitivity(t *testing.T) {
	content := []byte("var x=1;")
	// .min. detection should work regardless of case in path
	assert.True(t, IsMinified(content, "App.Min.JS"))
	assert.True(t, IsMinified(content, "APP.MIN.JS"))
}

// =============================================================================
// IsMinified - Line Length Detection Tests
// =============================================================================

// --- Happy Path Tests ---

func TestIsMinified_VeryLongAverageLineLength(t *testing.T) {
	// Average line > 500 chars indicates minification
	longLine := strings.Repeat("x", 600)
	content := []byte(longLine + "\n" + longLine + "\n")
	assert.True(t, IsMinified(content, "bundle.js"))
}

func TestIsMinified_NormalLineLength(t *testing.T) {
	content := []byte(`package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
`)
	assert.False(t, IsMinified(content, "main.go"))
}

// --- Edge Case Tests ---

func TestIsMinified_SingleVeryLongLine(t *testing.T) {
	// Single line > 10KB
	content := []byte(strings.Repeat("x", 15*1024))
	assert.True(t, IsMinified(content, "bundle.js"))
}

func TestIsMinified_SingleShortLine(t *testing.T) {
	content := []byte("console.log('hello');")
	assert.False(t, IsMinified(content, "one-liner.js"))
}

func TestIsMinified_ManyShortLines(t *testing.T) {
	lines := strings.Repeat("x\n", 1000) // 1000 lines of 1 char each
	assert.False(t, IsMinified([]byte(lines), "many-lines.txt"))
}

func TestIsMinified_ExactlyAtThreshold(t *testing.T) {
	// Exactly 500 char average - should NOT be minified
	line := strings.Repeat("x", 500)
	content := []byte(line + "\n" + line + "\n")
	assert.False(t, IsMinified(content, "border.js"))
}

func TestIsMinified_JustOverThreshold(t *testing.T) {
	// 501 char average - should be minified
	line := strings.Repeat("x", 501)
	content := []byte(line + "\n" + line + "\n")
	assert.True(t, IsMinified(content, "border.js"))
}

// =============================================================================
// IsMinified - Whitespace Ratio Detection Tests
// =============================================================================

// --- Happy Path Tests ---

func TestIsMinified_LowWhitespaceRatio(t *testing.T) {
	// Create content with < 5% whitespace
	// 2000 chars total, only 50 spaces = 2.5%
	content := make([]byte, 2000)
	for i := range content {
		content[i] = 'a'
	}
	for i := 0; i < 50; i++ {
		content[i*40] = ' '
	}
	assert.True(t, IsMinified(content, "compressed.js"))
}

func TestIsMinified_NormalWhitespaceRatio(t *testing.T) {
	// Normal code has 15-25% whitespace
	content := []byte(`package main

import (
    "fmt"
    "os"
)

func main() {
    args := os.Args
    for _, arg := range args {
        fmt.Println(arg)
    }
}
`)
	assert.False(t, IsMinified(content, "main.go"))
}

// --- Edge Case Tests ---

func TestIsMinified_AllWhitespace(t *testing.T) {
	content := []byte(strings.Repeat(" \t\n", 500))
	assert.False(t, IsMinified(content, "whitespace.txt"))
}

func TestIsMinified_SmallFileSkipsWhitespaceCheck(t *testing.T) {
	// Files < 1KB skip whitespace ratio check
	content := []byte("xxxxxxxxxxxxxxxx") // 16 chars, 0% whitespace
	assert.False(t, IsMinified(content, "tiny.js"))
}

func TestIsMinified_ExactlyAtSizeThreshold(t *testing.T) {
	// Exactly 1024 bytes with low whitespace
	content := make([]byte, 1024)
	for i := range content {
		content[i] = 'x'
	}
	assert.True(t, IsMinified(content, "threshold.js"))
}

// =============================================================================
// IsMinified - Empty and Special Cases
// =============================================================================

// --- Failure Tests (graceful handling) ---

func TestIsMinified_EmptyFile(t *testing.T) {
	assert.False(t, IsMinified([]byte{}, "empty.js"))
}

func TestIsMinified_EmptyPath(t *testing.T) {
	content := []byte("var x = 1;\n")
	assert.False(t, IsMinified(content, ""))
}

func TestIsMinified_NilContent(t *testing.T) {
	// Should handle nil gracefully
	assert.False(t, IsMinified(nil, "file.js"))
}

// --- Edge Case Tests ---

func TestIsMinified_BinaryContent(t *testing.T) {
	binary := make([]byte, 2000)
	for i := range binary {
		binary[i] = byte(i % 256)
	}
	// Binary might trigger minified detection, which is acceptable
	// Main thing is it shouldn't panic
	_ = IsMinified(binary, "data.bin")
}

func TestIsMinified_UnicodeContent(t *testing.T) {
	content := []byte("函数 こんにちは() { return '世界'; }\n")
	assert.False(t, IsMinified(content, "unicode.js"))
}

func TestIsMinified_OnlyNullBytes(t *testing.T) {
	content := make([]byte, 100)
	// Should handle gracefully
	_ = IsMinified(content, "nulls.bin")
}

// =============================================================================
// IsMinified - Real-World Examples
// =============================================================================

func TestIsMinified_RealMinifiedJavaScript(t *testing.T) {
	// Simulated minified JS from a real bundler
	minified := `!function(e,t){"object"==typeof exports&&"undefined"!=typeof module?module.exports=t():"function"==typeof define&&define.amd?define(t):(e="undefined"!=typeof globalThis?globalThis:e||self).Vue=t()}(this,(function(){"use strict";`
	content := []byte(strings.Repeat(minified, 50))
	assert.True(t, IsMinified(content, "vue.runtime.js"))
}

func TestIsMinified_RealNormalTypeScript(t *testing.T) {
	content := []byte(`import { useState, useEffect } from 'react';

interface User {
    id: number;
    name: string;
    email: string;
}

export function useUser(userId: number): User | null {
    const [user, setUser] = useState<User | null>(null);

    useEffect(() => {
        fetch('/api/users/' + userId)
            .then(res => res.json())
            .then(data => setUser(data));
    }, [userId]);

    return user;
}
`)
	assert.False(t, IsMinified(content, "useUser.ts"))
}

func TestIsMinified_RealMinifiedCSS(t *testing.T) {
	minified := `body{margin:0;padding:0;font-family:Arial,sans-serif}.container{max-width:1200px;margin:0 auto;padding:20px}.header{background:#333;color:#fff;padding:10px 20px}`
	content := []byte(strings.Repeat(minified, 30))
	assert.True(t, IsMinified(content, "styles.css"))
}

func TestIsMinified_RealNormalCSS(t *testing.T) {
	content := []byte(`/* Main styles */
body {
    margin: 0;
    padding: 0;
    font-family: Arial, sans-serif;
}

.container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 20px;
}

.header {
    background: #333;
    color: #fff;
    padding: 10px 20px;
}
`)
	assert.False(t, IsMinified(content, "styles.css"))
}

// =============================================================================
// IsMinifiedExtension Tests
// =============================================================================

func TestIsMinifiedExtension_CommonPatterns(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"app.min.js", true},
		{"style.min.css", true},
		{"vendor.min.js.map", false}, // .map is not in list
		{"app.bundle.js", true},
		{"vendor.bundle.css", true},
		{"normal.js", false},
		{"normal.css", false},
		{"main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsMinifiedExtension(tt.path))
		})
	}
}

func TestIsMinifiedExtension_CaseInsensitive(t *testing.T) {
	assert.True(t, IsMinifiedExtension("APP.MIN.JS"))
	assert.True(t, IsMinifiedExtension("Style.Min.Css"))
	assert.True(t, IsMinifiedExtension("VENDOR.BUNDLE.JS"))
}

func TestIsMinifiedExtension_EmptyPath(t *testing.T) {
	assert.False(t, IsMinifiedExtension(""))
}

// =============================================================================
// Custom Thresholds Tests
// =============================================================================

func TestIsMinifiedWithThresholds_StricterLineLength(t *testing.T) {
	thresholds := MinifiedThresholds{
		MaxAvgLineLength:          100, // Stricter
		MaxSingleLineSize:         10 * 1024,
		MinWhitespaceRatio:        0.05,
		MinSizeForWhitespaceCheck: 1024,
	}

	// 150 char average - passes default, fails custom
	line := strings.Repeat("x", 150)
	content := []byte(line + "\n" + line + "\n")

	assert.False(t, IsMinified(content, "code.js"))
	assert.True(t, IsMinifiedWithThresholds(content, "code.js", thresholds))
}

func TestIsMinifiedWithThresholds_StricterWhitespace(t *testing.T) {
	thresholds := MinifiedThresholds{
		MaxAvgLineLength:          500,
		MaxSingleLineSize:         10 * 1024,
		MinWhitespaceRatio:        0.15, // Stricter - require 15% whitespace
		MinSizeForWhitespaceCheck: 500,
	}

	// 10% whitespace - passes default (5%), fails custom (15%)
	content := make([]byte, 1000)
	for i := range content {
		if i%10 == 0 {
			content[i] = ' '
		} else {
			content[i] = 'x'
		}
	}

	assert.False(t, IsMinified(content, "code.js"))
	assert.True(t, IsMinifiedWithThresholds(content, "code.js", thresholds))
}
```

### Implementation

After tests are written and failing, implement in `internal/chunker/minified.go`:

```go
package chunker

import (
	"bytes"
	"strings"
)

// MinifiedThresholds contains the thresholds for minified file detection.
type MinifiedThresholds struct {
	MaxAvgLineLength          int
	MaxSingleLineSize         int
	MinWhitespaceRatio        float64
	MinSizeForWhitespaceCheck int
}

// DefaultMinifiedThresholds returns the default detection thresholds.
func DefaultMinifiedThresholds() MinifiedThresholds {
	return MinifiedThresholds{
		MaxAvgLineLength:          500,
		MaxSingleLineSize:         10 * 1024,
		MinWhitespaceRatio:        0.05,
		MinSizeForWhitespaceCheck: 1024,
	}
}

// IsMinified detects if file content appears to be minified/compressed code.
func IsMinified(content []byte, path string) bool {
	return IsMinifiedWithThresholds(content, path, DefaultMinifiedThresholds())
}

// IsMinifiedWithThresholds allows custom thresholds for testing.
func IsMinifiedWithThresholds(content []byte, path string, t MinifiedThresholds) bool {
	if len(content) == 0 {
		return false
	}

	// Check filename hint
	pathLower := strings.ToLower(path)
	if strings.Contains(pathLower, ".min.") {
		return true
	}

	// Count lines
	lineCount := bytes.Count(content, []byte("\n"))
	if lineCount == 0 {
		lineCount = 1
	}

	// Check average line length
	avgLineLength := len(content) / lineCount
	if avgLineLength > t.MaxAvgLineLength {
		return true
	}

	// Check single-line file size
	if lineCount == 1 && len(content) > t.MaxSingleLineSize {
		return true
	}

	// Check whitespace ratio for larger files
	if len(content) >= t.MinSizeForWhitespaceCheck {
		whitespaceCount := bytes.Count(content, []byte(" ")) +
			bytes.Count(content, []byte("\t")) +
			bytes.Count(content, []byte("\n"))
		whitespaceRatio := float64(whitespaceCount) / float64(len(content))

		if whitespaceRatio < t.MinWhitespaceRatio {
			return true
		}
	}

	return false
}

// MinifiedExtensions contains known minified file extensions.
var MinifiedExtensions = []string{
	".min.js",
	".min.css",
	".bundle.js",
	".bundle.css",
}

// IsMinifiedExtension checks if the path has a known minified extension.
func IsMinifiedExtension(path string) bool {
	pathLower := strings.ToLower(path)
	for _, ext := range MinifiedExtensions {
		if strings.HasSuffix(pathLower, ext) {
			return true
		}
	}
	return false
}
```

---

## Task 1.4: Database Schema Migration

### Files to Modify
- `internal/db/schema.go`
- `internal/db/schema_test.go`

### Test Specifications

Add these tests FIRST to `internal/db/schema_test.go`:

```go
// =============================================================================
// Migration V4 Tests - Chunk Splitting Support
// =============================================================================

// --- Happy Path Tests ---

func TestMigrateV4_AddsParentChunkIDColumn(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	var count int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('chunks')
		WHERE name = 'parent_chunk_id'
	`).Scan(&count)

	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestMigrateV4_AddsChunkIndexColumn(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	var count int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('chunks')
		WHERE name = 'chunk_index'
	`).Scan(&count)

	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestMigrateV4_AddsIsPartialColumn(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	var count int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('chunks')
		WHERE name = 'is_partial'
	`).Scan(&count)

	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestMigrateV4_CreatesParentIndex(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	var indexName string
	err := db.conn.QueryRowContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type='index' AND name='idx_chunks_parent'
	`).Scan(&indexName)

	require.NoError(t, err)
	assert.Equal(t, "idx_chunks_parent", indexName)
}

// --- Success Tests ---

func TestMigrateV4_ColumnsHaveCorrectDefaults(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert a chunk without specifying new columns
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO chunks (id, file_id, name, level, start_line, end_line, content, hash)
		VALUES ('test-1', 'file-1', 'test', 'method', 1, 10, 'content', 'hash123')
	`)
	require.NoError(t, err)

	// Verify defaults
	var parentID sql.NullString
	var chunkIndex int
	var isPartial int

	err = db.conn.QueryRowContext(ctx, `
		SELECT parent_chunk_id, chunk_index, is_partial FROM chunks WHERE id = 'test-1'
	`).Scan(&parentID, &chunkIndex, &isPartial)

	require.NoError(t, err)
	assert.False(t, parentID.Valid, "parent_chunk_id should be NULL by default")
	assert.Equal(t, 0, chunkIndex, "chunk_index should default to 0")
	assert.Equal(t, 0, isPartial, "is_partial should default to 0 (false)")
}

func TestMigrateV4_CanInsertSplitChunk(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert parent chunk
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO chunks (id, file_id, name, level, start_line, end_line, content, hash)
		VALUES ('parent-1', 'file-1', 'bigMethod', 'method', 1, 100, 'content', 'hash1')
	`)
	require.NoError(t, err)

	// Insert split chunks
	_, err = db.conn.ExecContext(ctx, `
		INSERT INTO chunks (id, file_id, name, level, start_line, end_line, content, hash,
		                    parent_chunk_id, chunk_index, is_partial)
		VALUES ('split-0', 'file-1', 'bigMethod', 'method', 1, 50, 'content1', 'hash2',
		        'parent-1', 0, 1)
	`)
	require.NoError(t, err)

	_, err = db.conn.ExecContext(ctx, `
		INSERT INTO chunks (id, file_id, name, level, start_line, end_line, content, hash,
		                    parent_chunk_id, chunk_index, is_partial)
		VALUES ('split-1', 'file-1', 'bigMethod', 'method', 40, 100, 'content2', 'hash3',
		        'parent-1', 1, 1)
	`)
	require.NoError(t, err)

	// Query splits by parent
	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, chunk_index FROM chunks
		WHERE parent_chunk_id = 'parent-1'
		ORDER BY chunk_index
	`)
	require.NoError(t, err)
	defer rows.Close()

	var splits []struct {
		ID    string
		Index int
	}
	for rows.Next() {
		var s struct {
			ID    string
			Index int
		}
		require.NoError(t, rows.Scan(&s.ID, &s.Index))
		splits = append(splits, s)
	}

	assert.Len(t, splits, 2)
	assert.Equal(t, "split-0", splits[0].ID)
	assert.Equal(t, 0, splits[0].Index)
	assert.Equal(t, "split-1", splits[1].ID)
	assert.Equal(t, 1, splits[1].Index)
}

// --- Idempotency Tests ---

func TestMigrateV4_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Run migration multiple times
	for i := 0; i < 3; i++ {
		err := db.migrateV4(ctx)
		require.NoError(t, err, "Migration %d should succeed", i+1)
	}

	// Verify columns still exist (exactly once each)
	var count int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('chunks')
		WHERE name IN ('parent_chunk_id', 'chunk_index', 'is_partial')
	`).Scan(&count)

	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

// --- Edge Case Tests ---

func TestMigrateV4_IndexUsedInQueries(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert many chunks with parent relationships
	for i := 0; i < 100; i++ {
		parentID := fmt.Sprintf("parent-%d", i/5)
		_, err := db.conn.ExecContext(ctx, `
			INSERT INTO chunks (id, file_id, name, level, start_line, end_line, content, hash,
			                    parent_chunk_id, chunk_index, is_partial)
			VALUES (?, 'file-1', 'method', 'method', 1, 10, 'content', ?,
			        ?, ?, 1)
		`, fmt.Sprintf("chunk-%d", i), fmt.Sprintf("hash-%d", i), parentID, i%5)
		require.NoError(t, err)
	}

	// Query by parent should use index (check EXPLAIN QUERY PLAN)
	rows, err := db.conn.QueryContext(ctx, `
		EXPLAIN QUERY PLAN
		SELECT * FROM chunks WHERE parent_chunk_id = 'parent-0'
	`)
	require.NoError(t, err)
	defer rows.Close()

	var usesIndex bool
	for rows.Next() {
		var id, parent, notused int
		var detail string
		rows.Scan(&id, &parent, &notused, &detail)
		if strings.Contains(detail, "idx_chunks_parent") {
			usesIndex = true
		}
	}
	assert.True(t, usesIndex, "Query should use idx_chunks_parent index")
}
```

### Implementation

Add to `internal/db/schema.go`:

```go
// migrateV4 adds chunk splitting support.
func (db *DB) migrateV4(ctx context.Context) error {
	migrations := []string{
		`ALTER TABLE chunks ADD COLUMN parent_chunk_id TEXT`,
		`ALTER TABLE chunks ADD COLUMN chunk_index INTEGER DEFAULT 0`,
		`ALTER TABLE chunks ADD COLUMN is_partial INTEGER DEFAULT 0`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_parent ON chunks(parent_chunk_id)`,
	}

	for _, sql := range migrations {
		if _, err := db.conn.ExecContext(ctx, sql); err != nil {
			// Ignore "duplicate column" errors for idempotency
			if !strings.Contains(err.Error(), "duplicate column") &&
				!strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("migration V4 failed: %w", err)
			}
		}
	}

	return nil
}
```

---

## Task 1.5: Update Chunk Model

### Files to Modify
- `internal/models/chunk.go`
- `internal/models/chunk_test.go`

### Test Specifications

Add these tests FIRST to `internal/models/chunk_test.go`:

```go
// =============================================================================
// Chunk Split Field Tests
// =============================================================================

// --- Happy Path Tests ---

func TestChunk_IsSplit_WithParent(t *testing.T) {
	chunk := Chunk{
		ID:            "chunk-1-split-0",
		ParentChunkID: "chunk-1",
		ChunkIndex:    0,
		IsPartial:     true,
	}
	assert.True(t, chunk.IsSplit())
}

func TestChunk_IsSplit_NoParent(t *testing.T) {
	chunk := Chunk{
		ID:            "chunk-1",
		ParentChunkID: "",
		ChunkIndex:    0,
		IsPartial:     false,
	}
	assert.False(t, chunk.IsSplit())
}

// --- Edge Case Tests ---

func TestChunk_IsSplit_EmptyParentID(t *testing.T) {
	chunk := Chunk{ParentChunkID: ""}
	assert.False(t, chunk.IsSplit())
}

func TestChunk_IsSplit_WhitespaceParentID(t *testing.T) {
	// Whitespace-only parent ID should be treated as no parent
	chunk := Chunk{ParentChunkID: "   "}
	// Implementation should trim, but if not, still works
	assert.True(t, chunk.IsSplit()) // Has non-empty string
}

// --- JSON Serialization Tests ---

func TestChunk_JSONSerialization_WithSplitFields(t *testing.T) {
	chunk := Chunk{
		ID:            "split-0",
		ParentChunkID: "parent-1",
		ChunkIndex:    0,
		IsPartial:     true,
	}

	data, err := json.Marshal(chunk)
	require.NoError(t, err)

	var decoded Chunk
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "parent-1", decoded.ParentChunkID)
	assert.Equal(t, 0, decoded.ChunkIndex)
	assert.True(t, decoded.IsPartial)
}

func TestChunk_JSONSerialization_OmitsEmptyParentID(t *testing.T) {
	chunk := Chunk{
		ID:            "chunk-1",
		ParentChunkID: "",
	}

	data, err := json.Marshal(chunk)
	require.NoError(t, err)

	// Should omit parent_chunk_id when empty
	assert.NotContains(t, string(data), "parent_chunk_id")
}

// --- Default Values Tests ---

func TestChunk_DefaultValues(t *testing.T) {
	chunk := Chunk{}

	assert.Equal(t, "", chunk.ParentChunkID)
	assert.Equal(t, 0, chunk.ChunkIndex)
	assert.False(t, chunk.IsPartial)
	assert.False(t, chunk.IsSplit())
}
```

### Implementation

Update `internal/models/chunk.go`:

```go
// Chunk represents a piece of code extracted from a source file.
type Chunk struct {
	ID        string     `json:"id"`
	FileID    string     `json:"file_id"`
	FilePath  string     `json:"file_path"`
	Name      string     `json:"name"`
	Level     ChunkLevel `json:"level"`
	StartLine int        `json:"start_line"`
	EndLine   int        `json:"end_line"`
	Content   string     `json:"content"`
	Hash      string     `json:"hash"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// Split chunk fields
	ParentChunkID string `json:"parent_chunk_id,omitempty"`
	ChunkIndex    int    `json:"chunk_index"`
	IsPartial     bool   `json:"is_partial"`
}

// IsSplit returns true if this chunk is part of a split.
func (c *Chunk) IsSplit() bool {
	return c.ParentChunkID != ""
}
```

---

## Verification Checklist

Run all tests after each task:

```bash
# Task 1.1: Token Estimation
go test -v -race ./internal/embedder/ -run "Token|Chars"

# Task 1.2: Provider Limits
go test -v -race ./internal/embedder/ -run "MaxContextTokens"

# Task 1.3: Minified Detection
go test -v -race ./internal/chunker/ -run "Minified"

# Task 1.4: Database Migration
go test -v -race ./internal/db/ -run "MigrateV4"

# Task 1.5: Chunk Model
go test -v -race ./internal/models/ -run "Chunk.*Split"

# Run ALL Phase 1 tests
go test -v -race ./internal/embedder/... ./internal/chunker/... ./internal/db/... ./internal/models/...
```

## Acceptance Criteria

| Criterion | Test Coverage |
|-----------|---------------|
| Token estimation works for empty string | ✅ TestEstimateTokens_EmptyString |
| Token estimation works for typical code | ✅ TestEstimateTokens_TypicalGoFunction |
| Token estimation works for large files | ✅ TestEstimateTokens_LargeCodeFile |
| Token estimation handles unicode | ✅ TestEstimateTokens_UnicodeCharacters |
| All providers have context limits | ✅ TestProviderType_MaxContextTokens_* |
| Unknown provider has safe default | ✅ TestProviderType_MaxContextTokens_UnknownProvider |
| Minified detection by filename | ✅ TestIsMinified_MinJsFilename |
| Minified detection by line length | ✅ TestIsMinified_VeryLongAverageLineLength |
| Minified detection by whitespace | ✅ TestIsMinified_LowWhitespaceRatio |
| Normal code not falsely detected | ✅ TestIsMinified_RealNormalTypeScript |
| Empty file handled gracefully | ✅ TestIsMinified_EmptyFile |
| DB migration adds columns | ✅ TestMigrateV4_Adds*Column |
| DB migration is idempotent | ✅ TestMigrateV4_Idempotent |
| Chunk model has split fields | ✅ TestChunk_IsSplit_* |

## Next Phase

After all Phase 1 tests pass, proceed to [Phase 2: Splitting Logic](./chunk-splitting-phase-2.md).
