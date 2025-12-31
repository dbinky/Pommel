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
		cmd = exec.Command("cmd", "/c", "ping -n 10 127.0.0.1 > NUL")
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
	if IsProcessRunning(99999999) {
		t.Error("process 99999999 should not be running")
	}
}

func TestIsProcessRunning_CurrentProcess(t *testing.T) {
	// Current process should always be running
	pid := os.Getpid()
	if !IsProcessRunning(pid) {
		t.Errorf("current process %d should be running", pid)
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

	// Process should no longer be running (though PID may be reused)
	// This is just a sanity check - if it passes, good; if not, just log
	if IsProcessRunning(pid) {
		t.Log("Note: PID was possibly reused or still in process table")
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
	tmpDir := t.TempDir()
	_, err := ReadPIDFile(filepath.Join(tmpDir, "nonexistent.pid"))
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
	tmpDir := t.TempDir()
	// Should not error if file doesn't exist
	err := RemovePIDFile(filepath.Join(tmpDir, "nonexistent.pid"))
	if err != nil {
		t.Errorf("should not error on non-existent file: %v", err)
	}
}

// === Process Termination Tests ===

func TestTerminateProcess_RunningProcess(t *testing.T) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "ping -n 60 127.0.0.1 > NUL")
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

	// Wait for process to exit (with retries to handle timing)
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if !IsProcessRunning(pid) {
			return // Success
		}
	}

	// If still running after 5 seconds, fail
	t.Error("process should not be running after termination (waited 5s)")
}

func TestTerminateProcess_NonExistentProcess(t *testing.T) {
	err := TerminateProcess(99999999)
	if err == nil {
		t.Error("should error when terminating non-existent process")
	}
}

// === Stale PID File Tests ===

func TestIsStalePIDFile_ProcessNotRunning(t *testing.T) {
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

func TestIsStalePIDFile_ProcessRunning(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")

	// Start a process
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "ping -n 30 127.0.0.1 > NUL")
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

func TestIsStalePIDFile_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	// Non-existent PID file should be considered stale
	if !IsStalePIDFile(filepath.Join(tmpDir, "nonexistent.pid")) {
		t.Error("non-existent PID file should be considered stale")
	}
}
