package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
)

// ScanResult contains the results of a filesystem scan.
type ScanResult struct {
	Modified []string
	Added    []string
	Deleted  []string
}

// TotalChanges returns the total number of changes detected.
func (r *ScanResult) TotalChanges() int {
	return len(r.Modified) + len(r.Added) + len(r.Deleted)
}

// StartupScanner detects file changes since the last scan.
type StartupScanner struct {
	projectRoot string
	config      *config.Config
	db          *db.DB
	ignorer     *Ignorer
}

// NewStartupScanner creates a new startup scanner.
func NewStartupScanner(projectRoot string, cfg *config.Config, database *db.DB, ignorer *Ignorer) *StartupScanner {
	return &StartupScanner{
		projectRoot: projectRoot,
		config:      cfg,
		db:          database,
		ignorer:     ignorer,
	}
}

// Scan compares filesystem state to database and returns changes.
func (s *StartupScanner) Scan(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{
		Modified: make([]string, 0),
		Added:    make([]string, 0),
		Deleted:  make([]string, 0),
	}

	// Get all indexed files from database
	indexed, err := s.db.ListFiles(ctx)
	if err != nil {
		return nil, err
	}
	indexedMap := make(map[string]time.Time)
	for _, f := range indexed {
		indexedMap[f.Path] = f.ModifiedAt
	}

	// Walk filesystem
	seenPaths := make(map[string]bool)

	err = filepath.Walk(s.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Get relative path
		relPath, err := filepath.Rel(s.projectRoot, path)
		if err != nil {
			return nil
		}

		// Skip directories and ignored files
		if info.IsDir() {
			if s.ignorer.ShouldIgnore(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches include patterns
		if !s.matchesIncludePatterns(relPath) {
			return nil
		}

		if s.ignorer.ShouldIgnore(relPath) {
			return nil
		}

		seenPaths[relPath] = true

		// Check if file is new or modified
		if lastMod, exists := indexedMap[relPath]; exists {
			// File exists in index - check if modified
			if info.ModTime().After(lastMod) {
				result.Modified = append(result.Modified, relPath)
			}
		} else {
			// New file
			result.Added = append(result.Added, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Find deleted files
	for path := range indexedMap {
		if !seenPaths[path] {
			result.Deleted = append(result.Deleted, path)
		}
	}

	return result, nil
}

// matchesIncludePatterns checks if a path matches any of the include patterns.
func (s *StartupScanner) matchesIncludePatterns(path string) bool {
	for _, pattern := range s.config.IncludePatterns {
		// Handle ** patterns
		if strings.Contains(pattern, "**") {
			if s.matchDoubleStarPattern(path, pattern) {
				return true
			}
			continue
		}

		// Check against basename for simple patterns
		base := filepath.Base(path)
		matched, err := filepath.Match(pattern, base)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// matchDoubleStarPattern matches patterns containing **.
func (s *StartupScanner) matchDoubleStarPattern(path, pattern string) bool {
	// Handle **/*.ext pattern - match at any depth
	if strings.HasPrefix(pattern, "**/") {
		subPattern := pattern[3:] // Remove **/
		// Check against basename
		base := filepath.Base(path)
		matched, err := filepath.Match(subPattern, base)
		if err == nil && matched {
			return true
		}
	}
	return false
}
