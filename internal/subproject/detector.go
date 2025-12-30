package subproject

import (
	"os"
	"path/filepath"
	"strings"
)

// MarkerDef defines a marker file pattern and its metadata.
type MarkerDef struct {
	Pattern  string
	Priority int // Lower = higher priority
	Language string
}

// DefaultMarkers defines marker files and their priorities.
var DefaultMarkers = []MarkerDef{
	// Priority 1: Solution files (encompass multiple projects)
	{Pattern: "*.sln", Priority: 1, Language: "csharp"},

	// Priority 2: Compiled language project files
	{Pattern: "*.csproj", Priority: 2, Language: "csharp"},
	{Pattern: "go.mod", Priority: 2, Language: "go"},
	{Pattern: "Cargo.toml", Priority: 2, Language: "rust"},
	{Pattern: "pom.xml", Priority: 2, Language: "java"},
	{Pattern: "build.gradle", Priority: 2, Language: "java"},

	// Priority 3: Interpreted language project files
	{Pattern: "package.json", Priority: 3, Language: "javascript"},
	{Pattern: "pyproject.toml", Priority: 3, Language: "python"},
	{Pattern: "setup.py", Priority: 3, Language: "python"},
}

// DetectedSubproject represents a subproject found during scanning.
type DetectedSubproject struct {
	ID           string
	Path         string
	MarkerFile   string
	LanguageHint string
}

// Detector scans a project directory for sub-projects.
type Detector struct {
	projectRoot string
	markers     []MarkerDef
	excludes    []string
}

// NewDetector creates a new sub-project detector.
func NewDetector(projectRoot string, markers []MarkerDef, excludes []string) *Detector {
	if markers == nil {
		markers = DefaultMarkers
	}
	return &Detector{
		projectRoot: projectRoot,
		markers:     markers,
		excludes:    excludes,
	}
}

// Scan walks the project directory and detects sub-projects.
func (d *Detector) Scan() ([]*DetectedSubproject, error) {
	found := make(map[string]*DetectedSubproject) // path -> subproject

	err := filepath.Walk(d.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip directories in exclude list
		if info.IsDir() {
			relPath, _ := filepath.Rel(d.projectRoot, path)
			if d.isExcluded(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches any marker
		for _, marker := range d.markers {
			if d.matchesMarker(info.Name(), marker.Pattern) {
				dirPath := filepath.Dir(path)
				relPath, _ := filepath.Rel(d.projectRoot, dirPath)

				// Skip if at project root
				if relPath == "." {
					continue
				}

				// Check if we already have a higher-priority marker for this path
				if existing, ok := found[relPath]; ok {
					existingPriority := d.getMarkerPriority(existing.MarkerFile)
					if marker.Priority >= existingPriority {
						continue // Keep existing higher-priority marker
					}
				}

				found[relPath] = &DetectedSubproject{
					ID:           d.generateID(relPath),
					Path:         relPath,
					MarkerFile:   info.Name(),
					LanguageHint: marker.Language,
				}
				break
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert map to slice
	result := make([]*DetectedSubproject, 0, len(found))
	for _, sp := range found {
		result = append(result, sp)
	}

	return result, nil
}

// generateID creates a slug from the path (uses last component).
func (d *Detector) generateID(path string) string {
	// Use last component of path as base
	base := filepath.Base(path)

	// Replace non-alphanumeric with dashes
	id := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, base)

	return strings.ToLower(id)
}

// matchesMarker checks if a filename matches a marker pattern.
func (d *Detector) matchesMarker(filename, pattern string) bool {
	return MatchesMarkerPattern(filename, pattern)
}

// getMarkerPriority returns the priority for a marker file.
func (d *Detector) getMarkerPriority(filename string) int {
	for _, m := range d.markers {
		if d.matchesMarker(filename, m.Pattern) {
			return m.Priority
		}
	}
	return DefaultPriority
}

// isExcluded checks if a path is in the exclude list.
func (d *Detector) isExcluded(path string) bool {
	for _, excl := range d.excludes {
		if strings.HasPrefix(path, excl) || path == excl {
			return true
		}
	}
	return false
}
