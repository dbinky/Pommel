package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCmd_CreatesDirectory(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init command
	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)
	require.NoError(t, err, "Init command should succeed")

	// Verify .pommel directory was created
	pommelDir := filepath.Join(tmpDir, ".pommel")
	info, err := os.Stat(pommelDir)
	require.NoError(t, err, ".pommel directory should exist")
	assert.True(t, info.IsDir(), ".pommel should be a directory")
}

func TestInitCmd_AlreadyInitialized(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create .pommel directory with config to simulate already initialized
	pommelDir := filepath.Join(tmpDir, ".pommel")
	err = os.MkdirAll(pommelDir, 0755)
	require.NoError(t, err)

	// Create a config file
	loader := config.NewLoader(tmpDir)
	_, err = loader.Init()
	require.NoError(t, err)

	// Run init command again - should show warning
	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)

	// Should either return an error or show a warning
	// The exact behavior depends on implementation
	if err != nil {
		assert.Contains(t, err.Error(), "already", "Error should mention already initialized")
	} else {
		assert.Contains(t, errBuf.String(), "already", "Should warn about already initialized")
	}
}

func TestInitCmd_CreatesConfig(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init command
	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)
	require.NoError(t, err, "Init command should succeed")

	// Verify config.yaml was created
	configPath := filepath.Join(tmpDir, ".pommel", "config.yaml")
	_, err = os.Stat(configPath)
	require.NoError(t, err, "config.yaml should exist")

	// Verify config can be loaded
	loader := config.NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err, "Config should be loadable")
	assert.NotNil(t, cfg, "Config should not be nil")

	// Verify default values are set
	defaultCfg := config.Default()
	assert.Equal(t, defaultCfg.Version, cfg.Version, "Config version should match default")
	assert.Equal(t, defaultCfg.Daemon.Port, cfg.Daemon.Port, "Daemon port should match default")
}

func TestInitCmd_CreatesDatabase(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init command
	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)
	require.NoError(t, err, "Init command should succeed")

	// Verify database file was created
	dbPath := filepath.Join(tmpDir, ".pommel", db.DatabaseFile)
	_, err = os.Stat(dbPath)
	require.NoError(t, err, "Database file should exist")

	// Verify database can be opened and has correct schema
	database, err := db.Open(tmpDir)
	require.NoError(t, err, "Database should be openable")
	defer database.Close()

	// Verify schema version
	ctx := context.Background()
	version, err := database.GetSchemaVersion(ctx)
	require.NoError(t, err, "Should be able to get schema version")
	assert.Equal(t, db.SchemaVersion, version, "Schema version should match")
}

func TestInitCmd_OutputsSuccess(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init command
	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)
	require.NoError(t, err, "Init command should succeed")

	// Should output success message
	output := outBuf.String()
	assert.Contains(t, output, "Initialized", "Output should indicate initialization")
}

func TestInitCmd_JSONOutput(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init command with JSON output
	var outBuf, errBuf bytes.Buffer
	err = runInitCmdJSON(tmpDir, &outBuf, &errBuf)
	require.NoError(t, err, "Init command should succeed")

	// Should output valid JSON
	output := outBuf.String()
	assert.Contains(t, output, "{", "Output should be JSON")
	assert.Contains(t, output, "\"success\"", "JSON should contain success field")
}

func TestInitCmd_InvalidDirectory(t *testing.T) {
	// Try to init in a non-existent directory
	nonExistent := "/nonexistent/path/that/does/not/exist"

	var outBuf, errBuf bytes.Buffer
	err := runInitCmd(nonExistent, &outBuf, &errBuf)

	// Should fail with error
	assert.Error(t, err, "Init should fail for non-existent directory")
}

func TestInitCmd_ReadOnlyDirectory(t *testing.T) {
	// Skip on CI where we might not have permission to create read-only dirs
	if os.Getenv("CI") != "" {
		t.Skip("Skipping read-only directory test in CI")
	}

	// Create a read-only directory
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Make it read-only
	err = os.Chmod(tmpDir, 0444)
	require.NoError(t, err)
	defer os.Chmod(tmpDir, 0755) // Restore permissions for cleanup

	// Try to init in read-only directory
	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)

	// Should fail with error
	assert.Error(t, err, "Init should fail for read-only directory")
}

// runInitCmd is a helper function to run the init command programmatically
// This function should be implemented in init.go
func runInitCmd(projectRoot string, out, errOut *bytes.Buffer) error {
	return runInit(projectRoot, out, errOut, false)
}

// runInitCmdJSON is a helper function to run the init command with JSON output
func runInitCmdJSON(projectRoot string, out, errOut *bytes.Buffer) error {
	return runInit(projectRoot, out, errOut, true)
}
