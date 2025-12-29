package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testProject is a helper that creates a temporary project directory with Pommel config
type testProject struct {
	Dir    string
	Config *config.Config
	Loader *config.Loader
}

// newTestProject creates a new temporary project with default config
func newTestProject(t *testing.T) *testProject {
	t.Helper()

	dir := t.TempDir()
	loader := config.NewLoader(dir)

	cfg, err := loader.Init()
	require.NoError(t, err)

	return &testProject{
		Dir:    dir,
		Config: cfg,
		Loader: loader,
	}
}

// newTestProjectWithConfig creates a new temporary project with custom config
func newTestProjectWithConfig(t *testing.T, cfg *config.Config) *testProject {
	t.Helper()

	dir := t.TempDir()
	loader := config.NewLoader(dir)

	err := loader.Save(cfg)
	require.NoError(t, err)

	return &testProject{
		Dir:    dir,
		Config: cfg,
		Loader: loader,
	}
}

// executeConfigCmd executes the config command with given args and returns output
func executeConfigCmd(t *testing.T, projectDir string, args ...string) (string, string, error) {
	t.Helper()

	// Save original values
	origProjectRoot := projectRoot
	origJSONOutput := jsonOutput

	// Cleanup
	defer func() {
		projectRoot = origProjectRoot
		jsonOutput = origJSONOutput
	}()

	// Set up for test
	projectRoot = projectDir
	jsonOutput = false

	// Check if --json flag is in args
	filteredArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	// Capture output
	var outBuf, errBuf bytes.Buffer
	configCmd.SetOut(&outBuf)
	configCmd.SetErr(&errBuf)

	// Reset command for testing
	configCmd.SetArgs(filteredArgs)

	err := configCmd.Execute()

	return outBuf.String(), errBuf.String(), err
}

func TestConfigCmd_ShowAll(t *testing.T) {
	// Test that 'pm config' with no args shows full config
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir)

	// Should succeed
	require.NoError(t, err, "config command should succeed with no args")

	// Should contain key config sections
	assert.Contains(t, stdout, "daemon", "output should contain daemon section")
	assert.Contains(t, stdout, "embedding", "output should contain embedding section")
	assert.Contains(t, stdout, "watcher", "output should contain watcher section")
	assert.Contains(t, stdout, "search", "output should contain search section")

	// Should show default port value
	assert.Contains(t, stdout, "7420", "output should contain default daemon port")
}

func TestConfigCmd_ShowAllJSON(t *testing.T) {
	// Test that 'pm config --json' outputs valid JSON
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "--json")

	// Should succeed
	require.NoError(t, err, "config command with --json should succeed")

	// Should be valid JSON
	var result map[string]interface{}
	err = json.Unmarshal([]byte(stdout), &result)
	require.NoError(t, err, "output should be valid JSON")

	// Should contain expected sections
	assert.Contains(t, result, "daemon", "JSON should contain daemon")
	assert.Contains(t, result, "embedding", "JSON should contain embedding")
	assert.Contains(t, result, "watcher", "JSON should contain watcher")
	assert.Contains(t, result, "search", "JSON should contain search")

	// Check nested values
	daemon, ok := result["daemon"].(map[string]interface{})
	require.True(t, ok, "daemon should be an object")
	assert.Equal(t, float64(7420), daemon["port"], "daemon.port should be 7420")
}

func TestConfigCmd_Get(t *testing.T) {
	// Test that 'pm config get daemon.port' returns the port value
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "daemon.port")

	// Should succeed
	require.NoError(t, err, "config get daemon.port should succeed")

	// Should output the port value
	assert.Contains(t, stdout, "7420", "output should contain port value 7420")
}

func TestConfigCmd_GetNested(t *testing.T) {
	// Test that 'pm config get daemon' returns the entire daemon section
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "daemon")

	// Should succeed
	require.NoError(t, err, "config get daemon should succeed")

	// Should contain all daemon fields
	assert.Contains(t, stdout, "host", "output should contain host")
	assert.Contains(t, stdout, "port", "output should contain port")
	assert.Contains(t, stdout, "log_level", "output should contain log_level")
	assert.Contains(t, stdout, "127.0.0.1", "output should contain default host value")
	assert.Contains(t, stdout, "7420", "output should contain default port value")
	assert.Contains(t, stdout, "info", "output should contain default log_level value")
}

func TestConfigCmd_GetJSON(t *testing.T) {
	// Test that 'pm config get daemon.port --json' returns JSON
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "daemon.port", "--json")

	// Should succeed
	require.NoError(t, err, "config get daemon.port --json should succeed")

	// Should be valid JSON with the value
	var result map[string]interface{}
	err = json.Unmarshal([]byte(stdout), &result)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, float64(7420), result["value"], "JSON value should be 7420")
	assert.Equal(t, "daemon.port", result["key"], "JSON key should be daemon.port")
}

func TestConfigCmd_Set(t *testing.T) {
	// Test that 'pm config set daemon.port 9000' changes config
	proj := newTestProject(t)

	// Execute set command
	_, _, err := executeConfigCmd(t, proj.Dir, "set", "daemon.port", "9000")

	// Should succeed
	require.NoError(t, err, "config set daemon.port 9000 should succeed")

	// Verify the change by loading config
	cfg, err := proj.Loader.Load()
	require.NoError(t, err)
	assert.Equal(t, 9000, cfg.Daemon.Port, "daemon.port should be updated to 9000")
}

func TestConfigCmd_SetString(t *testing.T) {
	// Test setting a string value
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "daemon.log_level", "debug")

	require.NoError(t, err, "config set daemon.log_level debug should succeed")

	// Verify the change
	cfg, err := proj.Loader.Load()
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.Daemon.LogLevel, "daemon.log_level should be updated to debug")
}

func TestConfigCmd_SetValidation(t *testing.T) {
	// Test that setting an invalid value returns error
	proj := newTestProject(t)

	testCases := []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "invalid port zero",
			key:   "daemon.port",
			value: "0",
		},
		{
			name:  "invalid port negative",
			key:   "daemon.port",
			value: "-1",
		},
		{
			name:  "invalid port too high",
			key:   "daemon.port",
			value: "70000",
		},
		{
			name:  "invalid log level",
			key:   "daemon.log_level",
			value: "invalid",
		},
		{
			name:  "invalid batch size",
			key:   "embedding.batch_size",
			value: "0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := executeConfigCmd(t, proj.Dir, "set", tc.key, tc.value)
			assert.Error(t, err, "setting invalid value should return error")
		})
	}
}

func TestConfigCmd_UnknownKey(t *testing.T) {
	// Test that getting/setting unknown key returns error
	proj := newTestProject(t)

	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "get unknown key",
			args: []string{"get", "unknown.key"},
		},
		{
			name: "get unknown nested key",
			args: []string{"get", "daemon.unknown"},
		},
		{
			name: "set unknown key",
			args: []string{"set", "unknown.key", "value"},
		},
		{
			name: "set unknown nested key",
			args: []string{"set", "daemon.unknown", "value"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := executeConfigCmd(t, proj.Dir, tc.args...)
			require.Error(t, err, "unknown key should return error")
			assert.Contains(t, err.Error(), "unknown", "error should mention unknown key")
		})
	}
}

func TestConfigCmd_SavesPersistently(t *testing.T) {
	// Test that changes persist to disk and survive reload
	proj := newTestProject(t)

	// Set a value
	_, _, err := executeConfigCmd(t, proj.Dir, "set", "daemon.port", "8888")
	require.NoError(t, err)

	// Create a new loader to simulate fresh load
	newLoader := config.NewLoader(proj.Dir)
	cfg, err := newLoader.Load()
	require.NoError(t, err)

	// Verify the value persisted
	assert.Equal(t, 8888, cfg.Daemon.Port, "daemon.port should persist to 8888")

	// Verify config file exists on disk
	configPath := filepath.Join(proj.Dir, config.PommelDir, config.ConfigFileName+"."+config.ConfigFileExt)
	_, err = os.Stat(configPath)
	assert.NoError(t, err, "config file should exist on disk")
}

func TestConfigCmd_NoConfigFile(t *testing.T) {
	// Test behavior when no config file exists
	dir := t.TempDir()

	_, _, err := executeConfigCmd(t, dir)

	// Should fail with meaningful error
	require.Error(t, err, "should error when no config file exists")
	assert.Contains(t, err.Error(), "config", "error should mention config")
}

func TestConfigCmd_InvalidSubcommand(t *testing.T) {
	// Test that invalid subcommand returns error
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "invalid")

	assert.Error(t, err, "invalid subcommand should return error")
}

func TestConfigCmd_SetMissingValue(t *testing.T) {
	// Test that 'set key' without value returns error
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "daemon.port")

	assert.Error(t, err, "set without value should return error")
}

func TestConfigCmd_SetTypeConversion(t *testing.T) {
	// Test that values are correctly converted to appropriate types

	testCases := []struct {
		name     string
		key      string
		value    string
		validate func(t *testing.T, cfg *config.Config)
	}{
		{
			name:  "integer port",
			key:   "daemon.port",
			value: "9999",
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, 9999, cfg.Daemon.Port)
			},
		},
		{
			name:  "string host",
			key:   "daemon.host",
			value: "0.0.0.0",
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "0.0.0.0", cfg.Daemon.Host)
			},
		},
		{
			name:  "integer debounce",
			key:   "watcher.debounce_ms",
			value: "1000",
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, 1000, cfg.Watcher.DebounceMs)
			},
		},
		{
			name:  "large integer max_file_size",
			key:   "watcher.max_file_size",
			value: "10485760",
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, int64(10485760), cfg.Watcher.MaxFileSize)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh project for each test
			testProj := newTestProject(t)

			_, _, err := executeConfigCmd(t, testProj.Dir, "set", tc.key, tc.value)
			require.NoError(t, err)

			cfg, err := testProj.Loader.Load()
			require.NoError(t, err)
			tc.validate(t, cfg)
		})
	}
}

func TestConfigCmd_GetTopLevel(t *testing.T) {
	// Test getting top-level scalar values
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "version")

	require.NoError(t, err, "config get version should succeed")
	assert.Contains(t, stdout, "1", "output should contain version 1")
}

func TestConfigCmd_SetConfirmation(t *testing.T) {
	// Test that set command outputs confirmation
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "set", "daemon.port", "9000")

	require.NoError(t, err)
	// Should output some confirmation message
	assert.NotEmpty(t, stdout, "set command should output confirmation")
}
