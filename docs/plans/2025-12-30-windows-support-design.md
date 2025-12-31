# Windows Support Design

**Date:** 2025-12-30
**Status:** Draft
**Branch:** dev-windows-support

## Overview

Add full native Windows support to Pommel, enabling installation and use via PowerShell with the same functionality as macOS/Linux.

### Goals

- Pommel runs natively on Windows (x64 and ARM64)
- PowerShell install script (`irm <url> | iex` pattern)
- All CLI commands work identically in PowerShell
- GitHub Actions produces Windows binaries automatically

### Non-Goals

- Windows Service integration (using background process model instead)
- GUI or system tray application
- Microsoft Store distribution

## What's Changing

| Area | Change |
|------|--------|
| Path handling | Support backslashes, drive letters, UNC paths |
| Daemon management | Cross-platform process start/stop (no Unix signals) |
| File watching | Validate fsnotify on Windows, handle locked files |
| Install script | New `scripts/install.ps1` for PowerShell |
| CI/CD | Windows runners, produce `.exe` binaries |
| Release assets | Add Windows x64 and ARM64 builds |

## What's NOT Changing

- Project-local `.pommel/` directory structure
- Daemon model (background process)
- CLI interface and commands
- Core chunking/embedding/search logic
- SQLite + sqlite-vec storage

## Code Changes

### Path Handling

**Files affected:** Most of `internal/` packages

**Requirements:**
- Use `filepath.Join()` consistently (already does OS-appropriate separators)
- Audit for hardcoded `/` in path construction
- Handle drive letters in path validation (e.g., `C:\projects\myapp`)
- Use `filepath.IsAbs()` for absolute path detection (works cross-platform)
- Support UNC paths (`\\server\share\project`)
- Handle paths with spaces
- Consider long path limitations (>260 chars)

**Test cases:**
```go
// Happy path
TestPathHandling_WindowsAbsolute       // C:\Users\dev\project
TestPathHandling_WindowsDriveLetter    // D:\code\myapp
TestPathHandling_UnixStyle             // /home/user/project (still works)

// Edge cases
TestPathHandling_UNCPath               // \\server\share\project
TestPathHandling_MixedSeparators       // C:/Users/dev\project -> normalizes
TestPathHandling_LongPath              // > 260 characters
TestPathHandling_PathWithSpaces        // C:\Program Files\My Project

// Error cases
TestPathHandling_InvalidDriveLetter    // Z:\ (non-existent drive)
TestPathHandling_MalformedUNC          // \\server (incomplete)
```

### Daemon Process Management

**Files affected:** `internal/daemon/daemon.go`, `internal/cli/start.go`, `internal/cli/stop.go`

**Current behavior:**
- Uses Unix signals (SIGTERM, SIGINT) for graceful shutdown
- PID file for tracking running daemon

**Windows changes:**
- Replace signal handling with `os.Process.Kill()` or cross-platform alternative
- PID files work the same on Windows
- Use `os.FindProcess()` + process handle for stop

**Test cases:**
```go
// Happy path
TestDaemon_StartOnWindows              // Daemon starts, PID file created
TestDaemon_StopOnWindows               // Daemon stops via process kill
TestDaemon_StatusOnWindows             // Correctly reports running/stopped

// Edge cases
TestDaemon_AlreadyRunning              // Error if daemon already running
TestDaemon_StalePIDFile                // Process died, PID file remains
TestDaemon_ProcessNotFound             // PID exists but process gone

// Error cases
TestDaemon_PermissionDenied            // Can't kill another user's process
```

### File Watching

**Files affected:** `internal/daemon/watcher.go`

**Current behavior:**
- Uses fsnotify, which supports Windows natively
- Debounces rapid changes

**Windows considerations:**
- Locked files (file open in editor/IDE)
- Different behavior on network drives
- Rapid successive changes

**Test cases:**
```go
// Happy path
TestWatcher_FileChangeWindows          // Basic file modification detected
TestWatcher_FileCreateWindows          // New file detected
TestWatcher_FileDeleteWindows          // Deleted file detected

// Edge cases
TestWatcher_LockedFile                 // File open in another process
TestWatcher_RapidChanges               // Multiple saves in quick succession
TestWatcher_DeepNestedPath             // Deeply nested directory structure

// Error cases
TestWatcher_PermissionDenied           // Can't read file/directory
```

## PowerShell Install Script

**File:** `scripts/install.ps1`

**Functionality (mirrors install.sh):**

1. Check prerequisites (PowerShell 5.1+)
2. Detect architecture (x64 or ARM64)
3. Download `pm.exe` and `pommeld.exe` from GitHub releases
4. Install to `$env:LOCALAPPDATA\Pommel\bin`
5. Add to user PATH (no admin required)
6. Check for/install Ollama via winget
7. Pull embedding model
8. Verify installation (`pm version`)
9. Print success message with next steps

**Error handling:**
- Check if winget is available (Windows 10 1709+ / Windows 11)
- Graceful fallback if Ollama install fails (print manual instructions)
- Verify downloads succeeded

**Invocation:**
```powershell
irm https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.ps1 | iex
```

**Script outline:**
```powershell
#Requires -Version 5.1

$ErrorActionPreference = "Stop"

# Configuration
$repo = "dbinky/Pommel"
$installDir = "$env:LOCALAPPDATA\Pommel\bin"
$ollamaModel = "unclemusclez/jina-embeddings-v2-base-code"

function Get-Architecture {
    if ([Environment]::Is64BitOperatingSystem) {
        if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
            return "arm64"
        }
        return "amd64"
    }
    throw "32-bit Windows is not supported"
}

function Install-Pommel { ... }
function Install-Ollama { ... }
function Add-ToPath { ... }
function Test-Installation { ... }

# Main
Install-Pommel
Install-Ollama
Add-ToPath
Test-Installation

Write-Host "Pommel installed successfully!" -ForegroundColor Green
Write-Host "Run 'pm init' in your project directory to get started."
```

## GitHub Actions Changes

### CI Workflow (`.github/workflows/ci.yml`)

Add Windows to test matrix:

```yaml
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go test ./...
```

### Release Workflow (`.github/workflows/release.yml`)

Add Windows builds:

```yaml
jobs:
  build:
    strategy:
      matrix:
        include:
          - os: ubuntu-latest
            goos: linux
            goarch: amd64
            ext: ""
          - os: ubuntu-latest
            goos: linux
            goarch: arm64
            ext: ""
          - os: macos-latest
            goos: darwin
            goarch: amd64
            ext: ""
          - os: macos-latest
            goos: darwin
            goarch: arm64
            ext: ""
          - os: windows-latest
            goos: windows
            goarch: amd64
            ext: ".exe"
          - os: windows-latest
            goos: windows
            goarch: arm64
            ext: ".exe"
```

### Release Assets

After implementation, releases will include:

```
pm-darwin-amd64
pm-darwin-arm64
pm-linux-amd64
pm-linux-arm64
pm-windows-amd64.exe
pm-windows-arm64.exe
pommeld-darwin-amd64
pommeld-darwin-arm64
pommeld-linux-amd64
pommeld-linux-arm64
pommeld-windows-amd64.exe
pommeld-windows-arm64.exe
```

## Implementation Phases

### Phase 1: CI/CD Setup
- Add Windows runners to CI workflow (x64 + arm64)
- Add Windows builds to release workflow
- Verify existing tests pass on Windows

### Phase 2: Path Handling (TDD)
- Write path handling tests for Windows scenarios
- Audit codebase for hardcoded `/` separators
- Fix any failures, ensure cross-platform consistency

### Phase 3: Daemon Process Management (TDD)
- Write Windows-specific daemon start/stop tests
- Replace Unix signal handling with cross-platform approach
- Test PID file handling on Windows

### Phase 4: File Watcher Validation (TDD)
- Write Windows file watcher edge case tests
- Add graceful handling for locked files
- Validate debouncing behavior

### Phase 5: PowerShell Install Script
- Write `install.ps1` mirroring `install.sh`
- Test on Windows machine (via beads handoff)
- Document in README

### Phase 6: Documentation & Release
- Update README with Windows instructions
- Update `pm init --claude` output to mention Windows
- Tag release, announce Windows support

## Testing on Windows

Development happens on macOS. Windows testing workflow:

1. Write tests and code on macOS
2. Push to branch, GitHub Actions runs tests on Windows
3. For manual testing, use Windows machine with separate Claude instance
4. Use beads to track Windows-specific testing tasks
5. Create `docs/plans/windows_testing.md` with context for Windows Claude instance

## Success Criteria

- [ ] `pm init` works in PowerShell
- [ ] `pm start` launches daemon successfully
- [ ] `pm search` returns results
- [ ] `pm stop` terminates daemon cleanly
- [ ] `pm status` reports correct state
- [ ] File changes trigger re-indexing
- [ ] Install script works via `irm | iex`
- [ ] All tests pass on Windows CI
- [ ] ARM64 builds work on Windows ARM devices
