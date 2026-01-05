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
