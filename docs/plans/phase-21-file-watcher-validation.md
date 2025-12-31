# Phase 21: File Watcher Validation for Windows

**Status:** Planning
**Branch:** dev-windows-support
**Depends on:** Phase 20 (Daemon Process Management)

## Objective

Validate and enhance the file watcher to handle Windows-specific behaviors and edge cases, ensuring reliable file change detection on Windows.

## Background

### Current Implementation

Pommel uses `fsnotify` for file watching, which supports Windows via the ReadDirectoryChangesW API. The current implementation includes:
- Directory watching with recursive traversal
- Debouncing to coalesce rapid changes
- .pommelignore pattern filtering

### Windows-Specific Concerns

1. **File locking:** Windows locks files more aggressively; files open in editors may be inaccessible
2. **Event differences:** Windows may emit different events than Unix for the same operations
3. **Network drives:** Behavior may differ on network/mapped drives
4. **Long paths:** Paths > 260 chars may cause issues
5. **Case insensitivity:** File renames changing only case
6. **Rapid changes:** IDE auto-save, build tools creating many files quickly

## Files to Review

| File | Purpose |
|------|---------|
| `internal/daemon/watcher.go` | Main file watcher implementation |
| `internal/daemon/watcher_test.go` | Existing watcher tests |
| `internal/daemon/ignorer.go` | Pattern matching for ignored files |
| `internal/daemon/indexer.go` | Handles file change events |

## Implementation Tasks

### Task 1: Review Current Watcher Implementation

**File:** `internal/daemon/watcher.go`

**Understand:**
- How fsnotify is initialized
- Event handling logic
- Debouncing implementation
- Error handling

### Task 2: Write Windows-Specific Watcher Tests (TDD)

**File:** `internal/daemon/watcher_windows_test.go`

```go
// +build windows

package daemon

import (
    "os"
    "path/filepath"
    "runtime"
    "testing"
    "time"
)

// === Basic Functionality Tests ===

func TestWatcher_FileCreate_Windows(t *testing.T) {
    tmpDir := t.TempDir()

    events := make(chan string, 10)
    watcher, err := NewWatcher(tmpDir, func(path string) {
        events <- path
    })
    if err != nil {
        t.Fatalf("failed to create watcher: %v", err)
    }
    defer watcher.Close()

    // Create a file
    testFile := filepath.Join(tmpDir, "test.go")
    err = os.WriteFile(testFile, []byte("package main"), 0644)
    if err != nil {
        t.Fatalf("failed to create file: %v", err)
    }

    // Wait for event
    select {
    case path := <-events:
        if path != testFile {
            t.Errorf("expected %s, got %s", testFile, path)
        }
    case <-time.After(5 * time.Second):
        t.Error("timeout waiting for create event")
    }
}

func TestWatcher_FileModify_Windows(t *testing.T) {
    tmpDir := t.TempDir()

    // Create file first
    testFile := filepath.Join(tmpDir, "test.go")
    err := os.WriteFile(testFile, []byte("package main"), 0644)
    if err != nil {
        t.Fatalf("failed to create file: %v", err)
    }

    events := make(chan string, 10)
    watcher, err := NewWatcher(tmpDir, func(path string) {
        events <- path
    })
    if err != nil {
        t.Fatalf("failed to create watcher: %v", err)
    }
    defer watcher.Close()

    // Drain any initial events
    time.Sleep(100 * time.Millisecond)
    for len(events) > 0 {
        <-events
    }

    // Modify the file
    err = os.WriteFile(testFile, []byte("package main\n\nfunc main() {}"), 0644)
    if err != nil {
        t.Fatalf("failed to modify file: %v", err)
    }

    // Wait for event
    select {
    case path := <-events:
        if path != testFile {
            t.Errorf("expected %s, got %s", testFile, path)
        }
    case <-time.After(5 * time.Second):
        t.Error("timeout waiting for modify event")
    }
}

func TestWatcher_FileDelete_Windows(t *testing.T) {
    tmpDir := t.TempDir()

    // Create file first
    testFile := filepath.Join(tmpDir, "test.go")
    err := os.WriteFile(testFile, []byte("package main"), 0644)
    if err != nil {
        t.Fatalf("failed to create file: %v", err)
    }

    events := make(chan string, 10)
    watcher, err := NewWatcher(tmpDir, func(path string) {
        events <- path
    })
    if err != nil {
        t.Fatalf("failed to create watcher: %v", err)
    }
    defer watcher.Close()

    // Drain initial events
    time.Sleep(100 * time.Millisecond)
    for len(events) > 0 {
        <-events
    }

    // Delete the file
    err = os.Remove(testFile)
    if err != nil {
        t.Fatalf("failed to delete file: %v", err)
    }

    // Wait for event
    select {
    case path := <-events:
        if path != testFile {
            t.Errorf("expected %s, got %s", testFile, path)
        }
    case <-time.After(5 * time.Second):
        t.Error("timeout waiting for delete event")
    }
}

// === Edge Case Tests ===

func TestWatcher_LockedFile_Windows(t *testing.T) {
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.go")

    // Create and keep file open (simulating editor)
    f, err := os.OpenFile(testFile, os.O_CREATE|os.O_RDWR, 0644)
    if err != nil {
        t.Fatalf("failed to create file: %v", err)
    }
    _, err = f.WriteString("package main")
    if err != nil {
        f.Close()
        t.Fatalf("failed to write: %v", err)
    }

    events := make(chan string, 10)
    errors := make(chan error, 10)
    watcher, err := NewWatcherWithErrorHandler(tmpDir,
        func(path string) { events <- path },
        func(err error) { errors <- err },
    )
    if err != nil {
        f.Close()
        t.Fatalf("failed to create watcher: %v", err)
    }
    defer watcher.Close()

    // Write to locked file (file is open by us)
    _, err = f.WriteString("\n\nfunc main() {}")
    if err != nil {
        f.Close()
        t.Fatalf("failed to write to locked file: %v", err)
    }
    f.Sync()

    // Should still get event (we own the lock)
    select {
    case <-events:
        // Good - event received
    case err := <-errors:
        t.Errorf("unexpected error: %v", err)
    case <-time.After(5 * time.Second):
        t.Error("timeout waiting for event on locked file")
    }

    f.Close()
}

func TestWatcher_RapidChanges_Windows(t *testing.T) {
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.go")

    err := os.WriteFile(testFile, []byte("v0"), 0644)
    if err != nil {
        t.Fatalf("failed to create file: %v", err)
    }

    eventCount := 0
    watcher, err := NewWatcher(tmpDir, func(path string) {
        eventCount++
    })
    if err != nil {
        t.Fatalf("failed to create watcher: %v", err)
    }
    defer watcher.Close()

    // Rapid modifications (simulate IDE auto-save)
    for i := 1; i <= 10; i++ {
        err = os.WriteFile(testFile, []byte(fmt.Sprintf("v%d", i)), 0644)
        if err != nil {
            t.Fatalf("failed to modify file: %v", err)
        }
        time.Sleep(10 * time.Millisecond)
    }

    // Wait for debouncing to settle
    time.Sleep(1 * time.Second)

    // With debouncing, should have fewer events than modifications
    // Exact number depends on debounce timing
    if eventCount >= 10 {
        t.Logf("Warning: %d events for 10 rapid changes (debouncing may not be effective)", eventCount)
    }
    if eventCount == 0 {
        t.Error("should have received at least one event")
    }
}

func TestWatcher_DeepNestedPath_Windows(t *testing.T) {
    tmpDir := t.TempDir()

    // Create deeply nested directory
    deepPath := tmpDir
    for i := 0; i < 20; i++ {
        deepPath = filepath.Join(deepPath, fmt.Sprintf("level%d", i))
    }
    err := os.MkdirAll(deepPath, 0755)
    if err != nil {
        t.Fatalf("failed to create deep path: %v", err)
    }

    events := make(chan string, 10)
    watcher, err := NewWatcher(tmpDir, func(path string) {
        events <- path
    })
    if err != nil {
        t.Fatalf("failed to create watcher: %v", err)
    }
    defer watcher.Close()

    // Create file in deep path
    testFile := filepath.Join(deepPath, "test.go")
    err = os.WriteFile(testFile, []byte("package main"), 0644)
    if err != nil {
        t.Fatalf("failed to create file: %v", err)
    }

    // Wait for event
    select {
    case path := <-events:
        if path != testFile {
            t.Errorf("expected %s, got %s", testFile, path)
        }
    case <-time.After(5 * time.Second):
        t.Error("timeout waiting for event in deep path")
    }
}

func TestWatcher_PathWithSpaces_Windows(t *testing.T) {
    tmpDir := t.TempDir()
    spacePath := filepath.Join(tmpDir, "My Project", "Source Files")
    err := os.MkdirAll(spacePath, 0755)
    if err != nil {
        t.Fatalf("failed to create path with spaces: %v", err)
    }

    events := make(chan string, 10)
    watcher, err := NewWatcher(tmpDir, func(path string) {
        events <- path
    })
    if err != nil {
        t.Fatalf("failed to create watcher: %v", err)
    }
    defer watcher.Close()

    // Create file in path with spaces
    testFile := filepath.Join(spacePath, "my file.go")
    err = os.WriteFile(testFile, []byte("package main"), 0644)
    if err != nil {
        t.Fatalf("failed to create file: %v", err)
    }

    // Wait for event
    select {
    case path := <-events:
        if path != testFile {
            t.Errorf("expected %s, got %s", testFile, path)
        }
    case <-time.After(5 * time.Second):
        t.Error("timeout waiting for event in path with spaces")
    }
}

func TestWatcher_CaseInsensitiveRename_Windows(t *testing.T) {
    tmpDir := t.TempDir()

    // Create file with lowercase
    testFile := filepath.Join(tmpDir, "test.go")
    err := os.WriteFile(testFile, []byte("package main"), 0644)
    if err != nil {
        t.Fatalf("failed to create file: %v", err)
    }

    events := make(chan string, 10)
    watcher, err := NewWatcher(tmpDir, func(path string) {
        events <- path
    })
    if err != nil {
        t.Fatalf("failed to create watcher: %v", err)
    }
    defer watcher.Close()

    // Drain initial events
    time.Sleep(100 * time.Millisecond)
    for len(events) > 0 {
        <-events
    }

    // Rename to uppercase (case-only change)
    newFile := filepath.Join(tmpDir, "Test.go")
    err = os.Rename(testFile, newFile)
    if err != nil {
        t.Fatalf("failed to rename file: %v", err)
    }

    // Should detect the rename
    select {
    case <-events:
        // Good - event received
    case <-time.After(5 * time.Second):
        t.Error("timeout waiting for case-change rename event")
    }
}

// === Error Handling Tests ===

func TestWatcher_PermissionDenied_Windows(t *testing.T) {
    // This test is tricky on Windows - permissions work differently
    // Skip for now, document as manual test
    t.Skip("Permission testing requires admin setup on Windows")
}

func TestWatcher_DirectoryRemoved_Windows(t *testing.T) {
    tmpDir := t.TempDir()
    watchDir := filepath.Join(tmpDir, "watched")
    err := os.Mkdir(watchDir, 0755)
    if err != nil {
        t.Fatalf("failed to create watch dir: %v", err)
    }

    errors := make(chan error, 10)
    watcher, err := NewWatcherWithErrorHandler(watchDir,
        func(path string) {},
        func(err error) { errors <- err },
    )
    if err != nil {
        t.Fatalf("failed to create watcher: %v", err)
    }
    defer watcher.Close()

    // Remove the watched directory
    err = os.RemoveAll(watchDir)
    if err != nil {
        t.Fatalf("failed to remove watch dir: %v", err)
    }

    // Should get an error
    select {
    case err := <-errors:
        t.Logf("Got expected error: %v", err)
    case <-time.After(5 * time.Second):
        // May not error immediately on all systems
        t.Log("No error received (may be expected)")
    }
}
```

### Task 3: Add Locked File Handling

**File:** `internal/daemon/indexer.go`

When a file is locked, retry with backoff:

```go
func (idx *Indexer) readFileWithRetry(path string) ([]byte, error) {
    var lastErr error
    for i := 0; i < 3; i++ {
        content, err := os.ReadFile(path)
        if err == nil {
            return content, nil
        }
        lastErr = err

        // Check if it's a permission/lock error
        if os.IsPermission(err) || isFileLocked(err) {
            time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
            continue
        }

        // Other error, don't retry
        return nil, err
    }
    return nil, fmt.Errorf("file locked after retries: %w", lastErr)
}

// isFileLocked checks if error indicates file is locked (Windows)
func isFileLocked(err error) bool {
    if runtime.GOOS != "windows" {
        return false
    }
    // Windows error codes for sharing violations
    // ERROR_SHARING_VIOLATION = 32
    // ERROR_LOCK_VIOLATION = 33
    var pathErr *os.PathError
    if errors.As(err, &pathErr) {
        if errno, ok := pathErr.Err.(syscall.Errno); ok {
            return errno == 32 || errno == 33
        }
    }
    return false
}
```

### Task 4: Verify Debouncing Effectiveness

**File:** `internal/daemon/watcher.go`

Review and potentially adjust debounce timing for Windows:

```go
const (
    // Debounce duration - may need adjustment for Windows
    debounceDelay = 100 * time.Millisecond // Unix default

    // Windows may need slightly longer debounce due to event batching
    debounceDelayWindows = 150 * time.Millisecond
)

func getDebounceDelay() time.Duration {
    if runtime.GOOS == "windows" {
        return debounceDelayWindows
    }
    return debounceDelay
}
```

### Task 5: Add Error Handler to Watcher

**File:** `internal/daemon/watcher.go`

Ensure errors are properly surfaced:

```go
type WatcherConfig struct {
    OnChange func(path string)
    OnError  func(err error)
}

func NewWatcherWithConfig(root string, config WatcherConfig) (*Watcher, error) {
    // ... existing setup ...

    go func() {
        for {
            select {
            case event, ok := <-watcher.Events:
                if !ok {
                    return
                }
                // Handle event
                config.OnChange(event.Name)

            case err, ok := <-watcher.Errors:
                if !ok {
                    return
                }
                if config.OnError != nil {
                    config.OnError(err)
                }
            }
        }
    }()

    return w, nil
}
```

### Task 6: Integration Tests

**File:** `internal/daemon/watcher_integration_test.go`

```go
// +build integration

func TestWatcher_FullWorkflow_Windows(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("Windows-only test")
    }

    // Create project structure
    // Initialize watcher
    // Simulate development workflow:
    //   - Create files
    //   - Edit files rapidly (IDE simulation)
    //   - Delete files
    //   - Rename files
    // Verify all changes detected
    // Verify debouncing working
}
```

## Test Cases Summary

### Unit Tests (Windows-specific)

| Test | Description |
|------|-------------|
| TestWatcher_FileCreate_Windows | Basic file creation detection |
| TestWatcher_FileModify_Windows | File modification detection |
| TestWatcher_FileDelete_Windows | File deletion detection |
| TestWatcher_LockedFile_Windows | Handle files locked by other processes |
| TestWatcher_RapidChanges_Windows | Debouncing with rapid saves |
| TestWatcher_DeepNestedPath_Windows | Deep directory structures |
| TestWatcher_PathWithSpaces_Windows | Paths containing spaces |
| TestWatcher_CaseInsensitiveRename_Windows | Case-only rename detection |
| TestWatcher_DirectoryRemoved_Windows | Watched directory removal |

### Integration Tests

| Test | Description |
|------|-------------|
| TestWatcher_FullWorkflow_Windows | Complete IDE-like workflow |

## Acceptance Criteria

- [ ] File create/modify/delete events detected on Windows
- [ ] Locked files handled gracefully with retry
- [ ] Rapid changes properly debounced
- [ ] Deep paths and paths with spaces work
- [ ] Case-insensitive renames detected
- [ ] Errors properly reported
- [ ] All tests pass on Windows CI

## Files Changed

| File | Change |
|------|--------|
| `internal/daemon/watcher.go` | Windows debounce tuning, error handling |
| `internal/daemon/watcher_windows_test.go` | Windows-specific tests |
| `internal/daemon/indexer.go` | Locked file retry logic |

## Notes

- fsnotify generally works well on Windows, but edge cases exist
- Locked file handling is the most important Windows-specific concern
- Network drives may have additional issues (defer to documentation)
- Consider logging when files are temporarily inaccessible
- Test with real IDEs (VS Code, IntelliJ) for realistic behavior
