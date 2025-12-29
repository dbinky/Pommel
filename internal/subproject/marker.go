package subproject

import (
	"path/filepath"
	"strings"
)

// DefaultPriority is the default priority for unknown marker files.
const DefaultPriority = 999

// DefaultMarkerPatterns are the default marker file patterns for sub-project detection.
// These are used when no custom markers are configured.
var DefaultMarkerPatterns = []string{
	"*.sln",
	"*.csproj",
	"go.mod",
	"Cargo.toml",
	"pom.xml",
	"build.gradle",
	"package.json",
	"pyproject.toml",
	"setup.py",
}

// MatchesMarkerPattern checks if a filename matches a marker pattern.
// Patterns starting with "*" match as suffix patterns (e.g., "*.sln" matches "Foo.sln").
// Other patterns require exact filename match.
func MatchesMarkerPattern(filename, pattern string) bool {
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(filename, pattern[1:])
	}
	return filename == pattern
}

// IsMarkerFile checks if a filename is a sub-project marker using the provided patterns.
// If no patterns are provided, DefaultMarkerPatterns is used.
func IsMarkerFile(filename string, patterns []string) bool {
	if len(patterns) == 0 {
		patterns = DefaultMarkerPatterns
	}

	for _, pattern := range patterns {
		if MatchesMarkerPattern(filename, pattern) {
			return true
		}
	}
	return false
}

// GetLanguageHint returns the language hint for a marker file.
func GetLanguageHint(markerFile string) string {
	switch markerFile {
	case "go.mod":
		return "go"
	case "package.json":
		return "javascript"
	case "Cargo.toml":
		return "rust"
	case "pom.xml", "build.gradle":
		return "java"
	case "pyproject.toml", "setup.py":
		return "python"
	default:
		if strings.HasSuffix(markerFile, ".sln") || strings.HasSuffix(markerFile, ".csproj") {
			return "csharp"
		}
		return ""
	}
}

// GenerateSubprojectID creates a slug-style ID from a path.
// Uses the last component of the path (base name).
func GenerateSubprojectID(path string) string {
	base := filepath.Base(path)
	id := strings.ToLower(base)
	id = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, id)
	return id
}
