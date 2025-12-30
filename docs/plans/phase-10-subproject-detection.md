# Phase 10: Sub-Project Detection

**Status:** Not Started
**Effort:** Medium
**Dependencies:** Phase 9 (Schema Changes)

---

## Objective

Implement automatic detection of sub-projects within a project via marker files (`.sln`, `go.mod`, `package.json`, etc.), with support for manual configuration overrides.

---

## Requirements

1. Scan project directory for marker files
2. Register detected sub-projects in database
3. Support configuration overrides and exclusions
4. Prioritize markers when multiple exist in same directory
5. Handle nested sub-projects (innermost wins)
6. Generate sub-project IDs from directory names

---

## Marker Files

| Language/Platform | Markers | Priority |
|-------------------|---------|----------|
| C# | `.sln`, `.csproj` | 1 (highest) |
| Go | `go.mod` | 2 |
| Rust | `Cargo.toml` | 2 |
| Java | `pom.xml`, `build.gradle` | 2 |
| Node.js/JS/TS | `package.json` | 3 |
| Python | `pyproject.toml`, `setup.py` | 3 |

---

## Implementation Tasks

### 10.1 Create Subproject Detector

**File:** `internal/subproject/detector.go` (new)

```go
package subproject

import (
    "os"
    "path/filepath"
    "strings"

    "github.com/dbinky/pommel/internal/models"
)

// DefaultMarkers defines marker files and their priorities (lower = higher priority)
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

type MarkerDef struct {
    Pattern  string
    Priority int
    Language string
}

type Detector struct {
    projectRoot string
    markers     []MarkerDef
    excludes    []string
}

type DetectedSubproject struct {
    ID           string
    Path         string
    MarkerFile   string
    LanguageHint string
}

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

// generateID creates a slug from the path
func (d *Detector) generateID(path string) string {
    // Use last component of path as base
    base := filepath.Base(path)

    // Replace non-alphanumeric with dashes
    id := strings.Map(func(r rune) rune {
        if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
            return r
        }
        return '-'
    }, base)

    return strings.ToLower(id)
}

func (d *Detector) matchesMarker(filename, pattern string) bool {
    if strings.HasPrefix(pattern, "*") {
        return strings.HasSuffix(filename, pattern[1:])
    }
    return filename == pattern
}

func (d *Detector) getMarkerPriority(filename string) int {
    for _, m := range d.markers {
        if d.matchesMarker(filename, m.Pattern) {
            return m.Priority
        }
    }
    return 999
}

func (d *Detector) isExcluded(path string) bool {
    for _, excl := range d.excludes {
        if strings.HasPrefix(path, excl) || path == excl {
            return true
        }
    }
    return false
}
```

### 10.2 Update Config Schema for Subprojects

**File:** `internal/config/config.go`

```go
type SubprojectsConfig struct {
    AutoDetect bool              `yaml:"auto_detect" mapstructure:"auto_detect"`
    Markers    []string          `yaml:"markers" mapstructure:"markers"`
    Projects   []ProjectOverride `yaml:"projects" mapstructure:"projects"`
    Exclude    []string          `yaml:"exclude" mapstructure:"exclude"`
}

type ProjectOverride struct {
    ID   string `yaml:"id" mapstructure:"id"`
    Path string `yaml:"path" mapstructure:"path"`
    Name string `yaml:"name" mapstructure:"name"`
}

type Config struct {
    // ... existing fields ...
    Subprojects SubprojectsConfig `yaml:"subprojects" mapstructure:"subprojects"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
    return &Config{
        // ... existing defaults ...
        Subprojects: SubprojectsConfig{
            AutoDetect: true,
            Markers: []string{
                "*.sln",
                "*.csproj",
                "go.mod",
                "package.json",
                "pyproject.toml",
                "setup.py",
                "Cargo.toml",
                "pom.xml",
                "build.gradle",
            },
            Projects: nil,
            Exclude:  nil,
        },
    }
}
```

### 10.3 Create Subproject Manager

**File:** `internal/subproject/manager.go` (new)

```go
package subproject

import (
    "context"

    "github.com/dbinky/pommel/internal/config"
    "github.com/dbinky/pommel/internal/db"
    "github.com/dbinky/pommel/internal/models"
)

type Manager struct {
    db          *db.Database
    projectRoot string
    config      *config.SubprojectsConfig
}

func NewManager(database *db.Database, projectRoot string, cfg *config.SubprojectsConfig) *Manager {
    return &Manager{
        db:          database,
        projectRoot: projectRoot,
        config:      cfg,
    }
}

// SyncSubprojects detects sub-projects and syncs with database.
// Returns (added, removed, unchanged) counts.
func (m *Manager) SyncSubprojects(ctx context.Context) (int, int, int, error) {
    // Get current subprojects from DB
    existing, err := m.db.ListSubprojects(ctx)
    if err != nil {
        return 0, 0, 0, err
    }
    existingMap := make(map[string]*models.Subproject)
    for _, sp := range existing {
        existingMap[sp.Path] = sp
    }

    // Detect subprojects if auto-detect enabled
    var detected []*DetectedSubproject
    if m.config.AutoDetect {
        detector := NewDetector(m.projectRoot, nil, m.config.Exclude)
        detected, err = detector.Scan()
        if err != nil {
            return 0, 0, 0, err
        }
    }

    // Merge with config overrides
    merged := m.mergeWithConfig(detected)

    // Sync with database
    added, removed, unchanged := 0, 0, 0
    seenPaths := make(map[string]bool)

    for _, sp := range merged {
        seenPaths[sp.Path] = true

        if _, exists := existingMap[sp.Path]; exists {
            // Update existing
            unchanged++
        } else {
            // Add new
            if err := m.db.InsertSubproject(ctx, sp); err != nil {
                return added, removed, unchanged, err
            }
            added++
        }
    }

    // Remove subprojects no longer detected (only auto-detected ones)
    for path, existing := range existingMap {
        if !seenPaths[path] && existing.AutoDetected {
            if err := m.db.DeleteSubproject(ctx, existing.ID); err != nil {
                return added, removed, unchanged, err
            }
            removed++
        }
    }

    return added, removed, unchanged, nil
}

// mergeWithConfig combines detected subprojects with config overrides
func (m *Manager) mergeWithConfig(detected []*DetectedSubproject) []*models.Subproject {
    result := make([]*models.Subproject, 0)

    // Convert detected to models
    detectedMap := make(map[string]*DetectedSubproject)
    for _, d := range detected {
        detectedMap[d.Path] = d
    }

    // Apply config overrides
    for _, override := range m.config.Projects {
        sp := &models.Subproject{
            ID:           override.ID,
            Path:         override.Path,
            Name:         override.Name,
            AutoDetected: false,
        }

        // Merge with detected if exists
        if det, ok := detectedMap[override.Path]; ok {
            if sp.ID == "" {
                sp.ID = det.ID
            }
            sp.MarkerFile = det.MarkerFile
            sp.LanguageHint = det.LanguageHint
            delete(detectedMap, override.Path)
        }

        result = append(result, sp)
    }

    // Add remaining detected
    for _, det := range detectedMap {
        result = append(result, &models.Subproject{
            ID:           det.ID,
            Path:         det.Path,
            MarkerFile:   det.MarkerFile,
            LanguageHint: det.LanguageHint,
            AutoDetected: true,
        })
    }

    return result
}

// GetSubprojectForPath finds which subproject contains a file path.
func (m *Manager) GetSubprojectForPath(ctx context.Context, filePath string) (*models.Subproject, error) {
    return m.db.GetSubprojectByPath(ctx, filePath)
}

// AssignSubprojectToChunk determines and sets subproject fields on a chunk.
func (m *Manager) AssignSubprojectToChunk(ctx context.Context, chunk *models.Chunk) error {
    sp, err := m.GetSubprojectForPath(ctx, chunk.FilePath)
    if err != nil {
        return err
    }

    if sp != nil {
        chunk.SubprojectID = &sp.ID
        chunk.SubprojectPath = &sp.Path
    }

    return nil
}
```

### 10.4 Integrate with Indexer

**File:** `internal/daemon/indexer.go`

Update indexer to assign subproject to chunks:

```go
func (i *Indexer) indexFile(ctx context.Context, filePath string) error {
    // ... existing chunking and embedding logic ...

    for _, chunk := range chunks {
        // Assign subproject
        if err := i.subprojectManager.AssignSubprojectToChunk(ctx, chunk); err != nil {
            log.Warnf("Failed to assign subproject to chunk: %v", err)
            // Continue - subproject assignment is not critical
        }

        // ... rest of indexing ...
    }
}
```

### 10.5 Add CLI Command for Listing Subprojects

**File:** `internal/cli/subprojects.go` (new)

```go
package cli

import (
    "encoding/json"
    "fmt"
    "os"
    "text/tabwriter"

    "github.com/spf13/cobra"
)

var subprojectsCmd = &cobra.Command{
    Use:   "subprojects",
    Short: "List detected sub-projects",
    RunE: func(cmd *cobra.Command, args []string) error {
        client, err := newClient()
        if err != nil {
            return err
        }

        subprojects, err := client.ListSubprojects()
        if err != nil {
            return err
        }

        if IsJSONOutput() {
            return json.NewEncoder(os.Stdout).Encode(subprojects)
        }

        if len(subprojects) == 0 {
            fmt.Println("No sub-projects detected")
            return nil
        }

        w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        fmt.Fprintln(w, "ID\tPATH\tMARKER\tLANGUAGE")
        for _, sp := range subprojects {
            fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
                sp.ID, sp.Path, sp.MarkerFile, sp.LanguageHint)
        }
        return w.Flush()
    },
}

func init() {
    rootCmd.AddCommand(subprojectsCmd)
}
```

---

## Testing

### Unit Tests

| Test | Description |
|------|-------------|
| `TestDetector_Scan_SingleMarker` | Detects single go.mod |
| `TestDetector_Scan_MultipleMarkers` | Detects multiple sub-projects |
| `TestDetector_Scan_NestedMarkers` | Innermost marker wins |
| `TestDetector_Scan_PriorityMarkers` | Higher priority marker wins |
| `TestDetector_Scan_Excludes` | Respects exclude patterns |
| `TestDetector_GenerateID` | Creates slug from path |
| `TestManager_SyncSubprojects_Add` | Adds new sub-projects |
| `TestManager_SyncSubprojects_Remove` | Removes missing auto-detected |
| `TestManager_MergeWithConfig` | Config overrides work |
| `TestManager_AssignSubprojectToChunk` | Assigns correct subproject |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestSubprojectDetection_RealMonorepo` | Full scan of test monorepo |
| `TestSubprojectPersistence` | Sync survives restart |
| `TestChunkIndexing_WithSubproject` | Chunks get subproject assigned |

---

## Acceptance Criteria

- [ ] Detector finds all marker files in project tree
- [ ] Marker priority respected when multiple in same dir
- [ ] Nested sub-projects handled (innermost wins)
- [ ] Config overrides work (custom ID, name)
- [ ] Config exclude patterns respected
- [ ] Sub-projects persisted to database
- [ ] Chunks assigned correct subproject_id
- [ ] `pm subprojects` lists detected sub-projects

---

## Files Modified

| File | Change |
|------|--------|
| `internal/subproject/detector.go` | New file: marker scanning |
| `internal/subproject/manager.go` | New file: sync and assignment |
| `internal/config/config.go` | Add SubprojectsConfig |
| `internal/daemon/indexer.go` | Assign subproject to chunks |
| `internal/cli/subprojects.go` | New file: list command |
| `internal/api/handlers.go` | Add /subprojects endpoint |
