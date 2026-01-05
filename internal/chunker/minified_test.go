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
	// Note: avgLineLength = len(content) / lineCount
	// For 500*2 + 2 newlines = 1002 bytes / 2 lines = 501, which IS over threshold
	// So we use 499 to get exactly 500: (499*2 + 2) / 2 = 500
	line := strings.Repeat("x", 499)
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
	// Need multiple lines to avoid triggering line length check
	var content []byte
	for i := 0; i < 20; i++ {
		// Each line: 45 'x' + 5 spaces = 50 chars + newline
		// Total: 20 * 51 = 1020 bytes, ~10% whitespace (100 spaces + 20 newlines = 120/1020)
		line := make([]byte, 50)
		for j := range line {
			if j%10 == 0 {
				line[j] = ' '
			} else {
				line[j] = 'x'
			}
		}
		content = append(content, line...)
		content = append(content, '\n')
	}

	assert.False(t, IsMinified(content, "code.js"))
	assert.True(t, IsMinifiedWithThresholds(content, "code.js", thresholds))
}
