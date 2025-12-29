package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	StateFile = "state.json"
	PIDFile   = "pommel.pid"
)

// DaemonState represents the persistent state of the Pommel daemon.
type DaemonState struct {
	Version int `json:"version"`
	Daemon  struct {
		PID       int       `json:"pid"`
		StartedAt time.Time `json:"started_at"`
		Port      int       `json:"port"`
	} `json:"daemon"`
	Index struct {
		LastFullIndex time.Time `json:"last_full_index"`
		TotalFiles    int       `json:"total_files"`
		TotalChunks   int       `json:"total_chunks"`
	} `json:"index"`
}

// StateManager handles persistence of daemon state and PID files.
type StateManager struct {
	pommelDir string
}

// NewStateManager creates a new StateManager for the given project root.
func NewStateManager(projectRoot string) *StateManager {
	return &StateManager{
		pommelDir: filepath.Join(projectRoot, ".pommel"),
	}
}

// ensureDir creates the .pommel directory if it doesn't exist.
func (s *StateManager) ensureDir() error {
	return os.MkdirAll(s.pommelDir, 0755)
}

// SaveState persists the daemon state to disk.
func (s *StateManager) SaveState(state *DaemonState) error {
	if err := s.ensureDir(); err != nil {
		return fmt.Errorf("failed to create .pommel directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	statePath := filepath.Join(s.pommelDir, StateFile)
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// LoadState reads the daemon state from disk.
// Returns a default DaemonState with Version=1 if the file doesn't exist.
func (s *StateManager) LoadState() (*DaemonState, error) {
	statePath := filepath.Join(s.pommelDir, StateFile)

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default state if file doesn't exist
			return &DaemonState{Version: 1}, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state DaemonState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// WritePID writes the daemon process ID to the PID file.
func (s *StateManager) WritePID(pid int) error {
	if err := s.ensureDir(); err != nil {
		return fmt.Errorf("failed to create .pommel directory: %w", err)
	}

	pidPath := filepath.Join(s.pommelDir, PIDFile)
	content := strconv.Itoa(pid)
	if err := os.WriteFile(pidPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// ReadPID reads the daemon process ID from the PID file.
func (s *StateManager) ReadPID() (int, error) {
	pidPath := filepath.Join(s.pommelDir, PIDFile)

	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID format: %w", err)
	}

	return pid, nil
}

// RemovePID deletes the PID file.
// Does not return an error if the file doesn't exist.
func (s *StateManager) RemovePID() error {
	pidPath := filepath.Join(s.pommelDir, PIDFile)

	err := os.Remove(pidPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}

	return nil
}

// IsRunning checks if the daemon process is currently running.
// Returns (true, pid) if running, (false, pid) if not running but PID file exists,
// or (false, 0) if no PID file exists.
// Cleans up stale PID files when the process is not running.
func (s *StateManager) IsRunning() (bool, int) {
	pid, err := s.ReadPID()
	if err != nil {
		return false, 0
	}

	// Check if process is alive using signal 0
	process, err := os.FindProcess(pid)
	if err != nil {
		// Process doesn't exist, clean up stale PID file
		_ = s.RemovePID()
		return false, pid
	}

	// Send signal 0 to check if process is running
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process is not running, clean up stale PID file
		_ = s.RemovePID()
		return false, pid
	}

	return true, pid
}
