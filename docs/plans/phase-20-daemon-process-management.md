# Phase 20: Daemon Process Management for Windows

**Status:** Planning
**Branch:** dev-windows-support
**Depends on:** Phase 19 (Path Handling)

## Objective

Ensure the Pommel daemon (`pommeld`) can be started, stopped, and monitored correctly on Windows without relying on Unix-specific signals.

## Background

### Current Unix Implementation

The daemon currently uses:
- **Signals:** SIGTERM, SIGINT for graceful shutdown
- **PID file:** `.pommel/pommel.pid` to track running daemon
- **Process detection:** Check if PID is alive using `os.FindProcess` + signal 0

### Windows Differences

- **No Unix signals:** Windows doesn't have SIGTERM/SIGINT in the same way
- **Process termination:** Use `os.Process.Kill()` or Windows API
- **Process detection:** `os.FindProcess` always succeeds on Windows; need different approach
- **Ctrl+C handling:** Windows has `SetConsoleCtrlHandler` for console events

## Files to Modify

| File | Current Functionality | Windows Change Needed |
|------|----------------------|----------------------|
| `internal/daemon/daemon.go` | Signal handling for shutdown | Cross-platform shutdown |
| `internal/cli/start.go` | Launches daemon process | May need Windows adjustments |
| `internal/cli/stop.go` | Sends signal to stop daemon | Use process kill instead |
| `internal/daemon/state.go` | PID file management | Verify Windows compatibility |

## Implementation Tasks

### Task 1: Write Daemon Process Tests (TDD)

**File:** `internal/daemon/process_test.go`

Write tests BEFORE implementation:

```go
package daemon

import (
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "testing"
    "time"
)

// === Process Detection Tests ===

func TestIsProcessRunning_RunningProcess(t *testing.T) {
    // Start a long-running process
    var cmd *exec.Cmd
    if runtime.GOOS == "windows" {
        cmd = exec.Command("cmd", "/c", "ping -n 10 127.0.0.1")
    } else {
        cmd = exec.Command("sleep", "10")
    }
    err := cmd.Start()
    if err != nil {
        t.Fatalf("failed to start process: %v", err)
    }
    defer cmd.Process.Kill()

    pid := cmd.Process.Pid
    if !IsProcessRunning(pid) {
        t.Errorf("process %d should be running", pid)
    }
}

func TestIsProcessRunning_NotRunningProcess(t *testing.T) {
    // Use a PID that's very unlikely to exist
    // Note: This is fragile; better approach below
    if IsProcessRunning(99999999) {
        t.Error("process 99999999 should not be running")
    }
}

func TestIsProcessRunning_TerminatedProcess(t *testing.T) {
    var cmd *exec.Cmd
    if runtime.GOOS == "windows" {
        cmd = exec.Command("cmd", "/c", "echo hello")
    } else {
        cmd = exec.Command("echo", "hello")
    }
    err := cmd.Run() // Run and wait for completion
    if err != nil {
        t.Fatalf("failed to run process: %v", err)
    }

    pid := cmd.Process.Pid
    // Give OS time to clean up
    time.Sleep(100 * time.Millisecond)

    // Process should no longer be running
    // Note: PID may be reused, so this test is somewhat fragile
    if IsProcessRunning(pid) {
        t.Log("Warning: PID was reused or still in process table")
    }
}

// === PID File Tests ===

func TestWritePIDFile_CreatesFile(t *testing.T) {
    tmpDir := t.TempDir()
    pidFile := filepath.Join(tmpDir, "test.pid")

    err := WritePIDFile(pidFile, 12345)
    if err != nil {
        t.Fatalf("failed to write PID file: %v", err)
    }

    // Verify file exists
    if _, err := os.Stat(pidFile); os.IsNotExist(err) {
        t.Error("PID file should exist")
    }

    // Verify content
    pid, err := ReadPIDFile(pidFile)
    if err != nil {
        t.Fatalf("failed to read PID file: %v", err)
    }
    if pid != 12345 {
        t.Errorf("expected PID 12345, got %d", pid)
    }
}

func TestReadPIDFile_NonExistent(t *testing.T) {
    _, err := ReadPIDFile("/nonexistent/path/test.pid")
    if err == nil {
        t.Error("should error on non-existent file")
    }
}

func TestReadPIDFile_InvalidContent(t *testing.T) {
    tmpDir := t.TempDir()
    pidFile := filepath.Join(tmpDir, "test.pid")

    // Write invalid content
    err := os.WriteFile(pidFile, []byte("not-a-number"), 0644)
    if err != nil {
        t.Fatalf("failed to write file: %v", err)
    }

    _, err = ReadPIDFile(pidFile)
    if err == nil {
        t.Error("should error on invalid PID content")
    }
}

func TestRemovePIDFile_ExistingFile(t *testing.T) {
    tmpDir := t.TempDir()
    pidFile := filepath.Join(tmpDir, "test.pid")

    // Create file
    err := WritePIDFile(pidFile, 12345)
    if err != nil {
        t.Fatalf("failed to write PID file: %v", err)
    }

    // Remove it
    err = RemovePIDFile(pidFile)
    if err != nil {
        t.Fatalf("failed to remove PID file: %v", err)
    }

    // Verify removed
    if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
        t.Error("PID file should be removed")
    }
}

func TestRemovePIDFile_NonExistent(t *testing.T) {
    // Should not error if file doesn't exist
    err := RemovePIDFile("/nonexistent/test.pid")
    if err != nil {
        t.Errorf("should not error on non-existent file: %v", err)
    }
}

// === Process Termination Tests ===

func TestTerminateProcess_RunningProcess(t *testing.T) {
    var cmd *exec.Cmd
    if runtime.GOOS == "windows" {
        cmd = exec.Command("cmd", "/c", "ping -n 60 127.0.0.1")
    } else {
        cmd = exec.Command("sleep", "60")
    }
    err := cmd.Start()
    if err != nil {
        t.Fatalf("failed to start process: %v", err)
    }

    pid := cmd.Process.Pid

    // Verify running
    if !IsProcessRunning(pid) {
        t.Fatal("process should be running before termination")
    }

    // Terminate
    err = TerminateProcess(pid)
    if err != nil {
        t.Fatalf("failed to terminate process: %v", err)
    }

    // Wait a moment
    time.Sleep(500 * time.Millisecond)

    // Verify not running
    if IsProcessRunning(pid) {
        t.Error("process should not be running after termination")
    }
}

func TestTerminateProcess_NonExistentProcess(t *testing.T) {
    err := TerminateProcess(99999999)
    if err == nil {
        t.Error("should error when terminating non-existent process")
    }
}

// === Stale PID File Tests ===

func TestIsStale_ProcessNotRunning(t *testing.T) {
    tmpDir := t.TempDir()
    pidFile := filepath.Join(tmpDir, "test.pid")

    // Write a PID that doesn't exist
    err := WritePIDFile(pidFile, 99999999)
    if err != nil {
        t.Fatalf("failed to write PID file: %v", err)
    }

    if !IsStalePIDFile(pidFile) {
        t.Error("PID file with non-running process should be stale")
    }
}

func TestIsStale_ProcessRunning(t *testing.T) {
    tmpDir := t.TempDir()
    pidFile := filepath.Join(tmpDir, "test.pid")

    // Start a process
    var cmd *exec.Cmd
    if runtime.GOOS == "windows" {
        cmd = exec.Command("cmd", "/c", "ping -n 30 127.0.0.1")
    } else {
        cmd = exec.Command("sleep", "30")
    }
    err := cmd.Start()
    if err != nil {
        t.Fatalf("failed to start process: %v", err)
    }
    defer cmd.Process.Kill()

    // Write its PID
    err = WritePIDFile(pidFile, cmd.Process.Pid)
    if err != nil {
        t.Fatalf("failed to write PID file: %v", err)
    }

    if IsStalePIDFile(pidFile) {
        t.Error("PID file with running process should not be stale")
    }
}
```

### Task 2: Implement Cross-Platform Process Detection

**File:** `internal/daemon/process.go`

```go
package daemon

import (
    "fmt"
    "os"
    "runtime"
    "strconv"
    "strings"
)

// IsProcessRunning checks if a process with the given PID is running
func IsProcessRunning(pid int) bool {
    process, err := os.FindProcess(pid)
    if err != nil {
        return false
    }

    if runtime.GOOS == "windows" {
        // On Windows, FindProcess always succeeds
        // We need to try to open the process to check if it exists
        // Use a simple approach: try to signal it
        // Note: This requires a Windows-specific implementation
        return isProcessRunningWindows(pid)
    }

    // On Unix, send signal 0 to check if process exists
    err = process.Signal(os.Signal(nil))
    return err == nil
}

// isProcessRunningWindows checks if process is running on Windows
func isProcessRunningWindows(pid int) bool {
    // Use tasklist command to check if process exists
    // Alternative: Use Windows API via syscall
    cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
    output, err := cmd.Output()
    if err != nil {
        return false
    }
    return strings.Contains(string(output), strconv.Itoa(pid))
}
```

### Task 3: Implement Cross-Platform Process Termination

**File:** `internal/daemon/process.go` (continued)

```go
// TerminateProcess terminates a process by PID
func TerminateProcess(pid int) error {
    process, err := os.FindProcess(pid)
    if err != nil {
        return fmt.Errorf("process not found: %w", err)
    }

    if runtime.GOOS == "windows" {
        // On Windows, use Kill() directly
        // There's no graceful shutdown signal equivalent
        return process.Kill()
    }

    // On Unix, try SIGTERM first for graceful shutdown
    err = process.Signal(syscall.SIGTERM)
    if err != nil {
        // If SIGTERM fails, try SIGKILL
        return process.Kill()
    }

    // Wait briefly for graceful shutdown
    time.Sleep(2 * time.Second)

    // Check if still running
    if IsProcessRunning(pid) {
        // Force kill
        return process.Kill()
    }

    return nil
}
```

### Task 4: Update Daemon Shutdown Handling

**File:** `internal/daemon/daemon.go`

Current signal handling:
```go
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
```

Cross-platform version:
```go
import (
    "os"
    "os/signal"
    "runtime"
)

func setupShutdownHandler(shutdown chan struct{}) {
    sigChan := make(chan os.Signal, 1)

    if runtime.GOOS == "windows" {
        // On Windows, only os.Interrupt is reliably supported
        signal.Notify(sigChan, os.Interrupt)
    } else {
        // On Unix, handle SIGTERM and SIGINT
        signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
    }

    go func() {
        <-sigChan
        close(shutdown)
    }()
}
```

### Task 5: Update Stop Command

**File:** `internal/cli/stop.go`

Ensure stop command uses cross-platform termination:

```go
func stopDaemon(pidFile string) error {
    pid, err := daemon.ReadPIDFile(pidFile)
    if err != nil {
        return fmt.Errorf("daemon not running (no PID file): %w", err)
    }

    if !daemon.IsProcessRunning(pid) {
        // Stale PID file, clean it up
        daemon.RemovePIDFile(pidFile)
        return fmt.Errorf("daemon not running (stale PID file)")
    }

    err = daemon.TerminateProcess(pid)
    if err != nil {
        return fmt.Errorf("failed to stop daemon: %w", err)
    }

    // Wait for process to exit
    for i := 0; i < 10; i++ {
        if !daemon.IsProcessRunning(pid) {
            daemon.RemovePIDFile(pidFile)
            return nil
        }
        time.Sleep(500 * time.Millisecond)
    }

    return fmt.Errorf("daemon did not stop within timeout")
}
```

### Task 6: Update Start Command

**File:** `internal/cli/start.go`

Ensure start command works on Windows:
- Detached process launching may differ
- Verify `cmd.Start()` works correctly for background process

```go
func startDaemon() error {
    // Check if already running
    if daemon.IsProcessRunning(existingPID) {
        return fmt.Errorf("daemon already running (PID %d)", existingPID)
    }

    // Start daemon process
    daemonPath := findDaemonExecutable() // pommeld or pommeld.exe

    cmd := exec.Command(daemonPath, "--project", projectRoot)

    if runtime.GOOS == "windows" {
        // On Windows, use CREATE_NEW_PROCESS_GROUP for detached process
        cmd.SysProcAttr = &syscall.SysProcAttr{
            CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
        }
    } else {
        // On Unix, use Setpgid for process group
        cmd.SysProcAttr = &syscall.SysProcAttr{
            Setpgid: true,
        }
    }

    err := cmd.Start()
    if err != nil {
        return fmt.Errorf("failed to start daemon: %w", err)
    }

    // Don't wait for the process (it's a daemon)
    return nil
}
```

### Task 7: Integration Tests

**File:** `internal/daemon/daemon_integration_test.go`

```go
// +build integration

func TestDaemonStartStop_Windows(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("Windows-only test")
    }

    tmpDir := t.TempDir()
    pidFile := filepath.Join(tmpDir, "pommel.pid")

    // Start daemon (would need actual daemon binary)
    // This is more of a smoke test

    // Verify PID file created
    // Verify process running
    // Stop daemon
    // Verify process stopped
    // Verify PID file removed
}
```

## Test Cases Summary

### Unit Tests

| Test | Description |
|------|-------------|
| TestIsProcessRunning_RunningProcess | Detect running process |
| TestIsProcessRunning_NotRunningProcess | Detect non-existent process |
| TestIsProcessRunning_TerminatedProcess | Detect terminated process |
| TestWritePIDFile_CreatesFile | PID file creation |
| TestReadPIDFile_NonExistent | Handle missing PID file |
| TestReadPIDFile_InvalidContent | Handle corrupt PID file |
| TestRemovePIDFile_ExistingFile | PID file removal |
| TestRemovePIDFile_NonExistent | Remove non-existent file |
| TestTerminateProcess_RunningProcess | Kill running process |
| TestTerminateProcess_NonExistentProcess | Handle missing process |
| TestIsStale_ProcessNotRunning | Detect stale PID file |
| TestIsStale_ProcessRunning | Detect valid PID file |

### Integration Tests

| Test | Description |
|------|-------------|
| TestDaemonStartStop_Windows | Full start/stop cycle on Windows |
| TestDaemonStartStop_Unix | Full start/stop cycle on Unix |

## Acceptance Criteria

- [ ] `IsProcessRunning` works on Windows and Unix
- [ ] `TerminateProcess` works on Windows and Unix
- [ ] PID file operations work on Windows
- [ ] `pm start` launches daemon on Windows
- [ ] `pm stop` terminates daemon on Windows
- [ ] `pm status` correctly reports daemon state on Windows
- [ ] Stale PID files are detected and cleaned up
- [ ] All tests pass on Windows CI

## Files Changed

| File | Change |
|------|--------|
| `internal/daemon/process.go` | New cross-platform process utilities |
| `internal/daemon/process_test.go` | Process utility tests |
| `internal/daemon/daemon.go` | Cross-platform signal handling |
| `internal/cli/start.go` | Windows process launching |
| `internal/cli/stop.go` | Cross-platform termination |

## Notes

- Windows doesn't have true equivalent of Unix signals
- `os.Process.Kill()` is the most reliable cross-platform termination
- Consider adding graceful shutdown timeout before force kill
- May need to handle Windows-specific console control events (Ctrl+C)
- `tasklist` command approach is simple but spawns a process; consider Windows API for performance
