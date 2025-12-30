# Phase 12: Runtime Detection

**Status:** Not Started
**Effort:** Small
**Dependencies:** Phase 11 (Startup Scanning)

---

## Objective

Extend the file watcher to detect creation of new marker files at runtime, automatically registering new sub-projects and indexing their contents without requiring a daemon restart.

---

## Requirements

1. Watch for creation of marker files (go.mod, package.json, etc.)
2. On marker file creation, register new sub-project
3. Scan and index all source files in new sub-project
4. Update subproject_id on any already-indexed files in that path
5. Handle marker file deletion gracefully (log warning, keep subproject)

---

## Implementation Tasks

### 12.1 Add Marker File Detection to Watcher

**File:** `internal/daemon/watcher.go`

```go
// IsMarkerFile checks if a filename is a sub-project marker
func (w *Watcher) IsMarkerFile(filename string) bool {
    markers := w.config.Subprojects.Markers
    if len(markers) == 0 {
        markers = subproject.DefaultMarkerPatterns
    }

    for _, pattern := range markers {
        if strings.HasPrefix(pattern, "*") {
            if strings.HasSuffix(filename, pattern[1:]) {
                return true
            }
        } else if filename == pattern {
            return true
        }
    }
    return false
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
    filename := filepath.Base(event.Name)

    // Check for marker file events
    if w.IsMarkerFile(filename) {
        w.handleMarkerEvent(event)
        return
    }

    // Regular file handling...
    w.handleSourceFileEvent(event)
}

func (w *Watcher) handleMarkerEvent(event fsnotify.Event) {
    relPath, err := filepath.Rel(w.projectRoot, event.Name)
    if err != nil {
        return
    }

    dirPath := filepath.Dir(relPath)
    if dirPath == "." {
        // Marker at project root - not a sub-project
        return
    }

    switch {
    case event.Op&fsnotify.Create == fsnotify.Create:
        w.handleMarkerCreated(dirPath, filepath.Base(event.Name))

    case event.Op&fsnotify.Remove == fsnotify.Remove:
        w.handleMarkerDeleted(dirPath, filepath.Base(event.Name))
    }
}
```

### 12.2 Handle Marker File Creation

**File:** `internal/daemon/watcher.go`

```go
func (w *Watcher) handleMarkerCreated(dirPath, markerFile string) {
    log.Infof("New marker file detected: %s/%s", dirPath, markerFile)

    ctx := context.Background()

    // Check if subproject already exists for this path
    existing, err := w.db.GetSubprojectByPath(ctx, dirPath)
    if err != nil {
        log.Warnf("Failed to check existing subproject: %v", err)
        return
    }
    if existing != nil {
        log.Debugf("Subproject already exists for %s", dirPath)
        return
    }

    // Detect language hint from marker
    langHint := w.getLanguageHint(markerFile)

    // Generate subproject ID
    id := w.generateSubprojectID(dirPath)

    // Register new subproject
    sp := &models.Subproject{
        ID:           id,
        Path:         dirPath,
        MarkerFile:   markerFile,
        LanguageHint: langHint,
        AutoDetected: true,
    }

    if err := w.db.InsertSubproject(ctx, sp); err != nil {
        log.Errorf("Failed to register subproject %s: %v", id, err)
        return
    }

    log.Infof("Registered new sub-project: %s (%s)", id, dirPath)

    // Scan directory for source files
    go w.scanAndIndexSubproject(ctx, dirPath, id)

    // Update existing chunks that may be in this path
    go w.updateExistingChunksSubproject(ctx, dirPath, id)
}

func (w *Watcher) getLanguageHint(markerFile string) string {
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

func (w *Watcher) generateSubprojectID(path string) string {
    base := filepath.Base(path)
    id := strings.ToLower(base)
    id = strings.Map(func(r rune) rune {
        if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
            return r
        }
        return '-'
    }, id)
    return id
}
```

### 12.3 Scan and Index New Subproject Files

**File:** `internal/daemon/watcher.go`

```go
func (w *Watcher) scanAndIndexSubproject(ctx context.Context, dirPath, subprojectID string) {
    absDir := filepath.Join(w.projectRoot, dirPath)

    var filesToIndex []string

    err := filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return nil
        }

        if info.IsDir() {
            relPath, _ := filepath.Rel(w.projectRoot, path)
            if w.ignorer.ShouldIgnore(relPath) {
                return filepath.SkipDir
            }
            return nil
        }

        relPath, _ := filepath.Rel(w.projectRoot, path)
        if w.ignorer.ShouldIgnore(relPath) {
            return nil
        }

        if w.matchesIncludePatterns(relPath) {
            filesToIndex = append(filesToIndex, relPath)
        }

        return nil
    })

    if err != nil {
        log.Warnf("Error scanning subproject %s: %v", subprojectID, err)
        return
    }

    log.Infof("Queuing %d files from new sub-project %s for indexing", len(filesToIndex), subprojectID)

    for _, path := range filesToIndex {
        w.indexQueue <- path
    }
}
```

### 12.4 Update Existing Chunks with Subproject

**File:** `internal/daemon/watcher.go`

```go
func (w *Watcher) updateExistingChunksSubproject(ctx context.Context, dirPath, subprojectID string) {
    // Update any chunks that are in this directory but don't have subproject_id set
    err := w.db.UpdateChunksSubproject(ctx, dirPath, subprojectID)
    if err != nil {
        log.Warnf("Failed to update existing chunks for subproject %s: %v", subprojectID, err)
    }
}
```

**File:** `internal/db/chunks.go`

```go
// UpdateChunksSubproject sets subproject_id for all chunks under a path.
func (db *Database) UpdateChunksSubproject(ctx context.Context, pathPrefix, subprojectID string) error {
    query := `
        UPDATE chunks
        SET subproject_id = ?, subproject_path = ?
        WHERE file_path LIKE ? || '%'
        AND (subproject_id IS NULL OR subproject_id = '')
    `
    _, err := db.conn.ExecContext(ctx, query, subprojectID, pathPrefix, pathPrefix)
    return err
}
```

### 12.5 Handle Marker File Deletion

**File:** `internal/daemon/watcher.go`

```go
func (w *Watcher) handleMarkerDeleted(dirPath, markerFile string) {
    log.Warnf("Marker file deleted: %s/%s - subproject will be retained until explicit removal", dirPath, markerFile)

    // We intentionally do NOT remove the subproject here.
    // The marker might be temporarily deleted during a git operation,
    // or the user might recreate it. Use `pm reindex --rescan-subprojects`
    // to clean up orphaned subprojects.
}
```

### 12.6 Add Watch for New Directories

**File:** `internal/daemon/watcher.go`

Ensure new directories are watched so we can detect marker files created within them:

```go
func (w *Watcher) handleDirCreated(path string) {
    // Add the new directory to the watch list
    if err := w.fsWatcher.Add(path); err != nil {
        log.Warnf("Failed to watch new directory %s: %v", path, err)
    }
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
    info, err := os.Stat(event.Name)
    if err == nil && info.IsDir() && event.Op&fsnotify.Create == fsnotify.Create {
        w.handleDirCreated(event.Name)
        return
    }

    // ... rest of handling
}
```

---

## Testing

### Unit Tests

| Test | Description |
|------|-------------|
| `TestIsMarkerFile_GoMod` | Detects go.mod as marker |
| `TestIsMarkerFile_PackageJson` | Detects package.json |
| `TestIsMarkerFile_Csproj` | Detects *.csproj |
| `TestIsMarkerFile_NotMarker` | Regular files not detected |
| `TestGetLanguageHint` | Correct language for each marker |
| `TestGenerateSubprojectID` | Slug generation from path |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestMarkerCreated_RegistersSubproject` | New go.mod triggers registration |
| `TestMarkerCreated_IndexesFiles` | Files in subproject indexed |
| `TestMarkerCreated_UpdatesExistingChunks` | Orphan chunks get subproject_id |
| `TestMarkerDeleted_KeepsSubproject` | Deletion only logs warning |
| `TestNewDirectory_AddedToWatch` | New dirs are watched |

---

## Acceptance Criteria

- [ ] Creating go.mod in new directory registers subproject within seconds
- [ ] All source files in new subproject directory are indexed
- [ ] Pre-existing chunks in that path get subproject_id updated
- [ ] Marker file deletion logs warning but keeps subproject
- [ ] New directories are automatically added to watch list
- [ ] `pm subprojects` shows newly detected subprojects

---

## Files Modified

| File | Change |
|------|--------|
| `internal/daemon/watcher.go` | Marker detection, subproject registration |
| `internal/db/chunks.go` | UpdateChunksSubproject method |
| `internal/subproject/detector.go` | Export DefaultMarkerPatterns |
