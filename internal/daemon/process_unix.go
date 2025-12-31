//go:build !windows

package daemon

import (
	"os"
	"syscall"
	"time"
)

// IsProcessRunning checks if a process with the given PID is running.
// On Unix, this uses signal 0 to check if the process exists.
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// TerminateProcess terminates a process by PID.
// On Unix, this first tries SIGTERM for graceful shutdown,
// then falls back to SIGKILL if the process doesn't exit.
func TerminateProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	// First try SIGTERM for graceful shutdown
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		// If SIGTERM fails, try SIGKILL
		if killErr := process.Kill(); killErr != nil {
			return killErr
		}
		// Wait for process to fully exit after SIGKILL
		_, _ = process.Wait()
		return nil
	}

	// Wait briefly for graceful shutdown
	for i := 0; i < 15; i++ {
		time.Sleep(200 * time.Millisecond)
		if !IsProcessRunning(pid) {
			return nil
		}
	}

	// Force kill if still running after 3 seconds
	if err := process.Kill(); err != nil {
		return err
	}

	// Wait for process to fully exit after SIGKILL
	_, _ = process.Wait()
	return nil
}
