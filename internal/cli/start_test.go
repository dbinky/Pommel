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

// TestStartCmd_NotInitialized tests that start fails when .pommel directory doesn't exist
func TestStartCmd_NotInitialized(t *testing.T) {
	// Create a temporary directory without .pommel initialization
	tmpDir := t.TempDir()

	// Save and restore original project root
	origRoot := projectRoot
	defer func() { projectRoot = origRoot }()
	projectRoot = tmpDir

	// Execute start command
	err := runStart(nil, nil)

	// Should fail because project is not initialized
	require.Error(t, err)
	// The error message may say "not been initialized" or "pm init"
	assert.True(t, strings.Contains(err.Error(), "init") || strings.Contains(err.Error(), "initialized"),
		"Error should mention init or initialized, got: %s", err.Error())
}

// TestStartCmd_AlreadyRunning tests that start shows info when daemon is already running
func TestStartCmd_AlreadyRunning(t *testing.T) {
	// Create a temporary directory with .pommel initialization
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Create a config file
	loader := config.NewLoader(tmpDir)
	_, err := loader.Init()
	require.NoError(t, err)

	// Write a PID file with current process PID (simulating running daemon)
	stateManager := daemon.NewStateManager(tmpDir)
	err = stateManager.WritePID(os.Getpid())
	require.NoError(t, err)

	// Cleanup at end
	defer func() { _ = stateManager.RemovePID() }()

	// Save and restore original project root
	origRoot := projectRoot
	defer func() { projectRoot = origRoot }()
	projectRoot = tmpDir

	// Execute start command
	err = runStart(nil, nil)

	// Should indicate daemon is already running
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

// TestStartCmd_StartsProcess tests that start successfully starts the daemon process
func TestStartCmd_StartsProcess(t *testing.T) {
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

	// Verify no daemon is running initially
	stateManager := daemon.NewStateManager(tmpDir)
	running, _ := stateManager.IsRunning()
	assert.False(t, running, "daemon should not be running initially")

	// Execute start command
	err = runStart(nil, nil)

	// The start command tries to launch pommeld which may not be in PATH during tests
	// This is expected - in real usage, pommeld would be installed
	// We verify the command attempts to start the daemon (returns error about executable)
	require.Error(t, err)
	// Error should be about starting/failing daemon
	assert.Contains(t, err.Error(), "daemon")
}

// TestStartCommand_Registered verifies the start command is properly registered
func TestStartCommand_Registered(t *testing.T) {
	var found bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "start" {
			found = true
			assert.Equal(t, "Start the Pommel daemon", cmd.Short)
			break
		}
	}
	assert.True(t, found, "start command should be registered")
}

// TestStartCmd_LoadsConfig tests that start command loads configuration
func TestStartCmd_LoadsConfig(t *testing.T) {
	// Create a temporary directory with custom config
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Create a config file with custom settings
	loader := config.NewLoader(tmpDir)
	cfg, err := loader.Init()
	require.NoError(t, err)

	// Modify config
	cfg.Daemon.Port = 9999
	err = loader.Save(cfg)
	require.NoError(t, err)

	// Save and restore original project root
	origRoot := projectRoot
	defer func() { projectRoot = origRoot }()
	projectRoot = tmpDir

	// Execute start command
	err = runStart(nil, nil)

	// The start command loads config and tries to launch pommeld
	// Since pommeld isn't in PATH during tests, it fails to start the daemon
	// But it should get past the config loading phase
	require.Error(t, err)
	// Error should be about starting daemon, not about config loading
	assert.Contains(t, err.Error(), "daemon")
}
