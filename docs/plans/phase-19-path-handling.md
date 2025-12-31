# Phase 19: Path Handling for Windows

**Status:** Planning
**Branch:** dev-windows-support
**Depends on:** Phase 18 (CI/CD Setup)

## Objective

Ensure all path handling in Pommel works correctly on Windows, including drive letters, backslashes, UNC paths, and edge cases like long paths and paths with spaces.

## Background

Windows paths differ from Unix paths in several ways:
- **Separators:** Backslash (`\`) vs forward slash (`/`)
- **Drive letters:** `C:\Users\...` vs `/home/...`
- **UNC paths:** `\\server\share\...` (network paths)
- **Case sensitivity:** Windows filesystems are typically case-insensitive
- **Path length:** 260 character limit by default (can be extended)
- **Reserved characters:** `<>:"|?*` cannot be in filenames

Go's `filepath` package handles most cross-platform concerns, but we need to audit for:
- Hardcoded `/` separators
- Assumptions about path structure
- Path validation logic

## Files to Audit

Use `pm search` and grep to find path-related code:

```bash
pm search "path handling file operations"
pm search "filepath operations"
grep -r "filepath\." internal/
grep -r '"/.*/"' internal/  # Hardcoded paths
```

### Primary Files

| File | Path Operations |
|------|-----------------|
| `internal/config/loader.go` | Config file paths |
| `internal/config/config.go` | Include/exclude patterns |
| `internal/daemon/watcher.go` | File watching paths |
| `internal/daemon/ignorer.go` | .pommelignore pattern matching |
| `internal/daemon/indexer.go` | File indexing paths |
| `internal/db/db.go` | Database file path |
| `internal/chunker/*.go` | Source file paths in chunks |
| `internal/cli/init.go` | Project initialization paths |
| `internal/cli/search.go` | Search result paths |
| `internal/models/chunk.go` | FilePath field |

## Implementation Tasks

### Task 1: Create Path Utility Package

**File:** `internal/pathutil/pathutil.go`

Create a small utility package for consistent path handling:

```go
package pathutil

import (
    "path/filepath"
    "runtime"
    "strings"
)

// Normalize converts a path to use OS-appropriate separators
// and cleans redundant separators
func Normalize(path string) string {
    return filepath.Clean(path)
}

// IsAbsolute checks if path is absolute (handles Windows drive letters)
func IsAbsolute(path string) bool {
    return filepath.IsAbs(path)
}

// IsUNC checks if path is a Windows UNC path (\\server\share)
func IsUNC(path string) bool {
    if runtime.GOOS != "windows" {
        return false
    }
    return strings.HasPrefix(path, `\\`)
}

// ToSlash converts path to forward slashes (for display/storage)
func ToSlash(path string) string {
    return filepath.ToSlash(path)
}

// FromSlash converts forward slashes to OS separator
func FromSlash(path string) string {
    return filepath.FromSlash(path)
}
```

### Task 2: Write Path Handling Tests (TDD)

**File:** `internal/pathutil/pathutil_test.go`

Write tests BEFORE implementation:

```go
package pathutil

import (
    "runtime"
    "testing"
)

// === Happy Path Tests ===

func TestNormalize_UnixPath(t *testing.T) {
    result := Normalize("/home/user/project")
    if runtime.GOOS == "windows" {
        // On Windows, this stays as-is (not a valid Windows path)
        expected := `\home\user\project`
        if result != expected {
            t.Errorf("expected %q, got %q", expected, result)
        }
    } else {
        expected := "/home/user/project"
        if result != expected {
            t.Errorf("expected %q, got %q", expected, result)
        }
    }
}

func TestNormalize_WindowsPath(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("Windows-only test")
    }
    result := Normalize(`C:\Users\dev\project`)
    expected := `C:\Users\dev\project`
    if result != expected {
        t.Errorf("expected %q, got %q", expected, result)
    }
}

func TestNormalize_MixedSeparators(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("Windows-only test")
    }
    result := Normalize(`C:/Users/dev\project`)
    expected := `C:\Users\dev\project`
    if result != expected {
        t.Errorf("expected %q, got %q", expected, result)
    }
}

func TestIsAbsolute_WindowsDriveLetter(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("Windows-only test")
    }
    if !IsAbsolute(`C:\Users\dev`) {
        t.Error("C:\\Users\\dev should be absolute")
    }
    if !IsAbsolute(`D:\`) {
        t.Error("D:\\ should be absolute")
    }
}

func TestIsAbsolute_UnixPath(t *testing.T) {
    if runtime.GOOS == "windows" {
        t.Skip("Unix-only test")
    }
    if !IsAbsolute("/home/user") {
        t.Error("/home/user should be absolute")
    }
}

func TestIsAbsolute_RelativePath(t *testing.T) {
    if IsAbsolute("relative/path") {
        t.Error("relative/path should not be absolute")
    }
    if IsAbsolute("./local") {
        t.Error("./local should not be absolute")
    }
}

// === Edge Case Tests ===

func TestIsUNC_ValidUNCPath(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("Windows-only test")
    }
    if !IsUNC(`\\server\share\folder`) {
        t.Error("\\\\server\\share\\folder should be UNC")
    }
}

func TestIsUNC_NotUNCPath(t *testing.T) {
    if IsUNC(`C:\Users\dev`) {
        t.Error("C:\\Users\\dev should not be UNC")
    }
    if IsUNC("/home/user") {
        t.Error("/home/user should not be UNC")
    }
}

func TestNormalize_PathWithSpaces(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("Windows-only test")
    }
    result := Normalize(`C:\Program Files\My App\project`)
    expected := `C:\Program Files\My App\project`
    if result != expected {
        t.Errorf("expected %q, got %q", expected, result)
    }
}

func TestNormalize_RedundantSeparators(t *testing.T) {
    result := Normalize("path//to///file")
    // filepath.Clean removes redundant separators
    if runtime.GOOS == "windows" {
        expected := `path\to\file`
        if result != expected {
            t.Errorf("expected %q, got %q", expected, result)
        }
    } else {
        expected := "path/to/file"
        if result != expected {
            t.Errorf("expected %q, got %q", expected, result)
        }
    }
}

func TestNormalize_TrailingSlash(t *testing.T) {
    result := Normalize("path/to/dir/")
    // filepath.Clean removes trailing slash
    if runtime.GOOS == "windows" {
        expected := `path\to\dir`
        if result != expected {
            t.Errorf("expected %q, got %q", expected, result)
        }
    } else {
        expected := "path/to/dir"
        if result != expected {
            t.Errorf("expected %q, got %q", expected, result)
        }
    }
}

// === Conversion Tests ===

func TestToSlash(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("Windows-only test")
    }
    result := ToSlash(`C:\Users\dev\project`)
    expected := "C:/Users/dev/project"
    if result != expected {
        t.Errorf("expected %q, got %q", expected, result)
    }
}

func TestFromSlash(t *testing.T) {
    result := FromSlash("path/to/file")
    if runtime.GOOS == "windows" {
        expected := `path\to\file`
        if result != expected {
            t.Errorf("expected %q, got %q", expected, result)
        }
    } else {
        expected := "path/to/file"
        if result != expected {
            t.Errorf("expected %q, got %q", expected, result)
        }
    }
}
```

### Task 3: Audit Config Loader

**File:** `internal/config/loader.go`

**Check for:**
- Hardcoded path separators
- Assumptions about `.pommel` directory location
- Config file path construction

**Example fix:**
```go
// Before
configPath := projectRoot + "/.pommel/config.yaml"

// After
configPath := filepath.Join(projectRoot, ".pommel", "config.yaml")
```

### Task 4: Audit Daemon Ignorer

**File:** `internal/daemon/ignorer.go`

**Check for:**
- Pattern matching with path separators
- Glob patterns need to work on both platforms

**Consideration:**
- `.pommelignore` patterns should use `/` (like `.gitignore`)
- Convert to OS separators when matching

### Task 5: Audit Chunker File Paths

**Files:** `internal/chunker/*.go`

**Check for:**
- FilePath stored in chunks - should be consistent format
- Relative path calculation

**Decision:** Store paths with forward slashes in database for consistency, convert to OS separator on display.

### Task 6: Audit CLI Commands

**Files:** `internal/cli/*.go`

**Check for:**
- Path arguments from user input
- Display of paths in output
- Search result path formatting

### Task 7: Audit Database Paths

**File:** `internal/db/db.go`

**Check for:**
- Database file location
- Path storage in database

### Task 8: Integration Tests

**File:** `internal/pathutil/integration_test.go`

Create integration tests that exercise full workflows:

```go
func TestFullWorkflow_WindowsPaths(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("Windows-only test")
    }

    // Create temp directory
    tmpDir := t.TempDir()

    // Initialize Pommel
    // Add files with various path patterns
    // Verify chunking works
    // Verify search returns correct paths
}
```

## Test Cases Summary

### Unit Tests (pathutil package)

| Test | Platforms | Description |
|------|-----------|-------------|
| TestNormalize_UnixPath | All | Unix paths handled correctly |
| TestNormalize_WindowsPath | Windows | Windows paths with backslash |
| TestNormalize_MixedSeparators | Windows | Forward/back slash mix |
| TestIsAbsolute_WindowsDriveLetter | Windows | C:\ detected as absolute |
| TestIsAbsolute_UnixPath | Unix | /path detected as absolute |
| TestIsAbsolute_RelativePath | All | Relative paths detected |
| TestIsUNC_ValidUNCPath | Windows | \\server\share detected |
| TestIsUNC_NotUNCPath | All | Non-UNC paths rejected |
| TestNormalize_PathWithSpaces | Windows | Spaces preserved |
| TestNormalize_RedundantSeparators | All | // cleaned to / |
| TestNormalize_TrailingSlash | All | Trailing slash removed |
| TestToSlash | Windows | Backslash to forward |
| TestFromSlash | All | Forward to OS separator |

### Integration Tests

| Test | Description |
|------|-------------|
| TestConfig_WindowsPaths | Config loading with Windows paths |
| TestIgnorer_WindowsPatterns | .pommelignore on Windows |
| TestChunker_WindowsFilePaths | Chunk FilePath on Windows |
| TestSearch_WindowsPathDisplay | Search results display |

## Acceptance Criteria

- [ ] `pathutil` package created with tests
- [ ] All path utility tests pass on Windows
- [ ] Config loader uses `filepath.Join` consistently
- [ ] Ignorer handles patterns cross-platform
- [ ] Chunker stores consistent path format
- [ ] CLI displays OS-appropriate paths
- [ ] No hardcoded `/` separators in path construction
- [ ] Integration tests pass on Windows CI

## Files Changed

| File | Change |
|------|--------|
| `internal/pathutil/pathutil.go` | New utility package |
| `internal/pathutil/pathutil_test.go` | Path handling tests |
| `internal/config/loader.go` | Use filepath.Join |
| `internal/daemon/ignorer.go` | Cross-platform patterns |
| `internal/chunker/*.go` | Consistent path storage |
| `internal/cli/*.go` | Path display fixes |
| `internal/db/db.go` | Database path handling |

## Notes

- Use `filepath.Join()` for ALL path construction
- Use `filepath.ToSlash()` when storing paths (consistency)
- Use `filepath.FromSlash()` when displaying to user
- Consider storing relative paths in database to avoid drive letter issues
- UNC path support is nice-to-have, may defer to later
