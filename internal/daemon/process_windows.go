//go:build windows

package daemon

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// IsProcessRunning checks if a process with the given PID is running.
// On Windows, os.FindProcess always succeeds, so we use tasklist to check.
func IsProcessRunning(pid int) bool {
	// Use tasklist command to check if process exists
	cmd := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH", "/FO", "CSV")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// tasklist returns "INFO: No tasks are running which match the specified criteria."
	// if no process is found, or a CSV line with the process info if found
	outputStr := string(output)
	if strings.Contains(outputStr, "INFO:") || strings.Contains(outputStr, "No tasks") {
		return false
	}

	// Check if the PID appears in the output
	return strings.Contains(outputStr, strconv.Itoa(pid))
}

// TerminateProcess terminates a process by PID.
// On Windows, there's no equivalent to Unix signals for graceful shutdown,
// so we use os.Process.Kill() directly.
func TerminateProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	// On Windows, Kill() is the standard way to terminate a process
	// There's no SIGTERM equivalent for graceful shutdown
	return process.Kill()
}
