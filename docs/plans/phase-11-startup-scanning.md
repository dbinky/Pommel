# Phase 11: Startup Scanning

**Status:** Not Started
**Effort:** Medium
**Dependencies:** Phase 10 (Sub-Project Detection)

---

## Objective

Implement full filesystem scanning on daemon startup to detect file changes that occurred while the daemon was stopped, and queue those files for re-indexing. Also scan for new/removed sub-projects.

---

## Requirements

1. Track last scan timestamp in state file
2. On startup, compare file mtimes against last scan
3. Queue modified/new files for indexing
4. Detect deleted files and remove from index
5. Scan for new sub-projects before file scanning
6. API remains available immediately (indexing happens in background)

---

## Implementation Tasks

### 11.1 Update State File Schema

**File:** `internal/daemon/state.go` (new or updated)

```go
package daemon

import (
    "encoding/json"
    "os"
    "path/filepath"
    "time"
)

type DaemonState struct {
    LastScan         time.Time `json:"last_scan"`
    DaemonPID        int       `json:"daemon_pid"`
    IndexedFiles     int       `json:"indexed_files"`
    SubprojectsHash  string    `json:"subprojects_hash"`
    StartedAt        time.Time `json:"started_at"`
}

func LoadState(pommelDir string) (*DaemonState, error) {
    path := filepath.Join(pommelDir, "state.json")

    data, err := os.ReadFile(path)
    if os.IsNotExist(err) {
        return &DaemonState{}, nil
    }
    if err != nil {
        return nil, err
    }

    var state DaemonState
    if err := json.Unmarshal(data, &state); err != nil {
        return nil, err
    }

    return &state, nil
}

func (s *DaemonState) Save(pommelDir string) error {
    path := filepath.Join(pommelDir, "state.json")

    data, err := json.MarshalIndent(s, "", "  ")
    if err != nil {
        return err
    }

    return os.WriteFile(path, data, 0644)
}
```

### 11.2 Create Startup Scanner

**File:** `internal/daemon/scanner.go` (new)

```go
package daemon

import (
    "context"
    "os"
    "path/filepath"
    "time"

    "github.com/dbinky/pommel/internal/config"
    "github.com/dbinky/pommel/internal/db"
)

type ScanResult struct {
    Modified []string
    Added    []string
    Deleted  []string
}

type StartupScanner struct {
    projectRoot string
    config      *config.Config
    db          *db.Database
    ignorer     *Ignorer
}

func NewStartupScanner(projectRoot string, cfg *config.Config, database *db.Database, ignorer *Ignorer) *StartupScanner {
    return &StartupScanner{
        projectRoot: projectRoot,
        config:      cfg,
        db:          database,
        ignorer:     ignorer,
    }
}

// Scan compares filesystem state to database and returns changes.
func (s *StartupScanner) Scan(ctx context.Context, lastScan time.Time) (*ScanResult, error) {
    result := &ScanResult{
        Modified: make([]string, 0),
        Added:    make([]string, 0),
        Deleted:  make([]string, 0),
    }

    // Get all indexed files from database
    indexed, err := s.db.ListIndexedFiles(ctx)
    if err != nil {
        return nil, err
    }
    indexedMap := make(map[string]time.Time)
    for _, f := range indexed {
        indexedMap[f.Path] = f.LastModified
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

func (s *StartupScanner) matchesIncludePatterns(path string) bool {
    for _, pattern := range s.config.IncludePatterns {
        matched, err := filepath.Match(pattern, filepath.Base(path))
        if err == nil && matched {
            return true
        }
        // Also try matching with directory
        matched, err = doublestar.Match(pattern, path)
        if err == nil && matched {
            return true
        }
    }
    return false
}
```

### 11.3 Add Database Method for Listing Indexed Files

**File:** `internal/db/chunks.go`

```go
type IndexedFile struct {
    Path         string
    LastModified time.Time
}

// ListIndexedFiles returns all unique file paths in the index.
func (db *Database) ListIndexedFiles(ctx context.Context) ([]IndexedFile, error) {
    query := `
        SELECT DISTINCT file_path, MAX(last_modified) as last_modified
        FROM chunks
        GROUP BY file_path
    `

    rows, err := db.conn.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var files []IndexedFile
    for rows.Next() {
        var f IndexedFile
        if err := rows.Scan(&f.Path, &f.LastModified); err != nil {
            return nil, err
        }
        files = append(files, f)
    }

    return files, rows.Err()
}

// DeleteFileChunks removes all chunks for a given file.
func (db *Database) DeleteFileChunks(ctx context.Context, filePath string) error {
    _, err := db.conn.ExecContext(ctx,
        "DELETE FROM chunks WHERE file_path = ?", filePath)
    return err
}
```

### 11.4 Update Daemon Startup Sequence

**File:** `internal/daemon/daemon.go`

```go
func (d *Daemon) Start(ctx context.Context) error {
    // Load state
    state, err := LoadState(d.pommelDir)
    if err != nil {
        log.Warnf("Could not load state: %v", err)
        state = &DaemonState{}
    }

    // Initialize components
    if err := d.initialize(ctx); err != nil {
        return err
    }

    log.Info("Scanning for sub-project changes...")
    added, removed, _, err := d.subprojectManager.SyncSubprojects(ctx)
    if err != nil {
        return err
    }
    if added > 0 || removed > 0 {
        log.Infof("Sub-projects: %d added, %d removed", added, removed)
    }

    log.Infof("Scanning for file changes since %s...", state.LastScan.Format(time.RFC3339))
    scanner := NewStartupScanner(d.projectRoot, d.config, d.db, d.ignorer)
    changes, err := scanner.Scan(ctx, state.LastScan)
    if err != nil {
        return err
    }

    totalChanges := len(changes.Modified) + len(changes.Added) + len(changes.Deleted)
    if totalChanges > 0 {
        log.Infof("Found %d modified, %d added, %d deleted files",
            len(changes.Modified), len(changes.Added), len(changes.Deleted))

        // Queue for indexing
        d.queueFilesForIndexing(changes.Modified)
        d.queueFilesForIndexing(changes.Added)

        // Handle deletions
        for _, path := range changes.Deleted {
            if err := d.db.DeleteFileChunks(ctx, path); err != nil {
                log.Warnf("Failed to delete chunks for %s: %v", path, err)
            }
        }
    } else {
        log.Info("No file changes detected")
    }

    // Update state
    state.LastScan = time.Now()
    state.DaemonPID = os.Getpid()
    state.StartedAt = time.Now()
    if err := state.Save(d.pommelDir); err != nil {
        log.Warnf("Failed to save state: %v", err)
    }

    // Start HTTP server (API available immediately)
    go d.startHTTPServer()

    // Process index queue in background
    go d.processIndexQueue(ctx)

    // Start file watcher
    return d.startWatcher(ctx)
}

func (d *Daemon) queueFilesForIndexing(paths []string) {
    for _, path := range paths {
        d.indexQueue <- path
    }
}

func (d *Daemon) processIndexQueue(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case path := <-d.indexQueue:
            if err := d.indexFile(ctx, path); err != nil {
                log.Warnf("Failed to index %s: %v", path, err)
            }
        }
    }
}
```

### 11.5 Update State on Successful Indexing

**File:** `internal/daemon/indexer.go`

```go
func (i *Indexer) updateStateAfterIndex() {
    state, err := LoadState(i.pommelDir)
    if err != nil {
        return
    }

    count, err := i.db.ChunkCount(context.Background())
    if err != nil {
        return
    }

    state.IndexedFiles = count
    state.LastScan = time.Now()
    state.Save(i.pommelDir)
}
```

### 11.6 Add Status Endpoint Enhancements

**File:** `internal/api/handlers.go`

```go
type StatusResponse struct {
    Daemon struct {
        Running       bool      `json:"running"`
        PID           int       `json:"pid"`
        UptimeSeconds int64     `json:"uptime_seconds"`
        StartedAt     time.Time `json:"started_at"`
    } `json:"daemon"`
    Index struct {
        TotalFiles     int       `json:"total_files"`
        TotalChunks    int       `json:"total_chunks"`
        LastIndexed    time.Time `json:"last_indexed"`
        PendingChanges int       `json:"pending_changes"`
    } `json:"index"`
    Subprojects struct {
        Count int `json:"count"`
    } `json:"subprojects"`
    Health struct {
        Status         string `json:"status"`
        EmbeddingModel string `json:"embedding_model"`
        Database       string `json:"database"`
    } `json:"health"`
}
```

---

## Testing

### Unit Tests

| Test | Description |
|------|-------------|
| `TestState_LoadSave` | State file round-trip |
| `TestScanner_DetectsNew` | Finds new files |
| `TestScanner_DetectsModified` | Finds modified files |
| `TestScanner_DetectsDeleted` | Finds deleted files |
| `TestScanner_RespectsIgnore` | Ignores .pommelignore patterns |
| `TestScanner_RespectsInclude` | Only scans matching patterns |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestStartup_FullScan` | Complete startup scan flow |
| `TestStartup_IndexQueueProcessing` | Files are indexed after scan |
| `TestStartup_DeletionHandling` | Deleted files removed from index |
| `TestStartup_SubprojectSync` | New subprojects detected |

---

## Acceptance Criteria

- [ ] State file tracks last scan timestamp
- [ ] Startup scan detects modified files (mtime > last_scan)
- [ ] Startup scan detects new files (not in index)
- [ ] Startup scan detects deleted files (in index, not on disk)
- [ ] Deleted files are removed from chunks table
- [ ] Modified/new files queued for background indexing
- [ ] API available immediately during background indexing
- [ ] Sub-projects synced before file scanning
- [ ] `pm status` shows pending_changes count

---

## Files Modified

| File | Change |
|------|--------|
| `internal/daemon/state.go` | New/updated: state persistence |
| `internal/daemon/scanner.go` | New: startup file scanner |
| `internal/daemon/daemon.go` | Startup sequence with scanning |
| `internal/daemon/indexer.go` | Update state after indexing |
| `internal/db/chunks.go` | ListIndexedFiles, DeleteFileChunks |
| `internal/api/handlers.go` | Enhanced status response |
