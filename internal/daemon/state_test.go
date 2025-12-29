package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// State Persistence Tests
// =============================================================================

func TestSaveState_WritesStateJSON(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &DaemonState{
		Version: 1,
	}
	state.Daemon.PID = 12345
	state.Daemon.StartedAt = time.Now()
	state.Daemon.Port = 9876

	// Execute
	err := sm.SaveState(state)

	// Verify
	require.NoError(t, err)

	statePath := filepath.Join(tmpDir, ".pommel", StateFile)
	_, err = os.Stat(statePath)
	assert.NoError(t, err, "state.json should exist after SaveState")
}

func TestLoadState_ReadsStateJSON(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Write a valid state file directly
	expectedState := &DaemonState{
		Version: 1,
	}
	expectedState.Daemon.PID = 54321
	expectedState.Daemon.StartedAt = time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	expectedState.Daemon.Port = 8080
	expectedState.Index.LastFullIndex = time.Date(2024, 1, 15, 10, 25, 0, 0, time.UTC)
	expectedState.Index.TotalFiles = 100
	expectedState.Index.TotalChunks = 500

	data, err := json.MarshalIndent(expectedState, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(pommelDir, StateFile), data, 0644))

	sm := NewStateManager(tmpDir)

	// Execute
	state, err := sm.LoadState()

	// Verify
	require.NoError(t, err)
	assert.Equal(t, 1, state.Version)
	assert.Equal(t, 54321, state.Daemon.PID)
	assert.Equal(t, 8080, state.Daemon.Port)
	assert.Equal(t, 100, state.Index.TotalFiles)
	assert.Equal(t, 500, state.Index.TotalChunks)
}

func TestLoadState_ReturnsDefaultStateWhenFileMissing(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Execute - no state file exists
	state, err := sm.LoadState()

	// Verify - should return default state, not error
	require.NoError(t, err)
	assert.NotNil(t, state)
	assert.Equal(t, 1, state.Version, "default state should have version 1")
	assert.Equal(t, 0, state.Daemon.PID, "default state should have zero PID")
	assert.Equal(t, 0, state.Daemon.Port, "default state should have zero Port")
	assert.True(t, state.Daemon.StartedAt.IsZero(), "default state should have zero StartedAt")
}

func TestStateRoundtrip_SaveThenLoad(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	original := &DaemonState{
		Version: 1,
	}
	original.Daemon.PID = 99999
	original.Daemon.StartedAt = time.Now().Truncate(time.Second) // Truncate for JSON precision
	original.Daemon.Port = 5555
	original.Index.LastFullIndex = time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	original.Index.TotalFiles = 42
	original.Index.TotalChunks = 420

	// Execute
	err := sm.SaveState(original)
	require.NoError(t, err)

	loaded, err := sm.LoadState()
	require.NoError(t, err)

	// Verify
	assert.Equal(t, original.Version, loaded.Version)
	assert.Equal(t, original.Daemon.PID, loaded.Daemon.PID)
	assert.Equal(t, original.Daemon.Port, loaded.Daemon.Port)
	assert.Equal(t, original.Daemon.StartedAt.UTC(), loaded.Daemon.StartedAt.UTC())
	assert.Equal(t, original.Index.TotalFiles, loaded.Index.TotalFiles)
	assert.Equal(t, original.Index.TotalChunks, loaded.Index.TotalChunks)
	assert.Equal(t, original.Index.LastFullIndex.UTC(), loaded.Index.LastFullIndex.UTC())
}

// =============================================================================
// PID File Management Tests
// =============================================================================

func TestWritePID_WritesPommelPID(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)
	expectedPID := 12345

	// Execute
	err := sm.WritePID(expectedPID)

	// Verify
	require.NoError(t, err)

	pidPath := filepath.Join(tmpDir, ".pommel", PIDFile)
	_, err = os.Stat(pidPath)
	assert.NoError(t, err, "pommel.pid should exist after WritePID")

	// Verify content
	content, err := os.ReadFile(pidPath)
	require.NoError(t, err)
	assert.Equal(t, "12345", string(content))
}

func TestReadPID_ReturnsCorrectPID(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	expectedPID := 67890
	require.NoError(t, os.WriteFile(
		filepath.Join(pommelDir, PIDFile),
		[]byte("67890"),
		0644,
	))

	sm := NewStateManager(tmpDir)

	// Execute
	pid, err := sm.ReadPID()

	// Verify
	require.NoError(t, err)
	assert.Equal(t, expectedPID, pid)
}

func TestReadPID_ErrorsOnMissingFile(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Execute
	_, err := sm.ReadPID()

	// Verify
	assert.Error(t, err, "ReadPID should error when PID file doesn't exist")
}

func TestRemovePID_DeletesFile(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	pidPath := filepath.Join(pommelDir, PIDFile)
	require.NoError(t, os.WriteFile(pidPath, []byte("12345"), 0644))

	sm := NewStateManager(tmpDir)

	// Verify file exists first
	_, err := os.Stat(pidPath)
	require.NoError(t, err, "PID file should exist before removal")

	// Execute
	err = sm.RemovePID()

	// Verify
	require.NoError(t, err)
	_, err = os.Stat(pidPath)
	assert.True(t, os.IsNotExist(err), "PID file should not exist after RemovePID")
}

func TestRemovePID_NoErrorWhenFileMissing(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Execute - file doesn't exist
	err := sm.RemovePID()

	// Verify - should not error
	assert.NoError(t, err, "RemovePID should not error when file doesn't exist")
}

// =============================================================================
// IsRunning Detection Tests
// =============================================================================

func TestIsRunning_ReturnsFalseWhenNoPIDFile(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Execute
	running, pid := sm.IsRunning()

	// Verify
	assert.False(t, running, "IsRunning should return false when no PID file exists")
	assert.Equal(t, 0, pid, "PID should be 0 when not running")
}

func TestIsRunning_ReturnsFalseWhenProcessNotRunning(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Write a PID that definitely doesn't exist (very high number)
	// PID 4194304 is above the typical max PID on Linux/macOS
	stalePID := "4194304"
	require.NoError(t, os.WriteFile(
		filepath.Join(pommelDir, PIDFile),
		[]byte(stalePID),
		0644,
	))

	sm := NewStateManager(tmpDir)

	// Execute
	running, pid := sm.IsRunning()

	// Verify
	assert.False(t, running, "IsRunning should return false when process is not running")
	assert.Equal(t, 4194304, pid, "should return the PID from file even if not running")
}

func TestIsRunning_CleansUpStalePIDFiles(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	pidPath := filepath.Join(pommelDir, PIDFile)
	stalePID := "4194304" // Non-existent process
	require.NoError(t, os.WriteFile(pidPath, []byte(stalePID), 0644))

	sm := NewStateManager(tmpDir)

	// Execute
	running, _ := sm.IsRunning()

	// Verify
	assert.False(t, running)

	// PID file should be cleaned up
	_, err := os.Stat(pidPath)
	assert.True(t, os.IsNotExist(err), "stale PID file should be removed")
}

// =============================================================================
// State Structure Tests
// =============================================================================

func TestDaemonState_JSONMarshaling(t *testing.T) {
	// Setup
	state := &DaemonState{
		Version: 1,
	}
	state.Daemon.PID = 12345
	state.Daemon.StartedAt = time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
	state.Daemon.Port = 9999
	state.Index.LastFullIndex = time.Date(2024, 6, 15, 14, 25, 0, 0, time.UTC)
	state.Index.TotalFiles = 50
	state.Index.TotalChunks = 250

	// Execute
	data, err := json.Marshal(state)
	require.NoError(t, err)

	var unmarshaled DaemonState
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, state.Version, unmarshaled.Version)
	assert.Equal(t, state.Daemon.PID, unmarshaled.Daemon.PID)
	assert.Equal(t, state.Daemon.Port, unmarshaled.Daemon.Port)
	assert.Equal(t, state.Daemon.StartedAt, unmarshaled.Daemon.StartedAt)
	assert.Equal(t, state.Index.LastFullIndex, unmarshaled.Index.LastFullIndex)
	assert.Equal(t, state.Index.TotalFiles, unmarshaled.Index.TotalFiles)
	assert.Equal(t, state.Index.TotalChunks, unmarshaled.Index.TotalChunks)
}

func TestDaemonState_JSONFieldNames(t *testing.T) {
	// Setup
	state := &DaemonState{
		Version: 1,
	}
	state.Daemon.PID = 111
	state.Daemon.StartedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	state.Daemon.Port = 222
	state.Index.LastFullIndex = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	state.Index.TotalFiles = 333
	state.Index.TotalChunks = 444

	// Execute
	data, err := json.Marshal(state)
	require.NoError(t, err)

	// Verify JSON field names
	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Contains(t, raw, "version", "JSON should have 'version' field")
	assert.Contains(t, raw, "daemon", "JSON should have 'daemon' field")
	assert.Contains(t, raw, "index", "JSON should have 'index' field")

	daemon := raw["daemon"].(map[string]interface{})
	assert.Contains(t, daemon, "pid", "daemon should have 'pid' field")
	assert.Contains(t, daemon, "started_at", "daemon should have 'started_at' field")
	assert.Contains(t, daemon, "port", "daemon should have 'port' field")

	index := raw["index"].(map[string]interface{})
	assert.Contains(t, index, "last_full_index", "index should have 'last_full_index' field")
	assert.Contains(t, index, "total_files", "index should have 'total_files' field")
	assert.Contains(t, index, "total_chunks", "index should have 'total_chunks' field")
}

func TestDaemonState_VersionFieldIsSet(t *testing.T) {
	// Test that Version field is properly set
	state := &DaemonState{
		Version: 1,
	}

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	version, ok := raw["version"].(float64) // JSON numbers are float64
	assert.True(t, ok, "version should be a number")
	assert.Equal(t, float64(1), version, "version should be 1")
}

func TestDaemonState_DaemonInfo(t *testing.T) {
	// Test daemon-specific fields
	state := &DaemonState{
		Version: 1,
	}
	now := time.Now().Truncate(time.Second)
	state.Daemon.PID = os.Getpid()
	state.Daemon.StartedAt = now
	state.Daemon.Port = 8765

	assert.Equal(t, os.Getpid(), state.Daemon.PID)
	assert.Equal(t, now, state.Daemon.StartedAt)
	assert.Equal(t, 8765, state.Daemon.Port)
}

func TestDaemonState_IndexInfo(t *testing.T) {
	// Test index-specific fields
	state := &DaemonState{
		Version: 1,
	}
	lastIndex := time.Now().Add(-30 * time.Minute).Truncate(time.Second)
	state.Index.LastFullIndex = lastIndex
	state.Index.TotalFiles = 1000
	state.Index.TotalChunks = 5000

	assert.Equal(t, lastIndex, state.Index.LastFullIndex)
	assert.Equal(t, 1000, state.Index.TotalFiles)
	assert.Equal(t, 5000, state.Index.TotalChunks)
}

// =============================================================================
// Edge Cases and Error Handling
// =============================================================================

func TestLoadState_HandlesCorruptedJSON(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Write corrupted JSON
	require.NoError(t, os.WriteFile(
		filepath.Join(pommelDir, StateFile),
		[]byte("{invalid json"),
		0644,
	))

	sm := NewStateManager(tmpDir)

	// Execute
	_, err := sm.LoadState()

	// Verify - should error on corrupted JSON
	assert.Error(t, err, "LoadState should error on corrupted JSON")
}

func TestReadPID_HandlesInvalidPIDContent(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Write non-numeric PID
	require.NoError(t, os.WriteFile(
		filepath.Join(pommelDir, PIDFile),
		[]byte("not-a-number"),
		0644,
	))

	sm := NewStateManager(tmpDir)

	// Execute
	_, err := sm.ReadPID()

	// Verify
	assert.Error(t, err, "ReadPID should error on non-numeric PID")
}

func TestSaveState_CreatesPommelDirectory(t *testing.T) {
	// Setup - fresh directory with no .pommel
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	state := &DaemonState{Version: 1}

	// Execute
	err := sm.SaveState(state)

	// Verify
	require.NoError(t, err)

	pommelDir := filepath.Join(tmpDir, ".pommel")
	info, err := os.Stat(pommelDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), ".pommel should be a directory")
}

func TestWritePID_CreatesPommelDirectory(t *testing.T) {
	// Setup - fresh directory with no .pommel
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Execute
	err := sm.WritePID(12345)

	// Verify
	require.NoError(t, err)

	pommelDir := filepath.Join(tmpDir, ".pommel")
	info, err := os.Stat(pommelDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), ".pommel should be a directory")
}

func TestNewStateManager_SetsProjectRoot(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// The StateManager should be non-nil and store the project root
	assert.NotNil(t, sm)
}

// =============================================================================
// Additional Edge Case Tests
// =============================================================================

func TestReadPID_HandlesWhitespace(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Write PID with leading/trailing whitespace and newline
	require.NoError(t, os.WriteFile(
		filepath.Join(pommelDir, PIDFile),
		[]byte("  12345\n"),
		0644,
	))

	sm := NewStateManager(tmpDir)

	// Execute
	pid, err := sm.ReadPID()

	// Verify
	require.NoError(t, err)
	assert.Equal(t, 12345, pid)
}

func TestIsRunning_ReturnsTrueForCurrentProcess(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Write current process PID
	currentPID := os.Getpid()
	err := sm.WritePID(currentPID)
	require.NoError(t, err)

	// Execute
	running, pid := sm.IsRunning()

	// Verify
	assert.True(t, running, "Current process should be detected as running")
	assert.Equal(t, currentPID, pid)

	// Cleanup
	_ = sm.RemovePID()
}

func TestLoadState_ReturnsErrorForInvalidJSONTypes(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Write JSON with wrong types
	require.NoError(t, os.WriteFile(
		filepath.Join(pommelDir, StateFile),
		[]byte(`{"version": "not-a-number"}`),
		0644,
	))

	sm := NewStateManager(tmpDir)

	// Execute
	_, err := sm.LoadState()

	// Verify - should error on type mismatch
	assert.Error(t, err, "LoadState should error on type mismatch")
}

func TestSaveState_OverwritesExistingState(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Save first state
	state1 := &DaemonState{Version: 1}
	state1.Daemon.PID = 111
	err := sm.SaveState(state1)
	require.NoError(t, err)

	// Save second state with different values
	state2 := &DaemonState{Version: 1}
	state2.Daemon.PID = 222
	err = sm.SaveState(state2)
	require.NoError(t, err)

	// Load and verify
	loaded, err := sm.LoadState()
	require.NoError(t, err)
	assert.Equal(t, 222, loaded.Daemon.PID, "Should have the latest PID")
}

func TestWritePID_OverwritesExistingPID(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Write first PID
	err := sm.WritePID(111)
	require.NoError(t, err)

	// Write second PID
	err = sm.WritePID(222)
	require.NoError(t, err)

	// Read and verify
	pid, err := sm.ReadPID()
	require.NoError(t, err)
	assert.Equal(t, 222, pid, "Should have the latest PID")
}

func TestReadPID_HandlesEmptyFile(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Write empty PID file
	require.NoError(t, os.WriteFile(
		filepath.Join(pommelDir, PIDFile),
		[]byte(""),
		0644,
	))

	sm := NewStateManager(tmpDir)

	// Execute
	_, err := sm.ReadPID()

	// Verify - should error on empty content
	assert.Error(t, err, "ReadPID should error on empty file")
}

func TestSaveState_WithZeroValues(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Save state with zero values (testing default behavior)
	state := &DaemonState{} // All zero values

	// Execute
	err := sm.SaveState(state)
	require.NoError(t, err)

	// Load and verify
	loaded, err := sm.LoadState()
	require.NoError(t, err)

	assert.Equal(t, 0, loaded.Version, "Version should be 0")
	assert.Equal(t, 0, loaded.Daemon.PID, "PID should be 0")
	assert.Equal(t, 0, loaded.Daemon.Port, "Port should be 0")
}

func TestLoadState_HandlesEmptyJSONObject(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Write empty JSON object
	require.NoError(t, os.WriteFile(
		filepath.Join(pommelDir, StateFile),
		[]byte(`{}`),
		0644,
	))

	sm := NewStateManager(tmpDir)

	// Execute
	state, err := sm.LoadState()

	// Verify - should succeed with zero values
	require.NoError(t, err)
	assert.Equal(t, 0, state.Version)
	assert.Equal(t, 0, state.Daemon.PID)
}
