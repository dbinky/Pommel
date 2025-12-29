package daemon

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Ignorer handles file and directory ignore patterns
type Ignorer struct {
	projectRoot string
	patterns    []pattern
}

// pattern represents a single ignore pattern
type pattern struct {
	original string
	negation bool
	dirOnly  bool
	pattern  string
}

// NewIgnorer creates a new Ignorer with the given project root and config patterns
func NewIgnorer(projectRoot string, configPatterns []string) (*Ignorer, error) {
	// Verify project root exists
	info, err := os.Stat(projectRoot)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, os.ErrNotExist
	}

	i := &Ignorer{
		projectRoot: projectRoot,
		patterns:    make([]pattern, 0),
	}

	// Always add .pommel directory as ignored
	i.addPattern(".pommel/")

	// Load patterns from .gitignore if it exists
	gitignorePath := filepath.Join(projectRoot, ".gitignore")
	if err := i.loadPatternsFromFile(gitignorePath); err != nil && !os.IsNotExist(err) {
		// Ignore file not existing, but return other errors
		return nil, err
	}

	// Load patterns from .pommelignore if it exists (can override/add to gitignore)
	pommelignorePath := filepath.Join(projectRoot, ".pommelignore")
	if err := i.loadPatternsFromFile(pommelignorePath); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Add config patterns (highest priority)
	for _, p := range configPatterns {
		i.addPattern(p)
	}

	return i, nil
}

// loadPatternsFromFile loads ignore patterns from a file
func (i *Ignorer) loadPatternsFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip empty lines and comments
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i.addPattern(line)
	}

	return scanner.Err()
}

// addPattern adds a pattern to the ignorer
func (i *Ignorer) addPattern(p string) {
	pat := pattern{
		original: p,
	}

	// Check for negation
	if strings.HasPrefix(p, "!") {
		pat.negation = true
		p = p[1:]
	}

	// Check for directory-only pattern
	if strings.HasSuffix(p, "/") {
		pat.dirOnly = true
		p = strings.TrimSuffix(p, "/")
	}

	pat.pattern = p
	i.patterns = append(i.patterns, pat)
}

// ShouldIgnore returns true if the given path should be ignored
func (i *Ignorer) ShouldIgnore(path string) bool {
	// Normalize the path to be relative to project root
	relPath := i.normalizePath(path)

	// Track whether the path is currently ignored
	ignored := false

	// Process patterns in order
	for _, pat := range i.patterns {
		if i.matchesPattern(relPath, pat) {
			if pat.negation {
				ignored = false
			} else {
				ignored = true
			}
		}
	}

	return ignored
}

// normalizePath converts a path to be relative to project root
func (i *Ignorer) normalizePath(path string) string {
	// If path is already relative, return as-is
	if !filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	// Try to make it relative to project root
	relPath, err := filepath.Rel(i.projectRoot, path)
	if err != nil {
		return filepath.Clean(path)
	}

	return relPath
}

// matchesPattern checks if a path matches a pattern
func (i *Ignorer) matchesPattern(path string, pat pattern) bool {
	// Get the pattern string
	p := pat.pattern

	// Handle ** patterns
	if strings.Contains(p, "**") {
		return i.matchDoubleStarPattern(path, p, pat.dirOnly)
	}

	// Handle directory patterns
	if pat.dirOnly {
		return i.matchDirectoryPattern(path, p)
	}

	// Handle simple glob patterns (*.log)
	if strings.Contains(p, "*") && !strings.Contains(p, "/") {
		// Check against basename of all path components and the file itself
		return i.matchGlobPattern(path, p)
	}

	// Handle path patterns (contains /)
	if strings.Contains(p, "/") {
		return i.matchPathPattern(path, p)
	}

	// Exact match against filename
	return i.matchExactPattern(path, p)
}

// matchDoubleStarPattern matches patterns containing **
func (i *Ignorer) matchDoubleStarPattern(path, pattern string, dirOnly bool) bool {
	// Handle **/*.ext pattern - match at any depth
	if strings.HasPrefix(pattern, "**/") {
		subPattern := pattern[3:] // Remove **/
		// Check against basename
		base := filepath.Base(path)
		matched, _ := filepath.Match(subPattern, base)
		if matched {
			return true
		}
		// Also check against all path components
		parts := strings.Split(path, string(filepath.Separator))
		for i := 0; i < len(parts); i++ {
			subPath := filepath.Join(parts[i:]...)
			matched, _ := filepath.Match(subPattern, subPath)
			if matched {
				return true
			}
		}
	}
	return false
}

// matchDirectoryPattern matches directory patterns (ending with /)
func (i *Ignorer) matchDirectoryPattern(path, pattern string) bool {
	// Normalize separators
	normalizedPath := filepath.ToSlash(path)
	normalizedPattern := filepath.ToSlash(pattern)

	// If pattern contains /, it's a multi-component directory pattern
	if strings.Contains(normalizedPattern, "/") {
		// Check if path starts with the pattern or contains it as a directory segment
		if strings.HasPrefix(normalizedPath, normalizedPattern+"/") ||
			normalizedPath == normalizedPattern ||
			strings.Contains(normalizedPath, "/"+normalizedPattern+"/") ||
			strings.HasSuffix(normalizedPath, "/"+normalizedPattern) {
			return true
		}
		// Also check if path starts with the pattern (for content inside)
		if strings.HasPrefix(normalizedPath, normalizedPattern) {
			return true
		}
		return false
	}

	// Single component directory pattern - check if it appears as a path component
	parts := strings.Split(normalizedPath, "/")
	for _, part := range parts {
		if part == pattern {
			return true
		}
	}

	return false
}

// matchGlobPattern matches simple glob patterns like *.log
func (i *Ignorer) matchGlobPattern(path, pattern string) bool {
	// Check against basename
	base := filepath.Base(path)
	matched, _ := filepath.Match(pattern, base)
	if matched {
		return true
	}

	// Also check each path component
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		matched, _ := filepath.Match(pattern, part)
		if matched {
			return true
		}
	}

	return false
}

// matchPathPattern matches patterns containing /
func (i *Ignorer) matchPathPattern(path, pattern string) bool {
	// Normalize separators
	normalizedPath := filepath.ToSlash(path)
	normalizedPattern := filepath.ToSlash(pattern)

	// Check if path starts with or contains the pattern
	if strings.HasPrefix(normalizedPath, normalizedPattern) {
		return true
	}
	if strings.Contains(normalizedPath, "/"+normalizedPattern) {
		return true
	}

	return false
}

// matchExactPattern matches exact filename patterns
func (i *Ignorer) matchExactPattern(path, pattern string) bool {
	// Check against basename
	base := filepath.Base(path)
	return base == pattern
}
