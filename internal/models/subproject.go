package models

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Subproject represents a sub-project within a monorepo or multi-project setup.
// Sub-projects are detected via marker files (go.mod, package.json, etc.) or
// can be explicitly configured.
type Subproject struct {
	ID           string    `json:"id"`
	Path         string    `json:"path"`
	Name         string    `json:"name,omitempty"`
	MarkerFile   string    `json:"marker_file,omitempty"`
	LanguageHint string    `json:"language_hint,omitempty"`
	AutoDetected bool      `json:"auto_detected"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// GenerateID creates a slug-style ID from the subproject path.
// Slashes and dots are replaced with dashes.
func (sp *Subproject) GenerateID() string {
	id := sp.Path
	id = strings.ReplaceAll(id, "/", "-")
	id = strings.ReplaceAll(id, "\\", "-")
	id = strings.ReplaceAll(id, ".", "-")
	return id
}

// IsValid checks if the subproject has required fields.
func (sp *Subproject) IsValid() error {
	if sp.ID == "" {
		return fmt.Errorf("ID is required")
	}
	if sp.Path == "" {
		return fmt.Errorf("path is required")
	}
	return nil
}

// ContainsPath checks if the given file path is within this subproject.
// It properly handles path boundaries to avoid false positives like
// "packages/frontend" matching "packages/frontend-admin".
func (sp *Subproject) ContainsPath(filePath string) bool {
	// Normalize paths
	spPath := filepath.Clean(sp.Path)
	fPath := filepath.Clean(filePath)

	// Root subproject contains all paths
	if spPath == "." {
		return true
	}

	// Exact match
	if fPath == spPath {
		return true
	}

	// Check if filePath starts with subproject path followed by separator
	prefix := spPath + string(filepath.Separator)
	return strings.HasPrefix(fPath, prefix)
}

// SetTimestamps sets both CreatedAt and UpdatedAt to the current time.
// Use this when creating a new subproject.
func (sp *Subproject) SetTimestamps() {
	now := time.Now()
	sp.CreatedAt = now
	sp.UpdatedAt = now
}

// Touch updates the UpdatedAt timestamp to the current time.
// Use this when modifying an existing subproject.
func (sp *Subproject) Touch() {
	sp.UpdatedAt = time.Now()
}

// markerLanguageMap maps marker files to their associated languages.
var markerLanguageMap = map[string]string{
	"go.mod":           "go",
	"go.sum":           "go",
	"package.json":     "javascript",
	"tsconfig.json":    "typescript",
	"Cargo.toml":       "rust",
	"pyproject.toml":   "python",
	"requirements.txt": "python",
	"setup.py":         "python",
	"pom.xml":          "java",
	"build.gradle":     "java",
}

// DetectLanguageFromMarker attempts to determine the primary language
// based on the marker file that identified this subproject.
func (sp *Subproject) DetectLanguageFromMarker() string {
	// Check exact matches first
	if lang, ok := markerLanguageMap[sp.MarkerFile]; ok {
		return lang
	}

	// Check for C# project files by extension
	ext := strings.ToLower(filepath.Ext(sp.MarkerFile))
	switch ext {
	case ".csproj", ".fsproj", ".vbproj":
		return "csharp"
	case ".sln":
		return "csharp"
	}

	return ""
}
