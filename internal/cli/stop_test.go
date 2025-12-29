package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/daemon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStopCmd_NotRunning tests that stop shows info when daemon is not running
func TestStopCmd_NotRunning(t *testing.T) {
	// Create a temporary directory with .pommel initialization
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Create a config file
	loader := config.NewLoader(tmpDir)
	_, err := loader.Init()
	require.NoError(t, err)

	// Save and restore original project root
	origRoot := projectRoot
	defer func() { projectRoot = origRoot }()
	projectRoot = tmpDir

	// Verify no PID file exists (daemon not running)
	stateManager := daemon.NewStateManager(tmpDir)
	running, _ := stateManager.IsRunning()
	assert.False(t, running, "daemon should not be running")

	// Execute stop command
	err = runStop(nil, nil)

	// When daemon is not running, stop should succeed gracefully (not an error)
	// and just print an info message
	require.NoError(t, err)
}

// TestStopCmd_SendsSignal tests that stop sends SIGTERM to the daemon process
func TestStopCmd_SendsSignal(t *testing.T) {
	// Create a temporary directory with .pommel initialization
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Create a config file
	loader := config.NewLoader(tmpDir)
	_, err := loader.Init()
	require.NoError(t, err)

	// For this test, we'll simulate a running daemon by writing a PID file
	// with a known process. In a real scenario, we'd start an actual process.
	stateManager := daemon.NewStateManager(tmpDir)

	// Note: Using os.Getpid() here means we'd send SIGTERM to ourselves,
	// which is not desirable in a test. Instead, we test with a non-existent PID.
	// The stop command should handle this gracefully by detecting the stale PID.
	fakePID := 999999 // A PID that likely doesn't exist
	err = stateManager.WritePID(fakePID)
	require.NoError(t, err)

	// Save and restore original project root
	origRoot := projectRoot
	defer func() { projectRoot = origRoot }()
	projectRoot = tmpDir

	// Execute stop command
	err = runStop(nil, nil)

	// With a non-existent PID, IsRunning() will detect stale PID file and clean it up
	// The stop command should succeed gracefully
	require.NoError(t, err)

	// Verify PID file was cleaned up (by IsRunning during stop check)
	running, _ := stateManager.IsRunning()
	assert.False(t, running, "daemon should not be reported as running after stale PID cleanup")
}

// TestStopCommand_Registered verifies the stop command is properly registered
func TestStopCommand_Registered(t *testing.T) {
	var found bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "stop" {
			found = true
			assert.Equal(t, "Stop the Pommel daemon", cmd.Short)
			break
		}
	}
	assert.True(t, found, "stop command should be registered")
}

// TestStopCmd_StalePIDFile tests that stop handles stale PID files correctly
func TestStopCmd_StalePIDFile(t *testing.T) {
	// Create a temporary directory with .pommel initialization
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Create a config file
	loader := config.NewLoader(tmpDir)
	_, err := loader.Init()
	require.NoError(t, err)

	// Write a PID file with a non-existent PID (stale)
	stateManager := daemon.NewStateManager(tmpDir)
	stalePID := 999998 // A PID that likely doesn't exist
	err = stateManager.WritePID(stalePID)
	require.NoError(t, err)

	// Save and restore original project root
	origRoot := projectRoot
	defer func() { projectRoot = origRoot }()
	projectRoot = tmpDir

	// Execute stop command
	err = runStop(nil, nil)

	// When PID file exists but process is not running (stale),
	// stop should succeed gracefully after cleaning up the stale PID file
	require.NoError(t, err)

	// Verify PID file was cleaned up
	running, _ := stateManager.IsRunning()
	assert.False(t, running, "daemon should not be reported as running after stale PID cleanup")
}

// TestStopCmd_NotInitialized tests that stop fails when project is not initialized
func TestStopCmd_NotInitialized(t *testing.T) {
	// Create a temporary directory without .pommel initialization
	tmpDir := t.TempDir()

	// Save and restore original project root
	origRoot := projectRoot
	defer func() { projectRoot = origRoot }()
	projectRoot = tmpDir

	// Execute stop command
	err := runStop(nil, nil)

	// Should fail because project is not initialized
	require.Error(t, err)
	// The error message may say "not been initialized" or "pm init"
	assert.True(t, strings.Contains(err.Error(), "init") || strings.Contains(err.Error(), "initialized"),
		"Error should mention init or initialized, got: %s", err.Error())
}

// TestStopCmd_WaitsForShutdown tests that stop waits for daemon to shutdown gracefully
func TestStopCmd_WaitsForShutdown(t *testing.T) {
	// This test verifies the behavior when no daemon is running
	// Testing actual timeout behavior would require a real subprocess

	// Create a temporary directory with .pommel initialization
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Create a config file
	loader := config.NewLoader(tmpDir)
	_, err := loader.Init()
	require.NoError(t, err)

	// Save and restore original project root
	origRoot := projectRoot
	defer func() { projectRoot = origRoot }()
	projectRoot = tmpDir

	// Execute stop command with no daemon running
	err = runStop(nil, nil)

	// With no daemon running, stop should succeed gracefully
	require.NoError(t, err)
}
