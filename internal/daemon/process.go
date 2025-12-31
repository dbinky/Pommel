package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// WritePIDFile writes a process ID to a file.
func WritePIDFile(path string, pid int) error {
	content := strconv.Itoa(pid)
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadPIDFile reads a process ID from a file.
func ReadPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID format: %w", err)
	}

	return pid, nil
}

// RemovePIDFile removes a PID file.
// Returns nil if the file doesn't exist.
func RemovePIDFile(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}
	return nil
}

// IsStalePIDFile checks if a PID file is stale (process not running).
func IsStalePIDFile(path string) bool {
	pid, err := ReadPIDFile(path)
	if err != nil {
		return true // Can't read file or invalid content = stale
	}

	return !IsProcessRunning(pid)
}
