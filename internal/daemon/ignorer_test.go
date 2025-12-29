package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewIgnorer verifies that NewIgnorer creates an ignorer successfully
func TestNewIgnorer(t *testing.T) {
	tmpDir := t.TempDir()

	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)
	require.NotNil(t, ignorer)
}

// TestNewIgnorerWithConfigPatterns verifies ignorer accepts config patterns
func TestNewIgnorerWithConfigPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	patterns := []string{"*.log", "temp/", "**/*.tmp"}
	ignorer, err := NewIgnorer(tmpDir, patterns)
	require.NoError(t, err)
	require.NotNil(t, ignorer)
}

// TestShouldIgnoreExactFilename verifies exact filename matching
func TestShouldIgnoreExactFilename(t *testing.T) {
	tmpDir := t.TempDir()

	patterns := []string{".DS_Store", "Thumbs.db", ".env"}
	ignorer, err := NewIgnorer(tmpDir, patterns)
	require.NoError(t, err)

	// Should ignore exact matches
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".DS_Store")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "Thumbs.db")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".env")))

	// Should not ignore non-matching files
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "main.go")))
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".env.example")))
}

// TestShouldIgnoreGlobPattern verifies glob pattern matching (*.log)
func TestShouldIgnoreGlobPattern(t *testing.T) {
	tmpDir := t.TempDir()

	patterns := []string{"*.log", "*.bak", "*.tmp"}
	ignorer, err := NewIgnorer(tmpDir, patterns)
	require.NoError(t, err)

	// Should ignore files matching glob
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "app.log")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "error.log")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "file.bak")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "data.tmp")))

	// Should not ignore non-matching files
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "main.go")))
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "logger.go")))
}

// TestShouldIgnoreDirectoryPattern verifies directory pattern matching (node_modules/)
func TestShouldIgnoreDirectoryPattern(t *testing.T) {
	tmpDir := t.TempDir()

	patterns := []string{"node_modules/", "vendor/", ".git/", "build/"}
	ignorer, err := NewIgnorer(tmpDir, patterns)
	require.NoError(t, err)

	// Should ignore directories and their contents
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "node_modules")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "node_modules", "lodash")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "node_modules", "lodash", "index.js")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "vendor", "github.com", "pkg")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".git", "objects")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "build", "output.js")))

	// Should not ignore non-matching paths
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "src", "main.go")))
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "internal", "node_modules.go")))
}

// TestShouldIgnoreDoubleStarPattern verifies ** pattern matching (**/*.tmp)
func TestShouldIgnoreDoubleStarPattern(t *testing.T) {
	tmpDir := t.TempDir()

	patterns := []string{"**/*.tmp", "**/*.cache", "**/test_*.go"}
	ignorer, err := NewIgnorer(tmpDir, patterns)
	require.NoError(t, err)

	// Should ignore files matching ** pattern at any depth
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "file.tmp")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "subdir", "file.tmp")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "a", "b", "c", "file.tmp")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "data.cache")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "pkg", "test_utils.go")))

	// Should not ignore non-matching files
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "main.go")))
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "subdir", "utils.go")))
}

// TestLoadsPatternsFromPommelignore verifies loading from .pommelignore file
func TestLoadsPatternsFromPommelignore(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .pommelignore file
	pommelignore := filepath.Join(tmpDir, ".pommelignore")
	content := `# Comment line
*.log
node_modules/
**/*.tmp

# Another comment
.env
`
	err := os.WriteFile(pommelignore, []byte(content), 0644)
	require.NoError(t, err)

	// Create ignorer without explicit patterns (should load from file)
	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	// Should ignore patterns from .pommelignore
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "app.log")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "node_modules", "pkg")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "subdir", "file.tmp")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".env")))

	// Should not ignore non-matching
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "main.go")))
}

// TestLoadsPatternsFromGitignore verifies loading from .gitignore file
func TestLoadsPatternsFromGitignore(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gitignore file (no .pommelignore)
	gitignore := filepath.Join(tmpDir, ".gitignore")
	content := `# Build outputs
bin/
dist/

# Dependencies
vendor/

# IDE
.vscode/
.idea/
`
	err := os.WriteFile(gitignore, []byte(content), 0644)
	require.NoError(t, err)

	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	// Should ignore patterns from .gitignore
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "bin", "app")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "dist", "bundle.js")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "vendor", "pkg")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".vscode", "settings.json")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".idea", "workspace.xml")))

	// Should not ignore source files
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "src", "main.go")))
}

// TestPommelignoreOverridesGitignore verifies .pommelignore takes precedence
func TestPommelignoreAndGitignoreCombined(t *testing.T) {
	tmpDir := t.TempDir()

	// Create both files
	gitignore := filepath.Join(tmpDir, ".gitignore")
	gitContent := `bin/
*.log
`
	err := os.WriteFile(gitignore, []byte(gitContent), 0644)
	require.NoError(t, err)

	pommelignore := filepath.Join(tmpDir, ".pommelignore")
	pommelContent := `vendor/
*.tmp
`
	err = os.WriteFile(pommelignore, []byte(pommelContent), 0644)
	require.NoError(t, err)

	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	// Should ignore patterns from both files
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "bin", "app")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "app.log")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "vendor", "pkg")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "file.tmp")))
}

// TestAlwaysIgnoresPommelDirectory verifies .pommel directory is always ignored
func TestAlwaysIgnoresPommelDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// No explicit patterns - should still ignore .pommel
	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	// .pommel directory should always be ignored
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".pommel")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".pommel", "config.yaml")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".pommel", "chroma", "data")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".pommel", "state.json")))
}

// TestAlwaysIgnoresPommelDirectoryEvenWithPatterns verifies .pommel ignored regardless of patterns
func TestAlwaysIgnoresPommelDirectoryEvenWithPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Patterns that don't include .pommel
	patterns := []string{"*.log", "vendor/"}
	ignorer, err := NewIgnorer(tmpDir, patterns)
	require.NoError(t, err)

	// .pommel should still be ignored
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".pommel")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".pommel", "config.yaml")))
}

// TestHandlesRelativePaths verifies handling of relative paths
func TestHandlesRelativePaths(t *testing.T) {
	tmpDir := t.TempDir()

	patterns := []string{"*.log", "node_modules/"}
	ignorer, err := NewIgnorer(tmpDir, patterns)
	require.NoError(t, err)

	// Should handle relative paths within project
	assert.True(t, ignorer.ShouldIgnore("app.log"))
	assert.True(t, ignorer.ShouldIgnore("node_modules/pkg"))
	assert.True(t, ignorer.ShouldIgnore("subdir/file.log"))

	// Should not ignore non-matching relative paths
	assert.False(t, ignorer.ShouldIgnore("main.go"))
	assert.False(t, ignorer.ShouldIgnore("src/utils.go"))
}

// TestHandlesAbsolutePaths verifies handling of absolute paths
func TestHandlesAbsolutePaths(t *testing.T) {
	tmpDir := t.TempDir()

	patterns := []string{"*.log", "vendor/"}
	ignorer, err := NewIgnorer(tmpDir, patterns)
	require.NoError(t, err)

	// Should handle absolute paths within project
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "error.log")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "vendor", "github.com")))

	// Should not ignore non-matching absolute paths
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "main.go")))
}

// TestNegationPatterns verifies negation patterns (!)
func TestNegationPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Ignore all logs except important.log
	patterns := []string{"*.log", "!important.log"}
	ignorer, err := NewIgnorer(tmpDir, patterns)
	require.NoError(t, err)

	// Should ignore regular logs
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "debug.log")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "error.log")))

	// Should NOT ignore negated pattern
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "important.log")))
}

// TestEmptyPatterns verifies behavior with no patterns
func TestEmptyPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	ignorer, err := NewIgnorer(tmpDir, []string{})
	require.NoError(t, err)

	// With no patterns, nothing should be ignored (except .pommel)
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "main.go")))
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "app.log")))
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "node_modules", "pkg")))

	// .pommel should still be ignored
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, ".pommel")))
}

// TestCommentLinesIgnored verifies comment lines are ignored in ignore files
func TestCommentLinesIgnored(t *testing.T) {
	tmpDir := t.TempDir()

	pommelignore := filepath.Join(tmpDir, ".pommelignore")
	content := `# This is a comment
*.log
# Another comment
vendor/
`
	err := os.WriteFile(pommelignore, []byte(content), 0644)
	require.NoError(t, err)

	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	// Should ignore patterns but not treat comments as patterns
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "app.log")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "vendor", "pkg")))

	// Should not literally match comment text
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "# This is a comment")))
}

// TestBlankLinesIgnored verifies blank lines are ignored in ignore files
func TestBlankLinesIgnored(t *testing.T) {
	tmpDir := t.TempDir()

	pommelignore := filepath.Join(tmpDir, ".pommelignore")
	content := `*.log

vendor/

*.tmp
`
	err := os.WriteFile(pommelignore, []byte(content), 0644)
	require.NoError(t, err)

	ignorer, err := NewIgnorer(tmpDir, nil)
	require.NoError(t, err)

	// Should ignore patterns normally despite blank lines
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "app.log")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "vendor", "pkg")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "file.tmp")))
}

// TestConfigPatternsOverrideFiles verifies config patterns are combined with file patterns
func TestConfigPatternsWithFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .pommelignore
	pommelignore := filepath.Join(tmpDir, ".pommelignore")
	fileContent := `*.log
`
	err := os.WriteFile(pommelignore, []byte(fileContent), 0644)
	require.NoError(t, err)

	// Also provide config patterns
	configPatterns := []string{"*.tmp", "build/"}
	ignorer, err := NewIgnorer(tmpDir, configPatterns)
	require.NoError(t, err)

	// Should ignore both file patterns and config patterns
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "app.log")))    // from file
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "file.tmp")))   // from config
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "build", "a"))) // from config
}

// TestIgnorerWithInvalidProjectRoot verifies error handling for invalid root
func TestIgnorerWithInvalidProjectRoot(t *testing.T) {
	ignorer, err := NewIgnorer("/nonexistent/path/that/does/not/exist", nil)
	assert.Error(t, err)
	assert.Nil(t, ignorer)
}

// TestTrailingSlashDirectory verifies directory patterns with trailing slash
func TestTrailingSlashDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Pattern with trailing slash should match directories
	patterns := []string{"logs/", "cache/"}
	ignorer, err := NewIgnorer(tmpDir, patterns)
	require.NoError(t, err)

	// Should ignore directories and their contents
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "logs")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "logs", "app.log")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "cache", "data.json")))

	// Should not ignore files named similarly
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "logs.txt")))
}

// TestCaseInsensitivity verifies pattern matching behavior regarding case
func TestCaseHandling(t *testing.T) {
	tmpDir := t.TempDir()

	patterns := []string{"README.md", "Makefile"}
	ignorer, err := NewIgnorer(tmpDir, patterns)
	require.NoError(t, err)

	// Exact case should match
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "README.md")))
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "Makefile")))

	// Different case - behavior depends on implementation/filesystem
	// On case-sensitive systems, these should NOT match
	// On case-insensitive systems, they might match
	// This test documents expected behavior (case-sensitive matching)
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "readme.md")))
	assert.False(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "makefile")))
}

// TestPathSeparatorNormalization verifies path separator handling
func TestPathSeparatorNormalization(t *testing.T) {
	tmpDir := t.TempDir()

	patterns := []string{"vendor/pkg/"}
	ignorer, err := NewIgnorer(tmpDir, patterns)
	require.NoError(t, err)

	// Should handle paths with OS-specific separators
	assert.True(t, ignorer.ShouldIgnore(filepath.Join(tmpDir, "vendor", "pkg", "file.go")))
}
